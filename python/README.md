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

vehicle = model["Vehicle"]                         # by short name or FQN
inst = model.instantiate("Demo::Vehicle")
inst.mass                                          # 1500.0

model.verify_satisfaction()                        # every assert satisfy … by …
model.save("model.ttl")                            # RDF Turtle
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
the served checksum — `$PYSYSML_ALLOW_UNPINNED_DOWNLOAD=1` accepts same-origin
trust explicitly, with a warning.

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
  [docs/guide/09-python.md](https://github.com/Open-MBEE/Systemica/blob/main/docs/guide/09-python.md)
  — installing the service binary, loading a model, instances, verification, conversion
  and queries
- The API surface, generated typed classes, latency and the module map:
  [docs/reference/python-api.md](https://github.com/Open-MBEE/Systemica/blob/main/docs/reference/python-api.md)
- Installing from source and running the tests:
  [INSTALL.md](https://github.com/Open-MBEE/Systemica/blob/main/python/INSTALL.md)
