# Pilot Differential Diagnostics

## Overview

**Reference:** [SysML v2 Pilot Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation), release `2026-07` (`jupyter-sysml-kernel` 0.61.0) — the same release the training corpus is pinned to
**Bridges:** two pinned plain-Java programs over the pilot's own validators — `scripts/pilot-sysml-validator/ValidateSysML.java` and `scripts/pilot-kerml-validator/ValidateKerML.java` — built against the shaded jar the [DeciSym/sysmlv2-validator](https://github.com/DeciSym/sysmlv2-validator) build (commit `0d706e5ba1e9c56730cb8600ee43602906e12058`) provisions
**Provision:** `./scripts/download-pilot-sysml-validator.sh` and `./scripts/download-pilot-kerml-validator.sh` (each needs Java 21+, and calls `download-pilot-validator.sh` for the pinned jar when it is absent; they write `build/pilot-sysml-validator/` and `build/pilot-kerml-validator/`)
**Run:** `go run ./cmd/pilot-diff` (writes `build/pilot-diff/pilot-diff.txt` and `build/pilot-diff/pilot-diff.json`, plus two CI-consumable renderings of the same run: `pilot-diff.xml`, JUnit XML with one suite per corpus root and one case per file that drew a diagnostic, and `pilot-diff.sarif`, SARIF 2.1.0 with one result per disagreeing diagnostic group located on the compared model file)
**Baseline:** the last committed run is [pilot-differential-baseline.json](pilot-differential-baseline.json), so a later run can be diffed against it
**Status:** advisory only — nothing here gates CI, and the harness reads the corpora without writing to them

**Labels:** this is an engineering record, and the short labels in it are internal cross-references, not
specification or product terms. A "wave" (and a "slice" within one) is a numbered development round of
this project, chronological and with no external meaning; `F<n>` is a row of the [follow-up table](#follow-ups-not-fixed-here--this-pr-is-advisory)
below, `K<n>` and `S<n>` are the KerML and SysML diagnostic classes the adjudications group findings
into, and `P<n>` is a probe of the reference. A reader who only wants the verdicts can ignore them.

[training-examples.md](training-examples.md) gates on
`internal/core/model/testdata/training_examples_expected.txt`, which is a snapshot of *our*
behavior: regenerating it records whatever the code now reports, so a regression re-baselines
as quietly as a fix. That gate answers "did we change?"; it cannot answer "are we right?".
This page is the other half: it asks the reference implementation the same question about the
same files and records where the two disagree.

It is a cross-check, not a gate, for a reason: the pilot is the reference implementation, but
not every difference is our bug (see the adjudications below — the pilot's own grammar is
stricter than ours in places, and some of its findings are rules we simply do not implement).

---

## Why this wrapper

Neither candidate is the pilot itself; both are thin CLIs over the pilot jars, which is
deliberate — a hand-written bridge into the pilot's Xtext internals would be our
interpretation of the reference rather than the reference.

| Candidate | Outcome |
|---|---|
| [DeciSym/sysmlv2-validator](https://github.com/DeciSym/sysmlv2-validator) | **Chosen for provisioning.** Builds from a pinned commit with Maven 3.6.3 and Java 21 here (`mvn -Psetup-dependency initialize && mvn package`), and its `setup-dependency` profile downloads the pilot release itself, so the pilot version is pinned in the same place as the wrapper. Emits GNU-format `file:line:col: severity: message`. |
| [Fabi303/sysmlv2tool](https://github.com/Fabi303/sysmlv2tool) | **Not used — could not be built here.** Its directory mode is the better fit (one batch, one resource set), but it builds the pilot from a submodule through Tycho, and the build fails under the Maven available in this environment: `No implementation for org.eclipse.tycho.core.resolver.MavenTargetLocationFactory was bound`. Re-tried with Maven 3.9.9 without success. Left as a follow-up rather than faked. |

### The limitation this used to force, and how the bridge removed it

The DeciSym CLI recurses into directories, but it validates each file with a separate
`interactive.process(content, true)` call against one accumulating `SysMLInteractive` session
— sequential, not a single batch parse. That made a file's verdict depend on when it was
validated, and it reported diagnostics by basename only, so the harness carried two
workarounds: an import topological sort (`cmd/pilot-diff/order.go`, `orderByImports`) and
splitting same-basename files into separate invocations (`batchByBaseName`).

F6 (#397) replaced the SysML oracle with
[`scripts/pilot-sysml-validator/ValidateSysML.java`](../../scripts/pilot-sysml-validator/ValidateSysML.java),
the SysML twin of the KerML bridge below: it reads every file of a corpus root into **one**
resource set and only then validates, and attributes each diagnostic to its path relative to
`--root`. Both workarounds are therefore deleted, and `cmd/pilot-diff` drives both languages
through one single-batch function. The DeciSym build stays in the picture only as the way the
pinned pilot release is provisioned — the pin is unchanged, its CLI is no longer the oracle.

Three pilot-only diagnostics disappeared with the ordering machinery (142 → 139), each an
order-dependence of the old wrapper rather than a changed verdict:

| Root | File | Diagnostic | Why it was reported before |
|---|---|---|---|
| `testdata` (30 → 29) | `parse/namespaces.sysml:4` | `Couldn't resolve reference to Membership 'E'.` | `private import E::**` resolves against `parse/expressions.sysml`, which declares `package E`; `orderByImports` did not recognise the `::**` recursive-import form, so the importer was validated before the provider was indexed. |
| `examples` (106 → 104) | `action-executor-demo.sysml:9` | `Couldn't resolve reference to Element 'x'.` and `Bound features should have conforming types` | `bind result = x * 2.0;` does not parse for the reference, and the recovered `x` now resolves globally against `phase-c-behavioral-bodies.sysml` / `repl-behavioral-demo.sysml`, either of which suppresses it. Alphabetically the demo used to be validated before both. |

Our side is run the way the corpus gates run it: every file in a root is opened into one
workspace *before* any diagnostic is requested, so cross-file imports resolve. Both sides are
now single-batch, which is what makes a per-file comparison meaningful.

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

It was the first of the two bridges, and since F6 the SysML side works the same way: one
resource set per root, diagnostics printed relative to the corpus root, no ordering to emulate
and no basename batching.

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
corpus directory records the repository and tag it was fetched from in a `.pilot-pin` stamp: one
stamped with the current pin is left alone (remove it to re-download), one stamped with another
pin is re-downloaded when the script runs again (the stale copy is kept until its replacement
has been fetched), and one without a stamp is left alone with a warning. A root whose directory
is absent is skipped with a warning.

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

**Only the Results table below states the current baseline.** Every other figure on this page —
the per-round tables, the movement history, the follow-up rows — is as measured at its own round
and is not the current baseline; the [README](../../README.md) and
[architecture](../internals/architecture.md) blocks restate the current one from the committed
baseline, regenerated and gated by `make docs-counts`.

Buckets per file: **agreement**, **only ours** (candidate false positives), **only the
pilot's** (candidate gaps), and **severity-only** — a (line, category) both implementations
flag with different severities. The last exists so such a pair is neither counted as agreement
nor double-counted as two independent disagreements.

---

## Results (pilot `2026-07`, 364 files)

| Root | Files | Fully agreeing | Ours | Pilot | Agreed | Severity-only | Only ours | Only pilot |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 100 | 0 | 0 | 0 | 0 | 0 | 0 |
| `examples/pilot-corpora/sysml-examples` | 99 | 95 | 7 | 0 | 0 | 0 | 7 | 0 |
| `examples/pilot-corpora/sysml-validation` | 56 | 56 | 0 | 0 | 0 | 0 | 0 | 0 |
| `examples/pilot-corpora/kerml-examples` | 58 | 51 | 3 | 6 | 0 | 0 | 3 | 6 |
| `testdata` | 17 | 10 | 38 | 55 | 34 | 1 | 3 | 20 |
| `examples` | 30 | 22 | 2 | 34 | 0 | 1 | 1 | 33 |
| `cmd/pilot-diff/testdata` (probes) | 4 | 1 | 6 | 0 | 0 | 0 | 6 | 0 |
| **Total** | **364** | **335** | **56** | **95** | **34** | **2** | **20** | **59** |

**Read the `only ours` total by root, never as one number.** Step 2 removes nine resolver false
positives from the reference's **own** corpora: `pilot-examples` 16 → **7** and
`pilot-validation` 1 → **0**, with `kerml-examples` unmoved at 3; the `2026-07` corpus then
retired one more of `pilot-examples` by publishing the conforming type its
[non-conforming redefinition](omg-issues.md) named, leaving 6, and the quantity-dimension error on
`Analysis Examples/Dynamics.sysml`:13 — a published product bound to a return typed by another
dimension — takes it to **7**. Our diagnostics on those roots therefore fall 20 → **10**. The
`examples` root carries 1, the non-standard-notation warning on the `junction` of
`pseudostates-demo.sysml`, the one demo that keeps the pseudostate notation because no SysML v2
spelling of it exists. It carried 64 before the demos were rewritten to standard notation: the
succession shorthands retired 30, removing `initial <state>;` and `transition <src> to <tgt>;`
retired 27 more, and the standard-notation round below retired the last 7. **Those that remain are
true positives about our own examples, not candidate false positives about our implementation** — the
column header is wrong for them, and the honest count of suspect diagnostics of ours against the
reference corpora is **10**. `severity-only` (2) holds pairs of the same shape:
where the pilot errors on a line we warn on, the pair sits in severity-only rather than either side
changing what it detects.

### Standard-notation demo round

Our own demos were the largest single source of divergence left on this page: six of them were
written in notation this project extends the grammar with, so we warned where the reference
hard-errored and its recovery then cascaded through the rest of the file. This round rewrites each
demo to the standard spelling wherever the [grammar audit](../reference/grammar/conformance-audit.md)
records one, keeps the notation that has none in the one demo that exists to show it, and changes
no parser, validator or runtime code — every demo's `%`-command output is unchanged.

| Count | Before the rewrite | Now |
|---|---:|---:|
| overall: fully agreeing | 330 | **332** |
| overall: our diagnostics | 69 | **58** |
| overall: pilot diagnostics | 110 | **83** |
| overall: only ours | 27 | **20** |
| overall: only pilot | 68 | **45** |
| overall: severity-only | 5 | **2** |
| `examples`: fully agreeing | 17 | **19** |
| `examples`: only pilot | 42 | **19** |

| File | What it now writes | Rows |
|---|---|---:|
| `action-executor-demo.sysml` | `then done;` in place of a standalone `done;` declaration | 3 → **0** |
| `phase-c-behavioral-bodies.sysml` | `entry`/`do`/`exit <action>` and named effect actions; declared Boolean features accepted with `accept when` | 3 → **0** |
| `views-demo.sysml` | the descent flow as `first`/`fork`/`join`/`decide` with successions; `Descender::mass` declared without a default the two landers rebind; the framing view declared last | 8 → **2** |
| `solver-demo.sysml` | `assert constraint` for an analysis case's own conditions; each objective redefines the subject it inherits | 15 → **5** |
| `disposal-robot-demo/robot.sysml` | the same objective subject redefinition; the framing view declared last | 17 → **7** |
| `pseudostates-demo.sysml` | a state named `ready` rather than one shadowing the library's `start` | 8 → **7** |

`require <constraint>` outside a requirement body and a standalone `done;` are the two the audit
answers outright: an analysis case's own conditions are ordinary `assert constraint` members, and
`done` is a member every action inherits, so referring to it is the standard spelling and declaring
it again is not. The objective subject is the reference's own: `TradeStudies::TradeStudy` writes
`subject :>> selectedAlternative;` in its objective, and writing it in ours retires the six
`Only one subject is allowed.` rows without touching what `%optimize` reports. The lander's
`Descender::mass` loses its `= 890.0` default for the same reason: both landers declare a mass of
their own, and a redefinition that rebinds an inherited value is `Cannot override a binding feature
value` on the reference. A `Descender` declared without a mass now has none, which is what a
declaration that states no value means.

What remains is adjudicated as extension notation this project supports deliberately:

- **`choice` and `junction`** — no SysML v2 production exists for pseudostates, so the notation stays
  supported and stays demonstrated. `pseudostates-demo.sysml` is now the only file that writes it, and
  says so; its 1 only-ours warning, 1 severity-only pair and 5 pilot rows are that file alone.
- **`attribute :>> best = <expression>` and a second objective** (`solver-demo.sysml`,
  `robot.sysml`, `disposal-team-demo/team.sysml`) — the trade-study contract this project reads
  (`internal/core/solve/doc.go`). The
  library binds `best`, so the reference reports `Cannot override a binding feature value` for the
  expression to improve, and it admits one objective per analysis case where we improve several
  lexicographically: 8 + 2 + 1 `unmapped` rows.
- **`frame concern` in a view usage** (`views-demo.sysml`, `robot.sysml`) — `FramedConcernMember` is
  a requirement-body member in the pilot grammar, not a view-body one, and the demos frame a concern
  in the view because that is what `%view` evaluates the exposed elements against. Declaring the
  framing view last confines the reference's recovery to the file's closing lines, 2 syntax rows
  each instead of the whole view package.

The `testdata` fixtures the same notation appears in are unchanged: `passes/import_no_visibility.sysml`
and `parse/namespaces.sysml` exist to exercise the diagnostics they carry, so their rows stay
adjudicated where they are rather than rewritten away.

### The team demo

`examples/disposal-team-demo/team.sysml` was written to exercise notation the robot demo does not
reach, and validates clean here. The reference reports four rows on it, none of them a rule of ours
that is missing:

| Row | The reference's reading | Verdict |
|---|---|---|
| `:29` (2 `warning: Bound features should have conforming types`) | `attribute payload : MassValue = sum(robots.mass) + sum(cradles.mass);` — the value is an operator expression, and the reference compares the argument types of the implicit binding | Same family as `BindingConnector_Invalid2.sysml.xt:42`: our `W9CBoundFeatureTypesPass` checks feature endpoints, and no numbered constraint was found for argument-level conformance on an expression |
| `:117` (`error: Referent must be time varying.` + the same warning) | `assign accepted := accepted + 1;` in a state's entry action, where `accepted` is an attribute of the enclosing `part def` | Reference-side asymmetry: the identical assignment written in an `action` of the same part def, nested or not, is clean on both sides, so the referent's `mayTimeVary` is not what the two implementations read differently — the state's entry action is |
| `:259` (`error`, `unmapped`) | `attribute :>> best = robotMass;` in the analysis objective | The trade-study notation adjudicated in the bullet above |

### Package-keyword round

`examples/semantic-layer/demo.sysml` declared three of its packages with KerML's `namespace`
keyword, which the SysML grammar has no production for: the reference could not parse those
declarations, and our own non-standard-notation pass warned on each of them. Writing them as
`package` — the spelling both implementations admit — makes the file fully agreeing and removes
every row it carried, on both sides:

| Count | Before the keyword change | Now |
|---|---:|---:|
| `examples`: only pilot | 49 | **19** |
| `examples`: severity-only | 7 | **1** |
| overall: fully agreeing | 329 | **332** |
| overall: our diagnostics | 72 | **58** |
| overall: pilot diagnostics | 120 | **83** |

The three severity-only pairs were our `kerml-notation` warning against the reference's parse
error on lines 35, 39 and 105; the seven pilot-only rows were that parse failure's recovery —
two `Duplicate of other owned member name` and the five `Must be an accessible feature` rows
the recovered namespaces produced for references into them. Nothing about either implementation
changed: the example did.

### Feature-initialization round

The reference's own diagnostics moved for the first time in several rounds, and none of our
columns did. Writing an output's value as an initializer instead of a separate binding —
`out result : Real = x * 2.0;` in place of `out result : Real;` followed by
`bind result = x * 2.0;` — is notation the reference parses, so its error recovery no longer
cascades through the rest of the file. The movement is entirely one file,
`examples/action-executor-demo.sysml`, whose pilot-only rows fall 24 → **3**:

| Count | Before the initializer rewrite | Now |
|---|---:|---:|
| only pilot | 82 | **59** |
| pilot diagnostics | 123 | **95** |
| severity-only | 9 | **2** |

The rewrite itself took only-pilot to 61 and pilot diagnostics to 101; the `Now` column states
those counts as the later rounds leave them.

The one diagnostic of ours that left is its severity-only partner: the line it warned on is the
`bind` that the rewrite removed, so neither tool has anything to report there. At this round the
combined figures were 324 fully agreeing / 27 only ours / 68 → 67 our diagnostics — agreement, fully
agreeing files, only-ours and every OMG root were unmoved by the rewrite, so nothing here is a
conformance change: it is our own demo written in a spelling the reference accepts. The rewrite itself
left only-pilot at 61 and pilot diagnostics at 101; the `Now` column tracks the current baseline, so it
also carries the interface-flow pairing round, the library-inherited-name round, the end-to-end
demo round and the package-keyword round that followed, and the Results table above states every figure as it is now.

### End-to-end demo round

`examples/disposal-robot-demo/robot.sysml` is one file added to the `examples` root, and the whole of that
round's movement is the reference's column: only pilot 32 → **49** and pilot diagnostics 42 → **59**
on the root, 58 → **75** and 103 → **120** overall, with our own column unmoved at 8 only-ours and
18 diagnostics, and the file clean under `-validate`. Its 17 pilot-only rows are all in the demo's
view and analysis packages, and each is a construct the pinned artifact does not have:

| What the demo writes | Rows | What the pilot reports |
|---|---:|---|
| `frame concern` in a view usage | 2 syntax | the member is not in its view grammar, and the cascade takes the file's closing brace |
| `view … : StateTransitionView` / `: ActionFlowView`, `render asElementTable` | 3 `unresolved-reference`, 4 `kind-mismatch` | our standard view definitions and rendering, which its libraries do not publish |
| `objective … { require constraint … }` with `attribute :>> best` | 6 `unmapped` | one subject per requirement, no rebinding of `best`, one objective per analysis case |
| a second objective for a lexicographic optimum | 2 `unmapped` | `Only one objective is allowed` |

Nothing in the demo's structure, calculations, action or state machine draws a pilot diagnostic, so
the rows above measure the reach of the reference's view and trade-study support, not a divergence in
the notation both implementations share. The behavioral half of the file was written to the spelling
the reference accepts for exactly that reason: transitions into a substate name it through the state
around it (`then approach.rolling`) rather than nesting the trigger in the substate.

### Quantity-dimension round

Judging a bound or written quantity by the dimension its target's declared quantity value type fixes
adds exactly one row to the census, and it is in the reference's own corpus:
`Analysis Examples/Dynamics.sysml`:13, adjudicated with the other published-model defects
[below](#the-remaining-only-ours-rows). The movement is that row and nothing else — `pilot-examples`
only-ours 6 → **7**, overall fully agreeing 330 → **329** and our diagnostics 71 → **72**, with
agreement, severity-only and every pilot column unmoved:

| Count | Before the dimension check | Now |
|---|---:|---:|
| `pilot-examples`: only ours | 6 | **7** |
| overall: fully agreeing | 330 | **329** |
| overall: our diagnostics | 71 | **72** |

### Phase C initial-state round

Giving each state machine in `examples/phase-c-behavioral-bodies.sysml` a transition out of its
entry action, and a value to the Boolean features its guards and triggers read, moves **no row**:
357 files, 332 fully agreeing, 36 agreed, 20 only ours, 45 only the pilot's, before and after. Both
sides read a transition out of an entry action and an initialized attribute the same way, so the
baseline is re-recorded for the `examples` digest alone.

### Step 2 resolver round

The control is a fresh-cache run of merge base `bbd3b2ec`; the head is an independent
fresh-cache run of this tree:

| Oracle | Control (`bbd3b2ec`) | Head — **historical snapshot**, measured at `bbd3b2ec`'s round, not the current figure |
|---|---:|---:|
| Xpect | 1293 agree / 248 wording-only / 30 disagree | 1293 / 248 / 30 |
| Differential | 321 fully agreeing / 92 only ours / 148 diagnostics of ours | 324 / 83 / 139 |
| Rejection | 116 both reject / 4 only pilot / 0 only ours / 0 both accept | 116 / 4 / 0 / 0 |

The differential movement is exactly the nine attributed resolver rows; agreed diagnostics,
severity-only, only-pilot, Xpect, and rejection do not move:

| Corpus row | Mechanism | Result |
|---|---|---|
| `Vehicle Example/Annex_A_VehicleViews.sysml:753` | recursive public import re-export for `@Safety` | closed |
| `Vehicle Example/Annex_A_VehicleViews.sysml:754` | recursive public import re-export for `@Security` | closed |
| `Vehicle Example/Annex_A_VehicleViews.sysml:757` | recursive public import re-export for `@Security` | closed |
| `Vehicle Example/Annex_A_VehicleViews.sysml:758` | recursive public import re-export for `@Safety` | closed |
| `Vehicle Example/Annex_A_VehicleViews.sysml:760` | recursive public import re-export for `@Safety` | closed |
| `Vehicle Example/Annex_A_VehicleViews.sysml:789` | recursive public import re-export for `@Safety` | closed |
| `State Space Representation Examples/EVSample1.sysml:351` | source-end implicit redefinition through cached `Transfers::Transfer` | closed |
| `State Space Representation Examples/EVSample1.sysml:354` | target-end implicit redefinition through cached `Transfers::Transfer` | closed |
| `09-Verification/9-Verification-simplified.sysml:55` | objective-role implicit redefinition | closed |
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml:38` | adjudicated divergence: `[mm]` applies to `110`, not the additive expression | retained |

### Step 3 obligation round

Independent fresh-cache reports from exact base `4b9baf2d` and this tree are byte-equivalent:

| Oracle | Base `4b9baf2d` | Step 3 — **historical snapshot**, measured at `4b9baf2d`'s round, not the current figure |
|---|---:|---:|
| Xpect | 1293 agree / 248 wording-only / 30 disagree | 1295 / 248 / 28 |
| Differential | 324 fully agreeing / 83 only ours / 66 only pilot | 324 / 83 / 66 |
| Rejection | 116 both reject / 4 only pilot / 0 only ours / 0 both accept | 116 / 4 / 0 / 0 |

The differential has **no row movement, recovery, regression, or newly unmasked diagnostic**.
Its only-ours counts remain `training` 0, `pilot-examples` 8, `pilot-validation` 0,
`kerml-examples` 3, `testdata` 3, `examples` 63, and `probes` 6. Pilot-side columns remain
populated and unchanged: 122 diagnostics total, 66 pilot-only. Step 3's two semantic recoveries are
Xpect assertions not present in these seven differential roots.

**Wave 12A moves the agreement column and nothing else.** Narrowing tier gating from the document to
the element (roadmap L2) turns 7 pilot-only rows into agreed ones: agreement 25 → **32**, only-pilot
73 → **66**, our diagnostics 168 → **175**, only-ours unmoved at **119** and no root other than
`testdata` touched. All 7 are a higher-tier rule that used to be silenced by an unrelated
name-resolution error in the same file: `parse/expressions.sysml` lines 2–6, `passes/errors.sysml`
line 3 and `resolve/errors.sysml` line 3. Lines 3–6 of `expressions.sysml` are wording-only
agreement in category, not in rule — we say `Must be model-level evaluable` where the pilot says
`Must have a Boolean result` — so they are counted as agreement by category and remain listed as an
open wording divergence below.

**Four of the 23 are a measurement correction, not a fix, and the distinction is the point.** The
142 this page previously reported was measured with a stale on-disk library index cache, whose records
outlived the implementation that wrote them; a fresh-cache run of the same pre-wave-11 tree measures
138. The cache defect is fixed by keying records by build identity and making a library index-only on
every load path, so a cache hit can no longer report different diagnostics than a miss. Wave 11's own
work accounts for the remaining 138 → **119**.

The `Vehicle` diamond false positive this page recorded is gone: `pilot-examples/Vehicle
Example/Annex_A_VehicleViews.sysml` 14 → 6, all 8 `Duplicate of inherited member name '<x>' from
Vehicle, vehicle_b` rows retired by 11F canonicalizing redefinition in the resolver's
`checkInheritedAmbiguity`, which is where they came from rather than the pass tier 11A fixed. What
remains on the OMG roots is 8 `unresolved-reference`, 5 `unmapped`, 1 `kind-mismatch` and 2
`units` of ours on `pilot-examples`, 1 `unresolved-reference` on `pilot-validation`, and on
`kerml-examples` only K5's three one-sided specialization cycles on `Circular.kerml`.

This table is a clean fresh-cache full run on the merged wave-11 tree at `3bc81ce3` — #492 (11G
annotation-body scope), #494 (11D metadata and evaluability), #493 (11F usage rules, the use-case
analogues and the diamond fix) and #484 (11E KerML structural residue) — with wave 10 before it — #450 (9A implicit
members), #454 (9D visibility and resolver residue), #455 (9C silent errors and warnings), #452 (9B
the only-pilot column classified, and the two example models it rewrote to the spec spelling), #451
(9F the negative corpus) and #449 (9E behaviour evidence) — on top of the wave-8 round and, before
it, #424 (wave 7A: metaclass reflection
answered from the element, and an implied base for transition members) merged on top of the wave-6
round — #409 (F90's conjugation check scoped to SysML typings), #415 (the reference's name-distinguishability rule, and
the duplicate-owned-member-name warning P4 named), #410/#413 (alias identity), #411 (a state
transitioning to itself), #412 (the expression referee) and #408 (the guard extended over
`.agents/skills/`) — on top of wave 5 (#403 the KerML declaration grammar, #405 cached-library
specialization edges, #397 F6 batch loading, #396 the unified corpus gates, #398 the doc-count
guard), wave 4 (#383 F65, #391 F68/F72/F52, #387 F99, #388 F100/F53, #382 F9), the F60–F69 round
(#358–#364) and the round after it (#372 symbols/typecheck, #373 fixtures, #374 KerML grammar, #375
behavior parser, #376 F21–F23). It is the run the committed baseline was regenerated from, and two
consecutive runs produce byte-identical JSON and text, so the numbers are not order- or
timing-dependent.

**Wave 7A closes both of the cross-package handovers wave 6 left open**, and it is the larger of the
two moves recorded here: only-ours 161 → **147**, fully agreeing 297 → **300**.

- **F93 is closed at every tier.** Metaclass reflection now answers from the element, so the
  element-filter false positives are gone: `kerml-examples/Simple Tests/Filtering.kerml` 3 → 0 (the
  file is fully agreeing now) and the two `testdata/passes/f93_element_filter.{kerml,sysml}`
  reproducers 4 → 0. Those four fixtures were committed by #409 to *record* an open gap; they now
  record a closed one, and `testdata` only-ours falls 7 → 3.
- **F68's implicit transition members close 9 rows on `pilot-examples`.** `Interaction Sequencing
  Examples/ServerSequenceRealization-2.sysml` 7 → 1 and `ServerSequenceOutsideRealization-2.sysml`
  4 → 1: the members implied by a transition are now present, so references to them resolve.
- **One new finding, and it is ours, not the reference's.** `Simple Tests/PartTest.sysml` goes 1 → 3:
  the single `unresolved-reference` at line 25 disappeared and **unmasked three
  `participates in a specialization cycle` errors** at lines 49–51, where the file really does
  declare `part p1 :> p2; part p2 :> p3; part p3 :> p1;`. The reference is silent on all three,
  which is K5's known one-sided cycle check. What is new is an internal inconsistency this exposes:
  line 53 of the same file declares `part p4 :> p4;` and we say **nothing** about it, so our own
  cycle check catches a three-element cycle and misses a self-loop. That is a defect in our check
  rather than a divergence from the reference, and it is unowned as of this rebaseline.

The wave-6 numbers below are kept as recorded, because the round-by-round movement table is only
readable if each round's column stays what that round measured.

**Why this page was stale, and the guard that did not catch it.** It stated 309 / 119 / 157 through
all of wave 10, because its counts are compared against the committed baseline rather than a fresh
run — the same failure mode the Xpect oracle had, where the baseline drifted 94 rows behind `main`
while CI stayed green. Wave 10 moved this oracle without any wave-10 slice owning it: 10C's D1/D2
conformance modes made the notation warnings above fire by default, and 10B's and 10F's rule work
retired false positives on the pilot corpora. Neither shows up until the baseline is regenerated,
which is what this round does.

**Wave 6 is a small move on this page and a large one elsewhere, and the two should not be
conflated.** Only-ours falls 167 → 161 and fully agreeing rises 291 → 297, which is a tenth of what
wave 5 moved. The round's principal result is not here at all: it is the pilot's own Xpect
expectations, where `linkedName` went 151/194 to **194/194** — see
[pilot-xpect.md](pilot-xpect.md). Alias identity is a *resolution* fix, and no corpus file in these
roots resolves a name through an alias, so a fix that closed 43 declared expectations of the
reference's authors moves this table by zero. That is a property of the corpora, not of the fix, and
it is the clearest example so far of why this page cannot be read as a compliance measure on its own.

Two things did move here, and one is a first:

- **Agreement rose, 20 → 22** — the first time in five rounds that a disagreement *converged*
  instead of one of our diagnostics disappearing. #415 implemented the reference's
  `Duplicate of other owned member name` warning, so `testdata/passes/corpus_notation.sysml:33,34`
  turns from two pilot-only rows into two agreed ones. That is the P4 row #402 rewrote from
  "wrapper artifact" to "a real reference rule we do not implement", now implemented.
- **`testdata` gained 4 files and 4 only-ours diagnostics, deliberately.** #409 committed
  `testdata/passes/f93_element_filter.{kerml,sysml}` as the reproducer for the F93 element-filter
  false positives it root-caused but did not own, so the fixtures **record an open gap**: all four
  diagnostics are ours, the reference is silent on all four, and the ratchet is supposed to carry
  them until F93 is fixed in `semantics/`. A ratchet reading of "only-ours went up on `testdata`"
  is correct and expected here.

Per category, the only-ours totals are: `pilot-examples` 4 `unmapped`, 2
`units`, 1 `kind-mismatch`; `kerml-examples` 3 `unmapped`; `examples` 1 syntax; `testdata` 2
`unmapped`, 1 `multiplicity`; `probes` 6 `unmapped`.
Only-pilot: `testdata` 12 `kind-mismatch`, 3 `unmapped`, 3 syntax, 2 `unresolved-reference`;
`examples` 10 syntax, 17 `unmapped`, 5 `kind-mismatch`, 1 `unresolved-reference` — of which
`relay-probe-demo/mission.sysml` carries three: two `unmapped` where the pilot rejects a snapshot
redefining the mass its individual binds (`Cannot override a binding feature value`) and one
`kind-mismatch` on its send of a `Telemetry` instantiation, the same two rules it already flags on
`solver-demo.sysml` and the send-statement demos — all of them
`.sysml`, none `.kerml`, which is the F96 fixture round below;
`kerml-examples` 6 `unmapped` (K6).

The architecture self-model under `examples/self-model` adds six of the shapes this root already
carries: four `unmapped` where `pipeline.sysml` redefines an inherited attribute's default
(`Cannot override a binding feature value`, the rule `solver-demo.sysml` and
`relay-probe-demo/mission.sysml` already draw) and two syntax rows where `views.sysml` frames a
concern, which the reference rejects on `views-demo.sysml` the same way.

**`pilot-examples` is the row to read carefully: its total falls 68 → 63 and its mix barely
resembles the old one.** All 31 syntax rows are gone, and `pilot-validation`'s 7 with them — wave 10D
parses notation we used to reject. But `unresolved-reference` rises 27 → 36, `unmapped` 5 → 17 and
`kind-mismatch` 4 → 9 in the same root, because a file we now parse runs the later tiers for the
first time. **Parsing more exposes more, and the net −5 in this root hides 31 syntax rows retired
against 26 newly surfaced** (`pilot-validation`'s 7 are its own row); a reader who takes the total as
"five defects fixed" has the wave backwards.

Batch loading is why `pilot-examples` has no pilot-only diagnostic at all: the reference reads
`SysML v2 Spec Annex A SimpleVehicleModel.sysml` and its importer, `Annex_A_VehicleViews.sysml`,
into one resource set, so the missing `SimpleVehicleModel` namespace at line 2 — 539 diagnostics'
worth of cascade when the root was validated file by file — is never reported. The importer still
has OpenSysML-only syntax diagnostics, which remain part of the parser-gap follow-up below.

`pilot-validation` never depended on that: it contributed 121 only-ours syntax diagnostics in the
pre-merge comparison, and 7 now (10 only-ours in total: 2 `kind-mismatch` and 1
`unresolved-reference` a since-retired syntax error had been masking).

The headline is the first row: on the 100-file OMG training corpus the pilot reports
**nothing at all**, and so do we. That is the corpus written to be valid, and it is the row
that most directly answers "are we right?".

What has moved since the adjudication, and why. Every fix PR measured its own movement from
whichever baseline it branched from, so their individually reported deltas do not sum; the two
columns below are the only combined measurements. "At #356" is the state the adjudications were
written against, "F60–F69" the round of #358–#364, "wave 3" the round of #372–#376, "wave 4" the
round of #383/#391/#387/#388/#382, "wave 5" the round of #403/#405/#397/#396/#398, "wave 6" the
round of #408–#415, "wave 7A" #424, "wave 8" everything landed on `main` between 7A and 8G (#438),
"wave 9" the round of #449–#455, "wave 12A" #500, and the last column the member-leading-succession diagnostic round (#526) — every column of that table, including its last, is the figure measured at its own round and none of them is the current measurement, which is the Results table above. The wave-3 and wave-5 reason columns are dropped for width; those reasons survive in the K- and S-class tables below and in this page's history:

For round 3, the fresh control column is the `1af78d94` base, before the wave-12F changes:

| Count | Base after wave 12D (`1af78d94`) | Now |
|---|---:|---:|
| overall: fully agreeing / only ours / our diagnostics | **317 / 119 / 175** | **335 / 20 / 56** |
| `pilot-examples`: only ours | **43** | **7** |
| `pilot-validation`: only ours | **1** | **0** |
| `kerml-examples`: only ours | **3** | **3** |
| `examples`: only pilot | **40** | **33** |
| `examples`: fully agreeing | **15** | **22** |
| `unmapped`, our side | **20** | **19** |

The `Now` column's movement since Step 2's resolver round is the removal of alias notation from
our own demos, in two rounds, and it lands entirely on the `examples` root. The succession
shorthands went first (only ours 84 → **34**, our diagnostics 141 → **50**); removing
`initial <state>;` and `transition <source> to <target>;` took it to **27** and 68. The
demos are now written in the standard spellings, so the warning that fired on 57 of their lines
has nothing left to report there. `examples`: only pilot rose 39 → 56 and severity-only fell
25 → 9 for the same reason and without either tool changing what it detects: the pilot errors on
those lines either way, and the rows it errors on are no longer paired with a warning of ours.
Since then two further changes land in this column: `action-executor-demo.sysml` stopped binding a
computed result with `bind`, which retires 21 of the pilot's syntax-and-cascade rows on the
`examples` root (only pilot 56 → **35**, severity-only 9 → **8**), and the interface-flow pairing
below retires one only-ours row on `pilot-examples` (only ours 27 → **26**, our diagnostics
68 → **66**, fully agreeing 324 → **325**). Agreement (**32**) is unmoved throughout.

This control is measured independently from the wave-12F head: its Xpect result is **1287 agree,
248 wording-only, 36 disagree**, and its rejection result is **116 both reject / 4 only pilot
rejects / 0 only ours rejects / 0 both accept**. The head is the last measured column below.

| Count | At #356 | After F60–F69 | After wave 3 | After wave 4 | After wave 5 | After wave 6 | After wave 7A | After wave 8 | After wave 9 | After wave 10 | After wave 11 | After wave 12A | At wave 12F (`bbd3b2ec`) — **historical snapshot**, not the current figure | Reason for the wave-12F move | Reason for the wave-12A move | Reason for the wave-11 move | Reason for the wave-10 move | Reason for the wave-9 move | Reason for the 8G move | Reason for the wave-6 move |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| overall: fully agreeing / only ours / our diagnostics | 221 / 560 / 596 | 254 / 343 / 379 | 273 / 281 / 317 | 283 / 232 / 268 | 291 / 167 / 203 | **297 / 161 / 199** | 300 / 147 / 185 | 308 / 119 / 157 | 309 / 119 / 157 | 311 / 142 / 191 | **317 / 119 / 168** | **317 / 119 / 175** | **321 / 92 / 148** | Wave 12F closes 27 only-ours diagnostics on the reference's own corpora: fully agreeing 317 → **321**, only ours 119 → **92**, our diagnostics 175 → **148**. The pilot's columns (122 diagnostics, 66 only-pilot), agreement (**32**) and severity-only (**24**) are all unmoved, so every movement this round is a false positive of ours disappearing: 23 rows are the connector-end parser fix, 4 the anonymous-`enum`-value parse fix. No only-ours row was added, and no remaining row changed bucket because of 12A's `must have` → `kind-mismatch` mapping. | Wave 12A narrows tier gating from the document to the element: agreement 25 → **32** and only-pilot 73 → **66**, our diagnostics 168 → **175**, with only-ours unmoved at **119**. Every one of the seven is a pilot row we now also report on a subject whose own resolution failed, on `testdata` (`parse/expressions.sysml` ×5, `passes/errors.sysml`, `resolve/errors.sysml`). | Wave 11 with the fresh-cache correction: fully agreeing 311 → **317**, only ours 142 → **119**, our diagnostics 191 → **168**. Four of the 23 retired rows are the stale-library-cache measurement correction (a fresh-cache run of the pre-wave-11 tree measures 138 only-ours, not 142); the other 19 are 11F's resolver work — the `Vehicle` diamond family and the redefinition canonicalization — plus 11E's and 11D's rules. The pilot's columns (122 diagnostics, 73 only-pilot), agreement (25) and severity-only (24) are all unmoved, so every movement this round is a false positive of ours disappearing. | No wave-10 slice owned this oracle and it moved anyway: 10C's conformance modes made the non-standard-notation warning fire by default on our own `examples/` demos (+35 only-ours, all true positives about those models), while 10B's and 10D's rule and parser work retired 12 rows on the reference's own corpora (82 → 70). Fully agreeing 309 → **311**, agreement 23 → **25**, severity-only 15 → **24** and only-pilot 85 → **73**, the last two because a line the pilot errors on and we now warn on leaves only-pilot for severity-only. **Read the 119 → 142 by root**: our diagnostics against the reference corpora fell; our warnings about our own examples rose. | Wave 9 moves this oracle by exactly one file, and moves the reference's column instead: only-pilot **137 → 85**. Both are 9B's: its notation warnings added 34 only-ours rows, and rewriting the two example models that provoked them to the spec spelling retired the same 34 — so our column returns to 119 rather than staying at 153 — while the pilot's parse of `examples/repl-behavioral-demo.sysml` now completes (the file becomes fully agreeing, the +1) and its parse of `examples/phase-c-behavioral-bodies.sysml` reaches line 147 instead of stopping at line 60, retiring 52 of its rows. 9A, 9C and 9D moved the Xpect and scope oracles, not this one; 9F moved the rejection oracle. | 8G (#438) measured its own delta against a clean rerun of its merge-base, 306 / 125 / 163 → **308 / 119 / 157**: prefix metadata on `require`/`assume` retires 5 only-ours rows on `Metadata Examples/RequirementMetadataExample.sysml` and a leading `not satisfy` the last one on `Simple Tests/RequirementTest.sysml`, so both files are now fully agreeing. `Simple Tests/DecisionTest.sysml` keeps 2 rows that change category, syntax → `kind-mismatch`: a guarded succession now parses as the transition it is, and `resolve.isVertex` rejects its action-node ends (pinned for wave 8A). The rest of this column is the waves that landed between 7A and 8G. Earlier 7A (#424): −14 only-ours and +3 fully-agreeing files. F93's metaclass-reflection fix removes 3 on `kerml-examples` and 4 on `testdata`, and F68's implied transition members remove 9 on `pilot-examples`, against +3 new `unmapped` specialization-cycle rows on `Simple Tests/PartTest.sysml` that a since-removed error of ours had been masking. The reference's column (137 / 175), agreement (22) and severity-only (16) are all unchanged, so every movement in this round is a false positive of ours disappearing. | Wave 6: −6 only-ours and +6 fully-agreeing files, from #409 (F90, −7 on `kerml-examples`) and #415 (F69's three metadata name conflicts, −3 on `pilot-examples`), against +4 only-ours that #409 deliberately added as committed F93 reproducers on `testdata`. **Agreement moves for the first time, 20 → 22**, because #415 implemented the reference's duplicate-owned-member-name warning (P4) and two pilot-only rows became agreed ones; the reference's diagnostic total (175) is unchanged, and its only-pilot column falls 139 → 137 by exactly those two. Severity-only (16) is unchanged for the fifth round. The round's real result is the Xpect `linkedName` column, not this one. |
| `pilot-examples`: only ours | 314 | 167 | 138 | 109 | 101 | **98** | 91 | 68 | 68 | 63 | **43** | **43** | **16** | −27, all of it here: the n-ary connector ends of `AHFSequences.sysml` and `CauseAndEffectExample.sysml` now parse, so their end names resolve, and `SimpleVehicleModel.sysml`'s `enum = 60 [mm];` members are no longer read as a member named `enum`. `unresolved-reference` 32 → **9** and `unmapped` 9 → **5**. | Unchanged — wave 12A moved the agreement and only-pilot columns, not this row. | −20: `Annex_A_VehicleViews.sysml` 14 → 6 (all 8 duplicate-inherited-name rows, 11F) and `SysML v2 Spec Annex A SimpleVehicleModel.sysml` 10 → 5, with `unmapped` 17 → 9 and `kind-mismatch` 9 → 1. `unresolved-reference` 36 → 32, of which 4 are the cache correction rather than a fix. | −5, and the mix is almost entirely new: all 31 syntax rows are gone (10D parses them), against `unresolved-reference` 27 → 36, `unmapped` 5 → 17 and `kind-mismatch` 4 → 9 unmasked in files that now reach the later tiers. `Annex_A_VehicleViews.sysml` +8 (two groups of four `Duplicate of inherited member name … from Vehicle, vehicle_b` — a false positive of ours where one feature arrives through two supertypes) against `SimpleVehicleModel.sysml` 24 → 10. | Unchanged — no wave-9 item touched this root's diagnostics. | −23 since 7A; 8G's own share is 5, the `RequirementMetadataExample.sysml` prefix-metadata rows. Earlier −7: `Interaction Sequencing Examples/ServerSequenceRealization-2.sysml` 7 → 1 and `ServerSequenceOutsideRealization-2.sysml` 4 → 1 on F68's implied transition members (−9), against the +3 `unmapped` cycle rows unmasked on `Simple Tests/PartTest.sysml`. `unresolved-reference` 37 → 27; the 57 syntax rows are unmoved and are 7C's. | −3, all `unmapped`, all #415: the three `name conflict: … is already the name of the inherited feature ModelingMetadata::…` rows on `Metadata Examples/IssueMetadataExample.sysml` (1) and `RationaleMetadataExample.sysml` (2). Both files are now fully agreeing, which is the +2 in this root's fully-agreeing column. This page previously called those three a **deliberate one-sided check of ours (F69) that stays**; #415 read the reference's `checkMembershipDistinguishability` and found our version wrong in both severity and scope, so the claim that they were intentional is withdrawn. Every other category in this root is unmoved — its 57 syntax and 37 `unresolved-reference` rows are wave 7's. |
| `pilot-validation`: only ours | 59 | 37 | 22 | 10 | 10 | **10** | 10 | 10 | 10 | 3 | **1** | **1** | **1** | Unchanged — its one `unresolved-reference` row is categorized as our defect in wave 12F's census and remains owned by the Step 2 implicit-typing resolver mechanism. | Unchanged — wave 12A moved the agreement and only-pilot columns, not this row. | 3 → 1: the 2 `kind-mismatch` rows are gone with 11F's usage-typing work, leaving 1 `unresolved-reference` on `9-Verification-simplified.sysml`. | **10 → 3**, the first movement in five rounds: the 7 syntax rows are gone with 10D's parser work, leaving 2 `kind-mismatch` and 1 `unresolved-reference`. | Unchanged for the fourth round. | Unchanged in 8G, and its 7 syntax rows are 1 fewer than at 7A. Earlier, unchanged for the third round: 8 syntax and 2 `kind-mismatch`, none of them reflection or transition shapes. | Unchanged for the second round: these 8 syntax and 2 `kind-mismatch` rows are neither KerML-declaration shapes nor conjugation or name-conflict rows. |
| `kerml-examples`: only ours | 150 | 98 | 80 | 72 | 15 | **8** | 5 | 4 | 4 | 4 | **3** | **3** | **3** | Unchanged — the three rows are K5's one-sided specialization-cycle check; 12F re-derived the rule, found the cycles real, and kept them as adjudicated divergences. | Unchanged — wave 12A moved the agreement and only-pilot columns, not this row. | 4 → 3: the `MetadataTest.kerml` `unresolved-reference` row closes with 11G's annotation-body scope. What is left is K5's three one-sided specialization cycles on `Circular.kerml`, so this root now carries nothing unadjudicated. | Unchanged for the third round — wave 10's KerML work shows up in the Xpect oracle, not here. | Unchanged — 9A's and 9D's resolver work shows up in the Xpect oracle's scope rows, not here. | −1 since 7A, none of it 8G's. Earlier −3, all F93: `Simple Tests/Filtering.kerml` 3 → 0 and fully agreeing, which is the +1 in this root. **F93 is now closed at every tier.** The 5 that remain are K5's three specialization cycles on `Circular.kerml` (one-sided by design), 1 `multiplicity` on `Associations.kerml` and 1 `unresolved-reference` on `MetadataTest.kerml`. | −7, all #409 (F90): the `'~' names the conjugated port definition of a port definition, found attributeUsage` check was applying to KerML typings, where `~` is a legal conjugation of any type. `Simple Tests/Conjugation.kerml` 2 → 0 (fully agreeing now, the +1 in this root's column), `Types.kerml` 4 → 0, `Features.kerml` 1 → 0 — the last two still carry the reference's own `do not refer to each other` disjoining rows, so they are not fully agreeing. `kind-mismatch` reaches **0** in this root and `unmapped` 5 → 3. The 8 that remain are unchanged and adjudicated: 3 F93 element-filter false positives on `Filtering.kerml` (the reference is silent; root-caused by #409 to metaclass reflection in `semantics/`, and now also committed as a `testdata` reproducer), K5's 3 specialization cycles on `Circular.kerml` (one-sided by design), 1 `multiplicity` on `Associations.kerml` and 1 `unresolved-reference` on `MetadataTest.kerml`. So F90 is closed at every tier, and F93 is closed in the parser and in `passes/` and open in `semantics/`. |
| `examples`: only pilot | 109 | 423 | 106 | 104 | 104 | **104** | 104 | 104 | 52 | 40 | **40** | **40** | **40** | Unchanged — 12F retires only-ours rows and moves no pilot row. | Unchanged — wave 12A moved the agreement and only-pilot columns, not this row. | Unchanged — no wave-11 item touched our own demo fixtures, and their non-spec notation is still the subject of this column. | −12, none of it a fix: the pilot still errors on the same lines, but we now warn on 12 of them, so the pairs move to severity-only (12 → 23). Our own demos' non-spec notation is the subject of both columns. | −52, all of it the two models 9B rewrote to the spec spelling: `repl-behavioral-demo.sysml` draws nothing from either tool now, and the pilot reads 87 more lines of `phase-c-behavioral-bodies.sysml` before giving up. The 52 that remain are 24 syntax, 15 `unmapped`, 6 `kind-mismatch`, 5 `unresolved-reference` and 2 `multiplicity`, and 16 of them are the `transition … to …` of F3 in that same file. | Unchanged — no wave-8 item touched our own demo fixtures either. Earlier, unchanged — 7A worked in `semantics/`, not on our own demo fixtures. Still the rejection question, now measured by the rejection oracle. | Unchanged — no wave-6 item touched our own demo fixtures either. These 104 are the rejection question, not the agreement one, and they are wave 7's rejection-oracle work. |
| `examples`: fully agreeing | 4 | 4 | 12 | 12 | 12 | 12 | 12 | 14 | 15 | 15 | **15** | **15** | **15** | Unchanged — 12F retires only-ours rows and moves no pilot row. | Unchanged — wave 12A moved the agreement and only-pilot columns, not this row. | Unchanged at 15. | Unchanged at 15 — every file that moved was already non-agreeing, and the notation warnings landed in files that stay non-agreeing. | +1: `repl-behavioral-demo.sysml`, once its calc, constraint and requirement bodies used the spec spelling. | +2 since 7A, none of it 8G's. | Unchanged. |
| `unmapped`, our side | 13 | 13 | 16 | 15 | 17 | **14** | 17 | 16 | 18 | 30 | **22** | **22** | **18** | −4: the `Duplicate of other owned member name` rows on `SimpleVehicleModel.sysml`, which were our parse defect rather than a name conflict. | Unchanged — wave 12A moved the agreement and only-pilot columns, not this row. | 30 → **22**: the 8 `Duplicate of inherited member name` rows of the `Vehicle` diamond, retired by 11F. The remaining 22 are the specialization-cycle family, the 5 `Duplicate of other owned member name` agreements-by-message-class and the `interface Mounting` conjugation row. | 18 → **30**: +12 rows, no new message class beyond the duplicate-inherited-name family — the `Vehicle` diamond rows above plus 3 more `Duplicate of other owned member name`. The cell counts diagnostics, not distinct messages. | Unchanged: no wave-9 item added an `unmapped` diagnostic of ours. The cell counts diagnostics, not distinct messages — the baseline lists 16 `unmapped` messages of ours summing to 18 rows, and the wave-8 column recorded the message count by mistake against the same 18. | +1 since 7A, none of it 8G's. Earlier +3: the `participates in a specialization cycle` rows on `Simple Tests/PartTest.sysml:49–51`, unmasked rather than newly emitted. The same file's `part p4 :> p4;` at line 53 draws nothing from us, so this round also exposes that our cycle check misses a self-loop. | −5 and +2. Gone: F90's two `'~' names the conjugated port definition` rows (#409) and F69's three `name conflict … ModelingMetadata::…` rows (#415). Added: two `Duplicate of other owned member name` rows of **ours** on `testdata/passes/corpus_notation.sysml:33,34` — which are not a disagreement at all but the two new **agreements**, counted here because the message maps to no category. The remaining 12 are K5/F4's 11 specialization cycles and the `interface Mounting` conjugation row. |
| new checks of ours | — | — | +3 rules | −2 rules relaxed | none | +1 rule, 1 narrowed | none | none | +1 rule, 1 relaxed | +1 rule, 1 relaxed | +6 rules | +6 rules | +6 rules | none — 12F adds no check. It fixes two parse defects and one resolver defect, and re-derives three existing rules (interface arity from `Links::BinaryLink` conformance, related-element counting through semantic supertypes, event occurrences as referential) so they stop firing on models the pilot accepts. | none — 12A changed when existing checks may speak, not what they check. | 11F adds the two use-case rules and the interface-end implicit `Ports::Port` base, 11E the KerML structural residue, 11D the model-level evaluability predicate and 11G annotation-body resolution. None of them adds a diagnostic to any OMG root; training stays 0/0. | 10C makes the wave-9B notation warning an error/warning by conformance mode rather than adding a rule, 10E removes the specialization relaxation from qualified-tail resolution, and 10A bounds scope re-entry. No new check of ours reaches an OMG root; training stays 0/0. | 9B adds the notation warning that names our non-conformant spellings, and 9D relaxes visibility so protected and private members are visible where the reference makes them visible — which is why the Xpect `noErrors` column improves and the declared-`errors` silence worsens in the same wave. | 8G adds no check: it accepts four notations the reference accepts and moves two classification residues into the export, so agreement did not move. | 7A adds no check. It removes false positives and supplies implied members, which is why agreement did not move. | #415 implements the reference's `Duplicate of other owned member name` warning (P4) — the only new check of wave 6, and the first check we have added that produced an **agreement** rather than a one-sided row. It also narrows the pre-existing name-conflict check to the reference's scope and severity: a warning, not an error, and not applied to imported memberships, automatically-constructed expressions, binding connectors or synthetic successions. Neither change adds a diagnostic to any OMG root, and training stays 0/0. |

The KerML root is now the *cleanest* of the three OMG roots in proportion: **3** only-ours against 6
only-pilot — the only root where the reference reports more than we do — with 50 of 58 files fully
agreeing (439 / 6 and 10 / 58 when the root was added, 72 and 39 before wave 5, 15 and 47 before
wave 6, 8 and 48 before 7A). None of the 3 is a syntax or `kind-mismatch` diagnostic — the notation
the reference accepts, we parse, and the checks we applied to KerML typings that it does not apply
are gone. What is left is K5's three specialization cycles, all adjudicated. The class tables below
are kept as measured when each class was adjudicated, so they describe the root at 150 rather than at 3.

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
recovery cascades. Their 136 pilot-only diagnostics are therefore dominated by a handful of
root causes, adjudicated next. The 314 that F34 surfaced on the 10 `.kerml` demo fixtures are
gone: F96 made those fixtures honest, and none of them is a `.kerml` diagnostic any more.

---

## Adjudications

### Only ours — candidate false positives (3, SysML side)

The three diagnostics below are the `testdata` only-ours set. The six cycle diagnostics on the
`probes` root are adjudicated with F4 below, the KerML root has its own tables further down,
and the 373 diagnostics on the two OMG SysML roots are adjudicated in
[SysML corpora — only ours](#sysml-corpora--only-ours-373), which supersedes this paragraph's
earlier "not adjudicated in this pass" note. The four productions called out here previously —
`connect a to b { ... }`, `flow a.x to b.y { ... }`, anonymous `interface a.p to b.q`, and
`accept` on an action usage declaration — are fixed: `02-Parts Interconnection/2a-Parts
Interconnection.sysml` is now fully agreeing, and `03-Function-based Behavior/3a-Function-based
Behavior-1.sysml` keeps only the S3 diagnostics adjudicated below.

The two keyword-as-name rows that stood here are **fixed** (F1): `on` appears as a literal in
none of the pilot's grammars, and `var` only in `KerML.xtext` (`BasicFeaturePrefix`), so both
are now matched contextually and are names everywhere else. The four diagnostics they produced
are gone from the three files listed in the movement table above.

| Files | Diagnostic | Verdict |
|---|---|---|
| ~~`passes/errors.sysml:4`, `resolve/errors.sysml:4`~~ | ~~`unresolved reference: Nowhere`~~ | **No longer a disagreement.** These were negative fixtures where the pilot was silent only because a bare `import` earlier in the same file broke its parse before it got there (see P1). Since F2 gave our fixtures an explicit visibility, the pilot parses them and reports `Nowhere` too: both rows are now agreement. |
| `passes/constraints.sysml:2,3` | `A`/`B` `participates in a specialization cycle` (`unmapped`) | **Ours is right, and the pilot has no such check** — settled by F4, both by reading its validators and by probing it on clean files (see [Specialization cycles](#specialization-cycles-f4)). The silence is not a parse cascade of the kind P1 describes: the same three cycle shapes in files with nothing else in them are accepted by the pilot with zero diagnostics. A one-sided finding, so it is our extension of the reference rather than a disagreement — kept `unmapped` because no coarse category honestly covers it. |
| `passes/constraints.sysml:9` | `multiplicity lower bound exceeds upper bound on lo` | **Ours is right**: `part lo [5..2];`. No pilot counterpart. |

### SysML corpora — only ours (373)

**The two OMG SysML roots' only-ours count when these classes were adjudicated was 373** —
`pilot-examples` 314 and `pilot-validation` 59: 274 syntax, 82
`unresolved-reference`, 14 `kind-mismatch`, 2 `unmapped`, 1 `units`, spread over 73 of the two
roots' 154 files (81 files carry none). Together with the KerML root's 150, `testdata`'s 3, the
probes' 6 and `examples`' 28 that is the entire only-ours column. The `examples` 28 are all
`nonstandard-notation` warnings on *our own* demo models — the F3 extensions firing exactly where
they are meant to: bare `transition <source> to <target>;` (24), `initial <state>;` (2),
`region <name> { … }` (1) and `junction <name>;` (1). They carry the `syntax` category at
`warning` severity, are one-sided by construction, and are adjudicated with F3, not here.

Every count in this section is the **pre-fix** measurement, and it is left as measured so the
adjudications stay checkable against the evidence that produced them. After the fix round
(#361, #362, #363, #364) the same two roots carry **204** only-ours diagnostics; see the movement
table above for where the 169 went and which categories were unmasked rather than removed.

Method, per file: take the **first** only-ours diagnostic, read the construct it sits on, write
the smallest file that shows the same construct, and run that file through both our checker and
the pinned pilot. 85 such reproducers were run (`bin/sysml -validate` and
`build/pilot-validator/validate-sysml` on the same file); they are quoted inline below rather
than committed, so that no corpus root gains a file and the baseline does not move. Where a
reproducer failed to reproduce the corpus diagnostic, that is said so; nothing below is
classified from a reproducer that did not discriminate.

Two things the counts do **not** mean:

- **175 of the 274 syntax diagnostics are recovery, not findings.** They are the two generic
  messages emitted as the enclosing bodies unwind after the first unparsed member:
  `expected a body member` (102) and `expected a namespace member` (73). The 198 remaining
  only-ours diagnostics carry a construct-specific message.
- **Attribution is per file, by the construct the file's first diagnostic sits on.** Long files
  mix classes: `Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml` is counted under
  S2 for its line 424, yet its 57 diagnostics also include S1's `expected ';' or '{' after a
  metadata usage` (3), S3's `expected ';' after send statement` (2) and S5's `expected ';'
  after return expression` (5). So the "Diags" column is what a class's files hold, not what its
  construct provably produces — and the per-class sections below name *representative* files, not
  every file the class owns (S2's 87, for instance, span 12 files of which 9 are named).

| # | Class — the construct the file's first only-ours diagnostic sits on | Files | Diags | Verdict | Follow-up |
|---|---|---:|---:|---|---|
| S1 | Prefix metadata used as a member's only keyword (`#M connect a to b;`, `#service x : PortDef;`, `end #original r1 : Req1;`) | 7 | 32 | **Ours** — the pilot's `ExtendedUsage` | [F60](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S2 | Keyword-less members: value-only, specialization-only, redefinition-only, anonymous enumerated values, result expressions, `locale` | 12 | 87 | **Ours** — `DefaultReferenceUsage`, `EnumeratedValue`, `ResultExpressionMember` | [F61](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S3 | State/occurrence behavior: qualified transition targets, bodies on `then`/`accept`/`send`, `exhibit` of a dotted state | 7 | 65 | **Ours** — `TargetTransitionUsage`, `SendNode`, `ExhibitStateUsage` | [F62](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S4 | Action nodes: bodies on control nodes, `decide`, typed `for` variables, redefining body parameters, `ref x { … }` | 5 | 33 | **Ours** — `ControlNode`, `ForVariableDeclaration`, `UsageBody` | [F63](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S5 | Calculations and expressions: `return` of a usage element, declarations inside expression bodies, `assert not` | 6 | 19 | **Ours** — `ReturnParameterMember`, `OperatorExpression` | [F64](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S6 | Connectors and interactions: typed `binding … bind`, `message … of P[1]`, `event x = y.start;` | 5 | 22 | **Ours** — `BindingConnectorAsUsage`, `MessageDeclaration`, `EventOccurrenceUsage` | [F65](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S7 | Requirement/case clauses: `assume constraint`, `verify … :>>`, `variant use case`, multiplicity after `redefines` | 5 | 25 | **Ours** — `RequirementConstraintMember`, `UseCaseUsage`, `FeatureSpecializationPart` | [F66](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S8 | Names of inherited, redefined and imported members (`item :>> shape : Box`, `variation … :> Diameter`, `filter @Safety`) | 12 | 43 | **Ours** — resolution, not syntax | [F67](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S9 | Members reached through a behavioral usage's implicit parameters (`subscribing.sub`, `producer.publish_request`) | 6 | 39 | **Ours** — resolution, not syntax | [F68](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| S10 | Semantic checks stricter than the reference's (kind table, binding types, name conflicts, units, conjugation) | 8 | 8 | **Mixed**: 5 ours, 3 one-sided | [F69](#follow-ups-not-fixed-here--this-pr-is-advisory) |

Every one of the 373 is accounted for by exactly one class; no file appears twice. The verdict
across the ten classes is **370 ours** (195 construct-specific plus the 175 recovery
diagnostics they drag in) and **3 one-sided** — checks we
have and the reference does not. **Not one diagnostic is a pilot artifact**: unlike the
`testdata`/`examples` rows, every file here was written by the reference's own authors for the
reference, and the pilot is silent on all 73 — both roots report `pilotOnly: 0` and
`pilotDiagnostics: 0`.

#### S1 — prefix metadata as a member's only keyword (F60, 32)

`Cause and Effect Examples/CauseAndEffectExample.sysml:25` is `#multicausation connect …` and
`Arrowhead Framework Example/AHFCoreLib.sysml:28` is `#service serviceDiscovery :
ServiceDiscovery ;`. Reproducers:

```sysml
package B2 { metadata def M; part a; part b; #M connect a to b; }
package B7 { port def ServiceDiscovery; metadata def service; part def P { #service sd : ServiceDiscovery; } }
```

Ours: `expected a namespace member` on the `#`, and — where the member does parse —
`attribute cannot be typed by portDef (kind mismatch)`. Pilot: clean on both. The grammar is
explicit: `UsagePrefix` ends in `UsageExtensionKeyword*` (`SysML.xtext:582`),
`UsageExtensionKeyword` *is* a `PrefixMetadataMember` (`:578`), and `ExtendedUsage` is
`UnextendedUsagePrefix UsageExtensionKeyword+ Usage` returning a plain `SysML::Usage`
(`:730`) — so one or more `#M` annotations may stand where a kind keyword would, both alone and
after `end`/`ref`/`abstract`, and the member is *not* an attribute usage. That second half is
why 6 of these are `kind-mismatch` rather than syntax: we do parse `end #original r1 : Req1;`
(`Requirements Examples/RequirementDerivationExample.sysml:10`) and
`#service sd : ServiceDiscovery;`, but as attribute usages, so the kind check then rejects a
requirement or port definition as their type. **Ours, one parser gap with two faces.**

#### S2 — keyword-less members (F61, 87)

The class the biggest files are led by, and the widest. Four productions, six corpus shapes:

| Corpus | Construct | Ours | Pilot |
|---|---|---|---|
| `Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml:424` (57 diags), `v1 Spec Examples/8.4.1 Wheel Hub Assembly/Wheel Package.sysml:9` | `distancePerVolume :> scalarQuantities = distance / volume;` — specialization and value, no keyword | `expected a namespace member` | clean (`Wheel Package.sysml`: a `Bound features should have conforming types` warning) |
| `Vehicle Example/VehicleUsages.sysml:14` | `T1 = 10.0 [N * m];` — value only | `expected a namespace member` | clean |
| `Simple Tests/EnumerationTest.sysml:48` | `= 60.0;` — an anonymous enumerated value | `expected a body member` | clean on the value itself |
| `Simple Tests/AnalysisTest.sysml:20`, `10-Analysis and Trades/10a-Analysis.sysml:52`, `Simple Tests/VerificationTest.sysml:21` | a bare expression as the last body member (`v.m`, `VerificationCases::PassIf(v.m == 0)`) | `expected a body member` | clean |
| `15-Properties-Values-Expressions/15_11-Variable Length Collection Types.sysml:15` | `value :>> elements: Integer;` — a redefinition only; `value` here is the member's *name*, reserved by neither grammar | `expected a body member` | clean |
| `Simple Tests/CommentTest.sysml:25` | `locale "en_US" /* … */` — an anonymous comment carrying a locale | `expected a namespace member` | clean; it rejects the reproducer only because that file has `locale` with *no* comment body after it, which is exactly what its `Comment` production requires |

Reproducers `a3`/`a4` (`torquePerCurrent :> scalarQuantities = 1.0;` at body level, `T1 = 10.0;`
at namespace level), `f6` (two anonymous `= 60.0;` enumerated values) and `f8` (`value :>>
elements : ScalarValues::Integer;`) each reproduce their corpus message with the pilot accepting
the member; `f7` reproduces ours and reads the reference wrong (see the last row). `DefaultReferenceUsage` (`SysML.xtext:632`) requires
no keyword at all — `('end')? RefPrefix UsageDeclaration ValuePart? UsageBody`, and a
`UsageDeclaration` may be a name, a specialization, a redefinition, or any combination;
`EnumeratedValue` (`:786`) makes both the keyword and the declaration optional
(`UsageExtensionKeyword* EnumerationUsageKeyword? Usage`); `ResultExpressionMember` (`:1967`)
allows a trailing expression as a body member; and `Comment` (`:86`) makes its `comment` keyword
optional, so `locale "en_US"` followed by a comment body is an anonymous comment with a locale —
not, as the `f7` reproducer assumed, part of the package declaration. **Ours, four parser
gaps.** This is where the recovery noise concentrates: 72 of the class's 87 are the
two generic messages, and the 15 that remain are other classes' constructs in the same files.

#### S3 — state and occurrence behavior (F62, 65)

| Corpus | Construct | Ours | Pilot |
|---|---|---|---|
| `Simple Tests/StateTest.sysml:30` | `then S2.S3;` — a qualified transition target | `expected ';' after transition` | clean |
| `05-State-based Behavior/5-State-based Behavior-1.sysml:86`, `-1a.sysml:87` | `then starting { … }` — a body on the target transition | `expected ';' after transition` | clean |
| `Vehicle Example/Annex_A_VehicleViews.sysml:518` (24 diags) | `action turnVehicleOn send ignitionCmd via driver.p1 { … }` | `expected ';' after send statement` | clean but for two `Duplicate of inherited member name 'self'` warnings |
| `Arrowhead Framework Example/AHFNorwayTopics.sysml:94` | `accept cl : CallGiveItems via tellu.APIS_HTTP` continued over lines | `expected a body member` | clean |
| `03-Function-based Behavior/3a-Function-based Behavior-1.sysml:85` | `first start then continue { … }` — a body on a succession | `expected ';' after initial node` | clean |
| `06-Individual and Snapshots/6-Individual and Snapshots.sysml:113` | `exhibit vehicleStates.on { … }` — dotted reference plus body | `expected '{' or ';'` | clean |

All six reproduce (`d1`, `d2`, `d4`, `e4`, `e5`). `TransitionUsage`, `TargetTransitionUsage`,
`SendNode` and `AcceptNode` all end in `ActionBody`, and `ExhibitStateUsage` is
`'exhibit' ( OwnedReferenceSubsetting FeatureSpecializationPart? | StateUsageKeyword
UsageDeclaration? ) ValuePart? StateUsageBody` — a dotted reference to an existing state, with a
body. In every case we accept the head of the construct and then require `;` where the reference
allows a body. **Ours, one shape of gap in six places:** a body is refused where the reference takes one.

#### S4 — action nodes (F63, 33)

| Corpus | Construct | Ours | Pilot |
|---|---|---|---|
| `Simple Tests/ControlNodeTest.sysml:13` | `then fork F { … }` | `expected ';' after fork node` | clean |
| `Simple Tests/DecisionTest.sysml:4` | `decide 'test x';` — a named decision node | `expected ';' after decision node` | clean |
| `Simple Tests/StructuredControlTest.sysml:32` | `for n : ScalarValues::Integer in (1, 2, 3) { … }` — a typed loop variable | `expected 'in' keyword after for variable` | clean |
| `Simple Tests/ActionTest.sysml:35` | `in :>> payload = s;` — a body parameter that only redefines | `expected ';' after body parameter` | clean |
| `Cause and Effect Examples/MedicalDeviceFailure.sysml:12` | `ref patient { … }` — a body on a bare `ref` | `expected a body member` | clean |

All five reproduce (`e1`, `e3`, `e7`, `e6`, `g7`). Each `ControlNode` alternative —
`MergeNode`, `DecisionNode`, `JoinNode`, `ForkNode` — is
`ControlNodePrefix isComposite ?= '<kw>' UsageDeclaration? ActionBody`, so it both takes an
optional name and ends in a body; `ForVariableDeclaration` (`:1637`) is a full
`UsageDeclaration`, so the loop variable may state its type before `in`; and a body parameter
needs only a `FeatureSpecializationPart` — a name is optional when the parameter redefines one.
**Ours, four parser gaps** (`fork`/`join`/`merge`/`decide` bodies are one).

#### S5 — calculations and expressions (F64, 19)

| Corpus | Construct | Ours | Pilot |
|---|---|---|---|
| `Metadata Examples/RationaleMetadataExample.sysml:22`, `10-Analysis and Trades/10d-Dynamics Analysis.sysml:60`, `10b-Trade-off Among Alternative Configurations.sysml:82` | `return selectedEngine :> engine;`, `return attribute accelerationProfile :> ISQ::acceleration[*] := ();` | `expected ';' after return expression` | clean |
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml:56`, `Analysis Examples/Vehicle Analysis Demo.sysml:207` | a `private attribute` declaration inside an expression body | `expected '}'` | clean on the declaration (it reports its own unrelated `forAll` resolution error) |
| `Simple Tests/ConstraintTest.sysml:89` | `assert not massLimitation { … }` | `expected '{' or ';'`, plus `"not" is a reserved keyword` | clean |

All three reproduce (`f1`/`f2`, `f5`, `f4`). `ReturnParameterMember` is `'return' UsageElement`
— a named, specializing, even keyword-carrying usage, not just an expression — and `assert`
takes an `OperatorExpression`, so `not` binds a constraint rather than naming one.
**Ours, three parser gaps.**

#### S6 — connectors and interactions (F65, 22)

| Corpus | Construct | Ours | Pilot |
|---|---|---|---|
| `Simple Tests/ConnectionTest.sysml:24` | `binding ab1 : AB bind a = b;` — a typed binding connector | `expected '{' or ';' after declaration`, then `"bind" is a reserved keyword` | rejects our reproducer's `binding def` (grammar has no such keyword) but takes `binding ab1 : AB bind x = y;` itself |
| `17-Sequence Modeling/17a-Sequence-Modeling.sysml:26`, `17b`, `Arrowhead Framework Example/AHFSequences.sysml:45` | `message publish_message of Publish[1] …` — a payload type with multiplicity | `expected '{' or ';' after declaration` | clean |
| `Interaction Sequencing Examples/ServerSequenceModelOutside.sysml:6` | `event publish_source_event = publish_message.start;` | `expected a body member` | reports its own `Must reference an occurrence` on the reproducer |

`c1`, `c2`/`c3` and `g8` reproduce the ours side; the pilot's side is clean only for `message`.
`BindingConnectorAsUsage` is `'binding' UsageDeclaration? 'bind' … '=' …`, so the declaration may
carry a type before `bind`; the `Payload` after `of` is `OwnedFeatureTyping
( OwnedMultiplicity )?`, i.e. `Publish[1]` exactly; and `EventOccurrenceUsage` ends in
`UsageCompletion` (`ValuePart? UsageBody`), so `= m.start` is its value.
**Ours, three parser gaps** — with the caveat that the `event` and `binding` reproducers each
also draw a *different* pilot diagnostic, so those two need a corpus-faithful reproducer before
a fix is validated against the reference rather than against the grammar alone.

#### S7 — requirement and case clauses (F66, 25)

| Corpus | Construct | Ours | Pilot |
|---|---|---|---|
| `Simple Tests/RequirementTest.sysml:6`, `08-Requirements/8-Requirements.sysml:111` | `assume constraint c1 : C;`, `assume constraint fuelConstraint { … }` | `expected '{' after 'assume constraint'` | clean |
| `09-Verification/9-Verification-simplified.sysml:55` | `verify vehicleMassRequirement :>> massRequirement;` | `expected '{' or ';'` | clean |
| `Simple Tests/VariabilityTest.sysml:29` | `variant use case uc11;` | `expected '{' or ';' after declaration`, plus `"use" is a reserved keyword` | clean |
| `v1 Spec Examples/8.4.5 Constraining Decomposition/Vehicle Decomposition.sysml:45` (12 diags) | `ref redefines cylinderBR[4];` — multiplicity after a redefinition | `expected '{' or ';' after declaration` | clean |

All four reproduce (`g1`/`g2`, `g3`, `g5`, `g4`). `RequirementConstraintMember` (`:2057`) is
`kind = ('assume'|'require')` plus a `RequirementConstraintUsage` — a full usage with a
declaration and an *optional* body, where we require a brace; `RequirementVerificationUsage`
begins `OwnedReferenceSubsetting FeatureSpecialization*`, so `verify` takes a usage that only
redefines; `UseCaseUsage` is reachable from
`VariantUsageElement`, so `use case` is a kind keyword after `variant`; and
`FeatureSpecializationPart` puts multiplicity after the specialization, not only after a name.
**Ours, four parser gaps.** The last is the clearest arithmetic in the class: six identical
lines × two diagnostics each = the file's 12.

#### S8 — inherited, redefined and imported names (F67, 43)

Not syntax: every diagnostic here is `unresolved reference`, and the pilot resolves the name.
The largest is `item :>> shape : Box [1] { … }` (`Geometry Examples/CarWithShapeAndCSG.sysml:48`,
`SimpleQuadcopter.sysml:15`), 12 diagnostics of `unresolved reference: shape — did you mean
LugBolt::shape?` — the same inherited-member lookup PR #331 fixed for `length`/`width`/`height`
through `ShapeItems::Box`, still failing when the redefinition *itself* introduces the type.
Others: `variation attribute def DiameterChoices :> Diameter { … }`
(`Variability Examples/VehicleVariabilityModel.sysml:71`, 14), an enumeration imported by
`private import RiskLevelEnum::*;` (`Metadata Examples/RiskMetadataExample.sysml:3`, and
`VerificationMetadataExample.sysml`) — the name is itself introduced by an import in the
imported namespace — `filter @Safety and (as Safety).isMandatory;`
(`11-View and Viewpoint/11b-Safety and Security Feature Views.sysml:57`), `actor :>> fueler =
driver;` (`18-Use Case/18-Use Case.sysml:48`), and `part aa subsets a;` where `a` is reachable
by feature chain (`Simple Tests/FeaturePathTest.sysml:24`).

Reproducers split: `h3` (import of an imported name) and `h5` (subsetting a feature reachable by
chain) reproduce with the pilot resolving; `h1` (redefined inherited shape) and `h7`
(`filter @Safety`) do **not** — both need the corpus's library imports and view context, and in
isolation both tools agree. So this class is **ours** on the two reproduced shapes and
**ours, unreproduced in isolation** on the rest: the corpus files themselves are the evidence,
and each will need its own fixture built from the real context before a fix is claimed.

#### S9 — members through implicit parameters (F68, 39)

All 39 are `unresolved member`, all in files the reference validates cleanly, and all reach
*through* a behavioral usage into what it implicitly parameterizes:
`subscribing.sub` (`Interaction Sequencing Examples/ServerSequenceRealization-2.sysml:42`, 13
diagnostics; `-OutsideRealization-2.sysml`, 10), `producer.publish_request`
(`-3.sysml:134`, 6; `-Outside-3.sysml`, 6), `x.p to a1.aa.receiver` in a `succession flow`
(`Simple Tests/PartTest.sysml:25`), and `rep inOCL language "ocl" /* self.x > 0.0 */`
(`Simple Tests/TextualRepresentationTest.sysml:7`, 3 — a `TextualRepresentation`
(`SysML.xtext:103`) as a member of a constraint body, which we read as an expression, hence
`unresolved reference: rep`, `inOCL`, `language`). The `h9`/`h6` reproducers do not
discriminate — minimal versions of `subscribing.sub` and `succession flow … receiver` are clean
on both sides — so this class is **ours, with the corpus files as the evidence**: the
membership our resolver exposes for an action/state usage's implicit parameters is narrower than
the reference's, but a faithful fixture has to come from the corpus context, and `rep` is a
separate, small lexer/parser item.

#### S10 — checks stricter than the reference (F69, 8)

Eight files, one diagnostic each, and the only class where the verdict splits.

| Corpus | Ours | Pilot | Verdict |
|---|---|---|---|
| `Simple Tests/ItemTest.sysml:11`, `IndividualTest.sysml:12` | `part cannot be typed by itemDef (kind mismatch)` | clean; its own counterpart message is `An occurrence, item or part must be typed by occurrence definitions` | **Ours**: the reference admits any occurrence definition for a part usage, we demand a part definition. Reproduced by `i1` |
| `Simple Tests/UseCaseTest.sysml:35` | `case cannot be typed by useCaseDef (kind mismatch)` | clean | **Ours**: a use case definition is a case definition. Reproduced by `i2` |
| `03-Function-based Behavior/3e-Function-based Behavior-item.sysml:43` | `cannot bind a value of type VehicleAssembly to a feature typed by AssembledVehicle` | clean | **Ours**: reproduced by `i5`; the value's type specializes the feature's through the corpus's definitions |
| `03-Function-based Behavior/3c-…-structure mod-1.sysml:42` | `type must be a definition, found calcUsage` on `OccurrenceFunctions::destroy` | *not* clean on the reproducer (`An action must be typed by action definitions`) | **Ours, unreproduced**: in the corpus the reference accepts the library `destroy`; our resolution of it yields a calc usage. Needs a library-faithful fixture |
| `Metadata Examples/IssueMetadataExample.sysml:7` | `name conflict: text is already the name of the inherited feature …::text` | clean | **One-sided**: the member redefines the inherited `text` implicitly; reproduced by `i6`, and no reference counterpart exists |
| `Vehicle Example/VehicleDefinitions.sysml:47` | `interface … ports … are not conjugate` (warning) | clean | **One-sided**: our own advice, reproduced by `i7`; the reference has no conjugation check |
| `Analysis Examples/Turbojet Stage Analysis.sysml:25` | `operator '+' combines incommensurable quantities` (`units`) | clean | **One-sided**: probed directly — `attribute s = a + t;` over `LengthValue` and `TemperatureValue` draws our dimension warning and *nothing* from the reference, which has no dimensional analysis at all. Like F4/K5, our extension |

The five "ours" rows are the honest count of semantic false positives in this historical slice:
five, out of 373. Its three one-sided rows stay, on the F4 precedent — a check the reference lacks
is not a disagreement. `VehicleGeometryAndCoordinateFrames.sysml:38` later joins Turbojet's units
row in the same adjudicated quantity-commensurability family; the current census above supersedes
this eight-row snapshot.

#### What this class list predicts

If S1–S7 are fixed, the 274 syntax diagnostics — 175 of them the recovery messages inside them —
go with them; S8/S9 move 82 `unresolved-reference`; S10's five ours-rows move 5. That empties
`kind-mismatch` (14 → 0): 9 sit in S1's two files (its six `attribute cannot be typed by …` plus
`RequirementDerivationExample.sysml:32,33,34`) and go with S1's fix, and the other 5 are S10's. The three one-sided diagnostics stay by
design. That is the movement any fix PR should be measured against, per root and per category,
and it is why the fixes are sequenced parser-first: while a file's first member fails to parse,
nothing downstream of it is measurable.

### KerML — only ours

**The root's only-ours count when these classes were adjudicated was 150** (140 syntax, 7
`unresolved-reference`, 3 `unmapped`); after the SysML-side parser fix round it is **98** (85
syntax, 10 `unresolved-reference`, 3 `unmapped`) — the parser fixes are language-independent, and
the `unresolved-reference` rise is unmasking, not regression (movement table above). The counts
below are left as measured so each verdict stays checkable against its evidence;
the table below is a **history**, not an addition. Each row's count is what the class measured
*when it was adjudicated* — K1–K5 were adjudicated when the root stood at 439, so those counts
sum to 439 and no longer describe the root. K3 carries F31's before/after figures. For the 150 as
they stood, adjudicated one diagnostic at a time, see
[The 150, adjudicated diagnostic by diagnostic (K7–K18)](#the-150-adjudicated-diagnostic-by-diagnostic-k7k18).

Verdicts across the five classes: **436 ours** and **3 one-sided** (K5, a check the reference
does not have), with none attributable to the bridge — it validates one batch in one resource
set, so it has no ordering or name-accumulation artifact to produce. Each class carries its
follow-up, struck through once that follow-up is done.

| # | Class | Count | Verdict |
|---|---|---:|---|
| ~~K1~~ | **Fixed by F30.** `featured by` is not parsed: `expected a body member: 'featured' relates the declaration written before it, so a member cannot begin with it` (43), then `expected '{' or ';' after declaration` (95) and `expected a namespace member` (169) as the enclosing bodies unwind | 307 | **Ours (over-restriction).** KerML's featuring relationship (`member feature inCart: ShoppingCart[0..1] featured by Product_Account;`) is notation the reference accepts silently. One unparsed keyword produces 70% of the root's diagnostics: `Association Examples/ProductSelection_N_ary.kerml:38,40,42` cascade to `:51,53,54`. The featuring relationship is now parsed as `ast.RelFeaturedBy` (`KerML.xtext:569` `TypeFeaturingPart`, `:659` `OwnedTypeFeaturing`) and warned as `kerml-notation` in `.sysml`. Re-measured alone, the root falls 439 → **268** and its syntax diagnostics 360 → 172. |
| ~~K2~~ | **Fixed by F30.** Other KerML notation we reject: `expected a body member` on n-ary connector end lists (36), `expected 'then' between connector ends` on a typed/redefining succession (8), `"at"`/`"while"`/`"merge"` `is a reserved keyword` inside `expr` bodies (8), `expected a name` (6), `expected '{' or ';'` (3) | 61 | **Ours (over-restriction).** `connector ps1 : ProductSelection (myCart, products, myAccount);` (`Association Examples/ProductSelection_N_ary.kerml:122,124`), `succession redefines p_before_d : MyPaint_Before_Dry_Link [1] first paint then dry;` (`KerML Spec Annex A Examples/A-3-6-Sequences.kerml:58,60`), and `expr at { ... }` / `expr while { ... }` (`Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:16,25`) are all accepted by the reference. The keyword rows are the KerML half of F8. All three named constructs are now parsed (`KerML.xtext:842` `NaryConnectorDeclaration`, `:891` `SuccessionDeclaration`, and `at`/`while`/`merge`/`decide` unreserved in `.kerml` since they are literals of `SysML.xtext` only); after K1+K2 the root's syntax diagnostics are **140** (360 before F30) and the root is **291**, the rise over K1's 268 being newly reachable unresolved references — see K3/F31. Left open: `abstract var feature x [0..*];` and `member abstract feature x …` (2 diagnostics, `Variable Feature Examples/TimeVaryingCarDriver.kerml:53,100`), follow-up F50. |
| K3 | `unresolved reference` / `unresolved member` | 43 → **123** → **7** | **Ours (name resolution). Fixed by F31, with 7 left open.** Re-measured on merged `origin/main` (with #343, #349 and #350 in), the KerML root is **269** only-ours and **123** of them are this class (91 `unresolved-reference` + 32 `unresolved-member`) — the F30 branch's figure reproduces exactly, so the denominator is unchanged. After F31 the root is **150** and the class is **7**: 140 syntax (unmoved), 0 `kind-mismatch` (3 before — the `A-3-4-OneToUnrestrictedConnectors.kerml:43,48,56` conformance rows F32 recorded as downstream of the unresolved `BikeFork` import, gone with it), 3 `unmapped` (K5's cycles, unmoved). Classification of the 123, each diagnostic in exactly one cause and each verdict backed by a minimal reproducer the pilot is silent on and we reported: **58 — implicit generalization was not part of inherited-member traversal.** KerML's keyword-implied supertypes (`class` → `Occurrences::Occurrence`, `struct` → `Objects::Object`, `assoc` → `Links::Link`, …) were never contributed to a `.kerml` declaration's supertypes, so no library member was inherited: the brief's shape (1), `portion focusedState: Camera subsets timeSlices;` (`Behavior Examples/Camera.kerml:4,5`) and all 25 of `A-3-8-ChangingFeatureValues.kerml`. Traversal also had to follow same-named subsettings/redefinitions to the *inherited* feature rather than the local binding, and the library cache had to carry semantic supertype edges (format 17) or a cached restore lost the inheritance. **15 — import visibility.** The brief's shape (2) is not a workspace-indexing or bridge artifact: `OneToOneConnectorsExecution` in the sibling file *is* indexed, and the failures were that a `public import` was not re-exported to importers of the importing namespace, that a root-level import was invisible from a nested package, and that an imported name could not serve as the prefix of a qualified name. **39 — a declaration's header did not see its own body.** Names written in a header (`featured by`, `crosses`, subsetting) that a member of the same declaration's body declares, and the same members reached from outside by qualified path or feature chain — the brief's shape (3): `member feature inCart: ShoppingCart[0..1] featured by Product_Account { member feature Product_Account : Account featured by Product; }` (`Association Examples/ProductSelection_N_ary.kerml:37,46,55`) and `member step merge : … featured by TakePicture_snapshots { … }` (`TimeVaryingSteps.kerml`). **4 — the implicit base was suppressed by any declared generalization.** `struct MyWheel1 specializes Wheel` (`A-3-5-TimingForStructures.kerml:148,154,159,164`) still implicitly specializes `Objects::Object`, because KerML 1.0 §8.4.2 suppresses the implicit specialization only when the type already specializes that base directly or indirectly, and `classifier Wheel;` reaches only `Base::Anything`; `ExtendedOccurrences.kerml:51` redefining `Objects::Object::self` under `struct ExtendedObject :> ExtendedOccurrence` is the pilot-side witness for the same reading. **4 — downstream of notation we do not parse, not ours:** `rep inOCL language "ocl"` (`Simple Tests/TextualRepresentation.kerml:7`, 3, follow-up F70) and `in timeslice : Timeslice;` inside `expr while { … }` (`ExtendedOccurrences.kerml:27`, follow-up F71 — the parameter's name lands in `Usage.Keyword` with an empty `Ident`, so it never reaches symbol construction). **3 — open, mechanism not established:** `Product_Account1 subsets Product_Account` and its two siblings (`ProductSelection_N_ary.kerml:93,101,109`, follow-up F72). No cause in this class is a reference-implementation artifact, and none is a fixture artifact. |
| K4 | SysML-shaped semantic checks firing on KerML: `only a definition may specialize; found a usage` (21), `type must be a definition, found attributeUsage` (2), `metaclass cannot specialize metaclass (kind mismatch)` (1), `rollsOn (typed by MyWheel) redefines rollsOn (typed by Wheel): types do not conform` (1) | 25 | **Ours.** KerML has no definition/usage split, so the first row misfires on ordinary declarations (`class Person specializes Object`, `Individuals Examples/JohnIndividualExample.kerml:4,12,34`; `Mass Roll-up Example/Vehicles_3.kerml:32`; `Simple Tests/Inheritance.kerml:21`). `metaclass <atom> AtomMetadata specializes Metaobject` (`KerML Spec Annex A Examples/A-2-Atoms.kerml:11`) is a metaclass specializing a metaclass, which the reference allows. The conformance row misses `classifier MyWheel unions MyWheel1, MyWheel2;` as a supertype of `Wheel` (`KerML Spec Annex A Examples/A-3-2-WithoutConnectors.kerml:32`). **Fixed in F32:** all 25 are gone, the root's only-ours count falling 439 → 417 with the SysML roots byte-identical. Three of the 22 the count moved by are replaced by a different diagnostic in `A-3-4-OneToUnrestrictedConnectors.kerml:43,48,56` — a redefinition conformance failure that only surfaces now that the type tier no longer errors in that file, and that is downstream of the unresolved `BikeFork` import in `A-3-3-OneToOneConnectors.kerml:33,35` (K3/F31), not of the definition/usage classification. |
| K5 | `x`/`y`/`z` `participates in a specialization cycle` (`unmapped`) | 3 | **Ours is right, and the reference has no such check** — the same one-sided finding F4 settled on the SysML side, now with a KerML witness the corpus's own authors committed: `feature x :> z; feature y :> x; feature z :> y;` in `Simple Tests/Circular.kerml:9-11` is a cycle, and `KerMLValidator.checkSpecialization` is exactly the validator F4 read. Our extension of the reference rather than a disagreement, so it stays `unmapped`. |

#### The 150, adjudicated diagnostic by diagnostic (K7–K18)

K1–K5 above are a history of a root that stood at 439. This section adjudicates the root as it
stands **now**: all **150** only-ours diagnostics, 140 `syntax` + 7 `unresolved-reference` + 3
`unmapped`, spread over **25** of the root's 58 files (33 carry none, which is the root's
`fullyAgreeing` count). Method is the SysML section's, tightened in one way: because the KerML
root is small enough to enumerate, **attribution is per diagnostic, not per file** — every one of
the 150 is read on the construct it sits on, so no class inherits diagnostics that belong to
another. `Simple Tests/Features.kerml` is the case that makes the difference: its 18 diagnostics
split across four classes, and grouping by its *first* one (line 8) would have credited all 18 to
`typed by`. 75 reproducers were run through both checkers (`bin/sysml <file>` and
`build/pilot-kerml-validator/validate-kerml <file>` on the same file), quoted inline below and
kept outside the repository, so no corpus root gains a file and the baseline does not move.

**118 of the 150 are recovery, not findings.** The generic messages our bodies unwind with after
the first unparsed member are `expected a namespace member` (95) and `expected a body member`
(23). The other 32 carry a construct-specific message: `expected '{' or ';' after declaration`
(17), `expected a name` (4), `expected '{' or ';'` (1), the 7 `unresolved-reference` and the 3
`unmapped`. So the 150 are produced by **12 constructs plus the 8 already settled**, and a single
accepted member can retire dozens: `Simple Tests/Expressions.kerml` alone is 34 diagnostics from
one gap.

**25 of the 150 are already adjudicated** and are not re-litigated here, only counted:
K5's specialization cycles (3, `Simple Tests/Circular.kerml:9-11`, one-sided and staying
`unmapped`), F50 (2, `Variable Feature Examples/TimeVaryingCarDriver.kerml:53,100`), F70 (3,
`Simple Tests/TextualRepresentation.kerml:7`), F71 (1,
`Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:27`), F72 (3,
`Association Examples/ProductSelection_N_ary.kerml:93,101,109`), F81 (8), F82 (1,
`Simple Tests/FeatureChains.kerml:28`) and F83 (4,
`KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:8,9`). Two of those counts are corrected
by the per-diagnostic pass, without any change to their verdicts: **F81's `differences` is 8, not
4** — beyond `Simple Tests/Classifiers.kerml:13` and `FeatureChains.kerml:31` it also leads
`Features.kerml:21` (`feature z1 intersects f,g differences y, y1, z;`) and `:28`
(`feature adult differences person, child;`), 2 diagnostics each — and **F83's is 4, not 2**,
because `A-2-ModelingInstances.kerml:8` carries the same `classifier MyBike [1] specializes
Bicycle;` shape as `:9`. The 125 that remain are the classes below.

| # | Class — the construct each diagnostic sits on | Files | Diags | Verdict | Follow-up |
|---|---|---:|---:|---|---|
| K7 | Keyword-less feature members: any at namespace level; and in a body, one whose declaration is a specialization without a typing, one whose multiplicity precedes the typing, or one prefixed `var`/`const` | 7 | 60 | **Ours** — `Feature`'s keyword-less alternative | [F84](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K8 | `type` declarations — the keyword itself, in every form the file writes it | 1 | 17 | **Ours** — `Type` | [F85](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K9 | Explicit relationship-member keywords: `specialization`/`subtype`/`subclassifier`/`typing`/`subset`/`redefinition`/`conjugation`/`inverse`/`inverting`/`featuring` | 5 | 19 | **Ours** — the `NonFeatureElement` relationship productions | [F86](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K10 | `typed by` as the long spelling of `:` in a feature declaration | 1 | 4 | **Ours** — `TypedBy` | [F87](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K11 | A connector end that is a feature chain (`connector f.a to a.g;`) | 2 | 6 | **Ours** — `ConnectorEnd`, `OwnedReferenceSubsetting` | [F88](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K12 | `binding`/`succession` declarations: anonymous with a body, and `binding n : T of a = b` | 1 | 5 | **Ours** — `BindingConnectorDeclaration`, `SuccessionDeclaration` | [F89](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K13 | Conjugation in a declaration: `conjugates` on a classifier, `~` in a feature declaration | 2 | 6 | **Ours** — `ConjugationPart`, `FeatureConjugationPart` | [F90](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K14 | `const` before `end` in an end-feature prefix | 1 | 2 | **Ours** — `EndFeaturePrefix` | [F91](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K15 | Annotating elements: an anonymous comment carrying only `locale`, and `doc` with a short name | 1 | 2 | **Ours** — `Comment`, `Documentation` | [F92](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K16 | A second filter bracket on a filter-package import (`::**[@A][cond]`) | 1 | 2 | **Ours** — `FilterPackage` | [F93](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K17 | Prefix metadata standing in for the `feature` keyword (`abstract #Classified z2;`) | 1 | 1 | **Ours** — `Feature`'s `PrefixMetadataMember` alternative | [F94](#follow-ups-not-fixed-here--this-pr-is-advisory) |
| K18 | A named `expr` whose body is a brace-enclosed expression with no `;` | 1 | 1 | **Ours** — `Expression` body | [F95](#follow-ups-not-fixed-here--this-pr-is-advisory) |

125 + the 25 settled = 150, each diagnostic in exactly one class. The verdict across the twelve
is **125 ours** — 12 parser gaps, no resolution defect among them, and **not one reference defect
or one-sided check**: every file here was written by the reference's own authors and the pinned
KerML validator is silent on all 25 (the root's `pilotOnly` 6 are K6's, adjudicated separately
below). Two reproducers draw a *different* pilot diagnostic and are marked as such (K9's
`redefinition` rows, K18's `;` variant); one candidate was **withdrawn** on the evidence rather
than promoted (K7's expression bodies, below).

#### K7 — keyword-less feature members (F84, 60)

The class the two biggest files are made of: `Simple Tests/Expressions.kerml` (33 of its 34),
`Vehicle Example/VehicleUsages.kerml` (14), `Simple Tests/Classifications.kerml` (6),
`Mass Roll-up Example/Vehicles_1.kerml` (2) and `Vehicles_2.kerml` (2),
`Simple Tests/Behaviors.kerml` (2), `Vehicle Example/VehicleDefinitions.kerml:16` (1). We do
accept a keyword-less member in a *body* when it is a plain name, a value, or a typing
(`p1;`, `p2 = 1;`, `p4 : Engine;`, `out p6;`, `composite p8;` all pass both checkers), which is
why the class is four faces of one gap rather than a blanket rejection. Face 1 —
**namespace level, at all** (`Classifications.kerml:3-7`, `Expressions.kerml:6-19`,
`VehicleDefinitions.kerml:16`):

```kerml
package KF1 {
	private import ScalarValues::*;
	classifier T;
	feature x : Integer;
	a : Integer;
	y = x as T;
}
```

Ours:

```text
kf1.kerml:5:2: error: expected a namespace member
	a : Integer;
 ^
kf1.kerml:6:2: error: expected a namespace member
	y = x as T;
 ^
sysml: kf1.kerml did not analyse cleanly
```

Pilot: `kf1.kerml:6:6: warning: Cast argument should have conforming types`, exit 0 — it accepts
both members. `x;` alone at namespace level draws the same single diagnostic from us and nothing
from the pilot. Faces 2, 3 and 4 — **in a body: a declaration whose specialization carries no
typing, a multiplicity before the typing, and a `var`/`const` prefix** (`VehicleUsages.kerml:48,
49,69,95,96`, `Vehicles_1.kerml:32`, `Vehicles_2.kerml:29`, `Behaviors.kerml:11,14`,
`Expressions.kerml:59`):

```kerml
package KF2 {
	private import ScalarValues::*;
	class V { feature m : Real; }
	feature v : V {
		composite e1 redefines V::m;
		p5[1] : Real;
		var p9 : Real;
	}
}
```

Ours:

```text
kf2.kerml:5:13: error: expected a body member
		composite e1 redefines V::m;
            ^~
kf2.kerml:6:3: error: expected a body member
		p5[1] : Real;
  ^~
kf2.kerml:7:3: error: expected a body member
		var p9 : Real;
  ^~~
sysml: kf2.kerml did not analyse cleanly
```

Pilot: clean, exit 0. `Feature` (`KerML.xtext:538`) has an alternative with no `feature` keyword
at all — `( EndFeaturePrefix | BasicFeaturePrefix ) FeatureDeclaration` (`:542-543`) — and
`BasicFeaturePrefix` (`:515`) is where `var`/`const` live, so `var p9 : Real;` is that
alternative and not an error. `FeatureDeclaration` (`:548`) is
`Identification ( FeatureSpecializationPart | FeatureConjugationPart )?`, and
`FeatureSpecializationPart` (`:574`) is `( -> FeatureSpecialization )+ MultiplicityPart?
FeatureSpecialization* | MultiplicityPart FeatureSpecialization*` — so a `redefines` with no
typing and a multiplicity written before the typing are both ordinary declarations. At namespace
level the member is a `NamespaceFeatureMember` (`:158`), which takes a `FeatureElement` with no
keyword requirement. **Ours, one parser gap with four faces**, and 55 of the 60 are the two
recovery messages.

**Withdrawn, not promoted:** the corpus's expression-body lines (`Expressions.kerml:15-19`,
`c = x->collect {in xx; xx + 1};` and siblings, 15 diagnostics) look like a second gap and are
not one. With the corpus's own imports both checkers accept them:

```kerml
package K8i {
	private import ScalarFunctions::*;
	private import ControlFunctions::*;
	feature x : ScalarValues::Integer;
	feature c = x->collect {in xx; xx + 1};
	feature d = x->select {in xx; xx != null};
	feature e = x->reduce {in s; in t; s + t}->reduce '+';
}
```

Ours: `✓ package K8i`. Pilot: clean. Those 15 are K7's face 1 with a cascade inside the braces,
so they are counted in K7 and no separate follow-up is opened. The same test retires
`Expressions.kerml:41`'s multi-line `if`/`else`: written after `feature`, it passes both.

#### K8 — `type` declarations (F85, 17)

Every diagnostic in `Simple Tests/Types.kerml` except its four relationship-keyword lines
(K9's `:17,18,25,26`) sits on the `type` keyword: `:2,3,6,8,10,15,20,22,23,24,28,29,31,33,34,35,36`.
Reproducer:

```kerml
package K9a {
	abstract type A specializes Base::Anything;
	type all x specializes A, Base::things;
	type Singleton[1] specializes Base::Anything;
	type B :> Base::Anything;
	type Conjugate3 conjugates A;
}
```

Ours:

```text
k9a.kerml:2:11: error: expected a namespace member
	abstract type A specializes Base::Anything;
          ^~~~
k9a.kerml:3:2: error: expected a namespace member
	type all x specializes A, Base::things;
 ^~~~
k9a.kerml:4:2: error: expected a namespace member
	type Singleton[1] specializes Base::Anything;
 ^~~~
k9a.kerml:5:2: error: expected a namespace member
	type B :> Base::Anything;
 ^~~~
k9a.kerml:6:2: error: expected a namespace member
	type Conjugate3 conjugates A;
 ^~~~
sysml: k9a.kerml did not analyse cleanly
```

Pilot: clean, exit 0. `Type` is `TypePrefix 'type' TypeDeclaration TypeBody`
(`KerML.xtext:319`), and `TypeDeclaration` (`:324`) carries `all`, an optional
`OwnedMultiplicity`, a `SpecializationPart | ConjugationPart` and `TypeRelationshipPart*` — so
the multiplicity of `:6`, the conjugation of `:28,29` and the `unions`/`intersects`/`differences`
of `:33,34,35` are all inside this one production, and the file's `}` cascades (`:22,36`) go with
it. `type A;` with no specialization is *rejected by the pilot too*
(`no viable alternative at input ';'`), which is why the reproducer specialises: the
`SpecializationPart | ConjugationPart` is not optional in `TypeDeclaration`, unlike
`ClassifierDeclaration` (`:468`). **Ours, one unparsed keyword.**

#### K9 — explicit relationship-member keywords (F86, 19)

KerML lets a relationship be written as a member in its own right, with the relationship's
keyword first. We parse none of the ten spellings the corpus uses:
`Simple Tests/Classifiers.kerml:5,6`, `Features.kerml:16,42,43,45,46,68,69,71`,
`FeatureChains.kerml:23,24,26`, `Inverses.kerml:11,12`, `Types.kerml:17,18,25,26`.
Reproducers:

```kerml
package K13d {
	classifier A; classifier B;
	feature f; feature g; feature person; feature parent;
	specialization t1 typing f typed by B;
	specialization t2 typing g : A;
	specialization Sub subset parent subsets person;
	specialization subset parent subsets person;
	subset g subsets f;
	subtype A specializes B;
}
```

Ours — one `expected a namespace member` per line, pointing at the leading keyword:

```text
k13d.kerml:4:2: error: expected a namespace member
	specialization t1 typing f typed by B;
 ^~~~~~~~~~~~~~
k13d.kerml:10:2: error: expected a namespace member
	subset g subsets f;
 ^~~~~~
k13d.kerml:11:2: error: expected a namespace member
	subtype A specializes B;
 ^~~~~~~
```

Pilot: clean on all of those, exit 0. Same shape for `subclassifier`
(`specialization Super subclassifier A specializes B;`), `inverse`/`inverting`
(`inverse B::g of A::f;`, `inverting Invert inverse B::g of A::f;`), `featuring`
(`featuring F of y by C;`) and `conjugation`
(`conjugation c1 conjugate Conjugate1 conjugates Original;`) — each is one
`expected a namespace member` from us and silence from the pilot. The productions are
`Specialization` (`KerML.xtext:390`, `( 'specialization' Identification? )? 'subtype' …` — so
both `specialization … subtype` and a bare `subtype` are legal), `Subclassification` (`:486`),
`FeatureTyping` (`:665`), `Subsetting` (`:683`), `Redefinition` (`:712`), `Conjugation` (`:408`),
`FeatureInverting` (`:634`) and `TypeFeaturing` (`:652`); all are `NonFeatureElement`
alternatives, so any of them may open a member. **Ours, one parser gap across ten keywords.**

**Reproducer caveat:** the `redefinition` rows do not discriminate in minimal form. With
package-level features the pilot reports `A package-level feature cannot be redefined`, and with
qualified targets it reports `Featuring types of redefining feature and redefined feature cannot
be the same` / `Must be an accessible feature (use dot notation for nesting)` — semantic
complaints, at a column inside the line, which prove it *parsed* the member but do not give a
silent-pilot pair. The corpus lines (`Features.kerml:68,69,71`, whose targets are members of two
different nested classes) are the evidence for those three; a fix must be validated against a
fixture built from that file's declarations, not against the reproducer above.

#### K10 — `typed by` (F87, 4)

`Simple Tests/Features.kerml:8` is `feature x typed by A, B references f subsets g;` and `:11` is
`feature x1 subsets g typed by A subsets f typed by B;`, 2 diagnostics each. Reproducer:

```kerml
package M1 {
	classifier A; classifier B;
	feature f; feature g; feature y; feature z;
	feature x1 typed by A;
}
```

Ours:

```text
m1.kerml:4:13: error: expected '{' or ';' after declaration
	feature x1 typed by A;
            ^~~~~
m1.kerml:4:13: error: expected a namespace member
	feature x1 typed by A;
            ^~~~~
sysml: m1.kerml did not analyse cleanly
```

Pilot: clean, exit 0. `TypedBy` (`KerML.xtext:600`) is
`( ':' | 'typed' 'by' ) ownedRelationship += OwnedFeatureTyping …` — the two spellings are the
same production, and we implement only the punctuation. Everything else on those two corpus lines
parses on its own: `feature x2 : A, B;`, `feature x3 references f;` and
`feature x8 :> g ::> f;` are all clean on both sides, and `feature x4 subsets g typed by A;`
fails at the `typed`, not at the second specialization. **Ours, one missing keyword spelling.**

#### K11 — feature-chain connector ends (F88, 6)

`Named Collection Members Example/VehicleTanks.kerml:28,31` (`connector tanks.main1 to
tanks.aux1;`) and `Simple Tests/FeatureChains.kerml:18` (`connector f.a to a.g;`), 2 diagnostics
each. Reproducer:

```kerml
package K12a {
	classifier A { feature g; }
	classifier F { feature a : A; }
	feature b : F {
		feature f : F;
		feature a : A;
		connector f.a to a.g;
	}
}
```

Ours:

```text
k12a.kerml:7:14: error: expected '{' or ';' after declaration
		connector f.a to a.g;
             ^
k12a.kerml:7:14: error: expected a body member
		connector f.a to a.g;
             ^
sysml: k12a.kerml did not analyse cleanly
```

Pilot: one unrelated `Duplicate of inherited member name` warning, exit 0 — it accepts the
connector. It is the *dotted* end and nothing else: `connector eng to x.g;` with a plain first
end is clean on both sides, so the failure is our first-end parse, not feature chains in general
and not the `to`. `ConnectorEnd` (`KerML.xtext:854`) ends in
`ownedRelationship += OwnedReferenceSubsetting`, and `OwnedReferenceSubsetting` (`:699`) is
`referencedFeature = [SysML::Feature | QualifiedName] | ownedRelatedElement +=
OwnedFeatureChain` — a feature chain is one of its two alternatives, at either end.
**Ours, one parser gap.**

#### K12 — `binding` and `succession` declarations (F89, 5)

`Simple Tests/Connectors.kerml:16` (`binding {`, 1), `:20` (`binding ab1 : AS of a = b;`, 3) and
`:24` (`succession {`, 1). Reproducers:

```kerml
package K14a {
	assoc struct AS { end a; end b; }
	class A {
		feature a : A;
		feature b : A;
		binding {
			end feature references a;
			end feature references b;
		}
	}
}
```

Ours: `k14a.kerml:6:11: error: expected a name` on the `{` — we require a name after `binding`.
The `succession` form is identical (`k14c.kerml:5:14: error: expected a name`). The typed form:

```text
k14b.kerml:6:15: error: expected '{' or ';' after declaration
		binding ab1 : AS of a = b;
              ^
k14b.kerml:6:20: error: expected '{' or ';' after declaration
		binding ab1 : AS of a = b;
                   ^~
k14b.kerml:6:20: error: expected a body member
		binding ab1 : AS of a = b;
                   ^~
sysml: k14b.kerml did not analyse cleanly
```

Pilot: clean on all three, exit 0. `BindingConnectorDeclaration` (`KerML.xtext:875`) is
`FeatureDeclaration ( 'of' … '=' … )? | ( isSufficient ?= 'all' )? ( 'of'? … '=' … )?` — so the
declaration, the `of`/`=` ends and the name are each optional, and a `binding` may open straight
into a body whose ends are ordinary members. `SuccessionDeclaration` (`:891`) is the same shape.
**Ours, two faces of one gap:** an anonymous binding/succession, and a typing before `of`.

#### K13 — conjugation in a declaration (F90, 6)

`Simple Tests/Conjugation.kerml:6` (`class B conjugates A;`) and `:8` (`feature g ~ B::f;`),
2 each, plus `Features.kerml:36` (`feature fuelOutPort ~ fuelInPort;`, 2). Reproducer:

```kerml
package K15a {
	class A { in feature f; }
	class B conjugates A;
	feature g ~ B::f;
}
```

Ours:

```text
k15a.kerml:3:10: error: expected '{' or ';' after declaration
	class B conjugates A;
         ^~~~~~~~~~
k15a.kerml:3:10: error: expected a namespace member
	class B conjugates A;
         ^~~~~~~~~~
k15a.kerml:4:12: error: expected '{' or ';' after declaration
	feature g ~ B::f;
           ^
k15a.kerml:4:12: error: expected a namespace member
	feature g ~ B::f;
           ^
sysml: k15a.kerml did not analyse cleanly
```

Pilot: clean, exit 0. `ClassifierDeclaration` (`KerML.xtext:468`) offers
`SuperclassingPart | ClassifierConjugationPart`, and `FeatureConjugationPart` (`:730`) is
`( '~' | 'conjugates' ) ownedRelationship += FeatureConjugation` — one production, two
spellings, and we implement neither in a declaration. **Ours, one parser gap.** (`Types.kerml:28,
29` write the same construct on a `type`; they are counted in K8, since the `type` keyword fails
first.)

#### K14 — `const` before `end` (F91, 2)

`Simple Tests/Associations.kerml:16,17`. Reproducer:

```kerml
package K16 {
	assoc struct C {
		const end [1] feature a;
		const end feature b;
	}
}
```

Ours:

```text
k16.kerml:3:3: error: expected a body member
		const end [1] feature a;
  ^~~~~
k16.kerml:4:3: error: expected a body member
		const end feature b;
  ^~~~~
sysml: k16.kerml did not analyse cleanly
```

Pilot: clean, exit 0. `end feature b;` without the `const` is accepted by us, so it is that one
token in that one position: `EndFeaturePrefix` (`KerML.xtext:511`) is
`( isConstant ?= 'const' )? isEnd ?= 'end'`. **Ours, one prefix ordering.**

#### K15 — annotating elements (F92, 2)

`Simple Tests/Comments.kerml:25` is an anonymous comment carrying only a locale and
`:43` is a documentation comment with a short name. Reproducers and our output:

```kerml
package K18a {
 locale "en_US" /*
 * AAAA
 */
}
```

```text
k18a.kerml:2:2: error: expected a namespace member
 locale "en_US" /*
 ^~~~~~
```

```kerml
package K18b {
	class A {
		doc <a> /* Documentation comment on A*/
	}
}
```

```text
k18b.kerml:3:7: error: expected a /* ... */ comment body
		doc <a> /* Documentation comment on A*/
      ^
k18b.kerml:3:7: error: expected a body member
		doc <a> /* Documentation comment on A*/
      ^
```

Pilot: clean on both, exit 0. `Comment` (`KerML.xtext:94`) makes its whole
`'comment' Identification? ('about' …)?` head optional and then allows
`( 'locale' locale = STRING_VALUE )? body = REGULAR_COMMENT`, so `locale "…"` plus a comment body
is an anonymous comment; `Documentation` (`:103`) is `'doc' Identification? ( 'locale' … )? body`,
so `doc <a>` is a documentation comment with a short name. We accept
`doc locale "en_US"/* … */`, which is why this is two narrow gaps rather than one: the optional
`comment` keyword, and `Identification` after `doc`. **Ours.**

#### K16 — a second filter bracket on an import (F93, 2)

`Simple Tests/Filtering.kerml:35` — the second `[ … ]` of
`private import DesignModel::**[@Structure][(as …).approved and (as …).level > 1];`.
Reproducer:

```kerml
package K19c {
	private import KerML::*;
	package DesignModel {
		struct System;
	}
	package One {
		private import DesignModel::**[@Structure];
		struct Test1 :> System;
	}
	package Two {
		private import DesignModel::**[@Structure][@Structure];
		struct Test2 :> System;
	}
}
```

Ours — the single-bracket import in `One` is accepted, the double-bracket import in `Two` is not:

```text
k19c.kerml:11:45: error: expected '{' or ';'
		private import DesignModel::**[@Structure][@Structure];
                                            ^
k19c.kerml:11:45: error: expected a namespace member
		private import DesignModel::**[@Structure][@Structure];
                                            ^
sysml: k19c.kerml did not analyse cleanly
```

Pilot: clean, exit 0. `FilterPackage` (`KerML.xtext:200`) is
`ownedRelationship += FilterPackageImport ( ownedRelationship += FilterPackageMember )+` — one or
more filter members, and we accept exactly one. A `filter` statement as a package member is
unaffected (both checkers agree on it). **Ours, a cardinality of one where the grammar says
one-or-more.**

#### K17 — prefix metadata in place of the `feature` keyword (F94, 1)

`Simple Tests/MetadataTest.kerml:33`, `abstract #Classified z2;`. Reproducer:

```kerml
package K20b {
	metaclass Classified;
	private #Classified feature z1;
	abstract #Classified z2;
}
```

Ours:

```text
k20b.kerml:4:11: error: expected a namespace member
	abstract #Classified z2;
          ^
sysml: k20b.kerml did not analyse cleanly
```

Pilot: clean, exit 0 — and it accepts line 3 as well, which we also accept. `Feature`
(`KerML.xtext:538`) is `FeaturePrefix ( 'feature' | ownedRelationship += PrefixMetadataMember )
FeatureDeclaration?`: the metadata annotation is an *alternative to* the keyword, not an addition
to it. This is the KerML twin of S1/F60 on the SysML side (`ExtendedUsage`). **Ours.**

#### K18 — a named `expr` with a brace body (F95, 1)

`Simple Tests/Expressions.kerml:23`, `in expr whileTest {v > 3}` inside a
`ControlPerformances::LoopPerformance` step. Reproducer:

```kerml
package K21 {
	private import ScalarValues::*;
	feature v : Integer;
	expr e1 {v > 3}
	expr e2 {1 + 1};
}
```

Ours:

```text
k21.kerml:4:11: error: expected a body member
	expr e1 {v > 3}
          ^
k21.kerml:5:11: error: expected a body member
	expr e2 {1 + 1};
          ^
k21.kerml:5:17: error: expected a namespace member
	expr e2 {1 + 1};
                ^
```

Pilot: `k21.kerml:5:17: error: extraneous input ';' expecting '}'`, exit 1 — it accepts line 4 and
rejects only the `;` we also mis-handle on line 5, so **line 4 is the discriminating pair** and
the trailing `;` is a defect in the reproducer, not in either implementation. `expr at { … }` and
`expr while { … }` were unreserved by F30; the remaining gap is a *named* `expr` whose body is an
expression rather than a member list. **Ours.**

#### What this class list predicts

If K7–K18 are fixed the root's 140 syntax diagnostics go with them, together with F50, F81, F82,
F83 and F70 — F70's 3 are `unresolved-reference` only because the `rep` member is unparsed. What
remains is F71 (1), F72 (3) and K5's 3 `unmapped` cycles, which are one-sided by design: the root
would stand at **7** only-ours from 150, with `syntax` empty. K7 and K8 alone are 77 of the 125,
and K7's `Expressions.kerml`/`VehicleUsages.kerml` are 47 — so the sequencing is parser-first and
largest-file-first, and nothing downstream of an unparsed first member is measurable until it
parses.

### KerML — only the pilot (6)

| # | Class | Count | Verdict |
|---|---|---:|---|
| K6 | `The opposite features 'owningType' of '…DisjoiningImpl{…}' and 'ownedDisjoining' of '…{…}' do not refer to each other` | 6 | **A defect in the reference implementation**, and it stays `unmapped`. All six are one cause, established rather than assumed — see [K6, diagnostic by diagnostic (F33)](#k6-diagnostic-by-diagnostic-f33). None is a model defect and none is ours: the pilot's own derived `Type::ownedDisjoining` does not contain the `Disjoining` whose `owningType` is that `Type`, so its Ecore `eOpposite` pair is internally inconsistent for every `disjoint from` written in a type declaration. |

#### K6, diagnostic by diagnostic (F33)

The six, exactly as the pinned reference reports them
(`build/pilot-kerml-validator/validate-kerml`, pilot `2026-05` / `jupyter-sysml-kernel`
0.60.1). Severity is `error` and the category `unmapped` on all six:

| # | File:line | Construct it is attached to | Owner in the message |
|---|---|---|---|
| 1 | `KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9` | `classifier YourBike [1] specializes Bicycle disjoint from MyBike;` | `ClassifierImpl` |
| 2 | `Simple Tests/Classifiers.kerml:13` | `classifier D disjoint from C differences A, B;` | `ClassifierImpl` |
| 3 | `Simple Tests/FeatureChains.kerml:31` | `feature h2 differences b.f, b.a intersects f.a, g disjoint from h1;` | `FeatureImpl` |
| 4 | `Simple Tests/Features.kerml:20` | `feature z unions f, g disjoint from y;` | `FeatureImpl` |
| 5 | `Simple Tests/Inverses.kerml:3` | `feature f : B inverse of B::g disjoint from h;` | `FeatureImpl` |
| 6 | `Simple Tests/Types.kerml:31` | `type C :> B disjoint from A;` | `TypeImpl` |

The messages differ only in that owner class and in the resource fragments. Verbatim, for #6:

```
The opposite features 'owningType' of 'org.omg.sysml.lang.sysml.impl.DisjoiningImpl{Simple Tests/Types.kerml#//@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.14/@ownedRelatedElement.0/@ownedRelationship.1}' and 'ownedDisjoining' of 'org.omg.sysml.lang.sysml.impl.TypeImpl{Simple Tests/Types.kerml#//@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.14/@ownedRelatedElement.0}' do not refer to each other
```

**It is not about the model.** The message is not the reference's own wording: it is EMF's
generic structural check, whose template ships in the pilot jar as
`_UI_UnpairedBidirectionalReference_diagnostic = The opposite features ''{0}'' of ''{1}'' and
''{2}'' of ''{3}'' do not refer to each other` and is raised by `EObjectValidator` over an
`EReference` pair, not by `KerMLValidator`. Its arguments are generated implementation objects
(`…impl.DisjoiningImpl`) addressed by resource fragment, and `owningType`/`ownedDisjoining` are
Ecore features, not KerML notation. An `eOpposite` mismatch is a property of the loaded
resource, so the diagnostic is a statement about the reference's own object graph.

**It is exhaustive and one-to-one with the notation.** The corpus contains exactly six
`disjoint from` clauses in a type declaration — the six above — and exactly six diagnostics.
The corpus's one *standalone* disjoining, `disjoint b.f.a from b.a;`
(`Simple Tests/FeatureChains.kerml:28`), draws none, which already localises the condition to
the declaration form.

**Minimal reproducer.** Three lines are enough; nothing else in the file, no import, no
library reference:

```kerml
package Decl {
    classifier A;
    classifier B disjoint from A;
}
```

The pilot reports the diagnostic on line 3, on `A` (the column follows the indentation, so
assert on the line and the message); OpenSysML reports nothing. Deleting the
`disjoint from A` clause, or writing it as the standalone `disjoint B from A;` in the same
package, silences the pilot — so the clause, and only the clause, provokes it.

**It is not a batching artifact.** Each of the six reproduces when its own file is validated
alone in a fresh resource set, at the same line, with the same message. (`A-2-ModelingInstances`
alone also emits unresolved-reference noise for the corpus siblings it no longer sees; the K6
diagnostic is unaffected.) So it is not an artifact of loading the root as one batch.

**It is not a bridge artifact of ours.** The bridge is out of the loop in the reproducer above:
it is a single file, validated on its own, and the diagnostic carries its own file and line from
the reference. Positional misattribution — the failure mode #343's first attempt had — cannot
produce a diagnostic that only ever lands on a `disjoint from` line, in six different files, at
six different lines, and never on the other 52 files of the root.

**The mechanism, from the reference's own objects.** `Disjoining::owningType` declares
`eOpposite="#//Type/ownedDisjoining"` in the pilot's `model/SysML.ecore`, and
`Type::ownedDisjoining` is derived, transient and volatile: its generated setting delegate
filters `ownedRelationship` for `Disjoining`s whose `typeDisjoined` is this `Type`. Probing the
loaded model of the reproducer through the pilot's own API (`SysMLUtil` + the KerML standalone
setup, the same entry points the bridge uses) gives:

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

`B.ownedRelationship` holds the `Disjoining`, its `typeDisjoined` *is* `B`, and `owningType` is
`B` — yet the derived `B.ownedDisjoining` the `eOpposite` points back through is empty. The two
ends of the pair contradict each other in the reference's own graph, which is precisely what
EMF then reports.

**The input is what the grammar prescribes.** `disjoint from` inside a type declaration is
`fragment DisjoiningPart returns SysML::Type : 'disjoint' 'from' ownedRelationship +=
OwnedDisjoining ( ',' ownedRelationship += OwnedDisjoining )*`
(`KerML.xtext:344`, reached from `TypeRelationshipPart` at `:340` in every type declaration),
and `OwnedDisjoining` (`:437`) sets only `disjoiningType` — the other end, `typeDisjoined`, is
the owning type, which is why the standalone `Disjoining` production (`:426`) has to name it
explicitly and the owned form does not. The reproducer is that production, written the way the
reference's own examples write it, and the reference parses it without complaint before its EMF
check fires on the objects it built. The derivation the empty list violates is the reference's
own: `Type::ownedDisjoining` is documented in the shipped metamodel as the `ownedRelationship`s
of this `Type` that are `Disjoining`s for which the `Type` is the `typeDisjoined` `Type`.

So the verdict is one cause for all six, and it is the reference's: not a model defect, not a
gap of ours, not the bridge. It stays `unmapped` — the five coarse categories describe the
model (a name that did not resolve, a metaclass used where it is not allowed, bounds, units,
notation that did not parse), and this describes the reference's metamodel state. Mapping it to
one of them would let it *agree* with one of our diagnostics some day, which would be a false
agreement. Reported upstream is the remaining action: **F80**.

Three only-ours gaps were isolated while reducing these six, all of them inside the root's
already-adjudicated 140 syntax diagnostics rather than new counts. Each is a construct the
pinned reference accepts in silence and we reject, reduced to its own probe: `differences A, B`
in a type declaration (**F81**, `Simple Tests/Classifiers.kerml:13`,
`FeatureChains.kerml:31` — `intersects` and `unions` at the same position parse for us, so it is
that one keyword), a standalone `disjoint B from A;` as a namespace member (**F82**, the
`NonFeatureElement` alternative at `KerML.xtext:257`, `FeatureChains.kerml:28`), and a
multiplicity in a classifier declaration, `classifier B [1] specializes A;` (**F83**,
`OwnedMultiplicity` at `KerML.xtext:470`, `A-2-ModelingInstances.kerml:9`). They are recorded
here because the K6 lines are where they surface, and left to their follow-ups: F33's remit is
the pilot-only class.

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
artifact of validating that root file by file, and batch loading resolves them (F6, #397, which
also retired the last three order-dependent diagnostics elsewhere). The remaining 139 SysML-side
pilot-only diagnostics are the same `testdata`/`examples` issues as before, at the lower counts
the merged import-visibility, keyword and non-standard-notation work left behind.

Grouped by root cause. The pilot's own grammar
(`org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext`) is quoted where it settles the
question.

| # | Class | Count (approx.) | Verdict |
|---|---|---|---|
| P1 | `mismatched input 'import' expecting '}'` / `missing EOF at 'import'` on a bare `import X::*;` | 10 of our 21 `testdata`/`examples` files | **Pilot is stricter, and its grammar is explicit**: `fragment ImportPrefix returns SysML::Import : visibility = VisibilityIndicator 'import' ...` — visibility is *mandatory* for an import, unlike `MemberPrefix`, where it is optional. `private import X::*;` parses cleanly. Whether the specification's concrete syntax makes visibility mandatory too is not settled here, so this is not booked as our bug: follow-up F2. **This is also the single largest cascade source** — once the import fails, the pilot abandons the enclosing body, which produces most of the `no viable alternative`, `extraneous input '}' expecting EOF`, `missing EOF`, `Couldn't resolve reference to Type 'Real'` and `A usage must be typed by definitions.` entries downstream. |
| P2 | `no viable alternative at input '<name>'` on `namespace N;` inside a package body | 4 files | **Ours is wrong (over-acceptance).** `namespace` is a KerML keyword; the pilot's `DefinitionElement` list has no namespace declaration, so `.sysml` notation has none. We parse it. Follow-up F3. |
| P3 | `no viable alternative at input 'region'` (`orthogonal-regions-demo.sysml`) | 1 file | **Ours is wrong (over-acceptance).** SysML v2 spells orthogonal regions as a `parallel` state body (`';' \| ( isParallel ?= 'parallel' )? '{' StateBodyPart '}'`); there is no `region` keyword. We accept one. Follow-up F3. |
| P4 | `Duplicate of other owned member name` (warning) | 25 (re-measured: 15) | **Re-derived from clean inputs (F110), and the earlier verdict was too broad.** The rule itself we implement and agree on: on inputs both implementations parse identically the warning matches line, column and multiplicity for repeated part, attribute, action, `enum` and calculation-parameter names (`calc c { in a : Real; return a; }` draws it twice from each side; `testdata/passes/corpus_notation.sysml:33-34` is the corpus instance, an agreement row). What the class actually held was two things: **measurement artifacts** — all 15 remaining pilot-side diagnostics sit in files whose pilot parse failed on the same or an earlier line, so they say nothing about the rule (see W12) — and **one real under-report of ours**, found only by clean reproducers: a simple state member of a state body (`state red;`) and a named transition (`transition t first a then b;`) contribute their names to their container's namespace, and our distinguishability check skipped both because those declarations carried no name span. Fixed at the root (both now record one), so their duplicate warnings match the reference's exactly. The rule is booked as implemented and agreeing; nothing here is our silence any more. |
| P5 | `Bound features should have conforming types`, `Must have a Boolean result`, `Must have at least two related elements`, `An attribute must be typed by attribute definitions.` | 23 | **Mostly downstream of P1/P2/P3**: with the imports or the enclosing body broken, the pilot type-checks a partially-recovered model. Not adjudicated individually; the honest reading is that these become meaningful only once P1–P3 are resolved and the files re-run. |
| P6 | `Must be an accessible feature (use dot notation for nesting)`, `Cannot identify flow end (use dot notation)`, `Must be model-level evaluable`, `Must invoke a behavior or a behavioral feature` | 9 | **Adjudicated per diagnostic below** (F5, done). 5 are downstream of P2, 2 are a real gap in our constraint tier, 2 are downstream of unresolved references both implementations report. The four *rules* behind them are all real, and three of them we do not implement: follow-ups F20–F23. |
| P7 | K6, the KerML `eOpposite` complaint | 6 | **A defect in the reference implementation**, `unmapped`, and the only pilot-only class on the KerML root. Adjudicated diagnostic by diagnostic under [K6 (F33)](#k6-diagnostic-by-diagnostic-f33): one cause, the reference's own `Disjoining`/`Type` `eOpposite` pair, with a three-line reproducer. Upstream report is F80. |

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

Rows 1–5 no longer have corpus instances: `semantic-layer/demo.sysml` now declares those three
packages with `package`, the spelling the reference parses, so the recovery that produced them is
gone and the file is fully agreeing. The verdicts stand as the adjudication of why they were there.

The rule behind rows 1–5 is real too, even though no corpus diagnostic is: with
`part def P { attribute n : Integer = 1; } package Q { filter E::P::n > 0; }` — which parses
cleanly for the pilot — it reports `Must be an accessible feature (use dot notation for nesting)`
(and `Must be model-level evaluable`). Our filter traversal now reports the same accessibility
diagnostic for metaclass-owned user features, while the type tier still masks it for
chain-shaped conditions whose referent is not model-level evaluable.

What each rule requires, at the pin
(`org.omg.kerml.xtext/src/org/omg/kerml/xtext/validation/KerMLValidator.xtend`,
`org.omg.sysml.logic/src/main/java/org/omg/sysml/…`):

| Rule | What it requires | Where it would live | False-positive risk if we get it slightly wrong |
|---|---|---|---|
| `validateSubsettingFeaturingTypes` — `Must be an accessible feature (use dot notation for nesting)` | Normative text on `Subsetting`: `subsettingFeature.canAccess(subsettedFeature)`. The pilot's `FeatureUtil.canAccess` holds when the subsetting feature has no `featuringType` and the subsetted feature is featured within nothing, or when some featuring type of the subsetting feature features the subsetted one — recursing through featuring types that are themselves features. A feature *of a type* is therefore not reachable by `::` from outside it; dot notation is what introduces the featuring chain that makes it reachable. | `passes/w8c_feature_reference.go` `FeatureReferencePass` at `LevelConstraint`, which traverses namespace and import filter conditions with the candidate as featuring context and accepts targets declared by library content (`resolver.Index().Library`). | Focused pass coverage includes user metaclass chains, library metaclass paths, cast dot notation, and metadata classification. Chain-shaped filters can still be masked by the type-tier model-level-evaluable error before this constraint tier runs. |
| `validateFlowEndSubsetting` — `Cannot identify flow end (use dot notation)` | `FeatureUtil.getSubsettedNotRedefinedFeaturesOf(flowEnd)` must be non-empty: each end of a flow has to name the *feature* the payload leaves from or arrives at, so it can redefine `Transfer::source::sourceOutput` / `Transfer::target::targetInput` (`FlowEnd` model doc). Naming the part alone leaves the end with nothing to subset. The pilot also warns `Flow ends should use dot notation` for the implicit-subsetting case. | `LevelConstraint`, beside `checkConnectorEndRedefinition` / `checkInterfaceEndConjugation` in `passes/constraint.go`, over `lower`/`semantics` connector ends. | A flow whose ends are already features (`from a.out to b.in`), ends typed through a library `Transfer` specialization, and succession flows; also `examples/views-demo.sysml:44` is our own model and would have to be fixed rather than exempted. |
| `validateElementFilterMembershipIsModelLevelEvaluable` — `Must be model-level evaluable` | `condition.isModelLevelEvaluable` (plus `condition.result.specializesFromLibrary('ScalarValues::Boolean')`). Evaluability is **not** "is a constant": an invocation is evaluable when its function is a model-level-evaluable library function *and* every argument is; a feature reference is evaluable when its referent is a self-reference, or owned by a `Metaclass`/`MetadataFeature`, or has **no featuring type** and its value expression (if any) is evaluable — and inevaluable when the referent is featured within a type (an instance-level feature) or the reference is circular. So `filter p.n > 1` over a top-level `part p : P` is *accepted* by the pilot (no featuring type), while `filter P::n > 0` is not, and `filter Twice(2) > 3` over a user `calc` is not (a user function is not model-level evaluable). | `passes/filter.go` `ElementFilterPass` (`filter-not-boolean`, `filter-not-evaluable`, and the non-blocking `filter-not-evaluated` warning, `LevelType`) over `semantics/filter.go` `Model.CheckElementFilter`; compiled predicates retain semantic result type and distinguish specification faults from evaluator limitations. | Focused semantics and pass tests cover metaclass-owned Boolean and non-Boolean chains, comparisons, user-struct chains, library chains, and package-level feature chains. Real model-level-evaluability faults remain errors; evaluator-only limitations warn and keep all candidates, while the type tier can still suppress constraint diagnostics for chain-shaped conditions. |
| `validateInvocationExpressionInstantiatedType` — `Must invoke a behavior or a behavioral feature` | `instantiatedType.oclIsKindOf(Behavior) or (instantiatedType.oclIsKindOf(Feature) and instantiatedType.type->exists(oclIsKindOf(Behavior)) and instantiatedType.type->size(1))` — what is invoked must be a behavior (`calc def`, `action def`, `function`), or a feature typed by exactly one behavior. | `LevelConstraint`, or the invocation checking already in `passes/typecheck_expr.go` (`inferInvocation`/`effectiveInParameters`), which today infers argument types and arity but never asks what kind of thing is being invoked. | An invocation of a library function reached through an alias or an index record with no parsed declaration, a feature typed by a behavior *through* a specialization chain, constructor-like invocations of a definition (which the notation does allow in other positions), and metadata-annotation invocations. Reporting only when the invoked symbol resolves to a declaration we can classify is the safe shape. |

### Only the pilot — the wave-9B row-by-row sweep (137)

P1–P7 above group this column by cause and adjudicate P6 diagnostic by diagnostic. This pass sweeps
the whole column and gives **every** row exactly one of three outcomes: **our defect** — a rule the
pinned reference enforces and we do not, named with the pilot rule that raises it and the package
that would own ours; **an adjudicated divergence** — a difference we have decided to keep, with the
reason; or **a defect of the pilot**, written up in [omg-issues.md](omg-issues.md). 137 rows in 19
files across three roots, classified as:

| Outcome | Rows |
|---|---|
| our defect | 19 |
| adjudicated divergence | 112 |
| defect of the pilot (F80) | 6 |

Three mechanisms account for 100 of the 112 divergences and none of them is a rule difference. The
first is **recovery surplus**: when the pilot's parse fails on notation of ours it keeps emitting per
token and then abandons the file, so one construct of ours becomes five, seven or eleven rows — the
primary row is classified on its own merits and the surplus is not a second finding. The second is
**secondary diagnostics over an unresolved reference**: the pilot type-checks and kind-checks a graph
in which a name did not resolve, where our tiers gate the higher checks behind the lower ones
(`AGENTS.md` §4). The third is notation this page has already adjudicated — the bare `import` of F2
and the state-machine words of F3 — where we warn and the pilot cannot parse at all.

The `Where` column names one row of the family; the reproducer under each family is the minimal file
that isolates it, run against the pinned single-file CLI
(`build/pilot-validator/validate-sysml`, pilot `2026-05` / `0.60.1`) and, for the KerML rows,
`build/pilot-kerml-validator/validate-kerml`. "Ours" is `bin/sysml -validate <file>`.

The census is the column as it stood when it was swept, and its line references go with it: the two
models behind W1, W2 and most of W10–W13 were rewritten to the spec spelling immediately afterwards,
which retired those rows rather than reclassifying them. The paragraph under W2 measures that.

| # | Family | Rows | Where | Outcome |
|---|---|---|---|---|
| W1 | a computed calculation result written `return <expression>;` | 5 | `examples/phase-c-behavioral-bodies.sysml:60,67,75`, `examples/repl-behavioral-demo.sysml:26,34` | **our defect** — fixed here, as a warning under the F3 precedent |
| W2 | `assert <expression>;` / `assume <expression>;` inside a constraint body | 2 | `examples/phase-c-behavioral-bodies.sysml:86,87` | **our defect** — fixed here, same shape |
| W4 | `done <name>;`, `then <source> <target>;`, `<source> then <target>;` | 5 | `examples/action-executor-demo.sysml:18,20`, `examples/views-demo.sysml:87`, `examples/orthogonal-regions-demo.sysml:16`, `examples/pseudostates-demo.sysml:16` | **our defect** — follow-up **F105** |
| W5 | an action-body member in a definition body (`first <node>;`) | 1 | `examples/views-demo.sysml:83` | **our defect** — follow-up **F106** |
| W6 | `require constraint { … }` in an analysis body | 3 | `examples/solver-demo.sysml:119,123,127` | **our defect** — follow-up **F107** |
| W7 | a connection definition with fewer than two ends | 1 | `examples/views-demo.sysml:34` | **our defect** — follow-up **F108** |
| W8 | an element-filter condition with a non-Boolean result | 1 | `testdata/parse/expressions.sysml:2` | **our defect** — follow-up **F109** |
| W9 | notation already adjudicated: the bare `import` of F2, `region`/`initial`/`transition … to …` of F3 | 7 | `testdata/passes/import_no_visibility.sysml:8,12`, `examples/orthogonal-regions-demo.sysml:10,17,18`, `examples/pseudostates-demo.sysml:9,17` | **adjudicated divergence** — F2, F3 |
| W10 | recovery surplus after a failed parse, including the one semantic row the pilot derives from a recovered succession | 30 | `examples/phase-c-behavioral-bodies.sysml:60` ×4, `:67` ×6, `:75` ×6; `examples/orthogonal-regions-demo.sysml:16` (`Must have at least two related elements`) | **adjudicated divergence** |
| W11 | the file-level give-up row | 4 | `examples/phase-c-behavioral-bodies.sysml:89`, `examples/repl-behavioral-demo.sysml:35`, `examples/views-demo.sysml:90`, `testdata/passes/import_no_visibility.sysml:13` | **adjudicated divergence** |
| W12 | `Duplicate of other owned member name` | 15 | `testdata/passes/import_no_visibility.sysml:3,8,12`, `examples/orthogonal-regions-demo.sysml:11,12,16` ×2, `examples/pseudostates-demo.sysml:9,10,16` ×2, `examples/semantic-layer/demo.sysml:35,105` | **measurement artifact** — every row is recovery-only; the rule agrees on clean inputs, and the one real gap the class hid is fixed (F110, done) |
| W13 | a kind rule over an unresolved or absent type | 19 | `testdata/lex/basic.sysml:4`, `examples/phase-c-behavioral-bodies.sysml:64,65,71,72,73,83,84` | **adjudicated divergence** |
| W14 | implicit binding connectors and filter rules over unresolved operands | 24 | `testdata/parse/expressions.sysml:3,4,5,6`, `examples/solver-demo.sysml:120,124` | **adjudicated divergence** |
| W15 | `Must be an accessible feature` downstream of `namespace` | 5 | `examples/semantic-layer/demo.sysml:44,45,46,50,51` | **adjudicated divergence** — F5, F20 |
| W16 | the `Type::ownedDisjoining` EMF pair | 6 | all six `kerml-examples` rows | **defect of the pilot** — F80 |

#### W1 — `return <expression>;` (5, our defect, fixed)

The pinned grammar's `ReturnParameterMember` is `MemberPrefix 'return' ownedRelatedElement +=
UsageElement` (`SysML.xtext:1961`): `return` introduces a *result parameter declaration*. A computed
result is the keyword-less `ResultExpressionMember` (`:1967`), which `CalculationBodyPart` (`:1951`)
admits once, as the last member of the body. So `return <name>;` and `return result : Real = <expr>;`
both have a production and `return <expression>;` has none — a distinction the corpus depends on,
because the OMG-authored files we assert clean write the first form
(`examples/pilot-corpora/sysml-examples/Simple Tests/CalculationTest.sysml:24`,
`examples/sysml-v2-training/30. Calculations/Calculation Usages-1.sysml:23`).

Matched reproducers, four files differing only in what follows `return`:

```sysml
package R { private import ScalarValues::*; calc c { in a : Real; return a; } }          // pilot: exit 0
package R { private import ScalarValues::*; calc c { in a : Real; return 42; } }         // pilot: no viable alternative at input 'return'
package R { private import ScalarValues::*; calc c { in a : Real; return (a); } }        // pilot: no viable alternative at input 'return'
package R { private import ScalarValues::*; calc c { in a : Real; return a * 2.0; } }    // pilot: no viable alternative at input 'return'
```

We accepted all four in silence. Three of the four are now warned; `return (a);` stays silent, and
deliberately: our parser collapses a single parenthesized expression to its inner node, so the pass
cannot tell it from the legal `return a;` without carrying parenthesis syntax in the AST, which is
not this slice's to change. No corpus row depends on it — `examples/repl-behavioral-demo.sysml:26` is
`return (x * x + y * y);`, whose inner node is an operator expression and is warned.

`passes/nonstandard_notation.go` now warns on the computed form — `LevelSyntax`, the existing
`nonstandard-notation` code, spanned on the keyword — so each of the five rows pairs with a warning of
ours at the same line and category instead of standing alone.

Since superseded: the computed form is no longer accepted at all, so it is a parse error rather than a
warning, and `return (a);` — an expression, not a `UsageElement` — goes with it. Only `return a;` and
`return result : Real = <expr>;` remain, as the result parameter declarations they are.

#### W2 — `assert <expression>;` in a constraint body (2, our defect, fixed)

`AssertConstraintUsage` (`SysML.xtext:2007`) takes a reference subsetting or a constraint
declaration, never an expression, and a constraint body states its condition as the same
keyword-less trailing expression W1 cites. Reproducer:

```sysml
package C { private import ScalarValues::*; constraint validRange { in x : Real; assert x >= 0; } }
```

The pilot reports `no viable alternative at input 'assert'` plus two recovery rows; we reported
nothing. Only **two** rows in this column are the construct itself —
`examples/phase-c-behavioral-bodies.sysml:86,87` — even though the file writes it three times, and
the second of the two is already degraded: at `:87` the pilot has lost the enclosing body and reports
`no viable alternative at input '<='` rather than naming `assert`, while the third occurrence
(`assume initialized;`, `:88`) draws nothing at all, the pilot's parse of the file having ended. That
asymmetry is also why fixing W1 and W2 *adds* rows in the only-OpenSysML column: our warnings land on
constructs past the point where the pilot stopped reading. Both directions were measured with the
oracle and are reported with the change rather than left for a later reader to discover.

The named forms stay silent, and must: `assert constraint c1 : C;`, `assert satisfy r by q;` and
`assume #goal constraint payloadMassLimit;` are all in the corpora we assert clean
(`examples/pilot-corpora/sysml-examples/Simple Tests/RequirementTest.sysml:6,22`,
`examples/pilot-corpora/sysml-examples/Metadata Examples/RequirementMetadataExample.sysml:30`).

The same construct in a **requirement-style** body — `assume <expression>;` and `require <expression>;`
in a requirement, concern, viewpoint, framed-concern, objective or satisfy body, every declaration
whose body the parser reads with `parseRequirementBody` — has no row in this column at all, because in
every file that writes it the pilot's parse has already ended earlier in the file. It is the same
defect: `RequirementConstraintMember` (`SysML.xtext:2057`) admits a reference or an anonymous
`require constraint { … }` body, never a bare expression, and the pinned single-file CLI rejects
`requirement r { attribute x : Real; assume x > 0; }` with `no viable alternative at input 'assume'`,
the `require` spelling with `no viable alternative at input 'require'`, and the same two inside a
`concern def` and a `viewpoint` body. It is warned here too rather than left inconsistent with
`assert` one node type away. A **dotted** condition is a reference, not an expression, and stays
silent in all of them: `requirement r { attribute x; require x.y; }` draws only
`Couldn't resolve reference to Feature 'y'` from the pilot, no syntax error — so does the concern-body
form. The OMG-authored spelling `require constraint { massActual <= massReqd }`
(`examples/sysml-v2-training/32. Requirements/Requirement Definitions.sysml:11,27`) stays silent too.

What the three warned forms moved, measured with `rm -rf build/pilot-diff && go run ./cmd/pilot-diff`
before and after: the seven W1/W2 rows leave this column for the severity-only bucket, the six pilot
recovery cascades behind them shrink by one row each, and **34** rows appear in the only-ours column —
of which 16 come from W1/W2 (`repl-behavioral-demo.sysml:40,46,53,62,67,68,73,78,79,84`,
`phase-c-behavioral-bodies.sysml:88,94,101,102,103,262`) and 18 from the requirement-style body form
(`repl-behavioral-demo.sysml:94,95,98,104,107,113,116,121,122,124,125`,
`phase-c-behavioral-bodies.sysml:112,125,126,127,132,254,255`). Every one of them is a construct past
the line where the pilot stopped reading its file, so the reference says nothing about them at all.
Both files were already non-agreeing, no file changed agreement status, and the severity-only and
agreed buckets are otherwise untouched. Counting only the column this sweep is about would report the
gain and hide the 34; they are the same finding seen from the side the pilot cannot reach.

Both files are ours, and every form the warning names has a spec spelling the pilot accepts, so the
**models** were then rewritten rather than the warning suppressed: a computed result as the body's
trailing expression, a constraint condition keyword-less, and `assume`/`require` as the anonymous
`constraint { … }` body `RequirementConstraintMember` admits. Two constraints that mixed an
assumption with an assertion became requirements, which is where the spec keeps assumptions.
Re-measured with `rm -rf build/pilot-diff && go run ./cmd/pilot-diff`: only ours **153 → 119**, only
the pilot's **130 → 85**, fully agreeing **308 → 309**, severity-only **22 → 15** — that is, the
whole cost of W1/W2 is repaid and 52 of the pilot's rows go with it, because its parse of
`examples/repl-behavioral-demo.sysml` now completes (the file draws nothing from either tool) and its
parse of `examples/phase-c-behavioral-bodies.sysml` reaches line 147 — `then start greenLight;`, W4 /
F105 — instead of stopping at line 60. That file keeps 16 rows, all of them the `transition … to …`
of F3. The warning and its strict-mode test are unchanged: what was removed is our own non-conformant
notation, not the rule, and the extension itself stays supported and tested
(`internal/core/model/constraint_params_test.go`). Every documented demo outcome of the REPL file —
five `%calc` results, five `%constraint` verdicts, four `%requirement` verdicts, including the two
that must fail — is unchanged.

#### W4–W7 — four more notation and structure gaps (10, our defect, not fixed here)

Each is established the same way — a production in the pinned grammars that the construct does not
match, and a minimal file the pilot rejects and we accept — and each needs context the notation pass
does not track today, which is why they are follow-ups rather than part of this diff.

| # | Reproducer (pilot verdict) | What the grammar says | Where ours would live |
|---|---|---|---|
| W4 | `action def A { first start; action compute; done finish; }` → `no viable alternative at input 'done'`; `then start compute;` in the same body → `no viable alternative at input 'then'`; `idle then next;` in a state body → `no viable alternative at input 'idle'`; the one-ended `then compute;` is accepted | a succession names its ends `'first' … 'then' …` (`SuccessionAsUsage`, `:1033`) or continues from the previous node with a *single* end (`TargetSuccessionMember`, reached from `ActionBodyItem`, `:1368`). Two names after `then`, and `done` as a keyword, have no production — `done` is a library name (F8) | `passes/nonstandard_notation.go`, on a succession member with two written ends and no `first` |
| W5 | `part def P { first start; action a; }` → `mismatched input ';' expecting 'then'` and `Must have at least two related elements`, where the same body under `action def P` is accepted with no diagnostics | `DefinitionBodyItem` (`:516`) admits a `SuccessionAsUsage`, which must name both ends (`'first' … 'then' …`, `:1033`); the one-ended initial-node form is `ActionBodyItem`'s alone (`:1368`). The construct is legal, in another body kind | `passes/nonstandard_notation.go`, which would need the enclosing body kind |
| W6 | `analysis def B { attribute p : Integer; require constraint { p <= 220 } }` → `no viable alternative at input 'require'`, where the same members under `requirement def B` are accepted | `RequirementConstraintMember` (`:2057`) is reachable only from `RequirementBodyItem` (`:2039`) | same as W5: the enclosing body kind |
| W7 | `package W { connection def FuelLine; }` → `Must have at least two related elements` | `KerMLValidator.checkAssociation` / `validateAssociationRelatedTypes`; a connection definition is an `Association`, and an association relates at least two types. Adding two `end` members clears it | `LevelConstraint`, beside the `flow-end-subsetting` check F21 added — our own model is the invalid one, as it was for F21 |

#### W8 — a non-Boolean filter condition (1, our defect, not fixed here)

`testdata/parse/expressions.sysml:2` is `filter 1 + 2 * 3;`. The pilot reports `Must have a Boolean
result` (`KerMLValidator.checkElementFilterMembership` /
`validateElementFilterMembershipIsBoolean`); `filter 1 + 2 * 3 > 0;` is accepted. We report something
at that line — `filter-not-evaluable` from `passes/filter.go` — but not this rule, so the pair is not
agreement in substance and the row stays here. This is the *third* direction of F22's alignment, and
the only W-family row in this column whose rule we partly implement already: follow-up **F109**.

The other five `Must have a Boolean result` rows are W14, not this: at
`testdata/parse/expressions.sysml:3,5,6`, `testdata/passes/errors.sysml:3` and
`testdata/resolve/errors.sysml:3` the condition's own references are unresolved — an agreement row in
each case — and the pilot's constraint reads `result.specializesFromLibrary('ScalarValues::Boolean')`,
which is false for a condition it could not link. The distinction is measurable rather than argued:
with the references declared, the pilot reports the rule only when the result is genuinely not
Boolean.

#### W10, W11 — recovery surplus and the give-up row (34, adjudicated divergence)

`examples/phase-c-behavioral-bodies.sysml:67` is the clearest case: one construct of ours
(`return sqrt(dx * dx + dy * dy);`) draws seven pilot rows — `no viable alternative at input
'return'`, `missing '}' at 'sqrt'`, then one per operator and parenthesis — and four
`Duplicate of other owned member name` warnings behind them. The construct is one finding (W1) and it
is booked once. `:89`'s `missing EOF at '}'` is the same event seen from the end of the file: the
pilot stops there, which is why every later line of that file draws nothing from it and why our
notation warnings past that point have no pilot row to pair with. Counting a recovery cascade as
separate gaps would make our notation debt look an order of magnitude larger than the number of
constructs behind it — 34 rows for what is, in this column, 19 constructs.

#### W12 — `Duplicate of other owned member name` (15, measurement artifact; F110 done)

Re-measured on current `main` with the succession-shorthand notation work landed, this class holds
**15 pilot-side diagnostics over 11 rows, 0 only-ours rows and 2 agreement rows**, and every
pilot-only row is class (b), a line the pilot reached only through recovery:
`testdata/passes/import_no_visibility.sysml:3,8,12` behind the bare `import` of F2 (the pilot errors
at `:8` and `:12` and reads `Lib` as a member of the enclosing body),
`examples/orthogonal-regions-demo.sysml:11,12,16` ×2 behind the `region` of F3 (its error at `:10`
flattens both regions into one namespace, which is what makes `start`/`red` repeat),
`examples/pseudostates-demo.sysml:9,10,16` ×2 behind the same class (error at `:9`), and
`examples/semantic-layer/demo.sysml:35,105` behind the `namespace` of F3, where the pilot's error is
on the duplicate's own line. None of these is evidence about the rule.

The rule itself is adjudicated on inputs both implementations parse identically
(KerML 7.2.2 / SysML v2 7.6.1, `validateNamespaceDistinguishability`: the names a namespace's
memberships declare must be distinguishable). It **agrees** for every ordinary member —
`part def P { part a; part a; }`, the same with `attribute`, `action`, `enum` and `part def`
members, and `calc c { in a : Real; return a; }` — matching line, column and multiplicity, and it
correctly stays silent where the names are in sibling namespaces
(`state def S { state r1 { state x; } state r2 { state x; } }`). Two clean reproducers did diverge,
and there we **under-reported**: `state def S { state red; state red; }` (also in a `state` usage, a
nested state and a `parallel` body; the same members in a `part def` body parse as ordinary usages
and were already reported) and
`state def S { state a; state b; transition t first a then b; transition t first b then a; }` draw
two warnings from the reference and none from us. Both declarations name a member of their
container, so the reference is right; our distinguishability check filters members by whether they
declare a name of their own, and these two nodes were the only named declarations not recording the
span of the name they declare, so they were dropped before the check. Recording it fixes both, and
our warnings then match the reference's line and column. The corpus counts do not move — no corpus
file has clean duplicate state or transition names, all 15 rows above being recovery — so this is a
reproducer-only movement, pinned by `passes/f110_state_duplicate_names_test.go`. Follow-up **F110**
is done.

#### W13, W14 — the tier boundary (43, adjudicated divergence)

`testdata/lex/basic.sysml:4` is the whole argument in one line. `attribute mass : Real;` in a file
with no `ScalarValues` import: both implementations report `Real` unresolved — that is an agreement
row — and the pilot *additionally* reports `An attribute must be typed by attribute definitions.`
(`SysMLValidator.checkAttributeUsage` / `validateAttributeUsageType_`), because its kind check runs
over a graph where the type is `null`. Our tiers stop at the name-resolution error, which
`AGENTS.md` §4 makes an invariant rather than an omission: the higher tiers do not run on a document
whose lower tier failed. Every W13 row has this shape (`A usage must be typed by definitions.`,
`An occurrence, item or part must be typed by occurrence definitions.`, and the unresolved
`Real`/`Boolean`/`Integer` rows the pilot reports for a *second* time after recovery lost the file's
imports), and every W14 row is the same thing one layer further out:
`checkImplicitBindingConnectors` / `validateBindingConnectorTypeConformance` comparing the types of
two ends of which at least one did not link, and the filter rules over the same conditions.
`examples/solver-demo.sysml:120,124`'s `drivePower` and `sciencePower` resolve for us and are
unresolved for the pilot only because W6 cost it the enclosing body.

Two of these rows are a genuinely different reporting decision rather than a tier boundary, and are
kept: at `testdata/parse/expressions.sysml:3` (`filter a.b.c;`) the pilot reports `b` and `c`
unresolved in addition to `a`, where we stop at the first unresolved segment of the chain. Reporting
each segment of a chain whose head is already unresolved adds no information about the model.

#### W16 — the six `kerml-examples` rows, checked one by one (6, defect of the pilot)

The wave brief asks whether all six really fall to F80 if upstream fixes it. They do, and each was
checked on its own rather than by family: every one of the six files contributes **exactly one**
pilot-only row, each row is EMF's unpaired-bidirectional-reference diagnostic over the
`Disjoining::owningType` / `Type::ownedDisjoining` pair, and each line is a `disjoint from` clause
written *in a type declaration* — the form F80's mechanism section pins to the `OwnedDisjoining`
production:

| File:line | The clause |
|---|---|
| `KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9` | `classifier YourBike [1] specializes Bicycle disjoint from MyBike;` |
| `Simple Tests/Classifiers.kerml:13` | `classifier D disjoint from C differences A, B;` |
| `Simple Tests/FeatureChains.kerml:31` | `feature h2 differences b.f, b.a intersects f.a, g disjoint from h1;` |
| `Simple Tests/Features.kerml:20` | `feature z unions f, g disjoint from y;` |
| `Simple Tests/Inverses.kerml:3` | `feature f : B inverse of B::g disjoint from h;` |
| `Simple Tests/Types.kerml:31` | `type C :> B disjoint from A;` |

Three details make the conclusion falsifiable rather than assumed. The clause appears on classifiers,
plain types and features alike, and the reported EMF class tracks the declaration
(`ClassifierImpl`, `TypeImpl`, `FeatureImpl`), so the defect is in the pair and not in one metaclass.
The three-line reproducer of F80 still fires at this pin, with `ClassifierImpl` naming the owner. And
the standalone form `disjoint b.f.a from b.a;` (`Simple Tests/FeatureChains.kerml:28`) is in the same
file as one of the six and reports nothing — so a fix to the derived `ownedDisjoining` delegate would
clear all six rows and would not silence anything else this root depends on. No `kerml-examples` file
carries a second pilot-only row of any kind, so this root's whole column is F80.

### Unmapped messages, verbatim

Recorded so the categorisation's debt is visible rather than hidden:

| Side | Message | Count |
|---|---|---|
| pilot | `Duplicate of other owned member name` | 25 |
| pilot | `Must be an accessible feature (use dot notation for nesting)` | 5 |
| pilot | `Cannot identify flow end (use dot notation)` | 2 |
| pilot | `Must be model-level evaluable` | 1 |
| pilot | `The opposite features 'owningType' … do not refer to each other` (K6, one row per file) | 6 |
| opensysml | `<name> participates in a specialization cycle` | 11 |
| ~~opensysml~~ | ~~`interface Mounting connects ports AxleMountIF and WheelHubIF, whose directed features are not conjugate; one end usually names the conjugate port (~AxleMountIF)`~~ | ~~1~~ |
| opensysml | `name conflict: text is already the name of the inherited feature ModelingMetadata::Issue::text` | 1 |
| ~~opensysml~~ | ~~`only a definition may specialize; found a usage` (K4)~~ | ~~21~~ |
| ~~opensysml~~ | ~~`packet data field redefines packet data field, but packet data field is not an inherited member of Thermal Data Packet`~~ | ~~1~~ |

The struck rows are gone from the run: F32 retired the K4 class, F31 the `Packets.sysml`
redefinition, and the conjugation row is retired by the interface-flow pairing described in
[the remaining only-ours rows](#the-remaining-only-ours-rows) below. Our side of the bucket is 12,
the pilot's 39.

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
| ~~F9~~ | **Done** (#382). A second list of contextual words now feeds the VS Code grammars and the LSP keyword completion, so `point`, `chain`, `on` and `var` are highlighted and completed without being reserved by the lexer. Contextual keywords are neither highlighted nor completed: the VS Code grammars and the LSP keyword completion are generated from `lexer.Keywords()`, which `point`, `chain`, `on` and `var` are deliberately absent from. `var` is real KerML notation, so a second list of contextual words for those two surfaces would restore it without reserving it. |
| ~~F2~~ | **Done.** A bare `import` (no visibility) is non-conforming: the pilot's grammars make the indicator mandatory — `fragment ImportPrefix returns SysML::Import : visibility = VisibilityIndicator 'import' ...` with no `?`, unlike the sibling `MemberPrefix` (`KerML.xtext:169-172`, `SysML.xtext:241-244`) — and all 574 imports across the 254 OMG-authored corpus files carry one. Decision: **warn, not error** (`passes/import_visibility.go`, `LevelSyntax`, code `syntax/import-visibility`, spanned on the `import` keyword). The form is unambiguous to parse, so hard-failing would reject existing models over notation; the warning surfaces the non-conformance without gating the higher tiers. `expose` is exempt — the pilot grammar gives it implicit `protected` visibility (`SysML.xtext:2366-2372`). Our own fixtures now write an explicit visibility, with `testdata/passes/import_no_visibility.sysml` kept to lock the warning in. |
| ~~F3~~ | **Done.** Audited in [conformance-audit.md](../reference/grammar/conformance-audit.md): `namespace` is a literal in `KerML.xtext` only (`:125`) and a `.sysml` root admits package members only (`SysML.xtext:38`), so **both** body forms are the defect, not the semicolon one — the wording in P2 above was wrong. `region`, `choice`, `junction`, the history forms, `entry`/`exit point`, `defer`, and the `initial`/`final`/`decision` spellings have no production either. Decision: **warn, not error**, as F2 did — `passes/nonstandard_notation.go`, `LevelSyntax`, codes `nonstandard-notation` and `kerml-notation`, spanned on the word. The notation stays parsed, so existing models keep working; `namespace` stays silent in `.kerml`, where it is legal. `TestStdlibHasNoNonstandardNotation` locks in that no OMG-authored library file draws either warning. |
| ~~F4~~ | **Done.** The pilot has no specialization-cycle check at `2026-05`: `KerMLValidator.checkSpecialization` implements only `validateSpecializationSpecificNotConjugated`, no message constant in either validator concerns cycles, and the pilot accepts self-, two- and three-element cycles with zero diagnostics on otherwise-empty probe files. Our diagnostic stays, and stays `unmapped` — the finding is one-sided and no coarse category describes it. See [Specialization cycles (F4)](#specialization-cycles-f4). |
| ~~F5~~ | **Done.** The nine P6 diagnostics are adjudicated per diagnostic above: 5 downstream of P2, 2 a real gap (flow ends), 2 downstream of unresolved references both sides report. It spawned F20–F23, one per pilot rule, since all four rules are real whatever their diagnostics in *our* fixtures turned out to be. |
| ~~F20~~ | **Done**, and the reopening condition was met by finding *where* we were silent rather than by re-attempting the predicate. `canAccess` over featuring types was already implemented (`passes/w8c_feature_reference.go`, `LevelConstraint`), but it only saw a usage's *value* expression, so eight constructs both implementations parse identically had the pilot reporting and us silent: a reference through the owning type's namespace (`P::Q::n`) written in a `constraint def` body, an `assert constraint` body, a `calc` `return` or a `calc` body's implicit (last-expression) result, a transition guard, a `require`/`assume` constraint, and an `assign` value. Confirmed one by one against the pinned validator. The check now walks those bodies, and a body-written reference is accessible when its own declaration features the target — the trap the reverted attempt fell into, since a body reaches its own type's features, inherited and redefined ones included, and a dot path (`s.mass`, `q.n`) reaches a nested one. Training corpus clean, no pilot-corpora row moved, pilot-diff unchanged (the reproducers are not corpus constructs, as the row predicted). Element-filter conditions are covered too — a `filter` member and an import's `[...]` clause — with the candidate element as the featuring context rather than an enclosing declaration: the pilot reports exactly when the referent is a feature of a *user-declared* type and never when it is library-declared, because its rule only runs where the referent has featuring types and a library feature it never transformed has none. So `filter R::M::a` is reported while `filter Element::name == "System" and not Type::isAbstract`, `filter (as Safety).isMandatory` and `filter @Safety` stay clean, which is what keeps `kerml-examples/Simple Tests/Filtering.kerml` valid. Both sides of that boundary were confirmed one by one against the pinned validator: a user `metaclass` feature, one whose metaclass specializes `Element`, a user `struct`, a user type specializing `Occurrence` and a `part def` attribute are all reported; five library referents (metaclass and not) and dot notation on a cast are all clean. A chain written in a filter draws this message rather than the referent one, which is what the pilot does on `filter M::a.b` — though our type tier reports every chain-shaped condition as not model-level evaluable first, so that mapping is unobservable through the CLI today. Where our type tier already errors on the condition — not Boolean, not model-level evaluable — the constraint tier stays suppressed, so those shapes remain only-pilot rows. Training corpus clean, no pilot-corpora row moved, pilot-diff aggregate unchanged. `validateSubsettingFeaturingTypes` (`Must be an accessible feature (use dot notation for nesting)`). Earlier: **attempted and reverted** (#376) for training-corpus false positives. |
| ~~F21~~ | **Done** (#376). Implemented as constraint-tier `flow-end-subsetting`; `examples/views-demo.sysml:44` corrected to dotted ends in the same commit. `validateFlowEndSubsetting` (`Cannot identify flow end (use dot notation)`): a flow end must name the feature the payload leaves from or arrives at. The only P6 real gap in the corpus, and it also means `examples/views-demo.sysml:44` (`flow of Fuel from tank to thruster;`) is invalid and needs dotted ends. Highest priority of F20–F23: bounded scope, our own model already violates it. |
| ~~F22~~ | **Done** (#376). Aligned in both directions: a literal-only filter const-folds instead of warning, and a reference to a feature featured within a type is no longer silently accepted. `validateElementFilterMembershipIsModelLevelEvaluable` (`Must be model-level evaluable`): align `passes/filter.go` `filter-not-evaluable` with the spec's `isModelLevelEvaluable` — the rule is not absent but divergent in both directions (we warn on `filter 1 + 2 > 0;`, which the pilot accepts; we are silent on a reference to a feature with a featuring type, which it rejects). Second priority: it is the only one of the four that can produce a *false positive today*. |
| ~~F23~~ | **Done** (#376). Implemented as type-tier `invocation-not-behavior`, following a single typing relationship transitively so `calc t : Twice;` invoked as `t(3)` is accepted. `isBehaviorKind` was left alone; the wider classification is separate. `validateInvocationExpressionInstantiatedType` (`Must invoke a behavior or a behavioral feature`): what is invoked must be a behavior, or a feature typed by exactly one behavior. Third priority. Its pilot-side category is now `kind-mismatch` rather than `unmapped` (see the note under the results table), so once implemented it can agree rather than merely coincide. |
| ~~F6~~ | **Done** (#397), and its premise was wrong. A pinned plain-Java bridge (`scripts/pilot-sysml-validator/ValidateSysML.java`) batch-loads the SysML side without needing a Tycho-capable Maven, so `orderByImports` and `batchByBaseName` are deleted and both languages share one single-batch path. It did **not** eliminate P4: all 25 `Duplicate of other owned member name` warnings survive, reproduce from a single file under both wrappers, and are intra-file duplicates — so P4 is a reference rule we do not implement, and that row is rewritten above. What it did remove is three order-dependent pilot-only diagnostics (142 → 139). |
| ~~F7~~ | **Done** (#389), as an optional third column in `cmd/pilot-diff` that leaves the committed two-way baseline reproducing byte for byte when SysIDE is absent. Scope limit worth stating: `syside check` is a *checker*, so it can corroborate static rows — name resolution, notation acceptance, static typing, kind rules — and says nothing about execution semantics. Add [Sensmetry `syside check`](https://github.com/sensmetry/syside) as an *additional* cross-check. It is a different implementation, not the reference, so it can only corroborate — never adjudicate. |
| ~~F10~~ | **Done.** The pinned pilot release *does* ship KerML validation — `org/omg/kerml/xtext/validation/KerMLValidator.class` and `KerMLStandaloneSetup.class` — and what was missing was only a CLI. `scripts/pilot-kerml-validator/ValidateKerML.java` supplies one over the pilot's own `IResourceValidator`, sanity-checked on malformed, unresolvable and known-good input, and `kerml-examples` is a root. |
| ~~F30~~ | **Done.** All four constructs are parsed: `featured by` (`KerML.xtext:569,659`), n-ary connector end lists (`:842`), a typed/redefining succession before `first … then` (`:891`), and `at`/`while`/`merge`/`decide` as names in a `.kerml` file — none is a literal of `KerML.xtext` or `KerMLExpressions.xtext`, so the F3/F8 precedent applies and they are unreserved by file kind. The KerML root's only-ours count is 439 before F30, **268** after K1, **291** after everything, its syntax diagnostics falling 360 → **140**; the net rise over K1 is unresolved references in bodies that now parse (K3/F31). The committed baseline is untouched. |
| ~~F50~~ | **Done** (#374). The KerML feature prefixes we rejected: `abstract var feature x [0..*];` and `member abstract feature x …` (`Variable Feature Examples/TimeVaryingCarDriver.kerml:53,100`, 2 diagnostics). A modifier before the `var` prefix, and a modifier after `member`, are both refused where each alone is accepted — the remainder of K2 after F30. |
| ~~F51~~ | **Done** (#360), as a per-snippet parse kind. A file's kind did not reach the REPL/`-validate` surface: `submitFiles` opens the accumulated buffer as one document named `<repl>` (`internal/repl/session.go:25,728`), so `source.KindOf` is `KindUnknown` and the file-kind gates in the parser never fire — `at`/`while`/`merge`/`decide` stay reserved there, while `-convert`, which passes the real path, accepts them. The pass layer already compensates for the `kerml-notation` *warning* alone (`dropKerMLNotationOfKerMLFiles`); since the buffer mixes `.sysml` and `.kerml` snippets, the fix is a per-snippet parse kind rather than a document one. |
| ~~F52~~ | **Done** (#391). The two fixtures redefined a *sibling* succession, which the reference rejects too; they now redefine an inherited succession and resolve. A succession's `redefines` target does not resolve: `succession redefines named : T [1] first a then b;` reports `redefines target must be a usage or definition, found unknown` in both `.sysml` and `.kerml` (the fixtures `succession_declared_multiplicity.sysml:12`, `kerml_succession_declaration.kerml:18`). Predates F30 — the same message comes from the pre-F30 binary for the keyword-less form — and is only reachable now that the typed declaration parses. Parse is clean; the defect is in resolving a succession usage as a redefinition target. |
| ~~F53~~ | **Done** (#388). The kind table was too narrow, as suspected: `SuccessionAsUsage` and `BindingConnectorAsUsage` (`SysML.xtext:1020,1033`) both route a declared type through a plain `UsageDeclaration`, and the reference accepts any *definition* there and rejects only usage targets — so the check now rejects only usage targets, with a negative test holding that line. Retired the unmasked `kind-mismatch` on `Simple Tests/ConnectionTest.sysml` as well. `succession s : SomeConnectionDef …` is reported as `succession cannot be typed by connectionDef (kind mismatch)` (`succession_declared_multiplicity.sysml:11,12`). `SuccessionAsUsage` (`SysML.xtext:1033`) types a succession through `UsageDeclaration` like any usage, so the kind table is likely too narrow — confirm against the reference before widening it, and see F32 for the rest of the kind-mismatch class. |
| ~~F60~~ | **Done** (#364), re-verified on `main` and pinned by the `prefix_metadata_and_keywordless_members` golden fixture: all four shapes parse clean, and the prefixed member is a `ReferenceUsage`, so the typing check no longer constrains it by the attribute kind. S1, 32 diagnostics over 7 files. Prefix metadata as a member's only keyword: `ExtendedUsage` (`SysML.xtext:730`) is `UnextendedUsagePrefix UsageExtensionKeyword+ Usage`, returning a plain `Usage`. Two symptoms, one gap: `#M connect a to b;` and `abstract #Classified z;` do not parse, while `#service sd : PortDef;` and `end #original r1 : Req1;` parse *as attribute usages* and then draw a `kind-mismatch` (6 of the 32). Parser-owned; the semantic half disappears with it. |
| ~~F61~~ | **Done** (#364), re-verified on `main` and pinned by the `prefix_metadata_and_keywordless_members` golden fixture: value-only, specialization-only and redefinition-only declarations, anonymous enumerated values, a trailing result expression and an anonymous `locale` comment all parse clean, and none of the 12 files reports a diagnostic. S2, 87 diagnostics over 12 files (72 of them the two generic recovery messages). Keyword-less members: a `DefaultReferenceUsage` whose declaration is only a value, only a specialization or only a redefinition (`distancePerVolume :> scalarQuantities = distance / volume;`, `T1 = 10.0 [N * m];`, `value :>> elements: Integer;`), an anonymous `EnumeratedValue` (`= 60.0;`), a `ResultExpressionMember` as the last body member (`v.m`), and an anonymous `Comment` carrying a locale (`locale "en_US" /* … */`, `Comment` at `:86` makes the keyword optional). The single largest class, and it leads the two biggest files in the corpus. |
| ~~F62~~ | **Done** (#363, #374, #462). Re-measured on current main before any further work: every one of the seven files reports zero diagnostics and all six constructs parse clean, so nothing was left to fix. `then S2.S3;`, the transition and succession bodies and the `send`/`accept` bodies came from reading a node's declaration and body in one place (#363); the dotted `exhibit` reference with a body from #374; the chained succession ends from #462, which also scoped a succession body to the action target succession — `entry; then starting { … }` stays refused, because `EntryTransitionMember` (`SysML.xtext:1796`) ends in `;` and not in a body. S3, 65 diagnostics over 7 files. Bodies and dotted references in state/occurrence behavior: `then S2.S3;`, `then starting { … }`, `action a send x via p { … }`, multi-line `accept c : C via p`, `first start then continue { … }`, `exhibit vehicleStates.on { … }`. `TransitionUsage`, `TargetTransitionUsage`, `SendNode` and `AcceptNode` all end in `ActionBody`; `ExhibitStateUsage` takes `OwnedReferenceSubsetting` plus `StateUsageBody`. |
| ~~F63~~ | **Done** (#363, #374, #462). Re-measured on current main before any further work: all five files report zero diagnostics and all five constructs parse clean, so nothing was left to fix. The control-node name and body, `decide 'test x';`, the typed `for` variable and the redefining body parameter came with #363; `ref patient { … }` with #374; the action nodes in a case body with #462. S4, 33 diagnostics over 5 files. Action nodes: a body on any `ControlNode` (`then fork F { … }`), a named `decide 'test x';`, a typed `for` variable (`for n : ScalarValues::Integer in (1, 2, 3)`), a body parameter that only redefines (`in :>> payload = s;`), and a body on a bare `ref` (`ref patient { … }`). |
| ~~F64~~ | **Done** (#375). A `return` is a usage when its declaration specializes, and a body expression is a calculation body that may declare features. A body-expression declaration parses but its name is not yet in scope for the result expression — F99. S5, 19 diagnostics over 6 files. `ReturnParameterMember` is `'return' UsageElement` (`:1961`), so `return selectedEngine :> engine;` and `return attribute accelerationProfile :> ISQ::acceleration[*] := ();` are usages, not expressions; a `private attribute` declaration inside an expression body is legal; and `assert not c { … }` is an `OperatorExpression`, so `not` must not be forced to name a constraint. |
| ~~F65~~ | **Done** (#383). All three forms discriminated against the reference once corpus-faithful fixtures were built — ours errors, the pilot is silent — so this was a real gap rather than a fixture artifact. `pilot-validation` syntax 20 → 8 (`17a`/`17b-Sequence-Modeling` 6 → 0 each) and `pilot-examples` syntax 75 → 65. Note the one increase in the wave: `Arrowhead Framework Example/AHFSequences.sysml` goes 6 → 15, because recovery was previously swallowing whole connection bodies and nine `unresolved reference` findings were masked behind the unparsed member. S6, 22 diagnostics over 5 files. `binding ab1 : AB bind a = b;` (`BindingConnectorAsUsage` allows a `UsageDeclaration` before `bind`), `message m of Publish[1] …` (the `Payload` after `of` is `OwnedFeatureTyping ( OwnedMultiplicity )?`), `event e = m.start;` (`EventOccurrenceUsage` ends in `ValuePart? UsageBody`). The `binding` and `event` reproducers draw a *different* pilot diagnostic, so both need a corpus-faithful fixture before a fix is validated against the reference rather than the grammar alone. |
| ~~F66~~ | **Done** (#375). `assume`/`require constraint` owns a declaration, not only a body. S7, 25 diagnostics over 5 files. `assume constraint c1 : C;` and `assume constraint c { … }` (`RequirementConstraintMember`, `:2057`, whose body is optional), `verify r :>> massRequirement;` (`RequirementVerificationUsage`), `variant use case uc11;` (`UseCaseUsage` is reachable from `VariantUsageElement`), and multiplicity after a redefinition (`ref redefines cylinderBR[4];` — six identical lines × 2 diagnostics = that file's 12). |
| ~~F67~~ | **Done before this round, and re-verified rather than re-fixed.** Measured on `main` with a fresh `cmd/pilot-diff` run over the pinned `2026-05` validators: all 12 files are fully agreeing, `pilot-examples` only-ours stands at 8 with no `unresolved-reference` among them, and the per-file corpus ratchet records no row for any of them. Each shape carries a committed regression test rather than only the corpus: the import-of-an-imported-name and feature-chain-subsetting shapes, plus the `include` actor redefinition and the bare `variant` reference, in `internal/core/resolve/f67_import_reexport_test.go`; the `item :>> shape : Box [1] { … }` shape against the real `SpatialItems`/`ShapeItems` library context — the corpus-faithful fixture this row asked for — in `internal/core/model/f67_inherited_shape_test.go`. As adjudicated: S8, 43 `unresolved-reference` over 12 files, all resolved by the reference. Two shapes reproduce and are ours: a name introduced into a namespace *by an import* and then wildcard-imported onward (`private import RiskLevelEnum::*;`), and subsetting a feature reachable by feature chain (`part aa subsets a;`). The largest shape, `item :>> shape : Box [1] { … }` (12 diagnostics), is the inherited-member lookup #331 fixed for `length`/`width`/`height`, still failing when the redefinition itself introduces the type — it needs a fixture built from the corpus's library context, since the minimal form agrees. |
| ~~F68~~ | **Done** (#391). Two rules, both from the corpus files rather than minimal forms: a transition's trigger fills a parameter slot of the transition itself, so a sibling names the payload through the transition; and a feature that takes its name from what it *redefines* is a reference-subsetting target, unlike a name borrowed from a reference. `pilot-examples` `unresolved-reference` 56 → 37. S9, 39 `unresolved member`/`unresolved reference` over 6 files, all in files the reference validates cleanly, all reaching through a behavioral usage into what it implicitly parameterizes (`subscribing.sub`, `producer.publish_request`, `succession flow x.p to a1.aa.receiver`). The minimal forms agree, so the corpus files are the evidence and a faithful fixture has to come from them. 3 of the 39 are `rep inOCL language "ocl"` — the same textual-representation gap as F70, on the SysML side, and they went with it: `Simple Tests/TextualRepresentationTest.sysml` validates clean. |
| ~~F69~~ | **Done** (ours half). #362 widened the usage-typing kind table to the reference's occurrence/case/behavior taxonomy and made bind conformance accept a value type conforming in either direction, and wave 10B (#467) recast the rules in the reference's wording — re-verified on `main`: all five ours files are clean and draw no only-ours row in `cmd/pilot-diff`. The 3 one-sided checks stay as adjudicated. Was handed to wave-10 slice 10B, which owns `internal/core/passes`: all 5 ours rows are that package's (`typecheck.go` `compatMessage`/kind table, `typecheck_value.go`). S10, 8 diagnostics over 8 files, one each — **5 ours, 3 one-sided.** Ours: `part x : ItemDef` and `use case uc : UseCaseDef` (the kind table is narrower than the reference's `An occurrence, item or part must be typed by occurrence definitions`; see F53 and F32 for the rest of that class), a bind whose value type specializes the feature's, and `action d : OccurrenceFunctions::destroy` resolving to a calc usage. One-sided (kept, on the F4 precedent): the inherited-name conflict on a metadata body's `text`, the interface-conjugation warning, and the units check — probed directly, the reference has no dimensional analysis at all. |
| ~~F31~~ | **Done.** All three shapes were ours, and none was cascade: measured on merged `origin/main` the class is 123 of the root's 269 only-ours, and 116 of the 123 are four genuine resolution defects — implicit generalization missing from inherited-member traversal (58), import visibility (15), a declaration's header not seeing its own body (39), and the implicit base suppressed by any declared generalization rather than only by one that already reaches it (4). The root falls **269 → 150** and the class **123 → 7**, with the four `.sysml` roots byte-identical per file and the committed baseline untouched. The 7 remaining are F70–F72. |
| ~~F70~~ | **Done** (#374). The identifier and the language string are preserved on `ast.TextualRepresentation`. `rep inOCL language "ocl"` (`Simple Tests/TextualRepresentation.kerml:7`, 3 diagnostics): a textual-representation member is not parsed, so `rep`, `inOCL` and `language` are read as names to resolve. Parser-owned, alongside F50. Re-verified on current `main`: both corpus files (`Simple Tests/TextualRepresentation.kerml` and `TextualRepresentationTest.sysml`, the SysML residue of the behavioral-member row) validate clean, the anonymous `language "alf" /* … */` spelling parses in both file kinds, and the member is adopted by the namespace it represents, so a sibling and a qualified name reach it. One residue closed with the re-verification: the identification may be a short name (`rep <ocl> inOCL language "ocl"`, `Identification` is `'<' Name '>' Name?`), which was still rejected. Represented text is carried, never interpreted — recorded as the boundary in `spec-compliance.md`. |
| ~~F71~~ | **Done** (#375). The cause was narrower than stated: `snapshot`/`timeslice` are SysML-only literals (`SysML.xtext:864`), so in a `.kerml` file they are names. A parameter's name is lost for `in timeslice : Timeslice;` inside `expr while { … }` (`Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:27`): the AST carries `Usage{Keyword: "timeslice", Ident: ""}`, so no symbol is built and `at(timeslice.interval)` cannot resolve. The name never reaches symbols, so this is a parser representation gap, not resolution — the shape it belongs to is F30's `at`/`while` work. |
| ~~F72~~ | **Done** (#391). The rule is the first of the two candidates: the body of a redefining feature sees the features nested under what it redefines, including an association end's implicit redefinition. `Association Examples/ProductSelection_N_ary.kerml` 3 → 0. `member feature Product_Account1 subsets Product_Account …` inside `assoc SingleProductSelection3` (`Association Examples/ProductSelection_N_ary.kerml:93,101,109`, 3 diagnostics): the target is a member of a *nested* member of the end feature this end redefines, and the pilot resolves it. Which rule makes it visible — inherited nested members through a redefined end, or a featuring path — is not established, so no fix was guessed at. |
| ~~F32~~ | **Done.** Adjudicated per row. The first two rows are SysML-only rules: KerML has no definition/usage distinction — a Specialization relates two Types and a FeatureTyping's type is any Type, a Feature among them (KerML 1.0 §8.3.3, §8.3.4.4) — so on a `.kerml` document `passes/typecheck.go` `compatMessage` checks only that the target *is* a type, reading the language from `source.KindOf` (the F3+F8 file-kind mechanism, no second notion). The metaclass row was our own bug in either language: `defSymbolKind` had no `ast.DefMetaclass` case, so a metaclass was incomparable with its own kind; `metaclass … specializes Metaobject` is a Class specializing a Class (§8.4.4). The `rollsOn` row is not a language gate but missing semantics: unioning was resolved nowhere, so `semantics/model.go` now resolves `unions` in its own cache (`UnioningTypes`) and `Conforms` accepts a union whose every unioning type conforms — a union is constrained by its members, not a generalization of them, so it stays out of `DirectSupertypes`. Nothing here was skipped wholesale: the KerML rows keep a non-type target an error, and `.sysml` counterpart tests lock in that each check still fires. |
| ~~F33~~ | **Done.** The six are one cause and it is the reference's: the derived `Type::ownedDisjoining` is empty for a `Disjoining` whose `owningType` is that `Type`, so the `eOpposite` pair EMF checks is inconsistent in the pilot's own graph. Reproduced in three lines, surviving validation of a single file in a fresh resource set, so neither batching nor the bridge is implicated; the notation is `KerML.xtext:344` `DisjoiningPart` as the reference's own examples write it. Category stays `unmapped`. See [K6, diagnostic by diagnostic (F33)](#k6-diagnostic-by-diagnostic-f33). Spawned F80–F83. |
| F80 | **Written, not filed.** The upstream report for K6 against the pilot at `2026-05` — `Type::ownedDisjoining`'s setting delegate does not see a `Disjoining` that `owningType` reports it owns, so every `disjoint from` in a type declaration draws EMF's unpaired-bidirectional-reference error — is in [omg-issues.md](omg-issues.md#typeowneddisjoining-does-not-contain-a-disjoining-whose-owningtype-is-that-type-pilot-2026-05), ready to paste into `Systems-Modeling/SysML-v2-Pilot-Implementation`. The reproducer and the probe output are in the K6 section; nothing on our side changes when it is fixed except the root's only-pilot count falling 6 → 0. |
| ~~F81~~ | **Done** (#374), as `ast.RelDifferences` with an RDF/export mapping. `differences A, B` in a type declaration is not parsed: `classifier D differences A, B;` gives `expected '{' or ';' after declaration` and `expected a namespace member`, where the pilot is silent (`KerML.xtext:359` `DifferencingPart`). `intersects` and `unions` at the same position parse, so it is that one keyword. 4 of the root's 140 syntax diagnostics (`Simple Tests/Classifiers.kerml:13`, `FeatureChains.kerml:31`). |
| ~~F82~~ | **Done** (#374), through the parser path namespace and body members share. A standalone disjoining as a namespace or body member is not parsed: `disjoint B from A;` gives `expected a namespace member` where the pilot is silent. `Disjoining` is a `NonFeatureElement` alternative (`KerML.xtext:257`, production at `:426`), so the keyword may open a member. 1 diagnostic (`Simple Tests/FeatureChains.kerml:28`). |
| ~~F83~~ | **Done** (#374), preserved on `ast.Definition.Multiplicity` and gated on the KerML declaration syntax — a SysML definition declaration has no such multiplicity slot. A multiplicity in a classifier declaration is not parsed: `classifier B [1] specializes A;` gives `expected '{' or ';' after declaration` and `expected a namespace member` where the pilot is silent. `ClassifierDeclaration` takes `( ownedRelationship += OwnedMultiplicity )?` before the superclassing part (`KerML.xtext:468-470`). 2 diagnostics (`KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9`). |
| ~~F84~~ | **Done** (#403). K7, 60 diagnostics over 7 files (55 of them the two generic recovery messages) — the KerML twin of S2/F61. Keyword-less feature members: any at namespace level (`a : Integer;`, `y = x as T;`, `x;`), and in a body one whose declaration specialises without typing (`composite e1 redefines V::m;`), one whose multiplicity precedes the typing (`p5[1] : Real;`), or one prefixed `var`/`const` (`var p9 : Real;`). `Feature`'s keyword-less alternative is `( EndFeaturePrefix \| BasicFeaturePrefix ) FeatureDeclaration` (`KerML.xtext:542`), `BasicFeaturePrefix` carries `var`/`const` (`:515`), `FeatureSpecializationPart` allows a multiplicity first (`:574`) and `NamespaceFeatureMember` (`:158`) needs no keyword. Retires `Simple Tests/Expressions.kerml` (33) and `Vehicle Example/VehicleUsages.kerml` (14) between them. |
| ~~F85~~ | **Done** (#403). K8, 17 diagnostics, all of `Simple Tests/Types.kerml` bar its four relationship-keyword lines. `type` is not parsed in any form the file writes it: `Type` is `TypePrefix 'type' TypeDeclaration TypeBody` (`KerML.xtext:319`) and `TypeDeclaration` (`:324`) carries `all`, an `OwnedMultiplicity`, a mandatory `SpecializationPart \| ConjugationPart` and `TypeRelationshipPart*`. Note the specialization is *not* optional — `type A;` is rejected by the reference too. |
| ~~F86~~ | **Done** (#403), with the relationship member represented approximately — the keyword-first form is parsed onto the same relationship nodes the punctuation form uses. K9, 19 diagnostics over 5 files. A relationship written as a member with its keyword first, ten spellings: `specialization`/`subtype` (`:390`), `subclassifier` (`:486`), `typing` (`:665`), `subset` (`:683`), `redefinition` (`:712`), `conjugation` (`:408`), `inverse`/`inverting` (`:634`) and `featuring` (`:652`), all `NonFeatureElement` alternatives. The `redefinition` rows' minimal reproducers draw *semantic* pilot errors rather than silence, so those three (`Simple Tests/Features.kerml:68,69,71`) need a corpus-faithful fixture before a fix is validated. |
| ~~F87~~ | **Done** (#403). K10, 4 diagnostics (`Simple Tests/Features.kerml:8,11`). `typed by` is the long spelling of `:` in the same production — `TypedBy` is `( ':' \| 'typed' 'by' ) …` (`KerML.xtext:600`) — and only the punctuation is implemented. |
| ~~F88~~ | **Done** (#403). K11, 6 diagnostics (`Named Collection Members Example/VehicleTanks.kerml:28,31`, `Simple Tests/FeatureChains.kerml:18`). A connector end that is a feature chain: `ConnectorEnd` ends in `OwnedReferenceSubsetting` (`KerML.xtext:854`) and that is `referencedFeature \| OwnedFeatureChain` (`:699`). A plain first end with a dotted second end already works, so the gap is the first end's parse. |
| ~~F89~~ | **Done** (#403). K12, 5 diagnostics (`Simple Tests/Connectors.kerml:16,20,24`). `BindingConnectorDeclaration` (`KerML.xtext:875`) and `SuccessionDeclaration` (`:891`) make the name, the declaration and the `of`/`=` ends all optional, so `binding { … }` with member ends and `binding ab1 : AS of a = b;` are both legal; we demand a name and reject a typing before `of`. |
| ~~F90~~ | **Done**: parser #403, downstream #409, which scoped the conjugated-port-typing rule to SysML typings — a KerML conjugation relates any two Types and demands no port — retiring all seven diagnostics; re-verified on `main`, the three `.kerml` files are clean and draw no only-ours row in `cmd/pilot-diff`. The declarations parse, and the seven diagnostics they reached were `passes/typecheck.go`'s conjugation check firing where the reference is silent: `'~' names the conjugated port definition of a port definition, found kermlType` on `Simple Tests/Conjugation.kerml:6` and `Types.kerml:25,26,28,29` (5 `kind-mismatch`) and `… found attributeUsage` on `Conjugation.kerml:8` and `Features.kerml:36` (2 `unmapped`). In KerML a conjugation relates any two Types, so the check must not require a port definition on a `.kerml` document — that is `passes/` work #403 did not own. K13, 6 diagnostics (`Simple Tests/Conjugation.kerml:6,8`, `Features.kerml:36`). Conjugation in a declaration: `ClassifierConjugationPart` in `ClassifierDeclaration` (`KerML.xtext:468`) and `FeatureConjugationPart` = `( '~' \| 'conjugates' ) …` (`:730`). |
| ~~F91~~ | **Done** (#403). K14, 2 diagnostics (`Simple Tests/Associations.kerml:16,17`). `EndFeaturePrefix` is `( isConstant ?= 'const' )? isEnd ?= 'end'` (`KerML.xtext:511`); `end feature b;` parses, `const end feature b;` does not. |
| ~~F92~~ | **Done** (#403). K15, 2 diagnostics (`Simple Tests/Comments.kerml:25,43`). Two annotating-element gaps: `Comment`'s whole head is optional so `locale "en_US" /* … */` is an anonymous comment (`KerML.xtext:94`), and `Documentation` takes an `Identification` so `doc <a> /* … */` is legal (`:103`). |
| ~~F93~~ | **Done at every tier** (parser #403, downstream wave 7A), and re-verified rather than re-fixed: on a fresh `cmd/pilot-diff` run `Simple Tests/Filtering.kerml` is fully agreeing, `kerml-examples` only-ours is 3 and all of it the one-sided specialization-cycle check, and the two `testdata/passes/f93_element_filter.{kerml,sysml}` fixtures analyse clean under `internal/core/passes/f93_element_filter_scope_test.go`. The repeated brackets conjoin into one `and` in `parser/namespace.go`, so both conditions reach the filter judge (`parser/f84_f95_kerml_declarations_test.go`, case `f93_two_filters`). K16, 2 diagnostics (`Simple Tests/Filtering.kerml:35`). `FilterPackage` is `FilterPackageImport ( FilterPackageMember )+` (`KerML.xtext:200`); we accept exactly one filter bracket where the grammar says one or more. |
| ~~F94~~ | **Done** (#403). K17, 1 diagnostic (`Simple Tests/MetadataTest.kerml:33`). `Feature` is `FeaturePrefix ( 'feature' \| ownedRelationship += PrefixMetadataMember ) FeatureDeclaration?` (`KerML.xtext:538`) — the annotation replaces the keyword, so `abstract #Classified z2;` is a feature. The KerML twin of S1/F60. |
| ~~F95~~ | **Done** (#403). K18, 1 diagnostic (`Simple Tests/Expressions.kerml:23`). A *named* `expr` whose body is a brace-enclosed expression (`in expr whileTest {v > 3}`); F30 unreserved `expr at`/`expr while` but the named form with an expression body is still unparsed. |
| ~~F34~~ | **Done** (#358). Compares our own 11 `.kerml` fixtures (`testdata/lex/basic.kerml`, `examples/parser_features_demo_*.kerml`) too: a root carries one language today, so they are collected as SysML and excluded (see the known limitation above). Needs per-file language dispatch within a root, and a second pilot invocation per root. |
| ~~F96~~ | **Done** (#373, whose title mislabels it "F84" — F84 is K7). Our own 10 `.kerml` demo fixtures carried SysML notation under a `.kerml` suffix, which is why F34 drew 314 pilot diagnostics on them. Two were SysML demos and were renamed (`parser_features_demo_connectors`, `parser_features_demo_messages_events`); the other eight were corrected to notation the KerML grammar accepts. The `examples` root now has **no** `.kerml` pilot diagnostic. F97/F98 were the two further fixture gaps #358 proposed under the same renumbering. |
| ~~F99~~ | **Done** (#387), in `internal/core/symbols/` and the evaluator: a declaration in an expression body is in scope for the body's result expression, with shadowing preserved, and the runtime evaluates it instead of returning `ErrUnsupportedBodyDeclaration`. `Analysis Examples/Vehicle Analysis Demo.sysml` 6 → 0 and `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml` 2 → 1. A declaration inside an expression body parses (#375, F64b) but its name is not visible to the body's result expression: `Analysis Examples/Vehicle Analysis Demo.sysml:214-218` now reports 6 `unresolved reference: nextSample`/`thisSample` diagnostics over 5 report lines (line 214 carries `x2`) where it previously reported 2 syntax errors, and the evaluator returns the typed `ErrUnsupportedBodyDeclaration` rather than a wrong result. The scope member is `internal/core/symbols/` work, which #375 did not own. Strictly an unmasking: the reference is silent on the file, and the diagnostic count rose because parsing advanced. |
| ~~F100~~ | **Done** (#388). It was a false positive of ours, as suspected: a redefinition target that is separately `featured by` need not be an inherited member of the immediately enclosing feature, and the check now falls back to the KerML accessibility rule cited in `internal/core/passes/`. Our `unmapped` total drops 16 → 15; the other two wave-3 unmaskings (F69's `Rationale` name conflicts) are a deliberate one-sided check and stay. `member feature isLicensed1 :>> Person1_::isLicensed featured by …` (`Variable Feature Examples/TimeVaryingCarDriver.kerml:93`, 1 `unmapped`) draws `isLicensed1 redefines isLicensed, but isLicensed is not an inherited member of driver`; the reference is silent. The line only parses now that F50 (#374) accepts `member abstract feature`, so this is unmasked, not new — but unlike F99 it looks like a **false positive of ours**: the redefinition target is *qualified* (`Person1_::isLicensed`) and separately `featured by`, so requiring it to be an inherited member of the immediately enclosing feature is too strict. Confirm against the reference on a reduced fixture before widening the check. |
| ~~F101~~ | **Done** (wave 10F). Re-measured on `main` before the fix, **2** of the 12 remained — both `no scope for member lookup in Actions::TransitionAction::effect`, on `ServerSequenceOutsideRealization-2.sysml:91` and `ServerSequenceRealization-2.sysml:96`; the `accepter`, `acceptedMessage` and `PartTest.sysml:25` `receiver` rows had already been retired by waves 7–9, so the count below is stale rather than wrong when written. The cause was not implicit typing: `kindBaseFQN` already gives `*ast.TransitionMember` the `Actions::TransitionAction` base, and the chain resolved to the library's abstract `effect` feature, which comes back from the library cache without a scope. A transition's own effect action *is* the `effect` it redefines (SysML v2 §7.19.2), so `symbols/builder.go` names it in the transition's scope and the chain reads that action; a `send` written as an action node is a SendActionUsage in its own right, so `semantics/model.go` types it by `Actions::SendAction` whether or not a usage declares it, which is what supplies `sentMessage`. Both files' rows go to 0 and `ServerSequenceRealization-2.sysml` becomes fully agreeing. **Unmasking:** with the name-resolution error gone the constraint tier now runs on `ServerSequenceOutsideRealization-2.sysml` and surfaces 3 `Must be a valid feature` rows on `:>> incomingTransferSort = Occurrences::earlierFirstIncomingTransferSort` (lines 18, 32, 57) — a KerML `bool` expression *is* a feature, so these are false positives of `passes/w8c_feature_reference.go`, they reproduce on `main` when nothing masks the tier, and they are handed to slice 10B with F69. Net on this oracle: fully agreeing 309 → 310, only ours 119 → 120. As handed back in wave 4: **12** of F68's 39 diagnostics remained, all implicit `Actions::TransitionAction` members. 11 are in the two files #391 measured — `ServerSequenceOutsideRealization-2.sysml` 10 → 4 and `ServerSequenceRealization-2.sysml` 13 → 7 — and name `accepter` (6), `effect` (3) and `acceptedMessage` (2); the twelfth is `Simple Tests/PartTest.sysml:25`'s `unresolved member: receiver`, unchanged at 1. `semantics/implicit.go` `kindBaseFQN` must give `*ast.TransitionMember` the base its usage kind already has — a layer #391 did not own. **Residue closed:** the two `effect` cases needed a cached library symbol to carry a scope, and fact-only library records (a library document is parsed on every load path) leave every restored symbol its declaration and scope, so both files are clean and the chain resolves identically cold and warm (`model/w8g_f68_effect_member_test.go`). |
| ~~F102~~ | **Done** (#428), re-verified on `main`: all three forms parse clean, pinned by the `w7c_f66_generalized_usage_declarations` golden fixture and the two neighbouring negative cases. Wave-4 handback from F66 (#375): `verify r :>> massRequirement;`, `variant use case uc11;` and `ref redefines cylinderBR[4];` stay rejected. All three are the generalized usage-declaration path in `parser/defusage.go` `parseUsage`, which #375 did not own; #383 owned that file but scoped itself to F65's three forms. |
| ~~F103~~ | **Done** (#428), re-verified on `main` and pinned by the `assert_not_named_constraint` golden fixture: `not` before a named constraint negates the asserted expression instead of modifying a declaration, and `Simple Tests/ConstraintTest.sysml` is clean. Wave-4 handback from F64 (#375): `assert not c { … }` stays rejected. `parser/defusage.go` `parseDefUsage` treats `not` as a negation only when a *kind keyword* follows it, so a named constraint after `not` is still read as a declaration where the grammar makes the argument an `OperatorExpression`. |
| ~~F104~~ | **Done** (#464), then superseded: the spelling was removed outright — an expression right end is now a parse error and the warning is gone, its one occurrence rewritten as the declaration's value (`out result : Real = x * 2.0;`). Feature-reference bindings remain silent. |
| ~~F105~~ | **Done.** Named `done`, `then <source> <target>;` and the member-leading `<source> then <target>;` form are nonstandard-notation findings, while the pilot-accepted one-ended `then <target>;` stays silent. |
| ~~F106~~ | **Done** (#464). A one-ended `first <node>;` is diagnosed outside an action body and remains silent inside one. |
| ~~F107~~ | **Done** (#464). Requirement constraints outside requirement-style bodies are diagnosed without changing the legal requirement-body form. |
| ~~F108~~ | **Done** (#467). Concrete connection definitions require two related elements, and `FuelLine` now declares both ends. |
| ~~F109~~ | **Done** (#467). A non-Boolean element filter reports `Must have a Boolean result` independently of model-level evaluability. |
| ~~F110~~ | **Done.** P4 re-derived from inputs both implementations parse identically: all 15 remaining `Duplicate of other owned member name` diagnostics are recovery artifacts (0 only-ours rows, 2 agreement rows), the rule agrees on every ordinary member, and the one real divergence the class hid — a simple state member and a named transition were skipped by our distinguishability check, because those declarations recorded no span for the name they declare — is fixed at the root and pinned by `passes/f110_state_duplicate_names_test.go`. Aggregate pilot-diff counts are unmoved: no corpus file has clean duplicate state or transition names. |

F6 is done, and it is the case for testing harness assumptions rather than reasoning about them:
it changed nothing about what either implementation says, but it turned 25 diagnostics that this
page dismissed as wrapper noise into a reference rule we do not implement.

After wave 5 the adjudicated syntax debt is **empty on the KerML side**: F84–F95 (K7–K18) all
landed in #403 and `kerml-examples` carries no syntax diagnostic at all, moving to 47 of 58 files
fully agreeing with 15 diagnostics of ours — against `pilot-examples` at 76 of 98 and
`pilot-validation` at 52 of 56. One of the twelve is closed in the parser and open downstream,
in a package #403 did not own: **F93** (3, the element-filter false positives in `resolve/`);
F90's downstream half was closed by #409. F93, F101–F103 and the SysML row F67 were the open
follow-ups then; each is closed above.

What the numbers support today changed with this round: the KerML notation the reference's own
corpus uses now parses in full, so the parsing claim is no longer SysML-only. It remains a claim
about parsing and static checking on these corpora, not about behavioral conformance.

---

## Re-running and diffing

```sh
./scripts/download-training-examples.sh   # the OMG training corpus (pinned 2026-07)
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

`-update` records a run as the committed baseline; `-check` re-runs the comparison and fails
unless the fresh report reproduces it, printing the differing fields. Both flags exist on all
three oracles, and `-check` is what a reader should run before quoting a figure.

### How this record is kept true

A baseline states what one run measured, so three mechanisms keep it from quietly ceasing to
describe this repository:

- **Provenance in every baseline.** `provenance` records the pinned tag and artifact, a digest of
  each validator bridge's source, and a digest and file count of every corpus root the run
  compared, alongside the ISO date it was recorded. No absolute path is an identity, so two
  machines that agree on the pin and the inputs record the same provenance.
- **A Java-free guard in the normal suite.** `TestCommittedBaselineStatesThisRepositorysProvenance`
  (in each oracle's package) compares that record against the repository as it stands. If the pin
  moves, a bridge is edited or a corpus this repository owns changes and a baseline is not
  re-recorded, it fails naming the field, the recorded value, the current value and the exact
  refresh command. It reads only committed files.
- **A scheduled Java-backed reproduction.** `.github/workflows/oracle-reproduction.yml` installs
  Java, provisions the pinned validators and corpora and runs all three oracles with `-check`
  daily and on demand. Its failure distinguishes a moved provenance — the pinned reference or an
  input changed underneath the baseline, so investigate the provisioning — from moved counts with
  matching provenance, which is an implementation movement to adjudicate and then re-record.

The Java-free guard cannot see a movement in the reference's own behaviour, and the scheduled run
is not a required check, because both the corpora and the validator are unvendored network
fetches. Together they bound how long a stale figure can survive to about a day.

---

## Current branch movement and adjudications

The settled control is a clean run of `466de743cbd46eaa6983fd8cf0cffc4097a2137f`,
after the merged wave-11 changes and before the remaining wave-11F changes. The
branch movement is measured from
`build/pilot-diff/pilot-diff.json`, keyed by corpus root, file, diagnostic line,
severity and category:

| Measurement | Control | Branch |
|---|---:|---:|
| Fully agreeing files | 313 | **317** |
| Agreed diagnostics | 25 | **25** |
| Only ours | 138 | **119** |
| Only the pilot | 73 | **73** |

The branch retires 13 only-ours table entries representing 19 diagnostics; no
new differential rows are introduced. Two entries account for four diagnostics
each, so the table-entry count and diagnostic count are intentionally different:

| File and line | Cause |
|---|---|
| `pilot-examples/Simple Tests/DecisionTest.sysml:17,18` | Objective cardinality and state/transition endpoint handling now match the normative case and state rules. |
| `pilot-examples/Simple Tests/StateTest.sysml:24` | Inherited state-action endpoint resolution now recognizes the legal vertex. |
| `pilot-examples/Vehicle Example/Annex_A_VehicleViews.sysml:472` | Same-feature inherited resolution now reaches the intended inherited declaration. |
| `pilot-examples/Vehicle Example/Annex_A_VehicleViews.sysml:686` (4 diagnostics) | Same-feature inherited-member canonicalization removes the false duplicate family. |
| `pilot-examples/Vehicle Example/Annex_A_VehicleViews.sysml:712` (4 diagnostics) | Same-feature inherited-member canonicalization removes the second false duplicate family. |
| `pilot-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml:85,600,647,664,670` | Inherited and redefined-name resolution now reaches the intended declarations. |
| `pilot-validation/05-State-based Behavior/5-State-based Behavior-1.sysml:136` | Legal inherited state endpoint handling. |
| `pilot-validation/05-State-based Behavior/5-State-based Behavior-1a.sysml:137` | Legal inherited state endpoint handling. |

The verified Xpect set also includes
`simpletests/DecisionTest.sysml.xt:52` among the recovered rows checked on the
branch.

The three rows at
`pilot-examples/Interaction Sequencing Examples/ServerSequenceOutsideRealization-2.sysml:18,32,57`
are explicitly excluded from this branch's claim. The merged wave-11 PRs
closed those `kind-mismatch` rows before `ae4fdf9e`; they explain the stale
`311 / 142 / 73` documentation-era comparison and are not movement produced by
wave 11F.

### Remaining Xpect adjudications

The following fourteen rows are settled adjudications rather than unclassified
disagreements.

* **Query syntax the pinned pilot does not parse:** `queryx/failing/QPE-Qualifier`,
  `QPE-Traversal`, and `QPE-Wildcard` declare file-wide silence, but the pinned
  pilot's own validator rejects them too (`no viable alternative at input '/'`),
  which is what `queryx/failing/` records. Wave 12D reclassifies these as a
  **pilot limitation**. See [wave12d-decisions.md](wave12d-decisions.md).
* **Parallel state syntax:** `TransitionUsage_invalid.sysml:45, 54, 68` closed in
  wave 12D, which parses `state … parallel { … }` and retains it in the AST so
  “A parallel state cannot have successions or transitions” and the
  accepter-source rule can be evaluated. Line 60 now reports
  `transition guard must be Boolean, found String` on the declared model line;
  its expression-only span keeps the Xpect row in `same-line`. See
  [wave12d-decisions.md](wave12d-decisions.md).
* **Fixture environment:** `Feature_invalid_noType.sysml:18,20` has no library
  resource in `XPECT_SETUP`. The pilot consequently lacks `Parts::Part`, so the
  implicit specialization has nothing to specialize and the feature has no
  implicit type. OpenSysML always bundles the standard library, making this
  fixture unsatisfiable for OpenSysML by construction. This is an environment
  difference, not a specification divergence.
* **Import recovery:** `Import_Visibility_Invalid.sysml:23,25` contains the
  pilot's ANTLR recovery texts `mismatched input 'import' expecting '}'` and
  `extraneous input '}' expecting EOF`. OpenSysML reports the
  specification-grounded error at the same location: SysML v2 requires a
  visibility indicator before `import`; the pilot's second error is a cascade
  of its first. This is a justified divergence, deliberately not a
  `wording-only` pair: that class is reserved for registered equivalent
  phrasing, and adding this recovery pair would move the metric cosmetically.
* **General interface end count:** `InterfaceUsage_Invalid.sysml:49` expects
  `Cannot have more than two ends` from the pilot. The normative library's
  `Interfaces::Interface::participant` has multiplicity `[2..*]`; exactly two
  ends is a property of `BinaryInterface`, not `Interface`. SysML v2 §7.14.1
  permits three or more ends on a general interface, while §7.14.2 and
  §8.3.14.2 constrain the binary subtype. Step 3 therefore classifies the
  universal expectation as a **pilot limitation**; OpenSysML's independent
  port-typed-end error remains at the same location.
* **Interface implicit Port base:** `InterfaceUsage_Invalid.sysml:78` expects
  `Duplicate of inherited member name 'self' from Part, Port`. Step 3 closes
  the row by giving exactly two-ended interfaces the normative
  `Interfaces::BinaryInterface` base; the existing positional implicit
  redefinition then supplies the matching inherited port end. Three-ended
  general interfaces and ordinary binary connections remain silent.
* **Assignment action time variance:** `AssignmentActionUsage_invalid.sysml:44`
  expects `Referent must be time varying`. Step 3 closes it with SysML v2
  §8.3.17.5 over the referent feature's derived `mayTimeVary` property
  (§8.3.6.4), in an element-scoped constraint pass so an unrelated lower-tier
  error does not mask the assignment diagnostic.

## Element-scoped tier gating (roadmap L2)

Narrowing the tier gate from the document to the element moves this oracle only: agreement 25 →
**32**, only-pilot 73 → **66**, our diagnostics 168 → **175**, only-ours unmoved at **119**, and the
Xpect and rejection oracles are byte-identical. The design, the control run with the gate removed
entirely (only-ours 119 → **166**), and the rows the gate turned out *not* to be hiding are in
[wave 12A](wave12a-element-gating.md).

The seven rows that became agreement are all on `testdata`, and six of them are now word-for-word:

| File | Line | Ours | The pilot's | Reading |
|---|---|---|---|---|
| `parse/expressions.sysml` | 2 | `Must have a Boolean result` | `Must have a Boolean result` | agreed, same rule |
| `parse/expressions.sysml` | 3, 5, 6 | `Must have a Boolean result` | `Must have a Boolean result` | agreed, same rule |
| `parse/expressions.sysml` | 4 | `Must be model-level evaluable` | `Must invoke a behavior or a behavioral feature` | agreed by category; **open wording divergence** |
| `passes/errors.sysml` | 3 | `Must have a Boolean result` | `Must have a Boolean result` | agreed, same rule |
| `resolve/errors.sysml` | 3 | `Must have a Boolean result` | `Must have a Boolean result` | agreed, same rule |

The five rows aligned here now agree on the exact rule, element, and wording. The remaining
`parse/expressions.sysml` line 4 row is counted as agreement by the harness's
`(line, severity, category)` matching, but its wording divergence remains open: our diagnostic
requires a model-level-evaluable condition, while the pilot requires an invocation of a behavior or
behavioral feature.

One category mapping moved with this and no count did on the base tree: `must have` now maps to
`kind-mismatch` on our side as it already did on the pilot's (`cmd/pilot-diff/category.go`), so our
own `Must have a Boolean result` can agree with the pilot's identical string instead of sitting in
`unmapped`. No diagnostic of ours in the corpus carried that wording before wave 12A.


## The remaining only-ours rows

The only-ours column is **27** as published and **26** with the declared errata applied, and every
row in it is adjudicated. Three quarters of them are not candidate false positives at all: 7 are our
own non-standard-notation warnings on our own demo models (`solver-demo.sysml`, 6 `require` outside a
requirement body, and `pseudostates-demo.sysml`, 1 `junction`), 3 are our own fixtures under
`testdata/passes/`, and 6+4+3 are the one-sided specialization-cycle family — the committed probes,
`Simple Tests/PartTest.sysml:49,50,51,53` and `Simple Tests/Circular.kerml:9,10,11` — whose
adjudication is [above](#specialization-cycles-f4). That leaves the reference's own corpora carrying
four rows, all four of them defects in the **published model text** rather than in either
implementation, and each is an entry of the declared errata overlay:

| Row | Reduced reproducer | Clause | Verdict |
|---|---|---|---|
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml:38`, `operator '+' combines incommensurable quantities` | `attribute radius = 22/2*25.4 + 110 [mm];` against `ISQ`/`SI`, with nothing else in the file | the bracket postfix binds to `PrimaryExpression` (`KerMLExpressions.xtext:308`), below `AdditiveExpression`; **SysML v2 §9.8.9.1** requires the operands of `+` to share a quantity dimension | **the corpus text is wrong.** `[mm]` qualifies `110` alone, so the addition mixes a dimensionless value with a length. Parenthesising the addition clears our warning; the pinned validator is silent on both texts. Retained, with a correction declared in the overlay |
| `Analysis Examples/Turbojet Stage Analysis.sysml:25`, same message | `attribute t : TemperatureValue; attribute v : VolumeValue; attribute s = 1/(2*c) * v^2 + t;` with `c : DimensionOneValue` | **SysML v2 §9.8.9.1** | **the corpus text is wrong.** `V : VolumeValue` makes `V^2` L^6 against `T_static`'s Θ. No intended reading can be inferred — the physics wants a speed, the model never says so — so the overlay documents it **without** a correction and the row stays in the census |
| `Analysis Examples/Dynamics.sysml:13`, `cannot bind a value of dimension L^4·M^2·T^-5 to a feature typed by AccelerationValue (dimension L·T^-2)` | `calc def A { in dt : TimeValue; in tp : PowerValue; return a : AccelerationValue = tp * dt * tp; }` against `ISQ` | **KerML 7.4.9**: the expression is the return feature's value, so it answers to that feature's type, whose dimension the imported ISQ definitions fix (`AccelerationUnit` L^1·T^-2 against `PowerUnit` L^2·M^1·T^-3 and `DurationUnit` T^1) | **the corpus text is wrong.** A power squared times a duration is L^4·M^2·T^-5, incommensurable with an acceleration. No intended reading can be inferred — the calculation declares no speed, and its unused `tm` cannot repair the exponents alone — so the overlay documents it **without** a correction and the row stays in the census. The pinned validator performs no dimensional analysis and is silent |
| `Individuals Examples/AnalysisIndividualExample.sysml:86`, `fuelConsumption (typed by FuelEconomyAnalysis_1) redefines fuelConsumption (typed by FuelConsumption): types do not conform` | an analysis definition holding `action a : A;`, an individual definition of it, and an individual usage whose `:>> a` is typed by that individual *analysis* definition rather than by an individual `A` | **KerML 7.4.9, 8.3.4.2**: a redefinition is a subsetting, so the redefining feature's type must conform to the redefined one's | **the corpus text is wrong**, on the same reading the [Xpect adjudication](pilot-xpect.md) already gives this rule's sibling rows. The file itself declares `individual action def FuelConsumption_1 :> FuelConsumption` and never uses it: that is the conforming type line 86 meant to name. Our error stays — the pilot validates subsetting conformance nowhere — and the overlay declares the substitution |

The fourth row that stood here, `Vehicle Example/VehicleDefinitions.sysml:47`
(`interface Mounting connects ports AxleMountIF and WheelHubIF, whose directed features are not
conjugate`), was **our defect** and is fixed rather than adjudicated. The reduced reproducer is the
corpus shape with everything else removed:

```sysml
port def Source { out item sent; }
port def Target { in item received; }
interface def Link {
	end a : Source;
	end b : Target;
	flow a.sent to b.received;
}
```

Conjugation pairs an interface's two ports' directed features (**SysML v2 §7.12.2**, §8.2.2.14), and
our check paired them **by name only**, so two ports whose features are named differently could never
match and drew the warning even where the interface's own `flow` states the pairing outright. It now
reads those flows first (`semantics.Model.interfaceFlowPairedFeatures`): a feature is exempt from the
name match when a flow of the interface pairs it with a feature of the *other* end whose direction is
complementary and whose type conforms. A flow with like directions, a flow naming only one end, and an
interface with no flow all still warn, which `TestConstraintInterfaceFlowPairsDirectedFeatures` and
`TestW8GInterfaceConjugationStaysAWarning` hold in place — the check keeps its scope, its severity and
its message, and the pilot remains silent on the whole family.

### The baseline this replaces

The committed baseline recorded **82** only-pilot and **123** pilot diagnostics where a clean run
measures **61** and **101**, with only-ours unmoved at 27 before this round. That gap is neither
movement nor an environment difference in the pilot validator: it is a corpus change on `main`. The
demo `examples/action-executor-demo.sysml` used to bind a computed result with `bind result = x * 2.0;`,
which the pinned validator rejects as a syntax error and then cascades semantic errors from; it now
declares `out result : Real = x * 2.0;`, which the validator accepts apart from three unrelated
duplicate-inherited-member warnings. The 21 vanished only-pilot rows are that file's, on the `examples`
root alone (only-pilot 56 → **35**, pilot diagnostics 64 → **42**), and the baseline was simply
recorded before that edit. Both are refreshed here from one clean run.

---

## The only-pilot column, adjudicated

An **only-pilot** row is a diagnostic the reference reports and we do not, so the column measures our
permissiveness. This round re-measures it from a cleared cache, closes the one genuinely
spec-derivable rule in it, and records a verdict for every remaining row so the column is a decision
list rather than raw output. The 82 → 61 movement an independent clean run showed against the older
baseline is the corpus change described in [The baseline this replaces](#the-baseline-this-replaces):
a measurement correction with no rule movement in it, confirmed here from a fresh `XDG_CACHE_HOME`
that is byte-identical to the ambient-cache run against the unchanged pinned artifact.

### Owned names against library-inherited members

**Verdict: a real gap of ours, implemented.** The reference's name-distinguishability rule compares a
type's owned member names against everything it inherits (KerML 8.2.4 with 8.4.3.2), including from
**library** supertypes. Our resolver-tier rule deliberately walks only the document's own supertypes,
and the pass beside it that does read library bases only reported a name two of them each supply — a
diamond — so a member colliding with the *one* base its declaration implies went unreported. Reduced
reproducer, which the pinned `validate-sysml` and `bin/sysml -validate` now answer identically at the
same line and column:

```sysml
package Q { part def Q { attribute portions; } }   // 'portions' is Occurrence::portions
package S { state S { state start; } }             // 'start' is StatePerformances::StateAction::start
```

The pass now checks owned and alias member names against the library bases as well as base-against-base,
with the exemptions the rule itself implies: a member that redefines or subsets the inherited feature
*is* that feature and is silent; a member whose declared redefinition target does not resolve is not
evidence of a duplicate; behavior parameters and the subject, actors, stakeholders and objective of a
case or requirement are implicit redefinitions; and the assignments in a metadata usage body are owned
redefinitions of the metadata definition's features (`MetadataBodyUsage` in the pinned grammar), not
second members of those names. Fixtures: `testdata/passes/inherited_name_library_base.sysml` (positive,
two warnings) and `..._clean.sysml` (negative, silent on both sides).

Movement, against the clean-cache run this branch merges (the census `main` records):

| Count | Control | This round |
|---|---:|---:|
| Files | 353 | 355 |
| Fully agreeing | 325 | 328 |
| Agreed | 32 | 37 |
| Only ours | 26 | 27 |
| Only the pilot's | 61 | 58 |

The two extra files are the two new fixtures, which both implementations agree on; they contribute the
two new agreed diagnostics on `testdata`. Three previously only-pilot rows became agreements —
`orthogonal-regions-demo.sysml`:12,26 and `pseudostates-demo.sysml`:10, each a `state start;` inside a
state. The one **new** only-ours row is `pseudostates-demo.sysml`:28, the same `state start;` one
region further down; the reference is silent there only because its grammar rejects `junction` and the
bare `entry;` earlier in that file and its recovery never reaches the declaration. It is a defended
true positive: the identical construct at line 10 is now an agreement.

### Adjudicated and deliberately left

| Rows | Where | Verdict |
|---|---|---|
| 6 `The opposite features 'owningType' of '…DisjoiningImpl{…}'` / `'ownedDisjoining' of '…'` | `kerml-examples`: `Types.kerml`:31, `Features.kerml`:20, `Inverses.kerml`:3, `FeatureChains.kerml`:31, `Classifiers.kerml`:13, `A-2-ModelingInstances.kerml`:9 | **Not spec-derivable.** The messages name EMF implementation classes and resource fragments and assert an opposite-reference invariant of the reference's own metamodel; KerML 1.1 states no rule a modeller could act on, and the files are valid. Left, documented. |
| 4 `Duplicate of inherited member name 'done' from Action` | `action-executor-demo.sysml`:16,35,55, `views-demo.sysml`:96 | **Ours is right, re-verified.** These are `done;` on its own, which we read as the anonymous final node of an action body; the pinned `SysML.xtext` contains neither `done` nor a final-node production, so the reference reads it as a reference usage declaring a member named `done`, which then duplicates `Actions::Action::done`. The rule's *scope* is not the gap it looks like: matched runs of `validate-sysml-batch` and `bin/sysml -validate` are byte-identical on `action def Sub :> MyAct { action done; }` (both `9:35 warning: Duplicate of inherited member name 'done' from Action`), on the same collision two user supertypes below a library base (`part def Leaf :> Mid { part portions; }`, both `6:30`), and on the redefinition escape hatch (`part :>> portions;`, both silent) — so a member inherited from a library type already conflicts exactly as one inherited from a user type does. What differs is only the spelling: `then done;`, the form the OMG corpora use, is silent on both sides, and a bare `done;` appears in no OMG-authored model. Left as a notation difference, recorded in the grammar conformance audit. |
| 9 `Bound features should have conforming types`, 1 `An attribute must be typed by attribute definitions.`, 1 `An occurrence, item or part must be typed by occurrence definitions.` | `parse/expressions.sysml`:3-6, `passes/errors.sysml`:3, `resolve/errors.sysml`:3, `solver-demo.sysml`:120,124, `lex/basic.sysml`:4, `passes/import_no_visibility.sysml`:9 | **Not a rule gap: a tier boundary.** All of them sit downstream of a name-resolution or syntax error in the same file, and the rule each one would need is already implemented and fires on a valid reduced model — `bind n = w;` between an `Integer` and a `Wheel`-typed attribute draws `Bound features should have conforming types` from both implementations. Reporting them too would mean running the type tier over subjects whose types are unknown, which the tier contract forbids. The typing-kind pair is now pinned by reproducer rather than by argument: `lex/basic.sysml` unchanged draws the reference's `Couldn't resolve reference to Type 'Real'` **and** its typing-kind error while we report the unresolved reference alone, and with `private import ScalarValues::*;` added both implementations fall silent. On a model whose types resolve the whole family agrees message-for-message and column-for-column — `attribute`, `part`, `item`, `port`, `action`, `state`, `connection` and `interface` usages each typed by `attribute def A` draw the eight reference wordings at identical spans from both. The second row sits in `passes/import_no_visibility.sysml`, one of the files the reference cannot parse, not in `resolve/errors.sysml`, which in isolation draws no typing-kind row from either implementation. |
| 1 `Must be model-level evaluable` | `parse/expressions.sysml`:4 | **Not a rule gap: the reference reports two type-tier errors on that line and we report one.** We report `Must be model-level evaluable` there too, at the same column and in the same words, and the rule agrees in all three directions a reduced model can test: an unresolved invocation (`filter coll->select(x);`) draws it from both, a resolvable but inevaluable one over a user `calc` draws it from both, and `filter 1 + 2 > 0;` is silent in both. Until this round the row was also **miscategorized**: `categorizePilot` left the message `unmapped` while `categorizeOpenSysML` mapped our identical text to `kind-mismatch`, so the two copies could not pair at all. The pilot side now applies the same `must be` clause ours does; our diagnostic pairs with one of the reference's two errors on the line and the surplus one stays, so the count does not move — what changes is that the surplus is now read as a second copy of a rule we agree on rather than as an unmapped divergence. |
| 5 `Duplicate of other owned member name` | `passes/import_no_visibility.sysml`:3,8,12, `semantic-layer/demo.sysml`:35,105 | **Recovery collateral, not a gap.** The fixture is *about* imports without a visibility keyword, which the pilot's grammar rejects (`no viable alternative at input '::'` on the same lines), and its recovery re-registers the fragments as duplicate members. We implement the owned-name rule, including short names. Left. |
| 7 `Couldn't resolve reference to …` (`b`, `c`, `sciencePower`, `drivePower`, `ignite`, `start`, `touchdown`) | `parse/expressions.sysml`:3, `solver-demo.sysml`:120,124, `views-demo.sysml`:88,90,108, `pseudostates-demo.sysml`:17 | **Split, both defensible, no code change.** Three are later segments of a chain whose head we already reported unresolved (`a.b.c`), where repeating the failure per segment adds nothing; four name action or state vertices in files the reference cannot parse past, so the names are missing from *its* model rather than invented by ours. Left. |
| 11 syntax errors (`no viable alternative at input 'entry'` / `'evaluate'` / `'if'` / `'then'`, `missing '}' at 'action'`, `mismatched input 'transition'`, `missing EOF`) | `phase-c-behavioral-bodies.sysml`:175,176, `pseudostates-demo.sysml`:12,18,19, `views-demo.sysml`:106,107,109 | **The reference failing to parse notation of ours.** These trace to retained extensions — `choice`/`junction` pseudostates, `entry;` as a bare entry marker, the inline `if`/`else` action form — for which the pinned grammar has no production. Not gaps of ours; we already warn on the non-standard ones under the conformance modes. Left. |
| 5 `Must be an accessible feature (use dot notation for nesting)` | `semantic-layer/demo.sysml`:44,45,46,50,51 | **Recovery collateral, not a gap** — the reduced model the previous round asked for now exists. All five references (`MathConstants::pi`, `::e`, `::Derived::twoPi`, and the two expression forms) transcribed into a file that declares `MathConstants` as a `package` are silent in both implementations; changing that one keyword to `namespace`, which the SysML grammar has no production for, makes the reference report `no viable alternative at input` on each namespace **and** exactly these five accessibility errors, at the same relative positions and in the same order as the file. Its recovery turns the unparsed namespace into a feature, so each qualified reference becomes a subsetting whose subsetted feature is featured within another feature and fails `canAccess`. The construct it claims to see is not the construct in the file. Left; no rule to add. The five rows carried the same categorizer asymmetry as the row above — we word this message exactly as the reference does — and are now categorized alike on both sides, which does not pair them, since we report nothing on those lines. |

### The census the verdicts above account for

Deduplicated by message, the 58 only-pilot occurrences distribute as follows. No entry is a rule the
reference has and we lack: every one is either adjudicated above or a diagnostic downstream of
notation the reference cannot parse.

| Occurrences | Message | Verdict |
|---:|---|---|
| 12 | `Bound features should have conforming types` | implicit binding connectors the reference synthesizes, in files that already carry agreed errors |
| 6 | `The opposite features … do not refer to each other` | the reference's own EMF metamodel invariant |
| 5 | `Must be an accessible feature (use dot notation for nesting)` | recovery collateral of `namespace` in a `.sysml` file |
| 5 | `Duplicate of other owned member name` | recovery collateral of imports without a visibility keyword |
| 4 | `Duplicate of inherited member name 'done' from Action` | a bare `done;`, which the pinned grammar cannot express |
| 2 | typing-kind (`attribute`, `occurrence/item/part`) | one downstream of an unresolved type, one in a file the reference cannot parse |
| 1 | `Must be model-level evaluable` | reported by both; a categorizer asymmetry in this harness |
| 23 | syntax and unresolved-reference cascades | `views-demo.sysml`, `passes/import_no_visibility.sysml`, `pseudostates-demo.sysml`, `phase-c-behavioral-bodies.sysml`, `solver-demo.sysml` — retained extensions the reference has no production for, plus what its recovery reports afterwards |

Every verdict here was taken from a matched pair of runs over a reduced model, not from the corpus
row: `build/pilot-sysml-validator/validate-sysml-batch --root <dir> <file>` against
`bin/sysml -validate <file>`, one construct per file, comparing message, severity, line and column.
A corpus row cannot settle any of these on its own, because in every one of these files the
reference has already failed to parse something before it reaches the diagnostic under discussion.

### One-sided categorizations, swept

An asymmetric category mapping is worse than a wrong count: it makes the instrument report a
divergence that does not exist. The report's `unmapped` block lists every message each side emits
that no mapping claimed, which makes the whole class checkable — for each entry, put the same text
through the other side's function and see whether it maps. Over the roots above that yields three
findings and no others:

- `Must be model-level evaluable` and `Must be an accessible feature (use dot notation for nesting)`
  are worded identically by both implementations, and only ours mapped. **Fixed:** `categorizePilot`
  now applies the same `must be` clause `categorizeOpenSysML` does, and `TestCategorizePilot` pins
  both. Six rows change category; no bucket total moves.
- `Duplicate of other owned member name`, `Duplicate of inherited member name …`, `Cannot identify
  flow end (use dot notation)` and `… participates in a specialization cycle` are unmapped on both
  sides. Already symmetric, and deliberately so: no category above describes them.
- The `opposite features …` messages would be caught by our side's `type` clause, through
  `owningType`, if it were applied to them. **Defended, not fixed:** that clause reads our own
  vocabulary, where `type` means the type system; the reference's text is an EMF field name, and
  mapping it would manufacture agreement for a diagnostic that states no rule. The pilot side
  enumerates its type-tier messages instead, which is why it does not.

The remaining known one-sided clause is `should be`, which the pilot side maps and ours reaches only
through `conform`. No diagnostic of ours is worded that way, so there is nothing to categorize; if
one is ever added, this is the clause to revisit.

## The declared errata overlay

Every root is compared a second time with the [declared errata](wave14-errata.md) applied to a
**copy** of it, so the published corpus stays byte-identical on disk. The as-published census above
is the conformance statement; the corrected one is a secondary diagnostic and is reported beside it:

```
355 file(s), 328 fully agreeing; 37 agreed, 27 only ours, 58 only the pilot's   (as published)
355 file(s), 330 fully agreeing; 37 agreed, 25 only ours, 58 only the pilot's   (errata applied)
```

Two corrections lie inside these roots — F82, `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:38
(SysML v2 §9.8.9.1), and F111, `Individuals Examples/AnalysisIndividualExample.sysml`:86
(KerML 7.4.9, 8.3.4.2). Both implementations are re-run over each corrected copy, because a correction
that clears our diagnostic while the reference still reports there would be a finding rather than a
fix. Here neither is: both report `ours 1->0, pilot 0->0` — the pinned pilot does no dimensional
analysis and validates subsetting conformance nowhere, so it is silent on all four texts and no pilot
verdict changed.

F83 (`Analysis Examples/Turbojet Stage Analysis.sysml`:25) is documented **without** a correction:
its dimensions are wrong (L^6 against Θ) with no intended reading to infer, so both figures keep the
published text and our warning at that line stays in the census above.
