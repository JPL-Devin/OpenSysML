"""Tests for Instance class."""
import pytest
from opensysml.errors import SlotError
from opensysml.proto import sysml_pb2
from opensysml.instance import Instance


def scalar_slot(name, **value_kwargs):
    """Build a materialized scalar SlotValue."""
    return sysml_pb2.SlotValue(
        feature_name=name,
        value=sysml_pb2.Value(**value_kwargs),
        materialized=True,
    )


def vehicle_graph():
    """Build a Vehicle instance holding a nested Engine, as Instantiate returns."""
    engine = sysml_pb2.Instance(
        id=2,
        type_symbol_id="Demo::Engine",
        slots={"power": scalar_slot("power", real_value=300.0)},
    )
    vehicle = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        slots={
            "mass": scalar_slot("mass", real_value=1500.0),
            "engine": scalar_slot("engine", instance_id=2),
        },
    )
    return vehicle, {1: vehicle, 2: engine}


def test_instance_properties():
    """Test Instance wraps protobuf correctly."""
    pb_inst = sysml_pb2.Instance(
        id=123,
        type_symbol_id="Test::MyPart",
        slots={"mass": scalar_slot("mass", int_value=100)},
    )

    inst = Instance(pb_inst)
    assert inst.id == 123
    assert inst.type_symbol_id == "Test::MyPart"
    assert inst.slots == {"mass": 100}


def test_instance_get_slot_returns_protobuf():
    """get_slot keeps the raw protobuf reachable."""
    pb_inst = sysml_pb2.Instance(
        id=456,
        type_symbol_id="Test::Vehicle",
        slots={"engine": scalar_slot("engine", instance_id=9)},
    )

    inst = Instance(pb_inst)
    slot = inst.get_slot("engine")
    assert isinstance(slot, sysml_pb2.SlotValue)
    assert slot.feature_name == "engine"
    assert slot.materialized is True
    assert slot.value.WhichOneof("kind") == "instance_id"

    assert inst.get_slot("nonexistent") is None
    assert inst.raw_slots["engine"] is slot


def test_slots_convert_scalars_and_sequences():
    """Slot values are converted to plain Python values."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        slots={
            "count": scalar_slot("count", int_value=3),
            "mass": scalar_slot("mass", real_value=1500.0),
            "ready": scalar_slot("ready", bool_value=True),
            "name": scalar_slot("name", string_value="v1"),
            "sizes": sysml_pb2.SlotValue(
                feature_name="sizes",
                values=[sysml_pb2.Value(int_value=1), sysml_pb2.Value(int_value=2)],
                materialized=True,
            ),
        },
    )

    inst = Instance(pb_inst)
    assert inst.slots == {
        "count": 3,
        "mass": 1500.0,
        "ready": True,
        "name": "v1",
        "sizes": [1, 2],
    }


def test_attribute_and_item_access():
    """Slots are reachable as attributes and by name."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    assert inst.mass == 1500.0
    assert inst["mass"] == 1500.0
    assert "mass" in inst

    with pytest.raises(AttributeError):
        inst.nonexistent
    with pytest.raises(KeyError):
        inst["nonexistent"]
    assert not hasattr(inst, "nonexistent")


def test_attribute_access_does_not_shadow_members():
    """Real attributes and methods win over slots of the same name."""
    pb_inst = sysml_pb2.Instance(
        id=7,
        type_symbol_id="Test::P",
        slots={
            "id": scalar_slot("id", int_value=99),
            "slots": scalar_slot("slots", int_value=5),
            "get_slot": scalar_slot("get_slot", int_value=5),
        },
    )

    inst = Instance(pb_inst)
    assert inst.id == 7
    assert isinstance(inst.slots, dict)
    assert callable(inst.get_slot)
    # Shadowed slots stay reachable by name.
    assert inst["id"] == 99


def test_dunder_lookup_raises_attribute_error():
    """Dunder lookups must not be answered from slots, so copy/pickle work."""
    import copy

    pb_inst = sysml_pb2.Instance(id=1, type_symbol_id="Test::P")
    inst = Instance(pb_inst)

    with pytest.raises(AttributeError):
        inst.__deepcopy__
    assert copy.copy(inst) is not None


def test_nested_instance_resolution():
    """An instance-valued slot resolves to an Instance from the graph."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    engine = inst.engine
    assert isinstance(engine, Instance)
    assert engine.id == 2
    assert engine.type_symbol_id == "Demo::Engine"
    assert engine.power == 300.0
    assert inst.slots["engine"].id == 2
    # Wrappers are shared within one graph.
    assert inst.engine is engine


def test_unresolvable_instance_id_falls_back_to_id():
    """Without the child in the graph, the bare id is returned."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        slots={"engine": scalar_slot("engine", instance_id=42)},
    )

    inst = Instance(pb_inst)
    assert inst.engine == 42


def test_error_slot_raises_slot_error():
    """A slot the service failed to evaluate raises instead of returning None."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::Cyclic",
        slots={
            "a": sysml_pb2.SlotValue(feature_name="a", error="cyclic slot dependency: a"),
        },
    )

    inst = Instance(pb_inst)
    with pytest.raises(SlotError) as excinfo:
        inst.a
    assert excinfo.value.feature_name == "a"
    assert "cyclic" in excinfo.value.message
    # slots exposes the error rather than raising, so the instance stays inspectable.
    assert isinstance(inst.slots["a"], SlotError)


def test_unmaterialized_slot_raises_slot_error():
    """An unmaterialized slot is not silently reported as None."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        slots={"lazy": sysml_pb2.SlotValue(feature_name="lazy")},
    )

    inst = Instance(pb_inst)
    with pytest.raises(SlotError, match="not materialized"):
        inst.lazy
    assert inst.get_slot("lazy").materialized is False


def test_unsupported_value_surfaces_as_slot_error():
    """A null carrying a reason is an error, an empty null is a real None."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        slots={
            "quantity": scalar_slot("quantity", null="unsupported: quantity value"),
            "empty": scalar_slot("empty", null=""),
        },
    )

    inst = Instance(pb_inst)
    with pytest.raises(SlotError, match="quantity"):
        inst.quantity
    assert inst.empty is None


def test_get_returns_default_for_missing_slot():
    """get() mirrors dict.get for unknown slots."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    assert inst.get("mass") == 1500.0
    assert inst.get("missing") is None
    assert inst.get("missing", 0) == 0


def test_instance_str():
    """Test string representation."""
    pb_inst = sysml_pb2.Instance(id=789, type_symbol_id="Test::Part")
    inst = Instance(pb_inst)

    assert "789" in str(inst)
    assert "Test::Part" in str(inst)

def test_graph_is_shared_not_copied():
    """Nested instances reuse one graph dict instead of copying it per node."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    assert inst._graph is graph
    assert inst.engine._graph is graph
