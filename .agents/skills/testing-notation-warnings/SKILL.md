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

## Verify oracle baselines against a LIVE run, not just against the docs

`cmd/pilot-diff/doc_counts_test.go` guards documentation prose against the **committed**
`docs/project/pilot-xpect-baseline.json` / `pilot-rejection-baseline.json`. It therefore cannot
notice that both the prose *and* the committed baseline have drifted away from what the code now
does — they stay self-consistent while both go stale, and `go test ./...` stays green. A change
that makes notation errors non-blocking moves the xpect numbers, because findings that used to be
gated now surface.

So always run the oracle and diff the totals yourself:

```bash
go run ./cmd/pilot-xpect -jobs 8
cmp build/pilot-xpect/pilot-xpect.json docs/project/pilot-xpect-baseline.json   # may legitimately differ
python3 -c "
import json
b=json.load(open('docs/project/pilot-xpect-baseline.json'))['totals']
l=json.load(open('build/pilot-xpect/pilot-xpect.json'))['totals']
[print(k,'baseline',b[k],'live',l.get(k),'<-- DIFFERS' if b[k]!=l.get(k) else '') for k in b]"
```

Watch `agree`, `disagree` and especially `foreignDiagnostics` — a drop of the last to 0 means
diagnostics that were previously attributed elsewhere are now landing on the expected rows.
`docs/project/pilot-xpect.md` carries the same numbers in prose (`agree N | disagree N | ...`),
so grep for the live figures there too. Report a mismatch as stale-baseline, not as a test failure.

## A new notation code can add strict-mode errors to OMG-published files

A notation rule that re-derives its finding from the lexer's keyword set rather than from what the
parser recovered fires on legal text: `enum = 60 [mm];` inside an `enum def`
(`examples/pilot-corpora/sysml-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml`)
and `attribute type : String[0..1];` in the bundled stdlib both became `-strict` errors that way,
refusing files that validate clean. `enum = <value>;` is an unnamed usage the parser models as a
member named `enum` (hence its `Duplicate of other owned member name` warning in every build), so a
keyword-as-name rule that trusts the tree inherits the mis-parse; `keywordAsName` now escalates only
the spans the parser itself reported as recovered. Treat any new notation code as suspect until you
have run it over the stdlib and all pilot corpora **in strict mode** and inspected each hit by hand:

```bash
sysml -validate -debug -strict "<file>"   # per file; gains under OMG/stdlib roots are the red flag
```

A gain under an OMG root or the stdlib is not automatically a bug, but it is never "expected" —
adjudicate it against the source text before calling it correct.

## Every diagnostic from the notation pass is Notation, whatever its code

Do not build a hardcoded list of "notation codes" for a differential harness. The pass marks its
whole output in one loop (`for i := range w.diags { w.diags[i].Notation = true }`), so
`nonstandard-notation`, `kerml-notation`, `sysml-notation` **and** `reserved-keyword-name` are all
notation. A hand-maintained set will misclassify one of them and produce a false
"NON-NOTATION ESCALATION" alarm. Grep the pass instead:

```bash
grep -rn "Notation:\s*true\|\.Notation = true" --include=*.go internal/ | grep -v _test
grep -n "Code[A-Za-z]* =" internal/core/passes/nonstandard_notation.go
```

## Surface and flag names that waste time

- The CLI strict flag is **`-strict`**, not `-conformance strict`. The latter is a flag-parse error:
  it dumps usage and exits 2, which looks exactly like a legitimate refusal and will silently fake
  a "strict refuses" pass. Confirm with `sysml -h | grep -i strict`. (`cmd/pilot-reject` *does*
  take `-conformance default|strict` — the two binaries differ.)
- There is no `-check` flag and no `%run` command. Use the check flags
  (`-instantiate`, `-constraint`, `-satisfy`, `-query`, ...) and `%strict on|off`.
- REPL `%load` goes through `Session.LoadFile`, **not** `LoadFileSummary`, so it does not exercise
  `renderSyntax`. Only `sysml -validate <file>` (via `cmd/sysml/check.go`) does. If you are asked to
  prove a load-time rendering fix, `%load` is a regression control, not the discriminator — say so
  rather than reporting it as evidence of the fix.

## Measure "exactly once" against a build that gets it wrong

For a de-duplication fix, keep the pre-fix binary and count on all three builds
(`main`, pre-fix, fixed). A bare "1 occurrence" on the fixed build is vacuous — 1 is also what you
get if the diagnostic is emitted once for an unrelated reason, or if your grep pattern is too
narrow. The useful shape is baseline **1**, pre-fix **2**, fixed **1**:

```bash
for b in main prefix fixed; do
  echo "$b: $(/tmp/sysml-$b -validate /tmp/loadbare.sysml 2>&1 | grep -c 'visibility indicator')"
done
```

Also check *position*, not just count: the pathology is the diagnostic printed **before** the
`✓ package ...` summaries (i.e. as a reason the file could not be read) and then again after.

## Recording CLI/REPL work on this box

`DISPLAY=:0` (not `:1`; check `ls /tmp/.X11-unix`). Konsole's font shortcut `ctrl+shift+plus` types
literal `+` characters through xdotool instead of resizing, so set the font at launch and maximize
with wmctrl:

```bash
DISPLAY=:0 konsole --hide-menubar -p "Font=Monospace,15" --workdir /path/to/repo &
DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Under `-strict` a refused file prints no `✓ package ...` summary at all — that is the refusal path,
not a regression of the declared-members summary. Compare summaries in **default** mode.

## Devin Secrets Needed

None. Everything runs against the locally provisioned `build/pilot-validator` and the committed
corpora; network access is needed only to re-provision them.
