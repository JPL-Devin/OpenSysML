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
from pysysml.capabilities import (
    CAPABILITY_CONVERT,
    CAPABILITY_VERIFICATION,
    ServerInfo,
    require,
)
from pysysml.conversion import Conversion
from pysysml.diagnostic import Diagnostic
from pysysml.errors import (
    ConnectionError,
    ConversionError,
    ExecutionError,
    ModelFileNotFoundError,
    ModelNotFoundError,
    UnsupportedValueError,
    from_rpc_error,
    translate_rpc_errors,
)
from pysysml.values import value_to_python
from pysysml.verdict import CalcResult, Verdict


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
    os.makedirs(os.path.dirname(refcount_path), exist_ok=True)
    
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


def _is_pidfile_stale():
    """Check if pidfile refers to dead/wrong process. Returns (stale, live_process).
    
    Caller must hold lockfile.
    
    Returns:
        tuple: (bool stale, psutil.Process or None)
               - (True, None): pidfile doesn't exist or points to dead process
               - (True, proc): pidfile points to wrong process (PID reused)
               - (False, proc): pidfile valid, process is sysml-grpc
    """
    pidfile_path = _get_pidfile_path()
    if not os.path.exists(pidfile_path):
        return (True, None)
    
    try:
        with open(pidfile_path, 'r') as f:
            pid = int(f.read().strip())
        
        process = psutil.Process(pid)
        
        # Verify process is actually sysml-grpc (not PID reuse)
        cmdline = ' '.join(process.cmdline())
        if 'sysml-grpc' in cmdline:
            return (False, process)  # Valid
        else:
            return (True, process)  # PID reused by different process
            
    except (psutil.NoSuchProcess, psutil.AccessDenied, ValueError, OSError):
        return (True, None)


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
        # Provenance of the service, so an error can name the binary at fault.
        # Refined by _ensure_service, which knows how it was reached.
        self._origin = f"service at {self._address} (not started by this client)"
        self._server_info = None
        # Only connections that took a reference may release one on close.
        self._holds_refcount = False
        
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
    
    def server_info(self):
        """Ask the service what it is and what it supports.

        The answer is cached for the life of the connection: a service does not
        change build while a channel is open to it.

        Returns:
            ServerInfo: Reported version and capabilities. ``answered`` is False
                when the service predates the GetServerInfo RPC, in which case
                it claims no capabilities.
        """
        if self._server_info is None:
            request = sysml_pb2.ServerInfoRequest()
            try:
                response = self._stub.GetServerInfo(request)
            except grpc.RpcError as e:
                if e.code() != grpc.StatusCode.UNIMPLEMENTED:
                    raise from_rpc_error(e) from e
                self._server_info = ServerInfo(
                    version='',
                    capabilities=frozenset(),
                    answered=False,
                    origin=self._origin,
                )
            else:
                self._server_info = ServerInfo(
                    version=response.version,
                    capabilities=frozenset(response.capabilities),
                    answered=True,
                    origin=self._origin,
                )
        return self._server_info

    def load(self, file_path, strict=False):
        """Load a SysML model from file.
        
        Args:
            file_path (str): Path to .sysml file
            strict (bool): Refuse a model the service reported errors for,
                instead of returning one whose lookups fail later. The
                :class:`~pysysml.errors.ModelError` raised carries the model, so
                its diagnostics stay inspectable.
        
        Returns:
            Model: Parsed model object
        
        Raises:
            ModelFileNotFoundError: If the service cannot read file_path
            ModelError: If strict and the model has error diagnostics
            ServiceError: If the service fails the call for any other reason
        """
        request = sysml_pb2.ParseFileRequest(file_path=file_path)
        with translate_rpc_errors(not_found=ModelFileNotFoundError):
            response = self._stub.ParseFile(request)
        model = Model(response, self, source_path=file_path)
        if strict:
            model.raise_for_errors()
        return model
    
    def load_from_content(self, content, strict=False):
        """Load a model from inline SysML content.
        
        Args:
            content (str): SysML source code
            strict (bool): Refuse a model the service reported errors for
            
        Returns:
            Model: Parsed model object

        Raises:
            ModelError: If strict and the model has error diagnostics
        """
        request = sysml_pb2.ParseFileRequest(content=content)
        with translate_rpc_errors():
            response = self._stub.ParseFile(request)
        model = Model(response, self)
        if strict:
            model.raise_for_errors()
        return model
    
    def convert(self, to_format, file_path=None, content=None, model_hash=None,
                from_format='', tolerate_syntax_errors=False):
        """Write a model out in another of the formats Systemica writes.

        The source is a loaded model, named by its hash, or one named the way
        :meth:`load` names it: a path the service opens, or content carried
        inline. A hash converts the source the service parsed, so a file edited
        since the load does not change the answer; a path is read afresh.

        Args:
            to_format (str): Format to write: 'sysml', 'kerml', 'text', 'ttl',
                'turtle' or 'rdf'
            file_path (str, optional): Path the service reads the source from
            content (str, optional): Source carried inline
            model_hash (str, optional): Hash of a loaded model, whose parsed
                source is converted
            from_format (str, optional): Format to read the source as; inferred
                from file_path's extension when omitted, notation for a
                model_hash, and required for inline content
            tolerate_syntax_errors (bool): Write notation back out even when the
                parser could not read all of it, reporting its syntax errors as
                the result's diagnostics. Notation to notation only: every other
                direction builds a graph, where unreadable declarations would go
                missing silently.

        Returns:
            Conversion: The converted model, the formats used and any tolerated
                syntax errors

        Raises:
            ValueError: If other than one of file_path, content and model_hash
                is given
            MissingCapabilityError: If the service cannot convert
            ConversionError: If the model could not be written in that format
            InvalidRequestError: If a format is unknown
            ModelFileNotFoundError: If the named file cannot be read
            ModelNotFoundError: If the model is no longer cached
        """
        given = [
            name
            for name, value in (
                ('file_path', file_path),
                ('content', content),
                ('model_hash', model_hash),
            )
            if value is not None
        ]
        if len(given) != 1:
            raise ValueError(
                "Provide exactly one of file_path, content or model_hash; got "
                + (", ".join(given) if given else "none")
            )
        require(
            self.server_info(),
            CAPABILITY_CONVERT,
            "upgrade the sysml-grpc service to a build whose GetServerInfo "
            "reports 'convert'",
        )

        request = sysml_pb2.ConvertRequest(
            to_format=to_format,
            from_format=from_format,
            tolerate_syntax_errors=tolerate_syntax_errors,
        )
        if file_path is not None:
            request.file_path = file_path
        elif content is not None:
            request.content = content
        else:
            request.model_hash = model_hash

        not_found = (
            ModelFileNotFoundError if file_path is not None else ModelNotFoundError
        )
        with translate_rpc_errors(not_found=not_found):
            response = self._stub.Convert(request)
        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ConversionError(response.error, diagnostics=diagnostics)
        return Conversion(
            content=response.content,
            from_format=response.from_format,
            to_format=response.to_format,
            diagnostics=diagnostics,
        )

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
        with translate_rpc_errors():
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
            ExecutionError: If evaluation fails
            ModelNotFoundError: If the service no longer holds the model
            UnsupportedValueError: If the result cannot be represented on the wire
        """
        req = sysml_pb2.EvaluateRequest(
            model_hash=model_hash,
            expression=expression,
            context_symbol_id=context_symbol_id or ""
        )
        
        with translate_rpc_errors():
            response = self._stub.Evaluate(req)
        
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
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
            ExecutionError: If instantiation fails
            ModelNotFoundError: If the service no longer holds the model
        """
        from pysysml.instance import Instance
        
        req = sysml_pb2.InstantiateRequest(
            model_hash=model_hash,
            symbol_id=symbol_id
        )
        
        with translate_rpc_errors():
            response = self._stub.Instantiate(req)
        
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        graph = {inst.id: inst for inst in response.instances}
        return Instance(response.instance, graph)
    
    def execute_action(self, action_symbol_id, model_hash, inputs=None):
        """Execute an action definition.
        
        Args:
            action_symbol_id (str): FQN of action def
            model_hash (str): Hash from ParseFile response
            inputs (dict, optional): Input parameter name → value
            
        Returns:
            dict: Output parameter name → value; an output the wire format cannot
                represent is reported as an UnsupportedValueError in its place,
                so one such output does not discard the rest
            
        Raises:
            ExecutionError: If execution fails
            ModelNotFoundError: If the service no longer holds the model
        """
        # Convert Python inputs to protobuf Values
        pb_inputs = {name: self._python_to_value(val) for name, val in (inputs or {}).items()}
        
        req = sysml_pb2.ExecuteActionRequest(
            model_hash=model_hash,
            action_symbol_id=action_symbol_id,
            inputs=pb_inputs
        )
        
        with translate_rpc_errors():
            response = self._stub.ExecuteAction(req)
        
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        return self._values_to_python(response.outputs)
    
    def execute_state(self, state_machine_symbol_id, model_hash, events=None):
        """Execute a state machine.
        
        Args:
            state_machine_symbol_id (str): FQN of state machine def
            model_hash (str): Hash from ParseFile response
            events (list, optional): Event names to process
            
        Returns:
            dict: {'states_visited': [...], 'final_context': {...}}; a context value
                the wire format cannot represent is reported as an
                UnsupportedValueError in its place
            
        Raises:
            ExecutionError: If execution fails
            ModelNotFoundError: If the service no longer holds the model
        """
        req = sysml_pb2.ExecuteStateRequest(
            model_hash=model_hash,
            state_machine_symbol_id=state_machine_symbol_id,
            events=events or []
        )
        
        with translate_rpc_errors():
            response = self._stub.ExecuteState(req)
        
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        return {
            'states_visited': list(response.states_visited),
            'final_context': self._values_to_python(response.final_context),
        }
    
    def verify_constraint(self, symbol_id, model_hash, subject_symbol_id=None):
        """Ask whether a constraint holds, as the REPL's ``%constraint`` does.

        Args:
            symbol_id (str): FQN of the constraint definition or usage
            model_hash (str): Hash from ParseFile response
            subject_symbol_id (str, optional): FQN of a part/usage to
                instantiate and evaluate against, so the verdict is about
                concrete values rather than declared defaults

        Returns:
            Verdict: The answer. A condition that evaluated to false is that
                answer, not an exception; a failure to evaluate is reported as
                ``verdict.error``.

        Raises:
            ExecutionError: If the request could not be answered at all — an
                unknown symbol, a subject that could not be instantiated
            MissingCapabilityError: If the service cannot verify
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        request = sysml_pb2.VerifyConstraintRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            subject_symbol_id=subject_symbol_id or "",
        )
        with translate_rpc_errors():
            response = self._stub.VerifyConstraint(request)
        return self._verdict_of(response)

    def verify_requirement(self, symbol_id, model_hash, subject_symbol_id=None):
        """Ask whether a requirement is satisfied, as ``%requirement`` does.

        Args:
            symbol_id (str): FQN of the requirement definition or usage
            model_hash (str): Hash from ParseFile response
            subject_symbol_id (str, optional): FQN of a part/usage to
                instantiate and evaluate against

        Returns:
            Verdict: The answer

        Raises:
            ExecutionError: If the request could not be answered at all
            MissingCapabilityError: If the service cannot verify
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        request = sysml_pb2.VerifyRequirementRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            subject_symbol_id=subject_symbol_id or "",
        )
        with translate_rpc_errors():
            response = self._stub.VerifyRequirement(request)
        return self._verdict_of(response)

    def verify_satisfaction(self, model_hash, symbol_id=None):
        """Ask whether the model's satisfaction assertions hold, as ``%satisfy`` does.

        Each assertion is evaluated against an object of its subject, built for
        the call, so a verdict is about the values that subject holds.

        Args:
            model_hash (str): Hash from ParseFile response
            symbol_id (str, optional): FQN limiting evaluation to the assertions
                stated within that element, or to that element itself when it is
                a named satisfaction assertion. Omitted evaluates every
                assertion the model states.

        Returns:
            list[Verdict]: One verdict per assertion, in declaration order. A
                model stating no assertion gives an empty list.

        Raises:
            ExecutionError: If the request could not be answered at all
            MissingCapabilityError: If the service cannot verify
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        request = sysml_pb2.VerifySatisfactionRequest(
            model_hash=model_hash,
            symbol_id=symbol_id or "",
        )
        with translate_rpc_errors():
            response = self._stub.VerifySatisfaction(request)

        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ExecutionError(response.error, diagnostics=diagnostics)
        instances = self._instances_of(response)
        return [
            Verdict(pb_verdict, instances=instances, diagnostics=diagnostics)
            for pb_verdict in response.verdicts
        ]

    def calc(self, symbol_id, model_hash, arguments=None):
        """Invoke a calculation, as the REPL's ``%calc`` does.

        Arguments are bound positionally. A calc usage named with no arguments
        binds its inputs from its own members and reports every output feature
        it computes (SysML 7.17).

        Args:
            symbol_id (str): FQN of the calc definition or usage
            model_hash (str): Hash from ParseFile response
            arguments (list, optional): Positional arguments, as Python values

        Returns:
            CalcResult: The value an invocation returned, or the output features
                a calc usage computed

        Raises:
            ExecutionError: If the calculation could not be evaluated
            MissingCapabilityError: If the service cannot verify
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        request = sysml_pb2.EvaluateCalcRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            arguments=[self._python_to_value(arg) for arg in (arguments or [])],
        )
        with translate_rpc_errors():
            response = self._stub.EvaluateCalc(request)

        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ExecutionError(response.error, diagnostics=diagnostics)

        outputs = {}
        for output in response.outputs:
            try:
                outputs[output.name] = self._value_to_python(output.value)
            except UnsupportedValueError as exc:
                outputs[output.name] = exc
        value = None
        if not outputs and response.HasField('result'):
            value = self._value_to_python(response.result)
        return CalcResult(value, outputs, diagnostics=diagnostics)

    def _require_verification(self):
        """Refuse a verification the connected service does not implement."""
        require(
            self.server_info(),
            CAPABILITY_VERIFICATION,
            "upgrade the sysml-grpc service to a build whose GetServerInfo "
            "reports 'verification'",
        )

    def _verdict_of(self, response):
        """Wrap a single-verdict verification response, raising its failure."""
        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ExecutionError(response.error, diagnostics=diagnostics)
        return Verdict(
            response.verdict,
            instances=self._instances_of(response),
            diagnostics=diagnostics,
        )

    def _instances_of(self, response):
        """Wrap the instance graph a verification returned, roots first."""
        from pysysml.instance import Instance

        graph = {inst.id: inst for inst in response.instances}
        wrappers = {}
        return [
            Instance(pb_inst, graph, _wrappers=wrappers)
            for pb_inst in response.instances
        ]

    def _python_to_value(self, py_value):
        """Convert Python type to protobuf Value."""
        from pysysml.instance import Instance
        
        if isinstance(py_value, bool):
            return sysml_pb2.Value(bool_value=py_value)
        elif isinstance(py_value, int):
            return sysml_pb2.Value(int_value=py_value)
        elif isinstance(py_value, float):
            return sysml_pb2.Value(real_value=py_value)
        elif isinstance(py_value, str):
            return sysml_pb2.Value(string_value=py_value)
        elif py_value is None:
            return sysml_pb2.Value(null="")
        elif isinstance(py_value, Instance):
            return sysml_pb2.Value(instance_id=py_value.id)
        elif isinstance(py_value, list):
            elements = [self._python_to_value(v) for v in py_value]
            return sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=elements))
        else:
            raise ValueError(f"Unsupported Python type: {type(py_value)}")
    
    def _value_to_python(self, pb_value):
        """Convert protobuf Value to Python type.

        Instance references outside an Instantiate response are returned as
        their integer id; there is no instance graph to resolve them against.
        """
        return value_to_python(pb_value)

    def _values_to_python(self, pb_values):
        """Convert a name → Value map, keeping an unsupported value as its error.

        Mirrors Instance.slots: one value the wire format cannot represent must
        not discard the entries around it.
        """
        result = {}
        for name, pb_value in pb_values.items():
            try:
                result[name] = self._value_to_python(pb_value)
            except UnsupportedValueError as exc:
                result[name] = exc
        return result
    
    def _probe_service(self, host, port, timeout=5.0):
        """Check if sysml-grpc service is running and responsive.
        
        Args:
            host (str): Service hostname
            port (int): Service port
            timeout (float): RPC timeout in seconds
        
        Returns:
            bool: True if service responds to health check, False otherwise
        """
        address = f"{host}:{port}"
        channel = grpc.insecure_channel(address)
        try:
            stub = sysml_pb2_grpc.SysMLServiceStub(channel)
            
            # Use GetDiagnostics as health check (lightweight RPC)
            request = sysml_pb2.DiagnosticsRequest(model_hash="health_check")
            stub.GetDiagnostics(request, timeout=timeout)
            
            return True
        except grpc.RpcError as e:
            # NOT_FOUND is expected for invalid hash - service is working
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return True
            return False
        except Exception:
            # Could be: service not ready, crashed, or network error
            return False
        finally:
            channel.close()
    
    def _ensure_service(self):
        """Ensure sysml-grpc service is running, with lockfile coordination.
        
        Uses filelock to coordinate between multiple Python processes.
        If service already running, returns immediately.
        Otherwise, acquires lock and starts service.
        
        Raises:
            ConnectionError: If service cannot be started or lockfile timeout
        """
        lockfile_path = _get_lockfile_path()
        lock = FileLock(lockfile_path, timeout=30)
        
        try:
            with lock:
                # Check for stale pidfile (SIGKILL'd process, PID reuse, etc.)
                stale, proc = _is_pidfile_stale()
                if stale:
                    # Clean up stale state (use try/except to handle TOCTOU races)
                    pidfile_path = _get_pidfile_path()
                    refcount_path = _get_refcount_path()
                    try:
                        if os.path.exists(pidfile_path):
                            os.remove(pidfile_path)
                    except FileNotFoundError:
                        pass  # Another process already removed it
                    try:
                        if os.path.exists(refcount_path):
                            os.remove(refcount_path)
                    except FileNotFoundError:
                        pass  # Another process already removed it
                
                # Check if service already running (another process may have started it)
                if self._probe_service(self.host, self.port):
                    # Service running - increment refcount and return
                    self._origin = (
                        f"service already listening on {self._address}, "
                        f"not started by this client"
                    )
                    _increment_refcount()
                    self._holds_refcount = True
                    atexit.register(self._cleanup_service)
                    return
                
                # Get binary path
                binary_path = ensure_binary()
                if not os.path.exists(binary_path):
                    raise ConnectionError(f"Binary not found after download: {binary_path}")
                self._origin = f"{binary_path}, started by this client"
                
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
                        os.makedirs(os.path.dirname(pidfile_path), exist_ok=True)
                        with open(pidfile_path, 'w') as f:
                            f.write(f"{process.pid}\n")
                        
                        # Increment refcount
                        _increment_refcount()
                        self._holds_refcount = True
                        
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
                raise ConnectionError(f"Service failed to start within {max_retries * retry_delay}s")
                
        except Timeout:
            raise ConnectionError(
                f"Timeout acquiring service lockfile after 30s. "
                f"Another process may be starting the service."
            )
    
    def _cleanup_service(self):
        """Clean up service process with reference counting."""
        if self._cleaned_up or not self._holds_refcount:
            return
        self._cleaned_up = True
        self._holds_refcount = False
        
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
