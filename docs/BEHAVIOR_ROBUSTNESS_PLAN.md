# Behavior Robustness & Correctness Implementation Plan

**Status:** Proposed
**Audience:** Engineers/agents working on `internal/core/parser/behavior.go`, `internal/core/runtime` (action/state executors, calc/constraint/requirement evaluation), and their tests.
**Companion doc:** `docs/PARSER_ROBUSTNESS_PLAN.md` (structural parser work). This plan applies the same *measurable-safety-net-first, then root-cause* philosophy to **behaviors** — both their **parsing** and their **execution**.

---

## 0. Context: Why This Work Exists

Two distinct robustness gaps exist for behaviors:

1. **Behavioral parsing is still coverage-driven (whitelist gating).** Unlike the general member grammar, the behavioral body parsers each enumerate an anticipated set of member keywords and emit a *terminal error* on anything else. This is the same anti-pattern the parser plan targeted, and it is now **concentrated in `behavior.go`**. It passes today only because the 94 stdlib files happen to use enumerated forms. Concrete evidence:
   - `parseRequirementMember` still ends in a terminal `expected 'subject', 'assume', 'require', 'actor'...` error and only falls back to general parsing when `atDefUsageStart()` is true — `internal/core/parser/behavior.go:1542-1565`.
   - `parseConstraintBody` was patched by *adding* `return` to its keyword whitelist rather than unifying — `internal/core/parser/behavior.go:1444-1458`.
   - `parseActionMember` (`:402`), `parseStateMember` (`:1827`), and `parseStateBody`/transition parsers follow the same enumerate-then-error shape.
   - Leftover debug cruft: `_ = tok // Keep for debugging` at `internal/core/parser/behavior.go:1533-1535`.

2. **Behavioral execution has no oracle-based conformance harness.** Parsing clean ≠ executing correctly. The runtime has substantial unit tests (13 test files) but **no golden execution traces and no oracle comparison**. There is exactly one runtime fixture (`internal/core/runtime/testdata/simple_calc.sysml`). So a change that silently alters token-flow ordering, state-visit sequence, or event scheduling can pass CI. This is the execution-side analog of the "zero diagnostics ≠ correct AST" problem.

**Guiding principle (same as parser plan):** *If I wanted hacks, I'd write it myself. Don't ever choose hacky over correct.* Prefer unifying grammar rules and building oracles over adding another keyword branch or a bespoke assertion.

### Reference facts (verified — do not re-derive)
- **Behavioral parsers** in `internal/core/parser/behavior.go`: `parseCalcBody` (`:11`), `parseActionBody` (`:218`), `parseActionMember` (`:402`), `parseConstraintBody` (`:1432`), `parseConstraintMember` (`:1476`), `parseRequirementBody` (`:1518`), `parseRequirementMember` (`:1530`), `parseStateBody` (`:1815`), `parseStateMember` (`:1827`), `parseTransitionMember` (`:2333`).
- **Runtime execution APIs** on `internal/core/runtime/context.go`: `ExecuteAction` (`:310`), `ExecuteState` (`:337`), `CreateActionExecutor` (`:379`), `CreateStateExecutor` (`:395`), `InvokeCalc` (`:228`), `EvaluateConstraint` (`:81`), `EvaluateRequirement` (`:148`).
- **Executors:** `ActionExecutor` (`internal/core/runtime/action_executor.go`): `Step()` (`:66`), `RunToCompletion()` (`:136`), `Tokens()` (`:670`), `SetBreakpoint()` (`:687`). `StateExecutor` (`internal/core/runtime/state_executor.go`): `ProcessNextEvent()` (`:543`), `CurrentState()` (`:502`).
- **Oracle:** OMG SysML-v2 Pilot Implementation (2026-05, commit `4c289b926`) is already the reference for parsing (`docs/grammar/`). It is also the behavioral-semantics reference (UML 2.5.1 activity/state-machine alignment claimed in `docs/ARCHITECTURE.md:206`).
- **Behavioral parsing is covered by the existing stdlib conformance gate** (`internal/core/libs/stdlib_conformance_test.go`) since stdlib contains behavioral bodies. Do not duplicate it; extend the correctness harness instead.

---

## Execution Order & Dependencies

Safety nets before refactors, same as the parser plan.

```
Phase B1 (behavioral golden ASTs + negatives)  ──►  Phase B2 (unify behavioral body parsing)
        │
        └──►  Phase B3 (execution conformance corpus + gate)  ──►  Phase B4 (golden execution traces)
                        │
                        └──►  Phase B5 (runtime negative/robustness)  ──►  Phase B6 (semantics traceability + docs)
```

**Rule:** `go build ./...` and `go test ./...` must be green between phases. Never weaken/delete a test to pass. Never add another keyword branch to a body parser — if you feel the urge, that is the anti-pattern Phase B2 removes.

---

## Phase B1 — Behavioral Parse Safety Nets (do first)

**Objective:** Lock in current behavioral parse *shape* so the Phase B2 refactor is provably behavior-preserving.

### Task B1.1 — Behavioral golden AST fixtures
- **Files:** add fixtures + goldens under `internal/core/parser/testdata/parse/` (reuse `TestGolden` harness in `internal/core/parser/golden_test.go`, `ast.Dump` + `-update`).
- **Coverage set (minimum), each as its own fixture:**
  - `action_control_flow` — initial/final/fork/join/merge/decision + succession edges with guards.
  - `action_mixed_params` — `in`/`out`/`inout` params with multiplicity before *and* after relationships.
  - `state_full` — entry/do/exit behaviors, substates, transitions with trigger/guard/effect.
  - `state_transition_variants` — `then`, anonymous transitions, `accept`/`when` triggers.
  - `calc_return` — `return` members with typed results and nested bodies.
  - `constraint_assert_assume` — `assert`/`assume`/`assert not` + bare-expression invariant.
  - `requirement_members` — `subject`/`assume`/`require`/`actor` + nested requirement + general feature member.
- **Acceptance:** `go test ./internal/core/parser/ -run TestGolden` green; goldens human-reviewed once (they encode intended shape and become the Phase B2 regression oracle).

### Task B1.2 — Behavioral negative tests
- **File:** extend `internal/core/parser/negative_test.go` (or add `behavior_negative_test.go`).
- **Cases (must each yield ≥1 diagnostic):** `state s { entry ; }`, `action a { fork }` (dangling), `transition first then`, `requirement r { require }`, `calc c { return }`, malformed guard `if { }`.
- **Acceptance:** all report ≥1 diagnostic.

**Phase B1 exit:** behavioral golden + negative suites committed and green.

---

## Phase B2 — Unify Behavioral Body Parsing (root-cause fix)

**Objective:** Replace the per-body keyword whitelists in `behavior.go` with **specialized-first, then true general fallback**, eliminating terminal `expected ... keyword` errors. This finishes the intent of `PARSER_ROBUSTNESS_PLAN.md` Phase 3 for the behavioral parsers specifically.

### Task B2.1 — Establish the pattern on `parseRequirementMember`
- Refactor `internal/core/parser/behavior.go:1530-1566`:
  1. Keep specialized dispatch (`subject`/`assume`/`require`/`actor`/`doc`) — these build specific nodes.
  2. On no specialized match, **fall through to `parseBodyMember` unconditionally** (the general member grammar) — remove the `atDefUsageStart()` gate at `:1554` and the terminal error at `:1559-1565`.
  3. Delete debug cruft at `:1533-1535`.
- **Rationale:** in the grammar, requirement bodies admit the general member set *plus* specialized members; general parsing already reports diagnostics for genuinely-invalid input, so the terminal error is redundant and over-restrictive.
- **Acceptance:** behavioral goldens unchanged (or intentional, reviewed diffs); stdlib gate 94/94; `TestNegative` still green (general parser must still reject the negative cases).

### Task B2.2 — Apply the same pattern to the other behavioral bodies
- `parseConstraintBody` (`:1432`): collapse the keyword ladder (`assert`/`assume`/`return`/relationship/`atDefUsageStart`) so specialized constraint members are tried first, then general fallback; remove the `return`-specific special case added earlier.
- `parseStateMember` (`:1827`) / `parseStateBody` (`:1815`): specialized (entry/do/exit/transition/substate) first, then general fallback; remove terminal `expected ... keyword` errors (`:2321` and siblings) where they gate valid general members.
- `parseActionMember` (`:402`) / `parseActionBody` (`:218`): specialized control-flow nodes first, then general fallback; keep genuine structural errors (e.g., an edge with no source/target) but do not reject general members.
- **Guardrail:** preserve every specialized AST node type; grep `internal/core/runtime`, `internal/lsp`, `internal/core/resolve`, `internal/core/semantics` for each node before touching its fields.
- **Acceptance:** stdlib gate 94/94 with empty allowlist; behavioral goldens stable/reviewed; negatives green; `go vet ./...` clean.

### Task B2.3 — Remove now-dead whitelist/error code
- Delete unreachable `expected ... keyword in <X> body` branches and debug leftovers once fallbacks are in place.
- **Acceptance:** grep for `keyword in requirement body`/`keyword in state body`/etc. returns only genuinely-structural errors (dangling edges, missing `then`), not member-set gates.

**Phase B2 exit:** no behavioral body parser rejects a valid general member; terminal member-whitelist errors removed; all suites green.

---

## Phase B3 — Execution Conformance Corpus + Gate (the big missing net)

**Objective:** Build the execution-side analog of the stdlib parse gate: behavioral models with **expected execution outcomes**, gated in CI. This is what catches silent execution-semantics regressions.

### Task B3.1 — Define the outcome schema
- **File:** create `internal/core/runtime/testdata/conformance/` with paired files:
  - `<case>.sysml` — the behavioral model (part def + action/state/calc/constraint/requirement).
  - `<case>.expected.json` — expected outcome: for actions, final output-slot values + terminal token count; for states, ordered state-visit sequence + final state; for calc, return value; for constraint/requirement, boolean satisfaction.
- **Acceptance:** schema documented in a short `internal/core/runtime/testdata/conformance/README.md`.

### Task B3.2 — Conformance runner + gate
- **File:** create `internal/core/runtime/conformance_test.go`.
- **Steps:** for each case, load via existing pipeline (parser → index → resolver → `runtime.Context`), run the matching API (`ExecuteAction` `context.go:310`, `ExecuteState` `:337`, `InvokeCalc` `:228`, `EvaluateConstraint` `:81`, `EvaluateRequirement` `:148`), compare to `.expected.json`. `t.Errorf` on mismatch. Support an allowlist file for known-unimplemented cases (mirror `stdlib_known_failures.txt`).
- **Seed cases (minimum):** sequential action (2 steps, output value); fork/join parallel action; decision-guard branch (both branches, distinct guard inputs); simple state machine (entry→transition→final); hierarchical state (LCA entry/exit); time-event-ordered state machine; calc invocation; satisfied + violated constraint; satisfied + violated requirement.
- **Acceptance:** `go test ./internal/core/runtime/ -run TestExecutionConformance` green with seeded cases; introducing a wrong result makes it fail.

### Task B3.3 — (Optional, high value) Pilot oracle cross-check
- Where feasible, generate the `.expected.json` from the OMG pilot implementation's execution of the same model (pilot is the reference, `docs/ARCHITECTURE.md:456`). Record provenance per case in the README.
- **Acceptance:** at least the core action + state cases carry pilot-derived expectations, or a documented deferral if the pilot cannot execute the construct.

**Phase B3 exit:** execution conformance gate committed and green; empty (or documented) allowlist.

---

## Phase B4 — Golden Execution Traces

**Objective:** Capture *how* a behavior executes, not just the final result — the execution analog of golden ASTs. Catches ordering/scheduling regressions that final-state comparison misses.

### Task B4.1 — Deterministic trace dump
- Add a test-only trace recorder that, per `Step()` (`action_executor.go:66`) / `ProcessNextEvent()` (`state_executor.go:543`), emits a stable line: active tokens + node (`Tokens()` `:670`), or current state + event consumed (`CurrentState()` `:502`).
- **Requirement:** output must be deterministic (sort concurrent tokens; fixed tie-break for the event queue). If nondeterminism is inherent, document the canonicalization.
- **Files:** `<case>.trace.golden` next to the Phase B3 cases; `-update` flag support.
- **Acceptance:** `go test ./internal/core/runtime/ -run TestExecutionTrace` green; traces human-reviewed once.

**Phase B4 exit:** golden traces for the fork/join, decision, and hierarchical-state cases at minimum.

---

## Phase B5 — Runtime Negative / Robustness Tests

**Objective:** Ensure malformed or pathological behaviors fail *gracefully* (typed error, detected deadlock) rather than panicking or hanging.

### Task B5.1 — Failure-mode tests
- **File:** `internal/core/runtime/robustness_test.go`.
- **Cases (each must return a typed error, never panic/hang):** deadlocked action (join awaiting a token that never arrives — exercise the deadlock detector referenced in `docs/ARCHITECTURE.md:214`); decision with no satisfied guard; state machine with unreachable/dangling transition; calc with unbound parameter; constraint referencing a missing feature; execution step budget exceeded (`context.go` `incrementStep` `:53`).
- **Acceptance:** all cases produce a diagnostic/error; a `-timeout` run confirms none hang.

**Phase B5 exit:** robustness suite green; no panics; deadlock/step-budget paths asserted.

---

## Phase B6 — Semantics Traceability & Doc Reconciliation

**Objective:** Make behavioral compliance auditable and align docs with measured reality — the behavioral analog of `docs/grammar/PRODUCTION_MAP.md` + parser-plan Phase 6.

### Task B6.1 — Behavioral semantics map
- **File:** `docs/BEHAVIOR_SEMANTICS_MAP.md`. Table: `UML/KerML semantic rule -> implementation (file:func) -> conformance case(s) -> status (faithful/approximate/todo)`.
- Cover: token-flow (initial/final/fork/join/merge/decision/object-flow), run-to-completion, hierarchical entry/exit via LCA, time/change events, guard evaluation, calc/constraint/requirement evaluation.
- Cross-reference `docs/grammar/PRODUCTION_MAP.md` behavioral rows (noted "approximate" there).
- **Acceptance:** every executor node type and evaluation path appears with a status and at least one conformance case (or a filed follow-up for `todo`).

### Task B6.2 — Reconcile docs to measured reality
- **Files:** `docs/ARCHITECTURE.md` (Tier 4/5 "✅ COMPLETE" claims at `:191-241`), `README.md`.
- Replace absolute claims with the conformance-gate result and link to `TestExecutionConformance` / `BEHAVIOR_SEMANTICS_MAP.md` as the source of truth. State which constructs are executable vs. parsed-only, matching the gate.
- **Acceptance:** no unverifiable superlatives; claims trace to a passing test.

### Task B6.3 — Testing contract
- Extend the "Parser Test Contract" section in `docs/ARCHITECTURE.md` with a "Behavior Test Contract": behavioral goldens, execution conformance, golden traces, runtime negatives. Reference from `CONTRIBUTING.md`.
- **Acceptance:** contract documented; new behavioral features require all four layers.

---

## Global Acceptance Criteria (Definition of Done)

- `go build ./...`, `go test ./...`, `go vet ./...` (touched pkgs) all green.
- Stdlib parse gate: **94/94 clean, empty allowlist** (unchanged).
- No behavioral body parser rejects a valid general member; terminal member-whitelist errors removed (Phase B2).
- Behavioral golden ASTs + behavioral negatives exist and are green.
- Execution conformance gate exists, is green, and fails on wrong results.
- Golden execution traces exist for concurrency/branching/hierarchy cases.
- Runtime robustness suite green; no panics/hangs; deadlock + step-budget asserted.
- `docs/BEHAVIOR_SEMANTICS_MAP.md` present; docs reflect measured execution coverage.

---

## Verification Commands (copy-paste)

```bash
go build ./...
go test ./...
go vet ./...

# Behavioral parse safety nets (Phase B1/B2)
go test ./internal/core/parser/ -run 'TestGolden|TestNegative' -v

# Stdlib gate still green after unify (Phase B2)
go test ./internal/core/libs/ -run TestStdlibConformance -v

# Execution conformance + traces (Phase B3/B4)
go test ./internal/core/runtime/ -run 'TestExecutionConformance|TestExecutionTrace' -v

# Runtime robustness, guard against hangs (Phase B5)
go test ./internal/core/runtime/ -run TestRuntimeRobustness -v -timeout 60s

# Regenerate goldens/traces intentionally (only after reviewing diffs)
go test ./internal/core/parser/ -run TestGolden -update
go test ./internal/core/runtime/ -run TestExecutionTrace -update
```

---

## Guardrails for Implementing Agents

- **One phase at a time; keep the tree green.**
- **Never add another `atKeyword("...")` branch to a body parser.** If a construct fails, fix it by widening to the general grammar (Phase B2), not by enumerating another case.
- **Never weaken/delete a test to pass.** Fix wrong tests and justify in the commit.
- **Determinism is mandatory for golden traces.** Canonicalize concurrent ordering; document any inherent nondeterminism.
- **Check consumers before changing AST/runtime types** (`grep` `runtime`, `lsp`, `resolve`, `semantics`).
- **Prefer pilot-oracle-derived expectations** over hand-authored ones where the pilot can execute the construct.
- **No emojis, no comment churn** unless the surrounding file already does so.

---

## Progress Log (agents append here)

> Append dated entries: phase/task, files touched, decisions (esp. trace determinism canonicalization, pilot-oracle deferrals, B2 node-preservation notes), and current conformance numbers.

- _(empty)_
