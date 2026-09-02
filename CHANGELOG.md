# Changelog

Notable changes per release. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Cutting a release
is described in [docs/project/releasing.md](docs/project/releasing.md).

## Unreleased

### Added

- **The bundled standard library opens in the editor.** Go-to-definition, find-references
  and the diagram panel used to report a standard library declaration at a path no editor
  could open, so a click on `ScalarValues::Integer` went nowhere. `sysml-lsp` now reports
  such a location under the `sysml-stdlib:` scheme — the file's path within the library —
  with its line and column computed from the bundled text, and serves that text through the
  `opensysml/stdlibContent` request, announced as `openSysmlStdlibContent` in the
  `initialize` result. The VS Code extension registers a provider for the scheme, so
  <kbd>Ctrl</kbd>+click on a library name opens the bundled file in a read-only editor on the
  declaring line; hover, go-to-definition, the outline and semantic highlighting work inside
  it, so navigation continues from one library file into the next. Opening or closing such a
  document changes nothing, and an edit to one is refused with an error rather than applied:
  the library is what every diagnostic is judged against. Other LSP clients get the same by
  registering a content provider for the scheme that calls the request.

- **A model's change set applies to a live repository, keyed by identity.**
  `sysml model.sysml -sync-apply http://localhost:8083` diffs the model against its project
  branch on a running SysML v2 API (Flexo MMS) and writes the change set as one commit through
  the service's own commit path: a rename, move or retype under a retained id is an update of
  that element — never a delete plus a create — a new id is a create, and a delete goes only
  when the run confirmed deletes with `-sync-confirm-deletes`. A change set holding a conflict
  or an unconfirmed delete is refused, as a typed error, before any write; nothing is resolved
  silently. On success the commit the service names becomes the last-seen commit in
  `<model>.sync.json`, never in the notation, and it is the baseline of the next run, so
  repository changes made behind the sync's back surface as conflicts and a second apply finds
  nothing to change. An apply that finds nothing to change still records the branch head it
  compared against, so a model first pushed by other means gets its baseline from the first
  run. The change set is computed at one head commit, and the commit is refused if the branch
  has moved since — someone else's edit between the read and the write is a stale-head error
  to diff again after, not a silent overwrite. `-sync-diff` takes the same endpoint URL and
  stays a dry run; with neither flag nothing is written. A bearer token never goes over
  plaintext `http://` to a host other than this machine: the compose stack on `localhost` works
  as documented, anything else needs `https://` or an explicit `FLEXO_ALLOW_PLAIN_HTTP=1`. An
  apply that mints ids writes the `-sync-annotate` model only after the commit holding them
  lands, and is refused when a minted element has no name to annotate — an id the notation
  cannot keep would be minted again on the next run. The exit status keeps its contract: 0
  applied or nothing to do, 1 a refusal or a repository failure — a read the stack would not
  answer included, reported with each change's fate — 2 an unusable run. Both sides of the
  diff are compared under what the service can store, so the properties it has no place for
  are reported as not compared rather than diffed forever. The opt-in Flexo harness measures
  the apply against the real stack — an initial load, a revision with a retained-id rename and
  gated deletes, a conflict staged behind the sync's back — and records what read back at the
  recorded commit ([the report](internal/interop/flexo/testdata/identity_apply_expected.txt)).
- **Action and state execution has a referee outside the executor.** Six conformance cases —
  a join fed by branches of unequal length, a join fed twice over one succession, a node two
  successions reach, two fork branches writing one feature, the specification's `ChargeBattery`
  merge loop, and a transition's guard, exit, effect and entry made observable through values —
  carry expected outcomes and traces derived by hand from the Kernel Semantic Library
  (`Occurrences.kerml` `HappensBefore`, `Performances.kerml`, `ControlPerformances.kerml`,
  `StatePerformances.kerml`, `TransitionPerformances.kerml`) and the Systems Library
  (`Actions.sysml`), not recorded from the executor. The derivation, sentence by sentence, and
  the orderings the library leaves open are in
  [the semantic oracle record](docs/project/behavior-semantic-oracle.md). Three cases state what
  the executor does not do yet and are listed as known failures rather than recorded as goldens:
  a join fires on the count of parked tokens instead of one per incoming succession and then
  deadlocks, a node reached over two successions is performed twice, and a merge admits one
  traversal per run so a loop is left after its first pass. The executor is unchanged; the
  compliance rows for the join, the merge and concurrent same-feature writes cite the cases and
  stay approximate.
- **The RDF round trip is measured over every example, and pinned per file.** `TestCorpusRoundTrip`
  converts each of the 345 models under `examples/` — the committed models, the OMG training
  corpus and the three pilot corpora — notation → Turtle → notation → Turtle and compares the two
  graphs as triple sets, so a writer or encoder change that moves any file's verdict in either
  direction fails the suite and is adjudicated, as the pilot corpora gate already does for
  diagnostics. The baseline records 166 files stable, 71 stable up to the whitespace inside
  `sysx:sourceText`, 14 that come back as a different graph, 15 that cannot be written back,
  2 whose written notation no longer converts and 77 refused on the first hop, each refusal
  classed by the construct it names. The mapping's reference now states that measurement in place
  of the claim that a second conversion yields the same graph, which held for the fixtures alone
  ([docs/project/rdf-corpus-roundtrip.md](docs/project/rdf-corpus-roundtrip.md)).

### Performance

- **A process starts in under 20 ms instead of 100.** Every `sysml`, `sysml-lsp` and `sysml-grpc`
  start, and every test that builds a model, first parsed the 97 bundled OMG library files, indexed
  them and expanded their wildcard imports — about 100 ms and 467k allocations before the model was
  looked at. The library's frozen index is now serialized once, at `go generate` time, into
  `internal/core/libs/stdlib.snapshot` — a hand-rolled binary format (varints over a string table,
  a node table per syntax-node type, index references in place of pointers; no `encoding/gob`, no
  reflection) that is embedded in the binary and decoded at start-up, reproducing the object graph a
  fresh load builds. `bin/sysml -memstats -e "2+3"` over a one-part model goes from 95–102 ms,
  53.3 MiB and 466.9k allocations to 17–23 ms, 32.4 MiB and 67.1k; the `sysml` binary grows from
  16.9 to 20.5 MB. The OMG files stay the source of truth: the snapshot records their digest and a
  format version, and a process whose bundled files, `OPENSYSML_LIBRARY_PATH` override or snapshot
  format do not match parses the files as before. `make stdlib-snapshot` regenerates it; a test and
  a CI check fail when the committed snapshot lags the files or the indexing code. The parse path
  itself is also faster — the files are hashed and parsed concurrently and added to the index in
  the same order as before, and wildcard expansion no longer re-sorts namespace children out of a
  map on every enumeration (33 ms → 31 ms over the library).

- **The calc evaluator does less work per invocation.** `runtime.Value` is 64 bytes instead of
  120, so a value returned through the evaluator's nested frames copies half as much; parsed
  literals and resolved invocation targets are memoized per evaluation context, keyed by the
  syntax node an edit replaces; a calc's parameters bind into slot-indexed frames resolved once
  per calc, with a bare name answered from the frames before the general resolution chain; and an
  invocation's arguments and frame stack are borrowed from per-context storage. A recursive
  `Fib(25)` costs 0.65 µs per calc invocation instead of 1.01 µs and allocates about 160 objects
  per evaluation instead of 971 000. Results, errors, traces and step counts are unchanged; the
  measurements are recorded in the
  [execution-performance record](docs/project/execution-performance-2026-09.md).

- **Pure calc bodies compile to a closure fast path.** A calc whose body is one scalar expression
  — Integer, Real and Boolean literals, its own `in` parameters, the arithmetic, comparison,
  equality, identity, logical and conditional operators, and invocations of other such calcs,
  recursion and cycles included — is compiled on its first invocation into a tree of Go closures
  over an unboxed scalar frame: parameters are slot indexes, callees are resolved once, and values
  are boxed only at the invocation boundary. A recursive `Fib(25)` costs 21–22 ns per calc
  invocation instead of 519–532 (CPython 3.12 takes 27 ns for the same function on the same
  machine). Values, errors, error timing and step counts are identical to the reference evaluator's
  — a differential test invokes every eligible calc in the fixture and example trees through both
  tiers on generated edge arguments — and anything outside the subset (calc usages, `out` features,
  feature chains, collections, quantities, strings, locals, non-literal defaults, redeclared
  parameters) stays on the evaluator, as does every traced, named-argument or non-scalar
  invocation. `OPENSYSML_CALC_COMPILE=0` turns the tier off for bisecting.

- **The compiled calc tier takes the bodies analysis models write.** Four constructs join the
  compiled subset, each reproducing the reference evaluator's values, error text and step counts
  exactly: statement bodies of body-local scalar declarations, `return` and `if`/`else`, compiled
  from the lowered statements into further slots of the scalar frame with the evaluator's
  declaration-order and shadowing rules (a local read before its declaration, a local without a
  value, a body that may run off its end, `out` features, loops and assignments stay on the
  evaluator); parameters redefined along the specialization chain (`in :>> x = 3.0;`,
  `in x :>> Base::x : Integer;`), laid out as the effective parameter list the evaluator binds; the
  standard library's scalar functions and constants (`sqrt`, `ln`, `exp`, `abs`, `floor`, `round`,
  `min`, `max`, the trigonometric functions, `deg`, `rad`, `TrigFunctions::pi`, …), dispatched
  through the resolved symbol to the Go implementation the evaluator uses — so an alias or an
  import reaches it and a model's own `sqrt` does not — and `sum`/`product` over a lone scalar;
  and named arguments (`Fib(k = n - 1)`), bound to slots at compile time with the evaluator's
  arity and unknown-name checks ahead of dispatch. Over the repository's fixtures, examples and
  the OMG corpora the tier now compiles 127 of 343 calc definitions (37%) instead of 42 of 263
  (16%); a recursive tree of three-local bodies calling `sqrt` runs 12.6× faster (10.4 ms instead
  of 132 for 131 071 invocations) and `Fib(25)` is unchanged at 21–23 ns per invocation. The
  differential test now also compares Reals bit for bit over ±0.0, ±Inf and NaN, and focused
  fixtures under `internal/core/runtime/testdata/compiled/` run each construct through both tiers.

### Changed

- **The documentation site's landing page describes the four oracles instead of quoting their
  totals.** The band below the hero used to state the differential's agreeing-file count, the Xpect
  suites' declared-diagnostic and scope tallies, the rejection corpus's size and the pilot pin, all
  regenerated by `make docs-counts`. Those figures are a census of the corpora we happen to run,
  not a measure a first-time reader can weigh, so the band now says what each comparison measures
  and links to the record that reports it; the numbers stay in `README.md`,
  `docs/internals/architecture.md` and the conformance records, where `doc-counts` still generates
  and gates them. `overrides/home.html` is no longer a `doc-counts` consumer.
- **The architecture self-model describes the library snapshot.** The standard library stage in
  [`examples/self-model`](examples/self-model/README.md) now carries the embedded snapshot its
  index is decoded from, the `internal/core/pack` and `internal/core/ast/astcodec` units that
  encode it and the generator that writes it; `LoadLibrary` models the load as an action whose two
  decisions — the digest and format match, then the checksum — choose between decoding the snapshot
  and parsing the files; `stdlib-snapshot-check` is an eighth, gating conformance oracle; and a
  ninth invariant, `snapshotIsDerived`, states that the snapshot is checked against the files and
  never the only way to load them. The evaluator now declares itself memoized, since it keeps its
  per-node caches in side tables beside the tree. The self-model test compares each new claim with
  the implementation: the override variable, whether the embedded snapshot decodes for the bundled
  files, the Make targets and the CI step, and the side tables `runtime.Context` keys by syntax
  node. The architecture document gains a section and diagram on loading the library, and the
  pilot differential baseline is re-recorded for the larger model: the eleven new rows are all the
  reference's, of shapes the self-model already drew.
- **The architecture self-model describes the compiled calc tier.** The evaluator now carries a
  `CalcCompiler` part — memoized, switched off by `OPENSYSML_CALC_COMPILE`, falling back to the
  evaluator — and `InvokeCalc` models one invocation as an action whose three decisions (a traced
  run, a pure body, positional scalar arguments) send it to the compiled tier or to the evaluator
  whole. A tenth invariant, `evaluatorIsReference`, states that the compiled tier is an optimization
  of the evaluator and never a second semantics; `CalcDifferential` names the parity and
  differential tests that verify it. The self-model test checks the variable's name against
  `runtime.CalcCompileEnvVar` and the environment reference, that a fresh `runtime.Context`
  compiles calcs until that variable says otherwise, and — invoking the model's own `StepBudget`
  through both tiers — that they agree and that a traced run takes the evaluator. The architecture
  document gains a paragraph and diagram on invoking a calc.

## 0.4.3 — 2026-09-01

Release 0.4.3 is where an element gets an identity the notation can carry. The SysML v2 textual
notation deliberately records no element identity, so a model saved as `.sysml` and re-parsed had
fresh ids everywhere and a rename was a delete plus a create. An element may now declare the
repository element it *is* — standard user-defined metadata (`@ElementId`, with a `ProjectRef`
binding a document to its repository once, at the root namespace), shipped as an `IdentityMetadata`
library extension that any conforming tool already parses and preserves. Identity is validated
(id shape, scope binding, uniqueness), survives `notation → RDF → notation`, and
`sysml -sync-diff` computes the change set between a model and a repository graph keyed by that
identity, so a rename or a retype is an update to the same element. The design is
[a project record](docs/project/element-identity-annotations.md), and the notation has been
submitted to OMG for standardization
([the issue text and its status](docs/project/omg-issues.md)).

A solver verdict is now the evaluator's verdict. The SMT translation reasons over exact rationals
while the evaluator computes in IEEE 754 binary64, and the difference is reachable: the exact
encoding holds `0.1 + 0.2 == 0.3` sat, which the evaluator rejects. Every `sat` witness is now
replayed through the evaluator's own arithmetic before it is reported, a query whose conditions the
evaluator rounds is marked and its exact-real `unsat` reported undecided rather than as an
evaluator verdict, and a whole-number quotient divides as an exact ratio rounded once — `5 / 2` is
`2.5`, as the reference evaluates it. The remaining alternative, an exact-rational evaluator value
representation, was adjudicated against the pinned pilot and the specification text and
[declined](docs/project/exact-rational-evaluation.md).

The four non-Go clients and the public Go API were each exercised by worked examples over a fully
capable model — quantities, enumerations, multiplicity, nesting, unvalued features — run by the
test suites so they cannot drift, and the defects that tour surfaced are the client and runtime
fixes below. Each client now has a reference page of its own, the Java client's package moves to
`org.openmbee.opensysml` (the client is unpublished, so no released consumer moves with it), and
the Python client 0.4.0 was published to PyPI. The conformance suite and the pilot differential
now also render their runs as JUnit XML — and the differential as SARIF — so CI shows them as test
results rather than artifacts to download.

A profiling pass across the toolchain removed the costs a September census found rather than the
costs assumed: a multi-file `-validate` batch is indexed once instead of once per file — 6.8 s to
0.30 s over the 100-file training corpus, the quadratic term gone — a calc invocation reuses a
pooled frame instead of allocating ~1.7 KiB per call, a run target resolves from a per-document
name table instead of an O(model) scope walk, the parser's token buffer became a bounded window
(a load allocates 30% fewer objects and holds 16% less live heap), and the `about`-metadata index
is cached when the library index freezes, restoring the empty-session floor the census flagged.
The census and an execution-performance measurement are recorded as project records
([performance census](docs/project/performance-census-2026-09.md),
[execution performance](docs/project/execution-performance-2026-09.md)).

No model that validated under 0.4.2 stops validating and no import path moves.

### Added

- **A complex number crosses the wire as one value.** `Value` gains a `complex` arm carrying the
  real and imaginary parts as two doubles, so a `Complex` feature value, evaluation result, action
  output or calc result arrives as one number rather than the `unsupported` null it was reported
  as, and a complex action input or calc argument is accepted. Every shipped client maps it to one
  native value — Go `opensysml.Complex`, Python `complex`, TypeScript `ComplexValue`, Java
  `Value.ComplexValue`, Rust `Value::Complex` — and prints it in rectangular form. The service
  advertises the `complex_values` capability; without it, a complex in a response is the
  unsupported null as before, and a complex sent in a request is refused with `UNIMPLEMENTED`.
  The Go and Python clients check the capability before sending one, so an older service is a
  capability error rather than an input silently read as null. The Python generator emits
  `complex` for a `Complex` feature (emission schema `4`).

- **An element declares its repository identity in the notation.** `@ElementId { id = "…"; }`
  annotates the element it is written about, and `@ProjectRef { projectId = "…"; }` on a root
  namespace binds the document to its repository, so element-level ids inherit their scope.
  Identity is opt-in per element: an element without an annotation keeps today's derived,
  latest-wins identity. The two metadata definitions ship as a non-normative library extension
  (`IdentityMetadata`, entering the same gates as the vendored files — the bundled-library check
  now reports 97/97 clean), and a constraint-tier pass validates id shape, scope binding and
  uniqueness across the workspace, including anonymous about-form usages, annotations declared in
  libraries, and targets outside the built roots.

- **Identity survives the RDF round trip.** The writer mints subject IRIs from the effective id —
  the declared one where an annotation exists, the encoded qualified name where none does — marks
  declared ids, writes `ProjectRef` bindings as provenance triples, and refuses colliding ids
  across a mixed-scope workspace rather than silently merging two elements. The reader keys
  subjects on the element id (with the name-encoding fallback for old graphs), reports a dangling
  id as its own error, and re-materializes the annotations on the way back to notation, so
  `notation → RDF → notation` preserves which repository element each declaration is.

- **A model diffs against a repository, keyed by identity.** `sysml model.sysml -sync-diff repo.ttl`
  reports the change set — creates, updates, deletes, and renames seen as updates to the same
  element — and never writes: applying is a separate step. `-sync-base` names the graph at the
  last-seen commit, so repository changes since then surface as conflicts rather than silent
  overwrites; deletes are reported always and confirmed with `-sync-confirm-deletes`;
  `-sync-mint-ids` mints a UUID for each unannotated element being created and `-sync-annotate`
  writes the model back out with each minted id declared, preserving the source text and quoting
  names as the notation requires. The last-seen commit is tool state beside the model
  (`<model>.sync.json`), never written into the notation.

- **The editor mints an element id on request.** `sysml-lsp` offers the `refactor.rewrite` code
  action "Annotate … with a minted element id" on the header of a declaration that carries no
  `ElementId`. Invoking it mints a UUID v4 and writes the annotation as a text edit that leaves
  every other byte of the file alone: inline at the head of a body, standalone about-form at the
  end of the file for a bodiless declaration, names quoted as the notation requires. Where the
  root carries no `ProjectRef`, the same edit binds it with a placeholder `projectId` to fill in,
  and a "Bind … to a project" action does that alone. Nothing mints during analysis, and no
  diagnostic asks for an annotation. A metadata usage in a constraint body (`@Tag { … }`) now
  parses as the member it is, distinct from a classification condition (`@Tag`).

- **The conformance suite and the pilot differential render as CI test results.** The conformance
  runner emits JUnit XML (stored even when the gate fails), and `pilot-diff` writes JUnit XML with
  one suite per corpus root alongside SARIF 2.1.0 with one result per disagreeing diagnostic
  group, located on the compared model file — the same run, in the renderings CI dashboards and
  code-scanning consoles read.

- **Worked examples for every client, run by the tests.** Runnable examples drive the Node, Java,
  Rust and Go clients over one capable model — parsing, diagnostics, symbol navigation, evaluation
  and instantiation — and each client gains a reference page
  ([Java](docs/reference/java-api.md), [Node](docs/reference/node-api.md),
  [Rust](docs/reference/rust-api.md)). The examples were written to find defects and did; the
  fixes are below.

- **The implementation models itself.** [`examples/self-model`](examples/self-model/README.md) is
  the analysis pipeline, surfaces, invariants and views of this implementation written as a SysML
  v2 model across five files whose packages import each other, with a make target rendering its
  diagrams and documents and a test evaluating its invariants and checking its figures against the
  implementation they describe.

- **`-render-document` takes a model of several files.** A document may query elements its sibling
  files declare: `sysml model/*.sysml -render-document Reports::MassReport -o report.md` loads the
  named files as one model.

- **Two design records.** [Exact-rational evaluation](docs/project/exact-rational-evaluation.md)
  adjudicates and declines a `big.Rat`-backed evaluator, with the pinned pilot's verbatim binary64
  answers as evidence and a census showing no marked-rounded query is recoverable by per-term
  narrowing; [the HTML document backend](docs/project/html-document-backend.md) records the agreed
  design for rendering documents as semantic, styleable HTML straight from the document IR —
  proposed, not implemented.

### Performance

- **A multi-file `-validate` batch is indexed once.** Validation loaded each file through a path
  that reopened the session document, reindexed it and re-expanded every wildcard import, so a
  batch of N files paid N full indexes over a growing buffer. The batch is now a single
  submission (`Session.LoadFilesSummary`), with each file's own syntax errors, load notices and
  summary still printed in file order. Over the training corpus: 0.26 s → 0.12 s at 25 files,
  0.59 s → 0.13 s at 50, 6.8 s → 0.30 s at 100 — a fixed floor plus a term that scales with the
  input, no quadratic term. The shared symbol walk also stops rebuilding a visited map that its
  cached, deduplicated symbol list already guarantees.

- **A calc invocation reuses a pooled frame.** Each invocation allocated a fresh parameter map,
  evaluation context, statement host and engine — ~1.7 KiB per call, and GC took half the CPU of
  recursion-heavy evaluations. Returned frames go on a free list and the next invocation runs in
  one; a frame is only ever held by one active invocation, so recursion never aliases. Default
  bindings now run in the invocation's own activation, so a value read while binding is memoized
  per invocation rather than leaking to the next one.

- **A run target resolves from a per-document name table.** `RunCalc`, `RunStateMachine` and
  `InstantiateNamed` re-walked the whole document scope tree per run, so a run cost O(model). The
  session now tabulates its documents' simple names once per scope tree and answers lookups from
  the table, rebuilt when a submission or reset replaces a document. At 4,000 elements a
  state-machine start goes 204 µs → 6.4 µs, a calc 222 µs → 4.5 µs, an instantiation
  217 µs → 3.3 µs, and the figures no longer scale with model size.

- **The parser's token buffer is a bounded window.** The parser buffered every non-trivia token of
  a file up front — ~48 bytes per token before reading any of them; consumed tokens are now
  dropped once no checkpoint can rewind to them, so backtracking still sees what it needs while
  the buffer stays bounded by the lookahead. With the REPL parsing each submitted file once
  rather than twice, a load allocates 30% fewer objects and holds 16% less live heap.

- **The `about`-metadata index is cached at index freeze time.** Building it walked the scope tree
  of every document — the bundled standard library included — once per session. A frozen index is
  immutable, so its `about`-usage symbols are recorded once at freeze and only workspace documents
  are walked per session; the empty-session floor returns to ~0.19 ms and 117 KiB allocated,
  −80% wall and −83% allocation, with the annotations found and their order identical.

- **A repeated REPL command does not re-parse its text.** The session keeps what an argument list
  and a command's name text parsed to, keyed by the exact text, so a repeated
  `%calc`/`%action`/`%state`/`%instantiate` does not rebuild a source, lexer and parser per call.
  Evaluation still runs on every call against the current session state.

- **Two measurement records.** The [September 2026 performance census](docs/project/performance-census-2026-09.md)
  measures week-over-week benchmark movement and whole-binary scaling, and flagged the regressions
  fixed above; the [execution-performance record](docs/project/execution-performance-2026-09.md)
  measures evaluation throughput — ~1.5 µs per calc invocation — and names the optimization gaps
  the profiles show.

### Changed

- **A `sat` is a witness the evaluator confirms; an `unsat` is claimed only where the arithmetics
  coincide.** Every satisfying assignment is replayed through the evaluator's float64 arithmetic
  and reported `unknown` with the reason when the replay rejects it; a query whose conditions the
  evaluator rounds is marked, `%check` and `%solve` report its exact-real `unsat` as undecided,
  and `%configure all` and `%optimize` decline the completeness claim outright. Narrower, and
  sound — what is given up is completeness on rounded queries, and the census over the
  repository's solver-facing corpora found none of them recoverable.

- **A whole-number quotient is a Rational.** `5 / 2` answers `2.5` for `Natural` and `Integer`
  operands alike, which is what the reference evaluator answers; the quotient is computed as an
  exact ratio rounded once to float64, so it agrees with the exact SMT encoding even beyond 2^53,
  where rounding each operand first moves the answer. The library's declared `Natural` return is
  recorded as a question for OMG in [omg-issues.md](docs/project/omg-issues.md).

- **The Java client's package is `org.openmbee.opensysml`**, the DNS-verified namespace every
  future Java artifact belongs under, rather than `io.opensysml`. The client has never been
  published, so no released consumer moves with it.

- **A pilot corpus records the pin it was fetched at.** Each corpus directory carries a stamp
  naming the repository and tag it came from: a stale stamp triggers a re-download when the script
  next runs (keeping the old copy until its replacement has been fetched), a current one is left
  alone, and a directory without a stamp is left alone with a warning.

### Fixed

- **An OSLC query's unknown-property diagnostic names properties the query can be written with.**
  `sysml:id="x"` answered with the Go API's own property names (`@id, @type, declaredName, …`), so a
  caller who wrote one back got a second, different error: `@type` and `@id` are not OSLC query text.
  The list is now the OSLC predicates (`rdf:type, sysml:declaredName, …`), derived from the mapping
  the parser reads, and `@type`/`@id` name their OSLC spelling instead — `rdf:type`, and identity,
  which every result reports rather than asks for. The list follows the query's own `oslc.prefix`
  bindings, since a rebound `sysml` or `rdf` changes what the parser accepts. An `oslc.prefix`
  binding whose prefix no prefixed name can be written with (`!s=<…>`) is refused where it is
  bound rather than accepted and never usable, and a prefix of letters outside ASCII
  (`sÿsml=<…>`) now scans as one name in a query instead of ending mid-letter.

- **An anonymous `doc` or `comment` before a kind keyword is kept.** `doc /* … */` followed by
  `attribute a;` in a definition or usage body parsed as an attribute prefixed by `doc`, so the
  documentation vanished silently from the model — absent from validation, printing, the LSP and
  RDF conversion (Open-MBEE/OpenSysML#85). A `doc`, `comment`, `rep` or `locale` annotation ends
  with its comment body and never qualifies the member after it; the bundled standard library
  gains the documentation this had been dropping.

- **A wider-typed expression binds to a narrower feature.** `return : Integer = 7 / 2;` was
  refused statically because the quotient's type is `Rational`, yet an expression's static type
  only bounds its values — `4 / 2` is whole. A binding, argument or index is now refused
  statically only when the two types are disjoint or the value is a literal, whose type is exact;
  everything else is deferred to evaluation, where the value it actually turns out to be is still
  checked. This matches the pinned pilot, which accepts the declaration and evaluates it.

- **Every exported `Session` method holds the session lock.** The run, check/solve, view, document
  and diagnostics entry points did not take it, so a Tab completion racing one of them touched the
  lazily built index, name table and runtime context unsynchronized — 144 races under `-race`,
  now none, with a test running every entry point concurrently.

- **Node client:** restarting the service waits for the previous process to exit before
  reconnecting, and does not wait on one that already exited.

- **Arithmetic outside a type's range is reported, not returned.** A wrapped Integer sum, the
  least Integer's negation and its remainder, an infinite Real from an out-of-range literal, a
  folded infinity, and a quantity magnitude outside the Real range each answered as if computed;
  every one is now a typed error naming the range. An ordinary negated literal evaluates rather
  than being mistaken for a fold, seeded outputs are reported for what they are, and an escaped
  attribute default is decoded before use.

- **An unbound requirement subject is reported as unbound.** A requirement checked with nothing
  supplying its subject read as a modelling mistake in the condition (`no value for feature
  sensor`); the diagnostic now names the subject and the three ways to supply one — bind it, check
  it on an object, or assert satisfaction by an element.

- **A debounced call a later trigger superseded does not run.** A timer firing as the next trigger
  arrived ran the work its successor now owned and deleted the successor's entry; a callback now
  confirms it is still the timer its key waits on.

- **Node client:** a short name is looked up on a model adopted by hash; a missing symbol's error
  names what was looked for rather than repeating the service's text; an RPC failure surfaces as a
  typed client error rather than a raw transport error; an impossible encoding or timeout is
  refused at construction rather than carried into a connection; and a failed handshake carries
  the status it failed with.

- **Java client:** a call that outlives its request timeout is reported as the timeout it is, not
  as the service being unavailable, and a value kind the client cannot read is a refusal rather
  than a value silently dropped from the sequence holding it.

- **Rust client:** an empty `$OPENSYSML_SERVICE` no longer selects an unnamed binary, a response
  above the transport's 10 MB default is read, a qualified value is read outside its declaring
  scope, string escapes decode, Integer overflow and non-finite Real folding are reported, and an
  instance graph iterates in declaration order rather than map order.

- **The toolchain download paths close their quality-gate findings.** The stall watchdog owns a
  thread rather than an executor, the pandoc fetch refuses a plaintext redirect, and the mermaid
  install runs no dependency's lifecycle script and finishes before calling itself present.

- **The pilot reference is pinned to a commit, and a stale copy of it is replaced.** The corpora,
  training examples, Xpect suites and grammars were fetched by release tag alone — a name the
  upstream repository can re-point — and a corpus directory fetched at an earlier release was kept
  with only a warning, so a checkout re-pinned from `2026-05` to `2026-07` still measured the old
  material (98 example files where the baselines record 99) and every provenance test failed at an
  unchanged pin. `scripts/pilot-pin.sh` now names the commit the tag must resolve to and every
  fetch refuses a tag that resolves elsewhere; each fetched directory is stamped with the tag,
  commit and repository, and a copy stamped with another pin or not stamped at all is re-fetched;
  and the committed baselines record the commit they measured next to the tag.

## 0.4.2 — 2026-08-31

Release 0.4.2 is where document generation from a model becomes a working pipeline. 0.4.1 shipped the
planning layer alone — nothing executed and nothing rendered. A document's queries now run (named
invocation under a shared budget, relationship traversal, computed expression columns), an evaluated
document renders as CommonMark Markdown or, through an external converter, as PDF, generated diagrams
and cross-document references are content the document declares, and a whole document set is written
as one atomic replacement. Every surface reaches it: `%run-query` and `%render-document` in the REPL,
`-run-query`, `-render-document`, `-doc-form` and `-render-documents` on the command line,
`RunDocumentQuery` and `RenderDocument` over gRPC and on the public Go API, and
`opensysml/documents`/`opensysml/renderDocument` in the LSP, behind the VS Code extension's
`SysML: Render Document`. The [document-generation manual](docs/manual/README.md) documents the
pipeline with examples that are rendered by the binary the release ships.

The public Go API stops being a subset of the service: behaviour, verification, search, reporting,
conversion and editing are all methods on `opensysml.Client`, and a model spread over several files
is parsed and indexed as one model rather than concatenated. The four non-Go clients now provision
the service the same way — each downloads a `sysml-grpc` release and verifies it against digests
pinned in one shared table, refusing an unverifiable release rather than answering from a cache.

Two example walkthroughs were written to find defects and did: a relay probe across its mission
phases and a bomb-disposal team around the robot. What they found is the runtime and parser half of
this release — a structural `first a then b;` that parsed as an initial node and shadowed the
snapshot it named, sends that could not cross a connector their owner declared or reach a nested
part's identity, signals matched by short name instead of resolved identity, and a bound subject
that did not carry its subject's type.

Configuration unifies on the `OPENSYSML_` prefix, with the `SYSML_` spellings still accepted, and the
load path allocates 15% less on a 12,000-element model. No model that validated under 0.4.1 stops
validating and no import path moves.

### Added

- **A document definition written in the model renders as Markdown.** `%render-document` in the REPL
  and `-render-document` on the command line compile a `part def` specializing
  `DocumentQueries::Document`, run its queries against the loaded model, render its diagram blocks
  through the view engine, and write CommonMark. Sections nest, paragraphs and lists compose
  statically-authored inline runs (`Span` with a `plain`/`emphasis`/`strong`/`code` style, `Link` to a
  URL, `Ref` to another block's anchor) with query-backed values styled through nested `SpanColumn`
  and `LinkColumn` column runs, a table's columns are the query's projected properties and its
  computed `Column` names, and a `groupBy` column writes one subtable per group value. A `Diagram`
  block embeds a declared view — or an element with a stated rendering kind — as a fenced `mermaid`
  block, a table-kind view as a pipe table, with an optional caption and flow direction. Rendering is
  deterministic: the same model and document produce the same bytes.

- **The same document renders as PDF, through a converter chosen at run time.** `-doc-form pdf`
  converts the rendered Markdown with `weasyprint` (default), `pandoc` or `prince`, selected by
  `-pdf-engine`, so the binary links no PDF renderer and Markdown output needs none of them;
  `-pdf-title-page`, `-pdf-toc` and `-pdf-number-sections` are the document-level options. A PDF is a
  binary artifact, so `-doc-form pdf` requires `-o`, and a missing tool stops the run with a typed
  error rather than a partial file.

- **A model's documents render as one linked set.** A `Ref` may target a block in another document,
  and `-render-documents <dir>` renders every document the model declares into a directory, one file
  per document, so those references resolve as on-disk links. The set is committed atomically: the
  rendered files replace their destinations together, a failure restores what was there, and a crash
  cannot leave half a set behind.

- **Document queries execute.** A query definition specializing `DocumentQueries::Query` runs as a
  pipeline over the model — filtering, ordering, projection, named relationship traversal, and
  invocation of another named query with explicit bindings under one shared visit-and-invocation
  budget — and a `Column(name = …, expression = …)` projection is evaluated per row. `%run-query` and
  `-run-query "<name> [<p>=<expr>…]"` report the rows directly, which is how a query is written and
  checked before a document consumes it.

- **The service, the LSP and the editors expose the pipeline.** gRPC gains `RunDocumentQuery` and
  `RenderDocument`; the LSP gains `opensysml/documents` and `opensysml/renderDocument` behind an
  `openSysmlRenderDocument` experimental capability, plus completion and hover for query authoring;
  and the VS Code extension's `SysML: Render Document` renders the model as currently typed into a
  Markdown preview beside the editor.

- **Every client provisions `sysml-grpc` from a verified release.** The Node, Java and Rust clients
  download the service binary for the host platform and verify it against a SHA-256 digest pinned in
  the repository, and the Python client resolves a named or `$PATH` binary the way the others do.
  Downloads are staged per process, bounded, and taken under a shared cache lock, an unverifiable
  release is refused rather than answered from a cache, and the pins themselves are generated into
  every client from one shared table.

- **The public Go API covers every operation the service answers.** `ExecuteAction`,
  `ExecuteState`, `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`, `EvaluateCalc`,
  `Query`, `QueryOSLC`, `RunDocumentQuery`, `RenderDocument`, `Convert`, `ConvertSource`,
  `ConvertFile` and `ApplyEdits` join parse, lookup, evaluation and instantiation on
  `opensysml.Client`, in-process and over Connect alike, so an embedding program no longer drops
  to the generated protobuf stubs for behaviour, verification, search, reporting, conversion or
  editing. Queries are written with typed conditions (`Equals`, `Greater`, `Less`, `All`, `Any`
  and `Not`, which De Morgans a composite rather than sending a shape the service rejects);
  a verdict that is false or undecided is returned as an answer, while a request that cannot be
  answered at all is a `VerifyError`, and a group of edits that will not apply is an `EditError`
  naming the failure and the elements still referring to the target.

- **A model spread over several files is parsed as one model.** `ParseFiles` and
  `ParseDocuments` on the public Go API — the `ParseSources` RPC and the `parse_sources`
  capability on the service — parse each document on its own and index all of them together, so
  an import between them resolves and every symbol of the set is one lookup, evaluation or
  instantiation away. Nothing is concatenated: a document keeps its own name, a diagnostic
  locates itself in the file it came from, and `Model.Roots` holds each document's root. The two
  operations that write one document's notation back out — conversion from a model hash, and
  editing — refuse a model of several documents rather than picking one.

- **A relay-probe walkthrough for the identity and lifecycle notation:**
  [`examples/relay-probe-demo`](examples/relay-probe-demo/README.md) models one individual probe
  across its mission phases — event occurrences ordered in time, snapshots and a timeslice of one
  individual, occurrences with multiplicity, a calculation reading a feature across two snapshots,
  a requirement whose bound subject is a snapshot, and a beacon inside a timeslice sending
  telemetry through the probe's own antenna. It was written to find defects, and found the three
  below.

- **A second bomb-disposal walkthrough, written for the notation the first one does not reach:**
  [`examples/disposal-team-demo`](examples/disposal-team-demo/README.md) models the team around
  the robot — quantities with units and a payload budget, `select` and `reduce` over the fleet,
  a command crossing the connector the site joins two parts by, a callout occurrence with a
  snapshot and a timeslice, and a requirement, use case, verification case and analysis case over
  the same subject. It was written to find defects, and found the three below.

- **A document-generation manual**, [`docs/manual/`](docs/manual/README.md): the concepts, the
  smallest working document end to end, a query cookbook, document authoring, the output forms and
  their determinism, the CLI/REPL/gRPC/Python interfaces, a worked example and troubleshooting. Every
  snippet in it parses and every rendered output shown was produced by the binary this release ships,
  and the documentation link checker now reads the bracketed and angle-bracketed destinations those
  pages use.

### Changed

- **Configuration is spelled `OPENSYSML_`.** Every variable `sysml`, `sysml-lsp` and `sysml-grpc`
  read — the library path, the six execution budgets and the gRPC index pool — uses that prefix. The
  legacy `SYSML_`-prefixed names remain accepted indefinitely; when both are set and the
  `OPENSYSML_` value is non-empty it wins, and setting only the legacy name prints a one-time
  deprecation warning naming the form to switch to.

### Performance

- **A load allocates 15% less.** Three allocation sources paid once per token or per name parsed — a
  fresh string for every keyword's text, a parts slice for every qualified name, and a
  redefinition-closure map per inherited symbol — now cost one allocation per file or none. On a
  12,000-element model that is 3.69M allocations rather than 4.33M and 474.1 MiB rather than 487.7,
  with wall time unchanged: the win is collector pressure, not bytes. Diagnostics and exit status
  were verified byte-identical against the previous binary.

### Fixed

- **An OSLC `<uri>` value selects what the prefixed name selects.** A term written
  `rdf:type=<https://www.omg.org/spec/SysML#PartUsage>` parsed and then matched nothing, because a
  URI value was compared whole while `rdf:type=sysml:PartUsage` was reduced to the local name the
  model holds. Both forms now reduce alike for the SysML namespace; a URI outside it is still
  compared whole.

- **An OSLC query parameter this implementation does not read is refused, not ignored.** A misspelt
  `oslc.wheree=…` was dropped and the query then selected the whole model — the widest possible
  wrong answer, reported as success. An unknown parameter, a parameter given twice, a parameter
  written with no value (`oslc.where=`, `oslc.select=`, `oslc.orderBy=`, `oslc.prefix=`), and a
  non-wildcard `oslc.properties` (which names `oslc.select` in its message) are now typed
  malformed-query errors.

- **An unquoted model qualified name says what to write instead.** `sysml:qualifiedName=Robot::Platform::battery`
  reported `OSLC prefix "Robot" is unbound`, naming a prefix the caller never wrote; it now names
  the quoted literal form the value needs.

- **A query that matches nothing says so.** Both query surfaces printed nothing at all, which a
  caller could not tell apart from a query that failed to run: `%query` now prints `no elements
  matched`, and `sysml -query` reports it on standard error, so the result rows on standard output
  remain one line per match.

- **A `*` value is refused off the multiplicity bounds, and empty `-query` text is a misuse.**
  `sysml:name=*` was compared as the literal value `*` and reported a successful no-match, while the
  same wildcard elsewhere in a query was a typed refusal; it is now refused on every property but
  `multiplicityLower` and `multiplicityUpper`, where `*` is the model's own infinity value.
  `sysml -query ''` was indistinguishable from an absent flag and started the interactive REPL
  instead of answering.

- **The public Go API holds its contract for a binary that imports it.**
  `ServerInfo.Version` reports the OpenSysML module version the importing program resolved rather
  than that program's own; an in-process call honours its context as the wire does, refusing a
  context already done; every call after `Close` is refused with `CodeUnavailable`, and closing
  twice is not an error; and a `StatusError`, a quantity, an enum literal and an unset value print
  as a caller would write them rather than as Go struct dumps.

- **A caption is no longer confusable with an emphasized paragraph.** The
  Markdown renderer wrote a table or diagram caption and an emphasis-only
  paragraph identically as `*text*`, so the PDF backend styled every such
  paragraph as a caption. The renderer now precedes every caption with a
  `<!-- caption -->` metadata line — invisible in ordinary Markdown
  renderers — and the PDF backend styles only marked lines as captions,
  rendering bare emphasized lines as ordinary paragraphs. A marker without
  a caption line after it is a typed `dangling-caption` error.

- **A structural `first a then b;` is a succession, not an initial node.** Ordering two
  snapshots of an individual in time (`first postSeparation then postFlyby;`) parsed as an
  initial-node member named `postSeparation`, which shadowed the snapshot it named — the first
  portion read as `<unknown>` and everything downstream of it (a calculation's default, a
  requirement's bound subject) failed. A two-ended `first ... then ...` in a structural body now
  parses as a `SuccessionAsUsage` over its members; the one-ended `first a;` stays an initial
  node, and an action-carrying body's `first` still opens its initial-node member.

- **An accepted signal message binds as an occurrence of its signal.** `send Telemetry(frames =
  3.0) via antenna` matched an `accept t : Telemetry` but bound nothing: the message carried its
  arguments, and the accept only understood a single carried value. The accepted name is now
  bound to an occurrence of the signal, its features set from the send's named and positional
  arguments — so a transition effect reads `t.frames`. A message carrying neither a value nor a
  signal is still `ErrNoValue`.

- **A send from inside a nested part finds its port on the enclosing part.** A beacon running in
  a timeslice of the probe sending `via antenna` — the probe's port, not the timeslice's — was
  unroutable: owner routing started at the sender itself, so the probe's connector was never
  consulted. Routing now starts at the object actually holding the resolved `via` port, so the
  enclosing part's connectors carry the message.

- **A connector end through a multi-valued feature fans out.** A send over
  `connect console.command to units.command` where `part units : Unit[2]` was
  `ErrUnroutableSend`: an end reached through a multi-valued feature resolved to no object. Such
  an end now denotes every element the feature holds (KerML 1.0 §7.3.4.6), so one send delivers
  one message per element, each on that element's own identity — of addressing generally, not
  only the owner-level route. The squad site of
  [`examples/disposal-team-demo`](examples/disposal-team-demo/README.md) shows it.

- **Message signals match by semantic identity, not short name.** Two same-named item or signal
  definitions in different packages were conflated, and an accept of a supertype did not take a
  message of a subtype. A message now carries its resolved signal symbol, and an accept takes a
  message whose signal conforms to the type it names — qualified identity plus subtype
  conformance through the semantic model.

- **`send x via p` routes by what `p` resolves to, not the name written.** Connector-end
  matching compared the written name, so a port a behavior declares under the name of one of the
  performer's connected ports did not divert the route. The `via` target is now resolved once at
  lowering with the usual scope-aware shadowing, so the behavior-local port is used and the outer
  connector receives nothing.

- **A part's own connector into a part it holds delegates inward.** `connect command to
  unit.command` delivered the message on the sender's own identity under the port path
  `unit.command`, so the nested part never accepted it. Every receiving end of a route is now
  resolved to the object holding the port it names, so the copy is held to the nested part's
  identity, as it already was for a connector an owner declares between two siblings. An end whose
  part holds no object this run has nothing behind it, so such a send is now `ErrUnroutableSend`
  rather than a message posted to a port path no consumer reads.

- **A send now crosses a connector its owner declared.** A part's port joined by its owner to a
  sibling's port reached nothing: routing consulted only the connections of the behavior and of
  the sending object, so a console commanding a unit over the connector their site declares
  reported `send reaches no receiving port`. Deliveries now also follow the connections of every
  object holding the sender, and arrive on the peer object's own identity.

- **An item object can be sent.** `send cmd via p`, where `cmd` is an `item cmd : Command { … }`,
  reported `message of kind instance has no signal type`: a message took its type from a scalar
  value only, so an object had none. An object's message is typed by the definition it
  materializes, which is the type an accept of it names.

- **A bound subject now carries the subject's type.** `requirement r : Req { subject truck = loaded; }`
  redefines the definition's `subject truck : Truck`, but the redefinition was not among the
  usage's supertypes, so `truck.payload` named no member and `%check` refused the requirement.
  Implicit role redefinitions are direct supertypes, so a subject or objective bound in a usage
  reads the members of the role it redefines.

- **`isReference` and `isComposite` are derived from what a usage declares.** Reflective reads
  reported the flags a declaration carried literally, so a query over metaclass features answered
  from the notation rather than from the reference semantics the usage has; every
  declaration-backed metaclass modifier flag is now reflected the same way, and a metaclass feature
  is read through metaclass conformance.

- **The REPL evaluates a bare expression.** A line that was neither a command nor a declaration was
  echoed rather than evaluated; it is now evaluated, a materialization failure is reported as one,
  qualified suggestions are ranked ahead of unqualified ones, and warnings are printed before the
  load lines they belong to rather than after them.

- **The orthogonal-regions demo terminates.** Its regions completed only on each other's completion,
  so running it livelocked; the demo now uses timed transitions, which is what the notation offers
  for a region that must advance on its own.

- **A rendered document set cannot be lost to a failed write.** Staged documents and their
  directories are synced before the set is committed, a destination that already exists is replaced
  portably rather than removed first, a case-aliased or colliding destination is rejected, and a
  rollback restores a backup even where the failed replacement had removed its destination.

## 0.4.1 — 2026-08-30

Release 0.4.1 is about what the tools *say* about a model. Every surface that names a declaration —
a rendering, the REPL's echo and search, a runtime diagnostic, LSP hover and completion — printed the
classification the implementation keeps internally rather than the notation the file was written in,
so a datatype read as an attribute and a KerML classifier grew a `def` it never had. The written form
now has one source, and those surfaces all read from it; hover and completion documentation render as
Markdown for a client that advertises it, and as plain text for one that does not.

One import path moves: the public Go API is now `github.com/Open-MBEE/OpenSysML/client/opensysml`.
The API itself is unchanged, but Go has no import alias, so **a Go consumer must edit the import
line** — the one change in this release that a user has to make. No model that validated under
0.4.0 stops validating.

Behind those, native document queries gain a compiled planning layer (planning only: nothing
executes or renders yet), the Python and Rust clients move under `clients/` beside the Java and Node
ones, and the SonarCloud gate is measuring the project it is meant to — every language's tests now
count toward coverage, the Java sources are analyzed with types, and the bug and vulnerability
backlog is empty.

### Changed

- **Every language's tests now count toward measured coverage.** The scan read only the Go
  profile, so the Java, Python and TypeScript suites — all of them passing in CI — had every line
  they cover counted as uncovered. Each client job now writes a report (JaCoCo, `pytest-cov`, `c8`)
  and the scan waits on those jobs. The Go profile is written with `-coverpkg`, which credits a
  package for the code it exercises elsewhere: `internal/core/ast/dump.go` measured 21% while the
  parser's golden tests ran 90% of it. `make python-coverage` and `make node-coverage` write the
  reports locally.

- **The public Go API moved to `github.com/Open-MBEE/OpenSysML/client/opensysml`**, from
  `.../pkg/opensysml` — the top-level `client/` directory Go projects conventionally publish a
  client library from. Go has no import alias, so this breaks every consumer's import path; update
  the import, nothing else. The API is unchanged and still ships with the core `v*` tags.

- **The Python and Rust clients live under `clients/`**, beside the Java and Node ones, rather than
  at the repository root. Every path that named them moves with them: the CI jobs and publish
  pipelines, the changed-area filters, the Makefile targets, the buf output paths, the analysis
  inclusions and the documentation. Neither published package changes name, version or contents.

### Added

- **Native document queries now have a compiled planning layer.** Query definitions specialize the
  bundled `DocumentQueries::Query` vocabulary, retain typed parameter/result metadata and source
  provenance, and may invoke other named queries with explicit named bindings. Planning produces an
  immutable dependency-ordered program and reports malformed definitions, unknown operations, bad
  bindings, positional query composition, and complete direct or indirect composition cycles as
  typed validation diagnostics. Execution and document rendering are not part of this release.

- **Hover renders as Markdown when the editor supports it**: the signature is a fenced `sysml`
  block and the doc comment reads as prose. A client that does not advertise Markdown still gets
  the plain text it did before.

### Fixed

- **An element is named the way its notation writes it**, on every surface that names one — a view
  rendering, the REPL's echo and search, a runtime diagnostic, and LSP hover and completion. These
  printed the internal classification of a declaration instead, so a `datatype` read as an attribute
  and a KerML classifier was given a `def` suffix it never had. A short name that is not a valid
  identifier is quoted as the notation requires, and emphasis a doc comment was authored with
  survives into the rendered documentation.

- **Hover keeps what the file says.** Each leading comment's delimiters are stripped on its own, a
  doc comment keeps the line breaks it was written with, a named relationship's prefix stays out of
  its signature, and a relationship is named by the keyword its name follows.

- **Both operands of `?` may be conditional expressions.** The parser accepted one only in the else
  branch, though `KerMLExpressions.xtext` makes both owned expressions and limits only the condition
  to a null-coalescing expression.

- **The Connect server's shutdown no longer runs on an already-cancelled context.** It derived its
  30-second grace period from `context.Background()`, dropping the request context's values; it now
  derives it from a cancellation-free copy of the server's own context.

- **The pilot validators normalize EMF object references with a linearly-scanned pattern.** Theirs
  began with a broad character class, so a message without a reference was rescanned from every
  position. It anchors on the `@` that starts the identity hash instead, and the qualified name
  before it is left in place rather than rewritten unchanged.

- **A failing Java, Node or public-Go-API job now fails the PR gate.** GitHub's `Build and test`
  check exists to give path-filtered jobs one stable required name, but its `needs` listed neither
  client test job nor the `client/opensysml` conformance run, so all three reported green through
  it. It waits on every job now, and `node-test` waits on `build` — the job that uploads the binary
  it downloads — rather than on the gate itself, which had chained it behind the whole workflow.

- **The scan analyzes the Java client with types and the Python client against its own version
  range.** It warned about missing `sonar.java.binaries`/`sonar.java.libraries` and fell back to a
  syntactic analysis of the Java sources, and assumed every Python 3 version, which drops the rules
  that depend on one. `java-test` now persists each module's compiled classes and its dependency
  jars, `sonar-project.properties` names them, and `sonar.python.version` names the `>=3.10`
  through 3.13 range `clients/python/pyproject.toml` declares.

- **The behavioral-bodies demo's state machines can be run.** Each of its four machines declared
  substates but no transition out of its entry action, so `%state PhaseC::Running` (and the other
  three) failed with `no initial state found`; the Boolean features its guards and triggers read had
  no value either. Each machine now names the state it starts in and those features are initialized,
  so all four start and step. No diagnostic moves on either side.

- **The demos are written in standard notation wherever one exists.** `then done;` in place of a
  standalone `done;`, `entry`/`do`/`exit <action>` and named effect actions in state bodies,
  `accept when <event>` triggers, `assert constraint` for an analysis case's own conditions, and the
  objective subject the trade-study library redefines. The pseudostate notation, which no SysML v2
  grammar has a production for, is now written in `examples/pseudostates-demo.sysml` alone and stays
  supported everywhere. Every demo's output is unchanged; the pilot differential baseline and the
  figures quoted from it move.

- **The semantic-layer demo declares its packages with `package`.** Its three `namespace`
  declarations are KerML notation — the SysML grammar has no `namespace` production — so the pinned
  pilot could not parse the file and the non-standard-notation pass warned on each. The file now
  agrees on both sides, which moves the pilot differential baseline and the figures quoted from it.

- **The header's Community Wiki link points at the wiki's landing page**, rather than at the wiki
  root, which lands on whatever page GitHub considers first.

- **A deserialized `ModelException` cannot claim an unbounded diagnostic count**, so a hostile or
  corrupt stream no longer has the Java client allocate for one.

### Project

- **The SonarCloud bug and vulnerability backlog is cleared.** A sort compares by an explicit
  code-unit comparator rather than an implicit collation, the Java client keeps its `Optional` and
  queue returns, workflow permissions are scoped per job, the CI Python installs are pinned, and the
  VS Code extension's webview ignores a message from any other origin. `sonar-project.properties`
  states, with its reason, each rule whose subject in one file is a developer command's documented
  behavior.

- **The maintainability findings behind it are cleared too**, across Go, Java, Python, Rust,
  TypeScript and the shell scripts: parameter lists over seven entries became option structs,
  switches over thirty cases dispatch through kind-scoped helpers or lookup tables, duplicated
  literals are hoisted, and the coverage script validates that the profile path it is given stays
  in the working tree and reports a symlink loop rather than a traceback. Behavior is unchanged
  throughout; the generated Python gRPC stubs and the cognitive complexity of Go test files are
  excluded from analysis, since neither is code a reviewer edits.

## 0.4.0 — 2026-08-28

Release 0.4.0 is about what a model *writes*. An `assign` used to put any value into any feature —
a `String` into a `Real` attribute, a length into a duration — statically and at run time; a write
now answers to the target feature's declared type, multiplicity and, where the target is a quantity,
its dimension, on every path that stores a value. **A model that relied on an unchecked write no
longer validates**, which is why this is a minor rather than a patch.

The runtime also learned the structure it was flattening away: a typed state usage materializes the
content of the definition typing it, a nested action node runs the flow it owns, an assignment target
may be a feature chain, and a performed action's features live on the performance occurrence that
holds them rather than on whatever like-named feature the performer happened to have. A behavior body
now resolves a name where the name is written rather than reaching into the performer for it.

Beyond execution, a view specializing `SequenceView` renders as a sequence diagram, a `Real` prints
as the shortest decimal that reads back as the same value, and the pinned OMG pilot implementation
moves to release `2026-07` (`jupyter-sysml-kernel` 0.61.0) with every oracle baseline re-recorded
against it.

### Changed

- **A write answers to its target feature's declared type and multiplicity.** An `assign` wrote any
  value into any feature: a `Real` attribute accepted a `String`, statically and at run time. KerML's
  FeatureWritePerformance "assigns the values of a feature on an occurrence to the given
  replacementValues", so those values are values of that feature — the rule already applied to an
  initial value. The type pass now walks every body an `assign` may stand in and checks the written
  value with the initial-value rule resolved against the target's declaration, and the runtime checks
  the same before storing, on every write path. A rejected write leaves the feature as it was.
  - **An output a declaration binds is checked too**, so a body-less `out a : Integer = n` no longer
    answers with whatever an untyped input carried: the rule holds however an output is given its
    value.
  - **A written decimal conforms to `Rational`**, as the type tier already read one. The two tiers
    disagreed, so a decimal written to a `Rational` feature validated clean and then failed at run
    time.

- **A bound or written quantity is judged by the dimension its target declares.** A feature declared
  with a quantity value type refuses a quantity measured in another dimension — statically where the
  value's dimension is determined, and at run time on every write path.

- **The pinned OMG pilot implementation is now release `2026-07` (`jupyter-sysml-kernel` 0.61.0)**,
  with the reference validators, the vendored standard library, the pinned corpora, grammars and
  Xpect suites, and every oracle baseline re-recorded at that pin. Two notations the new grammar
  admits now parse: a metadata usage that declares its own name or none at all
  (`@ m : Security;`, `@ : Security;`, `@ m typed by Security;`) and a constraint reference
  carrying a multiplicity (`assume c [0..*];`). The errata overlay drops the redefinition
  correction the published corpus now makes itself.

- **Why a listed view is not drawable is now written under the diagram** rather than only in the
  picker entry's tooltip, so a `geometry` or `textual` view says what it is that cannot be drawn
  without the reason having to be hovered for.

- **A Real prints as the shortest decimal that reads back as the same value**, on every surface
  that renders one — an evaluation result, a feature value listing, a quantity's magnitude, an
  execution trace and the simulation clock. Values were rendered to two decimal places, which
  reported a nonzero magnitude as zero (`0.0001` printed `0.00`) and rounded away precision the
  evaluation had kept (`1.0 / 3.0` printed `0.33`, `123456789.987654` printed `123456789.99`),
  and disagreed between surfaces. A whole Real keeps its `.0` so it is not mistaken for an
  Integer. Arithmetic is unchanged: the stored value was never rounded.

### Added

- **A typed state usage inherits the content of the definition typing it.** The definition's
  substates, initial transition, entry/do/exit behaviors, transitions, deferred events and
  attributes are materialized per usage rather than shared, including inside a parallel body.
  Recursive typing, and content lowering cannot represent, are typed errors rather than silence.

- **A nested action node runs the flow it owns.** A nested node's own members are its
  subperformances: lowering carries them as a subgraph and the executor performs that flow before
  the node completes. An action usage stating no body of its own resolves to the action definition
  typing it, so `%action` on a `perform` usage runs.

- **An assignment target may be a feature chain.** Lowering carries the whole walk and the runtime
  writes the feature on the object the chain reaches, resolved in the statement's own scope.

- **A view specializing `StandardViewDefinitions::SequenceView` (or `sv`) renders as a sequence
  diagram**, at the prompt (`%render`), from the command line (`sysml -render`) and over the LSP, in
  the text form and as a Mermaid `sequenceDiagram`. The occurrences an interaction declares in its
  body are its lifelines, a `message`/`flow` usage is a directed message between the lifelines its
  ends' events belong to, and the successions between those events order the messages — a cycle
  among them is reported and declaration order stands. What a sequence diagram cannot show — an
  exposed element that holds no occurrence, an undirected `connect`, a message stating no ends or
  attaching to something the view does not expose — is reported rather than dropped. A `geometry`
  view remains recognized but not drawn.

- **The `opensysml/views` response now reports supported pseudo-view specs**, so clients can offer
  newly supported rendering kinds without maintaining a second list.

### Fixed

- **A behavior body resolves a name where the name is written**, rather than reaching into the
  object performing it for any like-named feature — which name resolution does not admit. A
  performer feature is read and written only where the name resolves to it, and `this` inside an
  owned performance denotes the owning object.

- **A performed action's features are written to its performance occurrence.** A performed action's
  declared attributes and out parameters are features of the action performance the `perform` usage
  holds, so they are initialized from that occurrence's slots and written through it.

- **A state's attributes reach the occurrence exhibiting it**, so an exhibited state and the
  occurrence behind it no longer hold divergent data.

- **A send nested in a flow is routed by every flow around it.** A send saw only the action's own
  connectors and those of the flow it sat in, so a connector declared by an intermediate flow could
  not route it; each activation now carries the connectors of every enclosing flow.

- **An inherited transition retargets to a redeclared substate**, rather than to the substate the
  supertype declared.

- **A machine's own supertype is no longer read as recursive typing**, which rejected a state
  machine that specialized another.

- **A chained assignment binds no output of a calculation**: the chain's last segment was counted as
  the calculation computing an output of that name.

- **Clicking a sequence participant now reveals its declaration, and the cursor highlights it**
  using Mermaid's participant data attributes.

- **The site's OpenMBEE links point at the host that serves HTTPS.**

### Project

- **The end-to-end example is one model, driven three ways.** A bomb-disposal robot whose structure,
  calculations, fork/join and nested flows, hierarchical state machine, solver cases and views are
  the same model throughout, with a walkthrough of the commands that drive it from the CLI, the REPL
  and the Python client. A nested action node no longer reports the token-flow limitation when a
  view renders it.

- **The dimensional defect in the pilot's `Dynamics.sysml` is a declared erratum**, so the corrected
  copy is what the oracles re-run over and the published corpus is still never edited.

- The documentation site states the Open-MBEE and NumFOCUS affiliation in its footer, and the
  header's wiki link is labelled Community Wiki.

- **The Python client is unchanged in this release**, so no `opensysml` version is published with
  it: the client published as 0.3.2 installs a v0.4.0 `sysml-grpc` by taking the asset's digest from
  the release's signed `SHA256SUMS.txt`, which is what an unpinned release's verification is for.
  Pinning v0.4.0's digests in `python/opensysml/binary.py` and publishing 0.3.3 remains optional,
  and is worth doing only for callers still on `opensysml` 0.3.0, which predates that path and
  installs a release it pins no digest for only under `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`.

## 0.3.1 — 2026-08-27

A performance patch. Loading a model costs less of everything and answers the same: on a
12,000-element synthetic model, 4.62M allocations rather than 7.70M, 503.6 MiB allocated rather
than 744.9 MiB, 272 MiB peak resident rather than 353 MiB, and 0.92s rather than 1.14s. Nothing
about the language, the diagnostics or any API changed — the 895 bundled SysML and KerML files
report byte-identical diagnostics and exit status, and the 749 of them a conversion accepts produce
byte-identical `-convert sysml` and `-convert ttl` output, before and after.

### Performance

- **Whitespace is no longer recorded as trivia.** A node's leading trivia holds the comments and
  notes a consumer reads (doc-comment hover, the REPL's declaration printing) and nothing else,
  which is what dominated allocation on a large parse.
- **A span's text is served from one cached whole-file string** instead of copying the bytes per
  span, so the repeated reads name resolution and validation make cost nothing after the first.
- **A fully qualified name is compared without being built.** Checking whether a symbol *is*
  `Base::DataValue` walks the scope chain against the string rather than constructing the name to
  throw away; constructing one, where a caller genuinely needs the string, sizes its buffer once.
- **The inherited-name conflict pass reads the memoized base-member maps directly** rather than
  merging them per declaration, and merges only where a declaration has more than one base.
- **A scope's children are indexed lazily**, by map only where a scope is large enough for the map
  to pay for itself, and by scan below that.

### Project

- The release-gate counts on `README.md`, [the roadmap](docs/project/roadmap.md),
  [spec compliance](docs/project/spec-compliance.md) and
  [training examples](docs/project/training-examples.md) are recounted together against a real
  `go test -race -count=1 -v ./...` run: 8,361 tests and subtests, 380 execution conformance cases,
  118 golden traces, 256 runtime robustness cases, 146 golden ASTs and 261 negative parser subtests.
  The skip list is stated by what each skip wants, since five tests it named as skipping now pass.

## 0.3.0 — 2026-08-26

Release 0.3.0 spends itself on a single question: what does this implementation accept that the
specification does not? Since 0.2.0 the answer was a warning — thirteen OpenSysML-only spellings were
read, reported as non-standard notation, and executed anyway. That register is now closed. Each of the
thirteen is a parse error, the warning for it is gone, and the standard spelling that replaces it is
documented, migrated throughout this repository's models and guide transcripts, and covered by tests.
**A model relying on any of them no longer parses**, which is the point: an analyzer that quietly
accepts what no production admits teaches its users a notation no other tool reads. The version is a
minor rather than a patch for that reason.

The state machine's completion semantics were re-founded in the process: a machine completes on a
transition to the standard library's `done` rather than on a marker keyword, computed while lowering
and carried in the graph the runtime executes, so completion is a property of the machine rather than
of its syntax. Beyond notation, converted RDF states element ids and ownership as the abstract syntax
does, so a SysML v2 API service can address a graph this tool produced; `sysml-grpc` speaks the
Connect protocol by default, on the same port as gRPC and gRPC-Web; the Python client starts a private
service of its own instead of adopting whichever one is listening; and what a real Flexo MMS stack
does with our Turtle is now a recorded per-property measurement rather than a claim.

The service also stops being reachable from one language only. Four client surfaces ship with this
release — a public Go API that answers in process, and Node/TypeScript, Java and Rust clients that
speak the Connect protocol — and each of the four runs the same language-independent conformance
scenarios through its own public API rather than through generated stubs, so "it speaks to
`sysml-grpc`" is a measured claim in every one of them. None of the four is published yet.

Against the pinned OMG pilot (`2026-05`, jupyter-sysml-kernel 0.60.1), 328 of 355 files agree
diagnostic-by-diagnostic, there is no declared `errors` row in the reference's own suites we are
silent on, and 120 of 120 authored invalid models are rejected by both implementations.

### The notation this release no longer accepts, in one table

Each spelling was an OpenSysML extension warned as `nonstandard-notation`; each is now a parse error,
and its warning is gone. None of the words involved is reserved by the pinned grammars, so
`state final;`, `attribute initial : Boolean;` and `region` as a name keep working. Every row is
described in full below.

| No longer accepted | Write instead |
|---|---|
| `bind result = x * 2.0;` — an expression as a binding's right end | `out result : Real = x * 2.0;`, or bind to a feature holding the result |
| `assert <expr>;` / `assume <expr>;` in a constraint body | the bare condition: `constraint MassBudget { total <= limit }` |
| `assume <expr>;` / `require <expr>;` in a requirement body | a wrapped constraint: `require constraint { power > 0 }` |
| `return <expr>;` in a calculation body | the body's trailing expression: `calc def Add { in x; in y; x + y }` |
| `final <state>;` | `transition first running accept Stop then done;` |
| `initial <state>;` | `entry; then <state>;` |
| `transition <source> to <target>;` | `transition first <source> then <target>;` |
| `region <name> { … }` | mark the owning state's body `parallel`, one `state` substate per region |
| `initial <name>;` as an action node | `first <name> [then <target>];` |
| `final [<name>];` as an action node | `done;`, or `then done;` as a succession target |
| `decision <name>;` | `decide <name>;` |
| `done <name>;` — a named action final node | `done;` |
| `then <source> <target>;`, and a member-leading `<source> then <target>;` | `succession first <source> then <target>;` |

### A name a library supertype already supplies is reported

`state start;` inside a state, or `attribute portions;` inside a part definition, declares a
member indistinguishable from one the declaration inherits from the standard library —
`StatePerformances::StateAction::start`, `Occurrence::portions` — and is now a warning, as the
reference implementation and KerML 8.2.4 have it. Only a name two library supertypes each
supplied was reported before. A member that redefines or subsets the inherited feature is that
feature and stays silent, as do the implicit redefinitions: a positional behavior parameter, the
subject, actors, stakeholders and objective of a case or requirement, and the assignments in a
metadata usage body. Three diagnostics of the reference we did not report become agreements.

### Succession endpoints are checked against their enclosing body

State-machine succession and transition spellings now report a resolved endpoint
that is not a state or pseudostate, using the existing `not-a-vertex` diagnostic.
Action bodies report a resolved endpoint that is not an action node with the
`endpoint-not-a-node` diagnostic before lowering. Positional, implied, unresolved,
and flow endpoints retain their existing behavior. Inherited action nodes are
also collected lazily when named by an endpoint and lowered into executable
edges. The same routing removes a false unresolved-reference diagnostic for
`succession` and `then` ends naming nested or region-local vertices, covered by
`internal/core/parser/testdata/parse/state_history_entry_exit.sysml`,
`internal/core/runtime/testdata/conformance/state_deep_history.sysml`, and
`internal/core/runtime/testdata/conformance/state_shallow_history.sysml`.
Feature-chain endpoints rooted in a part usage are accepted during validation,
while execution still reports the existing lowering error because invoking an
action through that feature chain is not yet supported.

### A Java client, for a JVM host application

`clients/java/` adds `io.github.open-mbee:opensysml-client`, a Java client for the
`sysml-grpc` service, aimed at the JVM tools this ecosystem is full of: an Eclipse-based
tool, a Cameo plugin, a web service. It parses a file or inline content, evaluates an
expression, looks up a symbol, instantiates a definition and negotiates capabilities.

- **A JDK 17 baseline and one compile dependency.** `protobuf-java`, and nothing else by
  default: the transport is `java.net.http.HttpClient` speaking the Connect protocol with
  protobuf bodies, so there is no gRPC, no Netty and no `tcnative` to conflict with a host
  application's own. `Encoding.JSON` is the debugging affordance and needs the optional
  `protobuf-java-util`. 17 is what an Eclipse 2023-03 or Spring Boot 3 host can offer.
- **One private child per classloader**, the JVM analogue of the Python client's one per
  interpreter, so the copy a plugin loaded and the copy a web application loaded each own a
  service and share its parse cache within themselves; `isolatedService(true)` starts one for
  a single connection where a shared cache across tenants is not acceptable. A service the
  client did not start is explicit opt-in and is never stopped.
- **No orphans, by the same mechanism**: the client holds the write end of the child's stdin
  pipe and never writes to it, and the kernel closes that pipe even on `kill -9` of the JVM,
  which a shutdown hook or `ProcessHandle.onExit` does not survive. A test kills a child JVM
  with `SIGKILL` and asserts the service is gone.
- **The binary is not downloaded.** It is resolved from an explicit path,
  `$OPENSYSML_GRPC_BINARY`, `~/.opensysml/bin` or `PATH`, and a caller-pinned SHA-256 is
  verified before it is executed.
- **The conformance suite runs through the public API**, over both Connect encodings: 25 of
  59 scenarios pass and 34 are skipped as RPCs v1 does not cover (the edit API, RDF
  conversion, verification, behaviour execution and queries), with the report in the shape
  `cmd/conformance` writes. `-mutate` corrupts every answer to prove the comparison is not
  vacuous.
- Java stubs are generated by `buf` from a plugin entry in the root `buf.gen.yaml` with the
  version pinned inline, regenerated by `make proto`, and a `java-test` job in both CircleCI and
  the pull-request GitHub Actions workflow runs the tests and the suite, while the Go job in each
  checks the committed stubs are what `buf` generates.

Nothing is published. The build produces a signable, locally-installable artifact with
sources and javadoc jars, and [releasing.md](docs/project/releasing.md) says what a
maintainer must obtain — a verified namespace, a GPG key, Central portal tokens — before a
first upload.

### A public Go API, in process by default

`pkg/opensysml` is the Go surface of this engine: parse, symbol lookup, evaluation and
instantiation from Go code, with the parser, semantic engine and runtime the importing binary
already links. It is not a wrapper around a port.

- **`New` answers in process**, calling the same service implementation the wire transports serve,
  so the semantics are the service's: the same content-addressed parse cache and model hashes, the
  same capability list, the same in-band failures and the same runtime budgets. No child process is
  ever spawned — a Go process wanting the engine in process has no use for one. `Dial` is for a
  service someone else runs, addressed explicitly, over Connect with protobuf bodies.
- **Two failure modes, and the difference is part of the API**: a refused call is a `*StatusError`
  carrying the canonical gRPC status code (`errors.Is(err, opensysml.CodeNotFound)`), an answer that
  reports a failure is a `*FailureError`, and an engine panic arrives as `CodeInternal` rather than
  unwinding into the caller. Syntax errors are neither: parsing broken source succeeds and the
  diagnostics are on the model, in the shape the LSP and the wire report.
- **Everything returned is a copy the caller owns.** In process there is no serialization boundary
  to enforce that, so it is a documented promise with tests behind it: no returned value aliases
  engine state.
- The conformance suite gained `pkg` and `pkg-connect` protocols, so the covered scenarios run
  through this API both in process and against a started service.

### A Rust client, blocking by default

`rust/opensysml` is a blocking Rust client for the service, with no asynchronous runtime anywhere in
its default dependency tree: all fifteen RPCs are unary and the usual consumer talks to a local
child that answers in milliseconds, so a private `tokio::Runtime` inside a library would tax every
consumer for nothing. A test calls the client from inside `Runtime::block_on` to pin that it is safe
to use from an async program anyway. Rust 1.83 is the minimum supported version.

- **The lifecycle is the one the other clients use**: `Connection::private()` starts one child per
  process and shares its parse cache, `Connection::external` and `$OPENSYSML_SERVICE` are explicit
  opt-in and are never stopped by the client, `Drop` cleans up deterministically, and the child's
  stdin pipe is the guarantee that survives `SIGKILL` and `process::exit` — pinned by a test.
- **No binary is downloaded**: the service is resolved from `$OPENSYSML_GRPC_BINARY`,
  `~/.opensysml/bin` or `PATH`, and the client does not pretend to pin a child version.
- **The conformance runner reads answers through the typed API**, not the stubs, and skips only the
  RPCs the v1 surface does not cover plus the one request that surface cannot express; any other
  skip names its missing capability and fails the run. Publishing to crates.io is a maintainer
  action and CI never does it.

### A request for an unavailable capability is refused

`sysml-grpc` answers a request that asks for a capability it does not have with `UNIMPLEMENTED`,
naming the capability, instead of quietly doing something else; capabilities that only describe how
a response is populated omit those fields as before. The conformance gate runs the default service
and a test-only configuration with capabilities withheld, so both halves of the contract are
exercised, and the Python client turns a service-side refusal into `MissingCapabilityError` while
keeping the original error as its cause.

### RDF output states element ids and ownership

A converted graph now carries the two things a SysML v2 API service needs to address it.
Every element states `sysml:elementId` — exactly the id its own IRI ends in, so the triple
and an id derived from the IRI cannot disagree — and containment is stated as the abstract
syntax does: `sysml:owner` and `sysml:owningRelatedElement` as element references, with a
materialized `OwningMembership`, or a `FeatureMembership` where a type owns a feature,
between owner and member. Each membership is an element of its own with a deterministic IRI,
its own `sysml:elementId`, and the member and owner wiring a client walking from a root
follows; a relationship a namespace declares, such as an import, owns its member directly.
Visibility moves to the membership that declares it. A document now has one root instead of
every element reading as one.

The nodes of an expression graph are addressable too: a node's IRI held a `.` and the name
of the position it sits in, which an API service restricting ids to `[a-zA-Z0-9_-]+` refuses
outright, so the position is now joined with `_p` and encoded the way an element id is, and
the node states that id in `sysml:elementId`. A node is still not a model element.

`.ttl` output changes shape accordingly, and every graph regenerates. Reading is
backward compatible: a graph carrying only the previous compact `sysml:owningNamespace`
shape still converts unchanged, and that property is still written.
[reference/rdf-mapping.md](docs/reference/rdf-mapping.md) documents the ownership shape.
Loading a graph into a live triplestore and reading it back through the API is still not
demonstrated, and collection-valued properties still carry no JSON annotation.

### An expression as a binding's right end is no longer accepted

**Breaking change.** `bind <feature> = <expression>;` — a binding whose right side is an
expression rather than a feature, such as `bind result = x * 2.0;` — was an OpenSysML
extension no SysML v2 production admits, warned as `nonstandard-notation`. It is now a parse
error, and the warning for it is gone: a binding relates two connector ends, so each side must
name a feature. Write the expression as the feature's value instead
(`out result : Real = x * 2.0;`), or, where the feature is declared elsewhere, declare a
feature holding the result and bind to it (`attribute b2 = a + 1;` then `binding bind b = b2;`).
Standard bindings — `bind a = b;`, including qualified, chained and indexed ends — are
unchanged.

### A keyworded inline condition is no longer accepted

**Breaking change.** `assert <expression>;` and `assume <expression>;` in a constraint body, and
`assume <expression>;` and `require <expression>;` in a requirement-style body, were an OpenSysML
extension no SysML v2 production admits, warned as `nonstandard-notation`. They are now a parse
error, and the warning for them is gone.

- In a constraint body, write the condition on its own: `constraint MassBudget { total <= limit }`
  instead of `constraint MassBudget { assert total <= limit; }`.
- In a requirement body, which admits no keyword-less condition, wrap it in a constraint:
  `requirement R { require constraint { power > 0 } }` instead of
  `requirement R { require power > 0; }`.
- A negated condition keeps its truth value in the expression: `assert not (x < 0);` becomes
  `not (x < 0)`.
- What the keywords still state is unchanged: `assert [not] <reference>;`,
  `assert constraint { … }`, `assert constraint C;`, `assume`/`require <reference>;` and
  `assume`/`require constraint [name] { … }`. The separate warning for `assume`/`require` used
  outside a requirement body is unchanged too.

### The state final marker is removed; a machine completes on a transition to `done`

**Breaking change** for models that used it. `final <state>;` in a state body was an
OpenSysML-only notation no SysML v2 production admits — `StateBodyItem` has no such member and no
`final` literal appears in the pinned grammars — warned as `nonstandard-notation`. It is now a
parse error, and the warning for it is gone.

What replaces it is the completion the standard library already gives every state: a machine
completes when a transition reaches `done`.

```sysml
state monitor {
    entry; then running;
    state running;
    transition first running accept Stop then done;   // was: final stopped;
}
```

Entering completion runs the exit actions of the states it leaves and reports the machine
completed, exactly as entering a marked state did; an orthogonal machine completes only once every
concurrent region has reached `done` — the machine's own regions and those of a composite state
alike, so a region completing leaves its siblings running. Completion is
now computed while lowering and carried in the state graph the runtime executes, so it is a property
of the machine rather than of its syntax.

Completion is **stated, not inferred**: a state with no outgoing transition does not complete on
its own, because an ancestor or cross-region transition may still leave it. A machine naming a state
of its own `done` reaches that state, unchanged. `final` is reserved by no pinned grammar, so it
still names a state or a feature (`state final;`, `attribute final : Boolean;`); a state body that
writes `final <state>;` is reported as the parse error it now is.

### The state initial marker and the `to` transition spelling are removed

**Breaking change** for models that used them. Both were OpenSysML-only aliases of notation the
pinned grammars already spell, so the aliases and their `nonstandard-notation` warnings are gone
rather than kept:

- `initial <state>;` → declare the state and start the body's entry succession at it:
  `entry; then <state>;` (`SysML.xtext:1766` `EntryActionMember`, followed by `EntryTransitionMember`
  at `SysML.xtext:1796`). Inside a `region`, the entry succession designates that region's start, as
  the marker did. It lowers to the same `StateGraph` initial state, so no execution result moves.
- `transition <source> to <target>;` → `transition first <source> then <target>;`
  (`SysML.xtext:1854` `TransitionUsage`, whose target is introduced by `then`). Naming the
  transition, guards, triggers and effects are unchanged: `transition t first a if g then b;`. The
  source may also be written without `first`, as the grammar allows.

`initial` is not reserved by the pinned grammars, so it still names a feature
(`attribute initial : Boolean;`, `state initial;`). A state body that writes `initial <state>;` or
`transition <source> to <target>;` is reported as the parse error it now is — in particular
`transition a to b;` does not quietly become a transition between other ends.

The `final <state>;` marker is removed too, described below.

### The orthogonal-region member is no longer accepted

**Breaking change.** `region <name> { … }` in a state body was an OpenSysML extension no SysML v2
production admits, warned as `nonstandard-notation`. It is now a parse error, and the warning for it
is gone. Mark the owning state's body `parallel` and write one state substate per region:

```sysml
state working parallel {
    state left  { entry; then building; state building; }
    state right { entry; then checking; state checking; }
}
```

Region order becomes substate declaration order and each region keeps its name, so entry, exit,
event broadcast, history and completion behave as before. A state whose body is `parallel` may still
own directly what a region body could be reached from — its behaviors, `defer`, the pseudostates its
regions branch through and the edges between them — but every *state* substate of a parallel body is
a region, so a body that mixed one region with ordinary sequential substates has to put the region
set in a state of its own. `region` is now an ordinary name.

### Succession shorthand spellings removed

**Breaking change.** OpenSysML no longer accepts named action final nodes (`done <name>;`),
two-name succession shorthand (`then <source> <target>;`), or state-body member-leading
successions (`<source> then <target>;`). Write `done;` for an action final node,
`succession first <source> then <target>;` for an explicit succession, and use the same
standard succession form in state bodies.

### The action nodes spelled `initial`, `final` and `decision` are removed

**Breaking change** for models that used them. Each was an OpenSysML-only alias of a node the
standard notation already spells, so the alias and its `nonstandard-notation` warning are gone
rather than kept:

- `initial <name> [then <target>];` in an action body → write `first <name> [then <target>];`
  (`SysML.xtext:1385`).
- `final [<name>];` as an action node → write `done;` for the anonymous final node, or
  `then done;` when naming the library feature `Actions::Action::done` as a succession target.
- `decision <name>;` → write `decide <name>;` (`SysML.xtext:1672`).

None of the three words is reserved by the pinned grammars, so each still names a feature
(`attribute final : Boolean;`, `action initial;`). An action body that writes `initial <name>;`,
`final <name>;` or `decision <name>;` is reported as the parse error it now is. A bare
`then final;` is a **succession target**, not a node, so it is read as a reference to a member named
`final`: where nothing declares one, `sysml -validate` reports an unresolved reference at the name —
the same as any other undefined succession target (see below). The **state** markers
`final <state>;` and `initial <state>;` are removed as well, both described above. Named
action final syntax `done <name>;`
remains rejected as described above.

### An undefined succession or transition endpoint is reported at validation time

A succession or transition end naming a member nothing declares — `succession first start then zzz;`,
the guarded `succession first a if c then zzz;`, `then zzz;`, a decision's `if c then zzz;` and
`else zzz;`, and `transition first idle then zzz;` — is now an `unresolved` error of the
name-resolution tier, reported at the name itself. It used to analyse clean and fail only when the
action or state machine was executed, so a model could pass `sysml -validate` and still be
unrunnable. The lowering errors remain as the last check.

The endpoints the notation supplies are unaffected: `start` and `done` are the features
`Actions::Action` declares, an end bound to the member beside a member-attached `then` names nothing,
and a declared `done;` final node is reached as before.

### `return <expression>;` is no longer accepted in a calculation body

**Breaking change.** A computed calculation result is written as the body's trailing
expression, with no keyword — `calc def Add { in x; in y; x + y }` — which is what the
standard grammar admits and what OpenSysML already executed. The OpenSysML-only spelling
`return x + y;` had no production of its own; it is now a parse error suggesting the
trailing-expression form, and the `nonstandard-notation` warning that reported it is gone.

`return` itself is unchanged: it still declares the result parameter of a calculation, so
`return r : ScalarValues::Real;`, `return r : ScalarValues::Real = x + 1;` and the bare
`return r;` — a result parameter named `r` — all keep working. A trailing expression must be
the last item of the body, so a body that wrote its `return` before other members needs
those members moved above the expression.

### A Node/TypeScript client, `@opensysml/client`

`clients/node/` is a second first-class client: `load`, `loads`, `eval`, symbol lookup and
`instantiate` over the Connect protocol with protobuf bodies, with `Value`, `Verdict`,
`Quantity` and feature values modelled as discriminated unions a consumer switches on
exhaustively rather than as generated protobuf shapes. Not published yet.

- **The lifecycle is the Python one.** A connection that names no address starts a private
  child (`-port 0 -report-address -exit-with-parent`), learns its address from the child's
  first stdout line, and shares it across the thread's connections; the client holds the
  write end of the child's stdin and the child exits at end of file, which a test proves by
  `SIGKILL`ing the parent. A service someone else runs is explicit opt-in (an address or
  `OPENSYSML_SERVICE`) and is never stopped by closing the connection.
- **No native addon and no postinstall download.** The service binary comes from a
  per-platform optional dependency (`@opensysml/sysml-grpc-<os>-<cpu>`) selected by npm's
  `os`/`cpu`, falling back to `$OPENSYSML_BINARY`, `~/.opensysml/bin`, `$PATH`, or an
  external service.
- **A browser entry point**, explicit-address only: a browser cannot spawn a service. It
  needs the server's CORS origins and TLS, and does not use the `grpc-web-text` variant
  `connect-go` does not implement.
- The TypeScript stubs are generated by `buf` (`make proto-ts`, included in `make proto`)
  and committed. `conformance/scenarios` runs through the client's public API for the five
  RPCs v1 covers: 69 of 177 protocol-scenarios pass, 108 skip with their reason recorded,
  none fail.

### The Python client uses a private service of its own

**Behavior change.** A `Connection` that names no address no longer attaches to whatever
`sysml-grpc` happens to be listening on port 50051. It starts a **private child** of the
interpreter instead: the child binds port 0, is given a port by the kernel and reports the
address on its stdout, so no port is chosen, probed or retried and two interpreters starting
at once cannot collide. One child serves every connection of the interpreter that needs the
same service release, sharing its parse cache, and is stopped when the last of them closes.

- **Connecting to a service you manage is now explicit**, and unchanged otherwise: pass a
  host and port (`connect("localhost", 50051)` or `connect("localhost:50051")`), set
  `OPENSYSML_SERVICE=host:port`, or pass `auto_start=False`. Such a service is never stopped
  or replaced by the client. What is gone is *implicit* adoption — a script that happened to
  find a service listening now starts its own, which is also what makes it reproducible.
- **A private child cannot be orphaned.** The client holds the write end of a pipe on the
  child's stdin and never writes to it; the child exits at end of file. The kernel closes
  that pipe when the owning process goes away, so the child does not survive `SIGKILL`,
  `os._exit`, a fatal interpreter error, or a crash during shutdown — cases an `atexit` hook
  does not cover. A `fork()`ing parent disowns its inherited services in the child, so the
  service stays with the process that started it. `sysml-grpc` gained `-report-address` and
  `-exit-with-parent` for this, and accepts `-port 0`.
- **The ownership records are gone**: no `~/.opensysml/sysml-grpc-<port>.pid`, no process
  start times to authenticate a pid with, no stale-record cleanup, no lockfile, and no
  port-collision retry. The guarantee they protected is kept by construction — the client
  signals only the `Popen` of the child it started, so no pid it did not start, reused or
  otherwise, can be signalled. `OPENSYSML_STATE_DIR` moved those records and nothing else,
  so it no longer has any effect; the binary is cached in `~/.opensysml/bin` as before.
- `filelock` and `psutil` are no longer runtime dependencies of the client.

### What Flexo MMS does with our RDF is now measured, not argued

An opt-in gate loads a model's Turtle into a running Flexo MMS stack through Layer 1's graph
endpoint, reads it back through the SysML v2 API, and compares that against the same model
posted through that service's own commit path. It records a per-element, per-property report
that a human adjudicates, so the mapping's interoperability is a number that moves rather than
a claim: measured before element ids and ownership were stated, every element of the fixture was
listed and 86 of its 142 properties delivered, against 158 of 158 for the service's own
payloads. The reference mapping documents what the difference is made of.

The gate needs Docker and stays out of `go test ./...`: it skips loudly unless `FLEXO_INTEROP`
is set, and with it set an absent stack fails instead of skipping.
`.agents/skills/flexo-interop` documents the stack, the token and the traps. Nothing about the
RDF encoding changed.

### Connect is the default server transport

`sysml-grpc` now serves gRPC, gRPC-Web and the Connect protocol on one port, so a browser or
a plain `curl` reaches the service without a proxy and without generated code. Existing gRPC
clients — the `opensysml` Python client, `grpcurl`, any generated `grpc-go` stub — reach the
new default unchanged; `-transport grpc` still serves the gRPC-only server for anything that
needs exactly the old surface.

`GET /health` answers on the main port. `-health-port` still binds its second listener and
still works, now with a deprecation warning, and `-health-port 0` turns it off; the default
becomes `0` in a later release. Nothing in this repository polls it — the Python readiness
probe is a gRPC call.

Two browser prerequisites are now configurable rather than absent: `-cors-allowed-origins`
takes exact origins and refuses `*` at startup, and `-tls-cert`/`-tls-key` serve every
protocol over HTTPS on the same port. `application/grpc-web-text` remains unimplemented and
answers 415, which affects no `fetch`-based client.

Protobuf is the body encoding to use: a 468 KB `Query` answer costs ~6.5 ms as protobuf and
~40 ms as JSON, from `protojson` CPU rather than the 9.7% extra bytes. JSON is the debugging
affordance, a large JSON response now logs a warning, and
[reference/service-transports.md](docs/reference/service-transports.md) says so where a client
author will read it. The conformance suite runs every scenario once per protocol so the second
surface cannot rot.

`-transport stdio` stays as a prototype behind its flag: not the default, and not a transport
any published client speaks. The transports reference says so, and the binary's wiring of it is
now covered by a test.

### One dependency fewer

`github.com/fsnotify/fsnotify` is no longer a dependency. It was linked for a filesystem
watcher nothing reached: the language server is told which files changed by its client, and no
command watches a directory. Building from source now resolves one module less.

### Conformance figures for this release

Every figure is generated from the committed baselines and gated, so none of it is typed in by hand.
Against the pinned reference (`2026-05`, artifact `0.60.1`):

- **Corpus agreement:** 328 of 355 files agree diagnostic-by-diagnostic; 27 diagnostics are ours
  alone, 58 the reference's alone. Read by root, our diagnostics against the reference's own corpora
  fell while our notation warnings on our own example models rose — the removed spellings reporting
  as the errors they now are.
- **Declared-diagnostic silence:** of the 510 declared `errors` rows in the reference's Xpect suites,
  none is one we are silent on; 230 of 230 declared scope assertions match exactly.
- **Permissiveness gaps:** of 120 invalid models we authored, 120 both reject, two of them only when
  we are asked strictly. We wrote every case, so the denominator measures our corpus's reach and not
  our conformance.
- **Declared errata:** the registry declares three defects in the published reference material, two
  with a specification-derived correction. The figures above are as published and stay the
  conformance statement; the corrected-text run is reported beside them and is diagnostic only.
- The oracle baselines now record their own provenance — the pin, the validator bridge digests and the
  identities of the corpora compared — checked by tests that need no Java, with the Java-backed
  reproduction on a schedule. All oracles remain advisory: they inform judgment, they do not replace
  it.

### Fixed

The conformance comparer no longer compares integral fields within the tolerance it allows a
`Real`. Every numeric field was normalized to a `float64`, so a relative tolerance of 1e-9
reached counts, ids and spans as well — 1,000,000,000,500 matched 1,000,000,000,000 — and a
whole number above 2^53 could not be represented exactly in the first place. Integral values
now carry an integral type and are compared by their digits, expected numbers are read as
literals rather than floats, and the tolerance applies to `Real` alone.

- **Parallel machines and actions lower once and completely**: no duplicate nested regions; parallel
  action edges and state behaviors preserved; explicit action successions and implied endpoints
  lowered; explicit action starts required; an anonymous nested action final targeted correctly.
- **Parser**: a binary operator survives a parenthesis-less arrow invocation, and a keyword binary
  operator after a name reads as a calculation result.
- **Semantics**: interface flow features are paired before conjugate names are required.
- **Python client**: a failed child's log is read under a lock once drained, unset features read as no
  value in typed views, and calls into the private service are serialized across threads.
- **RDF**: an expression node id whose owner segment starts with `p` reverses unambiguously.
- **Conformance harness**: two distinct ports are reserved, and a service that exits early is
  reported instead of waited on.

### Python client (`opensysml` 0.3.2)

- **The client changes in this release reach PyPI as 0.3.2**, once `opensysml-v0.3.2` is tagged:
  the private service of its own, `MissingCapabilityError` for a service-side refusal, and the
  three client fixes listed under Fixed. Installing the v0.3.0 core does not wait on it — the
  published 0.3.1 takes an unpinned release's digest from the signed `SHA256SUMS.txt` it verifies
  with sigstore — so this is the client's own changes reaching users, not a prerequisite for the
  core release.
- The `v0.3.0` digests are pinned after the core release's assets publish, before
  `opensysml-v0.3.2` is tagged, so that release is verified against a committed digest rather
  than against the manifest path alone.

### Project

- **buf is the single source of protobuf codegen**, replacing ad-hoc `protoc` invocations: Go stubs
  through pinned plugins, Python stubs (including `.pyi` and the package-relative gRPC import)
  through a local plugin bridging to `grpcio-tools`, plus `buf lint` and `buf breaking` against `main`
  in the Makefile and in CI. TypeScript and Java stubs are generated the same way for the clients
  that now use them, with a Rust template defined beside them.
- **A language-independent gRPC conformance suite**: scenarios live as protobuf-JSON data under
  `conformance/`, and `cmd/conformance` builds and starts the service itself, so a client in any
  language can prove it speaks to `sysml-grpc` without re-deriving the Python suite. Every scenario
  runs once per protocol, so the second transport surface cannot rot.
- **`google.golang.org/grpc` is test-only for production code.** Service errors are connect-native
  with identical codes and messages, the `-transport grpc` server is isolated in one file, stdio
  dispatch no longer carries incidental grpc-go types, and a CI gate keeps grpc-go out of the
  production packages while the tests and the conformance runner keep using it. A consumer importing
  the public Go API no longer links a gRPC server to get a parser.
- **An oracle figure quoted outside the generated block must name the round it measured**, since only
  the `doc-counts` block states the current totals: `scripts/check-doc-figures.py` fails on an
  undated one across the differential, Xpect, rejection and execution oracles and runs in CI beside
  the document id and link checks. Pull-request checks are parallelized under standardized names,
  aggregated into one required status, and each client's job runs only when the pull request touches
  it.
- A core developer guide; the transport evaluation and the Connect surface documented on the site;
  a page per client, and the release procedure for each written down before anything is published;
  internal engineering records excluded from what is published; and the guide's transcripts and
  examples rewritten in standard notation against the real binary.

## 0.2.1 — 2026-08-24

A conformance patch, a round of performance work, and a supply-chain improvement. The
rejection oracle against the pinned OMG pilot (`2026-05`, jupyter-sysml-kernel 0.60.1) now
stands at 120 of 120 both-reject under default mode — the three reserved-keyword cases that
previously only the pilot rejected are errors by default — and every implementation-side
divergence the conformance records adjudicated as fixable is closed. Reporting findings on a
large model no longer costs its size times its findings, and loading one is faster and holds
less memory; every conformance oracle is byte-identical before and after. Releases now
publish a cosign-signed checksum manifest, and the Python client verifies it. No API was
removed or renamed.

### Reserved keyword names are errors by default

- **A reserved keyword used as a name is rejected in default mode**, not only under
  `-strict`: a keyword as a declaration name (`part if;`), a keyword behind an alias, and a
  SysML keyword used as a name in KerML. Parser recovery still produces a usable AST and the
  diagnostic still lands on the offending name; strict mode is unchanged, and no other
  notation extension was tightened. This closes the last three pilot-only rejection cases:
  120 of 120 negative cases both-reject, none by the pilot alone.

### Diagnostics the reference declares and 0.2.0 missed

- **A member-leading succession shorthand** (`then x;` opening a member) is diagnosed as
  nonstandard notation instead of accepted in silence.
- **Duplicate state and transition member names are reported.** State members and named
  transitions carried no declaration-name span, so the name-distinguishability rule never saw
  them; two states of one name in one body now warn as any other duplicate does.
- **Feature accessibility over behavioral bodies is corrected**: a shared `accept` payload is
  accessible from sibling nodes, references in state and transition bodies resolve in their
  own scopes rather than the enclosing one, and a body's implicit result expression is
  checked like an explicit one.
- **A textual representation accepts a short name**: `rep <ocl> inOCL language "ocl" /* … */`
  parses like the named and anonymous forms; contents remain unevaluated.
- **A transition guard that is not Boolean, and a subsetting whose kinds cannot relate, are
  reported even when the file has an unrelated syntax error.** Both checks now gate
  themselves per element rather than per document, so an error elsewhere in the file no
  longer silences them; an element whose own declaration failed to parse stays quiet.
- **A non-Boolean element filter reports the reference's rule by name**: a filter condition
  whose result cannot be Boolean is diagnosed as `Must have a Boolean result`, matching the
  pinned validator word for word, beside the model-level-evaluability requirement it already
  reported. Constant-folded filters such as `filter 1 + 2 * 3 > 0;` stay accepted.
- **Feature accessibility is checked inside element-filter conditions** — the `filter`
  member and the `[...]` clause of an import — with the candidate element as the featuring
  context: a library-declared referent passes, a feature of a user-declared type is
  reported, matching the pinned validator's boundary on both sides.
- **A chain-shaped filter condition is diagnosed by the rule it breaks**, read off the
  compiled predicate's resolved result type rather than the expression's shape: a condition
  the specification forbids reports `Must have a Boolean result` or `Must be model-level
  evaluable` as the reference does, while a condition only our evaluator cannot compute
  becomes a non-blocking warning — which also lets the accessibility check above speak on
  chains it previously never reached.

### Conformance records match the implementation

- The divergence follow-up rows that lagged earlier fixes are closed against live runs of the
  pinned validators, with pinning tests and fixtures added where coverage was missing — among
  them a cold/warm library-cache test proving a cached library symbol keeps its declaration,
  metamodel type and abstractness. Refreshed baselines: differential 353 files, 324 fully
  agreeing (84 diagnostics only ours, 65 only the pilot's); Xpect 1,295 of 1,323 rows agreeing
  (248 wording-only), 28 disagreeing. All oracles remain advisory.
- The semantic-rule map is re-measured for this release: of 727 tracked rules, 646 are
  faithful, 74 approximate, 1 not implemented and 6 deliberately divergent, each divergence
  named in its row. The roadmap reflects this release's targets and gate counts.

### Validating, loading and rolling back cost what they should

- **A source file's line index is built once and reused.** Locating a finding rebuilt the
  whole-file index on every call, so validating a model that reports something cost the
  file's size times the number of findings: a 16 000-element model warning on every usage
  took 123 s and allocated 258 GiB, and now takes 2.1 s and allocates 1.4 GiB. No diagnostic,
  position or message changed.
- **A failed creation rolls back from a log instead of a snapshot.** Every object
  materialization copied the live-instance set beforehand — 81% of everything allocated
  while instantiating; the context now records each registration and a rollback walks only
  what the failed creation added (156.6 KiB to 2.8 KiB per 250-element instantiation).
- **Loading a model does each traversal once.** Validation shared one ordered symbol
  traversal per analysis instead of re-collecting it per pass, direct-child lookups are
  cached with generation-based invalidation, redefinition closures are computed once and
  reused, and scope members are iterated in place instead of through copied slices. The
  lexer's token buffer is pre-sized from the source length.
- **A scope stores its members flat and indexes them lazily.** Most scopes never hold a
  named member, and most that do hold a handful, so the per-scope map is gone: names and
  symbols live in two declaration-ordered slices, scanned up to twelve names and indexed on
  demand above that. Loading holds ~7% less live heap; a whole-binary validation of a
  12 000-element model runs ~5% faster at lower peak memory.
- **Member iteration is deterministic.** Scope members, FQN registration and duplicate
  declarations now follow declaration order everywhere Go map order previously leaked
  through; for duplicate declarations the first declared wins, pinned by tests. No
  diagnostic, position, message or execution result changed anywhere in this section —
  the differential, Xpect and rejection oracles are byte-identical before and after.

### The release manifest is signed

- **`build-release` signs `dist/SHA256SUMS.txt`** with cosign keyless using the release
  pipeline's OIDC identity and publishes the sigstore bundle beside it as
  `SHA256SUMS.txt.bundle`. Nothing changes for a caller downloading binaries directly.

### Python client (`opensysml` 0.3.1)

- **A core release published after this client can now be installed.** Previously every
  installable core release needed a digest pinned in this client, so a new core release
  required a new PyPI release. Now, for a release with no pin, the client downloads the
  release's `SHA256SUMS.txt.bundle`, verifies it against the release pipeline's identity with
  sigstore, and takes the asset digest from the verified manifest. Pins stay authoritative:
  where one exists it wins, and a verified manifest that disagrees with a pin is reported as
  a mismatch, never used as a downgrade.
- **`sigstore` is an optional dependency, imported on demand.** An install without it refuses
  to verify an unpinned release — exactly the previous behavior — rather than failing to
  import `opensysml`.
- The `v0.2.1` digests are pinned after the core release's assets publish, before
  `opensysml-v0.3.1` is tagged.

### Project

- The documentation site is published at [opensysml.org](https://opensysml.org/); the old
  GitHub Pages address redirects there.
- GitHub issue forms and a pull request template.
- Reader-facing documentation no longer uses internal work-item labels, and
  `scripts/check-doc-ids.py` keeps it that way.

## 0.2.0 — 2026-08-24

Three more advisory oracles now judge this implementation against the pinned OMG pilot
(`2026-05`, jupyter-sysml-kernel 0.60.1), and most of the behavior below is what they found: the
expectations the pilot's own Xpect suites *declare*, the invalid models it rejects, and the part of
its surface that can referee execution at all — which is expression evaluation and nothing else.
Notation and validation rules moved with them: 324 of 353 differential files now agree
diagnostic-by-diagnostic (was 221 of 338), and of the 510 declared `errors` rows in the pilot's own
suites there is none we are silent on. Name resolution now enforces membership visibility, so a
model that named a `private` or `protected` member across a namespace boundary and analyzed clean in
0.1.2 is now reported. A new opt-in strict mode raises this implementation's own notation extensions
to errors; default behavior is unchanged. The standard library is loaded once per process and
shared, which takes a 100-model gRPC cache from 1598.3 MiB of retained heap to 1.1 MiB. The gRPC and
Python surfaces gain fields and calls; nothing was removed or renamed.

### Three new oracles, and one that says what it cannot judge

- **`cmd/pilot-xpect` adjudicates the expectations the pilot declares**, not its observed behavior:
  the 428 `.xt` files it ships in `org.omg.kerml.xpect.tests` (303) and `org.omg.sysml.xpect.tests`
  (125), read through the same `scripts/pilot-pin.sh` pin as every other corpus, over six assertion
  kinds (`errors`, `noErrors`, `linkedName`, `warnings`, `scope`, `exportedObjects`). The committed
  baseline (`docs/project/pilot-xpect-baseline.json`) stands at 1,323 rows from 1,261 assertions:
  1,295 agree — 248 of them wording-only — and 28 disagree, with 0 files unparsed, 0 rows unlocated
  and 0 not adjudicated. Declared scope assertions agree 230 of 230 and declared linked names 194 of
  194.
- **`cmd/pilot-reject` asks the reverse question every other oracle cannot**: does this
  implementation reject what the reference rejects? Both validators run over a negative corpus we
  wrote ourselves, grown from 34 cases to 120. 117 agree, 3 are rejected by the pilot alone, and
  none by us alone; 5 of the 117 agree only when we are asked strictly, and by default 8 of the 120
  are ours-accepted. The denominator is our own authorship, so it measures the reach of the corpus
  rather than conformance, and agreement reached only under strict mode is recorded as the weaker
  evidence it is.
- **`cmd/pilot-exec-diff` maps the pilot's execution surface before comparing anything**: there is
  no interpreter, simulator, scheduler, token or trace in the pinned artifact, so of the four
  behavior areas asked about, model-level expression evaluation is adjudicable and actions, state
  machines and classifier behaviors are out of reach. 125 of the tracked rules therefore have no
  external referee at all, and the figure is now stated wherever behavior compliance is claimed.
- **The differential covers 353 files in seven roots** (the OMG training corpus, the pilot's SysML
  example, SysML validation and KerML example corpora, our testdata, examples and probes): 324 agree
  exactly, 83 diagnostics are ours alone and 66 the reference's alone, from 139 and 122 diagnostics
  respectively. Read by root rather than in total — our diagnostics on the reference's own corpora
  fell while our non-standard-notation warnings on our own example models rose.
- **All four harnesses stay advisory.** Nothing in CI depends on a comparison with the pilot, whose
  Java validators CI does not provision; what CI gates is our own verdicts on the pinned files
  (below).

### An opt-in strict conformance mode

- **`sysml -strict`, `%strict on|off` in the REPL, `strict_conformance` on `ParseFileRequest` and
  `strict_conformance=True` from Python** judge the source as conforming SysML v2: notation this
  implementation accepts that no pinned OMG production admits becomes an error instead of a warning.
  It needed no second grammar and no edit to any model, example or golden — the mode raises the
  severity of the existing `nonstandard-notation` finding and nothing else. Strictness is part of
  the analysis cache key, so the two modes never serve each other's diagnostics, and
  `internal/core/conformance` holds the mode so no pass decides it locally.

### Name resolution follows the specification's two resolution rules

- **Membership visibility is enforced.** A qualified name, a feature chain link and an import all
  consulted a membership without consulting its visibility, so `A::X` resolved even where `X` is
  private in `A`. One predicate now applies the rule on all three routes and to the LSP surface:
  the 70 declared `Couldn't resolve reference to …` rows we were silent on fall to 4.
- **A qualified name's first segment resolves locally and every later segment visibly**, per KerML
  8.2.3.5.3–8.2.3.5.4, which is the distinction the pilot's `VisibilityTests_ProtectedImport_*`
  fixtures separate: a specializing namespace reaches a protected member by simple name, but what
  the *referring* namespace specializes cannot widen what a later segment sees. Generalization
  headers resolve outside the declaration body, qualified redefinition tails walk direct supertypes
  through a speculative probe that emits no diagnostic when it fails, and unqualified lookup
  distinguishes an inherited import from a direct nested one.
- **A supertype's non-private *imported* memberships are inherited** as its owned ones are (KerML
  8.3.3.1 with 8.4.3.2), so redeclaring a name a supertype imported is the distinguishability
  violation the reference declares.
- **Two root namespaces of one name are one global name, not an ambiguity** — resolution in the
  global namespace is single-valued (KerML 8.2.3.5) and distinguishable naming constrains a
  namespace's own members, which the global namespace is not.
- **The reference's name-distinguishability rule is implemented with its own scope and severity** —
  a warning, not an error — over owned, alias, inherited and diamond-inherited conflicts, with an
  explicit `:>>` redefinition suppressing the inherited-name conflict and a plain redeclaration,
  reference or subsetting not.
- **Derived scope paths are bounded** by one re-entry per name, count an inherited import as a
  derivation step, and inherit through a feature's declared type before its implicit base.
- **A redefined name is masked** from member enumeration, and the mask is built once per enumeration
  rather than per member.

### The validation rules the reference declares

- **Thirty-nine new level-scoped passes in `internal/core/passes`** implement rules the pilot's own
  Xpect suites declare and this implementation reported nowhere, each scoped from the constraint in
  `KerMLValidator.xtend` / `SysMLValidator.xtend` rather than from its message string — type
  unioning/intersecting/differencing and feature chaining, multiplicity bound types, reference
  subsetting, top-level import visibility, association end types, occurrence typing, connector
  featuring, flow ends, variability, implicit base and "features must have at least one type",
  conjugated specifics, and a portion that cannot be variable. Of the 510 declared `errors` rows,
  the number we report nothing for is 0; 243 match word for word and the rest differ only in wording
  or location.
- **Rules that fire where the reference warns now warn**: library inherited-name diamonds, short
  names, a user-declared standard library, non-conforming bindings, and a computed return.
- **Validation tier gating asks about the element, not the document.** A blocking error used to skip
  every higher-tier pass for the whole file; `passes.ElementScoped` plus one query on `Context` now
  gates per subject, so a valid declaration is still checked when an unrelated one failed to
  resolve.
- **Twenty-seven differential diagnostics we reported on models the pilot accepts are retired**, two
  of them parse/resolve defects rather than rule defects, with a key-by-key check that no row was
  added and no category re-bucketed. All 137 diagnostics the pilot reports and we did not are
  classified — our defect, adjudicated divergence, or a defect of the pilot — with a named reproducer
  per family.
- **The Step 3 conformance obligations land**: `UsageMayTimeVary` derived from occurrence ownership,
  assignment referents validated in an element-scoped pass, and `BinaryInterface` / `BinaryConnection`
  inferred only for exactly two-ended untyped declarations rather than by a universal arity limit.

### Notation the reference accepts now parses

- **The KerML declaration grammar cluster is closed**: a member with no kind keyword is a
  feature wherever a member is expected, decided by lookahead over the declaration head rather than
  by a keyword table, so `a : Integer;`, `x;`, `p5[1] : Real;` and `composite e1 redefines V::m;`
  parse. The pilot's KerML example corpus now reports no syntax diagnostic of ours at all.
- **Reserved words are read per grammar.** The lexer keeps one token set, but reservation is now the
  parser's decision from the kind of the file being read, so `part chains : T;` is legal in a
  `.sysml` file where `chains` is a word only `KerML.xtext` spells.
- **Index sequences, multiplicity subsetting, `..` recovery and exhibit references parse**, and every
  remaining parser-only divergence in the pilot corpora was probed against the pinned grammars and
  accepted only where a production derives it — three neighboring forms are not derivable and stay
  rejected, with the finding recorded per row. The 28 syntax diagnostics that remain in the
  differential are all this implementation's own registered extension warnings in `examples/`.

### Metadata annotations

- **A metadata annotation body has a scope and its names resolve.** Per KerML 7.4.7 and 8.3.3.3 a
  declaration in the body implicitly redefines a feature of the metadata definition, so in
  `@A { x = ~3; }` the name `x` is `A::x` while the value `~3` resolves in the enclosing scope chain.
  Nested `@Safety` / `@Security` annotations resolve through public namespace and membership
  re-exports.
- **Model-level evaluability is an explicit walk over the expression** (`Model.ModelLevelEvaluable`)
  rather than a by-product of filter compilation: literals, `null`, metadata access, sequences,
  `new T(…)` with evaluable arguments, invocations of the Kernel Function Library functions the model
  itself evaluates, and reads of features reaching an evaluable value are evaluable — being declared
  by the normative library is not the criterion, so `RealFunctions::sqrt(4.0)` is correctly rejected.
- **Metaclass reflection is answered from the element**, and a keyword-first relationship is
  classified by its own metaclass and is a first-class element with ordered ends.

### Execution

- **`send x via p to r` routes instead of reporting unsupported.** Lowering was dropping the
  receiver, so the runtime had nothing to route with; it now carries port, receiver name and sending
  object losslessly, with qualified and shadowed receiver names resolved.
- **A simple state transitioning to itself exits and re-enters.** `transition s to s` ran the effect
  and stayed put — no exit action, no entry action — because the enclosure test answered that the
  transition never left its source.
- **A name denoting one object evaluates to that object**, a vector's elements are its sequence, and
  a merge node's body runs with the traversal that wins rather than on every arrival.
- **Declarations in expression bodies are evaluated and scoped**, calc/constraint operations are
  invoked with the performer context preserved, value type classification is implemented, and
  executor budget errors are typed (`ErrInvalidActionFlow`, `ErrNoEnabledSuccession`,
  `ErrActionDeadlock`) so a malformed flow, an unenabled decision and a deadlock are distinguishable
  rather than one opaque failure.

### The standard library

- **One frozen library index is shared by every model**, with each model's documents in an overlay
  over it. Measured with gRPC at its default `--cache-size 100` over 100 distinct library-backed
  models: retained heap 1598.3 MiB → 1.1 MiB, RSS 2180.3 MiB → 76.5 MiB, library indexes built
  104 → 1. Four REPL sessions go from 122.7 MiB to 17.1 MiB. `Index.Freeze()` makes every write-like
  method fail loudly and `symbols.NewOverlay(base)` refuses a non-frozen or already-stacked base;
  evicting a model tombstones a base-owned document locally instead of deleting what another model
  still reads.
- **A cache hit can no longer produce a poorer semantic state than a miss.** Records held FQN-level
  symbols only, so with a warm cache a library type had no members, declared values or condition
  ASTs — the same commit diverged cold vs warm in user-visible ways (`internal/core/solve` failing
  cold, ~60 inherited library attributes reported instead of 5, unresolved-reference errors on a
  filtered library facade). The library is parsed on every path and the on-disk cache persists only
  derived facts, with a reflective test that fails both ways: a persisted field with no comparator,
  and a comparator for a field no longer persisted. The cache is keyed by build, so a semantics
  change is a miss rather than a stale hit.
- **Library provenance is asked of the index, not inferred.** Four consumers decided a member was
  not the model's own from the accident that a library symbol carries no declaration; they now ask
  `Index.Library`, so a user's own imported library is library content when the index says so and a
  model package named `Occurrences` is not.
- **`ApplyEdits` takes one library index per request**, not one per edit operation plus validation's
  — a 10-operation request built 11.

### Authoring, query and export surfaces

- **Elements can be created and deleted from Python** as source-preserving edits
  (`AddMemberEdit`, `DeleteEdit` over gRPC): every edit is a byte splice into the loaded source, the
  result is reparsed and reanalyzed, and the whole batch is refused if it introduces errors the
  original did not have. Five typed failures are added — `EDIT_FAILURE_OWNER_UNKNOWN`,
  `EDIT_FAILURE_OWNER_NOT_NAMESPACE`, `EDIT_FAILURE_ILLEGAL_KIND`, `EDIT_FAILURE_MEMBER_NAME_TAKEN`,
  `EDIT_FAILURE_DELETE_REFERENCED` — each with a Python exception. `loads()` parses inline content,
  and `ParseFileRequest.language` selects `sysml` or `kerml` for it.
- **Renaming a declaration rewrites the references to it**, and an alias segment is renamed as the
  name it wrote rather than as the element it reaches.
- **OSLC Query 3.0 text is a second query front end** beside the structured SysML v2 API `Query`,
  reachable as `sysml -query 'oslc.where=…'`, `%query` in the REPL and `QueryRequest.oslc_query`.
  Neither surface subsumes the other: structured queries keep `and`/`or` constraint trees, OSLC
  brings `!=`, `<=`, `>=`, `in [...]` and `oslc.orderBy`. The CLI and REPL front ends carry OSLC
  text only; a structured query stays on `QueryRequest.query`. This is not an OSLC server — no query
  capability documents, result containers, service providers or resource shapes.
- **RDF carries expression trees beside the source text** of an expression, states the features a
  binding head relates as structure, keeps a keyword-first relationship's declared visibility, and
  carries a multiplicity's subsetting through conversion.
- **`SymbolInfo.withheld_library_attributes` states what a projection withheld** instead of
  withholding it silently, and two symbol-projection defects are fixed.
- **The REPL parses each snippet as the kind of the file it came from**, defers a load-time notation
  error to the analysis report instead of vetoing the run, and no longer hides what a submission
  declared behind that error. Its prompt scope falls back to the root holding the last declaration.

### Editor surface

- **Contextual keywords are highlighted and completed.** Words the parser reads positionally —
  `chain`, `choice`, `decision`, `deep`, `defer`, `done`, `point`, `region`, `var` and the rest —
  were in neither the TextMate grammars nor LSP completion, because both derived their word list
  from the reserved-word table those words are deliberately absent from. One exported source of
  truth per language now feeds both surfaces, with lexing untouched.
- **The editor is no longer blind inside a metadata annotation body**: hover names the redefined
  feature and its type, go-to-definition jumps to it including an inherited one, completion offers
  the metadata definition's features at a declaration position and the enclosing scope's at a value
  position, and document/workspace symbols and semantic tokens cover the body.
- **Rename and find-references follow the name an alias segment wrote**, so an editor rename
  (<kbd>F2</kbd>) on an alias rewrites its uses, and the same rename on the target no longer
  rewrites a name that was never the target's.

### Declared errata: the published reference material can itself be wrong

- **`internal/errata` is a registry of declared defects in published reference material**, and all
  three corpus oracles now report every census twice — as published, which stays the conformance
  statement, and with the errata applied as a secondary diagnostic. The registry declares 2 defects,
  1 with a specification-derived correction and 1 documented without one because no intended reading
  can be inferred. The published corpus is never edited: a correction is applied to a materialized
  copy under the oracle's own gitignored output directory, and an erratum never reclassifies a
  divergence category.

### Measurement infrastructure

- **The four pinned OMG corpus gates share one mechanism** and keep their two deliberate policies:
  the training corpus is asserted clean, the three pilot corpora are a per-file ratchet whose every
  movement must be adjudicated. The three pilot corpora (212 files) were report-only before this and
  failed nothing in CI; what is gated is our own verdicts on those files, which need no Java
  validator and run in pure Go.
- **The refereed figures in `README.md` and `docs/internals/architecture.md` are generated from the
  committed baselines** by `make docs-counts`, which previously only checked a hand-maintained block.
  No number is typed in by hand, and `cmd/doc-counts -check` makes divergence a build failure.
- **The `~98% of targeted features` claim is gone.** `docs/project/spec-compliance.md` is a census of
  our own row list — 727 tracked semantic rules, 641 faithful, 75 approximate, 5 not implemented, 6
  deliberate divergence — stated as bookkeeping that moves when rows are rewritten and not when an
  oracle does. No percentage of the specification is claimed anywhere.

### What these numbers do not show

The OMG corpora are demonstrations rather than an official conformance suite; the differential is
one-directional; the Xpect suites are the pilot authors' test intent rather than a certification
oracle; the rejection corpus is our own authorship; and the pinned artifact evaluates expressions but
executes neither actions nor state machines, so the 125 behavior rows that carry the action,
state-machine and classifier-behavior semantics are self-assessed. 28 declared Xpect rows and 83
differential diagnostics remain, each adjudicated in `docs/project/pilot-xpect.md` and
`docs/project/pilot-differential.md` rather than left to be discovered.

## 0.1.2 — 2026-08-20

A measurement release. Two advisory harnesses now ask the OMG pilot implementation and the
pinned OMG grammars where this implementation differs from them, and most of the behavior below
is what they found — chiefly in KerML, which the pilot's own KerML validator now judges. A
`.kerml` file is read as KerML rather than as SysML written with other keywords, so a KerML
model 0.1.1 rejected may analyze clean here; notation accepted beyond the grammars now warns
where it used to be silent. Nothing was renamed and no interface changed.

### The pilot implementation judges every corpus, KerML included

- **`cmd/pilot-diff` compares our diagnostics against the pinned OMG pilot implementation**
  (`2026-05`, jupyter-sysml-kernel 0.60.1) file by file, over 349 files in seven roots: the OMG
  training corpus, the pilot's own SysML example and validation corpora, its KerML examples, our
  testdata, our examples and our probes. 254 files agree exactly. It is advisory — a harness that
  finds work, not a gate — so nothing in CI depends on it, and its committed baseline
  (`docs/project/pilot-differential-baseline.json`) makes a rerun a diff rather than a reading.
  On the 100-file training corpus both implementations report nothing at all.
- **The KerML corpus is judged by the pilot's own KerML validator**, not by our reading of the
  KerML specification: `scripts/pilot-kerml-validator/ValidateKerML.java` registers the pilot's
  `KerMLStandaloneSetup`, loads `sysml.library` and the corpus into one EMF `ResourceSet`, and
  asks the pilot's `IResourceValidator` with `CheckMode.ALL`. It contributes no rule of its own,
  so a disagreement it prints is the reference's verdict. Over the pilot's 58 KerML examples, the
  diagnostics only we reported fell from 439 to 98 as the work below landed — the syntax class
  from 360 to 85, and the genuine name-resolution class from 123 to 10 — and 35 of the 58 files
  now agree exactly. Ten of that root's remaining diagnostics are name resolution rather than
  seven: parsing notation that used to be rejected exposes references behind it, so the class
  rose by three as the parser improved.
- **Six diagnostics the pilot reports and we do not are a defect in the pilot**, not a gap here:
  EMF's unpaired-bidirectional-reference check firing on the pilot's own
  `Type::ownedDisjoining` / `Disjoining::owningType` opposite, reproducible from three lines
  (`classifier A; classifier B disjoint from A;`) in a fresh resource set. It is recorded with
  its reproducer rather than absorbed into our own numbers.
- **The harness picks the reference validator per file**, by the file's own language rather than
  per corpus, so a directory holding both `.sysml` and `.kerml` is judged by the SysML validator
  and the KerML one respectively, and a missing KerML validator is reported rather than silently
  skipped.
- **The 373 diagnostics only we reported on the two OMG SysML corpora were adjudicated construct
  by construct**, into ten classes recorded in `docs/project/spec-compliance.md` with the grammar
  production each one cites (`ExtendedUsage`, keyword-less members, node bodies, `return`,
  `binding`/`message`/`event` declarations, requirement members, resolution through imports and
  inheritance, and textual representation). Six of the ten are wholly or partly fixed below,
  taking those two corpora to 204; the rest name the site that rejects the notation, so the work
  is scoped rather than discovered. Where a newly-parsed declaration reaches the type tier for the
  first time it may report there instead, which is why one narrower class rises as the parser
  gains ground — those are unmasked diagnostics, adjudicated per file, not new false positives.
- **`cmd/grammar-coverage` measures the pinned OMG grammars against every corpus we hold**:
  483 of 727 productions and 802 of 807 notational forms have input-presence evidence. The
  number is deliberately an over-approximation — presence of an input a production admits, never
  parser-execution coverage or compliance — so the page's useful reading is the five forms with
  no evidence anywhere, each adjudicated: the `%` remainder operator and prefix metadata on a
  namespace are implemented but exercised by nothing, and the named `disjoining`, `conjugation`
  and `redefinition` relationship declarations are not implemented.

### A `.kerml` file is analyzed as KerML

- **A KerML declaration specializes the library type its keyword implies**, so the features of
  that library type are inherited: `class` reaches `Occurrences::Occurrence`, `struct` reaches
  `Objects::Object`, and so on through `assoc`, `behavior`, `function`, `interaction`,
  `metaclass`, `datatype`, `classifier` and `type`. No library member was inherited before, so
  `portion focusedState : Camera subsets timeSlices;` reported an unresolved reference to a
  feature the library declares. A declared generalization suppresses the implicit base only when
  it already reaches it, so `struct MyWheel specializes Wheel` still reaches `Objects::Object`,
  and a supertype restored from the index cache keeps those edges. A bare `feature` still gets
  no base: the SysML attribute base is not KerML's.
- **SysML's definition-and-usage checks no longer fire on KerML declarations.** `class Person
  specializes Object` was reported as "only a definition may specialize; found a usage" — a
  distinction KerML does not draw. A `.kerml` specialization or typing now requires only that
  its target be a type, from an explicit list of what a type is rather than a guess about what
  isn't, so `metaclass AtomMetadata specializes Metaobject` is accepted while a non-type target
  is still reported. The SysML files keep the kind checks they had.
- **A union conforms through its unioning types** — `classifier MyWheel unions MyWheel1,
  MyWheel2` — without unioning becoming a generalization.
- **A declaration's header sees its own body.** Names in a `featured by`, `crosses` or
  subsetting clause resolve against the members and imports of the body of the same declaration
  before the enclosing scope, and stay reachable afterwards by qualified name and feature chain.
  Resolution of a member inherited through an implicit base no longer recurses.
- **An unknown subsetting target is tolerated** rather than reported as a KerML type error about
  something else.
- **The REPL reads each snippet as the kind of file it came from**, so a `.kerml` snippet gets
  KerML's contextual names and a `.sysml` snippet keeps SysML's reservations, and a session mixing
  the two keeps each snippet's language rather than analyzing everything as SysML.
- **Inline content keeps the language it was submitted with** through parsing, analysis and type
  checking, so a service request that hands over `namespace N;` as KerML is judged by KerML's
  rules for the whole pipeline rather than only at the parse step.

### KerML notation the reference accepts now parses

- **`featured by`, an n-ary connector end list, and a typed or redefining succession parse**:
  `class Owner { member feature inCart : Product featured by Account; }`,
  `connector c : A (a, b, c);` and `succession s : Link [1] first paint then dry;` — whose own
  `[1]` belongs to the succession rather than to its first end — along with the `succession
  redefines s : …` spelling. A missing `by`, a missing target, a missing `then` and a trailing
  comma are still reported.
- **`at`, `while`, `merge` and `decide` are names in a `.kerml` file**, where they are not KerML
  keywords, as `about`, `bind` and the other SysML-only words already were. The remaining KerML
  feature-prefix forms — `abstract var feature x [0..*];` — are still not parsed and are recorded
  as such.

### Imports and visibility

- **A `public` import is re-exported to importers of the importing namespace**, transitively;
  before, re-export stopped after one namespace. A root-level import is visible from a nested
  package, an imported name may prefix a qualified name, and an import cycle terminates instead
  of recursing. A `protected` import is reachable through a specialization of the importing
  namespace and from nowhere else.
- **An import with no visibility indicator warns.** The grammar requires one, so `import Lib::*;`
  now reports `[syntax/import-visibility] import without a visibility indicator: SysML v2
  requires public, private or protected before 'import'`. It is a warning at the syntax tier and
  analysis continues through it; `expose` is exempt, its grammar supplying protected visibility
  implicitly.

### SysML notation the parser was refusing

- **A connector, interface or flow written with shorthand ends may have a body**:
  `connect x.p to r.p { ... }`, `interface b1.p to b2.p { ... }` and `flow s1.x to s2.x { ... }`
  parse with their members. An unclosed body, an interface without `to` and an unterminated flow
  body are still reported.
- **An accept node is an action statement**, so `then action engineStopped accept engineOff :
  EngineOff;` parses and executes. An accept in a loop or branch body remains unsupported by
  lowering and is reported when reached, rather than accepted and silently skipped.
- **`connect` requires its ends and reads a leading multiplicity on each of them**:
  `connect [1] a to [1] b`. `connect;`, `connect { ... }` and a missing target are reported where
  some were accepted and misrepresented.
- **Eleven words that no grammar production reserves are names again**, `on` and `var` among
  them, so `state on;`, `part on : On;` and `attribute var : ScalarValues::Integer;` parse as
  declarations named `on` and `var`. Each word remains a keyword in the position its grammar
  gives it, so `var a : Integer;` without a kind is still reported.
- **A modifier before a kind prefix keeps the kind**, which was dropped from the tree and from
  every diagnostic that read it.
- **Prefix metadata may stand in for a member's keyword**, as the grammar allows, so `#Classified
  connect a to b;` declares the connection, a prefix may follow a modifier (`abstract #Classified
  z;`) or `end`, and it composes with redefinition (`#service :>> sd : PD;`). Some accepted prefix
  forms still reach the type tier with the wrong usage kind and report a kind mismatch, recorded
  as an approximation rather than closed.
- **A member may be written with no keyword at all**: `T1 = 10.0;`, a typing-only member
  (`kpl : D = km;`), an enum value list (`= 60.0; = 70.0;`), a `locale "en_US"` body, and a
  bare result expression as the last member of an analysis or case. An assignment is still an
  assignment rather than a declaration, and an empty assignment, a malformed specialization-only
  member, a malformed enum value and an incomplete `locale` are still reported.
- **A case or analysis body's result expression may begin with a keyword or a word operator**, so
  `if v.m > 1.0 ? v.m else 1.0` and `small and large` are read as the body's result instead of
  being parsed as another member declaration.
- **Every node production's optional body parses, lowers and executes**: a transition, send or
  accept node and every control node may carry `{ ... }`, a transition target may be qualified
  (`then done.stop`), a `for` variable may be typed and a body parameter may redefine. A merge
  node's body runs with the traversal that wins rather than on every arrival. Three corpus forms
  stay rejected and are recorded as such: `exhibit vehicleStates.on { ... }`, a bare
  `ref patient { ... }`, and `send x via p to r`, which parses but is reported as not executable.

### Notation accepted beyond the grammars now says so

- **A construct we accept that no pinned production admits warns** at the syntax tier, from the
  conformance audit: `namespace`, `region`, `choice`, `junction`, an entry/exit/history point,
  `defer`, the `initial`/`final`/`decision` node spellings, and `transition <source> to
  <target>;`. `namespace P { }` in a `.sysml` file reports "`namespace` is KerML notation: the
  SysML v2 grammar has no namespace declaration, so write `package` here or move the declaration
  to a .kerml file"; `featured by` in a `.sysml` file reports the same for the featuring clause.
  The same notation in a `.kerml` file is silent, because there it is standard. These are
  warnings — the models that use this notation still parse, and no higher tier is gated — and the
  REPL warns for its own buffer, which it reads as SysML.

### Analysis corrections

- **An alias is followed through every type relationship** — specialization, typing, subsetting,
  redefinition, multiplicity and invocation inference — so `part def AvionicsLRU :> Box`, where
  `Box` aliases `RectangularCuboid`, no longer reports "part cannot specialize alias (kind
  mismatch)" and inherits what the aliased definition declares. An alias cycle terminates.
- **An invocation of an aliased action or function is type-checked** against its parameters
  instead of going unchecked.
- **An unqualified name resolves through imports and inheritance as a written reference does**, so
  a name a namespace acquired *by* an import is visible to a wildcard import of it, a feature
  reachable by feature chain may be subset, and a redefinition may introduce the type its own
  members are looked up through (`item :>> shape : Box [1] { ... }`). A private import still
  re-exports nothing.
- **A usage may be typed by anything its kind's taxonomy admits** — a part by any occurrence
  definition, a use case by a use-case definition, a succession by what the reference's own rule
  allows — where a narrower table reported a kind mismatch on models the reference accepts.
  `action d : OccurrenceFunctions::destroy` is still rejected: the cached library symbol for a
  KerML `function` is recorded as a calc usage, which is a different layer.
- **A declaration with a short name is listed once** among a document's members, not twice.
- **A part typing check that read an unrelated declaration is gone**, with the diagnostics it
  produced on valid models.

### Source-preserving edits over gRPC

- **`ApplyEdits` adds a member and deletes a declaration**, alongside the value-set and rename it
  already offered, and preserves the source it did not touch — comments, blank lines and layout
  survive, and the result is reparsed and reanalyzed rather than trusted. An edit that would break
  a reference is refused with a typed failure and no content: `EDIT_FAILURE_RENAME_REFERENCED` for
  renaming a referenced element, `EDIT_FAILURE_DELETE_REFERENCED` for deleting one without asking
  for a cascade. The client wrappers for these calls ship on the Python client's own tag.

### Diagrams in the VS Code extension

- **`SysML: Open Diagram` renders the open model in a panel** and keeps it current as the file
  changes, over three new LSP requests — `opensysml/render`, `opensysml/views` and
  `opensysml/renderChanged` — documented in `docs/reference/lsp.md`. The command is gated on the
  server capability `experimental.openSysmlRender`, and the panel is read-only and makes no
  network request.
- **Go-to-definition locates the identifier of a rendered element**, not only its declaration.

### Four pilot rules this release does not implement

Recorded in `docs/project/spec-compliance.md` with the divergence each one produces, rather than
left to be discovered: featuring-type access on a subsetting (`validateSubsettingFeaturingTypes`),
flow-end subsetting (`validateFlowEndSubsetting`, so `flow of Fuel from tank to thruster;` is
accepted here and rejected by the pilot), invocation instantiated type
(`validateInvocationExpressionInstantiatedType`, so `part w = Widget();` on a `part def` is
reported by the pilot and by nothing here), and model-level evaluability of a filter, which
diverges in both directions.

## 0.1.1 — 2026-08-19

A fix release: every change below corrects something 0.1.0 got wrong about a valid model, so a
model that 0.1.0 rejected or misread may analyze differently here. Nothing was renamed and no
interface changed.

### The OMG training corpus reports no errors, and two of its files were never buggy

- **Every definition body inherits the features of the library definition its kind implies.**
  Only behavior definitions had an implicit base, so `snapshot sale = start;` inside a `part def`
  reported `unresolved reference: start` even though `Items::Item` declares `start` and `done` and
  `Parts::Part` redefines both. The verdict recorded against `Time Slice and Snapshot Example` and
  `Individuals and Time Slices` — "bugs in the OMG files" — was wrong; both are clean now, and the
  corpus baseline lists no files.
- **Because those features are now inherited, a member that reuses one of their names is
  reported** where it used to shadow silently: `part def C { part start; }` conflicts with
  `Parts::Part::start`. Redefine it to keep the name — `part start :>> Parts::Part::start;` — which
  is what the model means.
- **A qualified redefinition of an inherited library feature is accepted:**
  `snapshot start :>> Parts::Part::start;` reported "start is not an inherited member of C" because
  a library supertype restored from the index cache carries no scope to compare against.
  Redefining a feature the owner does not inherit is still reported.
- **A metadata usage ends at its own `;` or body:** `@M part def Car;` was read as an annotation
  plus a declaration with no diagnostic. `#M part def Car;` is the prefix spelling.
- **A definition may specialize a definition of a comparable kind:** `individual item def Alice :>
  Person` was refused as a kind mismatch because `Person` is a `part def`. A part definition *is*
  an item definition, so specialization follows the definition taxonomy rather than an exact kind
  match; disjoint kinds — a part definition and an attribute definition — are still refused.
- **A transition may leave the entry action of the state that declares it:** `entry action begin
  { } transition begin then off;` reported the action as "not a state or pseudostate". The entry
  action stands in for a start pseudostate, so the transition designates the state the machine
  starts in rather than an edge between two vertices, and it executes as such. Only that bare
  completion shape designates a start: an ordinary action named as an endpoint, a transition *into*
  an entry action, and a triggered or guarded one out of it are all reported, rather than accepted
  with the trigger, guard or effect dropped. The designation is read from the body the transition
  is written in, so a name reaching another state's entry action, or one naming a junction rather
  than a state, is reported where it used to analyze clean and then fail to execute. A machine
  designated this way renders its initial state in a view that only exposes it.
- **A metadata usage member names a type**, so `@Securty;` reports an unresolved reference the way
  the `#` prefix spelling does instead of going unchecked.
- **A value part accepts every operator the grammar allows** — `= expr`, `:= expr`,
  `default expr`, `default = expr` and `default := expr` — wherever a usage, parameter, result or
  subject binds a value; only some spellings were accepted per position.
- **A metadata usage member (`@M;`) parses in a namespace, a body and a state body**, and RDF
  conversion refuses it with a typed diagnostic instead of writing an annotation on a different
  element.
- **The REPL no longer prints a syntax warning twice**, once from the load that defers the
  analysis and once from the analysis itself.

### The Python client accepts the 0.1.0 service

- **`opensysml` carries the pinned `sysml-grpc` digests for `v0.1.0`**, so
  `OPENSYSML_GRPC_VERSION=v0.1.0` downloads and verifies the service instead of refusing it as
  unpinned. `PINNED_SHA256` stopped at `v0.0.8`.

### A rendering read at a terminal

- **`sysml -render` writes the text form at a terminal**, where a person reads it, and the
  machine-readable form of the kind rendered — Mermaid for a diagram, Markdown for a table — into a
  file or a pipe, where a tool does. `sysml m.sysml -render Views::table` showed a Markdown table on
  screen; `> table.md`, `| tool` and `-o table.md` are unchanged, and `-render-form` still names
  either form whatever the destination.
- **The text form is ASCII**: the rendering header and a connection edge were written with an em
  dash, which a terminal drawing no more than ASCII showed as a replacement character.
- **A text table is written to fit the terminal**, wrapping a cell wider than its column over as
  many lines as it needs rather than truncating it or overflowing the window. Columns are narrowed
  no further than 8 characters, and a table written to a file or a pipe keeps every column as wide
  as its widest cell, so a saved artifact does not depend on the window it was written from.

## 0.1.0 — 2026-08-18

### The project is now OpenSysML

The rename is a clean break with no compatibility aliases: every name below has exactly one
spelling from this release on. Entries for earlier releases keep the old names, because the
artifacts they describe really were called that.

- **Go module path is `github.com/Open-MBEE/OpenSysML`.** `go install
  github.com/Open-MBEE/OpenSysML/cmd/sysml@latest`; the old path resolves only for `v0.0.x`.
- **The binaries are unchanged** — `sysml`, `sysml-lsp` and `sysml-grpc` keep their names.
- **The Python client is `opensysml`**, on PyPI and as the import: `pip install opensysml`,
  `import opensysml`. Its environment variables are `OPENSYSML_*` (`OPENSYSML_GRPC_VERSION`,
  `OPENSYSML_STATE_DIR`, `OPENSYSML_GITHUB_REPO`, `OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`,
  `OPENSYSML_REQUIRE_SERVICE`), the base error is `OpenSysMLError`, the generator entry point is
  `opensysml-generate`, the state directory is `~/.opensysml` and the release tag is
  `opensysml-v*`. Nothing reads the `pysysml` names, so a `~/.pysysml` left behind by an older
  install is dead weight and can be deleted. The first release under the new name is 0.3.0,
  carrying on from `pysysml` 0.2.0 rather than restarting, and `pysysml` gets one last version,
  0.2.1, which contains no client: it raises on import naming `opensysml`, so `pip install
  pysysml` reports the rename instead of resolving to the pre-rename 0.2.0. Pin
  `pysysml==0.2.0` to keep that release while migrating; nothing further is published under
  that name.
- **Release archives are `opensysml-<os>-<arch>.tar.gz`** (`.zip` on Windows), and the Homebrew
  formula is `opensysml`: `brew install Open-MBEE/tap/opensysml`. Assets already published under
  `v0.0.x` keep their old names.
- **The RDF extension namespace is `urn:opensysml:sysml:`**, still bound to the `sysx:` prefix. A
  `.ttl` file written before this release carries `urn:systemica:sysml:` properties, and reading one
  is refused rather than silently dropping what those properties said — re-export it from its
  notation source.
- **The non-normative math library is `OpenSysMLMathFunctions`**, in
  `OpenSysML Libraries/OpenSysMLMathFunctions.kerml`. A model that writes
  `import SystemicaMathFunctions::*;` must be updated; the unqualified `exp`, `ln`, `log` and
  `atan2` aliases are unaffected.
- **Environment variables are `OPENSYSML_*`** (`OPENSYSML_SMT`, `OPENSYSML_SMT_TIMEOUT`,
  `OPENSYSML_REQUIRE_SMT`, `OPENSYSML_REQUIRE_TRAINING_CORPUS`, `OPENSYSML_SMT_CORE_BUDGET`,
  `OPENSYSML_SMT_MAX_CONFIGURATIONS`). The
  `SYSTEMICA_*` names are not read.
- **The VS Code extension is `opensysml-sysml`** and its settings are `opensysml.server.path`,
  `opensysml.server.args`, `opensysml.server.enabled` and `opensysml.trace.server`, with the
  command `opensysml.restartServer`. Existing settings must be re-set under the new keys.

### Binding connector runtime semantics

- **Bindings declared in materialized type and usage bodies now propagate values in both
  directions**, including inherited and nested ends, with typed conflict and cycle errors;
  calc result bindings such as `bind result = x` are also evaluated. Package-owned bindings
  remain a documented limitation.

### A named control node is a member, and a chained binding declares none

- **`fork`, `join`, `merge` and `decision` register the name they declare**, the way `first`/`done`
  already do, so `first Jump then Land;` names a control node as source or target instead of
  reporting it unresolved. An unnamed control node declares no name and registers nothing.
 - **A binding's end no longer names the binding.** `bind a.b.c = d;` records `a.b.c` as a reference
   subsetting — the end it binds, not a name the binding answers to — so `%search` and the symbol
   table no longer carry a stray `c` in the binding's owner.

### A view renders

- **`%render <name>` turns a view's exposed set into the rendering its `render` member states**, and
  into a containment tree where it states none. The kinds produced are a tree (the exposed elements
  with their kinds and names, nested views as subtrees), an interconnection diagram (the exposed
  parts and the connections between them, read from the model's own connector and flow ends), a
  state machine (states and transitions), an action flow (nodes and successions) and a table (the
  exposed elements, what they declare and the views nested in the rendered one, as rows). State and
  action renderings read the lowered `StateGraph`/`ActionGraph` the runtime executes, so a rendering
  cannot drift from what runs.
- **`%render <name> <form>` writes the machine-readable form of the rendering**: `mermaid` for the
  graph-shaped kinds — `flowchart TD` for a tree or an action, `flowchart LR` for an
  interconnection, `stateDiagram-v2` for a state machine — and `markdown` for a table, which Mermaid
  has no grammar for. Either pastes into Markdown, a documentation site or an editor as-is, and text
  stays the default at the prompt. A form the kind is not written in is a typed error naming the one
  it is.
- **`sysml model.sysml -render <view>` renders without the prompt**: the artifact on stdout, the
  kind's machine-readable form by default and `-render-form text` for the read form, `-o` writing it
  to a file, and every human notice — what was loaded, an empty rendering, an element the rendering
  cannot represent — on stderr. Rendering decides nothing about the model, so it is not asked for
  together with `-convert` or a check flag.
- **Rendering is a read.** `%render` materializes no object, registers nothing in the session and
  leaves an `%action`/`%state` debugging session stepping the same graph and the same objects, so it
  can be asked between two `%step`s. `%view`'s report is unchanged.
- **The empty and error paths say what happened**: a view exposing nothing renders an empty artifact
  and says so, a rendering kind this build does not produce is a typed error naming the kind and the
  view rather than a substituted rendering, a name that is no view is `semantics.ErrNotAView` as
  `%view` answers, and an exposed element a rendering cannot draw is reported rather than dropped.
- The rendering itself is **tool-defined output**: SysML v2 §10.2 leaves rendering to the tool, so
  the notation is what is supported and the artifact is OpenSysML's own — recorded as such in
  [docs/project/spec-compliance.md](docs/project/spec-compliance.md).
- **An element reached twice is exposed and rendered once.** A wildcard or filtered `expose` walks
  the document's own scope tree and the global index, which build a symbol each for one
  declaration, so `expose P::*` and `expose P::**[@T]` used to show an element as many times as it
  was reached. The declaration a symbol was built from is now its identity, so exposure, filtering,
  rename and reference lookup all agree on when two symbols are one element.

### An object runs the behavior its type exhibits

- **Materializing an object starts the behaviors its type exhibits or performs.** An
  `exhibit state` machine written in a part definition is now that part's own machine: each object
  gets an execution of its own, so two objects of one type hold two current states, two event queues
  and two sets of feature values. Until now the body only parsed, resolved and lowered — running it
  meant `%state` on the state usage itself, detached from any object.
- **Identity carries through the run.** What an entry, `do`, exit or effect body reads and writes is
  the performing object's feature values, and a send addressed to an object reaches that object's
  machine and not a sibling's — a nested object now knows the object that owns it, so
  `send … to sibling` finds the sibling instead of materializing a second one.
- **Startup and quiescence are defined.** Feature values and constant defaults come first, so an
  entry action sees declared initial values; then the behaviors are initialized and run until
  nothing is due at the current time — a machine waiting on a timer is quiescent, and `%step` or
  `%advance` drives it. Cross-object messages are drained collectively, bounded by the state-event
  and do-step budgets, so a machine that never settles reports
  `object behaviors exceeded their budget` rather than hanging.
- **A second `%instantiate` of one name is a second object**, with its own identity and its own
  behaviors, and the command now says so (`note: P now denotes this object; object #1 is no longer
  named, with 1 behavior of its own`) instead of silently replacing the name. `occurrenceOf` is
  still the reuse path for a named occurrence.
- **`%invoke <object> <op> [<p>=<expr>]` runs an operation of the object's type**, performed by that
  object, binding arguments to its `in`/`inout` parameters by name and printing its outputs. Known
  limitation: only an action member is executable this way — an operation written as a `calc` or
  `constraint` is evaluated as an expression rather than performed, and reports that.
- **`%state <object>` attaches to the object's exhibited machine**, so `%step`, `%advance`,
  `%current`, `%events` and `%features` all describe that object. The object's identity and the debug
  session both survive an unrelated declaration.
- **A carried object's behaviors restart in the rebuilt analysis, and it is reported.** An execution
  belongs to the graph, names and message bus of the analysis it started in, so an object carried
  over an unrelated declaration keeps its identity but starts its behaviors again from their initial
  states — dropping what the discarded run wrote — with a `note:` naming what restarted. A `%state`
  session follows the object onto its restarted machine.
- **Rewriting a behavior drops the objects running it.** Re-declaring the machine or action an object
  runs changes what the object is, so the object itself is dropped with a reported reason instead of
  being carried over at all.
- **A feature holds the object it materializes before that object's behaviors start**, so two nested
  objects addressing each other reach one another instead of materializing a fresh copy per message
  until the event budget runs out.
- **A creation that fails leaves nothing naming what it removed**: a feature of a surviving object
  that reached one of the removed objects is read again, and messages addressed to them are dropped
  with them.
- **A `%state` session over a machine an object merely performs stays on that machine** across an
  unrelated declaration; only a session over the object's own exhibited machine follows a restart.
- **`%step` wakes a machine parked on a change condition**, so a condition made true from outside it
  — by `%invoke`, by another object, or by a later declaration — is dispatched instead of the machine
  reporting itself suspended forever.

### `%slots` is now `%features`, the name SysML v2 uses

- **`%features <name>` lists what an object holds for each feature of its type**, which is what
  `%slots` listed. "Slot" is UML/SysML v1 vocabulary (`InstanceSpecification::slot`); the v2/KerML
  pair for this concept is `Feature` and `FeatureValue`, and the listing's heading now reads
  `Features:` to match. `%instantiate` points at the new spelling too.
- **`%slots` is gone**, not kept as an alias: since it never shipped in a release, 0.1.0 takes the
  clean break rather than carrying the v1 spelling forward. Nothing else about the listing changed —
  the nested expansion, its bounds, the error lines and the exit status a non-interactive run takes
  from a feature value that could not be materialized are all as they were.
- **The vocabulary behind the command is v2 too.** What was printed and named as a "slot" is now a
  *feature value* (`FeatureValue`, `KerML.kerml`), and a state's `do` behavior is a *do action*
  (`States.sysml`) rather than a "do activity":
  - Message text changed: `feature value craft.volumes: multiplicity violation: …`,
    `cyclic feature value dependency`, `uninitialized feature value`,
    `no errors in the feature values checked`, and
    `state machine exceeded max do action steps (…)`. The `%budget` label reads `do action steps`.
    `SYSML_MAX_DO_STEPS` and every exit status are unchanged, but a script matching the old text
    needs updating.
  - The gRPC interface carries `Instance.feature_values` (`FeatureValue`) and the `feature_values`
    capability. `Instance.slots` and `SlotValue` are removed; field number 3 and the name `slots`
    stay reserved in `sysml.proto`, so the number is never reused. `opensysml` requires that
    capability before it hands back an object, so a service predating the rename — every published
    release does — is named rather than answering with an object that appears to hold nothing.
  - `opensysml` exposes `Instance.features`, `raw_features`, `get_feature`, `FeatureValueError`, and
    `typed.feature_value`/`optional_feature_value`/`list_feature_value`, which generated modules
    emit (emission schema `3`). The `slots`, `raw_slots`, `get_slot`, `SlotError`, `slot_to_python`
    and `slot` decoder spellings are removed.
  - The Go runtime API is renamed to match (`runtime.FeatureValue`, `Instance.FeatureValues`,
    `GetFeatureValue`, `FeatureValueError`, `ErrFeatureValueMaterialization`, `ErrCyclicFeatureValue`,
    `ErrUninitializedFeatureValue`). It is internal, so nothing outside the module depends on it.

### The prompt prints the model it holds

- **`%print` writes the session's model back as SysML notation at the prompt**, which until now
  needed `%save <file>` and another program to open the file. `%print <name>` prints one element
  and its body instead of the whole buffer, taking the quoted and qualified spellings every other
  command takes (`%print 'My Pkg'::Car`, `%print Top::'My Pkg'::Car`), and tab completes both the
  command and the names after it.
- It is the writer `%save` writes `.sysml` with — `export.SysMLElement` renders one element's
  source through the same `format.Source` path a whole-document save goes through — so comments and
  the text as typed survive, and a print submitted again rebuilds the same model. Notation only: no
  RDF notice follows a print.
- Printing is a read. No object is materialized, `%instances` and the buffer are unchanged, and an
  `%action`/`%state` debugging session keeps running across it. An empty session, a name nothing
  declares, and a symbol this session holds no source of (a library name) each answer in one line.

### An SMT solver decides what a model's conditions permit

The whole path is **experimental** and every surface says so: the vocabulary of the reports may
change, and a solver is optional at runtime — discovered on `PATH` or named by `OPENSYSML_SMT`,
with a build that has none reporting that rather than a verdict.

- **`%check <name>` asks an external SMT solver whether a constraint, requirement or satisfaction
  assertion *can* be satisfied**, and prints an assignment on `sat`. Conditions are translated to
  an SMT-LIB 2 script — one variable per logical feature with injective symbols, quantities in
  named base units, and truncating integer division whose well-definedness guard is hoisted only
  where the division always runs — and `sat`, `unsat` and `unknown` stay three distinct verdicts.
  Satisfiability is not evaluation: `%constraint` and `%satisfy` still answer what holds of an
  object.
- **`%explain <name>` says which conditions conflict** behind an `unsat`: an unsat core reduced to
  a minimal one by dropping a member at a time in fresh solver processes, bounded by member count
  and `OPENSYSML_SMT_CORE_BUDGET`, printed as the role, the condition as written, the declaring
  element and `file:line:col` in the query's assertion order. A declared domain (a `Natural` being
  non-negative) or a division guard can be the conflicting condition, a one-member conflict says it
  is the whole conflict, and a core that was refused, unreadable, empty, repeated or never issued is
  a typed `CoreError` rather than a shorter core presented as minimal. The time reported covers the
  reduction, not just the first verdict.
- **`%solve <name>` synthesises values that satisfy an assertion**, keeping fixed what already is —
  the values an object holds, else the ones the model declares — and reporting what was fixed and
  by whom, the values chosen, and that they are one witness of possibly many. `unsat` there means
  no values exist consistent with what is fixed, and names the fixed values that conflict; an
  object's fixed values survive an unrelated submission.
- **`%configure <name>` answers which variants an assertion permits**: with no argument one
  consistent selection, with `<variation>=<variant>` the named selection checked and the conflict
  named where it is not consistent, and with `all [<count>]` the selections enumerated up to
  `OPENSYSML_SMT_MAX_CONFIGURATIONS`. The report says whether they are all of them or were cut
  short — at the bound, or because the solver stopped deciding or ran out of time, in which case
  the selections found so far are still reported. An element that reads no variation point is an
  error pointing at `%check`.
- **`%optimize <name>` improves the `objective`s an `analysis def` states**, which until now parsed
  and then sat inert: the direction comes from the trade-study definition typing it
  (`TradeStudies::MinimizeObjective` or `MaximizeObjective`), the value from the expression the
  objective states for the library's `best` feature, and feasibility from the case's own conditions
  together with each objective's — all read through the runtime's own surfaces, re-parsing no
  declaration. Several objectives are improved lexicographically in declaration order, with
  `(set-option :opt.priority lex)` written into the script rather than left to a backend default,
  and every optimum is verified by asking whether anything does better, so an attained optimum, an
  unbounded objective, a bound no assignment attains and an answer that could not be verified stay
  four different reports and none of them fabricates a number.
- **What a query needs of a backend is modelled as capabilities**, probed once per executable (or
  declared by the caller) and cached, so a feature the backend lacks is an
  `UnsupportedCapabilityError` naming the backend, the feature and the operation instead of a silent
  degrade or a fabricated verdict. A query emits the narrowest standard SMT-LIB 2.6 logic it needs,
  falling back to the non-standard `ALL` only for datatypes and strings, which the standard logics
  cover with nothing. Optimization is a z3 extension, so cvc5 is refused there rather than answering
  a plain `check-sat` presented as an optimum. "Lacks the feature", "cannot be run" and "did not
  decide" are three distinct reports, an undecided probe settles nothing, and a reply SMT-LIB does
  not define — `maybe` — is a `SolverProcessError` about the executable rather than a capability
  refusal.
- **The path is gated in CI both with a solver and without one.** A differential gate requires the
  solver's `sat`/`unsat` to agree with the evaluator's verdict over the conformance corpus, the
  standard library, the OMG training corpus and deterministic randomized models — it found and
  fixed a real division by zero answered as an infinity, and a redefining variation usage given a
  sort of its own — and a portability harness reports pass/refuse/fail per capability against
  whatever `OPENSYSML_SMT` names, wired to both z3 and cvc5.
- `brew install opensysml` brings z3 along, so the path works out of the box on a Homebrew
  install, and the install guide, troubleshooting page, environment reference and REPL command
  reference each say how to get a solver otherwise.

### A model is edited through the source it was parsed from

- **`ApplyEdits` edits a loaded model by rewriting the bytes of its own source.**
  `internal/core/edit` is a span-level engine that sets a feature's value or renames a declaration
  and leaves every untouched byte identical, so comments and the text as typed survive an edit the
  way they survive a save. The edited source is re-parsed and re-analyzed before it is handed back,
  and the edits of a request are applied all of them or none. The source edited is the one the parse
  read, named by its hash, so a file changed since then is refused rather than edited blind.
- **An edit is judged only by the tiers the original's parse reached.** A model whose parse had
  errors was never analyzed, so its semantic baseline was empty and every pre-existing name or type
  error counted as one the edit introduced — refusing a good edit to a file with a syntax error
  elsewhere. Renaming a referenced element, and creating or deleting an element, are refused with a
  typed error rather than approximated.
- **`opensysml` exposes it as `model.edit()`** — `set_value(target, value)`, `rename(target, name)`
  and `apply()`, whose result saves the way a `Conversion` does — behind the `apply_edits`
  capability, so a service too old to offer it says so instead of failing as an unimplemented
  method.

### Resolution cost

- **Resolving a feature chain is linear in its length**, where a chain's prefixes used to be
  re-resolved per segment, and an operand of a chain that is no reference is preserved rather than
  dropped on the way.

### A model that states behavior converts to RDF

- The behavioral nodes of an action or state body now have metaclasses and the properties their
  notation is rebuilt from, so a model stating steps converts instead of being refused: the
  initial and final node, `perform`, `send`, `accept`, `terminate`, `assign`, the
  fork/join/merge/decision control nodes, `while`/`loop`/`for`, `if`/`else`, and the state
  machine's states, substates, regions, `entry`/`do`/`exit`, `defer`, pseudostates and
  transitions. Each category is covered by a `notation → RDF → notation` round trip asserting the
  body comes back byte-identically. The mapping is tabulated in
  [the RDF mapping](docs/reference/rdf-mapping.md) § Behavior; terms the OMG vocabulary has no
  counterpart for are named under the `sysx:` extension namespace.
- **102 of the 120 models under `examples/` now convert to Turtle, up from 71.** The remaining 18
  are refused with the node named, not partly converted: nine successions that do not name both
  of their ends, three prefix-metadata models, three duplicate declarations, two
  operator-expression members and one anonymous `snapshot`.
- A shorthand relationship no longer collides with the member it names: the `result` of
  `bind result = x;` and the `x` of `first x;` are carried as references to that member rather
  than as a name the element declares, which is what made those models fail as duplicate
  declarations.
- Two things the mapping used to lose quietly now survive it: a metadata annotation
  (`#Safety part def Car;`) is carried as the notation it was written as, and a feature that wrote
  no kind keyword (`in x : Real;`) comes back without one instead of gaining the kind's canonical
  keyword. The two annotation shapes that still cannot be written back — one carrying a body, and
  an `@` annotation the parser records on the declaration ahead of the one it prefixes — are
  reported with the line named.
- Two more shapes the mapping used to change quietly now come back as written: a combined state
  subaction keeps its `do` whatever separates it from its body (`entry do{ … }`), and a kind
  keyword named inside a comment in a declaration's head (`in /* attribute */ x : Real;`) is read
  as the trivia it is rather than as a keyword the author wrote.
- RDF conversion remains **experimental** — the vocabulary may still change, and no round trip
  through a running triplestore has been demonstrated (roadmap D1–D3).
- Every surface now states that status in one wording again: `-help` prints
  `export.ExperimentalNotice` rather than a copy of it, and the Python client's fallback notice,
  the `ConvertResponse.experimental` comment and the guide pages were brought back in step. A test
  pins the Python copy byte-identical to the constant, since Python cannot import it.

### A nested value body governs over an inherited value

- **A redefining declaration whose body values features holds those values.** `part def Ring
  { attribute cost : Cost = template; }` re-opened as `part r : Ring { attribute :>> cost
  { attribute :>> v = 11.0; } }` now reads `r.cost.v` as `11.0`: the more specific declaration of
  the feature governs (KerML 1.0 §7.3.4.5), where the inherited value used to win and the body's
  restatements were dropped with no diagnostic. A feature the body does not value takes its type's
  own default, the inherited value binding nothing there — the body supersedes that value rather
  than merging with it, since a `FeatureValue` binds a feature as a whole.
- Unchanged: a body on the *same* declaration that writes the value still reports
  `ErrValuedFeatureRestated` (two values, neither more specific), and a body that only re-declares
  features (`attribute :>> kept { attribute :>> v; }`) still reads the inherited value — at any
  depth of nesting, a value stated anywhere in the body being what makes it govern.
- **A check made without an object agrees with materializing one.** A condition naming a feature a
  body governs over reads it as uninitialized rather than against the superseded value, so the same
  model no longer passes or fails a check depending on whether an object was built for it, and an
  edit confined to a governing body changes the type's shape, so a carried-over object is
  re-materialized instead of keeping the value the body replaced.

### Names nested inside a `require`/`assume` body are resolved

- **A `require`/`assume` body now resolves to whatever depth it is written to.** Where only its
  direct members resolved, a declaration nested in it (`require Q::r { part p : P { :>> f; } }`)
  had its own body left unwalked, so a typo there produced no diagnostic at all; such a name is
  now resolved and, if it names nothing, reported at its own span.
- The body is a scope of its own: what it declares is visible to what nests inside it but is no
  member of the namespace that declares the member, and the referenced requirement's features stay
  offered to the body's direct members, which is what the reference subsetting inherits them to.
- **Every tier reads the body in that same scope.** Type checking and condition evaluation walked
  such a body in the *enclosing* scope, so a value written there was typed against the wrong set of
  names — silently missing a genuine type error, or judging the name against an unrelated
  declaration outside the body — and a condition stated in the body could not read a name the body
  declares.

### An unimported OpenSysML extension function no longer answers a call it is reported unresolved on

- **`exp`, `ln`, `log` and `atan2` now require the import that declares them.** They are declared by
  the non-normative `OpenSysMLMathFunctions` extension, which no OMG library carries, so a bare
  `exp(x)` is reported `unresolved reference: exp` — and used to be evaluated anyway by dispatch on
  the local name, which meant the diagnostic and the behavior disagreed: ignore the error and the
  model computed, trust it and the model looked broken. Such a call now fails with a typed error
  (`ErrUnimportedExtensionFunction`) naming the function and the `import OpenSysMLMathFunctions::*;`
  that makes it legal.
- **A model that imports the package, or writes the call qualified, is unaffected**, as is a bare
  call to an OMG function library (`sqrt`, `sin`, …), which every model may write whatever it
  imports.
- **`%builtins` still lists them, marked with the import they need.** Dropping the four names from
  the unqualified-dispatch registry also dropped them from the listing and from name completion,
  which made implemented functions look unsupported; the listing is now taken from what the build
  implements, and an extension function is listed with a `(needs import OpenSysMLMathFunctions::*;)`
  marker rather than silently omitted.
- **A root-level import in a document that declares nothing else now surfaces its names**, which is
  what a bare `import OpenSysMLMathFunctions::*;` at the REPL prompt is: the editor's own scope tree
  is identified by the document name stamped on it, and a document with no member had no symbol left
  to carry that name, so its own import was read as another document's private re-export.

### The corpus notation two verdicts were open on is adjudicated legal

- **A conjugated end (`end spacePort : ~CommunicationPort`) and a portion prefixed onto a kind
  keyword (`timeslice item item1`) are legal, and are accepted.** `ConjugatedPortTyping`
  specializes `FeatureTyping`, so any feature typing — a connection or interface end among them —
  may name a conjugated port definition, and `PortionKind` is an attribute of `OccurrenceUsage`,
  which an item usage is. Both are pinned clean over every validation tier by
  `testdata/passes/corpus_notation.golden`, so a regression is caught as the false positive on a
  flagship model that it would be. What the Open-MBEE models still report is other notation: the
  OMG-side `end;` outside an interface body and `'SysML Standard Diagrams'::gv`, and — in
  `DesertKite.sysml`, which lives only on that repository's `InitialDesign` branch — 7 errors that
  are ours: a qualified name refused as a `bind` end and `connection connect … ;` refused inside a
  `requirement` body, both owned by a separate session.

### A binding end may be a qualified name, and a requirement body takes a prefixed connector

- **`bind` now accepts the notation a connector end is written in:** `binding bind R::a = c;` and a
  feature chain whose chaining features are themselves qualified names
  (`bind 'Kite Environment'::'Region Earth Surface'.'Kite System'::'Desert Kite'.'Wall Height' = …`)
  parsed as far as the first `::` and then failed. A connector end names a feature by a
  `QualifiedName` (SysML `ConnectorEndMember` → KerML `OwnedReferenceSubsetting`,
  `OwnedFeatureChaining`), and every segment of the chain is now recorded and resolved, not
  collapsed to its last one.
- **The named form is no longer read as a redefinition:** `binding b1 bind R::a = c;` reported
  "b1 redefines a, but a is not an inherited member of P". `b1` is the binding's name and `R::a` its
  first end, which reference-subsets the feature it names, so it resolves where that feature is
  declared rather than as an inherited member of the binding's owner.
- **A connector, flow or message written with its kind keyword is a member of a requirement-like
  body:** `requirement r { connection connect r to x; }` reported `expected a body member` at the
  closing brace. A `requirement`, `constraint`, `concern`, `objective`, `use case` and `view` body
  admits usage elements, and a connector usage declares no name — its ends are what make it a
  declaration.
- The Open-MBEE `DesertKite.sysml` model parses clean as a result. What it still reports is
  recorded in [spec compliance](docs/project/spec-compliance.md) § Structural, Interface and
  Analysis Notation: the tool-specific `'SysML Standard Diagrams'::gv` namespace, an `OOSEM::MOE`
  reference to a member of `OOSEM::'OOSEM Measures'`, and references to a decision node whose name
  is not registered as a symbol.

### RDF conversion is experimental

- SysML ↔ RDF Turtle conversion is labelled **experimental**, in both directions, on every
  surface that offers it. The mapping covers a model's structure and behavior but not its
  expressions, a model it cannot write is refused with the construct named (the counts are
  above), its vocabulary may change without a compatibility path, and no round trip through a
  running triplestore has been demonstrated (roadmap D1–D3, D6). Saving and converting notation
  (`.sysml`, `.kerml`) is stable and unchanged.
- `sysml -convert` writes the status as a `note:` on **stderr**, so a conversion piped to a file
  or to stdout carries no extra bytes, and a refused conversion is labelled too. `-help` says
  the same.
- `%save model.ttl` prints the status before it writes, including when the model is refused.
- `ConvertResponse` carries `experimental` and `experimental_notice`, set before the conversion
  runs, so a client reads the status off a refusal as well as off a success. The `convert`
  capability is unchanged: the status is per conversion, not per service.
- `opensysml` raises the status as `ExperimentalFeatureWarning` and exposes it as
  `Conversion.experimental`/`.experimental_notice`, plus `opensysml.is_experimental(from, to)`.
  A service too old to send the fields is read from the formats it reports instead, so an RDF
  conversion warns either way. Silence it with
  `warnings.simplefilter("ignore", opensysml.ExperimentalFeatureWarning)` — no stable feature
  warns with that class.
- The wording lives once, in `export.ExperimentalNotice`, so no surface can drift from another.

### Documentation

- The RDF mapping reference opens with a **Status: experimental** section stating what the
  mapping covers, that the vocabulary may change, and that interoperability is unverified.
- The claim that a converted graph "loads into" Flexo MMS's triplestore is withdrawn from the
  reference and from `docs/project/spec-compliance.md`: the vocabulary and element IRIs match
  Flexo's `Namespaces.kt`, which is an addressing claim, not a demonstrated load.
- The README capability table splits notation save (complete) from RDF conversion
  (experimental), and the guide, CLI and REPL references, Python guide and roadmap say the same.
- Two example models come with a walkthrough of the commands that exercise them:
  [`examples/solver-demo.sysml`](examples/solver-demo.sysml) for `%check`, `%explain`, `%solve`,
  `%configure` and `%optimize`, and [`examples/views-demo.sysml`](examples/views-demo.sysml) for
  `%view` and `%render` across the five rendering kinds and the text, Mermaid and Markdown forms.

### Release process

- The CircleCI pipeline that builds release tags downloads the OMG training corpus and runs the
  suite with `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, so the corpus gate can no longer skip
  silently where a tag is cut. 0.0.9 listed that as a known limitation; it is closed.

### Known limitations

- Of what 0.0.9 listed, the untested tag pipeline is closed above and the RDF refusal of a model
  stating behavior is closed by the mapping's behavior coverage. What stands: expressions are not
  emitted as triples and end-binding heads depend on `sysx:sourceText`, so RDF conversion is
  stated as a feature status rather than a footnote; a `that` written inside a nested `action`,
  `constraint` or transition-guard body binds to the innermost enclosing usage; an unqualified
  standard library name requires an import, which is conformant and recorded as won't-do; and a
  port that accepts TCP but never answers gRPC costs the Python client about 9 s rather than the
  nominal 2.5 s `START_TIMEOUT`.
- A package-owned binding connector does not propagate values; a binding declared in a
  materialized type or usage body does.
- Constraint solving is experimental and needs an external solver: `%check`, `%explain`, `%solve`
  and `%configure` want z3 or cvc5, and `%optimize` wants z3, since optimization is a z3 extension
  cvc5 does not implement. A condition the translation has no SMT-LIB form for refuses the whole
  query rather than dropping the condition, and the guide covers the commands by reference rather
  than in a chapter of its own.
- An edit sets a feature's value or renames a declaration; creating or deleting an element, and
  renaming one that is referenced, are refused.
- Only an action member of a type is executable through `%invoke`; an operation written as a
  `calc` or `constraint` is evaluated as an expression and reports that.
- A rendering is tool-defined output (SysML v2 §10.2 leaves rendering to the tool), so what
  `%render` and `sysml -render` produce is OpenSysML's own notation rather than a standard
  interchange form.

## 0.0.9 — 2026-08-17

### Language and semantics

- A transition leaving a composite state fires while a substate is active, where it used to
  never be taken, and a transition between sibling regions exits only its source rather than
  the whole composite state; history is recorded per region.
- Succession and transition endpoints (`a then b`, a transition's source and target) are
  resolved at the name-resolution tier, in the scope they were written in, instead of being
  matched against a flat list of states and silently dropped where no match was found. An
  endpoint naming a vertex of another state machine, or a named `first`/`then` marker, is a
  check-time diagnostic rather than a failure to construct the executor; an endpoint no pass
  reported leaves its own edge out instead of failing the lowering.
- A `send` reaches its target by port direction, conjugation and the performing part, so a
  message declared on a conjugated port arrives where the model says it does and a state
  machine nested in a part reaches that part.
- A block owns its token flow: `for` iterates every collection it is given, an output the
  block's own flow assigns is counted, a result written among its flow nodes is returned, and
  a `for` over a non-collection is reported rather than iterated once.
- Library evaluation covers string operators and `StringFunctions`, `VectorFunctions` and
  `ComplexFunctions`, `@`/`@@` classification, a queryable exposed element set for views,
  library feature values read as names (`TrigFunctions::pi`, deg/rad) and `includingAt`
  insertion. A vector element or inner product beyond the `Real` range, and an argument named
  for no declared parameter, are reported rather than wrapped or ignored.
- An enumeration literal is a value, in the runtime and across the API.
- The subject of a check or evaluation is chosen deterministically and reported: keyed by
  declaration path rather than holder identity, bounded in its search, routed through
  `satisfy`, with the objects of one declaration counted once, an object held by another not
  a subject root, a nested definition's objects among them, and a nested redefinition on an
  object eligible. An ambiguous carrier is named by its definition, and a nested subject is
  named in verdicts, labels and over the wire.
- A calc body with an implicit result is type-checked, and calc recursion evaluates under a
  budget instead of exhausting the stack.
- A multi-valued default is honoured where it conforms and reported where it does not, and a
  default whose multiplicity is not declared is held to the assumed `1..1`.
- An accept node's payload resolves in its action body, a nameless payload no longer masks
  the feature it is named after, and shared payload visibility is limited to action bodies.
- Parser and classification: modifier-driven usage kinds, keyword-named parameters and loop
  variables, a classifier specializing any definition, a KerML datatype classified as a
  definition while `function` stays a calc, recursive `expose` traversal preserved through
  filtered namespaces, and classification judged by element name across index generations.

- A valueless feature of a value type reads as unset rather than as an empty
  object: `attribute d : Real;` reports `d = <unset>` where it used to report
  `d = Instance(ID: 2)` with `(no features)`. What materialization creates is
  unchanged — a `Real` has no features to instantiate, so the object it holds is
  empty — but every surface that reports a value now says so with one spelling:
  `-instantiate`/`-e`, `%slots`, the JSON report, and the wire, where
  `Value.unset` is a new arm the service sends and refuses to accept. A valued
  attribute (`k = 2.00`), an object of a class, and a value type that does
  declare features are unaffected.
- A member chain from `that` resolves: `attribute b : Real = that.a;` in a usage
  body reads `a` off the object featuring the value being written — the innermost
  enclosing usage, whose own and inherited members are both reached — instead of
  reporting `no scope for member lookup in Base::things::that`, since `that` is
  declared `Anything[1]` and owns no members ([KerML, 8.4.2]). A `that` written
  where no usage encloses it stays unresolved rather than resolving to the
  library's declaration.
- A root-level `private import X::*;` serves the document that wrote it. It was
  hidden from that document too, so a file opening with `private import
  ScalarValues::*;` — the spelling OMG's own training files use — reported `Real`
  unresolved. A root-level import still reaches no other document, at any
  visibility ([KerML, 8.2.3.3]).
- The import an editor offers for an unresolved library name is written `private
  import X::*;` explicitly, so applying the fix does not re-export the imported
  names onward.

### Tools

- `sysml-lsp` accepts `--stdio` and implements the `shutdown`/`exit` lifecycle, so the
  shipped VS Code extension can start the shipped server; it used to exit 2 and crash-loop.
- The REPL closes a cluster of usability gaps: unclosed submissions, load diagnostics,
  object-resolved subjects, `%view`, ranked suggestions, quoted qualified names, a pinned
  `%eval` context, reported reset loss, and an unreadable name reported as typed.
- A piped REPL session whose command could not materialize a slot exits 2, so a script can
  detect it, and the CLI reports materialization diagnostics instead of swallowing them.

### gRPC service

- A quantity crosses in both directions with its magnitude, the unit as written and the
  reduced unit term, and is read as a typed Python value. An unreduced unit, a zero unit
  scale and a named unit arriving without its reduction are rejected.
- `Evaluate` is subject-aware behind a capability, attributes are populated by following
  typing edges, and generalization bases are reported.

### Performance

- A cold `ParseFile` is served from a pool of prewarmed standard library indexes, taking
  under 1 ms where it used to rebuild the index per distinct model at ~110 ms. Each cache
  store writes its own temp file and the pool refills serially.

### Tests and documentation

- `cmd/sysml-grpc`, a published artifact that had no tests, is gated on process lifecycle —
  start, one RPC, shutdown, and the failure exits — along with the subtle `resolve` and
  `semantics` rules that were uncovered.
- The gate figures are counted at the first-subtest level and each has exactly one home;
  every other page links to it rather than restating a number that drifts.
- Behavioral semantics cite SysML v2 and KerML rather than UML 2.5.1.

### Python client (`pysysml`)

- `pysysml.UNSET` is what a slot holding no value reads as — falsy, spelled
  `<unset>`, and distinct from `None`, the model's `null`.
- A quantity can be *sent*, not only read: a `pysysml.values.Quantity` is
  accepted wherever a value is — an action input, a calc argument, an element of
  a sequence — and crosses as `Value.quantity` with its magnitude in the kind it
  was written in, the unit as written and the reduced unit term, so a quantity
  read from the service round-trips through an evaluation with both magnitude and
  unit preserved. A unit named without the reduction commensurability is decided
  over is refused before anything is sent, rather than compared by bare
  magnitude.
- A `Connection` that starts the service asks it at once and then backs off (10
  ms, 20 ms, 40 ms … capped at 250 ms) instead of sleeping half a second before
  the first probe, so starting a service that answers in milliseconds costs ~17
  ms rather than ~510 ms. Waiting is bounded by the same ~2.5 s and raises the
  same `ConnectionError`, now as the documented `connection.START_TIMEOUT` and
  covering the probing as well as the sleeping, so a port that accepts without
  ever answering no longer costs a whole probe timeout beyond the bound; a
  service that died is still detected before each probe and ownership,
  stale-service and pid authentication are unchanged.
- The two names that shadowed builtins are renamed: `pysysml.eval` is
  `pysysml.evaluate` and `pysysml.RuntimeError` is `pysysml.ExecutionError`. Each
  old name still resolves to the same object with a `DeprecationWarning` and is
  gone from `__all__`, so existing snippets keep working while a star-import
  shadows neither builtin. The `Model.eval` and `Connection.eval` methods are
  unchanged.
- A release this `pysysml` pins no digest for raises the new
  `UnpinnedReleaseError` instead of `ChecksumMismatchError`, which named the
  wrong cause. It subclasses `ChecksumMismatchError`, so an `except` clause
  written before it existed still catches it, and only it may be answered from a
  cached binary — a contradicted digest still never is.
- `pysysml.__version__` reports the declaration shipped beside the module, so an
  editable install whose checkout bumped `VERSION` after `pip install -e` no
  longer reports the version it had at install time. The version tests locate the
  installed package through the install's own PEP 610 record, which for an
  editable install is the checkout rather than a site-packages path holding no
  `pysysml/`.
- The generated protobuf stubs ship type annotations (`sysml_pb2.pyi`, generated
  by `make python-proto`), so `mypy` no longer reports the message classes and
  enum constants as undefined.

### Known limitations

- Two of 0.0.8's four listed limitations are closed by this release (the nested
  redefinition as a subject, and the unchecked implicit-result `calc` body). The RDF
  limitation stands: expressions are not emitted as triples and a model whose behavior is
  stated as action or state nodes is still reported rather than converted, so the RDF path
  should be read as experimental.
- A `that` written inside a nested `action`, `constraint` or transition-guard body binds to
  the innermost enclosing usage, so `that.k` naming a member of the enclosing part is
  unresolved. This is what the spec text says as written; the outward binding is not
  implemented.
- An unqualified standard library name still requires an import (`private import
  ScalarValues::*;`, the spelling OMG's own training files use). Only the public top-level
  elements of a root namespace are globally visible ([SysML, 7.2] over [KerML, 8.2.3.5]), so
  this is conformant rather than a gap, and is recorded as won't-do.
- A port that accepts TCP but never answers gRPC costs `pysysml` about 9 s of wall clock
  rather than the nominal 2.5 s `START_TIMEOUT`. The wait is bounded and raises a clear
  `ConnectionError`.
- The tag pipeline (`.circleci/config.yml`) does not download the OMG training corpus, so
  the corpus gate does not run there; it was run locally for this release.

### Release process

- `build-release` fails a release whose built artifacts do not report the tag
  they were cut from, before anything is stored or published.
- `python/scripts/pin_release_checksums.py` fails with a typed
  `MissingTokenError` naming `GITHUB_TOKEN`/`GH_TOKEN` when neither is set,
  instead of an opaque rate-limited HTTP 403 from an unauthenticated request; the
  scope it needs is documented in the release runbook.

## 0.0.8 — 2026-08-15

### Language and semantics

- A multi-valued feature that is both typed and given a default holds the
  default's values rather than an instantiation of its type: `attribute xs :
  Real[3] = (1.0, 2.0, 3.0);` materializes those three elements, a
  `part`-typed collection holds the very objects its default names, an
  expression default holds what the expression produced, and a quantity keeps
  its unit. A default whose element count does not conform to the declared
  multiplicity — one value against `[3]`, four against `[3]`, `()` against
  `[1..3]` — is a multiplicity violation, reported statically where the count is
  a literal one and when the slot materializes where only evaluating the
  expression knows it, rather than broadcast, padded or silently dropped. A
  feature whose multiplicity a redefinition does not restate is bound by the one
  it redefines. This was the second known limitation listed for 0.0.4 and 0.0.5.

### Diagnostics

- A comparison or sum of quantities whose dimensions are both statically
  determined and incommensurable (`mass < 1000.0 [m]`) is reported as a
  type-tier warning at validation time, from the stdlib `QuantityDimension`
  power factors, instead of only when the expression is evaluated. Evaluation
  keeps its hard error and a warning changes no exit status; a dimension a
  declaration does not determine stays unknown and is not reported.

### REPL

- A check of a condition declared on a definition is answered about the object
  that carries it, so `%constraint`, `%requirement` and `-constraint` on an
  instantiated model report the object's values rather than the declaration's
  defaults — a violating model used to be answered `✓ passed` with exit 0.
- `%eval` reads the object carrying the feature when the session holds one, so a
  check and an `%eval` in the same session no longer answer about different
  subjects; where several objects carry the feature it refuses to choose.
- A condition whose evaluation could not be carried out is worded as undecided
  (`? … could not be evaluated`) and names why, keeping exit 2, where it used to
  print a failure while exiting 2.
- A submission the parser cannot close — an unterminated body, block comment or
  quoted name, typed or in a loaded file — no longer absorbs the submissions
  after it: it is reported, kept in the buffer for `%list` and `%save`, and
  masked out of the text the session analyzes, so the next declaration parses
  and resolves as it would have before the bad one.
- A loaded file's syntax errors are printed the way a typed submission's are,
  against that file and its own line numbering, and count as errors for
  `HasErrors`, so a non-interactive run over a broken file fails instead of
  reporting nothing.
- An expression whose subject is reached through a declaration is evaluated on
  the object in effect for it, so `%eval Spec::c` honors a redefinition made on
  a nested object; two objects carrying the feature are still refused rather
  than chosen between.
- Two loaded files that open the same package are told apart explicitly: each
  opening stays a declaration of its own, both openings' members resolve
  qualified, and the load says to qualify a reference across them. Re-typing a
  package at the prompt still folds into the package already in the session.
- `%view <name>` is implemented, listing what a view exposes — its own `expose`
  relationships and the protected ones of the views it specializes — and the
  views nested in it; asking it of an element that is no view says so.
- The qualified names offered for an unresolved name are ranked and capped:
  what the session declares before the library, a package's member before a name
  nested in another element, and at most three, where an unresolved `length`
  used to list every same-named library member including function parameters.
- A `%satisfy` verdict quotes the inner names of the assertion it reports, so a
  requirement or subject whose name the notation quotes reads back as written.

### `sysml` command line

- A lone `-` names standard input wherever a model path is taken, `-convert`
  included, and is reported as `<stdin>`; it is read even when stdin is
  `/dev/null`, and stays distinct from a file named `-`.
- `sysml-lsp` parses its command line with the `flag` package, so `-version`
  works and an unreadable flag is a usage error rather than protocol mode.

### Editor support

- `textDocument/semanticTokens/full` and `/range` are implemented, over a new
  `internal/core/highlight` package, and `textDocument/codeAction` answers
  quick fixes carried as structured edits from the layer that reported the
  diagnostic — a located semicolon, a near-miss spelling, an importable
  namespace. Token deltas are not implemented and are not advertised.

### RDF interoperability

- The members that state a condition — a constraint body's conditions, a
  requirement's assumptions and required conditions, a subject and a result —
  have a mapping, so converting a model with a constraint no longer aborts.
  Conditions are carried as `sysx:condition` notation, as every
  expression-valued position in this mapping is.
- Turtle written back as SysML spells the notation: an unrestricted name gets
  its quotes, so a model with a quoted name re-parses.

### Python bindings and `sysml-grpc`

- `sysml-grpc` loads the standard library ahead of the requests that need it
  instead of once per model: the service keeps a small pool of prewarmed library
  indexes, and a model the service has not seen adds its document to one of them
  rather than loading and expanding the library again. A cold `ParseFile` on a
  163-line model measures ~0.5–0.9 ms where it measured ~100–128 ms, which is
  what makes a parameter sweep varying the model text practical. What a model
  resolves against is unchanged: an index carries the same library, an index is
  handed out once so cached models stay independent, and an empty pool builds one
  on the request path, so a result never depends on prewarming. Prewarming runs
  in the background, so startup stays prompt, and `SYSML_GRPC_INDEX_POOL` sizes
  the pool (default 4; 0 keeps the previous per-model behaviour).
- The library record cache writes each store to a temp file of its own, where two
  stores of one key shared a fixed `<key>.idx.tmp` path and could publish a
  truncated record that every later start missed on; `Prune` now also clears the
  temp files a crashed store left behind.
- `sysml-grpc -version` reports the metadata the linker sets, where a released
  binary said `version dev / commit unknown`.
- A cached `~/.pysysml/bin/sysml-grpc` records the release and repository it was
  downloaded from beside it, and a cache from another release is replaced rather
  than served. A failed integrity check is its own `ChecksumMismatchError` and
  is never answered from the cache; a download that fails on the network keeps
  the working binary.
- A service already listening is asked what it is and compared against the
  release and capabilities asked for, raising `StaleServiceError` naming the
  remedy instead of a `MissingCapabilityError` on the first newer call. It is
  stopped only when this client started it and no other client holds it.
- `Model` gained `instantiate`, `execute_action` and `execute_state`, so every
  call taking a model hash is reachable on the model it is about. `pysysml`
  0.2.0 carries these.
- `ChecksumMismatchError` is exported from `pysysml`, where it was reachable
  only as `pysysml.errors.ChecksumMismatchError` while every other documented
  exception was on the package.

### Documentation

- The pages are organized by what a reader is doing rather than by the feature
  that landed: a numbered handbook under `docs/guide/`, looked-up material under
  `docs/reference/`, design and internals under `docs/internals/`, and status
  under `docs/project/`. `QUICKSTART.md` and `RDF_INTEROP.md` are split into the
  chapters they were, the guide content stranded in `examples/*.md` and
  `python/README.md` is folded in, and the paths the released README linked leave
  pointers behind. `scripts/check-doc-links.py` gates every relative link and
  heading anchor in CI.

### Release automation

- Release assets are published with `ghr -replace` rather than `-delete`, which
  is an alias of `-recreate`: it deleted the release *and* its tag ref and
  recreated it empty, wiping hand-written release notes, title and the
  prerelease/latest flags on every re-run of the workflow for a tag.
- The Homebrew tap updates itself from a scheduled workflow in
  `Open-MBEE/homebrew-tap`, reading the latest release's `SHA256SUMS.txt`, with
  `scripts/render-homebrew-formula.sh` left as the manual fallback.

### Known limitations

- Converting a model whose behavior is stated as action or state nodes to RDF
  still reports the node and aborts (initial nodes, `perform`, `send`,
  `terminate`, loop nodes, state regions): 71 of the 120 models under `examples/`
  convert.
- A nested feature redefined on an instantiated object is not yet the subject of
  a check or an `%eval`, so those answer about the declaration while `%slots`
  shows the instantiated value.
- A `calc` body written without `return` is not expression-type-checked, so no
  static dimensional warning is reported inside it.
- Submitting a declaration the debugger depends on ends an active `%action` or
  `%state` session; a submission that changes something else carries it over.

## 0.0.7 — 2026-08-15

0.0.6 was tagged from this section before it was cut, so the changes it carried
are listed here rather than under a heading of their own.

### Language and semantics

- Element filters are evaluated: `filter <expr>;` in a package, definition or
  usage body, `import P::*[@T]` on an import, and a filter written at a
  document's root all gate what the names beside them bring into scope. A
  condition is a boolean predicate over one candidate with the candidate as the
  implicit `self` (KerML 8.2.4), so it is judged against a symbol and the
  metadata annotating it — prefix metadata, a metadata member of the body, and
  `metadata m about X` — with conformance through the candidate's supertypes, so
  `@Safety` matches a metadata type specializing `Safety`. A condition the
  evaluated subset does not cover is reported as such
  (`this filter condition cannot be evaluated, so it selects nothing and is not
  applied`) and one that does not yield a boolean is an error, rather than either
  silently selecting nothing. A root filter applies to its own document only, and
  a namespace's filter does not gate lookups made inside its own body.
- `@Safety` parses as the classification expression it is rather than a feature
  reference to the metadata type, which had lost the classification.
- A KerML `class`/`struct`/`assoc`/`behavior`/`predicate`/`interaction`
  declaration is classified rather than left unclassified, so the type checker
  judges it instead of exempting every unclassified usage — a binding's mismatch
  is still reported.
- A condition starting with an expression keyword (`true`, `null`, `if`) survives
  in a parameterised constraint body, where it used to be read as a nameless
  declaration and dropped.

### Runtime

- A fifth runaway bound, `SYSML_MAX_ELEMENTS` (default 1 000 000), bounds the
  collection elements one evaluation holds rather than the work a run does: an
  element is a 104-byte `Value` living as long as the collection holding it, so
  the default holds ~104 MB of them, in the band the other defaults were sized
  against. Every materializing path is charged — a range, a sequence literal,
  `->collect` and the other collection operations — and exceeding it is
  `ErrElementLimitExceeded` naming the variable, not the step limit: `1..10000000`
  used to conjure ~1 GB before the step budget reported it. A statement releases
  what it materialized, so a loop building a small collection each iteration is
  bounded by what it holds rather than by what it has produced in total.
- An action node's body ends the activation it ran in, so a run stepping the same
  body many times no longer holds what every execution's calc usages computed.
- A calc usage declared in an action's body or among a state machine's members
  binds its inputs from the values the behavior has reached, as one in a calc's
  body does: `calc t : Twice { in k = v; }` after `assign v := 2.0` reads 2.0
  rather than the value `v` was declared with.
- An evaluation outside a body — a decision or transition guard, a change
  condition or duration, an inline node expression, an attribute or slot default,
  an action argument, a constraint check — runs in a scope of its own, so what a
  calc usage answers it and the elements a collection it evaluates materializes
  live no longer than the step. A decision revisited after its body assigned
  reads the usage again over those values instead of the first evaluation's
  result, and a long run whose guard builds a small list is bounded by what it
  holds rather than stopped as a runaway. Reads within one step still share the
  scope, and a read through a part's feature chain belongs to the evaluation
  making it.
- `%budget` prints the five bounds a session runs on with the variable that
  raises each, and a literal expression that spends one is answered with that
  failure instead of "no declarations loaded".
- An action flow ends at a node with no succession, so an action whose last node
  is a plain nested action reaches `Completed` instead of failing the run with
  `nested action b has no successors`.
- `first s1 then s2;` starts the flow at `s1`, the node it names, rather than at
  an initial node of its own whose only edge reached `s2`: `s1` used to be
  skipped, losing what its body assigned, while the run still reported
  `Completed`. Written apart as `first s1; then s1 s2;` it behaves the same. A
  body states one start, so a `first` end naming the body's final node — a flow
  that would end where it starts — is now rejected rather than reported
  `Completed` with the declared node never run.
- A performance holds its values in one feature space its tokens share, because a
  fork duplicates control and not values: concurrent branches are steps of the
  one performance, so both branches' assignments survive where the last token to
  retire used to overwrite the others. Which write decides a feature two branches
  both assign is step order, stated in `docs/project/spec-compliance.md`.
- A runtime failure names SysML kinds and operands rather than Go types, a
  recursion reports a frame count and names the calc it collapsed, and a division
  by zero is reported as one.

### `sysml` command line

- **Breaking:** conversion is spelled `sysml model.sysml -convert ttl`: the model
  is a positional argument as it is in every other mode, and `-convert` names the
  format to convert it to. `-convert <file>` and `-to <format>` are gone — `-to`
  reports the replacement rather than "flag provided but not defined" — and the
  output path no longer chooses the format, so `-o /dev/null` or a FIFO needs
  nothing extra. `-from` still names an input format the extension does not.
- A flag may be written after the model it applies to (`sysml model.sysml -trace`),
  which Go's flag package would otherwise read as two files to load.
- A model is checked from a script or a build step without a prompt:
  `-validate`, `-constraint`, `-requirement`, `-satisfy`, `-instantiate`,
  `-calc`, `-action`, `-state -advance` and `-json`. The verdict comes from the
  runtime rather than from a printed line — one evaluation stands behind both the
  command and the prompt's `%constraint`/`%requirement`/`%satisfy`.
- **Exit status is meaningful on every path**: `0` when the requested operation
  succeeded and every requested check held, `1` when a check answered false, `2`
  when nothing was decided — a model that did not analyse, an expression that
  could not be evaluated, an unreadable input, a misused flag. A check is gated
  on analysis, so a verdict is never reported about a model nobody could read.
- Findings and diagnostics go to **stderr** and requested output to **stdout**,
  under one `sysml: ` prefix, so a pipeline consumes results and a log carries
  failures. Requested help (`-h`) is stdout and exit 0; an unknown flag stays
  stderr and exit 2. The interactive prompt is unchanged.
- A directory or a glob loads as a multi-file project — `sysml <dir>`,
  `sysml 'src/*.sysml'`, `%load <dir>` — expanded, sorted and deduplicated, and
  submitted as one submission, so resolution does not depend on load order.
  Diagnostics are reported against the file they came from at that file's own
  line numbers, and only model files among a glob's matches contribute.
- `-cpuprofile`, `-memprofile` and `-memstats` profile a load or a run.

### REPL

- A session no longer loses state silently. Re-typing a namespace merges into the
  one already in the buffer instead of replacing its body, additions are laid out
  where they belong, and every declaration, instance or debugging session that a
  submission did drop is reported, naming the submission that ended it.
- An instance and an active `%action`/`%state` session survive a submission that
  did not change what they depend on: an object whose declaration identity and
  resolved shape are unchanged is rebound into the new context, keeping its
  identity, its derived values, its connector ends and a selected variant, and
  only genuinely invalidated state is dropped with a notice. Declaring an
  unrelated `part def B;` no longer discards the instance of `A`. A surviving
  debugger keeps the executor it was started with rather than being re-lowered.
- The library is discoverable at the prompt: `%search <substring>` and
  `%builtins`, Tab completion over meta commands, symbols and paths, the nearest
  spelling of a mistyped command or symbol, and history kept outside the
  temporary directory.
- The diagnostic wording agrees with the other surfaces: `%eval`
  reports one parser diagnostic with a position and a caret rather than a cascade,
  an empty session no longer answers a real failure with "no declarations
  loaded", a blocked check names the line the unresolved error sits on and says
  so once, and a caret is drawn only for what was typed, counted in printed cells
  so multi-byte source stays aligned.
- `-satisfy` with no satisfaction assertion in the model is an undecided verdict
  like its siblings, so the command reports
  `sysml: no satisfaction assertion in the session` and exits 2.

### Editor support

- A first-party VS Code extension lives in `editors/vscode`: TextMate
  highlighting for `.sysml` and `.kerml`, comment/bracket configuration, and an
  LSP client that launches `sysml-lsp` from `systemica.server.path`, a
  workspace's `bin/sysml-lsp`, or `PATH` — highlighting still works when no
  server is found. It is built and side-loaded from this repository
  (`make vscode-package`) and is published to no marketplace. The grammars are
  generated from `internal/core/lexer.Keywords()` and a Go test fails when the
  committed ones are stale, so highlighting cannot drift from the lexer.
- LSP completion is typed and context-aware: items carry the kind, detail
  (`partUsage : Vehicle`) and documentation that hover shows, `v.` offers the
  members of `v`'s type — inherited ones included — and nothing else, `Pkg::`
  offers that namespace's members, and the standard library's top-level names
  are offered alongside the ones in scope. Prefix filtering stays on the client.
- `sysml-lsp` serves a session over one reader: it used to start a second read
  loop over its own stdio, so an editor's traffic raced two decoders and the
  server died with corrupted framing ("missing Content-Length header") within
  seconds of typing.
- Completion applies the element filters in force where the name is being
  completed, and resolves a filter condition's own names unfiltered, so the
  editor offers what the document can actually reach.

### Diagnostics

- An unresolved name carries the nearest spelling on **every** surface — command
  line, prompt and editor — where the hint used to exist only at the prompt:
  `unresolved reference: Whel — did you mean Wheel?`. A bare library name is
  offered its qualified spelling (`Integer` → `ScalarValues::Integer`), since the
  base library is not implicitly visible; the shipped examples import what they
  use, and every `examples/*.sysml` and `examples/*.kerml` now analyses cleanly.
- Candidates are ranked by how the reader would reach them, not by edit distance
  alone: the budget scales with the typed name's length (a name of two characters
  is not guessed at), a spelling as typed beats one differing in case, a name in
  scope beats one reachable only by a path, the reader's own declaration beats a
  bundled library one, a dominated candidate is dropped, and at most three are
  offered. A misspelling is not sent to a name nested in another element's body,
  which would take two corrections — so `Whel` beside your own `Wheel` offers
  `Wheel` alone, and with nothing close in the document it offers nothing rather
  than `SysML::Systems::TriggerKind::when`.

### Python bindings and `sysml-grpc`

- `Instantiate` returns every instance reachable from the root, so a Python caller
  expands a composite slot (`inst.engine.power`) instead of holding a bare instance
  id, and a slot the service could not evaluate is reported in `SlotValue.error`
  rather than as a null value. On the client, slot values convert to Python
  scalars, lists and nested `Instance`s, with the raw protobuf still reachable
  through `get_slot()`/`raw_slots`; attribute and item access raise
  `AttributeError`/`KeyError`/`SlotError` rather than returning `None`. (#110)
- `python -m pysysml.generate` emits a Python class per SysML definition —
  properties that carry the static type and perform the runtime delegation, so an
  editor completes `inst.mass` and a type checker rejects `inst.mas`. `GetSymbol`
  reports the type facts this needs (`type_info` with primitive reduction,
  `multiplicity`, all specialization edges), `pysysml` ships `py.typed`, and
  emission is deterministic so the output can be committed. (#111)
- `GetServerInfo` reports the service's build version and the capabilities it
  supports by name, so a client can require a capability instead of comparing
  version strings — versions of source and forked builds are not comparable, and a
  service that predates the RPC answers `UNIMPLEMENTED`, which is itself the
  answer. Typed generation requires the `type_facts` capability and fails naming
  the service in use, where it came from and how to replace it. Against a service
  without it, every generated feature was typed `object`, indistinguishable from a
  feature that is genuinely untyped: the v0.0.5 `sysml-grpc` predates `type_info`,
  so a caller letting `pysysml` download the released binary silently got a
  useless module.
- A generated module records the model source hash and the generator's emission
  schema, and `pysysml.generate --check` regenerates in memory and exits non-zero
  when the committed module is missing or would change, writing nothing — a stale
  module was previously found at attribute access, or never, since a feature
  removed from the model keeps type-checking.
- `TypedObject.from_instance` rejects an instance of another definition, naming
  both types, instead of accepting it and failing later with a confusing
  `TypeMismatchError` on the first slot read. An instance of a definition that
  specializes the expected one is accepted; an instance whose type no generated
  class describes is accepted too, because instantiating a usage reports the
  usage's own FQN, which the client cannot relate to a definition.
  `unchecked(instance)` is the explicit escape hatch.
- `Convert` writes a model back out — SysML/KerML notation or RDF Turtle, from a
  loaded model named by its `model_hash`, a path the service opens, or content
  carried inline — using the same exporter
  `sysml -convert` uses, so a Python caller round-trips a model instead of only
  reading one: `model.to_sysml()`, `model.to_turtle()`, `model.save("m.ttl")` and
  `pysysml.convert(...)`. Reported as the `convert` capability, so an older
  service fails naming the upgrade. A conversion that cannot be written faithfully
  returns the diagnostics that explain it as a `ConversionError` rather than
  partial output; `tolerate_syntax_errors` writes notation anyway and is rejected
  for the graph directions, where an unparsed declaration would vanish silently.
  A `Model` converts by hash, so a file edited between `load` and `save` does not
  change what is written; a model since evicted from the service cache is
  `NOT_FOUND` rather than something else, and `convert(file_path=...)` is how a
  caller asks for the file as it stands now.
- `ParseFile` hits its cache on the source it read — file name and content —
  rather than on the `content_hash` the request carried, which is now ignored:
  `pysysml` never sent one, so re-loading unchanged content re-parsed it and
  reloaded the standard library every time — ~35 ms where the cache costs
  ~0.5 ms — and a hash disagreeing with its content would have served an
  unrelated model.
- `python/scripts/bench_latency.py` reports p50/p95/p99 per client call, and
  `python/README.md` documents the measurements and what they mean for a real-time
  analytics loop.
- `Model.eval(expression, context_symbol_id=...)` evaluates against the model it
  is called on, so evaluation is no longer the one operation making a caller carry
  the hash back to the connection: `model.eval("1+1")` for
  `conn.eval("1+1", model.hash)`. The typed failures are the connection's —
  `ExecutionError` for an expression that cannot be evaluated, `ModelNotFoundError`
  for an evicted model.
- Naming an element of the wrong kind raises `WrongKindError` (an
  `ExecutionError`) from `verify_constraint`, `verify_requirement`,
  `verify_satisfaction` and `calc`, as naming an element that does not exist
  already did: verifying a part def as a constraint used to answer with a verdict
  whose `holds` was false, telling a caller its model does not hold when the
  answer was that it named a part def. The service reports the distinction as a
  typed `FailureReason` on `Verdict`, `VerifySatisfactionResponse` and
  `EvaluateCalcResponse`, so the client classifies it without reading the message
  text.
- A `host:port` address given as the host is read as one, on `connect` and on the
  module-level helpers taking `host`/`port`: `connect("localhost:50123")` reaches
  port 50123 instead of building the target `localhost:50123:50051` and reporting
  a service start timeout for an address nobody asked for. A port named twice with
  two values, and a port that is not a number, raise `ValueError` naming the
  mistake. The `pysysml.generate` and `bench_latency` command lines report a
  host/port disagreement as an `error: …` line and exit 2 rather than as a
  traceback.
- A `Query` RPC evaluates the SysML v2 API & Services query model (scope /
  select / where, primitive and composite constraints) over the symbol index and
  semantic model, with `model.query()` accepting the standard's JSON payloads
  verbatim. The standard's model has no traversal or transitive closure, so this
  is an interop surface for its clients rather than a query language;
  `docs/reference/api.md` and `docs/project/spec-compliance.md` state what is supported. An
  element with no qualified identity — a doc note, an anonymous usage, a
  `connect` — is omitted rather than answered under a non-unique `@id`.

### Performance

- Loading a large model is linear where it was quadratic: three lookups scanned a
  namespace's members or child scopes once per member. Child scopes are indexed
  by the declaration owning them, a namespace's imports are memoized, and a
  member's owner is found through the scope's owner link.
  `docs/internals/performance.md` records the measurements.
- `ParseFile` hits its cache on the source it read, so re-loading unchanged
  content costs ~0.5 ms instead of re-parsing and reloading the standard library
  (~35 ms).

### Removed

- `internal/core/deps` — the `sysml.toml` manifest, lockfile, git fetcher and
  resolver — is deleted. Nothing imported it: no manifest was ever looked for by
  the command line, the prompt or the server, and the README claim it backed is
  gone.

### Documentation

- `README.md`, `docs/guide/` and `docs/reference/cli.md` describe the
  shipped command line, editor and RDF surfaces, including the exit-status
  contract and the streams each finding is written to. The claims that overstated
  what ships — dependency management, and the IDE and Python verification
  caveats — are corrected.

## 0.0.5 — 2026-08-12

### Language and semantics

- A requirement or constraint condition is evaluated against the features of the
  element stating it, so it sees that element's own attributes, the ones it
  inherits from the definition it is typed by, and the values a usage rebinds
  (`attribute :>> maxVerticalSpeed = 1.5;`, `constraint limit : MassLimit { in m = mass; }`).
  This was the first known limitation listed for 0.0.4.
- `require <expr>;` and `assume <expr>;` parse in a requirement definition body,
  not only in a usage, as do the `concern def`, `viewpoint def` and
  `satisfy … by …` bodies that share the member set. A `subject` may redeclare
  the one it inherits (`subject subj : View[1] :>> RequirementCheck::subj;`).
- A condition stated through a nested constraint — `require constraint { <expr> }`,
  `assert constraint [name] { <expr> }` — is evaluated, with every condition of
  that body kept rather than only the last. A requirement carrying no condition
  still has no verdict rather than passing vacuously.
- A violated condition reports which condition failed
  (`Required condition evaluated to false: actualVerticalSpeed <= maxVerticalSpeed`),
  and a feature a condition names but which holds no value is reported as such
  rather than as unresolved.
- A quantity expression (`attribute maxVerticalSpeed = 1.5 [m/s];`) is evaluated.
  A quantity carries its magnitude and the measurement reference it is written
  in, as `Quantities::ScalarQuantityValue` (`num` + `mRef`) does. Units reduce to
  a scale factor over base units through the Quantities and Units library's own
  `unitConversion` and unit-defining expressions, so commensurable units convert
  before a comparison or a sum — `1.5 [m/s] <= 5.4 [km/h]` is true, exactly, at
  its boundary — and an operation whose unit is composed by it keeps that unit
  (`10 [m] / 2 [s]` is `5 [m/s]`, `4 [m] / 2 [m]` is `2`). An operation between
  units that measure different things (`1.5 [m/s] <= 2.0 [s]`) is an error, never
  a comparison of bare magnitudes that would equate `1.5 [m/s]` with
  `1.5 [km/h]`. A cached library record carries the unit reduction of its
  symbols, so its key now covers the digest of the whole library set: the
  reduction follows a prefix or reference unit declared in another file, and a
  key over one file's content alone kept converting with the factors of an
  edited `SYSML_LIBRARY_PATH` library's old definitions. A record no load has
  hit for 30 days is pruned, since a wider key leaves more records that nothing
  will look up again.
- `assert satisfy <requirement> by <part>;` has a verdict of its own: the
  assertion is evaluated as the requirement usage it is, with the requirement's
  subject parameter bound to an object of the part named by `by`, so the
  requirement's conditions — its own and the ones it inherits — read that
  object's values. A requirement feature carrying no value of its own is read
  from that object's feature of the same name, as it is when a requirement is
  evaluated on an instance.
- `%satisfy` evaluates the satisfaction assertions a model states — every one, or
  the ones a named element states — since `assert satisfy … by …` is anonymous
  and could not be named at the prompt before.
- An assertion can be negated: `assert not constraint { <expr> }` and
  `assert not satisfy <requirement> by <part>;` hold exactly when the conditions
  they deny do not, rather than parsing as a declaration named `not`. A negation
  denies the conditions of the constraint it is written on together — `not (a and
  b)`, not `not a and not b` — so it holds as soon as one of them fails.
- The KerML function library's scalar numeric functions are evaluable: `sqrt`,
  `abs`, `floor`, `round`, `max`, `min`, `isZero`, `isUnit`, `sin`, `cos`, `tan`,
  `cot`, `arcsin`, `arccos` and `arctan`. Dispatch is by the declaration's
  qualified name, so a model's own `calc sqrt` is evaluated from its body.
- Exponentiation (`**`, `^`) is evaluated, by one implementation the constant
  folder and the runtime share. Integer operands with a non-negative exponent
  give an Integer, any other numeric pair a Real.
- `exp`, `ln`, `log(x, base)` and `atan2(y, x)` are evaluable. The OMG Kernel
  Function Library declares no signature for any of them, so they are declared in
  a new non-normative Systemica extension library,
  `internal/core/libs/stdlib/Systemica Libraries/SystemicaMathFunctions.kerml`,
  which a model reaches with `import SystemicaMathFunctions::*;`. The vendored OMG
  files are unchanged; the stdlib parse gate is now 95/95 clean.
- A result that is not a finite value of the declared type — `sqrt(-1.0)`,
  `arcsin(2.0)`, `ln(0.0)`, `log(x, 1.0)`, `atan2(0.0, 0.0)`, `0.0 ** -1.0`,
  integer overflow — is reported where the expression is evaluated instead of
  folding to a NaN, an infinity or a wrapped integer.
- A unit written unqualified is resolved through the imports in scope, and a name
  a declaration shadows is reported with the declaration that shadows it and the
  way out: `m resolves to the attributeUsage m declared in SH, shadowing the
  measurement unit SI::metre — write SI::m to name the unit`. The rule and the
  message are the same wherever a quantity is evaluated — a part's attribute, an
  action or state body, a calc invocation, a constraint or requirement condition,
  and an expression typed at the prompt.
- A quantity value renders like the bare Real it measures, magnitude first and
  unit in brackets (`v = -15.20 [m/s]`, `= 5.00 [SI::m/SI::s]`), in results,
  execution traces and slot listings alike, in place of full float precision.
- `%calc` accepts quantity arguments in every argument form it accepts numbers
  in — comma-separated, whitespace-separated, invocation form, and a
  parenthesized subexpression — so `%calc P::Fall 10.0 [m/s], 3.0 [s]` invokes
  the calculation rather than reading the bracket as a sequence index.
- An attribute default written in an action or state body is evaluated in the
  scope that declares it, so a unit or a type an enclosing package imports
  resolves there (`attribute h : LengthValue = 500.0 [m];`), and a body-local
  name is not resolved against the namespace the session happens to be in.
- The loop and conditional statements of an action body execute: `while`, `loop`,
  `for … in …` and `if … { … } else { … }` lower to real decision and merge
  nodes, so a body that iterates reaches its final node with the values it
  computed rather than deadlocking.
- A `then` written as a member of a body (`then loopIt end;`, `then start
  compute;`) is a succession edge in the lowered graph, like the standalone form,
  rather than a member the runtime had no edge for.
- A relationship written with a keyword and one written with its symbol are the
  same relationship end to end — `specializes`/`:>`, `subsets`/`:>`,
  `redefines`/`:>>`, `references`/`::>` — so hover, go-to-definition, completion
  and the index report the same thing whichever spelling a model uses. A feature
  that takes its effective name from what it redefines is read under that name by
  the same paths, including its short name.
- `assert satisfy <requirement> by <part>;` parses with the `by` subject named
  by a qualified name, and an action body that is not braced ends where its
  statement ends rather than swallowing the member that follows it.

### Runtime and tooling

- A parser try-parse that gives up now rewinds: the token buffer is read through
  a cursor rather than re-sliced as tokens are consumed, so a checkpoint restores
  the position it was taken at, along with the diagnostics and warnings the
  abandoned attempt reported. Backtracking previously left the words the attempt
  had consumed behind, which made a condition beginning with a feature named
  `constraint` (`assert constraint x > 0;`) report a missing expression it did
  have, and could exceed the buffer's capacity. A reserved word used as a name
  in an expression still does not resolve; that is a separate gap.
- `redefines <target> = <value>` is read whatever the target's length: the member
  is recognized by parsing the target and rewinding when no `=` follows, in place
  of a scan capped at ten tokens ahead that read
  `redefines outer.middle.inner.leaf.deeper.deepest.last = 1;` as a body member
  it could not parse.
- The evaluation step budget is configurable through `SYSML_MAX_STEPS`, so a
  legitimately long run — a numeric integration in an action body, say — is not
  bounded by a fixed ceiling. A value that is not a positive integer is reported
  at REPL/CLI startup and at gRPC service construction, naming the variable and
  the value, rather than falling back to the default silently.
- The step-limit error reports the budget actually in force and names
  `SYSML_MAX_STEPS`, so the message says how to raise it.
- The three sibling runaway bounds are configurable the same way, each through
  its own variable, since they count incommensurable units: an action run's
  token-flow steps through `SYSML_MAX_ACTION_STEPS`, a state machine run's
  dispatched events through `SYSML_MAX_EVENTS` and its do-activity actions
  through `SYSML_MAX_DO_STEPS`. Each error names the
  variable that raises it, so a long simulation is no longer capped by a bound
  with no way out.
- The defaults are raised to 10 000 000 evaluation steps, 1 000 000 action
  token-flow steps, 1 000 000 events and 5 000 000 do-activity steps (from
  100 000 / 10 000 / 10 000 / 100 000). Execution allocates nothing per step —
  peak RSS is ~34 MB whether a run spends ten thousand steps or fifty million —
  so the sizes are set by how long a runaway takes to report: at ~13.6M
  evaluation steps/s and ~1.9M events/s each reports one within about a second,
  and a fully traced run at all four ceilings holds ~320 MB.
- The evaluation step budget bounds one run rather than a whole session: the
  counter is reset when a run begins - an evaluation, a constraint or
  requirement check, an instantiation, a calc invocation, an action or a state
  machine - so a REPL session of many small evaluations no longer exhausts its
  allowance and starts failing every one. A run started inside another shares
  the outer run's budget, as does every call into a run a caller drives step by
  step (the `%action`/`%state` debuggers), so a runaway cannot escape the bound
  by starting runs of its own.
- The REPL's `%advance` no longer stops after a fixed 10 000 events and
  do-activity actions, which could look like a machine that had settled. It is
  bounded by the session's event and do-activity budgets, and says which one cut
  a drain short.
- A session is written out: `%save <file>` writes the notation (`.sysml`) or the
  RDF graph (`.ttl`) chosen by the extension, atomically, replacing an existing
  file and saying so. A session that does not fully parse still saves as
  notation — the text as typed, re-indented, with the syntax errors reported as
  warnings — so work is never trapped in the REPL; `.ttl` keeps the refusal,
  since a graph built from a partly recovered tree would be quietly missing
  declarations.
- `sysml -convert` converts a model between the notation and RDF Turtle in both
  directions, round-tripping packages, definitions, usages, features, imports,
  connectors, successions (including a `then` written as a body member) and
  satisfy assertions. What the mapping normalizes and what it refuses is
  documented in [docs/reference/rdf-mapping.md](docs/reference/rdf-mapping.md); a refused construct
  is reported with its node and position rather than dropped.
- Removing a document unwinds what its wildcard re-exports contributed, so a
  name a removed file re-exported no longer resolves, and the workspace reuses
  the index slot the document held rather than growing one per edit — an editing
  session's memory no longer climbs with the number of reindexes.
- The `sysml-grpc` service binary is published with the release, one per
  platform (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
  windows/amd64), each with a `.sha256` sidecar and covered by
  `SHA256SUMS.txt`. `pysysml` downloads the binary matching the release it is
  told to use, verifies it against its sidecar, caches it under `~/.pysysml/bin`
  and starts it — so a Python caller needs no Go toolchain. (v0.0.4 described
  this; the release assets it publishes do not include the binaries, and this is
  the first release that does.)
- Homebrew installation is live: `brew install Open-MBEE/tap/systemica` installs
  both binaries from the published bundle, verified by checksum, and avoids the
  macOS quarantine prompt that a browser download sets. See
  [packaging/homebrew/README.md](packaging/homebrew/README.md).
- `pysysml` is published to PyPI by CircleCI, on its own `pysysml-v*` tag, from
  one declared version (`python/pysysml/_version.py`) that the packaging
  metadata and `pysysml.__version__` both read — a tag that disagrees with it
  fails the job before anything is uploaded. `python/setup.py` is gone;
  `pyproject.toml` declares the build. See
  [docs/project/releasing.md](docs/project/releasing.md#releasing-opensysml-to-pypi)
  (that section, and the package, are named `opensysml` since the rename).

### Known limitations

- RDF/Turtle conversion has no mapping for a member that states a condition or a
  step, so a model containing one is reported rather than converted: `require`
  and `assume` members, a constraint body's condition, `subject`, a computed
  `return`, `assign`, `if`/`while`/`loop`/`for`, substates, transitions and
  `entry`/`do`/`exit` (`cannot convert the *ast.RequireMember at <file>:<line>`).
  A requirement stating a condition, and any state machine or action body with
  statements, must be saved as `.sysml`. The full list is in
  [docs/reference/rdf-mapping.md](docs/reference/rdf-mapping.md).
- The REPL's prompt evaluates in the *last* namespace the session declared. After
  typing a second package, the first package's members and the units its imports
  brought in are reached by qualified name only (`1.0 [SI::m]`, not `1.0 [m]`).
- Re-typing a declaration whose name the session already holds replaces the
  earlier snippet rather than merging into it, so adding a member to a package by
  re-typing the package drops the members left out of the new text.
- A multi-valued feature that is both typed and given a default takes the typed
  instantiation; the default is not merged into it (as in 0.0.4). *(Fixed after
  this release; see 0.0.8.)*
- An attribute declared with a type but no value (`attribute diameter : Real;`)
  instantiates as an object of that type rather than an unset value, so `%slots`
  shows `diameter = Instance(ID: n)` with `(no features)` under it.
- The macOS and Windows binaries are unsigned, so a browser download is
  quarantined by Gatekeeper or flagged by SmartScreen. Install with Homebrew or
  `curl`; see [docs/project/macos-distribution.md](docs/project/macos-distribution.md).

## 0.0.4 — 2026-08-10

The first tagged release.

### Language and semantics

- Hand-written SysML v2 lexer and recursive-descent parser that never panics:
  malformed input yields error nodes and diagnostics. All 94 official SysML v2
  standard library files parse clean.
- Lazy, memoized name resolution and a type system covering conformance,
  multiplicity, specialization, redefinition and feature chains. An unnamed
  feature takes its effective name from what it redefines or reference-subsets
  (`:>> power = 250.0;` names, and overrides, `power`), and a nested usage that
  reuses an inherited name without redefining it is reported.
- Tiered validation (syntax → name resolution → typing → constraints), where a
  failing tier suppresses the ones above it rather than reporting noise.
- Measured spec compliance, rule by rule, in
  [docs/project/spec-compliance.md](docs/project/spec-compliance.md); 98/100 of the OMG
  training corpus parses and analyzes clean, with the two remaining files
  pinned as upstream source bugs.

### Execution

- Instantiation materializes objects from part definitions: literal defaults
  are folded, defaults written over sibling features are evaluated against the
  object, and a cyclic default is reported as a cycle rather than exhausting
  the step budget.
- Constraints and requirements are evaluated bound to a concrete instance, so a
  verdict is about an object rather than about a declaration. A false condition
  is a verdict, not an internal error.
- Action execution over lowered graphs: tokens, fork/join/decision/merge,
  control-flow keywords, nested invocation, `send`, and `accept` that suspends
  until its message arrives.
- State machine execution: transitions, guards, entry/do/exit behaviors,
  hierarchy, orthogonal regions, pseudostates including shallow and deep
  history, time and change and call triggers, deferred events.
- Every unsupported path returns a typed error; robustness cases cover
  deadlock, dangling transitions, unbound parameters, and budget exhaustion.

### `sysml` REPL

- Declarations, instantiation and inspection: `%instantiate`, `%slots`,
  `%instances`, `%eval`, `%calc`, `%constraint`, `%requirement`. Every command
  accepts a qualified name, so a model inside a `package` is reachable.
- Action debugging (`%action`, `%step`, `%continue`, `%tokens`, `%break`,
  `%stop`) and state machine debugging (`%state`, `%events`, `%current`,
  `%advance <time>`).
- Output modes: `-quiet` reports errors only, `-debug` widens diagnostics to
  the whole session buffer with absolute positions and originating pass, and
  `-trace` prints the execution trace — evaluation steps, calc invocation,
  token flow, transitions — as commands run. `%verbosity` and `%trace` are the
  prompt equivalents.
- A submission's report covers what was just typed: diagnostics are scoped to
  it and line numbers are relative to it.
- `sysml -e '<expr>'` evaluates without entering the prompt; `--version`
  reports the release tag, commit, build time and Go version.

### Tooling

- `sysml-lsp`, a Language Server Protocol server (diagnostics, hover,
  completion, go-to-definition).
- `sysml-grpc` plus Python bindings (`python/`) for driving parse and execution
  from a notebook, including DataFrame output. `Instantiate` reads slots the way
  the REPL does, so a derived attribute comes back evaluated rather than
  unmaterialized. The service binary is published with the release, so `pysysml`
  can fetch and checksum-verify one instead of requiring a Go toolchain:
  `download_binary('latest')`, or set `PYSYSML_GRPC_VERSION` and let
  `pysysml.connect()` start it.
- Releases publish per-binary and bundle archives, and the raw
  `sysml-grpc-<os>-<arch>` binaries with `.sha256` sidecars, for linux/amd64,
  linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64, with
  `SHA256SUMS.txt` over all of them. macOS and Windows binaries are unsigned —
  see [docs/project/macos-distribution.md](docs/project/macos-distribution.md).

### Known limitations

- A parameter bound by a constraint or requirement usage
  (`constraint limit : MassLimit { in m = mass; }`) is not passed into the
  conditions it inherits from its definition. *(Fixed after this release; see
  0.0.5.)*
- A multi-valued feature that is both typed and given a default takes the typed
  instantiation; the default is not merged into it. *(Fixed after this release;
  see 0.0.8.)*
