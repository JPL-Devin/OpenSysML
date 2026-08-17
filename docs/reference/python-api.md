# The Python client API

What `pysysml` costs, the typed classes it can generate for a model, and the modules behind
them. Using the client as a task is [guide chapter 9](../guide/09-python.md).

## Latency

Measured on this repo's benchmark (`python/scripts/bench_latency.py`, 8-core
x86-64 Linux, loopback, 20 part definitions / 808 bytes, 200 iterations, warm
`Connection`):

| operation | p50 | p95 | p99 |
| --- | --- | --- | --- |
| `load` / `load_from_content`, cache miss | 35 ms | 56 ms | 60 ms |
| `load` / `load_from_content`, cache hit | 0.5 ms | 1.0 ms | 1.0 ms |
| `eval("2 + 2")` on a cached model | 0.4 ms | 1.0 ms | 1.2 ms |
| `convert` sysml → sysml | 0.7 ms | 1.2 ms | 2.0 ms |
| `convert` sysml → ttl | 1.1 ms | 1.4 ms | 1.5 ms |
| `convert` ttl → sysml | 1.2 ms | 1.5 ms | 2.7 ms |

Reproduce with `make build-grpc && bin/sysml-grpc` and
`python python/scripts/bench_latency.py --iterations 200`.

The shape matters more than the absolute numbers: a parse costs two orders of
magnitude more than a query on the parsed result, because it loads the standard
library into a fresh symbol index and runs the semantic passes. Everything else
here is around a millisecond, of which the RPC itself — protobuf encode/decode
plus loopback — is a few hundred microseconds.

### Starting the service

A `Connection` that starts `sysml-grpc` itself probes the service immediately and
then on a doubling backoff (10 ms, 20 ms, 40 ms … capped at 250 ms), so a service
that answers in milliseconds costs milliseconds: ~17 ms on the machine above,
against a fixed 500 ms wait before 0.0.9. That wait is bounded by
`pysysml.connection.START_TIMEOUT` (2.5 s) — sleeping and probing together, since
no probe is given more than what is left of it — after which `ConnectionError` is
raised. A probe of a port nothing listens on is refused in a few milliseconds
rather than spending the per-probe RPC timeout, and a port that accepts without
answering costs the remaining bound, not another whole timeout on top of it. The
probe deciding whether to adopt a service already listening is made before that
wait starts and has its own 5 s timeout, so an address held by something that
never answers is reported in ~7.5 s. `Connection(auto_start=False)`
costs ~0.3 ms and the first RPC on it ~1 ms; `import pysysml` is ~120 ms, mostly
`grpc` (~48 ms), `filelock` and the generated protobuf modules.

### Real-time analytics

This is a request/response service over gRPC, not a hard real-time engine. It
gives no deadline guarantee, and nothing in it is scheduled: the runtime's step
and time budgets bound how long an execution may run, which caps a worst case
rather than promising one. Tails come from the Go garbage collector, the
scheduler, TCP, and — for a cache miss — a parse whose cost scales with the
document. Treat the p99 above as a soft budget, and measure your own p99 on your
model sizes, cache state and concurrency before trusting one.

What that means in practice:

- **Reuse one `Connection`.** Channel setup plus the first parse is tens of
  milliseconds; the module-level functions already share a singleton.
- **Parse once, then query by hash.** Repeated `load` of unchanged content hits
  the service cache and costs ~0.5 ms, but asking the model itself (`model.eval`,
  `model.instantiate`, `model.execute_*`, which pass its hash) skips even that. The cache holds 100 models
  (`-cache-size`) and evicts least-recently-used, so a stream of distinct
  sources will evict a model you still hold a hash for.
- **Batch.** One RPC carrying many samples beats one RPC per sample; at ~0.5 ms
  of overhead per call, a per-sample loop tops out in the low thousands of calls
  per second per connection.
- **Keep the hot loop local.** Filtering, thresholding and windowing over a
  telemetry stream belong in NumPy in your own process. Use the service for the
  coarse-grained step — resolving a model question, instantiating, running an
  action or state machine, converting a model — not for every sample.
- **Convert off the hot path.** Conversion re-parses its input on every call; it
  is not cached by content the way `load` is.

## Generated typed classes

`Instance` is dynamic, so an editor cannot complete `inst.mass` and a type checker
cannot reject `inst.mas`. `pysysml.generate` emits a Python class per SysML
definition, so both can:

```bash
python -m pysysml.generate internal/repl/testdata/vehicle_package.sysml -o demo_types.py
pysysml-generate model.sysml -o model_types.py     # same thing, as a console script
```

```python
import pysysml
from demo_types import Vehicle

model = pysysml.load("internal/repl/testdata/vehicle_package.sysml")
inst = model.instantiate("Demo::Vehicle")

v: Vehicle = Vehicle.from_instance(inst)   # a typed view over the Instance
v.mass                                      # 1500.0, typed float
v.engine.power                              # 300.0, through the generated Engine
v.instance                                  # the underlying Instance
```

mypy (or pyright) then reports `v.mas` as an unknown attribute and `v.mass + "x"`
as an unsupported operand pair. `pysysml` ships a `py.typed` marker, so its own
annotations are used too.

`from_instance` rejects an instance of another definition, naming both types,
rather than failing later at attribute access. An instance of a definition that
specializes the expected one is accepted, since its generated class derives from
the expected class. An instance whose type no generated class describes is
accepted: instantiating a *usage* reports the usage's own FQN (`Demo::myCar`, not
`Demo::SportsCar`), which the client cannot relate to a definition, so rejecting
it would break the ordinary way to obtain an instance. `Vehicle.unchecked(inst)`
is the explicit escape hatch for a deliberately unchecked view.

### Keeping a generated module honest

A generated module records what it came from, so a stale one can be detected
rather than discovered at attribute access (or never, when a removed feature keeps
type-checking):

```python
SYSML_GENERATOR_VERSION = "1"   # emission schema of this generator
SYSML_MODEL_HASH = "sha256:…"   # hash of the model source it was generated from
```

`--check` regenerates in memory and compares, writing nothing:

```bash
python -m pysysml.generate model.sysml -o model_types.py --check   # exits 1 if stale
```

It exits non-zero when the module is missing or would change, naming the command
that regenerates it, which makes it usable as a CI or pre-commit gate.

Generation requires a service that reports the `type_facts` capability, which it
asks for over `GetServerInfo`. A service too old to answer that RPC, or one that
answers without the capability, does not populate `SymbolInfo.type_info`, and
generating against it would type every feature `object` — indistinguishable from a
feature that is genuinely untyped. Generation therefore fails, naming the service
in use, where it came from, and how to replace it, rather than emitting a silently
useless module.

The generator emits a **runtime `.py`**, not a `.py` + `.pyi` pair: each feature is
a property that carries the annotation and performs the delegation, so the types
and the code that implements them cannot drift apart, and there is one artifact to
commit. Output is deterministic — definitions ordered by fully-qualified name (base
classes first), nothing environment-dependent written — so it can be committed and
diffed; `python/tests/golden/vehicle_types.py` is exactly that.

Generated classes are views, not copies: attribute access goes to the underlying
slot on every read, and Tier 1 behaviour is preserved. A slot that failed to
evaluate raises `SlotError`; a slot holding a value of another type than the model
declared raises `TypeMismatchError` rather than returning a wrongly typed value.

### SysML → Python mapping

| SysML | Python |
| --- | --- |
| `Real`, `Rational` | `float` |
| `Integer`, `Natural` | `int` |
| `Boolean` | `bool` |
| `String` | `str` |
| usage typed by a definition that reduces to a library scalar (`attribute def Celsius :> Real`) | that scalar (`float`) |
| usage typed by an `enum def` | `EnumLiteral`, the identity of the literal held |
| usage typed by any other definition in the model | that definition's generated class |
| multiplicity `1`, `1..1`, or undeclared | `X` |
| multiplicity `0..1` | `X \| None` |
| `*`, `0..*`, `n..m` with upper > 1 | `list[X]` |
| `Complex`, `Number` | `object`, with a comment naming the type |
| a type resolved outside the model (e.g. a library type) | `object`, with a comment naming its FQN |
| an unresolved or absent type | `object`, with a comment naming what was written |
| `specializes`, `subsets` or `redefines` a definition in the model | Python base class |

The fallback is always `object` and always says why in the property's docstring;
no feature is given a type the model does not support, and `Any` is never used to
dodge one.

### Known limitations

- **Behavioral and connector usages.** Only structural usages (attribute, part,
  item, occurrence, individual, port, enum) become properties. Action, state,
  calc, constraint, requirement, connection, flow, interface, allocation and case
  usages are not instance slots and are skipped.
- **Redefinition narrowing.** A redefinition reuses the redefined feature's name,
  so its property overrides the base class's, and takes over the type and
  multiplicity it does not restate; a redefinition that *narrows* the type is
  emitted with its own declared type, which Python does not check against the
  base property.
- **Multiple inheritance.** Emitted in declaration order, a target named twice
  appearing once, and a base another declared base already specializes is left
  implicit (`Hybrid :> Vehicle, Electric` with `Electric :> Vehicle` emits
  `class Hybrid(Electric)`). A hierarchy that linearizes no way at all keeps the
  bases it can and names the left-out edge in a comment, rather than emitting a
  module that fails to import.
- **Generics and enumerations.** No generic parameters. An `enumDef` becomes a plain
  class rather than a Python `Enum`; a usage typed by it is `EnumLiteral`, which
  carries the literal's declaration identity but does not enumerate its siblings.
- **Name collisions.** Two definitions with the same simple name both get
  path-qualified class names (`A_Thing`, `B_Thing`). A feature named like a member
  `TypedObject` provides (`instance`, `from_instance`, `sysml_id`) gets a trailing
  underscore (`instance_`); the SysML slot name it reads is unchanged.

`pysysml.connect(host, port, auto_start=True)` returns a `Connection` when you
want to manage the service yourself; the module-level functions share a lazily
created singleton connection instead. A `host:port` address written as the host
is read as one — `connect("localhost:50123")` reaches port 50123 — and a port
named twice with two values raises `ValueError` naming the disagreement rather
than timing out against an address nobody asked for. The helpers taking
`host`/`port` (`load`, `evaluate`, `convert`, `instantiate`) read it the same way.

`pysysml` never stops a service it did not start. A service it starts is
reference-counted *within the process that started it* and stopped when the last
connection holding it is closed or the interpreter exits; a connection that
attaches to a service already listening takes no reference and leaves it running,
whatever it does. The service is recorded in `~/.pysysml/sysml-grpc-<port>.pid`
(`$PYSYSML_STATE_DIR` overrides the directory) as its pid and process start time
plus the pid and start time of the process that started it, and every one of
those pids is re-checked against the start time written for it — a pid the
operating system has reused is a stale record, cleaned up rather than signalled,
and a command line that merely looks like `sysml-grpc` is not identity. A service
that crashes leaves such a record; the next connection removes it and starts one
of its own.

A service *already listening* on the port is checked the way the cached binary
is: it is asked what it is with `GetServerInfo`, and a release other than the one
asked for raises `StaleServiceError` naming the mismatch and the remedy, instead
of serving an old build whose first newer call fails as a
`MissingCapabilityError`. `connect(version=…, require_capabilities=[…])` asks
explicitly; `PYSYSML_GRPC_VERSION` asks for a release for the binary cache and
the running service alike, and with neither set whatever answers is accepted.
Such a service is stopped only when this process started it, no connection of its
own still holds it, and the binary that would be started in its place can be
shown to be the release asked for — a service you are running deliberately is
never killed, and one is never stopped only to start the same build again, so
the remedy asks you to stop it, name another port, or accept what is running. A
service that only lacks a required *capability* is therefore always reported as
`MissingCapabilityError`, never replaced: capabilities come with a release, so
restarting the same binary would report the same ones, and the class you catch
does not depend on who started the service. `auto_start=False` checks the release
too — reporting a mismatch stops nothing, so it needs no ownership — but stays
lazy: a service of yours that is not listening yet is checked once it answers.

## Development

```bash
pytest python/tests/                    # unit tests
pytest -m integration python/tests/     # needs a running sysml-grpc

# Tests needing a service skip without one; CI sets this so an absent service
# fails the run instead of quietly passing.
PYSYSML_REQUIRE_SERVICE=1 pytest python/tests/

# Regenerate the committed golden generated file (needs a running sysml-grpc)
python -m pysysml.generate internal/repl/testdata/vehicle_package.sysml \
    -o python/tests/golden/vehicle_types.py

# Regenerate protobuf bindings (from the repository root)
pip install grpcio-tools
make python-proto
```

## Modules

- `binary.py` — locates, downloads and checksum-verifies `sysml-grpc`
- `connection.py` — gRPC channel, service lifecycle, ownership of services it started
- `model.py` — a parsed model: root symbol and diagnostics
- `symbol.py` — lazy symbol proxy, fetches children on demand
- `instance.py` — instantiated object and its slots
- `conversion.py` — a written model, its formats and extension inference
- `query.py` — the standard's Query payload, translated and its answers
- `verdict.py` — a verification's answer and what a calculation computed
- `errors.py` — the exception hierarchy and the gRPC status translation
- `capabilities.py` — what the connected service reports it supports
- `typefacts.py` — a symbol's static type, multiplicity and supertypes
- `typed.py` — base class and slot decoders the generated classes are built on
- `generate.py` — emits typed classes from a parsed model
- `diagnostic.py` — one diagnostic with its source location
- `proto/` — generated message classes and stubs
