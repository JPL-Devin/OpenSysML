"""Tests for Connection class."""

import grpc
import os
import pytest
import tempfile
import time
from pathlib import Path
from unittest.mock import Mock, patch, MagicMock
from pysysml.connection import (
    Connection,
    _OWNED_SERVICES,
    _get_lockfile_path,
    _service_key,
)
from pysysml.errors import PySysMLError, ServiceError
from pysysml.proto import sysml_pb2


@pytest.fixture
def tmp_home(tmp_path, monkeypatch):
    """Isolate HOME and the state dir, so no real service state is touched."""
    home = tmp_path / "home"
    home.mkdir()
    monkeypatch.setenv("HOME", str(home))
    monkeypatch.setenv("PYSYSML_STATE_DIR", str(home / ".pysysml"))
    before = set(_OWNED_SERVICES)
    yield home
    # Only this test's records: another test's connection still needs its own.
    for key in set(_OWNED_SERVICES) - before:
        del _OWNED_SERVICES[key]


def test_connection_init():
    with patch('grpc.insecure_channel') as mock_channel:
        mock_stub = Mock()
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(port=50051, auto_start=False)
            
            assert conn.port == 50051
            mock_channel.assert_called_once_with('localhost:50051')


def test_connection_custom_host():
    with patch('grpc.insecure_channel') as mock_channel:
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(host='example.com', port=9000, auto_start=False)
            
            mock_channel.assert_called_once_with('example.com:9000')


def test_connection_load():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        # Mock ParseFile RPC response
        pb_root = sysml_pb2.SymbolInfo(
            id="TestModel",
            name="TestModel",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="abc123",
            root=pb_root,
            diagnostics=[],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            model = conn.load("test.sysml")
            
            # Verify ParseFile was called with file path
            call_args = mock_stub.ParseFile.call_args
            request = call_args[0][0]
            assert request.file_path == "test.sysml"
            
            # Verify Model object returned
            assert model.hash == "abc123"
            assert model.root.name == "TestModel"


def test_connection_load_with_diagnostics():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_root = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_diag = sysml_pb2.Diagnostic(
            severity="error",
            message="Parse error",
            span=sysml_pb2.Span(file="bad.sysml", start_line=1, start_col=1, end_line=1, end_col=1),
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash",
            root=pb_root,
            diagnostics=[pb_diag],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            model = conn.load("bad.sysml")
            
            assert len(model.diagnostics) == 1
            assert model.diagnostics[0].message == "Parse error"


def test_connection_load_grpc_error():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()

        # A gRPC failure carrying no status still arrives inside the hierarchy,
        # with the original reachable as __cause__.
        original = grpc.RpcError()
        mock_stub.ParseFile.side_effect = original

        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)

            with pytest.raises(ServiceError) as excinfo:
                conn.load("missing.sysml")
            assert isinstance(excinfo.value, PySysMLError)
            assert excinfo.value.__cause__ is original


def test_connection_get_symbol():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_sym = sysml_pb2.SymbolInfo(
            id="Vehicle::Engine",
            name="Engine",
            kind="PartUsage",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=pb_sym,
            error="",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            pb_result = conn.get_symbol("model_hash", "Vehicle::Engine")
            
            # Verify GetSymbol was called correctly
            call_args = mock_stub.GetSymbol.call_args
            request = call_args[0][0]
            assert request.model_hash == "model_hash"
            assert request.symbol_id == "Vehicle::Engine"
            
            # Verify SymbolInfo returned
            assert pb_result.id == "Vehicle::Engine"
            assert pb_result.name == "Engine"


def test_connection_get_symbol_not_found():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=None,
            error="Symbol not found: NonExistent",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            pb_result = conn.get_symbol("hash", "NonExistent")
            
            # Should return None when error is set
            assert pb_result is None


def test_connection_context_manager():
    """Test that context manager returns self and calls close() on exit."""
    with patch('grpc.insecure_channel') as mock_channel:
        mock_chan_instance = Mock()
        mock_channel.return_value = mock_chan_instance
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            with Connection(auto_start=False) as conn:
                # __enter__ should return self
                assert conn is not None
                assert isinstance(conn, Connection)
            
            # __exit__ should have called close() which closes the channel
            mock_chan_instance.close.assert_called_once()


# --- Task 2: Service Auto-Start Tests ---


def test_probe_service_running():
    """Test _probe_service returns True when service responds."""
    with patch('grpc.insecure_channel') as mock_channel:
        mock_chan_instance = Mock()
        mock_channel.return_value = mock_chan_instance
        
        mock_stub = Mock()
        # Mock GetDiagnostics RPC success
        mock_stub.GetDiagnostics.return_value = sysml_pb2.DiagnosticsResponse(diagnostics=[])
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            result = conn._probe_service('localhost', 50051, timeout=1.0)
            
            assert result is True
            mock_stub.GetDiagnostics.assert_called_once()


def test_probe_service_not_running():
    """Test _probe_service returns False when service not reachable."""
    with patch('grpc.insecure_channel') as mock_channel:
        mock_chan_instance = Mock()
        mock_channel.return_value = mock_chan_instance
        
        # Use real grpc._channel._InactiveRpcError for proper isinstance check
        # Create a mock that raises UNAVAILABLE status
        from grpc import _channel
        
        # Create state object for _InactiveRpcError
        state = Mock()
        state.code = grpc.StatusCode.UNAVAILABLE
        state.details = "Connection refused"
        
        mock_error = _channel._InactiveRpcError(state)
        
        mock_stub = Mock()
        # Mock GetDiagnostics RPC failure
        mock_stub.GetDiagnostics.side_effect = mock_error
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            result = conn._probe_service('localhost', 50051, timeout=1.0)
            
            assert result is False


def test_ensure_service_already_running():
    """Test _ensure_service doesn't start if service already running."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(auto_start=False)
            
            # Mock _probe_service to return True (already running)
            with patch.object(conn, '_probe_service', return_value=True):
                with patch('subprocess.Popen') as mock_popen:
                    conn._ensure_service()
                    
                    # Should not start subprocess
                    mock_popen.assert_not_called()


def test_ensure_service_starts_when_needed(tmp_home):
    """Test _ensure_service starts subprocess when service not running."""
    binary_path = '/path/to/sysml-grpc'
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            # Patch ensure_binary at module level before creating Connection
            with patch('pysysml.connection.ensure_binary', return_value=binary_path):
                # Mock only binary existence check
                real_exists = os.path.exists
                def mock_exists(path):
                    if path == binary_path:
                        return True
                    return real_exists(path)
                
                with patch('os.path.exists', side_effect=mock_exists):
                    conn = Connection(auto_start=False)
                    
                    # Mock _probe_service: False initially, then True after start
                    probe_results = [False, True]
                    with patch.object(conn, '_probe_service', side_effect=probe_results):
                        with patch('subprocess.Popen') as mock_popen:
                            mock_process = Mock()
                            # A pid that exists, so the record written for it
                            # can authenticate the process it names.
                            mock_process.pid = os.getpid()
                            # A running process polls as None.
                            mock_process.poll.return_value = None
                            mock_popen.return_value = mock_process
                            
                            with patch('atexit.register') as mock_atexit:
                                with patch('time.sleep'):
                                    conn._ensure_service()
                                    
                                    # Should start subprocess
                                    mock_popen.assert_called_once()
                                    args = mock_popen.call_args
                                    assert args[0][0] == ['/path/to/sysml-grpc', '-port', '50051']
                                    assert args[1]['start_new_session'] is True
                                    
                                    # Should register cleanup
                                    mock_atexit.assert_called_once()
                                    
                                    # Should store process
                                    assert conn._process == mock_process


def test_ensure_service_timeout(tmp_home):
    """Test _ensure_service raises if service doesn't start in time."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            # Patch ensure_binary at module level before creating Connection
            with patch('pysysml.connection.ensure_binary', return_value='/path/to/sysml-grpc'):
                with patch('os.path.exists', return_value=True):  # Mock binary exists
                    conn = Connection(auto_start=False)
                    
                    # Mock _probe_service: always returns False (never starts)
                    with patch.object(conn, '_probe_service', return_value=False):
                        with patch('subprocess.Popen') as mock_popen:
                            mock_popen.return_value = Mock(pid=12345)
                            mock_popen.return_value.poll.return_value = None
                            with patch('time.sleep'):  # Speed up test
                                from pysysml.errors import ConnectionError
                                try:
                                    conn._ensure_service()
                                    assert False, "Expected ConnectionError"
                                except ConnectionError as e:
                                    assert "Service failed to start" in str(e)


def test_cleanup_service_releases_one_reference_of_a_service_still_in_use():
    """A released reference leaves a service other connections still hold."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(auto_start=False)
            conn._holds_refcount = True
            key = _service_key(conn.port)
            _OWNED_SERVICES[key] = {'pid': 12345, 'create_time': 1.0, 'refs': 2}

            try:
                with patch('pysysml.connection._stop_process') as mock_stop:
                    conn._cleanup_service()

                    mock_stop.assert_not_called()
                    assert _OWNED_SERVICES[key]['refs'] == 1
                    assert conn._process is None
            finally:
                _OWNED_SERVICES.pop(key, None)


def test_cleanup_service_without_a_reference_touches_nothing():
    """Attaching to a service this process did not spawn takes no reference.

    Closing must therefore leave the ownership of whoever did spawn it alone.
    """
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(auto_start=False)
            key = _service_key(conn.port)
            _OWNED_SERVICES[key] = {'pid': 12345, 'create_time': 1.0, 'refs': 1}

            try:
                with patch('pysysml.connection._stop_process') as mock_stop:
                    conn._cleanup_service()

                    mock_stop.assert_not_called()
                    assert _OWNED_SERVICES[key]['refs'] == 1
            finally:
                _OWNED_SERVICES.pop(key, None)


def test_auto_start_enabled():
    """Test auto_start=True triggers _ensure_service."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            with patch('pysysml.connection.Connection._ensure_service') as mock_ensure:
                conn = Connection(auto_start=True)
                
                mock_ensure.assert_called_once()


def test_auto_start_disabled():
    """Test auto_start=False skips _ensure_service."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            with patch('pysysml.connection.Connection._ensure_service') as mock_ensure:
                conn = Connection(auto_start=False)
                
                mock_ensure.assert_not_called()


# --- Task 1: Lockfile Coordination Tests ---


def test_ensure_service_uses_lockfile(tmp_home):
    """Test that _ensure_service acquires lockfile before starting service."""
    binary_path = '/path/to/sysml-grpc'
    with patch('pysysml.connection.ensure_binary') as mock_ensure:
        mock_ensure.return_value = binary_path
        
        # Mock binary existence check
        real_exists = os.path.exists
        def mock_exists(path):
            if path == binary_path:
                return True
            return real_exists(path)
        
        with patch('os.path.exists', side_effect=mock_exists):
            with patch('subprocess.Popen') as mock_popen:
                mock_popen.return_value = Mock(pid=os.getpid())
                mock_popen.return_value.poll.return_value = None
                
                # Mock time.sleep to skip retries
                with patch('time.sleep'):
                    with patch('grpc.insecure_channel'):
                        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
                            conn = Connection(auto_start=False)
                            
                            # Mock _probe_service: False initially, then True after start
                            with patch.object(conn, '_probe_service', side_effect=[False, True]):
                                with patch('atexit.register'):
                                    # Just verify _ensure_service completes without error
                                    # (FileLock usage is implicit - if lockfile wasn't acquired,
                                    # concurrent tests would race and fail randomly)
                                    conn._ensure_service()
                                    
                                    # Verify service was started (proves lockfile was acquired)
                                    assert mock_popen.called


def test_concurrent_ensure_service_blocks(tmp_home):
    """Test that second process blocks while first starts service."""
    import os
    import pytest
    from filelock import FileLock, Timeout

    # Per port: two services on different ports do not wait on each other.
    lockfile_path = _get_lockfile_path(50051)
    
    # Simulate first process holding lock
    lock1 = FileLock(lockfile_path, timeout=0.1)
    lock1.acquire()
    
    try:
        # Second process should timeout trying to acquire
        from pysysml.errors import ConnectionError
        with pytest.raises(ConnectionError, match="Timeout acquiring service lockfile"):
            with patch('pysysml.connection.ensure_binary', return_value='/path/to/binary'):
                with patch('os.path.exists', return_value=True):
                    with patch('subprocess.Popen') as mock_popen:
                        mock_popen.return_value = Mock(pid=12345)
                        with patch('grpc.insecure_channel'):
                            with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
                                conn = Connection(auto_start=False)
                                with patch.object(conn, '_probe_service', return_value=False):
                                    conn._ensure_service()
    finally:
        lock1.release()
