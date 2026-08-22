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

## Devin Secrets Needed

None. Everything runs against the locally provisioned `build/pilot-validator` and the committed
corpora; network access is needed only to re-provision them.
