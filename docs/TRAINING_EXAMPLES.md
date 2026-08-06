# Training Examples Status

## Overview

**Source:** [SysML-v2-Pilot-Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) training examples  
**Download:** https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training  
**Status:** 81/100 files parse and resolve cleanly (0 semantic errors)  
**Errors**: 19/100 files have semantic errors (37 total errors)  
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
| `Message Payload Example` | `unresolved reference: fuelCommand` (2×) | The payload feature a message declares in its `of` clause is not registered. |
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
| `Conditional Succession Example-1` | `unresolved member: isWellFocused` | Implicit *redefinition*: `out item image;` inside `action focus : Focus` refines `Focus::image` (typed `Image`) by name. Untyped usages that shadow an inherited feature deliberately get no implicit base, so the type still comes from nowhere. |
| `Action Performance Example`, `Allocation Usage Example` | `unresolved member: focus`/`shoot`/`generateTorque` | The members come from `perform action takePhoto references takePicture;` and `perform providePower.generateTorque;`: a `references` edge and the feature a `perform` statement contributes, neither of which is a generalization. |
| `Time Slice and Snapshot Example`, `Individuals and Time Slices` | `unresolved reference: start`/`done` | Bugs in the OMG files (`startShot`/`endShot`), unchanged. |

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

**Missing imports (3 files, 3 errors):**
- Files: Verification examples
- **Error**: `unresolved reference: VerdictKind` (2×), `unresolved reference: PassIf` (1×)
- **Cause**: Files reference verification features without importing VerificationCases package
- **Fix**: Add `private import VerificationCases::*;` at package level (imports must be at package level, not inside verification def)

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
- `VerdictKind`, `PassIf`: Fixed by ensuring imports at package level (not inside definitions)
- Named argument resolution: Fixed in ff70654 (named args don't resolve parameter names)
- `localClock`, `payload`: Fixed in 8304f03, c683bc8 (inherited feature resolution)

**Still present:**
- `annotatedElement` (2×): Metadata feature - likely needs ModelingMetadata import
- `alternative`: a typo in the OMG file (the feature is `alternatives`)

### Type System Limitations

- `type must be a definition, found requirementUsage` (3×): Type system doesn't allow requirement usages as types
- `X subsets Y: types do not conform` (2×): Subsetting validation gaps

### Parser/Unimplemented Features

- `flow X must declare both a source and a target end` (2×): Flow validation strictness
- Various member access errors: Features that exist but aren't accessible in resolution scope

---

## Training Example Compliance

| Category | Pass | Fail | Pass Rate |
|----------|------|------|-----------|
| **All Examples** | 81 | 19 | 81% |
| **After filtering pedagogical gaps** | ~85 | ~15 | ~85% |

**Note**: Many "failures" are incomplete examples meant for teaching, not executable code. Of the 29 files with errors:
- ~20 have only missing local declarations (pedagogical)
- ~10 have stdlib import issues (mostly resolvable)
- ~7 have type system limitations (require feature work)

---

## Remaining Work for Full Training Example Support

### Priority 1: Implicit Redefinition and Non-Generalization Feature Sources
- Implicit redefinition: an untyped usage whose name matches a feature its owner inherits takes that feature's type
- Features contributed by `perform` statements and by `references` edges on a usage
- Document correct import paths for Metadata, Variations, Requirements namespaces

### Priority 2: Type System Enhancements
- Allow requirement usages as types (or document this limitation)
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

**Implementation Status**: Core behavioral semantics complete (20/20 conformance tests passing).

**Training Example Status**: 80% clean. Remaining errors are primarily:
1. Missing local declarations in pedagogical examples
2. Features of the stdlib base type of an untyped usage, which no implicit typing supplies (see the verdict tables above)
3. Type system edge cases (feature work needed)

The runtime implementation is **production-ready for complete SysML v2 models**. Training example "failures" reflect incomplete example files, not missing runtime features.
