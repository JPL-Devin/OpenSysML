# OpenSysML — Roadmap

Baseline: `main` @ `32f5a03`, verified locally on 2026-08-17 with Go 1.25.13.
Read `AGENTS.md` first; it governs everything below.

0.0.7 is released from `Open-MBEE/OpenSysML`, carrying `sysml`, `sysml-lsp` and `sysml-grpc`
archives. `main` now carries everything cut under 0.1.0 in `CHANGELOG.md`, which is awaiting its
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
`go test ./...`, `go test -race ./...`, and the corpus gate run locally at its ceiling.

| Gate | Count |
|---|---|
| OMG training corpus | **98/100 clean** — 2 files / 4 errors, both pinned OMG source bugs (the ceiling) |
| Stdlib parser conformance | 95/95 clean — 94 vendored OMG files and 1 non-normative OpenSysML extension |
| Execution conformance cases | 297 |
| gRPC conformance fixtures | 14 |
| Golden execution traces | 98 |
| Runtime robustness cases | 165 |
| gRPC robustness cases | 8 |
| Golden AST fixtures | 86 |
| Negative parser subtests | 129 |

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

## R1 — tag 0.1.0 (maintainer, blocking everything else in this section)

Releases live on `Open-MBEE/OpenSysML`; development happens on `JPL-Devin/OpenSysML`, which
has no tags at all. So the tag is preceded by promoting `main` upstream, as 0.0.4 was through
Open-MBEE PR #47:

```bash
# on Open-MBEE/OpenSysML, after main carries the release commit
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
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

The RDF direction ships **experimental** as of 0.1.0, because of D1–D3 below: its
vocabulary may change without a compatibility path, and no triplestore interop has
been demonstrated. Every surface says so (`export.ExperimentalNotice`), and
promoting it to stable is D3, not a documentation change. What that work
deliberately left open:

## D1 — expressions are carried as source text, not as triples

Feature values, multiplicity bounds, filter conditions and succession guards are stored as
their notation. They round-trip exactly, but SPARQL cannot see inside them, so a query like
"every part whose mass exceeds 1000" is not expressible against the graph. Mapping KerML
expression trees to RDF is the fix and is a feature in its own right: it needs a node-identity
scheme for subexpressions, which is the part to design first.

## D2 — end-binding heads depend on `sysx:sourceText`

`connect`, `bind`, `flow`, `succession`, `transition`, `accept` and `satisfy` keep their head
verbatim, so a graph produced by *another* tool converts to notation only as far as the
structural properties reach and then reports the element as unsupported. Emitting real end
triples (`sysml:source`/`sysml:target`/`sysml:connectorEnd`) would remove the dependency; the
parser already has the ends, so this is an encoder/decoder change rather than a parser one.

## D3 — no round trip against a real triplestore

The vocabulary and element IRIs match Flexo MMS's `Namespaces.kt`, and the round-trip tests
run entirely in-process. Nothing has yet loaded a converted graph into Fuseki via
`flexo-mms-sysmlv2` and read it back, which is the only way to confirm the interop claim.
The companion repo's `src/test/resources/docker-compose.yml` brings up Fuseki plus layer1,
so the harness already exists. Until it runs, no interop claim is made anywhere:
the docs say the vocabulary matches Flexo's namespaces, not that a graph loads into
it, and D3 is the gate on calling the RDF path stable.

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

RDF stays **experimental**: D1 (expressions as source text), D2 (end-binding heads) and D3 (no
triplestore round trip) are untouched by this, and D3 remains the gate on calling the path
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
2. **Track D** is independent of the rest and can run whenever. Take **D3** before **D1**/**D2**:
   it is the cheapest, and it is what would show whether the Flexo interop claim actually holds
   before more work is layered on the mapping. What **D4** left behind — a succession end that
   refers to an unnamed member — belongs with **D2**, since both want real end triples rather
   than names or text.
