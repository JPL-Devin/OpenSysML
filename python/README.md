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

print(model.eval("1 + 2 * 3"))
```

Evaluation is on the model, like every other operation, so a script never carries
the model hash back to the connection. `model.eval(expr, context_symbol_id=…)`
resolves the expression's names in that element's scope.

### Loading a model that has to be usable

A model with syntax errors still parses to a `Model` — the service reports what
it could read plus diagnostics — so a script that does not look at them queries a
model that is missing declarations. Ask for a valid one instead:

```python
model = pysysml.load("model.sysml")
model.ok                       # False when any diagnostic is an error
model.errors                   # just the error-severity diagnostics
model.raise_for_errors()       # raises ModelError, or returns the model

model = pysysml.load("model.sysml", strict=True)   # raises instead of returning
```

`strict=True` is available on `pysysml.load`, `Connection.load` and
`Connection.load_from_content`. The `ModelError` it raises carries the errors as
`.diagnostics` and the model itself as `.model`, so a caller that wants to report
them does not have to load twice:

```python
try:
    model = pysysml.load("model.sysml", strict=True)
except pysysml.ModelError as exc:
    for d in exc.diagnostics:
        print(d)
    partial = exc.model            # what the service did parse
```

### Inspecting symbols

`Model.find` takes a **short** name and searches the symbol tree, returning
`None` when there is no such symbol. `model["Vehicle"]` is the raising
counterpart: it names the symbol that is missing, where chaining off `find`'s
`None` would fail one call later as `AttributeError: 'NoneType' object has no
attribute 'attributes'`.

```python
vehicle = model["Vehicle"]        # SymbolNotFoundError if absent; also a KeyError
vehicle.attributes()              # [Symbol(id='Demo::Vehicle::mass', kind='attributeUsage')]
vehicle.parts()                   # [Symbol(id='Demo::Vehicle::engine', kind='partUsage')]
vehicle.get_attr("mass")          # Symbol, or None if there is no such attribute

model.find("Nope")                # None — for asking whether a symbol exists
"Vehicle" in model                # True
model["Vehcile"]                  # SymbolNotFoundError: ... did you mean 'Vehicle'?
```

The subscript takes a short name or an FQN. `model.get("Demo::Vehicle")` looks a
symbol up by fully-qualified name only, and returns `None` for a name it does not
find.

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

### Verification: constraints, requirements, satisfaction, calc

The questions the REPL answers with `%constraint`, `%requirement`, `%satisfy` and
`%calc` are RPCs too, so "does this model satisfy its requirements?" is
scriptable. They run the same runtime evaluation the REPL drives, not a second
implementation of it.

```python
model = pysysml.load("lander.sysml", strict=True)

for verdict in model.verify_satisfaction():        # every assert satisfy … by …
    print(verdict)
# ✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)
# ✗ satisfy touchdown by fastLander fails (on Landing::fastLander ID: 2): condition
#   evaluated to false: lander.verticalSpeed <= maxVerticalSpeed

model.satisfied()                                  # False — one assertion fails
model.verify_satisfaction("Landing::analysisContext")   # only what that element asserts
```

Constraints and requirements are asked about by name, optionally against a
subject to instantiate, so the verdict is about that object's values rather than
declared defaults:

```python
model.verify_constraint("Demo::Vehicle::massOK", subject="Demo::sedan")
model.verify_requirement("Demo::Vehicle::lightEnough", subject="Demo::sedan")
```

A `Verdict` is truthy when the condition holds, and carries why when it does not:

```python
verdict = model.verify_requirement("Demo::Vehicle::lightEnough", subject="Demo::truck")

bool(verdict)          # False
verdict.holds          # False
verdict.condition      # 'mass < 2000.0' — the condition that evaluated to false
verdict.element        # 'Demo::Vehicle::lightEnough', or the assertion as written
verdict.kind           # 'constraint', 'requirement' or 'satisfy'
verdict.instance_id    # the object the verdict is about, 0 for declared defaults
verdict.instances      # the objects the call built, as Instances
verdict.diagnostics    # diagnostics the service reported for the run
print(verdict.explain())
```

**A false verdict is an answer, not an exception.** A condition that evaluated to
false is what was asked, so it is returned; only a failure to evaluate at all (an
unbound feature, an exhausted step budget) is a malfunction. That failure does
not masquerade as a failing verdict either — it is `verdict.error`, with
`verdict.evaluated` False, and `verdict.raise_for_error()` turns it into an
`ExecutionError` where a script must not read it as "the requirement fails":

```python
for verdict in model.verify_satisfaction():
    verdict.raise_for_error()      # nothing raised for a verdict of false
    if not verdict:
        print(verdict.explain())
```

A request that cannot be answered at all — an unknown symbol, a subject that
cannot be instantiated — raises `ExecutionError` from the call itself rather than
returning a verdict. Narrowing to an element that states no satisfaction
assertion is not such a request: it answers with no verdicts, and `satisfied()`
is then vacuously `True`.

**Naming the wrong kind of element is a wrong request, not a verdict.** Asking
whether a part def holds as a constraint raises `WrongKindError` (an
`ExecutionError`) from `verify_constraint`, `verify_requirement`,
`verify_satisfaction` and `calc`, as naming an element that does not exist
already does — so a caller reading `.holds` is never told "your model does not
hold" when the answer is "you named a part def":

```python
model.verify_constraint("Demo::Wheel")
# pysysml.errors.WrongKindError: not a constraint: Demo::Wheel is a part def,
# not a constraint definition or usage
```

The kind is read from a typed `failure_reason` the service reports, never from
the message text.

`verify_satisfaction` answers many assertions in one call and reports one object
graph for them all, so `verdict.instances` holds every object that call built;
select the one a verdict is about with its `instance_id`:

```python
subject = next(i for i in verdict.instances if i.id == verdict.instance_id)
```

Calculations are invoked with positional arguments, and a calc *usage* named with
no arguments is evaluated from its own members, reporting every output feature it
computes (SysML 7.17):

```python
model.calc("Demo::add", arguments=[2.5, 4.0]).value    # 6.5
model.calc("Demo::c").outputs                          # {'a': 6, 'b': 10}
```

Verification is negotiated like conversion: against a service too old to report
the `verification` capability these calls raise `MissingCapabilityError` naming
the upgrade, rather than failing on an unimplemented method.

### Errors

Every failure a caller can act on is a `PySysMLError`. The service's gRPC status
codes are translated at the client boundary, so a script never has to `import
grpc` and switch on status codes; the original `grpc.RpcError` stays reachable as
`__cause__` for the debug string.

```
PySysMLError
├── ConnectionError            service unreachable or would not start (UNAVAILABLE)
├── ServiceError               any other status the service failed a call with
│   ├── ModelNotFoundError     the model hash is no longer in the service cache
│   ├── ModelFileNotFoundError the service could not read the path (also FileNotFoundError)
│   ├── InvalidRequestError    request rejected as malformed (also ValueError)
│   ├── ServiceTimeoutError    deadline exceeded or cancelled (also TimeoutError)
│   └── UnsupportedOperationError  the service does not implement the call
├── ExecutionError             eval/instantiate/execute/verify failed (also RuntimeError)
│   └── WrongKindError         the call named an element of another kind than it asks about
├── ModelError                 strict load of a model with error diagnostics
├── SymbolNotFoundError        model["Nope"] (also KeyError)
├── SlotError                  a slot could not be evaluated
├── ConversionError            the model could not be written in that format
├── UnsupportedValueError      a value the wire format cannot represent
├── TypeMismatchError          a slot's value contradicts its generated view
├── InstanceTypeError          a typed view was given an instance of another type
└── MissingCapabilityError     the connected service does not report the capability
```

`ServiceError.code` is the `grpc.StatusCode` behind it. A status this client has
never seen still arrives as a `ServiceError`, so nothing escapes the hierarchy.

The two most common failures are both `NOT_FOUND` on the wire but have different
fixes, and are told apart from what the service reports:

```python
pysysml.load("/tmp/nope.sysml")     # ModelFileNotFoundError: file not found: …
model.to_turtle()                   # ModelNotFoundError if the model was evicted
```

`ExecutionError` inherits from the built-in `RuntimeError`, so `except
RuntimeError:` catches it — which is what a traceback reading
`pysysml.errors.RuntimeError` used to promise and not deliver. That old name
remains as a deprecated alias of `ExecutionError` (same class, so existing
`except pysysml.errors.RuntimeError` keeps working) and emits a
`DeprecationWarning` on attribute access. Inheriting from the built-in was chosen
over renaming alone because it fixes existing code that never caught the old
class, and the alias is excluded from `__all__` so a star-import no longer
shadows the built-in.

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

A `Model` writes out the source the service parsed, named by `model.hash`, so a
file edited between `load` and `save` does not change what is written — the model
saved is the model that was inspected. `convert(file_path=…)` is the other
choice, reading the file as it stands now. The parsed source lives in the
service's bounded cache, so a model evicted since it was loaded raises
`ModelNotFoundError` instead of writing something else; load it again.

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

### Querying a model the standard's way

`model.query(...)` runs the query the **SysML v2 API & Services** standard
defines — `scope` / `select` / `where` — so a payload written for that API works
verbatim, which is what the API Cookbook notebooks and MATLAB System Composer's
`executeQuery` send:

```python
model = pysysml.load("model.sysml")

model.query({"@type": "Query", "where": {
    "@type": "PrimitiveConstraint",
    "operator": "=", "property": "@type", "value": ["PartUsage"]}})
# [Demo::vehicle (PartUsage), Demo::vehicle::wheels (PartUsage)]

# The same query in keyword form, narrowed and projected
model.query(
    scope=["Demo::vehicle"],
    select=["name", "qualifiedName"],
    where={"operator": "=", "property": "@type", "value": ["PartUsage"]},
)
```

Each answer is a `QueryElement`: `id` (the element's qualified name), `type` (its
metamodel type, e.g. `PartUsage`) and `properties`, the selected properties it
has — a property an element does not have is absent rather than empty. An unnamed
element (a `doc` note, an anonymous usage) is not answered at all: it has no
qualified name to be identified or scoped by.
`as_dict()` gives it back in the standard's JSON names. `scope` takes qualified
names or the standard's `{"@id": …}` references, and considers each named element
and everything nested inside it; an empty scope is the whole loaded model.

A payload the standard does not describe — an unknown operator, a constraint with
no property — raises `QueryError` before anything is sent. A property the service
does not have raises `InvalidRequestError` naming the properties that exist,
rather than answering with nothing, and an evicted model raises
`ModelNotFoundError`: like every other call, gRPC status codes stop at the client
boundary. Like conversion, the query is
negotiated: a service too old to report the `query` capability raises
`MissingCapabilityError`.

The standard's query model has **no graph traversal and no transitive closure**:
"everything under this part" is a `scope`, and "everything specializing this
definition" is not expressible at all. It is an interop surface, not Systemica's
expressive query story — [docs/API.md](../docs/API.md) states exactly what is
supported.

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
created singleton connection instead. A `host:port` address written as the host
is read as one — `connect("localhost:50123")` reaches port 50123 — and a port
named twice with two values raises `ValueError` naming the disagreement rather
than timing out against an address nobody asked for. The helpers taking
`host`/`port` (`load`, `eval`, `convert`, `instantiate`) read it the same way. The service is reference-counted across
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
- `query.py` — the standard's Query payload, translated and its answers
- `verdict.py` — a verification's answer and what a calculation computed
- `errors.py` — the exception hierarchy and the gRPC status translation
- `capabilities.py` — what the connected service reports it supports
- `typefacts.py` — a symbol's static type, multiplicity and supertypes
- `typed.py` — base class and slot decoders the generated classes are built on
- `generate.py` — emits typed classes from a parsed model
- `diagnostic.py` — one diagnostic with its source location
- `proto/` — generated message classes and stubs
