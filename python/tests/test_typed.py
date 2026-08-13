"""Tests for the typed view runtime that generated classes are built on."""

import pytest

from pysysml import typed as _t
from pysysml.errors import SlotError, TypeMismatchError
from pysysml.instance import Instance
from pysysml.proto import sysml_pb2


class Engine(_t.TypedObject):
    sysml_id = "Demo::Engine"

    @property
    def power(self) -> float:
        return _t.slot(self, "power", _t.as_float)


class Vehicle(_t.TypedObject):
    sysml_id = "Demo::Vehicle"

    @property
    def mass(self) -> float:
        return _t.slot(self, "mass", _t.as_float)

    @property
    def engine(self) -> Engine:
        return _t.slot(self, "engine", _t.as_typed(Engine))

    @property
    def broken(self) -> float:
        return _t.slot(self, "broken", _t.as_float)

    @property
    def label(self) -> str:
        return _t.slot(self, "label", _t.as_str)

    @property
    def spare(self):
        return _t.optional_slot(self, "spare", _t.as_typed(Engine))

    @property
    def ratios(self):
        return _t.list_slot(self, "ratios", _t.as_float)


def scalar_slot(name, **value_kwargs):
    return sysml_pb2.SlotValue(
        feature_name=name, value=sysml_pb2.Value(**value_kwargs), materialized=True
    )


def vehicle_instance(**extra_slots):
    engine = sysml_pb2.Instance(
        id=2, type_symbol_id="Demo::Engine", slots={"power": scalar_slot("power", real_value=300.0)}
    )
    slots = {
        "mass": scalar_slot("mass", real_value=1500.0),
        "engine": scalar_slot("engine", instance_id=2),
        "broken": sysml_pb2.SlotValue(feature_name="broken", error="evaluation failed"),
        "label": scalar_slot("label", int_value=7),
        "ratios": sysml_pb2.SlotValue(
            feature_name="ratios",
            values=[sysml_pb2.Value(real_value=1.5), sysml_pb2.Value(int_value=2)],
            materialized=True,
        ),
    }
    slots.update(extra_slots)
    vehicle = sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle", slots=slots)
    return Instance(vehicle, {1: vehicle, 2: engine})


def test_from_instance_reads_scalar_and_nested_slots():
    """A typed view delegates to the instance and wraps nested instances."""
    v = Vehicle.from_instance(vehicle_instance())
    assert v.mass == 1500.0
    assert isinstance(v.engine, Engine)
    assert v.engine.power == 300.0
    assert v.instance.id == 1


def test_slot_error_is_preserved():
    """A slot that failed to evaluate still raises SlotError, never None."""
    v = Vehicle.from_instance(vehicle_instance())
    with pytest.raises(SlotError) as excinfo:
        v.broken
    assert excinfo.value.feature_name == "broken"


def test_type_mismatch_is_reported():
    """A slot holding another type raises rather than returning a wrong value."""
    v = Vehicle.from_instance(vehicle_instance())
    with pytest.raises(TypeMismatchError) as excinfo:
        v.label
    assert excinfo.value.expected == "str"


def test_missing_required_slot_raises():
    """A required slot the instance never carried is an error, not None."""
    pb = sysml_pb2.Instance(id=3, type_symbol_id="Demo::Vehicle", slots={})
    with pytest.raises(TypeMismatchError):
        Vehicle.from_instance(Instance(pb)).mass


def test_integer_widens_to_float_but_bool_does_not():
    """An integer Real value widens; a Boolean is never a number."""
    assert _t.as_float("x", 3) == 3.0
    with pytest.raises(TypeMismatchError):
        _t.as_float("x", True)
    with pytest.raises(TypeMismatchError):
        _t.as_int("x", True)


def test_optional_slot_returns_none_when_absent():
    """An absent 0..1 slot is None; a present one is decoded."""
    v = Vehicle.from_instance(vehicle_instance())
    assert v.spare is None

    with_spare = Vehicle.from_instance(vehicle_instance(spare=scalar_slot("spare", instance_id=2)))
    assert isinstance(with_spare.spare, Engine)


def test_list_slot_decodes_every_element():
    """A multi-valued slot decodes each element."""
    v = Vehicle.from_instance(vehicle_instance())
    assert v.ratios == [1.5, 2.0]


def test_list_slot_is_empty_when_absent_or_null():
    """A collection slot the instance never carried, or holding null, reads as empty."""
    pb = sysml_pb2.Instance(id=4, type_symbol_id="Demo::Vehicle", slots={})
    assert Vehicle.from_instance(Instance(pb)).ratios == []

    null = vehicle_instance(ratios=scalar_slot("ratios", null=""))
    assert Vehicle.from_instance(null).ratios == []


def test_typed_objects_compare_by_instance_identity():
    """Two views of the same instance are equal; different classes are not."""
    inst = vehicle_instance()
    assert Vehicle.from_instance(inst) == Vehicle.from_instance(inst)
    assert Vehicle.from_instance(inst) != Engine.from_instance(inst)
