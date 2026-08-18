"""Connection class for communicating with sysml-grpc service."""

import atexit
import grpc
import json
import os
import psutil
import subprocess
import time
import warnings
from typing import Any, Dict, Tuple
from filelock import FileLock, Timeout
from pysysml.proto import sysml_pb2, sysml_pb2_grpc
from pysysml.model import Model
from pysysml.binary import cached_release, ensure_binary, resolve_latest_version
from pysysml.capabilities import (
    CAPABILITY_CONVERT,
    CAPABILITY_EVALUATE_SUBJECT,
    CAPABILITY_QUERY,
    CAPABILITY_VERIFICATION,
    ServerInfo,
    mismatch_reason,
    require,
    upgrade_remedy,
)
from pysysml.conversion import (
    Conversion,
    EXPERIMENTAL_NOTICE,
    ExperimentalFeatureWarning,
    is_experimental,
)
from pysysml.diagnostic import Diagnostic
from pysysml.enumeration import EnumLiteral
from pysysml.errors import (
    ChecksumMismatchError,
    ConnectionError,
    ConversionError,
    ExecutionError,
    ModelFileNotFoundError,
    ModelNotFoundError,
    PySysMLError,
    StaleServiceError,
    UnpinnedReleaseError,
    UnsupportedValueError,
    WrongKindError,
    from_rpc_error,
    translate_rpc_errors,
)
from pysysml.query import build_query, elements_of
from pysysml.values import Quantity, value_to_python
from pysysml.verdict import CalcResult, Verdict


#: Port the service listens on when a caller names none.
DEFAULT_PORT = 50051

#: A required release not looked up yet, distinct from 'none required'.
_UNRESOLVED = object()

#: Directory the lockfile and ownership records live in. $PYSYSML_STATE_DIR
#: overrides it, so a test or a sandbox can run a service beside another's.
STATE_DIR_ENV = 'PYSYSML_STATE_DIR'

#: Start times are floats read back from JSON, so they are compared to the
#: resolution the platform reports rather than for equality.
_START_TIME_TOLERANCE = 1e-3

#: Seconds a service started here is given to answer, sleeping or probing.
START_TIMEOUT = 2.5

#: Delay before the second probe, doubled up to START_PROBE_MAX_DELAY. The first
#: probe is immediate, so a service answering in milliseconds is not waited out.
START_PROBE_INITIAL_DELAY = 0.01

#: Longest delay between probes, so a slow start costs a bounded probe count.
START_PROBE_MAX_DELAY = 0.25

#: RPC timeout of one probe. A port nothing listens on refuses the connection in
#: about a millisecond, so an immediate probe does not spend this.
START_PROBE_RPC_TIMEOUT = 2.0

#: How long the service started here is given to exit before an answer from an
#: address it cannot yet have bound is attributed to it. One that could not bind
#: exits at once, so this is spent only when something else already answers.
START_CONFIRM_DELAY = 0.5

#: Services this process spawned, keyed by state directory and port, each with
#: the count of live connections holding it. Ownership does not outlive the
#: process that spawned a service, so it is not shared through a file: no other
#: process may stop that service, and this one stops it when its own last
#: reference is released.
_OWNED_SERVICES: Dict[Tuple[str, int], Dict[str, Any]] = {}


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


def _state_dir():
    """Directory the lockfile and ownership records are kept in.

    Returns:
        str: $PYSYSML_STATE_DIR, or ~/.pysysml
    """
    return os.path.expanduser(os.environ.get(STATE_DIR_ENV) or '~/.pysysml')


def _get_lockfile_path(port):
    """Path of the lockfile serializing starts of the service on a port."""
    state_dir = _state_dir()
    os.makedirs(state_dir, exist_ok=True)
    return os.path.join(state_dir, f'sysml-grpc-{port}.lock')


def _get_pidfile_path(port):
    """Path of the ownership record of the service on a port.

    One service listens per port, so its record is named after the port: a
    second service on another port is not the first one's record rewritten.
    """
    return os.path.join(_state_dir(), f'sysml-grpc-{port}.pid')


def _service_key(port):
    """Key a service is held under while this process owns it."""
    return (_state_dir(), port)


def _write_ownership_record(port, pid, create_time):
    """Record which process is the service on a port, and which process spawned it.

    The start time is written beside the pid so the record authenticates the
    process it names: a pid reused by an unrelated process fails the check
    instead of being taken for the service. Caller must hold the lockfile.

    Args:
        port (int): Port the service listens on
        pid (int): Process id of the service
        create_time (float): Start time psutil reports for that process

    Returns:
        dict: The record written
    """
    owner = psutil.Process()
    record = {
        'pid': pid,
        'create_time': create_time,
        'port': port,
        'owner_pid': owner.pid,
        'owner_create_time': owner.create_time(),
    }
    path = _get_pidfile_path(port)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        json.dump(record, f)
    return record


def _read_ownership_record(port):
    """The ownership record for the service on a port, or None when there is none.

    Caller must hold the lockfile.

    Returns:
        dict or None: What was recorded, or None when nothing readable was
    """
    try:
        with open(_get_pidfile_path(port), 'r') as f:
            record = json.load(f)
    except (OSError, ValueError):
        return None
    if not isinstance(record, dict) or not isinstance(record.get('pid'), int):
        return None
    return record


def _started_at(recorded, actual):
    """Whether a recorded start time is the one a process reports."""
    if not isinstance(recorded, (int, float)) or isinstance(recorded, bool):
        return False
    return abs(float(recorded) - actual) <= _START_TIME_TOLERANCE


def _authenticate_record(record):
    """The live service process a record names, or None when it names none.

    Identity is the pid together with the start time recorded for it, so a pid
    the operating system has since handed to an unrelated process is not taken
    for the service and is never signalled.

    Args:
        record (dict): An ownership record

    Returns:
        psutil.Process or None: The process, or None when the record is stale
    """
    try:
        process = psutil.Process(record['pid'])
        create_time = process.create_time()
    except (psutil.Error, KeyError, OSError, TypeError, ValueError):
        return None
    if not _started_at(record.get('create_time'), create_time):
        return None
    return process


def _recorded_service(port):
    """The record for the service on a port and the live process it authenticates.

    Caller must hold the lockfile.

    Returns:
        tuple[dict or None, psutil.Process or None]: The record, and the process
            it names when that process is still the one recorded
    """
    record = _read_ownership_record(port)
    if record is None:
        return (None, None)
    return (record, _authenticate_record(record))


def _recorded_by_this_process(record):
    """Whether this process wrote a record, so the service it names is its own.

    The spawner's start time is checked too: a record left behind by a process
    whose pid this one has since been given is not this process's ownership.
    """
    if record is None or record.get('owner_pid') != os.getpid():
        return False
    return _started_at(record.get('owner_create_time'), psutil.Process().create_time())


def _remove_service_state(port):
    """Forget the ownership record of a service that is gone.

    Caller must hold the lockfile.
    """
    try:
        os.remove(_get_pidfile_path(port))
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
                this client started can be replaced. A caller-managed service
                that is not listening yet is checked at the first call instead.
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
        # Only connections that took a reference may release one on close, and
        # only against the service identity it was taken on.
        self._holds_refcount = False
        self._referenced_service = None
        self._version = version or os.environ.get('PYSYSML_GRPC_VERSION') or None
        self._required_capabilities = frozenset(require_capabilities or ())
        self._resolved_release = _UNRESOLVED
        # A caller-managed service that was not listening yet is checked at the
        # first handshake, so auto_start=False stays free of eager I/O.
        self._check_release_on_handshake = False
        
        # Auto-start service if requested
        if auto_start:
            self._ensure_service()
        
        self._channel = grpc.insecure_channel(self._address)
        self._service = sysml_pb2_grpc.SysMLServiceStub(self._channel)
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

        Nothing can be started in its place, so a mismatch is reported rather
        than acted on; ownership does not matter, since nothing is stopped. A
        service the caller manages may not be listening yet, so one that cannot
        be asked is checked at the first handshake instead of refused here.
        """
        if self._required_release() is None:
            return
        info = self._running_service_info()
        if info is None:
            self._check_release_on_handshake = True
            return
        self._raise_if_release_mismatch(info)

    def _raise_if_release_mismatch(self, info):
        """Report a service that is not the release asked for.

        Raises:
            StaleServiceError: If what it reports differs from the requirement
        """
        required = self._required_release()
        if required is None:
            return
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
        """Close the gRPC channel and release any reference on the service."""
        if self._channel:
            self._channel.close()
        self._cleanup_service()
    
    def __enter__(self):
        """Context manager entry."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()
    
    @property
    def _stub(self):
        """The service stub, with any release check still owed done first.

        Every call goes through here, so a service that came up after the client
        was built is checked at whichever call reaches it first.
        """
        if self._check_release_on_handshake:
            self.server_info()
        return self._service

    def server_info(self):
        """Ask the service what it is and what it supports.

        The answer is cached for the life of the connection: a service does not
        change build while a channel is open to it.

        Returns:
            ServerInfo: Reported version and capabilities. ``answered`` is False
                when the service predates the GetServerInfo RPC, in which case
                it claims no capabilities.

        Raises:
            StaleServiceError: If a release was asked for and this first answer
                shows the service is another one
        """
        if self._server_info is None:
            request = sysml_pb2.ServerInfoRequest()
            try:
                response = self._service.GetServerInfo(request)
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
        # Cleared only once the check passes, so a mismatch keeps being reported
        # instead of the connection turning usable after one error.
        if self._check_release_on_handshake:
            self._raise_if_release_mismatch(self._server_info)
            self._check_release_on_handshake = False
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
        """Write a model out in another of the formats OpenSysML writes.

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

        Warns:
            ExperimentalFeatureWarning: If either format is RDF, whose mapping is
                experimental — see ``docs/reference/rdf-mapping.md``

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
        # Judged from the response, so an inferred format counts and a service
        # too old to mark the conversion is still read as experimental.
        experimental = response.experimental or is_experimental(
            response.from_format, response.to_format
        )
        notice = response.experimental_notice or (
            EXPERIMENTAL_NOTICE if experimental else ""
        )
        if experimental:
            # Warned before the error is raised: a refusal is the mapping's
            # experimental behavior, not a reason to say nothing about it.
            warnings.warn(notice, ExperimentalFeatureWarning, stacklevel=2)
        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ConversionError(response.error, diagnostics=diagnostics)
        return Conversion(
            content=response.content,
            from_format=response.from_format,
            to_format=response.to_format,
            diagnostics=diagnostics,
            experimental=experimental,
            experimental_notice=notice,
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
    
    def eval(
        self,
        expression,
        model_hash,
        context_symbol_id=None,
        subject_symbol_id=None,
    ):
        """Evaluate a SysML expression.
        
        Args:
            expression (str): SysML expression (e.g., "2 + 2")
            model_hash (str): Hash from ParseFile response
            context_symbol_id (str, optional): Symbol FQN for context scope
            subject_symbol_id (str, optional): FQN of a part/usage to
                instantiate and evaluate against, so a feature reads that
                object's value rather than the declared default
            
        Returns:
            Value from expression (int, float, bool, str, Instance, etc.)
            
        Raises:
            ExecutionError: If evaluation fails
            ModelNotFoundError: If the service no longer holds the model
            UnsupportedValueError: If the result cannot be represented on the wire
        """
        if subject_symbol_id:
            # A service that ignores the subject would answer with the declared
            # default, which is indistinguishable from the object's own value.
            require(
                self.server_info(),
                CAPABILITY_EVALUATE_SUBJECT,
                upgrade_remedy(CAPABILITY_EVALUATE_SUBJECT),
            )

        req = sysml_pb2.EvaluateRequest(
            model_hash=model_hash,
            expression=expression,
            context_symbol_id=context_symbol_id or "",
            subject_symbol_id=subject_symbol_id or "",
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
        elif isinstance(py_value, Quantity):
            return sysml_pb2.Value(quantity=py_value.to_pb())
        elif isinstance(py_value, EnumLiteral):
            return sysml_pb2.Value(enum_literal=sysml_pb2.EnumLiteral(
                literal_id=py_value.literal_id,
                enumeration_id=py_value.enumeration_id,
                name=py_value.name,
            ))
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

        Mirrors Instance.feature_values: one value the wire format cannot represent must
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
        """Release tag the service must report, or None if none is required.

        An unresolvable 'latest' requires nothing, as for the binary cache. The
        answer is resolved once, so every check of a connection uses the same one.
        """
        if self._resolved_release is _UNRESOLVED:
            if self._version != 'latest':
                self._resolved_release = self._version
            else:
                try:
                    self._resolved_release = resolve_latest_version()
                except ConnectionError:
                    self._resolved_release = None
        return self._resolved_release

    def _started_by_this_client(self):
        """The service process this process spawned on this port, if it still runs.

        Ownership is read from the record this process wrote, and the process it
        names is authenticated by its start time, so neither a service someone
        else started nor an unrelated process holding a reused pid is ever taken
        for one of ours. Caller must hold the lockfile.

        Returns:
            psutil.Process or None: The owned process, or None if there is none
        """
        record, process = _recorded_service(self.port)
        if process is None or not _recorded_by_this_process(record):
            return None
        return process

    def _owned_service(self):
        """The bookkeeping for a service this process spawned on this port.

        Returns:
            dict or None: Its recorded identity and reference count, or None
                when this process owns no service on the port
        """
        return _OWNED_SERVICES.get(_service_key(self.port))

    def _take_ownership_reference(self):
        """Hold the service this process spawned until this connection is closed.

        Caller must hold the lockfile. The last reference released stops the
        service, so every reference taken is released exactly once.
        """
        owned = self._owned_service()
        if owned is None or self._holds_refcount:
            return
        owned['refs'] += 1
        self._holds_refcount = True
        self._referenced_service = (owned['pid'], owned['create_time'])
        atexit.register(self._cleanup_service)

    def _adopt_running_service(self):
        """Take over the service already listening, or make room for the one asked for.

        Caller must hold the lockfile.

        Returns:
            bool: True when the running service was adopted, False when it was
                stopped and one must now be started in its place

        Raises:
            MissingCapabilityError: If it is the release asked for but lacks a
                required capability, which no replacement could add
            StaleServiceError: If it is not the service asked for and this
                client may not, or cannot usefully, stop it
        """
        required = self._required_release()
        info = self._running_service_info()
        if info is None:
            # A handshake that failed says nothing about the service, so it is
            # neither trusted for the rest of the session nor stopped.
            if required is None:
                # A required capability is checked over the connection's own
                # channel, which asks again rather than trusting a failed call,
                # and no replacement could add one without another release.
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
            # that is the release asked for is reported, never stopped. The
            # shortfall is the documented one, whoever started the service.
            for capability in sorted(self._required_capabilities):
                require(info, capability, upgrade_remedy(capability))

        reason = "; ".join(
            r for r in (release_reason, capability_reason) if r is not None
        )
        self._replace_mismatched_service(reason, info, required)
        return False

    def _hold_running_service(self, info):
        """Attach to the service already listening, taking ownership only if it is ours.

        A service this process did not spawn is used and left alone: no
        reference is taken and no record is written, so closing this connection
        never stops a service somebody else owns. Caller must hold the lockfile.
        ``info`` is None when the handshake could not be made, so it is asked
        again over the connection's own channel.
        """
        self._server_info = info
        if self._started_by_this_client() is None:
            return
        self._take_ownership_reference()

    def _replacement_serves(self, required):
        """Whether starting the binary would serve the release asked for.

        Stopping a service to start the same build gains nothing, so a
        replacement that cannot differ is reported instead of made. A download
        failing its checksum is raised, never read as "the same build"; a
        release this pysysml pins nothing for is only a build it cannot get.
        """
        try:
            ensure_binary(version=self._version)
        except UnpinnedReleaseError:
            return False
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
        owned = self._owned_service()
        holders = owned['refs'] if owned else 0
        if holders > 0:
            raise StaleServiceError(
                self._address, reason,
                f"{holders} other pysysml connection(s) in this process still "
                f"hold this service, so it is not this client's to stop; close "
                f"them and retry, or reach a service of your own with "
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
        _OWNED_SERVICES.pop(_service_key(self.port), None)
        _remove_service_state(self.port)

    def _ensure_service(self):
        """Ensure sysml-grpc service is running, with lockfile coordination.
        
        Uses filelock to coordinate between multiple Python processes.
        If service already running, returns immediately.
        Otherwise, acquires lock and starts service, which is probed at once and
        then on a backoff bounded by START_TIMEOUT.
        
        Raises:
            ConnectionError: If service cannot be started or lockfile timeout
        """
        lockfile_path = _get_lockfile_path(self.port)
        lock = FileLock(lockfile_path, timeout=30)
        
        try:
            with lock:
                # A record whose process is gone, or whose pid another process
                # now holds, is a service that crashed: it is cleaned, never
                # trusted, and the process it named is never signalled.
                record, process = _recorded_service(self.port)
                if record is not None and process is None:
                    _OWNED_SERVICES.pop(_service_key(self.port), None)
                    _remove_service_state(self.port)
                
                # Check if service already running (another process may have started it)
                answered_before_start = self._probe_service(self.host, self.port)
                if answered_before_start:
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
                
                # Wait for service to become healthy, probing at once and then
                # backing off: the service answers in milliseconds, so sleeping
                # first would be the whole cost of starting it.
                deadline = time.monotonic() + START_TIMEOUT
                delay = START_PROBE_INITIAL_DELAY
                # An answer is the started service's own unless something was
                # already answering the address and has not stopped since.
                its_own_answer = not answered_before_start

                while True:
                    # One that could not bind the address exits and leaves the
                    # old service answering the probe.
                    if process.poll() is not None:
                        self._service_started_here_died(process.poll())
                    # A port that accepts without answering would spend a probe's
                    # whole timeout, so no probe outlives the deadline either.
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        break
                    if self._probe_service(
                        self.host, self.port,
                        timeout=min(START_PROBE_RPC_TIMEOUT, remaining),
                    ):
                        if not its_own_answer:
                            self._wait_for_a_service_that_could_not_bind(process)
                        self._own_spawned_service(process)
                        return
                    its_own_answer = True
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        break
                    time.sleep(min(delay, remaining))
                    delay = min(delay * 2, START_PROBE_MAX_DELAY)
                
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
                raise ConnectionError(f"Service failed to start within {START_TIMEOUT}s")
                
        except Timeout:
            raise ConnectionError(
                f"Timeout acquiring service lockfile after 30s. "
                f"Another process may be starting the service."
            )
    
    def _wait_for_a_service_that_could_not_bind(self, process):
        """Report a started service that exits rather than serving the address.

        A service that could not bind the address exits at once, so an answer
        the started one cannot yet have given is attributed to it only after it
        is given START_CONFIRM_DELAY to exit. Only an address something already
        answers is waited on.

        Args:
            process (subprocess.Popen): The service this connection started

        Raises:
            StaleServiceError: If it exited while another service answers
        """
        try:
            exit_code = process.wait(timeout=START_CONFIRM_DELAY)
        except subprocess.TimeoutExpired:
            return
        self._service_started_here_died(exit_code)

    def _own_spawned_service(self, process):
        """Record the service started here, authenticated, and hold a reference.

        Args:
            process (subprocess.Popen): The service this connection started

        Raises:
            StaleServiceError: If it exited after answering and another service
                still serves the address
            ConnectionError: If it exited after answering and nothing does
        """
        try:
            spawned = psutil.Process(process.pid)
            create_time = spawned.create_time()
        except psutil.Error:
            # Gone between answering and being recorded, so whatever serves the
            # address is not what was started here.
            self._service_started_here_died(process.poll())
        record = _write_ownership_record(self.port, process.pid, create_time)
        _OWNED_SERVICES[_service_key(self.port)] = {
            'pid': record['pid'],
            'create_time': record['create_time'],
            'refs': 0,
        }
        self._take_ownership_reference()

    def _service_started_here_died(self, exit_code):
        """Report a service started here that exited instead of serving.

        Args:
            exit_code (int): What it exited with

        Raises:
            StaleServiceError: If another service still serves the address, so
                returning would talk to the one just refused
            ConnectionError: If nothing serves the address
        """
        self._process = None
        if self._probe_service(self.host, self.port, timeout=START_PROBE_RPC_TIMEOUT):
            raise StaleServiceError(
                self._address,
                f"the service started here exited ({exit_code}) while another "
                f"one kept serving the address, which is therefore not the "
                f"service that was asked for",
                f"stop whatever holds {self._address} yourself, then retry; or "
                f"reach another one with connect(port=<other port>)",
            )
        raise ConnectionError(
            f"Service exited with code {exit_code} without serving {self._address}"
        )

    def _cleanup_service(self):
        """Release this connection's reference, stopping the service if it was the last.

        Only a connection that took a reference releases one, and only a service
        this process spawned is ever stopped. The reference is dropped before the
        service is stopped, so a second call cannot stop it twice.
        """
        if self._cleaned_up or not self._holds_refcount:
            return
        self._cleaned_up = True
        self._holds_refcount = False
        self._process = None

        key = _service_key(self.port)
        owned = _OWNED_SERVICES.get(key)
        # A service that crashed is replaced under the same key, and a reference
        # taken on the one that died is not a reference on its replacement.
        if owned is None or (owned['pid'], owned['create_time']) != self._referenced_service:
            return
        owned['refs'] -= 1
        if owned['refs'] > 0:
            return
        del _OWNED_SERVICES[key]

        lock = FileLock(_get_lockfile_path(self.port), timeout=5)
        with lock:
            record, process = _recorded_service(self.port)
            if record is None or (record['pid'], record['create_time']) != (
                owned['pid'], owned['create_time']
            ):
                # Another service holds the port now; its record is not ours to
                # remove and its process is not ours to stop.
                return
            if process is not None:
                _stop_process(process)
            _remove_service_state(self.port)
