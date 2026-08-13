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

vehicle = model.root.find("Demo::Vehicle")
instance = pysysml.instantiate("Demo::Vehicle", file_path="model.sysml")
print(instance.slots)
print(pysysml.eval("1 + 2 * 3", file_path="model.sysml"))
```

`pysysml.connect(host, port, auto_start=True)` returns a `Connection` when you
want to manage the service yourself; the module-level functions share a lazily
created singleton connection instead. The service is reference-counted across
processes, so the last client to exit shuts it down.

## Development

```bash
pytest python/tests/                    # unit tests
pytest -m integration python/tests/     # needs a running sysml-grpc

# Regenerate protobuf bindings (from the repository root)
pip install grpcio-tools
python -m grpc_tools.protoc -Iapi --python_out=python/pysysml/proto \
  --grpc_python_out=python/pysysml/proto api/sysml.proto
```

## Modules

- `binary.py` — locates, downloads and checksum-verifies `sysml-grpc`
- `connection.py` — gRPC channel, service lifecycle, cross-process refcounting
- `model.py` — a parsed model: root symbol and diagnostics
- `symbol.py` — lazy symbol proxy, fetches children on demand
- `instance.py` — instantiated object and its slots
- `diagnostic.py` — one diagnostic with its source location
- `proto/` — generated message classes and stubs
