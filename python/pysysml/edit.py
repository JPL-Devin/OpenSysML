"""Changing a loaded model and writing it back, with its layout intact.

An edit is described, not typed out: an :class:`Editor` collects operations
naming elements the way a read result names them, and :meth:`Editor.apply` has
the service perform them on the source it parsed. The service edits the bytes the
operations reach and nothing else, so comments, blank lines and indentation
outside an edited span come back unchanged, and it re-parses what it edited
before returning it.

Two operations exist: setting a feature's value, and renaming a declaration.
Renaming rewrites the declaration's name token only and is refused for an element
that is referenced — see :class:`~pysysml.errors.RenameReferencedError`.
"""

from dataclasses import dataclass, field
from typing import List

from pysysml.conversion import Conversion, FORMAT_SYSML
from pysysml.proto import sysml_pb2
from pysysml.errors import (
    EditError,
    EditResultError,
    EditTargetError,
    InvalidEditError,
    NoEditsError,
    OverlappingEditsError,
    RenameReferencedError,
)

#: Refusal kinds, as the wire enum names them, and the error each raises. A kind
#: this client has not seen raises the base :class:`EditError`, so no refusal
#: escapes the hierarchy.
_FAILURE_ERRORS = {
    "EDIT_FAILURE_NO_OPERATIONS": NoEditsError,
    "EDIT_FAILURE_UNKNOWN_TARGET": EditTargetError,
    "EDIT_FAILURE_AMBIGUOUS_TARGET": EditTargetError,
    "EDIT_FAILURE_NOT_VALUED": EditTargetError,
    "EDIT_FAILURE_NOT_NAMED": EditTargetError,
    "EDIT_FAILURE_INVALID_VALUE": InvalidEditError,
    "EDIT_FAILURE_INVALID_NAME": InvalidEditError,
    "EDIT_FAILURE_RENAME_REFERENCED": RenameReferencedError,
    "EDIT_FAILURE_OVERLAPPING_EDITS": OverlappingEditsError,
    "EDIT_FAILURE_RESULT_INVALID": EditResultError,
}


def failure_name(failure):
    """Name a refusal kind, including one this client's enum has no name for.

    proto3 enums are open, so a newer service can refuse an edit for a reason
    this build has never heard of; that must still raise an :class:`EditError`
    rather than fail on the enum lookup.

    Args:
        failure (int): EditFailure number as the service sent it

    Returns:
        str: The enum's name, or a name naming the unknown number
    """
    try:
        return sysml_pb2.EditFailure.Name(failure)
    except ValueError:
        return f"EDIT_FAILURE_{failure}"


def error_for_failure(failure, message, diagnostics=None, referring_elements=None):
    """Build the error a refusal kind names.

    Args:
        failure (str): Refusal kind, as the wire enum names it
        message (str): Why the edit was refused
        diagnostics (list, optional): Diagnostics behind the refusal
        referring_elements (list, optional): Referrers of a refused rename

    Returns:
        EditError: The typed refusal, ready to raise
    """
    cls = _FAILURE_ERRORS.get(failure, EditError)
    return cls(
        message,
        failure=failure,
        diagnostics=diagnostics,
        referring_elements=referring_elements,
    )


@dataclass(frozen=True)
class AppliedEdit:
    """One byte range of the original source that an operation replaced.

    Attributes:
        operation_index: Position of the operation in the editor, so an applied
            edit is matched back to what asked for it.
        target: Element edited, by the id it was named with.
        offset: Byte offset in the original source where the replacement starts.
        length: Number of bytes replaced. Zero for a value added to a feature
            that had none: nothing was replaced, text was inserted.
        old_text: The bytes that were there.
        new_text: What replaced them.
    """

    operation_index: int
    target: str
    offset: int
    length: int
    old_text: str
    new_text: str

    def __str__(self):
        return f"{self.target}: {self.old_text!r} -> {self.new_text!r}"


@dataclass(frozen=True)
class EditResult(Conversion):
    """The edited notation, as a :class:`~pysysml.conversion.Conversion`.

    ``str(result)`` is the edited text and ``result.save(path)`` writes it, so an
    edit is written the way a conversion is.

    Attributes:
        applied: What each operation changed, in source order.
    """

    applied: List[AppliedEdit] = field(default_factory=list)

    def save(self, path):
        """Write the edited model to ``path``.

        Args:
            path (str): File to write, created or truncated

        Returns:
            str: The path written, for chaining
        """
        return self.write(path)


class Editor:
    """Operations to perform on a loaded model, and the call that performs them.

    Collected client-side and applied in one call, so the service edits and
    validates the model once. Every operation names its element by the id a read
    reports (:attr:`Symbol.id`), or by the :class:`~pysysml.symbol.Symbol` itself.

    An editor is applied once: it describes an edit of the model it was made
    from, and the edited model is a different model. Build another editor from
    the reloaded model to edit again.

    Example:
        >>> edit = model.edit()
        >>> edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
        >>> edit.apply().save("spacecraft.sysml")
        'spacecraft.sysml'
    """

    def __init__(self, model_hash, connection):
        """Initialize an editor over a loaded model.

        Args:
            model_hash (str): Hash of the model to edit
            connection: Connection the model was loaded over
        """
        self._model_hash = model_hash
        self._connection = connection
        self._operations = []
        self._applied = False

    @property
    def operations(self):
        """The operations collected so far, in the order they were added."""
        return list(self._operations)

    @property
    def applied(self):
        """Whether this editor has been applied."""
        return self._applied

    def __len__(self):
        return len(self._operations)

    def __bool__(self):
        # An editor with no operations is falsy, so `if edit:` asks what it reads
        # as: whether there is anything to apply.
        return bool(self._operations)

    def set_value(self, target, value):
        """Set the value expression of one of the model's features.

        Replaces an existing ``= <expr>``, or adds one before the terminating
        semicolon when the feature has none.

        Args:
            target (str or Symbol): Feature to edit, by FQN/id or symbol
            value (str): The new value, as SysML notation for one expression,
                e.g. ``"1050.0[SI::kg]"``, ``'"flight-2"'`` or ``"unitMass * 2"``

        Returns:
            Editor: self, so operations can be chained

        Raises:
            TypeError: If value is not a string: the notation is what is sent,
                and guessing notation for a Python object would guess its type
        """
        if not isinstance(value, str):
            raise TypeError(
                f"value must be SysML notation for an expression, not "
                f"{type(value).__name__}: write it as it should read in the file"
            )
        self._add(("set_value", _target_id(target), value))
        return self

    def rename(self, target, new_name):
        """Rename one of the model's declarations.

        Rewrites the declaration's name token only. References to the element are
        not updated, so a referenced element cannot be renamed this way: the
        service refuses it with :class:`~pysysml.errors.RenameReferencedError`,
        naming where the references are made.

        Args:
            target (str or Symbol): Declaration to rename, by FQN/id or symbol
            new_name (str): The new name, as it should read in the file

        Returns:
            Editor: self, so operations can be chained

        Raises:
            TypeError: If new_name is not a string
        """
        if not isinstance(new_name, str):
            raise TypeError(
                f"new_name must be a name, not {type(new_name).__name__}"
            )
        self._add(("rename", _target_id(target), new_name))
        return self

    def apply(self):
        """Have the service perform these operations and return the edited model.

        The service edits the source it parsed, re-parses and re-analyses the
        result, and refuses to return content the parser could not read back.

        Returns:
            EditResult: The edited notation, and what each operation changed

        Raises:
            NoEditsError: If no operation was added
            EditError: If the service refused the edit; the subclass names why
            MissingCapabilityError: If the service cannot apply edits
            ModelNotFoundError: If the service no longer holds this model
            RuntimeError: If this editor was already applied
        """
        if self._applied:
            raise RuntimeError(
                "this editor has already been applied: it describes an edit of "
                "the model it was made from, so build another editor from the "
                "edited model rather than applying this one twice"
            )
        if not self._operations:
            raise NoEditsError(
                "this editor has no operations: add a set_value or rename before "
                "applying it",
                failure="EDIT_FAILURE_NO_OPERATIONS",
            )
        result = self._connection.apply_edits(self._model_hash, self._operations)
        self._applied = True
        return result

    def _add(self, operation):
        if self._applied:
            raise RuntimeError(
                "this editor has already been applied: build another editor from "
                "the edited model to edit further"
            )
        self._operations.append(operation)

    def __repr__(self):
        return (
            f"Editor(model_hash={self._model_hash!r}, "
            f"operations={len(self._operations)}, applied={self._applied})"
        )


def _target_id(target):
    """The id an operation names its element by, from an id or a Symbol."""
    if isinstance(target, str):
        return target
    ident = getattr(target, "id", None)
    if isinstance(ident, str) and ident:
        return ident
    raise TypeError(
        f"target must be a symbol id (FQN) or a Symbol, not "
        f"{type(target).__name__}"
    )


def result_of(response, applied_source=FORMAT_SYSML):
    """Read an ``ApplyEditsResponse`` as an :class:`EditResult`.

    Args:
        response: sysml_pb2.ApplyEditsResponse protobuf message
        applied_source (str): Format the content is written in

    Returns:
        EditResult: The edited notation and what changed
    """
    return EditResult(
        content=response.content,
        from_format=applied_source,
        to_format=applied_source,
        applied=[
            AppliedEdit(
                operation_index=a.operation_index,
                target=a.target,
                offset=a.offset,
                length=a.length,
                old_text=a.old_text,
                new_text=a.new_text,
            )
            for a in response.applied
        ],
    )
