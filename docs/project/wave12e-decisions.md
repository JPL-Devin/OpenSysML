# Wave 12E — scope enumeration, visibility, and two resolver defects

Wave 12E owns the Xpect rows where **the set of names we consider visible differs from the pilot's**,
plus the two resolver defects [wave11e-decisions.md](wave11e-decisions.md) adjudicated and left open
(E3, E5). Thirteen rows were in scope; **eleven closed**, and the **four import-visibility rows are
adjudicated as a divergence we keep**. Row E4 is not this slice's and stays open.

Every number below is from a fresh-cache run on this branch, taken here, not quoted:

```
rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-xpect -out build/xpect-fresh
rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-diff
rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-reject
```

| Oracle | base `7504ff09` | this branch |
|---|---|---|
| xpect | 428 `.xt` files, 0 unparsed; 1269 agree (246 wording-only) / 54 disagree | 1280 agree (248 wording-only) / **43** disagree |
| xpect `scope` | 230 rows: 221 agree / 9 disagree | 230 rows: **230 agree** / 0 disagree |
| differential | 353 files, 317 fully agreeing; 32 agreed, **119 only ours**, 66 only the pilot's | **byte-identical** |
| rejection | 120 cases: 116 both reject, 4 only the pilot, 0 only us | **byte-identical** |

The differential and rejection reports are identical file-for-file to the base run, so the
only-ours multiset did not move: no row we report is new, and none was traded away.

The branch was rebased twice, onto wave 12B's #499 (`b21a30eb`) and then onto wave 12A's #500
(`7504ff09`), and every figure above was re-measured on each new base rather than carried over.
#500 moved the differential base itself (agreed 25 &rarr; 32, only-the-pilot's 73 &rarr; 66, only-ours
unmoved at 119); our deltas are unchanged, and the branch still reads byte-identical to its base.
Per-element tier gating lets `MetadataTypePass` and `ElementFilterPass` run on files that used to be
gated, so a resolution change here could newly surface a diagnostic there: comparing the two runs row
by row, **no row that agreed on `7504ff09` disagrees on this branch** (9 `scope` rows and 2 `errors`
rows move the other way), and the differential's only-ours set is identical file-for-file.
#499's annotation-body `bodyOwners` memo and this slice's `effNames` memo are independent fields of
the same resolver, and both are kept.

The four categories are those of [wave11e-decisions.md](wave11e-decisions.md): **our defect**,
**unimplemented obligation**, **pilot limitation**, **adjudicated divergence**.

---

## 1. How far a derived path is enumerated — one rule, eight rows

**Declared.** Eight `scope` rows disagreed only in path tails through `Base`'s implicit `self` and
`that`, in two opposite directions at once: the `_FT`/`_Rdef` fixtures got **3 extra** names each and
missed nothing, while `CircleProblem3`/`CircleProblem4` were **hundreds short**.

| Row | before (missing / extra) | after |
|---|---|---|
| `ShadowingTests_CircleProblem4.kerml.xt`:32 | 82 of 111: missing 30 (`A.B.B.b.b.self`, `A.B.B.b.b.that`, `A.B.B.b.b.that.self`, `A.B.b.B.b.self`, `A.B.b.B.b.that`, …; 18 reachable another way), extra 1 (`b.B.b.self.that`) | 111 of 111, exact |
| `ShadowingTests_CircleProblem4_FT.kerml.xt`:43 | 101 of 98: missing 0, extra 3 (`b.B.b.self`, `b.B.b.that`, `b.B.b.that.self`) | 98 of 98, exact |
| `ShadowingTests_CircleProblem4_Rdef.kerml.xt`:43 | 101 of 98: missing 0, extra 3 (identically) | 98 of 98, exact |
| `SimpleImportTests_ImportPackageAndInheritanceFromContainer_FT.kerml.xt`:23 | 48 of 45: missing 0, extra 3 (`a.A.a.self`, `a.A.a.that`, `a.A.a.that.self`) | 45 of 45, exact |
| `..._ImportPackageAndInheritanceFromContainer_Rdef.kerml.xt`:23 | 48 of 45: missing 0, extra 3 (identically) | 45 of 45, exact |
| `imports/recursive/Import_Recursive3.kerml.xt`:55 | 170 of 172: missing 2 (`RecursiveImport.s.self.that`, `s.self.that`, both reachable another way), extra 0 | 172 of 172, exact |
| `ShadowingTests_CircleProblem3.kerml.xt`:23 | 456 of 829: missing 394 (`A.B.B.aa.self`, `A.B.B.aa.that`, `A.B.B.aa.that.self`, `A.B.B.bb.aa.self`, …; 211 reachable another way), extra 21 (`a.aa.B`, `a.aa.B.aa`, `a.aa.B.bb`, `a.aa.B.self`, …) | 829 of 829, exact |
| `ShadowingTests_CircleProblem3.kerml.xt`:183 | 382 of 696: missing 328 (same shape; 181 reachable another way), extra 14 (`a.aa`, `a.bb`, `aa.aa.B`, `aa.aa.B.bb`, …) | 696 of 696, exact |

**Reading.** `Base.kerml` declares the derivation these tails come from:

```kerml
abstract classifier Anything {
    feature self: Anything[1] subsets things chains things.that
    feature things: Anything [1..*] nonunique
    feature that : Anything[1]
}
```

So `self` and `that` are not special: they are ordinary members contributed by a general type, and a
path through them continues exactly as far as membership does. KerML 8.2.3.5 makes a qualified name a
walk over memberships, and a Namespace's members include those inherited through its
generalizations (KerML 8.3.3.1.4) and those imported (8.2.3.4) — with no separate budget for derived
features. What bounds the walk is that a namespace may not be *re-entered along the same path*: a
circular containment declares `A.B.B` and stops, which is the one bound the corpus exhibits.

Wave 10A read that bound as a per-*name* occurrence count and wave 11C as a per-*depth* step budget.
Both are underdetermined by the specification and both fit only part of the corpus, which is why the
same rule produced extras on one fixture and omissions on another. The rule this slice implements is
per **member source**: a path may continue through a source it has not already traversed on that
path, so the enumeration blocks re-entry rather than depth, and a feature's declared type is entered
before its implicit base. That is one statement, and it is what closes all eight rows — the extras on
the `_FT`/`_Rdef` fixtures (a path re-entering a feature it had already traversed) and the omissions
on `CircleProblem3`/`CircleProblem4` (a path refused because *some* other path had traversed the
source) are the two halves of the same mistake.

`internal/core/model/scope_names.go` carries it: `enter`/`leave` maintain the traversed set for the
current path, `enterFor` exempts the first level, `chainTo`/`chainAvoiding` find a route to a
member's declarer and a detour when the direct route is already on the path, and `expand` falls back
to implicit members only when every supertype of a symbol is already traversed. Locked by
`TestVisibleNamesInheritedFeatureEndsThePath` and
`TestVisibleNamesMutualImportBoundsPathsNotImplicitMembers`, beside the existing re-entry,
implicit-member and determinism tests.

**Category — our defect, closed.** The bound was ours to get right and the enumeration now agrees
name-for-name on all 230 `scope` rows.

---

## 2. Which occurrence a `scope` note anchors at — one harness row

**Declared.** `imports/recursive/ShortName_Import_Valid1.kerml.xt`:25 declares 75 names at
`XPECT scope at c_Public`. Before: we offered **170** — missing 4 (`A`, `A`, `A.c_public`, `A_Id`) and
**107 extra** (`Test`, `Test.Test1`, `Test.Test1.A.c_public.self`, …). After: 75 of 75, exact.

**Reading.** The pilot's Xpect method takes the offset's `EObject` *and its cross-reference*, so the
`at` text names the reference the question is about. In this fixture the only occurrence of
`c_Public` is inside the longer identifier `c_Public_Id` on a `specializes`, and the pilot anchors
there; our identifier-boundary rule walked past it to a declaration in another namespace, and
adjudicated the unfiltered scope of the wrong element. `scopeAnchor`
(`cmd/pilot-xpect/scope.go`) now prefers the first occurrence that carries a reference and falls back
to the first whole identifier, while `locate` (`cmd/pilot-xpect/compare.go`) still requires a whole
identifier for *diagnostic* matching — relaxing that costs ten other rows and would let an assertion
match a prefix inside a longer name.

**Category — pilot limitation (harness), closed as a harness fix.** This is a rule about how the
oracle's question is read, not about our behaviour; wave 11C classified the row a pilot limitation
and it is now reproduced faithfully instead.

---

## 3. A bare `import` — adjudicated divergence, 4 rows

**Rows.** `parsing/ParsingTests_Import_Visibility.kerml.xt`:23 and :25, and
`Import_Visibility_Invalid.sysml.xt`:23 and :25. All four remain open, deliberately.

**Declared.** Two *parse* errors for `import ScalarValues;` in a package body:
`mismatched input 'import' expecting '}'` at `import`, then
`extraneous input '}' expecting EOF` at the closing brace.

**Ours.** One error at the same place:
`import without a visibility indicator: SysML v2 requires public, private or protected before
'import'`, and no cascade at the brace.

**Reading — from the grammar, not the message.** The textual notation makes the indicator part of the
import prefix and *not* optional (KerML 8.2.2, SysML v2 7.2.2). The pinned grammar states it directly,
and the contrast with the member prefix in the same file is the whole point:

```xtext
fragment ImportPrefix returns SysML::Import :
	visibility = VisibilityIndicator          // no '?'
	'import' ( isImportAll ?= 'all' )?
;

fragment MemberPrefix returns SysML::Membership :
	( visibility = VisibilityIndicator )?     // optional
;
```

So we and the pilot agree on the **rule**: a bare `import` is ill-formed. We differ only in what a
parser is required to *emit* about it. The pilot's two messages are one diagnostic plus the recovery
cascade it causes — the second error is at `}` and says `expecting EOF`, which describes the parser's
state, not the model. Our parser recovers at the import and reports the violated rule once, at the
same offset, with the same severity; no expectation of the fixture is about anything else.

**Category — adjudicated divergence, kept.** Reproducing the declared pair means either emitting a
diagnostic that names an internal parser state or degrading recovery so a bare import derails the rest
of the file. Both make our output worse to satisfy an artefact of a different parser generator, and
neither is spec-derived. The rows are counted as disagreements and not reclassified as wording-only:
the second declared error has no counterpart of ours at all, so calling them wording-only would be
false.

---

## 4. E3 `ConnectorTest_ConnectorEndSubsettingBadCase` — closed

**Declared.** `Couldn't resolve reference to Feature 'f'.` at line 31 of

```kerml
package test {
    feature x;
    feature y;
    connector c {
        feature f;
        end a ::> x :> f[1];
        end b ::> f[1];       // 'f' must not resolve
        end c ::> y[1];
    }
}
```

**Before.** No error anywhere in the file — `f` resolved to the connector's own feature.
**After.** `unresolved reference: f` at the declared offset; the row agrees as wording-only.

**Reading.** `::>` is reference subsetting (KerML 7.4.11), and on a connector end it names the
**participant** the end relates. A connector's ends relate features of the type that features the
connector (KerML 8.3.4.5), so the connector's *own* features are not candidates: `f` is featured by
`c`, and only `x` and `y`, featured by `test`, are. Line 30 is not an error because there `f` appears
as a subsetting (`:> f`), not as the participant.

`refFilter.featuredBy` (`internal/core/resolve/target.go`) hides the members of one scope from a
reference, and `resolveRelationships` (`document.go`) sets it to the enclosing scope for a
`RelReferences` target of an `end` whose owner is a connector-kind usage (`declaresConnector`). The
narrowing to connector kinds is load-bearing: an `end` inside a plain `feature`, as in
`examples/parser_features_demo_advanced_connectors.kerml`, relates nothing and keeps ordinary
visibility. Locked by `TestConnectorEndDoesNotReferenceAFeatureOfTheConnector` and
`TestEndOutsideAConnectorReferencesItsOwnersFeature`.

**Category — our defect (11E: resolver), closed here.**

---

## 5. E5 `VisibilityTests_Protected_FeatureChaining` — closed

**Declared.** Five errors; four already agreed as wording-only. The fifth,
`Couldn't resolve reference to Feature 'c'.` at line 34 of

```kerml
feature x { feature a; protected feature b; private feature c; }
feature y subsets x {
    feature redefines a;
    feature redefines b;
    feature redefines c;    // 'c' is private in x
}
```

**Before.** No error at line 34; our nearest was line 39. **After.** `unresolved reference: c` at the
declared offset; all five rows agree as wording-only.

**Reading.** A feature with no declared name takes the **effective name** of the feature it redefines
(KerML 7.3.4.5). `feature redefines c` therefore binds a name only if `c` resolves; `x::c` is private,
so it does not, and the member binds nothing. We indexed the borrowed name anyway, so the *later*
`feature redefines x.c` found the unnamed member of `y` and resolved — masking the error the fixture
declares. `bindsEffectiveName` (`internal/core/resolve/unqualified.go`) now excludes a member whose
effective name comes from a redefinition that names no visible feature, memoized in `Resolver.effNames`
with the existing `naming` cycle guard, and applied through `localBinding` on every local step of the
unqualified walk. Locked by `TestUnnamedRedefinitionOfAnInvisibleFeatureBindsNoName`.

Wave 10E's protected-import gains are intact: the `noErrors` and `scope` totals are up, not down, and
`internal/core/resolve/protected_test.go` passes unchanged.

**Category — our defect (11E: resolver), closed here.**

---

## Rows this slice did not close

| Row | Category | Owner |
|---|---|---|
| `ParsingTests_Import_Visibility.kerml.xt`:23, :25 | **adjudicated divergence** (§3) | 12E (this doc) |
| `Import_Visibility_Invalid.sysml.xt`:23, :25 | **adjudicated divergence** (§3) | 12E (this doc) |
| E4 `SimpleImportTestsFromOtherFile_Import3{,_FT}` | **adjudicated divergence** — not this slice's; see [wave11e-decisions.md](wave11e-decisions.md) E4 | 11E |
| E1 `Type_Multiplicity_invalid` | **unimplemented obligation** — blocked behind a missing parser production | parser slice |
| E2 `AssociationTest_CrossFeatures_invalid` | **unimplemented obligation** | parser/AST, then a KerML rule slice |
| `ParsingTests_ScopeWithFourDotAndDot.kerml.xt`:22 (2 expectations) | **our defect, open** — `Feature::chainingFeature` is typed `Feature`, so `OuterPackage::B.b` chaining through a classifier is a linking failure for the pilot; we resolve without checking the metaclass the reference position admits ([wave11b-parser-findings.md](wave11b-parser-findings.md)). Not a visibility question, so not touched here | resolver, a later slice |

The other 33 Xpect disagreements are outside this slice and carry their categories in
[pilot-xpect.md](pilot-xpect.md) and [wave11e-decisions.md](wave11e-decisions.md); nothing on this
branch reclassified any of them.

**What is *not* claimed.** The 230 `scope` rows agree exactly, which is evidence about the anchors the
pilot's corpus asks about and nothing more — a construct with no `scope` note is not endorsed by its
silence. The rule in §1 is derived from KerML 8.2.3.5/8.3.3.1.4 and `Base.kerml`, but the pilot's
truncation of a circular containment is a *fact of the corpus* we now match, not a sentence we can
cite; if a future fixture contradicts it, this is the paragraph to reopen.
