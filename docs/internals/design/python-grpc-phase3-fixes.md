# Phase 3 DoD Gaps - Fix Plan

**Goal:** Complete Phase 3 Definition of Done by implementing missing lockfile coordination, reference-counted shutdown, and checksum verification.

**Issues to Fix:**
1. Multi-process coordination missing (DoD line 1033)
2. Reference-counted shutdown missing (DoD line 1034)
3. `verify_checksum` dead code - downloads unverified

---

## File Structure

**Modified:**
- `pysysml/binary.py` - integrate checksum verification into download flow
- `pysysml/connection.py` - add lockfile coordination + reference counting
- `tests/test_binary.py` - verify checksums enforced
- `tests/test_connection.py` - test multi-process scenarios
- `tests/test_lifecycle.py` - verify reference-counted shutdown
- `setup.py` / `pyproject.toml` - add `filelock` dependency

**Created:**
- None (all changes to existing files)

---

## Task 1: Add Lockfile Coordination

**Files:**
- Modify: `pysysml/connection.py`
- Modify: `setup.py`, `pyproject.toml`
- Test: `tests/test_connection.py`

**Objective:** Prevent race condition when multiple processes try to auto-start service simultaneously.

**Design:**
- Lockfile path: `~/.pysysml/sysml-grpc.lock`
- Use `filelock` library (cross-platform)
- Lock acquired before checking if service running
- PID written to lockfile for reference tracking
- Lock released after service confirmed healthy

### Implementation

- [ ] **Step 1: Add filelock dependency**

Add to `setup.py`:
```python
install_requires=[
    "grpcio>=1.83.0",
    "protobuf>=7.35.1",
    "filelock>=3.0.0",  # Add this
]
```

Add to `pyproject.toml`:
```toml
dependencies = [
    "grpcio>=1.83.0",
    "protobuf>=7.35.1",
    "filelock>=3.0.0",  # Add this
]
```

- [ ] **Step 2: Install filelock**

Run: `pip install filelock`
Expected: Package installed successfully

- [ ] **Step 3: Write failing test for lockfile acquisition**

```python
# tests/test_connection.py
from filelock import FileLock
import os

def test_ensure_service_uses_lockfile():
    """Test that _ensure_service acquires lockfile before starting service."""
    with patch('pysysml.connection.ensure_binary') as mock_ensure:
        mock_ensure.return_value = '/path/to/sysml-grpc'
        
        with patch('pysysml.connection._probe_service', return_value=False):
            with patch('subprocess.Popen') as mock_popen:
                mock_popen.return_value = Mock(pid=12345)
                
                # Mock time.sleep to skip retries
                with patch('time.sleep'):
                    with patch('pysysml.connection._probe_service', side_effect=[False, True]):
                        conn = Connection(auto_start=True)
                        
                        # Verify lockfile was created
                        lockfile_path = os.path.expanduser('~/.pysysml/sysml-grpc.lock')
                        assert os.path.exists(lockfile_path)

def test_concurrent_ensure_service_blocks():
    """Test that second process blocks while first starts service."""
    lockfile_path = os.path.expanduser('~/.pysysml/sysml-grpc.lock')
    
    # Simulate first process holding lock
    lock1 = FileLock(lockfile_path, timeout=0.1)
    lock1.acquire()
    
    try:
        # Second process should timeout trying to acquire
        with pytest.raises(TimeoutError):
            with patch('pysysml.connection.ensure_binary', return_value='/path/to/binary'):
                with patch('pysysml.connection._probe_service', return_value=False):
                    Connection(auto_start=True)
    finally:
        lock1.release()
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_connection.py::test_ensure_service_uses_lockfile -v`
Expected: FAIL with "FileNotFoundError: lockfile_path"

- [ ] **Step 5: Implement lockfile coordination in _ensure_service**

```python
# pysysml/connection.py
from filelock import FileLock, Timeout
import os

def _get_lockfile_path():
    """Get path to service lockfile."""
    pysysml_dir = os.path.expanduser('~/.pysysml')
    os.makedirs(pysysml_dir, exist_ok=True)
    return os.path.join(pysysml_dir, 'sysml-grpc.lock')

def _get_pidfile_path():
    """Get path to service PID file."""
    pysysml_dir = os.path.expanduser('~/.pysysml')
    return os.path.join(pysysml_dir, 'sysml-grpc.pid')

def _ensure_service(self):
    """Ensure sysml-grpc service is running, with lockfile coordination.
    
    Uses filelock to coordinate between multiple Python processes.
    If service already running, returns immediately.
    Otherwise, acquires lock and starts service.
    """
    lockfile_path = _get_lockfile_path()
    lock = FileLock(lockfile_path, timeout=30)
    
    try:
        with lock:
            # Check if service already running (another process may have started it)
            if self._probe_service(self.host, self.port):
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
            
            self._service_process = process
            
            # Write PID to file for reference counting
            pidfile_path = _get_pidfile_path()
            with open(pidfile_path, 'w') as f:
                f.write(f"{process.pid}\n")
            
            # Wait for service to become healthy
            max_retries = 5
            retry_delay = 0.5
            startup_timeout = 10.0
            
            for attempt in range(max_retries):
                time.sleep(retry_delay)
                if self._probe_service(self.host, self.port, timeout=2.0):
                    # Register cleanup
                    atexit.register(self._cleanup_service)
                    return
            
            # Service didn't start in time
            self._cleanup_service()
            raise RuntimeError(f"Service failed to start within {startup_timeout}s")
            
    except Timeout:
        raise RuntimeError(
            f"Timeout acquiring service lockfile after 30s. "
            f"Another process may be starting the service."
        )
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_connection.py::test_ensure_service_uses_lockfile tests/test_connection.py::test_concurrent_ensure_service_blocks -v`
Expected: PASS (2 tests)

- [ ] **Step 7: Commit lockfile coordination**

```bash
git add pysysml/connection.py tests/test_connection.py setup.py pyproject.toml
git commit -m "feat(connection): add lockfile coordination for multi-process service startup"
```

---

## Task 2: Implement Reference-Counted Shutdown

**Files:**
- Modify: `pysysml/connection.py`
- Modify: `tests/test_lifecycle.py`

**Objective:** Track how many processes are using the service, shut down only when last process exits.

**Design:**
- Reference count file: `~/.pysysml/sysml-grpc.refcount`
- Atomic increment on service start
- Atomic decrement on cleanup
- Terminate service when refcount reaches 0
- Use lockfile for atomic refcount operations

### Implementation

- [ ] **Step 1: Write failing test for reference counting**

```python
# tests/test_lifecycle.py
def test_service_shuts_down_when_last_process_exits():
    """Test that service terminates when reference count reaches 0."""
    import time
    
    # First connection increments refcount to 1
    with patch('pysysml.binary.ensure_binary', return_value=get_binary_path()):
        conn1 = Connection(auto_start=True)
        
        # Get PID
        pidfile = os.path.expanduser('~/.pysysml/sysml-grpc.pid')
        with open(pidfile) as f:
            pid = int(f.read().strip())
        
        # Service should be running
        assert psutil.Process(pid).is_running()
        
        # Second connection increments to 2
        conn2 = Connection(auto_start=True)
        
        # Close first connection (refcount -> 1)
        conn1.close()
        time.sleep(0.5)
        
        # Service should still be running
        assert psutil.Process(pid).is_running()
        
        # Close second connection (refcount -> 0)
        conn2.close()
        time.sleep(0.5)
        
        # Service should be terminated
        assert not psutil.Process(pid).is_running()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_lifecycle.py::test_service_shuts_down_when_last_process_exits -v`
Expected: FAIL (service doesn't shut down)

- [ ] **Step 3: Implement reference counting**

```python
# pysysml/connection.py

def _get_refcount_path():
    """Get path to service reference count file."""
    pysysml_dir = os.path.expanduser('~/.pysysml')
    return os.path.join(pysysml_dir, 'sysml-grpc.refcount')

def _increment_refcount():
    """Atomically increment service reference count.
    
    Returns:
        int: New reference count
    """
    lockfile_path = _get_lockfile_path()
    lock = FileLock(lockfile_path, timeout=5)
    
    with lock:
        refcount_path = _get_refcount_path()
        
        # Read current count
        if os.path.exists(refcount_path):
            with open(refcount_path, 'r') as f:
                count = int(f.read().strip())
        else:
            count = 0
        
        # Increment
        count += 1
        
        # Write back
        with open(refcount_path, 'w') as f:
            f.write(str(count))
        
        return count

def _decrement_refcount():
    """Atomically decrement service reference count.
    
    Returns:
        int: New reference count (0 if file doesn't exist)
    """
    lockfile_path = _get_lockfile_path()
    lock = FileLock(lockfile_path, timeout=5)
    
    with lock:
        refcount_path = _get_refcount_path()
        
        if not os.path.exists(refcount_path):
            return 0
        
        # Read current count
        with open(refcount_path, 'r') as f:
            count = int(f.read().strip())
        
        # Decrement
        count = max(0, count - 1)
        
        # Write back
        if count > 0:
            with open(refcount_path, 'w') as f:
                f.write(str(count))
        else:
            # Remove file when count reaches 0
            os.remove(refcount_path)
        
        return count

# Update _ensure_service to increment refcount
def _ensure_service(self):
    """Ensure sysml-grpc service is running, with lockfile coordination."""
    # ... existing lockfile logic ...
    
    # After service confirmed healthy:
    _increment_refcount()
    atexit.register(self._cleanup_service)
    return

# Update _cleanup_service to check refcount
def _cleanup_service(self):
    """Clean up service process with reference counting.
    
    Only terminates service if reference count reaches 0.
    """
    # Decrement refcount
    new_count = _decrement_refcount()
    
    if new_count == 0:
        # Last connection - shut down service
        pidfile_path = _get_pidfile_path()
        
        if os.path.exists(pidfile_path):
            with open(pidfile_path, 'r') as f:
                pid = int(f.read().strip())
            
            try:
                import psutil
                process = psutil.Process(pid)
                process.terminate()
                process.wait(timeout=5)
            except (psutil.NoSuchProcess, psutil.TimeoutExpired):
                # Process already dead or timeout - force kill
                try:
                    process.kill()
                except psutil.NoSuchProcess:
                    pass
            
            # Clean up PID file
            os.remove(pidfile_path)
    
    # Clean up instance state
    if self._service_process:
        self._service_process = None
```

- [ ] **Step 4: Add psutil dependency**

Add to `setup.py` and `pyproject.toml`:
```python
install_requires=[
    "grpcio>=1.83.0",
    "protobuf>=7.35.1",
    "filelock>=3.0.0",
    "psutil>=5.9.0",  # Add this
]
```

- [ ] **Step 5: Install psutil**

Run: `pip install psutil`
Expected: Package installed successfully

- [ ] **Step 6: Run test to verify it passes**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_lifecycle.py::test_service_shuts_down_when_last_process_exits -v`
Expected: PASS

- [ ] **Step 7: Update existing persistence tests**

The tests `test_service_survives_connection_close` and `test_service_persists_across_multiple_loads` currently verify service persists. Update them to use multiple connections so refcount stays > 0.

```python
# tests/test_lifecycle.py - update test_service_survives_connection_close
def test_service_survives_connection_close():
    """Test that service persists when other connections still active."""
    # ... existing setup ...
    
    # Create TWO connections to keep refcount > 0
    conn1 = Connection(auto_start=False, host='localhost', port=50051)
    conn2 = Connection(auto_start=False, host='localhost', port=50051)
    
    # Close first connection
    conn1.close()
    time.sleep(0.5)
    
    # Service should still be running (conn2 keeps it alive)
    response = conn2.stub.GetDiagnostics(sysml_pb2.DiagnosticsRequest(model_hash="test"))
    assert response is not None  # Service responds
    
    conn2.close()
```

- [ ] **Step 8: Run full test suite**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/ -v`
Expected: All tests pass

- [ ] **Step 9: Commit reference counting**

```bash
git add pysysml/connection.py tests/test_lifecycle.py setup.py pyproject.toml
git commit -m "feat(connection): add reference-counted service shutdown"
```

---

## Task 3: Integrate Checksum Verification

**Files:**
- Modify: `pysysml/binary.py`
- Modify: `tests/test_binary.py`

**Objective:** Verify downloaded binaries against checksums before using them.

**Design:**
- Checksum file: `https://github.com/Open-MBEE/OpenSysML/releases/download/{version}/{binary_name}.sha256`
- Download checksum file alongside binary
- Verify before marking download complete
- Fail if checksum mismatch

### Implementation

- [ ] **Step 1: Write failing test for checksum enforcement**

```python
# tests/test_binary.py
def test_download_binary_verifies_checksum():
    """Test that download_binary fetches and verifies checksum."""
    version = 'v0.1.0'
    github_repo = 'Open-MBEE/OpenSysML'
    
    # Mock binary download
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    
    # Mock checksum file download
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        # First call: checksum file
        # Second call: binary
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data)))),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))))
        ]
        
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            result = download_binary(version, github_repo)
            
            # Should have called urlopen twice (checksum + binary)
            assert mock_urlopen.call_count == 2
            
            # Verify checksum URL was constructed correctly
            checksum_url = mock_urlopen.call_args_list[0][0][0]
            assert '.sha256' in checksum_url

def test_download_binary_fails_on_checksum_mismatch():
    """Test that download fails if checksum doesn't match."""
    version = 'v0.1.0'
    github_repo = 'Open-MBEE/OpenSysML'
    
    # Mock binary download
    mock_binary_data = b'fake binary content'
    
    # Wrong checksum
    wrong_checksum = 'deadbeef' * 8
    mock_checksum_data = f"{wrong_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data)))),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))))
        ]
        
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with pytest.raises(RuntimeError, match="Checksum mismatch"):
                download_binary(version, github_repo)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_download_binary_verifies_checksum tests/test_binary.py::test_download_binary_fails_on_checksum_mismatch -v`
Expected: FAIL (checksum not downloaded/verified)

- [ ] **Step 3: Update download_binary to fetch and verify checksum**

```python
# pysysml/binary.py
import hashlib

def download_binary(version='v0.1.0', github_repo='Open-MBEE/OpenSysML'):
    """Download sysml-grpc binary from GitHub releases with checksum verification.
    
    Args:
        version (str): Release version tag (e.g., 'v0.1.0')
        github_repo (str): GitHub repository (owner/repo)
    
    Raises:
        RuntimeError: If download fails or checksum mismatch
    """
    if version == 'latest':
        raise NotImplementedError(
            "version='latest' not yet supported. "
            "Specify explicit version tag like 'v0.1.0'"
        )
    
    goos, goarch = detect_platform()
    binary_name = f'sysml-grpc-{goos}-{goarch}'
    if goos == 'windows':
        binary_name += '.exe'
    
    # Construct URLs
    base_url = f'https://github.com/{github_repo}/releases/download/{version}'
    binary_url = f'{base_url}/{binary_name}'
    checksum_url = f'{base_url}/{binary_name}.sha256'
    
    binary_path = get_binary_path()
    os.makedirs(os.path.dirname(binary_path), exist_ok=True)
    
    try:
        # Download checksum file first
        import urllib.request
        
        with urllib.request.urlopen(checksum_url) as response:
            checksum_content = response.read().decode('utf-8')
        
        # Parse checksum (format: "hexdigest  filename\n")
        expected_checksum = checksum_content.split()[0]
        
        # Download binary
        with urllib.request.urlopen(binary_url) as response:
            binary_data = response.read()
        
        # Write to temporary file first
        temp_path = binary_path + '.tmp'
        with open(temp_path, 'wb') as f:
            f.write(binary_data)
        
        # Verify checksum
        if not verify_checksum(temp_path, expected_checksum):
            os.remove(temp_path)
            raise RuntimeError(
                f"Checksum mismatch for {binary_name}. "
                f"Expected {expected_checksum}, but download does not match. "
                f"Binary may be corrupted or tampered with."
            )
        
        # Checksum valid - move to final location
        os.rename(temp_path, binary_path)
        
        # Make executable
        os.chmod(binary_path, 0o755)
        
        return binary_path
        
    except urllib.error.URLError as e:
        raise RuntimeError(f"Failed to download binary from {binary_url}: {e}")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/test_binary.py::test_download_binary_verifies_checksum tests/test_binary.py::test_download_binary_fails_on_checksum_mismatch -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Verify verify_checksum is no longer dead code**

Run: `grep -n "verify_checksum" pysysml/binary.py`
Expected: Should show both definition (line ~102) and call site (in download_binary)

- [ ] **Step 6: Run full test suite**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/ -v`
Expected: All tests pass

- [ ] **Step 7: Commit checksum verification**

```bash
git add pysysml/binary.py tests/test_binary.py
git commit -m "feat(binary): add checksum verification for downloaded binaries"
```

---

## Final Verification

- [ ] **Run full test suite**

Run: `PYTHONPATH=/home/han/IdeaProjects/Systemica pytest tests/ -v`
Expected: All tests pass

- [ ] **Run Go tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Verify Phase 3 DoD complete**

Check each DoD item:
- [x] `import pysysml` auto-downloads binary on first use
- [x] `model = pysysml.load("A1.sysml")` works without manual service start
- [x] Multiple Python processes can import concurrently (lockfile prevents conflicts)
- [x] Service shuts down when last process exits (reference counting)
- [x] Tests pass: `pytest tests/test_binary.py tests/test_lifecycle.py`
- [x] Checksums verified for security

- [ ] **Manual test: Multi-process scenario**

```python
# test_concurrent.py
import pysysml
import multiprocessing
import time

def worker(file_path, worker_id):
    print(f"Worker {worker_id} starting...")
    model = pysysml.load(file_path)
    print(f"Worker {worker_id} loaded: {model.root.name}")
    time.sleep(2)  # Hold connection
    print(f"Worker {worker_id} done")

if __name__ == '__main__':
    # Start 3 processes concurrently
    processes = []
    for i in range(3):
        p = multiprocessing.Process(target=worker, args=("A1.sysml", i))
        processes.append(p)
        p.start()
    
    # Wait for all to complete
    for p in processes:
        p.join()
    
    print("All workers completed")
    # Service should still be running (refcount = 3 -> 0)
```

Run: `python test_concurrent.py`
Expected: 
- All 3 workers load successfully
- No port conflicts
- Service shuts down after all workers exit

---

## Definition of Done

All items must be ✅:

- [ ] Lockfile coordination implemented and tested
- [ ] Reference-counted shutdown implemented and tested
- [ ] Checksum verification integrated into download_binary
- [ ] All unit tests pass (pytest tests/)
- [ ] All integration tests pass (pytest -m integration)
- [ ] Go tests pass (go test ./...)
- [ ] Manual multi-process test succeeds
- [ ] No orphaned sysml-grpc processes after tests
- [ ] Phase 3 DoD checklist 100% complete
