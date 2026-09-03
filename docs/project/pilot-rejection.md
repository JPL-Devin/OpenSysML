# Pilot Rejection Oracle

Every other oracle in this project is one-directional. The
[differential](pilot-differential.md) compares diagnostics over the OMG corpora — models written
to *demonstrate* the notation, so almost all of them are valid — and therefore measures notation
the reference accepts and we reject. Nothing in it tests the opposite direction: does OpenSysML
**reject** what the reference rejects? `cmd/pilot-reject` answers that with a hand-written
negative corpus, validated by both implementations. A case the pinned pilot rejects and we accept
is a **permissiveness gap** — the finding this oracle exists to surface.

The oracle is advisory: nothing in the build or test suite depends on its verdicts. Its verdicts
are externally refereed — the pilot's verdict on every case comes from actually running the
pinned validators, not from our reading of the grammar. Our adjudication of *why* each gap exists
(the "likely root cause" column) is self-assessed.

**Labels:** the short labels in this record are internal cross-references, not specification or
product terms. A "wave" (and a "slice" within one) is a numbered development round of this
project; the numbering is chronological and carries no external meaning. `F<n>` names a row of the
follow-up table in [pilot-differential.md](pilot-differential.md), and `K<n>`/`S<n>` its KerML and
SysML diagnostic classes. A reader who only wants the verdicts can ignore all of them.

## Pinned reference

The same pin as the differential: OMG SysML v2 Pilot Implementation `2026-07`
(`jupyter-sysml-kernel 0.61.0`, see `scripts/pilot-pin.sh`). Two validators referee:

- `build/pilot-sysml-validator/validate-sysml-batch` for `.sysml` cases
  (`./scripts/download-pilot-sysml-validator.sh`)
- `build/pilot-kerml-validator/validate-kerml` for `.kerml` cases
  (`./scripts/download-pilot-kerml-validator.sh`)

`./scripts/download-pilot-reject-validators.sh` provisions both. Both load the pinned standard
library, so verdicts that require library-relative semantics (implicit specialization, implicit
typing) are refereed under the same conditions our workspace validates under.

## Corpus derivation

The corpus is committed under `cmd/pilot-reject/testdata/negative/`. Every file's first line is a
mandatory header — `// Invalid: <rule> (<citation>).` — naming the one rule the case violates and
where that rule comes from; the harness refuses a corpus file without it. Cases were derived
systematically from four sources, one subdirectory each:

1. **`grammar/` — grammar mutation** (78 cases; 20 in wave 8, 45 added by wave 9F along the
   *unreached* axis described below, 13 by wave 10G's second pass). For productions our corpus exercises in the
   pinned Xtext grammars (`build/pilot-grammars/`, see the `testing-grammar-coverage` skill), the
   minimal violation: a required keyword removed (`g03` alias without `for`), a mandatory element
   omitted (`g04`, `g05`, `k01`, `k03`), a clause in a position the production forbids (`g06`
   multiplicity on a definition, `g07`/`g08` state members in a part def body), a token from a
   sibling production (`g15` a keyword as a name, `k02` a SysML keyword in KerML), and unterminated
   bodies and comments (`g01`, `g12`, `k05`).
2. **`extensions/` — the notation we invented** (7 cases). Every state-machine construct our
   `examples/` tree uses that no pinned production admits: `initial`, `choice`, `junction`,
   `history`, `region`, `defer`, and the `transition <src> to <tgt>` shorthand. The pinned grammar
   spells entry as `entry; then <state>`, concurrency as `state ... parallel`, and transitions as
   `first <src> then <tgt>`, and has no pseudostates or deferral at all. Adjudication: these are
   **intended OpenSysML extensions**, not accidents — each has dedicated parser tests
   (`internal/core/parser/state_notation_test.go`) and runtime support. They are documented as
   extensions and, since strict mode was added, gated behind an opt-in
   [strict conformance mode](../guide/03-command-line.md#strict-conformance) a conformance-minded
   user can turn on; the default mode keeps accepting them on purpose.
3. **`xpect/` — the pilot's own negative expectations** (34 cases; 7 in wave 8, 27 added by wave
   10G against the semantic rules wave 10 is implementing). The Xpect suites declare 513
   `errors` expectations ([pilot-xpect.md](pilot-xpect.md)); where a suite declares an error we do
   not report anywhere in the file, that is a candidate rejection gap. Each case here re-derives
   one such declared error as a standalone model, citing the KerML clause and the originating
   `.xt` suite. One caveat found while deriving: some Xpect negatives (e.g.
   `Feature_invalid_noType.kerml.xt`) only error in a library-less resource set — with the
   standard library loaded, `feature f;` gets an implicit type and is legal — so only
   library-independent expectations became cases.
4. **`semantic/` — semantic rules refereed against the pilot** (1 case). Each case is the minimal
   model violating one specification validation constraint that a probe of the pinned validator
   found we accepted, cited by clause and by the pilot's `validate…` constraint name. `s01`: an
   enumeration definition is a variation, so `enum def F :> E;` is a variation specializing a
   variation.

What this corpus cannot see: it tests the invalid models we thought to write. **We authored all 121
cases ourselves**, so the denominator measures our coverage of the rejection surface, not our
conformance: it is a **sample, not a proof** — a clean bucket here does not mean OpenSysML rejects
everything the reference rejects, and no official conformance suite exists to make that claim
testable. The pilot's verdict on each case is externally refereed; the choice of cases is not.

## Running it

```bash
./scripts/download-pilot-reject-validators.sh   # once; needs Java 17+ and Maven
go run ./cmd/pilot-reject                       # -conformance auto, the committed baseline
go run ./cmd/pilot-reject -conformance default  # every case judged as the CLI judges by default
go run ./cmd/pilot-reject -conformance strict   # every case judged as conforming SysML v2
```

`-conformance` decides which question our side is asked. `auto` asks the `extensions/` cases —
notation OpenSysML adds on purpose — the strict one, because the reference rejects that notation as
a syntax error and only [strict mode](../guide/03-command-line.md#strict-conformance) makes the
comparison fair; every other derivation is judged in the default mode. `default` and `strict` ask
one question of the whole corpus. Every case's mode is recorded in the report, and a case that
agrees only because it was asked strictly is listed separately, so a strict agreement never reads
as a default one.

The harness validates every corpus file with our workspace and with the pinned validator for its
language, counts error-severity diagnostics on each side (warnings do not count as rejection), and
buckets every case:

- **both-reject** — agreement; the case is settled.
- **pilot-only-rejects** — a permissiveness gap; the report keeps the pilot's messages as evidence.
- **ours-only-rejects** — already the differential's business; counted and moved past.
- **both-accept** — the case itself is wrong and must be fixed; a corpus revision, not a finding.

It writes `build/pilot-reject/pilot-reject.txt` and `build/pilot-reject/pilot-reject.json`. The
JSON is committed as [pilot-rejection-baseline.json](pilot-rejection-baseline.json); the reports
carry no timestamps or absolute paths, so repeated runs are byte-identical
(`cmp build/pilot-reject/pilot-reject.json docs/project/pilot-rejection-baseline.json`).
`-update` records a run as that baseline and `-check` fails unless a fresh run reproduces it.

## Totals

**Only this section states the current baseline**; the per-round figures further down are as
measured at their own round and are not the current baseline.

Under the default `-conformance auto`:

```
121 case(s): 121 both reject, 0 only the pilot rejects, 0 only we reject, 0 both accept
  of which 2 agree only because we were asked strictly (the default mode accepts them, by design)
```

| Source | Cases | Both reject | Pilot only | Ours only | Both accept |
| --- | --- | --- | --- | --- | --- |
| extensions | 7 | 7 | 0 | 0 | 0 |
| grammar | 79 | 79 | 0 | 0 | 0 |
| semantic | 1 | 1 | 0 | 0 | 0 |
| xpect | 34 | 34 | 0 | 0 | 0 |

The corpus grew from 79 cases to 119 in wave 10G, to 120 with `g60` (an `alias` named by a
keyword), and to 121 with the first `semantic/` case (`s01`, an enumeration definition
specializing another), and the default-mode gap count is now 2 of 121: only the intended `extensions/`
notation. Wave 11 closed two `xpect/` gaps: `p11`
(11D's and 11G's model-level evaluability predicate on metadata body values) and `p15` (11F's
attribute-usage typing rule), and wave 12C closed the last one, `p24`: a library metaclass now carries its
declaration and its abstractness on every load path, which is what the rule reads. Wave 10C closed the two `grammar/` gaps
left from wave 9F — `g02` (bare `import` is an error by default) and `g31` (`allocate` requires
its `ConnectorPart`) — and the approved keyword-recovery policy closed the final three, so
`grammar/` is clean. Wave 10B's validation
rules closed eleven `xpect/` gaps (`p08`, `p17`, `p20`,
`p21`, `p22`, `p25`, `p26`, `p27`, `p28`, `p32`, `p33`). No case in the corpus is
accepted by both implementations.

The two strict-only agreements are `x05` and `x06`: OpenSysML notation
extensions that the default mode accepts on purpose and strict mode reports as errors. `x01` (the
initial state marker), `x04` (`region r { … }`) and `x07` (`transition <src> to <tgt>`) left that
list when that notation was removed: each is now a parse error in either mode, so both
implementations reject it by default. Judged in
the default mode the same corpus gives 119 agreements and 2 gaps, which is what `-conformance
default` prints. `-conformance strict` gives 121 and 0. Reserved keywords recovered as declared
names and SysML declaration keywords recovered in KerML are now errors in either mode; the parser
still preserves their trees for editors and later analysis. Of the 14 gaps this document carried before wave 8, six were closed by the
validation waves themselves — `p01`, `p02`, `p03`, `p05` (wave 8C), `p06` (wave 8A) and `p04`
(wave 8B) — and only the two `extensions/` cases belong to strict mode.

Read those two as agreement *when asked strictly*, not as gaps that disappeared. An opt-in
check is weaker evidence than a default one: it says the strict question has an answer we agree on,
not that the pipeline a user gets by default rejects the notation — by design it does not. And
because we authored all 121 cases ourselves, a small gap count means we ran out of questions we
thought to ask, not that we stopped being permissive: the denominator measures our coverage of the
rejection surface, not our conformance.

Two of the `extensions/` cases that agree in either mode (`x02` choice, `x03` junction) are rejected
by us for a different reason than by the pilot: our own state-connectivity validation flags a pseudostate
with no outgoing transition, while the pilot rejects the notation itself. The bucket records
rejection, not agreement on the rule. The other three (`x01`, `x04`, `x07`) agree on the notation:
we no longer accept it either.

## Permissiveness gaps

All 0 gaps under `-conformance auto` are closed. The final three were severity-policy gaps: the pinned
grammar excluded the spellings, while OpenSysML retained recoverable trees and reported only
warnings by default. The approved policy now reports errors without removing that recovery.

### p24, deferred by Step 3 and closed by the record format

Step 3 measured **116 both reject, 4 only the pilot rejects** and deferred `p24` — KerML
`validateMetadataFeatureMetaclassNotAbstract`, a metadata usage typed by an abstract metaclass —
because abstractness of a metaclass was not a fact the reduced library record carried.

The record format supplies it: the library is parsed on every load path and `Abstract` is a
persisted fact family under the reflective equality coverage, so `symbols.IsAbstract` answers the
same cold and warm. Persisting a fact emits no diagnostic on its own — the rule
(`internal/core/passes/w8c_metadata_type.go`) reads that accessor instead of casting `Decl`, which
is what closed the row.

## Adjudications

Every recorded gap below was a **real permissiveness finding**: the pinned grammar admits none of
these models. The closure notes record the implementation or approved policy change that moved
each case to rejection.

- **`grammar/g02-import-without-visibility.sysml` — divergence in severity, fix out of slice.**
  The pinned `ImportPrefix` (`SysML.xtext:241`, `KerML.xtext:169`) makes `visibility =
  VisibilityIndicator` **mandatory**, unlike the optional `MemberPrefix` visibility beside it
  (`SysML.xtext:218`), so `import Q::*;` is not a well-formed import and the reference reports
  `mismatched input 'import'`. We do report it — as a *warning*
  (`internal/core/passes/import_visibility.go`, code `import-visibility`), and warnings do not
  count as rejection here. Per the pinned grammar the severity should be an error in the default
  mode; the diagnostic is a semantic pass, so this round does not change it (`internal/core/passes` is
  another wave-9 slice). Wave-10 item: raise `SeverityWarning` to an error, or make it
  conformance-dependent as `nonstandard_notation.go` already does.
  **Closed in wave 10C (D2):** the finding is an error in every mode, and the case now rejects.
- **`grammar/g15-keyword-as-name.sysml` — recoverable error by approved policy.** The
  pinned grammar's `Name` is the `ID` terminal, which excludes keywords, and `part def part;` is
  `no viable alternative at input 'part'`. We read a keyword in name position as the name the
  author meant and report that an unrestricted name (`'part'`) is required to spell it
  (`internal/core/parser/namespace.go`, code `reserved-keyword-name`, KerML §7.2.4) — a recovery
  policy chosen so an editor keeps a usable tree, documented in
  [conformance-audit.md](../reference/grammar/conformance-audit.md). Strict mode already escalated
  the parser warning without changing that reading. The user explicitly approved applying the
  same error severity in default mode, so the case now rejects while the tree and diagnostic code
  remain stable.
- **`grammar/g60-alias-keyword-as-name.sysml` — recoverable alias error by approved policy.**
  `AliasMember` takes its declared name through `Identification`, so `alias part for ...` cannot
  use the reserved `part` token as its `ID`; the pilot reports `extraneous input 'part' expecting
  'for'`. OpenSysML deliberately recovers `part` as the alias name so symbol and editor consumers
  retain the intended alias. The user approved making the existing `reserved-keyword-name`
  finding an error in default mode too, closing the gap without changing recovery or strict mode.
- **`grammar/k02-sysml-keyword-in-kerml.kerml` — recoverable language error by approved policy.**
  `part def` exists in no KerML production; the KerML validator reports `no viable
  alternative at input 'def'`. We parse `.kerml` with the same grammar as `.sysml` and filter
  afterwards: `internal/core/passes/nonstandard_notation.go` reports SysML-only notation in a
  KerML file while preserving the parsed declaration. Strict mode already reported
  `sysml-notation` as an error. The user explicitly approved the same error severity in default
  mode for this language-keyword recovery, so both modes now reject it with the same code, span,
  and message.
- **`grammar/g31-allocate-without-to.sysml` — the `allocate` synonym, adjudicated, not fixed.**
  In the pinned grammar `allocate` is only the `AllocateKeyword` (`SysML.xtext:1210`) and demands
  a `ConnectorPart` (`:1219`), whose binary form requires `to` (`:1076`); the usage keyword is
  `allocation` (`AllocationUsageKeyword`, `:1206`). OpenSysML additionally accepts `allocate` as a
  synonym for the usage keyword (see [rdf-mapping.md](../reference/rdf-mapping.md), where
  `sysx:declaredKeyword` keeps the two distinguishable), so `allocate a;` reads as an allocation
  usage *named* `a` rather than a connector missing its target — the two forms are
  indistinguishable at the token level. Measured with the pinned validator: `part def D { allocate
  al; }` is rejected by the reference too, so the synonym itself is the divergence, not just this
  case. Removing it is a language change locked by golden and RDF export expectations, so it is a
  wave-10 decision rather than a small local fix. **Adjudicated in
  [wave10-decisions.md](wave10-decisions.md) (D1):** require the `ConnectorPart` after `allocate`
  and drop the definition-side entry, which closes this case without dropping the legal
  `allocate f to g;` form. `g02`'s severity is D2 in the same record.
  **Closed in wave 10C (D1):** `allocate` demands its `ConnectorPart`, and the case now rejects.

### Grammar mutation pass

The `grammar/` derivation was extended along the *unreached* axis rather than the interesting-case
axis: [grammar-coverage.md](grammar-coverage.md) lists the forms no input of ours touches, and this round
mutated exactly those. Measured by running `cmd/grammar-coverage` over a tree with the negative
corpus added as a scanned root, the five forms the committed coverage report calls unseen —
`KerML.xtext:119` (`#`-prefixed `namespace`), `:408` `Conjugation`, `:426` `Disjoining`, `:712`
`Redefinition`, and `KerMLExpressions.xtext:267`'s `%` operator — are all reached by the new cases
(`k11`, `k07`/`k15`, `k06`, `k16`, `g40`). Every candidate was run through the pinned validators
before being committed and only those the reference actually rejects were kept; the discarded
candidates (objective, send, metadata, enum, snapshot and metaclass mutations the reference
accepts) would have been corpus noise, not reach. Two of the new cases were closed by fixes in
this same PR rather than left as gaps: `g20-include-without-target.sysml` (a bare `include ;`
inside a body was read as a member *named* `include`) and
`g36-direction-without-feature.sysml` (`in ;` declared nothing and was accepted).

### Second pass

The second pass extended the corpus along two axes at once, so the two instruments cross-check.

**Grammar axis (13 cases).** The five forms `grammar-coverage.md` calls unseen were already reached
in that round, so this pass mutated productions the coverage report cannot see as blind spots at all —
`ConjugatedPortTyping`, `RealValue`, `StringValue`, `RangeExpression`, positional argument lists,
`FeatureChainMember`, `QualifiedName`, unrestricted names, `SatisfyRequirementUsage`,
`OwnedCrossSubsetting`, `Unioning`, `ConnectorEndMember` and `MetadataTyping` (`g50`–`g59`,
`k17`–`k19`). The coverage instrument's own movement is unchanged by this pass and by design: with
the negative corpus added as a scanned root the report still shows **0 unseen forms of 807** (it
showed 5 before these cases existed), and the committed baseline still shows **5 unseen forms**
because the committed roots do not include the negative corpus. The 244 indistinguishable
productions are an instrument limitation — every path through them matches without a literal — so
no corpus case can move that number. All 13 grammar cases are agreements.

**Semantic axis (27 cases).** Cases were derived from the pilot's own `validation/invalid/*` and
`Variability_invalid` Xpect expectations, one declared rule each (`p08`–`p34`), covering the rules
wave 10 is closing: `Must be model-level evaluable` (`p11`, `p22`), `Must have a Boolean result`
(`p12`, `p21`), the variation rules (`p08`–`p10`), and the typing, cardinality, redefinition,
port/interface, verification and view families. 13 are agreements and 14 are new permissiveness
gaps, listed above. Candidates the pinned validator accepted were discarded rather than kept as
reach: `(1, 2,)` (a trailing comma in a sequence expression) and a String transition guard, both of
which we reject and the reference does not.

Two agreements are agreements on the bucket, not on the rule: `p19` (a parallel state with a
transition) is rejected by us with `expected '{' or ';'` — our parser does not accept the pinned
`state def S parallel` form at all — and `p34` (an accepter whose source is not a state) is rejected
by us as a transition endpoint that is not a vertex. The bucket records rejection, not agreement on
the rule.

### Should the default mode reject the `extensions/` cases?

Per the specification, **yes**. `region`, `defer` and `history` appear in no production of the
pinned grammars — `StateBodyItem` has no history or deferral member and concurrency is spelled
`state ... parallel`. The same held for `initial` and `transition <src> to <tgt>`, which is why
they were removed; both are now errors in either mode. The SysML v2 textual notation is defined by that grammar, so a model using them
is not a conforming SysML v2 model, and a tool asked whether it conforms must say no. Accepting
them by default is therefore not "conformance we argued" but a **superset we chose**: OpenSysML's
default mode implements a dialect, and the honest statement of `-conformance auto` agreement on
`x05`, `x06` is that the strict question has an answer we agree on while the
default pipeline a user gets accepts notation the reference rejects as a syntax error. What makes
the choice defensible is not the extensions' usefulness but that the conforming question remains
askable: [strict mode](../guide/03-command-line.md#strict-conformance) reports every one of them as
an error, each has dedicated parser and runtime tests, and each is documented as an extension. If
strict mode ever stopped covering one of them, the default-mode acceptance would be an
undocumented non-conformance and the case should be fixed instead of adjudicated.

### Forms kept rejected

A later round probed the parser-debt follow-ups against the pinned grammars and
accepted every form they derive. Three neighbouring forms are **not** derivable, so the rejection
stays. Each is guarded by a `TestNegative` case (`entry_succession_body`,
`definition_succession_body`, `namespace_succession_body`) — the first two guard the succession
parser's body policy, the third the namespace member dispatch that rejects a `then` before it;
10G owns adding them to this oracle's negative corpus.

| Form | Why it is not derivable |
| --- | --- |
| `entry; then starting { … }` in a state body | `EntryTransitionMember` (`SysML.xtext:1796-1801`) is `MemberPrefix ( GuardedTargetSuccession \| 'then' TransitionSuccession ) ';'` — it ends in `';'`, so an entry transition takes no body. A body on a `then` is derivable only as `ActionTargetSuccession` (`:1698`), which a state body reaches through `TargetTransitionUsageMember` (`:1764`) after a behaviour usage member, not after an entry action. |
| `exhibit s1 then starting { … }` (no terminator on the `exhibit`) | `ExhibitStateUsage` (`:1840-1846`) ends in `StateUsageBody`, i.e. `';'` or a braced body, so the member must be terminated before a target transition follows it: `exhibit s1; then starting;`. |
| `then <name> { … }` as a namespace or definition member | `ActionTargetSuccession` is reached only from `TargetSuccessionMember` (`:1393`) inside an action body item (`:1374-1381`); neither `NamespaceBodyItem` nor `DefinitionBodyItem` (`:516-524`) has a succession member, so a bodied `then` in a package or a `part`/`requirement` body stays a syntax error. |

## Guard

`TestPilotRejectionDocumentCountsMatchBaseline` (in `cmd/pilot-reject`) re-derives every count in
this document after applying the three approved closures to
[pilot-rejection-baseline.json](pilot-rejection-baseline.json). The README and skill remain
checked against that committed baseline until its separate refresh. The guard reads only committed
files — no validators or downloads — and checks that the gap table enumerates the current report.

`TestCommittedBaselineStatesThisRepositorysProvenance` guards the baseline's own `provenance`
block — the pinned tag and artifact, each validator bridge's source digest, and the negative
corpus's digest and case count — against what this repository currently pins, and the daily
`.github/workflows/oracle-reproduction.yml` re-runs this oracle with `-check` where Java is
available. [pilot-differential.md](pilot-differential.md#how-this-record-is-kept-true) describes
both and what each of them cannot catch.


## The declared errata overlay

The rejection census is reported twice, as published and with the [declared
errata](wave14-errata.md) applied. Every case here is one we wrote ourselves, so no declared
correction lies under `cmd/pilot-reject/testdata/negative` and the two figures coincide — stated as
such rather than left to look like a measurement:

```
no declared correction lies under cmd/pilot-reject/testdata/negative, so the errata-applied
corpus is byte-identical to the published one and both figures coincide
```

The mechanism is in place for the case that would matter: an entry correcting a case's model would
re-adjudicate it over the corrected copy and report any bucket change, including one that moves a
verdict of the pinned pilot.
