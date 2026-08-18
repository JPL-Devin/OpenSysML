# pysysml

Python client for Systemica: parse, inspect and execute SysML v2 models over the
`sysml-grpc` service.

```bash
pip install pysysml             # from PyPI, once the first release is published
pip install -e python/          # or from a checkout, at the repository root
```

```python
import pysysml

model = pysysml.load("model.sysml", strict=True)   # raises on error diagnostics
print(model.eval("1 + 2 * 3"))                     # 7

print(model.eval("mass", subject="Demo::sedan"))   # 1200.0 — that object, not the default
                                                   # requires the service's evaluate_subject
                                                   # capability, rather than trusting a
                                                   # service that would ignore the subject

vehicle = model["Vehicle"]                         # by short name or FQN
vehicle.attributes()                               # own and inherited, with resolved facts
inst = model.instantiate("Demo::Vehicle")
inst.mass                                          # 1500.0 [kg] — a Quantity

model.verify_satisfaction()                        # every assert satisfy … by …
model.save("model.ttl")                            # RDF Turtle (experimental)
```

Every call goes through the `sysml-grpc` service, which `pysysml` starts for you from
`~/.pysysml/bin/sysml-grpc`; the guide below says how to put it there.

## Service ownership

`pysysml` never stops a service it did not start.

- A connection that finds a healthy service already listening uses it and takes
  no ownership of it: nothing is recorded, and closing the connection leaves the
  service running. Whoever started it decides when it stops.
- A service `pysysml` starts is recorded in
  `~/.pysysml/sysml-grpc-<port>.pid` (`$PYSYSML_STATE_DIR` overrides the
  directory) as the service's pid and process start time, plus the pid and start
  time of the process that started it. Only that process stops it, and only when
  the last connection holding it is closed or the interpreter exits.
- The start times are what authenticate the record: a pid is re-checked against
  the start time written for it, so a pid the operating system has since reused
  is treated as a stale record — cleaned up, never signalled. A command line
  that merely looks like `sysml-grpc` is not identity and is never acted on.
- A service that crashes leaves a record whose process is gone; the next
  connection detects that, removes it and starts a service of its own.

## Pinned release digests

A download is verified against `PINNED_SHA256` in `pysysml/binary.py`, which
pins the SHA-256 of every asset of a release. The `.sha256` served beside a
binary comes from whoever served the binary, so it detects corruption but not a
republished release; a pinned digest is independent of that origin. A download
with no pin fails with a message naming the version, rather than falling back to
the served checksum — `$PYSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for
any repository) accepts same-origin trust explicitly for what it names, with a
warning.

At release time, after the service binaries are published and final:

```bash
export GITHUB_TOKEN=...            # the release API rate-limits unauthenticated calls
python scripts/pin_release_checksums.py --version v0.0.9 --write
git commit -am 'chore(python): pin release digests for v0.0.9'
```

The script downloads every `sysml-grpc-*` asset of that release, hashes what it
downloaded, refuses the release if a `.sha256` sidecar disagrees with the asset
it describes, and rewrites the table in place. `--check` re-hashes the assets of
every pinned release and fails on any disagreement, catching a release
republished with another binary. A pysysml release therefore pins the service
releases published before it; asking for a newer one needs a newer pysysml (or
the explicit opt-in above), and leaves an already-downloaded binary serving
rather than refusing to start — only a digest that *contradicts* a pin is
treated as tampering and refuses to fall back.

## Version

`pysysml/_version.py` is the only declaration: the packaging metadata reads it,
`pysysml.__version__` reports the installed distribution's version, and
`scripts/check_version.py` fails a release whose tag names another version. The
version tests therefore require the tree under test to be the installed
distribution — `pip install -e python/`. A wheel of another version installed
beside the source tree makes them fail with that remedy: the artifact is what is
stale, not the declaration.

## Generated typed classes

`python -m pysysml.generate model.sysml -o model_types.py` emits one class per
SysML *definition*, and the generated hierarchy follows the model's
generalization edges: `specializes`, `subsets` and `redefines` all become base
classes, because Python has a single notion of inheritance. What tells the two
apart is the members, not the bases:

- a **redefinition** reuses the redefined feature's name, so its property
  overrides the base class's property of that name, and takes over the type and
  multiplicity it does not restate (`attribute :>> mass = 2.0;` stays `float`,
  and a redefined `0..*` feature stays a `list[...]`);
- a **subset** under a new name adds a property beside the base class's one, and
  likewise inherits the type and multiplicity it leaves out.

With **multiple supertypes**, bases are emitted in declaration order, a target
named twice appearing once, and Python resolves members left to right by its
usual MRO. A base another declared base already specializes is left implicit —
`Hybrid :> Vehicle, Electric` where `Electric :> Vehicle` emits
`class Hybrid(Electric)`, which Python can linearize and which keeps both
relationships and `Electric`'s properties. Where no order linearizes at all
(two bases specializing a shared pair in opposite orders), rather than emit a
module that fails to import, the generator keeps the bases it can and records
what it left out as a comment on the class, naming the edge:

```python
class Both(One):
    # specializes Demo::Two, left out: Python cannot linearize it with the bases above
```

A base outside the generated model is reported the same way. Both are the model's
hierarchy being wider than Python's, not facts being discarded — the service
reports every edge, and `Symbol.specializations` still carries them all.

**Limitation, unchanged:** only structural usages (`attribute`, `part`, `item`,
`occurrence`, `individual`, `port`, `enum`) become properties. Behavioral and
connector usages — `action`, `state`, `calc`, `constraint`, `requirement`,
`connection`, `flow`, `interface`, `allocation`, `case` — are not instance feature values,
so a generated class has no member for them; reach them through
`model["Demo::Vehicle"]`, `verify_constraint` and `verify_satisfaction`.

## Names that shadow builtins

Neither builtin name is a live part of the API any more: the module-level
evaluation function is `pysysml.evaluate`, and the execution error is
`pysysml.ExecutionError`. `pysysml.eval` and `pysysml.errors.RuntimeError`
remain as deprecated aliases that warn on use, out of their modules' `__all__`,
so a star-import binds neither.

```python
import pysysml

pysysml.evaluate("1 + 2", file_path="model.sysml")   # pysysml.eval warns
model.eval("mass", subject="Demo::sedan")            # a method shadows nothing
```

```python
from pysysml import eval          # shadows the builtin in this module — don't
```

Guidance for this package and for code around it:

- **Call `pysysml.evaluate`.** `pysysml.eval` still works and returns the same
  result, warning `DeprecationWarning`; it goes away in 1.0.0.
- Import the package, not its names, for anything named like a builtin.
- Catch `pysysml.ExecutionError` (or its base `pysysml.PySysMLError`), never
  `pysysml.errors.RuntimeError`, which warns and is due for removal.
- Do not name a new public function or exception after a builtin.

0.2.0 therefore publishes `evaluate` as the name to write, with both builtin
names deprecated rather than removed, so code written against 0.1.x keeps
running until 1.0.0.

## Running the tests

```bash
make build                                    # builds bin/sysml-grpc
pip install -e python/ && pip install pytest pytest-mock
python -m pytest python/tests/ -q             # service-backed tests skip
```

Tests that need a service skip when none answers on `localhost:50051` and no
binary is available to spawn one. Where a service *is* provided — as in CI —
export `PYSYSML_REQUIRE_SERVICE=1`, and its absence fails instead of skipping.

## Documentation

- Using the client:
  [docs/guide/09-python.md](https://github.com/Open-MBEE/OpenSysML/blob/main/docs/guide/09-python.md)
  — installing the service binary, loading a model, instances, verification, conversion
  and queries
- The API surface, generated typed classes, latency and the module map:
  [docs/reference/python-api.md](https://github.com/Open-MBEE/OpenSysML/blob/main/docs/reference/python-api.md)
- Installing from source and running the tests:
  [INSTALL.md](https://github.com/Open-MBEE/OpenSysML/blob/main/python/INSTALL.md)
