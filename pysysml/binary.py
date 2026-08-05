"""Binary management for sysml-grpc service."""

import hashlib
import os
import platform
import stat
import urllib.error
import urllib.request


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


def get_binary_path():
    """Get the local path where sysml-grpc binary should be stored.
    
    Returns:
        str: Absolute path to binary (e.g. ~/.pysysml/bin/sysml-grpc)
    """
    os_name, _ = detect_platform()
    binary_name = 'sysml-grpc.exe' if os_name == 'windows' else 'sysml-grpc'
    
    base_dir = os.path.expanduser('~/.pysysml/bin')
    return os.path.join(base_dir, binary_name)


def download_binary(version='latest', github_repo='Open-MBEE/Systemica'):
    """Download sysml-grpc binary from GitHub releases with checksum verification.
    
    Args:
        version (str): Release version tag (e.g., 'v0.1.0')
        github_repo (str): GitHub repository (owner/repo)
    
    Returns:
        str: Path to downloaded binary
    
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
