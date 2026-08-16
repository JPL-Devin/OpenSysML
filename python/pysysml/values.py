"""Conversion of protobuf Value/SlotValue messages into Python values."""

from dataclasses import dataclass
from typing import Dict, Tuple, Union

from pysysml.enumeration import EnumLiteral
from pysysml.errors import PySysMLError, SlotError, UnsupportedValueError

#: What a quantity's magnitude can be: the service keeps Integer and Real apart.
Magnitude = Union[int, float]


class IncommensurableUnitsError(PySysMLError):
    """Raised when quantities measuring different things are compared or combined.

    ``5.0 [kg] < 2.0 [m]`` relates no two magnitudes, so it is an error rather
    than an answer — the same answer the runtime gives the model.

    Attributes:
        left (Unit): Unit of the left operand
        right (Unit): Unit of the right operand
    """

    def __init__(self, operation, left, right):
        super().__init__(
            f"cannot {operation} a quantity in [{left}] and one in [{right}]: "
            f"{left} reduces to {left.reduction()} and {right} to {right.reduction()}, "
            f"which measure different things"
        )
        self.left = left
        self.right = right


@dataclass(frozen=True)
class UnitFactor:
    """One base unit of a unit's reduction, raised to an exponent.

    Attributes:
        unit_id (str): FQN of the base unit, e.g. ``SI::m``
        exponent (float): Power it is raised to
    """

    unit_id: str
    exponent: float


@dataclass(frozen=True)
class Unit:
    """A measurement reference as a quantity carries it.

    ``text`` is the unit as the model wrote it, for reading; the reduction —
    ``scale_num``/``scale_den`` over ``factors`` — is what decides whether two
    quantities are commensurable and by what factor one converts into the other.
    ``km/h`` is the text ``"km/h"`` reduced to 1000/3600 over ``SI::m`` and
    ``SI::s^-1``.

    Attributes:
        text (str): Unit as written; empty for one never written down
        scale_num (float): Numerator of the scale factor over the base units
        scale_den (float): Denominator of that scale factor
        factors (tuple[UnitFactor, ...]): The base units it reduces to
    """

    text: str = ""
    scale_num: float = 1.0
    scale_den: float = 1.0
    factors: Tuple[UnitFactor, ...] = ()

    @classmethod
    def from_pb(cls, text, pb_unit_term=None) -> "Unit":
        """Build from the ``unit``/``unit_term`` fields of a ``Quantity`` message."""
        if pb_unit_term is None:
            return cls(text=text)
        return cls(
            text=text,
            scale_num=pb_unit_term.scale_num,
            scale_den=pb_unit_term.scale_den,
            factors=tuple(
                UnitFactor(factor.unit_id, factor.exponent)
                for factor in pb_unit_term.factors
            ),
        )

    @property
    def dimensionless(self) -> bool:
        """Whether the unit reduces to no base unit, as a count or a ratio does."""
        return not self.exponents()

    def exponents(self) -> Dict[str, float]:
        """The reduction as base unit → exponent, order-independent: repeated
        base units summed and cancelled ones dropped, as the service reduces."""
        totals: Dict[str, float] = {}
        for factor in self.factors:
            totals[factor.unit_id] = totals.get(factor.unit_id, 0.0) + factor.exponent
        return {unit_id: exp for unit_id, exp in totals.items() if exp != 0}

    def commensurable(self, other: "Unit") -> bool:
        """Whether a magnitude in this unit converts into ``other``."""
        return self.exponents() == other.exponents()

    def reduction(self) -> str:
        """The reduction as text ("1000/3600·SI::m·SI::s^-1"), for a diagnostic."""
        parts = []
        if (self.scale_num, self.scale_den) != (1.0, 1.0):
            scale = (
                f"{self.scale_num:g}"
                if self.scale_den == 1.0
                else f"{self.scale_num:g}/{self.scale_den:g}"
            )
            parts.append(scale)
        for factor in self.factors:
            rendered = factor.unit_id
            if factor.exponent != 1.0:
                rendered += f"^{factor.exponent:g}"
            parts.append(rendered)
        return "·".join(parts) if parts else "1"

    def __str__(self) -> str:
        return self.text or self.reduction()


class Quantity:
    """A magnitude and the measurement reference it is expressed in.

    This is what a quantity-valued slot holds: ``inst.mass`` of
    ``attribute mass : ISQ::MassValue = 5.0 [SI::kg]`` is ``Quantity(5.0,
    Unit("kg"))``. The magnitude is the one the model wrote, in the unit it wrote
    it in — nothing is reduced to base units behind the caller's back — while
    comparisons and arithmetic go through the reduction, so ``1 [km]`` equals
    ``1000 [m]``.

    Equality holds between commensurable quantities of equal magnitude, and is
    false between quantities measuring different things: two such values are
    simply not the same value. Ordering and addition, which have no answer at
    all across incommensurable units, raise
    :class:`IncommensurableUnitsError` rather than compare bare magnitudes.

    Attributes:
        magnitude (int | float): The magnitude, in :attr:`unit`
        unit (Unit): The unit it is expressed in
    """

    __slots__ = ("magnitude", "unit")

    def __init__(self, magnitude: Magnitude, unit: Unit) -> None:
        self.magnitude = magnitude
        self.unit = unit

    @classmethod
    def from_pb(cls, pb_quantity) -> "Quantity":
        """Build from a ``Quantity`` protobuf message.

        Raises:
            UnsupportedValueError: If the message carries no magnitude, or names a
                unit without the reduction commensurability is decided over.
        """
        which = pb_quantity.WhichOneof('magnitude')
        if which == 'int_magnitude':
            magnitude: Magnitude = pb_quantity.int_magnitude
        elif which == 'real_magnitude':
            magnitude = pb_quantity.real_magnitude
        else:
            raise UnsupportedValueError(
                f"quantity in [{pb_quantity.unit}] carries no magnitude"
            )
        if not pb_quantity.HasField('unit_term'):
            # Dimension one is what an unnamed unit means, not what an unreduced
            # one means; the service rejects the latter and so does the client.
            if pb_quantity.unit:
                raise UnsupportedValueError(
                    f"quantity in [{pb_quantity.unit}] carries no reduction to base units"
                )
            return cls(magnitude, Unit())
        return cls(magnitude, Unit.from_pb(pb_quantity.unit, pb_quantity.unit_term))

    def base_magnitude(self) -> float:
        """The magnitude over the base units the unit reduces to."""
        return self.magnitude * self.unit.scale_num / self.unit.scale_den

    def in_unit(self, unit: Unit) -> float:
        """The magnitude expressed in ``unit``.

        The conversion is applied as one ratio, so an exact factor stays exact:
        ``5.4 [km/h]`` in ``m/s`` is 1.5, not 1.4999999999999998.

        Raises:
            IncommensurableUnitsError: If ``unit`` measures something else.
            ZeroDivisionError: If ``unit`` reduces to a zero scale factor.
        """
        if not self.unit.commensurable(unit):
            raise IncommensurableUnitsError("express", self.unit, unit)
        return (
            self.magnitude
            * (self.unit.scale_num * unit.scale_den)
            / (self.unit.scale_den * unit.scale_num)
        )

    def to(self, unit: Unit) -> "Quantity":
        """This quantity expressed in ``unit``, magnitude converted."""
        return Quantity(self.in_unit(unit), unit)

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Quantity):
            return NotImplemented
        if not self.unit.commensurable(other.unit):
            return False
        return self.base_magnitude() == other.base_magnitude()

    def __hash__(self) -> int:
        # Keyed on the base-unit form, so commensurable equal quantities — `1
        # [km]` and `1000 [m]` — hash alike, as equality requires.
        return hash((self.base_magnitude(), tuple(sorted(self.unit.exponents().items()))))

    def __lt__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") < 0

    def __le__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") <= 0

    def __gt__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") > 0

    def __ge__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") >= 0

    def _compare(self, other: "Quantity", operation: str) -> float:
        """Order two quantities over their base units, as the runtime does."""
        if not isinstance(other, Quantity):
            raise TypeError(
                f"cannot {operation} a quantity and {type(other).__name__}: "
                f"a bare number carries no unit"
            )
        if not self.unit.commensurable(other.unit):
            raise IncommensurableUnitsError(operation, self.unit, other.unit)
        return self.base_magnitude() - other.base_magnitude()

    def __add__(self, other: "Quantity") -> "Quantity":
        return self._sum(other, 1)

    def __sub__(self, other: "Quantity") -> "Quantity":
        return self._sum(other, -1)

    def _sum(self, other: "Quantity", sign: float) -> "Quantity":
        """Add or subtract, in this quantity's unit, as the runtime does."""
        if not isinstance(other, Quantity):
            return NotImplemented
        if not self.unit.commensurable(other.unit):
            operation = "add" if sign > 0 else "subtract"
            raise IncommensurableUnitsError(operation, self.unit, other.unit)
        return Quantity(self.magnitude + sign * other.in_unit(self.unit), self.unit)

    def __neg__(self) -> "Quantity":
        return Quantity(-self.magnitude, self.unit)

    def __abs__(self) -> "Quantity":
        return Quantity(abs(self.magnitude), self.unit)

    def __mul__(self, other: Magnitude) -> "Quantity":
        """Scale the magnitude by a number, keeping the unit.

        A product of two quantities has a unit composed from theirs, which this
        client does not compose: the runtime does, so `x * y` over quantities
        belongs in the model.
        """
        if isinstance(other, bool) or not isinstance(other, (int, float)):
            return NotImplemented
        return Quantity(self.magnitude * other, self.unit)

    __rmul__ = __mul__

    def __truediv__(self, other: Magnitude) -> "Quantity":
        """Divide the magnitude by a number, keeping the unit."""
        if isinstance(other, bool) or not isinstance(other, (int, float)):
            return NotImplemented
        return Quantity(self.magnitude / other, self.unit)

    def __str__(self) -> str:
        magnitude = f"{self.magnitude:g}" if isinstance(self.magnitude, float) else str(self.magnitude)
        return f"{magnitude} [{self.unit}]"

    def __repr__(self) -> str:
        return f"Quantity({self.magnitude!r}, {self.unit!r})"


def value_to_python(pb_value, resolve_instance=None):
    """Convert a protobuf Value into a plain Python value.

    Args:
        pb_value: sysml_pb2.Value message
        resolve_instance: optional callable mapping an instance id to an object;
            when omitted, instance references are returned as their integer id.

    Returns:
        int, float, bool, str, list, None, a :class:`Quantity`, an
        :class:`~pysysml.enumeration.EnumLiteral`, or the resolved instance object.

    Raises:
        UnsupportedValueError: If the service reported the value as unsupported.
    """
    kind = pb_value.WhichOneof('kind')
    if kind == 'int_value':
        return pb_value.int_value
    if kind == 'real_value':
        return pb_value.real_value
    if kind == 'bool_value':
        return pb_value.bool_value
    if kind == 'string_value':
        return pb_value.string_value
    if kind == 'quantity':
        return Quantity.from_pb(pb_value.quantity)
    if kind == 'instance_id':
        if resolve_instance is None:
            return pb_value.instance_id
        return resolve_instance(pb_value.instance_id)
    if kind == 'sequence':
        return [value_to_python(v, resolve_instance) for v in pb_value.sequence.elements]
    if kind == 'enum_literal':
        lit = pb_value.enum_literal
        return EnumLiteral(lit.literal_id, lit.enumeration_id, lit.name)
    if kind == 'null':
        # A non-empty null carries the reason the value could not be sent.
        if pb_value.null:
            raise UnsupportedValueError(pb_value.null)
        return None
    return None


def slot_to_python(feature_name, pb_slot, resolve_instance=None):
    """Convert a protobuf SlotValue into a plain Python value.

    Args:
        feature_name (str): Slot name, used for error reporting
        pb_slot: sysml_pb2.SlotValue message
        resolve_instance: optional callable mapping an instance id to an object

    Returns:
        The scalar value, or a list for collection slots.

    Raises:
        SlotError: If evaluation failed or the slot was never materialized.
    """
    if pb_slot.error:
        raise SlotError(feature_name, pb_slot.error)

    if pb_slot.HasField('value'):
        try:
            return value_to_python(pb_slot.value, resolve_instance)
        except UnsupportedValueError as exc:
            raise SlotError(feature_name, str(exc)) from exc

    if pb_slot.values:
        try:
            return [value_to_python(v, resolve_instance) for v in pb_slot.values]
        except UnsupportedValueError as exc:
            raise SlotError(feature_name, str(exc)) from exc

    if pb_slot.materialized:
        return []

    raise SlotError(feature_name, "slot is not materialized")
