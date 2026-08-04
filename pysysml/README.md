# pysysml

Python client library for Systemica SysML v2 parser.

## Installation

**Prerequisites:** Python 3.8+ with protobuf >= 7.35.1 and grpcio

```bash
# Install required dependencies
pip install "protobuf>=7.35.1" grpcio

# Verify protobuf bindings work
python -c "from pysysml.proto import sysml_pb2, sysml_pb2_grpc; print('pysysml protobuf bindings ready')"
```

**Note:** Generated protobuf code requires protobuf 7.35.1+, and gRPC stubs require grpcio. Install the package with `pip install -e .` from the repo root.

## Usage

```python
# Protobuf bindings are ready (client API coming in Phase 2)
from pysysml.proto import sysml_pb2, sysml_pb2_grpc

# Example: Create a parse request
request = sysml_pb2.ParseFileRequest(file_path="model.sysml")
print(f"Request ready: {request.file_path}")
```

Full client API (Connection, Model, Symbol) will be added in Phase 2.

## Development

```bash
# Regenerate protobuf bindings (from repo root, requires grpcio-tools)
pip install grpcio-tools
python -m grpc_tools.protoc -Iapi --python_out=pysysml/proto --grpc_python_out=pysysml/proto api/sysml.proto
```

Tests coming in Phase 2.

## Architecture

**Current (Phase 1):**
- `proto/sysml_pb2.py`: Generated protobuf message classes
- `proto/sysml_pb2_grpc.py`: Generated gRPC client stubs

**Coming in Phase 2:**
- **Connection**: Manages gRPC channel to sysml-grpc service
- **Model**: Wraps parsed model, provides root symbol and diagnostics
- **Symbol**: Lazy proxy for symbol navigation (fetches children on demand)
- **Diagnostic**: Parse error/warning messages with source locations

Phase 2 requires manually starting `sysml-grpc` service. Phase 3 will add auto-start.
