# OpenSysML — Roadmap

Baseline: `main` @ `1127a93b`, verified locally on 2026-08-27 with Go 1.25.0.
Read `AGENTS.md` first; it governs everything below.

0.3.0 is the newest release from `Open-MBEE/OpenSysML`, carrying `sysml`, `sysml-lsp` and
`sysml-grpc` archives, and the Python client is on PyPI as `opensysml` 0.3.1. `main` carries what
`CHANGELOG.md` lists as unreleased. Everything in "Release follow-through" is maintainer- or
account-gated; everything after it is ordinary engineering work.

Track status as of this baseline: **Tracks A, B, C and P are closed**, and their entries are
removed from this file rather than kept as a list of done work — `CHANGELOG.md` is the record of
what landed. Track P's last item, publishing the Python client to PyPI, is closed with the
`opensysml-v0.3.0` and `opensysml-v0.3.1` tags. **T1** — the deprecated "slot" spellings on the
wire, in the REPL and in the Python client — is closed too: they are removed before 0.1.0, with
proto field 3 and the name `slots` reserved. **Track D** (RDF) is the only engineering track
still open.

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`,
`go test ./...`, `go test -race ./...`, and the corpus gate run locally clean.

| Gate | Count |
|---|---|
| OMG training corpus | **100/100 clean** — no file reports a semantic error |
| Stdlib parser conformance | 97/97 clean — 94 vendored OMG files and 3 non-normative OpenSysML extensions |
| Execution conformance cases | 380 |
| gRPC conformance fixtures | 15 |
| Golden execution traces | 118 |
| Runtime robustness cases | 256 |
| gRPC robustness cases | 8 |
| Golden AST fixtures | 146 |
| Negative parser subtests | 261 |

Statement coverage, measured with `go test -cover ./...` at the baseline commit. It counts only
each package's own tests, which understates a package consumed by others: `internal/core/ast`
is at 90.3% and `internal/core/semantics` at 85.2% measured with `-coverpkg` over the whole
suite, and `cmd/sysml-grpc` is gated by a process lifecycle test whose child process
contributes no profile at all.

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/quickfix` | 100.0% | `internal/core/export` | 82.7% |
| `internal/core/format` | 97.2% | `internal/core/symbols` | 75.6% |
| `internal/core/suggest` | 93.2% | `cmd/sysml-lsp` | 71.7% |
| `internal/grpc` | 88.5% | `internal/core/lower` | 71.0% |
| `internal/repl` | 88.5% | `internal/core/source` | 70.6% |
| `internal/core/passes` | 88.2% | `internal/core/semantics` | 67.3% |
| `internal/core/lexer` | 87.7% | `cmd/sysml-grpc` | 34.9% |
| `internal/core/rdf` | 87.1% | `cmd/sysml` | 32.7% |
| `internal/lsp` | 85.3% | `internal/core/ast` | 19.0% |
| `internal/core/runtime` | 85.2% | | |
| `internal/core/resolve` | 84.0% | | |
| `internal/core/parser` | 83.9% | | |
| `internal/core/model` | 83.4% | | |
| `internal/core/libs` | 82.9% | | |

The corpus gate needs the corpus (`./scripts/download-training-examples.sh`) and never
re-baseline `internal/core/model/testdata/training_examples_expected.txt`: adjudicate each
drifted file and record the verdict in `docs/project/training-examples.md`.

A tag cannot be cut over a corpus regression: `.circleci/config.yml`'s `build-and-test`
downloads the corpus (cached on the download script) and runs the suite with
`OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, on `v*` tags as well as on branches.

---

# Release follow-through

Tagging a core release and publishing the Python client are both proven paths now: `v0.3.0` is
released on `Open-MBEE/OpenSysML` with its full archive set, and `opensysml-v0.3.1` uploaded the
client to PyPI, so `pip install opensysml` resolves. The client declares 0.3.2, whose PyPI upload
waits on the `opensysml-v0.3.2` tag. The procedure and its post-tag verification
are in `docs/project/releasing.md`.

## R3 — Homebrew tap

`packaging/homebrew/` holds a template with `__TAG__`/`__SHA256_*__` placeholders and
`scripts/render-homebrew-formula.sh` renders it from a tag's `SHA256SUMS.txt`. The tap
`Open-MBEE/homebrew-tap` exists and carries the 0.2.1 formula: `brew install
Open-MBEE/tap/opensysml` has been verified end to end on Linux (install, `brew test`,
`brew audit --strict --online`). The bump is automated: the tap updates itself from its own
scheduled workflow, rendering the formula from this repository's script and template at the
latest release tag — the 0.1.2, 0.2.0 and 0.2.1 bumps are all that workflow's commits, and
nothing here pushes to the tap. One thing remains:

- **Install it on a real Mac.** The darwin archives have never been executed on macOS; their
  checksums match the release manifest and nothing more.

`homebrew/core` — which would drop the tap and the trust step entirely — is gated on
[notability](https://docs.brew.sh/Package-Acceptance-Policy#notability) (75 stars / 30 forks /
30 watchers, or 225 / 90 / 90 self-submitted), so it is not a near-term option.

## R4 — code signing

macOS binaries are not Developer ID signed or notarized and Windows binaries are not
Authenticode signed, so a browser download trips Gatekeeper or SmartScreen. Root-caused in
`docs/project/macos-distribution.md`: it is `com.apple.quarantine`, not a missing signature — Go's
linker already ad-hoc signs darwin/arm64 — so ad-hoc `codesign` in CI would change nothing.
Notarization needs an Apple Developer account, a Developer ID certificate, an App Store Connect
API key in CI and a macOS runner. Windows needs an OV/EV certificate. Both are purchases, not
tasks.

## R5 — the VS Code extension is not released

Found in the 0.0.8 pre-release audit and still open. `editors/vscode` builds only as a PR CI
artifact: no `.vsix` is attached to a release and there is no marketplace listing, so a user
cannot install it without building it. The blocker that made the built extension unusable *is*
fixed — the client appends `--stdio`, which `sysml-lsp` rejected with exit 2, so it crash-looped;
`--stdio` is accepted now and the server also honours `shutdown`/`exit` instead of leaking a
process. What remains is packaging and publishing: `vsce package` in the release workflow, a
`.vsix` on the release, and (for the marketplace) a publisher account and a PAT in CI — the same
class of account gate as R4.

## L3 — a library contributes names but no bodies (largely done)

Every library file is parsed and indexed on every load path, so a library contributes its
declarations — members, declared values, condition bodies, feature multiplicity — whether or not
the cache held anything for it, and the library is built once, frozen, and read through a
per-model overlay (`libs.Loader`, `libs.SharedBase`, `symbols.NewOverlay`) rather than copied per
model. gRPC reports an element's own and non-library-inherited attributes and counts the
standard-library-inherited ones it withholds in a `SymbolInfo` field rather than omitting them
silently. The shared base measures 17.2 MiB once per process plus 1.5 MiB per model, where each
model used to carry a ~16 MiB index of its own.

**Still open:** `Model.Eval` declines to fold a library value expression that invokes a Kernel
Function Library function over a feature (`isSolid = isEmpty(voids)`,
`Systems Library/Items.sysml:105`, whose callee resolves), and the solver's translatable subset
does not yet take library-declared conditions — `solve`'s differential harness still indexes the
standard library as ordinary documents (`parseLibraries`) to reach them.

---

# Track D — model persistence and RDF interchange

Saving and SysML ↔ RDF Turtle conversion landed (`internal/core/rdf`,
`internal/core/export`, `%save`, `sysml -convert`); see
[the RDF mapping](../reference/rdf-mapping.md).

The RDF direction ships **experimental** as of 0.1.0, because of D1–D3 and D7 below: its
vocabulary may change without a compatibility path, and no triplestore interop has
been demonstrated. Every surface says so (`export.ExperimentalNotice`), and
promoting it to stable is D3, not a documentation change.

Measured on the built binary at this baseline, **260 of the 334 models under `examples/`
convert**, including their behavior: the initial and final node, `perform`, `send`, `accept`,
`terminate`, `assign`, the fork/join/merge/decision control nodes, `while`/`loop`/`for`,
`if`/`else`, and a state machine's states, substates, regions, `entry`/`do`/`exit`, `defer`,
pseudostates and transitions each round-trip `notation → RDF → notation` byte-identically
(`internal/core/export/behavior.go`, `behavior_test.go`, and the tables in
`docs/reference/rdf-mapping.md` § Behavior). A model the mapping cannot write back is refused
rather than converted lossily, and the 74 refusals sort into: 35 declarations that name no
element of their own (the notation cannot be rebuilt from a graph keyed by name), 18 prefix
metadata, 10 expression forms carried as source text (operator, feature-chain, feature-reference,
invocation and constructor expressions — **D1**), 7 successions whose end is not a basic name
(**D2**), and 4 duplicate declarations, where two members of one namespace share a name the
graph would merge.

## The target, stated precisely

The goal is that a graph OpenSysML writes can **stand in for the RDF
`flexo-mms-sysmlv2` produces**: loaded straight into `flexo-mms-layer1-service` as a
branch's model graph, and read back through the SysML v2 API surface as the same
elements, without that service having produced it. Two consequences shape D1–D3, and
both were read from the two services' sources rather than from our own docs:

- **Layer 1 imposes no vocabulary at all.** `routes/gsp/ModelLoad.kt` loads whatever
  triples the request body carries into a load graph, diffs it against staging and
  commits; sanitization (`sanitizeCrudObject`) applies to LDP CRUD objects — orgs,
  repos, branches, policies — not to model triples. So layer 1's requirements are
  transport-level: a Turtle body on `PUT .../branches/{branch}/graph` (or SPARQL update
  on `.../update`), the ETag precondition, an optional `?message=`, and the literal-size
  limit (`maximumLiteralSizeKib`). Named-graph layout, commits, locks and provenance are
  layer 1's own and are not ours to emit.
- **The vocabulary contract belongs to the reader in `flexo-mms-sysmlv2`.**
  `ElementApi.extractModelElementToJson` is what turns triples back into API payloads,
  and it is stricter than `Namespaces.kt` suggested. Matching the namespaces is
  necessary and nowhere near sufficient; the specific mismatches below are what D3 has
  to close, and each is a source-level fact about that reader, not a hypothesis.

What matches today: `sysml:` = `https://www.omg.org/spec/SysML#` and `elmt:` =
`urn:sysmlv2:element:` are identical to `Namespaces.kt`; `rdf:type` plus
`sysml:<property>` per scalar field is the shape the reader expects; and our typed
literals fall in the datatypes it maps (`xsd:boolean`, `xsd:integer`,
`xsd:decimal`/`xsd:double`, otherwise string).

## D3 — make a converted graph readable through Flexo, and prove it

D3 is not "run the harness": the harness is landed and the graph fails it today, for reasons
that are known. Of its five sub-items only collection-valued properties (**D3.4**) remain —
identity (D3.1), element ids (D3.2), ownership (D3.3) and the harness itself (D3.5) are done.

### D3.1 — element identity: qualified names cannot address an element

**Done.** `rdf.EncodeElementID` encodes a qualified name reversibly into `[A-Za-z0-9_-]+`
(`::` → `__`, a byte outside `[A-Za-z0-9-]` → `_` plus two lowercase hex digits, so
`A_B::C` → `A_5fB__C` and `A::B_C` → `A__B_5fC` stay distinct), `rdf.ElementIRI` mints
`urn:sysmlv2:element:<encoded id>`, and the decoder reads identity from
`sysml:qualifiedName` alone — an element referenced without that property is reported as
unsupported, never named from its IRI (`internal/core/rdf/ids.go`,
`internal/core/export/rdf_in.go`, `docs/reference/rdf-mapping.md` § Element IRIs).

That encoding is what the reader requires: it derives an element's `@id` as `substringAfterLast(':')`
and runs `requireValidId` (`[a-zA-Z0-9_-]+`) on every by-id path, so a qualified-name IRI both
collapses `A::Widget` and `B::Widget` onto one id and cannot be requested at all.

One cost of the readable encoding stays open to check: the OMG API's own implementations do
treat element ids as UUIDs, so a readable id may be rejected by a client other than Flexo —
that is inference from convention rather than something read out of Flexo's sources, and it is
worth checking against whichever client is meant to consume the graph.

### D3.2 — `sysml:elementId` is required and is not emitted

**Done.** Every element carries `sysml:elementId`, holding exactly the id its own IRI
ends in, so the triple and the reader's `urnSuffix`-derived `@id` cannot disagree
(`internal/core/export/rdf_out.go` `head`, `ownership_graph_test.go`). Paged listing
selects on `?e sysml:elementId ?id` (`listElementsConstructQuery`) and `QueryApi`
rewrites an `@id` constraint to it, so without the triple an element was invisible to
both even though a direct construct on the IRI found it.

An **expression node carries one too**. Its id was `<owner id>.<position>`, which
`requireValidId` rejects, so a direct read of a node was refused with 400 before the store
was touched — measured against a live service. The position is now joined with `_p` and
encoded, keeping the id inside `[a-zA-Z0-9_-]+` and disjoint from element and membership
ids (`internal/core/rdf/ids.go` `ExpressionNodeID`, `ids_test.go`), and the node states that
id in `sysml:elementId`. A node is still not a model element — no `qualifiedName`, no
ownership, reachable only from the position that holds it — so writing expressions as
elements owned through memberships remains D1.

### D3.3 — ownership: every element reads as a root

**Done.** Ownership is materialized as the abstract syntax states it, so the roots
endpoint — which filters on `sysml:owner` and `sysml:owningRelatedElement` being unbound
or `rdf:nil` — sees one root per document instead of every element
(`internal/core/export/rdf_out.go` `owningMembership`/`relationshipOwnership`,
`internal/core/rdf/ids.go` `OwningMembershipID`, `docs/reference/rdf-mapping.md`
§ Ownership):

- every member states `sysml:owner` as an element reference, plus
  `sysml:owningMembership`/`sysml:owningRelationship`, or `sysml:owningRelatedElement`
  where a relationship owns it;
- a namespace member is owned through a minted `OwningMembership`, a type's feature
  through a `FeatureMembership` (both concrete in KerML), each with its own IRI,
  `sysml:elementId`, `memberElement`/`ownedMemberElement`/`ownedRelatedElement`,
  `owningRelatedElement` and `membershipOwningNamespace`, and the owner's
  `ownedMember`/`ownedMembership`/`ownedRelationship` — and `ownedFeature`/
  `ownedFeatureMembership` for a feature — so a client walking the payloads from a root
  reaches every member;
- a relationship a namespace declares (an import, a dependency, a state's entry
  membership) is owned directly rather than through a minted membership, which is also
  what keeps namespace collection properties off a relationship's domain;
- membership ids are `rdf.OwningMembershipID`: the member's id plus `_om`, deterministic,
  reversible, and unable to collide with an element id;
- visibility moves to the membership, which is where the metamodel declares it;
- the decoder traverses memberships, still takes identity from `sysml:qualifiedName`, and
  still accepts the compact `sysml:owningNamespace`-only shape; a membership missing an
  end is a typed error naming `sysml:memberElement`.

Not demonstrated: a graph loaded into a live Flexo instance and read back. That is D3.5.

### D3.4 — collection-valued properties need the JSON annotation

The reader **skips** a `sysml:` predicate with more than one object and prefers the
annotation literal at `urn:sysmlv2:annotation:json:<key>`, which it parses as JSON.
Anything multi-valued we emit as bare repeated triples is silently dropped on read. So
the encoder must write both forms for collections, and the decoder must accept the
annotation form when reading a foreign graph.

### D3.5 — the harness, once D3.4 is in

Layer 1's `src/test/resources/docker-compose.yml` brings up Fuseki, MinIO and the store
service; `deploy/` generates `cluster.trig` for cluster init; the service needs
`FLEXO_MMS_ROOT_CONTEXT`, `FLEXO_MMS_QUERY_URL`, `FLEXO_MMS_UPDATE_URL` and
`FLEXO_MMS_GRAPH_STORE_PROTOCOL_URL`. The test: create org/repo/branch, `PUT` our
Turtle to the branch graph, then read the elements back through `flexo-mms-sysmlv2`
(`GET /projects/{p}/commits/{c}/elements`, and by id) and compare against the payloads
that service's own commit path stores for the same model. That comparison — our graph
vs. theirs for one model — is the actual compliance statement, and it is what promotes
the RDF path off experimental. Keep it out of `go test ./...`: an opt-in build tag or
environment gate, like the corpus gate.

**Landed, ahead of D3.1–D3.4 rather than after them**, as `internal/interop/flexo` and the
`FLEXO_INTEROP` gate `TestFlexoInterop`, documented in `.agents/skills/flexo-interop`. It
measures the gap instead of asserting the fix, so D3.1–D3.4 show up as movement in
`internal/interop/flexo/testdata/interop_expected.txt`. Two corrections to the setup described
above, from doing it: the published `openmbee/*` images are enough (no source build, no MinIO,
no regenerated `cluster.trig` — use `flexo-mms-sysmlv2/docker-compose/docker-compose.yml`,
which brings up the SysML v2 API as well), and the `sysmlv2` org that `POST /projects` needs is
not seeded by cluster init, so the harness creates it.

What the first run measures: 29 of 29 elements listed and 86 of 142 properties delivered on the
graph-load side, against 33 of 33 and 158 of 158 for the same model posted through the service's
own commit path. The 56 lost properties are the `sysx:` namespace plus multi-valued standard
properties (**D1**/**D2**/**D3.4**); no element carries `sysml:elementId` (**D3.2**) and none
carries `sysml:owner`, so all 29 read as roots (**D3.3**); the eight `expr:` node ids carry a
`.` and are refused by a direct read, which is **D3.1**'s identity question reaching the
expression nodes too. Two deployed behaviors differ from what the sources suggest: the element
listing ignores `pageSize`/`pageAfter` and returns every subject of the branch graph, so it also
never applies its own `sysml:elementId` filter — elements missing `elementId` are visible today
only for that reason — and project delete is a soft annotation that leaves the Layer 1 branch
behind.

That run measured the output as it stood before **D3.2**/**D3.3** landed: every element now
carries `sysml:elementId` and its ownership, and no `expr:` node id holds a `.` any more. The
recorded expectation has not been re-measured against the current output, so the movement those
two make is still unmeasured; a stack has to be brought up and
`go test ./internal/interop/flexo -run TestFlexoInterop -update-flexo` re-run for it.

## D1 — expressions are carried as source text, not as triples

Feature values, multiplicity bounds, filter conditions and succession guards are stored as
their notation. They round-trip exactly, but SPARQL cannot see inside them, so a query like
"every part whose mass exceeds 1000" is not expressible against the graph. Mapping KerML
expression trees to RDF is the fix and is a feature in its own right: it needs a node-identity
scheme for subexpressions, which is the part to design first — and it is the same identity-encoding
question D3.1 settles for elements, so design them together.

Under the drop-in target this is no longer only a queryability gap. The Flexo reader keeps
`sysml:` and `urn:sysmlv2:annotation:json:` predicates and **ignores everything else** (the
unrecognized-predicate error is commented out in `extractModelElementToJson`), so every
`sysx:` triple — `sourceText`, `memberIndex`, `hasBody`, `declaredKeyword`, the behavioral
properties — is dropped the moment a graph passes through that service. Whatever a model needs
in order to survive has to be standard, which makes D1 and D2 part of the interop goal rather
than refinements after it.

## D2 — end-binding heads depend on `sysx:sourceText`

`connect`, `bind`, `flow`, `succession`, `transition`, `accept` and `satisfy` keep their head
verbatim, so a graph produced by *another* tool converts to notation only as far as the
structural properties reach and then reports the element as unsupported. Emitting real end
triples (`sysml:source`/`sysml:target`/`sysml:connectorEnd`) would remove the dependency; the
parser already has the ends, so this is an encoder/decoder change rather than a parser one.
By the `sysx:`-is-dropped rule above, these ends are also what a Flexo-mediated round trip
would lose entirely, so D2 is the second-largest interop item after D3.

## D7 — reference-valued properties are emitted as strings, and one metaclass is abstract

The reader turns a resource-valued object into `{"@id": …}` and a literal into a string, so a
property the API defines as a reference has to be an element IRI in the graph. `imports.golden.ttl`
shows both halves of this gap: `sysml:importedNamespace "ISQ"` is a string where the API expects
a reference, and the metaclass is `sysml:Import`, which is abstract in KerML — the API's own
elements are `NamespaceImport` or `MembershipImport`. Audit every property the encoder writes
against the API schema for reference-vs-literal and every metaclass name for being concrete;
this is mechanical once D3.1 gives references something stable to point at.

The reference-vs-literal half of that audit is now mechanized, against the OWL ontology rather
than the API schema (D8): `TestGoldenGraphsMatchOntology` (`internal/core/export`) checks every
SysML-namespace triple in the 24 golden graphs against the metamodel's declared domain and range,
and the 135 triples it flags are inventoried key-by-key with a reason in
`internal/core/export/testdata/ontology-known-violations.txt`, so any *new* disagreement fails the
build. 20 of them are this item's own bug — an object property carrying a name as a literal, over
seven distinct metaclass/property pairs: `type` on `AttributeUsage`, `ReferenceUsage` and
`PartUsage`, `sourceFeature` on `SuccessionAsUsage` and `sysx:InitialNode`, `referent` on
`FeatureReferenceExpression`, and `targetFeature` on `FeatureChainExpression`.
`sysml:importedNamespace "ISQ"` is *not* among them, because the ontology declares that property on
`NamespaceImport` while we type the element `sysml:Import`, which makes the triple a domain
mismatch first; that is the same defect seen from the other side. The abstract-metaclass half is
not mechanizable from the ontology: `SysML.owl` records no ecore abstractness (see D8), so nothing
in the suite catches `sysml:Import` being abstract, and the audit against the API's own element
list stays manual.

## D8 — an optional second output profile: the Open-MBEE SysML v2 OWL ontology

[`Open-MBEE/sysmlv2-rdf-ontology`](https://github.com/Open-MBEE/sysmlv2-rdf-ontology) renders the
OMG metamodel (version 202407, from `SysML.ecore`) as OML and OWL: `sysml2/owl/.../SysML.owl`, 172
classes, 348 object properties, 63 datatype properties, with `rdfs:domain`/`rdfs:range` on each and
a reasoned bundle alongside it (`Owl Reason 2.12.0`). It uses the *same* namespace we do,
`https://www.omg.org/spec/SysML#`, and its class IRIs are the plain metaclass names we already
emit (`sysml:PartUsage`). The difference is the properties: each is qualified by the metaclass that
defines it — `sysml:Element_declaredName` (a datatype property), `sysml:Element_owningNamespace`
(an object property whose range is a `Namespace`), `sysml:Element_owner` (range
`OwningMembership`) — and a conformant instance graph therefore materializes the abstract syntax's
relationship elements rather than collapsing them. That is where the verbosity the Flexo team
objects to comes from, and it is inherent to the ontology rather than incidental.

So this is a **second profile selected by a flag, not a superset**: the property IRIs differ, so
one graph cannot satisfy both conventions, and Flexo's convention stays the default — under the
ontology's names Flexo's reader would hand a client payload keys like `Element_declaredName`. The
encoder already separates the term layer (`rdf.SysMLTerm`, `internal/core/rdf/vocab.go`) from the
structural decisions (`internal/core/export/convert.go`), so the profile is mostly a term-mapping
layer: property name → defining metaclass. Generate that table from `SysML.owl` and vendor it
rather than hand-writing 400 entries, so it can be regenerated when the ontology version moves.

Two things the flag does not buy. Full instance-level conformance still needs real triples where
we currently write `sysx:sourceText` (D1, D2):
`sysx:` has no place in that ontology at all, so an ontology-profile graph is conformant only as
far as those items have landed, and the profile should say so where it is documented. What it does
buy immediately is a **validation gate we do not have**: the TBox declares domain and range for
every property, so emitted triples can be checked against it with any OWL tool — a property
attached to the wrong metaclass is currently caught by nothing in our suite. That check is worth
running against the ontology's names even before the profile is user-facing.

**The table and the gate are done; the profile itself is not.**
`internal/core/rdf/ontology` holds the term table generated from `SysML.owl` by
`internal/core/rdf/ontology/gen`, which reads a local checkout of the ontology rather than
vendoring its 624 KB of RDF/XML and records the version (`202407`) and the upstream commit SHA in
the generated header (`README.md` documents regeneration). It carries 411 properties — 348 object,
63 datatype — each with its unqualified name, defining metaclass, full ontology IRI, kind and
declared `rdfs:range`, plus all 172 classes with their `rdfs:subClassOf` parents for the
ancestor-or-self test a domain check needs. Those 411 properties span only **336 distinct
unqualified names: 59 names are declared by more than one metaclass** (`type`, `value`,
`visibility`, `source`, `target`, `feature`, `result`, …), so the unqualified convention is
genuinely lossy in the other direction — an ontology-profile encoder cannot recover the qualified
IRI from the property name alone and has to pick by the subject's metaclass. `LookupProperty`
therefore returns every declaration and `AmbiguousNames` reports the set rather than one of them
being chosen silently.

One thing the ontology does **not** carry: **ecore abstractness**. The only abstractness in
`SysML.owl` is the metamodel's own `Type::isAbstract` property, which is about modeled types, not
metaclasses; the `owl:Class` declarations state no abstract/concrete marker. So the "the metaclass
must be concrete" check that would have caught D7's abstract `sysml:Import` is not possible from
this table and is not implemented — D7's abstract-metaclass audit stays manual.

The gate is `TestGoldenGraphsMatchOntology` (`internal/core/export/ontology_gate_test.go`), reading
the golden graphs from `testdata/convert` dynamically. Against the 24 of them it finds **133 triples
in 48 distinct metaclass/property violations**: 39 triples (10 keys) whose property is declared on a
metaclass that is not the subject's or an ancestor of it, 40 (14) typed by a class the ontology does
not declare, 34 (17) naming a property no metaclass declares, and 20 (7) giving an object property a
literal (D7). No datatype property gets an IRI, and no subject carrying SysML properties lacks an
`rdf:type`. Every one is inventoried with a one-line reason in
`testdata/ontology-known-violations.txt` and the gate fails on anything new, so the encoder was left
alone. The inventory is also the profile's own work list, since it sorts the gap into four causes:
properties the metamodel declares on a relationship or membership element that we collapse into the
element (`value` → `FeatureValue`, the multiplicity bounds → `MultiplicityRange`, `isNegated` →
`Invariant`, a transition's ends → `Connector`) — the same collapse D3.3 undid for ownership and
visibility; names as
literals — D7 and D2; 12 metaclasses of our own `sysx:` namespace plus two names the 202407
rendering does not have at all (`FlowUsage`, which it calls `FlowConnectionUsage`, and
`TerminateActionUsage`); and 17 properties we write into the SysML namespace that no metaclass
declares, each either a relationship the metamodel reifies as an element (`specializes`, `subsets`,
`redefines`, `references`, `aliasedElement`, `via`) or a notation flag with no metamodel property
(`isAccept`, `isResult`, `isSnapshot`, `isTimeslice`, `isChain`) — arguably those belong in `sysx:`
regardless of this item.

Scope: the profile plumbing is the remainder, and is about one session's work now that the table and
the gate exist; conformance beyond that is gated on D2 and D1 rather than on this item.

# Suggested sequencing

Tracks A, B, C, P and T1 are closed and their entries are removed. Within Track D, **D5** (the
`variant` and `include` keyword prefixes) and **D6** (behavioral nodes without a metaclass) are
closed as well. What is left is:

1. **R3**/**R4**/**R5** as the hardware and account access appears: a real Mac to install the
   tap's darwin bottle on, an Apple Developer and an OV/EV certificate to sign with, and a
   marketplace publisher for the extension. None of the three gates the others.
2. **Track D** is independent of the rest and can run whenever. Take **D3.4** first, the last
   of D3 — the harness is landed and measures the gap, so it and everything after it show up as
   movement in `internal/interop/flexo/testdata/interop_expected.txt` rather than as a claim.
   **D7** next, since it is mechanical now that identity is stable, then **D2** — a succession
   end that refers to an unnamed member belongs with it, both wanting real end triples rather
   than names or text — and **D1** last, as the largest piece of design. **D8** (the
   OWL-ontology output profile) is optional and additive: its domain/range gate is already in
   the suite, but the profile only becomes fully conformant behind D2 and D1, so it does not
   belong ahead of them.
3. **L3** is what remains of the library work: `Model.Eval` folding library value expressions
   and the solver taking library-declared conditions. It is independent of the release section
   and of Track D.
