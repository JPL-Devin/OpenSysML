# Validation Depth C — Full Constraint Parity — Design

**Date:** 2026-07-30
**Builds on:** the pluggable pass registry (`passes/registry.go`), the existing `SyntaxPass` (depth A syntax), `NameResolutionPass` (depth A name-res), and `TypeCheckPass` (partial depth B: kind-compatibility of specialize/subset/redefine/type edges). This is roadmap item #2 in `2026-07-25-sysml-v2-go-design.md` §15 — **required before the project is considered finished.**

## 1. Goal

Reach **constraint parity** with the pilot: port the SysML v2 well-formedness rules (validation depth C) so the language server reports the same structural/semantic errors a conformant tool would. The pilot expresses these as ~**115 `@Check` methods** — **42 in `KerMLValidator`** (foundational: specialization, feature typing, multiplicity, expressions, connectors) and **73 in `SysMLValidator`** (per-kind: definitions/usages, ports, connections, actions, states, requirements, cases, views). This design defines the **supporting semantic infrastructure** these checks need and a **phased port** of the checks themselves.

## 2. Scope

### In scope
- The semantic infrastructure the constraint set depends on: a **specialization/typing graph**, **inherited-member resolution**, **multiplicity extraction**, and a bounded **model-level expression evaluator**.
- A faithful port of the pilot's structural constraints (KerML + SysML `@Check`s) as pass-registry entries, grouped into rule families.
- Diagnostics with stable codes mirroring the pilot's message constants, so parity is testable against pilot examples.

### Deferred / out of scope
- OCL-level completeness for rules the pilot itself marks `TODO` (e.g. `validateMultiplicityBoundResults` from KERML-199, `SYSML2-783`).
- Full type-inference of expression result types beyond what bound/guard checks require (a targeted evaluator, not a general type system).
- Anything requiring the standard-library semantic model beyond name resolution (handled where `libs` already provides the stdlib index).

### Constraints discovered (from the pilot sources)
- Checks are overwhelmingly **structural**: they walk owned relationships/memberships and compare kinds, ends, multiplicities, and specialization reachability. Only a minority (`checkMultiplicityRange`, guard/bound checks) need expression evaluation.
- Many SysML checks assert **end counts** (e.g. connection has 2 ends, interface ends conform), **required members** (e.g. a `SubjectMembership` count ≤ 1), or **feature-kind of a referenced member** (e.g. `assert` targets a constraint) — these need §3.2 inherited-member resolution but **no** evaluator.
- `checkSpecialization` (specific-not-conjugated), `checkSubsetting` (multiplicity conformance), `checkRedefinition` — the foundational KerML trio — require the specialization graph (§3.1) and multiplicity (§3.3).

## 3. Semantic infrastructure (new)

This is the bulk of the cycle's engineering; the checks are thin once it exists. New package `internal/core/semantics/` (peer of `resolve`), holding side tables keyed by node, consistent with the design's "semantic info in side tables" rule.

### 3.1 Specialization / typing graph
Build a directed graph over declarations from the `specializes`/`subsets`/`redefines`/`:`(typing)/`references`/`crosses` relationships (already parsed and resolved by `resolve`). Provide:
- `Supertypes(sym) []*Symbol` (direct) and `AllSupertypes(sym)` (transitive, memoized).
- `Conforms(a, b) bool` — `a` specializes/redefines/subsets-reaches `b`.
- **Cycle detection** — feeds the `specialization cycle` family (§4.2).
Backed by a memoized side table; invalidated per-document on reparse like other resolve caches.

### 3.2 Inherited-member resolution
Extend scope lookup to include **inherited members** via the §3.1 graph: `MembersOf(sym)` = local members ∪ inherited (respecting redefinition/masking). Needed by the many SysML checks that inspect ends/subjects/objectives that may be inherited. Exposed as a resolve-layer helper so completion/hover can reuse it later.

### 3.3 Multiplicity extraction
`MultiplicityOf(feature) (lower, upper Bound, ok)` reading the parsed `ast.Multiplicity`. Bounds may be literals (common) or expressions; literal bounds resolve directly, expression bounds route through §3.4. Feeds subsetting multiplicity conformance and multiplicity-range validity.

### 3.4 Model-level expression evaluator (bounded)
A small evaluator over the KerML expression AST for the **model-level-evaluable** subset the pilot's checks use: integer/boolean/real literals, the arithmetic/relational/boolean operators, and `*` (infinity) for bounds. Returns "not evaluable" for anything outside the subset (checks then skip, matching pilot behavior). Explicitly **not** a general interpreter.

## 4. Rule families (the ported checks)

Grouped so each is an independently landable pass or check-set. Counts are approximate mappings to pilot `@Check`s.

### 4.1 Feature typing & specialization conformance (KerML core)
`checkSpecialization`, `checkSubsetting`, `checkCrossSubsetting`, `checkRedefinition`, `checkFeature`, `checkFeatureChaining`, `checkType`, `checkClassifier`. Includes specific-not-conjugated, subsetting multiplicity conformance, redefinition validity. Depends on §3.1, §3.3.

### 4.2 Specialization cycles & multiplicity
`checkMultiplicityRange` (bound result types, lower ≤ upper), and specialization-cycle detection. Depends on §3.1, §3.3, §3.4.

### 4.3 Connectors, associations, flows, bindings (KerML)
`checkConnector`, `checkAssociation`, `checkBindingConnector`, `checkFlow`, `checkFlowEnd`, `checkImplicitBindingConnectors`, `checkEndFeatureMembership` — end counts and end-feature well-formedness. Depends on §3.2.

### 4.4 Expressions & functions (KerML)
`checkExpression`, `checkInvocationExpression`, `checkOperatorExpression`, `checkFeatureReferenceExpression`, `checkFeatureChainExpression`, `checkIndexExpression`, `checkSelectExpression`, `checkCollectExpression`, `checkInstantiationExpression`, `checkConstructionExpression`, `checkReturnParameterMembership`, `checkResultExpressionMembership`, `checkParameterMembership`, `checkFunction`. Structural arity/return checks.

### 4.5 Definition/usage general (SysML)
`checkDefinition`, `checkUsage`, `checkReferenceUsage`, `checkVariantMembership`, attribute/enumeration/occurrence checks. Reference-usage constraints, variation/variant rules.

### 4.6 Ports, connections, interfaces, allocations (SysML)
`checkPortDefinition`, `checkConjugatedPortDefinition`, `checkPortUsage`, `checkConnectionUsage`, `checkInterfaceDefinitionEnds`, `checkInterfaceUsageEnds`, `checkInterfaceUsage`, `checkAllocationUsage`, flow def/usage. Connector-end conformance for Tier B kinds (finishing what the Remaining-Kinds cycle deferred: *kind-checking* connector/flow ends).

### 4.7 Behavioral: actions & states (SysML) — **depends on Tier C grammar**
`checkActionUsage`, `checkAcceptActionUsage`, `checkSendActionUsage`, `checkAssignmentActionUsage`, `checkIfActionUsage`, `checkForLoopActionUsage`, `checkWhileLoopActionUsage`, `checkControlNode`/`Decision`/`Fork`/`Join`/`Merge`, `checkPerformActionUsage`, `checkStateDefinition`, `checkStateUsage`, `checkStateSubactionMembership`, `checkTransitionUsage`, `checkTransitionFeatureMembership`, `checkExhibitStateUsage`, `checkSuccession`, `checkTriggerInvocationExpression`. **Requires the Tier C Behavioral Grammar phases C3/C4 to have landed.**

### 4.8 Requirements, cases, constraints (SysML) — **depends on Tier C grammar**
`checkConstraintUsage`, `checkAssertConstraintUsage`, `checkRequirementDefinition`, `checkRequirementUsage`, `checkSubjectMembership`, `checkRequirementConstraintMembership`, `checkFramedConcernUsage`, `checkActorMembership`, `checkStakeholderMembership`, `checkSatisfyRequirementUsage`, `checkCaseDefinition`, `checkCaseUsage`, `checkObjectiveMembership`, `checkAnalysisCaseUsage`, `checkVerificationCaseUsage`, `checkRequirementVerificationMembership`, `checkUseCaseUsage`, `checkIncludeUseCaseUsage`. **Requires Tier C phases C1/C2.**

### 4.9 Views, viewpoints, renderings, metadata (SysML)
`checkViewDefinition`, `checkViewUsage`, `checkViewpointUsage`, `checkRenderingUsage`, `checkViewRenderingMembership`, `checkExpose`, `checkMetadataUsage`. Structural.

## 5. Pass architecture

- Each rule family is registered as one `Pass` (or a small set) at **`LevelConstraint`** — a new level strictly above `LevelType`, so constraint checks run only when name-res and type-check are clean (the registry already skips higher levels after an error at a lower one).
- Checks share the §3 infrastructure via `ctx` (extend `passes.Context` with a lazily-built `*semantics.Model`).
- Diagnostics use `Source: "constraint"` and stable `Code`s named after the pilot constants (e.g. `INVALID_SPECIALIZATION_SPECIFIC_NOT_CONJUGATED`) to make parity assertions precise.
- `DefaultRegistry()` gains the new passes; ordering by `Level()` is automatic.

## 6. Phasing (dependency-ordered)

1. **V-C1 — Semantic infrastructure** (§3): specialization graph, inherited members, multiplicity, bounded evaluator. Ships with unit tests; no user-visible diagnostics yet.
2. **V-C2 — KerML core conformance** (§4.1, §4.2, §4.4): the foundational specialization/typing/multiplicity/expression rules. Highest value, no Tier C dependency.
3. **V-C3 — Connectors & structural SysML** (§4.3, §4.5, §4.6): connectors, ports, interfaces, allocations; completes connector/flow end kind-checking deferred earlier.
4. **V-C4 — Behavioral & requirements** (§4.7, §4.8): **gated on Tier C grammar**. Land after the relevant Tier C phases.
5. **V-C5 — Views & remainder** (§4.9): views/viewpoints/renderings/metadata; sweep any remaining pilot checks.

## 7. Testing & parity

- **Per-family unit tests** in `passes/` using the existing `typeDiags`-style helper generalized to filter `Source: "constraint"`.
- **Parity fixtures:** curate minimal `.sysml` snippets from the pilot's own validation test suite (`SysML-v2-Pilot-Implementation/.../tests`) that each trigger exactly one rule; assert code + span. This is the objective "parity" measure.
- **Negative/positive pairs** for every ported check (violation → diagnostic; conformant → clean).
- Regression: full `go test ./...`, `go vet`, and race detector on workspace concurrency stay green each phase.

## 8. Change checklist (per phase)
1. `internal/core/semantics/` — infrastructure (V-C1) or new check-set files (later phases).
2. `passes/registry.go` — register new passes; add `LevelConstraint`.
3. `passes/pass.go` / `context.go` — expose `*semantics.Model` on `Context`.
4. `passes/diagnostic.go` — constraint codes.
5. Tests in `passes/` + parity fixtures under `testdata/`.

## 9. Sequencing note vs. Tier C grammar
V-C1→V-C3 have **no** dependency on Tier C behavioral grammar and can proceed immediately. V-C4 is **gated** on the corresponding Tier C phases (C1/C2 for requirements/cases/constraints; C3/C4 for actions/states). Recommended interleave: land **V-C1 + V-C2** first (foundational, unblocks the most parity), then Tier C grammar phases, then V-C3/V-C4/V-C5 as their grammar dependencies arrive.
