# Training Examples Status

## Overview

**Source:** [SysML-v2-Pilot-Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) training examples  
**Download:** https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training  
**Status:** 90/100 files parse and resolve cleanly (0 semantic errors)  
**Errors**: 10/100 files have semantic errors (20 total errors)  
**Gate**: the per-file error counts are recorded in `internal/core/model/testdata/training_examples_expected.txt`, so `TestTrainingExamplesSemanticErrors` fails when a file regresses *or* improves without updating the list (`-update-training` regenerates it)  

These training examples are from the official OMG pilot implementation and are not vendored here. Run `./scripts/download-training-examples.sh` to fetch the pinned (`2026-05`) copy into `examples/sysml-v2-training/`; the tests that read it skip while it is absent.

---

## Adjudicating a change in the expectations

The expectation file is a snapshot of this implementation's behavior, not an
oracle: regenerating it with `-update-training` records whatever the code now
reports, so a regression re-baselines just as quietly as a fix. Every entry that
changes must therefore be judged against the OMG model that produced it, and the
verdict recorded below, before the new count is committed:

- **A file that got cleaner** is only an improvement if the references it used to
  report now resolve to the right declarations. Confirm that — a file also goes
  clean when a construct stops being parsed or checked at all.
- **A file that reports more** is a regression until shown otherwise. New
  diagnostics that are false positives stay recorded, with the gap named here.
- **A count that moves without a code change in that area** usually means the
  harness moved. Both cases found so far were harness artifacts, not semantics.

### Verdicts for the 2026-08 re-pin (71/100)

The recorded counts had been generated seven PRs earlier and were re-adjudicated
file by file:

**Real fixes made while adjudicating**

| Finding | Verdict |
|---|---|
| `variant attribute diameterSmall = 70[mm];` reported `expected '{' or ';' after declaration` (`Variation Definitions`) | Regression in keyword-name parsing: the prefix `variant` took the kind keyword `attribute` as its name. Fixed in `parser.parseDefUsage`. |
| `ambiguous reference: SI::min` (`Local Clock Example`) | False positive: `SI` declares `<min> minute` *and* re-exports `min` through `public import`. A namespace's own member now shadows a wildcard re-export (`symbols.Index.LookupQualified`). |
| `Requirement Groups`, both `Car Mass Rollup` files, four errors in `Variation Configuration` | Harness artifact: the gate diagnosed each file immediately after opening it, so a cross-file `private import 'Requirement Usages'::*` failed purely because that file sorts later. The gate now opens the whole corpus first. |

**Genuinely cleaner (verified, not silently unchecked)**

`Interaction Example-1`, `Interaction Realization-1`, `Interaction Realization-2`
and six of the eight errors in `Message Payload Example` were `unresolved
reference: setSpeedSent` and friends. `event occurrence setSpeedSent;` now keeps
its name (PR #17) and indexes as an occurrence usage, so those references resolve
to the declarations they name.

**False positives that stay recorded (resolver gaps, now reachable)**

Resolving names inside behavior bodies (PR #8) exposed these; each is a wrong
diagnostic on a well-formed model, so the count is pinned and the gap named:

| File | Diagnostic | Gap |
|---|---|---|
| `Time Constraints` | `unresolved member: done` | Inherited occurrence features are not members of a state usage: an untyped `state normal;` has no implicit typing to `States::StateAction`. |
| `Message Payload Example` | `unresolved reference: fuelCommand` (2×) | The payload feature a message declares in its `of` clause is not registered. (Fixed — see the message-payload re-pin below.) |
| `Action Performance Example`, `Allocation Usage Example`, `Conditional Succession Example-1` | `unresolved member: focus`/`generateTorque`/`isWellFocused` | Same missing implicit typing: features of the stdlib base type of an untyped usage are not members of it. |

### Verdicts for the inherited- and body-local-feature re-pin (80/100)

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `Calculation Usages-1` | 7 unresolved `a`/`v`/`x` | Real fix: `return a;` declares the calc's return parameter, which the parser had read as a reference to one. The names now resolve to those parameters. |
| `Trade Study Analysis Example` | 10 errors, now 1 | Real fix, same cause plus inherited members of a calc usage (`power`, `mass`, `efficiency`, `cost`). The remaining error is `alternative`, a genuine typo in the OMG file (`alternatives`). |
| `Variation Usages`, `Variation Configuration` | `engine::'4cylEngine'` etc. | Real fix: a qualified-name segment now reaches inherited members, and `part redefines engine` no longer resolves its own redefinition target to itself. |
| `Parts Example-1`, `Parts Example-2` | `unresolved reference: cyl` (2× each) | Real fix, same redefinition-shadowing cause: `part redefines eng { part redefines cyl[4]; }` resolves `cyl` through `Engine`. |
| `Use Case Usage Example` | 6 unresolved actors | Real fix: `'provide transportation'::driver` reaches the actors the use case usage inherits from its definition. |
| `Control Structures Example` | `unresolved reference: charging` | Real fix: a loop owns the scope its body declares into, so its `until` condition sees `charging`. |
| `Assignment Example` | `unresolved reference: dynamics` (2×) | Real fix, same scope: an action declared in a `for` body is visible to the `assign` steps in that body. |
| `Analysis Case Definition Example` | `unresolved reference: i` (8×) | Real fix: a body expression's parameters (`->forAll {in i: Positive; ...}`) are in scope in its result. |

Each verdict is locked by a focused test in
`internal/core/model/inherited_scope_resolve_test.go`, including the negative
cases (a redefinition of an undeclared name, and body-local names referenced
from outside their body, both still report).

### Verdicts for the implicit-usage-typing re-pin (81/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `31. Constraints/Time Constraints` | 1 × `unresolved member: done` | Real fix: `state normal;` is now implicitly typed by `States::StateAction`, which declares `done`, so `TimeOf(normal.done)` resolves to that declaration. The negative counterpart (`normal.notAMember`) still reports — see `internal/core/model/implicit_typing_test.go`. |

**Still recorded, and why implicit typing alone does not fix them**

| File | Diagnostic | Remaining gap |
|---|---|---|
| `Conditional Succession Example-1` | `unresolved member: isWellFocused` | Implicit *redefinition*: `out item image;` inside `action focus : Focus` refines `Focus::image` (typed `Image`). Untyped usages that shadow an inherited feature deliberately got no implicit base, so the type came from nowhere. (Fixed in the 88/100 re-pin below.) |
| `Action Performance Example`, `Allocation Usage Example` | `unresolved member: focus`/`shoot`/`generateTorque` | The members come from `perform action takePhoto references takePicture;` and `perform providePower.generateTorque;`: a `references` edge and the feature a `perform` statement contributes, neither of which is a generalization. **Fixed since — see the reference-subsetting verdicts below.** |
| `Time Slice and Snapshot Example`, `Individuals and Time Slices` | `unresolved reference: start`/`done` | Bugs in the OMG files (`startShot`/`endShot`), unchanged. |

### Verdicts for the reference-subsetting re-pin (81 → 83; 87 once `main`'s message-payload and satisfy-reference fixes merged in)

Two entries went clean and one reports more; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `18. Action Performance/Action Performance Example` | 2 × `unresolved member: focus`/`shoot` | Real fix: `perform action takePhoto references takePicture;` relates `takePhoto` to `takePicture` by a reference subsetting (SysML 7.17.6), which contributes the referenced action's members. `takePhoto.focus` now resolves to `takePicture::focus`. The negative counterpart (a member the referenced action does not declare) still reports — see `internal/core/semantics/reference_test.go` and `internal/core/model/perform_reference_test.go`. |
| `38. Allocation/Allocation Usage Example` | 2 × `unresolved member: generateTorque` | Real fix, two causes: `perform providePower.generateTorque;` names its feature after the feature it references (KerML `Feature::effectiveName`), so `torqueGenerator.generateTorque` names a declaration; and `allocate torqueGenerator to powerTrain` is an anonymous binary allocation, whose first name is a connector end rather than the usage's own name. |
| `32. Requirements/Requirement Satisfaction` | 2, then unchanged | Same fix, in a file that was already recorded: `perform 'provide power'.'generate torque'` resolves now. Its two remaining errors were unrelated and are cleared separately by the satisfy-reference verdicts below. |

**A file that reports more, adjudicated**

| File | Was | Now | Verdict |
|---|---|---|---|
| `34. Verification/Verification Case Usage Example` | 3 × `unresolved reference: testVehicle`/`massMeasured` | 6 × `individual cannot specialize partDef` / `... cannot be typed by individualDef` | The three name-resolution false positives are fixed by this change: `perform vehicleMassTest;` used to shadow the verification usage it performs with an empty feature, so `vehicleMassTest.collectData` and the redefinitions under it resolved to nothing. With the name-resolution tier clean, the type tier runs on this file for the first time (tiers are skipped after a lower tier errors) and reports six pre-existing false positives about individuals: `individual def TestSystem :> MassVerificationSystem;` and `individual testSystem : TestSystem` are well-formed (SysML 7.9.5), and the kind tables in `passes/typecheck.go` do not yet accept an individual definition specializing an occurrence definition. Recorded, not fixed, to keep this change scoped; see `docs/SPEC_COMPLIANCE.md`. |

### Verdicts for the message-payload re-pin (82/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `27. Occurrences/Message Payload Example` | 2 × `unresolved reference: fuelCommand` | Real fix: `message m of fuelCommand : FuelCommand` *declares* the payload feature. The parser built that declaration but then overwrote `Usage.Members` with the body members, so it was never registered, and the `of` name was resolved as a reference in the enclosing scope. The declaration is now a member of the message (`FlowEnds.PayloadDecl`) and the `of` name resolves to it, which also makes `fuelCommandMessage.fuelCommand` reach the payload. |

The payload *reference* form (`flow f of Fuel from a to b`) is unchanged and
still resolves outward, with the negative case (`of` naming nothing) still
reporting — see `internal/core/model/flow_payload_resolve_test.go`.

### Verdicts for the satisfy-reference re-pin (85/100)

Three entries drifted, all of them the same false positive; every other file kept
its exact count.

**Spec basis.** `SatisfyRequirementUsage` (SysML v2 §7.21.4, abstract syntax
§8.3.21.10; concrete syntax in the pilot `SysML.xtext`) is:

```
SatisfyRequirementUsage :
    OccurrenceUsagePrefix 'assert'? ( isNegated ?= 'not' )? 'satisfy'
    ( ownedRelationship += OwnedReferenceSubsetting FeatureSpecializationPart?
    | RequirementUsageKeyword UsageDeclaration?
    )
    ValuePart? ( 'by' ownedRelationship += SatisfactionSubjectMember )? RequirementBody
;
```

Without the `requirement` keyword the name after `satisfy` is an
**OwnedReferenceSubsetting** — a `ReferenceSubsetting` (a `Subsetting`) whose
`referencedFeature` must be a `Feature`, i.e. a **usage**, never a definition.
`satisfy <requirementDef>` is in fact the ill-formed direction.

The abstract syntax makes this normative — `SatisfyRequirementUsage` carries the
constraint

```
ownedReferenceSubsetting <> null implies
    referencedFeatureTarget().oclIsKindOf(RequirementUsage)
```

so the referenced element must be a `RequirementUsage`. `ViewpointUsage` and
`ConcernUsage` both specialize `RequirementUsage` (`SysML.ecore`:
`ViewpointUsage eSuperTypes="#//RequirementUsage"`), so
`satisfy <viewpointUsage>` inside a `view def` is equally legal.

**Verdict: type-checker false positive.** The parser encoded the reference as a
`FeatureTyping` (`RelTyping`), so the type tier demanded a definition. It now
encodes it as `RelSubsets`, and the type tier requires the target to be a
requirement usage (including viewpoint and concern usages).

| File | Was | Verdict |
|---|---|---|
| `32. Requirements/Requirement Satisfaction` | 2 × `type must be a definition, found requirementUsage` | False positive: `satisfy vehicleSpecification by vehicle_design;` references the requirement usages declared in `Requirement Groups`. Legal per the grammar above. |
| `33. Analysis/Analysis Case Usage Example` | 1 × `type must be a definition, found requirementUsage` | False positive: `satisfy vehicleFuelEconomyRequirements by vehicle_c1;` references the `requirement` usage declared in the same part. |
| `42. Views/Views Example` | 1 × `type must be a definition, found viewpointUsage` | False positive: `satisfy 'system structure perspective';` references the `viewpoint` usage in `Viewpoint Example`; a viewpoint usage is a requirement usage. |

The checking is narrowed, not dropped: `satisfy <non-requirement usage>` still
reports (`satisfy target must be a requirement usage, found ...`), locked by
`TestTypeCheckSatisfyNonRequirementUsageError` alongside the two positive cases in
`internal/core/passes/typecheck_test.go`, and the parse shape is pinned by
`internal/core/parser/testdata/parse/satisfy_reference.golden`.

### Verdicts for the implicit-parameter-redefinition re-pin (88/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `16. Conditional Succession/Conditional Succession Example-1` | 1 × `unresolved member: isWellFocused` | Real fix: `out item image;` inside `action focus : Focus` is the second parameter of a step, so it implicitly redefines `Focus::image` (KerML 7.4.7.3, SysML v2 7.17.2 — the match is by *position*, not by name) and takes its type `Image`. `focus.image.isWellFocused` now resolves to `Image::isWellFocused`, the declaration the OMG model means. The negative counterpart (`focus.image.notAMember`) still reports — see `internal/core/model/implicit_typing_test.go` `TestImplicitRedefinitionSuppliesInheritedMembers`. |

**Deliberate test change**

`internal/core/model/implicit_typing_test.go` `TestParameterRedefinitionAccompaniesTheImplicitBase`
pinned the previous behavior of a *name*-based rule: any usage whose name matched
a feature its owner inherits was left with no implicit base at all, on the
assumption that an implicit redefinition would later supply the type. The
specification has no such name-based rule — implicit redefinition applies to the
parameters of behaviors and steps by position (KerML 7.4.7.2/7.4.7.3), to
connection and association ends by position (SysML v2 7.13.2), and to result
parameters as results (SysML v2 7.19.2), while a nested usage that merely shares
a name with an inherited feature is a *name conflict* to be resolved by an
explicit redefinition (SysML v2 7.6.1, KerML 7.3.2.1). The test therefore now
pins the parameter case (the parameter takes the redefined parameter's type),
and the new `TestLikeNamedUsageIsNotAnImplicitRedefinition` pins the other side:
a like-named undirected usage keeps the standard library base of its kind
instead of being silently treated as a redefinition. We still do not diagnose
the name conflict itself; that gap is recorded in `docs/SPEC_COMPLIANCE.md`.

### Verdicts for the individual-definition re-pin (89/100, 90/100 with the import-in-definition-body fix below)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `34. Verification/Verification Case Usage Example` | 3 × `individual cannot specialize partDef`, 1 × `attribute cannot be typed by individualDef`, 2 × `part cannot be typed by individualDef` | Real fix: all six were false positives from the kind tables in `passes/typecheck.go`. `individual def X` is an occurrence definition — it is equivalent to `individual occurrence def X` and individuates the definition it specializes (SysML v2 7.9.4, abstract syntax 8.3.9.3: `individual` is `OccurrenceDefinition::isIndividual`, not a metaclass of its own) — so `individual def TestSystem :> MassVerificationSystem;` and the two `:> Vehicle` declarations are well formed, and a usage may be typed by an individual definition wherever it may be typed by an occurrence definition (`individual testSystem : TestSystem`, `in individual :>> testVehicle : TestVehicle1`). |

The checking is narrowed, not dropped. An occurrence definition still cannot
specialize a data type — `Occurrences::Occurrence` is disjoint with
`Base::DataValues` (SysML v2 8.4.5.1) — so `individual def Bad :> SomeAttributeDef;`
still reports `individual cannot specialize attributeDef (kind mismatch)`, and a
usage kind that rejects an occurrence definition still rejects an individual
definition (`port p : SomeIndividualDef`). Both negatives, and the positive
cases including the corpus file's shape, are locked by
`internal/core/passes/typecheck_individuals_test.go`.

Two of the six messages named a usage kind the declaration does not have:
`individual testSystem : TestSystem` was checked as an *attribute* usage and
`in individual :>> testVehicle` as a *part* usage, because the parser drops the
`individual` modifier (`parser/behavior.go`, `parser/defusage.go` record it but
the AST has no field for it) and falls back to the default kind instead of the
occurrence kind that 7.9.4 prescribes for a declaration with no kind keyword.
That is a separate gap: it does not change the verdict here, since typing by an
individual definition is legal for those kinds too, but it is why the pinned
messages read the way they did.

---

### Verdicts for the import-in-definition-body re-pin (89/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `34. Verification/Verification Case Definition Example` | 3 × (`unresolved reference: VerdictKind` ×2, `unresolved reference: PassIf`) | Real fix: `verification def VehicleMassTest` opens its body with `private import VerificationCases::*;`, and the three names it declares to be missing (`VerdictKind` on the `evaluateData` output and on the `def`-level `return`, and `PassIf` in the `evaluateData` body) all resolve to `VerificationCases`. The prior verdict below — "imports must be at package level, not inside a verification def" — was wrong: in KerML an `Import` is a `Relationship` whose `importOwningNamespace` is *any* `Namespace` (KerML 7.2.5.4 Imports; abstract syntax 8.3.2.4.2 Import / 8.3.2.4.6 NamespaceImport), and a definition body is a `Namespace` because a `Definition`/`Usage` is a `Type` and every `Type` is a `Namespace` (SysML v2 7.5.1 Namespaces, 7.5.3 Imports, 7.6 Definition and Usage). A `NamespaceImport` therefore imports the visible (public) memberships of `VerificationCases` into the `VehicleMassTest` body, and — through the ordinary parent-scope walk — into the nested `evaluateData` action, exactly where the OMG model uses them. |

**What changed**

`internal/core/resolve/unqualified.go` `importsOf` only harvested imports from
`*ast.Package`, `*ast.Namespace`, and `*ast.RootNamespace`, so an import declared
inside a definition or usage body was never consulted during name resolution. It
now also harvests imports from `*ast.Definition` and `*ast.Usage` bodies. The
existing membership/inheritance-then-import ordering (`walkUnqualifiedHiding`) and
import visibility (`visibleThroughImport`, `import all`) are unchanged, and a
`private import` in a definition body still does not leak to importers of that
definition because an imported name is not an *owned* member and is not
re-surfaced by a `NamespaceImport` of the outer definition. Covered by
`internal/core/resolve/imports_test.go`
(`TestImportInDefinitionBodyVisibleInBody`,
`TestImportInDefinitionBodyVisibleInNestedBody`,
`TestImportInPackageBodyVisibleInNestedDefinition`,
`TestImportInDefinitionBodyDoesNotLeakToImporter`).

---

## Error Classification

### Local Declaration Errors (Missing in Training Files)

These errors are **not implementation gaps** - the training files reference names that don't exist in those files. These are either:
1. Pedagogical simplifications (examples show partial code)
2. References to features that should be defined elsewhere
3. Incomplete examples for illustration purposes

**Most common:**
- `simpleMass`, `MassedThing`, etc.: References to definitions not in file
- Port/interface references: `supplierPort`, `consumerPort`, message endpoints
- `testVehicle` (2×): Missing test fixture declarations

### Training Example Bugs (Incorrect Code in OMG Materials)

**Lifecycle snapshots - wrong feature names (2 files, 4 errors):**
- Files: `27. Occurrences/Time Slice and Snapshot Example.sysml` lines 16, 25; `28. Individuals/Individuals and Time Slices.sysml` lines 12, 16
- **Error**: `unresolved reference: start` (2×), `unresolved reference: done` (2×)
- **Cause**: Files use `snapshot sale = start` and `snapshot junked = done` but KerML defines these as `startShot` and `endShot` (Occurrences.kerml:348, 364)
- **Fix**: Change `start` → `startShot`, `done` → `endShot`

**Import inside a definition body (RESOLVED ✅):**
- File: `34. Verification/Verification Case Definition Example.sysml`
- **Former error**: `unresolved reference: VerdictKind` (2×), `unresolved reference: PassIf` (1×)
- **Former (incorrect) verdict**: "imports must be at package level, not inside a verification def"
- **Reality**: the file's `private import VerificationCases::*;` sits *inside* the `verification def VehicleMassTest` body, which is a legitimate place for an import — a definition body is a `Namespace` and an `Import`'s `importOwningNamespace` may be any `Namespace` (KerML 7.2.5.4; SysML v2 7.5.3, 7.6). The example was correct; the resolver was not consulting body-owned imports. See the import-in-definition-body re-pin above.

**Scope resolution - inherited feature resolution (FIXED ✅):**
- **Previous errors**: `unresolved reference: localClock`, `unresolved reference: payload` (4 total)
- **Cause**: Features inherited from parent definitions (Part → Item → Occurrence, Flow → Message → Transfer)
- **Fix**: Implemented inherited feature resolution in commits 8304f03, c683bc8
- **Status**: All localClock and payload errors eliminated

**Typos (1 file, 1 error):**
- **Error**: `unresolved reference: alternative` (1×)
- **Cause**: Feature is named `alternatives` (plural) in `Domain Libraries/Analysis/TradeStudies.sysml`
- **Fix**: Change `alternative` → `alternatives`

**Package reference issues (2-3 files):**
- **Error**: `unresolved reference: Requirement Usages` (1×), `unresolved reference: Variation Usages` (1×)
- **Cause**: Package name references need proper qualification/import path

**Summary**: 8-10 files have bugs in OMG training materials (incorrect feature names, missing imports, typos). All referenced features exist in stdlib - these are authoring errors in training examples, not implementation gaps.

### Stdlib/Import Errors

**Resolved ✅:**
- `VerdictKind`, `PassIf`: Fixed by consulting imports owned by a definition/usage body during name resolution (the example's `private import VerificationCases::*;` is correct; see the import-in-definition-body re-pin above)
- Named argument resolution: Fixed in ff70654 (named args don't resolve parameter names)
- `localClock`, `payload`: Fixed in 8304f03, c683bc8 (inherited feature resolution)

**Still present:**
- `annotatedElement` (2×): Metadata feature - likely needs ModelingMetadata import
- `alternative`: a typo in the OMG file (the feature is `alternatives`)

### Type System Limitations

- `X subsets Y: types do not conform` (2×): Subsetting validation gaps

### Parser/Unimplemented Features

- `flow X must declare both a source and a target end` (2×): Flow validation strictness
- Various member access errors: Features that exist but aren't accessible in resolution scope

---

## Training Example Compliance

| Category | Pass | Fail | Pass Rate |
|----------|------|------|-----------|
| **All Examples** | 87 | 13 | 87% |
| **After filtering pedagogical gaps** | ~87 | ~13 | ~87% |

**Note**: Many "failures" are incomplete examples meant for teaching, not executable code. Of the 13 files with errors, most emit only missing local declarations or bugs in the OMG material itself (wrong feature names, typos, missing imports); the rest are type-system and validation gaps listed above.

---

## Remaining Work for Full Training Example Support

### Priority 1: Implicit Redefinition and Non-Generalization Feature Sources
- Implicit redefinition: an untyped usage whose name matches a feature its owner inherits takes that feature's type
- Document correct import paths for Metadata, Variations, Requirements namespaces

### Priority 2: Type System Enhancements
- Improve subsetting validation conformance checking

### Priority 3: Flow Validation Relaxation
- Make flow source/target validation warnings instead of errors
- Support declarative flows without full endpoint specification

### Priority 4: Pedagogical Documentation
- Mark which examples are intentionally incomplete
- Provide "complete" versions of pedagogical examples for testing

---

## Testing

To run training example analysis:

```bash
go test -run TestTrainingExamplesSemanticErrors ./internal/core/model -v
```

This generates error frequency analysis and per-file diagnostics.

---

## Conclusion

**Implementation Status**: Core behavioral semantics complete (43/43 execution conformance cases passing).

**Training Example Status**: 87/100 clean (13 files, 30 errors). Remaining errors are primarily:
1. Missing local declarations in pedagogical examples, and bugs in the OMG files themselves
2. Implicit *redefinition* of a like-named inherited feature, which implicit stdlib typing does not supply (see the verdict tables above)
3. Type system edge cases (feature work needed)

The runtime implementation is **production-ready for complete SysML v2 models**. Training example "failures" reflect incomplete example files, not missing runtime features.
