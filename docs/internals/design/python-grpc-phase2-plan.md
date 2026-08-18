# Python Client Library (Phase 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create Python package (opensysml) that connects to sysml-grpc service and provides Pythonic API for parsing SysML models.

**Architecture:** Pure Python client using grpc/protobuf. Package structure: opensysml/ with proto/, connection, model, symbol, diagnostic modules. Manual service connection only (Phase 3 will add auto-start).

**Tech Stack:** Python 3.8+, grpcio, grpcio-tools, protobuf, pytest

---

## File Structure

```
opensysml/                          # Python package root
  __init__.py                     # Package exports
  connection.py                   # Connection class - manual service connection
  model.py                        # Model class - wraps ParseFileResponse
  symbol.py                       # Symbol class - lazy proxy for symbols
  diagnostic.py                   # Diagnostic class - wraps protobuf diagnostic
  proto/                          # Generated protobuf code
    __init__.py
    sysml_pb2.py                  # Generated from sysml.proto
    sysml_pb2_grpc.py             # Generated gRPC stubs
tests/                            # Unit tests
  test_connection.py              # Connection tests with mocked gRPC
  test_model.py                   # Model tests
  test_symbol.py                  # Symbol navigation tests
  test_diagnostic.py              # Diagnostic parsing tests
setup.py                          # Package metadata and dependencies
pyproject.toml                    # Modern Python packaging config
README.md                         # Python package documentation
```

---

## Task 1: Package Setup and Protobuf Generation

**Goal:** Create Python package structure and generate protobuf stubs from api/proto/sysml.proto

**Files:**
- Create: `opensysml/__init__.py`
- Create: `opensysml/proto/__init__.py`
- Create: `setup.py`
- Create: `pyproject.toml`
- Create: `README.md`

---

- [ ] **Step 1: Create opensysml package directory structure**

```bash
mkdir -p opensysml/proto
touch opensysml/__init__.py
touch opensysml/proto/__init__.py
```

Run: `ls -la opensysml/`
Expected: `__init__.py` and `proto/` directory exist

---

- [ ] **Step 2: Create setup.py**

```python
from setuptools import setup, find_packages

setup(
    name="opensysml",
    version="0.1.0",
    description="Python client library for OpenSysML SysML v2 parser",
    author="Open-MBEE",
    packages=find_packages(),
    python_requires=">=3.8",
    install_requires=[
        "grpcio>=1.50.0",
        "protobuf>=4.21.0",
    ],
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-mock>=3.10.0",
            "grpcio-tools>=1.50.0",
        ],
    },
)
```

Run: `cat setup.py`
Expected: File content matches above

---

- [ ] **Step 3: Create pyproject.toml**

```toml
[build-system]
requires = ["setuptools>=45", "wheel"]
build-backend = "setuptools.build_backend"

[project]
name = "opensysml"
version = "0.1.0"
description = "Python client library for OpenSysML SysML v2 parser"
readme = "README.md"
requires-python = ">=3.8"
dependencies = [
    "grpcio>=1.50.0",
    "protobuf>=4.21.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.0.0",
    "pytest-mock>=3.10.0",
    "grpcio-tools>=1.50.0",
]
```

Run: `cat pyproject.toml`
Expected: File content matches above

---

- [ ] **Step 4: Create README.md**

```markdown
# opensysml

Python client library for OpenSysML SysML v2 parser.

## Installation

```bash
pip install -e .
```

## Usage

```python
from opensysml import Connection

# Connect to manually-started sysml-grpc service
conn = Connection(port=50051)

# Load a SysML model
model = conn.load("path/to/model.sysml")

# Access root symbol
root = model.root
print(f"Root: {root.name} ({root.kind})")

# Find symbol by name
part = model.find("SPACECRAFT_WET")
print(f"Part: {part.name}")

# Navigate children
for child in part.children():
    print(f"  {child.name}: {child.kind}")

# Check diagnostics
for diag in model.diagnostics:
    print(f"{diag.severity}: {diag.message} at {diag.span}")
```

## Development

```bash
# Install with dev dependencies
pip install -e ".[dev]"

# Run tests
pytest tests/
```

## Architecture

- **Connection**: Manages gRPC channel to sysml-grpc service
- **Model**: Wraps parsed model, provides root symbol and diagnostics
- **Symbol**: Lazy proxy for symbol navigation (fetches children on demand)
- **Diagnostic**: Parse error/warning messages with source locations

Phase 2 requires manually starting `sysml-grpc` service. Phase 3 will add auto-start.
```

Run: `cat README.md`
Expected: File content matches above

---

- [ ] **Step 5: Generate Python protobuf stubs**

```bash
python -m grpc_tools.protoc \
  -I api/proto \
  --python_out=opensysml/proto \
  --grpc_python_out=opensysml/proto \
  api/proto/sysml.proto
```

Run: `ls -la opensysml/proto/`
Expected: `sysml_pb2.py` and `sysml_pb2_grpc.py` generated

---

- [ ] **Step 6: Verify protobuf imports work**

```bash
python -c "from opensysml.proto import sysml_pb2, sysml_pb2_grpc; print('OK')"
```

Expected: Output `OK` (no import errors)

---

- [ ] **Step 7: Create empty opensysml/__init__.py with version**

```python
"""opensysml - Python client library for OpenSysML SysML v2 parser."""

__version__ = "0.1.0"

# Public API (will be filled in later tasks)
__all__ = []
```

Run: `python -c "import opensysml; print(opensysml.__version__)"`
Expected: Output `0.1.0`

---

- [ ] **Step 8: Commit**

```bash
git add opensysml/ setup.py pyproject.toml README.md
git commit -m "feat(python): add package structure and protobuf stubs"
```

---

## Task 2: Diagnostic Class

**Goal:** Implement Diagnostic class wrapping protobuf diagnostic message with Python properties

**Files:**
- Create: `opensysml/diagnostic.py`
- Create: `tests/test_diagnostic.py`

---

- [ ] **Step 1: Write failing test for Diagnostic class**

```python
# tests/test_diagnostic.py
from opensysml.proto import sysml_pb2
from opensysml.diagnostic import Diagnostic


def test_diagnostic_properties():
    pb_diag = sysml_pb2.Diagnostic(
        severity="error",
        message="Undefined symbol 'Foo'",
        span=sysml_pb2.Span(
            file="test.sysml",
            start_line=10,
            start_column=5,
            end_line=10,
            end_column=8,
        ),
    )
    
    diag = Diagnostic(pb_diag)
    
    assert diag.severity == "error"
    assert diag.message == "Undefined symbol 'Foo'"
    assert diag.file == "test.sysml"
    assert diag.start_line == 10
    assert diag.start_column == 5
    assert diag.end_line == 10
    assert diag.end_column == 8


def test_diagnostic_str():
    pb_diag = sysml_pb2.Diagnostic(
        severity="warning",
        message="Unused import",
        span=sysml_pb2.Span(
            file="model.sysml",
            start_line=5,
            start_column=1,
            end_line=5,
            end_column=20,
        ),
    )
    
    diag = Diagnostic(pb_diag)
    result = str(diag)
    
    assert "model.sysml:5:1" in result
    assert "warning" in result.lower()
    assert "Unused import" in result


def test_diagnostic_span_property():
    pb_span = sysml_pb2.Span(
        file="test.sysml",
        start_line=1,
        start_column=2,
        end_line=3,
        end_column=4,
    )
    pb_diag = sysml_pb2.Diagnostic(
        severity="error",
        message="Test",
        span=pb_span,
    )
    
    diag = Diagnostic(pb_diag)
    
    # span property returns the protobuf Span object
    assert diag.span == pb_span
    assert diag.span.file == "test.sysml"
```

---

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest tests/test_diagnostic.py -v`
Expected: FAIL with "ModuleNotFoundError: No module named 'opensysml.diagnostic'"

---

- [ ] **Step 3: Implement Diagnostic class**

```python
# opensysml/diagnostic.py
"""Diagnostic class wrapping protobuf diagnostic message."""


class Diagnostic:
    """Represents a parse diagnostic (error or warning).
    
    Attributes:
        severity (str): Diagnostic severity (e.g., "error", "warning")
        message (str): Diagnostic message
        file (str): Source file name
        start_line (int): Starting line number (1-based)
        start_column (int): Starting column number (1-based)
        end_line (int): Ending line number (1-based)
        end_column (int): Ending column number (1-based)
        span: Protobuf Span object
    """
    
    def __init__(self, pb_diagnostic):
        """Initialize Diagnostic from protobuf Diagnostic message.
        
        Args:
            pb_diagnostic: sysml_pb2.Diagnostic protobuf message
        """
        self._pb = pb_diagnostic
    
    @property
    def severity(self):
        """Get diagnostic severity."""
        return self._pb.severity
    
    @property
    def message(self):
        """Get diagnostic message."""
        return self._pb.message
    
    @property
    def span(self):
        """Get protobuf Span object."""
        return self._pb.span
    
    @property
    def file(self):
        """Get source file name."""
        return self._pb.span.file
    
    @property
    def start_line(self):
        """Get starting line number (1-based)."""
        return self._pb.span.start_line
    
    @property
    def start_column(self):
        """Get starting column number (1-based)."""
        return self._pb.span.start_column
    
    @property
    def end_line(self):
        """Get ending line number (1-based)."""
        return self._pb.span.end_line
    
    @property
    def end_column(self):
        """Get ending column number (1-based)."""
        return self._pb.span.end_column
    
    def __str__(self):
        """String representation: 'file:line:col: severity: message'."""
        return (
            f"{self.file}:{self.start_line}:{self.start_column}: "
            f"{self.severity}: {self.message}"
        )
    
    def __repr__(self):
        """Detailed representation."""
        return (
            f"Diagnostic(severity={self.severity!r}, "
            f"message={self.message!r}, "
            f"file={self.file!r}, "
            f"line={self.start_line})"
        )
```

---

- [ ] **Step 4: Run test to verify it passes**

Run: `pytest tests/test_diagnostic.py -v`
Expected: 3 tests PASS

---

- [ ] **Step 5: Commit**

```bash
git add opensysml/diagnostic.py tests/test_diagnostic.py
git commit -m "feat(python): implement Diagnostic class"
```

---

## Task 3: Symbol Class

**Goal:** Implement Symbol class as lazy proxy for symbol navigation with on-demand child loading

**Files:**
- Create: `opensysml/symbol.py`
- Create: `tests/test_symbol.py`

---

- [ ] **Step 1: Write failing test for Symbol basic properties**

```python
# tests/test_symbol.py
from opensysml.proto import sysml_pb2
from opensysml.symbol import Symbol


def test_symbol_properties():
    pb_sym = sysml_pb2.SymbolInfo(
        id="Vehicle::Engine",
        name="Engine",
        kind="PartDef",
        metadata={"visibility": "public", "abstract": "false"},
        child_ids=["Vehicle::Engine::combustionChamber"],
        attributes=[],
    )
    
    # Mock client (will be used for lazy loading)
    mock_client = None
    model_hash = "abc123"
    
    sym = Symbol(pb_sym, mock_client, model_hash)
    
    assert sym.id == "Vehicle::Engine"
    assert sym.name == "Engine"
    assert sym.kind == "PartDef"
    assert sym.metadata == {"visibility": "public", "abstract": "false"}


def test_symbol_str():
    pb_sym = sysml_pb2.SymbolInfo(
        id="MyPart",
        name="MyPart",
        kind="PartDef",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    sym = Symbol(pb_sym, None, "hash")
    result = str(sym)
    
    assert "MyPart" in result
    assert "PartDef" in result
```

---

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest tests/test_symbol.py::test_symbol_properties -v`
Expected: FAIL with "ModuleNotFoundError: No module named 'opensysml.symbol'"

---

- [ ] **Step 3: Implement Symbol class (basic properties only)**

```python
# opensysml/symbol.py
"""Symbol class for lazy symbol navigation."""


class Symbol:
    """Proxy for a SysML symbol with lazy child loading.
    
    Attributes:
        id (str): Fully-qualified symbol ID (e.g., "Vehicle::Engine")
        name (str): Symbol short name (e.g., "Engine")
        kind (str): Symbol kind (e.g., "PartDef", "AttributeUsage")
        metadata (dict): Symbol metadata (visibility, abstract, etc.)
    """
    
    def __init__(self, pb_symbol, client, model_hash):
        """Initialize Symbol from protobuf SymbolInfo.
        
        Args:
            pb_symbol: sysml_pb2.SymbolInfo protobuf message
            client: Client instance for lazy loading (or None)
            model_hash: Model hash for fetching children
        """
        self._pb = pb_symbol
        self._client = client
        self._model_hash = model_hash
        self._children_cache = None
    
    @property
    def id(self):
        """Get fully-qualified symbol ID."""
        return self._pb.id
    
    @property
    def name(self):
        """Get symbol short name."""
        return self._pb.name
    
    @property
    def kind(self):
        """Get symbol kind."""
        return self._pb.kind
    
    @property
    def metadata(self):
        """Get symbol metadata dict."""
        return dict(self._pb.metadata)
    
    def __str__(self):
        """String representation: 'name (kind)'."""
        return f"{self.name} ({self.kind})"
    
    def __repr__(self):
        """Detailed representation."""
        return f"Symbol(id={self.id!r}, kind={self.kind!r})"
```

---

- [ ] **Step 4: Run test to verify basic properties pass**

Run: `pytest tests/test_symbol.py::test_symbol_properties tests/test_symbol.py::test_symbol_str -v`
Expected: 2 tests PASS

---

- [ ] **Step 5: Write failing test for children() method**

```python
# Add to tests/test_symbol.py

from unittest.mock import Mock


def test_symbol_children_lazy_loading():
    # Parent symbol with two children
    pb_parent = sysml_pb2.SymbolInfo(
        id="Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["Vehicle::Engine", "Vehicle::Wheels"],
        attributes=[],
    )
    
    # Mock client that returns child symbols
    mock_client = Mock()
    
    # First child
    pb_child1 = sysml_pb2.SymbolInfo(
        id="Vehicle::Engine",
        name="Engine",
        kind="PartUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    # Second child
    pb_child2 = sysml_pb2.SymbolInfo(
        id="Vehicle::Wheels",
        name="Wheels",
        kind="PartUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    # Mock get_symbol to return children
    mock_client.get_symbol.side_effect = [pb_child1, pb_child2]
    
    model_hash = "abc123"
    parent = Symbol(pb_parent, mock_client, model_hash)
    
    # Call children() - should trigger lazy loading
    children = parent.children()
    
    assert len(children) == 2
    assert children[0].id == "Vehicle::Engine"
    assert children[0].name == "Engine"
    assert children[1].id == "Vehicle::Wheels"
    assert children[1].name == "Wheels"
    
    # Verify get_symbol called with correct IDs
    assert mock_client.get_symbol.call_count == 2
    mock_client.get_symbol.assert_any_call(model_hash, "Vehicle::Engine")
    mock_client.get_symbol.assert_any_call(model_hash, "Vehicle::Wheels")


def test_symbol_children_caching():
    pb_sym = sysml_pb2.SymbolInfo(
        id="Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["Vehicle::Engine"],
        attributes=[],
    )
    
    mock_client = Mock()
    pb_child = sysml_pb2.SymbolInfo(
        id="Vehicle::Engine",
        name="Engine",
        kind="PartUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    mock_client.get_symbol.return_value = pb_child
    
    model_hash = "hash"
    parent = Symbol(pb_sym, mock_client, model_hash)
    
    # Call children() twice
    children1 = parent.children()
    children2 = parent.children()
    
    # Should only call get_symbol once (cached)
    assert mock_client.get_symbol.call_count == 1
    assert children1 is children2  # Same list object


def test_symbol_children_empty():
    pb_sym = sysml_pb2.SymbolInfo(
        id="Leaf",
        name="Leaf",
        kind="AttributeUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    sym = Symbol(pb_sym, None, "hash")
    children = sym.children()
    
    assert children == []
```

---

- [ ] **Step 6: Run test to verify it fails**

Run: `pytest tests/test_symbol.py::test_symbol_children_lazy_loading -v`
Expected: FAIL with "AttributeError: 'Symbol' object has no attribute 'children'"

---

- [ ] **Step 7: Implement children() method**

```python
# Add to opensysml/symbol.py after metadata property

    def children(self):
        """Get list of child symbols (lazy loaded, cached).
        
        Returns:
            list[Symbol]: List of child Symbol objects
        """
        if self._children_cache is not None:
            return self._children_cache
        
        if not self._pb.child_ids:
            self._children_cache = []
            return self._children_cache
        
        if self._client is None:
            # No client available for lazy loading
            self._children_cache = []
            return self._children_cache
        
        # Fetch children via client
        children = []
        for child_id in self._pb.child_ids:
            pb_child = self._client.get_symbol(self._model_hash, child_id)
            if pb_child is not None:
                children.append(Symbol(pb_child, self._client, self._model_hash))
        
        self._children_cache = children
        return self._children_cache
```

---

- [ ] **Step 8: Run test to verify children() passes**

Run: `pytest tests/test_symbol.py::test_symbol_children_lazy_loading tests/test_symbol.py::test_symbol_children_caching tests/test_symbol.py::test_symbol_children_empty -v`
Expected: 3 tests PASS

---

- [ ] **Step 9: Write failing test for attributes() method**

```python
# Add to tests/test_symbol.py

def test_symbol_attributes_filtering():
    # Symbol with mixed children
    pb_parent = sysml_pb2.SymbolInfo(
        id="Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["Vehicle::mass", "Vehicle::Engine"],
        attributes=[],
    )
    
    mock_client = Mock()
    
    # Attribute child
    pb_attr = sysml_pb2.SymbolInfo(
        id="Vehicle::mass",
        name="mass",
        kind="AttributeUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    # Part child (not an attribute)
    pb_part = sysml_pb2.SymbolInfo(
        id="Vehicle::Engine",
        name="Engine",
        kind="PartUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    mock_client.get_symbol.side_effect = [pb_attr, pb_part]
    
    model_hash = "hash"
    parent = Symbol(pb_parent, mock_client, model_hash)
    
    # attributes() should filter for AttributeUsage/AttributeDef
    attrs = parent.attributes()
    
    assert len(attrs) == 1
    assert attrs[0].id == "Vehicle::mass"
    assert attrs[0].kind == "AttributeUsage"
```

---

- [ ] **Step 10: Run test to verify it fails**

Run: `pytest tests/test_symbol.py::test_symbol_attributes_filtering -v`
Expected: FAIL with "AttributeError: 'Symbol' object has no attribute 'attributes'"

---

- [ ] **Step 11: Implement attributes() method**

```python
# Add to opensysml/symbol.py after children() method

    def attributes(self):
        """Get list of attribute child symbols (filtered children).
        
        Returns only children with kind containing "Attribute".
        
        Returns:
            list[Symbol]: List of attribute Symbol objects
        """
        return [
            child for child in self.children()
            if "Attribute" in child.kind
        ]
```

---

- [ ] **Step 12: Run test to verify attributes() passes**

Run: `pytest tests/test_symbol.py::test_symbol_attributes_filtering -v`
Expected: PASS

---

- [ ] **Step 13: Run all symbol tests**

Run: `pytest tests/test_symbol.py -v`
Expected: All 6 tests PASS

---

- [ ] **Step 14: Commit**

```bash
git add opensysml/symbol.py tests/test_symbol.py
git commit -m "feat(python): implement Symbol class with lazy loading"
```

---

## Task 4: Model Class

**Goal:** Implement Model class wrapping ParseFileResponse with root symbol and diagnostics

**Files:**
- Create: `opensysml/model.py`
- Create: `tests/test_model.py`

---

- [ ] **Step 1: Write failing test for Model class**

```python
# tests/test_model.py
from unittest.mock import Mock
from opensysml.proto import sysml_pb2
from opensysml.model import Model


def test_model_properties():
    pb_root = sysml_pb2.SymbolInfo(
        id="MyModel",
        name="MyModel",
        kind="Package",
        metadata={},
        child_ids=["MyModel::Vehicle"],
        attributes=[],
    )
    
    pb_diag1 = sysml_pb2.Diagnostic(
        severity="error",
        message="Syntax error",
        span=sysml_pb2.Span(file="test.sysml", start_line=1, start_column=1, end_line=1, end_column=1),
    )
    
    pb_diag2 = sysml_pb2.Diagnostic(
        severity="warning",
        message="Unused symbol",
        span=sysml_pb2.Span(file="test.sysml", start_line=5, start_column=1, end_line=5, end_column=1),
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="abc123",
        root=pb_root,
        diagnostics=[pb_diag1, pb_diag2],
    )
    
    mock_client = Mock()
    model = Model(pb_response, mock_client)
    
    # Check hash
    assert model.hash == "abc123"
    
    # Check root is a Symbol
    assert model.root.id == "MyModel"
    assert model.root.name == "MyModel"
    assert model.root.kind == "Package"
    
    # Check diagnostics are Diagnostic objects
    assert len(model.diagnostics) == 2
    assert model.diagnostics[0].severity == "error"
    assert model.diagnostics[0].message == "Syntax error"
    assert model.diagnostics[1].severity == "warning"
    assert model.diagnostics[1].message == "Unused symbol"


def test_model_str():
    pb_root = sysml_pb2.SymbolInfo(
        id="TestModel",
        name="TestModel",
        kind="Package",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash123",
        root=pb_root,
        diagnostics=[],
    )
    
    model = Model(pb_response, None)
    result = str(model)
    
    assert "TestModel" in result
    assert "Package" in result
```

---

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest tests/test_model.py::test_model_properties -v`
Expected: FAIL with "ModuleNotFoundError: No module named 'opensysml.model'"

---

- [ ] **Step 3: Implement Model class (basic properties)**

```python
# opensysml/model.py
"""Model class wrapping parsed SysML model."""

from opensysml.symbol import Symbol
from opensysml.diagnostic import Diagnostic


class Model:
    """Represents a parsed SysML model.
    
    Attributes:
        hash (str): Model content hash (for cache lookups)
        root (Symbol): Root symbol of the model
        diagnostics (list[Diagnostic]): Parse diagnostics (errors/warnings)
    """
    
    def __init__(self, pb_response, client):
        """Initialize Model from protobuf ParseFileResponse.
        
        Args:
            pb_response: sysml_pb2.ParseFileResponse protobuf message
            client: Client instance for symbol navigation
        """
        self._pb = pb_response
        self._client = client
        self._hash = pb_response.model_hash
        self._root = Symbol(pb_response.root, client, self._hash)
        self._diagnostics = [
            Diagnostic(pb_diag) for pb_diag in pb_response.diagnostics
        ]
    
    @property
    def hash(self):
        """Get model content hash."""
        return self._hash
    
    @property
    def root(self):
        """Get root symbol."""
        return self._root
    
    @property
    def diagnostics(self):
        """Get list of diagnostics."""
        return self._diagnostics
    
    def __str__(self):
        """String representation: 'Model: name (kind)'."""
        return f"Model: {self.root.name} ({self.root.kind})"
    
    def __repr__(self):
        """Detailed representation."""
        diag_count = len(self.diagnostics)
        return (
            f"Model(hash={self.hash!r}, root={self.root.name!r}, "
            f"diagnostics={diag_count})"
        )
```

---

- [ ] **Step 4: Run test to verify basic properties pass**

Run: `pytest tests/test_model.py::test_model_properties tests/test_model.py::test_model_str -v`
Expected: 2 tests PASS

---

- [ ] **Step 5: Write failing test for find() method**

```python
# Add to tests/test_model.py

def test_model_find():
    # Model with nested symbols
    pb_root = sysml_pb2.SymbolInfo(
        id="MyModel",
        name="MyModel",
        kind="Package",
        metadata={},
        child_ids=["MyModel::Vehicle", "MyModel::Sensor"],
        attributes=[],
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash",
        root=pb_root,
        diagnostics=[],
    )
    
    mock_client = Mock()
    
    # Mock children for root
    pb_vehicle = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["MyModel::Vehicle::Engine"],
        attributes=[],
    )
    
    pb_sensor = sysml_pb2.SymbolInfo(
        id="MyModel::Sensor",
        name="Sensor",
        kind="PartDef",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    pb_engine = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle::Engine",
        name="Engine",
        kind="PartUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    # Mock get_symbol calls
    def mock_get_symbol(model_hash, symbol_id):
        mapping = {
            "MyModel::Vehicle": pb_vehicle,
            "MyModel::Sensor": pb_sensor,
            "MyModel::Vehicle::Engine": pb_engine,
        }
        return mapping.get(symbol_id)
    
    mock_client.get_symbol.side_effect = mock_get_symbol
    
    model = Model(pb_response, mock_client)
    
    # Find top-level symbol
    vehicle = model.find("Vehicle")
    assert vehicle is not None
    assert vehicle.id == "MyModel::Vehicle"
    assert vehicle.name == "Vehicle"
    
    # Find nested symbol
    engine = model.find("Engine")
    assert engine is not None
    assert engine.id == "MyModel::Vehicle::Engine"
    assert engine.name == "Engine"
    
    # Find non-existent symbol
    missing = model.find("NonExistent")
    assert missing is None


def test_model_find_short_circuit():
    # Model with one child
    pb_root = sysml_pb2.SymbolInfo(
        id="Root",
        name="Root",
        kind="Package",
        metadata={},
        child_ids=["Root::Target"],
        attributes=[],
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash",
        root=pb_root,
        diagnostics=[],
    )
    
    mock_client = Mock()
    
    pb_target = sysml_pb2.SymbolInfo(
        id="Root::Target",
        name="Target",
        kind="PartDef",
        metadata={},
        child_ids=["Root::Target::Nested"],
        attributes=[],
    )
    
    mock_client.get_symbol.return_value = pb_target
    
    model = Model(pb_response, mock_client)
    
    # Find should stop at first match (don't traverse into Target's children)
    target = model.find("Target")
    
    assert target is not None
    assert target.name == "Target"
    
    # Should have called get_symbol only once (for Target, not its children)
    # Note: children() is lazy, so get_symbol only called when explicitly requested
```

---

- [ ] **Step 6: Run test to verify it fails**

Run: `pytest tests/test_model.py::test_model_find -v`
Expected: FAIL with "AttributeError: 'Model' object has no attribute 'find'"

---

- [ ] **Step 7: Implement find() method**

```python
# Add to opensysml/model.py after diagnostics property

    def find(self, name):
        """Find symbol by short name (breadth-first search).
        
        Searches the symbol tree starting from root, returning the first
        symbol whose name matches. Returns None if not found.
        
        Args:
            name (str): Short name to search for (e.g., "Vehicle")
        
        Returns:
            Symbol or None: First matching symbol, or None if not found
        """
        # Check root first
        if self.root.name == name:
            return self.root
        
        # Breadth-first search
        queue = [self.root]
        while queue:
            current = queue.pop(0)
            
            # Check each child
            for child in current.children():
                if child.name == name:
                    return child
                queue.append(child)
        
        return None
```

---

- [ ] **Step 8: Run test to verify find() passes**

Run: `pytest tests/test_model.py::test_model_find tests/test_model.py::test_model_find_short_circuit -v`
Expected: 2 tests PASS

---

- [ ] **Step 9: Run all model tests**

Run: `pytest tests/test_model.py -v`
Expected: All 4 tests PASS

---

- [ ] **Step 10: Commit**

```bash
git add opensysml/model.py tests/test_model.py
git commit -m "feat(python): implement Model class with find() method"
```

---

## Task 5: Connection Class

**Goal:** Implement Connection class managing gRPC channel and providing high-level API

**Files:**
- Create: `opensysml/connection.py`
- Create: `tests/test_connection.py`

---

- [ ] **Step 1: Write failing test for Connection initialization**

```python
# tests/test_connection.py
import grpc
from unittest.mock import Mock, patch
from opensysml.connection import Connection


def test_connection_init():
    with patch('grpc.insecure_channel') as mock_channel:
        mock_stub = Mock()
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(port=50051)
            
            assert conn.port == 50051
            mock_channel.assert_called_once_with('localhost:50051')


def test_connection_custom_host():
    with patch('grpc.insecure_channel') as mock_channel:
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(host='example.com', port=9000)
            
            mock_channel.assert_called_once_with('example.com:9000')
```

---

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest tests/test_connection.py::test_connection_init -v`
Expected: FAIL with "ModuleNotFoundError: No module named 'opensysml.connection'"

---

- [ ] **Step 3: Implement Connection class (initialization only)**

```python
# opensysml/connection.py
"""Connection class for communicating with sysml-grpc service."""

import grpc
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class Connection:
    """Manages connection to sysml-grpc service.
    
    Phase 2: Manual connection only (service must be started externally).
    Phase 3 will add auto-start capabilities.
    
    Attributes:
        host (str): Service hostname
        port (int): Service port
    """
    
    def __init__(self, host='localhost', port=50051):
        """Initialize connection to sysml-grpc service.
        
        Args:
            host (str): Service hostname (default: 'localhost')
            port (int): Service port (default: 50051)
        """
        self.host = host
        self.port = port
        self._address = f"{host}:{port}"
        self._channel = grpc.insecure_channel(self._address)
        self._stub = sysml_pb2_grpc.SysMLServiceStub(self._channel)
    
    def close(self):
        """Close the gRPC channel."""
        if self._channel:
            self._channel.close()
    
    def __enter__(self):
        """Context manager entry."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()
```

---

- [ ] **Step 4: Run test to verify initialization passes**

Run: `pytest tests/test_connection.py::test_connection_init tests/test_connection.py::test_connection_custom_host -v`
Expected: 2 tests PASS

---

- [ ] **Step 5: Write failing test for load() method**

```python
# Add to tests/test_connection.py

from opensysml.proto import sysml_pb2


def test_connection_load():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        # Mock ParseFile RPC response
        pb_root = sysml_pb2.SymbolInfo(
            id="TestModel",
            name="TestModel",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="abc123",
            root=pb_root,
            diagnostics=[],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            model = conn.load("test.sysml")
            
            # Verify ParseFile was called with file path
            call_args = mock_stub.ParseFile.call_args
            request = call_args[0][0]
            assert request.file_path == "test.sysml"
            
            # Verify Model object returned
            assert model.hash == "abc123"
            assert model.root.name == "TestModel"


def test_connection_load_with_diagnostics():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_root = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_diag = sysml_pb2.Diagnostic(
            severity="error",
            message="Parse error",
            span=sysml_pb2.Span(file="bad.sysml", start_line=1, start_column=1, end_line=1, end_column=1),
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash",
            root=pb_root,
            diagnostics=[pb_diag],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            model = conn.load("bad.sysml")
            
            assert len(model.diagnostics) == 1
            assert model.diagnostics[0].message == "Parse error"


def test_connection_load_grpc_error():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        # Simulate gRPC error (e.g., file not found)
        mock_stub.ParseFile.side_effect = grpc.RpcError()
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            
            try:
                conn.load("missing.sysml")
                assert False, "Expected exception"
            except grpc.RpcError:
                pass  # Expected
```

---

- [ ] **Step 6: Run test to verify it fails**

Run: `pytest tests/test_connection.py::test_connection_load -v`
Expected: FAIL with "AttributeError: 'Connection' object has no attribute 'load'"

---

- [ ] **Step 7: Implement load() method**

```python
# Add to opensysml/connection.py after close() method

from opensysml.model import Model

    def load(self, file_path):
        """Load a SysML model from file.
        
        Args:
            file_path (str): Path to .sysml file
        
        Returns:
            Model: Parsed model object
        
        Raises:
            grpc.RpcError: If file not found or gRPC error occurs
        """
        request = sysml_pb2.ParseFileRequest(file_path=file_path)
        response = self._stub.ParseFile(request)
        return Model(response, self)
```

---

- [ ] **Step 8: Run test to verify load() passes**

Run: `pytest tests/test_connection.py::test_connection_load tests/test_connection.py::test_connection_load_with_diagnostics tests/test_connection.py::test_connection_load_grpc_error -v`
Expected: 3 tests PASS

---

- [ ] **Step 9: Write failing test for get_symbol() method**

```python
# Add to tests/test_connection.py

def test_connection_get_symbol():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_sym = sysml_pb2.SymbolInfo(
            id="Vehicle::Engine",
            name="Engine",
            kind="PartUsage",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=pb_sym,
            error="",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            pb_result = conn.get_symbol("model_hash", "Vehicle::Engine")
            
            # Verify GetSymbol was called correctly
            call_args = mock_stub.GetSymbol.call_args
            request = call_args[0][0]
            assert request.model_hash == "model_hash"
            assert request.symbol_id == "Vehicle::Engine"
            
            # Verify SymbolInfo returned
            assert pb_result.id == "Vehicle::Engine"
            assert pb_result.name == "Engine"


def test_connection_get_symbol_not_found():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=None,
            error="Symbol not found: NonExistent",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            pb_result = conn.get_symbol("hash", "NonExistent")
            
            # Should return None when error is set
            assert pb_result is None
```

---

- [ ] **Step 10: Run test to verify it fails**

Run: `pytest tests/test_connection.py::test_connection_get_symbol -v`
Expected: FAIL with "AttributeError: 'Connection' object has no attribute 'get_symbol'"

---

- [ ] **Step 11: Implement get_symbol() method**

```python
# Add to opensysml/connection.py after load() method

    def get_symbol(self, model_hash, symbol_id):
        """Fetch symbol by ID from cached model.
        
        Args:
            model_hash (str): Model content hash
            symbol_id (str): Fully-qualified symbol ID
        
        Returns:
            sysml_pb2.SymbolInfo or None: Symbol protobuf, or None if not found
        """
        request = sysml_pb2.GetSymbolRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
        )
        response = self._stub.GetSymbol(request)
        
        if response.error:
            # Symbol not found or other error
            return None
        
        return response.symbol
```

---

- [ ] **Step 12: Run test to verify get_symbol() passes**

Run: `pytest tests/test_connection.py::test_connection_get_symbol tests/test_connection.py::test_connection_get_symbol_not_found -v`
Expected: 2 tests PASS

---

- [ ] **Step 13: Run all connection tests**

Run: `pytest tests/test_connection.py -v`
Expected: All 7 tests PASS

---

- [ ] **Step 14: Commit**

```bash
git add opensysml/connection.py tests/test_connection.py
git commit -m "feat(python): implement Connection class with load() and get_symbol()"
```

---

## Task 6: Package Integration and Manual Testing

**Goal:** Wire up package exports and verify end-to-end functionality with real sysml-grpc service

**Files:**
- Modify: `opensysml/__init__.py`

---

- [ ] **Step 1: Update opensysml/__init__.py with public API**

```python
# opensysml/__init__.py
"""opensysml - Python client library for OpenSysML SysML v2 parser."""

__version__ = "0.1.0"

from opensysml.connection import Connection
from opensysml.model import Model
from opensysml.symbol import Symbol
from opensysml.diagnostic import Diagnostic

__all__ = [
    "Connection",
    "Model",
    "Symbol",
    "Diagnostic",
]
```

---

- [ ] **Step 2: Verify package imports work**

```bash
python -c "from opensysml import Connection, Model, Symbol, Diagnostic; print('OK')"
```

Expected: Output `OK` (no import errors)

---

- [ ] **Step 3: Install package in development mode**

```bash
pip install -e .
```

Expected: Package installed successfully

---

- [ ] **Step 4: Verify CLI import works**

```bash
python -c "import opensysml; print(opensysml.__version__)"
```

Expected: Output `0.1.0`

---

- [ ] **Step 5: Start sysml-grpc service manually**

```bash
# In separate terminal:
./bin/sysml-grpc
```

Expected: Service starts and listens on port 50051

---

- [ ] **Step 6: Write manual test script**

```python
# test_manual.py (temporary file, not committed)
from opensysml import Connection

# Connect to service
conn = Connection(port=50051)

# Test with A1.sysml if available
try:
    model = conn.load("testdata/A1.sysml")
    print(f"✓ Loaded model: {model.root.name} ({model.root.kind})")
    print(f"  Hash: {model.hash}")
    print(f"  Diagnostics: {len(model.diagnostics)}")
    
    # Try finding a symbol
    if model.root.children():
        first_child = model.root.children()[0]
        print(f"✓ First child: {first_child.name} ({first_child.kind})")
    
    print("\n✓ All manual tests passed!")
    
except Exception as e:
    print(f"✗ Error: {e}")
    import traceback
    traceback.print_exc()
finally:
    conn.close()
```

---

- [ ] **Step 7: Run manual test**

```bash
python test_manual.py
```

Expected: 
```
✓ Loaded model: [root name] (Package)
  Hash: [hash string]
  Diagnostics: [count]
✓ First child: [name] ([kind])

✓ All manual tests passed!
```

---

- [ ] **Step 8: Test context manager usage**

```python
# test_context.py (temporary file, not committed)
from opensysml import Connection

with Connection(port=50051) as conn:
    model = conn.load("testdata/A1.sysml")
    print(f"✓ Context manager works: {model.root.name}")

print("✓ Connection closed automatically")
```

Run: `python test_context.py`
Expected: No errors, connection closes automatically

---

- [ ] **Step 9: Clean up temporary test files**

```bash
rm test_manual.py test_context.py
```

---

- [ ] **Step 10: Commit**

```bash
git add opensysml/__init__.py
git commit -m "feat(python): export public API from package"
```

---

## Task 7: Comprehensive Unit Test Suite

**Goal:** Verify all unit tests pass and achieve good coverage

**Files:**
- Create: `tests/__init__.py`
- Create: `tests/conftest.py` (pytest fixtures)

---

- [ ] **Step 1: Create tests/__init__.py**

```python
# tests/__init__.py
"""Unit tests for opensysml package."""
```

---

- [ ] **Step 2: Create pytest fixtures**

```python
# tests/conftest.py
"""Pytest fixtures for opensysml tests."""

import pytest
from unittest.mock import Mock
from opensysml.proto import sysml_pb2


@pytest.fixture
def mock_symbol():
    """Create a mock protobuf SymbolInfo."""
    return sysml_pb2.SymbolInfo(
        id="Test::Symbol",
        name="Symbol",
        kind="PartDef",
        metadata={"visibility": "public"},
        child_ids=[],
        attributes=[],
    )


@pytest.fixture
def mock_diagnostic():
    """Create a mock protobuf Diagnostic."""
    return sysml_pb2.Diagnostic(
        severity="error",
        message="Test error",
        span=sysml_pb2.Span(
            file="test.sysml",
            start_line=10,
            start_column=5,
            end_line=10,
            end_column=10,
        ),
    )


@pytest.fixture
def mock_parse_response(mock_symbol):
    """Create a mock ParseFileResponse."""
    return sysml_pb2.ParseFileResponse(
        model_hash="test_hash_123",
        root=mock_symbol,
        diagnostics=[],
    )


@pytest.fixture
def mock_client():
    """Create a mock Connection client."""
    return Mock()
```

---

- [ ] **Step 3: Run complete test suite**

```bash
pytest tests/ -v
```

Expected: All tests PASS (should be 15+ tests total across all modules)

---

- [ ] **Step 4: Run tests with coverage**

```bash
pytest tests/ --cov=opensysml --cov-report=term-missing
```

Expected: 
- Coverage >= 85%
- All critical paths covered

---

- [ ] **Step 5: Check for any missing tests**

Review coverage report. If any important code paths are uncovered, add tests:

```bash
# If needed, add additional tests to existing test_*.py files
# Focus on error paths and edge cases
```

---

- [ ] **Step 6: Verify imports from package work**

```bash
python -c "
from opensysml import Connection, Model, Symbol, Diagnostic
print('All imports successful')
"
```

Expected: No errors

---

- [ ] **Step 7: Run full verification suite**

```bash
# Install with dev dependencies
pip install -e ".[dev]"

# Run tests
pytest tests/ -v

# Check code style (if tools available)
python -m pylint opensysml/ || true
python -m mypy opensysml/ || true
```

Expected: Tests pass, code quality checks clean (or warnings only)

---

- [ ] **Step 8: Commit**

```bash
git add tests/__init__.py tests/conftest.py
git commit -m "test(python): add pytest fixtures and verify test suite"
```

---

- [ ] **Step 9: Final verification - README example**

Test the exact example from README.md:

```python
# final_test.py
from opensysml import Connection

# Note: Requires sysml-grpc running on port 50051
# and testdata/A1.sysml to exist

conn = Connection(port=50051)

try:
    model = conn.load("testdata/A1.sysml")
    
    root = model.root
    print(f"Root: {root.name} ({root.kind})")
    
    # This won't work unless A1.sysml has SPACECRAFT_WET symbol
    # part = model.find("SPACECRAFT_WET")
    # if part:
    #     print(f"Part: {part.name}")
    
    for diag in model.diagnostics:
        print(f"{diag.severity}: {diag.message} at {diag.span}")
    
    print("\n✓ README example verified")
    
finally:
    conn.close()
```

Run: `python final_test.py` (with sysml-grpc running)
Expected: No crashes, reasonable output

---

- [ ] **Step 10: Clean up and final commit**

```bash
rm final_test.py
git status
```

Expected: Only tracked files, working directory clean
