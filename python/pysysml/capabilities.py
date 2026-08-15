"""What a connected sysml-grpc service can do, and how a client requires it.

The service is released separately from this client, so a client that needs a
feature must ask for it rather than infer it. It asks by *capability name*, not
by version: version strings of forks and development builds are not ordered
against each other (a source build reports ``dev``), while a capability name is
an exact contract the service either reports or does not.

A service too old to answer ``GetServerInfo`` fails the call with
``UNIMPLEMENTED``, which is itself an answer — :data:`ServerInfo.answered` is
then ``False``, and no capability is claimed.
"""

from dataclasses import dataclass
from typing import FrozenSet

from pysysml.errors import PySysMLError

#: Static type facts on ``SymbolInfo`` — ``type_info``, ``multiplicity`` and
#: ``specializations``. Typed code generation requires this: without it every
#: feature looks untyped, which is indistinguishable from a feature that is
#: genuinely untyped.
CAPABILITY_TYPE_FACTS = "type_facts"

#: The ``Convert`` RPC, which writes a model back out as SysML notation or RDF
#: Turtle. Without it a conversion request fails as an unimplemented method,
#: which is indistinguishable from a broken channel.
CAPABILITY_CONVERT = "convert"

#: The verification RPCs — ``VerifyConstraint``, ``VerifyRequirement``,
#: ``VerifySatisfaction`` and ``EvaluateCalc`` — which answer the questions the
#: REPL's ``%constraint``, ``%requirement``, ``%satisfy`` and ``%calc`` answer.
#: Without it those calls fail as unimplemented methods, which is
#: indistinguishable from a broken channel.
CAPABILITY_VERIFICATION = "verification"

#: The ``Query`` RPC, which evaluates a SysML v2 API & Services ``Query`` over a
#: loaded model. Without it a query fails as an unimplemented method, which is
#: indistinguishable from a broken channel.
CAPABILITY_QUERY = "query"

@dataclass(frozen=True)
class ServerInfo:
    """Self-description of the service a :class:`~pysysml.connection.Connection` talks to.

    Attributes:
        version: Build version the service reports, informational only. Empty
            when the service is too old to answer.
        capabilities: Capability names the service reports.
        answered: Whether the service answered the handshake at all. ``False``
            means it predates ``GetServerInfo``.
        origin: Human-readable provenance of the service — the binary path this
            client started, or the address it connected to. Used to name the
            offending binary in an error message.
    """

    version: str
    capabilities: FrozenSet[str]
    answered: bool
    origin: str

    def has(self, capability: str) -> bool:
        """Whether the service reports ``capability``."""
        return capability in self.capabilities

    def describe(self) -> str:
        """One-line description of the service, for an error message."""
        version = self.version or "unknown"
        if not self.answered:
            return (
                f"{self.origin} (version unknown: too old to answer GetServerInfo, "
                f"so it predates every capability)"
            )
        reported = ", ".join(sorted(self.capabilities)) or "none"
        return f"{self.origin} (version {version}, capabilities: {reported})"


class MissingCapabilityError(PySysMLError):
    """Raised when the connected service cannot supply a required capability.

    Attributes:
        capability (str): Capability name that was required
        info (ServerInfo): What the service reported about itself
    """

    def __init__(self, capability: str, info: ServerInfo, remedy: str):
        super().__init__(
            f"the sysml-grpc service does not support the {capability!r} capability, "
            f"which this operation requires.\n"
            f"  service: {info.describe()}\n"
            f"  fix:     {remedy}"
        )
        self.capability = capability
        self.info = info


def require(info: ServerInfo, capability: str, remedy: str) -> None:
    """Raise :class:`MissingCapabilityError` unless ``info`` reports ``capability``."""
    if not info.has(capability):
        raise MissingCapabilityError(capability, info, remedy)
