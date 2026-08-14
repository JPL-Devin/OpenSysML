# SysML v2 Specification Compliance

**Purpose:** Document implementation coverage of SysML v2 / KerML / UML 2.5.1 behavioral semantics.

**Related:** [`TESTING.md`](TESTING.md) (test contracts), [`ARCHITECTURE.md`](ARCHITECTURE.md) (runtime architecture)

---

## Current Implementation Status

### ✅ Fully Implemented & Tested (~98% of Targeted Features)

**Calculations (14/14 features):**
- Invocation with typed parameters
- Return expression evaluation (both `return <expr>;` and a bound return parameter `return : T = <expr>;`)
- Parameter binding (positional + named arguments)
- Parameter defaults (own and inherited)
- Inherited parameters and result through a typed calc usage, including redeclaration
- Nested calc invocation, and invocation from a constraint
- Statement bodies: local declarations, assignment, `if`/`else`, `while`, `loop … until`, `for`, and early `return`
- Purity and termination: a side effect or an outside assignment is rejected, and every loop iteration spends a step of the budget
- Control flow (if/else), including the conditional expression `if c ? a else b` evaluated lazily at runtime
- Unary operators (not, -, +)
- Type coercion (Integer→Real)
- Qualified names (A::B::C)
- Deterministic evaluation trace (parameter binding, sub-expression order, results)
- Error handling (unbound/unknown parameters, arity, missing return, recursion and step budgets)
- Calc usages as multi-output consumers: a usage's `out` features resolve, typecheck and evaluate as features (`attribute z = c.b;`), from one run of the body per usage per object
- Composition of multi-output calcs: a usage nested among a calc def's members binds its inputs from the enclosing evaluation's parameters and locals, one run per usage per object per bound input tuple

**Constraints (7/7 features):**
- Assert evaluation (boolean satisfaction)
- Assume evaluation (trusted preconditions)
- Bare expression as invariant
- Negated constraints (assert not)
- Unresolved feature detection
- Conditions of a nested constraint (`assert constraint [name] { <expr> }`)
- Parameters a typed usage binds (`constraint limit : MassLimit { in m = mass; }`)

**Requirements (8/8 features):**
- Require expression evaluation, in a requirement definition body as well as a usage
- Conditions stated through an anonymous nested constraint (`require constraint { <expr> }`)
- The requirement's own attributes, inherited or rebound, in its conditions
- Subject binding evaluation
- Actor binding evaluation  
- Assume expression evaluation, in both spellings
- Nested requirements
- A violated condition names the condition that failed

**Actions (18/18 features):**
- Initial/final node token placement
- Fork node (1→N parallelism)
- Join node (N→1 synchronization)
- Merge node (N→1 non-blocking)
- Decision node (guarded branching)
- Action execution nodes
- Nested action invocation
- Assignment statements in an action node's body
- Conditional statement (`if <cond> { … } else { … }`), nestable in either direction with a loop
- Pre-condition loop (`while <cond> { … }`) and post-condition loop (`loop { … } until <cond>;`)
- Iteration over a collection (`for <x> in <collection> { … }`, ⚠️ over a sequence or a set the expression layer can produce)
- Send statement (⚠️ typed messages addressed by name or routed through a connected port)
- Accept action (⚠️ takes the oldest message of its type; no suspension)
- Object flow (pin-to-pin data)
- Succession edges
- Deadlock detection
- Token-flow tracing (infrastructure ready)
- Step budget enforcement

**State Machines (core: faithful; advanced: partial):**
- Initial/final state identification
- State entry/exit actions
- State do behavior (runs while the state is active; concurrently active states interleave)
- Transition firing
- Transition guard evaluation
- Transition effect actions
- AcceptEvent triggers (when signal)
- Sourceless transitions (`accept...then`, nested form)
- ChangeEvent triggers (when expression)
- TimeEvent triggers (`after` duration, `at` instant)
- Signal discrimination (name matching)
- Unmatched signal dropped
- Signals sent from entry/do/exit/effect actions reaching the machine
- CallEvent triggers (`accept op(arg)` notation, operation and argument matching)
- Completion transitions (nil trigger with guard evaluation)
- Hierarchical substates
- Orthogonal regions (concurrent states)
- Choice pseudostates (dynamic branching)
- Junction pseudostates (static branching)
- Fork pseudostates (one branch per orthogonal region)
- Join pseudostates (waits for every branch)
- Entry/exit point pseudostates (`entry point <name>;` / `exit point <name>;`)
- Choice/junction/entry/exit reached from inside an orthogonal region
- Nested action invocation in entry/do/exit/effect behaviors
- Run-to-completion semantics
- Event queue management
- Dangling transition detection (⚠️ lenient)
- State visits tracking
- Multi-region event broadcasting
- History pseudostates: shallow and deep restoration (`history` / `shallow history` / `deep history <name>;`)
- Deferred events: retention and recall across hierarchy and orthogonal regions (`defer <event>[, <event>]*;`)

**Expression Evaluation:**
- Binary operators (+, -, *, /, <, >, ==, and, or)
- Exponentiation (`**`, `^`) over Integer and Real operands, folded and evaluated by one implementation
- Unary operators (-, not)
- Literal values (Integer, Real, Boolean, String)
- Feature reference resolution
- Qualified name resolution (A::B::C)
- Type coercion (Integer→Real)
- Unresolved reference error handling
- KerML function library: the scalar numeric functions of `RealFunctions`, `RationalFunctions`, `NumericalFunctions`, `IntegerFunctions`, `NaturalFunctions` and `TrigFunctions` (see the Function Library row below)

**Name Resolution:**
- Inherited feature resolution (follows specialization chains)
- Named argument parameter binding
- Redefinition target resolution (:>> featureName)
- Control flow node scope registration

**Test Coverage:**
- 201 conformance cases (all passing: calc×48, calcUsage×6, constraint×11, requirement×12, satisfy×5, action×46, state×39, instance×31, connector×3 — the calc cases include the fixed-step RK4 lunar descent whose stages are body-local usages read over a range, and the one-binding output case; the action and state cases include a decision and a transition guard reading a calc usage)
- 139 robustness subtests (deadlock, a calc output the body never assigns or only a branch that did not run would assign, an output bound both by its declaration and by an assignment or by two assignments, empty entry/do/exit bodies, a do body that never finishes, a behavior both performing an action and stating a body, a body-local usage typed by something that is not a calc, a body-local declaration with no execution, a range bound that is not an Integer, a range spending the step budget, a collection spending the element budget, a chain through a part stopping at a calc usage, an index naming no position, a collection operand of the wrong kind, a collection body of the wrong arity, a `select` predicate that is not a condition, a collection operation spending the step budget, a non-terminating loop, a calc usage leaving an input unbound, reading an output it does not declare or one with no value, a usage nested in a calc leaving an input unbound, reading an output it does not declare, an input default naming only itself, a nested usage chain reaching the recursion limit or spending the step budget, outputs valued from each other, a usage typed by something that is not a calc, a usage body spending the step budget, an invocation of a calc that computes several outputs and designates no result, a non-terminating calc loop, a calc body that never returns, a send or a `terminate` inside a calc, an assignment outside a calc body, a non-Boolean calc condition, a body-local declaration that must not leak, a body member that is not executable, a statement written directly among an action's members, accept suspension that can never end, guards, budgets, sourceless accept, fork/join misuse, pseudostate dead ends and cycles, non-numeric time trigger, misaddressed send, accept of an unsent type, send through an unconnected port, history misuse, non-deferrable deferred trigger, non-terminating do behavior, calc binding/arity/recursion failures, unhandled call, call argument of the wrong type, missing and cyclic `perform` references, a library function outside its domain or with the wrong arity, an extension library function outside its domain, exponentiation beyond the Integer range, a flow end that names no action node, a flow from a node that produced no value, a time-triggered accept with no clock, a non-Boolean change trigger, a variation with no variant selected, a selection that is not one of a variation's variants, two variants selected at once, a variation read through its declaration, a chain through an unselected variation part, two variation points selecting one variant without an owning object, a `variant` declared outside a variation, a variant under a redefined variation, a deep chain of redefinitions, conflicting redefinitions at several levels, one feature valued under two of its names, a feature both valued and restated in a body, a flow that names no feature to carry, an accepted message carrying no single value to bind, a transition that names no target, a connector end naming no reachable feature, a connector holding more than one object, a connector attached to itself or to one that names it back)
- 316 runtime test functions (`grep -c '^func Test' internal/core/runtime/*_test.go`), the conformance, trace and robustness gates above among them
- 73 golden AST fixtures (including the implicitly typed connector forms and the standard behavioral notation — a named flow with `from`, accept trigger expressions, an accept subsetting an event, sends, a succession to a loop with `until`, `then done`, a decision `else` branch, a bodied `exhibit state`, a transition with its trigger on its own line, a qualified namespace-level succession — body-local calc usages and ranges, the three loop forms, pseudostate, timed-trigger, call-trigger, calc default/invocation, calc statement bodies and n-ary connector-end parsing tests)
- 63 golden execution traces (entry/do/exit ordering of inline action bodies and a do body run to its end inside one round, the standard loop `until` with `then done`, a decision's guarded and `else` branches, a named flow carrying a value between action nodes, an accept with a `when` trigger, an accept subsetting an event, a send invocation through a port, a transition accepting through a port, loop and conditional bodies, one calc usage body run feeding several output reads, a usage whose outputs are read either side of an assignment to what its input named, a usage nested in a calc read for two of its outputs, calc statement bodies and their loop iterations, fork/join branch ordering, region entry/exit ordering, do behavior interleaving across orthogonal regions, send/accept, an accept parked until its message arrives, calc and constraint evaluation, library function invocation)
- 120 negative parser subtests
- 2,985 tests and subtests, of which 2,980 pass and 5 skip (`go test -race -count=1 -v ./...`; 1,605 top-level `Test` functions). The skips are unimplemented-expression and requirement-subject cases that skip themselves.

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
| Calc invocation with typed parameters | `invoke_calc.go` `InvokeCalc`/`invokeCalcShape` | `calc_simple_add.sysml` | ✅ Faithful |
| Return expression evaluation (`return <expr>;`) | `invoke_calc.go` `calcResult`/`resultExpression` + `eval.go` `Eval` | `calc_simple_add.sysml` | ✅ Faithful |
| Result as a bound return parameter (`return : T = <expr>;`) | `invoke_calc.go` `resultExpression` | `calc_return_parameter.sysml` | ✅ Faithful |
| Parameter binding (positional) | `invoke_calc.go` `bindCalcParameter` | `calc_simple_add.sysml` | ✅ Faithful |
| Parameter binding (named arguments) | `eval.go` `evalInvocation` + `invoke_calc.go` `InvokeCalcNamed` | `calc_named_arguments.sysml` | ✅ Faithful |
| Parameter default when no argument is passed | `invoke_calc.go` `bindCalcParameter` | `calc_parameter_defaults.sysml` | ✅ Faithful |
| Parameters and result inherited through a typed calc usage | `invoke_calc.go` `calcShapeOf`/`calcChain`/`calcParameters` | `calc_inherited_parameters.sysml`, `calc_return_parameter.sysml` | ✅ Faithful |
| Redeclared parameter keeps its inherited position and default, which stays the expression the supertype wrote and is evaluated where that calc wrote it | `invoke_calc.go` `calcParameters` (the owner carried down with the inherited default) | `calc_return_parameter.sysml`, `calc_usage_nested_shadowed_input.sysml`, `calc_usage_nested_test.go:TestNestedCalcUsageInheritedDefaultOfRedeclaredInput` | ✅ Faithful |
| Nested calc invocation | `eval.go` `evalInvocation` → `invoke_calc.go` `invokeCalc` | `calc_nested_invocation.sysml` | ✅ Faithful |
| Calc invoked from a constraint | `context.go` `EvaluateConstraint` → `eval.go` `evalInvocation` | `calc_from_constraint.sysml` | ✅ Faithful |
| Deterministic evaluation trace (binding, sub-expression order, result) | `trace.go` `RecordCalcEnter`/`RecordCalcBind`/`EndEval`, `eval.go` `Eval` | `*.trace.golden` via `TestExecutionTrace`, `trace_calc_test.go:TestCalcTraceIsStableAcrossRuns` | ✅ Faithful |
| Canonical rendering of unordered values in traces | `trace.go` `FormatTraceValue` | `trace_calc_test.go:TestFormatTraceValueCanonicalizesSets` | ✅ Faithful |
| Unbound parameter detection | `invoke_calc.go` `bindCalcParameter` (`ErrUnboundParameter`) | `robustness_test.go:testCalcUnboundParameter` | ✅ Faithful |
| Surplus positional arguments | `invoke_calc.go` `checkArgs` (`ErrCalcArity`) | `robustness_test.go:testCalcTooManyArguments` | ✅ Faithful |
| Named argument that names no parameter | `invoke_calc.go` `checkArgs` (`ErrUnknownParameter`) | `robustness_test.go:testCalcUnknownNamedArgument` | ✅ Faithful |
| Invoked symbol is not a calc | `invoke_calc.go` `calcShapeOf` (`ErrNotACalc`) | `robustness_test.go:testCalcSymbolIsNotACalc` | ✅ Faithful |
| Recursive calc (direct or mutual) is bounded | `invoke_calc.go` `invokeCalcShape` (`ErrCalcRecursionLimit`, depth 32) | `calc_recursive_base_case.sysml`, `robustness_test.go:testCalcDirectRecursion`, `:testCalcMutualRecursion` | ⚠️ Approximate (a recursion reaching its base case within depth 32 evaluates; a deeper one is rejected rather than evaluated) |
| Step budget bounds calc evaluation | `context.go` step counter (`ErrStepLimitExceeded`), budget from `budget.go` `BudgetsFromEnv` (`SYSML_MAX_STEPS`, default 10000000) | `robustness_test.go:testStepBudgetExceeded`, `budget_test.go:TestBudgetFromValue` | ✅ Faithful |
| Statement body (SysML v2 7.19, `CalculationBodyItem` carries the items of an action body): local declarations, assignment, `if`/`else`, `while`, `loop … until`, `for`, `return` | `parser/behavior.go` `parseCalcBody`/`atCalcStatement` → `lower/calc_body.go` `CalcBody` → `runtime/statements.go` `stmtEngine` driven by `invoke_calc.go` `runCalcBody` | `calc_statement_body.sysml` (golden AST), `calc_iterative_factorial.sysml`, `calc_conditional_branch.sysml`, `calc_for_over_sequence.sysml`, `calc_loop_until_body.sysml` | ✅ Faithful |
| Early `return` out of a branch or a loop unwinds the blocks entered | `lower/action_graph.go` `Return` + `runtime/statements.go` `flowReturn` | `calc_early_return_from_loop.sysml` | ✅ Faithful |
| A body-local declaration of a branch or loop body is that block's own and does not leak | `runtime/statements.go` `stmtEnv` | `passes/typecheck_calc_body_test.go:TestCalcBodyLoopLocalIsNotVisibleOutside` | ✅ Faithful |
| An inherited body evaluates in the scope of the calculation declaring it | `invoke_calc.go` `calcBody` (specialization chain, as `calcResult`/`calcParameters`) + statement `Scope` carried by the lowered IR | `invoke_calc_body_test.go:TestInheritedCalcBodyRunsInDeclaringScope` | ✅ Faithful |
| A calculation is pure: `send`, `perform`, `accept`, `terminate` and an assignment to a feature it does not declare are rejected | `runtime/calc_statements.go` (`ErrCalcSideEffect`, `ErrCalcExternalAssignment`) | `robustness_test.go:testCalcSendIsRejected`, `:testCalcTerminateIsRejected`, `:testCalcAssignmentOutsideTheCalc` | ✅ Faithful |
| A loop in a calculation terminates or fails: every iteration spends a step of the budget | `runtime/statements.go` `loop`/`forLoop` → `context.go` `incrementStep`; `InvokeCalc`/`InvokeCalcNamed` `beginRun` | `robustness_test.go:testCalcNonTerminatingLoop`, `budget_test.go:TestStepBudgetIsPerRunForInstancesAndCalcs` | ✅ Faithful |
| A body running to its end without returning is an error, not a null result | `invoke_calc.go` `runCalcBody` (`ErrCalcNoReturn`) | `robustness_test.go:testCalcBodyNeverReturns` | ✅ Faithful |
| Control flow (if/else) in calc, including the conditional expression `if c ? a else b` evaluated lazily | `runtime/statements.go` `ifStatement`; `eval.go` `evalConditional` | `calc_conditional_branch.sysml`, `calc_conditional_operator_base_case.sysml` | ✅ Faithful |
| A non-Boolean `if`/loop condition in a calc is a type error, and a diagnostic at runtime | `passes/typecheck.go` `checkBehaviorMember`; `runtime/statements.go` `condition` | `typecheck_calc_body_test.go:TestCalcBodyNonBooleanWhileCondition`, `robustness_test.go:testCalcNonBooleanCondition` | ✅ Faithful |
| Statements and loop iterations in the evaluation trace | `trace.go` `RecordStatement`/`RecordLoopIteration` | `calc_iterative_factorial.trace.golden`, `calc_early_return_from_loop.trace.golden` | ✅ Faithful |
| Missing return expression | `invoke_calc.go` `calcShapeOf` (`ErrNoResultExpression`: no result expression, no returning body, no output features and no output its body assigns) | `robustness_test.go:testCalcWithoutResult` | ✅ Faithful |
| An assignment in a body to an `out` the calculation declares binds that output for the activation, so a calc usage reads it (KerML 7.4.9: an invocation's outputs are features of that one evaluation). The parser gives `a = expr` and `a := expr` in statement position one `AssignmentActionNode`, so both spellings write the output alike | `runtime/statements.go` `stmtEngine.assign`/`stmtEnv.assignLocal` → `runtime/calc_statements.go` `calcStmtHost.declaredOutput`/`assignOuter`; `runtime/calc_usage.go` `calcRun.output`, `assignedOutputs`; `invoke_calc.go` `calcShape.BodyOutputs` | `parser/testdata/parse/calc_output_assignment.golden`, `calc_output_assigned_in_body.sysml` conformance (both spellings, a loop and a branch computing outputs) | ✅ Faithful |
| An `inout` is bound by the invocation and rebound by an assignment in the body, the read answering what the body left | `runtime/calc_usage.go` `calcOutput.IsInOut`/`calcRun.output`; `runtime/calc_statements.go` `calcStmtHost.declaredOutput` | `calc_output_assigned_in_body.sysml` conformance (`Bump`) | ✅ Faithful |
| An output given a value by its declaration *and* assigned in the body is a typed error rather than a silent pick (the precedent of a feature valued two ways). Assignments to an output within the body are imperative like a body local's: the last one to run is the activation's binding, so an output may be initialized and then accumulated into, including once per loop iteration | `runtime/calc_statements.go` `calcStmtHost.assignOuter` (`ErrConflictingOutput`) | `robustness_test.go:testCalcOutputValuedAndAssigned`, `:testCalcOutputAssignedTwice`, `calc_output_assigned_in_body.sysml` conformance (`Accum`) | ✅ Faithful |
| Reading a declared output the activation never assigned reports that output, not a missing result expression; an output only a branch that did not run would assign is unassigned for that activation | `runtime/calc_usage.go` `calcRun.output` (`ErrOutputNotAssigned`, a kind of `ErrNoValue`) | `robustness_test.go:testCalcOutputNeverAssignedByTheBody`, `:testCalcOutputAssignedInABranchNotTaken` | ✅ Faithful |
| A calc usage's members are the parameters and outputs of the calc it is typed by, reachable through a feature chain (SysML 7.6.6, 7.17) | `resolve/target.go` `ResolveTarget`/`memberChain` + `resolve/document.go` `resolveMemberChain` → `semantics` `Model.LookupMember` | `passes/typecheck_calc_usage_test.go:TestCalcUsageOutputTypesAsTheOutputItNames`, `:TestCalcUsageOutputInsideAPartDefinition` | ✅ Faithful |
| An output read through a chain types as that output declares, or as its default computes when it declares no type | `passes/typecheck_expr.go` `inferFeatureChain`/`featurePrimType` | `passes/typecheck_calc_usage_test.go:TestCalcUsageOutputTypedByItsDefaultIsChecked`, `:TestCalcUsageDeclaredOutputTypeIsChecked` | ✅ Faithful |
| Reading a name the calc declares no output for is unresolved, reported once by the name-resolution tier | `resolve/document.go` `resolveMemberChain`; `runtime/calc_usage.go` `calcRun.output` (`ErrUnknownOutput`) | `passes/typecheck_calc_usage_test.go:TestCalcUsageUnknownOutputIsUnresolved`, `robustness_test.go:testCalcUsageUnknownOutput` | ✅ Faithful |
| A calc usage evaluates its body once and every `out` feature it declares is readable from that run (SysML 7.17) | `runtime/calc_usage.go` `CalcUsageOutput`/`CalcUsageOutputs`/`calcUsageRun` → `invoke_calc.go` `calcShapeOf`/`bindCalcParameters` → `runtime/statements.go` `stmtEngine` | `calc_usage_multiple_outputs.sysml`, `calc_usage_statement_body.sysml`, `calc_usage_inherited_parameters.sysml`, `calc_usage_instance_slots.sysml` | ✅ Faithful |
| Reading several outputs of one usage runs the body once, per usage and per object, reset with the run | `runtime/calc_usage.go` `calcUsageRun` (`calcUsageKey`) + `context.go` `beginRun`/`beginExecutorRun` | `calc_usage_multiple_outputs.trace.golden`, `calc_usage_statement_body.trace.golden`, `calc_usage_inherited_parameters.trace.golden` | ✅ Faithful |
| A usage's inputs bind from its own member values, falling back to the defaults declared along its specialization chain, and may name a sibling feature of the object carrying the usage | `runtime/calc_usage.go` `calcUsageRun` → `invoke_calc.go` `bindCalcParameters`/`calcParameters` | `calc_usage_inherited_parameters.sysml`, `calc_usage_instance_slots.sysml` | ✅ Faithful |
| A usage declared in a behavior's body — a calc's or an action's — binds its inputs in the environment of the evaluation reading it: the enclosing parameters and locals as the running body holds them, then the enclosing lexical scope, so an input naming an attribute the body assigned reads the assigned value rather than the declared one (SysML 7.17, a usage is a feature of the body declaring it) | `runtime/calc_usage.go` `bindCalcUsage`/`enclosedByBehaviorBody`, `invoke_calc.go` `isActionSymbol`, `eval.go` `EvalContext.nestedEnv`, `invoke_calc.go` `bindCalcParameters` | `calc_usage_nested_in_calc.sysml`, `action_body_local_calc_usage.sysml`, `calc_usage_nested_test.go:TestNestedCalcUsageBindsFromEnclosingParameters`, `:TestNestedCalcUsageBindsFromEnclosingLocals`, `:TestNestedCalcUsageChain`, `:TestNestedCalcUsageReadsEnclosingObject`, `action_body_local_usage_test.go:TestActionBodyLocalUsageBindsCurrentValues`, `:TestActionBodyLocalUsageBindsPerIteration` | ✅ Faithful |
| A nested input bound from a name of its own (`in vx = vx`) reads the enclosing binding: the inputs being bound are not in the environment their own values are evaluated in, so every one of them resolves names in the enclosing environment alike — `in n = m; in m = n;` swaps the two values rather than the second reading the first's fresh binding | `runtime/calc_usage.go` `bindCalcUsage`, `eval.go` `EvalContext.nestedEnv`/`Lookup` | `calc_usage_nested_shadowed_input.sysml` (shadowed and swapped names), `calc_usage_nested_test.go:TestNestedCalcUsageShadowsEnclosingName`, `:TestNestedCalcUsageInputsDoNotSeeSiblings`, `robustness_test.go:testNestedCalcUsageSelfCycle` (a default with nothing outside to name stays `ErrCyclicSlot`) | ✅ Faithful |
| One run per usage, object and activation: two reads from one enclosing invocation run the body once, two invocations do not share it, and an iteration of a loop is an activation of its own. The activation replaces the input-value hash the memo key used, so two invocations whose inputs coincide still get their own run and no read can be answered from a run bound to other values | `runtime/calc_usage.go` `calcUsageKey`/`calcUsageRun`, `context.go` `newActivation`/`endActivation` | `calc_usage_nested_in_calc.trace.golden`, `calc_usage_nested_shadowed_input.trace.golden`, `calc_usage_nested_test.go:TestNestedCalcUsageRunsPerInputs`, `:TestNestedCalcUsageOwnOutputPerInvocation`, `calc_usage_snapshot_test.go:TestCalcUsageMemoDistinguishesEnclosingArguments` | ✅ Faithful |
| Every output read from one evaluation of a usage sees one binding of its inputs: the inputs are bound when the evaluation starts and a later assignment to a feature they named does not rebind them for a later output read, so two outputs of one usage can never come from different input bindings (KerML 7.4.9, SysML 7.17 — a usage's outputs are features of one evaluation) | `runtime/calc_usage.go` `calcUsageRun`/`calcUsageKey`/`forgetCalcUsage`, `context.go` `newActivation`/`endActivation` | `calc_usage_outputs_one_binding.sysml` + `.trace.golden`, `calc_rk4_lunar_descent.sysml`, `calc_usage_snapshot_test.go:TestCalcUsageOutputsShareOneInputBinding`, `:TestCalcUsageOutputsInAssignmentLoop`, `:TestCalcUsageMemoDistinguishesEnclosingArguments`, `:TestCyclicCalcUsageOutputStillDiagnosed` | ✅ Faithful |
| A calc usage declared in a loop or conditional body is executable, in the scope it is written in and with the lifetime of the block: an iteration binds it from that iteration's state and reading it again in the same iteration reuses that evaluation. This holds in an action's body as well as a calc's, both being run by the statement engine | `lower/calc_body.go` `usageStatement` → `lower.DeclareUsage`, `runtime/statements.go` `declareUsage`, `runtime/calc_usage.go` `bodyUsageSymbol`/`enclosedByBehaviorBody` | `calc_body_local_usage_and_range.sysml` (golden AST), `calc_rk4_lunar_descent.sysml`, `action_body_local_calc_usage.sysml`, `calc_usage_body_local_test.go:TestBodyLocalCalcUsageInLoopBindsPerIteration`, `:TestBodyLocalCalcUsageInBranch`, `:TestBodyLocalCalcUsageInNestedBodies`, `action_body_local_usage_test.go:TestActionBodyLocalUsageBindsCurrentValues`, `:TestActionBodyLocalUsageBindsPerIteration`, `robustness_test.go:testBodyLocalUsageOfANonCalc`, `:testBodyLocalDeclarationNotExecutable` | ✅ Faithful |
| A feature chain through a part reads what the part's features carry, the part being materialized as the occurrence it denotes; an output of a calc usage the part declares evaluates in that part's context. The object being evaluated answers first: a slot it carries for that part is the object read, and only a part no object in hand carries is materialized. A usage of several occurrences is a collection, not one object, so a chain through it is reported | `runtime/eval.go` `evalFeatureChain`/`chainBase`, `runtime/calc_usage.go` `occurrenceOperand`, `runtime/instance.go` `occurrenceOf`/`occursOnce` | `part_feature_chain_test.go:TestCalcUsageReadThroughPartChain`, `:TestPartChainReadsTheObjectInHand`, `:TestPartChainRejectsSeveralOccurrences`, `:TestCalcUsageChainDiagnostics`, `robustness_test.go:testUsageReadThroughAPartWithoutAnOutput` | ✅ Faithful |
| A chain through a multi-valued feature has, for its last feature, that feature's values over every object the features before it name (KerML 1.0 §7.3.4.6): `subsystem.volume` is the volumes of every object `subsystem` holds, in the collection's order, flattened one level per step as `->collect` flattens its mapping. An empty collection yields no values; a chain reaching an object with no value for the feature reports the unset slot | `runtime/eval.go` `chainMemberValue`/`chainOverElements` | conformance `feature_chain_rollup_over_subsets`, `feature_chain_nested_multivalued`, `feature_chain_empty_collection`, `cubesat_mass_rollup`; `robustness_test.go:feature_chain_through_an_unset_slot`, `:feature_chain_spends_the_element_budget`, `chain_trace_test.go:TestChainOverCollectionTraceOrder` | ✅ Faithful |
| A chain that stops at a calc usage rather than at one of its outputs names the outputs to read instead of reporting no value | `runtime/eval.go` `evalCalcUsageMembers` (`ErrNoValue`) | `part_feature_chain_test.go:TestCalcUsageChainDiagnostics`, `robustness_test.go:testUsageReadThroughAPartWithoutAnOutput` | ✅ Faithful |
| A nested usage chain is bounded: the depth counted while an output binding is evaluated, the budget spent by the bodies it runs. One evaluation counts one level however its answer is reached, so an invocation whose result is a designated output and outputs of one calc naming each other spend nothing extra | `invoke_calc.go` `enterCalc` (`ErrCalcRecursionLimit`)/`runCalcBody`, `runtime/calc_usage.go` `calcRun.enter`/`calcRun.value` | `robustness_test.go:testNestedCalcUsageRecursionDepth`, `:testNestedCalcUsageStepBudget`, `:testNestedCalcUsageUnboundInput`, `:testNestedCalcUsageUnknownOutput`, `calc_usage_nested_test.go:TestNestedCalcUsageDepthCountedOnce`, `:TestCalcOutputChainIsNotNesting` | ✅ Faithful |
| An `out` default evaluates in the calc's own scope and may name inputs, body locals and other outputs | `runtime/calc_usage.go` `calcRun.output`/`lookupOutput` + `eval.go` `EvalContext.calcRun` | `calc_usage_statement_body.sysml`, `calc_usage_inherited_parameters.sysml` | ✅ Faithful |
| An output feature fed into a feature's default value (the parametric-budget pattern) | `eval.go` `evalFeatureChain`/`evalCalcUsageMembers` | `calc_usage_multiple_outputs.sysml`, `calc_usage_instance_slots.sysml` | ✅ Faithful |
| Outputs valued from each other are a cyclic dependency, not a hang or a spent step budget | `runtime/calc_usage.go` `calcRun.output` (`ErrCyclicOutput`) | `robustness_test.go:testCalcUsageCyclicOutputs` | ✅ Faithful |
| A usage leaving an input unbound, or reading an output with no value, is reported | `runtime/calc_usage.go` `calcUsageRun` (`ErrUnboundParameter`), `calcRun.output` (`ErrNoValue`) | `robustness_test.go:testCalcUsageUnboundInput`, `:testCalcUsageOutputWithoutAValue` | ✅ Faithful |
| A calc usage typed by something that is not a calc is reported | `invoke_calc.go` `calcShapeOf` (`ErrNotACalc`) | `robustness_test.go:testCalcUsageSpecializesANonCalc` | ✅ Faithful |
| The step budget bounds a usage's body the way it bounds an invocation | `runtime/calc_usage.go` `CalcUsageOutput` `beginRun` → `context.go` `incrementStep` (`ErrStepLimitExceeded`) | `robustness_test.go:testCalcUsageStepBudget` | ✅ Faithful |
| An invocation yields exactly one result (KerML 7.4.9), so invoking a calc that computes several outputs and designates no result is rejected rather than answered with the first of them — and the diagnostic writes out the calc usage to declare instead, with the inputs to bind | `runtime/calc_usage.go` `calcShape.designatedOutput`/`usageSpelling` (`ErrAmbiguousResult`) | `robustness_test.go:testMultipleOutputsInvokedAsAnExpression`, `repl/runtime_commands_test.go:TestCalcWithSeveralOutputsIsNotInvocable` | ✅ Faithful |
| A calc with exactly one output and no `return` is invocable, that output being its result | `runtime/calc_usage.go` `calcShape.designatedOutput` | `calc_usage_single_output.sysml` | ✅ Faithful |
| `%calc` on a calc usage lists the outputs of one evaluation; a chain into a usage evaluates at the prompt | `repl/meta.go` `doCalc`/`calcUsageOutputs` | `repl/runtime_commands_test.go:TestCalcUsageOutputsAtThePrompt` | ✅ Faithful |
| A calc usage's outputs are evaluation results, not slots of an object | `runtime/calc_usage.go` (no instance materialization) | `calc_usage_instance_slots.sysml` (the features fed by the outputs are slots; the usage itself is not) | ⚠️ Approximate (`%instances` and export show the features valued from outputs, not the usage's outputs themselves) |
| Boolean operators evaluated at runtime (`and`, `or`, `xor`, `implies`, short-circuiting where they can) | `eval.go` `evalLogical` | `calc_boolean_operators.sysml` | ✅ Faithful |
| Identity (`===`, `!==`), null coalescing (`??`, lazy) and remainder (`%`) evaluated at runtime | `eval.go` `evalIdentity`/`evalNullCoalesce`/`evalArithmetic` | `calc_identity_operators.sysml`, `calc_null_coalesce.sysml`, `calc_modulo_operator.sysml` | ✅ Faithful |
| An operator with no runtime evaluation (classification, cast, `all`, bitwise complement) reports why | `eval.go` `unimplementedOperators` (`ErrUnsupportedOperator`) | `eval_operator_test.go:TestUnimplementedOperatorReportsWhy` | ❌ Not implemented (rejected with a typed diagnostic naming what it would need) |
| `lower..upper` is the ordered sequence of integers the library declares it to be (`IntegerFunctions::'..'` returns `Integer[0..*] ordered`, and `SequenceFunctions::subsequence` maps over it), so every sequence operation, index and `for` applies to it unchanged. A descending range is empty, a bound that is not an Integer is a type error, and each element generated costs a step and an element of the budgets, so a range wider than either — including one whose width overflows — is `ErrStepLimitExceeded` or `ErrElementLimitExceeded` rather than an allocation | `runtime/range.go` `evalRange`/`rangeSequence`/`rangeBound`/`builtinIntegerRange` (`ErrTypeMismatch`), registered in `builtins.go` | `calc_integer_range.sysml`, `range_test.go:TestIntegerRange`, `:TestIntegerRangeSequenceOperations`, `:TestIntegerRangeNonIntegerBound`, `:TestIntegerRangeSpendsTheStepBudget`, `:TestIntegerRangeExtremeBounds`, `:TestForOverIntegerRange`, `robustness_test.go:testRangeBoundIsNotAnInteger`, `:testRangeSpendsTheStepBudget` | ✅ Faithful |
| Unary operators (not, -, +) | `eval.go` `evalUnary` | `calc_unary_operators.sysml` | ✅ Faithful |
| Type coercion (Integer→Real) | `eval.go:344` toReal | `calc_type_coercion.sysml` | ✅ Faithful |
| Qualified names (A::B::C) | `eval.go` + `resolve/` | `calc_qualified_names.sysml` | ✅ Faithful |

A calc usage with several `out` features, a usage nested in a part or in a calc
def's body, and a chain into one (`c.a`) are all existing productions — a directed feature member of a
usage body and `FeatureChainExpr` — so multi-output calc usages added no grammar
and no golden AST fixture of their own. `calc_defaults_and_invocation.sysml`
(a typed calc usage binding an inherited parameter in its body) and
`calc_statement_body.sysml` lock the parse structure they reuse.

### Constraint

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Assert evaluation (boolean satisfaction) | `context.go:81` `EvaluateConstraint` | `constraint_literal.sysml` | ✅ Faithful |
| Assume evaluation (trusted precondition) | `context.go:81` (same path) | `constraint_assume.sysml` | ✅ Faithful |
| Bare expression as invariant | `context.go:81` | `constraint_literal.sysml` | ✅ Faithful |
| Unresolved feature reference | `resolve` package + `eval.go` | `robustness_test.go:testConstraintMissingFeature` | ✅ Faithful |
| Negated constraint (assert not) | `eval.go:483` evalNeg | `constraint_negation.sysml` | ✅ Faithful |
| A constraint a type carries is evaluated against an instance of it, so it reads that object's slots rather than declared defaults | `context.go` `EvaluateConstraintOn`, `eval.go` `NewEvalContextIn`/`selfSlotValue` | `instance_constraint_binding.sysml`, `repl/instance_test.go:TestConstraintBindsToInstance` | ✅ Faithful |
| A false assertion is a verdict, not a malfunction (`ErrViolated`), and is distinguishable from an evaluation failure | `errors.go` `ErrViolated`, `context.go` `EvaluateConstraintOn` | `repl/instance_test.go:TestConstraintEvaluationErrorIsNotAViolation` | ✅ Faithful |
| A constraint usage inherits its conditions from the definition it is typed by (`constraint limit : MassLimit;`) | `context.go` `chainMembers` over `semantics.Model.AllSupertypes` | `instance_inherited_constraint.sysml` | ✅ Faithful |
| A parameter a typed usage binds (`constraint limit : MassLimit { in m = mass; }`) is visible to the conditions it inherits, and masks both the declaration it redefines and a same-named member of the object carrying the usage — including the usage's own name | `condition.go` `conditionFeatures` over `Model.MembersOf`, `eval.go` `evalFeatureReference` | `instance_constraint_bound_parameter.sysml`, `instance_constraint_parameter_name_collision.sysml`, `runtime/condition_test.go:TestConstraintUsageBindsInheritedParameter` | ✅ Faithful |
| The conditions of a nested constraint (`assert constraint [name] { <expr> }`) are the conditions of the member stating it | `parser/behavior.go` `tryParseNestedConstraint`, `condition.go` `appendConditions` | `parser/behavior_require_member_test.go:TestConstraintMemberNestedBody` | ✅ Faithful |
| A constraint carrying no condition yields no verdict (`ErrNoConditions`) rather than a vacuous pass | `errors.go` `ErrNoConditions`, `context.go` `EvaluateConstraintOn`/`EvaluateRequirementOn` | `runtime/constraint_test.go:TestConstraintWithoutConditionsIsNotAVerdict` | ✅ Faithful |

### Instantiation and Feature Values (SysML v2 §7.6 Feature Values, KerML §8.3)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| A literal default is folded at instantiation | `instance.go` `Instantiate` | `instance_derived_slots.sysml` (`folded`) | ✅ Faithful |
| A default that reads sibling features is derived per instance, evaluated against that object's slots on demand | `instance.go` `GetSlot`/`evalSlotDefault`, `eval.go` `selfSlotValue` | `instance_derived_slots.sysml` (`doubled`) | ✅ Faithful |
| A default reaching through a nested part reads that part's own derived values | `eval.go` `evalFeatureChain` (via `GetSlot`) | `instance_derived_slots.sysml` (`total`) | ✅ Faithful |
| A default expression resolves declarations in the scope that declared the feature, while instance slots take precedence | `shape.go` `EffectiveFeature.DeclScope`, `eval.go` `EvalContext.self` | `instance_derived_slots.sysml` | ✅ Faithful |
| Mutually dependent defaults report a cycle rather than recursing to the step budget | `context.go` `derivingSlots`, `errors.go` `ErrCyclicSlot` | `robustness_test.go:cyclic_derived_slot` | ✅ Faithful |
| A default over an undeclared feature fails naming the slot | `instance.go` `evalSlotDefault` | `robustness_test.go:derived_slot_over_missing_feature` | ✅ Faithful |
| A multi-valued feature holds its default's contents; a single value written on it is the collection's one element | `instance.go` `GetSlot` | `runtime/instance_test.go:TestMultiValuedDefaultMaterializes`, `repl/instance_test.go:TestCollectionSlotsShowTheirContents` | ✅ Faithful |
| A nested part usage with a body of its own is instantiated as that usage, so what its body declares wins over what its type declares, and an untyped nested part (`part engine { ... }`) still materializes | `instance.go` `compositeType`, `GetSlot` | `instance_nested_usage_body.sysml`, `instance_unnamed_redefinition.sysml`, `runtime/instance_test.go:TestNestedUsageBodyOverridesItsType`, `TestUntypedNestedPartMaterializes` | ✅ Faithful (both the named form and an unnamed `:>> power = 250.0;`, which takes the name of what it redefines — see the KerML 7.3.4.5 row above) |
| A single-bound multiplicity is both bounds, unless it is unbounded: `[*]` is `0..*`, so a `[*]` feature materializes empty like `[0..*]` (KerML 1.0 §8.2.5.11, confirmed by OMG issue KERML11-204) | `semantics/multiplicity.go` `multiplicityRange` | `semantics/multiplicity_test.go:TestMultiplicitySingleBoundStar`, conformance `multiplicity_unbounded_single_bound`, `parse/multiplicity_unbounded_and_subsetting.golden`, `passes/constraint_test.go:TestConstraint_RedefinitionUnboundedMultiplicity` (a `[*]` redefinition keeps an inherited `0..*` and loosens an inherited `1..*`) | ✅ Faithful |
| A required lower bound is materialized eagerly, so an unbounded one (`[*..*]`) or one past the materialization bound reports a multiplicity violation instead of allocating | `instance.go` `GetSlot` (`maxMaterializedLowerBound`, `ErrMultiplicityViolation`) | `robustness_test.go:multiplicity_infinite_lower_bound`, `:multiplicity_lower_bound_too_large` | ⚠️ Approximate (a lower bound above 1000 is refused rather than materialized lazily) |
| The values of a subsetting feature are values of the feature it subsets, so a nested part declared `part a : Sub :> subsystem` is one of the objects `subsystem` holds and a roll-up over `subsystem` sums over it (KerML 1.0 §7.3.4.4) | `subsetting.go` `subsettingContributions`/`relatedFeatureNames`, `instance.go` `GetSlot` | conformance `feature_chain_rollup_over_subsets`, `cubesat_mass_rollup`, `robustness_test.go:mutually_subsetting_features` | ⚠️ Approximate (contributions are the subsetting features declared on the same object; the subsetted collection is read-only with respect to them, and a subsetting feature declared elsewhere for the same object contributes nothing) |
| A redefining feature is the feature it redefines, so `part subsystems : Component[*] :>> Subsystems` makes both names read one collection, and a chain of redefinitions (`dry :>> own :>> mass`) reads one slot under every name even when a usage restates the redefinition (`part sat : Sys { attribute :>> own = 10.0; }`) (KerML 1.0 §7.3.4.5) | `subsetting.go` `aliasRedefinedSlots`, `redefinitionGroups`, `sharedRedefinitionName`, `redefinedNames`, `isFeatureOf` | conformance `cubesat_mass_rollup`, `redefinition_restated_in_a_usage`, `redefinition_multilevel_base_name`, `redefinition_value_under_either_name`, `redefinition_valued_under_two_names`, `robustness_test.go:one_feature_valued_under_two_names` | ✅ Faithful (the shared slot is the one the most specific declaration writing a value created, whichever name it wrote — the redefining name, the base name, or a name in between, including where the value is written in an abstract part usage a configuration specializes; one declaration valuing two names of the feature is `ErrConflictingRedefinition` rather than a silent pick) |
| A usage of any kind whose body restates or adds features is instantiated as that usage, so a struct-typed attribute takes its value from a nested body (`attribute :>> material { attribute :>> v = 3.0; }`) at any depth, and a renaming redefinition that restates no type is typed by the feature it redefines (KerML 1.0 §7.4.7) | `instance.go` `CompositeTypeOf`, `declaresFeatures`, `shape.go` `extractType`, `declaredType` | conformance `attribute_nested_value_body`, `parse/nested_value_body.golden`, `shape_test.go:TestFeaturesOf_TypeInheritedThroughRedefinition` | ✅ Faithful |
| A feature bound to a value takes its own features from that value, so a body of the same declaration valuing one of them (`attribute :>> ringCost = 400.0 { attribute :>> v = 9.0; }`) states two values for it and is reported; a body that only re-declares features (the stdlib's `item :>> edges : Ellipse = shape { attribute :>> Shell::edges::innerSpaceDimension, Ellipse::innerSpaceDimension; }`) states no second value and reads the bound one | `instance.go` `restatedInValuedBody`, `GetSlot` (`ErrValuedFeatureRestated`) | `robustness_test.go:valued_feature_restated_in_a_body`, conformance `attribute_nested_value_body` (`valuedWithReDeclaredFeatures`) | ⚠️ Approximate (a body over a value the *redefined* declaration wrote reads that value and its own restatements are dropped, rather than the innermost body governing — pre-existing, see the known limitation below) |
| A redefining feature that declares no value holds the value the redefined declaration wrote, evaluated in the scope that wrote it, so `attribute grossMass :>> mass` reads the inherited default under either name (KerML 1.0 §7.3.4.5) | `shape.go` `redefinedDefault`, `EffectiveFeature.DefaultScope`, `instance.go` `evalSlotDefault` | conformance `instance_redefined_attribute_default`, `subsetting_test.go:TestRedefiningFeatureHoldsTheRedefinedDefault` | ✅ Faithful |
| A multi-valued feature given a default holds that default's contents whether or not it is also typed, so `attribute volumes : Real[0..*] = subsystem.volume` is the chain's values rather than an empty typed collection | `instance.go` `GetSlot`, `CompositeTypeOf` | conformance `feature_chain_rollup_over_subsets`, `feature_chain_nested_multivalued`, `instance_test.go:TestTypedMultiValuedDefaultHoldsItsContents` | ✅ Faithful |
| A relationship target that resolves outside the object names no feature of it, so `attribute totalmass :> ISQ::mass` specializes the library feature and contributes nothing to a same-named feature of the object; a target the object carries under its name — including one a restating declaration masks — is a feature of it, and an unqualified target the declaring scope cannot see is looked up among the object's members | `subsetting.go` `relatedFeatures`/`isFeatureOf` | `subsetting_test.go:TestSubsettingIgnoresALibraryFeatureOfTheSameName`, conformance `cubesat_mass_rollup` | ✅ Faithful |

### Redefinition in a Specialization (KerML §7.4.7 Redefinition, SysML v2 §7.6)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| A usage that redefines an inherited usage (`part derived :> base { part :>> inner { … } }`) specializes what it redefines, so it keeps every nested member the redefined usage declared and overrides only what it restates | `semantics/model.go` `NewModel` (attaches the model to `resolve.Resolver`, so a redefinition target reachable only through inheritance resolves and the redefining usage gains it as a supertype), consumed by `runtime/shape.go` `FeaturesOf` over `Model.MembersOf` | `redefinition_inherited_nested_values.sysml`, `ballandchain_variant_configuration.sysml`, `robustness_test.go:deep_specialization_chain_of_redefinitions`, `conflicting_redefinitions_at_several_levels` | ✅ Faithful (multi-level chains, a redefinition of a redefinition, and conflicting restatements where the innermost wins; the merge is the inherited-member view, not a slot-level merge in the instantiator) |

### Variation and Variant (SysML v2 §7.20 Variant Modelling)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `variation` and `variant` are recorded on the declaration they modify, in every position they may appear (`variation attribute`/`part`/`interface`, nested or top-level) | `parser/defusage.go` `applyFeatureMods`, `atKindPrefix`, `ast/defusage.go` `Usage.IsVariation`/`IsVariant`, round-tripped by `export/rdf_out.go`/`rdf_in.go` | `parser/testdata/parse/variation_and_variant.golden`, `parser/negative_test.go` (`variation_no_declaration`, `variation_attribute_no_name`, `variant_unclosed_body`, `variant_selection_no_variant_name`) | ✅ Faithful |
| A variation is an abstract classifier of its variants: a variant specializes its variation and so carries the variation's type and features | `semantics/model.go` `DirectSupertypes` via `semantics/variation.go` `VariationOwning` | `semantics/variation_test.go:TestVariationAndVariantModifiers`, `TestVariantsOfInheritedThroughRedefinition` | ✅ Faithful |
| A variant is reachable through the variation feature's name (`cut::cutIdeal`), including through a feature chain (`engagementRing.nesting::nestingTrue`) and through a feature that redefines or specializes the variation | `semantics/variation.go` `Model.IsVariationFeature`, `Model.VariantsOf`, `Model.VariantOf`, `runtime/variation.go` `EvalContext.variantSegment`, `runtime/eval.go` `evalFeatureChain` | `variation_attribute_selection.sysml`, `variation_part_selection.sysml`, `semantics/variation_test.go:TestIsVariationFeatureThroughSpecialization` | ✅ Faithful |
| Binding a variation usage to one of its variants selects that variant, with its nested values and its own nested features, whether the variation is read through an object's slot or through its declaration (`EvaluateConstraint`, `EvalWithScope`, REPL `%eval`) | `runtime/variation.go` `Context.bindVariation`, `variantValue`, `EvalContext.bindVariationOf`, `runtime/instance.go` `GetSlot` (variation slots resolve before ordinary defaults), `runtime/value.go` `ValVariant` | `variation_attribute_selection.sysml`, `variation_part_selection.sysml`, `variation_interface_selection.sysml`, `robustness_test.go:variation_read_through_its_declaration` | ✅ Faithful |
| A variation feature compares equal to the variant it is bound to (`x == x::variantName`), and unequal to any other variant — the form an asserted configuration constraint uses | `runtime/eval.go` equality over `ValVariant`, `runtime/value_equality.go` | `variation_attribute_selection.sysml`, `variation_interface_selection.sysml`, `variation_interface_mismatch.sysml`, `ballandchain_variant_configuration.sysml` | ✅ Faithful |
| A variation with no variant selected is a typed error, never a silently wrong value, and a variation is no occurrence of itself, so a chain through an unselected variation part reports the same rather than reading an object of the variation | `runtime/errors.go` `ErrVariationUnselected`, `runtime/instance.go` `GetSlot`, `runtime/eval.go` `evalFeatureReference`, `runtime/calc_usage.go` `occurrenceOperand` | `variation_unselected.sysml`, `robustness_test.go:variation_without_a_selected_variant`, `chain_through_an_unselected_variation_part` | ✅ Faithful |
| Selecting what is not a variant of the variation, or selecting two variants at once, are typed errors naming the variants available | `runtime/errors.go` `ErrNotAVariant`/`ErrMultipleVariants`, `runtime/variation.go` `bindOneVariant`, `variantSummary`, `runtime/eval.go` (a missing member under a variation feature) | `robustness_test.go:variation_bound_to_what_is_not_a_variant`, `variation_bound_to_two_variants`, `semantics/variation_test.go:TestSelectsVariantOfRejectsForeignVariant` | ✅ Faithful |
| A `variant` whose owner is not a variation offers no choice, so it stays an ordinary feature of its owner and the idle `variant` keyword is reported; an owner that is a variation point by specialization still offers its variants as choices | `passes/constraint.go` `checkVariantOutsideVariation` (warning `variant-outside-variation`), `runtime/shape.go` `buildFeatures` and `runtime/eval.go` (only a variant of a variation point is a choice rather than a slot, via `semantics/variation.go` `Model.VariationPointOwning` over `IsVariationFeature`, which `semantics/model.go` `DirectSupertypes` also uses so a variant specializes such a point and inherits its type, and `VariantsOf` uses so the choices offered are the choices a selection accepts) | `variant_outside_a_variation.sysml`, `variant_under_an_inherited_variation.sysml`, `robustness_test.go:variant_outside_a_variation`, `variant_under_a_redefined_variation`, `passes/constraint_test.go:TestConstraintVariantOutsideVariation`, `TestConstraintVariantInsideVariationOK`, `TestConstraintVariantUnderInheritedVariationOK`, `semantics/variation_test.go:TestVariantsOfExcludesAMisplacedInheritedVariant` | ✅ Faithful |
| The object a selected variant stands for belongs to the selection that made it: two owners, or two variation points read through their declarations, each get their own object | `runtime/variation.go` `variantObject` (keyed by owning object, variation point and variant), `variantValue` | `robustness_test.go:two_owners_selecting_one_variant`, `two_ownerless_selections_of_one_variant`, `repeated_reads_of_a_variant_object` | ✅ Faithful |
| `variation interface` and its `variant interface … connect …` members | `parser/defusage.go` (interface usages take the same modifiers), selection as above; the selected variant's connection is realized by `runtime/variation.go` `variantInstance` over `runtime/connector.go` `materializeConnector`, and routing follows it through `runtime/routing.go` `routableConnections`/`realizedConnections`/`selectedVariant` over `lower/connection.go` `Connection.Variation`/`Variant`/`Owner` and `ToObjectConnections`, with the object performing the behavior carried by `runtime/context.go` `ExecuteActionPerformedBy`/`ExecuteStatePerformedBy` | `variation_interface_selection.sysml`, `variation_interface_mismatch.sysml`, `ballandchain_interface_connected.sysml`, `ballandchain_interface_disconnected.sysml`, `ballandchain_variant_configuration.sysml`, `signal_test.go:TestRoutingHonorsTheSelectedVariantConnection`, `lower/connection_test.go:TestLowerVariantConnectionsCarryTheirVariation`, `:TestLowerObjectConnectionsAreOwnedByTheObject`, `variant_connection_per_owner.sysml`, `signal_test.go:TestRoutingIsPerOwnerVariantSelection` | ✅ Faithful (the variant is selected and compares equal, so a configuration constraint over interface variants evaluates, and the connection that variant declares is a real runtime connector whose ends are the connected features, so port communication follows the selected variant and not the variants left unselected; a connection an object declares routes for that object, so two objects of one type selecting different variants each route their own) |

⚠️ Variant selection is not ordering-sensitive — a variation slot resolves to one variant per instance — so no golden execution trace accompanies these rows.

### Requirement

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Require expression evaluation, in a requirement definition body as well as a usage | `parser/behavior.go` `parseRequirementBody` (both `parseDefinition` and `parseUsage` paths), `condition.go` `conditionsOf` | `requirement_literal.sysml`, `requirement_def_body_require.sysml`, `parser/behavior_require_member_test.go:TestRequirementConditionForms` | ✅ Faithful |
| A condition stated through an anonymous nested constraint (`require constraint { <expr> }`, the form the OMG Domain Libraries use) is evaluated, and every condition of that body is kept | `parser/behavior.go` `parseNestedConstraintConditions`, `condition.go` `appendConditions` | `requirement_nested_constraint.sysml`, `parser/behavior_require_member_test.go:TestRequireMemberRetainsConditions` | ✅ Faithful |
| A requirement's conditions see the requirement's own features — declared, inherited, or rebound by a typed usage (`attribute :>> maxVerticalSpeed = 1.5;`) | `condition.go` `conditionFeatures`, `eval.go` `evalFeatureReference` | `requirement_own_attribute.sysml`, `requirement_nested_constraint.sysml`, `runtime/condition_test.go:TestRequirementConditionSeesOwnAttributes` | ✅ Faithful |
| A feature a condition names but which carries no value reports that (`ErrNoValue`) rather than being unresolved | `errors.go` `ErrNoValue`, `eval.go` `evalFeatureReference` | `runtime/condition_test.go:TestRequirementConditionWithoutValueIsNotUnresolved` | ✅ Faithful |
| A violated condition names the condition that failed, not only the element stating it | `errors.go` `ViolationError`, `condition.go` `conditionText` | `requirement_violated.sysml`, `runtime/condition_test.go:TestRequirementConditionSeesOwnAttributes` | ✅ Faithful |
| Subject binding evaluation | `context.go:148` `EvaluateRequirement` (Pass 1) | `requirement_subject.sysml` | ✅ Faithful |
| Actor binding evaluation | `context.go:148` `EvaluateRequirement` (Pass 1) | `requirement_actor.sysml` | ✅ Faithful |
| Assume expression evaluation | `context.go:148` `EvaluateRequirement` (Pass 2, doesn't fail) | `requirement_assume.sysml` | ✅ Faithful |
| A false required condition is a verdict, not a malfunction (`ErrViolated`), like a false assertion | `context.go` `EvaluateRequirementOn`, `errors.go` `ErrViolated` | `repl/instance_test.go:TestRequirementViolationIsAVerdictNotAnError` | ✅ Faithful |
| A requirement usage inherits assume/require conditions from the definition it is typed by, and the values it rebinds are the ones those conditions see | `context.go` `chainMembers`, `condition.go` `conditionFeatures` | `requirement_nested_constraint.sysml`, `requirement_violated.sysml` | ✅ Faithful |
| A `subject` may redeclare the one it inherits (`subject subj : View[1] :>> RequirementCheck::subj;`) | `parser/behavior.go` `parseSubjectMember`, `resolve/document.go`, `passes/typecheck.go` `checkSubjectMember` | `parser/behavior_require_member_test.go:TestRequirementConditionForms`, `libs/stdlib_conformance_test.go` (`Systems Library/Views.sysml`) | ✅ Faithful |
| Nested requirements | `context.go:148` `EvaluateRequirement` (recursive) | `requirement_nested.sysml` | ✅ Faithful |
| `satisfy <name>` is an `OwnedReferenceSubsetting` of an existing usage, not a typing (SysML v2 §8.3.21.10 `SatisfyRequirementUsage`) | `parser/defusage.go` `parseDefUsage` (`ast.RelSubsets`) | `parser/testdata/parse/satisfy_reference.golden` | ✅ Faithful |
| `referencedFeatureTarget().oclIsKindOf(RequirementUsage)` — satisfy/verify may only reference a requirement usage (incl. viewpoint/concern usages) | `passes/typecheck.go` `compatMessage`, `isRequirementUsageKind` | `passes/typecheck_test.go` `TestTypeCheckSatisfyRequirementUsageOK`, `TestTypeCheckSatisfyViewpointUsageOK`, `TestTypeCheckSatisfyNonRequirementUsageError` | ✅ Faithful |
| `assert satisfy <requirement> by <part>;` is a verdict of its own: the assertion is evaluated as the requirement usage it is (`SatisfyRequirementUsage`, SysML v2 §8.3.21.10), with the requirement's subject parameter bound to the object the `by` feature supplies, so the conditions read that object's values | `runtime/satisfy.go` `SatisfyAssertionsIn`, `EvaluateSatisfactionOn`, `repl/meta.go` `doSatisfy` | `satisfy_subject_binding.sysml`, `satisfy_inherited_conditions.sysml`, `runtime/satisfy_test.go`, `repl/satisfy_test.go:TestSatisfyVerdicts` | ✅ Faithful |
| An assertion may be negated (`assert not constraint { … }`, `assert not satisfy … by …`; `Invariant::isNegated`, SysML v2 §8.3.21.10), and holds exactly when the conditions it denies do not | `ast/defusage.go` `Usage.IsNegated`, `parser/defusage.go` `applyFeatureMods`, `runtime/condition.go` `evaluateConditions` | `parser/testdata/parse/assert_negated.golden`, `satisfy_negated.sysml`, `runtime/negation_test.go` | ✅ Faithful |
| A negated element states no condition it can deny when its body holds only assumptions, which are trusted rather than checked, so it reports `no condition to evaluate` rather than a violation naming none | `runtime/condition.go` `evaluateConditions` | `runtime/condition_test.go` `TestNegatedConstraintWithOnlyAssumptionsIsNotAVerdict` | ✅ Faithful |
| A negation denies the conditions of the constraint it is written on **together** — `not (a and b)`, not `not a and not b` — so it holds as soon as one of them fails | `runtime/condition.go` `appendConditions`, `conditionHolds` | `runtime/condition_test.go` `TestNegatedNestedConstraintNegatesTheConjunction`, `constraint_negated_group.sysml` | ✅ Faithful |
| An `ObjectiveMembership`'s `ownedObjectiveRequirement` is a `RequirementUsage` (SysML v2 §8.3.22.4), so an `objective` is typed by a requirement definition or a specialization of one, never by a structural definition | `passes/typecheck.go` `compatibleTyping`, `isRequirementDefKind` | `passes/typecheck_kinds_test.go` `TestTypeCheckObjectiveTypedByRequirementDefOK`, `TestTypeCheckObjectiveTypedByConcernDefOK`, `TestTypeCheckObjectiveTypedByPartDefError`, `TestTypeCheckObjectiveTypedByActionDefError` | ✅ Faithful |
| A `SubjectMembership`'s `ownedSubjectParameter` is an unconstrained `Usage` (SysML v2 §8.3.21), so a definition of any kind types a `subject` — including the `port def` and `action def` the OMG training models use — and the rule applies however the requirement body is written, not only when the subject happens to parse as a usage | `passes/typecheck.go` `checkSubjectMember`, `compatibleTyping` | `passes/typecheck_subject_test.go` `TestTypeCheckSubjectIsCheckedWhateverPrecedesIt`, `TestTypeCheckRequirementUsageSubjectIsChecked`, `TestTypeCheckSubjectWithoutResolvableTypeIsNotATypeError`; `typecheck_kinds_test.go` `TestTypeCheckSubjectTypedByAnyDefKindOK`, `TestTypeCheckSubjectTypedByUsageError` | ✅ Faithful |
| A quantity expression (`1.5 [m/s]`) evaluates to a magnitude and the measurement reference it is written in (`Quantities::ScalarQuantityValue` is `num` + `mRef`), so a condition comparing values written with units reaches a verdict | `runtime/quantity.go` `evalIndexExpr`, `value.go` `ValQuantity` | `requirement_quantity_same_unit.sysml`, `runtime/quantity_test.go:TestQuantityEvaluation`, `parser/testdata/parse/quantity_expression.golden` | ✅ Faithful |
| **Name in the unit position of a quantity expression.** `x [u]` invokes `Quantities::'['(num, mRef)`, so `u` is an ordinary operand expression and its name is resolved by ordinary name resolution: resolution returns the *nearest* declaration the name reaches (KerML 8.2.3.5.3 Local and Visible Resolution, 8.2.3.5.4 Full Resolution), and the position's expected type (`ScalarMeasurementReference`) only decides whether what resolved conforms (KerML 8.2.3.5.1). A sibling named `m` therefore shadows an imported `SI::m` — resolution does **not** continue outward looking for a unit — and the quantity is rejected with a diagnostic naming the declaration, the namespace declaring it, and the qualified spelling of the unit it hid. One routine implements this for every evaluator (part slot default, action/state attribute default, calc return, condition) | `semantics/units.go` `unitTermOfName`, `ShadowedUnitError`, `unitOutside`; `runtime/context.go` `chainMembers` (a condition evaluates in its own body scope, as the other paths do) | `unit_shadowed_by_sibling_slot.sysml`, `unit_shadowed_by_sibling_action.sysml`, `unit_shadowed_by_sibling_calc.sysml`, `unit_shadowed_by_sibling_constraint.sysml`, `unit_shadowed_by_local_unit.sysml`, `unit_undeclared.sysml`, `robustness_test.go:quantity_unit_shadowed_by_sibling` | ✅ Faithful |
| A violated assertion renders a quantity operand as it was written (`1.0 [m] > 500.0 [m]`), since the bracket form is a quantity and not a sequence index | `runtime/condition.go` `conditionText` (`ast.IndexExpr`) | `runtime/condition_test.go:TestViolationRendersQuantityOperands` | ✅ Faithful |
| Commensurable units convert before a comparison or a sum, through `MeasurementUnit::unitConversion` and unit-defining expressions reduced to base units — `1.5 [m/s] <= 5.4 [km/h]` is true, exactly, at its boundary (a conversion factor is kept as a ratio, not evaluated) | `semantics/units.go` `UnitTermOf`, `Scale`, `ConvertMagnitude` | `requirement_quantity_converted_unit.sysml`, `constraint_quantity_sum.sysml`, `semantics/units_test.go:TestScaleStaysExact` | ✅ Faithful |
| A unit is composed by the operation over quantities (`10 [m] / 2 [s]` is `5 [m/s]`), and a ratio of like quantities is a number of no unit. A composed operand is parenthesized in the composed unit's text, so `(m/s) * (kg/s)` names `m/s*(kg/s)` rather than a unit that re-reads as `m/(s*kg)/s` | `runtime/quantity.go` `scaleQuantities`, `composedUnitText`, `groupUnitText`, `semantics/units.go` `UnitTerm.DividedBy` | `constraint_quantity_quotient.sysml`, `calc_quantity_ratio.sysml`, `runtime/quantity_test.go:TestComposedUnitText` | ✅ Faithful |
| A quantity raised to a constant exponent raises its unit with it, and its magnitude comes from the one `**` implementation the folder and the runtime share — so `(0.0 [m]) ** -1.0` and an overflowing magnitude are the same typed errors as for a bare number rather than an infinity carried in a unit, and `(2 [m]) ** 3` keeps an Integer magnitude | `runtime/quantity.go` `powQuantity`, `semantics/eval.go` `Pow`, `semantics/units.go` `UnitTerm.Pow` | `runtime/quantity_test.go:TestQuantityExponentiation`, `TestQuantityExponentiationReports` | ✅ Faithful |
| An execution trace of a unit-carrying value renders the magnitude and the unit (`5.0 [m/s]`), as the REPL prints a quantity | `runtime/trace.go` `FormatTraceValue` | `action_quantity_assign.trace.golden`, `runtime/quantity_test.go:TestFormatTraceValueQuantity` | ✅ Faithful |
| Incommensurable units are a typed error (`ErrIncommensurableUnits`), never a comparison of bare magnitudes that would equate `1.5 [m/s]` with `1.5 [km/h]` | `runtime/errors.go` `ErrIncommensurableUnits`, `quantity.go` `convertTo` | `runtime/robustness_test.go` `quantity_incommensurable_comparison`, `runtime/quantity_test.go:TestQuantityIncommensurable` | ✅ Faithful |

⚠️ The spec's own `QuantityCalculations::ConvertQuantity(x, targetMRef)` is not an invocable function, and a quantity is not yet an instantiated `ScalarQuantityValue` object whose `num`/`mRef` features can be read by name: the unit is carried on the runtime value, not modelled as a library object. The gRPC value schema has no magnitude-and-unit form, so a quantity crossing that boundary is reported as unsupported rather than serialized as a bare magnitude (`internal/grpc/convert.go`). Sequence indexing (`speeds#(3)`), which the parser represents with the same node, is evaluated as the index it is (see *Sequence Indexing and Collection Operations* below): the two forms are told apart at the node (`ast.IndexExpr.Bracket`), so an index is never read as a magnitude in a unit, nor a quantity as an index.

⚠️ A requirement feature that carries no value of its own is read from the satisfying object's feature of that name, which is how a requirement stated over the values it checks (`attribute verticalSpeed;` compared against a limit) reaches a verdict from `by`. The spec supplies a subject's values to a requirement through the subject parameter (`subject lander : Lander;` then `lander.verticalSpeed`) or an explicit binding, not by matching names, so this fallback — the same one `%requirement` applies on an instance — is an approximation, and a requirement whose unbound feature happens to share a name with an unrelated feature of the subject would be checked against it. A requirement whose value comes from neither its own binding nor the subject (the lunar lander model's `actualVerticalSpeed`, produced by an analysis) still has no value to check and reports `ErrNoValue`.

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
| Assignment statement in a body (`assign x := <expr>`) | `lower/action_graph.go` lowerStatement Assign; `runtime/action_statements.go` execStatement | `action_send_accept.sysml`, `lower/action_body_test.go:TestActionBodyLowering` | ✅ Faithful |
| Conditional statement (`if <cond> { … } else { … }`) | `lower/action_graph.go` lowerStatement/lowerBlock (`If`); `runtime/action_statements.go` execIf, execBlock | `action_if_else_then_branch.sysml`, `action_if_else_else_branch.sysml`, `action_if_no_else.sysml`, `action_nested_loop_if.sysml` + trace golden, `lower/action_body_test.go:TestActionBodyLoopAndConditionalLowering`, `passes/typecheck_test.go:TestTypeCheckNonBooleanControlFlowConditions` | ✅ Faithful (the condition is evaluated outside both branches; each branch body is a namespace of its own, so the names it declares do not reach the enclosing behavior) |
| Pre-condition loop (`while <cond> { … }`) | `lower/action_graph.go` lowerStatement (`Loop`, `ast.LoopWhile`); `runtime/action_statements.go` execLoop | `action_while_loop.sysml` + trace golden, `action_while_loop_zero_iterations.sysml`, `parse/action_loop_forms.golden` | ✅ Faithful (tested before every iteration, so the body may run no times) |
| Post-condition loop (`loop { … } until <cond>;`) | `parser/behavior.go` parseLoopAction; `lower/action_graph.go` (`ast.LoopUntil`); `runtime/action_statements.go` execLoop | `action_loop_until.sysml`, `action_loop_until_repeats.sysml` + trace golden | ✅ Faithful (tested after every iteration, so the body runs at least once) |
| Iteration over a collection (`for <x> in <collection> { … }`) | `ast/behavior.go` `WhileLoopActionNode.Variable`/`Collection`; `symbols/builder.go` (the variable is a member of the loop's own scope); `runtime/action_statements.go` execForLoop, forElements | `action_for_loop.sysml`, `parse/action_loop_forms.golden` | ⚠️ Approximate (the collection is evaluated once, before the loop is entered, and must evaluate to a sequence or a set — the only collections the expression layer produces; a set is visited in the order its canonical rendering sorts in, since a set has no order of its own) |
| A non-terminating loop ends the execution rather than hanging it | `runtime/action_statements.go` execLoop (a step per iteration), `context.go` incrementStep | `action_loop_step_budget.sysml`, `robustness_test.go:non_terminating_loop_exhausts_step_budget` | ✅ Faithful (reports `ErrStepLimitExceeded`, the same failure as any other runaway evaluation) |
| A legitimately long loop runs under a raised budget | `budget.go` `BudgetsFromEnv` (`SYSML_MAX_STEPS`) resolved at the REPL/CLI and gRPC entry points | `budget_test.go:TestRaisedBudgetRunsLongerLoop` | ✅ Faithful (a 10 000-iteration loop that exhausts a 100 000-step budget completes under the default) |
| The budget bounds one run, not a session | `context.go` `beginRun`/`beginExecutorRun` (the step counter is reset when a run begins; a nested run, and every call into a run a caller drives step by step, shares the outer one's budget) | `budget_test.go:TestStepBudgetIsPerRun`, `:TestStepBudgetHoldsAcrossExecutorDrivenRun`, `:TestStepBudgetIsPerRunForInstancesAndCalcs` | ✅ Faithful |
| A legitimately long action or simulation runs under raised sibling budgets | `budget.go` `Budgets` (`SYSML_MAX_ACTION_STEPS`, `SYSML_MAX_EVENTS`, `SYSML_MAX_DO_STEPS`), read by `action_executor.go` and `state_executor.go` from the context | `budget_test.go:TestActionStepBudgetIsConfigurable`, `budget_test.go:TestStateBudgetsAreConfigurable` | ✅ Faithful (each bound counts its own unit and its error names the variable that raises it) |
| A member-attached `then` sequences the members either side of it (`action a; then action b;`) | `parser/succession.go` (desugared at parse time to the `*ast.SuccessionEdge` the `then a b;` notation builds), lowered by `lower/action_graph.go` and `lower/state_graph.go` like any other edge | `conformance/action_member_then_order.sysml` + trace golden (declaration order is the reverse of the execution order), `conformance/state_member_then_order.sysml` (the same for a state's completion transitions), `parser/succession_test.go:TestMemberAttachedThenDesugars`, `parse/succession_member_then.golden` | ✅ Faithful (an end with no name to give is bound by position: `SuccessionEdge.SourceMember`/`TargetMember` refer to the member itself, so `then send Show(x) to screen;`, a `then` after an anonymous member and `then loop action { … }` all sequence what they are written beside. A `then` before a member the notation does not admit one in front of, such as an attribute or a definition, is a syntax error) |
| A succession end with no name is carried by identity, not by name | `ast/behavior.go` `SuccessionEdge.SourceMember`/`TargetMember`, `ControlFlowEdge.SourceMember`/`TargetMember`; `parser/succession.go` `bindPositionalSource`/`bodyBuilder.add`; `lower/action_graph.go` (the member is the graph node the edge reaches) | `parser/succession_test.go:TestSuccessionBindsUnnamedEndsByPosition`, `:TestPositionalSuccessionEndIsTheMemberItself`, `conformance/action_standard_loop_until_then_done.sysml` | ⚠️ Approximate (the RDF mapping names an edge's ends by qualified name, so exporting a model whose succession has a positional end is reported as unsupported rather than written back — see `docs/RDF_INTEROP.md`) |
| Named flow with explicit ends (`flow f from a.out to b.in;`, SysML.xtext `FlowUsage` → `PayloadFeatureSpecializationPart` + `FlowEndMember`) | `parser/defusage.go` `parseFlowEnds`/`parseFlowTo` (the name, `from` and feature-chain ends); `ast/defusage.go` `FlowEnds`; `lower/action_graph.go` `lowerFlow`/`flowEnd` (`ObjectFlow`, the flow's name included); `runtime/action_executor.go` `applyDataFlows` | `parse/behavior_flow_named_from.golden`, `parse/flow_payload_declaration.golden`, `negative_test.go:flow_from_without_to`, `:flow_named_from_no_source`, `conformance/action_flow_named_from.sysml`, `robustness_test.go:flow_end_naming_no_node`, `:flow_from_a_node_that_produced_nothing` | ✅ Faithful (both ends, feature chains included, name a node and its pin, and the value at the source's pin is what the target reads; `flow of x from a to b` names the pin at both ends. An end naming something that is not a node of the action, and a source pin the node left empty, are reported rather than dropped) |
| Accept with a trigger expression in an action body (`accept when <cond>`, `accept at <instant>`, `accept after <duration>`; SysML.xtext `TriggerValuePart`) | `parser/behavior.go` `parsePayloadParameter`/`parseTriggerExpression`; `lower/action_graph.go` `Accept.Trigger`; `runtime/action_executor.go` `triggerHolds` | `parse/behavior_accept_trigger.golden`, `conformance/action_accept_when_trigger.sysml`, `negative_test.go:accept_when_no_condition`, `:accept_at_no_instant`, `robustness_test.go:action_accept_time_trigger`, `:action_accept_non_boolean_change_trigger` | ⚠️ Approximate (a change trigger is tested each step and suspends the token until it holds; an action body has no clock, so `accept at`/`accept after` there reports `ErrNoClock` naming the state machine that does wait on time, rather than firing) |
| Accept subsetting a declared event (`action interrupt accept :> shutDown;`, SysML.xtext `PayloadParameter` → `PayloadFeatureSpecializationPart`) | `parser/behavior.go` `parsePayloadParameter`; `lower/action_graph.go` `subsettingTarget` → `Accept.SubsetsEvent`; `runtime/action_executor.go` (the subsetted event names the message the accept takes) | `parse/behavior_accept_subsets.golden`, `conformance/action_accept_subsets_event.sysml`, `negative_test.go:accept_subsets_no_event` | ✅ Faithful (a `send shutDown() to interrupt` is taken by the accept subsetting `shutDown`, as a typed accept takes a message of its type) |
| Send of an invoked signal or event (`send Data(reading) via commPort;`, `then send fullyCharged() to self;`) | `parser/behavior.go` `parseSendStatement`; `runtime/signal.go` `buildMessage`/`buildInvokedMessage`/`invokesCalc` | `parse/behavior_send_via.golden`, `conformance/action_send_invocation_via_port.sysml`, `negative_test.go:send_via_no_port`, `:send_no_target` | ✅ Faithful (the invoked name types the message and its arguments are its payload — a single positional argument also as `value`, which a typed accept binds; an invocation of a calc is still evaluated as an expression) |
| Succession to a loop node, and a loop's `until` condition (`then loop action { … } until battery >= 100;`, SysML.xtext `WhileLoopNode`) | `parser/behavior.go` `parseLoopAction`/`parseWhileLoopAction`; `ast/behavior.go` `WhileLoopActionNode.Until`; `lower/action_graph.go` (loop body and `Until` lowered); `runtime/statements.go` `iteration` | `parse/behavior_loop_until_succession.golden`, `conformance/action_standard_loop_until_then_done.sysml`, `negative_test.go:loop_until_no_condition`, `:loop_until_no_semicolon`, `passes/typecheck.go` `checkBehaviorMember` (the `until` condition is checked Boolean) | ✅ Faithful (`loop { … } until c` tests after the iteration; `while c action { … } until u` tests `c` before and `u` after) |
| A loop or branch body written as an action body parameter (`loop action [<name>] { … }`, `for x in c action { … }`, SysML.xtext `ActionBodyParameter`) is the body itself, named or not | `parser/behavior.go` `parseActionBodyParameter` (marks `ast.Usage.IsBodyParameter`); `lower/action_graph.go` `lowerStatement` (lowered to the block the usage's scope owns); `runtime/statements.go` `block` | `lower/action_body_test.go:TestActionBodyParameterLowersToItsBlock`, `conformance/action_named_loop_body_parameter.sysml` | ✅ Faithful (a name only scopes the members it declares — `loop action charging { … } until charging.done` — so the body runs either way; an empty body parameter runs as an empty body) |
| `then done;` — a final node as a successor target | `parser/behavior.go` `parseFinalNode` (reached by a succession); `lower/action_graph.go` `Finals`; `runtime/action_executor.go` `stepFinalNode` | `parse/behavior_loop_until_succession.golden`, `conformance/action_standard_loop_until_then_done.sysml`, `negative_test.go:then_done_no_semicolon` | ✅ Faithful (the token reaching it ends the flow, as for a declared `done end;`) |
| `else` branch of a decision in an action flow (`if c then a; else b;`, SysML.xtext `DefaultTargetSuccession`) | `parser/behavior.go` (the `else` clause builds `*ast.ControlFlowEdge{IsElse: true}`); `lower/action_graph.go` (the else edge is the guardless alternative); `runtime/action_executor.go` `stepDecisionNode` | `parse/behavior_decision_else.golden`, `conformance/action_decision_else_branch.sysml`, `conformance/action_decision_guarded_branch.sysml`, `negative_test.go:decision_else_no_target` | ✅ Faithful (the else edge is taken when no guarded branch out of the decision holds) |
| Qualified succession at namespace level (`first part1::action1 then requirement1;`) | `parser/namespace.go` (a succession is a namespace member), `parser/succession.go` | `parse/behavior_namespace_succession.golden`, `negative_test.go:namespace_succession_no_target` | ⚠️ Approximate (parsed and carried in the AST with both ends; a succession outside a behavior body has no token flow to lower into, so it is not executed) |
| A statement written directly among an action's own members is reported, not ignored | `lower/action_graph.go` ToActionGraph first pass, statementKeyword | `robustness_test.go:statement_directly_in_an_action_body` | ✅ Faithful (a statement runs as part of an action node's body; written beside `first`/`then` it has no name a succession could reach, so the execution reports it instead of dropping it) |
| A body member that is not an executable statement is reported, not skipped | `lower/action_graph.go` `Unsupported`; `runtime/action_statements.go` execStatement | `lower/action_body_test.go:TestActionBodyUnexecutableMemberIsLowered`, `robustness_test.go:loop_body_of_unexecutable_statement` | ✅ Faithful (a declaration in a loop or branch body that the runtime cannot perform — a nested action, a `perform` — fails the execution instead of producing a wrong answer silently) |
| Send statement (message passing) | `lower/action_graph.go` lowerBody; `runtime/signal.go` buildMessage, post | `action_send_accept.sysml`, `lower/action_body_test.go:TestActionBodyLowering`, `signal_test.go:TestActionMessageReachesStateMachine` | ✅ Faithful (a message is typed by what was sent and addressed to the named receiver) |
| Accept action (message consumption suspends the action) | `action_executor.go` stepNestedAction accept case (parks the token as `Token.Wait`), Step (StateWaiting), RunToCompletion, deadlockError; `executor_common.go` AcceptWait; `runtime/signal.go` TakeMessage | `action_accept_suspends_until_message.sysml` + trace golden, `action_accept_two_waiters.sysml` + trace golden, `action_send_accept.sysml`, `action_accept_message.sysml`, `signal_test.go:TestAcceptParksTokenUntilMessageArrives`, `:TestParkedAcceptTakesOnlyItsOwnMessage`, `robustness_test.go:accept_deadlock_never_satisfied`, `:accept_deadlock_reports_every_waiting_accept`, `:send_reaches_only_its_addressee`, `:accept_of_unsent_type`, `:send_via_unconnected_port` | ⚠️ Approximate (an accept with no message it can take suspends the action at that node and resumes when one arrives, from a parallel branch or from another executor sharing the context; a run whose every remaining token is parked reports `ErrAcceptDeadlock` rather than hanging. Suspension is bounded by the executor: a nested action invoked synchronously, and an action driven by `RunToCompletion`, cannot wait for a message posted after the call begins) |
| Send through a port (`send x via p`) | `lower/connection.go` lowerConnections, PeerPorts; `runtime/signal.go` postVia, arrivedAt | `action_port_communication.sysml` + trace golden, `lower/connection_test.go:TestLowerConnectionsFromActionBody`, `signal_test.go:TestSendViaPortReachesConnectedAccept`, `robustness_test.go:send_via_unconnected_port` | ⚠️ Approximate (the message reaches every port connected to the sending port by a connector declared in the same behavior body; a port of the enclosing part is not visible to the behavior, and conjugation and port direction do not restrict routing) |
| Accept through a port (`accept msg : T via p`) | `lower/action_graph.go` acceptPort; `runtime/action_executor.go` stepNestedAction accept case | `action_port_communication.sysml`, `lower/connection_test.go:TestLowerAcceptRecordsViaPort`, `signal_test.go:TestPortRoutedMessageBypassesPortlessAccept`, `:TestAddressedMessageBypassesPortAccept` | ✅ Faithful (an accept on a port takes only messages routed to that port, and an accept on none takes only addressed messages) |
| Object flow (pin-to-pin data) | `action_executor.go:673` applyDataFlows | `action_output.sysml` | ✅ Faithful |
| Succession edges | `lower/action_graph.go:ToActionGraph` | `action_control_flow.sysml` | ✅ Faithful |
| Deadlock detection | `action_executor.go:72` Step | `action_executor_test.go:TestActionExecutor_Deadlock_JoinStarvation` | ✅ Faithful |
| Step budget enforcement | `context.go` incrementStep; budget configured by `SYSML_MAX_STEPS` (`budget.go` `BudgetsFromEnv`) | `robustness_test.go:testStepBudgetExceeded`, `budget_test.go:TestRaisedBudgetRunsLongerLoop` | ✅ Faithful (the reported limit is the effective one, and names the variable that raises it) |

### State Machine (UML 2.5.1 §14 StateMachines)

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Initial state identification | `lower/state_graph.go:ToStateGraph`; `state_executor.go:686` initialize | `state_simple.sysml` | ✅ Faithful |
| A succession out of a state body's own entry subaction names the state it starts in (`entry; then off;`), the same as `initial start; start then off;` | `lower/state_graph.go` collectTransitions SuccessionEdge case + `isEntrySubaction` | `lower/state_notation_test.go:TestToStateGraph_EntrySuccessionNamesInitialState`, `state_entry_succession_initial.sysml` conformance, `parser/testdata/parse/behavior_exhibit_state_body.golden` | ✅ Faithful |
| Final state termination | `state_executor.go:288` processNextEvent | `state_simple.sysml` | ✅ Faithful |
| State entry actions | `state_executor.go:749` enterState | `state_do_behavior.sysml` | ✅ Faithful |
| State exit actions | `state_executor.go:810` exitState | `state_transition_effect.sysml` | ✅ Faithful |
| An entry/do/exit action given by reference (`entry warmUp;`) is a performed action usage subsetting the referenced action (`StateActionUsage` → `PerformedActionUsage` → `PerformActionUsageDeclaration`) | `parser/behavior.go` parseStateSubaction; `parser/defusage.go` parsePerformedActionReference | `parser/testdata/parse/state_subaction_reference.golden`, `parser/state_subaction_test.go`, `parser/negative_test.go:entry_reference_no_semicolon`, `resolve/state_subaction_test.go`, `runtime/state_behavior_test.go:TestStateSubactionByReferencePerformsAction` | ✅ Faithful |
| State do behavior runs while its state is active, one action per round | `state_executor.go` startDoActivity, runDoRound | `state_do_behavior.sysml`, `state_do_activity_test.go` | ✅ Faithful |
| An entry, do or exit behavior written as an inline action body (`entry action { … }`, `do action named { … }`, `exit action { … }`) executes the statements it states, locals and loops among them, in any nesting of composite states; an empty body is a behavior that does nothing | `lower/state_behavior.go` `LowerBehaviors`/`lowerStateBehavior` (the body is lowered to a `Block`, its locals in the block's own frame), `lower/state_graph.go` `StateGraph.Behaviors`; `runtime/state_statements.go` `executeBehavior`/`stateStmtHost` | `parser/testdata/parse/state_anonymous_action_body.golden`, `state_anonymous_action_body.sysml` + trace golden (entry/do/exit ordering, nesting, empty bodies), `robustness_test.go:testEmptyAnonymousActionBody`, `:testNonTerminatingAnonymousDoBody` (`ErrStepLimitExceeded`) | ✅ Faithful |
| An inline body is one action, so a do round runs it to its end: orthogonal regions interleave between rounds, not inside a body. The one-action-per-statement `do { … }` form is what interleaves statement by statement | `runtime/state_statements.go` `executeBehavior`; `state_executor.go` runDoRound | `state_anonymous_do_atomic.sysml` + trace golden (123456), against `state_concurrent_do.sysml` (124356) | ✅ Faithful |
| A statement of an inline body may perform an action (`entry action { assign c := c + 1; perform Bump; }`), the performed action being lowered as an effect rather than an unsupported usage | `lower/action_graph.go` `lowerStatement` (an action usage naming what it performs → `Effect{EffectPerform}`); `runtime/state_statements.go` `stateStmtHost.effect` | `state_anonymous_body_perform.sysml` conformance | ✅ Faithful |
| A behavior that both performs an action and states a body of its own is reported rather than one of the two being chosen silently | `lower/state_behavior.go` `lowerStateBehavior` (`Unsupported`) | `robustness_test.go:testBehaviorPerformingAnActionAndStatingABody` | ⚠️ Approximate (the form is rejected at execution, not at parse: whether SysML gives it a meaning is unadjudicated) |
| Concurrently active states interleave their do behaviors, in region declaration order | `state_executor.go` runDoRound, orderedActiveRegions | `state_concurrent_do.sysml` + trace golden | ✅ Faithful |
| Exiting a state abandons the rest of its do behavior | `state_executor.go` exitState, stopDoActivity | `state_do_activity_test.go:TestDoBehaviorIsCancelledWhenItsStateIsExited` | ✅ Faithful |
| A state completes only once its do behavior has finished | `state_executor.go` scheduleCompletionTransitions | `state_do_activity_test.go:TestCompletionWaitsForTheDoBehavior` | ✅ Faithful |
| Deferred events retained while a deferring state is active, delivered afterwards in arrival order | `parser/behavior.go` parseDeferMember (`defer <event>[, <event>]*;`); `lower/state_graph.go` stateNodeFromUsage, collectDeferred; `state_executor.go` defersEvent, recallDeferredEvents | `parser/testdata/parse/state_defer.golden`, `parser/state_notation_test.go:TestDeferMemberParsing`, `lower/state_notation_test.go:TestToStateGraph_DeferNotation`, `state_deferred_event.sysml` + `state_undeferred_event.sysml` conformance, `state_deferred_test.go` | ✅ Faithful |
| Earliest transfer first (`Occurrence::incomingTransferSort` defaults to `earlierFirstIncomingTransferSort`): a recalled event is dispatched before the events that arrived while it was deferred, and a completion event before either | `executor_common.go` eventHeap.Less, isCompletionEvent; `state_executor.go` recallDeferredEvents | `state_deferred_test.go:TestRecalledEventPrecedesLaterArrivals` | ✅ Faithful |
| An event reaches only the regions still active when it is dispatched | `state_executor.go` broadcastEvent | `state_deferred_test.go:TestExitedNestedRegionDoesNotReactToTheSameEvent` | ✅ Faithful |
| Deferral by an ancestor state and across orthogonal regions | `state_executor.go` defersEvent | `state_deferred_test.go:TestCompositeStateDefersForItsSubstates`, `TestDeferralSpansOrthogonalRegions` | ✅ Faithful |
| Deferring a non-dispatchable trigger reports | `lower/state_graph.go` collectDeferred | `robustness_test.go:defer_of_non_deferrable_trigger` | ✅ Faithful |
| Transition firing | `state_executor.go:535` fireTransition | `state_transition_effect.sysml` | ✅ Faithful |
| Transition guard evaluation | `state_executor.go:218` scheduleTransitionsForState | `state_choice_pseudostate.sysml` | ✅ Faithful |
| Transition effect actions, whether written as a statement (`do assign x := 1`) or as a performed action (`do perform Bump`) | `lower/state_behavior.go` `LowerBehaviors` (the membership a performed action is contributed through is unwrapped and each behavior is lowered to statements); `state_executor.go:535` fireTransition → `state_statements.go` `executeBehavior` | `state_transition_effect.sysml`, `state_transition_effect_perform.sysml` conformance | ✅ Faithful |
| AcceptEvent triggers (when signal) | `state_executor.go` matchesEvent | `state_signal_discriminate.sysml` | ✅ Faithful |
| Signals sent from state behaviors reaching the machine | `state_statements.go` `stateStmtHost.send`, `state_executor.go` deliverPendingSignal | `state_send_self_signal.sysml` + trace golden, `signal_test.go:TestSendOfNamedTypeReachesStateMachine` | ✅ Faithful |
| A signal in flight on the context bus is dispatched by a single step as well as by a run to completion, so the REPL debugger and `RunToCompletion` agree | `state_executor.go` `ProcessNextEvent`, `acceptableMessage`, `HasPendingSignal`, `HasPendingWork`; `repl/meta.go` `%advance` | `repl/runtime_commands_test.go:TestAdvanceDeliversPendingPortSignal`, `state_transition_accept_via_port.sysml` | ✅ Faithful |
| CallEvent triggers (`accept op(param)` notation, operation and argument matching, arguments bound for guard/effect) | `parser/behavior.go` parseTriggerEvent/parseCallEvent; `symbols/bodyscopes.go` newTriggerScope (parameters visible to the transition's own guard/effect); `state_executor.go` matchesEvent EventCall case, bindTriggerArguments, InvokeOperation | `parser/testdata/parse/state_call_trigger.golden`, `lower/trigger_test.go:TestTriggerClassification_CallTrigger`, `model/behavior_body_resolve_test.go` call-trigger parameter cases, `state_call_trigger{,_guard,_nested,_regions}.sysml` conformance, `signal_test.go:TestCallEventMatchesOperationName`, `:TestRejectedCallLeavesNoArgumentsBehind`, `robustness_test.go:call_of_unhandled_operation`, `:call_argument_of_wrong_type` | ✅ Faithful (a trigger on an enclosing composite state does not see events while a substate is active — the same limitation as every other trigger kind) |
| Sourceless transitions (`accept...then`) | `lower/state_graph.go:487` collectTransitions Usage case, `:302` resolve container | `accept_then_transition.sysml` | ✅ Faithful (nested form only; flat form errors intentionally) |
| ChangeEvent triggers (when expr) | `state_executor.go:401` matchesEvent; `:906` pollChangeEvents | `state_executor_test.go:TestStateChangeEvent` | ✅ Faithful |
| TimeEvent triggers (`accept after <duration>` relative, `accept at <time>` absolute) | `parser/behavior.go` parseAcceptTransition; `state_executor.go` scheduleTransitionsForState, `:401` matchesEvent | `state_timed_triggers.sysml` golden, `state_timed_transitions.sysml` conformance, `state_executor_test.go:TestStateExecutor_AbsoluteTimeEvent`, `robustness_test.go:non_numeric_time_trigger` | ✅ Faithful |
| Signal discrimination | `state_executor.go:401` matchesEvent signal name | `state_signal_discriminate.sysml` | ✅ Faithful |
| Unmatched signal dropped | `state_executor.go` matchesEvent | `state_signal_unmatched.sysml` | ✅ Faithful (an injected event no transition matches is dropped; a message on the bus no active transition accepts is left in flight for another consumer, `signal_test.go:TestStateMachineLeavesForeignSignalPending`) |
| Hierarchical substates | `state_executor.go:131` getParentChain, `:147` getLCA | `state_orthogonal_regions.sysml` | ✅ Faithful |
| Orthogonal regions | `state_executor.go` broadcastEvent, `state_region_transition.go` fireTransitionInRegion; region order from `lower.StateGraph.TopRegions` and `CompositeStates` | `state_orthogonal_regions.sysml`, `region_pseudostate_test.go:TestRegionPseudostateExitOrderIsDeterministic` | ✅ Faithful |
| Choice pseudostates | `state_region_transition.go` pseudostateBranch (guards in declaration order) | `state_choice_pseudostate.sysml`, `state_region_choice.sysml` | ✅ Faithful |
| Junction pseudostates | `state_region_transition.go` pseudostateBranch | `state_junction_pseudostate.sysml` | ✅ Faithful (evaluated when entered, like a choice, rather than statically before the incoming transition) |
| Fork pseudostates (bypass targeted regions' initial states) | `state_executor.go:706` fireForkTransition, `:1028` enterStateInto | `state_fork_join.sysml` golden, `state_fork_join_pseudostate.trace.golden`, `fork_join_test.go:TestForkBypassesTargetedRegionInitials` | ✅ Faithful |
| Join pseudostates | `state_executor.go:782` fireJoinTransition, `:827` joinSources (declaration order) | `pseudostate_test.go:TestJoinWaitsForEveryBranch`, `fork_join_test.go:TestForkJoinVisitOrderIsDeterministic` | ✅ Faithful |
| Entry/exit point pseudostates | `parser/behavior.go` parseStateMember (`entry point <name>;` / `exit point <name>;`, `point` matched contextually); `state_region_transition.go` pseudostateTarget (routed like a junction) | `parser/testdata/parse/state_history_entry_exit.golden`, `parser/state_notation_test.go:TestHistoryAndPointPseudostateParsing`, `:TestPointIsNotReserved`, `state_entry_exit_points.sysml` conformance, `pseudostate_test.go:TestEntryAndExitPointPseudostates`, `region_pseudostate_test.go:TestRegionPseudostateExitRecordsHistory` | ✅ Faithful |
| History pseudostates (shallow and deep) | `parser/behavior.go` parseStateMember (`history <name>;`, `shallow history <name>;`, `deep history <name>;`); `state_executor.go` fireHistoryTransition, `:deepestRecorded`, `exitState` (records the configuration left), `lower/state_graph.go` PseudostateOwner | `parser/testdata/parse/state_history_entry_exit.golden`, `parser/state_notation_test.go:TestHistoryAndPointPseudostateParsing`, `lower/state_notation_test.go:TestToStateGraph_HistoryAndPointNotation`, `state_shallow_history.sysml`, `state_deep_history.sysml`, `state_history_revisit.sysml` + trace golden, `history_test.go:TestShallowHistoryRestoresLastSubstate`, `:TestDeepHistoryRestoresInnermostSubstate`, `:TestHistoryRestoresOrthogonalRegions`, `:TestDeepHistoryRestoresBelowRegion`, `:TestHistoryTakesDefaultTransitionWhenUnvisited`, `robustness_test.go:history_outside_composite_state`, `:history_without_record_or_default` | ✅ Faithful |
| Composite state with regions entered by a plain transition | `state_executor.go` transitionToInto (keeps the region configuration entering it just built) | `history_test.go:TestHistoryRestoresOrthogonalRegions` | ✅ Faithful |
| Leaving a composite state exits only its own regions | `state_executor.go` exitState (scoped to `CompositeStates[state]`) | `history_test.go:TestExitingNestedRegionsKeepsSiblingRegions` | ✅ Faithful |
| Nested substates of a composite state declared textually | `lower/state_graph.go` stateNodeFromUsage (carries substates and nested pseudostates into the graph) | `lower/state_graph_nested_test.go:TestToStateGraph_NestedPseudostateOwner` | ✅ Faithful |
| Choice/junction/entry/exit reached from inside an orthogonal region | `state_region_transition.go` fireTransitionInRegion, moveWithinRegion, leaveRegion, pseudostateTarget | `state_region_choice.sysml`, `state_region_exit_pseudostate.sysml`, `state_region_cross_pseudostate.sysml` + their trace goldens, `region_pseudostate_test.go`, `robustness_test.go:region_pseudostate_without_satisfied_guard`, `:region_pseudostate_cycle` | ✅ Faithful (a branch staying in the source region moves only that region; one leaving it exits the region set in declaration order and re-enters the target's own region branches, recording history on the way out) |
| Pseudostate chains (a pseudostate routing into another) | `state_region_transition.go` pseudostateTarget (cycle detected) | `region_pseudostate_test.go:TestRegionLocalJunctionChainIsFollowed`, `robustness_test.go:region_pseudostate_cycle` | ✅ Faithful |
| Nested action invocation in entry/do/exit/effect | `state_executor.go:1075` executeAction, `invoke_action.go` invokeAction | `state_behavior_test.go:TestStateDoExitAndTransitionEffectPerformAction` | ✅ Faithful |
| Run-to-completion semantics | `state_executor.go:288` processNextEvent | `state_executor_test.go:TestStateRunToCompletion` | ✅ Faithful |
| Event queue management | `state_executor.go:1127` EventQueue | `state_executor_test.go` | ✅ Faithful |
| Deterministic dispatch order | `executor_common.go` eventHeap.Less (time, then arrival), `state_executor.go` orderedActiveRegions (region declaration order) | `state_call_trigger_regions.sysml` | ✅ Faithful |
| Dangling transition detection | `robustness_test.go:testStateDanglingTransition` (lenient) | `robustness_test.go:testStateDanglingTransition` | ⚠️ Approximate |
| Completion transitions | `state_executor.go:218` scheduleTransitionsForState nil trigger | `state_simple.sysml` | ✅ Faithful |
| A transition written in the standard `first`/`accept`/`then` form, with the trigger on a line of its own, and with a name of its own (SysML.xtext `TransitionUsage`) | `parser/behavior.go` `parseTransitionMember`/`parseTransitionTail`; `lower/state_graph.go` `Transition.Name`; `runtime/state_executor.go` `transitionDescription` (the name is what a diagnostic about the transition reports) | `parse/behavior_exhibit_state_body.golden`, `parse/state_transition_variants.golden`, `conformance/state_transition_accept_via_port.sysml`, `negative_test.go:transition_trigger_no_target`, `:transition_two_triggers`, `:transition_two_targets`, `:transition_do_without_action` | ✅ Faithful |
| `accept … via <port>` on a transition (SysML.xtext `AcceptParameterPart`) | `parser/behavior.go` `parseTransitionTail`; `lower/state_graph.go` `Transition.Via`; `runtime/state_executor.go` `matchesEvent`/`acceptsSignal`/`deliverPendingSignal` | `conformance/state_transition_accept_via_port.sysml` | ✅ Faithful (a transition naming a port fires only for an occurrence routed to that port; one naming none takes an addressed message, as before) |
| A transition's accept payload is bound for its guard and effect (`accept w : Warning do assign level := w`) | `lower/state_graph.go` `classifyTrigger` (`AcceptEvent.Payload`); `runtime/state_executor.go` `bindAcceptPayload` | `conformance/state_transition_accept_payload.sysml` | ✅ Faithful (bound while the transition is taken and unbound if it does not fire, as a call trigger's arguments are) |
| A transition triggered by a subsetted event (`accept :> shutDown`) | `lower/state_graph.go` `classifyTrigger` (`AcceptEvent.Subsets`); `runtime/state_executor.go` `triggerMatches`/`acceptsSignal` | `parse/behavior_accept_subsets.golden`, `conformance/action_accept_subsets_event.sysml` (the same matching rule in an action body) | ✅ Faithful |
| A transition's `do` effect written as a statement is terminated by the transition's own `;` (SysML.xtext `TransitionUsage` ends with `ActionBody`, while `EffectBehaviorUsage` carries no `;`), in the standard `first … then` spelling and in Systemica's compact `<source> to <target>` spelling alike | `parser/behavior.go` `expectStatementEnd`/`atTransitionEffectStatement`/`atEffectEnd`; `parser/defusage.go` `parseUsage`/`parseReferenceMemberUsage` | `parse/state_transition_effect_statement.golden`, `conformance/state_transition_effect_assign.sysml`, `conformance/state_transition_effect_assign_first_then.sysml` (with their `.trace.golden`), `negative_test.go:transition_effect_perform_two_semicolons`, `:transition_effect_assign_two_semicolons`, `:transition_effect_no_semicolon`, `:body_assignment_no_semicolon` | ✅ Faithful (a second `;` is an error, as `ActionBody` takes one terminator; a statement outside a transition effect still needs its own `;`) |
| Bodied `exhibit state` (`exhibit state spacecraftModes { … }`, SysML.xtext `ExhibitStateUsage`) | `parser/behavior.go` (the exhibited state's body is parsed as a state body); `ast/dump.go` (the `exhibit state` keyword is recorded) | `parse/behavior_exhibit_state_body.golden`, `negative_test.go:exhibit_state_unclosed_body` | ⚠️ Approximate (the body parses, resolves and lowers as a state machine; executing a part's exhibited state machine from the part itself is not driven by the runtime — `%state` on the state usage runs it) |

### Expression Evaluation

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Binary operators (+, -, *, /, %, **, <, >, ==, ===) | `eval.go` evalOperator → evalArithmetic/evalComparison/evalEquality/evalIdentity | `calc_simple_add.sysml`, `calc_modulo_operator.sysml`, `calc_identity_operators.sysml` | ✅ Faithful |
| Boolean operators (and, or, xor, implies), short-circuiting where they can | `eval.go` evalLogical | `constraint_literal.sysml`, `calc_boolean_operators.sysml` | ✅ Faithful |
| Unary operators (-, +, not) | `eval.go` evalUnary | `calc_unary_operators.sysml` | ✅ Faithful |
| Conditional (`if c ? a else b`) and null coalescing (`??`), both lazy | `eval.go` evalConditional/evalNullCoalesce | `calc_conditional_branch.sysml`, `calc_null_coalesce.sysml` | ✅ Faithful |
| Literal values (Integer, Real, Boolean, String) | `eval.go:109` evalLiteral* | `calc_simple_add.sysml` | ✅ Faithful |
| Feature reference resolution | `eval.go:141` evalFeatureReference | `constraint_literal.sysml` | ✅ Faithful |
| Qualified name resolution (A::B::C) | `eval.go:53` Eval + `resolve/qualified.go` | `calc_qualified_names.sysml` | ✅ Faithful |
| Type coercion (Integer→Real) | `eval.go:344` toReal | `calc_type_coercion.sysml` | ✅ Faithful |
| Exponentiation (`**`, `^`) — Integer operands with a non-negative exponent give an Integer (`IntegerFunctions::'**'`), any other numeric pair a Real (`RealFunctions::'**'`) | `semantics/eval.go` `Pow`, shared by the folder's `evalArithmetic` and `runtime/eval.go` `evalArithmetic` | `calc_library_functions.sysml`, `exponentiation_test.go` | ✅ Faithful |
| An unqualified name resolves as a written reference does — the enclosing scope chain, inherited members, imports, then the global index — and the declaration it finds is evaluated in *its own* declaring scope, so the imports in force where a value was written answer the names that value uses | `runtime/eval.go` `evalFeatureReference` (scope arm) via `resolve/unqualified.go` `Resolver.LookupName`, `EvalContext.evalIn` | `action_body_package_member.sysml`, `action_body_declarer_scope.sysml`, `body_scope_test.go:TestBodyScopeImportSpellings`, `robustness_test.go:action_body_unresolved_feature` | ✅ Faithful |

#### Scope of an expression in a behavior body

An expression written inside an action or state machine body resolves its names
in the scope it was **declared** in, and the values live above that scope: a
frame binding (a token's data, a block-local declaration, a call trigger's
argument) shadows a same-named declaration the scope reaches, and the innermost
frame wins. The scope travels with the IR — `internal/core/lower` records it on
the graph, on each lowered statement and block, on each state and on each
transition when it lowers them — so the executors read a scope rather than
re-deriving one from `symbol.Decl` (AGENTS.md §4).

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| An attribute default in a behavior body is evaluated in that body's scope, so a unit an import brought in resolves (`attribute h : LengthValue = 500.0 [m];`) | `lower/action_graph.go` `lowerAttributes` + `ActionGraph.Scope`; `runtime/action_executor.go` `initializeAttributes`; `lower/state_graph.go` + `runtime/state_executor.go` `initializeAttributes` | `action_body_quantity_descent.sysml`, `state_body_quantity_scope.sysml`, `parser/testdata/parse/action_body_quantity_statements.golden` | ✅ Faithful |
| A statement in a nested action node resolves in *that* node's scope; a statement in a loop body or an `if` branch in the block's own scope | `lower/action_graph.go` `lowerStatement`, `lowerBlock`, `childScope` (`lower/scope.go`); `runtime/action_statements.go` `evalIn` | `action_body_quantity_descent.sysml`, `action_body_shadows_enclosing_scope.sysml` | ✅ Faithful |
| A decision guard and an inline expression resolve in the action's own scope, the token's data shadowing it | `runtime/action_executor.go` `stepDecisionNode`, `stepActionExecutionNode` | `action_body_shadows_enclosing_scope.sysml`, `action_control_flow.sysml` | ✅ Faithful |
| A transition's guard, effect, time-event duration and change-event condition resolve in the scope the transition was written in; a call trigger's parameters and an accept trigger's payload are visible to its guard and effect and nowhere else | `lower/state_graph.go` `Transition.Scope`/`BodyScope` (via `symbols.TriggerScope`); `symbols/bodyscopes.go` `newTriggerScope`, `payloadParameterDefiner`; `resolve/document.go`; `runtime/state_executor.go` `passesGuard`, `scheduleTransitionsForState`, `pollChangeEvents`, `executeAction`; `runtime/state_region_transition.go` `runEffect` | `state_body_quantity_scope.sysml`, `state_call_trigger_guard.sysml`, `state_call_trigger_regions.sysml`, `state_transition_accept_payload.sysml`, `model/behavior_body_resolve_test.go` accept-payload case | ✅ Faithful |
| A state's entry, do and exit behaviors resolve in that state's scope, nested states and orthogonal regions included | `lower/state_graph.go` `StateGraph.StateScopes`, `collectStates`, `collectRegionStates`; `runtime/state_executor.go` `stateScope` | `state_body_quantity_scope.sysml`, `state_concurrent_do.sysml`, `state_region_cross_pseudostate.sysml` | ✅ Faithful |
| A body member of an inherited or performed behavior is evaluated in the *declarer's* scope, not in the scope performing it | `runtime/invoke_action.go` `invokeAction`, `runtime/context.go` `chainMembers` + `EvalContext.evalIn` | `action_body_declarer_scope.sysml` | ✅ Faithful |
| A frame binding shadows the enclosing scope, and an inner block shadows an outer one | `runtime/action_statements.go` `evalIn` (frames pushed over the scope), `runtime/eval.go` `evalFeatureReference` (frames consulted first) | `action_body_shadows_enclosing_scope.sysml`, `robustness_test.go:loop_body_declaration_does_not_leak` | ✅ Faithful |
| A name or unit the declaring scope does not reach is reported, not evaluated as a bare magnitude | `runtime/eval.go` (`ErrUnresolvedFeature`), `semantics/units.go` (`ErrNotAUnit`) | `robustness_test.go:action_body_unresolved_unit`, `:action_body_unresolved_feature`, `:state_body_unresolved_unit` | ✅ Faithful |
| A `%constraint`/`%requirement` verdict is evaluated in the element's declaring scope, with or without an instance | `repl/meta.go` `declaringScope` | `repl/runtime_commands_test.go:TestConstraintResolvesUnitsOfItsOwnPackage` | ✅ Faithful |
| An expression typed at the prompt is evaluated in the namespace the session is working in — the namespace a member typed there would be written in — so that namespace's members and the units its imports bring in resolve unqualified (`1.0 [m/s]`, `mass * 2`) | `repl/meta.go` `promptScope`, `doEval`; `repl/lookup.go` `lookupSymbol` (a name the session declares nowhere is resolved there) | `repl/runtime_commands_test.go:TestEvalResolvesImportedUnitsUnqualified`, `TestPromptScopeIsTheLastNamespaceDeclared` | ⚠️ Approximate (the notation says nothing about a prompt, so "the namespace the session works in" is the *last* one it declared: declaring a second package moves it, and the first package's members and imports are then reached by qualified name only) |
| **Arguments of a `%calc` command** are a list of expressions parsed by the expression parser, so an argument containing spaces — a quantity, a parenthesized expression, a nested call — is one argument; successive arguments are separated by a comma or by whitespace, and the invocation form `Fall(a, b)` is accepted | `repl/meta.go` `doCalc`, `parseExprList`, `splitCalcArgs`; `parser/parser.go` `Parser.Offset` | `repl/runtime_commands_test.go:TestCalcParsesExpressionArguments`, `TestCalcSeparatesSignedArguments` | ⚠️ Approximate (whitespace separates two arguments only where the first is a complete expression and the second is one — `5 -3` is two arguments, `5 - 3` one — since whitespace is no terminator in the notation; named arguments, `Fall(v0 = …)`, are reported as unsupported rather than bound: the notation writes them in an invocation's parentheses, a production the prompt's argument list is not) |
| A quantity's magnitude is rendered in a result table by the same convention as a bare Real, the stored value keeping its full precision | `repl/meta.go` `formatValue`, `formatConst`; `runtime/quantity.go` `Quantity.TextWithMagnitude` | `repl/runtime_commands_test.go:TestFormatValueQuantityUsesRealFormatting` | ✅ Faithful |

### KerML Function Library (KerML §9.3 Function Library)

The library declares these functions abstractly — a signature and no body — so
the runtime supplies the implementation. Dispatch is by the declaration's
qualified name, and a declaration that carries a body is evaluated from that
body, so a model's own `calc sqrt` is never hijacked. An unqualified call that
resolves to no declaration dispatches by local name, which is what makes
`sysml -e "sqrt(2.0)"` evaluable in a model that imports no part of the library.

Arguments follow the vendored signatures: a `Real` parameter accepts an Integer
(`ScalarValues` declares `Integer :> Rational :> Real`), an `Integer` parameter
rejects a Real rather than truncating it, and a `Natural` parameter rejects a
negative value. A result that is not a finite value of the declared type — the
square root of a negative, an inverse sine outside `[-1.0, 1.0]`, a `floor`
beyond the Integer range — is reported at evaluation rather than returned as a
NaN, an infinity or a wrapped integer.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `RealFunctions`: `sqrt`, `abs`, `floor`, `round`, `max`, `min` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| `RationalFunctions`/`NumericalFunctions`: `abs`, `max`, `min` (kind-preserving), `isZero`, `isUnit` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| `IntegerFunctions`: `abs`, `max`, `min`; `NaturalFunctions`: `max`, `min` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| `TrigFunctions`: `sin`, `cos`, `tan`, `cot`, `arcsin`, `arccos`, `arctan` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| Domain, arity and argument-type failures reported at evaluation | `runtime/library_functions.go` `bindAndApply` | `TestLibraryFunctionErrors` | ✅ Faithful |
| A declaration with a body is evaluated from that body | `runtime/library_functions.go` `libraryFunctionFor` | `TestLibraryFunctionDoesNotHijackADeclaredBody` | ✅ Faithful |
| Named argument binds to the parameter the signature declares (`sin(theta = 0.0)`) | `runtime/library_functions.go` `bindAndApply` | `TestLibraryFunctionNamedArguments` | ✅ Faithful |

Found, not fixed — numeric library features that remain unevaluable:

| Not implemented | Why |
|---|---|
| `VectorFunctions`, `MatrixFunctions` | Needs a vector value in the evaluator; every value is scalar today. |
| `ComplexFunctions` | Needs a complex value kind. |
| Library functions in the checker's own name resolution | An unqualified call to a library function the model does not import evaluates, but the `unresolved-reference` diagnostic still reports the name; importing `RealFunctions::*` clears it. |

### Sequence Indexing and Collection Operations (KerML §9.3 `SequenceFunctions`, `CollectionFunctions`, `ControlFunctions`)

A KerML sequence is not a value of its own kind: every value is a sequence — of
one element where it is a scalar, of none where it is null — which is how the
library's own `isEmpty` is `seq == null` and how `1->size()` is 1. The runtime
takes that view of a value in `runtime/collections.go` `elementsOf`, so the
operations agree with the library's definitions for a scalar and for null as
well as for a sequence or a set.

The index is **1-based**, verified against the vendored declaration rather than
assumed: `SequenceFunctions::'#'` declares `in index: Positive[1]` and
`SequenceFunctions::head` is defined as `seq#(1)`, `last` as `seq#(size(seq))`
and `subsequence` as `(startIndex..endIndex)->collect {in i; seq#(i)}`. An index
of 0 is therefore not a position, and is reported rather than read as the first
element.

The library declares each operation with a body — `size` recursively as `if
isEmpty(seq)? 0 else size(tail(seq)) + 1` — but that body is the specification
of the operation, not the way to compute it, so a name denoting the library
declaration dispatches to the implementation while a model's own declaration of
that name is still evaluated from its own body.

The three notations a model can write an operation in — the collect/select
notation (`xs.{in x; …}`, `xs.?{in x; …}`), the receiver form
(`xs->collect {…}`, `xs->size()`) and the plain call (`size(xs)`,
`SequenceFunctions::size(xs)`) — all reach one implementation per operation, so
they cannot drift apart.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `seq#(i)` is the i-th element counting from 1 (`SequenceFunctions::'#'`, `in index: Positive[1]`), and an index of 0, a negative index or one past the end is a typed error rather than an empty or zero value | `runtime/collections.go` `evalSequenceIndex`, `elementAt`, `indexOf`; `runtime/quantity.go` `evalIndexExpr` (non-bracket arm) | `runtime/collections_test.go` `TestSequenceIndexing`, `TestSequenceIndexingErrors`; conformance `calc_sequence_index`, `calc_sequence_index_out_of_range`, `calc_sequence_index_zero`, `calc_sequence_index_non_integer`; `robustness_test.go:sequence_index_names_no_position` | ✅ Faithful |
| The index and the quantity expression share one AST node and are told apart by the notation that produced them (`ast.IndexExpr.Bracket`), so `5 [m]` is a quantity and `xs#(1)` is an element | `parser/expr.go` (`Hash` and `LBracket` arms), `runtime/quantity.go` `evalIndexExpr` | `parser/testdata/parse/quantity_expression.golden`, `parse/collection_operations.golden`; `runtime/collections_test.go` `TestSequenceIndexKeepsQuantityForm`; conformance `calc_sequence_index_and_quantity_form`; `parser/negative_test.go` (`index_no_paren`, `index_bracket_empty`) | ✅ Faithful |
| An index that is not one whole number (a Real, a Boolean, a string, a collection) is a typed error, statically where it is written as a literal and at evaluation otherwise | `passes/typecheck_expr.go` `inferIndex` (the index conforms to `Integer`, a literal 0 and a literal past a written sequence's length; a whole number a model counts with is a position or not depending on its value, so an `Integer`-typed index is checked at evaluation rather than reported for not being declared `Natural`); `runtime/collections.go` `indexOf` | `passes/typecheck_index_test.go` `TestIndexNonIntegerIndexReported`, `TestIndexZeroReported`, `TestIndexPastWrittenSequenceReported`, `TestIndexTypedIntegerNotReported`, `TestIndexByLoopVariableNotReported`; `runtime/collections_test.go` `TestSequenceIndexingErrors` | ✅ Faithful |
| `collect` answers the mapper's result for each element, in order, with the parameter the body itself declares bound to the element and the scope the body was written in still visible | `runtime/collections.go` `builtinControlCollect`, `applyBody`; `runtime/eval.go` `evalCollectExpr`, `evalCollectionNotation` | `runtime/collections_test.go` `TestCollectionResults`; conformance `calc_collect_over_sequence`, `calc_collect_names_outer_variable`, `calc_nested_collection_operations` | ✅ Faithful |
| `select`/`reject`/`selectOne`/`forAll`/`exists` require the `Boolean[1]` result their `expr` parameter declares, and a selector answering anything else is a typed error rather than a dropped element | `runtime/collections.go` `filter`, `quantify`, `applyPredicate` | `runtime/collections_test.go` `TestCollectionResults`, `TestCollectionOperationErrors`; conformance `calc_select_over_sequence`, `calc_select_predicate_not_boolean`; `robustness_test.go:select_predicate_is_not_a_condition` | ✅ Faithful |
| A body called with a number of arguments it declares no parameters for is a typed error (`ErrBodyArity`), never a call with a parameter left unbound | `runtime/collections.go` `bodyOf` | `runtime/collections_test.go` `TestCollectionOperationErrors`; conformance `calc_collection_body_wrong_arity`; `robustness_test.go:collection_body_of_the_wrong_arity`; `parser/negative_test.go` (`body_param_no_name`) | ✅ Faithful |
| `SequenceFunctions`: `#`, `size`, `isEmpty`, `notEmpty`, `includes`, `includesOnly`, `excludes`, `equals`, `same`, `union`, `intersection`, `including`, `excluding`, `subsequence`, `excludingAt`, `head`, `tail`, `last` — each computing what the vendored body specifies, `equals` by value and `same` by identity | `runtime/collections.go`, registered in `runtime/builtins.go` | `runtime/collections_test.go` `TestCollectionResults`, `TestCollectionScalarResults`; conformance `calc_collection_aggregators` | ✅ Faithful, except the out-of-range endpoints of `subsequence`/`excludingAt` (see below) |
| An endpoint of `subsequence` or `excludingAt` that is outside the sequence is a typed error (`ErrIndexOutOfRange`), while an *empty range* inside it is the empty sequence, which is how `tail` is `subsequence(seq, 2)` of a one-element sequence | `runtime/collections.go` `builtinSequenceSubsequence`, `builtinSequenceExcludingAt` | `runtime/collections_test.go` `TestCollectionOperationErrors`, `TestCollectionResults` | ⚠️ Approximate: the vendored bodies compose `#`, which returns `Anything[0..1]`, so they answer the sequence unchanged for `(1,2,3)->excludingAt(4)` and truncate `subsequence(1, 4)`. A position the sequence does not have is reported here rather than answered as if the model had asked for one it has |
| `CollectionFunctions`: `size`, `isEmpty`, `notEmpty`, `contains`, `containsAll`, `head`, `tail`, `last`, `#` over a collection's elements, a set included | `runtime/collections.go`, `runtime/builtins.go` | `runtime/collections_test.go` `TestCollectionScalarResults`, `TestCollectionOperationsOverSets` | ✅ Faithful |
| An operation over an empty collection answers the empty collection and never calls its body, since there is no element to call it with | `runtime/collections.go` `elementsOf` (an empty collection yields no elements) | conformance `calc_collection_ops_over_empty`; `runtime/collections_test.go` `TestCollectionResults` | ✅ Faithful |
| `ControlFunctions`: `collect`, `select`, `selectOne`, `reject`, `reduce`, `forAll`, `exists`, `allTrue`, `anyTrue`, `minimize`, `maximize` | `runtime/collections.go`, `runtime/builtins.go` | `runtime/collections_test.go` `TestCollectionResults`, `TestCollectionScalarResults`, `TestCollectionOperationErrors` | ✅ Faithful |
| `NumericalFunctions::sum`/`product` and the specializations that fix the identity of an empty aggregation (`sum0(collection, 0)`, `product1(collection, 1)`), keeping the elements' kind: Integers sum to an Integer, a Real anywhere makes the result a Real, and an overflowing sum or product is reported rather than wrapped | `runtime/collections.go` `aggregate`, `foldNumeric` | `runtime/collections_test.go` `TestCollectionScalarResults`; conformance `calc_empty_collection_aggregation` | ✅ Faithful |
| A collection of quantities aggregates to a quantity, in the unit of its first element and converting the rest, as the binary operator does; mixing a bare number with a measured value reports incommensurable units | `runtime/collections.go` `aggregate`/`aggregateQuantities`, `quantity.go` `addQuantities`/`scaleQuantities` | conformance `cubesat_mass_rollup`; `runtime/collections_test.go:TestAggregateQuantities` | ⚠️ Approximate (an empty collection has no unit to answer in, so it aggregates to the numeric identity `0`/`1`) |
| The unqualified, qualified and receiver (`->`) forms of an operation are one implementation, so `(1,2,3)->size()`, `size((1,2,3))` and `SequenceFunctions::size((1,2,3))` cannot disagree; a name the model itself declares still resolves to that declaration | `runtime/builtins.go` `builtinsByLocalName`, `Context.builtinFor`; `runtime/eval.go` `evalInvocation` (receiver prepended as the first argument) | `runtime/collections_test.go` `TestCollectionScalarResults`; conformance `calc_collection_receiver_form` | ✅ Faithful |
| A sequence is flat: an element of a sequence expression that is itself a collection contributes its elements, so `(xs, ys)` is `xs->union(ys)` — which is how the library defines `union`, as the sequence expression `(seq1, seq2)` — and a mapper answering several values contributes them all | `runtime/eval.go` `evalSequenceExpr`; `runtime/collections.go` `builtinControlCollect`; `passes/typecheck_expr.go` `writtenLength` (a written length is knowable only where every element is a literal, so a literal index past a sequence of names is left to evaluation) | `runtime/collections_test.go` `TestSequenceExpressionsAreFlat`; conformance `calc_sequence_expression_is_flat` | ✅ Faithful |
| A receiver binds by position, so a call written with both a receiver and named arguments (`x->f(a = 1)`) states no parameter for the receiver and is a typed error (`ErrReceiverWithNamedArgs`) rather than a call the receiver is dropped from | `runtime/eval.go` `evalInvocation`; `passes/typecheck_expr.go` `checkArguments` | `runtime/collections_test.go` `TestCollectionOperationErrors`, `TestReceiverWithNamedArgumentsIsReported`; `passes/typecheck_expr_test.go` `TestExprInvocationReceiverWithNamedArguments` | ✅ Faithful |
| A collection operation is an expression wherever an expression is allowed, including inside a calc body's `while` and `for` loops | `runtime/collections.go`, `runtime/action_statements.go` | conformance `calc_collection_ops_in_for_loop`, `calc_collection_ops_in_while_loop` | ✅ Faithful |
| Every operation is bounded by the evaluation step budget, since each call of its body spends steps | `runtime/eval.go` `Context.step` | `robustness_test.go:collection_operation_step_budget` | ✅ Faithful |
| A materialized element is bounded on its own, since it is memory the collection keeps rather than work a step does: every path that adds one to a sequence — a range, a sequence literal, `->collect`, union/intersection/including/excluding/subsequence/tail/`select` — charges the element budget (`SYSML_MAX_ELEMENTS`, default 1000000, ~104MB of `Value`s) and reports `ErrElementLimitExceeded`, not the step limit. The count is what an evaluation holds, not what a run produced: a statement, and an evaluation outside a body alike (`beginStep`), releases the elements it materialized, so a loop or a long run building a small collection each step is bounded by its peak rather than its total, while a collection kept across statements had to be materialized in one of them and so was charged in full | `runtime/context.go` `chargeElements`/`elementScope`/`beginStep`, `runtime/statements.go` `statement`, `runtime/collections.go` `newSequence`, `runtime/range.go` `rangeSequence`, `runtime/eval.go` `evalSequenceExpr`; budget from `budget.go` | `element_budget_test.go:TestElementBudgetBoundsEveryMaterialization`, `:TestElementBudgetIsNotTheStepBudget`, `:TestElementBudgetCountsElementsHeldNotProduced`, `:TestElementBudgetIsReleasedByEveryStep`, `:TestElementBudgetIsPerRun`, `robustness_test.go:collection_spends_the_element_budget`, `grpc/budget_test.go:TestNewServiceResolvesBudgets` | ✅ Faithful |
| An activation ends with the body execution it belongs to, so what the calc usages read in a body computed is discarded when that execution ends rather than held for the whole run | `runtime/statements.go` `finish`/`enterActivation`, `runtime/action_statements.go` `executeBody`, `runtime/invoke_calc.go` `runCalcBody` | `action_activation_test.go:TestActionBodyActivationEndsWithTheBody`, `calc_usage_body_local_test.go` | ✅ Faithful |
| A calc usage declared among a state machine's members binds its inputs from the values the machine has reached, as one in a calc's or an action's body does, so a guard reading it is answered over the running attribute rather than over what it was declared with | `runtime/calc_usage.go` `enclosedByBehaviorBody`, `runtime/invoke_calc.go` `isStateSymbol` | conformance `state_guard_reads_calc_usage` | ✅ Faithful |
| An evaluation outside a body — a decision guard, an inline node expression, a transition guard, change condition or duration, an attribute default, a slot default, an action argument, a constraint or requirement check — is a scope of its own, so what a calc usage answers it, and the elements a collection it evaluates materializes, live no longer than that step: the next guard reads the usage again over the values the step before it assigned. A read through a part's feature chain belongs to the evaluation making it and shares its activation | `runtime/eval.go` `beginStep`, `runtime/state_executor.go` `evalStep`, `runtime/action_executor.go` `stepDecisionNode`/`stepActionExecutionNode`/`initializeAttributes`, `runtime/condition.go` `evaluateConditions`, `runtime/calc_usage.go` `calcUsageMemberValue` | conformance `action_guard_reads_calc_usage`; `calc_usage_step_test.go:TestDecisionGuardReadsCalcUsagePerStep`, `:TestDecisionGuardsShareOneCalcUsageEvaluation`, `:TestPartChainReadBelongsToTheReadingActivation` | ✅ Faithful |
| A failing expression of literals alone is answered at the prompt with the failure itself, so `sysml -e "(1,2,3)#(0)"` reports the index rather than "no declarations loaded" | `repl/meta.go` `tryEvalLiteral`, `isLiteralAnswerError` | `repl/runtime_commands_test.go` `TestEvalReportsTheAnswerOfALiteralExpressionThatFails` | ✅ Faithful |
| A name the session declares is answered by that declaration, so the prompt's literal pass declines an expression using one rather than letting a library operation of the same unqualified name stand in for it | `repl/meta.go` `tryEvalLiteral`, `declaresANameIn` | `repl/runtime_commands_test.go` `TestEvalPrefersASessionDeclarationOverALibraryOperation` | ✅ Faithful |

⚠️ A body parameter takes its type from the element type of whatever the operand
turns out to hold, which the expression checker does not track: an expression
over the parameter (`xs.?{in e; e + 1}`) therefore has no static type, and its
selector is checked at evaluation rather than where it is written. Statically the
checker reports what it can know — a selector whose result type *is* known and is
not Boolean, an index that is no whole number, a literal index of 0 or past a sequence
written out (`passes/typecheck_expr.go` `inferIndex`, `inferSelect`) — and the
runtime checks the rest, so no wrong answer results from what is left unchecked.

Found, not implemented — declared collection operations this runtime does not
evaluate. Each is a typed `unresolved reference` or `unsupported` error, never a
wrong answer:

| Not implemented | Why |
|---|---|
| `SequenceFunctions::includingAt` | The vendored body is `(seq->subsequence(1, index - 1), values, seq->subsequence(index + 1))`, which drops the element at `index` rather than inserting before it. Implementing it literally would silently lose an element; implementing the evident intent would invent semantics the vendored library does not state. Left out until the OMG text is adjudicated (compare `excludingAt`, whose body is consistent and which is implemented). |
| `CollectionFunctions::'array#'` and the `Array`/`Matrix` collections | Needs a multi-dimensional array value; the runtime's collection values are a sequence and a set. |
| `ComplexFunctions::sum`/`product`, `VectorFunctions`/`MatrixFunctions` aggregation | Needs a complex and a vector value kind (see the numeric library table above). |
| A reducer named rather than written (`->reduce min`, as the library's own `minimize` is defined) | A function-valued *name* is not a runtime value: `reduce` takes the body expression form (`->reduce {in a; in b; …}`), and `minimize`/`maximize` are implemented directly rather than through `reduce min`. A named reducer is reported as a type error, not read as a body. |
| `SequenceFunctions::add`/`addAt`/`remove`/`removeAt`, `CollectionFunctions` mutators | These are `behavior`s, not functions: they declare an `inout` sequence, so they need mutable accumulation the language layer does not have. Deliberately out of scope. |
| Set coverage in the conformance corpus | No expression of the language produces a set today — a `ValSet` arises only through the embedding API — so the operations over a set are pinned at the unit level (`TestCollectionOperationsOverSets`, `elementsOf`) rather than by a `.sysml` fixture. A set-valued expression is separate work. |
| `at`, `first`, `reverse` | Not declared by the Kernel Function Library at all (`head`, `#(1)` and `last` are the declared spellings). Not implemented rather than invented. |

### Systemica Extension Library (non-normative)

The OMG Kernel Function Library declares no exponential, no logarithm and no
two-argument arctangent: `RealFunctions` has `sqrt`/`floor`/`round`/`abs`/`max`/`min`/`'**'`/`'^'`,
`TrigFunctions` has `sin`/`cos`/`tan`/`cot`/`arcsin`/`arccos`/`arctan`, and that
is all. The vendored OMG files stay byte-identical, so the missing signatures are
declared in a clearly non-normative Systemica extension instead:
`internal/core/libs/stdlib/Systemica Libraries/SystemicaMathFunctions.kerml`. It
is bundled by the same `embed.FS` as the vendored tree and enters the same
gates — `TestStdlibConformance` now reports 95/95 clean. It is Systemica code under
Apache 2.0, not OMG code under EPL-2.0; `internal/core/libs/stdlib/NOTICE` carves
the subdirectory out of the OMG notice.

**Reachability.** A model writes `import SystemicaMathFunctions::*;` (or calls
`SystemicaMathFunctions::exp(x)` qualified); both resolve like any other library
package, with no diagnostic. A *bare* `exp(x)` with no import **evaluates**, by
the same unqualified-name dispatch a bare `sqrt(x)` uses, but the checker still
reports `unresolved reference: exp` on the name — exactly the rough edge the
vendored functions have, no better and no worse. ROADMAP A6 is the general fix.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `exp(x)` — e raised to the power x | `runtime/library_functions.go` (`math.Exp`) | `TestLibraryFunctionValues` | ✅ Faithful |
| `ln(x)` — natural logarithm, defined for `x > 0.0` | `runtime/library_functions.go` `naturalLog` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors` | ✅ Faithful |
| `log(x, base)` — logarithm to an explicit base, so base 10 and base e are never confused; base 10 and base 2 use `math.Log10`/`math.Log2`, which are exact where the ratio of logarithms is not | `runtime/library_functions.go` `logToBase` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors` | ✅ Faithful |
| `atan2(y, x)` — full-quadrant angle, parameters ordered as in IEEE 754 and `math.Atan2` | `runtime/library_functions.go` `atan2Real` | `TestLibraryFunctionValues`, `TestLibraryFunctionAtan2NamedArguments` | ✅ Faithful |
| `ln(0.0)`, `ln(-1.0)`, `log(x, 1.0)`, `log(-1.0, 10.0)`, `atan2(0.0, 0.0)` report a domain error; `exp` beyond the Real range reports an overflow | `runtime/library_functions.go` | `TestLibraryFunctionErrors`, `TestRuntimeRobustness/extension_library_function_outside_its_domain` | ✅ Faithful |
| The shipped declarations and the registered implementations cannot drift (names, parameter names, parameter order) | `runtime/library_functions.go` registry | `TestSystemicaMathFunctionsMatchTheShippedDeclarations` | ✅ Faithful |
| Evaluable from a `calc def` body | `runtime/invoke_calc.go` | `calc_systemica_math_functions.sysml` + golden trace | ✅ Faithful |

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
| Boolean-valued contexts (constraint/assume/require, `if`/`while`/`until`, guards) | `passes/typecheck.go` `checkBehaviorMember` (recursing into loop and branch bodies, so a nested condition is checked too) | `TestExprTransitionGuardMustBeBoolean`, `TestTypeCheckNonBooleanControlFlowConditions` | ⚠️ Approximate (a condition whose type the expression checker can infer is checked here, before execution; a bare feature reference infers Unknown — see `What We Don't (Yet) Support` — and is caught by the executor instead) |
| Change-event conditions (`accept when <expr>`, `transition ... when <expr>`) | `passes/typecheck.go` `checkTrigger` | `TestExprAcceptWhenConditionMustBeBoolean` | ⚠️ Approximate (`accept when` is always a condition; after `transition ... when` a bare name is a signal, so only expressions are checked there) |
| Division/exponentiation result types (`Natural/Natural -> Natural`, `Integer/Integer -> Rational`) | `passes/typecheck_expr.go` `divisionResult` | `TestExprWholeNumberDivisionAndPowerOK` | ✅ Faithful |
| Calc/action invocation arity, incl. inherited, partially redefined (`:>>`), and arrow-form receiver | `passes/typecheck_expr.go` `effectiveInParameters`/`checkArguments` | `TestExprPartiallyRedefinedParametersKeepInheritedSignature` | ✅ Faithful |
| Invocation argument types and named-argument names | `passes/typecheck_expr.go` `checkArguments` | `TestExprInvocationArgumentTypeMismatch` | ✅ Faithful |
| No false positives on the shipped library and examples | corpus guard | `model/typecheck_expr_corpus_test.go` | ✅ Faithful |
| Non-scalar conformance of bound values (specialization hierarchy, enumeration literals) | `passes/typecheck_value.go` `checkValueConformance` | `TestValueUnrelatedInstanceDoesNot`, `TestValueEnumerationLiteralOfOtherEnum` | ⚠️ Approximate (a value is typed only when it is a name or a literal; expressions producing an instance are not judged) |
| Multiplicity conformance of bound values | `passes/typecheck_value.go` `checkValueCount` | `TestValueTooManyValuesForUpperBound`, `TestValueTooFewValuesForLowerBound` | ⚠️ Approximate (only a collection literal or a literal has a statically known element count; a reference may itself be multi-valued and is left unchecked) |
| Collection element types (each element against the feature's type) | `passes/typecheck_expr.go` `checkUsageValue` | `TestValueCollectionElementTypes` | ✅ Faithful |

### View and Viewpoint Members (SysML v2 §8.3.20 Views, §8.3.26 Viewpoints; SysML.xtext `ViewRenderingMember`, `FramedConcernMember`, `StakeholderMember`, `ActorMember`)

Each of these keywords owns a **usage** through a dedicated membership, and each
usage is written either as a *reference* to an existing element or as a
*declaration* introduced by the kind keyword the notation spells out. A
reference declares no name of its own: the name it answers to is its
reference's, derived by `ast.EffectiveName` (KerML §7.3.4.5), which is why a
reference to an inherited element is not a name conflict.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `render` owns a RenderingUsage through a ViewRenderingMembership: `render asTree;` references the rendering the view uses, `render rendering r : AsTree;` declares one. No ValuePart, and no definition form. The declaration's UsageDeclaration is optional, so it may be anonymous (`render rendering : AsTree;`) | `ast/defusage.go` `UsageViewRendering`; `parser/defusage.go` `parseDefUsage` (render/frame dispatch) and `parseReferenceMemberUsage`; `symbols/builder.go` (`SymbolRenderingUsage`), `passes/typecheck.go` (typed by a `rendering def`), `semantics/implicit.go` (`Views::Rendering`), `export/kinds.go` (`ViewRenderingMembership`), `export/rdf_in.go` `usageHead` (which form to write back is decided by the reference, not the name, so an anonymous declaration keeps its kind keyword) | `parse/view_members.golden`, `export/testdata/convert/kind_keyword_synonyms.golden.sysml`, `parser/negative_test.go` (`render_definition`, `render_reference_value`), `passes/nameres_test.go` `TestRenderReferenceToInheritedRenderingIsNoConflict`, `TestRenderDeclarationOfInheritedNameConflicts`, corpus `42. Views/*` | ✅ Faithful (parse and naming; what a view *renders* is not derived — see "What We Don't (Yet) Support") |
| `frame` owns a ConcernUsage through a FramedConcernMembership: `frame 'system breakdown';` references the concern framed, `frame concern c : SafetyConcern;` declares one, possibly anonymously (`frame concern : SafetyConcern;`). Its body is a requirement body | `ast/defusage.go` `UsageFramedConcern`; `parser/defusage.go` (same dispatch, body `parseRequirementBody`); `symbols/builder.go` (`SymbolConcernUsage`), `passes/typecheck.go` (typed by a `concern def`), `semantics/implicit.go` (`Requirements::ConcernCheck`), `resolve/document.go` `parameterizedByName`, `export/kinds.go` (`FramedConcernMembership`) | `parse/view_members.golden`, `parser/negative_test.go` (`frame_definition`), corpus `42. Views/Viewpoint Example.sysml` | ⚠️ Approximate (`frame concern SafetyConcern;` follows the grammar's `ConstraintUsageDeclaration` and *declares* a concern usage named `SafetyConcern`; the reference to a concern of that name is `frame SafetyConcern;`. Framing is parsed, not checked against the viewpoint's concerns) |
| `stakeholder` and `actor` own a PartUsage through a StakeholderMembership / ActorMembership; both are declarations (`stakeholder se : Engineer;`, `actor driver : Person;`) and neither has a definition form | `ast/defusage.go` `UsageStakeholder`, `UsageActor` (the former `ast.ActorMember` node is gone); `parser/defusage.go` `parseDefUsage`; `symbols/builder.go` (`SymbolPartUsage`), `passes/typecheck.go` (typed by a `part def`), `semantics/implicit.go` (`Parts::Part`), `runtime/context.go` `memberBindings` (actor binding, name via `ast.EffectiveName`), `export/kinds.go` | `parse/view_members.golden`, `parse/requirement_members.golden`, `parser/behavior_test.go` `TestParseRequirementBody_Actor`, `parser/negative_test.go` (`stakeholder_definition`, `actor_definition`, `stakeholder_no_declaration`, `actor_no_declaration`), `runtime` `requirement_actor.sysml`, corpus `41. Use Cases/*`, `42. Views/*` | ✅ Faithful (a stakeholder or actor *definition* is rejected: the notation has only the usage, typed by the party's definition) |
| `satisfy` names the requirement satisfied (`satisfy vehicleSpecification by vehicle;`) or declares the satisfaction (`satisfy requirement r : Req1 by v;`); a view body's `satisfy viewpoint;` is the same form | `parser/defusage.go` `parseUsage` UsageSatisfy branch (`RequirementUsageKeyword UsageDeclaration?`, then `ValuePart?`, then `by`) | `parse/satisfy_reference.golden` (incl. the anonymous forms `satisfy requirement by vehicle;` and `satisfy requirement : VehicleSpecification by vehicle;`), `parse/view_members.golden`, `parser/satisfy_subject_test.go` | ⚠️ Approximate (the reference is recorded as a Subsetting rather than a ReferenceSubsetting, so `passes/typecheck.go` can require the target to be a requirement usage; a satisfy reference therefore takes no effective name — nothing reads one — and viewpoint conformance is not evaluated) |
| `frame` and `render` are also legal names (KerML has neither keyword; the Kernel Semantic Library writes `in frame : SpatialFrame[1]`). Only a name or the member's own kind keyword can follow the member keyword, so anything else — a multiplicity, a specialization, a type, a value, a body, `;` — declares a feature named after the keyword | `parser/defusage.go` `atMemberKeywordUsedAsKeyword` | `parse/view_members.golden` (`frame[0..1] : Engineer;`, `render :> frame;`), `libs/reserved_keyword_name_test.go`, `TestStdlibConformance` | ⚠️ Approximate (the parser does not track the enclosing body kind, so the reading is decided by the following token alone: `frame;` inside a viewpoint declares a feature named `frame` instead of being diagnosed as a framing with no concern) |
| `expose` in a view body is an Import | see the Name Resolution section's `expose` rows | `parser/expose_test.go`, `resolve/expose_test.go`, `parse/view_expose.golden` | ✅ Faithful (`validateExposeOwningNamespace` reports an `expose` outside a view usage — see the Name Resolution section's `expose` rows) |

### Structural, Interface and Analysis Notation (SysML v2 §7.12 Ports, §8.2.2.14 Interfaces, §8.2.2.19 Analysis Cases, §8.3.9.11 Occurrences)

Notation exercised by the Open-MBEE corpus models (`starkit`, `Dragon`,
`DesertKite/OOSEM`, the spacecraft example notebooks). Conjugation is a semantic
relationship, not parser sugar: the `~` is kept on the typing relationship
(`ast.Relationship.Conjugated`) and the reversal of `in`/`out` is computed in the
semantics layer over the conjugation parity of the typing/specialization chain.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `port p : ~P` types a port by the conjugated port definition `P::'~P'` (§7.12.3); its features are P's with `in`/`out` reversed and `inout` unchanged, and conjugation composes, so a conjugate of a conjugate has P's directions | `ast/defusage.go` `Relationship.Conjugated`; `parser/defusage.go` `parseRelationshipClauseTarget`; `semantics/conjugation.go` `ConjugateDirection`, `superEdges`, `typeEdge`, `featureEdges`, `conjugatedSupertypes`, `PortFeatures`, `IsConjugated` (a declaration's feature typing carries the conjugation, so it is read ahead of a redefinition clause written before it) | `parse/conjugated_port_type.golden`, `semantics/conjugation_test.go` `TestConjugationReversesDirections`, `TestDoubleConjugationRestoresDirections`, `TestConjugationOnRedefiningPort`, `parser/negative_test.go` (`conjugated_no_type`, `conjugated_no_type_after_name`) | ✅ Faithful |
| A port usage conforms to the definition it conjugates, and two ports match when each named feature of one has a feature of the other with a conforming type and the conjugate direction (§7.12.2) | `semantics/conjugation.go` `PortsConform`, `featuresMatchConjugate`, `featureTypesConform` | `semantics/conjugation_test.go` `TestConjugatedPortConformance` | ✅ Faithful |
| The ports at the two ends of an interface must have conjugate directed features: what one end sends the other receives | `semantics/conjugation.go` `InterfaceEndPortMismatch`, `endPortFeatures`; `passes/constraint.go` `checkInterfaceEndConjugation` (code `port-conjugation`) | `passes/constraint_test.go` `TestConstraintInterfaceEndConjugation`, `semantics/conjugation_test.go` `TestInterfaceEndConjugation` | ⚠️ Approximate (reported as a warning, and only for an interface whose two ends both declare a resolvable port type; undirected features carry no flow, so they are not required to match; the ends of a `connect`/`flow` clause take their types by implicit redefinition of the interface's ends, which is checked as end *identity*, not as direction) |
| Only a port usage or a connector end may be typed by a conjugated port definition, and `~` must name a port definition | `passes/typecheck.go` `checkConjugatedTyping` | `passes/typecheck_test.go` `TestTypeCheckConjugatedTyping` | ✅ Faithful |
| An interface body may declare a default end with no declaration at all: `end;` is an anonymous port usage (§8.2.2.14.1 `DefaultInterfaceEnd: isEnd ?= 'end' Usage`) | `parser/parser.go` `bodyContext`/`pushBodyContext`; `parser/defusage.go` `parseAnonymousEnd`, `parseAnonymousEndUsage`, `anonymousUsageKind` | `parse/end_usages.golden`, `resolve/analysis_test.go` `TestResolveDragonStructures` | ✅ Faithful |
| Anywhere else a bare `end;` is not standard notation: only `DefaultInterfaceEnd` makes the usage declaration optional, and every other `end` form (`ReferenceUsage`, `EndUsagePrefix` + a kind keyword) requires an `Identification` or a specialization part | `parser/defusage.go` `parseAnonymousEndUsage` (typed diagnostic naming the fix, `end ref;`) | `parser/negative_test.go` (`end_outside_connector`, `end_outside_connector_package`) | ✅ Faithful (see the known limitation on `Dragon.sysml` below) |
| `require Q::r { … }` / `assume Q::r { … }` subset a requirement by a *qualified* reference, and the body may redefine a feature of it by its qualified (`:>> R::f = expr`) or its plain name (`:>> f = expr`), which the member inherits through the reference subsetting | `ast/behavior.go` `RequireMember.Reference`, `AssumeMember.Reference` (a full `*ast.QualifiedName`, not a final segment); `parser/behavior.go` `parseRequireMember`, `parseAssumeMember`; `resolve/document.go` `resolveConstraintReference`, `walkConstraintBody`, `lookupConstraintRefFeature` | `parse/require_qualified_requirement.golden`, `resolve/analysis_test.go` `TestResolveQualifiedRequirement`, `TestResolveRequiredRequirementFeatureByPlainName`, `TestResolveRequiredRequirementUnknownPlainName`, `TestResolveQualifiedRequirementUnresolved`, `TestResolveQualifiedRedefinitionUnresolved`, `parser/negative_test.go` (`require_qualified_malformed_body`, `require_qualified_trailing_colons`) | ⚠️ Approximate (only the braced spelling is read as a reference — see the known limitation below) |
| `snapshot` and `timeslice` are the two portion kinds of an occurrence usage (`PortionKind`, §8.3.9.11 `OccurrenceUsage::portionKind`); either prefix makes the declaration an occurrence usage whatever kind keyword follows. `PortionUsage` ends in `Usage`, whose declaration is optional, so an anonymous portion (`timeslice;`) is standard notation and is accepted without a diagnostic. Both keywords are read by the same prefix, so they agree on every declaration spelling, including a quoted name (`snapshot 'launch event';`) | `ast/defusage.go` `PortionKind`, `Usage.Portion`; `parser/defusage.go` (portion prefix), `parser/behavior.go` (portion-prefixed behavior parameters); `ast/dump.go`; `passes/typecheck.go` `declKind.portion`, `isOccurrenceUsage`; `export/rdf_out.go`/`rdf_in.go` | `parse/occurrence_portions.golden`, `parser/occurrence_modifier_test.go`, `resolve/analysis_test.go` `TestResolveDragonStructures`, `parser/negative_test.go` (`timeslice_no_subject`, `timeslice_usage_no_type`, `timeslice_unterminated`) | ✅ Faithful (parse, naming and resolution; a portion is not related to its whole occurrence at runtime — see below) |
| The standard view/diagram library is part of the vendored stdlib, so `view v : StandardViewDefinitions::gv;` resolves | `libs/stdlib/Systems Library/StandardViewDefinitions.sysml` (already vendored: the eight standard view definitions with their short names) | `model/standard_views_test.go` `TestStandardViewDefinitionsBundled`, `TestStdlibConformance` | ✅ Faithful |
| A qualified reference whose *first* segment names no loaded namespace is reported as such, naming the declarations that do carry the trailing name | `resolve/qualified.go` (unresolved-namespace diagnostic), `symbols/index.go` `FQNsEndingIn` | `model/standard_views_test.go` `TestVendorViewNamespaceDiagnostic`, `resolve/analysis_test.go` `TestResolveMissingStandardViewNamespace` | ✅ Faithful |
| Every usage element subsets the most general base usage `Base::things`, whose `that` feature is therefore visible in a usage body (§7.6, [KerML 8.4.2]) | `semantics/implicit.go` `implicitBaseUsage`, `semantics/reference.go` `contributors` | `semantics/implicit_test.go` `TestImplicitBaseUsageContributesThat`, `model/that_constraint_test.go` `TestThatResolvesInAssertedConstraint` | ✅ Faithful (a member-contribution edge only: it is deliberately not a direct supertype, so conformance and `DirectSupertypes` are unchanged) |
| A succession may be written with no keyword at the start of a namespace member: `first a::b then c;` (`SuccessionAsUsage`) | `parser/namespace.go` `parseMember`, `parseSuccessionAsUsage` | `parse/succession_as_usage.golden` | ✅ Faithful |

**Known limitations of this notation**

- `Dragon.sysml` declares bare `end;` members inside `connection def`, `flow def`
  and nested `connection def` bodies (6 sites). This is not standard notation: a
  `connection def`/`flow def` body is an ordinary `DefinitionBody`, whose members
  are `NonOccurrenceUsageElement`/`OccurrenceUsageElement` — neither includes
  `DefaultInterfaceEnd` (only `InterfaceBodyItem` does), and the only other
  keyword-less end, `DefaultReferenceUsage`, requires a `UsageDeclaration`.
  Systemica reports these with a typed diagnostic naming the conforming form
  (`end ref;`, which `ReferenceUsage` does allow) rather than inventing grammar.
- `Dragon.sysml` and `OOSEM.sysml` type their views by `'SysML Standard
  Diagrams'::gv` (7 sites). No such namespace exists in the OMG release library
  or the pilot implementation — it is a tool-specific package, not part of the
  standard library — so it is not vendored under that name and no alias to
  `StandardViewDefinitions` is fabricated. The diagnostic says the namespace is
  not loaded and points at `StandardViewDefinitions::gv`.
- Only the braced spelling of a requirement-constraint reference sets
  `RequireMember.Reference`/`AssumeMember.Reference`. `CalculationBody` also
  allows `;`, so standard `require Q::r;` is a reference too, but Systemica reads
  a body-less `require`/`assume` member as a condition expression, which the
  runtime evaluates as Boolean. Distinguishing the two spellings needs the name's
  resolution, not its syntax, and no spelling requires the *referenced*
  requirement's own conditions at runtime yet (`runtime/condition.go`
  `appendConditions` walks only the member's own body), so the body-less form is
  left on the expression path rather than made a silent no-op.
- Only the *direct* members of a `require`/`assume` body resolve: a declaration
  nested inside one of them (`require Q::r { part p : P { :>> f; } }`) has its
  own body left unwalked, so a name there is neither resolved nor reported.
  Consequently the referenced requirement's features are offered to direct
  members only (`lookupConstraintRefFeature`), which is what the reference
  subsetting inherits them to.
- Conjugation is not a runtime concept here: nothing is executed differently for
  a conjugated port, because ports carry no transfer semantics in the runtime
  yet (see "Major UML/SysML Features Not Implemented").
- A `snapshot`/`timeslice` portion is recorded on the usage and resolves like any
  occurrence usage, but the runtime does not relate a portion to the occurrence
  it is a portion of, and no time ordering between portions is derived.

### Name Resolution

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Inherited feature resolution | `document.go:199` resolveRedefinition | `flow_payload_test.go` | ✅ Faithful |
| Declaration named with a keyword (`action flow { ... }`, `attribute item : Integer`) | `parser/defusage.go` `atKindPrefix`, `atSecondaryKind` | `parser/namespace_keywords_test.go` `TestParseKeywordAsNameAfterKindKeyword`, `model/behavior_body_resolve_test.go` `TestKeywordNamedDeclarationIsReferenceable` | ✅ Faithful (the name is kept and is referenceable) |
| Keywords reserved in name position (only an unrestricted name may spell one) | `parser/namespace.go` `parseIdentification` → `Parser.Warnings`, surfaced as `passes.SeverityWarning` code `reserved-keyword-name` by `model/workspace.go` | `parser/namespace_keywords_test.go` `TestParseKeywordAsNameIsReported`, `model/behavior_body_resolve_test.go` `TestReservedKeywordNameWarning`, `libs/reserved_keyword_name_test.go` `TestStdlibReservedKeywordNames` | ⚠️ Approximate (reported as a warning, not an error, because the normative OMG library itself uses unquoted keyword names — `step entry[1];`, `part done : Part;`, `attribute type : String[0..1];` — and must keep parsing clean; those eleven sites — including the Kernel Semantic Library's `in frame : SpatialFrame[1]`, whose name SysML reserves for a view/viewpoint member — are pinned by the libs test so the set cannot grow silently) |
| Keyword qualifying a kind keyword (`var feature x`, `assert constraint { ... }`, `item part Shape`) | `parser/defusage.go` `atKindPrefix`, `parseDefUsage` | `parser/namespace_keywords_test.go` `TestParseKeywordBeforeKindKeywordIsNotAName` | ✅ Faithful |
| Binding connector ends (`binding [1] bind [0..*] a.b = [0..*] c`) | `parser/defusage.go` `parseUsage` UsageBinding | `libs/reserved_keyword_name_test.go` (the library's `ShapeItems.sysml` sites) | ✅ Faithful (`bind` is the ends keyword, formerly read as the connector's name) |
| Named argument resolution | `document.go:205` (no name resolution) | `requirement_invocation_test.go` | ✅ Faithful |
| Control flow node registration | `builder.go` InitialNode/FinalNode | `transition_first_test.go` | ✅ Faithful |
| Redefinition target lookup | `document.go:328` searchInheritedFeatureViaIndex | `localclock_test.go` | ✅ Faithful |
| References in behavioral bodies (calc return, constraint/assume/require, assignment, entry/do/exit, transition guard and effect) | `resolve/document.go` `resolveDecl` | `model/behavior_body_resolve_test.go` `TestBehaviorBodyReferencesAreResolved` | ✅ Faithful |
| State def bodies are state bodies whatever their first member is | `parser/defusage.go` DefState case (always `parseStateBody`) | `parse/state_def_region_pseudostate.golden` (a state def whose first member is an attribute, followed by regions) | ✅ Faithful |
| States a region declares with a body (`state x { ... }`) | `lower/state_graph.go` collectRegionStates (memberships unwrapped, region and initial recorded) | `state_region_choice.sysml`, `region_pseudostate_test.go` | ✅ Faithful |
| Substate, region, and named-pseudostate declarations | `symbols/builder.go` StateNode/StateRegion/PseudostateNode | `model/behavior_body_resolve_test.go` `TestBehaviorDeclarationsAreVisible` | ✅ Faithful |
| Region-scoped state names (sibling regions may reuse a name) | `symbols/builder.go` StateRegion | `TestBehaviorDeclarationsAreVisible/sibling_regions_reuse_state_names` | ✅ Faithful |
| Requirement actor declaration | `symbols/builder.go` (`*ast.Usage` of kind `UsageActor`) | `TestBehaviorDeclarationsAreVisible/requirement_actor_binding` | ✅ Faithful |
| Inherited member through a qualified-name segment (`engine::'4cylEngine'`) | `resolve/qualified.go` `walkQualified` → `semantics.Model.LookupMember` | `model/inherited_scope_resolve_test.go` `TestInheritedMembersAreVisible` | ✅ Faithful |
| Redefinition target that the redefinition shadows (`part redefines engine`) | `semantics/model.go` `inheritedFeature` | `TestInheritedMembersAreVisible/nested_redefinition`, `TestRedefinitionDoesNotShadowItsTarget` | ✅ Faithful |
| Loop body as a namespace (`loop { action a; } until a.x`, `for x in c { ... }`) | `symbols/builder.go` WhileLoopActionNode (including a `for` loop's iteration variable), `resolve/document.go`, `passes/typecheck.go`, `lsp/walk.go`; at execution `runtime/action_statements.go` `stmtEnv` (a frame per entered block) | `TestBodyLocalDeclarationsAreVisible`, `TestBodyLocalNamesDoNotEscape`, `runtime/robustness_test.go:loop_body_declaration_does_not_leak` | ✅ Faithful |
| Body-expression parameters (`c->forAll { in i : Positive; f(i) }`) | `symbols/bodyscopes.go` `buildBodyScopes` (scope linked into the document tree), read back by `symbols.BodyExprScope` in `resolve/document.go` and `lsp/walk.go` | `TestBodyLocalDeclarationsAreVisible/body_expression_parameter`, `lsp` `TestRenameLeavesBodyExpressionParameters`, `TestRenameBodyExpressionParameterFromDeclaration`, `TestDefinitionBodyExpressionParameter` | ✅ Faithful |
| Features of the stdlib base type of an untyped usage (`state normal;` → `States::StateAction::done`) | `semantics/implicit.go` `implicitUsageBases`, `Model.implicitBase` via `semantics/model.go` `DirectSupertypes` | `model/implicit_typing_test.go` `TestImplicitUsageBaseTypes`, `TestInheritedMembersResolveThroughUntypedUsage`, `semantics/implicit_test.go`, `lsp/implicit_typing_test.go` | ⚠️ Approximate (the implicit base is the stdlib base *definition* of the usage kind, not the base *feature* it subsets, since library index records carry no specialization edges; connector/succession/flow/binding/satisfy/subject/objective usages take their type from what they relate to and get no base) |
| Implicit redefinition of behavior/step parameters by position (`out item image;` in `action focus : Focus` redefines `Focus::image`), KerML 7.4.7.2/7.4.7.3, SysML v2 7.17.2 | `semantics/redefinition.go` `Model.implicitParameterRedefinitions`, `Model.parametersOf`, reached from `semantics/model.go` `DirectSupertypes` | `semantics/redefinition_test.go`, `model/implicit_typing_test.go` `TestParameterRedefinitionAccompaniesTheImplicitBase`, `TestImplicitRedefinitionSuppliesInheritedMembers`, `passes/typecheck_expr_test.go` `TestExprRedeclaredParametersMatchByPositionNotName` (the invocation signature matches by the same rule) | ✅ Faithful (owned parameters in lexical order redefine the parameter at the same position of each general behavior or step, matching direction; parameters a single general behavior leaves un-redefined are inherited after the owned ones; the kind's standard library base still applies alongside the redefinition, since the redefined parameter may itself be untyped) |
| Implicit redefinition of a result parameter (`return` redefines the general calculation's result whatever its position), SysML v2 7.19.2 | `semantics/redefinition.go` `Model.implicitParameterRedefinitions` (`ast.Usage.IsResult`, set by `parser/behavior.go` `parseResultMember`) | `semantics/redefinition_test.go` `TestImplicitResultParameterRedefinition` | ✅ Faithful |
| A nested usage that is *not* a parameter and shares a name with an inherited feature | `resolve/document.go` `Resolver.checkInheritedNames`, `conflictable`, `parameterizedByName`, surfaced by `passes/nameres.go` (code `name-conflict`); the usage still only gets the standard library base of its kind from `semantics/implicit.go` | `passes/nameres_test.go` `TestNameResolutionPassReportsInheritedNameConflict`, `TestRedeclaredInheritedNameIsNoConflictWhenRedefined`, `TestInheritedNameConflictExemptsRedefiningFeatures`, `TestDistinctNestedNameIsNoConflict`, `model/implicit_typing_test.go` `TestLikeNamedUsageIsNotAnImplicitRedefinition` | ⚠️ Approximate (SysML v2 7.6.1 and KerML 7.3.2.1 make this a name conflict, reported at the name-resolution tier where inheritance is known; a feature that redefines what it shares the name with — explicitly, or implicitly by parameter position — does not conflict. The subject, actors and stakeholders of a case or requirement redefine the inherited ones by name (SysML v2 7.18.4, 7.19.4), which is not modelled and is not distinguishable from an ordinary feature at this tier, so the rule is not applied inside a case or requirement body at all (a concern and a viewpoint are requirements too) — a genuine conflict on an ordinary feature there goes unreported too) |
| Effective name of an unnamed redefining feature (`in item;` in `action shoot : Shoot` is named `image`), KerML 7.3.4.5, SysML v2 7.6.5 | `symbols/builder.go` `effectiveIdent` for a declared redefinition; for an implicit one `resolve/unqualified.go` `Resolver.implicitlyNamedMember` (with `impliesNamingFeature`), reached from `walkUnqualifiedHiding` and `resolve/target.go` `memberChain`, over `symbols/scope.go` `Scope.AnonymousMembers` and `semantics/model.go` `DirectSupertypes` | `passes/nameres_test.go` `TestImplicitlyRedefiningParameterBindsRedefinedName`, `TestImplicitlyRedefiningParameterDoesNotBindItsKeyword`, `symbols/builder_test.go` `TestUnnamedRedefinitionTakesRedefinedName` | ✅ Faithful (the name is bound in the owning scope, so a sibling resolves it by simple name; an implicit redefinition's target is known only to the semantic model, so that binding is resolved lazily rather than when scopes are built) |
| Implicit redefinition of connection/association ends by position (`connection : PressureSeat connect bead references t.bead to ...` redefines `PressureSeat::bead`), SysML v2 7.13.2, 7.14.2, KerML 7.4.6 | `semantics/connector.go` `Model.implicitEndRedefinitions`, `Model.endsOf`, reached from `semantics/model.go` `DirectSupertypes`; the ends themselves are declared by `symbols/builder.go` `buildConnectorEnds` (`ast.ConnectorEnd.DeclaredName`) and the arity check is `passes/constraint.go` `checkConnectorEndRedefinition` (`Model.UnmatchedConnectorEnds`) | `semantics/connector_test.go` (including `TestImplicitEndRedefinitionOfAssociationUsage`), `model/connector_ends_test.go` `TestConnectorEndNamesResolve`, `TestConnectorEndArityMismatch` | ✅ Faithful (an end of a `connect` clause that reference-subsets what it attaches to declares an end of the connector, in lexical order, and redefines the end at the same position of each connector the usage specializes; positions count every end of the clause, including one that only names what it attaches to; an explicit `:>>` governs, and an end past the general connector's last position is reported. Connection, interface, allocation, flow, succession, association and binding declarations are matched — a general whose own ends are not enumerable, such as an unparsed library type, suppresses the arity check rather than reporting) |
| Reference subsetting contributes members (`perform action takePhoto references takePicture;`, `perform providePower.generateTorque;`) | `semantics/reference.go` `Model.ReferencedFeature`, `Model.MemberSources`, consumed by `semantics/members.go` `MembersOf`/`LookupMember`; targets resolved by `resolve/target.go` `ResolveTarget`/`ResolveReferenceTarget` | `semantics/reference_test.go`, `resolve/target_test.go`, `model/perform_reference_test.go`, `parse/perform_reference.golden`, `runtime/testdata/conformance/action_perform_reference.sysml`, `runtime/robustness_test.go` (`perform_of_missing_action`, `perform_reference_cycle`) | ✅ Faithful (a member-contribution relation, deliberately **not** a generalization — see below) |
| Effective name of an unnamed feature that reference-subsets (`perform providePower.generateTorque;` declares `generateTorque`) or redefines (`part :>> engine;`, equivalently `part redefines engine;`, declares `engine`) | `ast/namespace.go` `NamingFeature`, `EffectiveName`, `TargetName`, used by `symbols/builder.go` `effectiveIdent`, `namingTargetNode`, `lower`, `runtime` and `passes`; the reference that named a feature is hidden from its own resolution by `symbols/symbol.go` `Symbol.NamingTarget` and `resolve/target.go` `refFilter` | `symbols/perform_test.go`, `symbols/builder_test.go` `TestUnnamedRedefinitionTakesRedefinedName`, `TestRedefinitionDoesNotOverrideDeclaredName`, `TestReferenceSubsettingOutranksRedefinitionAsNamingFeature`, `TestTwoRedefinitionsLeaveFeatureAnonymous`, `TestShortNameLeavesTheNamingFeatureInPlace`, `resolve/document_test.go` `TestRedefinitionTargetSkipsTheNameItGaveAway`, `model/perform_reference_test.go` | ✅ Faithful (a reference subsetting names the feature, and otherwise its single owned redefinition does; a declared name governs, and more than one redefinition leaves the feature anonymous. A declared short name does not suppress the derived name, since KerML derives effectiveName from declaredName alone. The naming feature's own effective name is approximated by the reference's last segment, since resolution has not run when scopes are built. A value on a member left anonymous by more than one redefinition reaches none of them, which `passes/constraint.go` `checkUnnamedRedefinitionValue` reports as a warning, code `redefinition-no-derived-name`, tested by `passes/constraint_test.go` `TestConstraintUnnamedRedefinitionValue`) |
| A reference subsetting resolves outside the name it contributes, while the members its owner inherits and imports stay visible (`part v : V { perform 'provide power'; }`) | `resolve/target.go` `refFilter`, `Resolver.ResolveReferenceTarget`, threaded through `resolve/unqualified.go` `walkUnqualifiedHiding` and applied in `resolve/document.go` `resolveRelationships`, `lsp/walk.go` `refCollector.relationships` (via `model.Workspace.ResolveReferenceInDoc`) and `runtime/invoke_action.go` `resolveActionSymbol`; the inherited half is `semantics/members.go` `Model.LookupContributedMember` | `resolve/target_test.go` `TestReferenceTargetSkipsSelfBinding`, `semantics/reference_test.go` `TestPerformOfInheritedAction`, `lsp/definition_test.go` `TestDefinitionPerformChainMember` (a chain member resolves through its operand, `resolve.Reference.Chain`), `semantics/reference_test.go` `TestReferenceFindsSiblingDeclaredAfterIt`, `model/perform_reference_test.go` (`perform shadowing the action it performs`), `lsp/definition_test.go` `TestDefinitionPerformReference`, `runtime` `TestPerformShorthandRunsTheReferencedAction`, `conformance/action_perform_shorthand.sysml` | ✅ Faithful |
| The `perform X;` shorthand is an action node named X | `lower/action_graph.go` `getNodeName` | `conformance/action_perform_shorthand.sysml` (`then start increment;` names the perform statement) | ✅ Faithful |
| N-ary connector ends (`connection link connect (a, b, c)`), SysML v2 7.13.2, 8.3.13 | `parser/defusage.go` `parseConnectorEnds` (parenthesized end list, reached by both the named declaration and the anonymous `connect …;` body member); `passes/constraint.go` `checkConnectorEnds` (arity by kind); `lower/connection.go` `lowerConnections`, `PeerPorts` | `parse/connection_nary.golden`, `parser/connector_ends_nary_test.go` `TestParseNaryConnectorEndsKeepsEveryEnd`, `parser/negative_test.go` (`nary_connect_unclosed`, `nary_connect_trailing_comma`, `nary_connect_empty`), `passes/constraint_test.go` `TestConstraintConnectionNaryEndCountReachesTheChecker`, `lower/connection_test.go` `TestLowerNaryConnectionKeepsEveryEnd` and `TestLowerAnonymousNaryConnectionKeepsEveryEnd`, `parser/connector_ends_nary_test.go` `TestParseAnonymousInlineConnectKeepsEveryEnd`, `conformance/action_port_communication_nary.sysml` and `action_port_communication_nary_anonymous.sysml` | ✅ Faithful (a connection, connector, interface or allocation keeps every end of a parenthesized list end to end — parse, constraint tier, lowering and port routing — whether or not it declares a name, and an interface or allocation beyond two ends is reported) |
| Anonymous binary allocation (`allocate torqueGenerator to powerTrain`) | `parser/defusage.go` `atAllocateShorthand` | `parse/perform_reference.golden` | ✅ Faithful (both names are connector ends; formerly the first was read as the usage's name) |
| An object of a connector usage holds the features it connects at its ends (`connection link : Link connect a.p to b.q` makes `link.source` **be** `a.p`), KerML 7.4.6, SysML v2 7.13.2 | `runtime/connector.go` `materializeConnectorSlot`, `materializeConnector`, `attachConnectorEnd`, `bindEndSlot`, `bindParticipants`, reached from `runtime/instance.go` `GetSlot`; end features synthesized by `runtime/shape.go` `connectorEndFeatures`; attachments and effective end names by `semantics/connector.go` `Model.ConnectorEndAttachments`, `Model.IsConnectorUsage`; inherited ends aliased by `runtime/subsetting.go` over `Model.ImplicitEndRedefinitions` | `connector_test.go` (`TestConnectorEndsAreTheConnectedFeatures`, `TestWritingAConnectedPortIsReadThroughTheEnd`, `TestConnectorEndFollowsAFeatureChain`, `TestConnectorEndAttachesToAPart`, `TestNaryConnectorKeepsEveryEnd`, `TestRedefinedEndSharesTheInheritedSlot`, `TestEveryConnectorKindAttachesItsEnds`), `conformance/connector_end_identity.sysml` (identity assertions), `semantics/connector_test.go:TestConnectorEndAttachments`, `robustness_test.go:unattachable_connector_end`, `multiplicity_on_a_connector`, `connector_attached_to_itself`, `mutually_attached_connectors` | ✅ Faithful (an end holds the very object the connector attaches to, so writing the connected port is read through the end and two connectors on different ports are distinguishable; ends are attached in declaration order, including n-ary and nested feature chains and an end attached to a part; an end that names no reachable feature is a typed `ErrConnectorEnd` with a source location rather than a fresh object or `<unknown>`, an end naming the connector it belongs to — directly or through another connector — is `ErrCyclicSlot`, and a connector usage holding more than one connector is reported with where it was written) |
| An untyped or anonymous connector usage materializes on the standard library base of its kind (`interface iface connect a.p to b.q;`, `connect a.p to b.q;`), SysML v2 7.13.2, 8.3.13 | `semantics/implicit.go` `implicitUsageBases` (`Connections::Connection`, `Interfaces::Interface`, `Allocations::Allocation`); `runtime/connector.go` `connectorBaseOf`, `anonymousConnectors`; `symbols/builder.go` `usageSymbolKind` (a KerML `connector` is a connection usage) and `runtime/shape.go` `isFeature` (an allocation usage is a feature) | `conformance/connector_end_identity.sysml`, `ballandchain_interface_connected.sysml`, `connector_test.go` (`TestUntypedConnectorUsageMaterializes`, `TestAnonymousConnectorJoinsItsEnds`, `TestAnonymousConnectorIsMaterializedOnce`, `TestAnonymousSuccessionIsNoConnector`, `TestEveryConnectorKindAttachesItsEnds`), `parse/connection_implicit_type.golden` | ✅ Faithful (a connection, interface, allocation or connector usage that names no definition is an object of its kind's library base with its ends attached, named form and anonymous form alike, and an anonymous one materializes once per object; a flow or binding states its ends by other syntax and is not a `connect` connector — its ends reach routing through lowering, not through connector-end slots) |
| A flow usage (`flow f from a.out to b.in`) and a binding usage (`binding b bind a.p = b.p`) are connectors of the kernel layer, but state their ends in their own syntax — `Usage.FlowEnds`, and a binding's source/target relationships — rather than in a `connect` clause | `parser/defusage.go` `parseFlowEnds` and the `UsageBinding` clause; `lower/connection.go` (flow ends reach routing through lowering); `semantics/connector.go` `Model.IsConnectorUsage` deliberately covers only the `connect` forms | `parse/connection_implicit_type.golden` (`flow f from a.p to b.p;`, `binding bnd bind a.p = b.p;`), `lower/connection_test.go` | ❌ Not implemented as a runtime connector object (a flow between action nodes carries its value, see the named-flow row above; what is missing is the connector *object*: a flow is a transfer performance carrying a payload and a binding makes one value of two features, so neither is materialized by `runtime/connector.go` and a slot holding one reads as unknown. Materializing them means giving a flow its payload transfer semantics and a binding its value identity, which is separate work) |
| A declared name wins over an effective one in the same namespace (`part v { perform p; action p; }`) | `symbols/scope.go` `PreferDeclared`, used by `LookupLocal` and `resolve/qualified.go`'s segment walk; `symbols/builder.go` (`Symbol.EffectiveName`) | `semantics/reference_test.go` `TestReferenceFindsSiblingDeclaredAfterIt`, `TestQualifiedNameThroughEffectiveNameIsNotAmbiguous`, `TestRepeatedPerformResolvesToTheAction` | ✅ Faithful |
| `individual def X :> PartDef`, `x : IndividualDef` kind compatibility, SysML v2 7.9.4 | `passes/typecheck.go` `occurrenceDefSymbolKinds`/`isOccurrenceDefKind` (specialization) and `isCompatibleTyping` (typing) | `passes/typecheck_individuals_test.go`, corpus gate (`Verification Case Usage Example` now clean) | ✅ Faithful (an `individual def` is an occurrence definition, so it may specialize an occurrence definition of any kind and may type a usage wherever an occurrence definition may; specializing a data type — an attribute or enumeration definition — stays an error per 8.4.5.1, and a usage kind that rejects an occurrence definition, such as a port usage, still rejects an individual definition) |
| `individual` / `snapshot` usage modifiers (`individual testSystem : TestSystem`, `snapshot occurrence takeoff : Flight`), SysML v2 7.9.4, abstract syntax 8.3.9.11 (`OccurrenceUsage::isIndividual`, `OccurrenceUsage::portionKind`) | `ast/defusage.go` `Usage.IsIndividual`/`Usage.IsSnapshot`, stored by `parser/defusage.go` `parseUsage` and `parser/behavior.go` `parseDirectionParameter`; consulted by `passes/typecheck.go` `declKind.isOccurrenceUsage`, `compatMessage` and `isCompatibleTyping` | `parser/occurrence_modifier_test.go`, `parse/occurrence_individual_snapshot.golden`, `parser/negative_test.go` (`individual_modifier_no_member`, `individual_usage_no_type`, `individual_usage_no_body`, `snapshot_usage_no_type`, `individual_parameter_no_type`), `passes/typecheck_individuals_test.go` `TestTypeCheckOccurrenceModifierWidensTypingOK`, `TestTypeCheckOccurrenceModifierRejectsDataType`, `TestTypeCheckDataTypeTypingWithoutModifierOK` | ⚠️ Approximate (the modifier is orthogonal to the keyword that declares the usage, so `individual part p` is a part usage that is an individual, and either modifier makes the usage an occurrence usage: it may be typed by an occurrence definition of any kind and may not be typed by a data type — an attribute or enumeration definition — per 8.4.5.1. An individual occurrence takes `Occurrences::Life` as its implicit base (`semantics/implicit.go` `implicitBase`). The modifier is not yet reflected in the usage's symbol kind, so `individual testSystem` is still indexed as an attribute usage — the typing widening compensates) |
| `if`/`else` branch bodies as namespaces | `ast/behavior.go` `IfBranchNode` (parsed by `parser/behavior.go` `parseIfBranch`), `symbols/builder.go` IfActionNode/IfBranchNode, `resolve/document.go`, `symbols/bodyscopes.go`, `lsp/walk.go` | `TestBodyLocalDeclarationsAreVisible/if_branch_body_reads_its_own_declaration`, `/else_branch_reuses_the_then_branch's_name`, `TestBodyLocalNamesDoNotEscape/if_branch_member_from_outside`, `/else_branch_member_from_the_then_branch`, `parse/action_if_branch_body.golden`, `lsp/if_branch_test.go`, `resolve` `TestImportRecursiveSkipsBodyLocalNames`, `repl` `TestLookupInScopeTreeSkipsBodyLocalNames` | ✅ Faithful (each branch owns a body-local scope: names declared in a branch resolve inside it, do not escape to the enclosing behavior or to the sibling branch, and — like loop bodies — are excluded from recursive imports and the REPL scope-tree search; the condition is evaluated before either branch is entered, so it resolves in the enclosing scope only) |
| Transition source/target names | — (deferred to `lower/state_graph.go`) | — | ⚠️ Approximate (not resolved as references, so a misspelled endpoint surfaces at lowering, not at the name-resolution tier) |
| Signal trigger names (`when sigX`) | — | `TestBehaviorDeclarationsAreVisible/signal_trigger` | ⚠️ Approximate (a bare trigger name is an injected event, not a declared element, so it is deliberately not resolved) |
| Payload feature a flow/message declares in its `of` clause (`message m of fuelCommand : FuelCommand`) | `parser/defusage.go` `parseFlowEnds` (declaration recorded as `FlowEnds.PayloadDecl` and kept as a member of the flow), `resolve/document.go` (the `of` name resolves in the flow's own scope) | `parse/flow_payload_declaration.golden`, `model/flow_payload_resolve_test.go` `TestDeclaredFlowPayloadIsAMember`, `TestFlowPayloadReferenceStillResolvesOutward` | ✅ Faithful (the declared payload is a member of the message, so the `of` name and `m.payload` both resolve; the reference form `of Type` still resolves in the enclosing scope) |
| Accept-parameter visibility to sibling action nodes | `runtime/action_executor.go` shared token data | `action_accept_message.sysml` | ⚠️ Approximate (the executor binds the payload into shared token data, which scoping does not model: a sibling node reading the parameter by simple name is reported unresolved) |
| Unqualified library names in files that do not import their library (`Boolean`, `Real`, `that`) | — | — | ❌ Not Yet Implemented (no implicit library import or KerML implicit features, so library files report large numbers of unresolved references) |
| A namespace re-exports what it imports with `import X::*`, transitively and wherever the name X resolves (`KerML::Element`, where `KerML` imports `Kernel::*`, which imports `Core::*`, which imports `Root::*`; KerML 7.2.5, 8.2.3.5) | `symbols/index.go` `ExpandWildcardImports` (repeats `expandRound` to a fixpoint over the importers in name order, deriving the re-exports of the ones a change reached and dropping those its imports no longer support) and `resolveWildcardTarget` (searches the importing package's enclosing namespaces before the global one) | `symbols/index_test.go` `TestExpandWildcardImportsChainsAndIsOrderIndependent`, `TestExpandWildcardImportsPrefersTheEnclosingTarget`, `TestExpandWildcardImportsFollowsAReexportedTarget`, `TestExpandWildcardImportsIgnoresAnAmbiguousTarget`, `libs/loader_cache_test.go` `TestParsedAndRestoredIndexesAreEquivalent`, `model/training_examples_test.go` `TestTrainingExamplesCacheStateIndependent` | ✅ Faithful (a chain of imports is followed to its end, and the result does not depend on iteration order or on whether the library was parsed or restored from the on-disk index cache; a target name resolves against the importing namespace's own imported memberships — `wildcardTargetAt` follows a name an earlier import re-exported to the FQN it was declared under — before the global namespace) |
| A private `import X::*` is not re-exported by its namespace, and the names it brings in are visible only within that namespace (KerML 8.2.3.3) | `symbols/index.go` `applyReexportMarks` / `exportedChildren` (a re-export is hidden while every document that surfaced it did so with a private import, and left out when that namespace is itself wildcard-imported) and `LookupQualifiedFrom` (a hidden name answers a lookup only when the referring namespace is the one that hid it, or is nested in it); `resolve/qualified.go` `referringNamespaceFQN` supplies that context for a qualified reference and `HiddenFrom` stops its member-lookup fallback, which reaches a cached symbol's children without consulting the marks, from resurfacing a hidden name; `resolve/alias.go` `resolveCachedAliasTarget` supplies the context for the target of a cached alias; `resolve/unqualified.go` `matchImport` enumerates a wildcard import's target through `symbols/index.go` `LookupDirectChildrenFrom`, which reads the same marks from the referring namespace unless the import is `import all` | `symbols/index_test.go` `TestExpandWildcardImportsDoesNotCarryOnAPrivateImport`, `TestLookupQualifiedFromSeesAPrivateImportOnlyFromWithin`, `TestHiddenFromReportsOnlyPrivatelySurfacedNames`, `TestLookupQualifiedReachesAPubliclyImportedName`, `TestLookupQualifiedAcrossAChainedPrivateImport`, `TestLookupDirectChildrenFromDropsPrivatelyImportedNames`; `resolve/qualified_test.go` `TestResolveQualifiedRejectsAPrivatelyImportedName`, `TestResolveQualifiedRejectsAPrivatelyImportedNameThroughMemberLookup`, `TestResolveQualifiedFromInsideAnUnnamedElement`, `TestResolveQualifiedReachesAPubliclyImportedName`; `resolve/visibility_test.go` `TestNamespaceImportSkipsAPrivatelyImportedName`, `TestNamespaceImportSkipsAPrivatelyImportedCachedName`; `resolve/alias_test.go` `TestAliasResolvesAPrivatelyImportedTargetFromCache`, `TestAliasResolvesAPrivatelyImportedTargetWhenParsed`; `model/visibility_reach_test.go` `TestPrivateWildcardImportIsNotReExportedAcrossDocuments` | ✅ Faithful (neither a qualified nor an unqualified reference reaches a privately imported name from outside the importing namespace, whether the index was parsed or restored from cache; an `import all` still takes the target's private memberships, and a reference inside the importing namespace still sees them) |
| Visibility of the members a recursive import surfaces (`import X::**`, KerML 7.2.5) | `resolve/unqualified.go` `matchImport` (both the membership and namespace branches filter through `resolve/visibility.go` `visibleThroughImport`) | `resolve/visibility_test.go` `TestRecursiveMembershipImportSkipsPrivate`, `TestNamespaceImportSkipsPrivate`, `TestImportAllReExportsPrivate` | ✅ Faithful (a recursive membership import hides private members of the subtree it walks unless it is `import all`) |
| `expose` in a view body is an Import (SysML v2 8.3.26.2 Expose, 8.3.26.3 MembershipExpose, 8.3.26.4 NamespaceExpose) | `parser/defusage.go` (`expose` shares `parser/namespace.go` `parseImportTail`, so `::*` yields a NamespaceExpose and `::**` a recursive MembershipExpose; `ast.Import.IsExpose`, `IsAll`, protected `Visibility`) | `parser/expose_test.go` `TestParseExposeImportKind`, `TestParseExposeIsImportAllAndProtected`, `resolve/expose_test.go` | ⚠️ Approximate (an Expose always imports all elements regardless of visibility — `validateExposeIsImportAll` — so its exposed elements resolve inside the view body, in views that specialize it, and not outside; `validateExposeOwningNamespace` is implemented — see the row below) |
| Protected import visible in specializations of the importing definition or usage (SysML v2 7.5.3) | `resolve/visibility.go` `inheritedThroughSpecialization` (a protected or public import reaches specializations, a private one does not — KerML 8.2.3.3) and `lookupInheritedImports` (walks `semantics.Model.DirectSupertypes` upward from the referring scope's owner, breadth-first and cycle-guarded, matching each supertype's inherited imports through the same `matchImport`); `resolve/unqualified.go` `walkUnqualifiedHiding` consults it after the imports declared in the scope itself | `resolve/protected_test.go` `TestProtectedImportReachesADirectSpecialization`, `TestProtectedImportReachesATransitiveSpecialization`, `TestProtectedImportReachesAUsageTypedByTheImporter`, `TestProtectedImportDoesNotReachAnUnrelatedNamespace`, `TestPrivateImportDoesNotReachASpecialization`, `TestProtectedImportAllReachesASpecializationWithPrivateMembers`, `TestExposeReachesASpecializingView`, `TestInheritedImportWalkTerminatesOnASpecializationCycle`; `model/visibility_reach_test.go` `TestProtectedImportReachesSpecializationsAcrossDocuments`, `TestProtectedImportDoesNotReachAnUnrelatedDocument`, `TestExposeReachesASpecializingViewAcrossDocuments` | ✅ Faithful (an `expose` is protected, so it reaches a specializing view the same way; a feature typing is a generalization edge — KerML 8.3.4.6 — so an import declared in a definition is also reached from a usage typed by it, and an unrelated namespace sees nothing) |
| `validateExposeOwningNamespace` — the importOwningNamespace of an Expose must be a ViewUsage (SysML v2 8.3.26.2) | `passes/expose.go` `checkExposeOwners` / `exposeOwnerDiagnostic`, run by `passes/constraint.go` `ConstraintPass` at `LevelConstraint`; code `expose-owning-namespace` | `passes/expose_test.go` `TestExposeOwningNamespace`, `model/expose_owner_test.go` `TestExposeOwnerAcrossDocuments` | ✅ Faithful (usage-only reading, per maintainer decision: an `expose` owned by a view usage is legal, one in a `view def` body is a **warning** since Systemica resolves it — `resolve/expose_test.go` `TestExposeInViewDefinitionBody` — and any other owner is an error; a package or namespace body rejects `expose` in the parser) |

#### Design note: `references` is a member-contribution edge, not a generalization

A `perform` action usage relates the action it performs through a
**ReferenceSubsetting**, written `references` or `::>` (SysML v2 §7.17.6; the
derived `PerformActionUsage::performedAction` comes from that owned reference
subsetting, §8.3.17.14). KerML makes ReferenceSubsetting a syntactically
distinguished kind of Subsetting (§8.3.3.3.9), which is why the referenced
feature's members are visible on the referencing one.

It is nevertheless kept out of `semantics.Model.DirectSupertypes`. Subsetting
in this implementation drives conformance and implicit typing, and a perform
statement is not a subtype of the action it performs for those purposes: making
it one would give `perform action takePhoto references takePicture;` the type of
`takePicture` and silently change conformance results elsewhere. Instead
`Model.MemberSources` — the union of the generalization edges and the reference
subsetting, breadth-first and cycle-guarded — is what member lookup consumes, so
`takePhoto.focus` resolves while `AllSupertypes(takePhoto)` stays free of
`takePicture`.

Two consequences of the spec's naming rules fall out of this and are implemented
alongside it: an unnamed feature takes the effective name of the feature it
references (KerML `Feature::effectiveName`), so `perform providePower.generateTorque;`
declares `generateTorque`; and because that name is bound in the same scope the
reference resolves in, the reference is resolved outside its own binding: a
`refFilter` hides just that borrowed binding for the duration of the lookup,
leaving each scope's declarations, inherited members and imports intact, so a
`perform` of an action the owner inherits from its type still resolves.

---

## What We Don't (Yet) Support

### Decisions to Reassess

Deliberate limitations whose *current* handling should be revisited once the
feature they wait on lands (this repository has issues disabled, so follow-ups
are tracked here):

| Deferred until | Reassess |
|---|---|
| A parameter of a behavior or step whose general type comes from the library index | `semantics/redefinition.go` `ownedParameters` reads the declaration's members, so a general behavior with no parsed AST (a cached library symbol) contributes no positions and its parameters are not redefined. Needs parameter order in `IndexRecord`, like the `Supers` row below. |
| Scalar type inference for a bare feature reference (`passes/typecheck_expr.go` `infer`) | A condition that is a plain name — `while total { … }`, `total : Integer` — infers Unknown, so `checkBoolean` passes it and the executor reports it (`runtime/action_statements.go` `evalCondition`) instead. Once a feature reference infers its declared scalar type, this becomes a typecheck error like the literal and operator cases, and the runtime check goes back to being unreachable. |
| Specialization edges in the library index (`libs/loader.go` `recordEntries` drops `Supers`) | `implicitUsageBases` maps each usage kind to its stdlib base *definition* because the base *feature* the spec has usages subset would be a dead end for member lookup. With the edges recorded, the map should name the base feature the spec names. |

### Major UML/SysML Features Not Implemented

**Activity Diagrams (Advanced):**
- Interruptible regions
- Expansion regions (parallel/iterative)
- Streaming pins
- Exception handlers
- Structured activities with pin connectors

**State Machines (Advanced):**
- Textual notation for history, entry and exit point pseudostates, and for deferred events (the runtime supports them; only the syntax is missing)
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
- Allocation execution semantics beyond materializing the allocation and its ends

### What Can't Be Claimed for Spec Compliance

**Intentionally Unspecified (No Normative Semantics):**
- Verification verdict evaluation (VerdictKind/PassIf) - SysML v2 §9.3.2: "evaluation... intentionally not specified normatively"
- Variability/variation selection - SysML v2 §9.4: "Selection of variants is not specified normatively" — Systemica selects the variant a variation usage is bound to (`attribute :>> cut = cut::cutIdeal;`) and errors on an unselected, unknown, or multiply-selected variation; see the Variation and Variant map
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
| `context.go` | Execution context, constraint/requirement evaluation | ~430 |
| `invoke_calc.go` | Calc invocation: parameter/result resolution across specialization, binding, recursion bound | ~300 |
| `action_executor.go` | Token-flow semantics, control flow nodes, nested actions | ~729 |
| `state_executor.go` | Event-driven state machines, transitions, hierarchical states, pseudostates | ~1149 |
| `eval.go` | Expression evaluation (operators, literals, features) | ~758 |
| `value.go` | Runtime value representation (ValConst, ValString, ValInstance) | ~150 |
| `trace.go` | Deterministic execution and calc-evaluation trace recording, canonical value rendering | ~290 |
| `conformance_test.go` | Conformance gate (71 cases) | ~480 |
| `robustness_test.go` | Failure-mode tests (39 subtests) | ~830 |
| `trace_test.go` | Golden trace test infrastructure | ~200 |
| `trace_calc_test.go` | Trace determinism and canonical rendering unit tests | ~180 |

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

**Test Counts** (re-counted from the checked-in fixtures and from `-v` runs):
- Execution conformance cases: 113 (all passing)
- gRPC conformance cases: 6 (all passing)
- Robustness subtests: 54 (all passing)
- Golden AST fixtures: 52
- Golden execution traces: 40
- Negative parser subtests: 49

**Coverage by Feature Type** (execution conformance cases, by fixture prefix, 113 total):
- Calc: 15 conformance + 12 golden traces (includes unary, coercion, qualified-name, KerML library and Systemica extension library function evaluation)
- Constraint: 7 conformance + 4 golden traces
- Requirement: 12 conformance
- Action: 35 conformance + 17 golden traces
- State: 30 conformance + 7 golden traces
- Accept: 1 conformance (`accept_then_transition`)
- Instance: 8 conformance (`instance_derived_slots`, `instance_constraint_binding`, `instance_inherited_constraint`, `instance_library_function_default`, `instance_nested_usage_body`, `instance_unnamed_redefinition` among them)

**Quality Gates:**
- Parser: 95/95 stdlib files clean (94 vendored OMG, 1 Systemica extension)
- Execution conformance: 113/113 cases passing
- Training examples: 98/100 clean (2 files / 4 errors, both pinned OMG source bugs, gated by `internal/core/model/testdata/training_examples_expected.txt`)
- No regressions: All tests pass on every commit

> The training-example gate needs the corpus, which is not vendored: run
> `./scripts/download-training-examples.sh` first. The gate skips while the corpus is
> absent, so run the script before claiming a change is clean locally. CI downloads it
> (`.github/workflows/pr.yml`) and sets `SYSTEMICA_REQUIRE_TRAINING_CORPUS=1`, which turns
> an absent corpus into a failure, so the gate can no longer skip green there.
> The gate runs against an empty semantic cache (`t.Setenv("XDG_CACHE_HOME", t.TempDir())`),
> so it reports the same 98/100 on any machine.

---

## Model Persistence and RDF Interchange

**Implementation:** `internal/core/rdf`, `internal/core/export`
**User surfaces:** `%save` (`internal/repl/meta.go`), `sysml -convert` (`cmd/sysml/main.go`)
**Reference:** [`RDF_INTEROP.md`](RDF_INTEROP.md) — the mapping, the CLI, and the limitations in full

Two representations are supported, SysML textual notation and RDF Turtle. No JSON
is used as an input, an output, or an intermediate form.

| Capability | Implementation | Test Case | Status |
|-----------|----------------|-----------|--------|
| Save a model as notation, preserving comments and notes | `export.Convert` → `format.Source` (token stream, not an AST re-print) | `export_test.go:TestSaveKeepsComments`, `repl/save_test.go:TestMetaSaveSysML` | ✅ Faithful |
| Save a model as RDF Turtle | `export.ToRDF` + `rdf.WriteTurtle` | `repl/save_test.go:TestMetaSaveTurtle`, golden `.golden.ttl` fixtures | ✅ Faithful |
| Notation → RDF for every definition/usage keyword the parser accepts | `export/kinds.go` metaclass tables, `rdf_out.go` `encode` | `export_test.go:TestGoldenConversions` (14 fixtures) | ✅ Faithful |
| RDF → notation for the mapped subset | `rdf_in.go` `ToSysML` | `export_test.go:TestGoldenConversions`, `TestConvertedNotationParses` | ✅ Faithful |
| Round trip preserves the graph (`sysml→ttl→sysml→ttl` is stable) | both directions | `export_test.go:TestRoundTripIsLossless` | ✅ Faithful |
| Deterministic, reversible element IRIs keyed by qualified name | `rdf/vocab.go` `ElementIRI`/`QualifiedNameOf` | `rdf_test.go:TestElementIRIRoundTrip`, `export_test.go:TestElementIRIsAreQualifiedNames` | ✅ Faithful |
| Declaration order preserved across a format with no order | `sysx:memberIndex` | `TestRoundTripIsLossless` | ✅ Faithful |
| Turtle writer/parser (prefixes, `a`, `;`/`,` grouping, typed and language literals, long strings, escapes, `@base`) | `rdf/turtle_write.go`, `rdf/turtle_parse.go` | `rdf_test.go:TestTurtleRoundTrip`, `TestParseTurtleForms`, `TestParseTurtleEscapes` | ✅ Faithful |
| Syntax errors rejected, never partially converted | `export.SyntaxError`, `rdf.ParseError` (with line) | `export_test.go:TestSyntaxErrorIsReported`, `cmd/sysml/convert_test.go:TestConvertErrors` | ✅ Faithful |
| Unsupported RDF reported, never silently dropped | `export.UnsupportedError` | `rdf_test.go:TestParseTurtleRejects`, `export_test.go:TestUnsupportedTurtleConstructs`/`TestUnknownMetaclassIsUnsupported`/`TestForeignGraph` | ✅ Faithful |
| Expression-valued positions (values, bounds, guards, filters) | carried as source text, not expression trees | `TestRoundTripIsLossless` | ⚠️ Approximate — converts back exactly, but not queryable by SPARQL |
| End-binding heads (`connect`, `bind`, `flow`, `succession`, `transition`, `accept`, `satisfy`) | carried as `sysx:sourceText` with structural properties alongside | `export_test.go:TestVerbatimHeadsRoundTrip` | ⚠️ Approximate — exact through Systemica; a foreign graph without the text is reported as unsupported rather than guessed |
| Accept-action shorthand (`action X accept p : T [via Port]`) | parameter encoded structurally; printer rebuilds the shorthand | `export_test.go` fixture `testdata/convert/accept.sysml`, `parser/testdata/parse/accept_action_shorthand.golden` | ✅ Faithful |
| `then` succession between members | `sysml:SuccessionAsUsage` with `sourceFeature`/`targetFeature`, from the one edge node every `then` parses to, in every body that admits one (a state's regions included) | `export_test.go:TestSuccessionRoundTrips`, `:TestSuccessionRoundTripsInEveryBody`, `parser/succession_test.go:TestMemberAttachedThenDesugars`, `:TestMemberAttachedThenInRegionDesugars` | ⚠️ Approximate — a `then` beside a member with no name cannot be named by these ends: it warns (`unnamed-succession-end`) and no edge is recorded |
| Two members of one namespace sharing a name | refused: the qualified name is an element's graph identity | `export_test.go:TestDuplicateNameIsUnsupported` | ❌ Rejected rather than merged |
| Ownership cycle in an input graph | refused: no root owns the element, so printing would emit an empty document | `export_test.go:TestOwnershipCycleIsUnsupported` | ❌ Rejected rather than emitting an empty file |
| Lexical `//` and `/* */` trivia across the RDF hop | no element owns trivia; `doc`/`comment` are declarations and do convert | `export_test.go:TestCommentsThroughRDF` | ❌ Not carried through `.ttl` (a direct `.sysml` save keeps it) |
| Blank nodes, RDF collections, bare literal shorthands | rejected by `rdf.ParseTurtle` | `rdf_test.go:TestParseTurtleRejects` | ❌ Not supported (by design; see RDF_INTEROP.md) |

**Vocabulary:** `sysml:` = `https://www.omg.org/spec/SysML#` and `elmt:` =
`urn:sysmlv2:element:` match the Flexo MMS SysML v2 service's `Namespaces.kt`, so
a converted graph loads into that triplestore. Properties the SysML metamodel
does not define are confined to `sysx:` = `urn:systemica:sysml:`: `memberIndex`,
`hasBody` and `sourceText` carry order, body presence and verbatim heads, and
`prefixMetadata`, `filter`, `isNamespaceImport`, `isRecursive` and `isExpose`
carry notation the metamodel has no property for.

**What can't be claimed:** this is not a normative SysML v2 → RDF/OWL mapping.
OMG's abstract syntax has no standard RDF serialization, so the property names
follow the metamodel's own attribute names and the Flexo service's conventions.
A model converted here is faithful to *itself* on a round trip; it is not
guaranteed to be interpreted identically by an unrelated SysML RDF tool.

---

## gRPC Service Layer

**Implementation:** internal/grpc/service.go  
**Status:** ✅ Functional, ✅ §5.2 test contract satisfied for the wrapper

### Runtime RPC Handlers

| RPC | Implementation | Status | Tests |
|-----|---------------|--------|-------|
| GetServerInfo | service.go `Service.GetServerInfo`, capability names in `Capabilities()` | ✅ Faithful — reports the build version (informational; a source build reports `dev`) and the capabilities this build supports by name, currently `type_facts`. A capability is added, never renamed or dropped with its behaviour intact, so a client requires one instead of comparing versions; a service predating this RPC answers `UNIMPLEMENTED`, which the client reads as supporting no capability | service_test.go:TestGetServerInfo, `TestGetServerInfoTypeFactsCapabilityIsHonest`, python/tests/test_capabilities.py |
| ParseFile | service.go:39-123 (parser + passes.Analyze + stdlib load) | ✅ Faithful | runtime_test.go:TestParseFile_*, every conformance case |
| GetSymbol | service.go:126-145; static type facts in typefacts.go (`SymbolInfo.type_info`, `.multiplicity`, `.specializations`) computed by a per-model resolver + semantics context cached on the model and locked for the duration of a conversion | ✅ Faithful — reports the declared and resolved type, the library scalar it reduces to, quantity/unit, the declared multiplicity, and *every* generalization edge (`specializes`, `subsets`, `redefines`, `typing`) in declaration order; an unresolved name is reported unresolved rather than guessed | service_test.go:TestGetSymbol_*, typefacts_test.go:TestTypeInfo*, `TestSpecializations*`, `TestMultiplicity*`, `TestSymbolContextConcurrentConversion` |
| GetDiagnostics | service.go:148-169 (parser + semantic) | ✅ Faithful | runtime_test.go (implicit) |
| Evaluate | service.go:172-227 | ✅ Faithful | runtime_test.go:TestEvaluate_*, conformance `evaluate_arithmetic` |
| Instantiate | service.go (slots read through `Instance.GetSlot`, so a derived default is evaluated against the instance; `InstanceGraphToProto` in convert.go returns every instance reachable from the root in `InstantiateResponse.instances`) | ✅ Faithful — a composite slot still marshals as the child's id, and that child is carried in the same response, so a nested object is reachable over gRPC without a follow-up RPC; expansion is bounded at depth 8 and stops at a type already on the path, as `%slots` bounds it, so a self-referential part cannot instantiate forever | runtime_test.go:TestInstantiate_*, instance_graph_test.go:TestInstantiate_ReturnsNestedInstances, `_ReturnsDeepNestedInstances`, `_CollectionOfInstances`, `_SlotErrorReported`, `_SelfReferentialPartTerminates`, `_MutuallyRecursivePartsTerminate`, conformance `instantiate_part`, `instantiate_derived_slot` |
| ExecuteAction | service.go:265-312 | ✅ Faithful | runtime_test.go:TestExecuteAction_*, conformance `execute_action_inputs`, `execute_action_no_initial` |
| ExecuteState | service.go:315-355 | ✅ Faithful | runtime_test.go:TestExecuteState_*, conformance `execute_state_transitions` |

### Test Coverage (AGENTS.md §5.2 Four-Layer Contract)

**Current:**
- ✅ Layer 1 (Golden AST): Covered via parser tests (fixtures in internal/core/parser/testdata/)
- ✅ Layer 2 (Execution conformance): `internal/grpc/conformance_test.go` drives `Evaluate`, `Instantiate`, `ExecuteAction` and `ExecuteState` from `.sysml` + `.expected.json` pairs in `internal/grpc/testdata/conformance/` (6 cases, one of them a failure mode), each parsed through the `ParseFile` RPC so the whole wrapper is exercised. Schema: that directory's `README.md`.
- ✅ Layer 3 (Golden traces): N/A — the wrapper adds no ordering behavior of its own; traces are pinned at the runtime tier.
- ✅ Layer 4 (Robustness): `internal/grpc/robustness_test.go` covers the wrapper's failure modes (unknown model hash, unknown symbol, malformed expression); execution-level failure modes stay pinned in `internal/core/runtime/robustness_test.go`.

**Rationale:** the gRPC layer is a protocol wrapper over `internal/core/runtime`, which carries full §5.2 compliance for execution semantics. Its own conformance cases assert what the wrapper is responsible for: symbol lookup by FQN, input binding, value marshalling in both directions (including which `Value` oneof arm is set), the state-visit trace, and in-band error reporting.

### Known Limitations (Non-blocking)

**Runtime:**
- an inline entry/do/exit body is one action, so an outgoing transition interrupts a do
  body only between rounds, never between its statements; the one-action-per-statement
  `do { … }` form is the interruptible spelling
- entering a composite state runs its own entry body before the region's initial
  transition reaches the substate, so a parent's do body can run before a substate's
  entry body (`state_anonymous_action_body.trace.golden`). Pre-existing region-entry
  scheduling, not specific to inline bodies
- an entry/do/exit behavior that both performs an action and states a body of its own is
  reported at execution rather than at parse time: which of the two SysML means is
  unadjudicated, so neither is chosen
- a calc output only a branch that did not run would assign is unassigned for that
  activation; the body is not statically required to bind every output it declares
- a calc whose only computation is rebinding an `inout` in its body, with no `out` and no
  return, is still reported as having no result expression: an `inout` is bound by the
  invocation, so it does not count as an output the body computes
- a nested body over a value the *redefined* declaration wrote (`part def Ring { attribute cost : Cost = template; }` re-opened as `part r : Ring { attribute :>> cost { attribute :>> v = 11.0; } }`) reads the inherited value and drops the body's restatements: a value takes precedence over instantiation regardless of which declaration wrote it, so the innermost body does not govern. Pre-existing; a body over a feature that has no inherited value materializes correctly

**Python bindings:**
- generated typed classes (`pysysml.generate`) cover structural usages only; `subsets`
  and `redefines` are exported as facts but do not become Python base classes, and
  redefinition narrowing is not checked
- `TypedObject.from_instance` rejects an instance whose type another generated class
  describes, and accepts a generated subclass of the expected one; it accepts a type
  no generated class describes, because `Instantiate` on a usage reports the usage's
  own FQN (`Demo::myCar`), which the client cannot relate to the definition typing
  it. A wrong-typed instance is therefore caught only when its type has a generated
  class of its own; `unchecked(instance)` bypasses the check deliberately
- connection.py:488 - PID ownership check uses substring match - spoofable
- an `instance_id` outside an `Instantiate` response (an `Evaluate` result, say)
  is still a bare int64: those responses carry no instance graph to resolve it
- __init__.py:11-16 - Shadows builtins (RuntimeError, eval)
- binary.py:82,89 - Checksum same-origin (no pinned hash)

**Standard behavioral notation:**
- a succession written at namespace level (`first part1::action1 then requirement1;`) is
  parsed and carried with both ends, but there is no enclosing behavior to lower it into,
  so it does not execute
- `accept at`/`accept after` inside an *action* body reports `ErrNoClock`: the action
  executor has no clock. The same trigger on a state transition waits on time and fires
- a succession end with no name is carried by identity, so a model whose succession has a
  positional end is reported as unsupported by the RDF export rather than written back
- a time trigger carrying a unit (`accept after 5 [s]`) reports `time duration must be
  constant, got quantity` when the transitions are scheduled; the unitless form waits
- the message bus identifies a send's addressee and a `via` send's port by simple name, so
  two unconnected parts owning ports of the same name see each other's port-routed traffic;
  an accept still takes only what reached its own port, never a broadcast
- a flow whose ends name no feature to carry (`flow a to b;` between two action nodes) and
  a flow whose end names something that is not a node of the action are reported when the
  graph is built: the notation needs a payload or a pin at each end
- a nested action node written inside a loop or branch body is lowered as unsupported and
  reported when reached; only statements, and the body parameter the body itself is written
  as, execute in those bodies
- the Open-MBEE corpus models still report structural diagnostics outside this scope —
  conjugated connection ends (`end spacePort : ~CommunicationPort`), `timeslice item item1`,
  `end ;`, and unresolved library references (`Scalarattributes::String`, `start`,
  `envelopingShapes`, `mRefs`)

**Go gRPC layer:**
- convert.go:40 - SymbolToProto.Attributes always empty (semantic layer not ready)
- `metadata["type"]`/`metadata["specializes"]` still report only the first edge, kept
  for compatibility; `specializations` is the complete list
- a quantity is exported as `type_info.quantity` + `unit`, but the wire `Value` has no
  magnitude-and-unit form, so the slot itself still reads as unsupported
- runtime instances are request-local, so an id is resolvable only against the
  response that carried it; there is no RPC that fetches an instance by id later

These are documented for transparency; none block production use.
