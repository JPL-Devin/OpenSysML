# OpenSysML — Roadmap

Baseline: `main` @ `c28b5f1`, verified locally on 2026-08-19 with Go 1.25.13.
Read `AGENTS.md` first; it governs everything below.

0.1.0 is released from `Open-MBEE/OpenSysML`, carrying `sysml`, `sysml-lsp` and `sysml-grpc`
archives. `main` now carries everything cut under 0.1.1 in `CHANGELOG.md`, which is awaiting its
tag. Everything in "Release follow-through" is maintainer- or account-gated; everything after it
is ordinary engineering work.

Track status as of this baseline: **Tracks A, B, C and P are closed**, and their entries are
removed from this file rather than kept as a list of done work — `CHANGELOG.md` is the record of
what landed. Track P's remaining item, publishing to PyPI, is R2 and account-gated. **T1** — the
deprecated "slot" spellings on the wire, in the REPL and in the Python client — is closed too:
they are removed before 0.1.0, with proto field 3 and the name `slots` reserved. **Track D**
(RDF) is the only engineering track still open.

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`,
`go test ./...`, `go test -race ./...`, and the corpus gate run locally clean.

| Gate | Count |
|---|---|
| OMG training corpus | **100/100 clean** — no file reports a semantic error |
| Stdlib parser conformance | 95/95 clean — 94 vendored OMG files and 1 non-normative OpenSysML extension |
| Execution conformance cases | 344 |
| gRPC conformance fixtures | 15 |
| Golden execution traces | 109 |
| Runtime robustness cases | 195 |
| gRPC robustness cases | 8 |
| Golden AST fixtures | 107 |
| Negative parser subtests | 167 |

Statement coverage, measured with `go test -cover ./...` at the baseline commit. It counts only
each package's own tests, which understates a package consumed by others: `internal/core/ast`
is at 85.7% and `internal/core/semantics` at 83.9% measured with `-coverpkg` over the whole
suite, and `cmd/sysml-grpc` is gated by a process lifecycle test whose child process
contributes no profile at all.

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/quickfix` | 100.0% | `internal/core/parser` | 75.8% |
| `internal/core/format` | 97.2% | `internal/core/model` | 74.4% |
| `internal/core/suggest` | 92.6% | `internal/core/symbols` | 71.3% |
| `internal/core/source` | 90.9% | `cmd/sysml-lsp` | 71.1% |
| `internal/grpc` | 89.9% | `internal/core/lower` | 63.9% |
| `internal/repl` | 89.3% | `internal/core/resolve` | 77.9% |
| `internal/core/export` | 89.0% | `internal/core/semantics` | 57.3% |
| `internal/core/rdf` | 86.7% | `cmd/sysml` | 24.7% |
| `internal/core/lexer` | 85.5% | `internal/core/ast` | 20.9% |
| `internal/core/runtime` | 85.0% | `cmd/sysml-grpc` | 18.3% |
| `internal/core/passes` | 84.9% | | |
| `internal/core/libs` | 84.4% | | |
| `internal/lsp` | 81.5% | | |

The corpus gate needs the corpus (`./scripts/download-training-examples.sh`) and never
re-baseline `internal/core/model/testdata/training_examples_expected.txt`: adjudicate each
drifted file and record the verdict in `docs/project/training-examples.md`.

The gap found in the 0.0.8 pre-release audit — only the GitHub Actions PR workflow downloaded the
corpus and set `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, so `.circleci/config.yml`, the pipeline that
*builds release tags*, skipped the gate silently and a tag could be cut over a corpus
regression — is closed: `build-and-test` downloads the corpus (cached on the download script) and
runs the suite with that variable set, and it runs on `v*` tags as well as on branches.

---

# Release follow-through

## R1 — tag 0.1.1 (maintainer, blocking everything else in this section)

`v0.1.0` is tagged and released on `Open-MBEE/OpenSysML`, so what remains is the same procedure
for the next release. Releases live on `Open-MBEE/OpenSysML`; development happens on
`JPL-Devin/OpenSysML`, which has no tags at all. So the tag is preceded by promoting `main`
upstream, as 0.0.4 was through Open-MBEE PR #47:

```bash
# on Open-MBEE/OpenSysML, after main carries the release commit
git tag -a v0.1.1 -m "v0.1.1"
git push origin v0.1.1
```

The publish job needs `GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project.
Without one the tag builds artifacts and then fails at publish, having created no release.
Nobody has verified which is set. Full procedure and post-tag verification:
`docs/project/releasing.md`.

## R2 — publish `opensysml` to PyPI (account-gated remainder)

The job exists: `publish-pypi` in the `release-python` workflow, filtered to `opensysml-v*`,
building a wheel and an sdist, checking them with `twine check --strict`, installing the wheel
into a clean virtualenv and only then uploading. The version is declared once, in
`python/opensysml/_version.py`, and a tag that disagrees with it fails before upload. The
package keeps its own version line on purpose: it resolves a `sysml-grpc` binary at runtime
from whichever release the caller names, so its version and the core's are not lockstep.
See `docs/project/releasing.md`.

One decision precedes the upload, found in the 0.0.8 pre-release audit: `python/opensysml/_version.py`
declares `0.2.0` while the newest published artifact is `0.1.1`, so the first upload has to be
`opensysml-v0.2.0` (the tag-versus-source check refuses anything else) and 0.2.0's Python-side
changes — `evaluate`/`ExecutionError`, pinned checksums, subject-aware `eval`, generated typed
classes — all land in that one release rather than incrementally.

What remains is account-gated and cannot be done from a session: create the PyPI project's
first release with an account-scoped token, then replace it with a project-scoped one; create
the restricted CircleCI context `PyPI` holding `PYPI_API_TOKEN` (and optionally
`TEST_PYPI_API_TOKEN` for pre-release tags).

Also decide the default download repository. `python/opensysml/binary.py` defaults to
`Open-MBEE/OpenSysML`, releases are currently cut from `JPL-Devin/OpenSysML`, and
`OPENSYSML_GITHUB_REPO` is the override. `sysml-grpc` assets ship from 0.0.5 onward,
so `opensysml` can fetch a binary from a released tag; `pip install opensysml` still waits on the
PyPI project above.

## R3 — Homebrew tap

`packaging/homebrew/` holds a template with `__TAG__`/`__SHA256_*__` placeholders and
`scripts/render-homebrew-formula.sh` renders it from a tag's `SHA256SUMS.txt`. The tap
`Open-MBEE/homebrew-tap` exists and carries the 0.0.4 formula: `brew install
Open-MBEE/tap/opensysml` has been verified end to end on Linux (install, `brew test`,
`brew audit --strict --online`). Two things remain:

- **Install it on a real Mac.** The darwin archives have never been executed on macOS; their
  checksums match the release manifest and nothing more.
- **Automate the bump** so the pinned hashes can't go stale (the old C3): a tag-triggered step
  that renders the formula and opens a PR against the tap. Needs a CI secret with write access
  to the tap repository.

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
class of account gate as R2/R4.

---

# Track D — model persistence and RDF interchange

Saving and SysML ↔ RDF Turtle conversion landed (`internal/core/rdf`,
`internal/core/export`, `%save`, `sysml -convert`); see
[the RDF mapping](../reference/rdf-mapping.md).

The RDF direction ships **experimental** as of 0.1.0, because of D1–D3 and D7 below: its
vocabulary may change without a compatibility path, and no triplestore interop has
been demonstrated. Every surface says so (`export.ExperimentalNotice`), and
promoting it to stable is D3, not a documentation change.

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

D3 is no longer "run the harness": the harness would fail today, and the reasons are
known. Five sub-items, in the order to take them.

### D3.1 — element identity: qualified names cannot address an element

**Done.** `rdf.EncodeElementID` encodes a qualified name reversibly into `[A-Za-z0-9_-]+`
(`::` → `__`, a byte outside `[A-Za-z0-9-]` → `_` plus two lowercase hex digits, so
`A_B::C` → `A_5fB__C` and `A::B_C` → `A__B_5fC` stay distinct), `rdf.ElementIRI` mints
`urn:sysmlv2:element:<encoded id>`, and the decoder reads identity from
`sysml:qualifiedName` alone — an element referenced without that property is reported as
unsupported, never named from its IRI (`internal/core/rdf/ids.go`,
`internal/core/export/rdf_in.go`, `docs/reference/rdf-mapping.md` § Element IRIs).

The reader derives an element's `@id` as `urnSuffix`, defined as
`substringAfterLast(':')`. So `elmt:Demo::Vehicle` reads back as `Vehicle`, and
`A::Widget` and `B::Widget` collapse onto one id. Every by-id path also runs
`requireValidId`, whose regex is `[a-zA-Z0-9_-]+`, so an id containing `::` cannot be
requested at all. Qualified-name IRIs are therefore unusable as Flexo element identity.

The fix is an id that satisfies that regex while keeping conversion deterministic. **A UUID
is not required**: Flexo types every id as a plain `String` (`Identified.atId`, and no
`format: uuid` anywhere in its `openapi/openapi.yaml`); `generateId()` returns a random UUID
only as the default when a request omits an id. So the id can stay readable — encode the
qualified name reversibly into `[a-zA-Z0-9_-]`, with `::` becoming a separator that a name
cannot itself produce (escape `_` before substituting, or `A_B::C` and `A::B_C` collide) and a
hex escape for the characters a name may legally carry but an id may not — non-ASCII names and
quoted names with spaces or punctuation. Readable ids also keep the graph diffable, which a
UUID scheme gives up. The cost, and the reason to decide it deliberately: the OMG API's own
implementations do treat element ids as UUIDs, so a readable id may be rejected by a client
other than Flexo — that is inference from convention, not something read out of Flexo's
sources, and it is worth checking against whichever client is meant to consume the graph
before the encoding is fixed.

Either way the qualified name lives in `sysml:qualifiedName` (already emitted), and the decoder
must stop recovering identity from the IRI (`rdf.QualifiedNameOf`) and read it from that
property instead — that decoupling, not the choice of encoding, is the substance of D3.1. This
is the item that changes every fixture under `internal/core/export/testdata/convert`, so it
goes first.

### D3.2 — `sysml:elementId` is required and is not emitted

Paged listing selects on `?e sysml:elementId ?id` (`listElementsConstructQuery`) and
`QueryApi` rewrites an `@id` constraint to `sysml:elementId`, so without that triple our
elements are invisible to paged listing and unreachable by query, even when a direct
construct on the IRI would find them. Emit it on every element, carrying the D3.1 id.

### D3.3 — ownership: every element reads as a root

The roots endpoint filters on `sysml:owner` and `sysml:owningRelatedElement` being
unbound or `rdf:nil`. We emit only `sysml:owningNamespace`, so *every* element passes
that filter and the model has no tree. Emit the API's ownership properties as element
references, and decide explicitly whether to materialize `OwningMembership` elements:
the API's payloads reach members through memberships, and a consumer walking
`ownedMember`/`ownedRelationship` finds nothing in our compact projection.

### D3.4 — collection-valued properties need the JSON annotation

The reader **skips** a `sysml:` predicate with more than one object and prefers the
annotation literal at `urn:sysmlv2:annotation:json:<key>`, which it parses as JSON.
Anything multi-valued we emit as bare repeated triples is silently dropped on read. So
the encoder must write both forms for collections, and the decoder must accept the
annotation form when reading a foreign graph.

### D3.5 — the harness, once D3.1–D3.4 are in

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
and the 131 triples it flags are inventoried key-by-key with a reason in
`internal/core/export/testdata/ontology-known-violations.txt`, so any *new* disagreement fails the
build. 19 of them are this item's own bug — an object property carrying a name as a literal, over
six distinct metaclass/property pairs: `type` on `AttributeUsage` and `PartUsage`, `sourceFeature`
on `SuccessionAsUsage` and `sysx:InitialNode`, and both bounds of `sysx:MultiplicityDeclaration`.
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

Two things the flag does not buy. Full instance-level conformance still needs the membership and
relationship elements (D3.3) and real triples where we currently write `sysx:sourceText` (D1, D2):
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
the golden graphs from `testdata/convert` dynamically. Against the 24 of them it finds **131 triples
in 45 distinct metaclass/property violations**: 41 triples (10 keys) whose property is declared on a
metaclass that is not the subject's or an ancestor of it, 39 (14) typed by a class the ontology does
not declare, 32 (15) naming a property no metaclass declares, and 19 (6) giving an object property a
literal (D7). No datatype property gets an IRI, and no subject carrying SysML properties lacks an
`rdf:type`. Every one is inventoried with a one-line reason in
`testdata/ontology-known-violations.txt` and the gate fails on anything new, so the encoder was left
alone. The inventory is also the profile's own work list, since it sorts the gap into four causes:
properties the metamodel declares on a relationship or membership element that we collapse into the
element (`value` → `FeatureValue`, the multiplicity bounds → `MultiplicityRange`, `visibility` →
`Membership`, `isNegated` → `Invariant`, a transition's ends → `Connector`) — D3.3; names as
literals — D7 and D2; 12 metaclasses of our own `sysx:` namespace plus two names the 202407
rendering does not have at all (`FlowUsage`, which it calls `FlowConnectionUsage`, and
`TerminateActionUsage`); and 15 properties we write into the SysML namespace that no metaclass
declares, each either a relationship the metamodel reifies as an element (`specializes`, `subsets`,
`redefines`, `references`, `aliasedElement`, `via`) or a notation flag with no metamodel property
(`isAccept`, `isResult`, `isSnapshot`, `isTimeslice`, `isChain`) — arguably those belong in `sysx:`
regardless of this item.

Scope: the profile plumbing is the remainder, and is about one session's work now that the table and
the gate exist; conformance beyond that is gated on D3.3, D2 and D1 rather than on this item.

## D5 — the parser drops the `variant` and `include` keyword prefixes

`variant part a : A;` and `include U;` prefix a kind keyword the AST already records on its
own, and the prefix itself is recorded nowhere: both parse to the same node as the unprefixed
form. A `notation → RDF → notation` round trip therefore returns `part a : A;` and a plain
use-case reference, which is the one place the RDF mapping changes a model without reporting
it (`docs/reference/rdf-mapping.md`, *Limitations*).

The synonym keywords that *are* distinguishable — `datatype`, `feature`, `function`,
`snapshot`, `timeslice`, `message`, `allocate` and the rest — are carried as
`sysx:declaredKeyword` and round-trip byte-identically
(`export_test.go:TestKindKeywordSynonymsSurviveRDF`). Doing the same for these two means the
parser recording the prefix, most likely as a field alongside `ast.Usage.Keyword`, after which
the encoder can carry it and the documented exception goes away. Worth checking at the same
time whether anything downstream *should* distinguish a variant from a plain member, since
variation semantics currently rest on the enclosing `variation` definition alone.

## D6 — a behavioral node has no metaclass, so a model stating steps cannot convert

**Done.** The behavioral nodes now have metaclasses and the properties their notation is
rebuilt from (`internal/core/export/behavior.go`): the initial and final node, `perform`, `send`, `accept`,
`terminate`, `assign`, the fork/join/merge/decision control nodes, `while`/`loop`/`for`,
`if`/`else`, and the state machine's states, substates, regions, `entry`/`do`/`exit`, `defer`,
pseudostates and transitions. Each is covered by a `notation → RDF → notation` round trip that
asserts the body comes back byte-identically (`export/behavior_test.go`), and the mapping is
tabulated in `docs/reference/rdf-mapping.md` § Behavior.

Measured on the built binary the same way, 102 of the 120 models under `examples/` convert,
up from 71. The 18 refusals are: nine successions that do not name both of their ends (a
`then` attached to a member states an order whose source end the notation leaves implicit, and
reconstructing it means inferring which node an edge belongs to from member position — silent
reattachment, so it is reported instead), three prefix-metadata models, three duplicate
declarations (two names genuinely declared twice in one namespace, which the graph would merge),
two operator-expression members, and one anonymous `snapshot`. One further model converts but is
not byte-stable in its notation across a second hop: the graph records the `ref` of
`end [*] ref cause : Situation;` faithfully and writes it back as `end ref attribute cause`,
which the parser reads with no reference flag — a parser gap, noted rather than worked around.

RDF stays **experimental**: D1 (expressions as source text), D2 (end-binding heads), D3 (no
triplestore round trip) and D7 (reference-valued properties and an abstract metaclass) are
untouched by this, and D3 remains the gate on calling the path
stable.

# How to run the next batch

Lessons that survived the last two batches, unchanged because they keep applying:

1. **Partition children by disjoint file sets, not by task independence.** Seven children once
   all edited `training_examples_expected.txt`, so every PR conflicted with every other and the
   corpus figure churned while sessions re-measured against a moving baseline. A PR that moves
   the corpus regenerates and commits that file *in the same PR*, and corpus-moving PRs run one
   at a time.
2. **Give every child an explicit file list and a stop rule** — "if you find a bug outside this
   list, write it up under 'Found, not fixed' and carry on". Cap review iteration at four
   rounds, then report the remainder.
3. **Children escalate spec disagreements; they do not settle them.** Relaxing a checker or
   re-pointing a test on a child's own reading of the spec should be a decision, not a commit.
4. **Devin cannot merge `main` here.** State the required merge order explicitly whenever PRs
   are stacked, and never plan work that assumes self-merging.

## Suggested sequencing

Tracks A, B, C, P and T1 are closed and their entries are removed; what is left is:

1. **R1** (tag), then **R2**/**R3**/**R5** as the account access appears. R1 gates the rest of
   the release section, and R2 is what makes the Python surface reachable by a user.
2. **Track D** is independent of the rest and can run whenever. Take **D3** first, in its own
   order: **D3.1** (identity) reshapes every fixture and everything else builds on it, then
   **D3.2**/**D3.3**/**D3.4**, then the **D3.5** harness, which is the first thing that can
   actually confirm or refute the interop claim. **D7** next, since it is mechanical once
   identity is stable, then **D2** — a succession end that refers to an unnamed member belongs
   with it, both wanting real end triples rather than names or text — and **D1** last, as the
   largest piece of design. **D5** can go whenever; it is independent of the interop work.
   **D8** (the OWL-ontology output profile) is optional and additive: its domain/range gate is
   worth having as soon as the profile's term table exists, but the profile only becomes fully
   conformant behind D3.3, D2 and D1, so it does not belong ahead of them.
