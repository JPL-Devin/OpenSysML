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

## Pinned reference

The same pin as the differential: OMG SysML v2 Pilot Implementation `2026-05`
(`jupyter-sysml-kernel 0.60.1`, see `scripts/pilot-pin.sh`). Two validators referee:

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
systematically from three sources, one subdirectory each:

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
   extensions and, since W8E, gated behind an opt-in
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

What this corpus cannot see: it tests the invalid models we thought to write. **We authored all 119
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

## Totals

Under the default `-conformance auto`:

```
120 case(s): 116 both reject, 4 only the pilot rejects, 0 only we reject, 0 both accept
  of which 5 agree only because we were asked strictly (the default mode accepts them, by design)
```

| Source | Cases | Both reject | Pilot only | Ours only | Both accept |
| --- | --- | --- | --- | --- | --- |
| extensions | 7 | 7 | 0 | 0 | 0 |
| grammar | 79 | 76 | 3 | 0 | 0 |
| xpect | 34 | 33 | 1 | 0 | 0 |

The corpus grew from 79 cases to 119 in wave 10G and to 120 with `g60` (an `alias` named by a
keyword), and the default-mode gap count is 9 of 120. Wave 11 closed two `xpect/` gaps: `p11`
(11D's and 11G's model-level evaluability predicate on metadata body values) and `p15` (11F's
attribute-usage typing rule), leaving 1. Wave 10C closed the two `grammar/` gaps
left from wave 9F — `g02` (bare `import` is an error by default) and `g31` (`allocate` requires
its `ConnectorPart`) — which is why `grammar/` reads 3 rather than 5, and wave 10B's validation
rules closed eleven `xpect/` gaps (`p08`, `p17`, `p20`,
`p21`, `p22`, `p25`, `p26`, `p27`, `p28`, `p32`, `p33`). No case in the corpus is
accepted by both implementations.

The five strict-only agreements are `x01`, `x04`, `x05`, `x06` and `x07`: OpenSysML notation
extensions that the default mode accepts on purpose and strict mode reports as errors. Judged in
the default mode the same corpus gives 111 agreements and 9 gaps, which is what `-conformance
default` prints — the extra five are those same `extensions/` cases, which the default mode accepts
on purpose. `-conformance strict` gives 119 and 1: wave 10C gave `g15` and `k02` a strict
escalation and `g60` follows the same rule, so every `grammar/` case is rejected when asked
strictly and only `p24` remains — agreement under an opt-in question, not default-mode conformance. Of the 14 gaps this document carried before wave 8, six were closed by the
validation waves themselves — `p01`, `p02`, `p03`, `p05` (wave 8C), `p06` (wave 8A) and `p04`
(wave 8B) — and only the five `extensions/` cases belong to strict mode.

Read those five as agreement *when asked strictly*, not as five gaps that disappeared. An opt-in
check is weaker evidence than a default one: it says the strict question has an answer we agree on,
not that the pipeline a user gets by default rejects the notation — by design it does not. And
because we authored all 120 cases ourselves, a small gap count means we ran out of questions we
thought to ask, not that we stopped being permissive: the denominator measures our coverage of the
rejection surface, not our conformance.

The two `extensions/` cases that agree in either mode (`x02` choice, `x03` junction) are rejected
by us for a different reason than by the pilot: our own state-connectivity validation flags a pseudostate
with no outgoing transition, while the pilot rejects the notation itself. The bucket records
rejection, not agreement on the rule.

## Permissiveness gaps

All 4 gaps, each with its reproducer (the corpus file is the minimal reproducer), both verdicts,
and the package the root cause is likely in. The three `grammar/` rows are rejected under
`-conformance strict` and accepted by default; the `xpect/` row no mode of ours checks.

| Reproducer (`cmd/pilot-reject/testdata/negative/`) | Ours | Pilot | Likely root cause |
| --- | --- | --- | --- |
| `grammar/g15-keyword-as-name.sysml` | accepts | `no viable alternative at input 'part'` | `internal/core/parser` — allows a reserved keyword as a declared name |
| `grammar/g60-alias-keyword-as-name.sysml` | accepts | `extraneous input 'part' expecting 'for'` | `internal/core/parser` — an `alias` recovers a reserved keyword as its declared name |
| `grammar/k02-sysml-keyword-in-kerml.kerml` | accepts | `no viable alternative at input 'def'` | `internal/core/parser` — `.kerml` files are parsed with the full SysML grammar; no per-language restriction |
| `xpect/p24-metadata-abstract-type.sysml` | accepts | `Must have a concrete type` | **unimplemented obligation, blocked on L3 slice B** — validation needs the semantic fact that the metaclass is abstract, but the reduced library record does not carry abstractness |

Each pilot message above is the first error the validator reports for the case; the full lists are
in the baseline JSON's `pilot` arrays.

### Step 3 measurement and p24 deferral

Independent fresh-cache runs of exact base `4b9baf2d` and this tree are unchanged:
**120 cases; 116 both reject, 4 only the pilot rejects, 0 only we reject, 0 both accept**.
There are no rejection-row recoveries, regressions, or unmasked diagnostics.

`p24` remains an **unimplemented obligation** under KerML
`validateMetadataFeatureMetaclassNotAbstract`: a metadata usage typed by an abstract metaclass must
be rejected. The missing semantic fact is abstractness of the metaclass after a standard-library
symbol is restored from the reduced record. This branch does not add a one-off `IsAbstract` field,
hardcode standard-library qualified names, or parse library declarations on demand; the record
format and its reflective fact-family equality checks are owned by L3 slice B.

## Adjudications (W9F)

Every gap below is a **real permissiveness finding**: the pinned grammar admits none of these
models, so a conforming SysML v2 tool rejects them and we do not. Adjudicating one says where the
divergence is deliberate and who owns the fix — never that it is not a divergence.

- **`grammar/g02-import-without-visibility.sysml` — divergence in severity, fix out of slice.**
  The pinned `ImportPrefix` (`SysML.xtext:241`, `KerML.xtext:169`) makes `visibility =
  VisibilityIndicator` **mandatory**, unlike the optional `MemberPrefix` visibility beside it
  (`SysML.xtext:218`), so `import Q::*;` is not a well-formed import and the reference reports
  `mismatched input 'import'`. We do report it — as a *warning*
  (`internal/core/passes/import_visibility.go`, code `import-visibility`), and warnings do not
  count as rejection here. Per the pinned grammar the severity should be an error in the default
  mode; the diagnostic is a semantic pass, so W9F does not change it (`internal/core/passes` is
  another wave-9 slice). Wave-10 item: raise `SeverityWarning` to an error, or make it
  conformance-dependent as `nonstandard_notation.go` already does.
  **Closed in wave 10C (D2):** the finding is an error in every mode, and the case now rejects.
- **`grammar/g15-keyword-as-name.sysml` — deliberate recovery policy, still a divergence.** The
  pinned grammar's `Name` is the `ID` terminal, which excludes keywords, and `part def part;` is
  `no viable alternative at input 'part'`. We read a keyword in name position as the name the
  author meant and warn that an unrestricted name (`'part'`) is required to spell it
  (`internal/core/parser/namespace.go`, code `reserved-keyword-name`, KerML §7.2.4) — a recovery
  policy chosen so an editor keeps a usable tree, documented in
  [conformance-audit.md](../reference/grammar/conformance-audit.md). It is a severity divergence,
  not a reading divergence: we do not silently accept the model, we accept it with a warning where
  the specification's grammar has no production for it. Since warnings are not rejection, the gap
  stands. Wave-10 item: escalate this warning to an error under `-conformance strict`, which keeps
  the recovery behaviour for editors while letting the strict question be answered correctly.
  **Wave 10C:** `reserved-keyword-name` is an error under `-conformance strict`; the default mode
  still warns and recovers, so the case remains a default-mode gap.
- **`grammar/k02-sysml-keyword-in-kerml.kerml` — one grammar for both languages, fix out of
  slice.** `part def` exists in no KerML production; the KerML validator reports `no viable
  alternative at input 'def'`. We parse `.kerml` with the same grammar as `.sysml` and filter
  afterwards: `internal/core/passes/nonstandard_notation.go` reports SysML-only notation in a
  KerML file, at a severity that depends on the conformance mode. As W9F measured it, the walker
  did not cover the SysML *definition and usage keywords* themselves, so `-conformance strict` left
  this case open too. Extending that walker is a `internal/core/passes` change, so W9F wrote it up
  rather than doing it.
  **Wave 10C:** the walker now reports SysML declaration keywords in a `.kerml` file, so strict
  mode rejects the case; the default mode warns, so it remains a default-mode gap.
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

### Grammar mutation pass (W9F)

The `grammar/` derivation was extended along the *unreached* axis rather than the interesting-case
axis: [grammar-coverage.md](grammar-coverage.md) lists the forms no input of ours touches, and W9F
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

### Second pass (W10G)

The second pass extended the corpus along two axes at once, so the two instruments cross-check.

**Grammar axis (13 cases).** The five forms `grammar-coverage.md` calls unseen were already reached
by W9F, so this pass mutated productions the coverage report cannot see as blind spots at all —
`ConjugatedPortTyping`, `RealValue`, `StringValue`, `RangeExpression`, positional argument lists,
`FeatureChainMember`, `QualifiedName`, unrestricted names, `SatisfyRequirementUsage`,
`OwnedCrossSubsetting`, `Unioning`, `ConnectorEndMember` and `MetadataTyping` (`g50`–`g59`,
`k17`–`k19`). The coverage instrument's own movement is unchanged by this pass and by design: with
the negative corpus added as a scanned root the report still shows **0 unseen forms of 807** (it
showed 5 before W9F's cases existed), and the committed baseline still shows **5 unseen forms**
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

### Should the default mode reject the five `extensions/` cases?

Per the specification, **yes**. `initial`, `region`, `defer`, `history` and `transition <src> to
<tgt>` appear in no production of the pinned grammars — `StateBodyItem` has no initial, history or
deferral member, concurrency is spelled `state ... parallel`, and `TransitionUsage` connects with
`first ... then`. The SysML v2 textual notation is defined by that grammar, so a model using them
is not a conforming SysML v2 model, and a tool asked whether it conforms must say no. Accepting
them by default is therefore not "conformance we argued" but a **superset we chose**: OpenSysML's
default mode implements a dialect, and the honest statement of `-conformance auto` agreement on
`x01`, `x04`, `x05`, `x06`, `x07` is that the strict question has an answer we agree on while the
default pipeline a user gets accepts notation the reference rejects as a syntax error. What makes
the choice defensible is not the extensions' usefulness but that the conforming question remains
askable: [strict mode](../guide/03-command-line.md#strict-conformance) reports every one of them as
an error, each has dedicated parser and runtime tests, and each is documented as an extension. If
strict mode ever stopped covering one of them, the default-mode acceptance would be an
undocumented non-conformance and the case should be fixed instead of adjudicated.

### Forms kept rejected (W10D)

Slice 10D probed the parser-debt follow-ups F60–F63, F102 and F103 against the pinned grammars and
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
this document, the README's rejection-oracle line, and the skill's headline from
[pilot-rejection-baseline.json](pilot-rejection-baseline.json), and checks the gap table above
enumerates exactly the baseline's `pilot-only-rejects` cases. It reads only committed files — no
validators, no downloads — so it runs in CI and fails the moment this prose goes stale.
