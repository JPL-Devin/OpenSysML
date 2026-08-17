# Changelog

Notable changes per release. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Cutting a release
is described in [docs/project/releasing.md](docs/project/releasing.md).

## Unreleased

### `%slots` is now `%features`, the name SysML v2 uses

- **`%features <name>` lists what an object holds for each feature of its type**, which is what
  `%slots` listed. "Slot" is UML/SysML v1 vocabulary (`InstanceSpecification::slot`); the v2/KerML
  pair for this concept is `Feature` and `FeatureValue`, and the listing's heading now reads
  `Features:` to match. `%instantiate` points at the new spelling too.
- **`%slots <name>` still works**, as a deprecated alias: the same listing, led by
  `note: %slots is deprecated — use %features`. Nothing else about it changed — the nested
  expansion, its bounds, the error lines and the exit status a non-interactive run takes from a
  feature value that could not be materialized are all as they were.
- **The vocabulary behind the command is v2 too.** What was printed and named as a "slot" is now a
  *feature value* (`FeatureValue`, `KerML.kerml`), and a state's `do` behavior is a *do action*
  (`States.sysml`) rather than a "do activity":
  - Message text changed: `feature value craft.volumes: multiplicity violation: …`,
    `cyclic feature value dependency`, `uninitialized feature value`,
    `no errors in the feature values checked`, and
    `state machine exceeded max do action steps (…)`. The `%budget` label reads `do action steps`.
    `SYSML_MAX_DO_STEPS` and every exit status are unchanged, but a script matching the old text
    needs updating.
  - The gRPC interface gained `Instance.feature_values` (`FeatureValue`) and the
    `feature-values` capability. The deprecated `Instance.slots` (`SlotValue`) is still populated
    identically, so an existing client keeps working.
  - `pysysml` gained `Instance.features`, `raw_features`, `get_feature`, `FeatureValueError`, and
    `typed.feature_value`/`optional_feature_value`/`list_feature_value`, which generated modules now
    emit (emission schema `3`). `slots`, `raw_slots`, `get_slot`, `SlotError` and the `slot`
    decoders remain as deprecated spellings, and `SlotError is FeatureValueError`.
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

## 0.1.0 — 2026-08-17

### RDF conversion is experimental

- SysML ↔ RDF Turtle conversion is labelled **experimental**, in both directions, on every
  surface that offers it. The mapping covers model structure only — 71 of the 120 models under
  `examples/` convert and the other 49 are refused with the construct named — its vocabulary
  may change without a compatibility path, and no round trip through a running triplestore has
  been demonstrated (roadmap D1–D3, D6). Saving and converting notation (`.sysml`, `.kerml`) is
  stable and unchanged.
- `sysml -convert` writes the status as a `note:` on **stderr**, so a conversion piped to a file
  or to stdout carries no extra bytes, and a refused conversion is labelled too. `-help` says
  the same.
- `%save model.ttl` prints the status before it writes, including when the model is refused.
- `ConvertResponse` carries `experimental` and `experimental_notice`, set before the conversion
  runs, so a client reads the status off a refusal as well as off a success. The `convert`
  capability is unchanged: the status is per conversion, not per service.
- `pysysml` raises the status as `ExperimentalFeatureWarning` and exposes it as
  `Conversion.experimental`/`.experimental_notice`, plus `pysysml.is_experimental(from, to)`.
  A service too old to send the fields is read from the formats it reports instead, so an RDF
  conversion warns either way. Silence it with
  `warnings.simplefilter("ignore", pysysml.ExperimentalFeatureWarning)` — no stable feature
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

### Release process

- The CircleCI pipeline that builds release tags downloads the OMG training corpus and runs the
  suite with `SYSTEMICA_REQUIRE_TRAINING_CORPUS=1`, so the corpus gate can no longer skip
  silently where a tag is cut. 0.0.9 listed that as a known limitation; it is closed.

### Known limitations

- Everything 0.0.9 listed still stands, with the RDF limitation now stated as a feature status
  rather than a footnote: expressions are not emitted as triples, a model whose behavior is
  stated as action or state nodes is refused, and end-binding heads depend on
  `sysx:sourceText`.

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
  [docs/project/releasing.md](docs/project/releasing.md#releasing-pysysml-to-pypi).

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
