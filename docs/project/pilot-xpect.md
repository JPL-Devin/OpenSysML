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
[spec-compliance.md](spec-compliance.md). They are broken out in their own section below, and they
produced the single clearest finding in this report.

---

## What this oracle can and cannot adjudicate

Can:

- **Name resolution.** `linkedName` declares the qualified name a given reference must resolve to.
  This is a direct, per-reference verdict on our scoping, import, inheritance and alias handling.
- **Name *visibility*.** `scope` declares the complete set of names visible at a point, so it catches
  both halves of a scoping defect — a name that should be visible and is not, and a name that is
  visible and should not be. `linkedName` can only see the first half. See
  [scope](#scope--73-of-230-agree-exactly).
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
- **Which standard-library file a fixture loads.** Each fixture declares its own resource set, often
  `/library/Base.kerml` alone, and the pilot's implicit supertypes therefore resolve only as far as
  that set reaches. We always carry our whole embedded library, which is what the `library-names`
  scope class below counts.
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
  both land in the disagreement list below, so dropping them would have flattered us by two rows.
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
agree 522 | disagree 803 | unlocated 0 | not adjudicated 1
```

| Kind | Expectations | Agree | Disagree | Not adjudicated | `same-location` | `same-line` | `severity-differs` | `elsewhere` | nothing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `errors` | 513 | 0 | 513 | 0 | 153 | 69 | 16 | 80 | 195 |
| `noErrors` | 275 | 244 | 31 | 0 | — | — | — | — | — |
| `linkedName` | 194 | 194 | 0 | 0 | — | — | — | — | — |
| `warnings` | 113 | 11 | 102 | 0 | 0 | 0 | 61 | 7 | 45 |
| `scope` | 230 | 73 | 157 | 0 | — | — | — | — | — |
| `exportedObjects` | 1 | 0 | 0 | 1 | — | — | — | — | — |

Per suite:

| Suite | Files | Expectations | Agree | Disagree | Not adjudicated |
|---|---:|---:|---:|---:|---:|
| `kerml` | 303 | 968 | 454 | 513 | 1 |
| `sysml` | 125 | 358 | 68 | 290 | 0 |

**Read the `errors` and `warnings` rows carefully: the zeros are a property of the strict rule, not a
measurement of how wrong we are.** Strict agreement demands the pilot's message text, and our
diagnostics are worded independently, so `errors` and `warnings` are effectively unable to agree
strictly and the interesting number is the tolerance breakdown. `noErrors` and `linkedName` are
wording-independent, and they are where this oracle actually adjudicates.

**These tables and the baseline are a fresh run on `main` plus wave 7A; the `linkedName`, `noErrors`
and `warnings` sections below, and items 1–3 of [Not fixed here](#not-fixed-here), still cite the
earlier run they were written for** (`linkedName` 151/194, `noErrors` 231/275, `warnings` 0/113). They
are rebaselined centrally, not here: this slice regenerated the baseline because the `scope` numbers
are its own to establish, and the count tables move with the baseline. Where a table and a narrative
section disagree, the table is the live number.

The **complete** per-row evidence — every disagreement with its file, line, declared expectation and
our actual behaviour — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json). The sections
below group the disagreements by cause and name the files; they do not repeat 803 rows.

---

## linkedName — 151 of 194 agree

This is the section that matters, and the result is unusually clean: **all 43 disagreements have two
causes, 41 of them one cause.**

### Cause 1 — an alias resolves to itself instead of to its target (41 of 43)

The pilot's `linkedName` reports the qualified name of the **element** a reference reaches. Where the
name reached it through an `alias`, that is the aliased element's own qualified name. We report the
qualified name of the **alias**:

```kerml
package test {
    classifier A;
    alias A_alias for A;
    //XPECT linkedName at A_alias --> test.A
    classifier B specializes A_alias;
}
```

`testsuite/MemberNameTests_LocalNamedMember.kerml.xt`:37 — declared `test.A`, ours `test.A_alias`.

The reference *resolves*; nothing downstream breaks. What differs is the identity of the thing it
resolved to: our symbol index makes an alias a symbol in its own right rather than a second name for
an existing element. **First read: the pilot is right.** A KerML `alias` declares a `Membership` with
a name whose `memberElement` is the existing element — it introduces a name, not an element — so the
resolved element's qualified name cannot contain the alias.

All 41 are the single-declaration form `alias X for <target>;`; what varies is where the target lives,
which is why this reads as one defect in `internal/core/symbols`/`internal/core/resolve` rather than
four:

| Target of the alias | Rows | Example |
|---|---:|---|
| an element in the same or an imported package | 28 | `testsuite/MultipleImport_ImportClassWithAlias.kerml.xt`:23 — declared `OuterPackage.A`, ours `test.aliass` |
| an *inherited* member | 9 | `testsuite/MemberNameTests_NamedMemberFromInheritance.kerml.xt`:41 — declared `test.A.a`, ours `test.A.aa` |
| a *private* member | 4 | `testsuite/MemberNameTests_NamedMemberForPrivate.kerml.xt`:27,29 — declared `test.something`, ours `test.k` |

The inherited-member rows are worth a second look during the fix: there the alias is declared on the
supertype and reached through inheritance, so the correct answer (`test.A.a`) names the *supertype's*
member while we name the subtype-visible alias. That is the same root cause plus an inheritance hop,
not a separate defect.

Full list of the 41, all KerML:

| File | Line | Declared | Ours |
|---|---:|---|---|
| `scoping/ShortName_Import_Valid5.kerml.xt` | 29 | `Test.Test2.VP.VVP.VVVP.VVVVP` | `Test.Test2.VP.VVP.VVVP.VVVVP_Id_alias` |
| `testsuite/MemberNameTests_LocalNamedMember.kerml.xt` | 37 | `test.A` | `test.A_alias` |
| `testsuite/MemberNameTests_LocalNamedMember.kerml.xt` | 41 | `test.A` | `test.A_Id_alias` |
| `testsuite/MemberNameTests_NamedMemberForPrivate.kerml.xt` | 27 | `test.something` | `test.k` |
| `testsuite/MemberNameTests_NamedMemberForPrivate.kerml.xt` | 29 | `test.something` | `test.kk` |
| `testsuite/MemberNameTests_NamedMemberForPrivate_FT.kerml.xt` | 28 | `test.something` | `test.k` |
| `testsuite/MemberNameTests_NamedMemberForPrivate_Rdef.kerml.xt` | 27 | `test.something` | `test.k` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance.kerml.xt` | 41 | `test.A.a` | `test.A.aa` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance_2.kerml.xt` | 32 | `test.A.a` | `test.A.aa` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance_Rdef.kerml.xt` | 50 | `test.A.a` | `test.A.aa` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance2.kerml.xt` | 29 | `test.A` | `test.AA` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance2.kerml.xt` | 47 | `test.A.a` | `test.A.aa` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance2_FT.kerml.xt` | 41 | `test.A` | `test.AA` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance2_FT.kerml.xt` | 61 | `test.A.a` | `test.A.aa` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance2_Rdef.kerml.xt` | 38 | `test.A` | `test.AA` |
| `testsuite/MemberNameTests_NamedMemberFromInheritance2_Rdef.kerml.xt` | 56 | `test.A.a` | `test.A.aa` |
| `testsuite/MemberNameTests_NamedMemberFromOtherPackage.kerml.xt` | 24 | `test.P.A` | `test.P.A_alias` |
| `testsuite/MemberNameTests_NamedMemberFromOtherPackage2.kerml.xt` | 30 | `test.P.A` | `test.P.A_alias` |
| `testsuite/MultipleImport_ImportClassWithAlias.kerml.xt` | 23 | `OuterPackage.A` | `test.aliass` |
| `testsuite/MultipleImport_ImportClassWithAlias_FT.kerml.xt` | 23 | `OuterPackage.A` | `test.aliass` |
| `testsuite/MultipleImport_ImportClassWithAlias_Rdef.kerml.xt` | 23 | `OuterPackage.A` | `test.aliass` |
| `testsuite/MultipleImport_ImportFeatureAlias.kerml.xt` | 24 | `OuterPackage.B.b` | `test.aliass` |
| `testsuite/MultipleImportTests_ImportFeatureWithAlias.kerml.xt` | 25 | `OuterPackage.B.b` | `test.bb` |
| `testsuite/ShadowingTests_ImportPackageAlias1.kerml.xt` | 40 | `PackageAlias1.A` | `PackageAlias1.A_alias` |
| `testsuite/ShadowingTests_ImportPackageAlias1_FT.kerml.xt` | 68 | `PackageAlias1.A` | `PackageAlias1.A_alias` |
| `testsuite/ShadowingTests_ImportPackageAlias1_Rdef.kerml.xt` | 68 | `PackageAlias1.A` | `PackageAlias1.A_alias` |
| `testsuite/ShadowingTests_ImportPackageAlias2.kerml.xt` | 27 | `test.A` | `test.A_alias` |
| `testsuite/ShadowingTests_ImportPackageAlias2_FT.kerml.xt` | 27 | `test.A` | `test.A_alias` |
| `testsuite/ShadowingTests_ImportPackageAlias2_Rdef.kerml.xt` | 27 | `test.A` | `test.A_alias` |
| `testsuite/SimpleImportAsFeatureTests_ImportClassAlias.kerml.xt` | 24 | `OuterPackage.A` | `test.aliass` |
| `testsuite/SimpleImportAsFeatureTests_ImportFeatureAlias.kerml.xt` | 24 | `OuterPackage.B.b` | `test.aliass` |
| `visibility/VisibilityTests_ImportAsFeatureInheritanceAlias.kerml.xt` | 23 | `VisibilityPackage.c_Public` | `Classes.aliass` |
| `visibility/VisibilityTests_ImportAsFeatureInheritanceAlias_FT.kerml.xt` | 23 | `VisibilityPackage.c_Public` | `Classes.aliass` |
| `visibility/VisibilityTests_ImportAsFeatureInheritanceAlias_Rdef.kerml.xt` | 23 | `VisibilityPackage.c_Public` | `Classes.aliass` |
| `visibility/VisibilityTests_ImportClassAndUseAlias2.kerml.xt` | 21 | `VisibilityPackage.c_Public_alias.c_public` | `VisibilityPackage.c_Public_alias.alias_public` |
| `visibility/VisibilityTests_PublicImportAsFeatureAlias.kerml.xt` | 36 | `VisibilityPackage.c_Public.c_public` | `Classes.aliass` |
| `visibility/VisibilityTests_PublicImportAsFeatureAlias.kerml.xt` | 38 | `VisibilityPackage.c_Public` | `Classes.Aliass` |
| `visibility/VisibilityTests_PublicImportAsFeatureAlias_FT.kerml.xt` | 57 | `VisibilityPackage.c_Public.c_public` | `Classes.aliass` |
| `visibility/VisibilityTests_PublicImportAsFeatureAlias_FT.kerml.xt` | 59 | `VisibilityPackage.c_Public` | `Classes.Aliass` |
| `visibility/VisibilityTests_PublicImportAsFeatureAlias_Rdef.kerml.xt` | 60 | `VisibilityPackage.c_Public.c_public` | `Classes.aliass` |
| `visibility/VisibilityTests_PublicImportAsFeatureAlias_Rdef.kerml.xt` | 62 | `VisibilityPackage.c_Public` | `Classes.Aliass` |

### Cause 2 — a member reached through a conjugated type is not a reference at all (2 of 43)

```kerml
classifier B conjugates A;
//XPECT linkedName at B::f --> test.A.f
feature g ~ B::f;
```

`testsuite/MemberNameTests_NamedMemberFromConjugation.kerml.xt`:41 and :43 — declared `test.A.f`,
ours *"we index no name reference at offset 1595"*. We index no reference for the `f` segment here at
all, and the same file's `noErrors` assertion also fails with `expected '{' or ';' after declaration`
at line 39: we do not parse the `feature g ~ B::f;` conjugation form, so nothing downstream can
resolve. **First read: the pilot is right, and the root cause is in the parser, not in resolution.**

### What the agreements are worth

The 151 agreements are not trivial passes: they cover qualified names through public and private
imports, `import ::*` recursion, short names and `<id>` forms, shadowing between an import and an
inner classifier, redefinition scoping, and inherited-member lookup. Where an alias is not involved,
our resolution matches the pilot's declared answer on 151 of 153 rows.

---

## noErrors — 231 of 275 agree

44 disagreements: we report an error where the pilot's implementers declared the file clean. Grouped
by our first diagnostic:

| Cause | Rows | Read |
|---|---:|---|
| **Parse recovery** — `expected a body member`, `expected a namespace member`, `expected '{' or ';' after declaration`, `expected ')'`, `expected a /* ... */ comment body`, `expected 'then' between connector ends` | 20 | **Ours is wrong.** These are notation the reference accepts and we do not parse; each one also cascades, so the count overstates the number of defects. |
| **Unresolved / ambiguous reference** — e.g. `unresolved reference: a1 — did you mean test::A::a1?`, `ambiguous reference: SamePackage::container (2 candidates)` | 11 | **Ours is wrong**, and this is the same shadowing/import family the `linkedName` rows probe. |
| **Name conflict on an inherited member** — `name conflict: p is already the name of the inherited feature A::p` | 4 | **Ours is wrong twice over:** the pilot declares these files clean, and where it does object to a duplicate inherited name it does so as a *warning* (see below), never an error. |
| **Specialization cycle** — `x participates in a specialization cycle` | 4 | **Ours is wrong.** Files the pilot declares clean (`PartTest.sysml.xt`, `Redefinition_OwningType_Cyclic_Gen.sysml.xt`) — our cycle detection is counting a legitimate redefinition chain as a cycle. |
| **Conformance / multiplicity** — `redefines y [1..*]: multiplicity bounds incompatible`, `types do not conform` | 3 | Needs adjudication per row; the declared expectation says clean, so ours is the suspect. |
| **State/transition** — `transition endpoint done is not a state or pseudostate` | 1 | `simpletests/StateTest.sysml.xt`:73. Ours is wrong; `done` is a legal endpoint. |
| **Library lookup** — `unresolved reference: Real — did you mean ScalarValues::Real?` | 1 | `validation/valid/KernelLibraryTest.sysml.xt`:80 — an unqualified library name we do not bring into scope. |

By suite: 29 KerML, 15 SysML. In every one of the 44 the declared expectation is *silence*, so every
one is a place where we reject something the reference accepts — the same class of finding as the
"only ours" column in [pilot-differential.md](pilot-differential.md), but here backed by a declared
intent rather than an observed verdict.

---

## warnings — 0 of 113 strictly, and one real finding

No `warnings` row agrees strictly, because none of our messages match the pilot's wording. The
tolerance columns carry the finding:

| Outcome | Rows | Read |
|---|---:|---|
| `severity-differs` — a diagnostic of ours **is** there, as an **error** | 64 | **Ours is wrong, and this is the report's second substantive finding.** |
| nothing of ours there at all | 49 | We do not implement these checks. |

The 64 are dominated by one rule. The pilot declares:

```
//* XPECT warnings ---
"Duplicate of inherited member name 'p' from A2" at "feature redefines"
--- */
```

and we produce, at the same line, `error: name conflict: p is already the name of the inherited
feature A2` (`validation/Redefinition_Diamond1_invalid.kerml.xt`:36). **First read: the pilot is
right on severity.** A duplicate inherited member name is a warning there — the model is still
usable — and we reject it. This is the same defect that fails 4 of the 44 `noErrors` rows, which is
the stronger evidence: the pilot ships files it declares *clean* that we reject over it. 23 of the 64
are the `Action, Part` diamond shape, the rest are other supertype pairs.

The 49 unimplemented warnings are, by declared message:

| Declared warning | Rows |
|---|---:|
| `Duplicate of inherited member name '...' from ...` (shapes where we emit nothing at all) | 27 |
| `Duplicate of other owned member name` | 8 |
| `Subsetting/redefining feature should not have larger multiplicity upper bound` | 8 |
| `Bound features should have conforming types` | 3 |
| other one-offs | 3 |

---

## errors — 0 of 513 strictly; where our diagnostics actually are

Strict agreement here requires our message to be the pilot's message, so **zero is the expected
result and not a measurement.** What the tolerances say:

| Tolerance | Rows | Meaning |
|---|---:|---|
| `same-location` | 152 | we flag the exact declared offset, in our own words — agreement in substance |
| `same-line` | 70 | we flag the declared line at a different offset — almost certainly the same defect |
| `severity-differs` | 15 | we report the declared defect as a *warning* |
| `elsewhere-in-file` | 89 | we report errors, but not where the declaration points |
| nothing | 187 | **we accept a file the pilot's implementers declared invalid** |

The split by suite is informative: KerML is 149 `same-location` / 11 `same-line`, SysML is 3 / 59.
The SysML suite's assertions anchor at a whole declaration (`at "part def P { ... }"`) while ours
land on the offending token inside it, so `same-line` there means what `same-location` means in
KerML. Together, **222 of 513 declared errors are ours at the declared location or line, in different
words** — the largest block of substantive agreement this harness finds, and the one it cannot score.

The 187 "nothing at all" rows are the actionable set: declared-invalid models we accept silently.
They are missing validation rules rather than parse failures, e.g. `Must be model-level evaluable`
and `Must have a Boolean result` (constraint/expression checks we do not perform),
`A variant must be an owned member of a variation.`, and the SysML `validation/invalid/*` family. The
15 `severity-differs` rows are the mirror image of the warnings finding — for example
`parsing/ParsingTests_Import_Visibility.kerml.xt`:23, where the pilot declares an error and we emit
`warning: import without a visibility indicator`.

Every one of the 513 rows, with the declared message and ours, is in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## scope — 73 of 230 agree exactly

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
| agree (exact) | 73 | the declared set, name for name |
| `library-names` | 96 | differs **only** in path tails through `Base`'s implicit `self`/`that` |
| `extra-names` | 32 | we offer names the pilot does not, and miss none |
| `other-paths` | 12 | every name we miss is an element we offer under a different path |
| `missing-names` | 10 | we miss declared names and offer no extra ones |
| `missing-and-extra` | 7 | both |

**This is a worklist, not a verdict, and it must not be averaged into a percentage.** 73 exact
agreements on sets that routinely run past 50 entries is real evidence that our visible-name
computation is broadly right; the 157 disagreements are four concrete defects, three of them ours to
report rather than to fix:

1. **`library-names` (96 rows) — implicit supertypes are resolved against our whole embedded library,
   not against the fixture's declared resource set.** Reproducer:
   `imports/global/DependencyPackageAlias0_A_alias.kerml.xt`:22 declares 20 names for
   `class A { class a; }`; we offer 56, the extra 36 being `A_alias.a.that`, `A_alias.that` and
   friends. The fixture loads `/library/Base.kerml` only, so the pilot's `class` — whose implicit
   supertype is in `Occurrences`, which is *not* loaded — inherits nothing, while its `classifier` and
   `feature` fixtures do declare `self`/`that` (see `imports/Import_Circular.kerml.xt`:38). We always
   have the whole library, so we always contribute the implicit members. This is a property of the
   harness's library substitution, documented at the top of this file, not a visibility defect — which
   is why it is its own class and not counted as `extra-names`.
2. **`extra-names` (32 rows) — a redefinition does not mask the inherited name it redefines.**
   28 of the 32 are `*_Rdef` fixtures. Reproducer:
   `name/MemberNameTests_MultipleInheritance2_Rdef.kerml.xt` — where a feature `redefines` an
   inherited member under a new name, the pilot stops offering the old name and we still offer it.
   Root cause in **`internal/core/semantics`** (inherited-member computation does not remove a
   redefined name); reported, not fixed here — `semantics/**` is outside this slice's ownership.
3. **`other-paths` (12 rows) — circular containment truncates one step earlier for us.**
   All 12 are `shadowing/ShadowingTests_Circle*`. Reproducer:
   `ShadowingTests_CircleProblem2.kerml.xt`:22 declares `A, A.B, A.B.B, B, B.B, Test1.A, Test1.A.B,
   Test1.A.B.B`; we offer every element it names but stop the path at the first repeat of an element,
   so `A.B.B` is missing while `A.B` and `B.B` are present. Emitting the re-entry was measured and
   costs more than it buys (four rows lost elsewhere), so the truncation rule stays and the class
   records it as a path-convention difference, not a missing name. All eight names that fixture
   declares do resolve when written as a reference — "missing" in a `scope` row always means missing
   from the *enumeration*, which is the conservative direction: it under-reports our agreement.
4. **`missing-names` / `missing-and-extra` (17 rows) — import plus inheritance from the container.**
   Reproducer: `imports/SimpleImportTests_ImportPackageAndInheritanceFromContainer.kerml.xt`:23, where
   `classifier A { public import test::*; classifier a specializes A; }` declares `A.A`, `A.A.a` and
   `A.a.A` paths we do not offer — the fixture's own authors annotate the missing ones in a trailing
   comment. Root cause in **`internal/core/resolve`** (a member imported into a namespace is not
   re-offered through the namespace's own inherited paths); reported, not fixed here.

Short-name membership imports were the one defect inside this slice's ownership and are fixed in the
surface: `public import VP::VP2::A_Id` where the element is declared `classifier <'A_Id'> B` now
surfaces both of the element's names, as `imports/ShortName_Import_Valid4.kerml.xt` declares, while an
alias membership import still surfaces the alias name only.

The per-row evidence — declared count, our count, and the first missing/extra names for each of the
157 disagreements — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## Not adjudicated

One assertion is read, counted and reported, and deliberately not scored:

| Kind | Assertions | Why |
|---|---:|---|
| `exportedObjects` | 1 | `indexing/NameEscape.kerml.xt` — declares the index contents for escaped names. One assertion, no general mechanism worth building. |

They are `not-adjudicated` in the report and the baseline, never counted as agreements.

---

## Not fixed here

**No disagreement in this report is fixed in this PR.** This session builds the oracle and records
what it says; the fixes are scoped from it. In priority order, as this report reads them:

1. **Alias identity in resolution** (41 `linkedName` rows) — an alias must resolve to its target
   element, not to itself.
2. **Duplicate inherited member name is a warning, not an error** (64 `warnings` rows + 4 `noErrors`
   rows) — the clearest severity defect the harness found.
3. **The 20 parse-recovery `noErrors` rows** — notation the reference accepts and we reject,
   including the `~ B::f` conjugation form that also costs 2 `linkedName` rows.
4. **The 187 declared errors we do not report** — missing validation rules, not parse defects.
5. **The 157 `scope` disagreements**, with the class breakdown and root-cause packages in
   [scope](#scope--73-of-230-agree-exactly): redefinition masking in `semantics`, import-plus-
   inheritance paths in `resolve`, and the library-substitution class that is a property of the
   harness rather than a defect.
