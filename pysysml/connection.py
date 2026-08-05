"""Connection class for communicating with sysml-grpc service."""

import atexit
import grpc
import os
import subprocess
import time
from pysysml.proto import sysml_pb2, sysml_pb2_grpc
from pysysml.model import Model
from pysysml.binary import ensure_binary


class Connection:
    """Manages connection to sysml-grpc service.
    
    Phase 3: Auto-start capabilities - service can be started automatically.
    
    Attributes:
        host (str): Service hostname
        port (int): Service port
    """
    
    def __init__(self, host='localhost', port=50051, auto_start=True):
        """Initialize connection to sysml-grpc service.
        
        Args:
            host (str): Service hostname (default: 'localhost')
            port (int): Service port (default: 50051)
            auto_start (bool): If True, automatically start service if not running (default: True)
        """
        self.host = host
        self.port = port
        self._address = f"{host}:{port}"
        self._process = None
        
        # Auto-start service if requested
        if auto_start:
            self._ensure_service()
        
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
    
    def _probe_service(self, host, port, timeout=5.0):
        """Check if sysml-grpc service is running and responsive.
        
        Args:
            host (str): Service hostname
            port (int): Service port
            timeout (float): RPC timeout in seconds
        
        Returns:
            bool: True if service responds to health check, False otherwise
        """
        try:
            address = f"{host}:{port}"
            channel = grpc.insecure_channel(address)
            stub = sysml_pb2_grpc.SysMLServiceStub(channel)
            
            # Use GetDiagnostics as health check (lightweight RPC)
            request = sysml_pb2.DiagnosticsRequest(model_hash="health_check")
            stub.GetDiagnostics(request, timeout=timeout)
            
            channel.close()
            return True
        except grpc.RpcError as e:
            # NOT_FOUND is expected for invalid hash - service is working
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return True
            return False
        except Exception as e:
            # Could be: service not ready, crashed, or network error
            return False
    
    def _ensure_service(self):
        """Ensure sysml-grpc service is running, starting it if necessary.
        
        Raises:
            RuntimeError: If service cannot be started
        """
        # Check if service already running
        if self._probe_service(self.host, self.port, timeout=1.0):
            return
        
        # Get binary path
        binary_path = ensure_binary()
        if not binary_path or not os.path.exists(binary_path):
            raise RuntimeError(f"Binary not found at: {binary_path}")
        
        # Start service subprocess
        self._process = subprocess.Popen(
            [binary_path, '-port', str(self.port)],
            start_new_session=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        
        # Register cleanup handler
        atexit.register(self._cleanup_service)
        
        # Wait for service to become ready
        # TODO: Consider making startup_timeout configurable for slow environments
        max_retries = 5
        retry_delay = 0.5
        
        for i in range(max_retries):
            time.sleep(retry_delay)
            if self._probe_service(self.host, self.port, timeout=1.0):
                return
        
        # Failed to start
        self._cleanup_service()
        raise RuntimeError(
            f"Failed to start sysml-grpc service after {max_retries * retry_delay}s"
        )
    
    def _cleanup_service(self):
        """Terminate subprocess if it was started by this Connection."""
        if self._process:
            self._process.terminate()
            try:
                self._process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._process.kill()
                self._process.wait()
            self._process = None
