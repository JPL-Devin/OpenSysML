# Systemica — Roadmap

Baseline: `main` @ `b1e2900`, verified locally on 2026-08-15 with Go 1.25.13.
Read `AGENTS.md` first; it governs everything below.

0.0.7 is released from `Open-MBEE/Systemica`, carrying `sysml`, `sysml-lsp` and `sysml-grpc`
archives. `main` since carries the nine post-0.0.7 fixes, the documentation reorganization and
the release-publishing fix, all listed under 0.0.8 in `CHANGELOG.md`, which is cut and awaiting
its tag. Everything in "Release follow-through" is maintainer- or account-gated; everything
after it is ordinary engineering work.

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`, `staticcheck ./...`,
`go test -race ./...`.

| Gate | Count |
|---|---|
| OMG training corpus | **98/100 clean** — 2 files / 4 errors, both pinned OMG source bugs (the ceiling) |
| Stdlib parser conformance | 95/95 clean — 94 vendored OMG files and 1 non-normative Systemica extension |
| Execution conformance cases | 211 |
| gRPC conformance cases | 8 |
| Golden execution traces | 69 |
| Runtime robustness subtests | 146 |
| Golden AST fixtures | 82 |
| Negative parser subtests | 127 |

Statement coverage, measured today with `go test -cover ./...`:

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/format` | 96.6% | `internal/core/model` | 79.5% |
| `internal/core/source` | 90.9% | | |
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
drifted file and record the verdict in `docs/project/training-examples.md`.

---

# Release follow-through

## R1 — tag the next release (maintainer, blocking everything else in this section)

Releases live on `Open-MBEE/Systemica`; development happens on `JPL-Devin/Systemica`, which
has no tags at all. So the tag is preceded by promoting `main` upstream, as 0.0.4 was through
Open-MBEE PR #47:

```bash
# on Open-MBEE/Systemica, after main carries the release commit
git tag -a v0.0.8 -m "v0.0.8"
git push origin v0.0.8
```

The publish job needs `GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project.
Without one the tag builds artifacts and then fails at publish, having created no release.
Nobody has verified which is set. Full procedure and post-tag verification:
`docs/project/releasing.md`.

## R2 — publish `pysysml` to PyPI (account-gated remainder)

The job exists: `publish-pypi` in the `release-python` workflow, filtered to `pysysml-v*`,
building a wheel and an sdist, checking them with `twine check --strict`, installing the wheel
into a clean virtualenv and only then uploading. The version is declared once, in
`python/pysysml/_version.py`, and a tag that disagrees with it fails before upload. The
package keeps its own version line on purpose: it resolves a `sysml-grpc` binary at runtime
from whichever release the caller names, so its version and the core's are not lockstep.
See `docs/project/releasing.md`.

What remains is account-gated and cannot be done from a session: create the PyPI project's
first release with an account-scoped token, then replace it with a project-scoped one; create
the restricted CircleCI context `PyPI` holding `PYPI_API_TOKEN` (and optionally
`TEST_PYPI_API_TOKEN` for pre-release tags).

Also decide the default download repository. `python/pysysml/binary.py` defaults to
`Open-MBEE/Systemica`, releases are currently cut from `JPL-Devin/Systemica`, and
`PYSYSML_GITHUB_REPO` is the override. `sysml-grpc` assets ship from 0.0.5 onward,
so `pysysml` can fetch a binary from a released tag; `pip install pysysml` still waits on the
PyPI project above.

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
`docs/project/macos-distribution.md`: it is `com.apple.quarantine`, not a missing signature — Go's
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

`python/tests/test_integration.py` and `test_runtime_integration.py` (20 tests) skip themselves
unless a service answers on `localhost:50051`. CI installs the binary but starts nothing, so
they skip there too, and the client↔service path has never been exercised by CI.

Starting a service in the `python-test` job is **not** the fix on its own — verified: with one
running, the 20 tests pass but `test_lifecycle.py::test_service_shuts_down_when_last_process_exits`
fails, because `Connection._ensure_service` returns early when it probes a healthy service and
writes no pidfile, so the refcount can never shut that service down and the test cannot find a
pid to watch. Half of that ownership model is now in place: a connection releases a reference
only if it took one, so attaching with `auto_start=False` no longer shuts down a service the
process never started. The rest of the gap remains: the test still stops a service another
live process owns, leaving a stale pidfile and refcount behind. Decide the rest of the
ownership model first — pysysml should probably not kill a service it did not spawn, and the
test should then not require it to — then start the service in CI. The
job's comment in `.circleci/config.yml` records this.

## P2 — a nested object is unreachable over gRPC — done

`Instantiate` reads slots through `Instance.GetSlot`, so a derived attribute comes back
evaluated instead of unmaterialized (conformance `instantiate_derived_slot`). A slot holding an
*object* still marshals as that child instance's id, but `InstantiateResponse.instances` now
carries every instance reachable from the root, so `part engine : Engine` expands for a Python
caller (`inst.engine.power`) as `%slots` expands it in the REPL. Expansion is bounded the way
`%slots` bounds it — depth 8, and no descent into a type already on the path, since reading a
composite slot materializes the object it holds and a self-referential part would otherwise
instantiate forever; an unexpanded child stays a bare id. A `GetInstance` RPC was
rejected: runtime instances live in the request's `runtime.Context` and do not survive the
call, so an id is only meaningful against the response that carried it — noted as a limitation
in `docs/project/spec-compliance.md`.

## P3 — generated typed classes for a parsed model — done

`python -m pysysml.generate model.sysml -o model_types.py` emits one class per SysML definition,
each a typed view over the `Instance` P2 made navigable, so `v.mass` completes in an editor and
`v.mass + "x"` is a mypy error; `pysysml` ships `py.typed`. Codegen reads facts the service now
resolves rather than scraping them: `SymbolInfo` carries `type_info` (declared and resolved type,
the library scalar it reduces to, and whether it is a quantity), `multiplicity`, and *every*
generalization edge in `specializations` — `extractMetadata` exported only the first `specializes`
of a definition and the first typing of a usage, which cannot express multiple supertypes or tell
subsetting from redefinition. Known limitations are listed in `python/README.md`; the load-bearing
ones are that a quantity slot is typed `object` because the wire `Value` has no
magnitude-and-unit form (the same reason `ValueToProto` reports one as unsupported), that only
structural usages become properties, and that `subsets`/`redefines` are reported but do not
become base classes.

## P4 — smaller Python-side items, all recorded in `docs/project/spec-compliance.md`

- `connection.py` verifies a pid is the service by substring-matching its cmdline — spoofable.
- `pysysml.eval` and `pysysml.RuntimeError` shadow builtins.
- `SymbolToProto.Attributes` is always empty (`convert.go:40`), so `Symbol.attributes()` and
  `to_dataframe()` under-report.
- The download verifies a checksum served from the same origin as the binary, which detects
  corruption but not a compromised release; a pinned hash per version would be stronger.
- `Model.eval` takes a scope, not a subject, so an expression cannot be evaluated against an
  object the way `%eval` does after `%instantiate`: `verify_constraint` takes a subject, `eval`
  does not, and a caller reads the declared default instead. Carrying the subject to
  `Evaluate` is a service-side change as well as a client one.

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

## A2 — a typed multi-valued feature ignores its default

A feature that is both typed and given a default takes the typed instantiation; the default is
not merged. Second known limitation in the changelog. Decide the semantics before coding — the
spec question is whether the default supplies elements or replaces the instantiation.

## A3 — a library value type does not resolve as a type in the REPL path

`attribute d : Real;` with no default reports `<unknown>` (reproduced today), because the
reference does not resolve to a type at all rather than because instantiation fails. Related to
A6; do A6 first and re-test this.

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

## A4 — executor approximations

- **Port routing ignores direction and conjugation.** A message reaches every port connected by
  a connector in the same behavior body, and a port of the enclosing part is invisible to the
  behavior.
- **Accept-parameter visibility.** The payload lives in the action's shared feature space, which scoping does
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
  `MatrixFunctions`, `ComplexFunctions` and the rest of `SequenceFunctions` are not (quantity
  arithmetic is, as of A1a). `TrigFunctions::pi` has no declared value, so the
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

## A10 — an enumeration literal has no runtime value — done

Landed: a literal declaring no value evaluates to itself — `runtime.Value{Kind: ValEnumLiteral}`
holding the literal's declaration symbol, so identity is the declaration and nothing else (not an
ordinal, not its name as a string). Two literals are equal exactly when they are the same
declaration, so a literal of another enumeration compares false rather than reporting, matching how
Systemica compares any two values of unrelated types; the static type checker is the tier that
reports the mismatch (`passes/typecheck_value.go`, `cannot bind a value of type Size to a feature
typed by Color`). A literal specializing a scalar type (`enum def GradePoints :> Real { A = 4.0; }`)
evaluates to the value it declares, since such a literal *is* that value and has to compute as one.
A literal is an occurrence of its enumeration, so its own features are read off one object per
literal (`Level::high.n`), and an enum-typed default materializes: the reproduction prints
`c = Color::red`. `runtime/value.go`, `eval.go`, `semantics/enumeration.go`, conformance cases
`enum_literal_default_slot`, `enum_literal_own_attributes`, `enum_literal_scalar_valued`.

## A5 — visibility rules

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
  `ViewBodyItem` only). A `view def` body is a **warning**, not an error, because Systemica
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
  Still open: `@`/`@@` in the runtime evaluator (A4), and a view's exposed-element set as a
  queryable API.

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

## A8 — a nested feature redefined on an object is not the subject of a check or an `%eval`

The direct case landed: a condition declared on a definition is checked against the object
carrying it. A *nested* redefinition is not reached, so `%eval A::Outer::b::c` and
`%constraint A::Inner::small` answer about the declaration while `%slots` shows the
instantiated value — the same class of wrong-subject answer, one level down.

## A9 — a `calc` body without `return` is not expression-type-checked

Such a body reaches no expression-typing pass, so the static dimensional warning (and every
other type-tier finding about its expressions) cannot be reported inside it. The gap is in the
pass's reach, not in the dimension inference.

---

# Track B — REPL refinements

None of these are capability gaps; the REPL executes models. They are the rough edges a new
user meets.

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
object was materialized under reads that object's slots, as an unpinned `%eval` does after
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
[the RDF mapping](../reference/rdf-mapping.md). What that work deliberately left open:

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
filled it: SysML.xtext's `EmptySuccession` is `'then'` plus two empty ends and has no guard slot
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

Constraint-bearing models convert since the condition members were mapped, but an action or
state body that carries nodes still reports the node and aborts: measured on the built binary,
71 of the 120 models under `examples/` convert (8 of the 19 top-level models, 62 of the 100
training copies, and the semantic-layer demo), and the 49 refusals are initial nodes, `perform`,
`send`, `terminate`, loop nodes, state regions and substates, plus four duplicate declarations.
`docs/reference/rdf-mapping.md` § Limitations lists the shapes. The fix is metaclasses for the behavioral
nodes, not a wider fallback — a graph missing a model's steps would be worse than a refusal.

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
3. **A2** — A1 is done end to end, so it is the limitation the changelog still admits to — then **A4**/**A5** in
   parallel (they share only `docs/project/spec-compliance.md`; the two `state_executor.go` items in A4
   must run one at a time).
4. **A6** last, gated on a per-file corpus diff.
5. **B1**, **B2** and **Track C** are good filler sessions: small, isolated, and each closes a
   rough edge a user would otherwise report.
6. **Track D** is independent of the rest and can run whenever. Take **D3** before **D1**/**D2**:
   it is the cheapest, and it is what would show whether the Flexo interop claim actually holds
   before more work is layered on the mapping. **D4** is done; what it left behind — a succession
   end that refers to an unnamed member — belongs with **D2**, since both want real end triples
   rather than names or text.
