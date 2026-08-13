"""Exception hierarchy for pysysml."""


class PySysMLError(Exception):
    """Base class for all pysysml errors."""


class ConnectionError(PySysMLError):
    """Raised when the client cannot connect to or start the sysml-grpc service."""


class UnsupportedValueError(PySysMLError):
    """Raised when the service sends a value the wire format cannot represent."""


class SlotError(PySysMLError):
    """Raised when a slot could not be evaluated or was never materialized.

    Attributes:
        feature_name (str): Name of the slot
        message (str): Error description reported by the service
    """

    def __init__(self, feature_name, message):
        super().__init__(f"slot {feature_name!r}: {message}")
        self.feature_name = feature_name
        self.message = message


class RuntimeError(PySysMLError):
    """Raised when a runtime operation (eval/instantiate/execute) fails.

    Attributes:
        message (str): Error description
        diagnostics (list): List of Diagnostic objects (if available)
    """

    def __init__(self, message, diagnostics=None):
        super().__init__(message)
        self.message = message
        self.diagnostics = diagnostics or []
