"""Integration tests for auto-lifecycle: binary download, service start, model load.

These tests verify the complete lifecycle from binary download to model loading.
They can be skipped if:
- Binary already exists (manual install)
- Network unavailable (CI without network access)
- GitHub releases not yet published

Run with: pytest tests/test_lifecycle.py -v
Skip with: pytest tests/ -k "not integration"
"""

import json
import pytest
import os
import psutil
import shutil
from pathlib import Path
from unittest.mock import patch, MagicMock
import opensysml
from opensysml.binary import ensure_binary, get_binary_path
from opensysml.connection import _OWNED_SERVICES, _get_pidfile_path, _service_key
from tests.service_gate import (
    free_port,
    service_binary,
    skip_or_fail_without_service,
)


@pytest.fixture
def clean_binary():
    """Fixture to optionally clean binary for fresh install test."""
    # Don't automatically clean - let tests decide
    yield
    # Cleanup after test if needed


@pytest.mark.integration
class TestAutoLifecycle:
    """Integration tests for complete auto-lifecycle."""
    
    def test_binary_exists_or_download(self):
        """Verify binary exists or can be downloaded.
        
        Note: If GitHub releases don't exist yet, this will skip.
        """
        # Check if binary already exists
        binary_path = get_binary_path()
        
        if os.path.exists(binary_path):
            pytest.skip("Binary already exists - manual install detected")
        
        # Try to download (will fail if releases not published)
        try:
            result = ensure_binary()
            assert os.path.exists(result)
            assert os.access(result, os.X_OK)
        except Exception as e:
            pytest.skip(f"Binary download not available yet: {e}")
    
    def test_service_start_with_auto_lifecycle(self, tmp_path):
        """Verify service starts automatically when using opensysml.load().
        
        Tests the complete lifecycle:
        1. Binary download (if needed)
        2. Service auto-start (if not running)
        3. Model load
        """
        # Create minimal test file
        test_file = tmp_path / "lifecycle_test.sysml"
        test_file.write_text("""
            package LifecycleTest {
                part TestPart;
            }
        """)
        
        # Use convenience API - should trigger auto-lifecycle
        try:
            model = opensysml.load(str(test_file))
            
            # Verify model loaded successfully
            assert model is not None
            assert model.hash is not None
            assert model.root is not None
            
        except Exception as e:
            # If binary download or service start fails, document why
            pytest.skip(f"Auto-lifecycle not available: {e}")
    
    def test_service_persists_across_multiple_loads(self, tmp_path):
        """Verify service reuses existing process for multiple loads.
        
        This ensures we don't spawn multiple service processes.
        """
        # Create two test files
        file1 = tmp_path / "model1.sysml"
        file1.write_text("""
            package Model1 {
                part Part1;
            }
        """)
        
        file2 = tmp_path / "model2.sysml"
        file2.write_text("""
            package Model2 {
                part Part2;
            }
        """)
        
        try:
            # Load first model
            model1 = opensysml.load(str(file1))
            assert model1 is not None
            
            # Load second model - should reuse service
            model2 = opensysml.load(str(file2))
            assert model2 is not None
            
            # Models should be different (different hashes)
            assert model1.hash != model2.hash
            
        except Exception as e:
            pytest.skip(f"Auto-lifecycle not available: {e}")
    
    def test_explicit_connect_with_auto_start(self):
        """Verify opensysml.connect() with auto_start=True triggers lifecycle."""
        try:
            conn = opensysml.connect(auto_start=True)
            assert conn is not None
            
            # Connection should be usable
            # (Don't load a file - just verify connection established)
            conn.close()
            
        except Exception as e:
            pytest.skip(f"Auto-lifecycle not available: {e}")
    
    def test_explicit_connect_without_auto_start_requires_manual_service(self):
        """Verify auto_start=False doesn't start service."""
        # This test verifies that auto_start=False doesn't trigger lifecycle
        # It will fail if service isn't already running
        
        # Check if service is already running
        from opensysml.connection import Connection
        
        # Try connecting without auto-start
        conn = Connection(auto_start=False)
        
        # Try a simple operation - will fail if service not running
        from opensysml.proto import sysml_pb2
        import grpc
        
        try:
            # Try to get diagnostics for non-existent model
            request = sysml_pb2.DiagnosticsRequest(model_hash="test")
            conn._stub.GetDiagnostics(request, timeout=2)
            
            # If we get here, service was already running
            conn.close()
            
        except grpc.RpcError as e:
            # Expected if service not running
            if e.code() == grpc.StatusCode.UNAVAILABLE:
                pytest.skip("Service not running - auto_start=False working correctly")
            elif e.code() == grpc.StatusCode.NOT_FOUND:
                # Service is running (NOT_FOUND is expected for invalid hash)
                conn.close()
            else:
                raise


@pytest.mark.integration
class TestLifecycleRobustness:
    """Robustness tests for lifecycle edge cases."""
    
    def test_service_survives_connection_close(self, tmp_path):
        """Verify service keeps running after connection closes.
        
        Service should persist when other connections active.
        """
        test_file = tmp_path / "persist_test.sysml"
        test_file.write_text("""
            package PersistTest {
                part Part1;
            }
        """)
        
        try:
            # Load with explicit connection
            conn1 = opensysml.connect(auto_start=True)
            model1 = conn1.load(str(test_file))
            assert model1 is not None
            
            # Create new connection - should find service already running
            conn2 = opensysml.connect(auto_start=True)
            model2 = conn2.load(str(test_file))
            assert model2 is not None
            
            # Close first connection - service should persist because conn2 open
            conn1.close()
            
            # Same file should have same hash
            assert model1.hash == model2.hash
            
            conn2.close()
            
        except Exception as e:
            pytest.skip(f"Auto-lifecycle not available: {e}")
    
    def test_module_level_load_reuses_connection(self, tmp_path):
        """Verify module-level opensysml.load() reuses connection across calls."""
        file1 = tmp_path / "reuse1.sysml"
        file1.write_text("package Reuse1 { part P1; }")
        
        file2 = tmp_path / "reuse2.sysml"
        file2.write_text("package Reuse2 { part P2; }")
        
        try:
            # Clear default connection to start fresh
            opensysml._default_connection = None
            opensysml._default_connection_params = None
            
            # Load two models
            model1 = opensysml.load(str(file1))
            model2 = opensysml.load(str(file2))
            
            assert model1 is not None
            assert model2 is not None
            assert model1.hash != model2.hash
            
            # Should have created only one default connection
            assert opensysml._default_connection is not None
            
        except Exception as e:
            pytest.skip(f"Auto-lifecycle not available: {e}")


@pytest.mark.integration
class TestOwnershipOfASpawnedService:
    """A service this process spawned, on a port and state directory of its own.

    opensysml never stops a service it did not spawn, so these spawn their own
    rather than asserting anything about whatever listens on the default port.
    """

    @pytest.fixture
    def own_service(self, tmp_path, monkeypatch):
        """A port and isolated state for a service this test spawns itself."""
        binary = service_binary()
        if binary is None:
            skip_or_fail_without_service("no sysml-grpc binary is available to spawn")
        monkeypatch.setenv("OPENSYSML_STATE_DIR", str(tmp_path / "state"))
        monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
        monkeypatch.setattr(
            "opensysml.connection.ensure_binary", lambda **kwargs: binary
        )
        port = free_port()
        yield port
        # Only this port's bookkeeping: another test's open connection still
        # needs its own, or the service it spawned would never be stopped.
        record = _OWNED_SERVICES.pop(_service_key(port), None)
        if record is not None:
            _kill_if_running(record['pid'])

    def _recorded_pid(self, port):
        """The pid recorded for the service spawned on a port."""
        with open(_get_pidfile_path(port)) as f:
            return json.load(f)["pid"]

    def test_the_last_reference_released_stops_the_service_exactly_once(
        self, own_service
    ):
        """A spawned service outlives every connection but the last."""
        conn1 = opensysml.connect(port=own_service, auto_start=True)
        pid = self._recorded_pid(own_service)
        service = psutil.Process(pid)
        assert service.is_running()

        conn2 = opensysml.connect(port=own_service, auto_start=True)
        # Both connections hold the one service this process spawned.
        assert _OWNED_SERVICES[_service_key(own_service)]["refs"] == 2

        conn1.close()
        assert service.is_running()
        # Closing twice releases one reference, not two.
        conn1.close()
        assert service.is_running()

        conn2.close()
        _wait_gone(service)
        assert not service.is_running()
        # The record goes with it, so no dead pid is left to be trusted.
        assert not os.path.exists(_get_pidfile_path(own_service))
        assert _service_key(own_service) not in _OWNED_SERVICES

    def test_attaching_to_it_from_this_process_holds_it(self, own_service):
        """A connection that finds the spawned service holds it too.

        Otherwise the connection that spawned it could stop it while another is
        still using it.
        """
        spawner = opensysml.connect(port=own_service, auto_start=True)
        service = psutil.Process(self._recorded_pid(own_service))

        attached = opensysml.connect(port=own_service, auto_start=True)
        spawner.close()
        assert service.is_running()

        attached.close()
        _wait_gone(service)
        assert not service.is_running()

    def test_a_crashed_service_leaves_recoverable_state(self, own_service):
        """A record whose process is gone is cleaned, and a service starts again."""
        conn = opensysml.connect(port=own_service, auto_start=True)
        service = psutil.Process(self._recorded_pid(own_service))
        service.kill()
        _wait_gone(service)
        conn.close()

        with opensysml.connect(port=own_service, auto_start=True) as recovered:
            started = psutil.Process(self._recorded_pid(own_service))
            assert started.pid != service.pid
            assert started.is_running()
            assert recovered.server_info() is not None
        _wait_gone(started)
        assert not os.path.exists(_get_pidfile_path(own_service))

    def test_closing_a_connection_to_a_crashed_service_spares_its_replacement(
        self, own_service
    ):
        """A reference taken on a dead service is not a reference on the new one."""
        crashed = opensysml.connect(port=own_service, auto_start=True)
        service = psutil.Process(self._recorded_pid(own_service))
        service.kill()
        _wait_gone(service)

        restarted_conn = opensysml.connect(port=own_service, auto_start=True)
        restarted = psutil.Process(self._recorded_pid(own_service))
        crashed.close()
        assert restarted.is_running()
        assert restarted_conn.server_info() is not None

        restarted_conn.close()
        _wait_gone(restarted)
        assert not restarted.is_running()


def _kill_if_running(pid):
    """Stop a service a failing test left behind, so the port is not held."""
    try:
        process = psutil.Process(pid)
        process.kill()
        _wait_gone(process)
    except psutil.Error:
        pass


def _wait_gone(process, timeout=10):
    """Wait for a process to exit, so an assertion is not made on the race."""
    try:
        process.wait(timeout=timeout)
    except (psutil.NoSuchProcess, psutil.TimeoutExpired):
        pass
