# Pilot Xpect Expectations

## Overview

**Reference:** the OMG pilot implementation's own Xpect test suites, [`org.omg.kerml.xpect.tests`](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/2026-05/org.omg.kerml.xpect.tests) and [`org.omg.sysml.xpect.tests`](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/2026-05/org.omg.sysml.xpect.tests), at release `2026-05` — the same pin as the corpora and the reference validators (`scripts/pilot-pin.sh`)
**Provision:** `./scripts/download-pilot-xpect.sh` (writes `build/pilot-xpect-corpus/{kerml,sysml}`, gitignored, not vendored — under `build/` rather than `examples/` because the `.kerml`/`.sysml` models the suites ship are inputs to this harness, and everything that walks `examples/` would otherwise adopt them)
**Run:** `go run ./cmd/pilot-xpect` (writes `build/pilot-xpect/pilot-xpect.txt` and `build/pilot-xpect/pilot-xpect.json`)
**Baseline:** the last committed run is [pilot-xpect-baseline.json](pilot-xpect-baseline.json), which carries every non-agreeing row, so a later run can be diffed against it
**Status:** advisory only — nothing here gates CI, for the same reason [pilot-differential.md](pilot-differential.md) does not: the corpus is an unvendored network fetch at the pinned tag, and this is a report, not a ratchet

[pilot-differential.md](pilot-differential.md) compares us against *observed* pilot behaviour: it runs
the pinned validator and records what it says. That answers "do we agree with the reference?" but not
"was the reference's behaviour intended?" — the question that has kept several rows there advisory.

These `.xt` files are the other kind of evidence. Every assertion in them is a **declared**
expectation, written inline by the pilot's implementers next to the model it constrains. We read them
as data — no Java, no Xtext, no Eclipse — and adjudicate each one against our own front end.

The `linkedName` assertions are the point of the exercise: they are the first external oracle we have
for **name resolution**, the largest block of self-assessed rows in
[spec-compliance.md](spec-compliance.md). They are broken out in their own section below. The first
run of this harness put them at 151 of 194; after the wave-6 alias-identity round they are **194 of
194**, and that section now records what the oracle found and what closed it.

---

## What this oracle can and cannot adjudicate

Can:

- **Name resolution.** `linkedName` declares the qualified name a given reference must resolve to.
  This is a direct, per-reference verdict on our scoping, import, inheritance and alias handling.
- **Name *visibility*.** `scope` declares the complete set of names visible at a point, so it catches
  both halves of a scoping defect — a name that should be visible and is not, and a name that is
  visible and should not be. `linkedName` can only see the first half. See
  [scope](#scope--221-of-230-agree-exactly).
- **Diagnostic presence and placement.** `errors`/`warnings` declare a severity and a message at a
  source location; `noErrors` declares silence over a whole resource set.
- **The pilot's *intent*.** When we disagree with a declared expectation, the pilot's behaviour on
  that construct is not incidental — its implementers wrote the expectation down.

Cannot:

- **Execution semantics.** Nothing here exercises action or state execution. The suites contain no
  execution assertions, and the pinned pilot has no headless action/state execution surface at all
  (see [pilot-execution-referee.md](pilot-execution-referee.md), and issue #386 for the referee's
  scope). Nothing in this report bears on a behaviour row.
- **Diagnostic wording.** Our messages are our own; a declared message and ours can describe the same
  defect in different words, and the specification does not settle whose words are right. The harness
  therefore admits a `wording-only` row into agreement only when it can verify that both messages
  state the same rule about the same element. See
  [How agreement is decided](#how-agreement-is-decided).
- **What a declared library file leaves unresolved.** A fixture's resource set is often a subset of
  the library its own files reference — `Objects.kerml` without `Occurrences.kerml`, say. Those
  unresolved references arrive against the file under test, because name resolution resolves the
  whole index rather than one document, and the harness drops them by their location: it counts them
  as *foreign* and adjudicates only diagnostics inside the fixture's own model text.
- **Anything the suites do not cover.** These are the pilot's *tests*, not the specification. A
  construct with no assertion is not endorsed by their absence.

The comparison loads what each fixture declares: the `.xt` body itself, the `/src/` files its
`XPECT_SETUP` `ResourceSet` names, and exactly the `/library*` copies it names — never our embedded
standard library in their place. A declared resource absent from the download is reported as such,
never silently treated as loaded.

---

## How agreement is decided

Agreement is **strict** on substance. A row agrees when it agrees word-for-word, or when it is
**wording-only**: the same rule about the same element, at the same severity and the same offset, in
our words rather than the pilot's. Wording-only rows are counted inside agreement with their own
sub-count, because the wording is not something the specification settles and nothing new is detected
when one is admitted:

| Kind | Agrees when |
|---|---|
| `errors`, `warnings` | we report a diagnostic of the declared severity **at the declared offset**, whose message either matches the declared one (whitespace and a trailing period aside) or states the same rule about the same element in our own words |
| `noErrors` | we report **no error** anywhere in the declared resource set |
| `linkedName` | the reference at the declared text resolves, and the resolved element's qualified name **equals** the declared one |
| `scope` | the set of names we enumerate at the anchor **equals** the declared set, name for name, after filtering by the metatype the anchor's cross-reference admits |

Wording-only is not a tolerance and is not granted on span and severity alone: the harness requires
the declared and our message to state the same rule about the same element
(`cmd/pilot-xpect/wording.go`). A different rule landing on the same token stays a disagreement, and
those rows are what the `same-location` tolerance now holds.

No tolerance ever turns a disagreement into an agreement. Weaker rules are recorded beside each
disagreement, as evidence about *how far off* we are, and are summarized per kind in the report:

| Tolerance | Meaning |
|---|---|
| `same-location` | right severity at the declared offset, but a *different rule* — wording alone would be agreement |
| `same-line` | right severity on the declared line, at a different offset |
| `severity-differs` | a diagnostic of ours is there, of the *other* severity |
| `elsewhere-in-file` | right severity, but nowhere near the declaration |
| *(none)* | nothing of ours is there at all |

The `scope` kind has its own tolerance classes, on the same rule — none of them turns a disagreement
into an agreement:

| Tolerance | Meaning |
|---|---|
| `other-paths` | every declared name we do not offer names an element we *do* offer under a different path — a spelling difference, not a visibility one |
| `extra-names` | we offer names the declaration does not, and miss none |
| `missing-names` | we miss declared names and offer no extra ones |
| `missing-and-extra` | both |
| `library-names` | every difference is a path tail through the implicit members `Base` contributes (`self`, `that`) — see the scope section |

Two further honest limits of the mapping:

- `noErrors` is adjudicated as file-wide silence. Where a declared `noErrors` names a target, we still
  require silence everywhere, which is the stricter reading — so a `noErrors` disagreement always
  means we report *an* error, not necessarily one at the named target.
- A declared location is matched by finding the declared text after the assertion, ignoring
  whitespace. Text inside XPECT notes is masked out first so an assertion cannot match itself.

---

## Corpus and census reconciliation

`./scripts/download-pilot-xpect.sh` fetches **303 KerML + 125 SysML = 428** `.xt` files, exactly the
expected count. **0 files are unparsed** and **0 declared resources are missing**, so no assertion is
silently dropped.

The reader recovers **1261 assertions**, declaring **1323 individual expectations** (a single
`errors`/`warnings` note may list several diagnostics; each is one adjudicated row). Reconciled
against the published per-kind census:

| Kind | Published | `//`-form notes | `//*` block notes | Reader total |
|---|---:|---:|---:|---:|
| `errors` | 459 | 459 | 27 | 486 |
| `noErrors` | 275 | 275 | 0 | 275 |
| `linkedName` | 192 | 194 | 0 | 194 |
| `warnings` | 58 | 58 | 17 | 75 |
| `scope` | 1 | 1 | 229 | 230 |
| `exportedObjects` | — | 0 | 1 | 1 |
| **Total** | **985** | **987** | **274** | **1261** |

Every difference from the published numbers is accounted for, and none of it is the reader guessing:

- **Block notes (274).** `//* XPECT errors --- ... --- */` is the multi-line form of the same
  assertion, and the suites use it heavily — for all but one `scope` assertion, for 27 `errors` and
  17 `warnings`. A line-oriented census counts only the single-line form. These are real assertions
  and are adjudicated exactly like their single-line equivalents.
- **`linkedName` 192 → 194.** Two notes are written with a tab after the slashes
  (`//\t\tXPECT linkedName at aa --> test.A.a`) rather than `// ` or `//`, so a grep for the two
  common spellings misses them: `MemberNameTests_NamedMemberFromInheritance.kerml.xt`:41 and
  `MemberNameTests_NamedMemberFromInheritance_Rdef.kerml.xt`:50. Both are ordinary executable notes;
  both were disagreements on the first run, so dropping them would have flattered us by two rows.
- **`exportedObjects` (1).** One note in `indexing/NameEscape.kerml.xt` opens with a bare `//*` and
  puts `XPECT exportedObjects` on the following line. Wave 8F adjudicates it against our index, and
  it agrees, so no kind is reported as not adjudicated any more.
- **The `at` clause and unescaped quotes (−3 expectations).** Wave 11D corrected the reader: an
  `at "…"` clause runs to the last quote on its line, because Xpect does not escape the quotes
  inside it (`at "filter new A(null, 1, "", false);"`). Four assertions were previously read as two
  expectations each, a truncated one and a junk one, so the declared population is **1323**, which is
  what the totals below are measured over. See
  [wave11d-metadata-evaluability.md](wave11d-metadata-evaluability.md).
- **XPECT-shaped text that is not a note (10).** Eight `XPECT scope`/`XPECT errors` fragments sit
  inside `/* ... */` comments and two are disabled by their authors as `// (TBD) XPECT noErrors`.
  These open no `//` or `//*` note, so the harness does not run them; all ten are listed by file and
  line in the report's *XPECT-shaped text outside a note* section rather than being dropped.

---

## Totals

```
428 .xt file(s), 0 unparsed, 0 missing declared resource(s)
1261 assertion(s) declaring 1323 expectation(s)
agree 1269 (of which wording-only 246) | disagree 54 | unlocated 0 | not adjudicated 0
```

| Kind | Expectations | Agree | of which wording-only | Disagree | Not adjudicated | `same-location` | `same-line` | `severity-differs` | `elsewhere` | nothing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `errors` | 510 | 479 | 246 | 31 | 0 | 6 | 9 | 0 | 12 | 4 |
| `noErrors` | 275 | 263 | — | 12 | 0 | — | — | — | — | — |
| `linkedName` | 194 | 194 | — | 0 | 0 | — | — | — | — | — |
| `warnings` | 113 | 111 | — | 2 | 0 | 0 | 0 | 1 | 0 | 1 |
| `scope` | 230 | 221 | — | 9 | 0 | — | — | — | — | — |
| `exportedObjects` | 1 | 1 | — | 0 | 0 | — | — | — | — | — |

Per suite:

| Suite | Files | Expectations | Agree | Disagree | Not adjudicated |
|---|---:|---:|---:|---:|---:|
| `kerml` | 303 | 966 | 932 | 34 | 0 |
| `sysml` | 125 | 357 | 337 | 20 | 0 |

**Read the `errors` row carefully: 246 of its 479 agreements are wording-only, so more than half of
that column is us stating the pilot's rule in our own words rather than a rule written against its
text.** The 233 word-for-word rows are the ones where a rule was implemented against the declared
message. `warnings` shows the same effect from the other side: its 111 agreements are the
duplicate-member-name, visibility and wave-9C library rules written against the pilot's declared
wording, so they match by construction rather than by luck. `noErrors` and `linkedName` are
wording-independent, and they are where this oracle adjudicates most directly.

Movement since the first run of this harness (the harness itself is unchanged; every difference is a
change in our behaviour):

| Kind | First run | Now | What moved |
|---|---|---|---|
| `linkedName` | 151 / 194 | **194 / 194** | alias-introduced names resolve to the aliased element, and the `~ B::f` conjugation form parses |
| `noErrors` | 231 / 275 | **263 / 275** | 6 `ParsingTests_*` files, 4 inherited-name-conflict files and 2 others no longer draw an error, wave 9D's protected/shadowed path reconciliation cleared 11 more, and wave 11C closed the 6 protected-import rows wave 10E had made unsatisfiable by modelling `noErrors` as Xpect's residue rather than as file-wide silence |
| `warnings` | 0 / 113 | **111 / 113** | the duplicate-member-name warnings, the wave-8 rules written against the declared wording, wave 9C's library rules, wave 10's warnings residue, and wave 11A/11F's usage-typing rules, which stopped another rule's error from standing where a warning is declared |
| `errors` | 0 / 510 | **479 / 510** | 233 rows are ours word-for-word; the other 246 are wording-only, admitted centrally in wave 10 after the rule and element were checked, not by adopting the pilot's phrasing |
| `scope` | 73 / 230 | **221 / 230** | wave 9A resolves implicit and inherited members through the library (`library-names` 125 → 27), wave 9D reconciles the protected and shadowed paths, wave 10A bounds re-entry to one per name, and wave 11C fixes the quoted anchor and stops a recursive import's descent carrying implicit generals |

**These tables and the baseline are a single fresh run on `main` with wave 11 merged** (all seven
slices: 11A–11G). The largest movement in the wave is detection, not classification: the 11
`severity-differs` `errors` rows are **0**, because wave 11A implemented the usage-typing and
specialization rules the pilot declares there instead of adjusting a severity, and 11F canonicalized
the resolver's inherited-name warning that had been standing in their place. `errors` silence falls
8 → **4**, `elsewhere-in-file` 42 → **12**, `same-line` 25 → **9**, and `noErrors` rises 248 → **263**.

**What the wording-only class did and did not buy.** It moved 246 rows from `same-location` into
agreement without changing what we detect — the same severity, the same offset, the same rule, the
same element, our phrasing (`unresolved reference: A::a1 — did you mean …` against `Couldn't resolve
reference to Classifier 'A::a1'.`). **Nothing was newly detected by it, and the sub-count exists so
that a future reader cannot book the jump as detection.** The 6 rows left in `same-location` are the
interesting residue: there we flag the declared offset for a *different* reason than the pilot does,
such as `ParsingTests_BadScopeWithOnlyTwoDot.kerml.xt`:26, where it cannot resolve `test` and we say
the reference resolves to a package where a type is required.

**Wave 9's `errors` regression is closed by wave 10E, and the rule it restores is the one wave 9D
collapsed.** KerML resolves a qualified name by resolving its qualification and then asking that
namespace for the last segment through *visible* resolution (8.2.3.5.3, 8.2.3.5.4): only public owned
memberships and public imports are read, and what the *referring* namespace specializes is not
consulted. Only the first segment uses *local* resolution, which does include the protected members a
specialization inherits. Wave 9D applied the local rule to every segment, so a protected member became
nameable through any namespace path a specialization could write. Wave 10E takes the specialization
relaxation out of the qualified tail (`namedThroughNamespace` in `resolve/visibility.go`, applied by
`walkQualifiedTail`) and leaves local and inherited resolution, `import all` and scope enumeration
untouched — no divergence from the specification had to be recorded for it.

All 12 rows we had gone silent on are back at the pilot's own location, and the 2
`VisibilityTests_Protected_0.kerml.xt` rows that landed elsewhere in the file now land on the declared
references; all 14 are admitted as wording-only. **The trade wave 10E appeared to force is gone, and
it was the harness's reading rather than a contradiction in the corpus:** wave 10E cost `noErrors`
six rows because those fixtures declare file-wide silence *and* the protected-import errors it
restores, and wave 11C showed that Xpect scores a `noErrors` note against what its sibling
expectations leave over, not against total silence (`consumedLine`, `cmd/pilot-xpect/compare.go`).
With that modelled, the six rows close without exempting a fixture or widening the wording-only class,
and **no expectation in this suite is now recorded as unsatisfiable.** The 42 wave-8A
private/protected rejections are unchanged.

The `nothing` column counts what neither agreement nor any tolerance accounts for, so it subtracts
the agreements — including the wording-only ones, which are no longer counted under `same-location`.

The **complete** per-row evidence — every disagreement and every wording-only row with its file,
line, declared expectation and our actual behaviour — is in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json). The sections below group the disagreements by
cause and name the files; they do not repeat 300 rows.

---

## linkedName — 194 of 194 agree

**This oracle is now clean, and it was not clean when it was built: the first run agreed on 151 of
194.** The section is kept because what it found is the reason the harness exists, and because a
future regression here is the single most informative failure this report can produce.

### What it found — an alias resolved to itself instead of to its target (41 of the 43)

The pilot's `linkedName` reports the qualified name of the **element** a reference reaches. Where the
name reached it through an `alias`, that is the aliased element's own qualified name; we reported the
qualified name of the **alias**:

```kerml
package test {
    classifier A;
    alias A_alias for A;
    //XPECT linkedName at A_alias --> test.A
    classifier B specializes A_alias;
}
```

`testsuite/MemberNameTests_LocalNamedMember.kerml.xt`:37 declared `test.A` and we answered
`test.A_alias`. The reference *resolved* — nothing downstream broke — so what differed was the
identity of the thing it resolved to: our symbol index made an alias a symbol in its own right rather
than a second name for an existing element. The pilot was right. A KerML `alias` declares a
`Membership` with a name whose `memberElement` is the existing element: it introduces a name, not an
element, so the resolved element's qualified name cannot contain the alias.

All 41 were the single-declaration form `alias X for <target>;`, spread over targets in the same or an
imported package (28), inherited members (9) and private members (4) — one defect in
`internal/core/symbols`/`internal/core/resolve` plus an inheritance hop, not four.

### What it found — a member reached through a conjugated type was not a reference at all (2 of the 43)

```kerml
classifier B conjugates A;
//XPECT linkedName at B::f --> test.A.f
feature g ~ B::f;
```

`testsuite/MemberNameTests_NamedMemberFromConjugation.kerml.xt`:41 and :43 declared `test.A.f`; we
indexed no name reference at that offset at all, because we did not parse the `feature g ~ B::f;`
conjugation form. The root cause was in the parser, not in resolution, and the same file's `noErrors`
assertion failed with it.

### What closed them

Both causes closed in the wave-6 round (alias-introduced names resolving to the aliased element, and
the conjugation form parsing), and the same file's `noErrors` row closed with the second. The split
of the 43 between the individual PRs in that round is **not** separately measured here: this report
compares two revisions of the tree, not one PR at a time.

### What the agreements are worth

The 194 are not trivial passes: they cover qualified names through public and private imports,
`import ::*` recursion, short names and `<id>` forms, shadowing between an import and an inner
classifier, redefinition scoping, inherited-member lookup, and every alias shape above. This is the
only external, per-reference verdict on our name resolution that exists at the pinned tag, and it is
also the narrowest: it says which element a written reference reaches, never which names *were*
visible. That second question is the 230 `scope` assertions below.

## noErrors — 263 of 275 agree

12 disagreements: we report an error where the pilot's implementers declared the file clean. Grouped
by our first diagnostic:

| Cause | Rows | Read |
|---|---:|---|
| **Unresolved reference** — `unresolved member: c` (`ParsingTests_Indexing.kerml.xt`:32), `unresolved reference: physical` (`AllocationTest.sysml.xt`:31), `unresolved reference: MassValue — did you mean ISQBase::MassValue?` (`KernelLibraryTest.sysml.xt`:72) | 3 | **Ours to fix.** Import and indexing shapes where the reference resolves to nothing for us and to an element for the pilot; `linkedName`'s 194 agreements do not reach them, because those rows only ask about references that *do* resolve. |
| **Parse recovery** — `expected a namespace member`, `expected '{' or ';' after declaration` | 4 | **Ours is wrong.** Notation the reference accepts and we do not parse; each one cascades, so the count overstates the number of defects. The three `QPE-*` query-path-expression files and `SemanticMetadata_valid.sysml.xt`. |
| **Specialization cycle** — `x participates in a specialization cycle` | 3 | **Adjudicated divergence, not a defect of ours.** All three fixtures declare a real cycle: `part p1 :> p2; part p2 :> p3; part p3 :> p1;` and `part p4 :> p4;` (`simpletests/PartTest.sysml.xt`:67-71), `part def A :> C` with `part def C :> A, B` (`Redefinition_OwningType_Cyclic_Gen.sysml.xt`:28-34), and `classifier a specializes b` / `classifier b specializes a` (`SimpleImportTests_CircleInheritanceInCircleImport.kerml.xt`:29,37). The pilot has no such check at all — the finding F4/K5 settled in [pilot-differential.md](pilot-differential.md#specialization-cycles-f4) — so closing them would mean deleting a correct rule. |
| **Conformance** — `try (typed by a1) redefines b (typed by A): types do not conform` | 2 | **Adjudicated divergence**, decided in [wave11e-decisions.md](wave11e-decisions.md) (E4): a redefinition is a subsetting (KerML 7.4.9, 8.3.4.2), so a non-conforming type describes an unsatisfiable model; the pilot validates subsetting conformance nowhere, so its silence records an absent check. Both rows are `SimpleImportTestsFromOtherFile_Import3{,_FT}`. |

The **inherited-name-conflict** family that cost 4 rows on the first run is gone: those files are the
same defect as the `warnings` severity finding below, and making a duplicate inherited name a warning
rather than an error made all four files clean. The two state/transition rows
(`simpletests/StateTest.sysml.xt`:73, `DecisionTest.sysml.xt`:69) closed in wave 11 with the
transition-endpoint reading, not by relaxing the rule.

By suite: 4 KerML, 8 SysML. In every one of the 12 the declared expectation is *silence*, so every
one is a place where we reject something the reference accepts — the same class of finding as the
"only ours" column in [pilot-differential.md](pilot-differential.md), but here backed by a declared
intent rather than an observed verdict. **7 of the 12 are ours; the other 5 are adjudicated
divergences where the pilot has no check.** The kind's history is worth keeping in view, because it
moved in both directions: 244 → 243 across wave 8 (four parse rows closed, six visibility rows
opened), 243 → 254 in wave 9, 254 → 248 when wave 10E restored the protected-import rejections, and
248 → **263** in wave 11 once 11C modelled `noErrors` as Xpect's residue and closed those six
without giving the rejections back. No row here is unsatisfiable any more.

---

## warnings — 111 of 113, and the severity finding is closed

111 rows agree, all of them word-for-word: no `warnings` row is wording-only. The first 11 were duplicate-member-name warnings implemented in the wave-6
round from the pilot's declared text — 6 in `MembershipTests_Distinguishability.kerml.xt`, and 5
across the `Redefinition_Diamond*_invalid` / `RedefinitionDiamond*_invalid` pairs; wave 8 added 12
more, the multiplicity-upper-bound rule among them; **wave 9C added 66**, the library inherited-name
diamond chief among them, wave 10 closed 10 more, and wave 11 closed 12 — the nested
`perform b.a;` / `exhibit s.sa;` offsets and four of the five rows that sat behind another rule's
error. All of them match by construction rather than by luck, because each was written against the
declared text.

The remaining 2:

| Outcome | Rows | Read |
|---|---:|---|
| `severity-differs` — a diagnostic of ours **is** there, as an **error** | 1 | `InterfaceUsage_Invalid.sysml.xt`:78, where `Duplicate of inherited member name 'self' from Part, Port` is declared and our `An interface end must be a port.` error stands at the line. The declared warning needs the `end part ::> tankAssy.fuel;` subsetting chain the diamond rule does not follow. |
| nothing of ours there at all | 1 | `BindingConnector_Invalid2.sysml.xt`:42 — a shape the rule does not reach. |

**The severity defect the first run found is closed: it was 60 rows before wave 9C and is 0 now.**
The pilot declares:

```
//* XPECT warnings ---
"Duplicate of inherited member name 'p' from A2" at "feature redefines"
--- */
```

and on 60 rows we produced an error at that line until wave 9C made the rule a warning over library
bases. What remains of the family is 1 of the 2 disagreements, and it is not a severity disagreement
of this rule's own making: another rule's error stands at the line. The `Subsetting/redefining feature should not have
larger multiplicity upper bound` rule, 8 rows of nothing on the first run, and
`User library packages should not be marked as standard`, 1 row, are both implemented and agreeing.

One reading trap in the per-kind table above: the `nothing` column subtracts the agreements as well
as the tolerances, so it reads **1** — the row where nothing of ours is there at all.

### Wave 9C: the library diamond, as a warning

The severity finding above is what wave 9C addressed, and the four rules it adds live in
`internal/core/passes` (`w9c_inherited_name_conflict.go`, `w9c_owned_name_and_library.go`,
`w9c_bound_feature_types.go`), registered in `analyze.go` and level-scoped: the distinguishability
and library-package rules at `LevelNameResolution`, the diamond and bound-feature rules at
`LevelType` (`w9c_rules_test.go:TestW9CPassesAreRegistered`).

The diamond rule is the one that moved the family. The pilot keeps an ill-typed declaration and
unions the implicit base of the declaration's kind with the base its *declared* type reaches, then
names, per conflicting member, the types that declare it — which is why one declaration draws
`'self' from AnalysisCase, Part` beside `'start' from Action, Part`
(`CaseUsage_Invalid.sysml.xt:75-77`). The rule reproduces that union over library bases only, at
warning severity, and stays silent where the diamond is not real: a variant references a member of
its variation rather than specializing it, a redefinition of an untyped library feature replaces its
implicit value typing, and a member another candidate redefines is not inherited twice. It reads
library members through `symbols.Index.LookupDirectChildren`, so it behaves identically whether the
standard library was parsed or restored from cache (each focused test runs both states).

What is still open in the family, by reproducer:

| Rows | Reproducer | Why it still disagrees |
|---:|---|---|
| 1 | `InterfaceUsage_Invalid.sysml.xt:78` | `Part, Port` through an `end part ::> tankAssy.fuel;` subsetting chain, which the rule does not follow; an interface-end error of another rule is at the line. |
| 1 | `BindingConnector_Invalid2.sysml.xt:42` | `Bound features should have conforming types` on `rearWheel+1`: one endpoint is an expression, so the rule has no feature type to compare. |
| 0 | `ActionUsage_invalid.sysml.xt:61`, `StateUsage_invalid.sysml.xt:87`, `OccurrenceUsage_invalid.sysml.xt:59` | Closed in wave 11: the warning now lands on the nested `perform b.a;` / `exhibit s.sa;` reference usage and the `b.a` expression inside it, where the pilot reports it, rather than on the referenced declaration. |
| 0 | `Specialization_invalid.kerml.xt:56,60` | Closed in wave 11E, which runs `validateSpecializationSpecificNotConjugated` at the type tier so a metaclass error in the same file no longer hides it. |
| 0 | `AttributeUsage_invalid.sysml.xt:47,52` | Closed in wave 11 with the declared-type reading, without reintroducing the `'self' from DataValue, …` false positives across the pilot-corpora roots. |
| 0 | `ShadowingTests_ImportAndInnerClassesNamesAreTheSameBadCase3_Rdef.kerml.xt:28` | Closed in wave 11C — see below. Not a library diamond, so the resolver's own rule owns it. |

### Wave 11C: a supertype's imports are memberships it has

`ShadowingTests_ImportAndInnerClassesNamesAreTheSameBadCase3_Rdef.kerml.xt:28` declares
`Duplicate of inherited member name 'B' from OuterPackage` where `inner1 subsets inner` and `inner`
publicly imports `OuterPackage::*`. We were silent because `Resolver.inheritableMembers` collected
only a supertype's *owned* members. KerML 8.4.3.2 derives inherited memberships from the
supertypes' non-private memberships, and 8.3.3.1 makes a namespace's memberships its owned ones
*plus* its imported ones, so a name a supertype imported is inherited exactly as an owned one is —
and redeclaring it in the subtype is the distinguishability violation the fixture declares.

`Resolver.importedMembers` now contributes those, subject to the same two limits the owned side
already has: a private import contributes nothing, and library elements are excluded because library
supertypes are not walked (that family is `passes/w9c_inherited_name_conflict.go`, deliberately left
as the only producer over library bases). The redefinition in the fixture was a red herring: the
`hasUnresolvedRedefinition` suppression was not what swallowed the warning — the same file with the
redefinition removed was silent too.

---

## errors — 479 of 510, of which 246 wording-only

Agreement here is 233 rows word-for-word plus 246 wording-only: the same rule about the same element
at the same offset and severity, in our phrasing. Almost all of the wording-only rows are one family,
`Couldn't resolve reference to <kind> 'X'.` against `unresolved reference: X — did you mean …?`, and
the harness admits them only after matching the rule and the element named, never on span and
severity alone. What is left:

| Tolerance | Rows | Meaning |
|---|---:|---|
| `same-location` | 6 | we flag the exact declared offset for a **different rule** |
| `same-line` | 9 | we flag the declared line at a different offset — almost certainly the same defect |
| `severity-differs` | 0 | **empty:** wave 11A implemented the declared rules instead |
| `elsewhere-in-file` | 12 | we report errors, but not where the declaration points |
| nothing | 4 | **we accept a file the pilot's implementers declared invalid** |

The disagreements split 21 KerML / 10 SysML. The SysML suite's assertions anchor at a whole
declaration (`at "part def P { ... }"`) while ours land on the offending token inside it, so
`same-line` there often means what `same-location` means in KerML. Together, **494 of 510 declared
errors are ours at the declared location or line.**

**The 6 remaining `same-location` rows are the ones the wording-only class deliberately refuses.**
They sit at the declared offset with the declared severity and state a *different rule*, so admitting
them would have hidden four distinct divergences:

- **2 parse-shape rows** (`ParsingTests_BadScopeWithOnlyTwoDot.kerml.xt`:26,
  `ParsingTests_BadScopeWithOnlyTwoSingleDotAtTheEnd.kerml.xt`:26) where the pilot cannot resolve
  `test` at all and we resolve it and reject the *kind* (`type must be a type, found package`) — the
  rows here where our answer is arguably the more precise one.
- **2 bare-import rows** (`ParsingTests_Import_Visibility.kerml.xt`:23,
  `Import_Visibility_Invalid.sysml.xt`:23), which wave 10C's D2 moved *into* this class: our
  `import without a visibility indicator: S` is now an error by default rather than a warning, so
  these left `severity-differs`. The pilot rejects the same line as a syntax error instead.
- **`InterfaceUsage_Invalid.sysml.xt`:49**, where the pilot counts connector ends and we require an
  interface end to be a port.
- **`TransitionUsage_invalid.sysml.xt`:45**, where the pilot reports the ANTLR failure
  (`A parallel state cannot have successions or transitions`) and we report what our recovery expected
  — same defect, differently attributed, and the declared text is a parser-internal message we would
  not adopt.

The two `Specialization_invalid.kerml.xt` specialization rows and
`CaseSubjectObjective_Invalid.sysml.xt`:80 left this class in wave 11: 11A implemented the
specialization metaclass rules and 11E re-attached the conjugation rule at the type tier, and the
objective count is now ours.

**The `nothing` column continues to fall: 49 → 8 in wave 10, 8 → 4 in wave 11**, and the four rows
left are each classified in [wave11e-decisions.md](wave11e-decisions.md):

- **`ParsingTests_ScopeWithFourDotAndDot.kerml.xt`:22 (two rows)** — we resolve a name the pilot does
  not, a visibility/path-shape question rather than a missing rule.
- **`ConnectorTest_ConnectorEndSubsettingBadCase.kerml.xt`:31** — **our defect** (E3): the declared
  `Couldn't resolve reference to Feature 'f'.` is a resolution verdict about which features a
  connector end may name, owned by the resolver rather than by any `passes` rule.
- **`Type_Multiplicity_invalid.kerml.xt`:20** — **unimplemented obligation** (E1): `Only one
  multiplicity is allowed` is a validation error on a form our parser has no production for, so the
  rule cannot be reached until the parser accepts the surplus member.

The four `Feature_invalid_noType` rows — `Features must have at least one type` and its implicit-base
half — closed in wave 11E, which implemented both halves in both suites.

**`severity-differs` is empty, and that is this wave's clearest result.** Every one of its 11 rows
declared a typing or specialization rule (`An action must be typed by action definitions.`,
`Cannot specialize class or association`) while what we put on the line was the inherited-name
warning. They were missing detection wearing a cosmetic label: wave 11A implemented the six rules,
11F added the use-case analogues and canonicalized the resolver's inherited-name warning so it stops
standing in for them, and the column closed by implementation rather than by relabelling.

Every row that is not word-for-word — all 246 wording-only and all 31 disagreements — is recorded
individually with the declared message and ours in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## scope — 221 of 230 agree exactly

`scope` is a different oracle from everything else here. `linkedName` asks what one reference
resolves to; a `scope` assertion declares **the complete set of names visible at a point**, so it
sees the half of a scoping defect nothing else we run can see — a name that is visible and should not
be. All 230 live in the KerML suite; 229 of them use the `//*` block form.

### What the assertion means

The pilot's Xpect method takes the offset's `EObject` **and its cross-reference**, asks the scope
provider for the `IScope` that reference sees, and compares the declared list against
`scope.getAllElements()`:

```java
IScope scope = scopeProvider.getScope(arg1.getEObject(), arg1.getCrossEReference());
expectation.assertEquals(new ScopeAllElements(scope), new IsInScope(converter, scope));
```

Read literally, that fixes six things, and each one had to be reproduced to adjudicate a row:

- **The anchor is a reference, not a position.** `XPECT scope at P1::A` anchors at the cross-reference
  written after the note; the metatype of that reference filters the result, so a specialization
  reference sees types and a feature-typing reference sees features. Where a `scope` note anchors at a
  declaration rather than a reference, the harness adjudicates the unfiltered scope at that offset.
- **Every spelling is a separate entry.** The declared lists are qualified-name *paths*, not elements:
  `A`, `P1.A`, `Import_Circular.P1.A` are three declared names for one element, and a path continues
  through members — `Test1.x.that.self` is declared as well as `Test1.x`.
- **Inherited, imported and recursively imported members are in**, under every path that reaches them.
- **Implicit library members are in** when the fixture's resource set loads the library file that
  declares them: `Base.kerml` gives a `classifier` `self` (hence `A.self`, `A.self.that`) and a
  `feature` both `self` and `that`.
- **A circular containment truncates.** `classifier A { import B::*; classifier B specializes A }`
  declares `A.B.B` and stops there.
- **Order is not meaningful** — the comparison is set-based. Our own output is sorted by name so two
  runs are byte-identical.

### The surface

`Workspace.VisibleNames` / `VisibleNamesAt` (`internal/core/model/scope_names.go`) is a read-only
enumeration over the same resolver the LSP and `resolve/unqualified.go` use — `Resolver.ResolveName`,
`visibility.go`'s import and specialization rules, `semantics.MembersOf` — with no parallel scope walk
in the harness, which is the only way the answer can be evidence about *our* behaviour rather than
about the harness. It walks the anchor scope and its ancestors, their inherited and imported members,
and the non-library index roots, emitting each path once, and returns `{Name, FQN, Kind, Depth}` so a
caller can filter by metatype above it. `internal/core/model/scope_names_test.go` locks locals,
private and protected members, imports, alias binding names, short-name membership imports,
inheritance through typing, library gating, circular imports, redefinition anchors and determinism.

### The result, by class

| Class | Rows | Reading |
|---|---:|---|
| agree (exact) | 221 | the declared set, name for name |
| `library-names` | 6 | differs **only** in path tails through `Base`'s implicit `self`/`that` |
| `extra-names` | 2 | we offer names the pilot does not, and miss none |
| `missing-and-extra` | 1 | both |
| `missing-names` | 0 | — |
| `other-paths` | 0 | — |

**This is a worklist, not a verdict, and it must not be averaged into a percentage.** Exact agreement
on sets that routinely run past 50 entries is real evidence that our visible-name computation is
broadly right; the class that dominated it through wave 8 — implicit members, 96 rows then 125 — was
two separate defects, both fixed in wave 9A, leaving 28 rows of a third, narrower one. Wave 9D then
emptied `other-paths` and took agreement to 183; wave 10A's one-re-entry bound and per-anchor
accounting took it to 212, cutting `library-names` 28 → 8 and `missing-and-extra` 14 → 3 — see item
3. Wave 11C took it to **221** by fixing the quoted `scope` anchor and stopping a recursive import's
descent from carrying implicit generals; the nine rows left are the expansion-bound question below,
which is an adjudication rather than a defect list.

1. **`library-names` (6 rows) — a typed feature is still offered the implicit members its type's
   own supertype would contribute.** Two distinct defects were reproduced and fixed here first, each
   measured on its own:
   - A KerML `class`/`struct`/`assoc`/`behavior`/`predicate` declaration is parsed as a usage node,
     and was therefore given a usage's implicit base (`Base::things`) and a feature's implicit base
     feature on top of its classifier supertype. That reached `Base`'s `self`/`that` directly, so a
     `class` inherited them even where the resource set omits `Occurrences`, which declares its real
     supertype. Reproducer: `imports/global/DependencyPackageAlias0_A_alias.kerml.xt`:22, whose
     resource set names `/library/Base.kerml` and `/src/DependencyPackageAlias1.kerml` only, declares
     20 names and got 92 — the extra ones all tails of `.self`/`.that`. Fixed in
     `internal/core/semantics/implicit.go`; locked by
     `TestVisibleNamesClassImplicitMembersNeedOccurrences`.
   - The enumeration re-derived through a type it had already inherited through on the same path, so
     `self` (declared in `Base::Anything`) re-expanded through `Base::things` and back, emitting
     `.self.that.self` and longer tails the declaration does not contain. The pilot's own notes fix
     the depth: `visibility/VisibilityTests_PublicImportAsFeature.kerml.xt`:27 declares `Try.self`
     and `Try.self.that` and stops. Fixed by tracking the derivation steps a path has taken in
     `internal/core/model/scope_names.go`; locked by `TestVisibleNamesImplicitMembersDoNotRechain`.
   **The earlier explanation — the harness substituting our whole embedded library — stays
   falsified.** What remains is narrower and unfixed: where a fixture declares `feature f : C` with
   `C` a `class` and does not load `Occurrences`, the pilot offers `f` no implicit members at all
   while we still offer `f.self`, `f.that`, `f.that.self`
   (`visibility/VisibilityTests_PublicImportAsFeature.kerml.xt`:27, 18 extra names). Suppressing the
   implicit base for any feature with a declared generalization was measured and is wrong — it moves
   this class from 27 rows back to 30 — so the rule is subtler. The six rows that remain are the
   `CircleProblem4` family, the two `ImportPackageAndInheritanceFromContainer` variants and
   `Import_Recursive3`, all of them `self`/`that` tails and all of them hanging off the expansion
   bound the wave-11C section below leaves open.
2. **`extra-names` (2 rows) — the two `ShadowingTests_CircleProblem3.kerml.xt` anchors.** The
   redefinition-masking defect that filled this class is gone: it was 32 rows, 28 of them `*_Rdef`
   fixtures where a `redefines` did not mask the inherited name it redefines, the wave-8 masking round
   in **`internal/core/semantics`** closed all but three, and wave 11C's recursive-import descent
   closed those. What is left is the deepest pair of circular fixtures, where our enumeration offers
   names the pilot's expansion bound stops before — the same open question as item 1.
3. **`other-paths` (0 rows) — emptied in wave 9D, and five of its rows overshoot now.** This class
   held 11 rows: circular containment truncated one step earlier for us, so
   `ShadowingTests_CircleProblem2.kerml.xt`:22 was missing `A.B.B` while offering `A.B` and `B.B`.
   Wave 9D emits the re-entry, and `CircleProblem2` and the
   `ShadowingTests_SameNames*` family agree exactly — which is most of its +11. But on the deeper
   circular fixtures the re-entry does not stop where the pilot stops: at
   `ShadowingTests_CircleProblem3.kerml.xt`:23 the note declares 829 names and we now offer **3362**,
   2871 of them extra. `CircleProblem4` and its `_FT`/`_Rdef` variants were recorded here as behaving
   the same way; re-measured for D3 they do not — on those six rows the pilot declares *more* names
   than we offer (111 vs 67, 76 vs 64, 98 vs 86 twice over), and `A.B.B` is the first missing name in
   each. Those eight rows are why `missing-and-extra` rose 6 → 14, and an enumeration that overshoots by that much
   is a worse answer than one that truncated, even though the class label improved. **Bounding the
   re-entry is a wave-10 item**, and it is the one place where this wave's scope movement is not a
   straight gain. **Adjudicated in [wave10-decisions.md](wave10-decisions.md) (D3):** the bound is
   per name rather than per depth — no name appears more than twice in any declared path in the
   corpus — and the eight rows are three defects, only two of which the bound reaches.
4. **`missing-names` (0 rows) and `missing-and-extra` (1 row) — the import-plus-inheritance family is
   closed, and the one row left is a harness question.** The reproducer
   `imports/SimpleImportTests_ImportPackageAndInheritanceFromContainer.kerml.xt`:23, where
   `classifier A { public import test::*; classifier a specializes A; }` declares `A.A`, `A.A.a` and
   `A.a.A` paths we did not offer, agrees exactly since wave 11C made an imported member re-offered
   through the namespace's own inherited paths in **`internal/core/resolve`**. The remaining row is
   `imports/recursive/ShortName_Import_Valid1.kerml.xt`:25, where the pilot's `at c_Public` matches
   inside `c_Public_Id` and ours does not — a **pilot limitation (harness)**, classified below.

Short-name membership imports were the one defect inside this slice's ownership and are fixed in the
surface: `public import VP::VP2::A_Id` where the element is declared `classifier <'A_Id'> B` now
surfaces both of the element's names, as `imports/ShortName_Import_Valid4.kerml.xt` declares, while an
alias membership import still surfaces the alias name only.

The per-row evidence — declared count, our count, and the first missing/extra names for each
disagreement — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## Wave 11C — the global namespace, and what the self/that residue really is

Four adjudications and one open question came out of the 34 rows wave 11C owned (all 18 `scope` rows
and the 16 `noErrors` rows). They are recorded here because each one decides what an answer *means*,
not merely how it is computed.

**1. Two root namespaces of the same name are not an ambiguity.**
`imports/global/DependencySamePackageName.kerml.xt` loads two files that each declare
`package SamePackage`, declares the file clean, and declares both subtrees visible under one name
(`SamePackage.container.A` *and* `SamePackage.container.B`, with `SamePackage` itself listed twice).
Its own authors left the comment `//What global scope should be?????`. Resolution in the global
namespace is single-valued in KerML 8.2.3.5 — a qualified name resolves to one membership or to
none — and distinguishable naming constrains a *Namespace's* own members, which the global namespace
is not. So the repeat is not ill-formed and the first root is the answer:
`Resolver.lookupGlobalTop` returns `syms[0]` instead of reporting `ambiguous`, and a qualified tail
walked from one namespace no longer picks up the same-named other one's members
(`notConflatedWith`, `internal/core/resolve/qualified.go`). That closes the four
`ambiguous reference: SamePackage::container (2 candidates)` `noErrors` rows and the
`DependencySamePackageName` `scope` row, which was missing exactly the second root's paths.
`TestNameResolutionPassResolvesARepeatedTopLevelNameToTheFirst` replaces the ambiguity test that
asserted the opposite; a `$::`-rooted name is exempt from the filter, since it names a path in the
global namespace where every root's members are reachable.

**2. `noErrors` is Xpect's residue, not "no error anywhere".** Xpect matches each issue against the
expectations' regions and fails a file on what is left over, so an error a sibling `errors`
expectation declares is not the file's residue. The harness now models that
(`consumedLine`, `cmd/pilot-xpect/compare.go`) — which is what the six protected-import
contradictions above were really about, and it closes them without exempting a fixture or extending
the wording-only class.

**3. A `scope` anchor may be spelled with quotes.** `scoping/ShortName_Scoping_Valid1.kerml.xt`
anchors `at 1` on the reference `specializes '1'`; the declared text omits the quotes our span
carries, so the harness indexed no reference there and adjudicated the *unfiltered* scope. Accepting
a segment that starts one byte before the located text fixes the anchor, not the enumeration.

**4. A recursive import's descent carries no implicit generals.** A membership named directly by an
import keeps its implicit members; a name the recursive descent *finds* is entered through the
import, and the path does not then traverse the general types the element has by its kind rather
than by declaration (`Model.ImplicitGenerals`, `internal/core/semantics/implicit.go`, applied by
`declaredSources` in `scope_names.go`). That is what closes `Import_Recursive1`, `_4` and `_5`.

**The open question — the pilot's expansion bound is not the one wave 10 recorded.** Six rows remain
whose extras or omissions are all `self`/`that` tails, and the pair
`ShadowingTests_CircleProblem4.kerml.xt` / `_FT` shows the bound is not a name-occurrence count:
the two fixtures differ only in `classifier A specializes A::B` versus `feature A : A::B`, and the
plain form declares `b.B.b.self` while the `_FT` form declares `b.B.b` and stops. Every `_FT` path
that stops one step early ends where a *feature* is re-entered, and the plain form's do not, so the
bound looks like a budget on the derivation steps a path takes through a type rather than on repeated
names. We have no specification statement that fixes either bound, and adopting the pilot's stopping
points without one would be fitting output. **This is an adjudication question, not a defect**, and
with it stand the two `ShadowingTests_CircleProblem3.kerml.xt` rows: 456 names at `A` against 829
declared and 382 at `B` against 696, where per-anchor filtering between two mutually
wildcard-importing scopes and the `A.B.B` re-entry both hang off the same bound.
`imports/recursive/ShortName_Import_Valid1.kerml.xt` is a seventh, and a harness question rather
than ours: its `at c_Public` matches inside `c_Public_Id` in the pilot, which anchors it on a
classifier reference in `Test1`, while our identifier-boundary rule walks past it to a declaration in
`VP::VP1`. Relaxing the rule globally costs ten other rows, so the anchor needs Xpect's own matching,
not a looser one.

### The rows wave 11C leaves open, each labelled

An open row with no label reads as a defect, so every one of the twelve is classified here. The
categories are: **our defect** (the specification says one thing and we do another), **spec-derived
obligation not implemented** (we agree what is required and have not built it), **pilot limitation**
(the reference is the one departing from the specification, or the harness cannot express the
question), and **adjudicated divergence** (a difference we keep deliberately, with the reasoning
recorded).

| Row | Shape | Category |
|---|---|---|
| `ShadowingTests_CircleProblem4.kerml.xt`:32 | 30 missing / 1 extra, all `self`/`that` tails | **Open adjudication — not yet classifiable.** The expansion bound is underdetermined by the specification (the question above); the mismatch is real but no reading makes it our defect or the pilot's until the bound is settled. |
| `ShadowingTests_CircleProblem4_FT.kerml.xt`:43 | 3 extra: `b.B.b.self`, `.that`, `.that.self` | same |
| `ShadowingTests_CircleProblem4_Rdef.kerml.xt`:43 | 3 extra, identically | same |
| `SimpleImportTests_ImportPackageAndInheritanceFromContainer_FT.kerml.xt`:23 | 3 extra: `a.A.a.self`, `.that`, `.that.self` | same |
| `SimpleImportTests_ImportPackageAndInheritanceFromContainer_Rdef.kerml.xt`:23 | 3 extra, identically | same |
| `Import_Recursive3.kerml.xt`:55 | 2 missing (`s.self.that`), both reachable by another path | same |
| `ShadowingTests_CircleProblem3.kerml.xt`:23 | 456 of 829, 21 extra | same, plus the per-anchor filtering carry-over from wave 10A, which hangs off the same bound |
| `ShadowingTests_CircleProblem3.kerml.xt`:183 | 382 of 696, 14 extra | same |
| `ShortName_Import_Valid1.kerml.xt`:25 | declared 75, ours 170 at a different anchor | **Pilot limitation (harness).** The pilot's `at c_Public` matches inside `c_Public_Id`; reproducing that needs Xpect's own substring matching, and loosening our identifier-boundary rule costs ten other rows. |
| `SimpleImportTests_CircleInheritanceInCircleImport.kerml.xt`:18 | `a participates in a specialization cycle` | **Adjudicated divergence.** The fixture declares `classifier a specializes b` / `b specializes a`; the pilot has no cycle check (finding F4/K5 in [pilot-differential.md](pilot-differential.md#specialization-cycles-f4)). Closing the row would mean deleting a correct rule. |
| `simpletests/PartTest.sysml.xt`:29 | `p1 participates in a specialization cycle` | same — `p1 :> p2 :> p3 :> p1` and `p4 :> p4` |
| `Redefinition_OwningType_Cyclic_Gen.sysml.xt`:25 | `A participates in a specialization cycle` | same — `A :> C` with `C :> A, B` |

None of the twelve is a *spec-derived obligation we have not implemented*: the eight scope rows are
enumeration-bound questions, the ninth is a harness one, and the three `noErrors` rows are a rule we
do implement and the reference does not.

### Oracle movement, measured on both trees

Control `main` at `b86aeb18` against `main` at `ae4fdf9e` (both wave-11C PRs merged, and the 11A/11B
and wave-12 commits between them):

| Oracle | `b86aeb18` | `ae4fdf9e` |
|---|---|---|
| xpect | 1197 agree (239 wording-only) / 129 disagree | 1221 agree (241 wording-only) / 105 disagree |
| differential | 353 files, **312** fully agreeing; 25 agreed, **139** only ours, 73 only the pilot's | identical |
| rejection | 120 cases: 115 both reject, 5 only the pilot, 0 only us | identical |

**Both columns are now known to be understated, and the reconciliation the note below asked for is
done: the discrepancy was a stale on-disk library index cache, not a measurement dispute.** Records
in `~/.cache/sysml-ls/libs` were keyed on content, library-set digest and record format but not on
the binary, so records written by a pre-wave-11 build stayed "valid" across every merge and changed
what later builds saw. Re-running the same commits with a fresh `XDG_CACHE_HOME` gives higher
agreement in every oracle, identically cold and warm. The cache key now includes a build identity and
a library is index-only on both load paths, so a hit and a miss expose the same state
(`internal/core/libs`). Every figure in this document is a fresh-cache run on `main` with all of wave
11 merged; the two columns above are kept as recorded history rather than corrected in place.

The wave-11 handoff quoted the differential at `b86aeb18` as `311` fully agreeing with `142` only
ours, against the `312`/`139` above: the same cache effect, and the difference is not averaged
anywhere.

---

## Not adjudicated

One assertion is read, counted and reported, and deliberately not scored:

| Kind | Assertions | Why |
|---|---:|---|
| `exportedObjects` | 1 | `indexing/NameEscape.kerml.xt` — declares the index contents for escaped names. One assertion, no general mechanism worth building. |

They are `not-adjudicated` in the report and the baseline, never counted as agreements.

---

## What this report does not fix

**No disagreement recorded here is fixed by this document**; it records what the oracle says, and the
fixes are scoped from it in later rounds. The first two items of the original list are now closed
(alias identity, and the duplicate-member-name warnings existing at all); what is open, in the order
this report reads it:

1. **`Duplicate of inherited member name` location and coverage** — the severity half is closed by
   wave 9C (60 rows ours as an error → 0) and wave 11 closed the offsets; 1 `warnings` row sits behind
   another rule's error and 1 draws nothing.
2. **The 4 declared errors we do not report** — down from 195 on the first run, 49 after wave 9D, 20
   before wave 10E restored the protected-import rejections and 8 after it. Each is classified in
   [wave11e-decisions.md](wave11e-decisions.md): two are references we resolve and the pilot does not,
   one is a resolver defect (E3) and one an unimplemented obligation blocked behind a missing parser
   production (E1).
3. **The 4 remaining parse-recovery `noErrors` rows** — notation the reference accepts and we reject,
   the three `QPE-*` query-path-expression fixtures and `SemanticMetadata_valid.sysml.xt`.
4. **The 3 unresolved-reference `noErrors` rows** — the import/indexing family, which `linkedName`'s
   194 agreements do not reach. The protected-import rows are **not** on this list and are no longer a
   contradiction either: wave 11C closed them by scoring a `noErrors` note against Xpect's residue
   rather than against file-wide silence.
5. **The 9 `scope` disagreements**, with the class breakdown and root-cause packages in
   [scope](#scope--221-of-230-agree-exactly): 8 of them hang off the expansion bound the
   specification does not fix — `Base`'s `self`/`that` tails on the circular and
   import-plus-inheritance fixtures — and the ninth is the pilot's own substring anchoring.
6. **The 5 rows we keep deliberately** — 3 specialization-cycle `noErrors` rows, where the reference
   has no cycle check at all, and the 2 `SimpleImportTestsFromOtherFile_Import3{,_FT}` rows, where it
   validates subsetting type conformance nowhere (E4). Both are adjudicated divergences, not backlog.

The KerML validation and visibility residue wave 11E left open is enumerated with a category and an
owner per row in [wave11e-decisions.md](wave11e-decisions.md): **E1** `Type_Multiplicity_invalid` and
**E2** `AssociationTest_CrossFeatures_invalid` are unimplemented obligations owned by the parser,
**E3** `ConnectorTest_ConnectorEndSubsettingBadCase` and **E5**
`VisibilityTests_Protected_FeatureChaining` are our defects owned by the resolver, and **E4** is the
adjudicated divergence above. Read that page beside this one: an open row with no category reads as a
defect, and two of these six are not.
