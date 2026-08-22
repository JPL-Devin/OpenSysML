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
  [scope](#scope--183-of-230-agree-exactly).
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
  defect in different words. Strict agreement requires the declared message, so an `errors` row can
  only agree by coincidence of wording. See [How agreement is decided](#how-agreement-is-decided).
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

Agreement is **strict**, and strict is what goes in the baseline:

| Kind | Agrees when |
|---|---|
| `errors`, `warnings` | we report a diagnostic of the declared severity **at the declared offset**, whose message matches the declared one (whitespace and a trailing period aside) |
| `noErrors` | we report **no error** anywhere in the declared resource set |
| `linkedName` | the reference at the declared text resolves, and the resolved element's qualified name **equals** the declared one |
| `scope` | the set of names we enumerate at the anchor **equals** the declared set, name for name, after filtering by the metatype the anchor's cross-reference admits |

No tolerance ever turns a disagreement into an agreement. Weaker rules are recorded beside each
disagreement, as evidence about *how far off* we are, and are summarized per kind in the report:

| Tolerance | Meaning |
|---|---|
| `same-location` | right severity at the declared offset, different wording |
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

The reader recovers **1261 assertions**, declaring **1326 individual expectations** (a single
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
- **XPECT-shaped text that is not a note (10).** Eight `XPECT scope`/`XPECT errors` fragments sit
  inside `/* ... */` comments and two are disabled by their authors as `// (TBD) XPECT noErrors`.
  These open no `//` or `//*` note, so the harness does not run them; all ten are listed by file and
  line in the report's *XPECT-shaped text outside a note* section rather than being dropped.

---

## Totals

```
428 .xt file(s), 0 unparsed, 0 missing declared resource(s)
1261 assertion(s) declaring 1326 expectation(s)
agree 845 | disagree 481 | unlocated 0 | not adjudicated 0
```

| Kind | Expectations | Agree | Disagree | Not adjudicated | `same-location` | `same-line` | `severity-differs` | `elsewhere` | nothing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `errors` | 513 | 95 | 418 | 0 | 232 | 62 | 24 | 51 | 49 |
| `noErrors` | 275 | 254 | 21 | 0 | — | — | — | — | — |
| `linkedName` | 194 | 194 | 0 | 0 | — | — | — | — | — |
| `warnings` | 113 | 89 | 24 | 0 | 3 | 0 | 4 | 13 | 4 |
| `scope` | 230 | 212 | 18 | 0 | — | — | — | — | — |
| `exportedObjects` | 1 | 1 | 0 | 0 | — | — | — | — | — |

Per suite:

| Suite | Files | Expectations | Agree | Disagree | Not adjudicated |
|---|---:|---:|---:|---:|---:|
| `kerml` | 303 | 968 | 642 | 326 | 0 |
| `sysml` | 125 | 358 | 203 | 155 | 0 |

**Read the `errors` row carefully: strict agreement demands the pilot's message text, so its 95 is a
coincidence of wording where a rule was implemented against the declared text, and the interesting
number remains the tolerance breakdown.** `warnings` shows the same effect: 89 of 113 agree
strictly, and they are the duplicate-member-name, visibility and wave-9C library rules written
against the pilot's declared wording, so they match by construction rather than by luck. `noErrors` and `linkedName` are
wording-independent, and they are where this oracle adjudicates most directly.

Movement since the first run of this harness (the harness itself is unchanged; every difference is a
change in our behaviour):

| Kind | First run | Now | What moved |
|---|---|---|---|
| `linkedName` | 151 / 194 | **194 / 194** | alias-introduced names resolve to the aliased element, and the `~ B::f` conjugation form parses |
| `noErrors` | 231 / 275 | **254 / 275** | 6 `ParsingTests_*` files, 4 inherited-name-conflict files and 2 others no longer draw an error, and wave 9D's protected/shadowed path reconciliation clears 11 more; one file gained an error a wave-8 rule reports |
| `warnings` | 0 / 113 | **89 / 113** | the duplicate-member-name warnings, the wave-8 rules written against the declared wording, and wave 9C's library rules: inherited-name diamonds, short names, user standard libraries and non-conforming bindings |
| `errors` | 0 / 513 | **95 / 513** | wave-8 rules written against the declared text agree word-for-word; the tolerance mix moved with them, and wave 9D moved 12 rows out of `same-location` into silence (see below) |
| `scope` | 73 / 230 | **183 / 230** | wave 9A resolves implicit and inherited members through the library (`library-names` 125 → 27), and wave 9D reconciles the protected and shadowed paths |

**These tables and the baseline are a single fresh run on `main` at `8faed39a`**, with wave 9 merged.
The largest single movement remains `errors`: 146 rows we were silent on at the first run now draw a
diagnostic, so "nothing at all" falls from 195 to 49 while strict agreement rises from 0 to 95.

**Wave 9's `errors` column moved the wrong way, and the trade is deliberate rather than hidden.**
Silence rose 37 → **49**: wave 9D's `fix(resolve): reconcile protected and shadowed paths` stops us
reporting 12 declared errors we used to report at the pilot's own location — 4 rows in
`VisibilityTests_ImportAsFeatureInheritance_1.kerml.xt`, 8 across `VisibilityTests_ProtectedImport_0`,
`_1`, `_3`, `_4` and `_5`, plus 2 in `VisibilityTests_Protected_0.kerml.xt` that survive as
`elsewhere-in-file`. The same change is what lifts `noErrors` 243 → 254 and `scope` to 183: we now
make protected members visible where the pilot makes them visible, and the cost is that we no longer
reject the protected imports it rejects. Under-rejection on protected import is the wave-10 item; the
42 wave-8A private/protected rejections it was checked against are unchanged.

The `nothing` column counts what neither strict agreement nor any tolerance accounts for. It was
equal to `rows - tolerances` for as long as the strict column was 0, and both doc guards computed it
that way; with `errors` and `warnings` now agreeing strictly, they subtract the agreements too.

The **complete** per-row evidence — every disagreement with its file, line, declared expectation and
our actual behaviour — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json). The sections
below group the disagreements by cause and name the files; they do not repeat 510 rows.

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

## noErrors — 254 of 275 agree

21 disagreements: we report an error where the pilot's implementers declared the file clean. Grouped
by our first diagnostic:

| Cause | Rows | Read |
|---|---:|---|
| **Unresolved / ambiguous reference** — e.g. `ambiguous reference: SamePackage::container (2 candidates)`, `unresolved reference: MassValue — did you mean ISQBase::MassValue?` | 8 | **Ours is wrong**, and this family *halved* in wave 9: 18 → 8. The six `A::c_Protect` visibility rows wave 8 had added are gone, closed by 9D's protected/shadowed path reconciliation, and 9A's library resolution closed four more. What is left is the shadowing/import shapes, which `linkedName`'s 194 agreements do not reach because these references resolve to nothing at all rather than to the wrong element. |
| **Parse recovery** — `expected a namespace member`, `expected '{' or ';' after declaration`, `expected ')'` | 6 | **Ours is wrong.** Notation the reference accepts and we do not parse; each one cascades, so the count overstates the number of defects. Down from 10: the three `QPE-*` query-path-expression files and `SemanticMetadata_valid.sysml.xt` are what remain, plus two `ParsingTests_*`. |
| **Specialization cycle** — `x participates in a specialization cycle` | 3 | **Ours is wrong.** Files the pilot declares clean (`PartTest.sysml.xt`, `Redefinition_OwningType_Cyclic_Gen.sysml.xt`) — our cycle detection is counting a legitimate redefinition chain as a cycle. |
| **Conformance** — `try (typed by a1) redefines b (typed by A): types do not conform` | 2 | The declared expectation says clean, so ours is the suspect. Both are `SimpleImportTestsFromOtherFile_Import3*`. |
| **State/transition** — `transition endpoint done names a state that is not a vertex of this state machine`, `transition endpoint A1 is not a state or pseudostate` | 2 | `simpletests/StateTest.sysml.xt`:73 and `DecisionTest.sysml.xt`:69. Ours is wrong; both endpoints are legal. |

The **inherited-name-conflict** family that cost 4 rows on the first run is gone: those files are the
same defect as the `warnings` severity finding below, and making a duplicate inherited name a warning
rather than an error made all four files clean.

By suite: 11 KerML, 10 SysML. In every one of the 21 the declared expectation is *silence*, so every
one is a place where we reject something the reference accepts — the same class of finding as the
"only ours" column in [pilot-differential.md](pilot-differential.md), but here backed by a declared
intent rather than an observed verdict. The net movement across wave 8 was one row worse (244 → 243),
and a flat number hid the real trade: four parse-recovery rows closed while six visibility rows
opened. Wave 9 repays that and more, 243 → **254**, by closing the six visibility rows it had opened
plus five others — but the same visibility change is what pushes the declared-`errors` silence 37 →
49, so read this kind together with that one rather than on its own.

---

## warnings — 89 of 113 strictly, and the severity finding is closed

89 rows agree strictly. The first 11 were duplicate-member-name warnings implemented in the wave-6
round from the pilot's declared text — 6 in `MembershipTests_Distinguishability.kerml.xt`, and 5
across the `Redefinition_Diamond*_invalid` / `RedefinitionDiamond*_invalid` pairs; wave 8 added 12
more, the multiplicity-upper-bound rule among them; **wave 9C added 66**, the library inherited-name
diamond chief among them. All of them match by construction rather than by luck, because each was
written against the declared text.

The remaining 24:

| Outcome | Rows | Read |
|---|---:|---|
| `same-location` — we warn where the declaration points, naming different types | 3 | The `Part, UseCase` shape below. |
| `severity-differs` — a diagnostic of ours **is** there, as an **error** | 4 | Another rule's error is at the line, whatever this rule does. |
| `elsewhere-in-file` | 13 | We warn, but not where the declaration points. |
| nothing of ours there at all | 4 | We do not implement these shapes. |

**The severity defect the first run found is closed: it was 60 rows before wave 9C and is 0 now.**
The pilot declares:

```
//* XPECT warnings ---
"Duplicate of inherited member name 'p' from A2" at "feature redefines"
--- */
```

and on 60 rows we produced an error at that line until wave 9C made the rule a warning over library
bases. What remains of the family is 20 of the 24 disagreements, and none of them is a severity
disagreement of this rule's own making: 13 land in the wrong place, 4 sit behind another rule's
error, and 3 name a different pair of types. The `Subsetting/redefining feature should not have
larger multiplicity upper bound` rule, 8 rows of nothing on the first run, and
`User library packages should not be marked as standard`, 1 row, are both implemented and agreeing.

One reading trap in the per-kind table above: with strict agreements now non-zero, the `nothing`
column subtracts them as well as the tolerances, so it reads **4** — the rows where nothing of ours
is there at all.

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
| 12 | `ActionUsage_invalid.sysml.xt:61`, `StateUsage_invalid.sysml.xt:87` | The pilot reports on the nested `perform b.a;` / `exhibit s.sa;` reference usage and on the `b.a` expression inside it; we report on the referenced declaration instead, so the rows land `elsewhere-in-file`. |
| 3 | `CaseUsage_Invalid.sysml.xt:86` | We warn at the declared location, but name `Case, Part` / `Action, Part` where the pilot names `Part, UseCase`: `use case uc3: B;` **inside a definition body** is built as a *case* usage, not a use-case usage (the same declaration at package level is a `useCaseUsage`), so it takes `Cases::Case` as its implicit base. A parser/symbol-kind defect outside this slice — see the wave-10 worklist. |
| 4 | `Specialization_invalid.kerml.xt:56,60`, `OccurrenceUsage_invalid.sysml.xt:59` | An error of another rule (specialization cycle; occurrence typing) is at the line, so the row reads `severity-differs` whatever this rule does. |
| 2 | `AttributeUsage_invalid.sysml.xt:47,52` | `DataValue, Part` / `DataValue, Port`: the rule deliberately draws no diamond through an attribute's implicit `Base::DataValue` typing when the declaration also has a declared type, because doing so reported `'self' from DataValue, …` across otherwise-clean pilot-corpora roots. |
| 1 | `InterfaceUsage_Invalid.sysml.xt:78` | `Part, Port` through an `end part ::> tankAssy.fuel;` subsetting chain, which the rule does not follow. |
| 1 | `ShadowingTests_ImportAndInnerClassesNamesAreTheSameBadCase3_Rdef.kerml.xt:28` | `'B' from OuterPackage`: an inherited *package* member name, not a library diamond. |
| 1 | `BindingConnector_Invalid2.sysml.xt:42` | `Bound features should have conforming types` on `rearWheel+1`: one endpoint is an expression, so the rule has no feature type to compare. |

---

## errors — 95 of 513 strictly; where our diagnostics actually are

Strict agreement here requires our message to be the pilot's message, so **the strict column measures
how many rules were written against the declared text, not how correct we are.** What the tolerances
say:

| Tolerance | Rows | Meaning |
|---|---:|---|
| `same-location` | 232 | we flag the exact declared offset, in our own words — agreement in substance |
| `same-line` | 62 | we flag the declared line at a different offset — almost certainly the same defect |
| `severity-differs` | 24 | we report the declared defect as a *warning* |
| `elsewhere-in-file` | 51 | we report errors, but not where the declaration points |
| nothing | 49 | **we accept a file the pilot's implementers declared invalid** |

The split by suite is informative: KerML is 34 strict / 225 `same-location` / 15 `same-line` /
10 `severity-differs` / 21 `elsewhere`, SysML is 61 / 7 / 47 / 14 / 30.

The SysML suite's assertions anchor at a whole declaration (`at "part def P { ... }"`) while ours
land on the offending token inside it, so `same-line` there means what `same-location` means in
KerML. Together, **389 of 513 declared errors are ours at the declared location or line** — the
largest block of substantive agreement this harness finds, and most of it is agreement it cannot
score strictly because the wording is ours.

The `nothing` row is the one that moved in wave 9, and it moved the wrong way: 37 → **49**, all of it
wave 9D's protected/shadowed reconciliation, enumerated under Totals above. It is the wave's single
regression against a declared expectation, and it bought 11 `noErrors` rows and 11 `scope` rows.

**The wave-8 round moved this mix, and it moved it in our favour for a reason worth stating:** the
validation, visibility and parser rounds added rules the pilot's suites declare, so 158 rows left
"nothing" (195 → 37) and the strict column rose from 0 to 95 wherever the new rule was written
against the pilot's own message text. Nothing here is a tolerance change: the harness is the one 8F
landed, and a run on `main` at this commit is byte-identical.

The 49 "nothing at all" rows are the actionable set: declared-invalid models we accept silently. 37
of them are missing validation rules rather than parse failures, e.g. `Must be model-level evaluable`
and `Must have a Boolean result` (constraint/expression checks we do not perform),
`A variant must be an owned member of a variation.`, and the SysML `validation/invalid/*` family; the
other 12 are wave 9D's, rules we *have* and no longer apply to protected imports. The
24 `severity-differs` rows are the mirror image of the warnings finding — for example
`parsing/ParsingTests_Import_Visibility.kerml.xt`:23, where the pilot declares an error and we emit
`warning: import without a visibility indicator`.

Every one of the 513 rows, with the declared message and ours, is in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## scope — 183 of 230 agree exactly

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
| agree (exact) | 183 | the declared set, name for name |
| `library-names` | 28 | differs **only** in path tails through `Base`'s implicit `self`/`that` |
| `missing-and-extra` | 14 | both |
| `extra-names` | 3 | we offer names the pilot does not, and miss none |
| `missing-names` | 2 | we miss declared names and offer no extra ones |
| `other-paths` | 0 | — |

**This is a worklist, not a verdict, and it must not be averaged into a percentage.** Exact agreement
on sets that routinely run past 50 entries is real evidence that our visible-name computation is
broadly right; the class that dominated it through wave 8 — implicit members, 96 rows then 125 — was
two separate defects, both fixed in wave 9A, leaving 28 rows of a third, narrower one. Wave 9D then
emptied `other-paths` and took agreement to 183, and the rows it moved did not all move cleanly — see
item 3.

1. **`library-names` (28 rows) — a typed feature is still offered the implicit members its type's
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
   this class from 27 rows back to 30 — so the rule is subtler and is wave 10's worklist. Wave 9D's
   protected-path reconciliation added one row here (27 → 28).
2. **`extra-names` (3 rows) — the residue of the redefinition-masking defect.** This class was 32
   rows, 28 of them `*_Rdef` fixtures, where a `redefines` did not mask the inherited name it
   redefines; the wave-8 masking round in **`internal/core/semantics`** closed all but three, and the
   three that remain (`Import_QualifiedName2`, `ShortName_Scoping_Valid1`,
   `MemberNameTests_NamedMemberFromInheritance2_Rdef`) differ in implicit-member tails of the same
   shape as item 1 rather than in a redefined name.
3. **`other-paths` (0 rows) — emptied in wave 9D, and five of its rows overshoot now.** This class
   held 11 rows: circular containment truncated one step earlier for us, so
   `ShadowingTests_CircleProblem2.kerml.xt`:22 was missing `A.B.B` while offering `A.B` and `B.B`.
   Wave 9D emits the re-entry, and `CircleProblem2` and the
   `ShadowingTests_SameNames*` family agree exactly — which is most of its +11. But on the deeper
   circular fixtures the re-entry does not stop where the pilot stops: at
   `ShadowingTests_CircleProblem3.kerml.xt`:23 the note declares 829 names and we now offer **3362**,
   2871 of them extra, and `CircleProblem4` and its `_FT`/`_Rdef` variants behave the same way. Those
   five rows are why `missing-and-extra` rose 6 → 14, and an enumeration that overshoots by that much
   is a worse answer than one that truncated, even though the class label improved. **Bounding the
   re-entry is a wave-10 item**, and it is the one place where this wave's scope movement is not a
   straight gain.
4. **`missing-names` / `missing-and-extra` (16 rows, 5 of them item 3's) — import plus inheritance
   from the container.** Reproducer: `imports/SimpleImportTests_ImportPackageAndInheritanceFromContainer.kerml.xt`:23, where
   `classifier A { public import test::*; classifier a specializes A; }` declares `A.A`, `A.A.a` and
   `A.a.A` paths we do not offer — the fixture's own authors annotate the missing ones in a trailing
   comment. Root cause in **`internal/core/resolve`** (a member imported into a namespace is not
   re-offered through the namespace's own inherited paths); reported, not fixed here.

Short-name membership imports were the one defect inside this slice's ownership and are fixed in the
surface: `public import VP::VP2::A_Id` where the element is declared `classifier <'A_Id'> B` now
surfaces both of the element's names, as `imports/ShortName_Import_Valid4.kerml.xt` declares, while an
alias membership import still surfaces the alias name only.

The per-row evidence — declared count, our count, and the first missing/extra names for each
disagreement — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json).

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
   wave 9C (60 rows ours as an error → 0); 13 `warnings` rows are still in the wrong place, 4 sit
   behind another rule's error, 3 name a different pair of types and 4 draw nothing.
2. **The 49 declared errors we do not report** — missing validation rules rather than parse defects,
   down from 195 on the first run but **up from 37**, the 12 added rows being wave 9D's protected and
   shadowed paths (see Totals).
3. **The 6 remaining parse-recovery `noErrors` rows** — notation the reference accepts and we reject,
   the three `QPE-*` query-path-expression fixtures among them.
4. **The 8 unresolved/ambiguous-reference `noErrors` rows** — the shadowing/import family, which
   `linkedName`'s 194 agreements do not reach. Wave 8's six visibility rows here are closed by
   wave 9D, at the cost of item 2.
5. **The 47 `scope` disagreements**, with the class breakdown and root-cause packages in
   [scope](#scope--183-of-230-agree-exactly): 28 of them are the residual implicit-member class —
   `Base`'s `self`/`that` offered to a feature whose declared resource set breaks the chain — plus
   import-plus-inheritance paths in `resolve` and the five circular fixtures where wave 9D's
   re-entry now overshoots the declared set.
