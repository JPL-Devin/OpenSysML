"""Tests for Model class."""

import pytest
from unittest.mock import Mock
from pysysml.proto import sysml_pb2
from pysysml.errors import ExecutionError
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


def test_model_find_accepts_fully_qualified_name():
    """A symbol's own id round-trips back into find()."""
    pb_root = sysml_pb2.SymbolInfo(
        id="Lander",
        name="Lander",
        kind="package",
        metadata={},
        child_ids=["Lander::Rhs"],
        attributes=[],
    )

    pb_rhs = sysml_pb2.SymbolInfo(
        id="Lander::Rhs",
        name="Rhs",
        kind="calcDef",
        metadata={},
        child_ids=[],
        attributes=[],
    )

    mock_client = Mock()
    mock_client.get_symbol.side_effect = lambda model_hash, symbol_id: {
        "Lander::Rhs": pb_rhs,
    }.get(symbol_id)

    model = Model(
        sysml_pb2.ParseFileResponse(model_hash="hash", root=pb_root, diagnostics=[]),
        mock_client,
    )

    by_short_name = model.find("Rhs")
    by_fqn = model.find("Lander::Rhs")

    assert by_short_name is not None
    assert by_fqn is not None
    assert by_fqn.id == by_short_name.id == "Lander::Rhs"
    assert by_fqn.kind == "calcDef"

    # The root's own id is accepted too, and a name no symbol carries is not.
    assert model.find("Lander") is not None
    assert model.find("Lander::Missing") is None


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


def test_model_get_by_fqn():
    pb_root = sysml_pb2.SymbolInfo(
        id="MyModel",
        name="MyModel",
        kind="Package",
        metadata={},
        child_ids=["MyModel::Vehicle"],
        attributes=[],
    )

    pb_vehicle = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["MyModel::Vehicle::engine"],
        attributes=[],
    )

    pb_engine = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle::engine",
        name="engine",
        kind="partUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )

    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash",
        root=pb_root,
        diagnostics=[],
    )

    mock_client = Mock()
    mapping = {
        "MyModel::Vehicle": pb_vehicle,
        "MyModel::Vehicle::engine": pb_engine,
    }
    mock_client.get_symbol.side_effect = lambda model_hash, symbol_id: mapping.get(symbol_id)

    model = Model(pb_response, mock_client)

    assert model.get("MyModel").id == "MyModel"
    assert model.get("MyModel::Vehicle").name == "Vehicle"
    assert model.get("MyModel::Vehicle::engine").name == "engine"

    # A short name is not an FQN, and an unknown FQN is not an error.
    assert model.get("Vehicle") is None
    assert model.get("MyModel::Missing") is None


class TestModelEval:
    """Evaluation is on the model, so a caller need not carry its hash."""

    def _model(self, client):
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash1",
            root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
        )
        return Model(pb_response, client)

    def test_eval_passes_the_models_hash(self):
        client = Mock()
        client.eval.return_value = 2

        assert self._model(client).eval("1+1") == 2
        client.eval.assert_called_once_with("1+1", "hash1", context_symbol_id=None)

    def test_eval_passes_a_context_symbol(self):
        client = Mock()
        client.eval.return_value = 1500.0

        model = self._model(client)
        assert model.eval("mass", context_symbol_id="Demo::sedan") == 1500.0
        client.eval.assert_called_once_with(
            "mass", "hash1", context_symbol_id="Demo::sedan"
        )

    def test_eval_raises_what_the_connection_raises(self):
        client = Mock()
        client.eval.side_effect = ExecutionError("division by zero")

        with pytest.raises(ExecutionError):
            self._model(client).eval("1/0")
