"""An enumeration literal as a Python value."""

from dataclasses import dataclass


@dataclass(frozen=True)
class EnumLiteral:
    """One literal of an enumeration definition.

    A literal is its own identity, so it arrives as the declaration it names
    rather than as a number or a string: two literals are the same exactly when
    their ``literal_id`` is. It is frozen so it can be a dict key or set member,
    as the same literal in a model is one value.

    Attributes:
        literal_id: FQN of the literal's declaration (``"D::Color::red"``)
        enumeration_id: FQN of the enumeration declaring it (``"D::Color"``)
        name: The literal as a reader writes it (``"Color::red"``)
    """

    literal_id: str
    enumeration_id: str = ""
    name: str = ""

    def __str__(self):
        return self.name or self.literal_id
