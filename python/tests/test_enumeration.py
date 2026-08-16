"""Tests for an enumeration literal as a Python value.

A literal travels as the declaration it names, so it must arrive as an
:class:`EnumLiteral` — not as a string, which would be indistinguishable from a
string attribute, and not as an unsupported null.
"""

import pytest

from pysysml import EnumLiteral
from pysysml.connection import Connection
from pysysml.errors import SlotError, UnsupportedValueError
from pysysml.proto import sysml_pb2
from pysysml.values import slot_to_python, value_to_python

RED = sysml_pb2.EnumLiteral(
    literal_id="D::Color::red", enumeration_id="D::Color", name="Color::red"
)
GREEN = sysml_pb2.EnumLiteral(
    literal_id="D::Color::green", enumeration_id="D::Color", name="Color::green"
)


def test_value_to_python_returns_the_literal():
    got = value_to_python(sysml_pb2.Value(enum_literal=RED))

    assert got == EnumLiteral("D::Color::red", "D::Color", "Color::red")
    assert str(got) == "Color::red"


def test_a_literal_is_not_a_string():
    """The literal and its rendering are different values."""
    assert value_to_python(sysml_pb2.Value(enum_literal=RED)) != "Color::red"


def test_literal_identity_is_the_declaration():
    same = value_to_python(sysml_pb2.Value(enum_literal=RED))
    other = value_to_python(sysml_pb2.Value(enum_literal=GREEN))

    assert same == value_to_python(sysml_pb2.Value(enum_literal=RED))
    assert same != other
    # Hashable, so a literal can key a dict and a repeated one collapses.
    assert len({same, other, value_to_python(sysml_pb2.Value(enum_literal=RED))}) == 2


def test_a_sequence_of_literals_keeps_them():
    seq = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        sysml_pb2.Value(enum_literal=RED),
        sysml_pb2.Value(enum_literal=GREEN),
    ]))

    assert value_to_python(seq) == [
        EnumLiteral("D::Color::red", "D::Color", "Color::red"),
        EnumLiteral("D::Color::green", "D::Color", "Color::green"),
    ]


def test_an_enum_slot_is_no_longer_unsupported():
    slot = sysml_pb2.SlotValue(
        feature_name="c", value=sysml_pb2.Value(enum_literal=RED), materialized=True
    )

    assert slot_to_python("c", slot) == EnumLiteral(
        "D::Color::red", "D::Color", "Color::red"
    )


def test_a_literal_is_sent_as_a_literal():
    """Round trip: what the client sends is what it reads back."""
    literal = EnumLiteral("D::Color::red", "D::Color", "Color::red")

    # _python_to_value uses no connection state, so no service is needed.
    pb_value = Connection._python_to_value(None, literal)

    assert pb_value.WhichOneof("kind") == "enum_literal"
    assert pb_value.enum_literal.literal_id == "D::Color::red"
    assert value_to_python(pb_value) == literal


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the reason, which stays an error."""
    with pytest.raises(UnsupportedValueError):
        value_to_python(sysml_pb2.Value(null="unsupported"))

    slot = sysml_pb2.SlotValue(
        feature_name="c", value=sysml_pb2.Value(null="unsupported"), materialized=True
    )
    with pytest.raises(SlotError):
        slot_to_python("c", slot)
