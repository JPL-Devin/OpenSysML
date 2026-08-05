"""Connection class for communicating with sysml-grpc service."""

import atexit
import grpc
import os
import psutil
import subprocess
import time
from filelock import FileLock, Timeout
from pysysml.proto import sysml_pb2, sysml_pb2_grpc
from pysysml.model import Model
from pysysml.binary import ensure_binary


def _get_lockfile_path():
    """Get path to service lockfile."""
    pysysml_dir = os.path.expanduser('~/.pysysml')
    os.makedirs(pysysml_dir, exist_ok=True)
    return os.path.join(pysysml_dir, 'sysml-grpc.lock')


def _get_pidfile_path():
    """Get path to service PID file."""
    pysysml_dir = os.path.expanduser('~/.pysysml')
    return os.path.join(pysysml_dir, 'sysml-grpc.pid')


def _get_refcount_path():
    """Get path to service reference count file."""
    pysysml_dir = os.path.expanduser('~/.pysysml')
    return os.path.join(pysysml_dir, 'sysml-grpc.refcount')


def _increment_refcount():
    """Increment service reference count. Caller must hold lockfile."""
    refcount_path = _get_refcount_path()
    if os.path.exists(refcount_path):
        with open(refcount_path, 'r') as f:
            count = int(f.read().strip())
    else:
        count = 0
    
    count += 1
    with open(refcount_path, 'w') as f:
        f.write(str(count))
    return count


def _decrement_refcount():
    """Decrement service reference count. Caller must hold lockfile."""
    refcount_path = _get_refcount_path()
    if not os.path.exists(refcount_path):
        return 0
    
    with open(refcount_path, 'r') as f:
        count = int(f.read().strip())
    
    count = max(0, count - 1)
    if count > 0:
        with open(refcount_path, 'w') as f:
            f.write(str(count))
    else:
        os.remove(refcount_path)
    return count


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
        self._cleaned_up = False
        
        # Auto-start service if requested
        if auto_start:
            self._ensure_service()
        
        self._channel = grpc.insecure_channel(self._address)
        self._stub = sysml_pb2_grpc.SysMLServiceStub(self._channel)
    
    def close(self):
        """Close the gRPC channel and decrement refcount."""
        if self._channel:
            self._channel.close()
        self._cleanup_service()
    
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
    
    def eval(self, expression, model_hash, context_symbol_id=None):
        """Evaluate a SysML expression.
        
        Args:
            expression (str): SysML expression (e.g., "2 + 2")
            model_hash (str): Hash from ParseFile response
            context_symbol_id (str, optional): Symbol FQN for context scope
            
        Returns:
            Value from expression (int, float, bool, str, Instance, etc.)
            
        Raises:
            RuntimeError: If evaluation fails
        """
        from pysysml.errors import RuntimeError as PyRuntimeError
        
        req = sysml_pb2.EvaluateRequest(
            model_hash=model_hash,
            expression=expression,
            context_symbol_id=context_symbol_id or ""
        )
        
        response = self._stub.Evaluate(req)
        
        if response.error:
            raise PyRuntimeError(response.error, diagnostics=list(response.diagnostics))
        
        # Convert protobuf Value to Python type
        return self._value_to_python(response.result)
    
    def instantiate(self, symbol_id, model_hash):
        """Instantiate a part/usage symbol.
        
        Args:
            symbol_id (str): FQN of part/usage to instantiate
            model_hash (str): Hash from ParseFile response
            
        Returns:
            Instance object
            
        Raises:
            RuntimeError: If instantiation fails
        """
        from pysysml.errors import RuntimeError as PyRuntimeError
        from pysysml.instance import Instance
        
        req = sysml_pb2.InstantiateRequest(
            model_hash=model_hash,
            symbol_id=symbol_id
        )
        
        response = self._stub.Instantiate(req)
        
        if response.error:
            raise PyRuntimeError(response.error, diagnostics=list(response.diagnostics))
        
        return Instance(response.instance)
    
    def execute_action(self, action_symbol_id, model_hash, inputs=None):
        """Execute an action definition.
        
        Args:
            action_symbol_id (str): FQN of action def
            model_hash (str): Hash from ParseFile response
            inputs (dict, optional): Input parameter name → value
            
        Returns:
            dict: Output parameter name → value
            
        Raises:
            RuntimeError: If execution fails
        """
        from pysysml.errors import RuntimeError as PyRuntimeError
        
        # Convert Python inputs to protobuf Values
        pb_inputs = {name: self._python_to_value(val) for name, val in (inputs or {}).items()}
        
        req = sysml_pb2.ExecuteActionRequest(
            model_hash=model_hash,
            action_symbol_id=action_symbol_id,
            inputs=pb_inputs
        )
        
        response = self._stub.ExecuteAction(req)
        
        if response.error:
            raise PyRuntimeError(response.error, diagnostics=list(response.diagnostics))
        
        # Convert outputs
        return {name: self._value_to_python(val) for name, val in response.outputs.items()}
    
    def execute_state(self, state_machine_symbol_id, model_hash, events=None):
        """Execute a state machine.
        
        Args:
            state_machine_symbol_id (str): FQN of state machine def
            model_hash (str): Hash from ParseFile response
            events (list, optional): Event names to process
            
        Returns:
            dict: {'states_visited': [...], 'final_context': {...}}
            
        Raises:
            RuntimeError: If execution fails
        """
        from pysysml.errors import RuntimeError as PyRuntimeError
        
        req = sysml_pb2.ExecuteStateRequest(
            model_hash=model_hash,
            state_machine_symbol_id=state_machine_symbol_id,
            events=events or []
        )
        
        response = self._stub.ExecuteState(req)
        
        if response.error:
            raise PyRuntimeError(response.error, diagnostics=list(response.diagnostics))
        
        return {
            'states_visited': list(response.states_visited),
            'final_context': {name: self._value_to_python(val) 
                             for name, val in response.final_context.items()}
        }
    
    def _python_to_value(self, py_value):
        """Convert Python type to protobuf Value."""
        if isinstance(py_value, bool):
            return sysml_pb2.Value(bool_value=py_value)
        elif isinstance(py_value, int):
            return sysml_pb2.Value(int_value=py_value)
        elif isinstance(py_value, float):
            return sysml_pb2.Value(real_value=py_value)
        elif isinstance(py_value, str):
            return sysml_pb2.Value(string_value=py_value)
        elif py_value is None:
            return sysml_pb2.Value(null=True)
        elif isinstance(py_value, list):
            elements = [self._python_to_value(v) for v in py_value]
            return sysml_pb2.Value(sequence=sysml_pb2.Sequence(elements=elements))
        else:
            raise ValueError(f"Unsupported Python type: {type(py_value)}")
    
    def _value_to_python(self, pb_value):
        """Convert protobuf Value to Python type."""
        kind = pb_value.WhichOneof('kind')
        if kind == 'int_value':
            return pb_value.int_value
        elif kind == 'real_value':
            return pb_value.real_value
        elif kind == 'bool_value':
            return pb_value.bool_value
        elif kind == 'string_value':
            return pb_value.string_value
        elif kind == 'instance_id':
            return pb_value.instance_id  # return ID for now
        elif kind == 'sequence':
            return [self._value_to_python(v) for v in pb_value.sequence.elements]
        elif kind == 'null':
            return None
        else:
            return None
    
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
        """Ensure sysml-grpc service is running, with lockfile coordination.
        
        Uses filelock to coordinate between multiple Python processes.
        If service already running, returns immediately.
        Otherwise, acquires lock and starts service.
        
        Raises:
            RuntimeError: If service cannot be started or lockfile timeout
        """
        lockfile_path = _get_lockfile_path()
        lock = FileLock(lockfile_path, timeout=30)
        
        try:
            with lock:
                # Check if service already running (another process may have started it)
                if self._probe_service(self.host, self.port):
                    # Service running - increment refcount and return
                    _increment_refcount()
                    atexit.register(self._cleanup_service)
                    return
                
                # Get binary path
                binary_path = ensure_binary()
                if not os.path.exists(binary_path):
                    raise RuntimeError(f"Binary not found after download: {binary_path}")
                
                # Start service
                process = subprocess.Popen(
                    [binary_path, '-port', str(self.port)],
                    start_new_session=True,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL
                )
                
                self._process = process
                
                # Wait for service to become healthy
                max_retries = 5
                retry_delay = 0.5
                
                for attempt in range(max_retries):
                    time.sleep(retry_delay)
                    if self._probe_service(self.host, self.port, timeout=2.0):
                        # Write PID file for reference counting
                        pidfile_path = _get_pidfile_path()
                        with open(pidfile_path, 'w') as f:
                            f.write(f"{process.pid}\n")
                        
                        # Increment refcount
                        _increment_refcount()
                        
                        # Register cleanup
                        atexit.register(self._cleanup_service)
                        return
                
                # Service didn't start in time
                # Cleanup without decrementing refcount (service never became healthy)
                if self._process:
                    self._process.terminate()
                    try:
                        self._process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        self._process.kill()
                        self._process.wait()
                    self._process = None
                raise RuntimeError(f"Service failed to start within {max_retries * retry_delay}s")
                
        except Timeout:
            raise RuntimeError(
                f"Timeout acquiring service lockfile after 30s. "
                f"Another process may be starting the service."
            )
    
    def _cleanup_service(self):
        """Clean up service process with reference counting."""
        if self._cleaned_up:
            return
        self._cleaned_up = True
        
        lockfile_path = _get_lockfile_path()
        lock = FileLock(lockfile_path, timeout=5)
        
        with lock:
            new_count = _decrement_refcount()
            
            if new_count == 0:
                # Last connection - shut down service
                pidfile_path = _get_pidfile_path()
                
                if os.path.exists(pidfile_path):
                    with open(pidfile_path, 'r') as f:
                        pid = int(f.read().strip())
                    
                    process = None
                    try:
                        process = psutil.Process(pid)
                        
                        # Verify this is our process before terminating
                        cmdline = process.cmdline()
                        if not any('sysml-grpc' in arg for arg in cmdline):
                            # Stale PID file - process is not sysml-grpc
                            os.remove(pidfile_path)
                            return
                        
                        # Safe to terminate
                        process.terminate()
                        process.wait(timeout=5)
                    except psutil.AccessDenied:
                        # Can't access process - clean up file
                        os.remove(pidfile_path)
                        return
                    except (psutil.NoSuchProcess, psutil.TimeoutExpired) as e:
                        if process and isinstance(e, psutil.TimeoutExpired):
                            try:
                                if process.is_running():
                                    process.kill()
                            except psutil.NoSuchProcess:
                                pass
                    
                    # Clean up PID file
                    os.remove(pidfile_path)
        
        # Clean up instance state
        if self._process:
            self._process = None
