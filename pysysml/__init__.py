"""pysysml - Python client library for Systemica SysML v2 parser."""

__version__ = "0.1.0"

from pysysml.connection import Connection
from pysysml.model import Model
from pysysml.symbol import Symbol
from pysysml.diagnostic import Diagnostic

__all__ = ["Connection", "Model", "Symbol", "Diagnostic", "load", "connect"]

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
