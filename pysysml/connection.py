"""Connection class for communicating with sysml-grpc service."""

import grpc
from pysysml.proto import sysml_pb2, sysml_pb2_grpc
from pysysml.model import Model


class Connection:
    """Manages connection to sysml-grpc service.
    
    Phase 2: Manual connection only (service must be started externally).
    Phase 3 will add auto-start capabilities.
    
    Attributes:
        host (str): Service hostname
        port (int): Service port
    """
    
    def __init__(self, host='localhost', port=50051):
        """Initialize connection to sysml-grpc service.
        
        Args:
            host (str): Service hostname (default: 'localhost')
            port (int): Service port (default: 50051)
        """
        self.host = host
        self.port = port
        self._address = f"{host}:{port}"
        self._channel = grpc.insecure_channel(self._address)
        self._stub = sysml_pb2_grpc.SysMLServiceStub(self._channel)
    
    def close(self):
        """Close the gRPC channel."""
        if self._channel:
            self._channel.close()
    
    def __enter__(self):
        """Context manager entry."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()
    
    def load(self, file_path):
        """Load a SysML model from file.
        
        Args:
            file_path (str): Path to .sysml file
        
        Returns:
            Model: Parsed model object
        
        Raises:
            grpc.RpcError: If file not found or gRPC error occurs
        """
        request = sysml_pb2.ParseFileRequest(file_path=file_path)
        response = self._stub.ParseFile(request)
        return Model(response, self)
    
    def get_symbol(self, model_hash, symbol_id):
        """Fetch symbol by ID from cached model.
        
        Args:
            model_hash (str): Model content hash
            symbol_id (str): Fully-qualified symbol ID
        
        Returns:
            sysml_pb2.SymbolInfo or None: Symbol protobuf, or None if not found
        """
        request = sysml_pb2.GetSymbolRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
        )
        response = self._stub.GetSymbol(request)
        
        if response.error:
            # Symbol not found or other error
            return None
        
        return response.symbol
