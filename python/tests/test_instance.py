"""Tests for Instance class."""
import pytest
from pysysml.proto import sysml_pb2
from pysysml.instance import Instance


def test_instance_properties():
    """Test Instance wraps protobuf correctly."""
    pb_inst = sysml_pb2.Instance(
        id=123,
        type_symbol_id="Test::MyPart",
        slots={
            "mass": sysml_pb2.SlotValue(
                feature_name="mass",
                value=sysml_pb2.Value(int_value=100),
                materialized=True
            )
        }
    )
    
    inst = Instance(pb_inst)
    assert inst.id == 123
    assert inst.type_symbol_id == "Test::MyPart"
    assert len(inst.slots) == 1
    assert "mass" in inst.slots


def test_instance_get_slot():
    """Test get_slot method."""
    pb_inst = sysml_pb2.Instance(
        id=456,
        type_symbol_id="Test::Vehicle",
        slots={"engine": sysml_pb2.SlotValue(feature_name="engine")}
    )
    
    inst = Instance(pb_inst)
    slot = inst.get_slot("engine")
    assert slot is not None
    assert slot.feature_name == "engine"
    
    missing = inst.get_slot("nonexistent")
    assert missing is None


def test_instance_str():
    """Test string representation."""
    pb_inst = sysml_pb2.Instance(id=789, type_symbol_id="Test::Part")
    inst = Instance(pb_inst)
    
    assert "789" in str(inst)
    assert "Test::Part" in str(inst)
