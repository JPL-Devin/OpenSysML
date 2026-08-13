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
  path-qualified class names (`A_Thing`, `B_Thing`).

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
- `typefacts.py` — a symbol's static type, multiplicity and supertypes
- `typed.py` — base class and slot decoders the generated classes are built on
- `generate.py` — emits typed classes from a parsed model
- `diagnostic.py` — one diagnostic with its source location
- `proto/` — generated message classes and stubs
