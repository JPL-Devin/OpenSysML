"""Binary management for sysml-grpc service."""

import hashlib
import json
import os
import platform
import stat
import urllib.error
import urllib.request
from pysysml.errors import ConnectionError

# Releases publish sysml-grpc-<goos>-<goarch> raw, with a .sha256 sidecar.
# PYSYSML_GITHUB_REPO overrides the repository they are fetched from.
DEFAULT_GITHUB_REPO = 'Open-MBEE/Systemica'


def default_github_repo():
    """Repository releases are downloaded from.

    Returns:
        str: owner/repo, from $PYSYSML_GITHUB_REPO or the default
    """
    return os.environ.get('PYSYSML_GITHUB_REPO') or DEFAULT_GITHUB_REPO


def resolve_latest_version(github_repo=None):
    """Resolve the tag of the repository's latest published release.

    Args:
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str: Release tag (e.g. 'v0.0.4')

    Raises:
        ConnectionError: If the release cannot be queried or carries no tag
    """
    repo = github_repo or default_github_repo()
    url = f'https://api.github.com/repos/{repo}/releases/latest'
    try:
        with urllib.request.urlopen(url) as response:
            release = json.loads(response.read().decode('utf-8'))
    except (urllib.error.URLError, ValueError) as e:
        raise ConnectionError(f"Failed to resolve latest release from {url}: {e}")

    tag = release.get('tag_name')
    if not tag:
        raise ConnectionError(f"Latest release of {repo} has no tag name")
    return tag


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
        raise ConnectionError(f"Unsupported operating system: {system}")
    
    # Map Python machine architecture to Go GOARCH
    machine = platform.machine().lower()
    if machine in ('x86_64', 'amd64'):
        arch = 'amd64'
    elif machine in ('aarch64', 'arm64'):
        arch = 'arm64'
    else:
        raise ConnectionError(f"Unsupported architecture: {machine}")
    
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


def download_binary(version='latest', github_repo=None):
    """Download sysml-grpc binary from GitHub releases with checksum verification.
    
    Args:
        version (str): Release version tag (e.g., 'v0.1.0'), or 'latest'
        github_repo (str, optional): GitHub repository (owner/repo)
    
    Returns:
        str: Path to downloaded binary
    
    Raises:
        RuntimeError: If download fails or checksum mismatch
    """
    github_repo = github_repo or default_github_repo()
    if version == 'latest':
        version = resolve_latest_version(github_repo)
    
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
            raise ConnectionError(
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
        raise ConnectionError(f"Failed to download binary from {binary_url}: {e}")


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


def ensure_binary(force_download=False, version=None):
    """Ensure sysml-grpc binary is available, downloading if necessary.
    
    Args:
        force_download (bool): If True, download even if binary exists
        version (str, optional): Specific version tag to download (e.g. 'v0.1.0'),
                                 or 'latest' for the newest release. If None,
                                 $PYSYSML_GRPC_VERSION is used; without it
                                 auto-download is disabled and the binary must be
                                 pre-installed via `make build` or downloaded manually.
    
    Returns:
        str: Path to binary
    
    Raises:
        ConnectionError: If binary cannot be obtained
    """
    from pysysml.errors import ConnectionError
    binary_path = get_binary_path()
    if version is None:
        version = os.environ.get('PYSYSML_GRPC_VERSION') or None
    
    # Check if binary already exists and is executable
    if not force_download and os.path.exists(binary_path):
        if os.access(binary_path, os.X_OK):
            return binary_path
    
    # If no binary and no version specified, cannot auto-download
    if version is None:
        raise ConnectionError(
            f"Binary not found at {binary_path} and auto-download disabled. "
            f"Build via `make build`, set $PYSYSML_GRPC_VERSION, or specify "
            f"version= parameter for download."
        )
    
    # Download binary with explicit version
    return download_binary(version=version)
