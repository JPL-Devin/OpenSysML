# Installing pysysml

## From PyPI

```bash
pip install pysysml
```

Published from CircleCI on a `pysysml-v*` tag; the first such release creates the
project on PyPI, so until it is cut this installs nothing and the source install
below is the only route. See
[docs/project/releasing.md](../docs/project/releasing.md#releasing-pysysml-to-pypi).

## From source

From the repository root:

```bash
# Install in development mode (editable)
pip install -e python/

# Or install with dev dependencies
pip install -e "python/[dev]"
```

## Running tests

From the repository root:

```bash
# Run all tests
pytest python/tests/

# Run with verbose output
pytest -v python/tests/

# Run specific test file
pytest python/tests/test_connection.py

# Run integration tests (requires sysml-grpc service)
pytest -m integration python/tests/
```

## Package structure

```
python/
├── pysysml/          # Package source
│   ├── *.py          # Core modules (connection, model, symbol, etc.)
│   └── proto/        # Generated protobuf stubs
├── tests/            # Test suite
├── setup.py          # Package metadata
├── pyproject.toml    # Build configuration
├── README.md         # Package documentation
└── INSTALL.md        # This file
```
