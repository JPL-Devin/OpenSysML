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
  [scope](#scope--74-of-230-agree-exactly).
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
agree 630 | disagree 696 | unlocated 0 | not adjudicated 0
```

| Kind | Expectations | Agree | Disagree | Not adjudicated | `same-location` | `same-line` | `severity-differs` | `elsewhere` | nothing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `errors` | 513 | 95 | 418 | 0 | 246 | 62 | 16 | 57 | 37 |
| `noErrors` | 275 | 243 | 32 | 0 | — | — | — | — | — |
| `linkedName` | 194 | 194 | 0 | 0 | — | — | — | — | — |
| `warnings` | 113 | 23 | 90 | 0 | 0 | 0 | 60 | 7 | 23 |
| `scope` | 230 | 74 | 156 | 0 | — | — | — | — | — |
| `exportedObjects` | 1 | 1 | 0 | 0 | — | — | — | — | — |

Per suite:

| Suite | Files | Expectations | Agree | Disagree | Not adjudicated |
|---|---:|---:|---:|---:|---:|
| `kerml` | 303 | 968 | 486 | 482 | 0 |
| `sysml` | 125 | 358 | 144 | 214 | 0 |

**Read the `errors` row carefully: strict agreement demands the pilot's message text, so its 95 is a
coincidence of wording where a rule was implemented against the declared text, and the interesting
number remains the tolerance breakdown.** `warnings` shows the same effect: 23 of 113 agree
strictly, and they are the duplicate-member-name and visibility rules written against the pilot's
declared wording, so they match by construction rather than by luck. `noErrors` and `linkedName` are
wording-independent, and they are where this oracle adjudicates most directly.

Movement since the first run of this harness (the harness itself is unchanged; every difference is a
change in our behaviour):

| Kind | First run | Now | What moved |
|---|---|---|---|
| `linkedName` | 151 / 194 | **194 / 194** | alias-introduced names resolve to the aliased element, and the `~ B::f` conjugation form parses |
| `noErrors` | 231 / 275 | **243 / 275** | 6 `ParsingTests_*` files, 4 inherited-name-conflict files and 2 others no longer draw an error; one file gained an error a wave-8 rule reports |
| `warnings` | 0 / 113 | **23 / 113** | the duplicate-member-name warnings and the wave-8 rules written against the declared wording |
| `errors` | 0 / 513 | **95 / 513** | wave-8 rules written against the declared text agree word-for-word; the tolerance mix moved with them |
| `scope` | 73 / 230 | **74 / 230** | one enumeration that had an extra name is now exact |

**These tables and the baseline are a single fresh run on the tree this branch lands on**, and that
run is byte-identical to one taken on `main` at the same commit — the wave-8 validation, visibility,
semantics, resource-set and parser rounds moved the verdicts, nothing in this branch does. The
largest single movement is `errors`: 158 rows we were silent on now draw a diagnostic, so "nothing at
all" falls from 195 to 37 while strict agreement rises from 0 to 95.

The `nothing` column counts what neither strict agreement nor any tolerance accounts for. It was
equal to `rows - tolerances` for as long as the strict column was 0, and both doc guards computed it
that way; with `errors` and `warnings` now agreeing strictly, they subtract the agreements too.

The **complete** per-row evidence — every disagreement with its file, line, declared expectation and
our actual behaviour — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json). The sections
below group the disagreements by cause and name the files; they do not repeat 696 rows.

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

## noErrors — 243 of 275 agree

32 disagreements: we report an error where the pilot's implementers declared the file clean. Grouped
by our first diagnostic:

| Cause | Rows | Read |
|---|---:|---|
| **Unresolved / ambiguous reference** — e.g. `unresolved reference: a1 — did you mean test::A::a1?`, `ambiguous reference: SamePackage::container (2 candidates)`, `unresolved reference: A::c_Protect` | 18 | **Ours is wrong**, and this family *grew* in wave 8. 12 are the shadowing/import shapes, which `linkedName`'s 194 agreements do not reach because these references resolve to nothing at all rather than to the wrong element. **6 are new, and they are wave 8's own doing:** the visibility round now refuses `A::c_Protect`-style paths in six `VisibilityTests_*` fixtures the pilot declares clean, so enforcement overshot. |
| **Parse recovery** — `expected a namespace member`, `expected '{' or ';' after declaration`, `expected ')'` | 6 | **Ours is wrong.** Notation the reference accepts and we do not parse; each one cascades, so the count overstates the number of defects. Down from 10: the three `QPE-*` query-path-expression files and `SemanticMetadata_valid.sysml.xt` are what remain, plus two `ParsingTests_*`. |
| **Specialization cycle** — `x participates in a specialization cycle` | 4 | **Ours is wrong.** Files the pilot declares clean (`PartTest.sysml.xt`, `Redefinition_OwningType_Cyclic_Gen.sysml.xt`) — our cycle detection is counting a legitimate redefinition chain as a cycle. |
| **Conformance** — `try (typed by a1) redefines b (typed by A): types do not conform` | 2 | The declared expectation says clean, so ours is the suspect. Both are `SimpleImportTestsFromOtherFile_Import3*`. |
| **State/transition** — `transition endpoint done names a state that is not a vertex of this state machine`, `transition endpoint A1 is not a state or pseudostate` | 2 | `simpletests/StateTest.sysml.xt`:73 and `DecisionTest.sysml.xt`:69. Ours is wrong; both endpoints are legal. |

The **inherited-name-conflict** family that cost 4 rows on the first run is gone: those files are the
same defect as the `warnings` severity finding below, and making a duplicate inherited name a warning
rather than an error made all four files clean.

By suite: 22 KerML, 10 SysML. In every one of the 32 the declared expectation is *silence*, so every
one is a place where we reject something the reference accepts — the same class of finding as the
"only ours" column in [pilot-differential.md](pilot-differential.md), but here backed by a declared
intent rather than an observed verdict. The net movement across wave 8 is one row worse (244 → 243)
and that flat number hides the real trade: four parse-recovery rows closed while six visibility rows
opened, so this kind is the one place the wave cost us something.

---

## warnings — 23 of 113 strictly, and the severity finding is narrowed, not closed

23 rows agree strictly. The first 11 were duplicate-member-name warnings implemented in the wave-6
round from the pilot's declared text — 6 in `MembershipTests_Distinguishability.kerml.xt`, and 5
across the `Redefinition_Diamond*_invalid` / `RedefinitionDiamond*_invalid` pairs; wave 8 added 12
more, the multiplicity-upper-bound rule among them. All of them match by construction rather than by
luck, because each was written against the declared text.

The remaining 90:

| Outcome | Rows | Read |
|---|---:|---|
| `severity-differs` — a diagnostic of ours **is** there, as an **error** | 60 | **Ours is still wrong.** Every one of the 60 is `Duplicate of inherited member name`. |
| `elsewhere-in-file` | 7 | All `Duplicate of inherited member name`: we warn, but not where the declaration points. |
| nothing of ours there at all | 23 | We do not implement these checks. |

**The severity defect the first run found is narrowed by roughly a sixth, not fixed.** The pilot
declares:

```
//* XPECT warnings ---
"Duplicate of inherited member name 'p' from A2" at "feature redefines"
--- */
```

and on 60 rows we still produce an error at that line. The shapes that closed are the ones the
wave-6 and wave-8 rules cover; the `Action, Part` diamond and the further supertype pairs are not
among them, so this stays the largest open severity finding in the report. The whole family accounts
for 82 of the 90 disagreements: 60 ours as an error, 7 ours in the wrong place, 15 drawing nothing at
all.

The 23 unimplemented warnings are, by declared message:

| Declared warning | Rows |
|---|---:|
| `Duplicate of inherited member name '...' from ...` (shapes where we emit nothing at all) | 15 |
| `Duplicate of other owned member name` | 4 |
| `Bound features should have conforming types` | 3 |
| `User library packages should not be marked as standard` | 1 |

The `Subsetting/redefining feature should not have larger multiplicity upper bound` rule, 8 rows of
nothing on the first run, is implemented and agreeing.

One reading trap in the per-kind table above: with strict agreements now non-zero, the `nothing`
column subtracts them as well as the tolerances, so it reads **23** — the rows where nothing of ours
is there at all.

---

## errors — 95 of 513 strictly; where our diagnostics actually are

Strict agreement here requires our message to be the pilot's message, so **the strict column measures
how many rules were written against the declared text, not how correct we are.** What the tolerances
say:

| Tolerance | Rows | Meaning |
|---|---:|---|
| `same-location` | 246 | we flag the exact declared offset, in our own words — agreement in substance |
| `same-line` | 62 | we flag the declared line at a different offset — almost certainly the same defect |
| `severity-differs` | 16 | we report the declared defect as a *warning* |
| `elsewhere-in-file` | 57 | we report errors, but not where the declaration points |
| nothing | 37 | **we accept a file the pilot's implementers declared invalid** |

The split by suite is informative: KerML is 34 strict / 239 `same-location` / 15 `same-line` /
6 `severity-differs`, SysML is 61 / 7 / 47 / 10.
The SysML suite's assertions anchor at a whole declaration (`at "part def P { ... }"`) while ours
land on the offending token inside it, so `same-line` there means what `same-location` means in
KerML. Together, **403 of 513 declared errors are ours at the declared location or line** — the
largest block of substantive agreement this harness finds, and most of it is agreement it cannot
score strictly because the wording is ours.

**The wave-8 round moved this mix, and it moved it in our favour for a reason worth stating:** the
validation, visibility and parser rounds added rules the pilot's suites declare, so 158 rows left
"nothing" (195 → 37) and the strict column rose from 0 to 95 wherever the new rule was written
against the pilot's own message text. Nothing here is a tolerance change: the harness is the one 8F
landed, and a run on `main` at this commit is byte-identical.

The 37 "nothing at all" rows are the actionable set: declared-invalid models we accept silently.
They are missing validation rules rather than parse failures, e.g. `Must be model-level evaluable`
and `Must have a Boolean result` (constraint/expression checks we do not perform),
`A variant must be an owned member of a variation.`, and the SysML `validation/invalid/*` family. The
16 `severity-differs` rows are the mirror image of the warnings finding — for example
`parsing/ParsingTests_Import_Visibility.kerml.xt`:23, where the pilot declares an error and we emit
`warning: import without a visibility indicator`.

Every one of the 513 rows, with the declared message and ours, is in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## scope — 74 of 230 agree exactly

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
| agree (exact) | 172 | the declared set, name for name |
| `library-names` | 27 | differs **only** in path tails through `Base`'s implicit `self`/`that` |
| `other-paths` | 11 | every name we miss is an element we offer under a different path |
| `missing-names` | 11 | we miss declared names and offer no extra ones |
| `missing-and-extra` | 6 | both |
| `extra-names` | 3 | we offer names the pilot does not, and miss none |

**This is a worklist, not a verdict, and it must not be averaged into a percentage.** Exact agreement
on sets that routinely run past 50 entries is real evidence that our visible-name computation is
broadly right; the class that dominated it through wave 8 — implicit members, 96 rows then 125 — was
two separate defects, both fixed in wave 9, leaving 27 rows of a third, narrower one.

1. **`library-names` (27 rows) — a typed feature is still offered the implicit members its type's
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
   this class from 27 rows back to 30 — so the rule is subtler and is wave 10's worklist.
2. **`extra-names` (3 rows) — the residue of the redefinition-masking defect.** This class was 32
   rows, 28 of them `*_Rdef` fixtures, where a `redefines` did not mask the inherited name it
   redefines; the wave-8 masking round in **`internal/core/semantics`** closed all but three, and the
   three that remain (`Import_QualifiedName2`, `ShortName_Scoping_Valid1`,
   `MemberNameTests_NamedMemberFromInheritance2_Rdef`) differ in implicit-member tails of the same
   shape as item 1 rather than in a redefined name.
3. **`other-paths` (11 rows) — circular containment truncates one step earlier for us.**
   Six are `testsuite/ShadowingTests_Circle*`; the other five are the
   `ShadowingTests_SameNamesInnerClassAndOuterClass*` family, which the implicit-member fixes moved
   into this class out of `library-names`. Reproducer:
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

1. **`Duplicate of inherited member name` severity and coverage** — 60 `warnings` rows still ours as
   an error, 15 drawing nothing, 7 in the wrong place. The wave-6 and wave-8 rules cover some shapes
   and not the `Action, Part` diamond; this is still the largest open severity finding.
2. **The 37 declared errors we do not report** — missing validation rules rather than parse defects,
   down from 195 on the first run.
3. **The 6 remaining parse-recovery `noErrors` rows** — notation the reference accepts and we reject,
   the three `QPE-*` query-path-expression fixtures among them.
4. **The 18 unresolved/ambiguous-reference `noErrors` rows** — 12 are the shadowing/import family,
   which `linkedName`'s 194 agreements do not reach; **6 are wave 8's visibility round refusing paths
   in `VisibilityTests_*` fixtures the pilot declares clean**, and they are the one place this wave
   made a kind worse.
5. **The 156 `scope` disagreements**, with the class breakdown and root-cause packages in
   [scope](#scope--74-of-230-agree-exactly): 125 of them are now the single implicit-member class —
   `Base`'s `self`/`that` inherited where the declared resource set breaks the chain, and re-expanded
   through each other — plus import-plus-inheritance paths in `resolve` and the deliberate
   circular-containment truncation.
