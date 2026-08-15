"""Connection class for communicating with sysml-grpc service."""

import atexit
import grpc
import os
import psutil
import subprocess
import time
import warnings
from filelock import FileLock, Timeout
from pysysml.proto import sysml_pb2, sysml_pb2_grpc
from pysysml.model import Model
from pysysml.binary import cached_release, ensure_binary, resolve_latest_version
from pysysml.capabilities import (
    CAPABILITY_CONVERT,
    CAPABILITY_QUERY,
    CAPABILITY_VERIFICATION,
    ServerInfo,
    mismatch_reason,
    require,
    upgrade_remedy,
)
from pysysml.conversion import Conversion
from pysysml.diagnostic import Diagnostic
from pysysml.errors import (
    ChecksumMismatchError,
    ConnectionError,
    ConversionError,
    ExecutionError,
    ModelFileNotFoundError,
    ModelNotFoundError,
    PySysMLError,
    StaleServiceError,
    UnsupportedValueError,
    WrongKindError,
    from_rpc_error,
    translate_rpc_errors,
)
from pysysml.query import build_query, elements_of
from pysysml.values import value_to_python
from pysysml.verdict import CalcResult, Verdict


#: Port the service listens on when a caller names none.
DEFAULT_PORT = 50051


def split_target(host, port=None):
    """Split a ``host:port`` string written as the host into host and port.

    ``connect("localhost:50123")`` names an address, not a hostname, so it is
    read as one rather than building ``localhost:50123:50051`` and reporting the
    service unreachable at an address nobody asked for.

    Args:
        host (str): Hostname, or a ``host:port`` address
        port (int, optional): Port; None is no port given, so an address's own
            port stands and a plain hostname gets DEFAULT_PORT

    Returns:
        tuple[str, int]: The host and port to connect to

    Raises:
        ValueError: If the address's port is not a number, or disagrees with a
            port also given
    """
    if not isinstance(host, str) or ':' not in host:
        return host, DEFAULT_PORT if port is None else port

    # A bare IPv6 address has colons of its own; only a bracketed one, or a
    # single colon, names a port.
    if host.startswith('['):
        closing = host.find(']')
        if closing == -1 or not host[closing + 1:].startswith(':'):
            return host, DEFAULT_PORT if port is None else port
        name, written = host[:closing + 1], host[closing + 2:]
    elif host.count(':') > 1:
        return host, DEFAULT_PORT if port is None else port
    else:
        name, written = host.split(':', 1)

    if not written.isdigit():
        raise ValueError(
            f"host={host!r} names no port this client can read; pass "
            f"host and port separately, as connect({name!r}, <port>)"
        )
    embedded = int(written)
    if port is not None and port != embedded:
        raise ValueError(
            f"host={host!r} and port={port} name different ports; give the "
            f"port once"
        )
    return name, embedded


def _failure_of(message, failure_reason, diagnostics):
    """Build the error for a failure the service classified.

    The kind is read from the response's typed reason, so a wrong request and a
    condition that could not be evaluated are told apart without reading the
    message text.
    """
    if failure_reason == sysml_pb2.FAILURE_REASON_WRONG_KIND:
        return WrongKindError(message, diagnostics=diagnostics)
    return ExecutionError(message, diagnostics=diagnostics)


def _raise_wrong_kind(pb_verdict, diagnostics):
    """Raise when a verdict reports the named element is of another kind.

    Such a verdict is no answer about the model, so it is raised rather than
    returned as an undecided one; every other failure stays in verdict.error.
    """
    if pb_verdict.failure_reason == sysml_pb2.FAILURE_REASON_WRONG_KIND:
        raise WrongKindError(pb_verdict.error, diagnostics=diagnostics)


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


def _read_refcount():
    """How many clients hold a reference to the service. Caller must hold lockfile."""
    refcount_path = _get_refcount_path()
    try:
        with open(refcount_path, 'r') as f:
            return int(f.read().strip())
    except (FileNotFoundError, ValueError):
        return 0


def _remove_service_state():
    """Forget the pidfile and refcount of a service that is gone.

    Caller must hold the lockfile.
    """
    for path in (_get_pidfile_path(), _get_refcount_path()):
        try:
            os.remove(path)
        except FileNotFoundError:
            pass


def _stop_process(process):
    """Stop a service process, killing it if it will not terminate.

    Args:
        process (psutil.Process): The process to stop

    Returns:
        bool: Whether the process is gone afterwards
    """
    try:
        process.terminate()
        process.wait(timeout=5)
    except psutil.NoSuchProcess:
        pass
    except (psutil.TimeoutExpired, psutil.AccessDenied):
        try:
            process.kill()
            process.wait(timeout=5)
        except psutil.NoSuchProcess:
            pass
        except (psutil.TimeoutExpired, psutil.AccessDenied):
            return False
    return True


class Connection:
    """Manages connection to sysml-grpc service.
    
    Phase 3: Auto-start capabilities - service can be started automatically.
    
    Attributes:
        host (str): Service hostname
        port (int): Service port
    """
    
    def __init__(self, host='localhost', port=None, auto_start=True,
                 version=None, require_capabilities=None):
        """Initialize connection to sysml-grpc service.
        
        Args:
            host (str): Service hostname, or a ``host:port`` address, whose port
                is used when no separate port is given (default: 'localhost')
            port (int, optional): Service port (default: 50051)
            auto_start (bool): If True, automatically start service if not running (default: True)
            version (str, optional): Release tag the service must report, or
                'latest'. Defaults to $PYSYSML_GRPC_VERSION, the same tag the
                binary cache is checked against; without either, whatever
                release answers is accepted. Checked whether the service is
                started here or managed by the caller, though only a service
                this client started can be replaced.
            require_capabilities (iterable, optional): Capability names the
                service must report, checked once at connect time rather than
                when the first call needing one is made

        Raises:
            ValueError: If host names a port that is unreadable or disagrees
                with port
            StaleServiceError: If another release is already listening on the
                address and this client may not stop it
            MissingCapabilityError: If the service lacks a required capability
        """
        host, port = split_target(host, port)
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
        self._version = version or os.environ.get('PYSYSML_GRPC_VERSION') or None
        self._required_capabilities = frozenset(require_capabilities or ())
        
        # Auto-start service if requested
        if auto_start:
            self._ensure_service()
        
        self._channel = grpc.insecure_channel(self._address)
        self._stub = sysml_pb2_grpc.SysMLServiceStub(self._channel)
        try:
            if not auto_start:
                self._check_managed_service_release()
            for capability in sorted(self._required_capabilities):
                require(self.server_info(), capability, upgrade_remedy(capability))
        except BaseException:
            # A refused connection is never returned, so nothing else can
            # release its channel or the reference it took on the service.
            self.close()
            raise
    
    def _check_managed_service_release(self):
        """Check the release of a service this client was told not to start.

        Nothing can be started in its place, so the mismatch is reported rather
        than acted on; ownership does not matter, since nothing is stopped.
        """
        required = self._required_release()
        if required is None:
            return
        info = self.server_info()
        reason = mismatch_reason(info, version=required)
        if reason is not None:
            raise StaleServiceError(
                self._address, reason,
                f"reach a {required} service with connect(port=<its port>), or "
                f"accept what is running by passing version=None and unsetting "
                f"$PYSYSML_GRPC_VERSION",
                info=info,
            )

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
            upgrade_remedy(CAPABILITY_CONVERT),
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

    def query(self, model_hash, payload=None, scope=None, select=None, where=None):
        """Run a SysML v2 API & Services Query over a loaded model.

        The query is the standard's JSON object, so a cookbook payload works
        verbatim, or the same thing as keywords. See :mod:`pysysml.query`.

        Args:
            model_hash (str): Hash of the model to query
            payload (dict, optional): The standard's ``Query`` object
            scope (list, optional): Elements to consider; empty is the whole model
            select (list, optional): Properties to report; empty reports every one
            where (dict, optional): Constraint to filter by

        Returns:
            list[QueryElement]: The elements selected, in declaration order

        Raises:
            QueryError: If the query is not one the standard's model describes
            MissingCapabilityError: If the service cannot query
            InvalidRequestError: If a property or scope is unknown to the service
            ModelNotFoundError: If the model is no longer cached
        """
        require(
            self.server_info(),
            CAPABILITY_QUERY,
            upgrade_remedy(CAPABILITY_QUERY),
        )
        request = sysml_pb2.QueryRequest(
            model_hash=model_hash,
            query=build_query(payload, scope=scope, select=select, where=where),
        )
        with translate_rpc_errors():
            response = self._stub.Query(request)
        return elements_of(response)

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
            WrongKindError: If symbol_id names an element that is not a
                constraint, which is a wrong request rather than a verdict
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
            WrongKindError: If symbol_id names an element that is not a
                requirement
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
            WrongKindError: If symbol_id names an element that can state no
                satisfaction assertion
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
            raise _failure_of(
                response.error, response.failure_reason, diagnostics
            )
        for pb_verdict in response.verdicts:
            _raise_wrong_kind(pb_verdict, diagnostics)
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
            WrongKindError: If symbol_id names an element that is not a calc
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
            raise _failure_of(
                response.error, response.failure_reason, diagnostics
            )

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
            upgrade_remedy(CAPABILITY_VERIFICATION),
        )

    def _verdict_of(self, response):
        """Wrap a single-verdict verification response, raising its failure."""
        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ExecutionError(response.error, diagnostics=diagnostics)
        _raise_wrong_kind(response.verdict, diagnostics)
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
    
    def _running_service_info(self, timeout=5.0):
        """Ask the service already listening what it is, over a channel of its own.

        Args:
            timeout (float): RPC timeout in seconds

        Returns:
            ServerInfo or None: What it reported; ``answered`` is False when it
                predates the handshake, which is itself an answer, and None when
                the call failed, so nothing was learned
        """
        channel = grpc.insecure_channel(self._address)
        try:
            stub = sysml_pb2_grpc.SysMLServiceStub(channel)
            response = stub.GetServerInfo(
                sysml_pb2.ServerInfoRequest(), timeout=timeout
            )
        except grpc.RpcError as e:
            if e.code() != grpc.StatusCode.UNIMPLEMENTED:
                return None
            return ServerInfo(
                version='',
                capabilities=frozenset(),
                answered=False,
                origin=self._origin,
            )
        finally:
            channel.close()
        return ServerInfo(
            version=response.version,
            capabilities=frozenset(response.capabilities),
            answered=True,
            origin=self._origin,
        )

    def _required_release(self):
        """Release tag the service must report, resolved, or None if none is required.

        An unresolvable 'latest' requires nothing, as for the binary cache.
        """
        if self._version != 'latest':
            return self._version
        try:
            return resolve_latest_version()
        except ConnectionError:
            return None

    def _started_by_this_client(self):
        """The service process this client's records say it started on this port.

        Ownership is read from the pidfile and the process's command line, so a
        service the user started is never taken for one of ours.

        Returns:
            psutil.Process or None: The owned process, or None if there is none
        """
        stale, process = _is_pidfile_stale()
        if stale or process is None:
            return None
        try:
            cmdline = process.cmdline()
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            return None
        if cmdline[-2:] != ['-port', str(self.port)]:
            return None
        return process

    def _adopt_running_service(self):
        """Take over the service already listening, or make room for the one asked for.

        Caller must hold the lockfile.

        Returns:
            bool: True when the running service was adopted, False when it was
                stopped and one must now be started in its place

        Raises:
            StaleServiceError: If it is not the service asked for and this
                client may not, or cannot usefully, stop it
        """
        required = self._required_release()
        info = self._running_service_info()
        if info is None:
            # A handshake that failed says nothing about the service, so it is
            # neither trusted for the rest of the session nor stopped.
            if required is None and not self._required_capabilities:
                self._hold_running_service(None)
                return True
            raise StaleServiceError(
                self._address,
                "the GetServerInfo call to it failed, so it cannot be shown to "
                "be the service that was asked for",
                "retry, since the service may answer next time; or reach "
                "another one with connect(port=<other port>)",
            )

        release_reason = mismatch_reason(info, version=required)
        capability_reason = mismatch_reason(
            info, capabilities=self._required_capabilities
        )
        if release_reason is None and capability_reason is None:
            self._hold_running_service(info)
            return True
        if release_reason is None:
            # Only another release can report other capabilities, so a service
            # that is the release asked for is reported, never stopped.
            missing = sorted(
                c for c in self._required_capabilities if not info.has(c)
            )
            raise StaleServiceError(
                self._address, capability_reason, upgrade_remedy(missing[0]),
                info=info,
            )

        reason = "; ".join(
            r for r in (release_reason, capability_reason) if r is not None
        )
        self._replace_mismatched_service(reason, info, required)
        return False

    def _hold_running_service(self, info):
        """Take a reference on the service already listening.

        Caller must hold the lockfile. ``info`` is None when the handshake could
        not be made, so it is asked again over the connection's own channel.
        """
        self._server_info = info
        _increment_refcount()
        self._holds_refcount = True
        atexit.register(self._cleanup_service)

    def _replacement_serves(self, required):
        """Whether starting the binary would serve the release asked for.

        Stopping a service to start the same build gains nothing, so a
        replacement that cannot differ is reported instead of made. A download
        failing its checksum is raised, never read as "the same build".
        """
        try:
            ensure_binary(version=self._version)
        except ChecksumMismatchError:
            raise
        except PySysMLError:
            return False
        return cached_release() == required

    def _replace_mismatched_service(self, reason, info, required):
        """Stop a mismatched service, if this client started it and nothing else uses it.

        Caller must hold the lockfile. A service this client cannot show to be
        its own is reported rather than killed: the user may be running it
        deliberately.

        Args:
            reason (str): How the running service differs from the one asked for
            info (ServerInfo): What it reported about itself
            required (str): Release tag the replacement must report

        Raises:
            StaleServiceError: If this client may not stop it, or stopping it
                would only start the same build again
        """
        process = self._started_by_this_client()
        if process is None:
            raise StaleServiceError(
                self._address, reason,
                f"stop the service listening on {self._address} yourself, then "
                f"retry so this client starts the one it asks for; or reach "
                f"another one with connect(port=<other port>); or ask for what "
                f"is running by unsetting $PYSYSML_GRPC_VERSION",
                info=info,
            )
        holders = _read_refcount()
        if holders > 0:
            raise StaleServiceError(
                self._address, reason,
                f"{holders} other pysysml connection(s) still hold this "
                f"service, so it is not this client's to stop; close them and "
                f"retry, or reach a service of your own with "
                f"connect(port=<other port>)",
                info=info,
            )
        if not self._replacement_serves(required):
            raise StaleServiceError(
                self._address, reason,
                f"this client started that service, but the binary it would "
                f"start in its place cannot be shown to be {required}, so "
                f"stopping it would serve the same build again; make {required} "
                f"reachable (or ask for the release you have) and retry",
                info=info,
            )
        if not _stop_process(process):
            raise StaleServiceError(
                self._address, reason,
                f"this client started that service but could not stop it "
                f"(pid {process.pid}); stop it yourself and retry",
                info=info,
            )
        warnings.warn(
            f"replaced the sysml-grpc service this client started on "
            f"{self._address}: {reason}",
            RuntimeWarning,
            stacklevel=2,
        )
        _remove_service_state()

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
                    self._origin = (
                        f"service already listening on {self._address}, "
                        f"not started by this client"
                    )
                    if self._adopt_running_service():
                        return
                
                # Get binary path
                binary_path = ensure_binary(version=self._version)
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
