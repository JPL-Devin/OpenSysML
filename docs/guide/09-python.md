# 9. From Python

`pysysml` is the Python client. Every call goes through the `sysml-grpc` service, which the
client starts and stops for you, so a script parses, inspects, executes and converts a model
without shelling out to `sysml`.

The full API surface, the generated typed classes and the measured latency are
[reference/python-api.md](../reference/python-api.md).

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
# 1. Download the release build (verified against the digest pinned in pysysml)
python -c "from pysysml.binary import download_binary; download_binary('latest')"

# 2. Let pysysml.connect() download it on first use
export PYSYSML_GRPC_VERSION=latest      # or a tag like v0.0.5

# 3. Build from source
make build-grpc && mkdir -p ~/.pysysml/bin && cp bin/sysml-grpc ~/.pysysml/bin/
```

Without one of those, `connect()` raises `ConnectionError` rather than
downloading anything unasked. `PYSYSML_GITHUB_REPO` overrides the repository
releases are fetched from (default `Open-MBEE/OpenSysML`).

A download records its release tag, repository and digest beside the binary
(`~/.pysysml/bin/sysml-grpc.json`), so a cache left by an earlier release — or by
another repository publishing the same tag — is replaced instead of being served
to a client asking for a newer one; otherwise an old build answers and the call
fails as a `MissingCapabilityError` naming a capability the requested release does
have. Asking for a release is what
triggers that check, so with `PYSYSML_GRPC_VERSION` unset a binary you put there
yourself (option 3) is left alone. If the release asked for cannot be downloaded
(no asset for your platform, no network), the cached binary keeps serving and the
warning says so, rather than the connection failing.

A download is checked against the SHA-256 `pysysml` pins for that release, not
the `.sha256` served beside the binary: the sidecar comes from whoever served the
binary, so it catches corruption but not a republished release. A release this
`pysysml` pins no digest for is refused, naming the version, and keeps a working
cached binary rather than trusting the served checksum; `export
PYSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for any repository, which a
fork's releases do not need) accepts same-origin trust explicitly for the repository
it names, with a warning. `PYSYSML_STATE_DIR` moves the state directory
(`~/.pysysml`) holding the binary cache and the service records.

The published releases up to v0.0.4 carry the `sysml`/`sysml-lsp` archives only;
`sysml-grpc` binaries are published from the next release onward, so until then
build it from source (option 3).

## A first script

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

## Loading a model that has to be usable

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

## Inspecting symbols

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

## Instances

Feature values come back as Python values, and a feature value holding an object
comes back as a nested `Instance`:

```python
inst = model.instantiate("Demo::Vehicle")

inst.mass                 # 1500.0
inst["mass"]              # 1500.0
inst.engine               # Instance(id=2, type='Demo::Engine', features=1)
inst.engine.power         # 300.0
inst.features             # {'mass': 1500.0, 'engine': Instance(...)}
inst.get("missing", 0)    # 0
```

Integers, reals, booleans, strings and sequences map to `int`, `float`, `bool`,
`str` and `list`. Unknown names raise `AttributeError` (attribute access) or
`KeyError` (item access), so `hasattr`, `copy` and `pickle` behave.

A feature value holding nothing — a valueless feature of a value type, `attribute d : Real;` —
reads as `pysysml.UNSET`, the same thing `%features` and `-instantiate` spell `<unset>`. It
is falsy and is not `None`, which stays the model's `null`:

```python
inst.d is pysysml.UNSET   # True
inst.d is None            # False
```

The service expands the object graph to depth 8 and stops at a type already on
the path, so a part containing its own kind terminates; a child it did not
expand comes back as its bare integer id rather than an `Instance`.

The raw protobuf stays reachable: `get_feature(name)` returns the `FeatureValue`
message, and `raw_features` is the whole map.

```python
inst.get_feature("mass").materialized         # True
inst.get_feature("engine").value.instance_id  # 2
```

A feature value the service could not evaluate — a cyclic derived attribute, say — is
never reported as `None`. Attribute and item access raise `FeatureValueError`, while
`features` carries the `FeatureValueError` as that entry's value so the rest of the
instance stays inspectable.

```python
cyclic.a               # raises FeatureValueError: feature value 'a': ... cyclic feature value dependency
cyclic.features["a"]   # FeatureValueError(...)
```

`FeatureValueError` is not an `AttributeError`, so `hasattr` on such a feature propagates it
rather than returning `False`; use `features` to inspect an instance whose feature values may
have failed.

`slots`, `raw_slots`, `get_slot` and `SlotError` remain as deprecated spellings of
`features`, `raw_features`, `get_feature` and `FeatureValueError`.

`eval` returns a single value, so a result the wire format cannot represent raises
`UnsupportedValueError` rather than being reported per entry.

Actions and state machines run on the model too:

```python
model.execute_action("Demo::addFive", inputs={"result": 10})   # {'result': 15}
model.execute_state("Demo::Machine", events=["go"])            # {'states_visited': [...], ...}
```

`execute_action` and `execute_state` apply the same policy to their result maps:
a value the wire format cannot represent is reported as an
`UnsupportedValueError` in that entry, leaving the other entries intact.

Every call about a loaded model is a `Model` method, and the module-level
`pysysml.instantiate`/`evaluate`/`convert` remain for instantiating straight out of a
file (`pysysml.instantiate("Demo::Vehicle", file_path="model.sysml")`) or against
a hash obtained elsewhere (`model_hash=…`).

## Verifying constraints, requirements, satisfaction and calculations

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

## Errors

Every failure a caller can act on is a `PySysMLError`. The service's gRPC status
codes are translated at the client boundary, so a script never has to `import
grpc` and switch on status codes; the original `grpc.RpcError` stays reachable as
`__cause__` for the debug string.

```
PySysMLError
├── ConnectionError            service unreachable or would not start (UNAVAILABLE)
│   ├── StaleServiceError      another release is already listening on that address
│   └── ChecksumMismatchError  a download contradicts the digest pinned for it
│       └── UnpinnedReleaseError  this pysysml pins no digest for that release
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
├── FeatureValueError          a feature value could not be evaluated
├── ConversionError            the model could not be written in that format
├── UnsupportedValueError      a value the wire format cannot represent
├── TypeMismatchError          a feature value contradicts its generated view
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

### Names that no longer shadow a built-in

Both names the package used to bind over a Python built-in were renamed before
0.2.0 published, so no deprecation cycle is owed:

| Old name | Use instead |
| --- | --- |
| `pysysml.eval` | `pysysml.evaluate` |
| `pysysml.RuntimeError`, `pysysml.errors.RuntimeError` | `pysysml.ExecutionError` |

Each old name still resolves to the same object — `pysysml.eval` *is*
`pysysml.evaluate` and `pysysml.RuntimeError` *is* `ExecutionError`, so existing
snippets and `except` clauses keep working — and emits a `DeprecationWarning` on
access. Neither is in `__all__`, so `from pysysml import *` no longer binds over
`eval` or `RuntimeError`. The `Model.eval`/`Connection.eval` *methods* keep their
name: an attribute of an object shadows nothing.

## Writing a model back out

A loaded model can be written back to SysML notation or RDF Turtle. The service
does the conversion with the same code `sysml -convert` uses, so the client adds
no second implementation of the mapping.

The Turtle direction is [experimental](../reference/rdf-mapping.md#status-experimental):
it carries model structure and the behavior its bodies state, refuses what it
cannot write back, and its vocabulary may change without a compatibility path.
Any conversion through it warns with `ExperimentalFeatureWarning` and sets
`Conversion.experimental`; notation is stable and warns about nothing.

```python
model = pysysml.load("model.sysml")

model.to_sysml()                 # Conversion: SysML notation
model.to_turtle()                # Conversion: RDF Turtle
model.save("model.ttl")          # writes Turtle; format taken from the extension
model.save("out.sysml")

pysysml.convert("ttl", file_path="model.sysml")            # without loading first
pysysml.convert("sysml", content=turtle, from_format="ttl")  # Turtle back to notation
```

A `Conversion` is the output text plus the formats it went between, and whether
the mapping it used is `experimental` (with `experimental_notice` saying why);
`str()` and `len()` give the text, and `write(path)` saves it. Formats are named `sysml`,
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
  See [the RDF mapping](../reference/rdf-mapping.md) for what a graph must carry
  for the round trip back.
- Syntax errors normally fail the conversion and come back as a
  `ConversionError` carrying `diagnostics`. `tolerate_syntax_errors=True` writes
  notation anyway and reports the errors as `Conversion.diagnostics`; it applies
  to notation → notation only, because every other direction builds a graph
  where an unparsed declaration would silently go missing.

Conversion is negotiated: against a service too old to report the `convert`
capability, these calls raise `MissingCapabilityError` naming the upgrade rather
than failing on an unimplemented method. A service too old to report the RDF
mapping's status is read from the formats it reports instead, so an RDF
conversion warns either way. Silence the warning with
`warnings.simplefilter("ignore", pysysml.ExperimentalFeatureWarning)`, which no
stable feature uses.

## Querying a model the standard's way

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
expressive query story — [the API reference](../reference/api.md) states exactly what is
supported.

---

Next: [10. Troubleshooting](10-troubleshooting.md).
