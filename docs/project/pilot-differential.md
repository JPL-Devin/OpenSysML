# Pilot Differential Diagnostics

## Overview

**Reference:** [SysML v2 Pilot Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation), release `2026-05` (`jupyter-sysml-kernel` 0.60.1) — the same release the training corpus is pinned to
**Wrapper:** [DeciSym/sysmlv2-validator](https://github.com/DeciSym/sysmlv2-validator) at commit `0d706e5ba1e9c56730cb8600ee43602906e12058`
**Provision:** `./scripts/download-pilot-validator.sh` (needs Java 21+ and Maven; writes `build/pilot-validator/`) and `./scripts/download-pilot-kerml-validator.sh` for the KerML side (writes `build/pilot-kerml-validator/`)
**Run:** `go run ./cmd/pilot-diff` (writes `build/pilot-diff/pilot-diff.txt` and `build/pilot-diff/pilot-diff.json`)
**Baseline:** the last committed run is [pilot-differential-baseline.json](pilot-differential-baseline.json), so a later run can be diffed against it
**Status:** advisory only — nothing here gates CI, and the harness reads the corpora without writing to them

[training-examples.md](training-examples.md) gates on
`internal/core/model/testdata/training_examples_expected.txt`, which is a snapshot of *our*
behavior: regenerating it records whatever the code now reports, so a regression re-baselines
as quietly as a fix. That gate answers "did we change?"; it cannot answer "are we right?".
This page is the other half: it asks the reference implementation the same question about the
same files and records where the two disagree.

It is a cross-check, not a gate, for a reason: the pilot is the reference implementation, but
not every difference is our bug (see the adjudications below — the pilot's own grammar is
stricter than ours in places, and its batch behavior produces artifacts of its own).

---

## Why this wrapper

Neither candidate is the pilot itself; both are thin CLIs over the pilot jars, which is
deliberate — a hand-written bridge into the pilot's Xtext internals would be our
interpretation of the reference rather than the reference.

| Candidate | Outcome |
|---|---|
| [DeciSym/sysmlv2-validator](https://github.com/DeciSym/sysmlv2-validator) | **Chosen.** Builds from a pinned commit with Maven 3.6.3 and Java 21 here (`mvn -Psetup-dependency initialize && mvn package`), and its `setup-dependency` profile downloads the pilot release itself, so the pilot version is pinned in the same place as the wrapper. Emits GNU-format `file:line:col: severity: message`. |
| [Fabi303/sysmlv2tool](https://github.com/Fabi303/sysmlv2tool) | **Not used — could not be built here.** Its directory mode is the better fit (one batch, one resource set), but it builds the pilot from a submodule through Tycho, and the build fails under the Maven available in this environment: `No implementation for org.eclipse.tycho.core.resolver.MavenTargetLocationFactory was bound`. Re-tried with Maven 3.9.9 without success. Left as a follow-up rather than faked. |

### The limitation this forces, and what the harness does about it

The DeciSym CLI recurses into directories, but it validates each file with a separate
`interactive.process(content, true)` call against one accumulating `SysMLInteractive` session
— it is sequential, not a single batch parse. Two consequences, both visible in the report:

- **Order matters.** A file that imports another only resolves if that other file was
  processed first. `cmd/pilot-diff/order.go` therefore topologically sorts each corpus root by
  its imports (parsing every file with our own parser to find what it declares and imports)
  before handing the list to the validator, and falls back to lexicographic order inside an
  import cycle.
- **Names accumulate across files.** Because every file lands in the same session, two files
  declaring the same package name collide. That is exactly the `Duplicate of other owned
  member name` warning below — an artifact of the wrapper, not a verdict on the model.
  Files that share a basename are also split into separate validator invocations
  (`batchByBaseName`), since the wrapper reports diagnostics by basename only.

Our side is run the way the training-examples gate runs it: every file in a root is opened
into one workspace *before* any diagnostic is requested, so cross-file imports resolve.

### The KerML side of the bridge

The DeciSym CLI is `.sysml`-only, so the KerML root is validated by a sibling program,
[`scripts/pilot-kerml-validator/ValidateKerML.java`](../../scripts/pilot-kerml-validator/ValidateKerML.java),
built against the *same* pinned pilot jar by `./scripts/download-pilot-kerml-validator.sh`
(which sources `scripts/pilot-pin.sh`, provisions the SysML wrapper first if its jar is
missing, and writes only under `build/pilot-kerml-validator/`). It is ~150 lines of glue and
contains no rule of its own: it registers `KerMLStandaloneSetup`
(`createInjectorAndDoEMFRegistration`), extends the pilot's own `SysMLUtil` to load
`sysml.library` and the corpus into one `ResourceSet`, then asks the injected Xtext
`IResourceValidator` — the pilot's `KerMLResourceValidator`, driving the pilot's
`KerMLValidator` — for `validate(resource, CheckMode.ALL, CancelIndicator.NullImpl)`, and
prints each `Issue` in the same GNU format the DeciSym wrapper emits, so `cmd/pilot-diff`
reads both with one parser. The verdicts are therefore the reference's.

Two differences from the SysML side, both in the bridge's favour:

- **One batch, one resource set.** Every corpus file is read and indexed before any file is
  validated, so there is no ordering to emulate (`order.go` is not used for this root) and no
  cross-file name accumulation, which is why the `Duplicate of other owned member name`
  artifact (P4) has no KerML counterpart.
- **Paths, not basenames.** Diagnostics are printed relative to the corpus root, so
  same-basename files need no batching and are attributed exactly.

EMF renders object references with an identity hash code and an absolute `file:` URI, which
would differ between runs and machines; the bridge rewrites those to the display path, so
repeated runs are byte-identical.

---

## Corpus roots

| Root | Directory | Provisioned by |
|---|---|---|
| `training` | `examples/sysml-v2-training` (`sysml/src/training`) | `scripts/download-training-examples.sh` |
| `pilot-examples` | `examples/pilot-corpora/sysml-examples` (`sysml/src/examples`) | `scripts/download-pilot-corpora.sh` |
| `pilot-validation` | `examples/pilot-corpora/sysml-validation` (`sysml/src/validation`) | `scripts/download-pilot-corpora.sh` |
| `kerml-examples` | `examples/pilot-corpora/kerml-examples` (`kerml/src/examples`) | `scripts/download-pilot-corpora.sh` |
| `testdata` | `testdata` | vendored |
| `examples` | `examples`, less the downloaded corpora | vendored |
| `probes` | `cmd/pilot-diff/testdata` | vendored |

The OMG corpora are not vendored, for the same licensing reason as the training corpus, and the
pilot release they are fetched at is pinned once in `scripts/pilot-pin.sh` — the same pin the
validator build reads, so corpus and reference can never come from different releases. Each
corpus directory is left alone when it already exists; remove it to re-download. A root whose
directory is absent is skipped with a warning.

### KerML: how the reference validates it

`kerml/src/examples` (58 `.kerml` files) is now a root. The earlier reading of this page —
that the pilot has no KerML validation to invoke — confused *entry points* with *validators*.
The two dead ends were real but narrower than they looked:

- The DeciSym wrapper refuses any other extension outright —
  `Error: File must have .sysml extension: <file>.kerml` — and its directory mode only collects
  `.sysml`. Renaming a `.kerml` file to `.sysml` is not a substitute either: the wrapper then
  parses KerML with the SysML grammar, so `class Entry { ... }` becomes `no viable alternative
  at input 'Entry'`, which measures the grammar mismatch, not agreement.
- `KerML2XMI` / `KerML2JSON` are indeed silent on malformed input, but that is because they
  parse, transform and serialize without ever calling `IResourceValidator` — not evidence that
  the reference has no KerML checks.

The pinned `jupyter-sysml-kernel` jar in fact ships the KerML twin of everything the SysML
comparison already consumes: `org/omg/kerml/xtext/validation/KerMLValidator.class` (~88 KB,
against ~79 KB for `org/omg/sysml/xtext/validation/SysMLValidator.class`),
`KerMLResourceValidator.class`, `AbstractKerMLValidator.class`, and
`org/omg/kerml/xtext/KerMLStandaloneSetup.class`. What was missing was only a CLI, which the
bridge above supplies (F10).

The oracle was sanity-checked before any comparison was drawn from it. On
`Address Book Example/AddressBookModel.kerml` it reports nothing and exits 0; on a malformed
file (`package Broken {` / `part def`) it exits 1 with

```
malformed.kerml:2:8: error: no viable alternative at input 'def'
malformed.kerml:2:11: error: no viable alternative at input '<EOF>'
```

and on a file that parses but does not resolve
(`feature x : NoSuchTypeAtAll; classifier C specializes AlsoMissing;`) it reports unresolved
references, so it is exercising name resolution too, not just the parser.

---

## What is compared

Each diagnostic is normalized to a tuple:

```
(file, line, severity, coarse category)
```

Message wording will never match between two implementations, so **message text is never
compared** — it is carried into the text report as an example for human adjudication only.
Categories are coarse and deliberately few:

| Category | Meaning |
|---|---|
| `syntax` | the file did not parse as written |
| `unresolved-reference` | a name did not resolve |
| `kind-mismatch` | a declaration used where its metaclass is not allowed |
| `multiplicity` | bounds, cardinality, subsetting caps |
| `units` | quantity/unit incompatibility |
| `unmapped` | **no rule claimed this message** |

`unmapped` is load-bearing: a message that does not fit a category stays in its bucket and is
also listed in a table of its own, rather than being mapped to something adjacent to make the
report look tidy. Tuples are compared as multisets, so when one side reports a tuple three
times and the other twice, two are agreement and the third stays a disagreement.

Buckets per file: **agreement**, **only ours** (candidate false positives), **only the
pilot's** (candidate gaps), and **severity-only** — a (line, category) both implementations
flag with different severities. The last exists so such a pair is neither counted as agreement
nor double-counted as two independent disagreements.

---

## Results (pilot `2026-05`, 335 files)

| Root | Files | Fully agreeing | Ours | Pilot | Agreed | Severity-only | Only ours | Only pilot |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 100 | 0 | 0 | 0 | 0 | 0 | 0 |
| `examples/pilot-corpora/sysml-examples` | 98 | 40 | 327 | 0 | 0 | 0 | 327 | 0 |
| `examples/pilot-corpora/sysml-validation` | 56 | 39 | 65 | 0 | 0 | 0 | 65 | 0 |
| `examples/pilot-corpora/kerml-examples` | 58 | 10 | 439 | 6 | 0 | 0 | 439 | 6 |
| `testdata` | 10 | 2 | 26 | 54 | 20 | 3 | 3 | 31 |
| `examples` | 12 | 4 | 0 | 121 | 0 | 0 | 0 | 121 |
| `cmd/pilot-diff/testdata` (probes) | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 |
| **Total** | **335** | **196** | **857** | **181** | **20** | **3** | **834** | **158** |

Only the `kerml-examples` row is new here. The other rows moved because of work merged since
the baseline was written, not because of this change: re-running the previous revision of
`cmd/pilot-diff` on `origin/main` reproduces every non-KerML root of this table exactly,
per-file and not just per-total, and the SysML side of this run is identical to it.

The ordering fix removed all 539 pilot-only diagnostics from `pilot-examples`: the reference
now processes `SysML v2 Spec Annex A SimpleVehicleModel.sysml` before its importer,
`Annex_A_VehicleViews.sysml`, so the missing `SimpleVehicleModel` namespace at line 2 is no
longer reported by the pilot. The importer still has OpenSysML-only syntax diagnostics, which
remain part of the parser-gap follow-up below.

`pilot-validation` was unchanged by the ordering fix: it contributed 121 only-ours syntax
diagnostics in the pre-merge comparison, and 65 now.

The headline is the first row: on the 100-file OMG training corpus the pilot reports
**nothing at all**, and so do we. That is the corpus written to be valid, and it is the row
that most directly answers "are we right?".

Counts that moved since the committed baseline, and why:

| Count | Was | Now | Reason |
|---|---|---|---|
| `kerml-examples`: everything | — | 58 files, 439 only ours, 6 only pilot | New root: the reference can validate KerML (F10). Adjudicated below. |
| `pilot-examples`: only ours | 334 | 327 | Merged since the baseline: the four parser productions and the keyword-as-name work. Not re-attributed line by line here. |
| `pilot-validation`: only ours | 118 | 65 | Same merged work. |
| `testdata`: files / only ours / only pilot / severity-only | 9 / 5 / 48 / 1 | 10 / 3 / 31 / 3 | F2: our fixtures now carry an explicit import visibility, so the pilot parses them instead of abandoning the body — which retires its cascade and the two `unresolved reference: Nowhere` rows below, and adds the two `import_no_visibility.sysml` severity-only rows that F2's warn-not-error decision implies. The extra file is that fixture. |
| `examples`: only pilot | 140 | 121 | Same F2 cascade reduction. |

None of those SysML-side movements come from this change: the previous revision of
`cmd/pilot-diff` run on `origin/main` reproduces them exactly.

The KerML row is the harshest in the table, and it is almost entirely ours: 439 only-ours
against 6 only-pilot, with 10 of 58 files fully agreeing. Where the reference validates the
corpus its authors wrote for it, we reject notation it accepts — adjudicated below. This is
reported, not fixed: this page is a comparison.

The `testdata`/`examples` rows are not a like-for-like verdict on our checker. `testdata/` and
`examples/` are largely *our* fixtures — several are deliberately malformed negative fixtures,
and many are written in notation the pilot's grammar rejects outright, after which its error
recovery cascades. Their 152 pilot-only diagnostics are therefore dominated by a handful of
root causes, adjudicated next.

---

## Adjudications

### Only ours — candidate false positives (3, SysML side)

The three diagnostics below are the adjudicated only-ours set outside the KerML root, which
has its own tables further down. The remaining 392 SysML-side only-ours diagnostics are not
adjudicated in this pass. The 121 `pilot-validation` syntax-only
discrepancies from the pre-merge comparison and the bulk of the `pilot-examples` syntax-only
discrepancies come from four missing productions: `connect a to b { ... }`, `flow a.x to b.y
{ ... }`, anonymous `interface a.p to b.q`, and `accept` on an action usage declaration
(`action got accept e : E { ... }`). Representative corpus locations are
`02-Parts Interconnection/2a-Parts Interconnection.sysml:97,157,162` and
`03-Function-based Behavior/3a-Function-based Behavior-1.sysml:58,102,112`. The child session is
fixing those parser gaps; they remain open here.

The two keyword-as-name rows that stood here are **fixed** (F1): `on` appears as a literal in
none of the pilot's grammars, and `var` only in `KerML.xtext` (`BasicFeaturePrefix`), so both
are now matched contextually and are names everywhere else. The four diagnostics they produced
are gone from the three files listed in the movement table above.

| Files | Diagnostic | Verdict |
|---|---|---|
| ~~`passes/errors.sysml:4`, `resolve/errors.sysml:4`~~ | ~~`unresolved reference: Nowhere`~~ | **No longer a disagreement.** These were negative fixtures where the pilot was silent only because a bare `import` earlier in the same file broke its parse before it got there (see P1). Since F2 gave our fixtures an explicit visibility, the pilot parses them and reports `Nowhere` too: both rows are now agreement. |
| `passes/constraints.sysml:2,3` | `A`/`B` `participates in a specialization cycle` (`unmapped`) | **Ours is right**: `part def A specializes B; part def B specializes A;`. The pilot reports nothing for the cycle. Recorded as `unmapped` because no pilot message pattern corresponds; whether the pilot has no such check or suppressed it is not established here. Follow-up F4. |
| `passes/constraints.sysml:9` | `multiplicity lower bound exceeds upper bound on lo` | **Ours is right**: `part lo [5..2];`. No pilot counterpart. |

### KerML — only ours (439)

Every one of the 439 falls in one of the classes below; the counts sum to 439. Verdicts:
**436 ours**, **3 genuine ambiguity**, and none attributable to the bridge — it validates one
batch in one resource set, so it has no ordering or name-accumulation artifact to produce.
Nothing here is fixed in this PR; each class carries its follow-up.

| # | Class | Count | Verdict |
|---|---|---:|---|
| K1 | `featured by` is not parsed: `expected a body member: 'featured' relates the declaration written before it, so a member cannot begin with it` (43), then `expected '{' or ';' after declaration` (95) and `expected a namespace member` (169) as the enclosing bodies unwind | 307 | **Ours (over-restriction).** KerML's featuring relationship (`member feature inCart: ShoppingCart[0..1] featured by Product_Account;`) is notation the reference accepts silently. One unparsed keyword produces 70% of the root's diagnostics: `Association Examples/ProductSelection_N_ary.kerml:38,40,42` cascade to `:51,53,54`. Follow-up F11. |
| K2 | Other KerML notation we reject: `expected a body member` on n-ary connector end lists (36), `expected 'then' between connector ends` on a typed/redefining succession (8), `"at"`/`"while"`/`"merge"` `is a reserved keyword` inside `expr` bodies (8), `expected a name` (6), `expected '{' or ';'` (3) | 61 | **Ours (over-restriction).** `connector ps1 : ProductSelection (myCart, products, myAccount);` (`Association Examples/ProductSelection_N_ary.kerml:122,124`), `succession redefines p_before_d : MyPaint_Before_Dry_Link [1] first paint then dry;` (`KerML Spec Annex A Examples/A-3-6-Sequences.kerml:58,60`), and `expr at { ... }` / `expr while { ... }` (`Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:16,25`) are all accepted by the reference. The keyword rows are the KerML half of F8. Follow-up F11. |
| K3 | `unresolved reference` / `unresolved member` | 43 | **Ours (name resolution).** Three shapes: inherited library features reached through implicit specialization (`portion focusedState: Camera subsets timeSlices;`, `Behavior Examples/Camera.kerml:4,5`); a package declared in a sibling corpus file (`private import OneToOneConnectorsExecution::MyWheel;` at `KerML Spec Annex A Examples/A-3-5-TimingForStructures.kerml:24`, declared at `A-3-3-OneToOneConnectors.kerml:21`); and members named through a feature chain (`succession step1 then camera.focusedState;`, `Behavior Examples/TakePicture.kerml:16,17`). Some are plausibly downstream of K1/K2 in the same or an imported file, which is why they are one class and not one verdict per line. Follow-up F12. |
| K4 | SysML-shaped semantic checks firing on KerML: `only a definition may specialize; found a usage` (21), `type must be a definition, found attributeUsage` (2), `metaclass cannot specialize metaclass (kind mismatch)` (1), `rollsOn (typed by MyWheel) redefines rollsOn (typed by Wheel): types do not conform` (1) | 25 | **Ours.** KerML has no definition/usage split, so the first row misfires on ordinary declarations (`class Person specializes Object`, `Individuals Examples/JohnIndividualExample.kerml:4,12,34`; `Mass Roll-up Example/Vehicles_3.kerml:32`; `Simple Tests/Inheritance.kerml:21`). `metaclass <atom> AtomMetadata specializes Metaobject` (`KerML Spec Annex A Examples/A-2-Atoms.kerml:11`) is a metaclass specializing a metaclass, which the reference allows. The conformance row misses `classifier MyWheel unions MyWheel1, MyWheel2;` as a supertype of `Wheel` (`KerML Spec Annex A Examples/A-3-2-WithoutConnectors.kerml:32`). Follow-up F13. |
| K5 | `x`/`y`/`z` `participates in a specialization cycle` (`unmapped`) | 3 | **Genuine ambiguity.** `feature x :> z; feature y :> x; feature z :> y;` in `Simple Tests/Circular.kerml:9-11` is a cycle, and the reference reports nothing — the same silence as `passes/constraints.sysml:2,3` on the SysML side. Whether the reference has no such check or suppresses it is still not established: F4, now with a KerML witness the corpus's own authors committed. |

### KerML — only the pilot (6)

| # | Class | Count | Verdict |
|---|---|---:|---|
| K6 | `The opposite features 'owningType' of '…DisjoiningImpl{…}' and 'ownedDisjoining' of '…{…}' do not refer to each other` | 6 | **Pilot artifact**, `unmapped`. Raised on `disjoint from` declarations in `KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9`, `Simple Tests/Classifiers.kerml:13`, `FeatureChains.kerml:31`, `Features.kerml:20`, `Inverses.kerml:3`, `Types.kerml:31`. It is an EMF `eOpposite` consistency complaint about the reference's own in-memory graph — it names `…Impl` objects and resource fragments, not model elements — so it is a statement about the pilot's transformation, not about the models, which are the pilot's own examples. Follow-up F14. |

### Severity-only (3)

One is adjudicated; the two added by work merged since the baseline are not re-adjudicated in
this pass, which is a comparison of the KerML root.

| File | Verdict |
|---|---|
| `passes/import_no_visibility.sysml:8,12` | The two rows added since the baseline, both a direct consequence of the F2 decision below: we report a bare `import` as a `warning` where the pilot's grammar makes it an `error`. Deliberate, and recorded here rather than re-argued. |
| `passes/constraints.sysml:6` | Both flag `part few subsets cap [0..10];` under `cap [0..3]` at the same line and category. We report `error`; the pilot reports `warning` (`Subsetting/redefining feature should not have larger multiplicity upper bound`). A real difference in strictness, kept in its own bucket rather than being counted as two disagreements. |

### Only the pilot — candidate gaps (152, SysML side)

The 539 pilot-only diagnostics that were previously concentrated in `pilot-examples` were an
ordering artifact and are resolved. The remaining 152 SysML-side pilot-only diagnostics are the
same `testdata`/`examples` issues as before, at the lower counts the merged import-visibility
and keyword work left behind.

Grouped by root cause. The pilot's own grammar
(`org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext`) is quoted where it settles the
question.

| # | Class | Count (approx.) | Verdict |
|---|---|---|---|
| P1 | `mismatched input 'import' expecting '}'` / `missing EOF at 'import'` on a bare `import X::*;` | 10 of our 21 `testdata`/`examples` files | **Pilot is stricter, and its grammar is explicit**: `fragment ImportPrefix returns SysML::Import : visibility = VisibilityIndicator 'import' ...` — visibility is *mandatory* for an import, unlike `MemberPrefix`, where it is optional. `private import X::*;` parses cleanly. Whether the specification's concrete syntax makes visibility mandatory too is not settled here, so this is not booked as our bug: follow-up F2. **This is also the single largest cascade source** — once the import fails, the pilot abandons the enclosing body, which produces most of the `no viable alternative`, `extraneous input '}' expecting EOF`, `missing EOF`, `Couldn't resolve reference to Type 'Real'` and `A usage must be typed by definitions.` entries downstream. |
| P2 | `no viable alternative at input '<name>'` on `namespace N;` inside a package body | 4 files | **Ours is wrong (over-acceptance).** `namespace` is a KerML keyword; the pilot's `DefinitionElement` list has no namespace declaration, so `.sysml` notation has none. We parse it. Follow-up F3. |
| P3 | `no viable alternative at input 'region'` (`orthogonal-regions-demo.sysml`) | 1 file | **Ours is wrong (over-acceptance).** SysML v2 spells orthogonal regions as a `parallel` state body (`';' \| ( isParallel ?= 'parallel' )? '{' StateBodyPart '}'`); there is no `region` keyword. We accept one. Follow-up F3. |
| P4 | `Duplicate of other owned member name` (warning) | 25 | **Harness/wrapper artifact**, `unmapped`. The wrapper feeds every file of a root into one accumulating interactive session, so identically-named packages in different files collide. Not a statement about any model. |
| P5 | `Bound features should have conforming types`, `Must have a Boolean result`, `Must have at least two related elements`, `An attribute must be typed by attribute definitions.` | 23 | **Mostly downstream of P1/P2/P3**: with the imports or the enclosing body broken, the pilot type-checks a partially-recovered model. Not adjudicated individually; the honest reading is that these become meaningful only once P1–P3 are resolved and the files re-run. |
| P6 | `Must be an accessible feature (use dot notation for nesting)`, `Cannot identify flow end (use dot notation)`, `Must be model-level evaluable`, `Must invoke a behavior or a behavioral feature` | 9 |
| P7 | K6, the KerML `eOpposite` complaint | 6 | **`unmapped`, unadjudicated.** These are pilot checks with no counterpart of ours and no obvious category; they are candidate gaps worth reading file by file. Follow-up F5. |

### Unmapped messages, verbatim

Recorded so the categorisation's debt is visible rather than hidden:

| Side | Message | Count |
|---|---|---|
| pilot | `Duplicate of other owned member name` | 25 |
| pilot | `Must be an accessible feature (use dot notation for nesting)` | 5 |
| pilot | `Cannot identify flow end (use dot notation)` | 2 |
| pilot | `Must be model-level evaluable` | 1 |
| pilot | `Must invoke a behavior or a behavioral feature` | 1 |
| pilot | `The opposite features 'owningType' … do not refer to each other` (K6, one row per file) | 6 |
| opensysml | `only a definition may specialize; found a usage` (K4) | 21 |
| opensysml | `<name> participates in a specialization cycle` | 6 |
| opensysml | `interface Mounting connects ports AxleMountIF and WheelHubIF, whose directed features are not conjugate; one end usually names the conjugate port (~AxleMountIF)` | 1 |
| opensysml | `name conflict: text is already the name of the inherited feature ModelingMetadata::Issue::text` | 1 |
| opensysml | `packet data field redefines packet data field, but packet data field is not an inherited member of Thermal Data Packet` | 1 |

---

## The alias case that motivated this

The reported defect was `unresolved reference: length` on

```sysml
part def AvionicsLRU :> Box {
    :>> length = 100 [mm];
}
```

where `ShapeItems::Box` is `alias Box for RectangularCuboid`, so `length` is only reachable if
aliases are followed through type relationships. The model is now a probe in the corpus:
[`cmd/pilot-diff/testdata/alias-supertype-lru.sysml`](../../cmd/pilot-diff/testdata/alias-supertype-lru.sysml).

| Implementation | Result |
|---|---|
| Pilot `2026-05` | **Accepts it** — zero diagnostics. |
| OpenSysML at the merge base of #331 (`e81da048`) | `error: part cannot specialize alias (kind mismatch)` — the same defect class as the reported message, surfacing one tier earlier once the model is reduced to this shape. |
| OpenSysML on `main` (with [#331](https://github.com/JPL-Devin/OpenSysML/pull/331)) | **Accepts it** — zero diagnostics, agreeing with the pilot. |

So #331 is the fix that closed the motivating discrepancy, and this harness is now the way
that class of behavior is checked against the reference instead of against our own snapshot.
The probes root exists to keep such cases in the comparison: the OMG corpus does not contain
one.

---

## Follow-ups (not fixed here — this PR is advisory)

| # | Follow-up |
|---|---|
| ~~F1~~ | **Done.** Keyword-as-name for `on` and `var`. The scope came from the pilot's grammars rather than the failing files: `on` is a literal in none of `KerML.xtext`, `SysML.xtext` or `Expr.xtext` — the premise that it is a trigger keyword (`accept ... on ...`) was wrong — and `var` is a literal only in `KerML.xtext` `BasicFeaturePrefix` (`isVariable ?= 'var'`). Both are now contextual, like `point`. |
| F8 | The other words we reserve that appear as a literal in none of the pilot's grammars: `choice`, `decision`, `deep`, `defer`, `done`, `final`, `history`, `initial`, `junction`, `region`, `shallow`. Unlike `on` and `var`, each is read as a keyword by our parser in a position of its own (mostly the state-machine notation of [grammar/README.md](../reference/grammar/README.md), some of which is an OpenSysML invention), so each needs its own contextual rule and its own decision about whether the notation stays. `done` is the pressing one: it is a *name* in five files of the normative OMG library (`Systems Library/Actions.sysml`, `Items.sysml`, `Parts.sysml`, `States.sysml`, `UseCases.sysml`), where we report it — see `TestStdlibReservedKeywordNames`. |
| F9 | Contextual keywords are neither highlighted nor completed: the VS Code grammars and the LSP keyword completion are generated from `lexer.Keywords()`, which `point`, `chain`, `on` and `var` are deliberately absent from. `var` is real KerML notation, so a second list of contextual words for those two surfaces would restore it without reserving it. |
| ~~F2~~ | **Done.** A bare `import` (no visibility) is non-conforming: the pilot's grammars make the indicator mandatory — `fragment ImportPrefix returns SysML::Import : visibility = VisibilityIndicator 'import' ...` with no `?`, unlike the sibling `MemberPrefix` (`KerML.xtext:169-172`, `SysML.xtext:241-244`) — and all 574 imports across the 254 OMG-authored corpus files carry one. Decision: **warn, not error** (`passes/import_visibility.go`, `LevelSyntax`, code `syntax/import-visibility`, spanned on the `import` keyword). The form is unambiguous to parse, so hard-failing would reject existing models over notation; the warning surfaces the non-conformance without gating the higher tiers. `expose` is exempt — the pilot grammar gives it implicit `protected` visibility (`SysML.xtext:2366-2372`). Our own fixtures now write an explicit visibility, with `testdata/passes/import_no_visibility.sysml` kept to lock the warning in. |
| F3 | Over-acceptance in our parser: `namespace N;` and `region` in a state body are not SysML v2 notation. |
| F4 | Specialization cycles: confirm whether the pilot checks them at all, and categorise the diagnostic (currently `unmapped`). |
| F5 | Read the P6 pilot checks (accessible-feature, flow-end, model-level-evaluable, behavior-invocation) file by file; each is a candidate gap in our constraint tier. |
| F6 | Build `Fabi303/sysmlv2tool` (needs a Tycho-capable Maven) and re-run with true single-batch loading, which would eliminate the P4 artifact and the ordering machinery. |
| F7 | Add [Sensmetry `syside check`](https://github.com/sensmetry/syside) as an *additional* cross-check. It is a different implementation, not the reference, so it can only corroborate — never adjudicate. |
| ~~F10~~ | **Done.** The pinned pilot release *does* ship KerML validation — `org/omg/kerml/xtext/validation/KerMLValidator.class` and `KerMLStandaloneSetup.class` — and what was missing was only a CLI. `scripts/pilot-kerml-validator/ValidateKerML.java` supplies one over the pilot's own `IResourceValidator`, sanity-checked on malformed, unresolvable and known-good input, and `kerml-examples` is a root. |
| F11 | KerML notation we reject and the reference accepts (K1, K2): `featured by`, n-ary connector end lists, `first … then` on a typed/redefining succession, and `at`/`while`/`merge` as names in `expr` bodies. K1 alone accounts for 307 of the root's 439 diagnostics. |
| F12 | KerML name resolution (K3): inherited library features reached through implicit specialization, packages declared in a sibling corpus file, and members named through a feature chain. Re-measure after F11 — part of this class is plausibly cascade. |
| F13 | Checks that assume the SysML definition/usage split and fire on KerML (K4), plus `unions` as a supertype in redefinition conformance. |
| F14 | The pilot's `eOpposite` complaint on `disjoint from` (K6): confirm against the reference's `KerMLResourceValidator`/`ElementUtil.transformAll` whether the batch bridge can avoid it, or report it upstream. |

---

## Re-running and diffing

```sh
./scripts/download-training-examples.sh   # the OMG training corpus (pinned 2026-05)
./scripts/download-pilot-corpora.sh       # the other OMG corpora, same pin
./scripts/download-pilot-validator.sh     # the pilot validator (pinned wrapper + release)
./scripts/download-pilot-kerml-validator.sh  # the KerML oracle, same pin
go run ./cmd/pilot-diff                   # writes build/pilot-diff/{pilot-diff.txt,pilot-diff.json}
diff <(jq -S . docs/project/pilot-differential-baseline.json) \
     <(jq -S . build/pilot-diff/pilot-diff.json)
```

The JSON carries tuples and counts but no message text, so it diffs cleanly; the text report
carries the messages for adjudication. When the baseline is refreshed, the verdicts above must
be re-adjudicated the same way [training-examples.md](training-examples.md) requires — a
moved count is a claim about one of the two implementations, and it needs a reason.
