# pysysml

Python client for Systemica: parse, inspect and execute SysML v2 models over the
`sysml-grpc` service.

## Installation

```bash
pip install pysysml             # from PyPI, once the first release is published
pip install -e python/          # or from a checkout, at the repository root
```

Dependencies (`grpcio`, `protobuf>=7.35.1`, `filelock`, `psutil`) come with it.
They publish wheels for CPython 3.10 and later only, which is what
`requires-python` says.

## Getting the service binary

Every call goes through `sysml-grpc`. `pysysml` starts one for you and expects to
find it at `~/.pysysml/bin/sysml-grpc`. Three ways to put it there:

```bash
# 1. Download the release build (checksum-verified against its .sha256 sidecar)
python -c "from pysysml.binary import download_binary; download_binary('latest')"

# 2. Let pysysml.connect() download it on first use
export PYSYSML_GRPC_VERSION=latest      # or a tag like v0.0.5

# 3. Build from source
make build-grpc && mkdir -p ~/.pysysml/bin && cp bin/sysml-grpc ~/.pysysml/bin/
```

Without one of those, `connect()` raises `ConnectionError` rather than
downloading anything unasked. `PYSYSML_GITHUB_REPO` overrides the repository
releases are fetched from (default `Open-MBEE/Systemica`).

The published releases up to v0.0.4 carry the `sysml`/`sysml-lsp` archives only;
`sysml-grpc` binaries are published from the next release onward, so until then
build it from source (option 3).

## Usage

```python
import pysysml

model = pysysml.load("model.sysml")
for d in model.diagnostics:
    print(d)

print(pysysml.eval("1 + 2 * 3", file_path="model.sysml"))
```

### Inspecting symbols

`Model.find` takes a **short** name and searches the symbol tree:

```python
vehicle = model.find("Vehicle")   # not model.root.find(...), and not an FQN
vehicle.attributes()              # [Symbol(id='Demo::Vehicle::mass', kind='attributeUsage')]
vehicle.parts()                   # [Symbol(id='Demo::Vehicle::engine', kind='partUsage')]
vehicle.get_attr("mass")          # Symbol, or None if there is no such attribute
```

`model.get("Demo::Vehicle")` looks a symbol up by fully-qualified name instead.

A symbol also carries its static type facts, resolved by the service:

```python
engine = model.get("Demo::Vehicle::engine")
engine.type_facts        # TypeFacts(declared='Engine', resolved_id='Demo::Engine', ...)
engine.multiplicity      # Multiplicity(lower='0', upper='1'), or None if undeclared
engine.specializations   # [Specialization(kind='typing', target_id='Demo::Engine', ...)]
```

### Instances

Slot values come back as Python values, and a slot holding an object comes back
as a nested `Instance`:

```python
inst = pysysml.instantiate("Demo::Vehicle", model_hash=model.hash)

inst.mass                 # 1500.0
inst["mass"]              # 1500.0
inst.engine               # Instance(id=2, type='Demo::Engine', slots=1)
inst.engine.power         # 300.0
inst.slots                # {'mass': 1500.0, 'engine': Instance(...)}
inst.get("missing", 0)    # 0
```

Integers, reals, booleans, strings and sequences map to `int`, `float`, `bool`,
`str` and `list`. Unknown names raise `AttributeError` (attribute access) or
`KeyError` (item access), so `hasattr`, `copy` and `pickle` behave.

The service expands the object graph to depth 8 and stops at a type already on
the path, so a part containing its own kind terminates; a child it did not
expand comes back as its bare integer id rather than an `Instance`.

The raw protobuf stays reachable: `get_slot(name)` returns the `SlotValue`
message, and `raw_slots` is the whole map.

```python
inst.get_slot("mass").materialized         # True
inst.get_slot("engine").value.instance_id  # 2
```

A slot the service could not evaluate — a cyclic derived attribute, say — is
never reported as `None`. Attribute and item access raise `SlotError`, while
`slots` carries the `SlotError` as that entry's value so the rest of the
instance stays inspectable.

```python
cyclic.a             # raises SlotError: slot 'a': ... cyclic slot dependency
cyclic.slots["a"]    # SlotError(...)
```

`SlotError` is not an `AttributeError`, so `hasattr` on such a slot propagates it rather than
returning `False`; use `slots` to inspect an instance whose slots may have failed.

`eval` returns a single value, so a result the wire format cannot represent raises
`UnsupportedValueError` rather than being reported per entry.

`execute_action` and `execute_state` apply the same policy to their result maps:
a value the wire format cannot represent is reported as an
`UnsupportedValueError` in that entry, leaving the other entries intact.

### Writing a model back out

A loaded model can be written back to SysML notation or RDF Turtle. The service
does the conversion with the same code `sysml -convert` uses, so the client adds
no second implementation of the mapping.

```python
model = pysysml.load("model.sysml")

model.to_sysml()                 # Conversion: SysML notation
model.to_turtle()                # Conversion: RDF Turtle
model.save("model.ttl")          # writes Turtle; format taken from the extension
model.save("out.sysml")

pysysml.convert("ttl", file_path="model.sysml")            # without loading first
pysysml.convert("sysml", content=turtle, from_format="ttl")  # Turtle back to notation
```

A `Conversion` is the output text plus the formats it went between; `str()` and
`len()` give the text, and `write(path)` saves it. Formats are named `sysml`,
`kerml`, `text`, `ttl`, `turtle` or `rdf`. A file path's format is inferred from
its extension; inline `content` has no extension, so it needs `from_format`.

What each direction preserves:

- **Notation → notation** re-emits the model from its source, so comments and
  layout survive. It is source-preserving, not a general AST printer: a model
  the client built element by element cannot be printed this way.
- **Notation → Turtle → notation** returns an equivalent model, not identical
  bytes. Comments do not survive the graph, since RDF has nowhere to keep them.
  See [docs/RDF_INTEROP.md](../docs/RDF_INTEROP.md) for what a graph must carry
  for the round trip back.
- Syntax errors normally fail the conversion and come back as a
  `ConversionError` carrying `diagnostics`. `tolerate_syntax_errors=True` writes
  notation anyway and reports the errors as `Conversion.diagnostics`; it applies
  to notation → notation only, because every other direction builds a graph
  where an unparsed declaration would silently go missing.

Conversion is negotiated: against a service too old to report the `convert`
capability, these calls raise `MissingCapabilityError` naming the upgrade rather
than failing on an unimplemented method.

### Latency

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

#### Real-time analytics

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
  the service cache and costs ~0.5 ms, but passing `model.hash` to `eval`,
  `instantiate` and `execute_*` skips even that. The cache holds 100 models
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

### Generated typed classes

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
inst = pysysml.instantiate("Demo::Vehicle", model_hash=model.hash)

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

#### Keeping a generated module honest

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

#### SysML → Python mapping

| SysML | Python |
| --- | --- |
| `Real`, `Rational` | `float` |
| `Integer`, `Natural` | `int` |
| `Boolean` | `bool` |
| `String` | `str` |
| usage typed by a definition that reduces to a library scalar (`attribute def Celsius :> Real`) | that scalar (`float`) |
| usage typed by any other definition in the model | that definition's generated class |
| multiplicity `1`, `1..1`, or undeclared | `X` |
| multiplicity `0..1` | `X \| None` |
| `*`, `0..*`, `n..m` with upper > 1 | `list[X]` |
| `Complex`, `Number` | `object`, with a comment naming the type |
| a type resolved outside the model (e.g. a library type) | `object`, with a comment naming its FQN |
| an unresolved or absent type | `object`, with a comment naming what was written |
| `specializes` a definition in the model | Python base class |

The fallback is always `object` and always says why in the property's docstring;
no feature is given a type the model does not support, and `Any` is never used to
dodge one.

#### Known limitations

- **Quantities.** A value with a measurement unit (`attribute mass = 1500.0 [kg]`)
  is typed `object` and its docstring names the unit. The wire format has no
  magnitude-and-unit value, so the slot itself is reported as unsupported at
  runtime — this is a service limitation, not a codegen one.
- **Behavioral and connector usages.** Only structural usages (attribute, part,
  item, occurrence, individual, port, enum) become properties. Action, state,
  calc, constraint, requirement, connection, flow, interface, allocation and case
  usages are not instance slots and are skipped.
- **`subsets` and `redefines`.** Reported by the service and available on
  `Symbol.specializations`, but only `specializes` becomes a Python base class. A
  redefinition that narrows a feature's type is emitted with its own declared
  type, which Python does not check against the base property.
- **Multiple inheritance.** Emitted in declaration order; a SysML hierarchy whose
  Python equivalent has no consistent MRO produces a module that fails to import.
- **Generics and enumerations.** No generic parameters, and an `enumDef` becomes a
  plain class rather than a Python `Enum`.
- **Name collisions.** Two definitions with the same simple name both get
  path-qualified class names (`A_Thing`, `B_Thing`). A feature named like a member
  `TypedObject` provides (`instance`, `from_instance`, `sysml_id`) gets a trailing
  underscore (`instance_`); the SysML slot name it reads is unchanged.

`pysysml.connect(host, port, auto_start=True)` returns a `Connection` when you
want to manage the service yourself; the module-level functions share a lazily
created singleton connection instead. The service is reference-counted across
processes, so the last client to exit shuts it down.

## Development

```bash
pytest python/tests/                    # unit tests
pytest -m integration python/tests/     # needs a running sysml-grpc

# Regenerate the committed golden generated file (needs a running sysml-grpc)
python -m pysysml.generate internal/repl/testdata/vehicle_package.sysml \
    -o python/tests/golden/vehicle_types.py

# Regenerate protobuf bindings (from the repository root)
pip install grpcio-tools
make python-proto
```

## Modules

- `binary.py` — locates, downloads and checksum-verifies `sysml-grpc`
- `connection.py` — gRPC channel, service lifecycle, cross-process refcounting
- `model.py` — a parsed model: root symbol and diagnostics
- `symbol.py` — lazy symbol proxy, fetches children on demand
- `instance.py` — instantiated object and its slots
- `conversion.py` — a written model, its formats and extension inference
- `typefacts.py` — a symbol's static type, multiplicity and supertypes
- `typed.py` — base class and slot decoders the generated classes are built on
- `generate.py` — emits typed classes from a parsed model
- `diagnostic.py` — one diagnostic with its source location
- `proto/` — generated message classes and stubs
