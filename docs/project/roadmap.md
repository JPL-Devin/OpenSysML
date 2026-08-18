# OpenSysML — Roadmap

Baseline: `main` @ `32f5a03`, verified locally on 2026-08-17 with Go 1.25.13.
Read `AGENTS.md` first; it governs everything below.

0.0.7 is released from `Open-MBEE/OpenSysML`, carrying `sysml`, `sysml-lsp` and `sysml-grpc`
archives. `main` now carries the whole 0.0.8 batch — the post-0.0.7 fixes, the documentation
reorganization, the release-publishing fix, and the Track A/B/P work below — all listed under
0.0.8 in `CHANGELOG.md`, which is cut and awaiting its tag. Everything in "Release
follow-through" is maintainer- or account-gated; everything after it is ordinary engineering
work.

Track status as of this baseline: **Track B is closed**, **Track P is closed** on the
engineering side (its remaining item, publishing to PyPI, is R2 and account-gated), and
**Track A is closed**: A6 (implicit library import) is resolved as **won't do** — requiring an
import is the conforming behavior (see A6) — and A3's shape question is answered: a valueless
feature of a value type keeps the empty object it materializes and reads as `<unset>` on every
surface. Tracks C and D are untouched by this batch.

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
contributes no profile at all (Track C).

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

## R1 — tag the next release (maintainer, blocking everything else in this section)

Releases live on `Open-MBEE/OpenSysML`; development happens on `JPL-Devin/OpenSysML`, which
has no tags at all. So the tag is preceded by promoting `main` upstream, as 0.0.4 was through
Open-MBEE PR #47:

```bash
# on Open-MBEE/OpenSysML, after main carries the release commit
git tag -a v0.0.8 -m "v0.0.8"
git push origin v0.0.8
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

# Track P — the Python/gRPC surface — done

Every engineering item below is on `main`; what remains for a Python user is R2, publishing the
wheel, which is account-gated rather than work.

This is where the release just changed shape: `sysml-grpc-<os>-<arch>` binaries now ship with a
`.sha256` sidecar, and `opensysml` downloads and verifies one, so a Python user no longer needs a
Go toolchain. That makes the following gaps user-visible for the first time.

## P1 — the integration tests skip in CI — done

The 20 tests in `python/tests/test_integration.py` and `test_runtime_integration.py` skipped
themselves unless a service answered on `localhost:50051`, and CI started none, so the
client↔service path was never exercised there. Starting one was not the fix on its own: the
ownership model shut down services the process had not spawned, so
`test_service_shuts_down_when_last_process_exits` failed as soon as a service ran.

The ownership rule decided and implemented: **opensysml never stops a service it did not spawn.**
A connection attaching to a service already listening takes no reference and leaves it running;
a service `opensysml` starts is refcounted within the starting process only (`_OWNED_SERVICES`,
keyed by state dir and port), against the `(pid, create_time)` the reference was taken on, and
stopped once when its last reference goes. That test now spawns its own service on its own port
and state dir and asserts *that* one dies (`TestOwnershipOfASpawnedService`).

Both `python-test` jobs (`.github/workflows/pr.yml` is the gate that runs on a PR;
`.circleci/config.yml` matches it) start `sysml-grpc` on 50051, wait for the port and run with
`OPENSYSML_REQUIRE_SERVICE=1`, which turns "no service, so skip" into a failure — a developer
without a binary still skips.

## P2 — a nested object is unreachable over gRPC — done

`Instantiate` reads feature values through `Instance.GetFeatureValue`, so a derived attribute comes back
evaluated instead of unmaterialized (conformance `instantiate_derived_slot`). A feature value holding an
*object* still marshals as that child instance's id, but `InstantiateResponse.instances` now
carries every instance reachable from the root, so `part engine : Engine` expands for a Python
caller (`inst.engine.power`) as `%slots` expands it in the REPL. Expansion is bounded the way
`%slots` bounds it — depth 8, and no descent into a type already on the path, since reading a
composite feature value materializes the object it holds and a self-referential part would otherwise
instantiate forever; an unexpanded child stays a bare id. A `GetInstance` RPC was
rejected: runtime instances live in the request's `runtime.Context` and do not survive the
call, so an id is only meaningful against the response that carried it — noted as a limitation
in `docs/project/spec-compliance.md`.

## P3 — generated typed classes for a parsed model — done

`python -m opensysml.generate model.sysml -o model_types.py` emits one class per SysML definition,
each a typed view over the `Instance` P2 made navigable, so `v.mass` completes in an editor and
`v.mass + "x"` is a mypy error; `opensysml` ships `py.typed`. Codegen reads facts the service now
resolves rather than scraping them: `SymbolInfo` carries `type_info` (declared and resolved type,
the library scalar it reduces to, and whether it is a quantity), `multiplicity`, and *every*
generalization edge in `specializations` — `extractMetadata` exported only the first `specializes`
of a definition and the first typing of a usage, which cannot express multiple supertypes or tell
subsetting from redefinition. Known limitations are listed in `python/README.md`; the load-bearing
ones are that a quantity feature value is typed `object` because the wire `Value` has no
magnitude-and-unit form (the same reason `ValueToProto` reports one as unsupported), and that only
structural usages become properties. `specializes`, `subsets` and `redefines` each produce the
corresponding Python base class; an edge Python cannot linearize is named in a comment on the
class rather than dropped silently.

## P4 — smaller Python-side items, all recorded in `docs/project/spec-compliance.md` — done

- ~~`connection.py` verifies a pid is the service by substring-matching its cmdline — spoofable.~~
  **Done:** the spawner writes the service's pid *and* process start time (plus its own), and a
  pid is trusted only while `psutil.Process(pid).create_time()` still matches; a reused pid or a
  lookalike cmdline is a stale record, cleaned up and never signalled.
- ~~`opensysml.eval` and `opensysml.RuntimeError` shadow builtins.~~ **Done:** the module-level function
  is `opensysml.evaluate` and the error class is `opensysml.ExecutionError`; both old names resolve to
  the new object through `__getattr__` with a `DeprecationWarning` and are out of `__all__`, so a
  star-import shadows neither built-in. Removal in 1.0.0.
- ~~`SymbolToProto.Attributes` is always empty (`convert.go:40`), so `Symbol.attributes()` and
  `to_dataframe()` under-report.~~ **Done:** the service reports each symbol's own and inherited
  attributes with resolved type, multiplicity, unit and constant default, behind the
  `symbol_attributes` capability; redefinitions mask the inherited name and a non-constant
  default is omitted rather than guessed.
- ~~The download verifies a checksum served from the same origin as the binary, which detects
  corruption but not a compromised release; a pinned hash per version would be stronger.~~
  **Done:** `PINNED_SHA256` in `binary.py` pins every asset's SHA-256 per release, generated from
  the published assets by `python/scripts/pin_release_checksums.py`; an unpinned release is
  refused rather than falling back to the sidecar, unless
  `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1`) accepts same-origin trust
  explicitly, for the repository it names.
- ~~`Model.eval` takes a scope, not a subject, so an expression cannot be evaluated against an
  object the way `%eval` does after `%instantiate`: `verify_constraint` takes a subject, `eval`
  does not, and a caller reads the declared default instead. Carrying the subject to
  `Evaluate` is a service-side change as well as a client one.~~ **Done:** `EvaluateRequest`
  carries `subject_symbol_id`, the service instantiates and binds it as the REPL does after
  `%instantiate`, and `Model.eval(expr, subject=…)` reads that object's values behind the
  `evaluate_subject` capability; a call without a subject is unchanged.

## P5 — the standard library was rebuilt on every cold `ParseFile` — done

`ParseFile` built a new `symbols.Index` on every cache miss — loading every standard library unit
into it and expanding the library's wildcard imports — so ~99% of a cold load (28-31 ms loading,
38-41 ms expanding) was work that does not depend on the model. The REPL and the LSP pay it once
per session; the service paid it per distinct model, which is what made a Python parameter sweep
varying the model text impractical at 70 ms x N of pure overhead.

The service now keeps a small pool of prewarmed library indexes (`internal/grpc/libindex.go`,
sized by `SYSML_GRPC_INDEX_POOL`, default 4): a cache miss takes one, adds its document and keeps
it with its `CachedModel`. An index is handed out once, so cached models stay independent and
nothing is shared behind a mutex; the pool refills in the background and an empty pool builds one
inline, so a result never depends on prewarming. Measured on this VM with
`examples/combined-behavioral-demo.sysml`, a cold `ParseFile` went from ~100-128 ms to ~0.5-0.9 ms
with identical diagnostics. What a model resolves against is pinned by test rather than asserted:
`TestPooledIndexMatchesFreshlyBuiltIndex` compares diagnostics and every qualified lookup of a
pooled index against one built on the request path.

## P6 — opensysml was read-only: a change could not be persisted — done

Nothing in the Python surface could change a model. No RPC accepted one, no `opensysml` object was
mutable, and the only writing path (`Convert`) re-emits the source it parsed, so the loop a
modeling script wants — read a value, compute, write it back — ended at "read".

Landed as a **source-level edit**, not a model-mutation API: `ApplyEdits`
(`internal/core/edit`, `internal/grpc/edit.go`, capability `apply_edits`) rewrites bytes of the
source a model was parsed from, at the spans the parse recorded, and re-parses and re-analyses the
result before returning it. The AST stays immutable and nothing is reformatted, so a file comes
back with its comments, blank lines and indentation byte-identical outside the edited spans:

```python
model = opensysml.load("spacecraft.sysml")
edit = model.edit()
edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
edit.apply().save("spacecraft.sysml")
```

Two operations, targeted by the id a read reports (`Symbol.id`): set a feature's value (replacing
an existing `= <expr>`, or adding one before the `;`), and rename a declaration. Every refusal is
typed and carries the diagnostics behind it (`EditError` and its subclasses in Python; `EditFailure`
on the wire), and a refusal returns no content — the service does not hand out notation its own
parser cannot read back.

**Known limitations, by design and recorded in `docs/project/spec-compliance.md`
§ Source-Preserving Model Editing:** a rename rewrites the declaration's name token only, so a
rename whose element is referenced anywhere is *refused* — naming the namespaces the references
are made from — rather than leaving the model unresolvable; and no element is created or deleted,
since there is no AST printer and a model assembled client-side has no source to preserve. Both
remain open: reference-rewriting renames need the reference spans a rename would have to update
(the resolver already collects them, so the work is deciding what counts as a reference *worth*
rewriting — an import, an alias, a redefinition — not finding them), and creation needs a printer
for new elements spliced into existing source, which is a separate feature.

---

# Track A — runtime and semantic gaps

These are the ⚠️/❌ rows in `docs/project/spec-compliance.md`, in descending order of value. Each is
one session under the §5.2 four-layer contract.

## A1 — a usage's bound parameter is not passed to inherited conditions — done

Landed: a condition is evaluated against the features of the element stating it, so a
requirement's own attributes and a parameter a typed usage binds
(`constraint limit : MassLimit { in m = mass; }`) are visible to conditions inherited from the
definition, `require <expr>;` parses in a requirement definition body, and the conditions of an
anonymous nested constraint (`require constraint { <expr> }`) are evaluated. `runtime/condition.go`,
conformance cases `requirement_own_attribute`, `requirement_def_body_require`,
`requirement_nested_constraint`, `requirement_violated`, `instance_constraint_bound_parameter`.

What came out of it, recorded under Requirement in `docs/project/spec-compliance.md`:

- **A1a — a quantity expression is not evaluated — done.** A quantity evaluates to a magnitude
  **and** the measurement reference it is written in (`Quantities::ScalarQuantityValue` is `num` +
  `mRef`), units reduce to a scale factor over base units read from the Quantities and Units
  library, and commensurable units convert before a comparison or a sum — `1.5 [m/s] <= 5.4 [km/h]`
  is true, exactly, at its boundary. Incommensurable units (`1.5 [m/s] <= 2.0 [s]`) are
  `ErrIncommensurableUnits`, never a comparison of bare magnitudes. The Open-MBEE lunar lander
  model's `TouchdownRequirement` now reaches a verdict as the model writes it.
  `semantics/units.go`, `runtime/quantity.go`, conformance cases `requirement_quantity_*`,
  `constraint_quantity_*`, `calc_quantity_ratio`.
- **A1b — `assert satisfy <requirement> by <part>;` reaches a verdict — done.** The assertion is
  evaluated as the requirement usage it is, with the requirement's subject parameter bound to an
  object of the `by` feature, so its conditions — its own, and the ones it inherits — read that
  object's values. `%satisfy` evaluates every assertion a model states, or the ones one element
  states, since such an assertion is anonymous; `assert not satisfy … by …` parses and inverts the
  verdict. `runtime/satisfy.go`, conformance cases `satisfy_subject_binding`,
  `satisfy_subject_features`, `satisfy_inherited_conditions`, `satisfy_negated`,
  `satisfy_without_conditions`.

  What is left, recorded as ⚠️ in `docs/project/spec-compliance.md` under Requirement: a requirement
  feature carrying no value of its own is read from the satisfying object's feature of that name,
  which the spec does not state — it supplies a subject's values through the subject parameter or
  an explicit binding. The fallback is the one `%requirement` already applies on an instance, and
  the alternative is to report `ErrNoValue` for the shape the lunar lander model writes and require
  a subject reference in the condition instead. The lunar model's own
  `assert satisfy touchdown by lander01;` reaches no verdict either way: its `1.5 [m/s]` evaluates
  now that A1a is done, but its `actualVerticalSpeed` is produced by a descent analysis rather than
  bound by the requirement or held by the part.

## A2 — a typed multi-valued feature ignores its default — done

Landed on the maintainer's ruling (option 3 of the three readings offered): a default is
*honoured where it conforms* and *reported where it does not*, never invented and never dropped.
`attribute speeds : Real[3] = (1.0, 2.0, 3.0)` materializes its three values; a scalar default
against `[3]` is not broadcast into three copies — that would supply values the model never
wrote — but a `multiplicity violation: 2 value(s) bound to a feature with multiplicity lower
bound 3` diagnostic, which is what the silent drop the changelog admitted to used to hide. A
feature declaring no multiplicity is held to the assumed `1..1` (`AssumedRange`,
`Model.EffectiveMultiplicityOf`), so the same rule decides the single-valued case rather than
exempting it.

The diagnostic is reported wherever the value is materialized, with the same verdict on every
surface: `-instantiate … -validate` exits 2, a `%slots` in a piped REPL session exits 2 (it
exited 0 before, so a script could not detect the failure), the JSON output carries it under
`runtime.materialize`, and a walk elided by the depth or self-containment bound marks itself
partial rather than printing `✓ no errors`. Duplicate diagnostics for one redefined feature value are
suppressed. `runtime/instance.go`, `runtime/shape.go`, `semantics/multiplicity.go`,
`cmd/sysml/report.go`.

## A3 — a library value type materializes as an empty object rather than a value — done

Half of this was already gone: `attribute d : Real;` no longer reports `<unknown>`, and
`attribute k : Real = 2.0` was always correct (`k = 2.00`). What was left was how the valueless
case *reads*, and it was decided as a reporting question, not a materialization one.

The spec does not settle what materialization creates. `Base::DataValue` is
`abstract datatype DataValue specializes Anything` — "entities that are values that do not change
over time" — and `ScalarValues::Real` is a datatype with no features of its own, so an empty
object is a value type's whole content; nothing in KerML or SysML prescribes a runtime
representation, and a `FeatureValue` is optional (a feature has at most one). So the maintainer's
ruling stands: the empty object stays what materialization holds, and every surface that reports a
value says so. `Context.HoldsNoValue` (`runtime/instance.go`) recognizes an empty object of a value
type, and `-instantiate`/`-e`, `%slots`, the JSON report and the gRPC/Python view all spell it
`<unset>` — `pb.Value.unset` on the wire, `opensysml.UNSET` in Python — while a valued attribute, an
object of a class, and a value type that does have features are unchanged. Unset is sent, never
accepted: `ProtoToValueIn` rejects it with `ErrUnsetNotAccepted`.

Left open deliberately: no diagnostic is owed for a valueless `1..1` feature. The multiplicity
check applies to values a model binds (A2, `semantics/multiplicity.go`); a feature declaring no
value binds none, and the spec does not make that ill-formed.

The other half went away with A6's won't-do verdict: the unqualified spelling was never a defect —
`attribute d : Real;` without an import is *correctly* unresolved, and the probe was simply missing
`private import ScalarValues::*;`.

## A3a — a measurement unit does not resolve inside a condition in the REPL path — done

Closed by the body-scope work. What the entry blamed was wrong on both counts, checked against
`main` @ `b098dbf` with the corpus gate green:

- The **per-submission index rebuild** is gone. `Session.symbolIndex` keeps one index for the
  session and re-indexes only the document (#95), and `ExpandWildcardImports()` runs on every
  submission (`internal/repl/session.go`), so the wildcard re-exports of `SI::*` are present when
  the condition is evaluated. Nothing in `internal/core/symbols` needed to unwind them.
- The residual failure was a **scope**, not an index: `%constraint`/`%requirement` evaluated an
  element with no instance against the *document root* scope, which reaches only what the root
  itself declares — so `20.0 [m]` in a condition inside `package QTest` could not see the `m` that
  package imported, while `attribute d = 100.0 [m]` could, because a feature default carries its
  own declaring scope (`EffectiveFeature.DeclScope`). Both meta-commands now evaluate in the
  element's declaring scope (`declaringScope`, `internal/repl/meta.go`), which is what they already
  used when an instance carried the element.

The entry's model now passes in the REPL, pinned by
`TestConstraintResolvesUnitsOfItsOwnPackage` (`internal/repl/runtime_commands_test.go`). The same
defect in the action and state executors — where every evaluation used a nil scope — is fixed in
the same change; see `docs/project/spec-compliance.md` under "Scope of an expression in a behavior body".

Both residual items are closed by the unit-resolution work; see `docs/project/spec-compliance.md` under
"Name in the unit position of a quantity expression" and "Arguments of a `%calc` command".

- A member whose name is a unit's shadows that unit — as ordinary name resolution prescribes
  (KerML 8.2.3.5.3), the position expecting a measurement unit only decides whether what resolved
  conforms. That is now the rule on every path, including conditions, which used to reach past the
  nearer declaration; the diagnostic names the declaration, where it is declared, and the
  qualified spelling of the unit it hid.
- `%calc` parses its argument list as expressions, so a quantity, a parenthesized expression or a
  nested call survives. Named arguments (`v0 = …`) remain out: the notation writes those inside an
  invocation's parentheses, and the prompt reports the limitation instead of misreading them.

## A4 — executor approximations — all but two bullets done

- ~~**Port routing ignores direction and conjugation.**~~ **Done.** A message is routed by the
  direction of each end and by conjugation (`~`), the ports of the enclosing part are reachable
  from the behavior, and a send with no conforming route is a typed error rather than a silent
  no-op.
- ~~**Accept-parameter visibility.**~~ **Done.** The payload is scoped so a sibling node reads it
  by simple name, and reading it before the accept is reported rather than answered.
- ~~**Transition endpoint names** are resolved at lowering, not at the name-resolution tier.~~
  **Done**, with the affected tests moved deliberately: a misspelled endpoint is a
  name-resolution finding, and the two shapes handed over with this work — an endpoint naming a
  vertex of a *different* state machine, and one naming a named `first`/`then` marker — are
  check-time diagnostics rather than executor-construction failures.
- ~~**Dangling transition detection is lenient.**~~ **Done**, over the hard cases the entry
  named: a target in a sibling orthogonal region and an entry/exit point on a composite state are
  legal, a target in an unrelated machine, a target resolving to a non-vertex and a junction chain
  terminating nowhere are reported, and the sourceless `accept … then` form stays legal.
- ~~**Calc recursion** is depth-bounded and rejected rather than evaluated.~~ **Done.** A
  recursive `calc` evaluates (`Fact(5) = 120`); the bound remains as a budget against
  non-terminating recursion rather than as a refusal of recursion itself.
- ~~**Numeric library coverage is scalar only.**~~ **Done.** `VectorFunctions`,
  `MatrixFunctions`, `ComplexFunctions` and the rest of `SequenceFunctions` dispatch
  (`norm((3.0, 4.0, 0.0)) = 5.00`), `includingAt` inserts at its position on the maintainer's
  ruling, and the library *feature* seam supplies `TrigFunctions::pi`, so the library's own
  `deg`/`rad` bodies evaluate (`deg(1.0) = 57.30`) and an expression reading `pi` answers.
  Residual, small and REPL-side: `%eval TrigFunctions::pi` on its own reports "has no value to
  evaluate" because the meta-command reads the declaration's default rather than the seam, while
  `2.0 * TrigFunctions::pi` inside a model evaluates.
- ~~**A `for` loop iterates a sequence or a set only.**~~ **Done.** A loop iterates every
  collection the expression layer produces — a range ascends, a filter keeps the order of what it
  filtered, a collection-valued calc is iterated in the order it returned, and an empty sequence
  or a descending range is iterated not at all (conformance case
  `action_for_over_produced_collections`). A `for` written directly in an action body, with no
  position in the token flow, is reported as such rather than run in declaration order.
- ~~**A body member that is not an executable statement fails the run.**~~ **Done.** A block has
  its own token flow (`lower/block_graph.go`), so a nested action declaration or a `perform`
  inside a loop or an `if` branch executes; what genuinely has no flow is still lowered to
  `lower.Unsupported` and reported when reached rather than dropped.

Both remaining bullets are statements of fact rather than gaps:

- **An unqualified library function call is reported unresolved, and conformantly so.** Written
  with no import in force the name is genuinely unresolved (A6), so the diagnostic is right. For a
  name only the OpenSysML extension declares the behavior now agrees with it: an unimported
  `exp(x)` fails with `ErrUnimportedExtensionFunction` naming the import, rather than being
  answered by dispatch on the local name. An OMG function library name still evaluates unimported,
  the rough edge A6 addresses. `import OpenSysMLMathFunctions::*;` (or `MathFunctions` for the OMG
  ones) clears the diagnostic and is the spelling a model should use.
- **`exp`, `ln`, `log` and `atan2` are an OpenSysML extension, not OMG.** The vendored library
  declares no signature for any of them, and the vendored files stay byte-identical, so they are
  declared in `internal/core/libs/stdlib/OpenSysML Libraries/OpenSysMLMathFunctions.kerml` — a
  non-normative package a model reaches with `import OpenSysMLMathFunctions::*;`. A model meant
  to be portable to another SysML v2 tool cannot rely on it.

## A11 — string operators and the string function library — done

Found in the 0.0.8 pre-release audit: no operator was defined over two strings (`s + "d"` reported
`operator '+' is not defined for a string and a string`) and no `StringFunctions` declaration was
evaluable, so a model that builds a message, compares text or measures a string could not run.

Landed: every function the vendored `StringFunctions` declares is evaluable, and each of the
operators it declares evaluates over two String operands — `runtime/library_functions.go`
`registerStringFunctions` (`'+'`, `Length`, `Substring`, `'<'`, `'>'`, `'<='`, `'>='`, `'=='`,
`ToString`) plus the operator dispatch in `runtime/eval.go` (`evalArithmetic` for `'+'`,
`evalComparison` for the four comparisons). Semantics, and where each comes from:

- **Length counts characters, not bytes**: one per Unicode code point, so `Length("héllo")` is 5
  though the string is 6 bytes. The declaration returns `Natural[1]`, a count of the string's
  characters, and the KerML 1.0 String description is a sequence of characters; a byte count would
  depend on the encoding, which the library never mentions.
- **Substring positions are 1-based and inclusive**, over characters, matching the library's own
  `SequenceFunctions::subsequence` (`in lower`/`in upper`, `Positive` indices elsewhere in the
  library). A position outside `1..Length(x)` is `ErrIndexOutOfRange` rather than clamped, and an
  upper below lower selects no character, exactly as `subsequence` answers nothing for such a range.
- **Comparison is by character order**: UTF-8 orders bytes as it orders code points, so a Go string
  comparison *is* code-point order.
- **No coercion**: an operand that is not a String is an `OperandTypeError` wrapping
  `ErrTypeMismatch`, naming the operator and both operand types. `'=='` specializes
  `DataFunctions::'=='`, so as an *operator* `s == 3` is `false` (equality is defined over any two
  values); called explicitly, `StringFunctions::'=='` declares `String[0..1]` operands and reports a
  non-String argument, like every other signature in the package. `'=='` over `[0..1]` operands also
  answers for an omitted one: two omitted operands are equal, an omitted one and a string are not.
- The library declares `Length` and `Substring`, not `size`/`substring` — the audit's repro used the
  latter spelling, which no vendored file declares, so the implemented names follow the library.

Conformance cases `string_operators`, `string_comparison`, `string_functions`, `string_empty`,
`string_substring_out_of_range`, `string_compared_with_a_number`; unit coverage in
`runtime/eval_operator_test.go` and `runtime/library_functions_test.go`, the latter's
`TestVendoredFunctionsAreAllDispatchable` now gating `StringFunctions` so a declaration cannot drift
away from an implementation; robustness case `string_operand_of_the_wrong_kind`.

## A10 — an enumeration literal has no runtime value — done

Landed: a literal declaring no value evaluates to itself — `runtime.Value{Kind: ValEnumLiteral}`
holding the literal's declaration symbol, so identity is the declaration and nothing else (not an
ordinal, not its name as a string). Two literals are equal exactly when they are the same
declaration, so a literal of another enumeration compares false rather than reporting, matching how
OpenSysML compares any two values of unrelated types; the static type checker is the tier that
reports the mismatch (`passes/typecheck_value.go`, `cannot bind a value of type Size to a feature
typed by Color`). A literal specializing a scalar type (`enum def GradePoints :> Real { A = 4.0; }`)
evaluates to the value it declares, since such a literal *is* that value and has to compute as one.
A literal is an occurrence of its enumeration, so its own features are read off one object per
literal (`Level::high.n`), and an enum-typed default materializes: the reproduction prints
`c = Color::red`. `runtime/value.go`, `eval.go`, `semantics/enumeration.go`, conformance cases
`enum_literal_default_slot`, `enum_literal_own_attributes`, `enum_literal_scalar_valued`.

## A5 — visibility rules — done

- ~~**Protected imports are treated as private.**~~ **Done.** A protected or public import now
  reaches the bodies that specialize the definition or usage declaring it (SysML v2 §7.5.3):
  `resolve/visibility.go` `lookupInheritedImports` walks `semantics.Model.DirectSupertypes`
  upward from the referring scope's owner, and `resolve/unqualified.go` `walkUnqualifiedHiding`
  consults it after the imports declared in the scope itself. An `expose` is protected, so it
  reaches a specializing view the same way. A feature typing is a generalization edge
  (KerML 8.3.4.6), so an import declared in a definition is also reached from a usage typed by
  it — `part p : Base` sees what `Base` protectedly imports.
- ~~**`validateExposeOwningNamespace`**~~ **Done**, on the maintainer's usage-only reading:
  `passes/expose.go` `checkExposeOwners` reports every `expose` whose owning namespace is not a
  view usage (SysML v2 8.3.26.2, and the normative Xtext grammar admits an Expose in
  `ViewBodyItem` only). A `view def` body is a **warning**, not an error, because OpenSysML
  resolves an `expose` there (`resolve/expose_test.go` `TestExposeInViewDefinitionBody`,
  `parse/view_expose.sysml`) and the OMG corpus (`42. Views`) never writes one; any other owner
  is an error. A package or namespace body rejects `expose` in the parser already.
- ~~**A privately wildcard-imported name is still reachable unqualified.**~~ **Done.**
  `resolve/unqualified.go` `matchImport` enumerates a wildcard import's target through
  `symbols/index.go` `LookupDirectChildrenFrom`, which drops what the target imported privately
  unless the referring namespace is the target itself (or nested in it) or the import is
  `import all`.
- ~~**Element filters are parsed and then dropped.**~~ **Done.** A namespace's `filter` now
  restricts the imported memberships it re-exports, and an import's or expose's `[...]`
  restricts what that import brings in (KerML 8.2.4, SysML v2 7.4.4). The condition is a
  model-level predicate over one candidate symbol — `semantics/filter.go`, not the runtime value
  evaluator, since there is no instance at name-resolution time — evaluated against the
  candidate's annotations from every form the parser accepts, with conformance through
  `Model.AllSupertypes`. `symbols/index.go` records the conditions along each route to a
  re-exported name and `resolve/filter.go` evaluates them, so the qualified and unqualified
  routes agree and a restored index cache decides a filter the same way a parsed library does.
  A condition outside the evaluated subset is reported (`passes/filter.go`) and not applied.
  ~~Still open: `@`/`@@` in the runtime evaluator (A4), and a view's exposed-element set as a
  queryable API.~~ **Done.** `runtime/eval.go` `evalClassification` evaluates `@`/`@@` as
  operators of an ordinary expression — a constraint, a calc body, an `%eval` — against the
  element its subject denotes (an explicit name, `self`, or the object being evaluated), and
  reaches its verdict through the same `semantics/filter.go` predicate an element filter is
  decided by (`Model.EvalClassification`), so the two paths cannot drift; a subject or type
  outside the evaluable subset is reported (`ErrFilterUnevaluable`) rather than answered false.
  `semantics/expose.go` `Model.ExposedElements` answers a view's exposed set, enumerated through
  `resolve/filter.go` `Resolver.ImportedElements` so exposes are admitted and filtered exactly as
  name resolution admits them, with `Model.NestedViews` to walk a view tree. ~~Still open: a REPL
  surface for the query.~~ **Done:** `%view <name>` prints what a view exposes and the views
  nested in it (`internal/repl/meta.go`, `internal/repl/view_test.go`), so the query has a user
  surface as well as an API.

## A6 — implicit library import — won't do (the current behavior is the conforming one)

A6 asked that `Real`, `Boolean` and `that` resolve in a file that imports no library.
Implementing that would be **non-conformant**, so the item is closed as won't do rather than
deferred. Only the *public top-level* members of a root namespace are globally visible
([SysML, 7.2] over [KerML, 8.2.3.5]); `Real` is a member of `ScalarValues`, not a top-level
member, so it is reachable only through an import or a qualified name. OMG's own 100 training
files agree — each writes `private import ScalarValues::*;` — and those files are downloaded
unmodified by `scripts/download-training-examples.sh`, so they are upstream evidence rather than
our own convention. Implementing A6 would also make genuinely-unresolved names resolve, which is
exactly the corpus-masking risk this entry always flagged.

What the convenience A6 wanted looks like *within* the spec, all already implemented: a
document-local root import serves that document (`private import ScalarValues::*;` at the top of
a file), and each document imports what it names — a root-level import is a member of the
importing document's own root namespace and reaches no other document, at any visibility
([KerML, 8.2.3.3]), pinned by `model/that_test.go`
`TestRootImportServesItsOwnDocumentOnly`. The LSP offers both fixes on the diagnostic, and the
import it inserts is now written `private import …` explicitly (`resolve/fixes.go` `importFix`)
rather than leaving the visibility to the default.

The one genuine conformance gap this entry carried was its `that` bullet, and it is **done**:
`that` alone resolved but `that.a` reported `no scope for member lookup in Base::things::that`,
because `that` is declared `Anything[1]` and owns no members. A member chain from `that` now
reads the usage enclosing the expression — the object featuring the value being written
([KerML, 8.4.2]) — and the runtime answers `that` with the object being evaluated:
`resolve/document.go` `featuringOf`, `runtime/eval.go` `evalName`, tests
`runtime/that_test.go` and `model/that_test.go` `TestThatChainsThroughTheFeaturingType`.
A `that` written where no usage encloses it stays unresolved rather than resolving to the
library's declaration.

## A7 — parser items, verified — done

Every item was reproduced on `main` at `89b14fd` first; the observation is recorded with the fix
because two of the four descriptions no longer matched the code.

- ~~`individual part ip : Vehicle;` parses as a usage of kind `individual` named `part`.~~
  **Done, and the description was already stale:** at `89b14fd` that form parses as a part usage
  named `ip`, so PR #51's failure mode is gone. What did reproduce is the other half — the
  modifier reached neither the kind nor the symbol kind (`individual ip : Vehicle` was a plain
  `attributeUsage`), and the nameless `individual part : Vehicle` was accepted silently. A lone
  occurrence modifier now declares its own kind (`parser/defusage.go` `modifierImpliedKind`:
  `individual` → individual usage, `snapshot`/`timeslice`/`event` → occurrence usage, per
  SysML.xtext `IndividualUsage`, `PortionUsage`, `EventOccurrenceUsage`), so
  `symbols/builder.go` classifies it as `individualUsage`/`occurrenceUsage`. A modifier followed
  by a kind keyword and no name keeps that kind — `individual part` and `individual item` stay
  distinct, which is what #51 lost — and is reported as `ambiguous-modifier-kind`
  (`warnAmbiguousModifierKind`), for `ref` on the same footing as `individual`, `snapshot` and
  `timeslice`.
- ~~`for step in c { … }` is rejected.~~ **Done.** The loop variable is a usage declaration
  (SysML.xtext `ForVariableDeclaration: UsageDeclaration`), so `parseForAction` now names it with
  `parseIdentification` rather than demanding an `Identifier`: any keyword may name the loop
  variable, with the usual `reserved-keyword-name` warning. Reproduced as described — two errors,
  and the recovery misread `in` as a step usage and `c` as an enum usage. ~~Still open (the REPL
  layer is not touched here): `internal/repl` does not print load-time parse diagnostics.~~
  **Closed by the REPL work in this batch:** `%load` of a file with parse errors prints them
  rather than reporting the file as accepted.
- ~~`action a { in snapshot ; }` is silently accepted.~~ **Done, and it is legal.** SysML.xtext
  makes the declaration of a usage optional (`Usage: UsageDeclaration? UsageCompletion`), so an
  anonymous parameter is well-formed and needs no diagnostic. It did build a symbol, but as a
  plain part usage: the parameter path recorded `snapshot`/`event` and then ignored them.
  `parseDirectionParameter` now shares `modifierImpliedKind` and `warnAmbiguousModifierKind` with
  declarations (a direction is a usage prefix — SysML.xtext `RefPrefix`), so `in snapshot ;` and
  `in event ;` are anonymous occurrence usages and `in individual v : Vehicle` an individual
  usage. Still open: a parameter with no kind keyword at all (`in x : Real`) keeps the parameter
  path's own *part* default, where the same declaration outside a parameter list is a plain
  feature; that default is load-bearing for existing goldens and is out of scope here.
- ~~**A7-4 (0.0.8 audit): a KerML `datatype` is classified as an attribute usage.**~~ **Done.**
  In a `.kerml` file, `datatype D; classifier C specializes D; feature f : D;` reported `class
  cannot specialize attributeUsage` and `type must be a definition, found attributeUsage`, so
  nothing could specialize `D` or be typed by it; `function` failed the same way through
  `calcUsage`, while `class`, `struct`, `assoc`, `behavior` and `interaction` were already clean.
  The root cause was that `symbols/builder.go` `classifyUsage` decided a `datatype` from its
  relationships — `datatype Real specializes Complex` was a definition, bare `datatype D;` a
  usage — so a `datatype` is now a definition whatever it specializes, and `feature f : D;` is
  clean. `function` is deliberately left a `calcUsage`: a KerML function is invoked as a calc and
  the runtime resolves it through `SymbolCalcUsage`, so classifying it as a type breaks every
  library operator (`IntegerFunctions::+` and the rest) — the fidelity gap there is that nothing
  distinguishes a function *definition* from a calc usage, which needs a function kind in
  `ast.UsageKind`. Still open, and outside this item's files: `classifier C specializes D;` now
  reports `class cannot specialize attributeDef (kind mismatch)`, because
  `passes/typecheck.go` `specializationDiag` requires a plain `classifier` to specialize another
  classifier-kind definition, and reports it as `class`. KerML 1.0 §8.3.2 makes a `DataType` a
  `Classifier`, and only `Class` is disjoint with it (§8.3.3), so a plain `classifier` may
  specialize a datatype; the kind-compatibility matrix is the error site A7-4 forbids patching.

## A8 — a nested feature redefined on an object is not the subject of a check or an `%eval` — done

Landed: a nested redefinition is reached, so the meta-commands answer about the object rather
than the declaration and agree with `%slots`. On the entry's own model — `part def Outer { part b
: Inner { attribute redefines c = 9.0; } }` over an `Inner` defaulting `c` to `1.0` —
`%eval A::Outer::b::c` answers `9.00` and labels the subject it used
(`on A::o::b ID: 2`), and `%constraint A::Inner::small` fails against that same object rather
than passing against the declaration. The subject is labelled on every surface that resolves one,
including over gRPC, where an ambiguous subject is a stated failure reason rather than a silent
choice.

## A9 — a `calc` body without `return` is not expression-type-checked — done

Landed: the type tier reaches the expressions of a returnless `calc` body, so a body whose only
member is `attribute bad : Real = m + t;` over a mass and a duration reports
`operator '+' combines incommensurable quantities: MassValue (dimension M) and DurationValue
(dimension T)` where it previously reported nothing. The dimension inference was already right;
what changed is the pass's reach.

---

# Track B — REPL refinements — done

None of these were capability gaps; the REPL executes models. They were the rough edges a new
user meets, and all three are closed. Two REPL residuals that came out of the batch are recorded
with the items they belong to rather than reopening this track: `%eval TrigFunctions::pi` reads
the declaration rather than the library-feature seam (A4), and `Session.accept` still supersedes
an earlier snippet whose declared names intersect the new one, so re-typing a package body
replaces it rather than merging into it (B1).

## B1 — a declaration ends an in-progress debugger session — done

`Session.Submit` used to clear everything derived from the previous document, so any declaration
typed mid-session silently ended an `%action`/`%state` session and wiped instances. It is now
scoped to what the submission changed: `mergeSubmission` folds a re-typed namespace into the one
already in the session instead of replacing its body, `carryOverObjects` carries instantiated
objects whose declarations are untouched, and `dropStaleDebugSessions` ends only a session over a
declaration the submission rewrote — reporting it as a `note:` rather than failing the next
`%step` with "no active session". The wholesale paths follow the same rule now: a `%load` carries an
object over when the reloaded text still resolves the declaration to the shape the object was
materialized against (`Context.Adopt` is what proves it), and `%clear`, which replaces every
declaration and so can prove nothing, reports what it took and why — the next `%instances`, `%slots`
or `%step` explains the loss instead of reading as a session that materialized nothing. Documented in
`docs/guide/04-repl.md`.

## B2 — `%eval` of a compound expression cannot reach a package member — done

The prompt evaluates in the namespace the session is working in (`Session.promptScope`), which is
the namespace a member typed at the prompt would be written in, so `mass * 2` and `1.0 [m/s]` name
that namespace's members and imports. What remains is a choice, not a defect: that namespace is the
*last* one the session declared. A context is now named explicitly to decide it:
`%eval in Demo::Vehicle : mass * 2` evaluates in that element's namespace, and pinning a name an
object was materialized under reads that object's feature values, as an unpinned `%eval` does after
`%instantiate`. The default is unchanged — the last declared namespace — so nothing moved under an
existing session. Documented in `docs/guide/04-repl.md` and in `%help`.

## B3 — a quoted fully-qualified name breaks the meta-commands — done

A name the notation has to quote — a space, a keyword as a name, punctuation — was split on its
space before it was parsed, so `%instantiate 'My Pkg'::Car` failed with `unresolved reference: 'My`
while the CLI expression path and `%search` handled the same name. The prompt now reads a quoted
unrestricted name as one argument (`parseArgs`, and the raw tail `%calc` takes), normalizes it to the
name the index records at the one lookup every name-taking command shares (`Session.lookupSymbol`),
and reports a resolved name back in the spelling that can be typed into the next command
(`notationName`). The gRPC service follows the same rule: `internal/grpc/service.go` resolves a
symbol ID written either way, so `Instantiate`, `GetSymbol`, `Evaluate`, `ExecuteAction` and
`ExecuteState` accept `'My Pkg'::Car` as well as the unquoted `My Pkg::Car` clients already send.

---

# Track C — test coverage where it is thin — done

The three bullets, with what closed them:

- **`cmd/sysml-grpc` had no gate at all and is a published artifact — closed.** 0% → 18.3%.
  `cmd/sysml-grpc/lifecycle_test.go` builds the binary once and drives it as a process the way
  the LSP smoke test does: an ephemeral `-port 0`, readiness taken from the server's own
  listening line rather than a sleep, a real `GetServerInfo` → `ParseFile` → `Instantiate`
  exchange over the `internal/grpc` conformance fixture, then `SIGTERM`, exit status 0 and no
  leaked process. The failure lifecycle a user hits is covered too: an unknown flag, an occupied
  port, an invalid cache size and a missing model each exit non-zero with a typed message rather
  than hanging, and shutdown is asserted with a client connection still open. The reported
  percentage counts only the two in-process helpers (`server_test.go`), since a child process
  contributes nothing to the parent test binary's profile — the lifecycle itself is gated by the
  assertions, not by the number.
- **`internal/core/resolve` 56.3% → 77.9% and `internal/core/semantics` 54.8% → 57.3%.** The
  tests went to the subtle rules rather than to the percentage: a redefinition target found two
  supertypes up, redeclaration-versus-redefinition name conflicts, requirement parameters,
  reference collection and its resolution kinds, what an import actually surfaces through a
  filtered and a re-exporting route, and — on both the parsed and the cache-restored index — the
  same resolution answers for inherited features, feature chains, aliases and conformance
  (`resolve/cached_index_test.go`, `semantics/cached_library_test.go`), plus namespace-filter
  classification and annotation facts across the cache, and multiplicity bounds that are not
  evaluable. The semantics number moves least because its statements are largely reached from
  upstream package tests, not from its own: measured with `-coverpkg` over the whole suite the
  package is at 83.9%, and what remains uncovered there is scattered error branches rather than
  rules (`annotations.go`, `filter.go`, `units.go` diagnostics paths).
- **`internal/core/ast` stays at 20.9%, deliberately.** The number is misleading, as suspected:
  the 27 functions with no package-local coverage are accessors and formatting helpers
  (`DeclaredName`, `SimpleName`, `EffectiveName`, `AsQualifiedName`, `ReferencedTarget`, the
  `dump.go` writers, trivia accessors), and over the whole suite the package is at 85.7% —
  they are exercised by the parser, resolver and REPL tests that consume them. Tests written
  against them here would restate their bodies, so none were added.

Two production defects the new tests found are recorded as skipped tests naming them, not
worked around: a cache-restored library feature loses its declared multiplicity (`libs`
`symRecord` persists none, so it takes the assumed `1..1`), and a same-named subsetting target
(`part wheel :> wheel;`) resolves to the subsetting feature itself rather than to the inherited
feature. Both live outside the files this track owns; the fixes belong with `internal/core/libs`
and `resolve.resolveSpecialization` respectively.

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

## D4 — the parser records `then` ambiguously, so successions cannot convert — done

Landed: a member-attached `then` is desugared at parse time into the `*ast.SuccessionEdge` the
edge notation (`then a b;`) already built, synthesised into the enclosing body with the member
before the keyword as its source and the member after it as its target
(`parser/succession.go`). `HasSuccession`, `SuccessionTarget` and `SuccessionGuard` are gone
from `ast.Membership` along with every site that set them, so there is one representation of a
succession, and the lowering and RDF paths that already honoured the edge form now honour every
`then`: execution follows the edges rather than member order (conformance case
`action_member_then_order`, whose declaration order is the reverse of its execution order), and
the mapping emits `sysml:SuccessionAsUsage` with `sourceFeature`/`targetFeature` and reads it
back (`export_test.go:TestSuccessionRoundTrips`). The one-name form (`then b;`) is completed the
same way, from the member before it, rather than leaving its source empty for a consumer to
guess at.

What the account of the bug above got wrong, measured before the change: the two sites did not
disagree about which member the flag marked. The body loop tested for `then` immediately after
parsing the previous member, so the trailing site claimed the keyword first and the prefix site
was reachable only for a leading `then` — one rule plus an unreachable-in-practice branch, not
two contradictory ones. `SuccessionGuard` was indeed never assigned, and no notation would have
filled it: SysML.xtext's `EmptySuccession` is `'then'` plus two empty ends and has no guard field
— a guard needs `GuardedSuccession`, its own member spelled
`first <source> if <guard> then <target>` — so the field went without replacement, and
`then part b if a;` is the syntax error it already was.

What is left, recorded as ⚠️ in `docs/project/spec-compliance.md`: a succession edge names its ends, so
a `then` beside a member with no name — `then send Show(x) to screen;`, or a `then` after an
anonymous member — declares an order this representation cannot carry. It warns
(`unnamed-succession-end`) rather than silently dropping the keyword or failing a legal model.
Carrying it needs an end that refers to a member by identity rather than by name, which is a
change to the edge node, the lowering that resolves ends by name, and the RDF ends alike.

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

Tracks A, B and P are closed, so what is left reorders:

1. **R1** (tag), then **R2**/**R3**/**R5** as the account access appears. R1 gates the rest of
   the release section, and R2 is what makes Track P's work reachable by a user.
2. **Track C** is done: `cmd/sysml-grpc` now has the process lifecycle gate it lacked, and the
   resolver and semantics rules that were tested only on the parsed index are tested on the
   cache-restored one as well.
3. **Track D** is independent of the rest and can run whenever. Take **D3** before **D1**/**D2**:
   it is the cheapest, and it is what would show whether the Flexo interop claim actually holds
   before more work is layered on the mapping. **D4** is done; what it left behind — a succession
   end that refers to an unnamed member — belongs with **D2**, since both want real end triples
   rather than names or text.
