"""pysysml - Python client library for Systemica SysML v2 parser."""

import warnings

from pysysml._version import VERSION as _declared_version
from pysysml.connection import Connection, DEFAULT_PORT, split_target
from pysysml.model import Model
from pysysml.symbol import Symbol
from pysysml.diagnostic import Diagnostic
from pysysml.enumeration import EnumLiteral
from pysysml.instance import Instance
from pysysml.typed import TypedObject
from pysysml.typefacts import (
    AttributeFacts,
    Multiplicity,
    Specialization,
    SymbolFacts,
    TypeFacts,
)
from pysysml.capabilities import MissingCapabilityError, ServerInfo
from pysysml.values import UNSET, UnsetType
from pysysml.verdict import CalcResult, Verdict
from pysysml.query import QueryElement, QueryError
from pysysml.conversion import (
    FORMAT_SYSML, FORMAT_TURTLE, Conversion, ExperimentalFeatureWarning,
    format_of_path, is_experimental,
)
from pysysml.edit import AppliedEdit, EditResult, Editor
from pysysml.errors import (
    PySysMLError, ChecksumMismatchError, ConnectionError, ConversionError,
    EditError, EditResultError, EditTargetError, ExecutionError,
    InvalidEditError, NoEditsError, OverlappingEditsError,
    RenameReferencedError,
    InstanceTypeError, InvalidRequestError, ModelError,
    ModelFileNotFoundError, ModelNotFoundError, ServiceError,
    ServiceTimeoutError, SlotError, StaleServiceError, SymbolNotFoundError,
    TypeMismatchError, UnpinnedReleaseError, UnsupportedOperationError,
    UnsupportedValueError, WrongKindError,
)

__all__ = [
    "Connection", "Model", "Symbol", "Diagnostic", "EnumLiteral", "Instance",
    "TypedObject", "TypeFacts", "Multiplicity", "Specialization", "SymbolFacts",
    "AttributeFacts",
    "ServerInfo",
    "UNSET", "UnsetType",
    "Conversion", "FORMAT_SYSML", "FORMAT_TURTLE", "format_of_path",
    "ExperimentalFeatureWarning", "is_experimental",
    "Editor", "EditResult", "AppliedEdit",
    "Verdict", "CalcResult",
    "QueryElement", "QueryError",
    "PySysMLError", "ChecksumMismatchError", "ConnectionError",
    "ConversionError", "ExecutionError",
    "EditError", "NoEditsError", "EditTargetError", "InvalidEditError",
    "RenameReferencedError", "OverlappingEditsError", "EditResultError",
    "InstanceTypeError", "InvalidRequestError", "MissingCapabilityError",
    "ModelError", "ModelFileNotFoundError", "ModelNotFoundError",
    "ServiceError", "ServiceTimeoutError", "SlotError", "StaleServiceError",
    "SymbolNotFoundError",
    "TypeMismatchError", "UnpinnedReleaseError",
    "UnsupportedOperationError", "UnsupportedValueError",
    "WrongKindError",
    "load", "connect", "convert",
    # "eval" is deprecated in favour of "evaluate", so it is not exported.
    "evaluate", "instantiate",
    "DEFAULT_PORT", "split_target",
    "__version__"
]

# The declaration ships beside this module, so this is the version of the code
# being imported: right for an editable install, whose dist-info metadata is
# frozen at install time, and the same value a wheel's metadata is built from.
__version__ = _declared_version

# Module-level default connection (lazy singleton)
_default_connection = None
_default_connection_params = None


def _get_default_connection(host='localhost', port=None):
    """Get or create the default module-level connection.
    
    If called with different host/port than last time, creates new connection.
    A ``host:port`` address written as the host names the same service as the
    two given separately, so it reuses the same connection.
    
    Args:
        host (str): Service hostname, or a ``host:port`` address
        port (int, optional): Service port (default: 50051)
    
    Returns:
        Connection: Singleton default connection

    Raises:
        ValueError: If host names a port that is unreadable or disagrees with port
    """
    global _default_connection, _default_connection_params
    host, port = split_target(host, port)
    params = (host, port)
    
    # Create new connection if params changed or no connection exists
    if _default_connection is None or _default_connection_params != params:
        # Close old connection to avoid refcount leak
        if _default_connection is not None:
            _default_connection.close()
        _default_connection = Connection(host, port, auto_start=True)
        _default_connection_params = params
    
    return _default_connection


def load(file_path, host='localhost', port=None, strict=False):
    """Load a SysML model from file using the default connection.
    
    Convenience function that uses a module-level singleton connection.
    
    Args:
        file_path (str): Path to .sysml file
        host (str): Service hostname, or a ``host:port`` address
        port (int, optional): Service port (default: 50051)
        strict (bool): Refuse a model the service reported errors for, rather
            than returning one whose lookups fail later
    
    Returns:
        Model: Parsed model object
    
    Raises:
        ModelFileNotFoundError: If the service cannot read file_path
        ModelError: If strict and the model has error diagnostics
        ConnectionError: If the service is unreachable
        ValueError: If host names a port that is unreadable or disagrees with port
    """
    conn = _get_default_connection(host, port)
    return conn.load(file_path, strict=strict)


def connect(host='localhost', port=None, auto_start=True, version=None,
            require_capabilities=None):
    """Create a new connection to sysml-grpc service.
    
    Convenience function that creates a new Connection instance.
    
    Args:
        host (str): Service hostname, or a ``host:port`` address, whose port is
            used when no separate port is given (default: 'localhost')
        port (int, optional): Service port (default: 50051)
        auto_start (bool): If True, automatically start service if not running (default: True)
        version (str, optional): Release tag the service must report, or
            'latest'; defaults to $PYSYSML_GRPC_VERSION. Checked whether the
            service is started here or managed by the caller
        require_capabilities (iterable, optional): Capability names the service
            must report, checked at connect time
    
    Returns:
        Connection: New connection instance

    Raises:
        ValueError: If host names a port that is unreadable or disagrees with port
        StaleServiceError: If another release is already listening on the
            address and this client may not stop it
        MissingCapabilityError: If the service lacks a required capability

    Example:
        >>> conn = pysysml.connect("localhost:50123")
        >>> conn.port
        50123
    """
    host, port = split_target(host, port)
    return Connection(
        host, port, auto_start=auto_start, version=version,
        require_capabilities=require_capabilities,
    )


def convert(to_format, file_path=None, content=None, model_hash=None,
            from_format='', tolerate_syntax_errors=False, host='localhost',
            port=None):
    """Write a model out in another format (module-level convenience).

    Args:
        to_format (str): 'sysml', 'kerml', 'text', 'ttl', 'turtle' or 'rdf'
        file_path (str, optional): Path the service reads the source from
        content (str, optional): Source carried inline
        model_hash (str, optional): Hash of a loaded model, whose parsed source
            is converted
        from_format (str, optional): Format to read the source as; inferred from
            file_path's extension when omitted, notation for a model_hash, and
            required for inline content
        tolerate_syntax_errors (bool): Write notation back out even when the
            parser could not read all of it
        host (str): Service hostname, or a ``host:port`` address
        port (int, optional): Service port (default: 50051)

    Returns:
        Conversion: The converted model; ``str()`` of it is the text

    Warns:
        ExperimentalFeatureWarning: If either format is RDF, whose mapping is
            experimental — see ``docs/reference/rdf-mapping.md``

    Example:
        >>> import pysysml
        >>> turtle = pysysml.convert("ttl", file_path="model.sysml")
        >>> turtle.write("model.ttl")
        'model.ttl'
    """
    conn = _get_default_connection(host, port)
    return conn.convert(
        to_format,
        file_path=file_path,
        content=content,
        model_hash=model_hash,
        from_format=from_format,
        tolerate_syntax_errors=tolerate_syntax_errors,
    )


def evaluate(expression, file_path=None, model_hash=None, context_symbol_id=None,
             host='localhost', port=None, subject=None):
    """Evaluate a SysML expression (module-level convenience).

    A model in hand has :meth:`Model.eval`, which needs neither the hash nor the
    connection; this form is for an expression evaluated against a file.
    
    Args:
        expression (str): SysML expression
        file_path (str, optional): Parse this file first, get model_hash
        model_hash (str, optional): Use existing model hash
        context_symbol_id (str, optional): Context for evaluation
        host (str): Service hostname, or a ``host:port`` address
        port (int, optional): Service port (default: 50051)
        subject (str, optional): FQN of a part/usage to instantiate and evaluate
            against, so a feature reads that object's value rather than the
            declared default. Last, so a positional call written before it
            still binds the address it meant
        
    Returns:
        Evaluated value
        
    Raises:
        ValueError: If neither file_path nor model_hash provided, if both are,
            or if host names a port that is unreadable or disagrees with port
        ExecutionError: If evaluation fails
        
    Example:
        >>> import pysysml
        >>> result = pysysml.evaluate("2 + 2", file_path="test.sysml")
        >>> print(result)  # 4
    """
    conn = _get_default_connection(host, port)
    
    # Validate params: exactly one of file_path or model_hash required
    if file_path and model_hash:
        raise ValueError("Provide either file_path or model_hash, not both")
    
    if not file_path and not model_hash:
        raise ValueError("Must provide either file_path or model_hash")
    
    if file_path:
        model = conn.load(file_path)
        model_hash = model.hash
    
    return conn.eval(
        expression,
        model_hash,
        context_symbol_id=context_symbol_id,
        subject_symbol_id=subject,
    )


def instantiate(symbol_id, file_path=None, model_hash=None, host='localhost',
                port=None):
    """Instantiate a part/usage (module-level convenience).

    A model in hand has :meth:`Model.instantiate`, which needs neither the hash
    nor the connection; this form is for instantiating out of a file.

    Args:
        symbol_id (str): FQN of symbol to instantiate
        file_path (str, optional): Parse this file first
        model_hash (str, optional): Use existing model hash
        host (str): Service hostname, or a ``host:port`` address
        port (int, optional): Service port (default: 50051)
        
    Returns:
        Instance: Instance object
        
    Raises:
        ValueError: If neither file_path nor model_hash provided, if both are,
            or if host names a port that is unreadable or disagrees with port
        ExecutionError: If instantiation fails
        
    Example:
        >>> import pysysml
        >>> instance = pysysml.instantiate("SPACECRAFT_WET", file_path="A1.sysml")
        >>> print(instance.id)
    """
    conn = _get_default_connection(host, port)
    
    # Validate params: exactly one of file_path or model_hash required
    if file_path and model_hash:
        raise ValueError("Provide either file_path or model_hash, not both")
    
    if not file_path and not model_hash:
        raise ValueError("Must provide either file_path or model_hash")
    
    if file_path:
        model = conn.load(file_path)
        model_hash = model.hash
    
    return conn.instantiate(symbol_id, model_hash)


#: Names that shadowed a built-in, and what each is called now. Served through
#: __getattr__ so a star-import no longer binds over the built-in.
_RENAMED_NAMES = {"eval": "evaluate", "RuntimeError": "ExecutionError"}


def __getattr__(name):
    """Serve a renamed name with the object it became, warning about its use.

    ``pysysml.eval`` is :func:`evaluate` and ``pysysml.RuntimeError`` is
    :class:`~pysysml.errors.ExecutionError`, so existing snippets keep working.
    """
    replacement = _RENAMED_NAMES.get(name)
    if replacement is None:
        raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
    warnings.warn(
        f"pysysml.{name} is deprecated; use pysysml.{replacement} instead",
        DeprecationWarning,
        stacklevel=2,
    )
    return globals()[replacement]
