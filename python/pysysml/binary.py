"""Binary management for sysml-grpc service."""

import hashlib
import http.client
import json
import os
import platform
import stat
import urllib.request
import warnings
from pysysml.errors import (
    ChecksumMismatchError,
    ConnectionError,
    UnpinnedReleaseError,
)

# Releases publish sysml-grpc-<goos>-<goarch> raw, with a .sha256 sidecar.
# PYSYSML_GITHUB_REPO overrides the repository they are fetched from.
DEFAULT_GITHUB_REPO = 'Open-MBEE/Systemica'

# A cached binary is now checked against the releases API before being reused, and
# that happens while the service-start lock is held, so it must not hang there.
NETWORK_TIMEOUT = 15

#: SHA-256 digest this release of pysysml expects of each release asset, keyed by
#: repository, release tag and asset name. A digest pinned here does not come from
#: the origin serving the download, so a release republished with another binary is
#: refused rather than trusted. Produced by python/scripts/pin_release_checksums.py;
#: see "Pinned release digests" in python/README.md for the release procedure.
PINNED_SHA256 = {
    'Open-MBEE/Systemica': {
        'v0.0.5': {
            'sysml-grpc-darwin-amd64': 'ab2933f168341bed3157bac0026d2c5a51bdb1c4629618178123f7ca8e071e72',
            'sysml-grpc-darwin-arm64': '6e839899c93954671d39ebc91a751302d8329864e9671fc38f7549aa52bddde9',
            'sysml-grpc-linux-amd64': '9bc8203f05a3dfa3d5c4b784f87e38d70f0db416732a73ddf56aaa4c57c2a566',
            'sysml-grpc-linux-arm64': 'fd6addc9cb717787d12225c3092d653708e6f5aac19d987582228180b529ca13',
            'sysml-grpc-windows-amd64.exe': '0033248b904e0d10f83a5e033953372108ccad024ae530092431325339bc1179',
        },
        'v0.0.6': {
            'sysml-grpc-darwin-amd64': '5954ae0838858ea585943c7a986a049ac9109874047310e9612b292259c07937',
            'sysml-grpc-darwin-arm64': 'a46792146388f0ed8b736754c6f675d287769bb026a8dd5503f1abc8ed9036c3',
            'sysml-grpc-linux-amd64': 'cb6a4a6152b79321c1d432cdc895acd4d0c4d8e43dd72d38ce6b07d7ce427dd6',
            'sysml-grpc-linux-arm64': '8133f113c07b6f9f2772077add61c3c7e4ba3065000e8405d2c01bc67f3ec17d',
            'sysml-grpc-windows-amd64.exe': 'ae01081af3acc467fad7dbbfce10ec15c9307995a98a30773885622ba2e467db',
        },
        'v0.0.7': {
            'sysml-grpc-darwin-amd64': 'f83bc05ca77137e142e682f8b5e3858d223dc4be5637755b989d19094d6c9cf0',
            'sysml-grpc-darwin-arm64': '84d6e87ae7af59cad17d1e41b0a5234a78a2c1495c298ba9184e856d5582b9f5',
            'sysml-grpc-linux-amd64': '00ede1e52851ac9cf52e6a69d090ff81a0b188cd7b08ca83d75e27e708bddb58',
            'sysml-grpc-linux-arm64': '54a92ccd80868c3c02fb95f90d3714d0d517d68e9d42dbe5d0d1421ee6b52951',
            'sysml-grpc-windows-amd64.exe': '0bee1545673423dcad761e3d02453d6736ee56989aa17ee82c9a26553bb5bae6',
        },
        'v0.0.8': {
            'sysml-grpc-darwin-amd64': 'ac0b8f3d24ffe5250d1c375c90425014eed87a786bb977b848f274b2eaeb2ced',
            'sysml-grpc-darwin-arm64': '0e15230e8636f19a0c145652296faa7f4a18279755f201b396f23d49ba159a6e',
            'sysml-grpc-linux-amd64': '3afc97748c206cd9e52cb55d3e5918fe456f577a121f2fd67800e7383d4ec139',
            'sysml-grpc-linux-arm64': '31e3b10edc8e0cc53537f3be0d56e1a78ea703899afaec76875b502b9919c438',
            'sysml-grpc-windows-amd64.exe': '6bf92438d139e21f1005c41c3fcca5693bad3abf9c5fd8ca6dd18a041d20b101',
        },
    },
}

#: Set to the repository whose unpinned downloads may be accepted (`1` for any),
#: which is same-origin trust: the checksum then comes from whoever served the
#: binary.
ALLOW_UNPINNED_ENV = 'PYSYSML_ALLOW_UNPINNED_DOWNLOAD'


def default_github_repo():
    """Repository releases are downloaded from.

    Returns:
        str: owner/repo, from $PYSYSML_GITHUB_REPO or the default
    """
    return os.environ.get('PYSYSML_GITHUB_REPO') or DEFAULT_GITHUB_REPO


def pinned_digest(version, asset, github_repo=None):
    """The digest this release of pysysml pins for a release asset.

    Args:
        version (str): Release tag, resolved (never 'latest')
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str or None: SHA-256 hex digest, or None when nothing is pinned for it
    """
    repo = github_repo or default_github_repo()
    return PINNED_SHA256.get(repo, {}).get(version, {}).get(asset)


def unpinned_downloads_allowed(github_repo=None):
    """Whether an unpinned download from a repository may fall back to same-origin trust.

    Args:
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        bool: True when the variable is set to '1' (any repository) or names this one
    """
    allowed = os.environ.get(ALLOW_UNPINNED_ENV, '').strip()
    if allowed.lower() in ('', '0', 'false', 'no'):
        return False
    if allowed.lower() in ('1', 'true', 'yes'):
        return True
    repo = github_repo or default_github_repo()
    # Naming repositories keeps the trust with the fork it is granted for.
    return repo in [named.strip() for named in allowed.split(',')]


def expected_digest(version, asset, served_digest, github_repo=None):
    """The digest a download must have, preferring the pin over what was served.

    Args:
        version (str): Release tag, resolved (never 'latest')
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')
        served_digest (str): Digest from the .sha256 served beside the binary
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str: The digest to verify the download against

    Raises:
        ChecksumMismatchError: If the served digest contradicts the pinned one,
            which is a release republished with another binary
        UnpinnedReleaseError: If nothing is pinned for it and same-origin trust
            was not allowed explicitly
    """
    repo = github_repo or default_github_repo()
    pinned = pinned_digest(version, asset, repo)
    if pinned is None:
        if not unpinned_downloads_allowed(repo):
            raise UnpinnedReleaseError(
                f"pysysml pins no SHA-256 digest for {asset} of {version} of {repo}, so "
                f"the only checksum available is the one served beside the binary, which "
                f"a compromised release would serve too. Upgrade pysysml to a release "
                f"that pins {version}, ask for a pinned release with version=, or accept "
                f"same-origin trust for this repository by setting "
                f"${ALLOW_UNPINNED_ENV}={repo} (or =1 for any repository)."
            )
        warnings.warn(
            f"pysysml pins no digest for {asset} of {version} of {repo}; verifying it "
            f"against the checksum served beside it, which detects corruption but not a "
            f"compromised release (${ALLOW_UNPINNED_ENV} is set).",
            RuntimeWarning,
            stacklevel=3,
        )
        return served_digest
    if served_digest != pinned:
        raise ChecksumMismatchError(
            f"Checksum mismatch for {asset} of {version}: {repo} serves {served_digest}, "
            f"but pysysml pins {pinned}. The release was republished with another binary, "
            f"or the download is being tampered with; it was not installed."
        )
    return pinned


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
        with urllib.request.urlopen(url, timeout=NETWORK_TIMEOUT) as response:
            release = json.loads(response.read().decode('utf-8'))
    # urlopen leaves read-phase failures unwrapped, so a timeout, a reset
    # connection or a truncated body is not a URLError.
    except (OSError, http.client.HTTPException, ValueError) as e:
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


def release_asset_name(goos=None, goarch=None):
    """Name a release publishes the service binary for a platform under.

    Args:
        goos (str, optional): GOOS, detected when omitted
        goarch (str, optional): GOARCH, detected when omitted

    Returns:
        str: Asset name (e.g. 'sysml-grpc-linux-amd64')
    """
    if goos is None or goarch is None:
        detected_os, detected_arch = detect_platform()
        goos = goos or detected_os
        goarch = goarch or detected_arch
    name = f'sysml-grpc-{goos}-{goarch}'
    return name + '.exe' if goos == 'windows' else name


def get_binary_path():
    """Get the local path where sysml-grpc binary should be stored.
    
    Returns:
        str: Absolute path to binary (e.g. ~/.pysysml/bin/sysml-grpc)
    """
    os_name, _ = detect_platform()
    binary_name = 'sysml-grpc.exe' if os_name == 'windows' else 'sysml-grpc'
    
    base_dir = os.path.expanduser('~/.pysysml/bin')
    return os.path.join(base_dir, binary_name)


def metadata_path():
    """Path of the record of which release the cached binary was downloaded from.

    Returns:
        str: Absolute path to the sidecar (e.g. ~/.pysysml/bin/sysml-grpc.json)
    """
    return get_binary_path() + '.json'


def read_metadata():
    """Read the record beside the cached binary.

    Returns:
        dict: What was recorded, or empty when nothing readable was
    """
    try:
        with open(metadata_path(), 'r') as f:
            recorded = json.load(f)
    except (OSError, ValueError):
        return {}
    return recorded if isinstance(recorded, dict) else {}


def write_metadata(version, sha256, github_repo=None):
    """Record which release of which repository the cached binary is, and its digest.

    Args:
        version (str): Release tag downloaded, resolved (never 'latest')
        sha256 (str): SHA-256 hex digest of the binary written
        github_repo (str, optional): GitHub repository (owner/repo) downloaded from
    """
    path = metadata_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        json.dump({
            'version': version,
            'sha256': sha256,
            'repo': github_repo or default_github_repo(),
        }, f)


def cached_release(github_repo=None):
    """Release tag the cached binary was downloaded from, if from github_repo.

    The digest is re-checked, so a binary swapped in by hand is not read as the
    release it displaced. Forks publish the same tags, so a cache from another
    repository does not answer for this one either.

    Args:
        github_repo (str, optional): GitHub repository (owner/repo) asked about

    Returns:
        str or None: Release tag, or None when the cache cannot be vouched for
    """
    recorded = read_metadata()
    version, digest = recorded.get('version'), recorded.get('sha256')
    if not version or not digest:
        return None
    # A record without a repository predates one being kept, so it says nothing.
    if recorded.get('repo') != (github_repo or default_github_repo()):
        return None
    try:
        if not verify_checksum(get_binary_path(), digest):
            return None
    except OSError:
        return None
    return version


def stale_cache_reason(version, github_repo=None):
    """Why the cached binary is not the release that was asked for.

    A cache left by an earlier release otherwise serves a client asking for a
    newer one, which then fails on whatever that build cannot do.

    Args:
        version (str or None): Release tag asked for, 'latest', or None when
            nothing was asked for, which any cached binary answers
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str or None: Why the cache does not answer for version, or None when it does
    """
    if version is None:
        return None
    repo = github_repo or default_github_repo()
    if version == 'latest':
        try:
            version = resolve_latest_version(repo)
        except ConnectionError:
            # Unreachable releases are no reason to discard a working cache.
            return None

    have = cached_release(repo)
    if have == version:
        return None
    if have is None:
        recorded_repo = read_metadata().get('repo')
        if recorded_repo and recorded_repo != repo:
            return (
                f"the binary cached at {get_binary_path()} was downloaded from "
                f"{recorded_repo}, but {version} of {repo} was asked for"
            )
        return (
            f"the binary cached at {get_binary_path()} was not downloaded by this "
            f"client, so which release it is cannot be told, and {version} was asked for"
        )
    return f"the binary cached at {get_binary_path()} is {have}, but {version} was asked for"


def download_binary(version='latest', github_repo=None):
    """Download sysml-grpc binary from GitHub releases with checksum verification.
    
    Args:
        version (str): Release version tag (e.g., 'v0.1.0'), or 'latest'
        github_repo (str, optional): GitHub repository (owner/repo)
    
    Returns:
        str: Path to downloaded binary
    
    Raises:
        ChecksumMismatchError: If the download does not match the digest pinned
            for it, or the release serves a digest contradicting the pin
        UnpinnedReleaseError: If nothing is pinned for the release
        ConnectionError: If the download or its installation fails
    """
    github_repo = github_repo or default_github_repo()
    if version == 'latest':
        version = resolve_latest_version(github_repo)
    
    binary_name = release_asset_name()

    # Construct URLs
    base_url = f'https://github.com/{github_repo}/releases/download/{version}'
    binary_url = f'{base_url}/{binary_name}'
    checksum_url = f'{base_url}/{binary_name}.sha256'
    
    binary_path = get_binary_path()
    os.makedirs(os.path.dirname(binary_path), exist_ok=True)
    
    try:
        # Download checksum file first
        with urllib.request.urlopen(checksum_url, timeout=NETWORK_TIMEOUT) as response:
            checksum_content = response.read().decode('utf-8')
        
        # Parse checksum (format: "hexdigest  filename\n")
        served_checksum = checksum_content.split()[0]
        # The pinned digest wins, so a release republished with another binary
        # cannot vouch for itself through the sidecar it serves.
        expected_checksum = expected_digest(
            version, binary_name, served_checksum, github_repo
        )
        
        # Download binary
        with urllib.request.urlopen(binary_url, timeout=NETWORK_TIMEOUT) as response:
            binary_data = response.read()
        
        # Write to temporary file first
        temp_path = binary_path + '.tmp'
        with open(temp_path, 'wb') as f:
            f.write(binary_data)
        
        # Verify checksum
        if not verify_checksum(temp_path, expected_checksum):
            os.remove(temp_path)
            raise ChecksumMismatchError(
                f"Checksum mismatch for {binary_name}. "
                f"Expected {expected_checksum}, but download does not match. "
                f"Binary may be corrupted or tampered with."
            )
        
        # Checksum valid - move to final location. os.replace overwrites a cache
        # being replaced, which os.rename refuses to do on Windows.
        try:
            os.replace(temp_path, binary_path)
        except OSError as e:
            os.remove(temp_path)
            raise ConnectionError(
                f"Downloaded {version} but could not install it at {binary_path}: {e}. "
                f"A running service holding that file is the usual cause."
            )
        
        # Make executable
        os.chmod(binary_path, 0o755)
        
        # Record the release, so a later run can tell what the cache holds.
        write_metadata(version, expected_checksum, github_repo)
        
        return binary_path
        
    except ConnectionError:
        raise
    except (OSError, http.client.HTTPException) as e:
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


def ensure_binary(force_download=False, version=None, github_repo=None):
    """Ensure sysml-grpc binary is available, downloading if necessary.
    
    A cached binary is reused only when it is the release asked for; when no
    version is asked for, whatever is cached stands, locally built included. A
    replacement that cannot be downloaded leaves the working cache in place.
    
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
    cached = None
    if not force_download and os.path.exists(binary_path):
        if os.access(binary_path, os.X_OK):
            stale = stale_cache_reason(version, github_repo)
            if stale is None:
                return binary_path
            cached = binary_path
            warnings.warn(
                f"Replacing the cached sysml-grpc: {stale}. Downloading {version}.",
                stacklevel=2,
            )
    
    # If no binary and no version specified, cannot auto-download
    if version is None:
        raise ConnectionError(
            f"Binary not found at {binary_path} and auto-download disabled. "
            f"Build via `make build`, set $PYSYSML_GRPC_VERSION, or specify "
            f"version= parameter for download."
        )
    
    # Download binary with explicit version
    try:
        return download_binary(version=version, github_repo=github_repo)
    except UnpinnedReleaseError as e:
        # A release this pysysml pins nothing for contradicts nothing, so a
        # working cache stands.
        if cached is None:
            raise
        warnings.warn(
            f"Keeping the cached sysml-grpc at {cached}: {version} was not downloaded "
            f"({e}). It may be an older release than asked for.",
            stacklevel=2,
        )
        return cached
    except ChecksumMismatchError:
        # A download that may have been tampered with is never answered from the
        # cache, so it must not reach the ConnectionError fallback below.
        raise
    except ConnectionError as e:
        if cached is None:
            raise
        # A release with no binary to fetch is no reason to lose a working one.
        warnings.warn(
            f"Keeping the cached sysml-grpc at {cached}: {version} could not be "
            f"downloaded ({e}). It may be an older release than asked for.",
            stacklevel=2,
        )
        return cached
