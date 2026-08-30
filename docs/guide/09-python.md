# 9. From Python

`opensysml` is the Python client. Every call is made through the `sysml-grpc` service, which the
client starts and stops automatically, so a script can parse, inspect, execute and convert a model
without invoking `sysml` as a subprocess.

The complete API surface, the generated typed classes and the measured latency are documented in
[reference/python-api.md](../reference/python-api.md).

## Installation

```bash
pip install opensysml             # from PyPI
pip install -e clients/python/          # or from a checkout, at the repository root
```

The dependencies (`grpcio`, `protobuf>=7.35.1`, `filelock`, `psutil`) are installed alongside it.
Those projects publish wheels for CPython 3.10 and later only, which is the range declared by
`requires-python`.

## Getting the service binary

Every call is made through `sysml-grpc`. The client starts an instance of the service and expects
to find the binary at `~/.opensysml/bin/sysml-grpc`. There are three ways to provide it:

```bash
# 1. Download the release build (verified against the digest pinned in opensysml)
python -c "from opensysml.binary import download_binary; download_binary('latest')"

# 2. Have opensysml.connect() download it on first use
export OPENSYSML_GRPC_VERSION=latest      # or a tag like v0.0.5

# 3. Build from source
make build-grpc && mkdir -p ~/.opensysml/bin && cp bin/sysml-grpc ~/.opensysml/bin/
```

If none of these is done, `connect()` raises `ConnectionError` rather than downloading anything
unrequested. `OPENSYSML_GITHUB_REPO` overrides the repository releases are fetched from (default
`Open-MBEE/OpenSysML`).

A download records its release tag, repository and digest beside the binary
(`~/.opensysml/bin/sysml-grpc.json`), so a cache left by an earlier release, or by another
repository publishing the same tag, is replaced rather than served to a client requesting a newer
one. Without this check an older build would answer and the call would fail as a
`MissingCapabilityError` naming a capability the requested release provides. The check is
triggered by requesting a release, so when `OPENSYSML_GRPC_VERSION` is unset a manually installed
binary (option 3) is left untouched. If the requested release cannot be downloaded, because no
asset exists for the platform or no network is available, the cached binary continues to serve and
the warning reports this rather than the connection failing.

A download is verified against the SHA-256 digest that `opensysml` pins for that release, not
against the `.sha256` served beside the binary. The sidecar file originates from whoever served
the binary, so it detects corruption but not a republished release. A release for which this
`opensysml` pins no digest is refused, with the version named, and a working cached binary is
retained rather than the served checksum being trusted. Setting
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for any repository, which is not
required for a fork's releases) explicitly accepts same-origin trust for the repository named,
with a warning. The binary is cached in `~/.opensysml/bin`. A service started by the client is a
private child of the interpreter and retains no state on disk, so `OPENSYSML_STATE_DIR`, which
previously relocated the service records, no longer has any effect.

Releases up to v0.0.4 contain the `sysml` and `sysml-lsp` archives only. `sysml-grpc` binaries are
published from the following release onward; until then, build the service from source
(option 3).

## A first script

```python
import opensysml

model = opensysml.load("model.sysml")
for d in model.diagnostics:
    print(d)

print(model.eval("1 + 2 * 3"))
```

Evaluation is performed on the model, as is every other operation, so a script never has to carry
the model hash back to the connection. `model.eval(expr, context_symbol_id=…)` resolves the
expression's names in that element's scope.

## Requiring a usable model

A model containing syntax errors still parses to a `Model`, because the service reports what it
could read together with diagnostics. A script that does not inspect those diagnostics therefore
queries a model that is missing declarations. To require a valid model instead:

```python
model = opensysml.load("model.sysml")
model.ok                       # False when any diagnostic is an error
model.errors                   # only the error-severity diagnostics
model.raise_for_errors()       # raises ModelError, or returns the model

model = opensysml.load("model.sysml", strict=True)   # raises instead of returning
```

`strict=True` is available on `opensysml.load`, `Connection.load` and
`Connection.load_from_content`. The `ModelError` it raises carries the errors as
`.diagnostics` and the model itself as `.model`, so a caller that wants to report
them does not have to load twice:

```python
try:
    model = opensysml.load("model.sysml", strict=True)
except opensysml.ModelError as exc:
    for d in exc.diagnostics:
        print(d)
    partial = exc.model            # what the service did parse
```

`strict_conformance=True`, available on the same three calls, addresses a different question:
whether the source is conforming SysML v2. OpenSysML's own notation, such as `defer` and the
pseudostates, is then reported as an error rather than a warning
([3. Strict conformance](03-command-line.md#strict-conformance)). The two settings are
independent: `strict` determines whether errors raise, and `strict_conformance` determines what
constitutes an error.

```python
model = opensysml.load("model.sysml", strict_conformance=True)
model.ok                       # False if the model uses OpenSysML notation
```

A service that predates the field raises `MissingCapabilityError` rather than silently answering
the default question. Support is advertised as `strict_conformance` in
`Connection.server_info().capabilities`.

## Inspecting symbols

`Model.find` takes a **short** name and searches the symbol tree, returning `None` when no such
symbol exists. `model["Vehicle"]` is the raising counterpart: it names the missing symbol, whereas
chaining from `find`'s `None` would fail one call later with `AttributeError: 'NoneType' object
has no attribute 'attributes'`.

```python
vehicle = model["Vehicle"]        # SymbolNotFoundError if absent; also a KeyError
vehicle.attributes()              # [Symbol(id='Demo::Vehicle::mass', kind='attributeUsage')]
vehicle.parts()                   # [Symbol(id='Demo::Vehicle::engine', kind='partUsage')]
vehicle.get_attr("mass")          # Symbol, or None if there is no such attribute

model.find("Nope")                # None — for asking whether a symbol exists
"Vehicle" in model                # True
model["Vehcile"]                  # SymbolNotFoundError: ... did you mean 'Vehicle'?
```

The subscript accepts a short name or a fully-qualified name. `model.get("Demo::Vehicle")` looks a
symbol up by fully-qualified name only and returns `None` for a name it does not find.

A symbol also carries its static type facts, as resolved by the service:

```python
engine = model.get("Demo::Vehicle::engine")
engine.type_facts        # TypeFacts(declared='Engine', resolved_id='Demo::Engine', ...)
engine.multiplicity      # Multiplicity(lower='0', upper='1'), or None if undeclared
engine.specializations   # [Specialization(kind='typing', target_id='Demo::Engine', ...)]
```

## Instances

Feature values are returned as Python values, and a feature value holding an object is returned as
a nested `Instance`:

```python
inst = model.instantiate("Demo::Vehicle")

inst.mass                 # 1500.0
inst["mass"]              # 1500.0
inst.engine               # Instance(id=2, type='Demo::Engine', features=1)
inst.engine.power         # 300.0
inst.features             # {'mass': 1500.0, 'engine': Instance(...)}
inst.get("missing", 0)    # 0
```

Integers, reals, booleans, strings and sequences map to `int`, `float`, `bool`, `str` and `list`.
Unknown names raise `AttributeError` for attribute access or `KeyError` for item access, so
`hasattr`, `copy` and `pickle` behave as expected.

A feature value holding nothing, such as a valueless feature of a value type declared
`attribute d : Real;`, reads as `opensysml.UNSET`, which `%features` and `-instantiate` render as
`<unset>`. It is falsy and is not `None`, which continues to represent the model's `null`:

```python
inst.d is opensysml.UNSET   # True
inst.d is None            # False
```

The service expands the object graph to depth 8 and stops at a type already present on the path,
so a part containing its own kind terminates. A child that was not expanded is returned as its
bare integer id rather than as an `Instance`.

The raw protobuf remains accessible: `get_feature(name)` returns the `FeatureValue` message, and
`raw_features` is the complete map.

```python
inst.get_feature("mass").materialized         # True
inst.get_feature("engine").value.instance_id  # 2
```

A feature value the service could not evaluate, such as a cyclic derived attribute, is never
reported as `None`. Attribute and item access raise `FeatureValueError`, while `features` carries
the `FeatureValueError` as that entry's value so the remainder of the instance stays inspectable.

```python
cyclic.a               # raises FeatureValueError: feature value 'a': ... cyclic feature value dependency
cyclic.features["a"]   # FeatureValueError(...)
```

`FeatureValueError` is not an `AttributeError`, so `hasattr` on such a feature propagates it
rather than returning `False`. Use `features` to inspect an instance whose feature values may have
failed to evaluate.

`eval` returns a single value, so a result the wire format cannot represent raises
`UnsupportedValueError` rather than being reported per entry.

Instances are capability-negotiated in the same way as conversion: a service that does not report
the `feature_values` capability, which applies to every release published before 0.1.0, raises
`MissingCapabilityError` naming the required upgrade rather than returning an object whose values
all appear to be missing.

Actions and state machines run on the model too:

```python
model.execute_action("Demo::addFive", inputs={"result": 10})   # {'result': 15}
model.execute_state("Demo::Machine", events=["go"])            # {'states_visited': [...], ...}
```

`execute_action` and `execute_state` apply the same policy to their result maps:
a value the wire format cannot represent is reported as an
`UnsupportedValueError` in that entry, leaving the other entries intact.

Every call concerning a loaded model is a `Model` method. The module-level
`opensysml.instantiate`, `opensysml.evaluate` and `opensysml.convert` remain available for
instantiating directly from a file (`opensysml.instantiate("Demo::Vehicle",
file_path="model.sysml")`) or against a hash obtained elsewhere (`model_hash=…`).

## Verifying constraints, requirements, satisfaction and calculations

The checks the REPL performs with `%constraint`, `%requirement`, `%satisfy` and `%calc` are also
available as RPCs, so whether a model satisfies its requirements can be determined from a script.
These calls use the same runtime evaluation the REPL drives rather than a second implementation.

```python
model = opensysml.load("lander.sysml", strict=True)

for verdict in model.verify_satisfaction():        # every assert satisfy … by …
    print(verdict)
# ✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)
# ✗ satisfy touchdown by fastLander fails (on Landing::fastLander ID: 2): condition
#   evaluated to false: lander.verticalSpeed <= maxVerticalSpeed

model.satisfied()                                  # False — one assertion fails
model.verify_satisfaction("Landing::analysisContext")   # only what that element asserts
```

Constraints and requirements are named directly, optionally with a subject to instantiate, so that
the verdict concerns that object's values rather than declared defaults:

```python
model.verify_constraint("Demo::Vehicle::massOK", subject="Demo::sedan")
model.verify_requirement("Demo::Vehicle::lightEnough", subject="Demo::sedan")
```

A `Verdict` is truthy when the condition holds, and reports the reason when it does not:

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

**A false verdict is a result, not an exception.** A condition that evaluated to false is the
answer to the question asked, so it is returned. Only a failure to evaluate at all, such as an
unbound feature or an exhausted step budget, constitutes a malfunction, and such a failure is not
reported as a failing verdict. It appears as `verdict.error` with `verdict.evaluated` set to
`False`, and `verdict.raise_for_error()` converts it into an `ExecutionError` where a script must
not interpret it as a failed requirement:

```python
for verdict in model.verify_satisfaction():
    verdict.raise_for_error()      # nothing raised for a verdict of false
    if not verdict:
        print(verdict.explain())
```

A request that cannot be answered at all, such as one naming an unknown symbol or a subject that
cannot be instantiated, raises `ExecutionError` from the call itself rather than returning a
verdict. Narrowing to an element that states no satisfaction assertion is not such a request: it
returns no verdicts, and `satisfied()` is then vacuously `True`.

**Naming an element of the wrong kind is an invalid request, not a verdict.** Requesting whether a
part def holds as a constraint raises `WrongKindError`, a subclass of `ExecutionError`, from
`verify_constraint`, `verify_requirement`, `verify_satisfaction` and `calc`, exactly as naming a
nonexistent element does. A caller reading `.holds` is therefore never told that the model does
not hold when the actual problem is that a part def was named:

```python
model.verify_constraint("Demo::Wheel")
# opensysml.errors.WrongKindError: not a constraint: Demo::Wheel is a part def,
# not a constraint definition or usage
```

The kind is read from a typed `failure_reason` reported by the service, never from the message
text.

`verify_satisfaction` evaluates many assertions in one call and reports a single object graph for
all of them, so `verdict.instances` holds every object that the call built. Select the object a
verdict concerns using its `instance_id`:

```python
subject = next(i for i in verdict.instances if i.id == verdict.instance_id)
```

Calculations are invoked with positional arguments. A calc *usage* named without arguments is
evaluated from its own members and reports every output feature it computes (SysML 7.17):

```python
model.calc("Demo::add", arguments=[2.5, 4.0]).value    # 6.5
model.calc("Demo::c").outputs                          # {'a': 6, 'b': 10}
```

Verification is capability-negotiated in the same way as conversion: against a service that does
not report the `verification` capability, these calls raise `MissingCapabilityError` naming the
required upgrade rather than failing on an unimplemented method.

## Errors

Every failure a caller can act on is an `OpenSysMLError`. The service's gRPC status codes are
translated at the client boundary, so a script never needs to `import grpc` and switch on status
codes. The original `grpc.RpcError` remains accessible as `__cause__` for its debug string.

```
OpenSysMLError
├── ConnectionError            service unreachable or would not start (UNAVAILABLE)
│   ├── StaleServiceError      another release is already listening on that address
│   └── ChecksumMismatchError  a download contradicts the digest pinned for it
│       └── UnpinnedReleaseError  this opensysml pins no digest for that release
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

`ServiceError.code` is the underlying `grpc.StatusCode`. A status this client does not recognize
is still delivered as a `ServiceError`, so nothing escapes the hierarchy.

The two most common failures are both `NOT_FOUND` on the wire but require different remedies, and
are distinguished from what the service reports:

```python
opensysml.load("/tmp/nope.sysml")     # ModelFileNotFoundError: file not found: …
model.to_turtle()                   # ModelNotFoundError if the model was evicted
```

`ExecutionError` inherits from the built-in `RuntimeError`, so `except RuntimeError:` catches it,
which a traceback reading `opensysml.errors.RuntimeError` previously implied but did not deliver.
The former name remains as a deprecated alias of `ExecutionError` — it is the same class, so
existing `except opensysml.errors.RuntimeError` clauses continue to work — and emits a
`DeprecationWarning` on attribute access. Inheriting from the built-in was chosen over renaming
alone because it also corrects existing code that never caught the old class, and the alias is
excluded from `__all__` so a star-import no longer shadows the built-in.

### Names that no longer shadow a built-in

Both names that the package previously bound over a Python built-in were renamed before 0.2.0 was
published, so no deprecation cycle is required:

| Old name | Use instead |
| --- | --- |
| `opensysml.eval` | `opensysml.evaluate` |
| `opensysml.RuntimeError`, `opensysml.errors.RuntimeError` | `opensysml.ExecutionError` |

Each former name still resolves to the same object — `opensysml.eval` *is* `opensysml.evaluate`
and `opensysml.RuntimeError` *is* `ExecutionError`, so existing snippets and `except` clauses
continue to work — and emits a `DeprecationWarning` on access. Neither appears in `__all__`, so
`from opensysml import *` no longer binds over `eval` or `RuntimeError`. The `Model.eval` and
`Connection.eval` *methods* retain their name, because an attribute of an object shadows nothing.

## Writing a model back out

A loaded model can be written back to SysML notation or RDF Turtle. The service performs the
conversion using the same code as `sysml -convert`, so the client contributes no second
implementation of the mapping.

The Turtle direction is [experimental](../reference/rdf-mapping.md#status-experimental): it
carries model structure and the behavior stated by model bodies, refuses any construct it cannot
write back, and its vocabulary may change without a compatibility path. Any conversion using it
issues an `ExperimentalFeatureWarning` and sets `Conversion.experimental`. The notation direction
is stable and issues no warning.

```python
model = opensysml.load("model.sysml")

model.to_sysml()                 # Conversion: SysML notation
model.to_turtle()                # Conversion: RDF Turtle
model.save("model.ttl")          # writes Turtle; format taken from the extension
model.save("out.sysml")

opensysml.convert("ttl", file_path="model.sysml")            # without loading first
opensysml.convert("sysml", content=turtle, from_format="ttl")  # Turtle back to notation
```

A `Conversion` consists of the output text, the formats converted between, and whether the mapping
used is `experimental`, with `experimental_notice` explaining why. `str()` and `len()` return the
text, and `write(path)` saves it. Formats are named `sysml`, `kerml`, `text`, `ttl`, `turtle` or
`rdf`. A file path's format is inferred from its extension; inline `content` has no extension and
therefore requires `from_format`.

A `Model` writes out the source the service parsed, identified by `model.hash`, so a file edited
between `load` and `save` does not affect what is written: the model saved is the model that was
inspected. `convert(file_path=…)` is the alternative, reading the file in its current state. The
parsed source is held in the service's bounded cache, so a model evicted since it was loaded
raises `ModelNotFoundError` rather than writing different content; load it again.

Each direction preserves the following:

- **Notation → notation** re-emits the model from its source, so comments and layout are
  preserved. It is source-preserving rather than a general AST printer, so a model the client
  built element by element cannot be printed this way.
- **Notation → Turtle → notation** returns an equivalent model rather than identical bytes.
  Comments do not survive the graph, because RDF provides no place to record them. See
  [the RDF mapping](../reference/rdf-mapping.md) for the content a graph must carry to support the
  return trip.
- Syntax errors normally fail the conversion and are returned as a `ConversionError` carrying
  `diagnostics`. `tolerate_syntax_errors=True` writes notation regardless and reports the errors
  as `Conversion.diagnostics`. It applies to the notation → notation direction only, because every
  other direction builds a graph in which an unparsed declaration would be silently omitted.

Conversion is capability-negotiated: against a service that does not report the `convert`
capability, these calls raise `MissingCapabilityError` naming the required upgrade rather than
failing on an unimplemented method. For a service that does not report the RDF mapping's status,
the status is derived from the formats it reports, so an RDF conversion warns in either case.
Suppress the warning with `warnings.simplefilter("ignore",
opensysml.ExperimentalFeatureWarning)`, which no stable feature uses.

## Changing a model and writing it back

A loaded model can be edited in place — setting the value of a feature, renaming a declaration —
and written back with its comments, blank lines and indentation intact. The edit is described
rather than typed out: the client sends operations naming elements by the same ids that a read
reports, and the service applies them to the source it parsed.

```python
model = opensysml.load("spacecraft.sysml")

edit = model.edit()
edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
edit.rename("Demo::sc::margin", "massMargin")

result = edit.apply()             # re-parsed and validated service-side
result.save("spacecraft.sysml")   # every byte outside an edited span unchanged
```

`model.edit()` returns an `Editor`. `set_value(target, value)` replaces an existing `= <expr>` on
a feature, or adds one before the terminating `;` when the feature has none; `value` is SysML
notation for a single expression, such as `"1050.0[SI::kg]"`, `'"flight-2"'`, `"true"` or
`"unitMass * count"`. `rename(target, new_name)` rewrites a declaration's name token and the
references to it. A target is a symbol id (its fully-qualified name, as reported by `Symbol.id`)
or a `Symbol` itself, so an element located by a read can be edited by passing it back. Both calls
return the editor, so operations can be chained, and `len(edit)` counts them.

The same editor authors declarations:

```python
model = opensysml.loads("package Demo {}", strict=True)
result = model.edit().add_part_def("", "Vehicle").apply()
```

`add_member(owner, kind, name, type=None, multiplicity=None, value=None, specializes=None)`
accepts notation strings for the declaration. Typed `add_*` helpers cover the common SysML and
KerML kinds, and `delete(target, cascade=False)` removes declarations transactionally.

`apply()` sends the operations in a single call and returns an `EditResult`, which *is* a
`Conversion`: `str(result)` is the edited notation, and `result.save(path)` and
`result.write(path)` write it. `result.applied` lists the changes as
`AppliedEdit(operation_index, target, offset, length, old_text, new_text)` in source order, where
`length == 0` marks a value added to a feature that previously had none.

The edit mechanism provides the following properties:

- **Spans rather than text search.** The value expression and the name token are located through
  the parsed model's own spans, so a comment or string that happens to contain the same text is
  never modified.
- **Bytes are spliced and nothing is reformatted.** Edits are applied right-to-left by offset in a
  single pass, and every byte outside an edited span is byte-identical to the source the service
  parsed. No content is re-indented or re-wrapped.
- **The result is read back before it is returned.** The edited source is re-lexed, re-parsed and
  re-analysed. An edit that would introduce a syntax or name-resolution error is refused with the
  diagnostics explaining why, and no content is returned, so the service never emits a file its
  parser cannot read. Errors the model already contained are not attributed to the edit; only
  errors the edit introduces cause a refusal.

Every refusal is a typed error, never a silent no-op:

| Situation | Error |
| --- | --- |
| No operation was added to the editor | `NoEditsError` |
| No such element, an ambiguous name, or an element that cannot carry a value or a name | `EditTargetError` |
| A new value that does not parse as one expression, or a new name that is not an identifier or already means something where the element is declared | `InvalidEditError` |
| A rename that would capture or shadow another name, at the declaration or at any reference | `InvalidEditError` |
| Two operations that would edit overlapping bytes | `OverlappingEditsError` |
| The edited model does not read back cleanly — a value naming something that does not resolve, say | `EditResultError` |

All of these are subclasses of `EditError`, which carries `failure` (the refusal kind),
`diagnostics`, and `referring_elements` for a refused rename or a refused non-cascade delete. An
`EditResultError`'s diagnostics are spanned against the edited text. `referring_elements` names
each namespace from which a reference is made, indicating where to look rather than which
expression is at fault.

```python
try:
    model.edit().rename("Demo::SC::margin", "label").apply()
except opensysml.InvalidEditError as refused:
    print(refused.failure)   # 'EDIT_FAILURE_INVALID_NAME'
```

A rename rewrites the declaration's name token and every reference to it made by the model's
source — the matching segment of a qualified name, an alias target and an import — so the model
retains its original meaning.

The following limitations are intentional:

- **A rename is refused where it would change what a name means.** A new name that already has a
  meaning where the element is declared — as a sibling of that name, or one reached through an
  enclosing namespace, an import or a supertype — or that already has a meaning at one of the
  references being rewritten, is refused. Such a rename would be either ambiguous or shadow an
  existing declaration, and in either case unrelated expressions would begin resolving to the
  renamed element. References made from another file are not rewritten, because an edit sees only
  the source of the model it was given.
- **A model cannot be constructed from Python.** Declarations are added to and deleted from a
  model that is already loaded; there is no way to author one from nothing, and no object facade.
  A declaration is described by the notation arguments of an `add_*` call rather than by a mutable
  Python object.
- An editor is applied **once**. It describes an edit of the model it was created from, so
  applying it twice raises `RuntimeError`. Load the saved file and edit that instead.
- The parsed source is held in the service's bounded cache, so a model evicted since it was loaded
  raises `ModelNotFoundError` rather than editing different content; load it again.

Editing is capability-negotiated in the same way as conversion: a service that does not report the
`apply_edits` capability raises `MissingCapabilityError` naming the required upgrade before any
call is made.

## Querying a model using the standard query model

`model.query(...)` runs the query defined by the **SysML v2 API & Services** standard, using
`scope`, `select` and `where`, so a payload written for that API works verbatim. This is the form
sent by the API Cookbook notebooks and by MATLAB System Composer's `executeQuery`:

```python
model = opensysml.load("model.sysml")

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

Each result is a `QueryElement` consisting of `id` (the element's qualified name), `type` (its
metamodel type, for example `PartUsage`) and `properties`, the selected properties the element
has. A property an element does not have is absent rather than empty. An unnamed element, such as
a `doc` note or an anonymous usage, is not returned at all, because it has no qualified name by
which it could be identified or scoped. `as_dict()` returns the result using the standard's JSON
names. `scope` accepts qualified names or the standard's `{"@id": …}` references and considers
each named element together with everything nested inside it; an empty scope covers the whole
loaded model.

A payload the standard does not describe, such as one using an unknown operator or a constraint
without a property, raises `QueryError` before anything is sent. A property the service does not
provide raises `InvalidRequestError` naming the properties that exist rather than returning no
results, and an evicted model raises `ModelNotFoundError`. As with every other call, gRPC status
codes stop at the client boundary. The query is capability-negotiated in the same way as
conversion: a service that does not report the `query` capability raises `MissingCapabilityError`.

The standard's query model provides **no graph traversal and no transitive closure**: "everything
under this part" is expressed as a `scope`, while "everything specializing this definition" cannot
be expressed at all. It is an interoperability surface rather than OpenSysML's expressive query
facility; [the API reference](../reference/api.md) states precisely what is supported.

---

Next: [10. Troubleshooting](10-troubleshooting.md).
