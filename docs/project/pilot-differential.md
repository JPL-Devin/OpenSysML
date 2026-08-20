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

`kerml-examples` is collected as KerML; every other root is collected as SysML, which leaves our
own `.kerml` fixtures out of the comparison (see the known limitation below).

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

**Known limitation:** a root has one language, so only `kerml-examples` is compared as KerML.
Our own 11 `.kerml` fixtures — `testdata/lex/basic.kerml` and
`examples/parser_features_demo_*.kerml` — sit in roots collected as `.sysml` and are therefore
*not* compared, even though the bridge could now validate them and our side already analyses
them. The counts for `testdata` and `examples` below are SysML-only for that reason. Comparing
them means letting one root collect both extensions and dispatching each language to its own
oracle: follow-up F34.

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

## Results (pilot `2026-05`, 338 files)

| Root | Files | Fully agreeing | Ours | Pilot | Agreed | Severity-only | Only ours | Only pilot |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 100 | 0 | 0 | 0 | 0 | 0 | 0 |
| `examples/pilot-corpora/sysml-examples` | 98 | 40 | 316 | 0 | 0 | 0 | 316 | 0 |
| `examples/pilot-corpora/sysml-validation` | 56 | 39 | 59 | 0 | 0 | 0 | 59 | 0 |
| `examples/pilot-corpora/kerml-examples` | 58 | 10 | 439 | 6 | 0 | 0 | 439 | 6 |
| `testdata` | 10 | 2 | 27 | 54 | 20 | 4 | 3 | 30 |
| `examples` | 12 | 4 | 40 | 121 | 0 | 12 | 28 | 109 |
| `cmd/pilot-diff/testdata` (probes) | 4 | 1 | 6 | 0 | 0 | 0 | 6 | 0 |
| **Total** | **338** | **196** | **887** | **181** | **20** | **16** | **851** | **145** |

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
diagnostics in the pre-merge comparison, and 59 now.

The headline is the first row: on the 100-file OMG training corpus the pilot reports
**nothing at all**, and so do we. That is the corpus written to be valid, and it is the row
that most directly answers "are we right?".

Counts that moved since the committed baseline, and why:

| Count | Was | Now | Reason |
|---|---|---|---|
| `kerml-examples`: everything | — | 58 files, 439 only ours, 6 only pilot | New root: the reference can validate KerML (F10). Adjudicated below. |
| `pilot-examples`: only ours | 334 | 316 | Merged since the baseline: the four parser productions and the keyword-as-name work. Not re-attributed line by line here. |
| `pilot-validation`: only ours | 118 | 59 | Same merged work. |
| `testdata`: files / only ours / only pilot / severity-only | 9 / 5 / 48 / 1 | 10 / 3 / 30 / 4 | F2: our fixtures now carry an explicit import visibility, so the pilot parses them instead of abandoning the body — which retires its cascade and the two `unresolved reference: Nowhere` rows below, and adds the `import_no_visibility.sysml` severity-only rows that F2's warn-not-error decision implies. The extra file is that fixture. |
| `examples`: only ours / only pilot / severity-only | 0 / 140 / 0 | 28 / 109 / 12 | F3's warn-not-error decision on non-standard notation: the words the pilot's grammars have no production for are now warned on rather than accepted silently, so lines the pilot already errored on become severity-only instead of pilot-only. |
| `probes`: files / only ours | 1 / 0 | 4 / 6 | The three specialization-cycle probes F4 was settled with are part of this root. |

None of those SysML-side movements come from this change: the previous revision of
`cmd/pilot-diff` run on `origin/main` reproduces them exactly.

The KerML row is the harshest in the table, and it is almost entirely ours: 439 only-ours
against 6 only-pilot, with 10 of 58 files fully agreeing. Where the reference validates the
corpus its authors wrote for it, we reject notation it accepts — adjudicated below. This is
reported, not fixed: this page is a comparison.

One category label moved with this adjudication and **no count did**:
`Must invoke a behavior or a behavioral feature` is now `kind-mismatch` rather than `unmapped`
(`cmd/pilot-diff/category.go`, `must invoke`). It is a constraint on the metaclass of what is
invoked — "a declaration used where its metaclass is not allowed" — which is exactly what that
category means, and it is the only one of the four P6 messages a category honestly fits. Its
single occurrence (`testdata/parse/expressions.sysml:4`) has no diagnostic of ours at that line
and category, so it stays a pilot-only disagreement and every total above is unchanged — the
regenerated baseline JSON moves that one diagnostic between category buckets and drops its
`unmapped` row, and nothing else. The other three stay `unmapped`: featuring-accessibility,
flow-end identification and model-level evaluability are none of the five categories, and
mapping them would risk accidental agreement rather than record the debt.

The `testdata`/`examples` rows are not a like-for-like verdict on our checker. `testdata/` and
`examples/` are largely *our* fixtures — several are deliberately malformed negative fixtures,
and many are written in notation the pilot's grammar rejects outright, after which its error
recovery cascades. Their 139 pilot-only diagnostics are therefore dominated by a handful of
root causes, adjudicated next.

---

## Adjudications

### Only ours — candidate false positives (3, SysML side)

The three diagnostics below are the adjudicated only-ours set outside the KerML root, which
has its own tables further down. The six cycle diagnostics on the `probes` root are adjudicated
with F4 below. The remaining 403 SysML-side only-ours diagnostics are not adjudicated in this
pass. The 121 `pilot-validation` syntax-only
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
| `passes/constraints.sysml:2,3` | `A`/`B` `participates in a specialization cycle` (`unmapped`) | **Ours is right, and the pilot has no such check** — settled by F4, both by reading its validators and by probing it on clean files (see [Specialization cycles](#specialization-cycles-f4)). The silence is not a parse cascade of the kind P1 describes: the same three cycle shapes in files with nothing else in them are accepted by the pilot with zero diagnostics. A one-sided finding, so it is our extension of the reference rather than a disagreement — kept `unmapped` because no coarse category honestly covers it. |
| `passes/constraints.sysml:9` | `multiplicity lower bound exceeds upper bound on lo` | **Ours is right**: `part lo [5..2];`. No pilot counterpart. |

### KerML — only ours (439)

Every one of the 439 falls in one of the classes below; the counts sum to 439. Verdicts:
**436 ours** and **3 one-sided** (K5, a check the reference does not have), with none
attributable to the bridge — it validates one
batch in one resource set, so it has no ordering or name-accumulation artifact to produce.
Nothing here is fixed in this PR; each class carries its follow-up.

| # | Class | Count | Verdict |
|---|---|---:|---|
| ~~K1~~ | **Fixed by F30.** `featured by` is not parsed: `expected a body member: 'featured' relates the declaration written before it, so a member cannot begin with it` (43), then `expected '{' or ';' after declaration` (95) and `expected a namespace member` (169) as the enclosing bodies unwind | 307 | **Ours (over-restriction).** KerML's featuring relationship (`member feature inCart: ShoppingCart[0..1] featured by Product_Account;`) is notation the reference accepts silently. One unparsed keyword produces 70% of the root's diagnostics: `Association Examples/ProductSelection_N_ary.kerml:38,40,42` cascade to `:51,53,54`. The featuring relationship is now parsed as `ast.RelFeaturedBy` (`KerML.xtext:569` `TypeFeaturingPart`, `:659` `OwnedTypeFeaturing`) and warned as `kerml-notation` in `.sysml`. Re-measured alone, the root falls 439 → **268** and its syntax diagnostics 360 → 172. |
| ~~K2~~ | **Fixed by F30.** Other KerML notation we reject: `expected a body member` on n-ary connector end lists (36), `expected 'then' between connector ends` on a typed/redefining succession (8), `"at"`/`"while"`/`"merge"` `is a reserved keyword` inside `expr` bodies (8), `expected a name` (6), `expected '{' or ';'` (3) | 61 | **Ours (over-restriction).** `connector ps1 : ProductSelection (myCart, products, myAccount);` (`Association Examples/ProductSelection_N_ary.kerml:122,124`), `succession redefines p_before_d : MyPaint_Before_Dry_Link [1] first paint then dry;` (`KerML Spec Annex A Examples/A-3-6-Sequences.kerml:58,60`), and `expr at { ... }` / `expr while { ... }` (`Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:16,25`) are all accepted by the reference. The keyword rows are the KerML half of F8. All three named constructs are now parsed (`KerML.xtext:842` `NaryConnectorDeclaration`, `:891` `SuccessionDeclaration`, and `at`/`while`/`merge`/`decide` unreserved in `.kerml` since they are literals of `SysML.xtext` only); after K1+K2 the root's syntax diagnostics are **140** (360 before F30) and the root is **291**, the rise over K1's 268 being newly reachable unresolved references — see K3/F31. Left open: `abstract var feature x [0..*];` and `member abstract feature x …` (2 diagnostics, `Variable Feature Examples/TimeVaryingCarDriver.kerml:53,100`), follow-up F50. |
| K3 | `unresolved reference` / `unresolved member` | 43 | **Ours (name resolution).** Three shapes: inherited library features reached through implicit specialization (`portion focusedState: Camera subsets timeSlices;`, `Behavior Examples/Camera.kerml:4,5`); a package declared in a sibling corpus file (`private import OneToOneConnectorsExecution::MyWheel;` at `KerML Spec Annex A Examples/A-3-5-TimingForStructures.kerml:24`, declared at `A-3-3-OneToOneConnectors.kerml:21`); and members named through a feature chain (`succession step1 then camera.focusedState;`, `Behavior Examples/TakePicture.kerml:16,17`). Some are plausibly downstream of K1/K2 in the same or an imported file, which is why they are one class and not one verdict per line. Follow-up F31. |
| K4 | SysML-shaped semantic checks firing on KerML: `only a definition may specialize; found a usage` (21), `type must be a definition, found attributeUsage` (2), `metaclass cannot specialize metaclass (kind mismatch)` (1), `rollsOn (typed by MyWheel) redefines rollsOn (typed by Wheel): types do not conform` (1) | 25 | **Ours.** KerML has no definition/usage split, so the first row misfires on ordinary declarations (`class Person specializes Object`, `Individuals Examples/JohnIndividualExample.kerml:4,12,34`; `Mass Roll-up Example/Vehicles_3.kerml:32`; `Simple Tests/Inheritance.kerml:21`). `metaclass <atom> AtomMetadata specializes Metaobject` (`KerML Spec Annex A Examples/A-2-Atoms.kerml:11`) is a metaclass specializing a metaclass, which the reference allows. The conformance row misses `classifier MyWheel unions MyWheel1, MyWheel2;` as a supertype of `Wheel` (`KerML Spec Annex A Examples/A-3-2-WithoutConnectors.kerml:32`). Follow-up F32. |
| K5 | `x`/`y`/`z` `participates in a specialization cycle` (`unmapped`) | 3 | **Ours is right, and the reference has no such check** — the same one-sided finding F4 settled on the SysML side, now with a KerML witness the corpus's own authors committed: `feature x :> z; feature y :> x; feature z :> y;` in `Simple Tests/Circular.kerml:9-11` is a cycle, and `KerMLValidator.checkSpecialization` is exactly the validator F4 read. Our extension of the reference rather than a disagreement, so it stays `unmapped`. |

### KerML — only the pilot (6)

| # | Class | Count | Verdict |
|---|---|---:|---|
| K6 | `The opposite features 'owningType' of '…DisjoiningImpl{…}' and 'ownedDisjoining' of '…{…}' do not refer to each other` | 6 | **Pilot artifact**, `unmapped`. Raised on `disjoint from` declarations in `KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9`, `Simple Tests/Classifiers.kerml:13`, `FeatureChains.kerml:31`, `Features.kerml:20`, `Inverses.kerml:3`, `Types.kerml:31`. It is an EMF `eOpposite` consistency complaint about the reference's own in-memory graph — it names `…Impl` objects and resource fragments, not model elements — so it is a statement about the pilot's transformation, not about the models, which are the pilot's own examples. Follow-up F33. |

### Severity-only (16)

One is adjudicated; the fifteen added by work merged since the baseline are not re-adjudicated
in this pass, which is a comparison of the KerML root.

| File | Verdict |
|---|---|
| `passes/import_no_visibility.sysml:8,12`, `parse/namespaces.sysml:5` (3) | A direct consequence of the F2 decision below: we report a bare `import` as a `warning` where the pilot's grammar makes it an `error`. Deliberate, and recorded here rather than re-argued. |
| `examples/` non-standard notation (12) | The same shape under F3: notation with no production in the pilot's grammars is now a warning of ours on a line the pilot errors on, so the pair is severity-only instead of pilot-only. |
| `passes/constraints.sysml:6` | Both flag `part few subsets cap [0..10];` under `cap [0..3]` at the same line and category. We report `error`; the pilot reports `warning` (`Subsetting/redefining feature should not have larger multiplicity upper bound`). A real difference in strictness, kept in its own bucket rather than being counted as two disagreements. |

### Only the pilot — candidate gaps (139, SysML side)

The 539 pilot-only diagnostics that were previously concentrated in `pilot-examples` were an
ordering artifact and are resolved. The remaining 139 SysML-side pilot-only diagnostics are the
same `testdata`/`examples` issues as before, at the lower counts the merged import-visibility,
keyword and non-standard-notation work left behind.

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
| P6 | `Must be an accessible feature (use dot notation for nesting)`, `Cannot identify flow end (use dot notation)`, `Must be model-level evaluable`, `Must invoke a behavior or a behavioral feature` | 9 | **Adjudicated per diagnostic below** (F5, done). 5 are downstream of P2, 2 are a real gap in our constraint tier, 2 are downstream of unresolved references both implementations report. The four *rules* behind them are all real, and three of them we do not implement: follow-ups F20–F23. |
| P7 | K6, the KerML `eOpposite` complaint | 6 | **Pilot artifact**, `unmapped`, and the only pilot-only class on the KerML root. Adjudicated with K6 above. Follow-up F33. |

#### P6, diagnostic by diagnostic (F5)

Each verdict below is backed by a matched pair of minimal reproducers run against the pinned
validator (`build/pilot-validator/validate-sysml <file>`, pilot `2026-05` / `0.60.1`): one file
that isolates the construct and one that differs in the single respect under test. The
reproducer outputs are quoted; "ours" is `bin/sysml <file>` on this branch.

| # | File:line | Message | Verdict |
|---|---|---|---|
| 1–3 | `semantic-layer/demo.sysml:44,45,46` | `Must be an accessible feature (use dot notation for nesting)` | **Downstream of P2.** The three lines are `attribute usePi = MathConstants::pi;`, `useE = MathConstants::e`, `nestedLookup = MathConstants::Derived::twoPi` — every reference into the `namespace MathConstants` the pilot could not parse (`demo.sysml:35 no viable alternative at input 'MathConstants'`). Reproducer pair: with `package MathConstants { attribute pi = 3.14159; } attribute usePi = MathConstants::pi;` the pilot is **silent** (exit 0); changing that one keyword to `namespace` gives `2:12: error: no viable alternative at input 'MathConstants'` *and* `5:20: error: Must be an accessible feature (use dot notation for nesting)` — the same two-diagnostic shape, at the same relative positions, as the file. Recovery turns the unparsed namespace into a feature, so the qualified reference becomes a subsetting whose subsetted feature is featured within another feature and fails `canAccess`. Fix P2 (F3) and these disappear. |
| 4–5 | `semantic-layer/demo.sysml:50,51` | `Must be an accessible feature (use dot notation for nesting)` | **Downstream of P2**, same mechanism: `expr2 = MathConstants::pi > 3` and `expr3 = -(MathConstants::e) < 0` are the only other lines in the file that reference `MathConstants`, and the neighbouring `expr1`, `expr4`, `expr5` — identical in shape but with no such reference — draw nothing. |
| 6–7 | `views-demo.sysml:44` | `Cannot identify flow end (use dot notation)` ×2 | **Real gap, and our fixture is the invalid model.** Line 44 is `flow of Fuel from tank to thruster;` inside `part def Descender`; the pilot parses the file cleanly to that point (its only earlier diagnostic there is line 32) so nothing is cascading. Reproducer pair: the same declaration with undotted ends draws `8:3: error: Must have at least two related elements`, `8:21: error: Cannot identify flow end (use dot notation)`, `8:29: error: Cannot identify flow end (use dot notation)`; writing the ends as `from tank.fuelOut to thruster.fuelIn` (against `out item fuelOut : Fuel` / `in item fuelIn : Fuel`) is accepted, exit 0. Ours reports nothing in either case. The companion `Must have at least two related elements` at the same line (booked under P5) has the same root cause, so P5's count for that file follows this verdict rather than P1/P2/P3. Follow-up **F21**. |
| 8 | `parse/expressions.sysml:4` | `Must be model-level evaluable` | **Downstream of the unresolved references both implementations report** at that line: `filter coll->select(x);` in a fixture that declares none of `coll`, `select`, `x` (agreement rows: ours `unresolved reference: coll/select/x`, pilot `Couldn't resolve reference to Element 'coll'/'select'/'x'`). `InvocationExpression::modelLevelEvaluable` is `function !== null && function.isModelLevelEvaluable && argument->forAll(modelLevelEvaluable)`, so an unresolved operator makes it `false` unconditionally. Reproducer chain: unresolved `filter coll->select(x);` → both P6 messages; a *resolvable* but non-evaluable invocation (`filter Twice(2) > 3;` over a local `calc def Twice`) → `Must be model-level evaluable` **only**; a resolvable evaluable one (`filter 1 + 2 > 0;`) → **silent**. So the message here is a consequence of the unresolved name, not of a construct we accept and it rejects. The rule itself is real and we have a *divergent* counterpart — see **F22**. |
| 9 | `parse/expressions.sysml:4` | `Must invoke a behavior or a behavioral feature` | **Downstream of the same unresolved references.** The constraint is over `instantiatedType`, which is `null` when the invoked `select` does not resolve; the resolvable-but-unevaluable reproducer above draws no invocation error at all, isolating the cause. The rule is real and unimplemented on our side: with `part def Widget; part w = Widget();` the pilot reports `3:11: error: Must invoke a behavior or a behavioral feature` and nothing else, while we report nothing. Follow-up **F23**. |

The rule behind rows 1–5 is real too, even though no corpus diagnostic is: with
`part def P { attribute n : Integer = 1; } package Q { filter E::P::n > 0; }` — which parses
cleanly for the pilot — it reports `Must be an accessible feature (use dot notation for nesting)`
(and `Must be model-level evaluable`), and we report nothing. Follow-up **F20**.

What each rule requires, at the pin
(`org.omg.kerml.xtext/src/org/omg/kerml/xtext/validation/KerMLValidator.xtend`,
`org.omg.sysml.logic/src/main/java/org/omg/sysml/…`):

| Rule | What it requires | Where it would live | False-positive risk if we get it slightly wrong |
|---|---|---|---|
| `validateSubsettingFeaturingTypes` — `Must be an accessible feature (use dot notation for nesting)` | Normative text on `Subsetting`: `subsettingFeature.canAccess(subsettedFeature)`. The pilot's `FeatureUtil.canAccess` holds when the subsetting feature has no `featuringType` and the subsetted feature is featured within nothing, or when some featuring type of the subsetting feature features the subsetted one — recursing through featuring types that are themselves features. A feature *of a type* is therefore not reachable by `::` from outside it; dot notation is what introduces the featuring chain that makes it reachable. | `LevelConstraint`, over the subsetting/redefinition relationships `semantics` already records; it needs a `canAccess` predicate over featuring types, which our side tables do not compute today. | Inherited and redefined features (a redefinition is a subsetting), features reached through a feature chain, connector and flow ends whose subsetting is implicit, and library features with no parsed AST — each is a place where a too-eager `canAccess` would reject a valid model. Warning-first, and silence whenever the featuring chain is not fully known, is the safe shape. |
| `validateFlowEndSubsetting` — `Cannot identify flow end (use dot notation)` | `FeatureUtil.getSubsettedNotRedefinedFeaturesOf(flowEnd)` must be non-empty: each end of a flow has to name the *feature* the payload leaves from or arrives at, so it can redefine `Transfer::source::sourceOutput` / `Transfer::target::targetInput` (`FlowEnd` model doc). Naming the part alone leaves the end with nothing to subset. The pilot also warns `Flow ends should use dot notation` for the implicit-subsetting case. | `LevelConstraint`, beside `checkConnectorEndRedefinition` / `checkInterfaceEndConjugation` in `passes/constraint.go`, over `lower`/`semantics` connector ends. | A flow whose ends are already features (`from a.out to b.in`), ends typed through a library `Transfer` specialization, and succession flows; also `examples/views-demo.sysml:44` is our own model and would have to be fixed rather than exempted. |
| `validateElementFilterMembershipIsModelLevelEvaluable` — `Must be model-level evaluable` | `condition.isModelLevelEvaluable` (plus `condition.result.specializesFromLibrary('ScalarValues::Boolean')`). Evaluability is **not** "is a constant": an invocation is evaluable when its function is a model-level-evaluable library function *and* every argument is; a feature reference is evaluable when its referent is a self-reference, or owned by a `Metaclass`/`MetadataFeature`, or has **no featuring type** and its value expression (if any) is evaluable — and inevaluable when the referent is featured within a type (an instance-level feature) or the reference is circular. So `filter p.n > 1` over a top-level `part p : P` is *accepted* by the pilot (no featuring type), while `filter P::n > 0` is not, and `filter Twice(2) > 3` over a user `calc` is not (a user function is not model-level evaluable). | Already ours: `passes/filter.go` `ElementFilterPass` (`filter-not-evaluable`, warning, `LevelType`) over `semantics.Model.CheckElementFilter`. The gap is **alignment**, not absence. | Both directions are live today: we warn on `filter 1 + 2 > 0;` (`the '+' operator is not supported in a filter condition`) where the pilot is silent, and we are silent on `filter P::n > 0;` where it errors. Widening our evaluator without the featuring-type half would trade one false positive for a false negative. |
| `validateInvocationExpressionInstantiatedType` — `Must invoke a behavior or a behavioral feature` | `instantiatedType.oclIsKindOf(Behavior) or (instantiatedType.oclIsKindOf(Feature) and instantiatedType.type->exists(oclIsKindOf(Behavior)) and instantiatedType.type->size(1))` — what is invoked must be a behavior (`calc def`, `action def`, `function`), or a feature typed by exactly one behavior. | `LevelConstraint`, or the invocation checking already in `passes/typecheck_expr.go` (`inferInvocation`/`effectiveInParameters`), which today infers argument types and arity but never asks what kind of thing is being invoked. | An invocation of a library function reached through an alias or an index record with no parsed declaration, a feature typed by a behavior *through* a specialization chain, constructor-like invocations of a definition (which the notation does allow in other positions), and metadata-annotation invocations. Reporting only when the invoked symbol resolves to a declaration we can classify is the safe shape. |

### Unmapped messages, verbatim

Recorded so the categorisation's debt is visible rather than hidden:

| Side | Message | Count |
|---|---|---|
| pilot | `Duplicate of other owned member name` | 25 |
| pilot | `Must be an accessible feature (use dot notation for nesting)` | 5 |
| pilot | `Cannot identify flow end (use dot notation)` | 2 |
| pilot | `Must be model-level evaluable` | 1 |
| pilot | `The opposite features 'owningType' … do not refer to each other` (K6, one row per file) | 6 |
| opensysml | `only a definition may specialize; found a usage` (K4) | 21 |
| opensysml | `<name> participates in a specialization cycle` | 11 |
| opensysml | `interface Mounting connects ports AxleMountIF and WheelHubIF, whose directed features are not conjugate; one end usually names the conjugate port (~AxleMountIF)` | 1 |
| opensysml | `name conflict: text is already the name of the inherited feature ModelingMetadata::Issue::text` | 1 |
| opensysml | `packet data field redefines packet data field, but packet data field is not an inherited member of Thermal Data Packet` | 1 |

The cycle rows stay `unmapped` **by adjudication, not by omission** (F4): the finding is
one-sided, and none of the five coarse categories describes a cycle in the specialization graph
— it is neither a name that failed to resolve, nor a metaclass used where it is not allowed, nor
bounds, units or syntax. Inventing a sixth category for a single check would empty the bucket
without adding a comparison, since the pilot has nothing to put in it.

### Specialization cycles (F4)

The question was whether the pilot is silent because it has no cycle check or because our
harness never got the question asked. Evidence, in the pinned release `2026-05`
(`jupyter-sysml-kernel` 0.60.1):

- **The validators have no such check.** `KerMLValidator.checkSpecialization(Specialization)`
  ([`KerMLValidator.xtend:531-537`](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/2026-05/org.omg.kerml.xtext/src/org/omg/kerml/xtext/validation/KerMLValidator.xtend#L531-L537),
  tag `2026-05`, commit `fa709f28`) implements exactly one constraint,
  `validateSpecializationSpecificNotConjugated`. Across `KerMLValidator` and `SysMLValidator` in
  the pinned jar, 219 diagnostic message constants mention no cycle, circularity or recursion.
  The pilot *does* check self-reference elsewhere — `Type cannot union with itself`,
  `... intersect ...`, `... difference ...`, `Feature cannot have itself in a feature chain` —
  so the absence for specialization is a gap in the checks, not in the idiom.
- **The normative model shipped with the pilot names nine `validate*Specialization` constraints**
  (`org.omg.sysml/src/org/omg/sysml/generation/SysML.uml`: binary association/connector,
  behavior, class, data type, cross-feature, structure, definition- and usage-variation), none
  about cycles. Circularity appears in that model only as something the *derivation* operations
  must tolerate (`the closure operation automatically handles circular relationships`), never as
  something to report; the pilot's scoping does the same, excluding a specialization edge "to
  avoid possible circular name resolution".
- **Probed directly**, which is the strongest of the three. Each probe is a package with nothing
  in it but the cycle, so no parse can fail earlier:

| Probe | Pilot `2026-05` | OpenSysML |
|---|---|---|
| [`specialization-cycle-self.sysml`](../../cmd/pilot-diff/testdata/specialization-cycle-self.sysml) (`part def A specializes A;`) | no diagnostics, exit 0 | 1 error, `constraint/specialization-cycle` |
| [`specialization-cycle-pair.sysml`](../../cmd/pilot-diff/testdata/specialization-cycle-pair.sysml) (`B1` ↔ `B2`) | no diagnostics, exit 0 | 2 errors |
| [`specialization-cycle-three.sysml`](../../cmd/pilot-diff/testdata/specialization-cycle-three.sysml) (`C1` → `C2` → `C3` → `C1`) | no diagnostics, exit 0 | 3 errors |

That the pilot reports *something* in such a file when there is something to report was checked
the same way: adding `part p : Nowhere;` to the pair probe makes it emit
`Couldn't resolve reference to Type 'Nowhere'.` and exit 1. Silence on the cycle is therefore a
result, not an unreached validation stage.

The probes are part of the `probes` root, so their six only-ours diagnostics are in the results
table and the refreshed baseline above.

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
| ~~F8~~ | **Done.** None of `choice`, `decision`, `deep`, `defer`, `done`, `final`, `history`, `initial`, `junction`, `region`, `shallow` is a literal in any pinned grammar, so all eleven are unreserved and matched contextually where our notation needs them ([conformance-audit.md](../reference/grammar/conformance-audit.md)). `done` now resolves as the library name it is in five files of the normative library; `TestStdlibReservedKeywordNames` no longer pins it. The state-machine notation of [grammar/README.md](../reference/grammar/README.md) that used those words still parses, now under the F3 warning. |
| F9 | Contextual keywords are neither highlighted nor completed: the VS Code grammars and the LSP keyword completion are generated from `lexer.Keywords()`, which `point`, `chain`, `on` and `var` are deliberately absent from. `var` is real KerML notation, so a second list of contextual words for those two surfaces would restore it without reserving it. |
| ~~F2~~ | **Done.** A bare `import` (no visibility) is non-conforming: the pilot's grammars make the indicator mandatory — `fragment ImportPrefix returns SysML::Import : visibility = VisibilityIndicator 'import' ...` with no `?`, unlike the sibling `MemberPrefix` (`KerML.xtext:169-172`, `SysML.xtext:241-244`) — and all 574 imports across the 254 OMG-authored corpus files carry one. Decision: **warn, not error** (`passes/import_visibility.go`, `LevelSyntax`, code `syntax/import-visibility`, spanned on the `import` keyword). The form is unambiguous to parse, so hard-failing would reject existing models over notation; the warning surfaces the non-conformance without gating the higher tiers. `expose` is exempt — the pilot grammar gives it implicit `protected` visibility (`SysML.xtext:2366-2372`). Our own fixtures now write an explicit visibility, with `testdata/passes/import_no_visibility.sysml` kept to lock the warning in. |
| ~~F3~~ | **Done.** Audited in [conformance-audit.md](../reference/grammar/conformance-audit.md): `namespace` is a literal in `KerML.xtext` only (`:125`) and a `.sysml` root admits package members only (`SysML.xtext:38`), so **both** body forms are the defect, not the semicolon one — the wording in P2 above was wrong. `region`, `choice`, `junction`, the history forms, `entry`/`exit point`, `defer`, and the `initial`/`final`/`decision` spellings have no production either. Decision: **warn, not error**, as F2 did — `passes/nonstandard_notation.go`, `LevelSyntax`, codes `nonstandard-notation` and `kerml-notation`, spanned on the word. The notation stays parsed, so existing models keep working; `namespace` stays silent in `.kerml`, where it is legal. `TestStdlibHasNoNonstandardNotation` locks in that no OMG-authored library file draws either warning. |
| ~~F4~~ | **Done.** The pilot has no specialization-cycle check at `2026-05`: `KerMLValidator.checkSpecialization` implements only `validateSpecializationSpecificNotConjugated`, no message constant in either validator concerns cycles, and the pilot accepts self-, two- and three-element cycles with zero diagnostics on otherwise-empty probe files. Our diagnostic stays, and stays `unmapped` — the finding is one-sided and no coarse category describes it. See [Specialization cycles (F4)](#specialization-cycles-f4). |
| ~~F5~~ | **Done.** The nine P6 diagnostics are adjudicated per diagnostic above: 5 downstream of P2, 2 a real gap (flow ends), 2 downstream of unresolved references both sides report. It spawned F20–F23, one per pilot rule, since all four rules are real whatever their diagnostics in *our* fixtures turned out to be. |
| F20 | `validateSubsettingFeaturingTypes` (`Must be an accessible feature (use dot notation for nesting)`): implement `canAccess` over featuring types in `LevelConstraint`. No corpus diagnostic is a real gap (all 5 are downstream of P2), so this is the *lowest* priority of F20–F23. F3 landed as a *warning*, so we still parse the `namespace` bodies the pilot loses, and the corpus therefore still does not show whether this rule would fire on a model both implementations read the same way. |
| F21 | `validateFlowEndSubsetting` (`Cannot identify flow end (use dot notation)`): a flow end must name the feature the payload leaves from or arrives at. The only P6 real gap in the corpus, and it also means `examples/views-demo.sysml:44` (`flow of Fuel from tank to thruster;`) is invalid and needs dotted ends. Highest priority of F20–F23: bounded scope, our own model already violates it. |
| F22 | `validateElementFilterMembershipIsModelLevelEvaluable` (`Must be model-level evaluable`): align `passes/filter.go` `filter-not-evaluable` with the spec's `isModelLevelEvaluable` — the rule is not absent but divergent in both directions (we warn on `filter 1 + 2 > 0;`, which the pilot accepts; we are silent on a reference to a feature with a featuring type, which it rejects). Second priority: it is the only one of the four that can produce a *false positive today*. |
| F23 | `validateInvocationExpressionInstantiatedType` (`Must invoke a behavior or a behavioral feature`): what is invoked must be a behavior, or a feature typed by exactly one behavior. Third priority. Its pilot-side category is now `kind-mismatch` rather than `unmapped` (see the note under the results table), so once implemented it can agree rather than merely coincide. |
| F6 | Build `Fabi303/sysmlv2tool` (needs a Tycho-capable Maven) and re-run with true single-batch loading, which would eliminate the P4 artifact and the ordering machinery. |
| F7 | Add [Sensmetry `syside check`](https://github.com/sensmetry/syside) as an *additional* cross-check. It is a different implementation, not the reference, so it can only corroborate — never adjudicate. |
| ~~F10~~ | **Done.** The pinned pilot release *does* ship KerML validation — `org/omg/kerml/xtext/validation/KerMLValidator.class` and `KerMLStandaloneSetup.class` — and what was missing was only a CLI. `scripts/pilot-kerml-validator/ValidateKerML.java` supplies one over the pilot's own `IResourceValidator`, sanity-checked on malformed, unresolvable and known-good input, and `kerml-examples` is a root. |
| ~~F30~~ | **Done.** All four constructs are parsed: `featured by` (`KerML.xtext:569,659`), n-ary connector end lists (`:842`), a typed/redefining succession before `first … then` (`:891`), and `at`/`while`/`merge`/`decide` as names in a `.kerml` file — none is a literal of `KerML.xtext` or `KerMLExpressions.xtext`, so the F3/F8 precedent applies and they are unreserved by file kind. The KerML root's only-ours count is 439 before F30, **268** after K1, **291** after everything, its syntax diagnostics falling 360 → **140**; the net rise over K1 is unresolved references in bodies that now parse (K3/F31). The committed baseline is untouched. |
| F50 | KerML feature prefixes we still reject: `abstract var feature x [0..*];` and `member abstract feature x …` (`Variable Feature Examples/TimeVaryingCarDriver.kerml:53,100`, 2 diagnostics). A modifier before the `var` prefix, and a modifier after `member`, are both refused where each alone is accepted — the remainder of K2 after F30. |
| F51 | A file's kind does not reach the REPL/`-validate` surface: `submitFiles` opens the accumulated buffer as one document named `<repl>` (`internal/repl/session.go:25,728`), so `source.KindOf` is `KindUnknown` and the file-kind gates in the parser never fire — `at`/`while`/`merge`/`decide` stay reserved there, while `-convert`, which passes the real path, accepts them. The pass layer already compensates for the `kerml-notation` *warning* alone (`dropKerMLNotationOfKerMLFiles`); since the buffer mixes `.sysml` and `.kerml` snippets, the fix is a per-snippet parse kind rather than a document one. |
| F31 | KerML name resolution (K3): inherited library features reached through implicit specialization, packages declared in a sibling corpus file, and members named through a feature chain. Re-measure after F30 — part of this class is plausibly cascade. |
| F32 | Checks that assume the SysML definition/usage split and fire on KerML (K4), plus `unions` as a supertype in redefinition conformance. |
| F33 | The pilot's `eOpposite` complaint on `disjoint from` (K6): confirm against the reference's `KerMLResourceValidator`/`ElementUtil.transformAll` whether the batch bridge can avoid it, or report it upstream. |
| F34 | Compare our own 11 `.kerml` fixtures (`testdata/lex/basic.kerml`, `examples/parser_features_demo_*.kerml`) too: a root carries one language today, so they are collected as SysML and excluded (see the known limitation above). Needs per-file language dispatch within a root, and a second pilot invocation per root. |

F6 and F7 are deprioritised: they change how the comparison is *run*, not what it says about
either implementation, so they rank below every rule follow-up above them.

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
