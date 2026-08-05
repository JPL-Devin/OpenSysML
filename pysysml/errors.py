"""Exception hierarchy for pysysml."""


class PySysMLError(Exception):
    """Base class for all pysysml errors."""


class ConnectionError(PySysMLError):
    """Raised when the client cannot connect to or start the sysml-grpc service."""


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
