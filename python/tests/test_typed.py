"""Tests for the typed view runtime that generated classes are built on."""

import pytest

from pysysml import typed as _t
from pysysml.errors import InstanceTypeError, SlotError, TypeMismatchError
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
    # An Engine view over a Vehicle is what from_instance now rejects, so the
    # different-class case is built through the deliberate escape hatch.
    assert Vehicle.from_instance(inst) != Engine.unchecked(inst)


class SportsCar(Vehicle):
    """Stands in for a generated class of a definition specializing Demo::Vehicle."""

    sysml_id = "Demo::SportsCar"

    @property
    def top_speed(self) -> float:
        return _t.slot(self, "topSpeed", _t.as_float)


def sports_car_instance():
    pb = sysml_pb2.Instance(
        id=5,
        type_symbol_id="Demo::SportsCar",
        slots={
            "mass": scalar_slot("mass", real_value=1200.0),
            "topSpeed": scalar_slot("topSpeed", real_value=250.0),
        },
    )
    return Instance(pb, {5: pb})


def test_as_quantity_decodes_a_quantity_and_rejects_a_bare_number():
    """A quantity slot decodes to a Quantity; a unitless value is a mismatch."""
    from pysysml.values import Unit

    quantity = _t.Quantity(5.0, Unit(text="SI::kg"))

    assert _t.as_quantity("mass", quantity) is quantity
    with pytest.raises(TypeMismatchError) as excinfo:
        _t.as_quantity("mass", 5.0)
    assert excinfo.value.expected == "Quantity"


def test_the_type_errors_are_reachable_from_the_package():
    """A caller catches these the documented way, through the package namespace."""
    import pysysml

    assert pysysml.InstanceTypeError is InstanceTypeError
    assert issubclass(pysysml.MissingCapabilityError, pysysml.PySysMLError)


def test_from_instance_rejects_an_instance_of_another_type():
    """A wrong-type instance fails at from_instance, naming both types."""
    with pytest.raises(InstanceTypeError) as excinfo:
        Engine.from_instance(vehicle_instance())
    assert excinfo.value.expected == "Demo::Engine"
    assert excinfo.value.actual == "Demo::Vehicle"
    assert "Demo::Engine" in str(excinfo.value)
    assert "Demo::Vehicle" in str(excinfo.value)


def test_from_instance_accepts_a_subtype_instance():
    """A subtype instance is legitimate where its base class is expected."""
    inst = sports_car_instance()
    as_base = Vehicle.from_instance(inst)
    assert as_base.mass == 1200.0
    assert SportsCar.from_instance(inst).top_speed == 250.0


def test_from_instance_accepts_a_type_no_generated_class_describes():
    """An instantiated usage reports its own FQN, which no class carries.

    The client cannot relate such an id to a definition, so it does not reject
    it: doing so would break instantiating a usage, the ordinary way to get an
    instance. Slot decoding still reports a wrong shape.
    """
    pb = sysml_pb2.Instance(
        id=6,
        type_symbol_id="Demo::myCar",
        slots={"mass": scalar_slot("mass", real_value=900.0)},
    )
    assert Vehicle.from_instance(Instance(pb, {6: pb})).mass == 900.0


def test_from_instance_accepts_an_instance_with_no_reported_type():
    """An instance carrying no type at all is not evidence of a mismatch."""
    pb = sysml_pb2.Instance(id=7, slots={"mass": scalar_slot("mass", real_value=1.0)})
    assert Vehicle.from_instance(Instance(pb, {7: pb})).mass == 1.0


def test_unchecked_bypasses_the_type_check():
    """The escape hatch is explicit at the call site and does not check."""
    view = Engine.unchecked(vehicle_instance())
    assert view.instance.type_symbol_id == "Demo::Vehicle"
    with pytest.raises(TypeMismatchError):
        view.power


def test_nested_slot_of_the_wrong_type_is_rejected():
    """The guard also applies to a nested instance decoded into a typed view."""
    nested = vehicle_instance(engine=scalar_slot("engine", instance_id=1))
    with pytest.raises(InstanceTypeError):
        nested_view = Vehicle.from_instance(nested)
        nested_view.engine
