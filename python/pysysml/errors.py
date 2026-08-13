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


class TypeMismatchError(PySysMLError):
    """Raised when a slot holds a value of another type than its generated view declares.

    Attributes:
        feature_name (str): Name of the slot
        expected (str): Type the generated class declares
        value: The value actually decoded from the slot
    """

    def __init__(self, feature_name, expected, value):
        super().__init__(
            f"slot {feature_name!r}: expected {expected}, got {value!r}"
        )
        self.feature_name = feature_name
        self.expected = expected
        self.value = value


class InstanceTypeError(PySysMLError):
    """Raised when a generated typed view is asked to wrap an instance of another type.

    Attributes:
        expected (str): FQN of the definition the generated class views
        actual (str): FQN the instance reports as its type
    """

    def __init__(self, expected, actual):
        super().__init__(
            f"instance of {actual!r} is not a {expected!r}; call the generated "
            f"class's unchecked(instance) to view it without this check"
        )
        self.expected = expected
        self.actual = actual


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
