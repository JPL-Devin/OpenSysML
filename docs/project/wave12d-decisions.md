# Wave 12D — parser recovery and grammar gaps

Wave 12D owns the wave-12 Xpect rows where **our parser is the reason we disagree**: it recovers at a
different token than the pilot does, or it has no production for a form the pilot's grammar admits, so
a validation rule can never be reached. This page states, per row, what the pilot declares, what we
do, the specification reading behind the change, the **category** and the owner. Every number is from
a fresh run on this branch with a cold cache, not quoted from an earlier report.

Categories, one per open row — an unlabelled open row reads as a defect:

| Category | Meaning |
|---|---|
| **our defect** | we are wrong and the pilot is right |
| **unimplemented obligation** | the specification states the rule; we do not implement it yet |
| **pilot limitation** | the pilot's declared expectation does not follow from the specification |
| **adjudicated divergence** | we deliberately differ, with the reading recorded |

## Measurements

Taken with `rm -rf /tmp/c-$$ && XDG_CACHE_HOME=/tmp/c-$$ go run ./cmd/pilot-{xpect,diff,reject}`.

Re-measured after rebasing onto `7504ff09` (wave 12A's per-element tier gating merged); the three
baseline JSONs are regenerated from these runs rather than carried across the rebase.

| Oracle | Base `7504ff09` | This branch |
|---|---|---|
| Xpect | 428 files, 0 unparsed; 1269 agree (246 wording-only) / **54 disagree** | 1276 agree (246 wording-only) / **47 disagree** |
| Differential | 353 files, 317 fully agreeing; 32 agreed, **119 only ours**, 66 pilot-only | 317 fully agreeing; 32 agreed, **119 only ours**, 66 pilot-only |
| Rejection | 120 cases: 116 both reject, 4 pilot-only, 0 ours-only | unchanged: 116 both reject, 4 pilot-only, 0 ours-only |

Wave 12A moved the differential's agreed/pilot-only split (25/73 → 32/66) and left the Xpect totals
where they were, so this slice's movement is the same on the rebased base as on `0d4eb14f`.

**Xpect rows retired — 7, and none surfaced.** Compared as a multiset of (file, line, kind, declared):

| Retired row | Why |
|---|---|
| `Type_Multiplicity_invalid.kerml.xt`:20 | E1 below |
| `AssociationTest_CrossFeatures_invalid.kerml.xt`:52 | E2 below |
| `TransitionUsage_invalid.sysml.xt`:45, :54, :68 | parallel-state and accepter-source rules below |
| `SemanticMetadata_valid.sysml.xt`:53 | prefix metadata on `subject`/`actor`/`stakeholder` members below |
| `ParsingTests_Indexing.kerml.xt`:32 | a feature typed only by its value below |

Two rows kept their verdict and changed tolerance, both in the bad-scope family:
`ParsingTests_BadScopeWithOnlyTwoDot.kerml.xt`:21 moved `elsewhere-in-file` → `same-location`, and
`ParsingTests_ScopeWithFourDotAndDot.kerml.xt`:22 moved from silence (`nothing`) → `same-location` +
`same-line`. Both are now diagnostics at the pilot's own offset stating a metaclass finding instead of
a failed lookup; the reading is under *bad-scope recovery* below.

**Differential: the only-ours count is flat at 119, and one row inside it moved.**
`Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:62 (`unresolved-reference`, error) is
gone — the reference resolves now — and `:38` (`units`, warning) appears in its place:

```sysml
:>> radius = 22/2*25.4 + 110 [mm];
```

`operator '+' combines incommensurable quantities: a dimensionless value and mm (dimension L)` is our
existing dimensional rule, which the name-resolution error used to gate away (higher tiers are skipped
when a lower one fails). It is a pre-existing **adjudicated divergence** — the pilot performs no
dimensional analysis, the family is recorded in
[pilot-differential.md](pilot-differential.md) — and not a new rule of this slice. Reported here
because it is technically a new only-ours row even though the count did not move.

**Rejection: one case detail improved, no bucket moved.** The parallel-state case now reports 1 error
(`A parallel state cannot have successions or transitions.`) instead of 3, because the 2 parse errors
it used to raise are gone.

---

## E1 — `Type_Multiplicity_invalid`: a surplus `multiplicity` member is a validation error

**Declared.** `error: "Only one multiplicity is allowed"` at `multiplicity subsets Base::zeroToMany;`.

**Was.** `expected '{' or ';'` — we had no production for a second `multiplicity` member.

**Reading.** KerML `Type::multiplicity` is single-valued (KerML 8.3.3.1.1): a type has *at most one*
multiplicity. A second declaration is therefore a well-formed piece of syntax that violates a
multiplicity constraint of the metamodel — a validation error, not a parse error. The grammar's
`TypeBody` admits `multiplicity` members without an upper bound, so the parser must accept the surplus
member for any rule to be able to name it.

**Done.** The parser accepts repeated `multiplicity` members;
`passes/at_most_one_member.go` collects the declaration-level and member-level multiplicities of a
type and reports every one after the first. **Closed** — the rule sits in the existing
`at-most-one-member` pass rather than in a new KerML slice, because that pass already owns the
single-valued-member family.

---

## E2 — `AssociationTest_CrossFeatures_invalid`: the end's inline owned cross feature

**Declared.** `error: "Must be the cross feature"` at `x1 [0..1]`, on

```kerml
end x1 [0..1] feature x : C1 crosses y.b { public import y::y1; }
```

**Was.** No error: `internal/core/ast` recorded `RelCrosses` but not the cross feature the end
declares *inline*, so there was nothing to compare it against.

**Reading.** KerML 8.3.4.5: an association end's cross feature is the feature that the end's owner
crosses to; where an end declares its cross feature inline, that owned feature *is* the end's cross
feature, so it must be the feature `crosses` names. When the two differ the model states two
different crossings of one end, which is unsatisfiable.

**Done.** A dedicated immutable `ast.CrossFeatureMember` records the inline declaration with its
identification, multiplicity and relationships, so the AST keeps the syntax without conflating it with
the end usage; `passes/w10b_cross_features.go` compares it with the `crosses` target and reports the
owned one at its own span. **Closed.** No wave-12A tier-gating interaction was needed: the rule reads
resolved endpoints only, so it runs at the name-resolution tier the fixture already reaches.

---

## Parse gaps that masked validation rules

### `state … parallel { … }` — SysML `isParallel`

**Declared.** `A parallel state cannot have successions or transitions.` at
`TransitionUsage_invalid.sysml.xt`:45 and :68.

**Was.** `expected '{' or ';'` at the declaration and `expected a namespace member` after it: the
`parallel` keyword was consumed but not represented, and the recovery hid the body.

**Reading.** SysML v2 §7.16 gives `StateDefinition`/`StateUsage` an `isParallel` attribute, written
with the `parallel` keyword before the body, and constrains a parallel state to own no successions or
transitions: its substates are concurrent regions, so a transition between them has no meaning.

**Done.** The parser records `IsParallel` on both definition and usage (requiring a body, so
`state def S parallel ;` stays a parse error) and `passes/state_transition.go` reports each succession
or transition a parallel state owns. Both rows **closed**.

### A transition with an accepter must have a state as its source

**Declared.** `A transition with an accepter must have a state as its source.` at
`TransitionUsage_invalid.sysml.xt`:54, on `transition init accept A then S2_1;`.

**Reading.** SysML v2 §7.16: a transition with an accepter (a trigger) is enabled while its source is
*active* and the accepted event occurs. Only a state is active over an interval; the `entry action
init` performed by a state is not, so an accepter attached to an action source can never be waiting.

**Done.** `checkAccepterSource` reports the rule when the source resolves to a performed action, at
the trigger's own span. It is deliberately narrow: a transition out of a state whose entry action is
named is still legal, and the pre-existing endpoint rule still covers sources that are neither.
**Closed.**

### `Must be a Boolean expression.` — open, and the reason is tier gating

`TransitionUsage_invalid.sysml.xt`:60 (`if "test"`) is still **open**. The guard rule is implemented
and fires in isolation (`transition guard must be Boolean, found String`), but in this fixture the
parallel-state error at line 46 is a **name-resolution**-tier diagnostic, and the guard's type is
checked at the **type** tier, which the registry skips once a lower tier has errored
(`passes/registry.go`). The pilot validates every rule in one flat pass and so reports both.

**Category — adjudicated divergence, owned by tier gating (wave 12A).** Tiering is an architecture
invariant of this implementation (`AGENTS.md` §4: higher tiers are skipped when a lower tier errors),
chosen so that type diagnostics are never derived from unresolved names. Reporting this row would
require either flattening the tiers or exempting a rule from them; both belong to the slice that owns
tier gating, not to the parser. Nothing in the specification requires a particular *number* of
diagnostics for one file, so this is a divergence in reporting completeness rather than in a rule.

### `SemanticMetadata_valid.sysml.xt`:53 — a false positive on a valid file

**Was.** `expected '{' or ';' after declaration` at line 90 — the most serious row of this slice,
because the file declares file-wide silence.

**Reading.** The declaration in question is

```sysml
#g requirement r2 {
    subject;
    actor #B a;
    assume #g constraint b;
    require #g r1;
}
```

KerML 8.2.4 (`PrefixMetadataMember`) allows a metadata prefix on *any* member declaration, and
SysML §7.20's `SubjectMember`, `ActorMember` and `StakeholderMember` productions are member
declarations like the rest. Our specialised parsers for those three did not consume a prefix, so the
`#B` was read as the start of a new member.

**Done.** The three members carry `Prefixes` in the AST and the parser consumes prefix metadata
before the name. Row **closed**; the fixture is now silent for us.

---

## Bad-scope recovery — per fixture

Six fixtures, all in `src/org/omg/kerml/xpect/tests/parsing/`. They share a shape: a malformed
qualified name (`Test3::A:a`, `A..b`, `OuterPackage::B.b`) after an earlier line that also has an
unresolved reference (`feature aa subsets non;`). The pilot's ANTLR parser reports the syntax failure
*and* keeps linking, so it also reports the earlier reference; our recursive-descent recovery reports
one diagnostic at the point it resynchronises.

**What the specification fixes and what it does not.** KerML 8.2 defines the *grammar* and the
*resolution* of a qualified name; nothing in KerML or SysML states how an implementation must recover
from a name that does not parse, nor how many diagnostics one file must produce. The pilot's declared
texts here are ANTLR internals (`no viable alternative at input '..'`, `mismatched input`), which the
harness itself cannot expect us to reproduce. What *is* spec-derivable is that a reference whose
metaclass is wrong is an error at that reference — and that is what we changed.

| Fixture | Row | Verdict now | Category / owner |
|---|---|---|---|
| `BadScopeWithOnlyTwoDot` | :21 `Couldn't resolve reference to Feature 'non'.` | `same-location` — we report `subsets target must be a feature, found kermlType` at the declared offset | **adjudicated divergence** (parser, this slice): same offset, same severity, a metaclass finding rather than a failed lookup. KerML 8.3.4.5 makes a `subsets` target a Feature, and our resolution is not filtered by metaclass, so we resolve the classifier and reject its kind. |
| `BadScopeWithOnlyTwoDot` | :26 `Couldn't resolve reference to Type 'test'.` | `same-location` — `type must be a type, found package` | **adjudicated divergence** (pre-existing): we resolve a name the pilot does not and reject the kind; arguably the more precise answer. |
| `BadScopeWithOnlyTwoDotAtTheEnd` | :21 | `elsewhere-in-file` | **adjudicated divergence** (parser recovery): the malformed name ends the file, so our recovery consumes to EOF and the earlier `non` is not reported. Recovery strategy is unspecified. |
| `BadScopeWithOnlyTwoSingleDot` | :21, :26 ×3 | `elsewhere-in-file` + 3 `same-line` | **adjudicated divergence**: the three declared rows at :26 are one ANTLR failure reported three times (`'..'`, `'::'`, `'A'`); we report `expected a name after '.'` once. Reproducing an ANTLR alternative trace is not a specification obligation. |
| `BadScopeWithOnlyTwoSingleDotAtTheEnd` | :21, :26 ×3 | `elsewhere-in-file` + `same-location` + 2 `same-line` | as above. |
| `ScopeWithFourDotAndDot` | :22 ×2 | was **silence** — now `same-location` + `same-line`, `feature chain segment must be a feature, found kermlType` | **adjudicated divergence** (was a false negative, now reported): `feature c : OuterPackage::B.b;` chains through `OuterPackage::B`, a classifier. KerML 8.3.4.7 makes every segment of a feature chain a Feature, so the chain is invalid; we now say so at the declared offset. The pilot instead fails to *resolve* the two names. Same element, same offset, different rule — deliberately not admitted as wording-only. |
| `ParsingTests_Indexing` | :32 `noErrors` | **closed** | see below. |

**`ParsingTests_Indexing` was a real defect of ours.**

```kerml
feature a : A[*];
feature b = a#(1).b;
feature c = b.c;
```

We reported `unresolved member: c`. KerML 7.4.9: a feature with a `FeatureValue` and no declared type
is typed by its value, so `b`'s members are the members of `a#(1).b`; and `a#(1)` names one element of
the sequence `a`, of `a`'s own type. `resolve/document.go` now falls back to the value's type when a
usage declares none (guarded against cycles by `valuesInProgress`), and `resolve/target.go` resolves
`#(…)` indexing through its operand. Bracket indexing is deliberately *not* routed this way: `[…]`
carries quantity and selection meanings that are not "one element of".

**No bad-scope row is left classified as a false negative.** The one that was — `ScopeWithFourDotAndDot`
— is now a reported error at the declared offset.

---

## Query path expressions — a pilot limitation

`queryx/failing/QPE-Qualifier`, `QPE-Traversal` and `QPE-Wildcard` each declare `XPECT noErrors`, and
we report `expected a namespace member` (2–4 errors each). The forms are

```sysml
value v1_i: Integer[0..*] = .*/.*[Integer];
value v2_r: Real[0..*] = .**[Real];
value v_redefining: Integer = ./vehicle_1/cylinders/@redefining;
value vw_recursive: Integer[0..*] = .**/cylinders;
```

**What `failing/` means, established from the pinned artifacts rather than from the directory name.**
Running the pinned pilot's own SysML validator (`0.60.1`, the same jar the differential oracle uses) on
the three fixtures' model bodies reports, among others:

```
QPE-Wildcard.sysml:9:11: error: no viable alternative at input 'vw_single'
QPE-Wildcard.sysml:9:38: error: For input string: "."
QPE-Wildcard.sysml:9:40: error: no viable alternative at input '/'
QPE-Traversal.sysml:7:57: error: no viable alternative at input '@'
QPE-Qualifier.sysml:9:40: error: no viable alternative at input '/'
```

So **the pinned pilot does not parse these files either.** Their `XPECT_SETUP` runs them through
`SysMLQueryFailingTest extends KerMLXtextTests`, and the directory records the outcome: the declared
silence is the notation's *intent*, not behaviour of the pinned release. Nothing in KerML 8.2/SysML
§7 admits `.*`, `.**`, `/`-separated traversal or `@redefining` as expression syntax at this version.

**Category — pilot limitation. Owner: upstream.** Implementing these productions would mean inventing
notation neither the pinned grammar nor the specification defines, and it would move three `noErrors`
rows only by guessing at semantics. The question of whether query path expressions are intended for a
later version is drafted for upstream in [omg-issues.md](omg-issues.md).

---

## Rows this slice does not close

| Row | Category | Owner |
|---|---|---|
| `TransitionUsage_invalid.sysml.xt`:60 `Must be a Boolean expression.` | adjudicated divergence (tier gating: the guard's type tier is skipped after a name-resolution error) | tier gating (wave 12A) |
| `queryx/failing/QPE-Qualifier`, `QPE-Traversal`, `QPE-Wildcard` (3 `noErrors` rows) | pilot limitation — the pinned pilot rejects the same files | upstream ([omg-issues.md](omg-issues.md)) |
| `BadScopeWithOnlyTwoDot`:21, :26 | adjudicated divergence — same offset and severity, metaclass finding vs failed lookup | parser (this slice, recorded) |
| `BadScopeWithOnlyTwoDotAtTheEnd`:21 | adjudicated divergence — recovery to EOF, unspecified | parser (this slice, recorded) |
| `BadScopeWithOnlyTwoSingleDot`:21, :26 ×3 | adjudicated divergence — one recovery diagnostic against an ANTLR alternative trace | parser (this slice, recorded) |
| `BadScopeWithOnlyTwoSingleDotAtTheEnd`:21, :26 ×3 | adjudicated divergence — as above | parser (this slice, recorded) |
| `ScopeWithFourDotAndDot`:22 ×2 | adjudicated divergence — chain-segment metaclass rule at the declared offset, no longer a false negative | parser (this slice, recorded) |
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:38 (differential, only-ours warning) | adjudicated divergence — our dimensional analysis, which the pilot does not perform; unblocked here, not introduced here | units rules (pre-existing) |

The rows this slice **closed** are the seven listed under *Measurements*. Nothing else in wave 12D's
assignment is silently dropped: E1 and E2 are closed, the three parse-gap rules are closed, the
`SemanticMetadata_valid` false positive is closed, and the 14 bad-scope rows measured on the base
commit resolve into 13 recorded divergences plus 1 closure (`ParsingTests_Indexing`), with
`ScopeWithFourDotAndDot`'s two rows moved out of silence onto the declared offset.
