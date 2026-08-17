"""A slot holding no value, as a valueless feature of a value type has.

The service sends such a slot materialized with the ``unset`` arm of ``Value``,
so the client reads it as :data:`pysysml.UNSET` — falsy, spelled ``<unset>`` as
every other surface spells it, and distinct from ``None``, the model's ``null``.
"""

import pysysml
from pysysml.errors import SlotError
from pysysml.instance import Instance
from pysysml.proto import sysml_pb2
from pysysml.values import UNSET, UnsetType, slot_to_python, value_to_python

import pytest


def unset_slot(name):
    """A materialized scalar slot holding no value, as the service sends one."""
    return sysml_pb2.SlotValue(
        feature_name=name,
        value=sysml_pb2.Value(unset=True),
        materialized=True,
    )


def test_unset_reads_as_the_same_spelling_every_surface_uses():
    assert str(UNSET) == "<unset>"
    assert repr(UNSET) == "<unset>"
    assert not UNSET
    assert UnsetType() is UNSET


def test_unset_is_not_none():
    assert UNSET is not None
    assert value_to_python(sysml_pb2.Value(null="")) is None
    assert value_to_python(sysml_pb2.Value(unset=True)) is UNSET


def test_unset_in_a_sequence_is_read_element_by_element():
    sequence = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        sysml_pb2.Value(unset=True),
        sysml_pb2.Value(real_value=2.0),
    ]))
    assert value_to_python(sequence) == [UNSET, 2.0]


def test_a_slot_holding_no_value_reads_unset():
    assert slot_to_python("d", unset_slot("d")) is UNSET


def test_a_valued_slot_and_an_object_valued_one_are_unaffected():
    valued = sysml_pb2.SlotValue(
        feature_name="k", value=sysml_pb2.Value(real_value=2.0), materialized=True
    )
    assert slot_to_python("k", valued) == 2.0

    object_valued = sysml_pb2.SlotValue(
        feature_name="engine", value=sysml_pb2.Value(instance_id=7), materialized=True
    )
    assert slot_to_python("engine", object_valued) == 7


def test_an_unmaterialized_slot_is_still_an_error():
    """Unset is what a materialized slot holds, not what an absent one is."""
    with pytest.raises(SlotError):
        slot_to_python("d", sysml_pb2.SlotValue(feature_name="d"))


def test_an_instance_exposes_an_unset_slot_as_unset():
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        slots={
            "d": unset_slot("d"),
            "k": sysml_pb2.SlotValue(
                feature_name="k", value=sysml_pb2.Value(real_value=2.0), materialized=True
            ),
        },
    )
    inst = Instance(pb_inst)
    assert inst.d is UNSET
    assert inst.k == 2.0
    assert inst.slots == {"d": UNSET, "k": 2.0}
    # The HTML view spells it the same, escaped for display.
    assert "&lt;unset&gt;" in inst._repr_html_()


def test_unset_is_exported():
    assert pysysml.UNSET is UNSET
    assert pysysml.UnsetType is UnsetType
