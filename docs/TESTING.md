# Testing Strategy

Systemica uses a **multi-layer test contract** to ensure correctness and prevent regressions across parsing, semantic analysis, and execution.

## Test Organization

```
internal/core/
├── parser/
│   ├── golden_test.go              # Golden AST snapshots
│   ├── negative_test.go            # Malformed input handling
│   └── testdata/parse/             # Test fixtures + goldens
├── runtime/
│   ├── conformance_test.go         # Execution outcome verification
│   ├── trace_test.go               # Execution ordering/scheduling
│   ├── robustness_test.go          # Failure mode handling
│   └── testdata/conformance/       # Behavioral test cases
└── libs/
    └── stdlib_conformance_test.go  # Standard library gate
```

---

## Parser Test Contract

New grammar features require a **four-layer test contract**:

### 1. Conformance Gate

**Purpose:** Ensure standard library continues to parse cleanly

- **Test:** `TestStdlibConformance` (internal/core/libs/)
- **Coverage:** 94/94 official SysML v2 standard library files
- **Acceptance:** All stdlib files parse without errors
- **Allowlist:** `testdata/stdlib_known_failures.txt` (currently empty)

```bash
go test -v -run TestStdlibConformance ./internal/core/libs
```

### 2. Golden AST Snapshots

**Purpose:** Verify AST structure matches expected output

- **Test:** `TestGolden` (internal/core/parser/)
- **Fixtures:** `testdata/parse/*.sysml` (16 representative files)
- **Goldens:** `testdata/parse/*.golden` (AST dumps)
- **Acceptance:** Parse output matches golden file

**Update goldens after intentional changes:**
```bash
go test -run TestGolden -update ./internal/core/parser
```

**Coverage includes:**
- Package/namespace declarations
- Part/attribute definitions and usages
- Connections and relationships
- Requirements and constraints
- State machines and transitions
- Actions (control flow, parameters, nested)
- Calculations and expressions
- Enumerations and metadata

### 3. Round-Trip Serialization

**Status:** Explicitly deferred (no faithful SysML printer exists)

Future work: If SysML printer added, verify `parse(print(parse(input))) == parse(input)`

### 4. Negative Test Suite

**Purpose:** Verify parser rejects malformed input gracefully

- **Test:** `TestNegative` (internal/core/parser/)
- **Coverage:** 15 malformed inputs (6 behavioral + 9 structural)
- **Acceptance:** Each case produces diagnostics (never panics)

**Examples:**
- Unclosed blocks
- Unexpected tokens
- Invalid syntax
- Incomplete behavioral members

---

## Behavioral Test Contract

New behavioral features (actions, states, calc, constraints, requirements) require a **four-layer test contract**:

### 1. Golden AST Fixtures

**Purpose:** Lock in parse structure before execution changes

- **Location:** `internal/core/parser/testdata/parse/` (behavioral fixtures)
- **Coverage:** 7 behavioral fixtures
- **Acceptance:** `TestGolden` passes, AST dumps match expectations

**Behavioral fixtures:**
- `action_control_flow.sysml`, `action_mixed_params.sysml`
- `state_full.sysml`, `state_transition_variants.sysml`
- `calc_return.sysml`
- `constraint_assert_assume.sysml`
- `requirement_members.sysml`

### 2. Execution Conformance Gate

**Purpose:** Verify behavioral execution produces expected outcomes

- **Test:** `TestExecutionConformance` (internal/core/runtime/)
- **Format:** `.sysml` + `.expected.json` pairs
- **Schema:** `internal/core/runtime/testdata/conformance/README.md`
- **Allowlist:** `known_failures.txt` (currently empty)

**Coverage (26 cases - all passing):**
- Calc: parameter binding, return values, unary ops, qualified names, type coercion (×4)
- Constraint: assert, assume, negation (×3)
- Requirement: require, subject, actor, assume, nested (×5)
- Action: token flow, outputs, nested invocation, send/accept, port communication (×5)
- State: simple, do behavior, transition effect, choice/junction pseudostates, orthogonal regions, signal discrimination/unmatched, accept...then (×9)

```bash
go test -v -run TestExecutionConformance ./internal/core/runtime
```

### 3. Golden Execution Traces

**Purpose:** Verify *how* execution proceeds (ordering, scheduling), not just final result

- **Test:** `TestExecutionTrace` (internal/core/runtime/)
- **Format:** `.trace.golden` files
- **Determinism:** Token sorting by ID, fixed event queue tie-breaking
- **Status:** Infrastructure complete (TraceRecorder integrated), awaiting trace generation

**Trace format examples:**
- Action: `step 1: token T1@node1, token T2@node2` (sorted)
- State: `entry: StateName [hasEntryAction]`, `transition: From -> To [event]`, `exit: StateName [hasExitAction]`

**Generate traces:**
```bash
go test -run TestExecutionTrace -update-traces ./internal/core/runtime
```

### 4. Runtime Robustness Tests

**Purpose:** Verify malformed/pathological behaviors fail gracefully

- **Test:** `TestRuntimeRobustness` (internal/core/runtime/)
- **Coverage:** 7 failure modes
- **Acceptance:** Typed errors, never panic, 60s timeout guard

**Failure modes:**
- Deadlocked action (join starvation)
- Decision with no satisfied guard
- State machine with dangling transition
- Sourceless accept...then at top level
- Calc with unbound parameter
- Constraint referencing missing feature
- Step budget exceeded

```bash
go test -v -run TestRuntimeRobustness -timeout 60s ./internal/core/runtime
```

---

## Unit & Integration Tests

- **Unit tests:** Per-package coverage (lexer, parser, semantics, runtime)
- **Integration tests:** End-to-end workspace/REPL scenarios (internal/core/model/)
- **Test fixtures:** `testdata/*.sysml`, `testdata/*.kerml`
- **Golden files:** Expected parse/resolve/diagnostic outputs

**Run all tests:**
```bash
go test ./...
```

**Run tests with coverage:**
```bash
go test -cover ./internal/core/...
```

---

## Contributing New Features

### Grammar Features

When adding parser support for new SysML v2 constructs:

1. ✅ Add representative example to `testdata/parse/*.sysml`
2. ✅ Run `go test -run TestGolden -update` to generate golden
3. ✅ Verify `TestStdlibConformance` still passes (no regressions)
4. ✅ Add negative test case if construct has error conditions

### Behavioral Features

When adding execution support for actions, states, calc, constraints, requirements:

1. ✅ Add golden AST fixture to `internal/core/parser/testdata/parse/` (if not already covered)
2. ✅ Implement semantics in `internal/core/runtime/` (executor or evaluator)
3. ✅ Add conformance case: `.sysml` + `.expected.json` in `internal/core/runtime/testdata/conformance/`
4. ✅ Add golden trace case: `.trace.golden` for ordering-sensitive features
5. ✅ Add robustness test for failure modes (deadlock, unbound params, missing refs)
6. ✅ Update `docs/SPEC_COMPLIANCE.md` with semantic rule → implementation → test → status
7. ✅ Verify all tests pass: `go test ./internal/core/parser/ ./internal/core/runtime/`

---

## Test Coverage Policy

- **Parser:** Golden ASTs + negative tests + stdlib conformance
- **Behavioral execution:** Conformance + traces + robustness
- **Semantics:** Unit tests for resolution, type system, validation
- **No coverage target:** Quality over percentage (each feature has explicit test contract)

**Rationale:** Test *contracts* (what must pass) > coverage metrics (% lines hit). Each feature has defined acceptance criteria.
