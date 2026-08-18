"""Tests for what happens when another release is already listening.

A cached binary is release-aware, but the process already serving the port is
what actually answers, so the same check has to be made against it. Each test
runs a real gRPC server answering the handshake like a particular build, and
drives ``Connection`` against it with a release asked for.
"""

import json
import os
import socket
import subprocess
import sys
import tempfile
from concurrent import futures

import grpc
import psutil
import pytest

from opensysml.capabilities import (
    CAPABILITY_CONVERT,
    CAPABILITY_QUERY,
    MissingCapabilityError,
    ServerInfo,
    mismatch_reason,
)
from opensysml.connection import (
    Connection,
    _OWNED_SERVICES,
    _get_pidfile_path,
    _service_key,
    _write_ownership_record,
)
from opensysml.errors import ChecksumMismatchError, ConnectionError, StaleServiceError
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class OldService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc of an earlier release, healthy and answering."""

    def __init__(self, version="v0.0.5", capabilities=(CAPABILITY_CONVERT,),
                 answers_handshake=True, handshake_failures=0):
        self._version = version
        self._capabilities = list(capabilities)
        self._answers_handshake = answers_handshake
        self._handshake_failures = handshake_failures

    def GetServerInfo(self, request, context):
        if not self._answers_handshake:
            context.abort(grpc.StatusCode.UNIMPLEMENTED, "unknown method GetServerInfo")
        if self._handshake_failures > 0:
            self._handshake_failures -= 1
            context.abort(grpc.StatusCode.INTERNAL, "busy")
        return sysml_pb2.ServerInfoResponse(
            version=self._version, capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        # The health probe reads NOT_FOUND for an unknown hash as "up".
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")


@pytest.fixture
def running_service():
    """Start an OldService on an ephemeral port; yields a factory of ports."""
    servers = []

    def start(**kwargs):
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        sysml_pb2_grpc.add_SysMLServiceServicer_to_server(OldService(**kwargs), server)
        port = server.add_insecure_port("localhost:0")
        server.start()
        servers.append(server)
        return port

    yield start
    for server in servers:
        server.stop(None)


@pytest.fixture
def tmp_home(tmp_path, monkeypatch):
    """Isolate HOME and the state dir, so ownership records are this test's own."""
    home = tmp_path / "home"
    home.mkdir()
    monkeypatch.setenv("HOME", str(home))
    monkeypatch.setenv("OPENSYSML_STATE_DIR", str(home / ".opensysml"))
    before = set(_OWNED_SERVICES)
    yield home
    # Drop only what this test recorded: another test's connection still needs
    # the bookkeeping for the service it spawned.
    for key in set(_OWNED_SERVICES) - before:
        del _OWNED_SERVICES[key]


def test_a_foreign_service_of_another_release_is_reported_not_killed(
    running_service, tmp_home, monkeypatch
):
    """The user's own service is left running, and the mismatch is raised.

    Reusing it would serve the client an older build, whose first newer call
    fails as a MissingCapabilityError naming a capability the release asked for
    does have — the wrong error, several calls later.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    error = excinfo.value
    assert error.address == f"localhost:{port}"
    assert "v0.0.5" in error.reason and "v0.0.7" in error.reason
    assert f"stop the service listening on localhost:{port} yourself" in error.remedy
    assert error.info.version == "v0.0.5"
    # Still serving: nothing was killed.
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION")
    with Connection(port=port, auto_start=False) as conn:
        assert conn.server_info().version == "v0.0.5"
    assert not os.path.exists(_get_pidfile_path(port))
    assert _service_key(port) not in _OWNED_SERVICES


def test_a_foreign_service_that_cannot_say_what_it_is_is_reported(
    running_service, tmp_home, monkeypatch
):
    """A service that cannot say what it is cannot be the release asked for."""
    port = running_service(answers_handshake=False)
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    assert "did not answer GetServerInfo" in excinfo.value.reason


def test_a_foreign_service_lacking_a_required_capability_is_reported(
    running_service, tmp_home, monkeypatch
):
    """A missing capability is the documented error, whoever started the service.

    Which process happens to be listening must not change the class a caller
    has to catch for one condition.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(capabilities=[CAPABILITY_CONVERT])

    with pytest.raises(MissingCapabilityError) as excinfo:
        Connection(port=port, require_capabilities=[CAPABILITY_QUERY])

    assert CAPABILITY_QUERY in str(excinfo.value)


def test_a_handshake_that_fails_is_not_taken_for_an_answer(
    running_service, tmp_home, monkeypatch
):
    """A call that failed says nothing, so it is neither cached nor trusted.

    Recording it as a service reporting no capabilities would refuse every
    capability-gated call for the life of the connection.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(version="v0.0.5", capabilities=[CAPABILITY_QUERY],
                           handshake_failures=1)

    with Connection(port=port) as conn:
        info = conn.server_info()
        assert info.version == "v0.0.5"
        assert info.has(CAPABILITY_QUERY)


def test_a_capability_asked_for_survives_a_handshake_that_fails_once(
    running_service, tmp_home, monkeypatch
):
    """A capability asked for is settled by asking again, not by refusing.

    No release was asked for, so nothing could be started in place of this
    service anyway, and which error a caller catches must not turn on RPC luck.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(capabilities=[CAPABILITY_QUERY], handshake_failures=1)

    with Connection(port=port, require_capabilities=[CAPABILITY_QUERY]) as conn:
        assert conn.server_info().has(CAPABILITY_QUERY)


def test_a_handshake_that_fails_is_reported_when_a_release_was_asked_for(
    running_service, tmp_home, monkeypatch
):
    """A service that could not be asked is reported, not stopped."""
    port = running_service(handshake_failures=1)
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    assert "GetServerInfo call to it failed" in excinfo.value.reason


def test_a_refused_connection_releases_what_it_took(tmp_home, monkeypatch):
    """A connection refused for a missing capability holds no reference afterwards.

    The connection is never returned, so nothing else can close its channel or
    release the reference it took on the service it started.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = _free_port()
    monkeypatch.setattr(
        "opensysml.connection.ensure_binary",
        lambda **kwargs: _fake_service_binary(capabilities=()),
    )

    with pytest.raises(MissingCapabilityError):
        Connection(port=port, require_capabilities=[CAPABILITY_QUERY])

    assert _service_key(port) not in _OWNED_SERVICES
    assert not os.path.exists(_get_pidfile_path(port))


def test_a_service_the_caller_manages_is_checked_too(
    running_service, tmp_home, monkeypatch
):
    """A release asked for is checked even when this client starts nothing.

    Reporting the mismatch needs no ownership, since nothing is stopped; only
    replacing the service does.
    """
    port = running_service(version="v0.0.5")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port, auto_start=False, version="v0.0.7")

    assert "v0.0.5" in excinfo.value.reason and "v0.0.7" in excinfo.value.reason
    assert _service_key(port) not in _OWNED_SERVICES
    with Connection(port=port, auto_start=False) as conn:
        assert conn.server_info().version == "v0.0.5"


def test_the_newest_release_is_looked_up_once_per_connection(
    running_service, tmp_home, monkeypatch
):
    """A lookup of 'latest' that later fails cannot turn the check off.

    Resolving it per check would let a flaky second lookup read as "no release
    asked for", and would cost a lookup on every check besides.
    """
    port = running_service(version="v0.0.5")
    lookups = []

    def resolve_latest_version():
        lookups.append(None)
        if len(lookups) > 1:
            raise ConnectionError("release lookup unavailable")
        return "v0.0.7"

    monkeypatch.setattr(
        "opensysml.connection.resolve_latest_version", resolve_latest_version
    )

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port, auto_start=False, version="latest")

    assert "v0.0.5" in excinfo.value.reason and "v0.0.7" in excinfo.value.reason
    assert len(lookups) == 1


def test_a_service_the_caller_has_not_started_yet_is_checked_at_the_first_call(
    running_service, tmp_home, monkeypatch
):
    """auto_start=False stays lazy: an unreachable service is not refused early.

    Callers build the client before starting their own service, so the release
    is checked once the service answers rather than at construction.
    """
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    conn = Connection(port=_free_port(), auto_start=False)

    try:
        with pytest.raises(ConnectionError):
            conn.server_info()
    finally:
        conn.close()

    port = running_service(version="v0.0.5")
    # As if that service had come up only after the client was built.
    monkeypatch.setattr(
        Connection, "_running_service_info", lambda self, timeout=5.0: None
    )
    conn = Connection(host=f"localhost:{port}", auto_start=False)
    try:
        with pytest.raises(StaleServiceError) as excinfo:
            conn.server_info()
        assert "v0.0.5" in excinfo.value.reason
        # A caller that carries on past the first refusal is refused again,
        # rather than served by the release it did not ask for.
        with pytest.raises(StaleServiceError):
            conn.server_info()
    finally:
        conn.close()


def test_a_deferred_check_is_made_by_whichever_call_comes_first(
    running_service, tmp_home, monkeypatch
):
    """Any call answers a check still owed, not only the ones asking what it is.

    Scripts load and evaluate without ever asking for capabilities, so a check
    reached only through server_info() would never be made for them.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")
    # As if that service had come up only after the client was built.
    monkeypatch.setattr(
        Connection, "_running_service_info", lambda self, timeout=5.0: None
    )

    conn = Connection(port=port, auto_start=False)
    try:
        with pytest.raises(StaleServiceError):
            conn.load("/does/not/matter.sysml")
    finally:
        conn.close()


def test_a_matching_service_is_adopted(running_service, tmp_home, monkeypatch):
    """A running service that is what was asked for is used, and left alone.

    This client did not spawn it, so it takes no ownership of it: nothing is
    recorded, and closing the connection cannot stop somebody else's service.
    """
    port = running_service(version="v0.0.7", capabilities=[CAPABILITY_CONVERT])
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with Connection(port=port, require_capabilities=[CAPABILITY_CONVERT]) as conn:
        assert conn.server_info().version == "v0.0.7"
        assert _service_key(port) not in _OWNED_SERVICES
        assert not os.path.exists(_get_pidfile_path(port))

    # Attaching left nothing behind, and the service is still serving.
    assert _service_key(port) not in _OWNED_SERVICES
    assert not os.path.exists(_get_pidfile_path(port))
    with Connection(port=port, auto_start=False) as conn:
        assert conn.server_info().version == "v0.0.7"


def test_a_service_asked_for_no_release_is_adopted_whatever_it_is(
    running_service, tmp_home, monkeypatch
):
    """With no release asked for, whichever build answers is accepted.

    This is the same rule the binary cache follows: a binary the user put there
    is left alone until a client says which release it needs.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(version="v0.0.5")

    with Connection(port=port) as conn:
        assert conn.server_info().version == "v0.0.5"


def test_a_service_this_client_started_is_replaced(
    running_service, tmp_home, monkeypatch
):
    """A mismatched service this client started is stopped and started again.

    Ownership is what makes stopping it this client's call: the pidfile records
    the process, and no other connection holds a reference to it.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _own_service(port, owned.pid)

    # Starting the replacement then fails on the binary, which is beside the
    # point: what this test is about is that the mismatched one was stopped.
    monkeypatch.setattr(
        "opensysml.connection.ensure_binary", lambda **kwargs: "/nonexistent/sysml-grpc"
    )
    monkeypatch.setattr("opensysml.connection.cached_release", lambda: "v0.0.7")

    with pytest.warns(RuntimeWarning, match="replaced the sysml-grpc service"):
        with pytest.raises(ConnectionError, match="Binary not found"):
            Connection(port=port)

    assert owned.poll() is not None  # the service it started was stopped
    assert not os.path.exists(_get_pidfile_path(port))
    assert _service_key(port) not in _OWNED_SERVICES


def test_a_service_another_connection_holds_is_not_replaced(
    running_service, tmp_home, monkeypatch
):
    """A service this client started but another still uses is reported instead."""
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _own_service(port, owned.pid, refs=1)

    try:
        with pytest.raises(StaleServiceError) as excinfo:
            Connection(port=port)
        assert "1 other opensysml connection(s)" in excinfo.value.remedy
        assert owned.poll() is None  # still running
    finally:
        owned.terminate()
        owned.wait(timeout=5)


def test_a_service_this_client_started_is_kept_when_replacing_it_gains_nothing(
    running_service, tmp_home, monkeypatch
):
    """Stopping a service to start the same build again is no replacement.

    The binary that would be started cannot be shown to be the release asked
    for, since a download that fails leaves the cache as it was.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _own_service(port, owned.pid)
    monkeypatch.setattr(
        "opensysml.connection.ensure_binary", lambda **kwargs: "/nonexistent/sysml-grpc"
    )
    monkeypatch.setattr("opensysml.connection.cached_release", lambda: "v0.0.5")

    try:
        with pytest.raises(StaleServiceError) as excinfo:
            Connection(port=port)
        assert "serve the same build again" in excinfo.value.remedy
        assert owned.poll() is None  # still running
    finally:
        owned.terminate()
        owned.wait(timeout=5)


def test_a_download_that_fails_its_checksum_is_raised_not_read_as_a_mismatch(
    running_service, tmp_home, monkeypatch
):
    """A possibly tampered download is never reported as \"the same build\".

    The replacement check resolves the binary, so the one failure the client
    refuses to treat as a transport failure must pass through it.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _own_service(port, owned.pid)

    def refuse(**kwargs):
        raise ChecksumMismatchError("sha256 of the download does not match")

    monkeypatch.setattr("opensysml.connection.ensure_binary", refuse)

    try:
        with pytest.raises(ChecksumMismatchError):
            Connection(port=port)
        assert owned.poll() is None  # still running
    finally:
        owned.terminate()
        owned.wait(timeout=5)


def test_a_service_lacking_only_a_capability_is_never_stopped(
    running_service, tmp_home, monkeypatch
):
    """No release was asked for, so starting the same binary changes nothing.

    The service is left running and the missing capability reported, rather
    than stopped to make room for a build reporting the same capabilities.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(capabilities=[CAPABILITY_CONVERT])

    owned = _fake_owned_service(port)
    _own_service(port, owned.pid)

    try:
        with pytest.raises(MissingCapabilityError) as excinfo:
            Connection(port=port, require_capabilities=[CAPABILITY_QUERY])
        assert CAPABILITY_QUERY in str(excinfo.value)
        assert owned.poll() is None  # still running
    finally:
        owned.terminate()
        owned.wait(timeout=5)


def test_a_replacement_that_could_not_take_the_address_is_reported(
    running_service, tmp_home, monkeypatch
):
    """A replacement that never served the address is not taken for one.

    The old service can keep listening, in which case the started one exits and
    the health probe is answered by what was refused.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _own_service(port, owned.pid)
    monkeypatch.setattr(
        "opensysml.connection.ensure_binary", lambda **kwargs: _exiting_binary()
    )
    monkeypatch.setattr("opensysml.connection.cached_release", lambda: "v0.0.7")

    with pytest.warns(RuntimeWarning, match="replaced the sysml-grpc service"):
        with pytest.raises(StaleServiceError) as excinfo:
            Connection(port=port)

    assert "kept serving the address" in excinfo.value.reason
    # Nothing is recorded, so no dead pid is left standing for the real service.
    assert not os.path.exists(_get_pidfile_path(port))
    assert _service_key(port) not in _OWNED_SERVICES
    owned.wait(timeout=5)


class TestOwnershipIsAuthenticated:
    """Which process a record names is proved, not guessed from a command line.

    A pid alone is not identity: the operating system reuses pids, and any
    process can be called sysml-grpc. Each of these leaves the process it
    describes running, since none of them is the service this process spawned.
    """

    def test_a_record_naming_a_reused_pid_is_cleaned_not_trusted(
        self, running_service, tmp_home, monkeypatch
    ):
        """A pid the record's start time does not match is another process."""
        port = running_service(version="v0.0.5")
        monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")
        unrelated = _fake_owned_service(port)
        # As if that pid had been the service's and been handed on since.
        _write_record(
            port, unrelated.pid, psutil.Process(unrelated.pid).create_time() - 60
        )

        try:
            with pytest.raises(StaleServiceError) as excinfo:
                Connection(port=port)
            assert "stop the service listening" in excinfo.value.remedy
            assert unrelated.poll() is None  # never signalled
            # The unusable record is cleaned rather than left to be retried.
            assert not os.path.exists(_get_pidfile_path(port))
        finally:
            unrelated.terminate()
            unrelated.wait(timeout=5)

    def test_a_record_another_process_wrote_is_not_this_process_s_to_act_on(
        self, running_service, tmp_home, monkeypatch
    ):
        """Only the process that spawned a service may stop it."""
        port = running_service(version="v0.0.5")
        monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")
        spawned_elsewhere = _fake_owned_service(port)
        process = psutil.Process(spawned_elsewhere.pid)
        _write_record(
            port, process.pid, process.create_time(),
            owner_pid=process.pid, owner_create_time=process.create_time(),
        )

        try:
            with pytest.raises(StaleServiceError) as excinfo:
                Connection(port=port)
            assert "stop the service listening" in excinfo.value.remedy
            assert spawned_elsewhere.poll() is None  # still running
            # Another process's record, so not this one's to remove either.
            assert os.path.exists(_get_pidfile_path(port))
        finally:
            spawned_elsewhere.terminate()
            spawned_elsewhere.wait(timeout=5)

    def test_a_process_that_only_looks_like_the_service_is_never_stopped(
        self, running_service, tmp_home, monkeypatch
    ):
        """A command line is not identity: an unrecorded look-alike is not ours."""
        port = running_service(version="v0.0.5")
        monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")
        script = os.path.join(tempfile.mkdtemp(), "sysml-grpc")
        with open(script, "w") as f:
            f.write("import time\nwhile True:\n    time.sleep(1)\n")
        lookalike = subprocess.Popen([sys.executable, script, "-port", str(port)])

        try:
            with pytest.raises(StaleServiceError):
                Connection(port=port)
            assert lookalike.poll() is None  # still running
        finally:
            lookalike.terminate()
            lookalike.wait(timeout=5)


def _write_record(port, pid, create_time, owner_pid=None, owner_create_time=None):
    """Write an ownership record verbatim, including one no longer authentic."""
    record = {
        "pid": pid,
        "create_time": create_time,
        "port": port,
        "owner_pid": owner_pid if owner_pid is not None else os.getpid(),
        "owner_create_time": (
            owner_create_time if owner_create_time is not None
            else psutil.Process().create_time()
        ),
    }
    path = _get_pidfile_path(port)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(record, f)
    return record


def _exiting_binary():
    """An executable that exits at once, as one that cannot bind the port does."""
    path = os.path.join(tempfile.mkdtemp(), "sysml-grpc")
    with open(path, "w") as f:
        f.write(f"#!{sys.executable}\nraise SystemExit(1)\n")
    os.chmod(path, 0o755)
    return path


def _free_port():
    """A port nothing is listening on, for a service started by the client."""
    with socket.socket() as sock:
        sock.bind(("localhost", 0))
        return sock.getsockname()[1]


def _fake_service_binary(capabilities=(), version="v0.0.7"):
    """An executable serving the handshake like a release, for the start path."""
    path = os.path.join(tempfile.mkdtemp(), "sysml-grpc")
    with open(path, "w") as f:
        f.write(f"""#!{sys.executable}
import sys
from concurrent import futures
import grpc
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class Service(sysml_pb2_grpc.SysMLServiceServicer):
    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version={version!r}, capabilities={list(capabilities)!r}
        )

    def GetDiagnostics(self, request, context):
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")


server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
sysml_pb2_grpc.add_SysMLServiceServicer_to_server(Service(), server)
server.add_insecure_port("localhost:" + sys.argv[2])
server.start()
server.wait_for_termination()
""")
    os.chmod(path, 0o755)
    return path


def _fake_owned_service(port):
    """A live process standing in for a service this process spawned.

    Identity comes from the record, not the command line, so the stand-in needs
    no particular name: the port is served by the fixture's server anyway.
    """
    return subprocess.Popen(
        [sys.executable, "-c", "import time\nwhile True:\n    time.sleep(1)\n"]
    )


def _own_service(port, pid, refs=0):
    """Record pid as the service this process spawned, as _ensure_service does.

    Args:
        port (int): Port it serves
        pid (int): Its process id
        refs (int): Connections in this process holding it
    """
    record = _write_ownership_record(port, pid, psutil.Process(pid).create_time())
    _OWNED_SERVICES[_service_key(port)] = {
        "pid": record["pid"],
        "create_time": record["create_time"],
        "refs": refs,
    }
    return record


class TestMismatchReason:
    """The comparison itself, without a service to run."""

    def _info(self, version="v0.0.5", capabilities=(), answered=True):
        return ServerInfo(
            version=version,
            capabilities=frozenset(capabilities),
            answered=answered,
            origin="origin",
        )

    def test_nothing_asked_for_is_no_mismatch(self):
        assert mismatch_reason(self._info()) is None

    def test_the_same_release_is_no_mismatch(self):
        assert mismatch_reason(self._info(), version="v0.0.5") is None

    def test_another_release_is_a_mismatch(self):
        assert "v0.0.5" in mismatch_reason(self._info(), version="v0.0.7")

    def test_an_unanswered_handshake_cannot_be_the_release_asked_for(self):
        reason = mismatch_reason(self._info(answered=False), version="v0.0.7")
        assert "did not answer GetServerInfo" in reason

    def test_a_missing_capability_is_a_mismatch(self):
        reason = mismatch_reason(
            self._info(capabilities=[CAPABILITY_CONVERT]),
            capabilities=[CAPABILITY_QUERY, CAPABILITY_CONVERT],
        )
        assert repr(CAPABILITY_QUERY) in reason
        assert repr(CAPABILITY_CONVERT) not in reason

    def test_both_differences_are_named(self):
        reason = mismatch_reason(
            self._info(), version="v0.0.7", capabilities=[CAPABILITY_QUERY]
        )
        assert "v0.0.7" in reason and repr(CAPABILITY_QUERY) in reason
