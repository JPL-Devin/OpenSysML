"""Tests for what happens when another release is already listening.

A cached binary is release-aware, but the process already serving the port is
what actually answers, so the same check has to be made against it. Each test
runs a real gRPC server answering the handshake like a particular build, and
drives ``Connection`` against it with a release asked for.
"""

import os
import socket
import subprocess
import sys
import tempfile
from concurrent import futures

import grpc
import pytest

from pysysml.capabilities import (
    CAPABILITY_CONVERT,
    CAPABILITY_QUERY,
    MissingCapabilityError,
    ServerInfo,
    mismatch_reason,
)
from pysysml.connection import (
    Connection,
    _get_pidfile_path,
    _get_refcount_path,
)
from pysysml.errors import ChecksumMismatchError, ConnectionError, StaleServiceError
from pysysml.proto import sysml_pb2, sysml_pb2_grpc


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
    """Isolate HOME so the pidfile and refcount are this test's own."""
    home = tmp_path / "home"
    home.mkdir()
    monkeypatch.setenv("HOME", str(home))
    return home


def test_a_foreign_service_of_another_release_is_reported_not_killed(
    running_service, tmp_home, monkeypatch
):
    """The user's own service is left running, and the mismatch is raised.

    Reusing it would serve the client an older build, whose first newer call
    fails as a MissingCapabilityError naming a capability the release asked for
    does have — the wrong error, several calls later.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    error = excinfo.value
    assert error.address == f"localhost:{port}"
    assert "v0.0.5" in error.reason and "v0.0.7" in error.reason
    assert f"stop the service listening on localhost:{port} yourself" in error.remedy
    assert error.info.version == "v0.0.5"
    # Still serving: nothing was killed.
    with Connection(port=port, auto_start=False) as conn:
        assert conn.server_info().version == "v0.0.5"
    assert not os.path.exists(_get_pidfile_path())
    assert not os.path.exists(_get_refcount_path())


def test_a_foreign_service_that_cannot_say_what_it_is_is_reported(
    running_service, tmp_home, monkeypatch
):
    """A service that cannot say what it is cannot be the release asked for."""
    port = running_service(answers_handshake=False)
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    assert "did not answer GetServerInfo" in excinfo.value.reason


def test_a_foreign_service_lacking_a_required_capability_is_reported(
    running_service, tmp_home
):
    """Capabilities asked for at connect time are checked against it too."""
    port = running_service(capabilities=[CAPABILITY_CONVERT])

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port, require_capabilities=[CAPABILITY_QUERY])

    assert repr(CAPABILITY_QUERY) in excinfo.value.reason


def test_a_handshake_that_fails_is_not_taken_for_an_answer(
    running_service, tmp_home, monkeypatch
):
    """A call that failed says nothing, so it is neither cached nor trusted.

    Recording it as a service reporting no capabilities would refuse every
    capability-gated call for the life of the connection.
    """
    monkeypatch.delenv("PYSYSML_GRPC_VERSION", raising=False)
    port = running_service(version="v0.0.5", capabilities=[CAPABILITY_QUERY],
                           handshake_failures=1)

    with Connection(port=port) as conn:
        info = conn.server_info()
        assert info.version == "v0.0.5"
        assert info.has(CAPABILITY_QUERY)


def test_a_handshake_that_fails_is_reported_when_a_release_was_asked_for(
    running_service, tmp_home, monkeypatch
):
    """A service that could not be asked is reported, not stopped."""
    port = running_service(handshake_failures=1)
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    assert "GetServerInfo call to it failed" in excinfo.value.reason


def test_a_refused_connection_releases_what_it_took(tmp_home, monkeypatch):
    """A connection refused for a missing capability holds no reference afterwards.

    The connection is never returned, so nothing else can close its channel or
    release the reference it took on the service it started.
    """
    monkeypatch.delenv("PYSYSML_GRPC_VERSION", raising=False)
    port = _free_port()
    monkeypatch.setattr(
        "pysysml.connection.ensure_binary",
        lambda **kwargs: _fake_service_binary(capabilities=()),
    )

    with pytest.raises(MissingCapabilityError):
        Connection(port=port, require_capabilities=[CAPABILITY_QUERY])

    assert not os.path.exists(_get_refcount_path())
    assert not os.path.exists(_get_pidfile_path())


def test_a_matching_service_is_adopted(running_service, tmp_home, monkeypatch):
    """A running service that is what was asked for is used, as before."""
    port = running_service(version="v0.0.7", capabilities=[CAPABILITY_CONVERT])
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    with Connection(port=port, require_capabilities=[CAPABILITY_CONVERT]) as conn:
        assert conn.server_info().version == "v0.0.7"
        # Adopted, so it is reference-counted like any shared service.
        assert os.path.exists(_get_refcount_path())


def test_a_service_asked_for_no_release_is_adopted_whatever_it_is(
    running_service, tmp_home, monkeypatch
):
    """With no release asked for, whichever build answers is accepted.

    This is the same rule the binary cache follows: a binary the user put there
    is left alone until a client says which release it needs.
    """
    monkeypatch.delenv("PYSYSML_GRPC_VERSION", raising=False)
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
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    # Records of a service this client started on that port: a live process
    # whose command line is a sysml-grpc serving it.
    owned = _fake_owned_service(port)
    _record_pidfile(owned.pid)

    # Starting the replacement then fails on the binary, which is beside the
    # point: what this test is about is that the mismatched one was stopped.
    monkeypatch.setattr(
        "pysysml.connection.ensure_binary", lambda **kwargs: "/nonexistent/sysml-grpc"
    )
    monkeypatch.setattr("pysysml.connection.cached_release", lambda: "v0.0.7")

    with pytest.warns(RuntimeWarning, match="replaced the sysml-grpc service"):
        with pytest.raises(ConnectionError, match="Binary not found"):
            Connection(port=port)

    assert owned.poll() is not None  # the service it started was stopped
    assert not os.path.exists(_get_pidfile_path())


def test_a_service_another_connection_holds_is_not_replaced(
    running_service, tmp_home, monkeypatch
):
    """A service this client started but another still uses is reported instead."""
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _record_pidfile(owned.pid)
    with open(_get_refcount_path(), "w") as f:
        f.write("1")

    try:
        with pytest.raises(StaleServiceError) as excinfo:
            Connection(port=port)
        assert "1 other pysysml connection(s)" in excinfo.value.remedy
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
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _record_pidfile(owned.pid)
    monkeypatch.setattr(
        "pysysml.connection.ensure_binary", lambda **kwargs: "/nonexistent/sysml-grpc"
    )
    monkeypatch.setattr("pysysml.connection.cached_release", lambda: "v0.0.5")

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
    monkeypatch.setenv("PYSYSML_GRPC_VERSION", "v0.0.7")

    owned = _fake_owned_service(port)
    _record_pidfile(owned.pid)

    def refuse(**kwargs):
        raise ChecksumMismatchError("sha256 of the download does not match")

    monkeypatch.setattr("pysysml.connection.ensure_binary", refuse)

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
    monkeypatch.delenv("PYSYSML_GRPC_VERSION", raising=False)
    port = running_service(capabilities=[CAPABILITY_CONVERT])

    owned = _fake_owned_service(port)
    _record_pidfile(owned.pid)

    try:
        with pytest.raises(StaleServiceError) as excinfo:
            Connection(port=port, require_capabilities=[CAPABILITY_QUERY])
        assert repr(CAPABILITY_QUERY) in excinfo.value.reason
        assert owned.poll() is None  # still running
    finally:
        owned.terminate()
        owned.wait(timeout=5)


def _free_port():
    """A port nothing is listening on, for a service started by the client."""
    with socket.socket() as sock:
        sock.bind(("localhost", 0))
        return sock.getsockname()[1]


def _fake_service_binary(capabilities=(), version="v0.0.7"):
    """An executable serving the handshake like a release, for the start path.

    Named sysml-grpc, since the lifecycle recognizes its own service by the
    command line it started.
    """
    path = os.path.join(tempfile.mkdtemp(), "sysml-grpc")
    with open(path, "w") as f:
        f.write(f"""#!{sys.executable}
import sys
from concurrent import futures
import grpc
from pysysml.proto import sysml_pb2, sysml_pb2_grpc


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
    """A live process whose command line looks like a service started for port.

    Ownership is read from the command line, so the stand-in needs one: what it
    does is irrelevant, since the port is served by the fixture's server.
    """
    script = os.path.join(tempfile.mkdtemp(), "sysml-grpc")
    with open(script, "w") as f:
        f.write("import time\nwhile True:\n    time.sleep(1)\n")
    return subprocess.Popen([sys.executable, script, "-port", str(port)])


def _record_pidfile(pid):
    """Record pid as the service this client started, as _ensure_service does."""
    path = _get_pidfile_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(f"{pid}\n")


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
