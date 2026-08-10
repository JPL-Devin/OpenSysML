---
name: testing-sysml-cli
description: How to validate hand-written SysML/KerML models end-to-end with the built CLI (bin/sysml), read its diagnostics, and avoid the stdlib name-collision trap that silently makes imports unresolvable.
---

# End-to-end validation with the Systemica CLI

## Build

```bash
export PATH=/usr/local/go/bin:$PATH
make build          # -> bin/sysml (REPL/validator), bin/sysml-lsp, bin/sysml-grpc
```

## Validate a model file non-interactively

`bin/sysml` is a REPL; there is no `validate` subcommand. The trick is to combine a
file argument with `-e` so it loads the file, prints diagnostics, evaluates, and exits:

```bash
bin/sysml -e 1 model.sysml
```

Internally this is `%load` -> `repl.Session.Submit` -> `model.Workspace.Diagnostics`
(`internal/repl/meta.go`, `internal/repl/session.go`), i.e. the full tiered pass
pipeline including name resolution (`internal/core/passes/nameres.go`).

Output shape:
- Clean model: one summary line per declared top-level element (`package Foo`),
  then the `-e` result (`✓ 1` / `  = 1`).
- Problem: `L:C: error: <message>` followed by the source line and a `^~~~` caret
  (`internal/repl/render.go`). Name-resolution failures read
  `unresolved reference: <Qualified::Name>`.
- Exit code is 0 even when diagnostics are reported — count `error:` lines instead
  of relying on `$?`: `bin/sysml -e 1 m.sysml | grep -c 'error:'`.

## Trap: your package names collide with the stdlib

The workspace always loads the SysML/KerML standard library, so common names such as
`Base`, `Core`, `Kernel`, `Metaobjects`, `Occurrences`, `Parts`, `Items`, `ISQ` already
exist. If a test fixture declares `package Base { ... }` and another package does
`import Base::*`, the wildcard target is *ambiguous* and
`symbols.Index.resolveWildcardTarget` silently drops it — so **nothing** resolves
through that import and you get confusing "unresolved reference" errors unrelated to
the change under test. Always give fixtures unique names (`MyBase`, `Demo1`, …).

## Proving a resolution/visibility change is actually the cause

Diagnostics alone don't tell you whether a failure is new. Build a baseline binary from
the merge base in a throwaway worktree and diff the CLI output:

```bash
git worktree add /tmp/base <merge-base-sha>
(cd /tmp/base && go build -o /tmp/base/sysml-base ./cmd/sysml)
# ... compare /tmp/base/sysml-base vs bin/sysml on the same fixture ...
git worktree remove --force /tmp/base     # clean up afterwards
```

Also add a *control* reference to a genuinely nonexistent name in the same scope
(e.g. `part w : Mid::NoSuchThing;`). If the control errors while the name under test
stays clean, you have proved resolution really ran there rather than being skipped.

## Known pre-existing noise (not regressions)

`examples/parser_features_demo_action_semantics.sysml` (~19 errors) and
`examples/phase-c-behavioral-bodies.sysml` (~59 errors) report errors on `main` too.
Use `examples/state-machine-demo.sysml`, `action-executor-demo.sysml`,
`combined-behavioral-demo.sysml`, `pseudostates-demo.sysml` as clean regression baselines.

## Devin Secrets Needed

None.
