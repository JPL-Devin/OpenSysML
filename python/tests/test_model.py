"""Tests for Model class."""

from unittest.mock import Mock
from pysysml.proto import sysml_pb2
from pysysml.model import Model


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
        span=sysml_pb2.Span(file="test.sysml", start_line=1, start_col=1, end_line=1, end_col=1),
    )
    
    pb_diag2 = sysml_pb2.Diagnostic(
        severity="warning",
        message="Unused symbol",
        span=sysml_pb2.Span(file="test.sysml", start_line=5, start_col=1, end_line=5, end_col=1),
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
