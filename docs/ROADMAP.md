# Systemica — Roadmap

Baseline: `main` @ `a6c5fd8`, verified locally on 2026-08-11 with Go 1.25.0.
Read `AGENTS.md` first; it governs everything below.

0.0.4 is prepared but **not tagged**. Everything in "Release follow-through" is
maintainer- or account-gated; everything after it is ordinary engineering work.

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`, `staticcheck ./...`,
`go test -race ./...`.

| Gate | Count |
|---|---|
| OMG training corpus | **98/100 clean** — 2 files / 4 errors, both pinned OMG source bugs (the ceiling) |
| Stdlib parser conformance | 95/95 clean — 94 vendored OMG files and 1 non-normative Systemica extension |
| Execution conformance cases | 78 |
| gRPC conformance cases | 6 |
| Golden execution traces | 37 |
| Runtime robustness subtests | 43 |
| Golden AST fixtures | 42 |
| Negative parser subtests | 49 |

Statement coverage, measured today with `go test -cover ./...`:

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/format` | 96.6% | `internal/core/model` | 79.5% |
| `internal/core/source` | 90.9% | `internal/core/deps` | 76.4% |
| `internal/core/lexer` | 87.7% | `internal/lsp` | 72.4% |
| `internal/core/libs` | 87.8% | `internal/core/lower` | 62.1% |
| `internal/repl` | 86.9% | `internal/core/symbols` | 62.0% |
| `internal/grpc` | 80.1% | `internal/core/parser` | 61.5% |
| `internal/core/passes` | 80.2% | `internal/core/semantics` | 61.1% |
| `internal/core/runtime` | 79.7% | `internal/core/resolve` | 54.3% |
| | | `internal/core/ast` | 27.4% |
| | | `cmd/sysml` | 6.4% |
| | | `cmd/sysml-lsp`, `cmd/sysml-grpc` | 0% |

The corpus gate needs the corpus (`./scripts/download-training-examples.sh`) and never
re-baseline `internal/core/model/testdata/training_examples_expected.txt`: adjudicate each
drifted file and record the verdict in `docs/TRAINING_EXAMPLES.md`.

---

# Release follow-through

## R1 — tag 0.0.4 (maintainer, blocking everything else in this section)

```bash
git checkout main && git pull
git tag -a v0.0.4 -m "v0.0.4"
git push origin v0.0.4
```

The publish job needs `GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project.
Without one the tag builds artifacts and then fails at publish, having created no release.
Nobody has verified which is set. Full procedure and post-tag verification:
`docs/RELEASING.md`.

## R2 — publish `pysysml` to PyPI

`python/pyproject.toml` declares `pysysml` 0.1.0 and nothing has ever been uploaded, so the
`pip install pysysml` promised in `docs/design/python-grpc-bindings.md` does not work; the only
install route is `pip install -e python/` from a clone. Needs: a PyPI project and API token in
CI, a publish job gated on the tag, and a decision on version coupling — the package version
(0.1.0) and the Systemica release it fetches a service binary from (0.0.4) are independent
today, which is defensible but undocumented.

Also decide the default download repository. `python/pysysml/binary.py` defaults to
`Open-MBEE/Systemica`, releases are currently cut from `JPL-Devin/Systemica`, and
`PYSYSML_GITHUB_REPO` is the override. A PyPI package pointing at a repository with no releases
would be worse than no package.

## R3 — Homebrew tap

`packaging/homebrew/` holds a template with `__TAG__`/`__SHA256_*__` placeholders and
`scripts/render-homebrew-formula.sh` renders it from a tag's `SHA256SUMS.txt`. The tap
`Open-MBEE/homebrew-tap` exists and carries the 0.0.4 formula: `brew install
Open-MBEE/tap/systemica` has been verified end to end on Linux (install, `brew test`,
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
`docs/MACOS_DISTRIBUTION.md`: it is `com.apple.quarantine`, not a missing signature — Go's
linker already ad-hoc signs darwin/arm64 — so ad-hoc `codesign` in CI would change nothing.
Notarization needs an Apple Developer account, a Developer ID certificate, an App Store Connect
API key in CI and a macOS runner. Windows needs an OV/EV certificate. Both are purchases, not
tasks.

---

# Track P — the Python/gRPC surface

This is where the release just changed shape: `sysml-grpc-<os>-<arch>` binaries now ship with a
`.sha256` sidecar, and `pysysml` downloads and verifies one, so a Python user no longer needs a
Go toolchain. That makes the following gaps user-visible for the first time.

## P1 — the integration tests skip in CI, and cannot simply be un-skipped

`python/tests/test_integration.py` and `test_runtime_integration.py` (15 tests) skip themselves
unless a service answers on `localhost:50051`. CI installs the binary but starts nothing, so
they skip there too, and the client↔service path has never been exercised by CI.

Starting a service in the `python-test` job is **not** the fix on its own — verified: with one
running, the 15 tests pass but `test_lifecycle.py::test_service_shuts_down_when_last_process_exits`
fails, because `Connection._ensure_service` returns early when it probes a healthy service and
writes no pidfile, so the refcount can never shut that service down and the test cannot find a
pid to watch. Decide the ownership model first — pysysml should probably not kill a service it
did not spawn, and the test should then not require it to — then start the service in CI. The
job's comment in `.circleci/config.yml` records this.

## P2 — a nested object is unreachable over gRPC

`Instantiate` now reads slots through `Instance.GetSlot`, so a derived attribute comes back
evaluated instead of unmaterialized (conformance `instantiate_derived_slot`). A slot holding an
*object* still marshals as that child instance's id, and no RPC resolves an id to an
`Instance`, so `part engine : Engine` is a dead end for a Python caller where `%slots` expands
it in the REPL. Either return the reachable sub-instances with the response or add a
`GetInstance` RPC; recorded as ⚠️ in `docs/SPEC_COMPLIANCE.md`.

## P3 — smaller Python-side items, all recorded in `docs/SPEC_COMPLIANCE.md`

- `connection.py` verifies a pid is the service by substring-matching its cmdline — spoofable.
- `pysysml.eval` and `pysysml.RuntimeError` shadow builtins.
- `SymbolToProto.Attributes` is always empty (`convert.go:40`), so `Symbol.attributes()` and
  `to_dataframe()` under-report.
- The download verifies a checksum served from the same origin as the binary, which detects
  corruption but not a compromised release; a pinned hash per version would be stronger.

---

# Track A — runtime and semantic gaps

These are the ⚠️/❌ rows in `docs/SPEC_COMPLIANCE.md`, in descending order of value. Each is
one session under the §5.2 four-layer contract.

## A1 — a usage's bound parameter is not passed to inherited conditions — done

Landed: a condition is evaluated against the features of the element stating it, so a
requirement's own attributes and a parameter a typed usage binds
(`constraint limit : MassLimit { in m = mass; }`) are visible to conditions inherited from the
definition, `require <expr>;` parses in a requirement definition body, and the conditions of an
anonymous nested constraint (`require constraint { <expr> }`) are evaluated. `runtime/condition.go`,
conformance cases `requirement_own_attribute`, `requirement_def_body_require`,
`requirement_nested_constraint`, `requirement_violated`, `instance_constraint_bound_parameter`.

What is left, both recorded as ⚠️ in `docs/SPEC_COMPLIANCE.md` under Requirement:

- **A1a — a quantity expression is not evaluated.** `attribute maxVerticalSpeed = 1.5 [m/s];`
  parses and type-checks, but the runtime has no case for it, so a condition comparing values
  written with units reports `unsupported node type: *ast.IndexExpr` instead of a verdict. This is
  what stops the Open-MBEE lunar lander model's `TouchdownRequirement` from reaching a verdict as
  the model writes it. Runtime values carry no unit, so the semantics to decide first is whether a
  quantity evaluates to its magnitude (silently ignoring a mismatch between `m/s` and `km/h`) or
  values carry a unit and comparison converts.
- **A1b — `assert satisfy <requirement> by <part>;` is not an evaluation entry point.** It parses
  and type-checks (#78), and `%requirement` reaches a verdict for a requirement whose attributes
  are bound to the subject's values, but the assertion itself is never evaluated and an anonymous
  one cannot be named. Evaluating it means binding the requirement's subject parameter to the `by`
  operand's instance and evaluating the conditions against that object.

## A2 — a typed multi-valued feature ignores its default

A feature that is both typed and given a default takes the typed instantiation; the default is
not merged. Second known limitation in the changelog. Decide the semantics before coding — the
spec question is whether the default supplies elements or replaces the instantiation.

## A3 — a library value type does not resolve as a type in the REPL path

`attribute d : Real;` with no default reports `<unknown>` (reproduced today), because the
reference does not resolve to a type at all rather than because instantiation fails. Related to
A6; do A6 first and re-test this.

## A4 — executor approximations

- **Port routing ignores direction and conjugation.** A message reaches every port connected by
  a connector in the same behavior body, and a port of the enclosing part is invisible to the
  behavior.
- **Accept-parameter visibility.** The payload lives in shared token data, which scoping does
  not model, so a sibling node reading it by simple name reports unresolved.
- **Transition endpoint names** are resolved at lowering, not at the name-resolution tier, so a
  misspelled endpoint surfaces late. Error timing is part of the contract (AGENTS.md §4):
  moving it is the point of the task, and the affected tests must be updated deliberately.
- **Dangling transition detection is lenient.** UML 2.5.1 §14.2.3.9 wants exactly one source
  and one target vertex. Hard cases to name in the prompt: a target in a sibling orthogonal
  region (legal), in an unrelated machine (illegal), an entry/exit point on a composite state
  (legal), a target resolving to a non-vertex (illegal), the sourceless `accept … then` form
  (legal), a junction chain terminating nowhere (illegal, and not a cycle).
- **Calc recursion** is depth-bounded and rejected rather than evaluated.
- **Numeric library coverage is scalar only.** The KerML function library's scalar numeric
  functions and `**` are evaluable (`runtime/library_functions.go`); `VectorFunctions`,
  `MatrixFunctions`, `ComplexFunctions`, the rest of `SequenceFunctions`, and unit-aware
  arithmetic (`1.62[m/s^2]`) are not. `TrigFunctions::pi` has no declared value, so the
  library's own `deg`/`rad` bodies cannot be evaluated — that needs a library *feature* value,
  a different seam from function dispatch.
- **An unqualified library function call still reports `unresolved-reference`** while evaluating
  correctly, because dispatch by local name is a runtime fallback and the checker does not know
  the library is implicitly in force (A6 is the general fix). This applies equally to the
  Systemica extension functions: `import SystemicaMathFunctions::*;` clears the diagnostic, a
  bare `exp(x)` evaluates but is still reported.
- **`exp`, `ln`, `log` and `atan2` are a Systemica extension, not OMG.** The vendored library
  declares no signature for any of them, and the vendored files stay byte-identical, so they are
  declared in `internal/core/libs/stdlib/Systemica Libraries/SystemicaMathFunctions.kerml` — a
  non-normative package a model reaches with `import SystemicaMathFunctions::*;`. A model meant
  to be portable to another SysML v2 tool cannot rely on it.
- **A `for` loop iterates a sequence or a set only.** `runtime/action_statements.go` `forElements`
  reports anything else, because those are the only collections the expression layer produces; a
  collection built by an expression (a range, a filter) has to wait on that layer. A set is
  visited in the order its canonical rendering sorts in, since a set has no order of its own.
- **A body member that is not an executable statement fails the run.** A nested action
  declaration or a `perform` written inside a loop or an `if` branch body is lowered to
  `lower.Unsupported` and reported when reached, since neither has succession semantics inside a
  block. Executing them means giving a block its own token flow.

## A5 — visibility rules

- **Protected imports are treated as private.** SysML v2 §7.5.3 makes a protected import
  visible in specializations of the importing definition or usage; today its members resolve in
  the owning body only. This is the general rule, not an `expose` quirk — do it first.
- **`validateExposeOwningNamespace`.** An `expose` outside a view body is parsed and resolved
  rather than diagnosed.
- **A privately wildcard-imported name is still reachable unqualified.** The qualified route was
  closed; `resolve/unqualified.go` `matchImport` enumerates a wildcard import's target through
  `symbols/index.go` `LookupDirectChildren`, which does not consult the hidden marks, so
  `package App { import Mid::*; }` still sees what `Mid` imported privately.

## A6 — implicit library import (do this LAST)

`❌ Unqualified library names in files that do not import their library` (`Boolean`, `Real`,
`that`). Deferred repeatedly for one reason: it can mask corpus regressions by making
unresolved references resolve for the wrong reason. Gate it with a **file-by-file** corpus diff,
never the total, and land it only while the corpus sits at its 98/100 ceiling.

## A7 — parser items, verify before tasking

- `individual part ip : Vehicle;` parses as a usage of kind `individual` named `part`, so it is
  not a part usage at all. PR #51 attempted this and was **closed**: making the modifier
  nameable that way made `individual part : Vehicle` indistinguishable from
  `individual item : Integer`, while `ref item : Integer` — a modifier in the same position —
  was left alone. Read that thread before starting. The right shape is on the naming side:
  require an explicit name and diagnose the ambiguous form. Reaches
  `internal/core/symbols/builder.go`, since the modifier is also not reflected in the symbol
  kind.
- `for step in c { … }` is rejected: `parseForAction` wants an `Identifier`, and `step` is the
  KerML keyword, so the loop becomes an error node with three diagnostics. The keyword-in-name
  handling that `parser/defusage.go` `atKindPrefix` does for declarations is missing here (and
  the REPL does not print load-time parse diagnostics, so the file looks accepted).
- `action a { in snapshot ; }` parses with zero diagnostics — an anonymous untyped parameter is
  silently accepted (also `in event ;`). Reproduce first; it was never re-verified after the
  occurrence-modifier work.

---

# Track B — REPL refinements

None of these are capability gaps; the REPL executes models. They are the rough edges a new
user meets.

## B1 — a declaration ends an in-progress debugger session

`Session.Submit` clears everything derived from the previous document, including `s.actionExec`
and `s.stateExec`, so typing any declaration mid-session silently ends an `%action`/`%state`
session and wipes instances. The maintainer accepted this as "fine for now" and asked for
refinement: keep the session alive when the declarations it depends on are unchanged, or at
minimum say the session was dropped instead of failing the next `%step`/`%advance` with "no
active session".

Related: `Session.accept` drops any earlier snippet whose declared names intersect the new one,
so re-typing `package Demo { … }` to add a member replaces the whole package body rather than
merging it.

## B2 — `%eval` of a compound expression cannot reach a package member

Simple identifiers and qualified names resolve, but a compound expression needs a chosen
evaluation context. This is a design decision, not a bug: decide whether `%eval` takes an
optional context (`%eval in Demo::Vehicle : mass * 2`), inherits the last `%instantiate`, or
keeps requiring qualified names.

---

# Track C — test coverage where it is thin

Ordered by what a bug there would cost:

- **`cmd/` is thinly tested** (`cmd/sysml` was 6.4%, the other two 0%). `cmd/sysml/main_test.go`
  covers only how flags become session options. `cmd/sysml/convert_test.go` now builds and
  drives the binary as a process for the conversion paths, so the pattern exists to copy; the
  REPL and LSP/gRPC binaries still have no smoke test — start it, exchange one message, shut it
  down — and `sysml-grpc` is a published artifact.
- **`internal/core/resolve` at 54.3%** is the lowest of the semantic packages while carrying the
  most subtle rules (feature chains, redefinition, aliases, cached targets).
- **`internal/core/ast` at 27.4%** is mostly declarations, so the number is misleading; check
  what is actually uncovered before writing tests for their own sake.

---

# Track D — model persistence and RDF interchange

Saving and SysML ↔ RDF Turtle conversion landed (`internal/core/rdf`,
`internal/core/export`, `%save`, `sysml -convert`); see
[`RDF_INTEROP.md`](RDF_INTEROP.md). What that work deliberately left open:

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
so the harness already exists.

## D4 — the parser records `then` ambiguously, so successions cannot convert

`ast.Membership.HasSuccession` is a bare flag saying a `then` was written, with no record of
which two members it sequences. Worse, the sites that set it disagree on which side of the
keyword the flag belongs to: `parser/defusage.go:1582` marks the member *after* `then` (the
prefix form), while `parser/behavior.go:199` marks the member *before* it (the trailing form).
`SuccessionTarget` and `SuccessionGuard` exist on the struct but are never assigned by any
path, and nothing downstream reads any of the three — the runtime does not consume this flag
at all, which is why the inconsistency has gone unnoticed.

Because of that, `ToRDF` refuses a member carrying a succession rather than guessing a
position that could reorder execution (`export_test.go:TestSuccessionIsUnsupported`). Fixing
this means giving the parser one consistent representation — most likely resolving the target
member and populating `SuccessionTarget` — after which the mapping can carry a real succession
edge and the restriction lifts. Worth checking whether the runtime *should* be consuming this
flag, since a model's step order currently depends on member order alone.

## D5 — the parser drops the `variant` and `include` keyword prefixes

`variant part a : A;` and `include U;` prefix a kind keyword the AST already records on its
own, and the prefix itself is recorded nowhere: both parse to the same node as the unprefixed
form. A `notation → RDF → notation` round trip therefore returns `part a : A;` and a plain
use-case reference, which is the one place the RDF mapping changes a model without reporting
it (`docs/RDF_INTEROP.md`, *Limitations*).

The synonym keywords that *are* distinguishable — `datatype`, `feature`, `function`,
`snapshot`, `timeslice`, `message`, `allocate` and the rest — are carried as
`sysx:declaredKeyword` and round-trip byte-identically
(`export_test.go:TestKindKeywordSynonymsSurviveRDF`). Doing the same for these two means the
parser recording the prefix, most likely as a field alongside `ast.Usage.Keyword`, after which
the encoder can carry it and the documented exception goes away. Worth checking at the same
time whether anything downstream *should* distinguish a variant from a plain member, since
variation semantics currently rest on the enclosing `variation` definition alone.

---

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

1. **R1** (tag), then **R2**/**R3** as the account access appears. R1 gates the rest of the
   release section.
2. **P1** and **P2** next: the release now ships the service binary, so the Python surface is
   the newest promise and the least CI-verified.
3. **A1a**/**A1b** (what is left of A1) and **A2** — the limitations the changelog admits to — then **A4**/**A5** in
   parallel (they share only `docs/SPEC_COMPLIANCE.md`; the two `state_executor.go` items in A4
   must run one at a time).
4. **A6** last, gated on a per-file corpus diff.
5. **B1**, **B2** and **Track C** are good filler sessions: small, isolated, and each closes a
   rough edge a user would otherwise report.
6. **Track D** is independent of the rest and can run whenever. Take **D3** before **D1**/**D2**:
   it is the cheapest, and it is what would show whether the Flexo interop claim actually holds
   before more work is layered on the mapping. **D4** is the one with a correctness question
   attached — it is a parser inconsistency that predates the conversion work and is worth
   settling on its own merits, not just to unblock the mapping.
