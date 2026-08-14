# Changelog

Notable changes per release. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Cutting a release
is described in [docs/RELEASING.md](docs/RELEASING.md).

## Unreleased

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

### `sysml` command line

- **Breaking:** conversion is spelled `sysml model.sysml -convert ttl`: the model
  is a positional argument as it is in every other mode, and `-convert` names the
  format to convert it to. `-convert <file>` and `-to <format>` are gone — `-to`
  reports the replacement rather than "flag provided but not defined" — and the
  output path no longer chooses the format, so `-o /dev/null` or a FIFO needs
  nothing extra. `-from` still names an input format the extension does not.
- A flag may be written after the model it applies to (`sysml model.sysml -trace`),
  which Go's flag package would otherwise read as two files to load.

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
  documented in [docs/RDF_INTEROP.md](docs/RDF_INTEROP.md); a refused construct
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
  [docs/RELEASING.md](docs/RELEASING.md#releasing-pysysml-to-pypi).

### Known limitations

- RDF/Turtle conversion has no mapping for a member that states a condition or a
  step, so a model containing one is reported rather than converted: `require`
  and `assume` members, a constraint body's condition, `subject`, a computed
  `return`, `assign`, `if`/`while`/`loop`/`for`, substates, transitions and
  `entry`/`do`/`exit` (`cannot convert the *ast.RequireMember at <file>:<line>`).
  A requirement stating a condition, and any state machine or action body with
  statements, must be saved as `.sysml`. The full list is in
  [docs/RDF_INTEROP.md](docs/RDF_INTEROP.md).
- The REPL's prompt evaluates in the *last* namespace the session declared. After
  typing a second package, the first package's members and the units its imports
  brought in are reached by qualified name only (`1.0 [SI::m]`, not `1.0 [m]`).
- Re-typing a declaration whose name the session already holds replaces the
  earlier snippet rather than merging into it, so adding a member to a package by
  re-typing the package drops the members left out of the new text.
- A multi-valued feature that is both typed and given a default takes the typed
  instantiation; the default is not merged into it (as in 0.0.4).
- An attribute declared with a type but no value (`attribute diameter : Real;`)
  instantiates as an object of that type rather than an unset value, so `%slots`
  shows `diameter = Instance(ID: n)` with `(no features)` under it.
- The macOS and Windows binaries are unsigned, so a browser download is
  quarantined by Gatekeeper or flagged by SmartScreen. Install with Homebrew or
  `curl`; see [docs/MACOS_DISTRIBUTION.md](docs/MACOS_DISTRIBUTION.md).

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
  [docs/SPEC_COMPLIANCE.md](docs/SPEC_COMPLIANCE.md); 98/100 of the OMG
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
  see [docs/MACOS_DISTRIBUTION.md](docs/MACOS_DISTRIBUTION.md).

### Known limitations

- A parameter bound by a constraint or requirement usage
  (`constraint limit : MassLimit { in m = mass; }`) is not passed into the
  conditions it inherits from its definition. *(Fixed after this release; see
  0.0.5.)*
- A multi-valued feature that is both typed and given a default takes the typed
  instantiation; the default is not merged into it.
