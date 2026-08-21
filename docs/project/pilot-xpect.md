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
- **Diagnostic presence and placement.** `errors`/`warnings` declare a severity and a message at a
  source location; `noErrors` declares silence over a whole resource set.
- **The pilot's *intent*.** When we disagree with a declared expectation, the pilot's behaviour on
  that construct is not incidental — its implementers wrote the expectation down.

Cannot:

- **Execution semantics.** Nothing here exercises action or state execution. The suites contain no
  execution assertions, and the pinned pilot has no headless action/state execution surface at all
  (see [pilot-execution-referee.md](pilot-execution-referee.md), and issue #386 for the referee's
  scope). Nothing in this report bears on a behaviour row.
- **Scope contents.** 230 `scope` assertions declare the complete set of visible names at a point.
  We read and count them but do **not** adjudicate them (see [Not adjudicated](#not-adjudicated));
  our symbol tables do not expose an equivalent enumeration, and inventing one for this harness
  would compare our reading of the assertion, not our behaviour.
- **Diagnostic wording.** Our messages are our own; a declared message and ours can describe the same
  defect in different words. Strict agreement requires the declared message, so an `errors` row can
  only agree by coincidence of wording. See [How agreement is decided](#how-agreement-is-decided).
- **Anything the suites do not cover.** These are the pilot's *tests*, not the specification. A
  construct with no assertion is not endorsed by their absence.

The comparison also replaces one input deliberately: each suite ships its own copy of the standard
library under `/library*`, and the harness loads **our** embedded stdlib instead. Comparing against
their library copy would mean adjudicating a library-import path rather than the file under test.

---

## How agreement is decided

Agreement is **strict**, and strict is what goes in the baseline:

| Kind | Agrees when |
|---|---|
| `errors`, `warnings` | we report a diagnostic of the declared severity **at the declared offset**, whose message matches the declared one (whitespace and a trailing period aside) |
| `noErrors` | we report **no error** anywhere in the declared resource set |
| `linkedName` | the reference at the declared text resolves, and the resolved element's qualified name **equals** the declared one |

No tolerance ever turns a disagreement into an agreement. Weaker rules are recorded beside each
disagreement, as evidence about *how far off* we are, and are summarized per kind in the report:

| Tolerance | Meaning |
|---|---|
| `same-location` | right severity at the declared offset, different wording |
| `same-line` | right severity on the declared line, at a different offset |
| `severity-differs` | a diagnostic of ours is there, of the *other* severity |
| `elsewhere-in-file` | right severity, but nowhere near the declaration |
| *(none)* | nothing of ours is there at all |

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
  puts `XPECT exportedObjects` on the following line. The kind is unsupported here and reported as
  not adjudicated rather than ignored.
- **XPECT-shaped text that is not a note (10).** Eight `XPECT scope`/`XPECT errors` fragments sit
  inside `/* ... */` comments and two are disabled by their authors as `// (TBD) XPECT noErrors`.
  These open no `//` or `//*` note, so the harness does not run them; all ten are listed by file and
  line in the report's *XPECT-shaped text outside a note* section rather than being dropped.

---

## Totals

```
428 .xt file(s), 0 unparsed, 0 missing declared resource(s)
1261 assertion(s) declaring 1326 expectation(s)
agree 449 | disagree 646 | unlocated 0 | not adjudicated 231
```

| Kind | Expectations | Agree | Disagree | Not adjudicated | `same-location` | `same-line` | `severity-differs` | `elsewhere` | nothing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `errors` | 513 | 0 | 513 | 0 | 153 | 69 | 16 | 80 | 195 |
| `noErrors` | 275 | 244 | 31 | 0 | — | — | — | — | — |
| `linkedName` | 194 | 194 | 0 | 0 | — | — | — | — | — |
| `warnings` | 113 | 11 | 102 | 0 | 0 | 0 | 61 | 7 | 45 |
| `scope` | 230 | 0 | 0 | 230 | — | — | — | — | — |
| `exportedObjects` | 1 | 0 | 0 | 1 | — | — | — | — | — |

Per suite:

| Suite | Files | Expectations | Agree | Disagree | Not adjudicated |
|---|---:|---:|---:|---:|---:|
| `kerml` | 303 | 968 | 381 | 356 | 231 |
| `sysml` | 125 | 358 | 68 | 290 | 0 |

**Read the `errors` row carefully: its zero is a property of the strict rule, not a measurement of
how wrong we are.** Strict agreement demands the pilot's message text, and our diagnostics are
worded independently, so an `errors` row can only agree by coincidence of wording and the
interesting number is the tolerance breakdown. `warnings` shows what that coincidence costs: 11 of
113 now agree strictly, and all 11 are the duplicate-member-name rules implemented in the wave-6
round against the pilot's declared text, so their wording matches by construction rather than by
luck. `noErrors` and `linkedName` are wording-independent, and they are where this oracle adjudicates
most directly.

Movement since the first run of this harness (the harness itself is unchanged; every difference is a
change in our behaviour):

| Kind | First run | Now | What moved |
|---|---|---|---|
| `linkedName` | 151 / 194 | **194 / 194** | alias-introduced names resolve to the aliased element, and the `~ B::f` conjugation form parses |
| `noErrors` | 231 / 275 | **244 / 275** | 6 `ParsingTests_*` files, 4 inherited-name-conflict files and 3 others no longer draw an error |
| `warnings` | 0 / 113 | **11 / 113** | the duplicate-inherited and duplicate-owned member-name warnings now exist, with the declared wording |
| `errors` | 0 / 513 | 0 / 513 | strict agreement unchanged; the tolerance mix moved, and not only in our favour (see below) |

The **complete** per-row evidence — every disagreement with its file, line, declared expectation and
our actual behaviour — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json). The sections
below group the disagreements by cause and name the files; they do not repeat 646 rows.

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
visible. That second question is the 230 unadjudicated `scope` assertions below.

## noErrors — 244 of 275 agree

31 disagreements: we report an error where the pilot's implementers declared the file clean. Grouped
by our first diagnostic:

| Cause | Rows | Read |
|---|---:|---|
| **Parse recovery** — `expected a namespace member`, `expected '{' or ';' after declaration`, `expected ')'`, `expected 'then' between connector ends` | 10 | **Ours is wrong.** Notation the reference accepts and we do not parse; each one also cascades, so the count overstates the number of defects. 10 of the original 20 closed in the wave-6 round, all six `ParsingTests_*` files among them. |
| **Unresolved / ambiguous reference** — e.g. `unresolved reference: a1 — did you mean test::A::a1?`, `ambiguous reference: SamePackage::container (2 candidates)` | 11 | **Ours is wrong**, and unmoved: this is the shadowing/import family, and it is the one `linkedName`'s 194 agreements do *not* reach, because these files' references resolve to nothing at all rather than to the wrong element. |
| **Specialization cycle** — `x participates in a specialization cycle` | 4 | **Ours is wrong.** Files the pilot declares clean (`PartTest.sysml.xt`, `Redefinition_OwningType_Cyclic_Gen.sysml.xt`) — our cycle detection is counting a legitimate redefinition chain as a cycle. |
| **Conformance / multiplicity** — `y1 [0..*] redefines y [1..*]: multiplicity bounds incompatible`, `types do not conform` | 4 | The declared expectation says clean, so ours is the suspect. One of the four surfaced *because* a parse-recovery row above it closed: the count rose from 3 while the underlying behaviour did not change. |
| **State/transition** — `transition endpoint done is not a state or pseudostate` | 1 | `simpletests/StateTest.sysml.xt`:73. Ours is wrong; `done` is a legal endpoint. |
| **Library lookup** — `unresolved reference: Real — did you mean ScalarValues::Real?` | 1 | `validation/valid/KernelLibraryTest.sysml.xt`:80 — an unqualified library name we do not bring into scope. |

The **inherited-name-conflict** family that cost 4 rows on the first run is gone: those files are the
same defect as the `warnings` severity finding below, and making a duplicate inherited name a warning
rather than an error made all four files clean.

By suite: 18 KerML, 13 SysML. In every one of the 31 the declared expectation is *silence*, so every
one is a place where we reject something the reference accepts — the same class of finding as the
"only ours" column in [pilot-differential.md](pilot-differential.md), but here backed by a declared
intent rather than an observed verdict.

---

## warnings — 11 of 113 strictly, and the severity finding is narrowed, not closed

11 rows now agree strictly, which is the first strict agreement any `errors`/`warnings` row in this
harness has produced. All 11 are duplicate-member-name warnings implemented in the wave-6 round from
the pilot's declared text — 6 in `MembershipTests_Distinguishability.kerml.xt`, and 5 across the
`Redefinition_Diamond*_invalid` / `RedefinitionDiamond*_invalid` pairs. Their wording matches by
construction, not by luck.

The remaining 102:

| Outcome | Rows | Read |
|---|---:|---|
| `severity-differs` — a diagnostic of ours **is** there, as an **error** | 61 | **Ours is still wrong.** 57 of the 61 are `Duplicate of inherited member name`. |
| `elsewhere-in-file` | 7 | All `Duplicate of inherited member name`: we warn, but not where the declaration points. |
| nothing of ours there at all | 34 | We do not implement these checks. |

**The severity defect the first run found is narrowed by roughly a sixth, not fixed.** The pilot
declares:

```
//* XPECT warnings ---
"Duplicate of inherited member name 'p' from A2" at "feature redefines"
--- */
```

and on 57 rows we still produce an error at that line. The shapes that closed are the ones the
wave-6 rule covers; the 23-row `Action, Part` diamond and 34 further supertype pairs are not among
them, so this stays the largest open severity finding in the report. Counting the family end to end:
of the 82 `Duplicate of inherited member name` rows, 11 agree, 57 are ours as an error, 7 are ours in
the wrong place and 18 draw nothing at all.

The 34 unimplemented warnings are, by declared message:

| Declared warning | Rows |
|---|---:|
| `Duplicate of inherited member name '...' from ...` (shapes where we emit nothing at all) | 18 |
| `Subsetting/redefining feature should not have larger multiplicity upper bound` | 8 |
| `Duplicate of other owned member name` | 4 |
| `Bound features should have conforming types` | 3 |
| `User library packages should not be marked as standard` | 1 |

One reading trap in the per-kind table above: its `nothing` column is computed as expectations minus
the tolerance columns, so for `warnings` it reads **45** — the 34 rows where nothing of ours is there
plus the 11 that agree strictly, which have no tolerance recorded because they need none. The 34 here
is the count of checks we do not implement.

---

## errors — 0 of 513 strictly; where our diagnostics actually are

Strict agreement here requires our message to be the pilot's message, so **zero is the expected
result and not a measurement.** What the tolerances say:

| Tolerance | Rows | Meaning |
|---|---:|---|
| `same-location` | 153 | we flag the exact declared offset, in our own words — agreement in substance |
| `same-line` | 69 | we flag the declared line at a different offset — almost certainly the same defect |
| `severity-differs` | 16 | we report the declared defect as a *warning* |
| `elsewhere-in-file` | 80 | we report errors, but not where the declaration points |
| nothing | 195 | **we accept a file the pilot's implementers declared invalid** |

The split by suite is informative: KerML is 150 `same-location` / 10 `same-line` / 6 `severity-differs`,
SysML is 3 / 59 / 10.
The SysML suite's assertions anchor at a whole declaration (`at "part def P { ... }"`) while ours
land on the offending token inside it, so `same-line` there means what `same-location` means in
KerML. Together, **222 of 513 declared errors are ours at the declared location or line, in different
words** — the largest block of substantive agreement this harness finds, and the one it cannot score.

**The wave-6 round moved this mix in both directions, and the adverse move is the interesting one.**
Eight rows left `elsewhere-in-file` for `nothing` — 4 in `Redefinition_DirectionConformance_invalid.kerml.xt`
and 4 in `ResultExpressionMembership_Invalid.kerml.xt` — because the only errors we raised in those
files were duplicate-inherited-name errors, and demoting that rule to a warning left us silent on
files the pilot declares invalid. That is the correct severity paying for a missing rule: two rows
also gained `same-location` and one gained `severity-differs`, so the net is 187 → 195 on "nothing".
Fixing the severity did not fix the rule those files are actually testing.

The 195 "nothing at all" rows are the actionable set: declared-invalid models we accept silently.
They are missing validation rules rather than parse failures, e.g. `Must be model-level evaluable`
and `Must have a Boolean result` (constraint/expression checks we do not perform),
`A variant must be an owned member of a variation.`, and the SysML `validation/invalid/*` family. The
16 `severity-differs` rows are the mirror image of the warnings finding — for example
`parsing/ParsingTests_Import_Visibility.kerml.xt`:23, where the pilot declares an error and we emit
`warning: import without a visibility indicator`.

Every one of the 513 rows, with the declared message and ours, is in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## Not adjudicated

231 assertions are read, counted and reported, and deliberately not scored:

| Kind | Assertions | Why |
|---|---:|---|
| `scope` | 230 | Declares the full set of names visible at a point, often 70+ entries deep including `self`/`that` chains. We have no equivalent enumeration to compare against; producing one for this harness would adjudicate our reading of the assertion rather than our behaviour. This is the largest piece of unexploited evidence in the suites and is a natural follow-up for the name-resolution wave — it is a *scope* oracle, where `linkedName` is only a *reference* oracle. |
| `exportedObjects` | 1 | `indexing/NameEscape.kerml.xt` — declares the index contents for escaped names. One assertion, no general mechanism worth building. |

They are `not-adjudicated` in the report and the baseline, never counted as agreements.

---

## What this report does not fix

**No disagreement recorded here is fixed by this document**; it records what the oracle says, and the
fixes are scoped from it in later rounds. The first two items of the original list are now closed
(alias identity, and the duplicate-member-name warnings existing at all); what is open, in the order
this report reads it:

1. **`Duplicate of inherited member name` severity and coverage** — 57 `warnings` rows still ours as
   an error, 18 drawing nothing, 7 in the wrong place. The wave-6 rule covers some shapes and not the
   `Action, Part` diamond; this is now the largest open severity finding.
2. **The 195 declared errors we do not report** — missing validation rules, not parse defects, and 8
   of them are files where demoting the name-conflict error left us silent on a declared-invalid
   model.
3. **The 10 remaining parse-recovery `noErrors` rows** — notation the reference accepts and we
   reject.
4. **The 11 unresolved/ambiguous-reference `noErrors` rows** — the shadowing/import family, which
   `linkedName`'s 194 agreements do not reach.
5. **Adjudicating the 230 `scope` assertions**, which needs a scope-enumeration surface we do not
   have yet — the last large block of unexploited external evidence at the pinned tag.
