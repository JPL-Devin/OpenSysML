# The declared errata overlay

> **Labels.** This is an engineering record. A "wave" (or a "slice" within one) is a numbered
> development round of this project — chronological, with no meaning outside this repository — and
> `F<n>`, `K<n>` and `S<n>` name follow-up rows and diagnostic classes of
> [pilot-differential.md](pilot-differential.md). A reader who only wants the outcome can ignore them.

## The gap this closes

Every divergence row against the pinned pilot implementation carries one of the
four categories of [wave 11E](wave11e-decisions.md): *our defect*, *unimplemented
obligation*, *pilot limitation*, *adjudicated divergence*. A fifth case has no
category and no mechanism: **the OMG-published material is itself wrong**. Such a
row was written up in [omg-issues.md](omg-issues.md) and then stayed in the
divergence list for good, because the oracles read the published file exactly as
published — the only two outcomes available were "change our behaviour to match a
defective example" or "carry the row forever".

The overlay adds the missing one: the defect is declared, cited, and the
correction is applied to *what the oracles read a second time*, with both figures
reported.

## What it is

`internal/errata` is a registry of corrections to published reference material.
One entry is one line of one file:

| Field | Meaning |
|---|---|
| `ID` | the row in [omg-issues.md](omg-issues.md) documenting the defect |
| `Path`, `Line` | the defective location, relative to the repository root |
| `AsPublished` | that line's exact bytes, verbatim |
| `Corrected` | what the oracles read instead — **empty means documented without a correction** |
| `Citation` | the specification clause the published text violates |
| `Derivation` | one line deriving the defect from that clause |

The registry as it stands:

| ID | Location | Citation | Shape |
|---|---|---|---|
| F82 | `sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:38 | SysML v2 §9.8.9.1 | corrected — `22/2*25.4 + 110 [mm]` → `(22/2*25.4 + 110) [mm]` |
| F83 | `sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml`:25 | SysML v2 §9.8.9.1 | documented without a correction |
| F111 | `sysml-examples/Individuals Examples/AnalysisIndividualExample.sysml`:86 | KerML 7.4.9, 8.3.4.2 | corrected — `fuelConsumption : FuelEconomyAnalysis_1` → `fuelConsumption : FuelConsumption_1` |

## The invariants, all of them tests

- **The published corpus is never written to.** Corrections are applied to a copy
  under the oracle's own output directory (`Overlay.Materialize`), which is
  removed after the run; `examples/pilot-corpora/` and the pilot checkout are
  read-only to this mechanism. A test copies a root, corrects the copy and
  asserts the published tree is byte-identical afterwards.
- **An entry cannot rot.** `Overlay.Verify` reads each entry's file and fails
  unless `AsPublished` still matches the bytes on disk, so re-vendoring the
  corpus invalidates the entry loudly instead of silently skipping the
  correction.
- **No entry without provenance.** A missing citation, a missing derivation, a
  missing `omg-issues.md` row, a correction identical to the published text, a
  line that does not exist, or a path outside the published roots are all
  rejected by `Entry.Validate` and covered by a test.
- **Documented-only entries substitute nothing.** An entry with no `Corrected`
  text is carried for provenance; both figures keep the published line.
- **Errata are not a reclassification route.** The overlay changes no category in
  [wave 11E](wave11e-decisions.md) terms and no analyzer behaviour. F82 and F111
  stay true positives of ours; what the overlay records is that the *examples* are wrong.

## Both figures, and which one is the statement

Each oracle reports its census twice: as published, and with the corrections
applied. **The as-published figure is the conformance statement**; the
errata-applied one is a secondary diagnostic, and the generated block in
`README.md` and [architecture](../internals/architecture.md) says so in the same
sentence it prints them. Both come from the same run and the same committed
baseline (`internal/doccounts` reads the baselines' `errata` sections), so the
two figures cannot drift apart or be composed from different trees.

Measured on this branch with fresh caches:

| Oracle | As published | With the errata applied |
|---|---|---|
| `pilot-diff` | 353 files, 325 fully agreeing; 32 agreed, 26 only ours, 61 only the pilot's | 353 files, **327** fully agreeing; 32 agreed, **24** only ours, 61 only the pilot's |
| `pilot-xpect` | 428 `.xt` files, 1261 assertions, 1323 rows, 1295 agree, 28 disagree | identical — no declared correction lies under `build/pilot-xpect-corpus` |
| `pilot-reject` | 120 cases: 120 both reject, 0 only the pilot rejects | identical — no declared correction lies under `cmd/pilot-reject/testdata/negative` |

Where no correction applies, the oracle says so in that many words rather than
printing a coincidentally equal number: *no declared correction lies under
`<root>`, so the errata-applied corpus is byte-identical to the published one and
both figures coincide.*

## The pilot is run against the corrected text too

A correction that clears *our* diagnostic while the reference still reports at
that line is a finding, not a fix, so each corrected root is re-run through
**both** implementations and the per-entry outcome is printed:

```
F82 …/VehicleGeometryAndCoordinateFrames.sysml:38 ours 1->0, pilot 0->0:
  our diagnostic is cleared and the pilot is silent on both texts
F111 …/AnalysisIndividualExample.sysml:86 ours 1->0, pilot 0->0:
  our diagnostic is cleared and the pilot is silent on both texts
```

The pinned pilot performs no dimensional analysis and validates subsetting
conformance nowhere, so it is silent on both texts of both entries and no pilot
verdict has changed yet. When one does, the finding carries
`pilotVerdictChanged: true` and the report says which way it moved.

## Adding an entry

1. Adjudicate the row: is the *published material* wrong, or are we? If the
   answer is "we are", it is a defect of ours and does not belong here.
2. Write the `omg-issues.md` row: published text verbatim, the citation, the
   derivation, and the correction if the intended reading is unambiguous. Do not
   invent one to close a row — document it without a correction instead.
3. Add the `errata.Entry`, copying the published line byte-for-byte, trailing
   whitespace included.
4. `go test ./internal/errata`, then re-run the three oracles with fresh caches
   and `make docs-counts`.
