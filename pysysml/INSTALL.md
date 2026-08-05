# Installing pysysml

## From source

From the repository root:

```bash
# Install in development mode (editable)
pip install -e pysysml/

# Or install with dev dependencies
pip install -e "pysysml/[dev]"
```

## Running tests

From the repository root:

```bash
# Run all tests
pytest pysysml/tests/

# Run with verbose output
pytest -v pysysml/tests/

# Run specific test file
pytest pysysml/tests/test_connection.py

# Run integration tests (requires sysml-grpc service)
pytest -m integration pysysml/tests/
```

## Package structure

```
pysysml/
├── *.py              # Core modules (connection, model, symbol, etc.)
├── proto/            # Generated protobuf stubs
├── tests/            # Test suite
├── setup.py          # Package metadata
├── pyproject.toml    # Build configuration
└── README.md         # Package documentation
```
