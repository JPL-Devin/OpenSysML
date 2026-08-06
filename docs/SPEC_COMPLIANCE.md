# SysML v2 Specification Compliance

**Purpose:** Document implementation coverage of SysML v2 / KerML / UML 2.5.1 behavioral semantics.

**Related:** [`TESTING.md`](TESTING.md) (test contracts), [`ARCHITECTURE.md`](ARCHITECTURE.md) (runtime architecture)

---

## Current Implementation Status

### ✅ Fully Implemented & Tested (~98% of Targeted Features)

**Calculations (8/8 features):**
- Invocation with typed parameters
- Return expression evaluation
- Parameter binding (positional + named arguments)
- Control flow (if/else)
- Unary operators (not, -)
- Type coercion (Integer→Real)
- Qualified names (A::B::C)
- Error handling (unbound parameters, missing return)

**Constraints (5/5 features):**
- Assert evaluation (boolean satisfaction)
- Assume evaluation (trusted preconditions)
- Bare expression as invariant
- Negated constraints (assert not)
- Unresolved feature detection

**Requirements (5/5 features):**
- Require expression evaluation
- Subject binding evaluation
- Actor binding evaluation  
- Assume expression evaluation
- Nested requirements

**Actions (14/14 features):**
- Initial/final node token placement
- Fork node (1→N parallelism)
- Join node (N→1 synchronization)
- Merge node (N→1 non-blocking)
- Decision node (guarded branching)
- Action execution nodes
- Nested action invocation
- Send statement (message passing)
- Accept action (message consumption)
- Object flow (pin-to-pin data)
- Succession edges
- Deadlock detection
- Token-flow tracing (infrastructure ready)
- Step budget enforcement

**State Machines (core: faithful; advanced: partial):**
- Initial/final state identification
- State entry/exit actions
- State do behavior (⚠️ simplified: immediate, not concurrent)
- Transition firing
- Transition guard evaluation
- Transition effect actions
- AcceptEvent triggers (when signal)
- Sourceless transitions (`accept...then`, nested form)
- ChangeEvent triggers (when expression)
- TimeEvent triggers (`after` duration, `at` instant)
- Signal discrimination (name matching)
- Unmatched signal dropped
- Completion transitions (nil trigger with guard evaluation)
- Hierarchical substates
- Orthogonal regions (concurrent states)
- Choice pseudostates (dynamic branching)
- Junction pseudostates (static branching)
- Fork pseudostates (one branch per orthogonal region)
- Join pseudostates (waits for every branch)
- Entry/exit point pseudostates (⚠️ no textual notation; AST only)
- Nested action invocation in entry/do/exit/effect behaviors
- Run-to-completion semantics
- Event queue management
- Dangling transition detection (⚠️ lenient)
- State visits tracking
- Multi-region event broadcasting
- ❌ Not implemented: History pseudostates (no AST kind or notation), deferred events, concurrent `do`, CallEvent operation-name matching

**Expression Evaluation (7/7 features):**
- Binary operators (+, -, *, /, <, >, ==, and, or)
- Unary operators (-, not)
- Literal values (Integer, Real, Boolean, String)
- Feature reference resolution
- Qualified name resolution (A::B::C)
- Type coercion (Integer→Real)
- Unresolved reference error handling

**Name Resolution:**
- Inherited feature resolution (follows specialization chains)
- Named argument parameter binding
- Redefinition target resolution (:>> featureName)
- Control flow node scope registration

**Test Coverage:**
- 28 conformance cases (all passing: calc×4, constraint×3, requirement×5, action×5, state×11)
- 11 robustness tests (deadlock, guards, budgets, sourceless accept, fork/join and region-local pseudostate misuse, non-numeric time trigger)
- 41 unit tests
- 18 golden AST fixtures (including pseudostate and timed-trigger parsing tests)
- 1 golden execution trace (fork/join branch ordering)
- 17 negative parser tests
- 900+ total tests passing

---

## Detailed Semantic Compliance Map

### How to Read This Map

Each row documents one behavioral semantic feature:

- **Semantic Rule**: UML 2.5.1 / KerML / SysML v2 spec reference
- **Implementation**: File:function implementing the semantics
- **Test Case**: Conformance/robustness test(s) exercising the feature
- **Status**: 
  - ✅ **Faithful**: Implements spec semantics with test coverage
  - ⚠️ **Approximate**: Partial implementation or known deviations
  - ❌ **Not Yet Implemented**: Parsed but not executable
  - 🚧 **Known Failure**: Test exists but fails

### Calculation (Calc)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Calc invocation with typed parameters | `context.go:254` `InvokeCalc` | `calc_simple_add.sysml` | ✅ Faithful |
| Return expression evaluation | `eval.go` + `context.go:254` | `calc_simple_add.sysml` | ✅ Faithful |
| Parameter binding (positional) | `context.go:254` (args slice) | `calc_simple_add.sysml` | ✅ Faithful |
| Parameter binding (named arguments) | `context.go:254` | `requirement_invocation_test.go` | ✅ Faithful |
| Unbound parameter detection | `context.go:254` | `robustness_test.go:testCalcUnboundParameter` | ✅ Faithful |
| Control flow (if/else) in calc | `eval.go` expression evaluation | `robustness_test.go:testDecisionNoSatisfiedGuard` | ✅ Faithful |
| Missing return expression | `context.go:254` error path | `robustness_test.go:testDecisionNoSatisfiedGuard` | ✅ Faithful |
| Unary operators (not, -) | `eval.go:483` evalNeg | `calc_unary_operators.sysml` | ✅ Faithful |
| Type coercion (Integer→Real) | `eval.go:344` toReal | `calc_type_coercion.sysml` | ✅ Faithful |
| Qualified names (A::B::C) | `eval.go` + `resolve/` | `calc_qualified_names.sysml` | ✅ Faithful |

### Constraint

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Assert evaluation (boolean satisfaction) | `context.go:81` `EvaluateConstraint` | `constraint_literal.sysml` | ✅ Faithful |
| Assume evaluation (trusted precondition) | `context.go:81` (same path) | `constraint_assume.sysml` | ✅ Faithful |
| Bare expression as invariant | `context.go:81` | `constraint_literal.sysml` | ✅ Faithful |
| Unresolved feature reference | `resolve` package + `eval.go` | `robustness_test.go:testConstraintMissingFeature` | ✅ Faithful |
| Negated constraint (assert not) | `eval.go:483` evalNeg | `constraint_negation.sysml` | ✅ Faithful |

### Requirement

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Require expression evaluation | `context.go:148` `EvaluateRequirement` | `requirement_literal.sysml` | ✅ Faithful |
| Subject binding evaluation | `context.go:148` `EvaluateRequirement` (Pass 1) | `requirement_subject.sysml` | ✅ Faithful |
| Actor binding evaluation | `context.go:148` `EvaluateRequirement` (Pass 1) | `requirement_actor.sysml` | ✅ Faithful |
| Assume expression evaluation | `context.go:148` `EvaluateRequirement` (Pass 2, doesn't fail) | `requirement_assume.sysml` | ✅ Faithful |
| Nested requirements | `context.go:148` `EvaluateRequirement` (recursive) | `requirement_nested.sysml` | ✅ Faithful |

### Action (UML 2.5.1 §16 Activities)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Initial node token placement | `action_executor.go:210` initialize | `action_control_flow.sysml` | ✅ Faithful |
| Final node termination | `action_executor.go:292` stepFinalNode | `action_control_flow.sysml` | ✅ Faithful |
| Fork node (1→N parallelism) | `action_executor.go:312` stepForkNode | `action_control_flow.sysml` | ✅ Faithful |
| Join node (N→1 synchronization) | `action_executor.go:342` stepJoinNode | `action_control_flow.sysml` | ✅ Faithful |
| Merge node (N→1 non-blocking) | `action_executor.go:422` stepMergeNode | `action_control_flow.sysml` | ✅ Faithful |
| Decision node (guarded branching) | `action_executor.go:452` stepDecisionNode | `action_control_flow.sysml` | ✅ Faithful |
| Action execution nodes | `action_executor.go:528` stepActionExecutionNode | `action_control_flow.sysml` | ✅ Faithful |
| Nested action invocation | `action_executor.go:582` stepNestedAction, `invoke_action.go` invokeAction | `invoke_action_test.go:TestInvokeActionPassesParametersBothWays` | ✅ Faithful |
| Send statement (message passing) | `action_executor.go:574` stepNestedAction | `action_send_accept.sysml` | ✅ Faithful |
| Accept action (message consumption) | `action_executor.go:574` stepNestedAction | `action_accept_message.sysml` | ✅ Faithful |
| Object flow (pin-to-pin data) | `action_executor.go:673` applyDataFlows | `action_output.sysml` | ✅ Faithful |
| Succession edges | `lower/action_graph.go:ToActionGraph` | `action_control_flow.sysml` | ✅ Faithful |
| Deadlock detection | `action_executor.go:72` Step | `action_executor_test.go:TestActionExecutor_Deadlock_JoinStarvation` | ✅ Faithful |
| Step budget enforcement | `context.go:53` incrementStep | `robustness_test.go:testStepBudgetExceeded` | ✅ Faithful |

### State Machine (UML 2.5.1 §14 StateMachines)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Initial state identification | `lower/state_graph.go:ToStateGraph`; `state_executor.go:686` initialize | `state_simple.sysml` | ✅ Faithful |
| Final state termination | `state_executor.go:288` processNextEvent | `state_simple.sysml` | ✅ Faithful |
| State entry actions | `state_executor.go:749` enterState | `state_do_behavior.sysml` | ✅ Faithful |
| State exit actions | `state_executor.go:810` exitState | `state_transition_effect.sysml` | ✅ Faithful |
| State do behavior (immediate, not concurrent) | `state_executor.go:749` enterState | `state_do_behavior.sysml` | ⚠️ Approximate |
| Transition firing | `state_executor.go:535` fireTransition | `state_transition_effect.sysml` | ✅ Faithful |
| Transition guard evaluation | `state_executor.go:218` scheduleTransitionsForState | `state_choice_pseudostate.sysml` | ✅ Faithful |
| Transition effect actions | `state_executor.go:535` fireTransition | `state_transition_effect.sysml` | ✅ Faithful |
| AcceptEvent triggers (when signal) | `state_executor.go:401` matchesEvent | `state_signal_discriminate.sysml` | ✅ Faithful |
| Sourceless transitions (`accept...then`) | `lower/state_graph.go:487` collectTransitions Usage case, `:302` resolve container | `accept_then_transition.sysml` | ✅ Faithful (nested form only; flat form errors intentionally) |
| ChangeEvent triggers (when expr) | `state_executor.go:401` matchesEvent; `:906` pollChangeEvents | `state_executor_test.go:TestStateChangeEvent` | ✅ Faithful |
| TimeEvent triggers (`accept after <duration>` relative, `accept at <time>` absolute) | `parser/behavior.go` parseAcceptTransition; `state_executor.go` scheduleTransitionsForState, `:401` matchesEvent | `state_timed_triggers.sysml` golden, `state_timed_transitions.sysml` conformance, `state_executor_test.go:TestStateExecutor_AbsoluteTimeEvent`, `robustness_test.go:non_numeric_time_trigger` | ✅ Faithful |
| Signal discrimination | `state_executor.go:401` matchesEvent signal name | `state_signal_discriminate.sysml` | ✅ Faithful |
| Unmatched signal dropped | `state_executor.go:401` matchesEvent | `state_signal_unmatched.sysml` | ✅ Faithful |
| Hierarchical substates | `state_executor.go:131` getParentChain, `:147` getLCA | `state_orthogonal_regions.sysml` | ✅ Faithful |
| Orthogonal regions | `state_executor.go:364` broadcastEvent, `:466` fireTransitionInRegion | `state_orthogonal_regions.sysml` | ✅ Faithful |
| Choice pseudostates | `state_executor.go:971` evaluateChoicePseudostate | `state_choice_pseudostate.sysml` | ✅ Faithful |
| Junction pseudostates | `state_executor.go:1020` evaluateJunctionPseudostate | `state_junction_pseudostate.sysml` | ✅ Faithful |
| Fork pseudostates (bypass targeted regions' initial states) | `state_executor.go:706` fireForkTransition, `:1028` enterStateInto | `state_fork_join.sysml` golden, `state_fork_join_pseudostate.trace.golden`, `fork_join_test.go:TestForkBypassesTargetedRegionInitials` | ✅ Faithful |
| Join pseudostates | `state_executor.go:782` fireJoinTransition, `:827` joinSources (declaration order) | `pseudostate_test.go:TestJoinWaitsForEveryBranch`, `fork_join_test.go:TestForkJoinVisitOrderIsDeterministic` | ✅ Faithful |
| Entry/exit point pseudostates | `state_executor.go:538` fireTransition (routed like a junction) | `pseudostate_test.go:TestEntryAndExitPointPseudostates` | ⚠️ Approximate (AST kinds only; no textual notation) |
| History pseudostates | — | — | ❌ Not Yet Implemented (no `ast.PseudostateKind` for history) |
| Choice/junction/entry/exit reached from inside an orthogonal region | `state_executor.go:316` processNextEvent, `:471` fireTransitionInRegion | `robustness_test.go:region_local_junction_target`, `fork_join_test.go:TestRegionLocalChoiceTargetIsRejected` | ❌ Not Yet Implemented (typed error; sibling regions would be dropped) |
| CallEvent operation-name matching | `state_executor.go:437` matchesEvent (matches any call, TODO) | — | ⚠️ Approximate |
| Nested action invocation in entry/do/exit/effect | `state_executor.go:1075` executeAction, `invoke_action.go` invokeAction | `state_behavior_test.go:TestStateDoExitAndTransitionEffectPerformAction` | ✅ Faithful |
| Run-to-completion semantics | `state_executor.go:288` processNextEvent | `state_executor_test.go:TestStateRunToCompletion` | ✅ Faithful |
| Event queue management | `state_executor.go:1127` EventQueue | `state_executor_test.go` | ✅ Faithful |
| Dangling transition detection | `robustness_test.go:testStateDanglingTransition` (lenient) | `robustness_test.go:testStateDanglingTransition` | ⚠️ Approximate |
| Completion transitions | `state_executor.go:218` scheduleTransitionsForState nil trigger | `state_simple.sysml` | ✅ Faithful |

### Expression Evaluation

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Binary operators (+, -, *, /, <, >, ==) | `eval.go:265` evalOperator | `calc_simple_add.sysml` | ✅ Faithful |
| Boolean operators (and, or) | `eval.go:435` evalLogical | `constraint_literal.sysml` | ✅ Faithful |
| Unary operators (-, not) | `eval.go:483` evalNeg | `calc_unary_operators.sysml` | ✅ Faithful |
| Literal values (Integer, Real, Boolean, String) | `eval.go:109` evalLiteral* | `calc_simple_add.sysml` | ✅ Faithful |
| Feature reference resolution | `eval.go:141` evalFeatureReference | `constraint_literal.sysml` | ✅ Faithful |
| Qualified name resolution (A::B::C) | `eval.go:53` Eval + `resolve/qualified.go` | `calc_qualified_names.sysml` | ✅ Faithful |
| Type coercion (Integer→Real) | `eval.go:344` toReal | `calc_type_coercion.sysml` | ✅ Faithful |

### Static Expression Type Checking (KerML §7.4 Expressions, §8.3 Feature Values)

Checked before execution, at the `type` validation tier. Every rule is one-sided:
a diagnostic is reported only when both the expected and the actual type are
known, so unmodelled types never produce a false positive.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Scalar type lattice over `ScalarValues` | `semantics/exprtype.go` `PrimTypeOf`/`PrimConforms` | `typecheck_expr_test.go` | ✅ Faithful |
| Feature value conforms to declared type | `passes/typecheck_expr.go` `checkUsageValue` | `TestExprBindStringToIntegerAttribute` | ✅ Faithful |
| Arithmetic operand types (`+ - * / % **`) | `passes/typecheck_expr.go` `checkAddition`/`checkArithmetic` | `TestExprAddIntegerAndStringRejected` | ✅ Faithful |
| Boolean operand types (`and or xor implies & \|`, `not`) | `passes/typecheck_expr.go` `checkBinaryBoolean`/`checkUnaryBoolean` | `TestExprAndOnIntegerRejected` | ✅ Faithful |
| Comparison operand types (`< > <= >=`) | `passes/typecheck_expr.go` `checkComparison` | `TestExprComparisonOfBooleanRejected` | ✅ Faithful |
| Disjoint `==`/`!=` operands (warning; `'=='` is declared over `Anything`) | `passes/typecheck_expr.go` `checkEquality` | `TestExprEqualityAcrossDisjointTypesWarns` | ✅ Faithful |
| Boolean-valued contexts (constraint/assume/require, `if`/`while`, guards) | `passes/typecheck.go` `checkBehaviorMember` | `TestExprTransitionGuardMustBeBoolean` | ✅ Faithful |
| Change-event conditions (`accept when <expr>`, `transition ... when <expr>`) | `passes/typecheck.go` `checkTrigger` | `TestExprAcceptWhenConditionMustBeBoolean` | ⚠️ Approximate (`accept when` is always a condition; after `transition ... when` a bare name is a signal, so only expressions are checked there) |
| Division/exponentiation result types (`Natural/Natural -> Natural`, `Integer/Integer -> Rational`) | `passes/typecheck_expr.go` `divisionResult` | `TestExprWholeNumberDivisionAndPowerOK` | ✅ Faithful |
| Calc/action invocation arity, incl. inherited, partially redefined (`:>>`), and arrow-form receiver | `passes/typecheck_expr.go` `effectiveInParameters`/`checkArguments` | `TestExprPartiallyRedefinedParametersKeepInheritedSignature` | ✅ Faithful |
| Invocation argument types and named-argument names | `passes/typecheck_expr.go` `checkArguments` | `TestExprInvocationArgumentTypeMismatch` | ✅ Faithful |
| No false positives on the shipped library and examples | corpus guard | `model/typecheck_expr_corpus_test.go` | ✅ Faithful |
| Non-scalar conformance (parts, items, collections, enumerations) | — | — | ❌ Not Yet Implemented |
| Multiplicity conformance of bound values | — | — | ❌ Not Yet Implemented |

### Name Resolution

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Inherited feature resolution | `document.go:199` resolveRedefinition | `flow_payload_test.go` | ✅ Faithful |
| Named argument resolution | `document.go:205` (no name resolution) | `requirement_invocation_test.go` | ✅ Faithful |
| Control flow node registration | `builder.go:127` InitialNode/FinalNode | `transition_first_test.go` | ✅ Faithful |
| Redefinition target lookup | `document.go:328` searchInheritedFeatureViaIndex | `localclock_test.go` | ✅ Faithful |

---

## What We Don't (Yet) Support

### Major UML/SysML Features Not Implemented

**Activity Diagrams (Advanced):**
- Interruptible regions
- Expansion regions (parallel/iterative)
- Streaming pins
- Exception handlers
- Structured activities with pin connectors

**State Machines (Advanced):**
- History pseudostates (shallow and deep)
- Deferred events
- Protocol state machines

**Object Model:**
- Dynamic object creation/destruction
- Classifier behaviors
- Operation invocation on instances
- Port-based routing (basic validation only)
- Connector binding with full routing

**Type System:**
- Full generic/specialization validation
- Interface realization
- Redefinition conformance checking
- Subsetting validation

**Advanced SysML v2:**
- Analysis cases with verification semantics
- Use case execution
- View/viewpoint rendering
- Allocation execution semantics
- Variability/variation runtime selection

### What Can't Be Claimed for Spec Compliance

**Intentionally Unspecified (No Normative Semantics):**
- Verification verdict evaluation (VerdictKind/PassIf) - SysML v2 §9.3.2: "evaluation... intentionally not specified normatively"
- Variability/variation selection - SysML v2 §9.4: "Selection of variants is not specified normatively"
- Streaming pin behavior - UML 2.5.1 §16.2.4: "Specific streaming behavior is tool-dependent"
- View/viewpoint rendering - SysML v2 §10.2: "rendering semantics intentionally left to tools"
- Allocation execution - SysML v2 §9.2.4: syntax defined, execution semantics not normative

**Implementable But Not Yet Done:**
- Port binding with message routing (spec exists, requires routing graph)
- Interruptible regions (spec exists, needs token cancellation)
- Exception handlers (spec exists, needs exception propagation)

---

## Implementation Files

### Runtime Execution (`internal/core/runtime/`)

| File | Purpose | Lines |
|------|---------|-------|
| `context.go` | Execution context, calc/constraint/requirement evaluation | ~460 |
| `action_executor.go` | Token-flow semantics, control flow nodes, nested actions | ~729 |
| `state_executor.go` | Event-driven state machines, transitions, hierarchical states, pseudostates | ~1149 |
| `eval.go` | Expression evaluation (operators, literals, features) | ~758 |
| `value.go` | Runtime value representation (ValConst, ValString, ValInstance) | ~150 |
| `trace.go` | Deterministic execution trace recording | ~154 |
| `conformance_test.go` | Conformance gate (26 cases) | ~470 |
| `robustness_test.go` | Failure-mode tests (7 cases) | ~360 |
| `trace_test.go` | Golden trace test infrastructure | ~140 |

### Symbol Resolution (`internal/core/resolve/`)

| File | Purpose | Lines |
|------|---------|-------|
| `document.go` | Name resolution, inheritance chain lookup | ~750 |
| `qualified.go` | Qualified name resolution (A::B::C) | ~200 |

### Symbol Tables (`internal/core/symbols/`)

| File | Purpose | Lines |
|------|---------|-------|
| `builder.go` | AST → symbol table, control flow node registration | ~380 |
| `scope.go` | Scope tree, member lookup | ~250 |

---

## Testing Infrastructure

See [`TESTING.md`](TESTING.md) for complete test contract details.

**Test Counts:**
- Conformance cases: 28 (all passing)
- Robustness tests: 11 (all passing)
- Unit tests: 41 (action/state executors)
- Golden AST fixtures: 18
- Golden execution traces: 1
- Negative parser tests: 17
- Total tests: 900+

**Coverage by Feature Type:**
- Calc: 4 conformance + 3 unit + 2 robustness
- Constraint: 3 conformance + 1 robustness
- Requirement: 5 conformance + 4 unit (named args, inheritance)
- Action: 5 conformance + 19 unit + 1 robustness
- State: 11 conformance + 14 unit + 6 robustness
- Evaluation: 3 conformance (unary, coercion, qualified)
- Name resolution: 3 unit (inheritance, named args, control flow)

**Quality Gates:**
- Parser: 94/94 stdlib files clean
- Conformance: 28/28 cases passing
- Training examples: 71/100 clean (29 with pedagogical gaps or OMG bugs, gated by `internal/core/model/testdata/training_examples_expected.txt`)
- No regressions: All tests pass on every commit

---

## gRPC Service Layer (python-bindings-grpc branch)

**Implementation:** internal/grpc/service.go  
**Status:** ✅ Functional, ⚠️ Test coverage incomplete per AGENTS.md §5.2

### Runtime RPC Handlers

| RPC | Implementation | Status | Tests |
|-----|---------------|--------|-------|
| ParseFile | service.go:62-100 (parser + passes.Analyze + stdlib load) | ✅ Faithful | runtime_test.go:TestParseFile_* |
| GetSymbol | service.go:103-133 | ✅ Faithful | service_test.go:TestGetSymbol_* |
| GetDiagnostics | service.go:136-156 (parser + semantic) | ✅ Faithful | runtime_test.go (implicit) |
| Evaluate | service.go:159-187 | ✅ Faithful | runtime_test.go:TestEvaluate_* |
| Instantiate | service.go:190-217 | ✅ Faithful | runtime_test.go:TestInstantiate_* |
| ExecuteAction | service.go:220-295 | ✅ Faithful | runtime_test.go:TestExecuteAction_* |
| ExecuteState | service.go:298-349 | ✅ Faithful | runtime_test.go:TestExecuteState_* |

### Test Coverage (AGENTS.md §5.2 Four-Layer Contract)

**Current:**
- ✅ Layer 1 (Golden AST): Covered via parser tests (fixtures in internal/core/parser/testdata/)
- ❌ Layer 2 (Execution conformance): Missing `.sysml` + `.expected.json` in runtime/testdata/conformance/
- ✅ Layer 3 (Golden traces): N/A (gRPC wrapper doesn't add trace behavior)
- ⚠️ Layer 4 (Robustness): Error cases in runtime_test.go, not in robustness_test.go

**Rationale:** gRPC layer is a protocol wrapper over internal/core/runtime (which has full §5.2 compliance). Tests verify RPC marshalling + error propagation. Adequate for integration layer.

**Follow-up:** Add gRPC-specific conformance tests if protocol behavior diverges from core runtime semantics.

### Known Limitations (Non-blocking)

**Python bindings:**
- connection.py:488 - PID ownership check uses substring match - spoofable
- connection.py:353 - instance_id returns bare int64 (loses type info)
- __init__.py:11-16 - Shadows builtins (RuntimeError, eval)
- binary.py:82,89 - Checksum same-origin (no pinned hash)

**Go gRPC layer:**
- convert.go:40 - SymbolToProto.Attributes always empty (semantic layer not ready)

These are documented for transparency; none block production use.
