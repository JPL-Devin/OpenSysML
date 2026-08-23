# Wave 12F — our false positives on the reference's own valid models

Wave 12F owns the rows where **we** report something on a model the pinned OMG pilot
(tag `2026-05`, artifacts `0.60.1`) accepts. A false positive on the reference's own corpus is the
most expensive divergence we can carry, because it makes our validator unusable on models the pilot
considers valid — so the slice is measured on the reference's corpora and its Xpect fixtures, not on
ours.

Every number below is from a fresh-cache run on this branch, never quoted from a report:

```
rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-xpect -out build/xpect-fresh
rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-diff
rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-reject
```

## What moved

| Harness | Base `7504ff09` | This branch |
|---|---|---|
| Xpect | 428 files, 0 unparsed; 1269 agree (246 wording-only) / **54 disagree** | 1275 agree (246 wording-only) / **48 disagree**, 0 unlocated, 0 not adjudicated |
| Differential | 353 files, **317 fully agreeing**; 32 agreed, **119 only ours**, 66 only pilot, 175 ours / 122 pilot | **321 fully agreeing**; 32 agreed, **92 only ours**, 66 only pilot, 148 ours / 122 pilot |
| Rejection | 120 cases: 116 both reject, 4 pilot-only, 0 ours-only | unchanged: 116 / 4 / 0, 0 both accept |

**The only-ours multiset moves in exactly two buckets and no bucket rises**, which is the test that
matters here — a total that falls while some family grows is a trade, not progress:

| Only-ours bucket | Base | Now |
|---|---:|---:|
| `error` / `unresolved-reference` | 33 | **10** |
| `warning` / `unmapped` | 20 | **16** |
| `warning` / `syntax` | 63 | 63 |
| `warning` / `units` | 1 | 1 |
| `error` / `kind-mismatch` | 1 | 1 |
| `error` / `multiplicity` | 1 | 1 |
| **total** | **119** | **92** |

Four files become fully agreeing, all on the reference's own corpora: `Arrowhead Framework
Example/AHFSequences.sysml` 15 → 0, `Cause and Effect Examples/CauseAndEffectExample.sysml` 6 → 0,
`Simple Tests/AllocationTest.sysml` 1 → 0, and `Vehicle Example/SysML v2 Spec Annex A
SimpleVehicleModel.sysml` 5 → 0. The pilot's own columns (122 diagnostics, 66 only-pilot rows), the
32 agreements and the 24 severity-only pairs are unmoved: nothing we did changed what the pilot
detects or moved a row of ours out of agreement.

On the Xpect side all six moved rows go from disagreement to agreement: `errors` 479 → **483** with
`same-line` 9 → **5**, and `noErrors` 263 → **265**.

## What we implemented, and the specification it comes from

### 1. `parseConnectorEnd` consumed the commas between n-ary connector ends (parser)

`AllocationTest.sysml.xt:31` declares file-wide silence and we reported
`unresolved reference: physical — did you mean AllocationTest::Logical_to_Physical::physical or
Physical?`. The hint named the right element, so the lookup was not the defect: the *parse* was.
`parseConnectorEnd` delegated to the general relationship parser, which keeps consuming
comma-separated relationship targets, so in

```sysml
allocate logical references l, physical references p;
```

the second and later ends were folded into the first end's relationship list and then resolved in the
wrong scope. The grammar allows exactly one target per end:

```
ConnectorEnd returns SysML::Feature :
    ( ownedRelationship += OwnedCrossingMultiplicityMember )?
    ( declaredName = Name ReferencesKeyword )?
    ownedRelationship += OwnedReferenceSubsetting
;
```

so the end now parses one target plus only the relationship forms this production admits (`::>`,
`references`, and an explicit `:>>`), and stops at the comma that separates ends.

**Both directions.** `internal/core/parser/connector_ends_nary_test.go` pins the n-ary,
parenthesized, binary, `from … to …` and succession forms, the declared-name/`references` split
(`ConnectorEnd.DeclaredName()`, `ConnectorEnd.ReferencedTarget()`), and that a missing target still
produces a diagnostic; the one-end parenthesized connector still draws its arity diagnostic and the
negative parser suite is unchanged.

### 2. A recursive `import X::**` did not follow re-exported memberships (resolver)

`KernelLibraryTest.sysml.xt:72` declares silence and we reported `unresolved reference: MassValue —
did you mean ISQBase::MassValue?` under `import ISQ::**`. KerML 8.2.3.5 makes a recursive import
import the memberships of the target namespace *and*, recursively, of the namespaces it contains —
and a namespace's memberships include the ones it re-exports publicly, which is exactly how `ISQ`
publishes `ISQBase`'s names. Our recursive branch walked the containment tree of scopes but consulted
only each scope's own declarations, so a publicly re-exported name was visible through `ISQ::*` and
invisible through `ISQ::**`. It now goes through the same re-export-aware traversal (`appendSubtree`)
the non-recursive path uses, which keeps the cycle guard, `importAdmits`, filters, visibility and the
body-local exclusion.

This is **not** the index-only library contract (roadmap L3): the same name resolves through the
index under `ISQ::*`, so nothing here asks the index to carry more than it does.

**Both directions.** `internal/core/resolve/f67_import_reexport_test.go` and `filter_test.go` cover
`B::*`, `B::**`, a public re-export, a private import that must not leak, filter clauses, `ISQ::*`,
`ISQ::**`, and names that must stay unresolved staying unresolved.

### 3. Anonymous enumerated values were named after their keyword (parser)

`SysML v2 Spec Annex A SimpleVehicleModel.sysml` writes

```sysml
enum def DiameterChoices :> ISQ::LengthValue {
    enum = 60 [mm];
    enum = 80 [mm];
    enum = 100 [mm];
}
```

The `EnumeratedValue` production admits a keyworded value with no declared name, but our enum body
took the `enum` keyword as the value's *name* whenever a name was absent, so all three values were
called `enum` and we reported three `Duplicate of other owned member name` warnings plus one
`Duplicate of inherited member name 'enum'`. Four only-ours rows, all of them ours, none of them a
duplicate in the model. The body now recognizes `enum` followed by `=` or `:=` as an anonymous value.

**Both directions.** `enum red;` still declares `red` and `enum x = 60.0;` still declares `x`; a body
that really repeats a name still draws its four duplicate diagnostics
(`internal/core/parser/f61_keywordless_members_test.go`).

### 4. Metadata evaluability was reported on the value, not on the binding (span)

`MetadataTests_MetadataFeature_invalid.kerml.xt:66,69` and `MetadataUsage_Invalid.sysml.xt:82,85`
declare `Must be model-level evaluable` — our rule, our severity, our line, our element — and only
the offset differed, so the rows counted as `same-line` disagreements. The pilot points at the whole
binding (`= ~3`, `= f((as A).y)`, `= f((as A).z)`), which is what SysML 7.24's
`checkMetadataBodyFeature` judges: the *FeatureValue*, whose notation begins at the binding operator,
not the expression alone. The predicate is wave 11G/11D's and is untouched; the parser now records
the `=` / `:=` / `default =` operator's span on the usage, and only the metadata diagnostic reads it.

**Both directions.** A model-level evaluable body value (`x = 1 + 2;`) stays silent, every other
diagnostic's span is unmoved (the four operator-span fields that no consumer reads were removed
rather than left dead), and the span ends at the last consumed operator token, so it never swallows
trailing trivia (`internal/core/parser/feature_value_operator_test.go`,
`internal/core/passes/w8c_metadata_annotation_test.go`).

### 5. Five rules re-derived so the corrected parses stay clean

Correcting a parse makes constructs visible to rules that never saw them before, and three of ours
then fired on models the pilot accepts. I measured this rather than assumed it: a scratch tree with
the two parser/resolver fixes above but **all five rule changes below reverted** to their `0d4eb14f`
form measures `353 files, 318 fully agreeing; 25 agreed, 103 only ours, 73 only the pilot's` — 11
only-ours rows that are on neither the base tree nor this branch. So the parser and import fixes
retire 27 rows and unmask 11, and closing those 11 is what keeps the "must not add only-ours rows"
rule satisfied. Xpect is byte-identical between the two trees (1275 / 48), so none of the five is
paid for with an Xpect row.

The 11 unmasked rows, and the rule each one belongs to:

| Rows | Where | Ours, unmasked | Rule change | Derivation |
|---|---|---|---|---|
| 9 | `pilot-examples`: `Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml`:788, 792, 800, 803, 806, 813, 825, 837, 840 | `Nested usages in a port usage (other than ports) must be referential.` | an event occurrence in a port body is referential, not composite (`passes/w10b_structural.go` `w10bIsComposite`) | the normative library says so: `metadata def EventOccurrenceUsage specializes OccurrenceUsage { derived attribute isReference : Boolean[1..1] redefines isReference; … }` (`Systems Library/SysML.sysml`), so an event occurrence usage is a reference and the rule's own exemption applies. The declaring direction still holds: a composite `part b4 : B;` in a port body is reported (`PortUsage_Invalid.sysml.xt`:50, and `passes/w10b_structural_test.go` keeps a redefined port with a composite child diagnosed) |
| 1 | `pilot-examples`: `Arrowhead Framework Example/AHFSequences.sysml`:80 | `an interface connection must be binary (exactly two ends)` | the arity rule applies only to an interface conforming to a binary base (`passes/constraint.go` `interfaceIsBinary` over `semantics.Model.IsBinaryConnector`) | `Interfaces::Interface` declares `ref port :>> participant : Port[2..*] nonunique ordered` — it is n-ary, and only `BinaryInterface` narrows it to two participants with `source`/`target`. A probe confirmed the split against the pilot: six named ends typed by `Interface` draw no arity diagnostic from it, three ends typed by `BinaryInterface` do. Declaring direction: `InterfaceUsage_Invalid.sysml.xt`:49 still expects `Cannot have more than two ends` at the definition level and is still open below — this narrowing does not close it |
| 1 | `pilot-examples`: `Simple Tests/AllocationTest.sysml`:22 | `Must have at least two related elements` | ends are counted through semantic direct supertypes, and an index-only binary base supplies its two known ends (`passes/w10b_related_elements.go`, `semantics/connector.go`) | `allocation def Allocation :> BinaryConnection` (`Systems Library/Allocations.sysml`) inherits two effective ends, so an allocation definition with no declared end relates two elements; `Links::BinaryLink` is index-only in `internal/core/libs`, with specialization edges but no parsed body, which is why the count has to come from semantics. Declaring direction: a generic one-end concrete connection is still reported (`Relationship_invalid_relatedElement1.sysml.xt`:52 and `passes/w10b_related_elements_test.go`) |

Two further rules of mine fire on shapes the pilot accepts without any corpus row exercising them,
so they move no measured number; I still corrected them, because a latent false positive is the same
defect as a measured one, and because both were blanket exemptions that suppressed real diagnostics:

- **Accessibility of a `satisfy`/`verify` target.** The earlier code skipped every `UsageSatisfy`,
  which suppressed the rule outright. It now skips only a dotted feature-chain target
  (`verify vehicleSpecification.vehicleMassRequirement`) — the notation the message itself prescribes
  — while a `::`-qualified target of another type (`satisfy R::nested by p`) is still reported as
  `Must be an accessible feature (use dot notation for nesting)`, which is the pilot's behaviour on a
  probe of both forms and F20's reading of `FeatureUtil.canAccess`.
- **Subject position.** `Subject must be first parameter.` is a statement about the declaration's
  parameters in lexical order, so position is judged over local declarations, and the semantic member
  order is consulted only where no local parameter can be inspected. Both directions are locked:
  a subject before a later input is silent, a subject after an input is reported
  (`CaseSubjectObjective_Invalid.sysml.xt`:73, 82, 90 and `RequirementSubject_Invalid.sysml.xt`:72,
  79 continue to agree), and `TestW7GLocalResultSuppressesInheritedParameterFallback` pins the
  boundary — a locally declared result must not fall through to an inherited input.

## The rows we did not close

The four categories are the ones `wave11e-decisions.md` fixed:

| Category | Meaning |
|---|---|
| **our defect** | we are wrong and the pilot is right |
| **unimplemented obligation** | the specification states the rule; we do not implement it yet |
| **pilot limitation** | the pilot's declared expectation does not follow from the specification |
| **adjudicated divergence** | we deliberately differ, with the reading recorded |

### All 92 only-ours differential diagnostics

Every row of the census is grouped, categorized and owned. `examples` is our own demo corpus; the
other roots are the reference's.

| Rows | Root / files | Family | Category | Owner |
|---:|---|---|---|---|
| 26 | `examples`: `action-executor-demo` (12), `views-demo` (7), `phase-c-behavioral-bodies` (6), `orthogonal-regions-demo` (1) | `then <source> <target>;` has no SysML v2 production | adjudicated divergence — an OpenSysML extension we warn about in our own models, documented in [03-command-line.md](../guide/03-command-line.md) | OpenSysML examples; not a divergence about the reference's models |
| 24 | `examples`: `phase-c-behavioral-bodies` (16), `pseudostates-demo` (6), `orthogonal-regions-demo` (2) | `transition <source> to <target>;` has no production | adjudicated divergence, as above | OpenSysML examples |
| 6 | `examples`: `solver-demo` | `require` outside a requirement body | adjudicated divergence, as above | OpenSysML examples |
| 3 | `examples`: `action-executor-demo` (2), `views-demo` (1) | `done <name>;` has no production | adjudicated divergence, as above | OpenSysML examples |
| 4 | `examples`: `orthogonal-regions-demo` (2), `pseudostates-demo` (2) | `initial <state>;`, `region … { … }`, `junction <name>;` | adjudicated divergence, as above | OpenSysML examples |
| 15 | `pilot-examples`: `Simple Tests/PartTest.sysml` (4); `kerml-examples`: `Simple Tests/Circular.kerml` (3); `testdata`: `passes/constraints.sysml` (2); `probes` (6) | `<x> participates in a specialization cycle` | adjudicated divergence — the same F4/K5 reading as the three Xpect rows below; every fixture declares a real cycle and the pilot has no such check | 12F (this doc) |
| 6 | `pilot-examples`: `Vehicle Example/Annex_A_VehicleViews.sysml`:753–789 | `unresolved reference: Safety` / `Security — did you mean SimpleVehicleModel::Definitions::MetadataDefinitions::Safety?` | our defect | resolver — an `@Safety` / `@Security` prefix annotation inside a nested part body, whose metadata definition the file reaches through imports; the mechanism is not derived in this slice |
| 2 | `pilot-examples`: `State Space Representation Examples/EVSample1.sysml`:351,354 | `unresolved reference: sourceOutput` / `targetInput — did you mean Transfers::Transfer::source::sourceOutput?` | our defect | resolver — `attribute :>> sourceOutput :>> output.voltage;` inside a `flow`'s `end ::> battery`, so the redefined name is inherited through the end's implicit `Transfer` typing |
| 1 | `pilot-examples`: `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:62 | `unresolved member: transformation` (from `lbcf.transformation == trs`) | our defect | resolver — a feature-chain member on a locally declared coordinate frame |
| 1 | `pilot-validation`: `09-Verification/9-Verification-simplified.sysml`:55 | `unresolved reference: massRequirement — did you mean MassRequirement?` | our defect | resolver — `verify vehicleMassRequirement :>> massRequirement;`, the same shape as the `EVSample1` rows: a `:>>` target inherited from the enclosing verification's type |
| 1 | `pilot-examples`: `Vehicle Example/VehicleDefinitions.sysml`:47 | `interface Mounting connects ports … whose directed features are not conjugate` | adjudicated divergence — a warning of ours where the pilot has no check | 11A (usage typing), kept |
| 1 | `pilot-examples`: `Analysis Examples/Turbojet Stage Analysis.sysml`:25 | `operator '+' combines incommensurable quantities` | unexamined — no category yet | units/quantity slice |
| 1 | `pilot-examples`: `Individuals Examples/AnalysisIndividualExample.sysml`:86 | `fuelConsumption (typed by FuelEconomyAnalysis_1) redefines fuelConsumption (typed by FuelConsumption): types do not conform` | unexamined — no category yet | conformance slice |
| 1 | `testdata`: `passes/constraints.sysml` | `multiplicity lower bound exceeds upper bound on lo` | adjudicated divergence — our own fixture, declared behaviour | 12F (this doc) |

Two rows carry **unexamined** rather than one of the four categories, and that is deliberate: this
slice did not derive their rules, and guessing `our defect` or `pilot limitation` without the
derivation would be the reclassification the effort forbids. They are the only two rows in the census
without an adjudicated category, and each has an owner.

Read the totals by root, never as one number: **63 of the 92 are true positives on our own
`examples` corpus** — our extension warnings, firing where the pilot's grammar has no production —
and the honest count of suspect diagnostics of ours against the reference's corpora is **20**, of
which 10 are unresolved-reference defects of ours, 7 are adjudicated specialization cycles, 1 is the
adjudicated conjugation warning, and 2 are the unexamined single rows.

### Xpect rows this slice looked at and left open

| Row | Declared | Ours | Category | Owner |
|---|---|---|---|---|
| `SimpleImportTests_CircleInheritanceInCircleImport.kerml.xt:18` | silence | 2 errors, first `a participates in a specialization cycle` at :29 | adjudicated divergence | 12F (this doc) |
| `PartTest.sysml.xt:29` | silence | 4 errors, first `p1 participates in a specialization cycle` at :67 | adjudicated divergence | 12F (this doc) |
| `Redefinition_OwningType_Cyclic_Gen.sysml.xt:25` | silence | 2 errors, first `A participates in a specialization cycle` at :28 | adjudicated divergence | 12F (this doc) |
| `InterfaceUsage_Invalid.sysml.xt:49` | `error: Cannot have more than two ends` | `An interface definition end must be a port.` at :53 | unimplemented obligation | 12A (tier gating) with a definition-level arity rule |
| `InterfaceUsage_Invalid.sysml.xt:78` | `warning: Duplicate of inherited member name 'self' from Part, Port` | no warning; our `An interface end must be a port.` error at :79 | unimplemented obligation | the diamond rule (resolver), which must follow the `end part ::> tankAssy.fuel;` subsetting chain |
| `BindingConnector_Invalid2.sysml.xt:42` | `warning: Bound features should have conforming types` | nothing at that line | unimplemented obligation | a binding-conformance rule (`semantics/conformance`) |

### Why the three specialization-cycle rows stay

The fixture names say what the pilot thinks — a circular *import* is not a specialization cycle, and a
cyclic redefinition of an owning type is legal — so the question is what the specification's
specialization relation actually contains. It contains exactly what these fixtures write:

- `SimpleImportTests_CircleInheritanceInCircleImport.kerml.xt`:29,37 — `classifier a specializes b`
  and `classifier b specializes a`. The circular *import* is irrelevant to the cycle: the two
  `specializes` relationships alone close it, and KerML 7.3.4.2 requires the specialization relation
  to be a strict partial order (a type may not be a proper supertype of itself), so the pair is not
  satisfiable by any model.
- `PartTest.sysml.xt`:67–71 — `part p1 :> p2; part p2 :> p3; part p3 :> p1;` and `part p4 :> p4;`.
  `:>` is subsetting, whose transitive closure is likewise irreflexive (KerML 8.3.4.2); the self-loop
  is the same rule at length one.
- `Redefinition_OwningType_Cyclic_Gen.sysml.xt`:28–34 — `part def A :> C` with `part def C :> A, B`.
  Redefinition of an owning type is indeed legal, and this fixture does not only redefine: it
  declares the two specializations that close the cycle.

The pilot has no cycle check of any kind — the F4/K5 finding recorded in
[pilot-differential.md](pilot-differential.md#specialization-cycles-f4) — so its silence records an
absent check rather than a reading of the specification. Closing these rows means deleting a correct
rule, so they are **adjudicated divergences**: 3 Xpect rows and 15 differential rows, all of them
enumerated above.

## Verification

Both directions were checked for every row closed: the fixture that *declares* the rule still gets
it, and no only-ours bucket rose. Beyond the harnesses, the gate is `go build ./...`, `go vet ./...`,
`gofmt -l .`, `go test ./...`, `make docs-counts` and `python3 scripts/check-doc-links.py`, plus the
SMT gates (`z3`, `cvc5`) and the training/pilot corpus gates with
`OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1`.
`internal/core/model/testdata/pilot_corpora_expected.txt` was regenerated on this tree through the
documented update command, as were `docs/project/pilot-differential-baseline.json` and
`docs/project/pilot-xpect-baseline.json`; `pilot-rejection-baseline.json` is byte-identical, because
nothing in this slice moved a rejection case.
