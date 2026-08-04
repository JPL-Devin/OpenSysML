# AGENTS.md — Guide for AI Agents Working on Systemica

> **Prime directive:** If I wanted hacks, I'd write it myself. **Don't ever choose hacky over correct.**
> Fix root causes upstream, not symptoms. Never weaken, skip, or delete tests to make them pass.

Systemica is a production-grade SysML v2 implementation in **Go 1.23+** (module `github.com/Open-MBEE/Systemica`).
It provides a hand-written lexer/parser, semantic engine, execution runtime, LSP server (`sysml-lsp`), and REPL (`sysml`).

---

## 1. Golden Rules

1. **Correctness over expedience.** No shortcuts, no stubs left behind, no lossy conversions. If a proper fix is large, do it properly or stop and flag it.
   - **For features specifically: do not minimize code changes or dodge complexity.** Implement the feature fully and correctly even if it touches many files, adds new types, or requires refactoring. Completeness beats diff size. See §9.
2. **Root-cause first.** Before editing, confirm *why* something fails (read the code, add a temporary debug print, write a focused test). Then make the minimal correct change.
3. **Never regress.** `main` is green. Any test passing on `main` must still pass on your branch. Diff against `main` if unsure: `git stash && git checkout main && go test ./... ; git checkout - && git stash pop`.
4. **Respect the architecture invariants** (see §4). The AST is immutable; semantics live in side tables; execution consumes lowered IR — do not bypass these.
5. **Tests are the contract.** Existing tests encode intended behavior (including *when* and *where* errors surface). Make code satisfy tests, not the reverse — unless the test is provably wrong, in which case explain before changing it.
6. **Leave no dead code.** Remove superseded helpers/structs. Run `go vet ./...` to catch it.

---

## 2. Build, Test & Verify

Use the Makefile (preferred) or raw `go` commands. **Never `cd`** — run from repo root.

```bash
make build            # build bin/sysml and bin/sysml-lsp (with version ldflags)
make build-sysml      # REPL binary only
make build-lsp        # LSP binary only
make test             # full suite: go test -race -coverprofile ... ./...
make test-short       # faster, no race detector
make clean            # remove build artifacts
```

Raw equivalents / targeted runs:

```bash
go build ./...                                   # must always be clean
go vet ./...                                     # must be clean (catches unused/dead code)
go test ./...                                    # all tests
go test ./internal/core/runtime/...             # one package tree
go test -run TestExecutionConformance ./internal/core/runtime
go test -race ./...                             # race detector (CI runs this)
gofmt -l .                                      # must print nothing (CI enforces gofmt)
```

**Definition of done for any change:** `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), and `go test ./...` all pass. Paste the results.

---

## 3. Repository Map

```
cmd/
  sysml/                 REPL binary
  sysml-lsp/             LSP server binary
internal/core/
  source/                source files, spans, line indexing
  lexer/                 hand-written scanner (~200 keywords)
  parser/                recursive-descent parser (never panics; emits ErrorNodes)
  ast/                   syntax tree nodes — IMMUTABLE after parse
  symbols/               symbol tables, scope trees
  resolve/               lazy, memoized name resolution
  semantics/             type system, conformance, multiplicity, const-folding eval
  passes/                tiered validation (syntax → nameres → type → constraint)
  lower/                 AST → execution IR (ActionGraph, StateGraph) for the runtime
  runtime/               execution engine (eval, instances, action/state executors)
  model/                 workspace / document management
  libs/                  stdlib bundling + conformance gate
  deps/                  dependency resolution
internal/lsp/            LSP protocol implementation
internal/repl/           REPL loop
testdata/                shared fixtures (.sysml, .kerml, .golden)
examples/                example models and demos
docs/                    ARCHITECTURE.md, SPEC_COMPLIANCE.md, QUICKSTART.md, etc.
```

Read `docs/ARCHITECTURE.md` before non-trivial work — it documents the pipeline, tiers, and test contracts in depth.

---

## 4. Architecture Invariants (do not violate)

- **Immutable AST.** `internal/core/ast` is syntax-only and is never mutated after parsing. All derived/semantic data lives in **side tables keyed by node/symbol**.
- **Parser never fails.** `parser.New(src).ParseFile()` always returns a tree; malformed input yields `ErrorNode`s + diagnostics, never a panic.
- **Lazy + memoized semantics.** Name resolution and type queries compute on demand and cache. Don't force eager work.
- **Tiered passes.** Higher validation tiers are skipped when a lower tier errors. Keep passes independent and level-scoped.
- **Runtime consumes lowered IR.** Executors should operate on `internal/core/lower` graphs (`ActionGraph`/`StateGraph`) as the single source of truth — do **not** re-parse `symbol.Decl` inside executors, and do not build parallel/duplicate structures that can drift. Lowering must be lossless (carry guards, triggers, effects, pseudostate edges).
- **Error timing is part of the contract.** Constructors (`newActionExecutor`, `newStateExecutor`) succeed on structurally-empty inputs; "no initial node/state" errors surface at `initialize()`. Don't move error points without updating the corresponding tests intentionally.

---

## 5. Testing Strategy

### 5.1 Parser features — four-layer contract
When touching the lexer/parser or adding grammar:
1. **Conformance gate:** `go test -run TestStdlibConformance ./internal/core/libs` — all official stdlib files must still parse clean (no regressions).
2. **Golden ASTs:** `go test -run TestGolden ./internal/core/parser`. Add a representative fixture under `internal/core/parser/testdata/parse/*.sysml`.
3. **Negative tests:** `go test -run TestNegative ./internal/core/parser` — malformed input must produce diagnostics without panicking.
4. **Update goldens only after intentional changes:** `go test -run TestGolden -update ./internal/core/parser`, then review the diff carefully.

### 5.2 Behavioral features (actions/states/calc/constraints/requirements) — four-layer contract
1. **Golden AST fixture** locking parse structure (`internal/core/parser/testdata/parse/`).
2. **Execution conformance:** add `.sysml` + `.expected.json` under `internal/core/runtime/testdata/conformance/`; run `go test -run TestExecutionConformance ./internal/core/runtime`. Schema is documented in that dir's `README.md`.
3. **Golden execution traces** for ordering-sensitive behavior (fork/join, transitions): `go test -run TestExecutionTrace ./internal/core/runtime` (update flag: `-update-traces`).
4. **Robustness:** add a failure-mode case to `robustness_test.go` (deadlock, unbound params, missing refs, dangling transitions, step budget). Must return typed errors, never panic or hang.

Then update `docs/SPEC_COMPLIANCE.md` mapping: semantic rule → implementation (file:function) → test → status (✅ faithful / ⚠️ approximate / ❌ not implemented / 🚧 known failure).

### 5.3 General
- Unit tests live beside code as `*_test.go`, one concern per test.
- Design/adjust tests **before or alongside** implementation; don't retrofit weak tests afterward.
- Prefer real SysML models in `testdata/` over hand-built ASTs when exercising end-to-end behavior; hand-built ASTs are fine for targeted unit tests.

---

## 6. Development Workflow

1. **Understand first.** Grep/read the relevant package and its tests. Diff the branch against `main` to see what changed and why.
2. **Reproduce.** Run the failing test(s) and read the exact error before changing anything.
3. **Locate the root cause** in the correct layer (lexer vs parser vs lower vs runtime). Bugs in specialized layers are often upstream of where they surface.
4. **Implement the minimal correct fix.** Keep edits scoped; match existing style; keep imports at the top.
5. **Add/adjust tests** to lock in the fix and cover the failure mode.
6. **Verify** with the full gate in §2. Remove any temporary debug code and dead code.
7. **Commit** using Conventional Commits (see §7). Keep PRs focused (one feature/fix each).

---

## 7. Code Style & Commits

- **Formatting:** `gofmt` is mandatory (CI-enforced). Follow *Effective Go*. Document exported types/functions.
- **Comments:** don't add or remove comments/docs unrelated to your change.
- **Commit messages — Conventional Commits:** `<type>(<scope>): <description>`
  - types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`
  - e.g. `fix(runtime): preserve transition effects when lowering state graph`

---

## 8. Common Pitfalls

- **Bridge/adapter shims that copy IR back into legacy fields** — these drift and lose data (e.g. dropping a transition's `Effect`). Migrate consumers to the IR instead.
- **Re-parsing `symbol.Decl` inside executors** — bypasses the lowering layer; use the graph.
- **Hard-failing in `ToActionGraph`/`ToStateGraph` on missing initial** — breaks the constructor-succeeds/`initialize()`-errors contract.
- **Forgetting `-update`/`-update-traces`** after an intentional golden change, or running them blindly and masking a real regression — always review the golden diff.
- **Leaving unused structs/functions** after a refactor — `go vet ./...` and clean them up.
- **Assuming where states/members live** (Members vs Substates vs Regions) — verify against actual parser output.

See `CONTRIBUTING.md` and `docs/ARCHITECTURE.md` for the authoritative, detailed references.
