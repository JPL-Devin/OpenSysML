# SysML v2 Specification Compliance

**Purpose:** Document implementation coverage of SysML v2 / KerML / UML 2.5.1 behavioral semantics.

**Related:** [`TESTING.md`](TESTING.md) (test contracts), [`ARCHITECTURE.md`](ARCHITECTURE.md) (runtime architecture)

---

## Current Implementation Status

### ✅ Fully Implemented & Tested (~98% of Targeted Features)

**Calculations (12/12 features):**
- Invocation with typed parameters
- Return expression evaluation (both `return <expr>;` and a bound return parameter `return : T = <expr>;`)
- Parameter binding (positional + named arguments)
- Parameter defaults (own and inherited)
- Inherited parameters and result through a typed calc usage, including redeclaration
- Nested calc invocation, and invocation from a constraint
- Control flow (if/else)
- Unary operators (not, -)
- Type coercion (Integer→Real)
- Qualified names (A::B::C)
- Deterministic evaluation trace (parameter binding, sub-expression order, results)
- Error handling (unbound/unknown parameters, arity, missing return, recursion and step budgets)

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
- 121 conformance cases (all passing: calc×15, constraint×7, requirement×12, satisfy×5, unit×7, action×35, state×31, accept×1, instance×8)
- 57 robustness subtests (deadlock, a non-terminating loop, a body-local declaration that must not leak, a body member that is not executable, a statement written directly among an action's members, accept suspension that can never end, guards, budgets, sourceless accept, fork/join misuse, pseudostate dead ends and cycles, non-numeric time trigger, misaddressed send, accept of an unsent type, send through an unconnected port, history misuse, non-deferrable deferred trigger, non-terminating do behavior, calc binding/arity/recursion failures, unhandled call, call argument of the wrong type, missing and cyclic `perform` references, a library function outside its domain or with the wrong arity, an extension library function outside its domain, exponentiation beyond the Integer range)
- 233 runtime test functions (`grep -c '^func Test' internal/core/runtime/*_test.go`), the conformance, trace and robustness gates above among them
- 52 golden AST fixtures (including the three loop forms, pseudostate, timed-trigger, call-trigger, calc default/invocation and n-ary connector-end parsing tests)
- 40 golden execution traces (loop and conditional bodies, fork/join branch ordering, region entry/exit ordering, do behavior interleaving across orthogonal regions, send/accept, an accept parked until its message arrives, calc and constraint evaluation, library function invocation)
- 59 negative parser subtests
- 2,465 tests and subtests, of which 2,460 pass and 5 skip (`go test -race -count=1 -v ./...`; 1,384 top-level `Test` functions). The skips are unimplemented-expression and requirement-subject cases that skip themselves.

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
| Redeclared parameter keeps its inherited position and default | `invoke_calc.go` `calcParameters` | `calc_return_parameter.sysml` | ✅ Faithful |
| Nested calc invocation | `eval.go` `evalInvocation` → `invoke_calc.go` `invokeCalc` | `calc_nested_invocation.sysml` | ✅ Faithful |
| Calc invoked from a constraint | `context.go` `EvaluateConstraint` → `eval.go` `evalInvocation` | `calc_from_constraint.sysml` | ✅ Faithful |
| Deterministic evaluation trace (binding, sub-expression order, result) | `trace.go` `RecordCalcEnter`/`RecordCalcBind`/`EndEval`, `eval.go` `Eval` | `*.trace.golden` via `TestExecutionTrace`, `trace_calc_test.go:TestCalcTraceIsStableAcrossRuns` | ✅ Faithful |
| Canonical rendering of unordered values in traces | `trace.go` `FormatTraceValue` | `trace_calc_test.go:TestFormatTraceValueCanonicalizesSets` | ✅ Faithful |
| Unbound parameter detection | `invoke_calc.go` `bindCalcParameter` (`ErrUnboundParameter`) | `robustness_test.go:testCalcUnboundParameter` | ✅ Faithful |
| Surplus positional arguments | `invoke_calc.go` `checkArgs` (`ErrCalcArity`) | `robustness_test.go:testCalcTooManyArguments` | ✅ Faithful |
| Named argument that names no parameter | `invoke_calc.go` `checkArgs` (`ErrUnknownParameter`) | `robustness_test.go:testCalcUnknownNamedArgument` | ✅ Faithful |
| Invoked symbol is not a calc | `invoke_calc.go` `calcShapeOf` (`ErrNotACalc`) | `robustness_test.go:testCalcSymbolIsNotACalc` | ✅ Faithful |
| Recursive calc (direct or mutual) is bounded | `invoke_calc.go` `invokeCalcShape` (`ErrCalcRecursionLimit`, depth 32) | `robustness_test.go:testCalcDirectRecursion`, `:testCalcMutualRecursion` | ⚠️ Approximate (depth-bounded; recursion is rejected rather than evaluated) |
| Step budget bounds calc evaluation | `context.go` step counter (`ErrStepLimitExceeded`), budget from `budget.go` `BudgetsFromEnv` (`SYSML_MAX_STEPS`, default 10000000) | `robustness_test.go:testStepBudgetExceeded`, `budget_test.go:TestBudgetFromValue` | ✅ Faithful |
| Control flow (if/else) in calc | `eval.go` expression evaluation | `robustness_test.go:testDecisionNoSatisfiedGuard` | ✅ Faithful |
| Missing return expression | `invoke_calc.go` `calcShapeOf` (`ErrNoResultExpression`) | `robustness_test.go:testCalcWithoutResult` | ✅ Faithful |
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

⚠️ A multi-valued feature that is both typed and given a default takes the typed instantiation; the default is not merged into it.

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

⚠️ The spec's own `QuantityCalculations::ConvertQuantity(x, targetMRef)` is not an invocable function, and a quantity is not yet an instantiated `ScalarQuantityValue` object whose `num`/`mRef` features can be read by name: the unit is carried on the runtime value, not modelled as a library object. The gRPC value schema has no magnitude-and-unit form, so a quantity crossing that boundary is reported as unsupported rather than serialized as a bare magnitude (`internal/grpc/convert.go`). Sequence indexing (`speeds#(3)`), which the parser represents with the same node, is still unevaluated: the two forms are told apart at the node (`ast.IndexExpr.Bracket`), so indexing is not read as a magnitude in a unit.

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
| A member-attached `then` sequences the members either side of it (`action a; then action b;`) | `parser/succession.go` (desugared at parse time to the `*ast.SuccessionEdge` the `then a b;` notation builds), lowered by `lower/action_graph.go` and `lower/state_graph.go` like any other edge | `conformance/action_member_then_order.sysml` + trace golden (declaration order is the reverse of the execution order), `conformance/state_member_then_order.sysml` (the same for a state's completion transitions), `parser/succession_test.go:TestMemberAttachedThenDesugars`, `parse/succession_member_then.golden` | ⚠️ Approximate (an edge names its ends, so a `then` beside a member with no name — after an anonymous member, or before a `send` statement — warns as `unnamed-succession-end` and records no edge; the notation is otherwise carried wherever a succession is legal, and a `then` before a member the notation does not admit one in front of, such as an attribute or a definition, is a syntax error) |
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
| Final state termination | `state_executor.go:288` processNextEvent | `state_simple.sysml` | ✅ Faithful |
| State entry actions | `state_executor.go:749` enterState | `state_do_behavior.sysml` | ✅ Faithful |
| State exit actions | `state_executor.go:810` exitState | `state_transition_effect.sysml` | ✅ Faithful |
| An entry/do/exit action given by reference (`entry warmUp;`) is a performed action usage subsetting the referenced action (`StateActionUsage` → `PerformedActionUsage` → `PerformActionUsageDeclaration`) | `parser/behavior.go` parseStateSubaction; `parser/defusage.go` parsePerformedActionReference | `parser/testdata/parse/state_subaction_reference.golden`, `parser/state_subaction_test.go`, `parser/negative_test.go:entry_reference_no_semicolon`, `resolve/state_subaction_test.go`, `runtime/state_behavior_test.go:TestStateSubactionByReferencePerformsAction` | ✅ Faithful |
| State do behavior runs while its state is active, one action per round | `state_executor.go` startDoActivity, runDoRound | `state_do_behavior.sysml`, `state_do_activity_test.go` | ✅ Faithful |
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
| Transition effect actions | `state_executor.go:535` fireTransition | `state_transition_effect.sysml` | ✅ Faithful |
| AcceptEvent triggers (when signal) | `state_executor.go` matchesEvent | `state_signal_discriminate.sysml` | ✅ Faithful |
| Signals sent from state behaviors reaching the machine | `state_executor.go` executeAction send case, deliverPendingSignal | `state_send_self_signal.sysml` + trace golden, `signal_test.go:TestSendOfNamedTypeReachesStateMachine` | ✅ Faithful |
| CallEvent triggers (`accept op(param)` notation, operation and argument matching, arguments bound for guard/effect) | `parser/behavior.go` parseTriggerEvent/parseCallEvent; `symbols/bodyscopes.go` newCallTriggerScope (parameters visible to the transition's own guard/effect); `state_executor.go` matchesEvent EventCall case, bindTriggerArguments, InvokeOperation | `parser/testdata/parse/state_call_trigger.golden`, `lower/trigger_test.go:TestTriggerClassification_CallTrigger`, `model/behavior_body_resolve_test.go` call-trigger parameter cases, `state_call_trigger{,_guard,_nested,_regions}.sysml` conformance, `signal_test.go:TestCallEventMatchesOperationName`, `:TestRejectedCallLeavesNoArgumentsBehind`, `robustness_test.go:call_of_unhandled_operation`, `:call_argument_of_wrong_type` | ✅ Faithful (a trigger on an enclosing composite state does not see events while a substate is active — the same limitation as every other trigger kind) |
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
| A transition's guard, effect, time-event duration and change-event condition resolve in the scope the transition was written in; a call trigger's parameters are visible to its guard and effect and nowhere else | `lower/state_graph.go` `Transition.Scope`/`BodyScope` (via `symbols.CallTriggerScope`); `runtime/state_executor.go` `passesGuard`, `scheduleTransitionsForState`, `pollChangeEvents`, `executeAction`; `runtime/state_region_transition.go` `runEffect` | `state_body_quantity_scope.sysml`, `state_call_trigger_guard.sysml`, `state_call_trigger_regions.sysml` | ✅ Faithful |
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
| `SequenceFunctions` beyond `size`/`isEmpty`/`includes` | Needs the sequence semantics of the library's own function bodies, not just element access. |
| `ComplexFunctions` | Needs a complex value kind. |
| Remainder (`%`) outside constant folding | `semantics/eval.go` folds it over literals, but `runtime/eval.go` `evalOperator` routes only `+ - * / **` to arithmetic, so `%` over a feature reports `unsupported operator`. |
| Library functions in the checker's own name resolution | An unqualified call to a library function the model does not import evaluates, but the `unresolved-reference` diagnostic still reports the name; importing `RealFunctions::*` clears it. |

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

### Name Resolution

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| Inherited feature resolution | `document.go:199` resolveRedefinition | `flow_payload_test.go` | ✅ Faithful |
| Declaration named with a keyword (`action flow { ... }`, `attribute item : Integer`) | `parser/defusage.go` `atKindPrefix`, `atSecondaryKind` | `parser/namespace_keywords_test.go` `TestParseKeywordAsNameAfterKindKeyword`, `model/behavior_body_resolve_test.go` `TestKeywordNamedDeclarationIsReferenceable` | ✅ Faithful (the name is kept and is referenceable) |
| Keywords reserved in name position (only an unrestricted name may spell one) | `parser/namespace.go` `parseIdentification` → `Parser.Warnings`, surfaced as `passes.SeverityWarning` code `reserved-keyword-name` by `model/workspace.go` | `parser/namespace_keywords_test.go` `TestParseKeywordAsNameIsReported`, `model/behavior_body_resolve_test.go` `TestReservedKeywordNameWarning`, `libs/reserved_keyword_name_test.go` `TestStdlibReservedKeywordNames` | ⚠️ Approximate (reported as a warning, not an error, because the normative OMG library itself uses unquoted keyword names — `step entry[1];`, `part done : Part;`, `attribute type : String[0..1];` — and must keep parsing clean; those ten sites are pinned by the libs test so the set cannot grow silently) |
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
| Requirement actor declaration | `symbols/builder.go` ActorMember | `TestBehaviorDeclarationsAreVisible/requirement_actor_binding` | ✅ Faithful |
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
| A private `import X::*` is not re-exported by its namespace, and the names it brings in are visible only within that namespace (KerML 8.2.3.3) | `symbols/index.go` `applyReexportMarks` / `exportedChildren` (a re-export is hidden while every document that surfaced it did so with a private import, and left out when that namespace is itself wildcard-imported) and `LookupQualifiedFrom` (a hidden name answers a lookup only when the referring namespace is the one that hid it, or is nested in it); `resolve/qualified.go` `referringNamespaceFQN` supplies that context for a qualified reference and `HiddenFrom` stops its member-lookup fallback, which reaches a cached symbol's children without consulting the marks, from resurfacing a hidden name; `resolve/alias.go` `resolveCachedAliasTarget` supplies the context for the target of a cached alias | `symbols/index_test.go` `TestExpandWildcardImportsDoesNotCarryOnAPrivateImport`, `TestLookupQualifiedFromSeesAPrivateImportOnlyFromWithin`, `TestHiddenFromReportsOnlyPrivatelySurfacedNames`, `TestLookupQualifiedReachesAPubliclyImportedName`, `TestLookupQualifiedAcrossAChainedPrivateImport`; `resolve/qualified_test.go` `TestResolveQualifiedRejectsAPrivatelyImportedName`, `TestResolveQualifiedRejectsAPrivatelyImportedNameThroughMemberLookup`, `TestResolveQualifiedFromInsideAnUnnamedElement`, `TestResolveQualifiedReachesAPubliclyImportedName`; `resolve/alias_test.go` `TestAliasResolvesAPrivatelyImportedTargetFromCache`, `TestAliasResolvesAPrivatelyImportedTargetWhenParsed` | ⚠️ Approximate (a qualified reference no longer reaches a privately imported name from outside the importing namespace, and an alias declared inside it still does; what remains is the *unqualified* route — `resolve/unqualified.go` `matchImport` enumerates a wildcard import's target through `symbols/index.go` `LookupDirectChildren`, which does not consult the hidden marks, so `package App { import Mid::*; }` still sees what `Mid` imported privately) |
| Visibility of the members a recursive import surfaces (`import X::**`, KerML 7.2.5) | `resolve/unqualified.go` `matchImport` (both the membership and namespace branches filter through `resolve/visibility.go` `visibleThroughImport`) | `resolve/visibility_test.go` `TestRecursiveMembershipImportSkipsPrivate`, `TestNamespaceImportSkipsPrivate`, `TestImportAllReExportsPrivate` | ✅ Faithful (a recursive membership import hides private members of the subtree it walks unless it is `import all`) |
| `expose` in a view body is an Import (SysML v2 8.3.26.2 Expose, 8.3.26.3 MembershipExpose, 8.3.26.4 NamespaceExpose) | `parser/defusage.go` (`expose` shares `parser/namespace.go` `parseImportTail`, so `::*` yields a NamespaceExpose and `::**` a recursive MembershipExpose; `ast.Import.IsExpose`, `IsAll`, protected `Visibility`) | `parser/expose_test.go` `TestParseExposeImportKind`, `TestParseExposeIsImportAllAndProtected`, `resolve/expose_test.go` | ⚠️ Approximate (an Expose always imports all elements regardless of visibility — `validateExposeIsImportAll` — so its exposed elements resolve inside the view body and, being protected, not outside the view; the extra reach protected visibility gives an import — visibility in specializations of the importing definition or usage — is not modelled for any import, expose included) |
| Protected import visible in specializations of the importing definition or usage (SysML v2 7.5.3) | — | — | ❌ Not Yet Implemented (a protected import, and therefore an `expose`, is treated as private: its members resolve in the owning body only, not in bodies that specialize it) |
| `validateExposeOwningNamespace` — the importOwningNamespace of an Expose must be a ViewUsage | — | — | ❌ Not Yet Implemented (an `expose` in a non-view definition or usage body is parsed and resolved rather than diagnosed) |

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
| ParseFile | service.go:39-123 (parser + passes.Analyze + stdlib load) | ✅ Faithful | runtime_test.go:TestParseFile_*, every conformance case |
| GetSymbol | service.go:126-145 | ✅ Faithful | service_test.go:TestGetSymbol_* |
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

**Python bindings:**
- connection.py:488 - PID ownership check uses substring match - spoofable
- an `instance_id` outside an `Instantiate` response (an `Evaluate` result, say)
  is still a bare int64: those responses carry no instance graph to resolve it
- __init__.py:11-16 - Shadows builtins (RuntimeError, eval)
- binary.py:82,89 - Checksum same-origin (no pinned hash)

**Go gRPC layer:**
- convert.go:40 - SymbolToProto.Attributes always empty (semantic layer not ready)
- runtime instances are request-local, so an id is resolvable only against the
  response that carried it; there is no RPC that fetches an instance by id later

These are documented for transparency; none block production use.
