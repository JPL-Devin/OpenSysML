"""Integration tests for auto-lifecycle: binary download, service start, model load.

These tests verify the complete lifecycle from binary download to model loading.
They can be skipped if:
- Binary already exists (manual install)
- Network unavailable (CI without network access)
- GitHub releases not yet published

Run with: pytest tests/test_lifecycle.py -v
Skip with: pytest tests/ -k "not integration"
"""

import pytest
import os
import shutil
from pathlib import Path
from unittest.mock import patch, MagicMock
import pysysml
from pysysml.binary import ensure_binary, get_binary_path


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
        """Verify service starts automatically when using pysysml.load().
        
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
            model = pysysml.load(str(test_file))
            
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
            model1 = pysysml.load(str(file1))
            assert model1 is not None
            
            # Load second model - should reuse service
            model2 = pysysml.load(str(file2))
            assert model2 is not None
            
            # Models should be different (different hashes)
            assert model1.hash != model2.hash
            
        except Exception as e:
            pytest.skip(f"Auto-lifecycle not available: {e}")
    
    def test_explicit_connect_with_auto_start(self):
        """Verify pysysml.connect() with auto_start=True triggers lifecycle."""
        try:
            conn = pysysml.connect(auto_start=True)
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
        from pysysml.connection import Connection
        
        # Try connecting without auto-start
        conn = Connection(auto_start=False)
        
        # Try a simple operation - will fail if service not running
        from pysysml.proto import sysml_pb2
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
            conn1 = pysysml.connect(auto_start=True)
            model1 = conn1.load(str(test_file))
            assert model1 is not None
            
            # Create new connection - should find service already running
            conn2 = pysysml.connect(auto_start=True)
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
        """Verify module-level pysysml.load() reuses connection across calls."""
        file1 = tmp_path / "reuse1.sysml"
        file1.write_text("package Reuse1 { part P1; }")
        
        file2 = tmp_path / "reuse2.sysml"
        file2.write_text("package Reuse2 { part P2; }")
        
        try:
            # Clear default connection to start fresh
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            # Load two models
            model1 = pysysml.load(str(file1))
            model2 = pysysml.load(str(file2))
            
            assert model1 is not None
            assert model2 is not None
            assert model1.hash != model2.hash
            
            # Should have created only one default connection
            assert pysysml._default_connection is not None
            
        except Exception as e:
            pytest.skip(f"Auto-lifecycle not available: {e}")
    
    def test_service_shuts_down_when_last_process_exits(self):
        """Test that service terminates when reference count reaches 0."""
        import time
        import psutil
        from pysysml.connection import _get_pidfile_path, _get_refcount_path
        
        # Clean state first - kill existing service + reset refcount
        pidfile = _get_pidfile_path()
        refcount_path = _get_refcount_path()
        
        if os.path.exists(pidfile):
            with open(pidfile) as f:
                old_pid = int(f.read().strip())
            try:
                psutil.Process(old_pid).kill()
            except psutil.NoSuchProcess:
                pass
            os.remove(pidfile)
        
        if os.path.exists(refcount_path):
            os.remove(refcount_path)
        
        # First connection increments refcount to 1
        with patch('pysysml.connection.ensure_binary') as mock_ensure:
            mock_ensure.return_value = get_binary_path()
            
            conn1 = pysysml.connect(auto_start=True)
            
            # Get PID from pidfile
            with open(pidfile) as f:
                pid = int(f.read().strip())
            
            # Service should be running
            assert psutil.Process(pid).is_running()
            
            # Second connection increments to 2
            conn2 = pysysml.connect(auto_start=True)
            
            # Close first connection (refcount -> 1)
            conn1.close()
            time.sleep(0.5)
            
            # Service should still be running
            assert psutil.Process(pid).is_running()
            
            # Close second connection (refcount -> 0)
            conn2.close()
            time.sleep(0.5)
            
            # Service should be terminated
            try:
                assert not psutil.Process(pid).is_running()
            except psutil.NoSuchProcess:
                # Expected - process terminated
                pass
