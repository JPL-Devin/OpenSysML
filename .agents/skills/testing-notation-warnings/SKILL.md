---
name: testing-notation-warnings
description: How to end-to-end test a new `nonstandard-notation` warning in internal/core/passes/nonstandard_notation.go — which surfaces show it, how to build non-vacuous false-positive scans over the four OMG corpora, how strict-conformance escalation is observed, and how to referee the boundary against the pinned pilot validator.
---

# Testing a new `nonstandard-notation` warning

`internal/core/passes/nonstandard_notation.go` emits one code, `nonstandard-notation`, at
`LevelSyntax`. Each rule matches an AST node shape and reports
``<spelling> is an OpenSysML extension with no SysML v2 production: <advice>``. Testing a newly
added rule means proving four things: it fires at the user-facing surfaces, it does **not** fire on
the legal spellings, strict mode escalates it, and the pilot reference agrees on which side of the
boundary each spelling sits.

## The CLI strict flag is `-strict`, not `-conformance strict`

`bin/sysml` has **no** `-conformance` flag: asking for one prints
`flag provided but not defined: -conformance` plus the whole usage block and exits **2**, which
reads exactly like the model failing to analyse. `-conformance auto|default|strict` belongs to
`cmd/pilot-reject` (and the other pilot harnesses) only. The CLI spelling is `-validate -strict`,
the REPL's is `%strict on`, and the LSP's is the `strictConformance` initialization option.

## Severity-only claims must be tested against the pass tiers, not just the message

`nonstandard-notation`, `import-visibility` and friends live at `LevelSyntax`, and the pass
runner **skips the higher tiers when a lower tier errors** (AGENTS.md §4). A notation finding
therefore sets `Diagnostic.Notation`, which exempts it from that gate; without the flag, promoting
a syntax-tier finding to error silently deletes every name-resolution, type and constraint
diagnostic in the document. Test it like this, always against a `main` binary built from a
worktree so the comparison is real:

```bash
git worktree add /tmp/osm-main main && (cd /tmp/osm-main && go build -o /tmp/sysml-main ./cmd/sysml)
cat > /tmp/blast.sysml <<'EOF'
package Q { part def A; }
package R {
  import Q::*;          # the newly-escalated construct
  part x : NoSuchType;  # a higher-tier finding that must survive
}
EOF
/tmp/sysml-main -validate -debug /tmp/blast.sysml | grep -cE 'error:|warning:'   # e.g. 2
bin/sysml        -validate -debug /tmp/blast.sysml | grep -cE 'error:|warning:'   # 1 ⇒ regression
```

`-debug` is what makes this legible: it appends `[tier/code]` to every line, so you can see which
tier stopped producing output. A count that drops is a lost diagnostic, not a severity move.

## "No diagnostics lost" is not enough — also check what the model can still *do*

A severity move can preserve every diagnostic and still change behavior, because several gates
ask "is there an error?" independently of the pass runner. Grep for them before believing a
severity-only claim; as of wave 10C the predicate is `Diagnostic.Blocking()`
(`SeverityError && !Notation`) and the gates that use it are `Registry.Run`'s `hasError` and the
REPL's `hasError` + `analysisBlocked` (`internal/repl/render.go`), while these deliberately keep a
**raw** `SeverityError` check:

- `internal/repl/run.go`'s `Session.hasAnalysisErrors` → `HasErrors()`, which gates the CLI check
  flags (`cmd/sysml/check.go`), `%query` (`internal/repl/oslc_query.go`) and `LoadReport.Errors`
- `internal/core/edit/validate.go`'s `errorsOnly`

```bash
grep -rn 'SeverityError' --include=*.go internal/ cmd/ | grep -v internal/core/passes/
```

So test three distinct things, not one: (1) no diagnostic is lost, (2) the REPL still prints its
`✓ <member>` declared-members summary, and (3) the model can still be *used* — instantiated and
checked. A notation error that blocks (3) is a change to model use even though every diagnostic
is still reported.

The trap to watch for: a diagnostic that is **both** `Notation` and `SeverityError by default`
will disagree with `Blocking()` on every raw-severity gate. Find them with

```bash
grep -rn -B8 'Notation:\s*true\|Notation\s*=\s*true' --include=*.go internal/core/passes/ | grep -E 'Severity|func '
```

Historically `import-visibility` is the only such case (`nonstandard-notation` is a warning by
default and an error only under strict), which makes "a file whose only error is a bare `import`"
the single fixture that exposes this class of bug. Build it so the bare import is the *only*
finding — declare the imported package, and qualify library types as `ScalarValues::Natural`, or
an unresolved-reference error will mask the effect on both binaries:

```sysml
package Q { part def Helper; }
package P {
  import Q::*;                                    // the ONLY finding
  part def Widget {
    attribute mass : ScalarValues::Natural = 5;
    constraint massOK { mass > 3 }
  }
}
```

```bash
# must behave the same on main and on the branch in DEFAULT mode
$BIN -instantiate 'P::Widget' -constraint 'P::Widget::massOK' /tmp/barecheck.sysml; echo $?
# 0 + "✓ Constraint ... passed"  vs  2 + "did not analyse cleanly; no check was made"
```

Note the asymmetry that makes this subtle: `cmd/sysml/strict_test.go` legitimately requires a
**strict**-mode notation error to exit 2, so the raw check cannot simply be swapped for
`Blocking()`. Any fix has to distinguish default from strict rather than treating all notation
alike.

## Surface names in this repo (do not trust a brief's shorthand)

There is **no `-check` flag** and **no `%run` REPL command**, though both get referred to that way.
Verify with `sysml -h` and `%help` before building a plan around them:

- the "check" surface is the *check flags* — `-instantiate`, `-constraint`, `-satisfy`,
  `-requirement`, `-calc`, `-action`, `-state`, `-eval`, `-query` — whose refusal message is
  `"... did not analyse cleanly; no check was made"`
- the REPL has `%query` (OSLC), `%load`, `%print`, `%strict`, `%trace`, … and `%query` needs real
  OSLC syntax (`oslc.where=...`); a bare name is rejected by the parser *before* the error gate,
  which silently makes the probe vacuous

## Corpus differentials: one file per process, parallel, and prove non-vacuity

`-validate` accepts many files at once, but batching lets them resolve against each other and
invents cross-file diagnostics — keep one file per invocation and parallelize instead. At ~60 ms
per file, 429 files × 2 binaries × 2 modes finishes in well under a minute:

```bash
find examples internal/core/libs/stdlib -name '*.sysml' -o -name '*.kerml' | sort > /tmp/files.txt
xargs -a /tmp/files.txt -d '\n' -P8 -I{} /tmp/scan.sh /tmp/sysml-pr "" {} > /tmp/diag_pr.txt
```

Compare severity-insensitively (`sed -E 's/: (error|warning|note): /: /' | sort -u`) so a severity
move is not counted as a loss plus a gain. Two vacuity controls are mandatory, or the scan proves
nothing: the gained count must be **> 0**, and the OMG/stdlib roots must be demonstrably scanned —
report files-per-root alongside rows-per-root, because those roots are nearly clean and a broken
path glob looks exactly like a clean result.

## Driving the LSP surface without an editor

The third surface is worth a real check, and a 30-line JSON-RPC client is enough — it also proves
the strict option is read, which the CLI cannot:

```python
import json,subprocess,time
def fr(o):
    b=json.dumps(o).encode(); return b"Content-Length: %d\r\n\r\n"%len(b)+b
msgs=[{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":None,"rootUri":None,
        "capabilities":{},"initializationOptions":{"strictConformance":True}}},
      {"jsonrpc":"2.0","method":"initialized","params":{}},
      {"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{
        "uri":"file:///tmp/f.sysml","languageId":"sysml","version":1,"text":open("/tmp/f.sysml").read()}}}]
p=subprocess.Popen(["bin/sysml-lsp"],stdin=subprocess.PIPE,stdout=subprocess.PIPE)
for m in msgs: p.stdin.write(fr(m))
p.stdin.flush(); time.sleep(3); p.stdin.close()
for chunk in p.stdout.read().split(b"Content-Length: "):
    if b"publishDiagnostics" in chunk:
        for d in json.loads(chunk.split(b"\r\n\r\n",1)[1])["params"]["diagnostics"]:
            print(d["severity"], d["range"]["start"], d.get("code"))
```

LSP `severity` is **1 = error, 2 = warning**, so strict escalation shows up as the same spans
flipping 2 → 1. Omit `initializationOptions` for the default-mode run.

## Proving strict moves severity and nothing else

Diff the two runs with the severity word normalised; only the trailer may differ:

```bash
diff <(bin/sysml -validate f.sysml 2>&1 | sed 's/warning:/SEV:/') \
     <(bin/sysml -validate -strict f.sysml 2>&1 | sed 's/error:/SEV:/')
```

A clean diff over the diagnostic lines is the assertion — same message, same `line:col`, same
caret line. Eyeballing two screenshots is not.

## `TestStdlibConformance` is a parser-only gate

It parses the 95 embedded stdlib files and asserts zero **parse** diagnostics against
`testdata/stdlib_known_failures.txt`; it never runs the passes. So it is *not* evidence that a new
pass rule or a severity change leaves the stdlib alone. For that, grep the stdlib for the
construct directly (e.g. `grep -rlE '^[[:space:]]*import ' internal/core/libs/stdlib` returns 0
files, which is the real reason an import-visibility change cannot touch it).

## Recording the CLI/REPL surfaces

There is no xterm on the box; `konsole` is the terminal emulator that exists. Launch, focus and
maximise it before recording, then zoom the font so the long messages stay readable:

```bash
DISPLAY=:0 nohup konsole --hide-menubar --hide-tabbar --workdir "$PWD" -e /bin/bash &
DISPLAY=:0 wmctrl -a Konsole && DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Zoom in with `xdotool key ctrl+plus` (**not** `ctrl+shift+plus`, which types literal `+`
characters into the shell); ~112 columns is a good target. The diagnostic messages are ~180
characters and will wrap — that is fine, but it means a `clear` before each command is worth it.

## Surfaces that show the warning (and the one that is language-blind)

| Surface | Command | Notes |
|---|---|---|
| CLI, single file | `bin/sysml -validate <f>` | GNU-format `path:line:col: warning: …` plus a caret line. Exit **0** when warnings are the only findings, and it still prints `✓ <f>: no errors`. |
| CLI, strict | `bin/sysml -validate -strict <f>` | same lines as `error:`, then `sysml: <f> did not analyse cleanly; no check was made`, exit **2**. |
| REPL | pipe snippets into `bin/sysml`, then `%strict on` | The buffer is named `<repl>` and has **no file kind**, so this is the surface that proves the rule is not gated on the file extension. `%strict on` **re-reports the whole buffer** at the new severity, which is the cheapest strict-mode proof. |
| differential | `build/pilot-diff/pilot-diff.txt` | `opensysml:` message lines, grouped under root + file. |

**There is no `%validate` meta-command** — the REPL analyses on every submit, so just submit the
snippet and read the diagnostics. Asking for `%validate` prints
`unknown command "%validate" (try %help)`, which is easy to mistake for the rule not firing.

Driving the REPL non-interactively (`%%` is needed because `printf` eats a single `%`):

```bash
printf 'calc d { in a; return a * 2.0; }\nconstraint k { in x; assert x >= 0; }\n%%strict on\n%%quit\n' | bin/sysml
```

## The notation pass runs even when the file has resolution errors

The passes are tiered, but `nonstandard-notation` is a **syntax-tier** rule, so it is emitted even
when the same file collects `unresolved reference: …` name-resolution errors. That matters a lot for
scanning: a bare `package P { … : Real … }` with no `private import ScalarValues::*;` yields
`unresolved reference: Real` **and** the notation warnings. So a single-file `bin/sysml -validate`
over a corpus file that imports siblings is still a valid surface for *presence/absence of a
notation message*, even though it is not a valid surface for the file's overall cleanliness.

Use an explicit `private import ScalarValues::*;` only when you need the file to analyse **cleanly**
(i.e. for the "exit 0 in default mode / exit 2 under `-strict`" assertion).

## False-positive scan over the four OMG corpora — and making it non-vacuous

The OMG-authored corpora are asserted clean, so **any** hit is a defect. Roots:
`examples/pilot-corpora` (includes `sysml-examples` and `kerml-examples`),
`examples/sysml-v2-training`, and the stdlib `internal/core/libs/stdlib`.

```bash
while IFS= read -r f; do
  bin/sysml -validate "$f" 2>&1 | grep -E "<the new message text>" | sed "s|^|HIT |"
done < <(grep -rl --include=*.sysml --include=*.kerml -E '(^|[^A-Za-z_])(return|assert|assume) ' \
           examples/pilot-corpora examples/sysml-v2-training internal/core/libs/stdlib)
```

- Use `grep -rl` on the *keyword* to pick candidate files (99 files for `return`/`assert`/`assume`),
  then grep the *validator output* for the message — grepping the source for the message is
  meaningless.
- Anchor the keyword grep with `(^|[^A-Za-z_])` or you match `returnValue`, `asserted`, etc.
- **A zero-hit scan proves nothing on its own.** Always run the identical `grep -E` against a
  known-positive file and show a non-zero count (`examples/repl-behavioral-demo.sysml` gave 12).
  Without that control, a typo in the message pattern looks exactly like a clean corpus.
- Independent cross-check: `awk` the differential report to attribute every occurrence of the new
  message to its root and file, and confirm no OMG root appears:
  ```bash
  awk '/^[a-z-]+ \(/{root=$0} /^  [^ ]/{file=$0} /<message>/{print root " ||" file}' \
      build/pilot-diff/pilot-diff.txt | sort | uniq -c
  ```

## The boundary fixture must contain the positive twins

A fixture of only-legal spellings that stays silent is indistinguishable from a rule that never
fires. Put the must-warn and must-stay-silent shapes in **one** file and assert the exact set of
warned line numbers. For the `return` / `assert` family the shapes that matter:

- silent: `return a;`, `return P::q;`, `return (a);` (the parser unwraps the parens, so a
  parenthesised bare reference is still a reference expression), `return result : Real = a;`
  (a result *parameter declaration*, not a computed result), a keyword-less trailing condition
  `{ in x : Real; x >= 0 }`, `assert constraint c1 : C;`, `assert satisfy R by q;`,
  `assume #goal constraint m;`
- warned: `return a * 2.0;`, `return 42;`, `assert x >= 0;`, `assert not x < 0;`, `assume x >= 0;`

## Refereeing the boundary against the pinned pilot

`build/pilot-validator/validate-sysml <f>` (~15 s, capture `$?` **without** a pipe) is the oracle
for "does a production admit this spelling". The interesting asymmetry:

- pilot **exit 1** with `no viable alternative at input '<keyword>'` + our warning ⇒ correct.
- pilot **exit 0** + our warning ⇒ **false positive of the rule**, the worst outcome.
- pilot **exit 1** + our **silence** ⇒ a *coverage gap*: the reference has no production for it and
  we accept it silently. Not a regression, but exactly what such a PR is meant to remove, so hunt
  for these deliberately.

Note the pilot's syntax error recovery cascades (`missing '}' at 'a'`, `extraneous input '}'
expecting EOF`) and it also emits its own `Duplicate of other owned member name` warnings on
`calc c { in a : Real; return a; }` — judge the verdict by the presence of the
`no viable alternative at input '<keyword>'` line and the exit code, not by output volume.

## Requirement-body members are a *different* AST node from constraint-body members

The single most likely coverage gap when a rule covers "assert/assume conditions": a condition in a
**constraint** body is `*ast.ConstraintMember` (fields `Keyword`, `IsNegated`, `Expression`, `Name`,
`Body`), but the same-looking condition in a **requirement** body is `*ast.AssumeMember` /
`*ast.RequireMember` (`internal/core/ast/behavior.go`, "Phase C2: Requirement Body Members"), with a
different field set. A rule keyed on `*ast.ConstraintMember` therefore covers
`constraint c { assert x >= 0; }` and misses `requirement r { assume x > 0; }` and
`requirement r { require x > 0; }` — both of which the pinned pilot rejects with
`no viable alternative at input 'assume'` / `'require'`. `examples/phase-c-behavioral-bodies.sysml`
contains both families, so always grep the whole keyword inventory of a fixture
(`grep -n 'assert \|assume \|require '`) and reconcile *every* line against warned/not-warned rather
than only checking the lines the task named.

Likewise `*ast.ResultMember` (`return <expr>;` in a calc body) is separate from any
requirement/action result spelling.

## Devin Secrets Needed

None. Everything runs against the locally provisioned `build/pilot-validator` and the committed
corpora; network access is needed only to re-provision them.
