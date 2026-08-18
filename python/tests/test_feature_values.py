"""Tests for the feature-value names and their deprecated slot aliases."""
import pytest
from google.protobuf import descriptor_pb2, descriptor_pool, message_factory
from pysysml.errors import FeatureValueError, SlotError
from pysysml.instance import Instance
from pysysml.proto import sysml_pb2


def feature_value(name, **value_kwargs):
    """Build a materialized single-valued FeatureValue."""
    return sysml_pb2.FeatureValue(
        feature_name=name,
        value=sysml_pb2.Value(**value_kwargs),
        materialized=True,
    )


def slot_value(name, **value_kwargs):
    """Build the same thing in the deprecated SlotValue shape."""
    return sysml_pb2.SlotValue(
        feature_name=name,
        value=sysml_pb2.Value(**value_kwargs),
        materialized=True,
    )


def test_feature_values_are_read_from_the_current_field():
    """A service that fills feature_values is read through either spelling."""
    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={"mass": feature_value("mass", real_value=1500.0)},
    ))

    assert inst.features == {"mass": 1500.0} == inst.slots
    assert inst.mass == 1500.0
    assert inst["mass"] == 1500.0
    assert "mass" in inst
    assert inst.get("mass") == 1500.0
    assert inst.get_feature("mass").feature_name == "mass"
    assert inst.get_slot("mass").feature_name == "mass"
    assert set(inst.raw_features) == {"mass"} == set(inst.raw_slots)


def test_slots_only_instances_still_work():
    """An older service that fills only the deprecated map reads the same."""
    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        slots={"mass": slot_value("mass", real_value=1500.0)},
    ))

    assert inst.features == {"mass": 1500.0} == inst.slots
    assert inst.mass == 1500.0
    assert inst.get_feature("mass").feature_name == "mass"


def pre_rename_instance_class():
    """Build the Instance shape as it was before feature_values was added."""
    pool = descriptor_pool.DescriptorPool()
    for dep in [*sysml_pb2.DESCRIPTOR.dependencies, sysml_pb2.DESCRIPTOR]:
        proto = descriptor_pb2.FileDescriptorProto()
        dep.CopyToProto(proto)
        pool.Add(proto)

    old = descriptor_pb2.FileDescriptorProto(
        name="old_instance.proto", package="old", syntax="proto3",
        dependency=[sysml_pb2.DESCRIPTOR.name])
    msg = old.message_type.add(name="Instance")
    msg.field.add(name="id", number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_INT64,
                  label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL)
    msg.field.add(name="type_symbol_id", number=2,
                  type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                  label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL)
    entry = msg.nested_type.add(name="SlotsEntry")
    entry.options.map_entry = True
    entry.field.add(name="key", number=1, type=descriptor_pb2.FieldDescriptorProto.TYPE_STRING,
                    label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL)
    entry.field.add(name="value", number=2, type=descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE,
                    label=descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL,
                    type_name=".sysml.SlotValue")
    msg.field.add(name="slots", number=3, type=descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE,
                  label=descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED,
                  type_name=".old.Instance.SlotsEntry")
    pool.Add(old)
    return message_factory.GetMessageClass(pool.FindMessageTypeByName("old.Instance"))


def test_instances_from_a_client_generated_before_the_rename_still_work():
    """A message without the feature_values field at all is read through slots."""
    old_instance = pre_rename_instance_class()
    pb = old_instance(id=1, type_symbol_id="Demo::Vehicle")
    # The pool is its own, so the value is carried over as bytes.
    pb.slots["mass"].ParseFromString(
        slot_value("mass", real_value=1500.0).SerializeToString())

    inst = Instance(pb)

    assert "feature_values" not in pb.DESCRIPTOR.fields_by_name
    assert inst.features == {"mass": 1500.0} == inst.slots
    assert inst.mass == 1500.0


def test_both_maps_agree_when_both_are_filled():
    """The current field wins when a service fills both, as ours does."""
    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={"mass": feature_value("mass", real_value=1500.0)},
        slots={"mass": slot_value("mass", real_value=1500.0)},
    ))

    assert inst.features == inst.slots == {"mass": 1500.0}


def test_slot_error_is_the_same_class_as_feature_value_error():
    """`except SlotError` keeps catching what `except FeatureValueError` does."""
    assert SlotError is FeatureValueError

    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={"mass": sysml_pb2.FeatureValue(
            feature_name="mass", error="cyclic feature value dependency")},
    ))

    with pytest.raises(SlotError, match="feature value 'mass'"):
        inst.mass
    assert isinstance(inst.features["mass"], FeatureValueError)


def test_unknown_feature_says_feature():
    inst = Instance(sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle"))

    with pytest.raises(AttributeError, match="no attribute or feature 'nope'"):
        inst.nope
    with pytest.raises(KeyError):
        inst["nope"]
