"""Runtime support for generated, statically typed views over instances.

A generated class derives from :class:`TypedObject` and exposes one property per
SysML feature, each delegating to the underlying Tier 1 :class:`~pysysml.instance.Instance`
through :func:`slot`, :func:`optional_slot` or :func:`list_slot`. Slot decoding,
including its ``SlotError`` behaviour, is unchanged: this layer only states what
type a decoded value is expected to have, and reports a mismatch rather than
returning a wrongly typed value.
"""

from typing import Callable, ClassVar, List, Optional, Set, TypeVar

from pysysml.errors import InstanceTypeError, TypeMismatchError
from pysysml.instance import Instance

T = TypeVar("T")
TypedObjectT = TypeVar("TypedObjectT", bound="TypedObject")

# FQNs of every definition a generated class exists for in this process, so a
# wrong type is distinguishable from one this client has never heard of.
_generated_ids: Set[str] = set()


class TypedObject:
    """Base class of generated typed views over an :class:`Instance`."""

    sysml_id: ClassVar[str] = ""
    """FQN of the SysML definition this class was generated from."""

    __slots__ = ("_instance",)

    def __init__(self, instance: Instance) -> None:
        self._instance = instance

    def __init_subclass__(cls, **kwargs: object) -> None:
        super().__init_subclass__(**kwargs)
        if cls.sysml_id:
            _generated_ids.add(cls.sysml_id)

    @classmethod
    def from_instance(cls: "type[TypedObjectT]", instance: Instance) -> TypedObjectT:
        """Return a typed view over ``instance``, rejecting one of another type.

        Accepted:

        * an instance of exactly this class's definition;
        * an instance of a definition that specializes it and has a generated
          class of its own — legitimate polymorphism, recognized because
          generation emits SysML specialization as Python inheritance;
        * an instance whose type no generated class in this process describes.
          The service reports the type of an instantiated *usage* as the usage's
          own FQN (``Demo::myCar``, not ``Demo::SportsCar``), which no generated
          class carries, so the client cannot relate it to a definition at all.
          Rejecting it would break instantiating a usage, which is the ordinary
          way to get an instance, so an unrecognized type is accepted — the
          per-slot decoding in this module still reports a wrong shape.

        Rejected: an instance whose type *is* described by a generated class that
        is not this class or a subclass of it.

        Use :meth:`unchecked` to bypass this deliberately.

        Raises:
            InstanceTypeError: If the instance's type is known to be another one.
        """
        actual = instance.type_symbol_id
        if actual and actual != cls.sysml_id and actual in _generated_ids:
            if not any(sub.sysml_id == actual for sub in _subclasses(cls)):
                raise InstanceTypeError(cls.sysml_id or cls.__name__, actual)
        return cls(instance)

    @classmethod
    def unchecked(cls: "type[TypedObjectT]", instance: Instance) -> TypedObjectT:
        """Return a typed view over ``instance`` without checking its type.

        For a caller who knows better than the reported type — reading the slots
        of a partially materialized instance, for instance. Accessing a property
        the instance does not have still raises from the slot decoder.
        """
        return cls(instance)

    @property
    def instance(self) -> Instance:
        """Return the underlying Tier 1 instance."""
        return self._instance

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, TypedObject):
            return NotImplemented
        return type(self) is type(other) and self._instance.id == other._instance.id

    def __hash__(self) -> int:
        return hash((type(self), self._instance.id))

    def __repr__(self) -> str:
        return f"{type(self).__name__}(instance={self._instance!r})"


def _subclasses(cls: "type[TypedObject]") -> List["type[TypedObject]"]:
    """Every class deriving from ``cls``, transitively."""
    found: List["type[TypedObject]"] = []
    pending = list(cls.__subclasses__())
    while pending:
        subclass = pending.pop()
        found.append(subclass)
        pending.extend(subclass.__subclasses__())
    return found


def _mismatch(feature_name: str, expected: str, value: object) -> TypeMismatchError:
    return TypeMismatchError(feature_name, expected, value)


def as_bool(feature_name: str, value: object) -> bool:
    """Decode a Boolean slot value."""
    if isinstance(value, bool):
        return value
    raise _mismatch(feature_name, "bool", value)


def as_int(feature_name: str, value: object) -> int:
    """Decode an Integer/Natural slot value."""
    if isinstance(value, bool) or not isinstance(value, int):
        raise _mismatch(feature_name, "int", value)
    return value


def as_float(feature_name: str, value: object) -> float:
    """Decode a Real/Rational slot value; an integer value widens to float."""
    if isinstance(value, bool):
        raise _mismatch(feature_name, "float", value)
    if isinstance(value, float):
        return value
    if isinstance(value, int):
        return float(value)
    raise _mismatch(feature_name, "float", value)


def as_str(feature_name: str, value: object) -> str:
    """Decode a String slot value."""
    if isinstance(value, str):
        return value
    raise _mismatch(feature_name, "str", value)


def as_object(feature_name: str, value: object) -> object:
    """Decode a slot whose SysML type has no sound Python type."""
    return value


def as_typed(cls: "type[TypedObjectT]") -> Callable[[str, object], TypedObjectT]:
    """Return a decoder wrapping a nested instance in the generated class ``cls``."""

    def decode(feature_name: str, value: object) -> TypedObjectT:
        if isinstance(value, Instance):
            return cls.from_instance(value)
        raise _mismatch(feature_name, cls.__name__, value)

    return decode


def slot(obj: TypedObject, feature_name: str, decode: Callable[[str, object], T]) -> T:
    """Return the decoded value of a required single-valued slot.

    Raises:
        SlotError: If the slot failed to evaluate or was never materialized.
        TypeMismatchError: If the slot is absent or holds another type.
    """
    value = _require(obj, feature_name)
    if isinstance(value, list):
        raise _mismatch(feature_name, "a single value", value)
    return decode(feature_name, value)


def optional_slot(
    obj: TypedObject, feature_name: str, decode: Callable[[str, object], T]
) -> Optional[T]:
    """Return the decoded value of a ``0..1`` slot, or None when it holds no value."""
    instance = obj.instance
    if feature_name not in instance:
        return None
    value = instance[feature_name]
    if value is None:
        return None
    if isinstance(value, list):
        if not value:
            return None
        if len(value) > 1:
            raise _mismatch(feature_name, "at most one value", value)
        return decode(feature_name, value[0])
    return decode(feature_name, value)


def list_slot(
    obj: TypedObject, feature_name: str, decode: Callable[[str, object], T]
) -> List[T]:
    """Return the decoded values of a multi-valued slot; an absent or null slot is empty."""
    instance = obj.instance
    if feature_name not in instance:
        return []
    value = instance[feature_name]
    if value is None:
        return []
    if not isinstance(value, list):
        return [decode(feature_name, value)]
    return [decode(feature_name, element) for element in value]


def _require(obj: TypedObject, feature_name: str) -> object:
    instance = obj.instance
    if feature_name not in instance:
        raise _mismatch(feature_name, "a value", None)
    value = instance[feature_name]
    if value is None:
        raise _mismatch(feature_name, "a value", None)
    return value
