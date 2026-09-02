# OpenSysML — Roadmap

Baseline: `main` @ `1f136d27`, verified locally on 2026-09-02 with Go 1.25.0.
Read `AGENTS.md` first; it governs everything below.

> **Labels.** This is an engineering record. The RDF items keep the `D` numbers (`D1`, `D2`,
> `D3.4`, `D7`, `D8`) that other records, the known-violations inventory and the ontology
> package's README cross-reference; `L3` names the remaining library item, `N` the native
> compilation track, `R` the release follow-through, and `E` the behavior-execution semantics
> the runtime does not yet have. Each is stated in full where it is introduced, and a reader who
> wants only the gap can ignore the label.

`v0.4.3` is the newest tag on `Open-MBEE/OpenSysML` (`99e02003`, 2026-09-02, the "Identity
Release"); the tag's CI release job publishes `sysml`, `sysml-lsp` and `sysml-grpc` for five
platforms and the Homebrew bundles, and the Python client is on PyPI as `opensysml` 0.4.0. The
tag includes this roadmap's baseline commit, so no track below describes work that is still
unreleased. `CHANGELOG.md` has not caught up with it: the entries under **Unreleased** (the
serialized library snapshot, the calc-evaluator work and the self-model updates describing them)
are ancestors of the tag and shipped in it, while the **0.4.3** heading is dated 2026-09-01 — see
R1. Everything in "Release follow-through" is maintainer- or account-gated; everything after it is
ordinary engineering work.

**What closed since the last baseline** (`1127a93b`, 2026-08-27) is recorded in `CHANGELOG.md`
rather than kept here, but the roadmap items it retired are: element identity in the notation
(`@ElementId`/`@ProjectRef`, the RDF identity round trip, `-sync-diff`, and the OMG submission —
[design record](element-identity-annotations.md)); document generation from a `doc def` in the
model, as Markdown and as PDF, with executing document queries and a linked multi-document set
([manual](../manual/README.md)); the September performance census and every gap it found
([census](performance-census-2026-09.md), [execution](execution-performance-2026-09.md)); the
Node, Java and Rust clients and the public Go API exercised end to end by worked examples; the
solver's witnesses replayed through the evaluator; the pilot pinned to a commit; the Homebrew
tap's remaining question (see R3 below); and, within the RDF track, expression trees and
end-binding structure in the graph (which narrows D1 and D2 to what is stated under them).

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`, `go test ./...`, and the
corpus gates run locally clean at the baseline, with the pilot material re-fetched at its pin
(`./scripts/download-pilot-corpora.sh`, `./scripts/download-pilot-xpect.sh`). A stale local copy
of the pilot corpora fails `cmd/pilot-diff`, `cmd/pilot-xpect` and the `TestPilotCorpora` gate
with a provenance message naming the drift; that is the gate working, not a regression — re-fetch
before re-recording anything.

| Gate | Count at this baseline |
|---|---|
| OMG training corpus | **100/100 clean** — asserted, not ratcheted: no file reports a semantic error |
| OMG pilot corpora (ratchet) | 213 files; 5 report a diagnostic, each adjudicated in [pilot-corpora.md](pilot-corpora.md) and [omg-issues.md](omg-issues.md) |
| Stdlib parser conformance | 97/97 clean — 94 vendored OMG files and 3 non-normative OpenSysML extensions |
| Execution conformance cases | 444 (`TestExecutionConformance`) |
| Golden execution traces | 125 (`TestExecutionTrace`) |
| Runtime robustness cases | 251 first-level subtests of `TestRuntimeRobustness` |
| gRPC conformance fixtures / robustness cases | 15 / 10 |
| Golden AST fixtures | 156 (`TestGolden`) |
| Negative parser subtests | 261 (`TestNegative`) |

The pilot differential, the Xpect oracle, the scope oracle and the rejection oracle are the
external conformance statement, and their figures are generated into `README.md` by `make
docs-counts` from the committed baselines; they are not repeated here.

The figures above are counted by hand, and the hand-counted surfaces have drifted: `README.md`
still says 380 conformance cases, 118 traces, 146 golden ASTs, 256 robustness cases and 8 gRPC
robustness cases, and `docs/project/spec-compliance.md` says 389 and 266. `releasing.md` requires
the four surfaces allowed to repeat these counts to agree and be recounted in one commit. That
recount — or, better, folding these counts into `cmd/doc-counts` so they are generated like the
pilot figures and can no longer drift — is a small open item and is listed under sequencing.

Statement coverage, measured with `go test -cover ./...` at the baseline. It counts only each
package's own tests, which understates a package consumed by others (`internal/core/ast` is
exercised by every parser test; `cmd/sysml-grpc` is gated by a process lifecycle test whose child
process contributes no profile).

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/quickfix` | 100.0% | `internal/core/export` | 83.2% |
| `internal/core/conformance` | 100.0% | `internal/core/queryplan` | 82.1% |
| `internal/core/ast/astcodec` | 99.5% | `internal/core/project` | 81.0% |
| `internal/core/format` | 97.2% | `internal/core/symbols` | 80.0% |
| `internal/core/identity` | 93.2% | `internal/core/queryexec` | 78.6% |
| `internal/core/rdf/ontology` | 92.1% | `internal/core/resolve` | 77.2% |
| `internal/repl` | 88.4% | `internal/core/query` | 72.5% |
| `internal/grpc` | 87.7% | `cmd/sysml-lsp` | 71.7% |
| `internal/core/passes` | 87.5% | `client/opensysml` | 71.2% |
| `internal/core/runtime` | 85.9% | `internal/core/lower` | 69.7% |
| `internal/lsp` | 85.7% | `internal/core/semantics` | 60.9% |
| `internal/core/rdf` | 85.3% | `cmd/sysml-grpc` | 34.7% |
| `internal/core/parser` | 84.6% | `internal/interop/flexo` | 24.1% (the live-stack half is gated) |
| `internal/core/solve` | 84.0% | `cmd/sysml` | 23.7% |
| `internal/core/libs` | 83.6% | `internal/core/ast` | 20.1% |

The corpus gate needs the corpus (`./scripts/download-training-examples.sh`) and never
re-baseline `internal/core/model/testdata/training_examples_expected.txt`: adjudicate each
drifted file and record the verdict in `docs/project/training-examples.md`.

A tag cannot be cut over a corpus regression: `.circleci/config.yml`'s `build-and-test`
downloads the corpora (cached on the download scripts) and runs the suite with
`OPENSYSML_REQUIRE_TRAINING_CORPUS=1` and `OPENSYSML_REQUIRE_PILOT_CORPORA=1`, on `v*` tags as
well as on branches.

---

# Release follow-through

Tagging a core release, publishing the Python client and the Homebrew bump are all proven paths:
`v0.4.3` is tagged and its release job runs the same path `v0.4.2` completed with its full archive
set, `opensysml-v0.4.0` uploaded the client to PyPI,
and the tap `Open-MBEE/homebrew-tap` bumps itself from its own scheduled workflow on each tag,
rendering the formula from this repository's `scripts/render-homebrew-formula.sh` and template.
The procedure and its post-tag verification are in `docs/project/releasing.md`.

## R1 — the changelog says "Unreleased" about work `v0.4.3` shipped

`v0.4.3` was tagged at `99e02003`, which contains everything `CHANGELOG.md` files under
**Unreleased** above the **0.4.3** heading; a reader of the changelog concludes the 20 ms
start-up and the calc-evaluator work are not in the release they are running. Fold those entries
into the 0.4.3 section and date it to the tag (2026-09-02), leaving Unreleased empty. Small, and
the gate recount above belongs in the same pass, since `releasing.md` checks the four count
surfaces before the next tag.

## R2 — the Node, Java and Rust clients are unpublished

Each has its release workflow (`client-node-v*` to npm, `opensysml-java-v*` to Maven Central,
`opensysml-rust-v*` to crates.io) and a worked example the tests run, and none has ever been
tagged. The Java package name already moved to `org.openmbee.opensysml`, the DNS-verified
namespace, so nothing blocks Maven Central but the account. npm and crates.io need a publisher
token in CI; Maven Central needs the Sonatype account and a signing key. These are account gates
like R4, not engineering.

## R3 — Homebrew: install it on a real Mac

Everything about the tap is automated and verified on Linux (install, `brew test`,
`brew audit --strict --online`), and the open PR shipping man pages adds them to the bundles the
formula installs. The one thing never done is running the darwin bottle on macOS: the darwin
archives' checksums match the release manifest and nothing more.

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

`editors/vscode` builds only as a PR CI artifact: no `.vsix` is attached to a release and there
is no marketplace or Open VSX listing, so a user cannot install it without building it. The
client works against the shipped `sysml-lsp` (`--stdio` is accepted and `shutdown`/`exit` are
honoured). What remains is packaging and publishing: `vsce package` in the release workflow, a
`.vsix` on the release, and (for the marketplace) a publisher account and a PAT in CI — the same
class of account gate as R4.

## Upstream follow-through

Filed and waiting on the other side: the identity-annotation enhancement against SysML 2.0
(`INBOX-2510`, maintainer-approved 2026-09-01) and the `ownedDisjoining` EMF defect against the
pilot (`SysML-v2-Pilot-Implementation#790`). Drafted and waiting on a maintainer to authorise
posting: the three dimensional-analysis errata in the pilot's example corpus and the question
about the `queryx/failing` Xpect fixtures. All four are in [omg-issues.md](omg-issues.md), body
and status; none needs code here until an answer arrives.

## L3 — evaluation does not reach standard-library-inherited features

The library work is otherwise closed: every library file is parsed and indexed on every load path,
built once and frozen, and read through a per-model overlay (`libs.Loader`, `libs.SharedBase`,
`symbols.NewOverlay`) rather than copied per model; the serialized library snapshot brings a
process up in under 20 ms; and gRPC reports an element's own and non-library-inherited attributes
and counts the standard-library-inherited ones it withholds rather than omitting them silently.

**Still open**, and unchanged: a feature a user type inherits from the standard library has no
value at run time. With `part def Box :> Item; part b : Box;`, both `b.isSolid` (declared
`isSolid = isEmpty(voids)` in `Systems Library/Items.sysml`) and `b.voids` report `member … not
found in instance`, so `Model.Eval` never gets as far as folding the library's value expression.
The solver has the mirror gap: its translatable subset does not take library-declared conditions,
and `solve`'s differential harness still indexes the standard library as ordinary documents
(`parseLibraries`) to reach them. [wave12c-lossless-library-records.md](wave12c-lossless-library-records.md)
records what was measured.

---

# Track N — native compilation

The interpreter in `internal/core/runtime` is the reference semantics and is fast in absolute terms
(about a microsecond per calc invocation after the September pass), but a compute-bound analysis —
a recursive calc, a long numeric loop — is still three orders of magnitude off native code. The
goal is that a calc, and eventually a whole analysis case, can be compiled ahead of time into a
standalone native program that computes exactly what `sysml -calc` computes, prints it the same
way and fails on the same inputs, or refuses to compile with a typed error naming the construct
outside the subset. Nothing is compiled approximately.

## N1 — scalar calcs to C or Go (in review)

[PR #778](https://github.com/JPL-Devin/OpenSysML/pull/778) is the spike and is open: `sysml
model.sysml -compile Pkg::Fib -o fib` translates a `calc def` (or a calc usage) into an
executable through C (`cc -O3 -flto`, the default) or Go, with `-source` writing the generated
source alone. The pipeline is `parser → resolve → semantics` plus `lower.CalcBody` into
`codegen.Compiler`, a typed IR (`codegen.Program`), and `EmitC`/`EmitGo`; its design and
measurements are in the native-compilation record that PR adds under `docs/project/` (link it
from here once it merges). The compiled
subset is scalar `Integer`/`Natural`/`Positive`/`Real`/`Boolean` parameters, literals, checked
arithmetic and comparison, the logical and conditional operators, body-local attributes and
assignment, `if`/`while`/`loop … until`, and direct and mutual recursion between compilable calcs.
Everything else — strings, sequences, structured values, defaults, library-function invocations,
`for` and the collection operations, quantities and units, redefinition with members — refuses
with `codegen.UnsupportedError`. A differential test runs every compiled program against the
interpreter, and the Go backend is the second implementation the C is checked against.

Measured (Xeon 8559C, GCC 11.4, 2026-09-02): `Fib(25)` 261 ms interpreted, 919 µs as Go, 221 µs
as C; `SumTo(1000000)` 1216 ms / 764 µs / 379 µs; `Collatz(27)` 206 µs / 5.2 µs / 0.98 µs. C beats
Go by 2–5× on every loop or recursion, which is what justified keeping two backends: C is the
default and Go the fallback where no C compiler is installed.

To land it: review, and the gate must stay green with `cc` absent (the C tests skip; the Go ones
do not).

## N2 — what the spike leaves open

In the order they matter:

1. **The step budget.** The interpreter stops a runaway loop at `OPENSYSML_MAX_STEPS`; compiled
   code counts nothing, so a `while` that never terminates runs forever. This is the one bound
   the interpreter has that compiled code lacks. Decide whether a compiled program carries an
   optional iteration counter (cheap in C, and it would let the differential test keep the budget
   on) or whether "a compiled program is a program" is the documented contract.
2. **Collections and structured values.** Sequences, `for`, the collection operations
   (`select`/`collect`), quantities with units, and structured parameters are what stand between
   "a calc compiles" and "an analysis case compiles". The IR needs a sequence type and a record
   type before any of them; design those first.
3. **Library functions.** `ScalarFunctions::sqrt` and the rest of the Kernel Function Library
   the interpreter evaluates natively need C and Go equivalents with the interpreter's exact
   results and failure behaviour (domain errors, Integer range).
4. **Linking into the toolchain.** The output is a standalone executable. A stable C ABI entry
   point would let the REPL and `sysml-grpc` call a compiled calc in place of interpreting it,
   and would drop the per-run `setjmp`/`recover` harness that bounds the trivial cases at 12 ns.
   Not before N2.1 is decided, since a linked-in calc must honour the host's budget.
5. **Packaging.** The C backend needs GNU C (`__int128`, `__builtin_*_overflow`,
   `setjmp`/`longjmp`); document the toolchain requirement in the install guide when the flag
   ships, and decide whether the release bundles should carry a prebuilt runtime shim.

Actions, state machines, constraints, requirements and instance graphs are out of scope for this
track until N2.2 is in: they are token and event semantics, not arithmetic, and the interpreter's
trace is the contract there.

---

# Track D — model persistence and RDF interchange

Saving and SysML ↔ RDF Turtle conversion landed (`internal/core/rdf`,
`internal/core/export`, `%save`, `sysml -convert`, `-sync-diff`); see
[the RDF mapping](../reference/rdf-mapping.md).

The RDF direction ships **experimental**, because of D1, D2, D3.4 and D7 below: its vocabulary
may change without a compatibility path, and the one triplestore interop measured — Flexo — still
drops what those items carry. Every surface says so (`export.ExperimentalNotice`), and promoting
it to stable is closing D3.4 and re-measuring the harness, not a documentation change.

Measured on the built binary at this baseline, **268 of the 345 models under `examples/`
convert** (the training corpus, the three pilot corpora and this repository's own demos),
including their behavior: action and state bodies round-trip `notation → RDF → notation`
byte-identically (`internal/core/export/behavior.go`, `docs/reference/rdf-mapping.md` § Behavior).
A model the mapping cannot write back is refused rather than converted lossily, and the 77
refusals sort into: 35 declarations that name no element of their own (the notation cannot be
rebuilt from a graph keyed by name), 18 prefix metadata, 13 expressions standing as a body
member — a calc's trailing result expression such as `a - b`, which the mapping writes only in a
valued position (**D1**), 7 successions whose end is not a basic name (**D2**), and 4 duplicate
declarations, where two members of one namespace share a name the graph would merge. The
refusal classes are the same as at the last baseline; the counts moved with the corpus.

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
  and it is stricter than `Namespaces.kt` suggested. It keeps `sysml:` and
  `urn:sysmlv2:annotation:json:` predicates and **ignores everything else** (the
  unrecognized-predicate error is commented out), so every `sysx:` triple is dropped the moment
  a graph passes through that service. Whatever a model needs in order to survive has to be
  standard, which is what makes D1 and D2 part of the interop goal rather than refinements
  after it.

What matches today: `sysml:` = `https://www.omg.org/spec/SysML#` and `elmt:` =
`urn:sysmlv2:element:` are identical to `Namespaces.kt`; `rdf:type` plus `sysml:<property>` per
scalar field is the shape the reader expects; our typed literals fall in the datatypes it maps;
every element carries `sysml:elementId` equal to the id its IRI ends in, so listing by id and the
`@id` derivation agree; and ownership is materialized as the abstract syntax states it, so the
roots endpoint sees one root per document. Those were D3.1–D3.3 and D3.5, closed and now measured
rather than claimed — see the next section.

## D3 — make a converted graph readable through Flexo, and prove it

The harness is `internal/interop/flexo`, the `FLEXO_INTEROP` gate `TestFlexoInterop`, documented
in `.agents/skills/flexo-interop`; it brings up the published `openmbee/*` images, `PUT`s our
Turtle to a branch graph, reads every element back through `flexo-mms-sysmlv2` and compares with
what the service's own commit path stores for the same model. It measures the gap instead of
asserting the fix, so every item below shows up as movement in
`internal/interop/flexo/testdata/interop_expected.txt`. Keep it out of `go test ./...`.

What the current recording measures, for the identity-carrying fixture: **49 of 49 elements
listed and 355 of 424 properties delivered** on the graph-load side, against 33 of 33 and 158 of
158 for the same model posted through the service's own commit path; 9 of 49 read as roots, and 9
have no owner in the model; every element is readable directly by id; no subject of the graph is
outside the element namespace. The 69 lost properties are exactly two things:

- **8 property keys in `sysx:`** — `sourceText`, `hasBody`, `memberIndex`, `argumentIndex`,
  `declaredKeyword`, `endForm`, `endIndex`, `relatedFeature` — dropped unread. That is the D1/D2
  residue below, and it is the reason the expression trees and end structure the mapping now
  writes do not survive the hop.
- **0 of 15 multi-valued standard properties delivered** — `ownedMember`, `ownedMembership`,
  `ownedRelationship` (0/3 each), `ownedFeature`, `ownedFeatureMembership` (0/2 each),
  `specializes` (0/1) — which is **D3.4**.

The commit path delivers 6 of 6 of its own multi-valued properties, because it stores each array
whole as a JSON annotation literal alongside the typed triples. Two deployed behaviours differ
from the sources: the element listing ignores `pageSize`/`pageAfter` and returns every subject,
and project delete is a soft annotation that leaves the Layer 1 branch behind.

### D3.4 — collection-valued properties need the JSON annotation

The reader **skips** a `sysml:` predicate with more than one object and prefers the
annotation literal at `urn:sysmlv2:annotation:json:<key>`, which it parses as JSON.
Anything multi-valued we emit as bare repeated triples is silently dropped on read. So
the encoder must write both forms for collections, and the decoder must accept the
annotation form when reading a foreign graph. This is the last of D3 and the smallest item in
the track; when it lands, re-record the harness (`go test ./internal/interop/flexo -run
TestFlexoInterop -update-flexo` against a live stack) and the 15 become 15 of 15.

## D1 — expression trees are standard in shape, non-standard in vocabulary

Every expression-valued position — a feature value, a multiplicity bound, a guard, a filter, a
condition, a send payload — is now a **tree of typed nodes** in the `expr:` namespace
(`rdf-mapping.md` § Expressions): standard metaclasses (`OperatorExpression`,
`FeatureReferenceExpression`, `LiteralRational`, …), `sysml:argument` and `sysml:referent`
linking operands and referents, a deterministic per-position id every node states in
`sysml:elementId`, and a decoder that reads a foreign tree from its structure alone. SPARQL can
see inside a value now; "every part whose mass exceeds 1000" is expressible.

What remains is what the Flexo hop still loses and the metamodel still does not recognise:

- the operator, the operand order and the source text ride in `sysx:` (`sysx:operator`,
  `sysx:argumentIndex`, `sysx:sourceText`), so after the hop a tree keeps its nodes and loses
  their meaning. The metamodel spells the operator `OperatorExpression::operator` and orders
  arguments through `ownedFeatureMembership`s; emit those;
- a node is not a model element — no `qualifiedName`, no ownership, reachable only from the
  position that holds it — where the abstract syntax makes an expression a `Feature` owned through
  a `FeatureMembership`. Writing expressions as owned elements is the same materialization D3.3
  did for ownership, and it is what the ontology gate's `value` → `FeatureValue` findings (D8) are
  waiting on;
- an expression standing as a body member — a calc's trailing result expression — has no mapping
  at all and refuses to convert (13 of the 77 refusals above). It is a `ResultExpressionMembership`
  in the metamodel; carry it as one.

## D2 — end bindings are structure, but in `sysx:`

`connect`, `bind`, `flow`, `succession`, `transition`, `accept` and `satisfy` now state their ends
as structure beside the verbatim head — one expression node per end under `sysx:relatedFeature`
with `sysx:endIndex`/`sysx:endRole`, and `sysx:endForm` naming the notation the ends are written
in — so a graph from another tool converts to notation with no text at all, and a succession
carries both its ends including the unnamed member a `then` sequences (`rdf-mapping.md`
§ End-binding heads). The form is stated only when rebuilding it reproduces the head exactly;
heads that state more than their ends (a multiplicity, a `references` clause, an inline payload
declaration, a body) stay text-only and are reported, not guessed, when the text is absent.

What remains: the vocabulary is ours, so the hop drops it (`endForm`, `endIndex`,
`relatedFeature` are three of the eight lost keys). The metamodel's shape is
`Connector::connectorEnd` — end features owned through `EndFeatureMembership`s — with
`sourceFeature`/`targetFeature` over them, the same `Connector_sourceFeature`/`targetFeature`
domain findings the ontology gate records for transitions (D8). Emitting those is an
encoder/decoder change, not a parser one; the ends are already in hand. The 7 refusals whose end
is not a basic name (`drive vehicle`, `1stGear`) belong with it: a real end triple names the
element by IRI and needs no basic name.

## D7 — reference-valued properties are emitted as strings, and one metaclass is abstract

The reader turns a resource-valued object into `{"@id": …}` and a literal into a string, so a
property the API defines as a reference has to be an element IRI in the graph. `imports.golden.ttl`
shows both halves of this gap: `sysml:importedNamespace "ISQ"` is a string where the API expects
a reference, and the metaclass is `sysml:Import`, which is abstract in KerML — the API's own
elements are `NamespaceImport` or `MembershipImport`.

The reference-vs-literal half is mechanized against the OWL ontology (D8):
`TestGoldenGraphsMatchOntology` (`internal/core/export`) checks every SysML-namespace triple in
the 25 golden graphs against the metamodel's declared domain and range, finds **136 triples in 46
distinct metaclass/property violations**, and every one is inventoried key-by-key with a reason in
`internal/core/export/testdata/ontology-known-violations.txt`, so any *new* disagreement fails the
build. The object-property-carrying-a-literal group is this item's own bug: `type` on
`AttributeUsage`, `ReferenceUsage` and `PartUsage`, `sourceFeature` on `SuccessionAsUsage` and
`sysx:InitialNode`, `referent` on `FeatureReferenceExpression` where the referent resolves outside
the graph, and `targetFeature` on `FeatureChainExpression`. Identity is stable (D3.1), so each is
mechanical: resolve the name and emit the IRI, and fall back to the literal only where the
referent is outside the graph, as feature references already do. The abstract-metaclass half is
not mechanizable from the ontology: `SysML.owl` records no ecore abstractness (see D8), so nothing
in the suite catches `sysml:Import` being abstract, and that audit against the API's own element
list stays manual.

## D8 — an optional second output profile: the Open-MBEE SysML v2 OWL ontology

[`Open-MBEE/sysmlv2-rdf-ontology`](https://github.com/Open-MBEE/sysmlv2-rdf-ontology) renders the
OMG metamodel (version 202407, from `SysML.ecore`) as OML and OWL: `SysML.owl`, 172 classes, 348
object properties, 63 datatype properties, with `rdfs:domain`/`rdfs:range` on each. It uses the
*same* namespace we do and its class IRIs are the plain metaclass names we already emit; the
difference is the properties, each qualified by the metaclass that defines it
(`sysml:Element_declaredName`, `sysml:Element_owner` with range `OwningMembership`), and a
conformant instance graph therefore materializes the abstract syntax's relationship elements
rather than collapsing them.

So this is a **second profile selected by a flag, not a superset**: the property IRIs differ, so
one graph cannot satisfy both conventions, and Flexo's convention stays the default. The encoder
already separates the term layer (`rdf.SysMLTerm`, `internal/core/rdf/vocab.go`) from the
structural decisions (`internal/core/export/convert.go`), so the profile is mostly a term-mapping
layer: property name → defining metaclass.

**Done:** the table and the gate. `internal/core/rdf/ontology` holds the term table generated
from `SysML.owl` by `internal/core/rdf/ontology/gen` from a local checkout (version `202407`,
upstream commit in the generated header): 411 properties spanning only **336 distinct unqualified
names — 59 names are declared by more than one metaclass** (`type`, `value`, `source`, `target`,
…), so the unqualified convention is genuinely lossy in the other direction and a profile encoder
has to pick by the subject's metaclass (`LookupProperty` returns every declaration;
`AmbiguousNames` reports the set). The gate is `TestGoldenGraphsMatchOntology`, whose inventory is
also the profile's work list, sorted into four causes: properties the metamodel declares on a
relationship or membership element that we collapse into the element (`value` → `FeatureValue`,
the multiplicity bounds → `MultiplicityRange`, `isNegated` → `Invariant`, a transition's ends →
`Connector`) — the same collapse D3.3 undid for ownership and D1/D2 will undo for expressions and
ends; names as literals (D7); 12 metaclasses of our own `sysx:` namespace plus two names the
202407 rendering does not have (`FlowUsage`, which it calls `FlowConnectionUsage`, and
`TerminateActionUsage`); and 17 properties we write into the SysML namespace that no metaclass
declares, each either a relationship the metamodel reifies as an element (`specializes`,
`subsets`, `redefines`, `references`, `aliasedElement`, `via`) or a notation flag with no
metamodel property (`isAccept`, `isResult`, `isSnapshot`, `isTimeslice`, `isChain`) — arguably
those belong in `sysx:` regardless of this item.

**In review:** [PR #774](https://github.com/JPL-Devin/OpenSysML/pull/774) ships the ontology
itself as **41 leaf Turtle modules and 6 layer ontologies** under `ontology/sysmlv2/`, cut along
the package hierarchy of the normative KerML/SysML XMI (`KerML/Root/Elements.ttl`,
`KerML/Kernel/Expressions.ttl`, `SysML/Systems/Requirements.ttl`, …) with a `catalog.tsv` from
term to declaring module and `owl:imports` computed from use. The generator
(`cmd/ontology-modules`, `internal/core/rdf/ontology/modules`) errors rather than dropping data —
every source triple lands in exactly one module and the union is graph-isomorphic to upstream —
and `make ontology-modules-check` in CI keeps the committed output equal to the pinned sources
(`scripts/download-ontology-sources.sh`). It is additive to the term table and the Flexo export;
once merged, the profile can import the module a metaclass lives in rather than the monolith, and
a consumer wanting only, say, the requirements vocabulary has a file to import.

**Open:** the profile plumbing itself — about one session now that the table, the gate and the
modules exist. Conformance beyond that is gated on D1 and D2 rather than on this item: `sysx:`
has no place in the ontology, so an ontology-profile graph is conformant only as far as those
have landed, and the profile's documentation should say so.

---

# Track E — behavior execution

The runtime executes actions, state machines, calculations and constraints against the lowered
IR (`internal/core/lower` `ActionGraph`/`StateGraph`, `internal/core/runtime`), and
`docs/project/spec-compliance.md` § "What We Don't (Yet) Support" lists what it does not
execute: interruptible regions, expansion regions, streaming pins, protocol state machines,
operation invocation with positional arguments, and routing a send to a second object of one
usage; a `terminate` inside a body is refused by the runtime with a typed error and appears in no
list. The behavior-execution review after `v0.4.3` found nothing missing beyond those, and this
track records each as work with a stated scope, dependency order and acceptance gate rather than
as a bullet or an error message alone. **None of it is near-term.** No conformance fixture or
trace golden exercises any of them, each has a typed refusal or a documented limitation in place
of a wrong result, and the items above this track (release follow-through, the nested-action
frames the review did find, Track N, Track D) come first. Each item below therefore ends with what
would move it forward; until that happens, the honest status is the "not supported" bullet or the
refusal.

Two things about the list's own terms. First, four of the seven items — interruptible regions,
expansion regions, streaming pins, protocol state machines — are UML 2.5.1 concepts that SysML v2
(`formal/2026-03-02`) does not carry as notation: its actions (§7.17) spell termination,
acceptance, loops and flows directly and its states (§7.18) have no protocol variant. Each of
those items therefore starts by naming the SysML v2 spelling it corresponds to, and the target is
that spelling's semantics in the Kernel Semantic Library and the Systems Library, never the UML
feature by name. Second, the proof for every item is the four-layer contract of `AGENTS.md`
§5.2 — a conformance fixture (`.sysml` + `.expected.json` under
`internal/core/runtime/testdata/conformance/`), a trace golden where ordering matters
(`TestExecutionTrace`, `-update-traces`), a robustness case in `robustness_test.go` for the
failure mode, and the row in `spec-compliance.md` moving out of the "not supported" list — and
every item is self-assessed, since the pinned pilot evaluates expressions and executes no action
or state machine ([pilot-execution-referee.md](pilot-execution-referee.md)).

## E1 — `terminate` in a body

**Today.** The parser accepts a terminate action usage in every position the grammar allows
(`ast.TerminateStatement`, with the terminated occurrence as `Target` or nil for the containing
action), and lowering carries it losslessly: as a node of the flow (`then terminate;`,
`lower/action_nodes.go`), as a statement of a control node's or a state's `entry`/`do`/`exit`
body (`lower/action_graph.go` `lowerStatement` → `Effect{Kind: EffectTerminate}`;
`lower/state_graph.go` through `BodyStatementMembers`). The runtime then refuses to execute it,
with one message wherever it is reached: `runtime/action_statements.go`
`(*actionStmtHost).effect` and `runtime/state_statements.go` `(*stateStmtHost).effect` return
`<where>: 'terminate' in a body is not executable`; a calculation refuses it as
`ErrCalcSideEffect` (`runtime/calc_statements.go`), and that refusal is correct and stays — a
calculation is pure (`robustness_test.go:calc_terminate_is_rejected`). So an action whose flow
reaches `then terminate;` fails with that error rather than ending. The training corpus's
`19. Terminate Actions/Terminate Actions Example-1.sysml` (`MonitoredActivity`) uses it twice
and does not run at this baseline: it stops earlier, at the nested action node whose members are
`perform`s with no `first` (the nested-action frames item the same review found), and would stop
at the first `terminate` once that is in.

One position is not refused. A nested action node written as a statement body
(`action a { assign n := 1; terminate; }`) is lowered by `lower/action_graph.go` `lowerBody`,
whose statement cases are `send`, `assign`, `while`/`loop`/`for` and `if` — `terminate` is not
among them, so it is dropped and the node completes as if it were not written (the sibling that
follows runs; measured at this baseline). That is a silent no-op of the kind `AGENTS.md` §8
forbids and it is a defect, not part of this item: closing it means adding
`*ast.TerminateStatement` to that case so the statement is refused there as everywhere else,
with a robustness case, and it should be fixed whenever it is next touched rather than waiting
for E1.

**Target.** SysML v2 §7.17.10: a terminate action usage "forces the lifetime of the terminated
occurrence to end by the completion of the `TerminateAction`"; when no occurrence is given, the
default is "the immediately containing action of the terminate action usage", and for a nested
action "it is that nested action that is terminated, not any containing actions"; `terminate
this;` in a part's action ends the part; the occurrence may also arrive by a `flow` into the
`terminatedOccurrence` parameter. The library is `Actions::TerminateAction` (`Systems
Library/Actions.sysml`: `in occurrence terminatedOccurrence[1]`, performed by
`terminateOccurrence : destroy`) and the base usage `Actions::terminateActions`, whose
`terminatedOccurrence` defaults to `that as Occurrence`. In a state body the containing action is
the entry, do or exit behavior it is written in — a `StatePerformance`'s `entry`/`do`/`exit` step
(`Kernel Semantic Library/StatePerformances.kerml`) — so it ends that behavior, not the state
machine, unless the machine's exhibiting occurrence is named.

**Work.** Give `lower.Effect` the terminated occurrence as an evaluable target (nil for the
containing performance), and give both executors a notion of *ending a performance that is still
ongoing*: for the action executor, dropping every token the terminated node or action still holds
— including a forked sibling branch still running, as `MonitoredActivity` requires — and
completing it with the outputs it has; for the state executor, ending the running entry/do/exit
behavior at that statement (`runtime/state_executor.go` `startDoAction`/`runDoRound` hold the
running do behaviors) and, for a named occurrence, the object that holds it. A `terminate` naming
an occurrence that is not ongoing, or a value that is no occurrence, is a typed error. Nothing
else in the track depends on anything but this item, and E2 depends on it.

**Proof.** Conformance: the training example above runs to completion, `performCriticalActivity`
ended by its own `terminate` while `monitorCriticalActivity` is still ongoing and `stop` ending
the whole action; `then terminate;` in a flow ends the action with the outputs assigned so far;
`terminate` in a `do` body ends that behavior and the state stays active; a nested action's
`terminate` ends the nested action and its parent continues. A trace golden for
the fork case (which tokens are dropped, in what order). Robustness: terminate of a completed
occurrence, of a non-occurrence, and the calculation refusal unchanged. `spec-compliance.md`: the
Actions map gains a `terminate` row per position (today it has none — the refusal is the only
record), and the pointer to this track under "not supported" drops the `terminate` clause.
**Prioritize when** a corpus model or a user model needs `terminate` to run rather than to be
refused — the training example is the first candidate, once the nested-action frames it also
needs are in.

## E2 — interrupting an ongoing performance ("interruptible regions")

**Today.** SysML v2 has no interruptible-region notation; what UML models with one is spelled in
SysML v2 as an `accept` followed by a `terminate` in a forked branch (§7.17.10's
`MonitoredActivity`: "Terminates `performCriticalActivity` even if `monitorCriticalActivity` is
still ongoing"), or as a transition leaving a state whose `do` is running (§7.18.3: "If the
source state has a do action that is still being performed, that is interrupted."). The action
half is E1 and does not exist yet. The state half runs, with one documented approximation
(`spec-compliance.md` § Known Limitations, *Runtime*): an inline `do` body is one action, so
`runDoRound` advances it as a unit and an outgoing transition interrupts it only between rounds,
never between its statements; the one-action-per-statement `do { … }` form is the interruptible
spelling. There is no refusal here — the approximation is a documented ordering, not an error.

**Target.** §7.18.3's transition semantics, step 1: the source state's do action, "if it is
still being performed, is interrupted" when the transition is triggered — `StatePerformance::do`
is a `step` of the state's performance, and the `StateTransitionPerformance` that leaves it is
keyed on its `accept` step and `transitionLink` (`StatePerformances.kerml`,
`TransitionPerformances.kerml`) — so the do behavior's remaining statements do not run once the
trigger is accepted, whether the body was written as one action or several. For actions, the
target is E1's: the terminate ends the performance whatever else it has in flight.

**Work.** After E1: make an inline `do` body resumable between statements so a round can leave it
mid-body (the statement hosts already run a body statement at a time through `stmtEnv` frames;
what is missing is a do behavior that yields after each statement rather than after the whole
inline body), and drop the pending statements when the state exits. Depends on E1 for the shared
notion of ending an ongoing performance; nothing depends on it.

**Proof.** Conformance and a trace golden for a `do` body of three statements interrupted by a
signal after the first; the existing do-interruption fixtures unchanged; the Known Limitations
bullet removed and the `entry`/`do`/`exit` row in the State Machine map re-stated.
**Prioritize when** a model's result depends on a `do` body being interrupted between two of its
statements — until then the documented spelling (`do { … }` as one action per statement) gives
the same result.

## E3 — concurrent per-element performance ("expansion regions")

**Today.** SysML v2 has no expansion-region notation. Its iterative half is the `for` loop
(§7.17.12, `Actions::ForLoopAction`), which the runtime executes in every body position
(`runtime/action_statements.go`, `runtime/statements.go` `forLoop`), one iteration after the
other, with the step budget bounding it. Its parallel half — the body performed once per element
of a collection, all performances ongoing at once — has no spelling the runtime refuses, because
no spelling for it has been established here: a `for` is sequential by definition
(`ForLoopAction` walks `seq` by an `index`), and a `fork` duplicates control, not a collection.

**Target.** To be settled before any executor work, in a design record under `docs/project/`
like the others: whether SysML v2 §7.17.2's multiplicity on a performed action usage, with a
`flow` delivering the collection to its input, is the standard spelling of concurrent per-element
performance, and what `Performances.kerml` then says about the ordering of those performances.
If the reading is that no such spelling exists, the item closes as "the iterative form is `for`;
the parallel form is not SysML v2" and the bullet is re-worded to say so.

**Work.** The record first; then, if there is a spelling, one performance per element with its
own token and pins, joined when all complete, on top of the concurrency the fork/join executor
already has — and the interaction with E4 (a streaming consumer of the elements) decided with it.
No other item depends on this one.

**Proof.** Conformance for a collection of three performed concurrently with per-element outputs
collected; a trace golden for the interleaving; robustness for an empty collection and a body
that fails on one element. **Prioritize when** someone needs it: a model with a per-element
behavior whose sequential `for` result is wrong or too slow.

## E4 — streaming flows ("streaming pins")

**Today.** A `flow` between two action parameters executes as a *succession* flow: the value
at the source pin is moved to the target pin when the source node completes
(`runtime/action_executor.go` `applyDataFlows`, called from the node-completion paths), so the
target reads it when its own token arrives. SysML v2 §7.16 draws the distinction the runtime does
not: "the input and output parameters are streaming unless designated as succession flows" — a
streaming `flow` "can be ongoing while both the source and target action are being performed",
while a `succession flow` "cannot begin until the source completes". The parser accepts
`succession flow` and drops the distinction (`parser/defusage.go`: the keyword is consumed and the
usage is parsed as a plain `flow`), `ast.FlowEnds`/`lower.ObjectFlow` carry no kind, and no
position refuses anything — both spellings run, both as the succession reading. That is a wrong
result only for a model whose target reads before its source completes, and no conformance
fixture writes one.

**Target.** `Flows::Flow :> Message, FlowTransfer` for a streaming flow and
`Flows::SuccessionFlow :> Flow, FlowTransferBefore` for the succession form (`Systems
Library/Flows.sysml`, `Kernel Semantic Library/Transfers.kerml`): a streaming flow transfers each
value the source parameter takes while both performances are ongoing; a succession flow transfers
after the source completes. What runs today is the second, applied to both.

**Work.** Carry the kind from the parser (`succession flow` as an AST flag) through
`lower.ObjectFlow` to the executor; keep the succession behavior for the `succession flow`
spelling; for a plain `flow`, deliver on each write to the source parameter while the target is
ongoing, which needs a node to be readable while it still holds a token — the same notion of an
ongoing performance E1 and E2 introduce. Depends on E1 for that notion; E3's parallel form would
feed it.

**Proof.** Conformance: a producer loop writing three values to an `out` streamed to a consumer
that accumulates them, with the `succession flow` variant of the same model receiving only the
last; a trace golden for the interleaving; robustness for a stream whose source never writes.
`spec-compliance.md`: the Actions map's object-flow rows split by kind and the bullet leaves the
list. **Prioritize when** a model's result differs between the two readings — a consumer that
reads before its producer completes.

## E5 — protocol state machines

**Today.** No SysML v2 notation exists for a protocol state machine (UML 2.5.1 §14.4), so nothing
is parsed, lowered or refused; the bullet in `spec-compliance.md` is the whole record. What SysML
v2 does have is a state machine exhibited by an occurrence (§7.18.4 `exhibit`), which the runtime
runs during materialization of an object of the exhibiting type (the Classifier Behaviors map).

**Target.** None is stated, and none should be invented here: a UML protocol state machine
constrains the order of operation calls on an interface, and the SysML v2 rendering of that
constraint is a design question — an exhibited state machine on a port definition, with an
out-of-order message refused as a typed error, is the obvious candidate — to be settled in a design
record if the need arises.

**Work.** The record; then whatever it concludes. Independent of every other item.

**Proof.** Set by the record. **Prioritize when** a user brings a model that needs the order of
messages on a port checked at run time; until then the bullet stays as it is.

## E6 — operation invocation with positional arguments

**Today.** `Context.InvokeOperation(inst, name, args map[string]Value)`
(`runtime/invoke_operation.go`) runs a member of an object's type with the object as performer,
whichever behavior the member is — an action through `ExecuteActionPerformedBy`, a calc through
the calc invocation with the object as its featuring object, a constraint through condition
evaluation — and `TestInvokeOperationPerformedByTheObject` (`runtime/classifier_behavior_test.go`)
covers all three. The compliance bullet used to name an operation "given as a `calc` or
`constraint`" beside the positional form; that half is closed by the Classifier Behaviors row that
says so, the bullet is re-worded to the positional form with this record, and this item is that
form only. Arguments bind by name and only by name: `operationInputs` takes a map, binds each
`in`/`inout` parameter by its name and refuses a missing one (`ErrUnboundParameter: parameter …
has no argument and no default`) and an unknown one (`ErrUnboundParameter: … is no input
parameter of operation …`). The REPL's `%invoke <object> <op> [<p>=<expr>]` (`repl/meta.go`
`operationArguments`) refuses an argument not written `<parameter>=<expression>`. There is no
positional form on either surface, and no refusal specific to one — the row says so: "no
invocation surface expresses positional operation arguments". An operation call *written in a
model* is an `InvocationExpression` and is bound by the expression machinery, where positional
arguments already work (`runtime/invoke_calc.go` `bindCalcParameter`; for a performed action,
`runtime/invoke_action.go` `bindArguments` binds `inv.args` in parameter order and refuses a
surplus with `action … takes N input parameter(s), got M argument(s)`).

**Target.** KerML 1.0 §8.2.5.8.3 gives an invocation's `ArgumentList` as either a
`PositionalArgumentList` or a `NamedArgumentList`, never a mix, and §8.4.4.9.5 binds a positional
list to the behavior's parameters in declaration order (`feature a redefines F::a = e1; feature b
redefines F::b = e2; …`) — as `bindArguments` and `bindCalcParameter` already do for a call in the
model. The API and the REPL should offer the same two forms, so `%invoke rover1 drive 10 20` binds
`10` and `20` to the first two `in` parameters, a list mixing the two forms is refused, and a
surplus is refused with the same arity error.

**Work.** Small and self-contained: an ordered argument list on `InvokeOperation` (or a second
entry point) sharing `bindArguments`'s rules, and `%invoke` accepting a list of bare expressions
in place of its `<p>=<expr>` pairs. Depends on nothing; nothing depends on it.

**Proof.** `runtime/classifier_behavior_test.go` and `repl/classifier_behavior_test.go` gain the
positional, mixed and surplus cases; `robustness_test.go` the arity failure; the bullet leaves the
list. **Prioritize when** a REPL or API user asks for it — it is the smallest item in the track
and the one most likely to be done on demand.

## E7 — an addressed send to a second object of one usage

**Today.** A `send … to <target>` resolves its target through `runtime/signal.go`
`resolveAddresses` → `featureAddresses` → `addressOwner`: a name that is a feature of the sending
object (or of an object holding it) reaches that feature's value, and otherwise the shortest prefix
naming an occurrence usage that `occursOnce` reaches *the* occurrence this context holds for it
(`ctx.occurrenceOf(sym)`, `runtime/instance.go`). A `via` send follows the connections
(`runtime/routing.go`, `signal.go` `postVia`) to the object at the other end. In both forms the
object reached is the one this context materialized as the usage's occurrence; a second object
instantiated of that same usage is a different object, which a send addressed to the usage does
not reach. There is no refusal: the send reaches the held occurrence, and
`spec-compliance.md` § Known Limitations (*Standard behavioral notation*) documents it.

**Target.** §7.17.7: a `SendAction` has three input parameters — the payload, a *sender*
occurrence (`via`) and a *receiver* occurrence (`to`) — and "the behavior of a `SendAction` is to
transfer the payload from the sender to the receiver"; the receiver is a value the `to` expression
(or a flow or binding into the `receiver` parameter) supplies, and the message is a
`MessageTransfer` between those two occurrences (`Transfers.kerml`). So a send whose receiver
expression yields a particular object reaches that object, whichever usage it was instantiated
from.

**Work.** Address by value rather than by usage: evaluate the `to` expression to an object
reference and post to that object's identity (`objectID`), with the by-usage resolution kept for a
target that is a name. That is only meaningful once a context can hold more than one object of one
usage, which is the "dynamic object creation/destruction" bullet beside this one in the compliance
list and outside this track: an object materialized by `new` or held in a feature the sender
reads. Depends on that object-model item; independent of E1–E6.

**Proof.** Conformance: two objects of one usage, a send addressed to the second, only the second's
accept fires (with the `via` form of the same model); a trace golden; robustness for a target
expression yielding no object. `spec-compliance.md`: the Known Limitations bullet and the "not
supported" bullet both leave. **Prioritize when** the object-model item lands, since without it
there is no second object to address.

---

# Proposed, not started

**An HTML document backend.** [html-document-backend.md](html-document-backend.md) designs a
direct `docrender.HTML` backend from the document IR (`-doc-form html`, a default stylesheet
in a cascade layer that reader CSS overrides without specificity fights, `-html-css` and a
fragment option) and the migration of the PDF path onto it, replacing the Markdown → converter
hop. Status there is *proposed — nothing on that page is implemented*. It is independent of every
track above and waits only on someone wanting HTML.

**The pilot as an execution referee.** [pilot-execution-referee.md](pilot-execution-referee.md)
established that the pinned pilot evaluates model-level expressions and nothing else, so
`cmd/pilot-exec-diff` can adjudicate the expression rows of `spec-compliance.md` and no external
implementation adjudicates actions or state machines. Widening that referee means finding one,
not more harness work.

# Suggested sequencing

1. **R1** first — fold the shipped Unreleased entries into the 0.4.3 notes — with the gate-count
   recount (or its move into `cmd/doc-counts`) in the same pass, because `releasing.md` checks it
   before the next tag. **R2**–**R5** as the accounts and hardware appear: publisher tokens
   for npm, Maven Central and crates.io, a real Mac for the tap, an Apple Developer and an OV/EV
   certificate to sign with, and a marketplace publisher for the extension. None of them gates
   the others or anything below.
2. **Track N**: land N1 (PR #778), then decide N2.1 (the budget) before anything else in the
   track, since collections (N2.2) and linking (N2.4) both depend on the answer. N2.3 (library
   functions) can go in parallel with N2.2.
3. **Track D** is independent of the rest. Take **D3.4** first — it is small, it is the last of D3,
   and it is the item that moves the harness from 355/424 toward the commit path's 158/158. Then
   **D7**, mechanical now that identity is stable. Then **D2** and **D1** together, since both are
   the same move — from `sysx:` structure to the metamodel's own elements and properties — and
   the ontology gate's inventory is their checklist; D2 is the smaller and goes first. **D8**'s
   profile after PR #774 merges, since it only becomes conformant behind D1 and D2.
4. **L3** is independent of everything above and is a runtime item: materialize
   library-inherited features on an instance, then fold their value expressions, then hand
   library-declared conditions to the solver.
5. **Track E** is not scheduled; each item states what would prioritize it. If one is picked up
   without such a trigger, the order is **E1** (termination of an ongoing performance, which
   **E2** and **E4** build on), then **E2**, then **E4**; **E6** whenever asked, being a day's
   work; **E3** and **E5** only after their design records; **E7** after the object-model item
   it depends on. The `lowerBody` silent drop noted under E1 is a defect and does not wait for
   the track.
