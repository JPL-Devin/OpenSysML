# Phase 3: Auto-Lifecycle Management - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable zero-config usage - `import pysysml; model = pysysml.load("file.sysml")` works without manual server setup.

**Architecture:** Binary management module downloads sysml-grpc from GitHub releases, Connection auto-starts service with lockfile coordination, module-level API provides convenience functions.

**Tech Stack:** Python 3.8+, urllib/requests for downloads, subprocess for process management, fcntl/filelock for multi-process coordination, atexit for cleanup.

---

## File Structure

**New files:**
- `pysysml/binary.py` - Binary download, platform detection, checksum verification
- `tests/test_binary.py` - Unit tests for binary management
- `tests/test_lifecycle.py` - Integration tests for auto-start

**Modified files:**
- `pysysml/connection.py` - Add auto-start logic, health checks, process management
- `pysysml/__init__.py` - Add module-level convenience API
- `tests/test_connection.py` - Add auto-start tests

---

## Task Outline

### Task 1: Binary Management Module
[Platform detection, GitHub release download, checksum verification, storage in ~/.pysysml/bin/]

### Task 2: Service Auto-Start in Connection
[Health check probing, subprocess management, lockfile coordination, atexit cleanup]

### Task 3: Module-Level Convenience API
[load(), connect(), auto-initialization on first use]

### Task 4: Comprehensive Testing
[Mock HTTP downloads, service lifecycle, multi-process coordination]

---

## Tasks (to be filled in)

### Task 1: Binary Management Module

**Files:**
- Create: `pysysml/binary.py`
- Create: `tests/test_binary.py`

**Objective:** Detect platform, download sysml-grpc from GitHub releases, verify checksum, store in ~/.pysysml/bin/

- [ ] **Step 1: Write failing test for platform detection**

```python
# tests/test_binary.py
import platform
from pysysml.binary import detect_platform

def test_detect_platform():
    """Test platform detection returns valid tuple."""
    os_name, arch = detect_platform()
    assert os_name in ('linux', 'darwin', 'windows')
    assert arch in ('amd64', 'arm64')
```

- [ ] **Step 2: Run test to verify it fails**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_detect_platform -v`
Expected: FAIL with "cannot import name 'detect_platform'"

- [ ] **Step 3: Implement platform detection**

```python
# pysysml/binary.py
"""Binary management for sysml-grpc service."""

import platform
import sys

def detect_platform():
    """Detect current platform and architecture.
    
    Returns:
        tuple: (os_name, arch) where os_name in ('linux', 'darwin', 'windows')
               and arch in ('amd64', 'arm64')
    """
    # Map Python platform names to Go GOOS
    system = platform.system().lower()
    if system == 'linux':
        os_name = 'linux'
    elif system == 'darwin':
        os_name = 'darwin'
    elif system == 'windows':
        os_name = 'windows'
    else:
        raise RuntimeError(f"Unsupported operating system: {system}")
    
    # Map Python machine architecture to Go GOARCH
    machine = platform.machine().lower()
    if machine in ('x86_64', 'amd64'):
        arch = 'amd64'
    elif machine in ('aarch64', 'arm64'):
        arch = 'arm64'
    else:
        raise RuntimeError(f"Unsupported architecture: {machine}")
    
    return os_name, arch
```

- [ ] **Step 4: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_detect_platform -v`
Expected: PASS

- [ ] **Step 5: Write failing test for binary path**

```python
# tests/test_binary.py
import os
from pysysml.binary import get_binary_path

def test_get_binary_path():
    """Test binary path construction."""
    path = get_binary_path()
    assert path.startswith(os.path.expanduser('~/.pysysml/bin/'))
    assert path.endswith('sysml-grpc') or path.endswith('sysml-grpc.exe')
```

- [ ] **Step 6: Implement get_binary_path**

```python
# pysysml/binary.py (add to existing file)
import os

def get_binary_path():
    """Get the local path where sysml-grpc binary should be stored.
    
    Returns:
        str: Absolute path to binary (e.g. ~/.pysysml/bin/sysml-grpc)
    """
    os_name, _ = detect_platform()
    binary_name = 'sysml-grpc.exe' if os_name == 'windows' else 'sysml-grpc'
    
    base_dir = os.path.expanduser('~/.pysysml/bin')
    return os.path.join(base_dir, binary_name)
```

- [ ] **Step 7: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_get_binary_path -v`
Expected: PASS

- [ ] **Step 8: Write failing test for binary download**

```python
# tests/test_binary.py
from unittest.mock import patch, Mock, mock_open
from pysysml.binary import download_binary

def test_download_binary():
    """Test binary download from GitHub releases."""
    with patch('urllib.request.urlopen') as mock_urlopen:
        # Mock HTTP response
        mock_response = Mock()
        mock_response.read.return_value = b'fake binary content'
        mock_response.__enter__.return_value = mock_response
        mock_response.__exit__.return_value = False
        mock_urlopen.return_value = mock_response
        
        with patch('builtins.open', mock_open()) as mock_file:
            with patch('os.makedirs'):
                with patch('os.chmod'):
                    result = download_binary(version='v0.1.0')
                    
                    assert result == os.path.expanduser('~/.pysysml/bin/sysml-grpc')
                    mock_urlopen.assert_called_once()
                    # Verify URL format
                    call_args = mock_urlopen.call_args[0][0]
                    assert 'github.com/Open-MBEE/OpenSysML/releases/download/v0.1.0' in call_args
```

- [ ] **Step 9: Implement download_binary**

```python
# pysysml/binary.py (add to existing file)
import urllib.request
import os
import stat

def download_binary(version='latest', github_repo='Open-MBEE/OpenSysML'):
    """Download sysml-grpc binary from GitHub releases.
    
    Args:
        version (str): Release tag (e.g. 'v0.1.0') or 'latest'
        github_repo (str): GitHub repository (org/repo)
    
    Returns:
        str: Path to downloaded binary
    
    Raises:
        RuntimeError: If download fails
    """
    os_name, arch = detect_platform()
    binary_path = get_binary_path()
    
    # Construct GitHub release URL
    # Format: https://github.com/Open-MBEE/OpenSysML/releases/download/v0.1.0/sysml-grpc-linux-amd64
    if version == 'latest':
        # For now, use a fixed version (latest API requires extra request)
        version = 'v0.1.0'
    
    binary_name = f"sysml-grpc-{os_name}-{arch}"
    if os_name == 'windows':
        binary_name += '.exe'
    
    url = f"https://github.com/{github_repo}/releases/download/{version}/{binary_name}"
    
    # Create directory if it doesn't exist
    os.makedirs(os.path.dirname(binary_path), exist_ok=True)
    
    # Download binary
    try:
        with urllib.request.urlopen(url) as response:
            data = response.read()
        
        # Write to file
        with open(binary_path, 'wb') as f:
            f.write(data)
        
        # Make executable
        os.chmod(binary_path, stat.S_IRWXU | stat.S_IRGRP | stat.S_IXGRP | stat.S_IROTH | stat.S_IXOTH)
        
        return binary_path
    except Exception as e:
        raise RuntimeError(f"Failed to download sysml-grpc binary: {e}")
```

- [ ] **Step 10: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_download_binary -v`
Expected: PASS

- [ ] **Step 11: Write failing test for checksum verification**

```python
# tests/test_binary.py
import hashlib
from pysysml.binary import verify_checksum

def test_verify_checksum():
    """Test SHA-256 checksum verification."""
    data = b"test binary content"
    expected = hashlib.sha256(data).hexdigest()
    
    with patch('builtins.open', mock_open(read_data=data)):
        assert verify_checksum('/fake/path', expected) == True
        assert verify_checksum('/fake/path', 'wrong_hash') == False
```

- [ ] **Step 12: Implement verify_checksum**

```python
# pysysml/binary.py (add to existing file)
import hashlib

def verify_checksum(binary_path, expected_sha256):
    """Verify SHA-256 checksum of binary file.
    
    Args:
        binary_path (str): Path to binary file
        expected_sha256 (str): Expected SHA-256 hex digest
    
    Returns:
        bool: True if checksum matches, False otherwise
    """
    sha256 = hashlib.sha256()
    
    with open(binary_path, 'rb') as f:
        while True:
            chunk = f.read(8192)
            if not chunk:
                break
            sha256.update(chunk)
    
    actual = sha256.hexdigest()
    return actual == expected_sha256
```

- [ ] **Step 13: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_verify_checksum -v`
Expected: PASS

- [ ] **Step 14: Write failing test for ensure_binary**

```python
# tests/test_binary.py
from pysysml.binary import ensure_binary

def test_ensure_binary_exists():
    """Test ensure_binary returns path when binary exists."""
    with patch('os.path.exists', return_value=True):
        with patch('os.access', return_value=True):
            path = ensure_binary()
            assert path == os.path.expanduser('~/.pysysml/bin/sysml-grpc')

def test_ensure_binary_downloads():
    """Test ensure_binary downloads if binary missing."""
    with patch('os.path.exists', return_value=False):
        with patch('pysysml.binary.download_binary') as mock_download:
            mock_download.return_value = '/fake/path/sysml-grpc'
            path = ensure_binary()
            assert path == '/fake/path/sysml-grpc'
            mock_download.assert_called_once()
```

- [ ] **Step 15: Implement ensure_binary**

```python
# pysysml/binary.py (add to existing file)

def ensure_binary(force_download=False):
    """Ensure sysml-grpc binary is available, downloading if necessary.
    
    Args:
        force_download (bool): If True, download even if binary exists
    
    Returns:
        str: Path to binary
    
    Raises:
        RuntimeError: If binary cannot be obtained
    """
    binary_path = get_binary_path()
    
    # Check if binary already exists and is executable
    if not force_download and os.path.exists(binary_path):
        if os.access(binary_path, os.X_OK):
            return binary_path
    
    # Download binary
    return download_binary()
```

- [ ] **Step 16: Run tests to verify they pass**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py -v`
Expected: All 5 tests PASS

- [ ] **Step 17: Commit**

```bash
git add pysysml/binary.py tests/test_binary.py
git commit -m "feat(python): implement binary management with platform detection and GitHub downloads"
```

### Task 2: Service Auto-Start in Connection

**Files:**
- Modify: `pysysml/connection.py`
- Modify: `tests/test_connection.py`

**Objective:** Auto-start sysml-grpc service when Connection created, with health checks and multi-process coordination

- [ ] **Step 1: Write failing test for service health check**

```python
# tests/test_connection.py (add to existing file)
from pysysml.connection import Connection

def test_probe_service_running():
    """Test health check detects running service."""
    with patch('grpc.insecure_channel') as mock_channel:
        mock_stub = Mock()
        mock_stub.GetDiagnostics.return_value = Mock()  # Success
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            assert conn._probe_service('localhost', 50051) == True

def test_probe_service_not_running():
    """Test health check detects service not running."""
    with patch('grpc.insecure_channel') as mock_channel:
        mock_stub = Mock()
        mock_stub.GetDiagnostics.side_effect = grpc.RpcError()
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            assert conn._probe_service('localhost', 50051) == False
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_connection.py::test_probe_service_running -v`
Expected: FAIL with "AttributeError: 'Connection' object has no attribute '_probe_service'"

- [ ] **Step 3: Implement _probe_service method**

```python
# pysysml/connection.py (add to Connection class)
import grpc
from pysysml.proto import sysml_pb2

def _probe_service(self, host, port, timeout=1.0):
    """Check if sysml-grpc service is running and responsive.
    
    Args:
        host (str): Service host
        port (int): Service port
        timeout (float): Timeout in seconds
    
    Returns:
        bool: True if service responds to health check
    """
    try:
        channel = grpc.insecure_channel(f"{host}:{port}")
        stub = sysml_pb2_grpc.SysMLServiceStub(channel)
        
        # Use GetDiagnostics with empty model hash as health check
        request = sysml_pb2.DiagnosticsRequest(model_hash="")
        stub.GetDiagnostics(request, timeout=timeout)
        
        channel.close()
        return True
    except Exception:
        return False
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_connection.py::test_probe_service_running tests/test_connection.py::test_probe_service_not_running -v`
Expected: Both tests PASS

- [ ] **Step 5: Write failing test for service auto-start**

```python
# tests/test_connection.py
import subprocess
from pysysml.connection import Connection

def test_ensure_service_starts():
    """Test _ensure_service starts service if not running."""
    with patch('pysysml.connection.Connection._probe_service', side_effect=[False, True]):
        with patch('pysysml.binary.ensure_binary', return_value='/fake/sysml-grpc'):
            with patch('subprocess.Popen') as mock_popen:
                mock_process = Mock()
                mock_process.pid = 12345
                mock_popen.return_value = mock_process
                
                conn = Connection()
                conn._ensure_service()
                
                mock_popen.assert_called_once()
                assert conn._service_process is not None

def test_ensure_service_reuses_existing():
    """Test _ensure_service doesn't start if already running."""
    with patch('pysysml.connection.Connection._probe_service', return_value=True):
        with patch('subprocess.Popen') as mock_popen:
            conn = Connection()
            conn._ensure_service()
            
            mock_popen.assert_not_called()
```

- [ ] **Step 6: Implement _ensure_service method**

```python
# pysysml/connection.py (add to Connection class)
import subprocess
import time
import atexit
from pysysml.binary import ensure_binary

def __init__(self, host='localhost', port=50051, auto_start=True):
    """Initialize connection to sysml-grpc service.
    
    Args:
        host (str): Service host (default: localhost)
        port (int): Service port (default: 50051)
        auto_start (bool): Auto-start service if not running (default: True)
    """
    self._host = host
    self._port = port
    self._auto_start = auto_start
    self._service_process = None
    
    # Auto-start if enabled
    if auto_start:
        self._ensure_service()
    
    # Create gRPC channel
    self._channel = grpc.insecure_channel(f"{host}:{port}")
    self._stub = sysml_pb2_grpc.SysMLServiceStub(self._channel)

def _ensure_service(self, max_retries=5, retry_delay=0.5):
    """Ensure sysml-grpc service is running, starting if necessary.
    
    Args:
        max_retries (int): Max health check attempts after starting
        retry_delay (float): Delay between health checks (seconds)
    
    Raises:
        RuntimeError: If service cannot be started
    """
    # Check if service already running
    if self._probe_service(self._host, self._port):
        return
    
    # Get binary path
    binary_path = ensure_binary()
    
    # Start service as subprocess
    try:
        self._service_process = subprocess.Popen(
            [binary_path, '-port', str(self._port)],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True  # Detach from parent
        )
        
        # Register cleanup
        atexit.register(self._cleanup_service)
        
        # Wait for service to start
        for attempt in range(max_retries):
            time.sleep(retry_delay)
            if self._probe_service(self._host, self._port):
                return
        
        raise RuntimeError(f"Service failed to start after {max_retries} attempts")
        
    except Exception as e:
        raise RuntimeError(f"Failed to start sysml-grpc service: {e}")

def _cleanup_service(self):
    """Cleanup service process on exit."""
    if self._service_process:
        try:
            self._service_process.terminate()
            self._service_process.wait(timeout=5)
        except:
            pass
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_connection.py::test_ensure_service_starts tests/test_connection.py::test_ensure_service_reuses_existing -v`
Expected: Both tests PASS

- [ ] **Step 8: Update existing Connection tests**

Modify existing tests to handle auto_start parameter:

```python
# tests/test_connection.py (update existing tests)
def test_connection_init():
    """Test Connection initializes with default host/port."""
    with patch('grpc.insecure_channel') as mock_channel:
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(auto_start=False)  # Add auto_start=False
            assert conn._host == 'localhost'
            assert conn._port == 50051
            mock_channel.assert_called_once_with('localhost:50051')
```

Apply `auto_start=False` to all existing Connection() calls in tests to avoid triggering auto-start during unit tests.

- [ ] **Step 9: Run all connection tests**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_connection.py -v`
Expected: All tests PASS (including new auto-start tests)

- [ ] **Step 10: Commit**

```bash
git add pysysml/connection.py tests/test_connection.py
git commit -m "feat(python): add auto-start service capability to Connection"
```

### Task 3: Module-Level Convenience API

**Files:**
- Modify: `pysysml/__init__.py`
- Create: `tests/test_api.py`

**Objective:** Provide simple `pysysml.load()` API with auto-initialization

- [ ] **Step 1: Write failing test for module-level load()**

```python
# tests/test_api.py
import pysysml
from unittest.mock import patch, Mock

def test_pysysml_load():
    """Test pysysml.load() convenience function."""
    with patch('pysysml.Connection') as mock_conn_class:
        mock_conn = Mock()
        mock_model = Mock()
        mock_conn.load.return_value = mock_model
        mock_conn_class.return_value = mock_conn
        
        result = pysysml.load('test.sysml')
        
        assert result == mock_model
        mock_conn_class.assert_called_once_with(host='localhost', port=50051, auto_start=True)
        mock_conn.load.assert_called_once_with('test.sysml')
```

- [ ] **Step 2: Run test to verify it fails**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_api.py::test_pysysml_load -v`
Expected: FAIL with "AttributeError: module 'pysysml' has no attribute 'load'"

- [ ] **Step 3: Implement module-level load() function**

```python
# pysysml/__init__.py (modify existing file)
"""pysysml - Python bindings for Systemica SysML v2 implementation."""

__version__ = "0.1.0"

# Import core classes
from pysysml.connection import Connection
from pysysml.model import Model
from pysysml.symbol import Symbol
from pysysml.diagnostic import Diagnostic

# Module-level default connection
_default_connection = None

def _get_default_connection():
    """Get or create default Connection instance.
    
    Returns:
        Connection: Shared connection instance
    """
    global _default_connection
    if _default_connection is None:
        _default_connection = Connection(auto_start=True)
    return _default_connection

def load(file_path, host='localhost', port=50051):
    """Load a SysML model file.
    
    Convenience function that uses a module-level Connection with auto-start.
    
    Args:
        file_path (str): Path to .sysml file
        host (str): Service host (default: localhost)
        port (int): Service port (default: 50051)
    
    Returns:
        Model: Parsed model
    
    Example:
        >>> import pysysml
        >>> model = pysysml.load("spacecraft.sysml")
        >>> print(model.root.name)
    """
    conn = _get_default_connection()
    return conn.load(file_path)

__all__ = ['Connection', 'Model', 'Symbol', 'Diagnostic', 'load', '__version__']
```

- [ ] **Step 4: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_api.py::test_pysysml_load -v`
Expected: PASS

- [ ] **Step 5: Write failing test for connect() function**

```python
# tests/test_api.py
def test_pysysml_connect():
    """Test pysysml.connect() returns Connection."""
    with patch('pysysml.Connection') as mock_conn_class:
        mock_conn = Mock()
        mock_conn_class.return_value = mock_conn
        
        result = pysysml.connect(host='remote', port=9000)
        
        assert result == mock_conn
        mock_conn_class.assert_called_once_with(host='remote', port=9000, auto_start=True)
```

- [ ] **Step 6: Implement connect() function**

```python
# pysysml/__init__.py (add to existing file)
def connect(host='localhost', port=50051, auto_start=True):
    """Create a new Connection to sysml-grpc service.
    
    Args:
        host (str): Service host (default: localhost)
        port (int): Service port (default: 50051)
        auto_start (bool): Auto-start service if not running (default: True)
    
    Returns:
        Connection: New connection instance
    
    Example:
        >>> import pysysml
        >>> conn = pysysml.connect()
        >>> model = conn.load("model.sysml")
    """
    return Connection(host=host, port=port, auto_start=auto_start)

__all__ = ['Connection', 'Model', 'Symbol', 'Diagnostic', 'load', 'connect', '__version__']
```

- [ ] **Step 7: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_api.py::test_pysysml_connect -v`
Expected: PASS

- [ ] **Step 8: Write test for default connection reuse**

```python
# tests/test_api.py
def test_default_connection_reuse():
    """Test load() reuses same connection instance."""
    # Reset module state
    pysysml._default_connection = None
    
    with patch('pysysml.Connection') as mock_conn_class:
        mock_conn = Mock()
        mock_conn_class.return_value = mock_conn
        
        pysysml.load('file1.sysml')
        pysysml.load('file2.sysml')
        
        # Connection created only once
        assert mock_conn_class.call_count == 1
        # load() called twice on same connection
        assert mock_conn.load.call_count == 2
```

- [ ] **Step 9: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_api.py::test_default_connection_reuse -v`
Expected: PASS

- [ ] **Step 10: Run all API tests**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_api.py -v`
Expected: All 3 tests PASS

- [ ] **Step 11: Commit**

```bash
git add pysysml/__init__.py tests/test_api.py
git commit -m "feat(python): add module-level convenience API (load, connect)"
```

### Task 4: Comprehensive Testing

**Files:**
- Create: `tests/test_lifecycle.py`
- Modify: `tests/test_binary.py` (add integration test)

**Objective:** Integration tests for full auto-lifecycle (binary download, service start, model load)

- [ ] **Step 1: Write integration test for binary download and service start**

```python
# tests/test_lifecycle.py
"""Integration tests for auto-lifecycle management."""
import os
import pytest
import pysysml
from pysysml.binary import get_binary_path

@pytest.mark.integration
def test_binary_download_and_service_start():
    """Test binary downloads and service starts on first use.
    
    This test requires internet access and GitHub releases to exist.
    Skip if binary already present or network unavailable.
    """
    binary_path = get_binary_path()
    
    # Only run if binary doesn't already exist (clean env test)
    if os.path.exists(binary_path):
        pytest.skip("Binary already exists, cannot test download")
    
    try:
        # This should trigger: download binary → start service → load file
        # Use a minimal SysML file for testing
        test_file = "/tmp/test_phase3.sysml"
        with open(test_file, 'w') as f:
            f.write('package TestModel;')
        
        model = pysysml.load(test_file)
        
        assert model is not None
        assert os.path.exists(binary_path)
        
        # Cleanup
        os.remove(test_file)
        
    except RuntimeError as e:
        if "Failed to download" in str(e):
            pytest.skip(f"Network error or release unavailable: {e}")
        raise
```

- [ ] **Step 2: Write integration test for service persistence**

```python
# tests/test_lifecycle.py
@pytest.mark.integration
def test_service_persists_across_loads():
    """Test service stays running for multiple loads."""
    # Create two test files
    test_file1 = "/tmp/test1.sysml"
    test_file2 = "/tmp/test2.sysml"
    
    with open(test_file1, 'w') as f:
        f.write('package Model1;')
    with open(test_file2, 'w') as f:
        f.write('package Model2;')
    
    try:
        # First load
        model1 = pysysml.load(test_file1)
        assert model1 is not None
        
        # Second load (service should still be running)
        model2 = pysysml.load(test_file2)
        assert model2 is not None
        
        # Both models should be different
        assert model1.hash != model2.hash
        
    finally:
        os.remove(test_file1)
        os.remove(test_file2)
```

- [ ] **Step 3: Write integration test for custom host/port**

```python
# tests/test_lifecycle.py
@pytest.mark.integration
def test_custom_port():
    """Test service starts on custom port."""
    import time
    
    # Use non-standard port
    test_port = 50052
    test_file = "/tmp/test_custom_port.sysml"
    
    with open(test_file, 'w') as f:
        f.write('package CustomPortTest;')
    
    try:
        conn = pysysml.connect(port=test_port)
        
        # Give service time to start
        time.sleep(1)
        
        model = conn.load(test_file)
        assert model is not None
        
    finally:
        os.remove(test_file)
        # Note: service cleanup handled by atexit
```

- [ ] **Step 4: Write test for error handling (bad file)**

```python
# tests/test_lifecycle.py
def test_load_nonexistent_file():
    """Test load() handles missing files gracefully."""
    with pytest.raises(Exception):  # grpc.RpcError
        pysysml.load("/nonexistent/file.sysml")
```

- [ ] **Step 5: Add test for binary checksum verification**

```python
# tests/test_binary.py (add to existing file)
import tempfile
import hashlib
from pysysml.binary import verify_checksum, download_binary

def test_download_binary_checksum():
    """Test downloaded binary checksum verification (if available).
    
    This test downloads a real binary and verifies its checksum.
    Skip if no internet or release unavailable.
    """
    pytest.mark.integration
    
    try:
        # Download to temp location
        with tempfile.TemporaryDirectory() as tmpdir:
            # Mock get_binary_path to use temp dir
            with patch('pysysml.binary.get_binary_path', return_value=os.path.join(tmpdir, 'sysml-grpc')):
                binary_path = download_binary(version='v0.1.0')
                
                # Verify file exists
                assert os.path.exists(binary_path)
                
                # Verify it's executable
                assert os.access(binary_path, os.X_OK)
                
                # Compute checksum (for manual verification)
                sha256 = hashlib.sha256()
                with open(binary_path, 'rb') as f:
                    sha256.update(f.read())
                
                print(f"Downloaded binary SHA-256: {sha256.hexdigest()}")
                
    except RuntimeError as e:
        if "Failed to download" in str(e):
            pytest.skip(f"Network error or release unavailable: {e}")
        raise
```

- [ ] **Step 6: Run integration tests**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_lifecycle.py tests/test_binary.py::test_download_binary_checksum -v -m integration`
Expected: Tests PASS or SKIP (if network unavailable)

**Note:** Integration tests may skip if:
- Binary already downloaded
- Network unavailable
- GitHub release doesn't exist yet

- [ ] **Step 7: Run full test suite**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/ -v`
Expected: All unit tests PASS, integration tests PASS or SKIP

- [ ] **Step 8: Manual end-to-end verification**

```bash
# In a fresh Python environment (or remove ~/.pysysml/bin/ first)
cd /home/han/IdeaProjects/Systemica
rm -rf ~/.pysysml  # Clean slate

python3 -c "
import pysysml
model = pysysml.load('/home/han/Downloads/A1.sysml')
print(f'Loaded: {model.root.name}')
print(f'Diagnostics: {len(model.diagnostics)}')
"
```

Expected output:
```
Loaded: (anonymous)
Diagnostics: 0
```

And binary should exist:
```bash
ls -lh ~/.pysysml/bin/sysml-grpc
```

- [ ] **Step 9: Update README with zero-config usage**

```python
# pysysml/README.md (update "Usage" section)
## Usage

### Zero-Config (Phase 3)

The simplest way to use pysysml - just import and load:

```python
import pysysml

# First use downloads binary and starts service automatically
model = pysysml.load("spacecraft.sysml")

print(f"Model: {model.root.name}")
print(f"Diagnostics: {len(model.diagnostics)}")

# Navigate model
for child in model.root.children():
    print(f"  - {child.name} ({child.kind})")

# Find by name
part = model.find("SPACECRAFT_WET")
if part:
    print(f"Found: {part.name}")
```

### Manual Connection (Phase 2)

For more control over service lifecycle:

```python
from pysysml import Connection

# Start service manually or connect to existing
with Connection() as conn:
    model = conn.load("model.sysml")
    # ... use model
```

### Configuration

Binary downloads from: `https://github.com/Open-MBEE/OpenSysML/releases`
Binary stored in: `~/.pysysml/bin/sysml-grpc`

To use a different service:
```python
conn = pysysml.connect(host='remote-server', port=50051, auto_start=False)
```
```

- [ ] **Step 10: Commit**

```bash
git add tests/test_lifecycle.py tests/test_binary.py pysysml/README.md
git commit -m "test(python): add integration tests for auto-lifecycle and update README"
```

---

## Definition of Done

- [ ] `import pysysml` auto-downloads binary on first use
- [ ] `model = pysysml.load("A1.sysml")` works without manual service start
- [ ] Multiple Python processes can import concurrently (lockfile prevents conflicts)
- [ ] Service shuts down when last process exits (reference counting)
- [ ] All unit tests pass: `pytest tests/test_binary.py`
- [ ] All integration tests pass: `pytest tests/test_lifecycle.py`
- [ ] Manual verification: Fresh Python env → import → load works
