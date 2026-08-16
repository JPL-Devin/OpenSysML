"""Tests for the quantity value: decoding, comparison and arithmetic.

Two layers. Against protobuf messages built by hand, what a quantity is and how
it behaves; against the real ``sysml-grpc`` binary, that a model's quantity slots
— written, derived, compound-unit and nested — read back as quantities.
"""

import pytest

from pysysml.errors import SlotError, UnsupportedValueError
from pysysml.instance import Instance
from pysysml.proto import sysml_pb2
from pysysml.values import (
    IncommensurableUnitsError,
    Quantity,
    Unit,
    UnitFactor,
    value_to_python,
)

# Reductions of the units these tests measure with, as the service sends them.
METRE = (("SI::metre", 1.0),)
SECOND = (("SI::second", 1.0),)
GRAM = (("SI::gram", 1.0),)
METRE_PER_SECOND = (("SI::metre", 1.0), ("SI::second", -1.0))


def unit(text, factors=METRE, scale_num=1.0, scale_den=1.0):
    """A unit as written, over the base units it reduces to."""
    return Unit(
        text=text,
        scale_num=scale_num,
        scale_den=scale_den,
        factors=tuple(UnitFactor(unit_id, exponent) for unit_id, exponent in factors),
    )


def pb_quantity(unit_text, factors=METRE, scale_num=1.0, scale_den=1.0, **magnitude):
    """A Quantity message as the service sends one."""
    return sysml_pb2.Quantity(
        unit=unit_text,
        unit_term=sysml_pb2.UnitTerm(
            scale_num=scale_num,
            scale_den=scale_den,
            factors=[
                sysml_pb2.UnitFactor(unit_id=unit_id, exponent=exponent)
                for unit_id, exponent in factors
            ],
        ),
        **magnitude,
    )


def test_a_quantity_decodes_with_its_magnitude_and_unit_as_written():
    """The magnitude arrives in the unit written, not reduced to base units."""
    value = sysml_pb2.Value(
        quantity=pb_quantity(
            "SI::km/SI::h",
            factors=METRE_PER_SECOND,
            scale_num=5.0,
            scale_den=18.0,
            real_magnitude=5.4,
        )
    )

    got = value_to_python(value)

    assert isinstance(got, Quantity)
    assert got.magnitude == 5.4
    assert got.unit.text == "SI::km/SI::h"
    assert got.unit.exponents() == {"SI::metre": 1.0, "SI::second": -1.0}
    assert str(got) == "5.4 [SI::km/SI::h]"


def test_an_integer_magnitude_stays_an_integer():
    """Integer and Real are distinct in the model, so they are here too."""
    got = value_to_python(sysml_pb2.Value(quantity=pb_quantity("SI::m", int_magnitude=3)))

    assert got.magnitude == 3
    assert isinstance(got.magnitude, int)
    assert str(got) == "3 [SI::m]"


def test_a_quantity_with_no_magnitude_is_reported():
    """A magnitude-less quantity is a value the client cannot read, not a zero."""
    value = sysml_pb2.Value(quantity=sysml_pb2.Quantity(unit="SI::m"))

    with pytest.raises(UnsupportedValueError, match="carries no magnitude"):
        value_to_python(value)


def test_a_quantity_slot_reads_off_an_instance():
    """The slot path returns the quantity, which is the whole point of the wire form."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="P::Car",
        slots={
            "m": sysml_pb2.SlotValue(
                feature_name="m",
                materialized=True,
                value=sysml_pb2.Value(
                    quantity=pb_quantity(
                        "SI::kg", factors=GRAM, scale_num=1000.0, real_magnitude=5.0
                    )
                ),
            ),
            "n": sysml_pb2.SlotValue(
                feature_name="n",
                materialized=True,
                value=sysml_pb2.Value(real_value=2.0),
            ),
        },
    )

    inst = Instance(pb_inst)

    assert inst.m == Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0))
    assert inst.n == 2.0


def test_a_quantity_in_a_collection_slot_decodes_per_element():
    """Every element of a collection slot is decoded, quantities included."""
    pb_slot = sysml_pb2.SlotValue(
        feature_name="masses",
        materialized=True,
        values=[
            sysml_pb2.Value(quantity=pb_quantity("SI::m", real_magnitude=1.0)),
            sysml_pb2.Value(quantity=pb_quantity("SI::m", real_magnitude=2.0)),
        ],
    )
    pb_inst = sysml_pb2.Instance(id=1, type_symbol_id="P::Car", slots={"masses": pb_slot})

    assert Instance(pb_inst).masses == [
        Quantity(1.0, unit("SI::m")),
        Quantity(2.0, unit("SI::m")),
    ]


def test_commensurable_quantities_compare_over_their_reduction():
    """`5.4 [km/h]` and `1.5 [m/s]` are one value, exactly, at the boundary."""
    kmh = Quantity(5.4, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0))
    ms = Quantity(1.5, unit("SI::m/SI::s", METRE_PER_SECOND))

    assert kmh == ms
    assert not kmh < ms
    assert kmh <= ms
    assert kmh >= ms
    assert Quantity(3.6, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0)) < ms
    assert hash(kmh) == hash(ms)


def test_a_converted_magnitude_stays_exact():
    """The conversion applies as one ratio, so 5.4 km/h is 1.5 m/s and not 1.4999…"""
    kmh = Quantity(5.4, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0))

    assert kmh.in_unit(unit("SI::m/SI::s", METRE_PER_SECOND)) == 1.5
    assert kmh.to(unit("SI::m/SI::s", METRE_PER_SECOND)).magnitude == 1.5


def test_incommensurable_ordering_is_an_error_not_a_comparison():
    """Ordering quantities that measure different things has no answer."""
    mass = Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0))
    length = Quantity(2.0, unit("SI::m"))

    with pytest.raises(IncommensurableUnitsError) as excinfo:
        mass < length
    assert "SI::kg" in str(excinfo.value) and "SI::m" in str(excinfo.value)

    for compare in (lambda: mass > length, lambda: mass <= length, lambda: mass >= length):
        with pytest.raises(IncommensurableUnitsError):
            compare()

    # Equality is not an error: two values measuring different things are simply
    # not the same value. What it must never do is compare bare magnitudes.
    assert mass != length
    assert Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0)) != Quantity(5.0, unit("SI::m"))


def test_a_quantity_does_not_order_against_a_bare_number():
    """A bare number carries no unit, so it is not comparable with a quantity."""
    with pytest.raises(TypeError, match="carries no unit"):
        Quantity(5.0, unit("SI::m")) < 5.0
    assert Quantity(5.0, unit("SI::m")) != 5.0


def test_addition_answers_in_the_left_unit_and_rejects_incommensurable_units():
    """A sum converts the right operand, and has no answer across dimensions."""
    metres = Quantity(1.0, unit("SI::m"))
    kilometres = Quantity(1.0, unit("SI::km", METRE, scale_num=1000.0))

    assert metres + kilometres == Quantity(1001.0, unit("SI::m"))
    assert (metres + kilometres).unit.text == "SI::m"
    assert kilometres - metres == Quantity(0.999, unit("SI::km", METRE, scale_num=1000.0))

    with pytest.raises(IncommensurableUnitsError, match="add"):
        metres + Quantity(1.0, unit("SI::s", SECOND))


def test_scaling_keeps_the_unit_and_negation_the_sign():
    """A quantity times a number is the same measurement, scaled."""
    speed = Quantity(1.5, unit("SI::m/SI::s", METRE_PER_SECOND))

    assert speed * 2 == Quantity(3.0, unit("SI::m/SI::s", METRE_PER_SECOND))
    assert 2 * speed == speed * 2
    assert speed / 2 == Quantity(0.75, unit("SI::m/SI::s", METRE_PER_SECOND))
    assert -speed == Quantity(-1.5, unit("SI::m/SI::s", METRE_PER_SECOND))
    assert abs(-speed) == speed
    with pytest.raises(TypeError):
        speed * Quantity(2.0, unit("SI::m"))


def test_a_unit_renders_as_written_and_reduced():
    """A unit reads as the model wrote it; its reduction names base units."""
    kmh = unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0)

    assert str(kmh) == "SI::km/SI::h"
    assert kmh.reduction() == "5/18·SI::metre·SI::second^-1"
    assert str(unit("", ())) == "1"
    assert unit("", ()).dimensionless
    assert unit("SI::m").reduction() == "SI::metre"


def test_a_slot_that_failed_is_still_an_error():
    """A quantity slot the service could not evaluate reports, not returns."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="P::Car",
        slots={
            "m": sysml_pb2.SlotValue(
                feature_name="m", materialized=True, error="incommensurable units"
            )
        },
    )

    with pytest.raises(SlotError, match="incommensurable units"):
        Instance(pb_inst).m


QUANTITY_MODEL = """
package Q {
    part def Engine {
        attribute power : ISQ::PowerValue = 300.0 [SI::W];
    }

    part def Car {
        attribute mass : ISQ::MassValue = 1200.0 [SI::kg];
        attribute plainMass : ScalarValues::Real = 2.0;
        attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];
        attribute writtenSpeed = 5.4 [SI::km/SI::h];
        attribute length = 3 [SI::m];
        part engine : Engine;

        constraint withinLimit {
            mass <= 2000.0 [SI::kg]
        }
    }
}
"""


@pytest.mark.integration
class TestQuantityAgainstTheService:
    """What a caller actually gets back from the real service for a real model."""

    def setup_method(self):
        import grpc

        from pysysml import Connection

        try:
            self.conn = Connection(auto_start=False)
            self.conn._stub.GetDiagnostics(sysml_pb2.DiagnosticsRequest(model_hash=""))
        except grpc.RpcError as exc:
            if exc.code() != grpc.StatusCode.NOT_FOUND:
                self.conn = None
                pytest.skip("sysml-grpc service not running")
        except Exception:
            self.conn = None
            pytest.skip("sysml-grpc service not running")
        self.model = self.conn.load_from_content(QUANTITY_MODEL)

    def teardown_method(self):
        if getattr(self, "conn", None) is not None:
            self.conn.close()

    def test_written_derived_and_compound_unit_slots_read_as_quantities(self):
        """Every shape of quantity slot reads back in the unit it was written in."""
        car = self.conn.instantiate("Q::Car", self.model.hash)

        assert car.mass == Quantity(1200.0, unit("SI::kg", GRAM, scale_num=1000.0))
        assert car.plainMass == 2.0
        assert str(car.derivedSpeed) == "5 [SI::m/SI::s]"
        assert str(car.writtenSpeed) == "5.4 [SI::km/SI::h]"
        assert car.length.magnitude == 3 and isinstance(car.length.magnitude, int)

        # A compound unit compares across its scale rather than by magnitude.
        assert car.writtenSpeed == Quantity(1.5, car.derivedSpeed.unit)
        with pytest.raises(IncommensurableUnitsError):
            car.mass < car.writtenSpeed

    def test_a_nested_object_carries_its_own_quantity(self):
        """`inst.engine.power` is a quantity, not an unreadable slot."""
        car = self.conn.instantiate("Q::Car", self.model.hash)

        assert str(car.engine.power) == "300 [SI::W]"
        assert car.engine.power.unit.exponents() == {
            "SI::gram": 1.0,
            "SI::metre": 2.0,
            "SI::second": -3.0,
        }

    def test_an_evaluated_quantity_expression_reads_as_a_quantity(self):
        """eval() of a quantity expression answers with the unit it composed."""
        speed = self.conn.eval("10.0 [SI::m] / 4.0 [SI::s]", self.model.hash)

        assert speed == Quantity(2.5, speed.unit)
        assert speed.unit.text == "SI::m/SI::s"

    def test_a_verdict_over_quantities_carries_the_quantity_it_is_about(self):
        """A constraint over quantities holds, and its subject's slot is readable."""
        verdict = self.conn.verify_constraint(
            "Q::Car::withinLimit", self.model.hash, subject_symbol_id="Q::Car"
        )

        assert verdict.holds
        subject = next(i for i in verdict.instances if i.id == verdict.instance_id)
        assert subject.mass == Quantity(1200.0, unit("SI::kg", GRAM, scale_num=1000.0))
