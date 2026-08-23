# Wave 12C — making library records lossless (roadmap L3): the design

Roadmap item **L3** asks for library records to become *lossless*: a restored library must equal a
parsed one, so a library keeps its members, declared values and condition bodies on both load paths.
This page is step 1 of the three the item prescribes — design the record format and its
version-compatibility surface, then prove parsed/restored equality as an enforceable test, then
migrate consumers. It is written for review **before** any of the format changes, because the
measurements below moved the plan: the cheapest lossless design turns out not to serialize member,
value and condition structure at all.

Nothing in this page changes behaviour. The three oracles are reproduced on the base commit as
controls, and they are unchanged because no code moved.

| Oracle | `main` at `0d4eb14f`, fresh cache | This branch |
|---|---|---|
| Xpect | 428 `.xt` files, 0 unparsed; 1269 agree (246 wording-only), 54 disagree | identical |
| Differential | 353 files, 317 fully agreeing; 25 agreed, 119 only ours, 73 only the pilot's | identical |
| Rejection | 120 cases: 116 both reject, 4 only the pilot rejects, 0 only we reject | identical |

Every number on this page was measured on this machine, on `0d4eb14f`, with a fresh cache
(`XDG_CACHE_HOME` pointed at an empty directory per run).

---

## 1. What is being replaced, and what the replacement owes

A standard library is **index-only**: `libs.Loader.LoadAll` parses what the cache missed and then
`reduce` replaces every parsed document with the same `IndexRecord` a hit restores
(`internal/core/libs/loader.go`). A library symbol therefore carries a name, a kind, a span,
specialization and `featured by` targets, an alias target, unit/dimension facts, behavior parameter
lists, annotation facts and compiled filter predicates — and **no `Decl`**.

That contract exists because the cache was previously observable: a hit produced a poorer state than
a miss, so `solve` evaluated library-inherited invariants and gRPC reported dozens of inherited
library attributes only until the cache warmed, and the conformance oracles gave two answers for one
tree. `internal/core/libs/index_only_test.go` is the proof that the cache is now free of semantic
effect, and it proves it the strong way: **no symbol of a library document has a `Decl` on either
path**, including with no cache at all.

Any replacement owes the same proof in the same enforceable form. Being lossless is not enough on its
own; the property to preserve is that **the presence, age or absence of a cache entry cannot change
one query's answer**. Concretely, the successor test must assert:

1. **Path equality.** For every symbol of every library document, every fact any consumer can read —
   kind, span, supertypes, members, multiplicity, declared value, condition body, annotations,
   filters — is equal cold and warm.
2. **No-cache equality.** The same holds with `cache == nil`, so disabling persistence is not a way
   to see a different library.
3. **Staleness is a miss.** A record produced by different code, a different file, or a different
   sibling file is never restored. Build identity (`libs.buildID`) and the set digest
   (`Loader.setDigest`) already do this and stay.

---

## 2. What "lossless" can mean, measured

Three designs satisfy "a restored library equals a parsed one". They differ in *what is persisted*,
and the measurements decide between them.

The load costs of the bundled library (95 files, 1.35 MiB of source, 19288 registered FQNs, 17129
distinct symbols):

| Operation | Time | Heap held | On disk |
|---|---|---|---|
| Parse all 95 files, discard the trees | 41 ms | — | — |
| Parse **and index** all 95 files as ordinary documents | 84 ms | 32.7 MiB | — |
| Today's cold load (parse → derive facts → replace with records) | 1.90 s | 15.7 MiB | 1.6 MiB of records |
| Today's warm load (restore 95 records) | 47 ms | 15.7 MiB | 1.6 MiB of records |

The decisive number is the first one: **parsing the whole standard library costs 41 ms**. Parsing is
not what the cache buys. What the cache buys is the 1.86 s of *derivation* that follows the parse, and
that derivation is dominated by two fact families:

| Fact family, over all 17129 symbols, each on a fresh model | Time |
|---|---|
| Unit reduction and dimension (`IsMeasurementUnit`, `UnitTermOf`, `DimensionFactsOf`) | 480 ms |
| Direct supertypes (`DirectSupertypes`) | 354 ms |
| Annotation facts (`AnnotationFactsOf`) | 6.8 ms |
| Behavior parameter lists (`BehaviorFactsOf`) | 0.6 ms |

### Option A — persist member, value and condition structure

The reading L3 was written with: grow `symRecord` with members, multiplicity, declared values and
condition bodies, and teach the consumers to read them.

This is the most expensive option and the least provable. A declared value or a condition body is an
expression tree; persisting it "structurally" means either persisting the tree (which is option B) or
inventing a second representation of expressions that the evaluator, the solver translation and hover
all have to understand *in addition to* `ast`. Every semantic rule then has two implementations — one
over `Decl`, one over the record — and path equality becomes a claim about two hand-written
implementations agreeing, re-argued for every rule added afterwards. It also contradicts
`AGENTS.md` §4 (semantics derive from the immutable AST; no parallel structures that can drift).

**Rejected**, on cost and on provability.

### Option B — persist the AST

Serialize the parse tree, restore it, `AddDocument` it. Equality is structural and provable by
round-tripping (`ast.Dump` is already a canonical rendering, and the dumps of the bundled library
total 4.1 MiB).

Two concrete obstacles, both verified in the tree rather than assumed:

- `ast.NodeBase` holds `leading`/`trailing` trivia in **unexported** fields, which `encoding/gob`
  silently drops. Doc-comment hover — one of the two reasons L3 exists — would be lost exactly where
  it is wanted, and lost *quietly*. Fixing it means exporting trivia or hand-writing
  `GobEncode`/`GobDecode` for every node.
- The tree is interface-valued (`Members []Node`, expression operands), so gob needs a
  `gob.Register` for each of the ~83 concrete node types in `internal/core/ast`, and a node type
  added later without a registration is a decode error — i.e. a silent miss at best.

And the payoff is inverted: this pays a large new on-disk format, a real version-compatibility
surface and ~83 registrations to avoid a **41 ms** parse, while still needing the fact derivation
that actually costs 1.86 s. **Rejected**, on cost/benefit.

### Option C — parse always; cache only the derived facts (recommended)

Stop treating the record as a *substitute* for the document and treat it as a *memo* of what the
document's analysis derives:

- `Loader.load` always parses and always `AddDocument`s. The library keeps its bodies on every path
  because they were parsed on every path — there is no restored-vs-parsed distinction left to be
  lossy about.
- The cache holds exactly the facts whose derivation is expensive: unit reduction, dimension,
  direct supertypes, `featured by` targets, behavior parameter lists, annotation facts, compiled
  filter predicates. On a hit these are installed as a pre-filled memo of `semantics.Model`; on a
  miss they are derived as today and stored.
- `AddRecords`, `symbols.RecordEntry`, and the `Decl`-less symbol shape they create are **deleted**.
  Nothing in the system then has a second, poorer notion of what a library symbol is.

This satisfies losslessness by construction, and it makes the cache's freedom from semantic effect a
*narrower* claim than today's: instead of "the record must reproduce every consumer-visible fact", it
is "a restored fact equals the fact the model would have derived", which is decidable per fact family
and is exactly what §4 states as a test.

Costs, measured, not speculative:

- **Warm load 47 ms → ~84 ms plus fact restore** (the parse-and-index floor). Still 20× better than
  the 1.90 s cold path, and the cache keeps its entire real win.
- **Heap 15.7 MiB → 32.7 MiB per index holding the library** (+17 MiB), because the trees stay
  reachable. `internal/grpc` prewarms `DefaultIndexPoolSize = 4` library indexes, so a default gRPC
  service pays about +68 MiB. This is the one cost that needs a decision from the reviewer
  (§6, open row **L3-4**).
- **The on-disk format does not grow.** It shrinks: names, kinds, spans, short names, alias targets
  and wildcard-import targets stop being persisted, because the parsed document states them.

---

## 3. The record format under option C, and its version-compatibility surface

The persisted type stays `libs.IndexRecord`, gob-encoded, one file per library document, at
`<cache>/sysml-ls/libs/<key>.idx`. What changes is that it becomes a fact table keyed by FQN rather
than a symbol table:

```go
// FactRecord is the persisted derivation of one library document's analysis.
type FactRecord struct {
    Name  string
    Facts []symFacts // in document order, keyed by FQN
}

type symFacts struct {
    FQN         string
    Supers      []string                  // resolved generalization targets
    FeaturedBy  []string                  // resolved `featured by` targets
    Unit        *unitFacts                // measurement units only
    Dimension   *symbols.DimensionFacts   // measurement units only
    Behavior    *symbols.BehaviorFacts    // behaviors and steps only
    Annotations []symbols.AnnotationFacts
    Filters     []*symbols.FilterPredicate // compiled namespace filters
    Imports     []wildcardImportFacts      // compiled filters of `import X::*[…]`
}
```

Dropped relative to `symRecord`: `ShortName`, `Kind`, `Span`, `AliasTarget`, and the `Target`/
`Private` fields of a wildcard import — all of them re-derived from the document at no measurable
cost, because the document is now always there.

**Version compatibility.** The rule is unchanged and it is the strong one: *a stale record is a miss,
never a wrong answer.* Three independent components of the cache key enforce it, and all three stay:

| Key component | What it invalidates | Why it is needed |
|---|---|---|
| `sha256(content)` | an edit to the file itself | the facts are derived from it |
| `-s<setDigest>` | an edit to **any** file of the library set | a unit reduction follows a reference unit declared in a sibling file |
| `-b<buildID>` | a change to the code that derived the facts | a fact is a function of our code, which no input covers; a dirty tree gets a per-build identity, so a modified working copy never reuses a record |
| `-v<formatVersion>` | a change to the persisted shape | a decode of an old shape is not attempted |

Two consequences worth stating explicitly, because they are what makes "stale is a miss" true rather
than aspirational:

1. **Decode failure is a miss.** `Cache.Load` already treats an absent file, a read error and a
   decode error identically. A `FilterPredicate` or `DimensionFacts` whose shape changed without a
   `formatVersion` bump is caught by `buildID` first; the version exists for the case where the code
   did not change but the encoding did (e.g. a field reordering in a vendored type).
2. **A partial record is impossible to observe.** `Cache.Store` writes a per-key temp file and
   renames it, so a crashed or concurrent write is never read.

The format version becomes `24` when this lands. `maxIdleAge` pruning is unaffected.

---

## 4. Step 2 — the equality proof, as an enforceable test

`index_only_test.go` is replaced, not deleted, by a test of the same strength. The successor asserts
three properties over the bundled library and over a purpose-built fixture that declares what only a
declaration exposes (members, a declared value, a condition body, a `[0..1]` multiplicity):

1. **`TestLibraryFactsAreEqualColdAndWarm`** — load twice through one cache directory and compare,
   for every symbol of every library document: the fact families of §3 (deep-equal), plus the
   consumer-visible answers `EffectiveMultiplicityOf`, `MembersOfIncludingRedefined`,
   `AnnotationFactsOf`, `Conforms` over every recorded supertype edge, and `Eval` of every declared
   value. A single loop over `idx.FQNs()` covers 19288 names, so the test cannot rot by omission the
   way an enumerated list would.
2. **`TestRestoredFactsEqualDerivedFacts`** — the sharper form: restore a record, then re-derive the
   same facts from the parsed document with a fresh `semantics.Model`, and require equality. This is
   the property that replaces "the cache has no semantic effect": a pre-filled memo that differs from
   what the derivation would produce is a bug the test names.
3. **`TestLibraryBodiesArePresentOnEveryPath`** — the inverse of today's assertion: every library
   symbol *has* a `Decl`, cold, warm and with `cache == nil`. Today's test proves the absence is
   uniform; this proves the presence is.

`TestMultiplicityOfALibraryFeatureIsTheSameColdAndWarm`
(`internal/core/semantics/cached_library_test.go:253`) flips at this point: it stops asserting the
assumed `1..1` and asserts the declared `0..1`, on both paths. Its comment already says so.

---

## 5. Step 3 — the consumers, and what each one starts seeing

A library body becomes visible to everything that walks declarations. This is a diagnostics change to
adjudicate per consumer, not plumbing. What is measured today, over the bundled library parsed as
ordinary documents:

- The parser reports **0 diagnostics** on all 95 files, so nothing arrives from the parse itself.
- `passes.Analyze` over those documents reports **307** diagnostics: 26 `unresolved` errors,
  118 `name-conflict` warnings, 94 `library-package` warnings, 40 `usage-typing` errors,
  14 `feature-reference-featuring-types` errors, 5 `specialization-cycle` errors,
  4 `invocation-not-behavior` errors, 3 `result-expression-at-most-one` errors, 2 `one-type` errors
  and 1 `redefinition-no-derived-name` warning.

None of those 307 reach a user by themselves: `Workspace` analyses only the documents it was given
(`w.docs`), and a library document is indexed, not opened. The 94 `library-package` warnings are an
artefact of the probe not marking the documents as library content, and would not appear through the
loader. But the list is the honest size of the residue this item uncovers, and two of its rows are
real work:

- **`solve`.** `internal/core/solve/differential_corpus_test.go` parses the library itself
  (`parseLibraries`) *because* records hold no conditions; that workaround becomes unnecessary and is
  deleted. In exchange, the translation must decide what to do with library invariants that are
  outside the SMT-translatable subset — the pre-index-only behaviour was to try, and that is what
  made a warm cache differ from a cold one. The gate is `solve`'s own subset check, which must skip a
  library condition it cannot translate rather than reporting on it.
- **`internal/grpc`.** `attributesOf` reads `sym.Decl` (`internal/grpc/attributes.go:142,167`), so
  inherited library attributes start appearing in responses again — dozens of them, which is the
  regression the index-only contract was adopted to stop. The difference is that now they appear on
  *both* paths, so the answer is stable; whether the API *should* report inherited library attributes
  is a separate adjudication, and it must be decided before this lands, not discovered by the
  oracles (open row **L3-3**).

A third consumer-visible gap is not a diagnostics question but a capability one:

- **Library value expressions must evaluate.** The roadmap names `isSolid = isEmpty(voids)`
  (`Systems Library/Items.sysml:105`) as a resolution failure. Measured, it is not one: with the body
  parsed, the callee resolves to `SequenceFunctions::isEmpty`, and it is `semantics.Model.Eval` that
  declines to fold the invocation (`ok == false`). The gap is in evaluation, not resolution, and the
  wording in `roadmap.md` should be corrected when this item is picked up.

---

## 6. Rows this page does not close

Every row below is open, with a category from
[the wave-11E categories](wave11e-decisions.md) where it is a divergence, and an owner.

| Row | What it is | Category | Owner |
|---|---|---|---|
| **L3-1** | The design itself is unimplemented: records are still index-only, and a library still contributes no bodies. | unimplemented obligation | L3, next slice |
| **L3-2** | `Model.Eval` cannot fold a library value expression that invokes a Kernel Function Library function over a feature (`isEmpty(voids)`). Resolution is fine; evaluation declines. | unimplemented obligation | L3, before consumers migrate |
| **L3-3** | Whether the gRPC element API should report attributes inherited from the standard library at all. Under option C it will, unless a policy says otherwise. | adjudicated divergence, once decided | wave owner + `internal/grpc` |
| **L3-4** | +17 MiB per library-holding index (+~68 MiB for a default gRPC pool of 4) is the price of keeping the trees. Accept, or make the pool size the mitigation. | not a divergence — a cost decision | wave owner |
| **L3-5** | The 26 `unresolved` errors the passes report over the parsed library (`that`, feature chains such as `CartesianVectorOf::result::dimension`, and implicit-redefinition targets). Each needs a category of its own once a consumer actually surfaces it. | not yet adjudicated | L3, after step 2 |
| **L3-6** | `roadmap.md`'s L3 text says library value expressions do not resolve; measured, they resolve and fail to evaluate. Also it says `TestMultiplicityOfALibraryFeatureIsTheSameColdAndWarm` skips, while it asserts. | our defect (documentation) | this page; corrected in the roadmap by this PR |

No oracle row moved, in either direction, so no conformance claim is made or implied here. The Xpect,
differential and rejection multisets are the controls in the table at the top of this page, measured
before and after, identical.

---

## 7. Sequencing

1. **This PR** — the design above, the measurements, and the roadmap correction. Nothing behavioural.
2. **Next slice** — option C's mechanism plus the three tests of §4, with `AddRecords` /
   `RecordEntry` deleted and `formatVersion` bumped to 24. No consumer behaviour change beyond what
   keeping the bodies forces, each movement attributed.
3. **Then** — the consumers of §5, one PR each: `solve`'s subset gate (and the deletion of
   `parseLibraries`), then the gRPC adjudication of **L3-3**, then `Model.Eval` for **L3-2**.

If the reviewer prefers option B, §2 states what it additionally needs (trivia and ~83 gob
registrations); if the reviewer rejects the +17 MiB of **L3-4**, option C does not survive it and the
item returns to option A, whose cost is stated in §2. Both are decisions for the review, which is why
this page ships before the mechanism.
