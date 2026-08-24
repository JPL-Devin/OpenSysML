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
(maintainer ruling, PR for task S4), so `includingAt` is a divergence from the
vendored body and is recorded here for review against a future OMG release.

---

## Defects in published OMG example models

These rows are true positives found by OpenSysML's quantity analysis in models published with the
OMG example corpora. The pinned pilot is silent because it does not perform the corresponding
static dimensional check.

| Example | Expression as published | Defect | Specification reading | Status |
|---|---|---|---|---|
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml:38` | `22/2*25.4 + 110 [mm]` | `[mm]` binds only to `110`, so `+` combines a dimensionless value with a length | KerML 1.0 §8.2.5.8.1–8.2.5.8.2 makes the bracket construction a primary expression; SysML v2.0 §9.8.9.1 requires addition operands to have the same quantity dimension. The evident intended spelling is `(22/2*25.4 + 110) [mm]`. | **not filed** |
| `Analysis Examples/Turbojet Stage Analysis.sysml:25` | `1/(2 * Cp) * V^2 + T_static`, with `Cp : DimensionOneValue`, `V : VolumeValue`, and `T_static : TemperatureValue` | the declared types make the operands L^6 and Θ | SysML v2.0 §9.8.9.1 requires addition operands to have the same quantity dimension and top-level quantity type. The formula needs dimensionally appropriate parameter declarations or conversion before addition. | **not filed** |

They form one adjudicated quantity-commensurability family in
[wave12f-false-positives.md](wave12f-false-positives.md): OpenSysML retains both warnings, and
nothing has been posted upstream. Both are also entries of the declared errata overlay — F82 with a
correction, F83 without one — quoted verbatim with their derivations in
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
| `org.omg.sysml` — `Type::ownedDisjoining` setting delegate | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | every `disjoint from` clause in a type declaration draws EMF's `The opposite features 'owningType' … and 'ownedDisjoining' … do not refer to each other` | [K6 / F33](pilot-differential.md#k6-diagnostic-by-diagnostic-f33) — one cause for all six corpus diagnostics, reproduced in three lines and probed through the pilot's API | filed upstream as [Systems-Modeling/SysML-v2-Pilot-Implementation#790](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/790) (W10G), **pending adjudication**, body below |
| `org.omg.sysml` — the `queryx/failing` Xpect fixtures | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | `QPE-Qualifier`, `QPE-Traversal` and `QPE-Wildcard` declare `XPECT noErrors`, yet the pinned validator rejects all three with `no viable alternative at input '/'`, `For input string: "."` and `no viable alternative at input '@'` | [wave12d-decisions.md](wave12d-decisions.md) — established by running the pinned pilot's own SysML validator on the three fixtures, not from a disagreement | **not filed** — question drafted below, awaiting maintainer authorisation |

### F80 — `Type::ownedDisjoining` does not contain a `Disjoining` whose `owningType` is that `Type` (pilot `2026-05`)

Filed as
[Systems-Modeling/SysML-v2-Pilot-Implementation#790](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/790);
the body below is what was submitted, and the supporting analysis is
[K6, diagnostic by diagnostic](pilot-differential.md#k6-diagnostic-by-diagnostic-f33).

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
[the wave-9B sweep](pilot-differential.md#only-the-pilot--the-wave-9b-row-by-row-sweep-137)
(W16). The clause appears on classifiers, plain types and features alike and the
reported EMF class tracks the declaration, so the defect is in the pair rather
than in one metaclass; the standalone form `disjoint b.f.a from b.a;`
(`Simple Tests/FeatureChains.kerml:28`) sits in the same file as one of the six
and reports nothing. No `kerml-examples` file carries a second pilot-only row of
any kind, so a fix to the derived `ownedDisjoining` delegate clears this root's
column entirely and silences nothing else it depends on.

---

### F81 — are the `queryx/failing` query path expressions intended notation? (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The finding is wave 12D's, and the adjudication is in
[wave12d-decisions.md](wave12d-decisions.md).

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

---

## The errata overlay entries for these models

The second section's rows are also entries of the declared errata overlay
(`internal/errata`, [wave14-errata.md](wave14-errata.md)): the published bytes on
disk are never edited, and a row that carries a correction has that correction
applied to the *second* figure every oracle reports, never to the headline one.
The overlay adds no category and reclassifies nothing — both rows stay the
adjudicated quantity-commensurability family wave 12F recorded.

An entry is accepted only with a specification citation and a written
derivation, and only while its as-published text still matches the corpus on
disk; both are tests (`internal/errata`), not conventions. A defect with no
unambiguous intended reading is documented **without** a correction rather than
closed by a guess.

| ID | File | Line | Citation | Overlay | Status |
|---|---|---:|---|---|---|
| F82 | `sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml` | 38 | SysML v2 §9.8.9.1 | corrected | **not filed** — drafted below, awaiting maintainer authorisation |
| F83 | `sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml` | 25 | SysML v2 §9.8.9.1 | documented without a correction | **not filed** — drafted below, awaiting maintainer authorisation |

Filing is the user's decision: nothing here has been posted to
`Systems-Modeling/SysML-v2-Pilot-Implementation` or any other upstream repository.

### F82 — `radius = 22/2*25.4 + 110 [mm]` adds a dimensionless value to a length (pilot `2026-05`)

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

### F83 — `1/(2 * Cp) * V^2 + T_static` adds L^6 to Θ (pilot `2026-05`)

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
