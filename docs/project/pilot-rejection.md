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

1. **`grammar/` — grammar mutation** (86 cases; 20 in wave 8, 45 added by wave 9F along the
   *unreached* axis described below, 13 by wave 10G's second pass, and 7 body-position cases
   `g61`–`g67` from the constraint census described under `semantic/`). For productions our corpus exercises in the
   pinned Xtext grammars (`build/pilot-grammars/`, see the `testing-grammar-coverage` skill), the
   minimal violation: a required keyword removed (`g03` alias without `for`), a mandatory element
   omitted (`g04`, `g05`, `k01`, `k03`), a clause in a position the production forbids (`g06`
   multiplicity on a definition, `g07`/`g08` state members in a part def body), a token from a
   sibling production (`g15` a keyword as a name, `k02` a SysML keyword in KerML), and unterminated
   bodies and comments (`g01`, `g12`, `k05`).
2. **`extensions/` — the notation we invented** (8 cases). Every state-machine construct our
   `examples/` tree uses that no pinned production admits: `initial`, `choice`, `junction`,
   `history`, `region`, `defer`, and the `transition <src> to <tgt>` shorthand, plus `require`
   outside a requirement body (`x08`), which our grammar admits as an extension. The pinned grammar
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
4. **`semantic/` — the pilot's declared SysML validation constraints** (45 cases). The pinned
   `SysMLValidator` names each constraint it implements (`validate<Metaclass><Rule>`, the names of
   the specification's own constraint clauses). Every SysML constraint name was mapped to a case:
   an existing one where an `xpect/` or `grammar/` case already violates that rule (their headers
   now cite the `validate*` name), a new minimal model here otherwise. Each header is
   `// Invalid: <rule> (<clause>; pilot validate<Name>).` so a census can join on the name. Every
   candidate was refereed by the pinned validator before being kept: candidates it accepted were
   discarded, and the constraints behind them are listed under [Constraints the pilot declares but
   does not enforce](#constraints-the-pilot-declares-but-does-not-enforce) rather than kept as
   `both accept` corpus noise. The full name-by-name mapping is the
   [constraint census](#sysml-constraint-census) at the end of this document.

What this corpus cannot see: it tests the invalid models we thought to write. **We authored all 173
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
173 case(s): 164 both reject, 9 only the pilot rejects, 0 only we reject, 0 both accept
  of which 3 agree only because we were asked strictly (the default mode accepts them, by design)
```

| Source | Cases | Both reject | Pilot only | Ours only | Both accept |
| --- | --- | --- | --- | --- | --- |
| extensions | 8 | 8 | 0 | 0 | 0 |
| grammar | 86 | 80 | 6 | 0 | 0 |
| semantic | 45 | 42 | 3 | 0 | 0 |
| xpect | 34 | 34 | 0 | 0 | 0 |

The corpus grew from 79 cases to 119 in wave 10G, to 120 with `g60` (an `alias` named by a
keyword), and to 173 with the SysML constraint census (45 `semantic/`, 7 `grammar/` and 1
`extensions/` case). The census opened nine new permissiveness gaps, listed below; before it the
default-mode gap count was 2 of 120, only the intended `extensions/` notation. Wave 11 closed two `xpect/` gaps: `p11`
(11D's and 11G's model-level evaluability predicate on metadata body values) and `p15` (11F's
attribute-usage typing rule), and wave 12C closed the last one, `p24`: a library metaclass now carries its
declaration and its abstractness on every load path, which is what the rule reads. Wave 10C closed the two `grammar/` gaps
left from wave 9F — `g02` (bare `import` is an error by default) and `g31` (`allocate` requires
its `ConnectorPart`) — and the approved keyword-recovery policy closed the final three, so
`grammar/` is clean. Wave 10B's validation
rules closed eleven `xpect/` gaps (`p08`, `p17`, `p20`,
`p21`, `p22`, `p25`, `p26`, `p27`, `p28`, `p32`, `p33`). No case in the corpus is
accepted by both implementations.

The three strict-only agreements are `x05`, `x06` and `x08`: OpenSysML notation
extensions that the default mode accepts on purpose and strict mode reports as errors. `x01` (the
initial state marker), `x04` (`region r { … }`) and `x07` (`transition <src> to <tgt>`) left that
list when that notation was removed: each is now a parse error in either mode, so both
implementations reject it by default. Judged in
the default mode the same corpus gives 161 agreements and 12 gaps, which is what `-conformance
default` prints. `-conformance strict` gives 164 and 9. Reserved keywords recovered as declared
names and SysML declaration keywords recovered in KerML are now errors in either mode; the parser
still preserves their trees for editors and later analysis. Of the 14 gaps this document carried before wave 8, six were closed by the
validation waves themselves — `p01`, `p02`, `p03`, `p05` (wave 8C), `p06` (wave 8A) and `p04`
(wave 8B) — and only the three `extensions/` cases belong to strict mode.

Read those three as agreement *when asked strictly*, not as gaps that disappeared. An opt-in
check is weaker evidence than a default one: it says the strict question has an answer we agree on,
not that the pipeline a user gets by default rejects the notation — by design it does not. And
because we authored all 173 cases ourselves, a small gap count means we ran out of questions we
thought to ask, not that we stopped being permissive: the denominator measures our coverage of the
rejection surface, not our conformance.

Two of the `extensions/` cases that agree in either mode (`x02` choice, `x03` junction) are rejected
by us for a different reason than by the pilot: our own state-connectivity validation flags a pseudostate
with no outgoing transition, while the pilot rejects the notation itself. The bucket records
rejection, not agreement on the rule. The other three (`x01`, `x04`, `x07`) agree on the notation:
we no longer accept it either.

## Permissiveness gaps

All 9 gaps under `-conformance auto` were opened by the SysML constraint census; every gap the
corpus carried before it is closed. The last three of those were severity-policy gaps: the pinned
grammar excluded the spellings, while OpenSysML retained recoverable trees and reported only
warnings by default. The approved policy now reports errors without removing that recovery.

Each open gap below has its reproducer (the corpus file is the minimal reproducer), both verdicts,
and the package the root cause is likely in. None of them is a strict-mode question: our side is
silent in either mode.

| Reproducer (`cmd/pilot-reject/testdata/negative/`) | Ours | Pilot | Likely root cause |
| --- | --- | --- | --- |
| `grammar/g61-actor-outside-requirement-body.sysml` | accepts | `mismatched input 'actor' expecting '}'` | `internal/core/parser` — the definition-body member dispatch admits `actor` in any body; the pilot's `ActorMember` is a requirement, case or viewpoint body item only |
| `grammar/g62-subject-outside-requirement-body.sysml` | accepts | `mismatched input 'subject' expecting '}'` | `internal/core/parser` — same dispatch admits `subject` in a part def body (`SubjectMember` is a requirement or case body item) |
| `grammar/g63-objective-outside-case-body.sysml` | accepts | `mismatched input 'objective' expecting '}'` | `internal/core/parser` — same dispatch admits `objective` outside a case body (`ObjectiveMember` is a `CaseBodyItem`) |
| `grammar/g64-stakeholder-outside-requirement-body.sysml` | accepts | `mismatched input 'stakeholder' expecting '}'` | `internal/core/parser` — same dispatch admits `stakeholder` outside a requirement body |
| `grammar/g65-entry-action-outside-state-body.sysml` | accepts | `mismatched input 'entry' expecting '}'` | `internal/core/parser/defusage.go` `atKindPrefix` — outside a state body, `entry` before `action` is taken as a kind prefix and dropped, so the member parses as a plain `action init` with no diagnostic; the pilot's `EntryActionMember` is a `StateBodyItem` |
| `grammar/g66-render-outside-view-body.sysml` | accepts | `mismatched input 'render' expecting '}'` | `internal/core/parser` — `render` is parsed in a part def body; the pilot's `ViewRenderingMember` is a `ViewBodyItem` (`expose`, the sibling case `g67`, is already rejected by `expose-owning-namespace`) |
| `semantic/s04-assert-references-non-constraint.sysml` | accepts | `Must reference a constraint.` | `internal/core/passes/typecheck.go` — the referent-kind check on reference subsetting covers `satisfy` (`satisfy target must be a requirement usage`) but not `assert`; no pass checks that an asserted usage is a constraint usage |
| `semantic/s23-metadata-typed-by-part-def.sysml` | accepts | `A metadata usage must be typed by one metadata definition.` / `Must have a concrete type` | `internal/core/passes/typecheck.go` `compatibleTyping` — a metadata usage is typed like an item, so any occurrence definition passes; `w8c_metadata_type.go` checks only abstractness. Pass exists, misses the shape |
| `semantic/s43-assign-to-non-feature.sysml` | accepts | `An assignment must have a referent.` | `internal/core/passes/constraint.go` `assignmentReferentChecker` — returns silently when the resolved target is not a `*ast.Usage`, so assigning to a definition reports nothing. Pass exists, misses the shape |

Each pilot message above is the first error the validator reports for the case; the full lists are
in the baseline JSON's `pilot` arrays. The six `grammar/` gaps share one cause: the pilot's grammar
scopes requirement, case, state and view body items to their own body productions, while our
parser reads those member keywords in any body — `actor`, `subject`, `objective`, `stakeholder` and
`render` as ordinary usage kinds from a body-independent table (`internal/core/parser/defusage.go`),
`entry` as a dropped kind prefix — and only `expose` has a follow-up owning-namespace check.

### Constraints the pilot declares but does not enforce

These SysML constraint names appear in the pinned `SysMLValidator`, but no valid model the pinned
validator admits triggers them as an error, so no case exists for them (a `both accept` case is a
corpus bug by this oracle's rules). Evidence is from reading the pinned source and running
candidate models through the validator.

| Constraint | Evidence |
| --- | --- |
| `validateItemUsageType`, `validatePartUsageType`, `validatePartUsagePartDefinition` | `checkItemUsage` and `checkPartUsage` are commented out in `SysMLValidator.xtend`; the generic `checkUsage` requires only a `Classifier`. `item i : AD;` (an attribute definition) is rejected as `validateOccurrenceUsageType` (`An occurrence, item or part must be typed by occurrence definitions.`, which we also report), and `part p : ID;` (an item definition) is accepted by both implementations. |
| `validateOperatorExpressionQuantity` | Reported as a **warning** (`Should be a measurement reference (unit).`), and warnings do not count as rejection on either side. |
| `validateUseCaseUsageReference` | Only the name and message constants are declared; `checkUseCaseUsage` implements the typing rule (`validateUseCaseUsageType`, `s38`) and nothing reads `INVALID_USE_CASE_USAGE_REFERENCE`. The `include` form is covered by `validateIncludeUseCaseUsageReference` (`s20`). |
| `validateTransitionFeatureMembershipGuardExpression` | The error path (`Must be a Boolean expression.`) exists and the pilot's `TransitionUsage_invalid.sysml.xt` expects it for `if "test"`, but that fixture loads a reduced library without `ScalarValues`. With the full standard library the pinned validator accepts `first s1 if "test" then s2` and `if 1 + 2` — the same shape as the fixture — while we reject both (`transition guard must be Boolean, found String`). Recorded as a question in [omg-issues.md](omg-issues.md#a-non-boolean-transition-guard-is-accepted-with-the-full-library-loaded-pilot-2026-07). |

### Constraints with no valid pilot-admitted violating model

These constraints are enforced by the pilot validator, but every textual model that violates them
is already a syntax error in the pinned grammar (or the grammar's post-processing normalises the
violation away), so the constraint never fires on parsed text and no standalone case can isolate
it. Each is listed with the reason.

| Constraint | Why no case |
| --- | --- |
| `validateAcceptActionUsageParameters`, `validateAssignmentActionUsageArguments`, `validateForLoopActionUsageLoopVariable`, `validateForLoopActionUsageParameters`, `validateIfActionUsageParameters`, `validateWhileLoopActionUsageParameters`, `validateTransitionUsageParameters` | The productions (`AcceptNode`, `AssignmentNode`, `ForLoopNode`, `IfNode`, `WhileLoopNode`, `TransitionUsage`) always synthesise the required parameters (`EmptyParameterMember`, the payload, the loop variable), so a parsed model cannot have too few; the checks guard models built through the API. |
| `validateAttributeUsageIsReferential`, `validateReferenceUsageIsReference`, `validateUsageIsReferential`, `validateEventOccurrenceUsageIsReference`, `validatePortUsageIsReference`, `validateDefinitionVariationIsAbstract`, `validateUsageVariationIsAbstract` | The pilot's adapters (`UsageAdapter.postProcess`, `PortUsageAdapter.postProcess`) set `isComposite = false` for attributes, directed, end and package-level usages, references, events and ports, and `isAbstract = true` for every variation, before validation runs; the textual notation has no way to declare the violating value. |
| `validateAttributeDefinitionFeatures`, `validateAttributeUsageFeatures` | Every nested usage of an attribute definition or usage is normalised to referential by the same adapter, so `checkAllNotComposite` never finds a composite one. |
| `validateConjugatedPortDefinitionConjugatedPortDefinition`, `validatePortDefinitionConjugatedPortDefinition` | The conjugated port definition is created implicitly for every port definition and never for a conjugated one; the notation cannot declare or omit it. |
| `validateFramedConcernMembershipConstraintKind`, `validateRequirementVerificationMembershipKind` | `frame` and `verify` are the only spellings of their memberships and each fixes `kind = requirement` in the grammar (`FramedConcernKind`, `RequirementVerificationKind`). |
| `validateObjectiveMembershipIsComposite`, `validateRequirementConstraintMembershipIsComposite` | `objective` and `require`/`assume` members take no `ref` prefix in the grammar (`ObjectiveMember`, `RequirementConstraintMember`), so the owned usage is always composite. |
| `validateExposeIsImportAll` | `ExposePrefix` has no `all` token (unlike `ImportPrefix`), and `ExposeImpl` constructs every expose with `isImportAll = true`, so a parsed expose can never fail the check. |
| `validateTransitionFeatureMembershipOwningType`, `validateTransitionFeatureMembershipEffectAction`, `validateTransitionFeatureMembershipTriggerAction` | `TransitionFeatureMembership` is only produced by `TriggerActionMember`, `GuardExpressionMember` and `EffectBehaviorMember` inside a transition, each of which fixes the member's metaclass (`AcceptActionUsage`, `Expression`, `ActionUsage`). |

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

## SysML constraint census

One row per SysML constraint name in the pinned `SysMLValidator` that this corpus owns (100 names;
the control-node, variation-specialization, trigger-argument, binding-conformance, send-action and
feature-value-overriding constraints are covered by their own implementation records). *Case* is
the corpus file that violates the constraint — `existing:` where an `xpect/` case predating the
census already did — or `none:` with the reason no case exists. *Bucket* is the committed baseline's
verdict under `-conformance auto`; the pilot and OpenSysML messages are the error-severity
diagnostics each side reports for the case (`/` separates several). The last column is only filled
for the gaps, and names where in OpenSysML the rule would have to fire.

| Constraint | Case | Bucket | Pilot message | Our diagnostic | Why we are silent |
| --- | --- | --- | --- | --- | --- |
| `validateAcceptActionUsageParameters` | none: `AcceptNode` always parses a payload parameter | no violating model | — | — | — |
| `validateActionUsageType` | `semantic/s01-action-typed-by-part-def.sysml` | both-reject | An action must be typed by action definitions. | An action must be typed by action definitions. | — |
| `validateActorMembershipOwningType` | `grammar/g61-actor-outside-requirement-body.sysml` | pilot-only-rejects | mismatched input 'actor' expecting '}' / extraneous input '}' expecting EOF | — | `actor` is dispatched body-independently in `parser/defusage.go`; no pass checks the owner kind |
| `validateAllocationUsageType` | `semantic/s02-allocation-typed-by-connection-def.sysml` | both-reject | An allocation must be typed by allocation definitions. | An allocation must be typed by allocation definitions. | — |
| `validateAnalysisCaseUsageType` | `semantic/s03-analysis-typed-by-case-def.sysml` | both-reject | An analysis case must be typed by one analysis case definition. | An analysis case must be typed by one analysis case definition. | — |
| `validateAssertConstraintUsageReference` | `semantic/s04-assert-references-non-constraint.sysml` | pilot-only-rejects | Must reference a constraint. | — | `typecheck.go` checks the `satisfy` referent kind but has no `assert` counterpart |
| `validateAssignmentActionUsageArguments` | none: `AssignmentNode` always parses both arguments | no violating model | — | — | — |
| `validateAssignmentActionUsageReferent` | `semantic/s43-assign-to-non-feature.sysml` | pilot-only-rejects | An assignment must have a referent. | — | the assignment referent checker in `constraint.go` returns silently when the referent is not a usage |
| `validateAssignmentActionUsageReferentIsTimeVarying` | `semantic/s05-assign-to-package-level-attribute.sysml` | both-reject | Referent must be time varying. | Referent must be time varying. | — |
| `validateAttributeDefinitionFeatures` | none: nested attribute features are normalised to referential | no violating model | — | — | — |
| `validateAttributeUsageEnumerationType` | `semantic/s06-enum-attribute-two-types.sysml` | both-reject | An enumeration attribute cannot have more than one type. | An enumeration attribute cannot have more than one type. | — |
| `validateAttributeUsageFeatures` | none: nested attribute features are normalised to referential | no violating model | — | — | — |
| `validateAttributeUsageIsReferential` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateAttributeUsageType` | existing: `xpect/p15-attribute-typed-by-part-def.sysml` | both-reject | An attribute must be typed by attribute definitions. | An attribute must be typed by attribute definitions. | — |
| `validateCalculationUsageType` | `semantic/s07-calc-typed-by-action-def.sysml` | both-reject | A calculation must be typed by one calculation definition. | A calculation must be typed by one calculation definition. | — |
| `validateCaseDefinitionOnlyOneObjective` | `semantic/s08-case-def-two-objectives.sysml` | both-reject | Only one objective is allowed. | Only one objective is allowed. | — |
| `validateCaseDefinitionOnlyOneSubject` | `semantic/s09-case-def-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateCaseDefinitionSubjectParameterPosition` | `semantic/s10-case-def-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateCaseUsageOnlyOneObjective` | `semantic/s11-case-two-objectives.sysml` | both-reject | Only one objective is allowed. | Only one objective is allowed. | — |
| `validateCaseUsageOnlyOneSubject` | `semantic/s12-case-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateCaseUsageSubjectParameterPosition` | `semantic/s13-case-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateCaseUsageType` | `semantic/s14-case-typed-by-action-def.sysml` | both-reject | A case must be typed by one case definition. | A case must be typed by one case definition. | — |
| `validateConjugatedPortDefinitionConjugatedPortDefinition` | none: the grammar never creates a conjugated port definition inside another | no violating model | — | — | — |
| `validateConnectionUsageType` | `semantic/s15-connection-typed-by-part-def.sysml` | both-reject | A connection must be typed by connection definitions. | A connection must be typed by connection definitions. | — |
| `validateDefinitionVariationIsAbstract` | none: adapter post-processing forces `isAbstract = true` | no violating model | — | — | — |
| `validateDefinitionVariationMembership` | existing: `xpect/p09-variation-member-not-variant.sysml` | both-reject | An owned usage of a variation must be a variant. | An owned usage of a variation must be a variant. | — |
| `validateEnumerationUsageType` | existing: `xpect/p18-enum-two-types.sysml` | both-reject | An enumeration must be typed by one enumeration definition. | An enumeration must be typed by one enumeration definition. | — |
| `validateEventOccurrenceUsageIsReference` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateEventOccurrenceUsageReferent` | `semantic/s16-event-references-non-occurrence.sysml` | both-reject | Must reference an occurrence. | Must reference an occurrence. | — |
| `validateExhibitStateUsageReference` | `semantic/s17-exhibit-references-non-state.sysml` | both-reject | Must reference a state. | Must reference a state. | — |
| `validateExposeIsImportAll` | none: `ExposeImpl` constructs every expose with `isImportAll = true` | no violating model | — | — | — |
| `validateExposeOwningNamespace` | `grammar/g67-expose-outside-view-body.sysml` | both-reject | mismatched input 'expose' expecting '}' / extraneous input '}' expecting EOF | expose is only allowed in a view usage body (SysML v2 8.3.26.2) | — |
| `validateFlowDefinitionConnectionEnds` | `semantic/s18-flow-def-three-ends.sysml` | both-reject | A flow connection definition can have at most two ends. | A flow connection definition can have at most two ends. | — |
| `validateFlowUsageType` | `semantic/s19-flow-typed-by-connection-def.sysml` | both-reject | A flow connection must be typed by flow connection definitions. | A flow connection must be typed by flow connection definitions. | — |
| `validateForLoopActionUsageLoopVariable` | none: `ForLoopNode` always parses the loop variable | no violating model | — | — | — |
| `validateForLoopActionUsageParameters` | none: `ForLoopNode` always parses both parameters | no violating model | — | — | — |
| `validateFramedConcernMembershipConstraintKind` | none: `frame` fixes `kind = requirement` in the grammar | no violating model | — | — | — |
| `validateIfActionUsageParameters` | none: `IfNode` always parses condition and body parameters | no violating model | — | — | — |
| `validateIncludeUseCaseUsageReference` | `semantic/s20-include-references-non-use-case.sysml` | both-reject | Must reference a use case. | Must reference a use case. | — |
| `validateInterfaceDefinitionEnd` | `semantic/s21-interface-def-end-not-port.sysml` | both-reject | An interface definition end must be a port. | An interface definition end must be a port. | — |
| `validateInterfaceUsageEnd` | existing: `xpect/p27-interface-end-not-port.sysml` | both-reject | An interface end must be a port. | An interface end must be a port. | — |
| `validateInterfaceUsageType` | `semantic/s22-interface-typed-by-connection-def.sysml` | both-reject | An interface must be typed by interface definitions. | An interface must be typed by interface definitions. | — |
| `validateItemUsageType` | none: `checkItemUsage` is commented out in the pinned validator | not enforced by the pilot | — | — | — |
| `validateMetadataUsageType` | `semantic/s23-metadata-typed-by-part-def.sysml` | pilot-only-rejects | A metadata usage must be typed by one metadata definition. / Must have a concrete type | — | `w8c_metadata_type.go` checks only abstractness of the metadata type, not that it is a metadata definition |
| `validateObjectiveMembershipIsComposite` | none: `objective` admits no `ref` prefix | no violating model | — | — | — |
| `validateObjectiveMembershipOwningType` | `grammar/g63-objective-outside-case-body.sysml` | pilot-only-rejects | mismatched input 'objective' expecting '}' / extraneous input '}' expecting EOF | — | `objective` is dispatched body-independently in `parser/defusage.go`; no pass checks the owner kind |
| `validateOccurrenceUsageIndividualDefinition` | existing: `xpect/p25-two-individual-definitions.sysml` | both-reject | At most one individual definition is allowed. | At most one individual definition is allowed. | — |
| `validateOccurrenceUsageIndividualUsage` | existing: `xpect/p33-individual-typed-by-plain-def.sysml` | both-reject | An individual must be typed by one individual definition. | An individual must be typed by one individual definition. | — |
| `validateOccurrenceUsageIsPortion` | `semantic/s45-snapshot-outside-occurrence.sysml` | both-reject | Must be owned by an occurrence definition or usage. | Must be owned by an occurrence definition or usage. | — |
| `validateOccurrenceUsageType` | existing: `xpect/p16-part-typed-by-attribute-def.sysml` | both-reject | An occurrence, item or part must be typed by occurrence definitions. | An occurrence, item or part must be typed by occurrence definitions. | — |
| `validateOperatorExpressionQuantity` | none: reported as a warning only | not enforced by the pilot | — | — | — |
| `validatePartUsagePartDefinition` | none: `checkPartUsage` is commented out in the pinned validator | not enforced by the pilot | — | — | — |
| `validatePartUsageType` | none: `checkPartUsage` is commented out in the pinned validator | not enforced by the pilot | — | — | — |
| `validatePerformActionUsageReference` | `semantic/s24-perform-references-non-action.sysml` | both-reject | Must reference an action. | Must reference an action. | — |
| `validatePortDefinitionConjugatedPortDefinition` | none: `PortDefinition` always synthesises exactly one conjugated definition | no violating model | — | — | — |
| `validatePortDefinitionOwnedUsagesNotComposite` | existing: `xpect/p26-port-def-nonreferential-usage.sysml` | both-reject | Owned usages of a port definition (other than ports) must be referential. | Owned usages of a port definition (other than ports) must be referential. | — |
| `validatePortUsageIsReference` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validatePortUsageNestedUsagesNotComposite` | `semantic/s25-port-nested-composite-part.sysml` | both-reject | Nested usages in a port usage (other than ports) must be referential. | Nested usages in a port usage (other than ports) must be referential. | — |
| `validatePortUsageType` | `semantic/s26-port-typed-by-part-def.sysml` | both-reject | A port must be typed by port definitions. | A port must be typed by port definitions. | — |
| `validateReferenceUsageIsReference` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateRenderingUsageType` | `semantic/s27-rendering-typed-by-part-def.sysml` | both-reject | A rendering must be typed by one rendering definition. | A rendering must be typed by one rendering definition. | — |
| `validateRequirementConstraintMembershipIsComposite` | none: `require`/`assume` admit no `ref` prefix | no violating model | — | — | — |
| `validateRequirementConstraintMembershipOwningType` | `extensions/x08-require-outside-requirement-body.sysml` | both-reject | mismatched input 'require' expecting '}' / extraneous input '}' expecting EOF | `require` outside a requirement body is an OpenSysML extension with no SysML v2 production: only a requirement, concern, viewpoint or objective body admits it | — |
| `validateRequirementDefinitionOnlyOneSubject` | existing: `xpect/p14-requirement-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateRequirementDefinitionSubjectParameterPosition` | `semantic/s28-requirement-def-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateRequirementUsageOnlyOneSubject` | `semantic/s29-requirement-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateRequirementUsageSubjectParameterPosition` | `semantic/s30-requirement-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateRequirementUsageType` | `semantic/s31-requirement-typed-by-constraint-def.sysml` | both-reject | A requirement must be typed by one requirement definition. | A requirement must be typed by one requirement definition. | — |
| `validateRequirementVerificationMembershipKind` | none: `verify` fixes `kind = requirement` in the grammar | no violating model | — | — | — |
| `validateRequirementVerificationMembershipOwningType` | existing: `xpect/p29-verify-outside-objective.sysml` | both-reject | A requirement verification must be in the objective of a verification case. | A requirement verification must be in the objective of a verification case. | — |
| `validateSatisfyRequirementUsageReference` | `semantic/s32-satisfy-references-non-requirement.sysml` | both-reject | Must reference a requirement. | satisfy target must be a requirement usage, found constraintUsage | — |
| `validateStakeholderMembershipOwningType` | `grammar/g64-stakeholder-outside-requirement-body.sysml` | pilot-only-rejects | mismatched input 'stakeholder' expecting '}' / extraneous input '}' expecting EOF | — | `stakeholder` is dispatched body-independently in `parser/defusage.go`; no pass checks the owner kind |
| `validateStateDefinitionParallelSubactions` | existing: `xpect/p19-parallel-state-with-transition.sysml` | both-reject | A parallel state cannot have successions or transitions. | A parallel state cannot have successions or transitions. | — |
| `validateStateDefinitionSubactionKind` | existing: `xpect/p13-state-two-entry-actions.sysml` | both-reject | A state may have at most one entry action. | A state may have at most one entry action. | — |
| `validateStateSubactionMembershioOwningType` | `grammar/g65-entry-action-outside-state-body.sysml` | pilot-only-rejects | mismatched input 'entry' expecting '}' / extraneous input '}' expecting EOF | — | `parser/defusage.go` `atKindPrefix` takes `entry` as a kind prefix of `action` and drops it, so the member parses as a plain `action init` |
| `validateStateUsageParallelSubactions` | `semantic/s33-parallel-state-usage-with-succession.sysml` | both-reject | A parallel state cannot have successions or transitions. | A parallel state cannot have successions or transitions. | — |
| `validateStateUsageSubactionKind` | `semantic/s34-state-usage-two-entry-actions.sysml` | both-reject | A state may have at most one entry action. | A state may have at most one entry action. | — |
| `validateStateUsageType` | `semantic/s35-state-typed-by-part-def.sysml` | both-reject | A state must be typed by state definitions. | A state must be typed by state definitions. | — |
| `validateSubjectMembershipOwningType` | `grammar/g62-subject-outside-requirement-body.sysml` | pilot-only-rejects | mismatched input 'subject' expecting '}' / extraneous input '}' expecting EOF | — | `subject` is dispatched body-independently in `parser/defusage.go`; no pass checks the owner kind |
| `validateTransitionFeatureMembershipEffectAction` | none: `TransitionEffectMember` only parses an `ActionUsage` | no violating model | — | — | — |
| `validateTransitionFeatureMembershipGuardExpression` | none: not observed with the full library loaded | not enforced by the pilot | — | — | — |
| `validateTransitionFeatureMembershipOwningType` | none: `TransitionFeatureMembership` is only produced inside a `TransitionUsage` | no violating model | — | — | — |
| `validateTransitionFeatureMembershipTriggerAction` | none: `TriggerActionMember` only parses an `AcceptActionUsage` | no violating model | — | — | — |
| `validateTransitionUsageParameters` | none: `TransitionUsage` always synthesises its `EmptyParameterMember`s | no violating model | — | — | — |
| `validateTransitionUsageSuccession` | `semantic/s44-transition-target-not-action.sysml` | both-reject | A transition must own a succession to its target. | transition endpoint p is not a state or pseudostate | — |
| `validateTransitionUsageTriggerActions` | existing: `xpect/p34-accepter-source-not-state.sysml` | both-reject | A transition with an accepter must have a state as its source. | A transition with an accepter must have a state as its source. | — |
| `validateUsageIsReferential` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateUsageType` | `semantic/s36-usage-typed-by-feature.sysml` | both-reject | A usage must be typed by definitions. | A usage must be typed by definitions. | — |
| `validateUsageVariationIsAbstract` | none: adapter post-processing forces `isAbstract = true` | no violating model | — | — | — |
| `validateUsageVariationMembership` | `semantic/s37-variation-usage-member-not-variant.sysml` | both-reject | An owned usage of a variation must be a variant. | An owned usage of a variation must be a variant. | — |
| `validateUseCaseUsageReference` | none: constant declared, no check reads it | not enforced by the pilot | — | — | — |
| `validateUseCaseUsageType` | `semantic/s38-use-case-typed-by-case-def.sysml` | both-reject | A use case must be typed by one use case definition. | A use case must be typed by one use case definition. | — |
| `validateVariationMembershipOwningNamespace` | existing: `xpect/p08-variant-outside-variation.sysml` | both-reject | A variant must be an owned member of a variation. | A variant must be an owned member of a variation. | — |
| `validateVerificationCaseUsageType` | `semantic/s39-verification-typed-by-case-def.sysml` | both-reject | A verification case must be typed by one verification case definition. | A verification case must be typed by one verification case definition. | — |
| `validateViewDefinitionOnlyOnvViewRendering` | `semantic/s40-view-def-two-renderings.sysml` | both-reject | A view definition may have at most one view rendering. | A view definition may have at most one view rendering. | — |
| `validateViewRenderingMembershipOwningType` | `grammar/g66-render-outside-view-body.sysml` | pilot-only-rejects | mismatched input 'render' expecting '}' / extraneous input '}' expecting EOF | — | `render` is dispatched body-independently in `parser/defusage.go`; no pass checks the owner kind |
| `validateViewUsageOnlyOneRendering` | existing: `xpect/p30-two-view-renderings.sysml` | both-reject | A view may have at most one view rendering. | A view may have at most one view rendering. | — |
| `validateViewUsageType` | `semantic/s41-view-typed-by-part-def.sysml` | both-reject | A view must be typed by one view definition. | A view must be typed by one view definition. | — |
| `validateViewpointUsageType` | `semantic/s42-viewpoint-typed-by-requirement-def.sysml` | both-reject | A requirement must be typed by one requirement definition. / A viewpoint must be typed by one viewpoint definition. | A viewpoint must be typed by one viewpoint definition. | — |
| `validateWhileLoopActionUsageParameters` | none: `WhileLoopNode` always parses condition and body parameters | no violating model | — | — | — |
