# Pilot Differential Diagnostics

## Overview

**Reference:** [SysML v2 Pilot Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation), release `2026-05` (`jupyter-sysml-kernel` 0.60.1) — the same release the training corpus is pinned to
**Wrapper:** [DeciSym/sysmlv2-validator](https://github.com/DeciSym/sysmlv2-validator) at commit `0d706e5ba1e9c56730cb8600ee43602906e12058`
**Provision:** `./scripts/download-pilot-validator.sh` (needs Java 21+ and Maven; writes `build/pilot-validator/`)
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

---

## Corpus roots

| Root | Directory | Provisioned by |
|---|---|---|
| `training` | `examples/sysml-v2-training` (`sysml/src/training`) | `scripts/download-training-examples.sh` |
| `pilot-examples` | `examples/pilot-corpora/sysml-examples` (`sysml/src/examples`) | `scripts/download-pilot-corpora.sh` |
| `pilot-validation` | `examples/pilot-corpora/sysml-validation` (`sysml/src/validation`) | `scripts/download-pilot-corpora.sh` |
| `testdata` | `testdata` | vendored |
| `examples` | `examples`, less the downloaded corpora | vendored |
| `probes` | `cmd/pilot-diff/testdata` | vendored |

The OMG corpora are not vendored, for the same licensing reason as the training corpus, and the
pilot release they are fetched at is pinned once in `scripts/pilot-pin.sh` — the same pin the
validator build reads, so corpus and reference can never come from different releases. Each
corpus directory is left alone when it already exists; remove it to re-download. A root whose
directory is absent is skipped with a warning.

### KerML: fetched, not compared

`kerml/src/examples` (58 `.kerml` files) is downloaded to
`examples/pilot-corpora/kerml-examples`, but it is **not** a root, because the reference side
cannot validate it:

- The DeciSym wrapper refuses any other extension outright —
  `Error: File must have .sysml extension: <file>.kerml` — and its directory mode only collects
  `.sysml`.
- The pinned pilot release has no KerML equivalent of `SysMLInteractive` to invoke instead. The
  `jupyter-sysml-kernel` jar ships `org.omg.kerml.xtext.*` grammar and scoping support, but the
  only KerML entry points are the `KerML2XMI` / `KerML2JSON` converters, which emit no
  diagnostics at all: run over a deliberately malformed `.kerml` file they still report
  `Transforming... / Resolving proxies... / Writing ...` and exit 0.
- Renaming a `.kerml` file to `.sysml` is not a substitute: the wrapper then parses KerML with
  the SysML grammar, so `class Entry { ... }` becomes `no viable alternative at input 'Entry'`.
  That measures the grammar mismatch, not agreement.

Our own side has no such gap — `model.Workspace` analyses `.kerml` — so the root becomes
comparable as soon as a reference-side KerML entry point exists (follow-up F8).

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

## Results (pilot `2026-05`, 276 files)

| Root | Files | Fully agreeing | Ours | Pilot | Agreed | Severity-only | Only ours | Only pilot |
|---|---|---|---|---|---|---|---|---|
| `examples/sysml-v2-training` | 100 | 98 | 2 | 0 | 0 | 0 | 2 | 0 |
| `examples/pilot-corpora/sysml-examples` | 98 | 38 | 337 | 539 | 0 | 0 | 337 | 0 |
| `examples/pilot-corpora/sysml-validation` | 56 | 33 | 121 | 0 | 0 | 0 | 121 | 0 |
| `testdata` | 9 | 0 | 24 | 67 | 18 | 1 | 5 | 48 |
| `examples` | 12 | 1 | 2 | 140 | 0 | 0 | 2 | 140 |
| `cmd/pilot-diff/testdata` (probes) | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 |
| **Total** | **276** | **171** | **486** | **207** | **18** | **1** | **467** | **188** |

The important change in this run is the second corpus row: the pilot-only diagnostics in
`pilot-examples` fell from 539 to 0 after the root order stopped relying on filenames and
started ordering by declared namespace names. `Annex_A_VehicleViews.sysml` now runs after
`SysML v2 Spec Annex A SimpleVehicleModel.sysml`, so the reference no longer sees a missing
`SimpleVehicleModel` namespace at line 2.

`pilot-validation` is unchanged by that harness fix: it still contributes 121 only-ours
syntax diagnostics, and those are parser gaps, not ordering artifacts.

---

## Adjudications

### Only ours — candidate false positives (9)

The 121 `pilot-validation` syntax-only discrepancies and the bulk of the
`pilot-examples` syntax-only discrepancies come from four missing productions: `connect a to b
{ ... }`, `flow a.x to b.y { ... }`, anonymous `interface a.p to b.q`, and `accept` on an
action usage declaration (`action got accept e : E { ... }`). The child session is fixing
those parser gaps; they remain open here.

| Files | Diagnostic | Verdict |
|---|---|---|
| `24. States/State Actions.sysml:26`, `25. Transitions/Transition Actions.sysml:34` | `"on" is a reserved keyword; write 'on' to use it as a name` | **Ours is wrong.** Both OMG files declare `state on { ... }` and write `then on;`. The pilot accepts both files completely. `on` is a keyword only in trigger position (`accept ... on ...`); our lexer/parser rejects it as a name. Follow-up F1. |
| `parser_features_demo_action_semantics.sysml:38,65` | `"var" is a reserved keyword; write 'var' to use it as a name` | **Ours is wrong**, same class: `attribute var : Integer;` is accepted by the pilot. Follow-up F1. |
| `passes/errors.sysml:4`, `resolve/errors.sysml:4` | `unresolved reference: Nowhere` | **Ours is right**, and these are negative fixtures where the diagnostic is the point. The pilot is silent only because a bare `import` earlier in the same file broke its parse before it got there (see P1) — a cascade artifact, not a disagreement about `Nowhere`. |
| `passes/constraints.sysml:2,3` | `A`/`B` `participates in a specialization cycle` (`unmapped`) | **Ours is right**: `part def A specializes B; part def B specializes A;`. The pilot reports nothing for the cycle. Recorded as `unmapped` because no pilot message pattern corresponds; whether the pilot has no such check or suppressed it is not established here. Follow-up F4. |
| `passes/constraints.sysml:9` | `multiplicity lower bound exceeds upper bound on lo` | **Ours is right**: `part lo [5..2];`. No pilot counterpart. |

### Severity-only (1)

| File | Verdict |
|---|---|
| `passes/constraints.sysml:6` | Both flag `part few subsets cap [0..10];` under `cap [0..3]` at the same line and category. We report `error`; the pilot reports `warning` (`Subsetting/redefining feature should not have larger multiplicity upper bound`). A real difference in strictness, kept in its own bucket rather than being counted as two disagreements. |

### Only the pilot — candidate gaps (188)

The 539 pilot-only diagnostics that used to sit entirely in `pilot-examples` are gone after
ordering by declared namespace names. The remaining 188 pilot-only diagnostics are the same
`testdata`/`examples` issues as before.

Grouped by root cause. The pilot's own grammar
(`org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext`) is quoted where it settles the
question.

| # | Class | Count (approx.) | Verdict |
|---|---|---|---|
| P1 | `mismatched input 'import' expecting '}'` / `missing EOF at 'import'` on a bare `import X::*;` | 10 of our 21 `testdata`/`examples` files | **Pilot is stricter, and its grammar is explicit**: `fragment ImportPrefix returns SysML::Import : visibility = VisibilityIndicator 'import' ...` — visibility is *mandatory* for an import, unlike `MemberPrefix`, where it is optional. `private import X::*;` parses cleanly. Whether the specification's concrete syntax makes visibility mandatory too is not settled here, so this is not booked as our bug: follow-up F2. **This is also the single largest cascade source** — once the import fails, the pilot abandons the enclosing body, which produces most of the `no viable alternative`, `extraneous input '}' expecting EOF`, `missing EOF`, `Couldn't resolve reference to Type 'Real'` and `A usage must be typed by definitions.` entries downstream. |
| P2 | `no viable alternative at input '<name>'` on `namespace N;` inside a package body | 4 files | **Ours is wrong (over-acceptance).** `namespace` is a KerML keyword; the pilot's `DefinitionElement` list has no namespace declaration, so `.sysml` notation has none. We parse it. Follow-up F3. |
| P3 | `no viable alternative at input 'region'` (`orthogonal-regions-demo.sysml`) | 1 file | **Ours is wrong (over-acceptance).** SysML v2 spells orthogonal regions as a `parallel` state body (`';' \| ( isParallel ?= 'parallel' )? '{' StateBodyPart '}'`); there is no `region` keyword. We accept one. Follow-up F3. |
| P4 | `Duplicate of other owned member name` (warning) | 30 | **Harness/wrapper artifact**, `unmapped`. The wrapper feeds every file of a root into one accumulating interactive session, so identically-named packages in different files collide. Not a statement about any model. |
| P5 | `Bound features should have conforming types`, `Must have a Boolean result`, `Must have at least two related elements`, `An attribute must be typed by attribute definitions.` | 23 | **Mostly downstream of P1/P2/P3**: with the imports or the enclosing body broken, the pilot type-checks a partially-recovered model. Not adjudicated individually; the honest reading is that these become meaningful only once P1–P3 are resolved and the files re-run. |
| P6 | `Must be an accessible feature (use dot notation for nesting)`, `Cannot identify flow end (use dot notation)`, `Must be model-level evaluable`, `Must invoke a behavior or a behavioral feature` | 9 | **`unmapped`, unadjudicated.** These are pilot checks with no counterpart of ours and no obvious category; they are candidate gaps worth reading file by file. Follow-up F5. |

### Unmapped messages, verbatim

Recorded so the categorisation's debt is visible rather than hidden:

| Side | Message | Count |
|---|---|---|
| pilot | `Duplicate of other owned member name` | 30 |
| pilot | `Must be an accessible feature (use dot notation for nesting)` | 5 |
| pilot | `Cannot identify flow end (use dot notation)` | 2 |
| pilot | `Must be model-level evaluable` | 1 |
| pilot | `Must invoke a behavior or a behavioral feature` | 1 |
| opensysml | `A participates in a specialization cycle` | 1 |
| opensysml | `B participates in a specialization cycle` | 1 |

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
| F1 | Keyword-as-name: `on` (OMG training corpus, 2 files) and `var` are rejected as names; the pilot accepts both. Our lexer reserves them unconditionally. This is the only false positive of ours on the OMG corpus. |
| F2 | Decide whether a bare `import` (no visibility) is legal SysML v2 textual notation. The pilot's grammar requires visibility; we accept its absence. Whichever way it lands, it affects 10 of our own fixtures. |
| F3 | Over-acceptance in our parser: `namespace N;` and `region` in a state body are not SysML v2 notation. |
| F4 | Specialization cycles: confirm whether the pilot checks them at all, and categorise the diagnostic (currently `unmapped`). |
| F5 | Read the P6 pilot checks (accessible-feature, flow-end, model-level-evaluable, behavior-invocation) file by file; each is a candidate gap in our constraint tier. |
| F6 | Build `Fabi303/sysmlv2tool` (needs a Tycho-capable Maven) and re-run with true single-batch loading, which would eliminate the P4 artifact and the ordering machinery. |
| F7 | Add [Sensmetry `syside check`](https://github.com/sensmetry/syside) as an *additional* cross-check. It is a different implementation, not the reference, so it can only corroborate — never adjudicate. |

---

## Re-running and diffing

```sh
./scripts/download-training-examples.sh   # the OMG training corpus (pinned 2026-05)
./scripts/download-pilot-corpora.sh       # the other OMG corpora, same pin
./scripts/download-pilot-validator.sh     # the pilot validator (pinned wrapper + release)
go run ./cmd/pilot-diff                   # writes build/pilot-diff/{pilot-diff.txt,pilot-diff.json}
diff <(jq -S . docs/project/pilot-differential-baseline.json) \
     <(jq -S . build/pilot-diff/pilot-diff.json)
```

The JSON carries tuples and counts but no message text, so it diffs cleanly; the text report
carries the messages for adjudication. When the baseline is refreshed, the verdicts above must
be re-adjudicated the same way [training-examples.md](training-examples.md) requires — a
moved count is a claim about one of the two implementations, and it needs a reason.
