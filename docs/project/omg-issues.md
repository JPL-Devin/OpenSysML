# Bugs in the OMG materials

One place to look for defects found in the OMG-published sources this
implementation consumes. This page records defects in the **vendored specification
libraries** (`internal/core/libs/stdlib/`), in the **published example corpora**, and
in the **OMG pilot implementation** the differential is measured against.

Each row quotes the vendored declaration verbatim so a reviewer can judge it
without opening the library, and names what OpenSysML implements instead. Every
divergence is also a row in [spec-compliance.md](spec-compliance.md).

| Library file | Declaration | What the vendored body says | What we implement | Why |
|---|---|---|---|---|
| `Kernel Libraries/Kernel Function Library/NaturalFunctions.kerml` | `function '/'` | `function '/' specializes IntegerFunctions::'/' { in x: Natural[1]; in y: Natural[1]; return : Natural[1]; }` — a Natural quotient, which `7 / 2` cannot inhabit without truncation | the quotient of two whole numbers is a Rational, never normalised back to a whole number even when exact: `divisionResult` types `Natural/Natural` as `Rational`, and the runtime answers a Real (`runtime/eval.go` `evalArithmetic`) | The pilot's evaluator answers `LiteralRational 2.5` for `5 / 2` even when both operands are `Natural`-typed attributes — it dispatches on value kind, and a whole-number value divides through `RationalFunctions::'/'`, which `IntegerFunctions::'/'` specializes with `return : Rational[1]`. The declared `Natural[1]` return is unimplementable without truncating, which the reference does not do; the draft below asks which of the two the specification intends |
| `Domain Libraries/Quantities and Units/VectorCalculations.sysml` | `calc def inner`, `calc def norm` | `calc def inner :> VectorFunctions::inner { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : Number[1]; }` and `calc def norm :> VectorFunctions::norm { in : VectorQuantityValue[1]; return : Number[1]; }` — a bare `Number`, where `QuantityCalculations::'*'`, `'/'` and `sqrt` over scalar quantities return `ScalarQuantityValue[1]`, so the norm of a length vector has no unit while the square root of a length squared keeps one | the declaration: `inner`, `norm` and `angle` of a vector quantity answer the `Number` computed over the vector's `num` components (`norm(⟨3.0, 4.0⟩ [m])` is `5.0`, `inner` is `25.0`), never a quantity, and a `Number` feature takes them; the unit is dropped by declaration (`runtime/vector_functions.go` `vectorInner`, `vectorNorm`) | The checker already types the calls by their declared `Number` return, so a runtime answering a quantity would disagree with it; the pinned pilot evaluates neither, so there is no reference answer to follow. Recorded as a library inconsistency for review, not as a defect OpenSysML corrects |
| `Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml` | `function includingAt` | `(seq->subsequence(1, index - 1), values, seq->subsequence(index + 1))` — the prefix before `index`, then the values, then the tail from `index + 1`, so the element **at** `index` is dropped from the result | insertion: the values are inserted before the 1-based `index`, the tail from that position shifts right, and the result is longer than `seq` by the values inserted. `index == size + 1` appends; any other index outside `1..size + 1` is `ErrIndexOutOfRange` (`runtime.builtinSequenceIncludingAt`) | The body contradicts the declarations around it in the same file. `excludingAt` is the operation that removes at an index, and the behavior pairs are additive/subtractive: `add` calls `including` as `remove` calls `excluding`, and `addAt` calls `includingAt` (`seq->includingAt(values, index)`) as `removeAt` calls `excludingAt`. A removing `includingAt` would leave the library with two ways to delete at an index and none to insert at one, and would make `addAt` remove. The vendored expression is an off-by-one slip in the tail: the insertion body is `(seq->subsequence(1, index - 1), values, seq->subsequence(index))` |

## `includingAt` — the vendored declaration

Quoted verbatim from
`internal/core/libs/stdlib/Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml`:

```kerml
function includingAt{ in seq: Anything[0..*] ordered nonunique; in values: Anything[0..*] ordered nonunique;
    in index: Positive[1];
    return : Anything[0..*] ordered nonunique =
        (seq->subsequence(1, index - 1), values, seq->subsequence(index + 1));
}
```

`subsequence(1, index - 1)` is the prefix ending before `index`, and
`subsequence(index + 1)` is the tail starting after `index`; the element at
`index` appears in neither, so evaluating the body as written *replaces* it with
`values` rather than inserting before it. OpenSysML implements insertion
(maintainer ruling), so `includingAt` is a divergence from the
vendored body and is recorded here for review against a future OMG release.

## `NaturalFunctions::'/'` — the declared Natural return against the pilot's Rational answer

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Kernel Libraries/Kernel Function Library/NaturalFunctions.kerml`:

```kerml
function '/' specializes IntegerFunctions::'/' { in x: Natural[1]; in y: Natural[1]; return : Natural[1]; }
```

````markdown
**Question, not a bug report:** `NaturalFunctions::'/'` declares
`return : Natural[1]`, but the pinned pilot implementation (`2026-05`,
`jupyter-sysml-kernel` 0.60.1) evaluates `5 / 2` to `LiteralRational 2.5` even
when both operands are `Natural`-typed attributes
(`attribute a : ScalarValues::Natural = 5; attribute b : ScalarValues::Natural = 2;`).
Its evaluator dispatches on the value's kind rather than the declared type, so
the division runs through `RationalFunctions::'/'` — the function
`IntegerFunctions::'/'` specializes with `return : Rational[1]` — and no
truncating Natural division is observable. A `Natural[1]` return would require
the quotient to be truncated or the call to be rejected, and the pilot does
neither. Is the declared return type intended to be `Rational[1]` (matching
`IntegerFunctions::'/'`), or is a conforming evaluator expected to truncate?
````

OpenSysML follows the pilot's observed behavior for the operator: the type
checker (`passes/typecheck_expr.go` `divisionResult`) types `Natural/Natural`
division as `Rational`, and the runtime answers a Real, so a non-whole quotient
bound to a `Natural`-typed feature is reported rather than truncated. The
function called by name, `NaturalFunctions::'/'(x, y)`, is the one place the
declaration itself is the contract: it returns the Natural quotient when `y`
divides `x` and reports a non-whole quotient (`ErrArithmeticDomain`) rather
than truncating or answering a Rational (`runtime/library_operators.go`
`naturalDivision`).

---

## `VectorCalculations::inner`/`norm` — a `Number` where the scalar calculations return a quantity

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Domain Libraries/Quantities and Units/VectorCalculations.sysml`:

```sysml
	calc def inner :> VectorFunctions::inner { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : Number[1]; }
	calc def norm :> VectorFunctions::norm { in : VectorQuantityValue[1]; return : Number[1]; }
	calc def angle :> VectorFunctions::angle { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : Number[1]; }
```

and from `QuantityCalculations.sysml` in the same directory:

```sysml
	calc def '*' specializes NumericalFunctions::'*' { in x: ScalarQuantityValue[1]; in y: ScalarQuantityValue[1]; return : ScalarQuantityValue[1]; }
	calc def '/' specializes NumericalFunctions::'/' { in x: ScalarQuantityValue[1]; in y: ScalarQuantityValue[1]; return : ScalarQuantityValue[1]; }
	calc def sqrt{ in x: ScalarQuantityValue[1]; return : ScalarQuantityValue[1]; }
```

````markdown
**Library inconsistency, question rather than bug report:** the scalar
quantity calculations keep the quantity through an operation —
`QuantityCalculations::'*'`, `'/'` and `sqrt` all declare
`return : ScalarQuantityValue[1]`, so `sqrt(q * q)` of a length `q` is a
length — but the vector quantity calculations drop it: `VectorCalculations::inner`
and `norm` declare `return : Number[1]` over `VectorQuantityValue` operands.
The norm of a length vector is therefore a bare number, and the inner product of
two length vectors a bare number too, although each is a quantity of the operands'
unit (or its square) in the same way `q * q` is. Only `angle` is naturally
dimensionless. Is `Number[1]` the intended return for `inner` and `norm`, with
the unit understood to be implied by the operands, or should they return
`ScalarQuantityValue[1]` as the scalar calculations do? (A redefinition of
`VectorFunctions::inner`/`norm`, whose returns are `Number`, could not narrow
to `ScalarQuantityValue` since that is not a `Number`; a resolution would need
the vector calculations declared independently of `VectorFunctions`, as
`QuantityCalculations::'*'` is of `NumericalFunctions::'*'` only by
specialization.)
````

The pinned pilot implementation (`2026-07`, `jupyter-sysml-kernel` 0.61.0)
evaluates none of these: asked through `build/pilot-evaluator/eval-sysml` for
`VectorFunctions::norm(CartesianVectorOf((3.0, 4.0)))`,
`VectorCalculations::norm(...)`, a `Number` attribute bound to the former,
`QuantityCalculations::sqrt(q * q)` and `q * q` over `q : ScalarQuantityValue = 2 [m]`,
it answers the unevaluated `InvocationExpression norm`, `InvocationExpression sqrt`
and `OperatorExpression *` for every case, so it offers no reference for either
reading.

OpenSysML follows the declarations: the checker types `inner`, `norm` and `angle`
of vector quantities as `Number` (a `ScalarQuantityValue` feature bound to one is a
static error, a `Number` feature is accepted), and the runtime answers the bare
number computed over the vector's `num` components — `norm(⟨3.0, 4.0⟩ [m])` is
`5.0`, `inner(⟨1.0, 2.0⟩ [m], ⟨3.0, 4.0⟩ [m])` is `11.0` — the unit dropped by
declaration (`runtime/vector_functions.go`; conformance
`calc_library_vector_quantity_norm`). The scalar calculations keep their quantity
results as declared (`runtime/quantity_functions.go`).

---

## Defects in published OMG example models

These rows are true positives found by OpenSysML's static analysis in models published with the
OMG example corpora. The pinned pilot is silent on each because it does not perform the
corresponding check.

| Example | Expression as published | Defect | Specification reading | Status |
|---|---|---|---|---|
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml:38` | `22/2*25.4 + 110 [mm]` | `[mm]` binds only to `110`, so `+` combines a dimensionless value with a length | KerML 1.0 §8.2.5.8.1–8.2.5.8.2 makes the bracket construction a primary expression; SysML v2.0 §9.8.9.1 requires addition operands to have the same quantity dimension. The evident intended spelling is `(22/2*25.4 + 110) [mm]`. | **not filed** |
| `Analysis Examples/Turbojet Stage Analysis.sysml:25` | `1/(2 * Cp) * V^2 + T_static`, with `Cp : DimensionOneValue`, `V : VolumeValue`, and `T_static : TemperatureValue` | the declared types make the operands L^6 and Θ | SysML v2.0 §9.8.9.1 requires addition operands to have the same quantity dimension and top-level quantity type. The formula needs dimensionally appropriate parameter declarations or conversion before addition. | **not filed** |
| `Analysis Examples/Dynamics.sysml:13` | `return a : AccelerationValue = tp * dt * tp;`, with `tp : PowerValue` and `dt : TimeValue` | a power squared times a duration has dimension L^4·M^2·T^-5, not the L·T^-2 an `AccelerationValue` is measured in | KerML 7.4.9 makes the expression the return feature's value, so it answers to that feature's type, and the ISQ definitions the file imports fix the dimensions: `AccelerationUnit` is L^1·T^-2, `PowerUnit` is L^2·M^1·T^-3, and `TimeValue` aliases `DurationValue` (T^1). No grouping of the published product is an acceleration. | **not filed** |
| `Individuals Examples/AnalysisIndividualExample.sysml:86` | `individual action :>> fuelConsumption : FuelEconomyAnalysis_1` | the redefining action is typed by the enclosing analysis definition, which does not conform to the redefined feature's `FuelConsumption` | KerML 7.4.9 and 8.3.4.2 make a redefinition a subsetting, so the redefining feature's type must conform to the redefined one's. The file declares `individual action def FuelConsumption_1 :> FuelConsumption` and never uses it: that is the intended type. | **fixed upstream at `2026-07`** — the corpus now publishes `fuelConsumption : FuelConsumption_1`, the type this row named, so the row is closed and its overlay entry retired |

The first two rows form one adjudicated quantity-commensurability family in
[the adjudications record](adjudications.md); the third is the same physics read through a
declared type rather than through an addition, so it is an error and not a warning; and the fourth
was the corpus instance of the subsetting-conformance divergence adjudicated in
[the same record](adjudications.md), which the `2026-07` corpus corrects on its own.
OpenSysML retains the three open diagnostics, and nothing has been posted upstream. All three are
entries of the declared errata overlay — the geometry row with a correction, the turbojet and
dynamics rows without one — quoted verbatim with their derivations in
[the fourth section](#the-errata-overlay-entries-for-these-models).

---

## Defects in the pilot implementation

The first section records a defect in a vendored library body, and the second records defects in
published example models. This section records defects in the **OMG SysML v2 pilot implementation**
(`Systems-Modeling/SysML-v2-Pilot-Implementation`), which
[pilot-differential.md](pilot-differential.md) uses as the reference oracle. A
row lands here only when it is established from the pilot's own artifacts — its
grammar, its `.ecore`, or its loaded object graph probed through its own API —
and not from a disagreement alone.

| Component | Pinned version | Symptom | Adjudication | Status |
|---|---|---|---|---|
| `org.omg.sysml` — `Type::ownedDisjoining` setting delegate | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | every `disjoint from` clause in a type declaration draws EMF's `The opposite features 'owningType' … and 'ownedDisjoining' … do not refer to each other` | [one cause for all six corpus diagnostics](pilot-differential.md#k6-diagnostic-by-diagnostic-f33), reproduced in three lines and probed through the pilot's API | filed upstream as [Systems-Modeling/SysML-v2-Pilot-Implementation#790](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/790) **pending adjudication**, body below |
| `org.omg.sysml` — the `queryx/failing` Xpect fixtures | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | `QPE-Qualifier`, `QPE-Traversal` and `QPE-Wildcard` declare `XPECT noErrors`, yet the pinned validator rejects all three with `no viable alternative at input '/'`, `For input string: "."` and `no viable alternative at input '@'` | [adjudications.md](adjudications.md) — established by running the pinned pilot's own SysML validator on the three fixtures, not from a disagreement | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.xtext` — `checkTransitionFeatureMembership` (`validateTransitionFeatureMembershipGuardExpression`) | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | `TransitionUsage_invalid.sysml.xt` expects `Must be a Boolean expression.` at `if "test"`, yet the pinned validator with the full standard library accepts a `String` or arithmetic guard in the same shape | [pilot-rejection.md](pilot-rejection.md#constraints-the-pilot-declares-but-does-not-enforce) — established by running the pinned pilot's own SysML validator on the fixture's shape, not from a disagreement alone | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.xtext` — `SysMLValidator.checkControlNode`, `checkDecisionNode`, `checkForkNode`, `checkJoinNode`, `checkMergeNode` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a fork or decision node with two incoming successions, a join or merge node with two outgoing, and a succession end whose written multiplicity is not the one SysML v2 §7.17.3 requires all validate clean; only `validateControlNodeOwningType` is reported | established from the pilot's source: eight of the nine constraints are `// TODO: Check validate… (?)` comments in the check methods (`SysMLValidator.xtend:857–888` at `c7fc737`); the reproducers are `cmd/pilot-reject/testdata/negative/semantic/cn01`–`cn04`, `cn06`–`cn09`, run through the pinned batch validator | **not filed** — drafted below, awaiting maintainer authorisation |
| `org.omg.kerml.xtext` — `KerMLValidator.checkFeature`, the `validateFeatureOwnedCrossSubsetting` check | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a feature with two `crosses` clauses reports `Error executing EValidator` instead of `At most one cross subsetting is allowed`: the loop indexes `refSubsettings` (the reference subsettings, collected for the check above it) with the cross-subsetting index, and throws | established from the pinned `KerMLValidator.xtend` line 649 and reproduced with `cmd/pilot-reject/testdata/negative/semantic/k42-two-cross-subsettings.kerml`; the same file is byte-identical at upstream `master` `13c32ea2` (2026-09-01), so the defect is still present; [pilot-rejection.md](pilot-rejection.md#permissiveness-gaps) records the case as a gap of ours | filed upstream as [Systems-Modeling/SysML-v2-Pilot-Implementation#794](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/794) **pending adjudication**, body below |
| `org.omg.sysml.xtext` — `SysMLValidator`, invocation argument count | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | a positional invocation of a `calc def` with fewer arguments than the calc declares `in` parameters validates clean: `ln(m0 / mf)` against `calc <ln> naturalLogarithm { in x; in y; … }` and `calculateDeltaV(isp, initialMass, finalMass)` against a four-input `calc def calculateDeltaV` | established by running the pinned batch validator over the whole `airbus/apollo-11-sysml-v2` model at `6e9c93f` (`validate-sysml-batch --root . <every .sysml>`): no diagnostic, while OpenSysML reports the three `requires N argument(s), found M` errors [performance.md](../internals/performance.md#a-real-model-apollo-11) records — defects in that model, not in any OMG material, so they are not rows of this page | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.xtext` — `SysMLValidator.isDuration`/`isTime`, behind `validateTriggerInvocationActionAfterArgument` and `…AtArgument` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | with `d : DurationValue` and `t : TimeInstantValue`, `accept after d * d` and `accept at t * t` validate clean although the product has dimension T², while `accept after 10 [m] / 2 [m/s]`, whose quotient has dimension T, is refused | established from the pinned `SysMLValidator` class: an operator argument is a duration or an instant when its operator is one of `-`, `+`, `*`, `%`, `^`, `**` (`isQuantityOperator`) and every operand is itself one — `/` is not in the list and no dimension is computed; reproduced with the pinned batch validator, transcript below | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.interactive` — the expression evaluator over `OccurrenceFunctions` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | `OccurrenceFunctions::'==='(w1, w1)` evaluates to `false` while `w1 === w1` and `BaseFunctions::'==='(w1, w1)` evaluate to `true`; `isDuring(1)` and `isDuring("x")` evaluate to `true`; `create`, `destroy`, `addNew` and `addNewAt` answer their `occ` argument for any argument, an out-of-range `addNewAt` index included | established by evaluating the calls through the pinned pilot's own headless evaluator (`build/pilot-evaluator/eval-sysml --cases`, transcript below): the evaluator folds each declared body over the *declarations* (`x.portionOfLife == y.portionOfLife` over features no value has, `notEmpty(during)` over the function's own feature) rather than over occurrences, so its answers contradict its own operator | **not filed** — question drafted below, awaiting maintainer authorisation |

### `Type::ownedDisjoining` does not contain a `Disjoining` whose `owningType` is that `Type` (pilot `2026-05`)

Filed as
[Systems-Modeling/SysML-v2-Pilot-Implementation#790](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/790);
the body below is what was submitted, and the supporting analysis is
[the disjoining diagnostics, one by one](pilot-differential.md#k6-diagnostic-by-diagnostic-f33).

````markdown
### Every `disjoint from` clause in a type declaration reports an unpaired bidirectional reference

**Version:** `2026-05` (validated through `jupyter-sysml-kernel` 0.60.1, the KerML
standalone setup + `SysMLUtil`).

#### Minimal reproduction

`Decl.kerml`, complete — no imports, no library references:

```kerml
package Decl {
    classifier A;
    classifier B disjoint from A;
}
```

Validate it on its own, in a fresh resource set.

#### Expected

No diagnostics. `disjoint from` in a type declaration is
`DisjoiningPart` (`org.omg.sysml.xtext/src/org/omg/sysml/xtext/KerML.xtext:344`,
reached from `TypeRelationshipPart` at `:340`), and this is how the shipped
example models write it — six of the `.kerml` files under
`org.omg.sysml.examples`/`kerml-examples` use exactly this clause
(`Simple Tests/Types.kerml:31`, `Simple Tests/Classifiers.kerml:13`,
`Simple Tests/Features.kerml:20`, `Simple Tests/Inverses.kerml:3`,
`Simple Tests/FeatureChains.kerml:31`,
`KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9`).

#### Actual

One error per clause, on the clause's line:

```
The opposite features 'owningType' of 'org.omg.sysml.lang.sysml.impl.DisjoiningImpl{Simple Tests/Types.kerml#//@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.14/@ownedRelatedElement.0/@ownedRelationship.1}' and 'ownedDisjoining' of 'org.omg.sysml.lang.sysml.impl.TypeImpl{Simple Tests/Types.kerml#//@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.14/@ownedRelatedElement.0}' do not refer to each other
```

This is EMF's `_UI_UnpairedBidirectionalReference_diagnostic`, raised by
`EObjectValidator` over an `EReference` pair — not a `KerMLValidator` rule — so
it is a statement about the loaded object graph rather than about the model.
All six example files above report it; the parse itself succeeds, and the
standalone form `disjoint b.f.a from b.a;`
(`Simple Tests/FeatureChains.kerml:28`) does not report it. It is not a batching
artifact: each file reproduces the diagnostic when validated alone in a fresh
resource set.

#### Mechanism

`Disjoining::owningType` declares `eOpposite="#//Type/ownedDisjoining"` in
`org.omg.sysml/model/SysML.ecore`, and `Type::ownedDisjoining` is derived,
transient and volatile — its setting delegate selects the `Type`'s
`ownedRelationship`s that are `Disjoining`s whose `typeDisjoined` is that
`Type`. Probing the reproducer's loaded model through the pilot's own API gives:

```
Disjoining //@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.1/@ownedRelatedElement.0/@ownedRelationship.0
  owner                = ClassifierImpl(B)
  owningRelatedElement = ClassifierImpl(B)
  typeDisjoined        = ClassifierImpl(B)
  disjoiningType       = ClassifierImpl(A)
  owningType           = ClassifierImpl(B)
  owner.ownedDisjoining        = []
  owner.ownedRelationship size= 1
    rel DisjoiningImpl same=true
```

`B.ownedRelationship` contains the `Disjoining`, that `Disjoining`'s
`typeDisjoined` and `owningType` are both `B` — and yet the derived
`B.ownedDisjoining`, the other end of the `eOpposite` pair, is empty. So the
delegate does not return a `Disjoining` that satisfies its own documented
derivation, and EMF's check on the pair then fails for every `disjoint from`
clause written in a type declaration.

`OwnedDisjoining` (`KerML.xtext:437`) sets only `disjoiningType`; the owned form
leaves `typeDisjoined` to be the owning type, which the standalone `Disjoining`
production (`:426`) instead names explicitly — consistent with the standalone
form being unaffected.
````

#### Does a fix upstream clear the whole `kerml-examples` column?

Yes, and it was checked file by file rather than by family, since the answer
decides whether this root's pilot-only rows are ours to act on at all. Each of
the six files contributes **exactly one** pilot-only row, each row is the EMF
pair diagnostic above, and each line is a `disjoint from` clause written in a
type declaration — the form the mechanism section pins to `OwnedDisjoining`. The
per-clause table is in
[the row-by-row sweep of pilot-only diagnostics](pilot-differential.md#only-the-pilot--the-row-by-row-sweep-137). The clause appears on classifiers, plain types and features alike and the
reported EMF class tracks the declaration, so the defect is in the pair rather
than in one metaclass; the standalone form `disjoint b.f.a from b.a;`
(`Simple Tests/FeatureChains.kerml:28`) sits in the same file as one of the six
and reports nothing. No `kerml-examples` file carries a second pilot-only row of
any kind, so a fix to the derived `ownedDisjoining` delegate clears this root's
column entirely and silences nothing else it depends on.

---

### Are the `queryx/failing` query path expressions intended notation? (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The adjudication is in
[the parser-recovery decisions](adjudications.md).

````markdown
**Question, not a bug report:** three Xpect fixtures under
`sysml/src/org/omg/sysml/xpect/tests/queryx/failing/` declare file-wide silence
(`// XPECT noErrors ---> ""`) while the pinned release's own validator rejects
them. Is the notation planned for a later version, or are the fixtures kept as a
record of a proposal that the grammar deliberately does not admit?

The forms are

```sysml
value v1_i: Integer[0..*] = .*/.*[Integer];       // QPE-Qualifier
value v_redefining: Integer = ./vehicle_1/cylinders/@redefining;  // QPE-Traversal
value vw_recursive: Integer[0..*] = .**/cylinders;  // QPE-Wildcard
```

Running the release's SysML validator (`jupyter-sysml-kernel-0.60.1-all.jar`,
tag `2026-05`) over the three model bodies reports, among others:

```
QPE-Wildcard.sysml:9:38: error: For input string: "."
QPE-Wildcard.sysml:9:40: error: no viable alternative at input '/'
QPE-Traversal.sysml:7:57: error: no viable alternative at input '@'
QPE-Qualifier.sysml:9:40: error: no viable alternative at input '/'
```

Their `XPECT_SETUP` names `org.omg.sysml.xpect.tests.query.failing.SysMLQueryFailingTest`
while the runner in the directory is `org.omg.sysml.xpect.tests.queryx.failing.SysMLQueryFailingTest`
and extends `KerMLXtextTests`.

A second implementation reading the corpus cannot tell from the fixtures alone
whether the declared silence is an obligation or an aspiration, which is the
reason for asking rather than implementing.
````

### `validateFeatureOwnedCrossSubsetting` indexes the wrong list and throws (pilot `2026-07`)

Filed as
[Systems-Modeling/SysML-v2-Pilot-Implementation#794](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/794);
the body below is what was submitted. The reproduction is the rejection-corpus case
`cmd/pilot-reject/testdata/negative/semantic/k42-two-cross-subsettings.kerml`.
Checked against upstream `master` at `13c32ea26680323921c14e76755897fc551ec258`
(2026-09-01): `KerMLValidator.xtend` is byte-identical to the pinned `2026-07`
copy and the tag-to-master diff touches no validation or grammar source, so the
reproduction below stands for the current head as well as for the pin (no
master build was run — Maven Central was unreachable from the sandbox).

````markdown
### A feature with two `crosses` clauses reports `Error executing EValidator`

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, the KerML standalone
setup); the offending lines are unchanged on `master` at `13c32ea2`.

#### Minimal reproduction

```kerml
package K42TwoCrossSubsettings {
    class A {
        feature x : A;
        feature y : A;
    }
    assoc S {
        end a : A;
        end b : A crosses a.x crosses a.y;
    }
}
```

The grammar admits the second `crosses` (`FeatureSpecializationPart` repeats
`FeatureSpecialization`), and `validateFeatureOwnedCrossSubsetting` is meant to
report it as `At most one cross subsetting is allowed`. Instead the validator
reports

```
k42-two-cross-subsettings.kerml:0:0: error: Error executing EValidator
k42-two-cross-subsettings.kerml:9:39: error: The opposite features 'crossingFeature' of '...CrossSubsettingImpl{...@ownedRelationship.2}' and 'ownedCrossSubsetting' of '...FeatureImpl{...}' do not refer to each other
```

The second line is EMF's opposite-consistency check on the extra
`CrossSubsetting` (`Feature::ownedCrossSubsetting` is single-valued), not the
intended constraint message. Dropping the second clause (`end b : A crosses a.x;`)
makes the model validate clean, so the second `crosses` is the only defect.

#### Cause

In `KerMLValidator.checkFeature` (`KerMLValidator.xtend`, the
`validateFeatureOwnedCrossSubsetting` block):

```xtend
val crossSubsettings = f.ownedRelationship.filter[r | r instanceof CrossSubsetting].toList
if (crossSubsettings.size > 1) {
    for (var i = 1; i < crossSubsettings.size; i++)
        error(INVALID_FEATURE_OWNED_CROSS_SUBSETTING_MSG, refSubsettings.get(i), null, INVALID_FEATURE_OWNED_CROSS_SUBSETTING)
}
```

`refSubsettings.get(i)` reads the reference-subsetting list collected for the
`validateFeatureOwnedReferenceSubsetting` check just above; with no `references`
clause on the feature that list is empty and the `get(1)` throws, which Xtext
surfaces as `Error executing EValidator`. The intended target is
`crossSubsettings.get(i)`.
````

---

### A non-Boolean transition guard is accepted with the full library loaded (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The observation is recorded under [constraints the pilot
declares but does not enforce](pilot-rejection.md#constraints-the-pilot-declares-but-does-not-enforce).

````markdown
**Question, not a bug report:** `validateTransitionFeatureMembershipGuardExpression`
is implemented in `SysMLValidator.checkTransitionFeatureMembership` and
`validation/invalid/TransitionUsage_invalid.sysml.xt` expects its message:

```sysml
transition
    first S2_1
    // XPECT errors ---> "Must be a Boolean expression." at "if \"test\""
    if "test"
    then S2_2;
```

That fixture's `XPECT_SETUP` loads a reduced resource set (`Base`, `Occurrences`,
`Performances`, `States`, ... but not `ScalarValues`). Running the release's
SysML validator (`jupyter-sysml-kernel-0.61.0-all.jar`, tag `2026-07`) with the
full standard library over the same shape reports no error:

```sysml
package T2 {
    state def S2 {
        state S2_1;
        transition first S2_1 if "test" then S2_2;
        state S2_2;
    }
    state def S3 {
        state a;
        state b;
        transition t first a if 1 + 2 then b;
    }
}
```

Is the guard check intended to fire in a full-library workspace? A second
implementation that rejects `if "test"` with the full library loaded, as the
fixture suggests it should, currently disagrees with the release's validator on
the same text.
````

---

### A trigger's time arithmetic is judged by its operator, not its dimension (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML judges the argument by the dimension of its value
(`spec-compliance.md`, the `after`/`at` trigger rows), so the two implementations
disagree on the shapes below in both directions.

````markdown
**Question, not a bug report:** `SysMLValidator.isDuration` and `isTime`, which
`checkTriggerInvocationExpression` uses for `validateTriggerInvocationActionAfterArgument`
and `…AtArgument`, admit an `OperatorExpression` when its operator is one of
`-`, `+`, `*`, `%`, `^`, `**` and every operand is itself a duration or a time
instant. With the release's validator (`jupyter-sysml-kernel-0.61.0-all.jar`,
tag `2026-07`) and the full standard library:

```sysml
package T {
    private import ISQ::*;
    private import Time::*;
    private import SI::*;
    attribute d : DurationValue;
    attribute t : TimeInstantValue;
    state def S {
        state s1; state s2;
        transition first s1 accept after d * d then s2;             // no error: dimension T²
        transition first s1 accept at t * t then s2;                // no error: dimension T²
        transition first s1 accept after 10 [m] / 2 [m/s] then s2;  // An after expression must be a DurationValue: dimension T
    }
}
```

Is the operator list the intended reading of `checkTriggerInvocationExpressionAfterArgument`
(the argument's result conforms to `ISQ::DurationValue`)? A product of two durations
is not a duration, and a quotient of a length by a speed is one; a second
implementation that judges the value's dimension accepts the last line and
refuses the first two.
````

---

### Eight control-node succession constraints are unimplemented `TODO`s (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The rules are implemented on our side by
`internal/core/passes/control_node.go` and refereed against the specification
text; the adjudication is in
[pilot-differential.md](pilot-differential.md#control-node-successions-the-pilot-does-not-validate).

````markdown
### `SysMLValidator` does not check the succession constraints on control nodes

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-sysml-batch` over the
shipped standard library).

SysML v2 8.3.17 (`ControlNode`, `DecisionNode`, `ForkNode`, `JoinNode`, `MergeNode`)
declares nine validation constraints. `SysMLValidator.xtend` (`:857–888`) declares an
error code for each, but implements only `validateControlNodeOwningType`; the other
eight are `// TODO: Check validate… (?)` comments in otherwise empty `@Check` methods:
`validateControlNodeIncomingSuccessions`, `validateControlNodeOutgoingSuccessions`,
`validateDecisionNodeIncomingSuccessions`, `validateDecisionNodeOutgoingSuccessions`,
`validateForkNodeIncomingSuccessions`, `validateJoinNodeOutgoingSuccessions`,
`validateMergeNodeIncomingSuccessions`, `validateMergeNodeOutgoingSuccessions`.

#### Minimal reproduction

```sysml
package ForkTwoIncoming {
    action def A {
        action a;
        action b;
        action c;
        fork f;
        first a then f;
        first b then f;
        first f then c;
    }
}
```

`f` has two incoming successions; `validateForkNodeIncomingSuccessions`
(`targetConnector->selectByKind(Succession)->size() <= 1`, SysML v2 8.3.17 `ForkNode`) is
violated.

#### Expected

An error on `fork f`.

#### Actual

No diagnostics. The same holds for a join or merge node with two outgoing successions, a
decision node with two incoming ones, and for the end multiplicities — `succession s first
[0..1] a then [1] m;` into a merge is accepted where `validateMergeNodeIncomingSuccessions`
requires source multiplicity `0..1`, and `succession s first a then [0..1] f;` into a fork
is accepted where `validateControlNodeIncomingSuccessions` requires target multiplicity
`1..1`.

#### Note

The grammar admits every one of these models, and the specification says the rules "shall
be enforced in the abstract syntax, even if not shown explicitly in the concrete syntax
notation for a model" (7.17.3), so a validator is the only place they can be caught. One
reading question may be behind the `(?)` on the `TODO` lines: `multiplicityHasBounds`
requires `mult <> null`, and a connector end written without a multiplicity (`first a then
f;`) is given none by the pilot's `SuccessionAdapter`/`ConnectorAdapter`, so a literal evaluation of the four
multiplicity constraints would reject the specification's own examples. Treating an
unwritten end multiplicity as the required one, and checking only written ones, is what a
second implementation has to assume; a note in the release on the intended reading would
help.
````

---

### `OccurrenceFunctions` are evaluated over declarations rather than occurrences (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML's own semantics for the six functions are recorded in
[spec-compliance.md](spec-compliance.md) (the *Occurrences have a lifetime* rows).

The probe model and the pinned evaluator's verbatim answers
(`jupyter-sysml-kernel-0.61.0-all.jar`, tag `2026-07`, through
`build/pilot-evaluator/eval-sysml --cases`; the evaluator exited 0, every answer is
its own):

```sysml
package OccProbe {
    private import ScalarValues::*;
    private import OccurrenceFunctions::*;
    private import SequenceFunctions::*;
    part def Widget { attribute mass : Real = 1.0; }
    part w1 : Widget;
    part w2 : Widget;
    part group : Widget[0..*] ordered nonunique;
}
```

| Expression | Pilot answer |
|---|---|
| `OccProbe::w1 === OccProbe::w1` | `LiteralBoolean true` |
| `OccProbe::w1 === OccProbe::w2` | `LiteralBoolean false` |
| `OccProbe::w1 !== OccProbe::w2` | `LiteralBoolean true` |
| `BaseFunctions::'==='(OccProbe::w1, OccProbe::w1)` | `LiteralBoolean true` |
| `OccurrenceFunctions::'==='(OccProbe::w1, OccProbe::w1)` | `LiteralBoolean false` |
| `OccurrenceFunctions::'==='(OccProbe::w1, OccProbe::w2)` | `LiteralBoolean false` |
| `OccurrenceFunctions::'==='(null, null)` | `LiteralBoolean true` |
| `OccurrenceFunctions::isDuring(OccProbe::w1)` | `LiteralBoolean true` |
| `OccurrenceFunctions::isDuring(1)` | `LiteralBoolean true` |
| `OccurrenceFunctions::isDuring("x")` | `LiteralBoolean true` |
| `OccurrenceFunctions::create(OccProbe::w1)` | `PartUsage w1` |
| `OccurrenceFunctions::create(1)` | `LiteralInteger 1` |
| `OccurrenceFunctions::destroy(OccProbe::w1)` | `PartUsage w1` |
| `OccurrenceFunctions::destroy(null)` | (nothing) |
| `OccurrenceFunctions::addNew(OccProbe::group, OccProbe::w1)` | `PartUsage w1` |
| `OccurrenceFunctions::addNewAt(OccProbe::group, OccProbe::w1, 1)` | `PartUsage w1` |
| `OccurrenceFunctions::addNewAt(OccProbe::group, OccProbe::w1, 5)` | `PartUsage w1` |
| `OccurrenceFunctions::addNewAt((1, 2), 3, 9)` | `LiteralInteger 3` |
| `SequenceFunctions::includingAt((1, 2), 3, 5)` | `EXCEPTION:java.lang.IndexOutOfBoundsException: toIndex = 4` |

````markdown
**Question, not a bug report:** the interactive evaluator answers the
`OccurrenceFunctions` declarations by folding their declared bodies over the
model elements the arguments name, which gives answers that contradict its own
operators. With `part w1 : Widget;` in scope, `w1 === w1` and
`BaseFunctions::'==='(w1, w1)` are `true` but `OccurrenceFunctions::'==='(w1, w1)`
is `false` — the body `x.portionOfLife == y.portionOfLife` is evaluated over
features that hold no value. `isDuring(1)` and `isDuring("x")` are `true`: the
body `notEmpty(during)` is evaluated over the function's own `during` feature
rather than the argument's lifetime, so a data value that is no occurrence is
reported as happening during. `create`, `destroy`, `addNew` and `addNewAt` answer
their `occ` argument for any argument, an `addNewAt` index past the group's end
included, while `SequenceFunctions::includingAt` with the same index throws
`IndexOutOfBoundsException`. Is the evaluator intended to answer these six at all
outside an executing performance? If so, is `OccurrenceFunctions::'==='` intended
to agree with the `===` operator, and `isDuring` to reject an argument that is
not an `Occurrence`, as the declared parameter types say?
````

### A positional invocation with too few arguments validates clean (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream, to the pilot or to the model's repository.

````markdown
**Question, not a bug report:** is the number of positional arguments of an
`InvocationExpression` meant to be checked against the invoked behavior's `in`
parameters? KerML 7.4.9 binds the i-th positional argument to the i-th input
parameter of the invoked behavior, so a call with fewer arguments leaves an input
unbound, and one with more has an argument that binds nothing.

The release's SysML validator (`jupyter-sysml-kernel-0.60.1-all.jar`, tag
`2026-05`) over the public `airbus/apollo-11-sysml-v2` model (commit `6e9c93f`)
reports no diagnostic for either of these:

```sysml
calc <ln> naturalLogarithm { in x: DataValue[1]; in y: DataValue[1]; return : DataValue[1]; }

calc def calculateDeltaV {
    in isp :> specificImpulse;
    in g0 :> ISQ::acceleration;
    in m0 :> ISQ::mass;
    in mf :> ISQ::mass;
    return deltaV :> ISQ::speed = isp * g0 * ln(m0 / mf);
}

calc def calculateStageDeltaV {
    // …
    return deltaV :> ISQ::speed = calculateDeltaV(isp, initialMass, finalMass);
}
```

A second implementation reports `naturalLogarithm requires 2 argument(s), found 1`
and `calculateDeltaV requires 4 argument(s), found 3`, which the model's authors
would presumably want to hear about. Is silence here a deliberate reading of the
specification (an unbound input is legal and merely unvalued), or a check that has
not been implemented yet?
````

---

## The errata overlay entries for these models

The second section's rows are also entries of the declared errata overlay
(`internal/errata`, [the declared errata overlay](errata-overlay.md)): the published bytes on
disk are never edited, and a row that carries a correction has that correction
applied to the *second* figure every oracle reports, never to the headline one.
The overlay adds no category and reclassifies nothing — the two quantity rows stay the adjudicated
commensurability family recorded in the false-positive audit.

An entry is accepted only with a specification citation and a written
derivation, and only while its as-published text still matches the corpus on
disk; both are tests (`internal/errata`), not conventions. A defect with no
unambiguous intended reading is documented **without** a correction rather than
closed by a guess.

| Finding | File | Line | Citation | Overlay | Status |
|---|---|---:|---|---|---|
| dimensionless addend | `sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml` | 38 | SysML v2 §9.8.9.1 | corrected | **not filed** — drafted below, awaiting maintainer authorisation |
| mismatched dimensions | `sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml` | 25 | SysML v2 §9.8.9.1 | documented without a correction | **not filed** — drafted below, awaiting maintainer authorisation |
| a return typed by a dimension its value does not have | `sysml-examples/Analysis Examples/Dynamics.sysml` | 13 | KerML 7.4.9 | documented without a correction | **not filed** — drafted below, awaiting maintainer authorisation |
| non-conforming redefinition | `sysml-examples/Individuals Examples/AnalysisIndividualExample.sysml` | 86 | KerML 7.4.9, 8.3.4.2 | retired at `2026-07` | **closed** — fixed upstream, never filed by us |

Filing is the user's decision: nothing here has been posted to
`Systems-Modeling/SysML-v2-Pilot-Implementation` or any other upstream repository.

### `radius = 22/2*25.4 + 110 [mm]` adds a dimensionless value to a length (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Published, `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:38:

```sysml
:>> radius = 22/2*25.4 + 110 [mm];
```

Corrected by the overlay:

```sysml
:>> radius = (22/2*25.4 + 110) [mm];
```

`'[' SequenceExpression ']'` is a postfix on `PrimaryExpression`
(`build/pilot-grammars/KerMLExpressions.xtext:308`), below `AdditiveExpression`,
so `[mm]` qualifies `110` alone and the `+` combines a dimensionless value with
a length. **SysML v2 §9.8.9.1** requires the operands and the result of an
addition to share a quantity dimension, so the published expression is not
satisfiable under any reading, and the evident intent — a radius in millimetres —
is the parenthesised form. OpenSysML's warning at that line
(`operator '+' combines incommensurable quantities`) is therefore a true
positive. The pinned pilot performs no dimensional analysis and is silent both
on the published line and on the corrected one, so the correction changes our
verdict and not the pilot's.

### `1/(2 * Cp) * V^2 + T_static` adds L^6 to Θ (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Published, `Analysis Examples/Turbojet Stage Analysis.sysml`:25:

```sysml
return : TemperatureValue = 1/(2 * Cp) * V^2 + T_static;
```

`V` is declared `VolumeValue` (L^3) and `Cp` `DimensionOneValue`, so the first
operand has dimension L^6 while `T_static` is a `TemperatureValue` (Θ);
**SysML v2 §9.8.9.1** requires both operands and the result of `+` to share a
quantity dimension, and the declared return type Θ agrees with the second
operand only. The physics the calculation names (a total-temperature rise,
`V^2 / (2·Cp)`) wants `V` to be a speed rather than a volume, but the published
model does not say so anywhere: correcting it would mean choosing a type, a
unit and a dimension on the example's behalf.

So this row is **documented without a correction**. The overlay carries it for
provenance only; both figures every oracle reports keep the published text, and
OpenSysML's warning at that line stays in the differential census.

### `return a : AccelerationValue = tp * dt * tp` returns L^4·M^2·T^-5 (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Published, `Analysis Examples/Dynamics.sysml`:13, in `calc def Acceleration`
whose parameters are `in dt : TimeValue; in tm : MassValue; in tp: PowerValue`:

```sysml
return a : AccelerationValue = tp * dt * tp;
```

The file imports `ISQ::*`, whose definitions fix every dimension involved:
`AccelerationValue` declares `:>> mRef: AccelerationUnit[1]` and
`AccelerationUnit`'s power factors are L^1 and T^-2; `PowerValue` declares
`:>> mRef: PowerUnit[1]`, whose factors are L^2, M^1 and T^-3; and `TimeValue`
is `alias TimeValue for DurationValue`, T^1. The published product is therefore
(L^2·M·T^-3)^2 · T = L^4·M^2·T^-5, while **KerML 7.4.9** makes that expression
the value of the return feature, which answers to the `AccelerationValue` the
same line declares. The two dimensions are incommensurable, so the model is
unsatisfiable as published.

No correction is derivable. Acceleration from power is `tp / (tm * v)`, but this
calculation declares no speed among its parameters and its unused `tm` cannot
alone repair the exponents — `tp * dt / tm` is L^2·T^-2, not L·T^-2. Two of the
three plausible repairs also change the caller's contract, so the entry is
**documented without a correction** and the published text is what every oracle
reads.

The pinned pilot performs no dimensional analysis and is silent on the line, so
this is a diagnostic OpenSysML raises alone
(`cannot bind a value of dimension L^4·M^2·T^-5 to a feature typed by
AccelerationValue (dimension L·T^-2)`).

Earlier in the same file, at line 9, `calc def Power` has a second defect of the
same family that OpenSysML does **not** report:

```sysml
return tp : PowerValue = whlpwr - Cd * v - Cf * tm * v;
```

`whlpwr` is a `PowerValue` (L^2·M·T^-3), `Cd` and `Cf` are `Real`, `v` a
`SpeedValue` (L·T^-1) and `tm` a `MassValue`, so the three subtraction operands
have dimensions L^2·M·T^-3, L·T^-1 and M·L·T^-1 — which **SysML v2 §9.8.9.1**
requires to agree. The static dimensional check judges no product, by the
[documented design](spec-compliance.md), so no warning is raised here and the
overlay declares no entry for it either: an overlay entry covers one line per
file, and line 13 is the line an oracle reports. It is recorded here because a
report about this file should name both.

### `fuelConsumption : FuelEconomyAnalysis_1` redefines an action typed by `FuelConsumption` (pilot `2026-05`, fixed at `2026-07`)

**Never filed, and now closed.** The `2026-07` corpus publishes
`individual action :>> fuelConsumption : FuelConsumption_1` — the reading derived
below — so the overlay entry was retired and this section is kept as the record
of the finding.

Published at `2026-05`, `Individuals Examples/AnalysisIndividualExample.sysml`:86:

```sysml
individual action :>> fuelConsumption : FuelEconomyAnalysis_1 {
```

Corrected by the overlay, and published as such since `2026-07`:

```sysml
individual action :>> fuelConsumption : FuelConsumption_1 {
```

The redefined feature is `action fuelConsumption : FuelConsumption` of
`FuelEconomyAnalysis`, and **KerML 7.4.9** and **8.3.4.2** make a redefinition a
subsetting, whose subsetting feature's type must conform to the subsetted one's.
`FuelEconomyAnalysis_1` is the individual *analysis* definition specializing the
enclosing `FuelEconomyAnalysis`, so it conforms to nothing that `FuelConsumption`
specializes and the published model is unsatisfiable. Two lines above, the same
file declares `individual action def FuelConsumption_1 :> FuelConsumption` and
never mentions it again: the individual counterpart of the redefined feature's
type, which is what line 86 evidently meant to name. Substituting it clears our
error and leaves the rest of the file's verdict unchanged.

The pinned pilot validates subsetting conformance nowhere, so it is silent on
both texts, and the correction changes our verdict and not the pilot's.

---

## Proposed specification issue: identity annotations in the textual notation

**Filed** (maintainer-approved, 2026-09-01) against the **SysML 2.0** specification
(the textual-notation clauses, formal/26-03-02) as
[INBOX-2510](https://issues.omg.org/browse/INBOX-2510) — a temporary key that
redirects to the permanent one once the issue is assigned to a task force. The design
this draft distills is
[element-identity-annotations.md](element-identity-annotations.md); the working
prototype is OpenSysML's `IdentityMetadata` library, its validation pass, and the
RDF round trip. The body below is the submission text.

````markdown
**Title:** Textual notation cannot carry element identity, severing round trips
with the repositories the specification's own API defines

**Nature:** request for enhancement (interchange gap). **Severity:** significant.

The textual notation deliberately omits `Element::elementId`: text is treated as a
projection, and identity as the repository's concern. But the notation is the form
engineers version, diff and review, and the Systems Modeling API and Services
specification addresses every element by that id. The combination severs round
trips: any tool that serializes a model to text and reads it back has lost the
correlation with the repository it came from, so a rename — same element, new
name — is indistinguishable from a delete plus a create. Implementations are
already inventing workarounds (sidecar mapping files, IRI conventions, comment
conventions), none of which survive interchange through another conforming tool.

**Proposal:** standardize identity annotations — either a normative metadata
library, or dedicated surface syntax if the taskforce prefers. A minimal library
form, implementable today because user-defined metadata is already conforming
notation:

```sysml
standard library package IdentityMetadata {
    metadata def ElementId {
        attribute id : ScalarValues::String;
    }
    metadata def ProjectRef {
        attribute projectId : ScalarValues::String;
        attribute branch : ScalarValues::String[0..1];
        attribute org : ScalarValues::String[0..1];
    }
}
```

Applied opt-in: `@ElementId { id = "8f3a41d0-…"; }` on an element pins its
repository identity; one `@ProjectRef` on the root namespace binds the document to
a project (branch selecting a version, never contributing to identity). Elements
without an annotation keep tool-derived identity, so unannotated models are
unaffected and annotation cost is paid only where correlation matters.

**Implementation experience:** OpenSysML (github.com/Open-MBEE/OpenSysML)
implements exactly this shape: the metadata library, a validation pass (duplicate
and malformed ids, project-scope conflicts), RDF export keyed by the effective id,
and reader re-materialization closing the notation → RDF → notation round trip
byte-for-byte — all without any specification change, demonstrating that only the
*standardization* of the spelling is missing. Round-trip measurements against a
live Flexo MMS repository are maintained in the project's committed
interoperability report.
````

Submitted 2026-09-01 via the
[OMG issue reporting form](https://issues.omg.org/issues/create-new-issue); the
key above updates once a task force takes the issue.
