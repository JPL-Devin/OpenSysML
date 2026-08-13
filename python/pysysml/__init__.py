"""pysysml - Python client library for Systemica SysML v2 parser."""

from importlib.metadata import PackageNotFoundError, version as _distribution_version

from pysysml._version import VERSION as _declared_version
from pysysml.connection import Connection
from pysysml.model import Model
from pysysml.symbol import Symbol
from pysysml.diagnostic import Diagnostic
from pysysml.instance import Instance
from pysysml.errors import (
    PySysMLError, ConnectionError, RuntimeError, SlotError, UnsupportedValueError,
)

__all__ = [
    "Connection", "Model", "Symbol", "Diagnostic", "Instance",
    "PySysMLError", "ConnectionError", "RuntimeError", "SlotError",
    "UnsupportedValueError",
    "load", "connect",
    "eval", "instantiate",
    "__version__"
]

try:
    # The version of the distribution actually installed, so a wheel reports
    # what was published rather than a string kept in step by hand.
    __version__ = _distribution_version("pysysml")
except PackageNotFoundError:
    # Running from a source tree that was never installed.
    __version__ = _declared_version

# Module-level default connection (lazy singleton)
_default_connection = None
_default_connection_params = None


def _get_default_connection(host='localhost', port=50051):
    """Get or create the default module-level connection.
    
    If called with different host/port than last time, creates new connection.
    
    Args:
        host (str): Service hostname (default: 'localhost')
        port (int): Service port (default: 50051)
    
    Returns:
        Connection: Singleton default connection
    """
    global _default_connection, _default_connection_params
    params = (host, port)
    
    # Create new connection if params changed or no connection exists
    if _default_connection is None or _default_connection_params != params:
        # Close old connection to avoid refcount leak
        if _default_connection is not None:
            _default_connection.close()
        _default_connection = Connection(host, port, auto_start=True)
        _default_connection_params = params
    
    return _default_connection


def load(file_path, host='localhost', port=50051):
    """Load a SysML model from file using the default connection.
    
    Convenience function that uses a module-level singleton connection.
    
    Args:
        file_path (str): Path to .sysml file
        host (str): Service hostname (default: 'localhost')
        port (int): Service port (default: 50051)
    
    Returns:
        Model: Parsed model object
    
    Raises:
        grpc.RpcError: If file not found or gRPC error occurs
    """
    conn = _get_default_connection(host, port)
    return conn.load(file_path)


def connect(host='localhost', port=50051, auto_start=True):
    """Create a new connection to sysml-grpc service.
    
    Convenience function that creates a new Connection instance.
    
    Args:
        host (str): Service hostname (default: 'localhost')
        port (int): Service port (default: 50051)
        auto_start (bool): If True, automatically start service if not running (default: True)
    
    Returns:
        Connection: New connection instance
    """
    return Connection(host, port, auto_start=auto_start)


def eval(expression, file_path=None, model_hash=None, context_symbol_id=None):
    """Evaluate a SysML expression (module-level convenience).
    
    Args:
        expression (str): SysML expression
        file_path (str, optional): Parse this file first, get model_hash
        model_hash (str, optional): Use existing model hash
        context_symbol_id (str, optional): Context for evaluation
        
    Returns:
        Evaluated value
        
    Raises:
        ValueError: If neither file_path nor model_hash provided, or if both provided
        RuntimeError: If evaluation fails
        
    Example:
        >>> import pysysml
        >>> result = pysysml.eval("2 + 2", file_path="test.sysml")
        >>> print(result)  # 4
    """
    conn = _get_default_connection()
    
    # Validate params: exactly one of file_path or model_hash required
    if file_path and model_hash:
        raise ValueError("Provide either file_path or model_hash, not both")
    
    if not file_path and not model_hash:
        raise ValueError("Must provide either file_path or model_hash")
    
    if file_path:
        model = conn.load(file_path)
        model_hash = model.hash
    
    return conn.eval(expression, model_hash, context_symbol_id)


def instantiate(symbol_id, file_path=None, model_hash=None):
    """Instantiate a part/usage (module-level convenience).
    
    Args:
        symbol_id (str): FQN of symbol to instantiate
        file_path (str, optional): Parse this file first
        model_hash (str, optional): Use existing model hash
        
    Returns:
        Instance: Instance object
        
    Raises:
        ValueError: If neither file_path nor model_hash provided, or if both provided
        RuntimeError: If instantiation fails
        
    Example:
        >>> import pysysml
        >>> instance = pysysml.instantiate("SPACECRAFT_WET", file_path="A1.sysml")
        >>> print(instance.id)
    """
    conn = _get_default_connection()
    
    # Validate params: exactly one of file_path or model_hash required
    if file_path and model_hash:
        raise ValueError("Provide either file_path or model_hash, not both")
    
    if not file_path and not model_hash:
        raise ValueError("Must provide either file_path or model_hash")
    
    if file_path:
        model = conn.load(file_path)
        model_hash = model.hash
    
    return conn.instantiate(symbol_id, model_hash)
