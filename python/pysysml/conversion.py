"""Writing a model back out, in SysML notation or RDF Turtle.

The service converts; this module is the client's side of it. The formats are
the two Systemica can write, named as the ``sysml`` CLI's ``-from``/``-to``
name them, so a script and a command line agree.

A round trip is defined on the model, not on the bytes: notation written back
out is re-indented from the original source, and a trip through Turtle returns
an equivalent model rather than identical text. ``docs/RDF_INTEROP.md`` states
what survives.
"""

import os
from dataclasses import dataclass, field
from typing import List

#: SysML v2 / KerML textual notation. ``kerml`` and ``text`` name it too.
FORMAT_SYSML = "sysml"
#: RDF in Turtle syntax. ``turtle`` and ``rdf`` name it too.
FORMAT_TURTLE = "ttl"

#: Formats this client will ask for, and the file extensions they are written as.
_EXTENSIONS = {
    ".sysml": FORMAT_SYSML,
    ".kerml": FORMAT_SYSML,
    ".ttl": FORMAT_TURTLE,
}


def format_of_path(path):
    """Infer the format to write ``path`` as, from its extension.

    Args:
        path (str): File name or path.

    Returns:
        str: Format name, as :func:`~pysysml.connection.Connection.convert` takes it.

    Raises:
        ValueError: If the extension names no format this client can write.
    """
    ext = os.path.splitext(path)[1].lower()
    try:
        return _EXTENSIONS[ext]
    except KeyError:
        known = ", ".join(sorted(_EXTENSIONS))
        raise ValueError(
            f"cannot tell the format to write {path!r} as: expected one of {known}, "
            f"or pass to_format explicitly"
        ) from None


@dataclass(frozen=True)
class Conversion:
    """A model written out in one of the formats Systemica writes.

    Attributes:
        content: The converted model. ``str(conversion)`` is this text, so a
            conversion can be written or compared directly.
        from_format: Format the source was read as. Reported even when it was
            inferred, so a caller learns what the inference decided.
        to_format: Format ``content`` is written in.
        diagnostics: Syntax errors the service tolerated under
            ``tolerate_syntax_errors``. Empty otherwise: a conversion that
            failed raises instead.
    """

    content: str
    from_format: str
    to_format: str
    diagnostics: List[object] = field(default_factory=list)

    def __str__(self):
        return self.content

    def __len__(self):
        return len(self.content)

    def write(self, path):
        """Write the converted model to ``path``.

        Args:
            path (str): File to write, created or truncated.

        Returns:
            str: The path written, for chaining.
        """
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(self.content)
        return path
