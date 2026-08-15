"""Tests for binary management module."""

import hashlib
import json
import os
import platform
import pytest
from unittest.mock import patch, Mock, mock_open
from pysysml.binary import (
    cached_release,
    default_github_repo,
    detect_platform,
    get_binary_path,
    download_binary,
    metadata_path,
    resolve_latest_version,
    stale_cache_reason,
    verify_checksum,
    write_metadata,
    ensure_binary
)


@pytest.fixture
def cache(tmp_path, monkeypatch):
    """A cache directory this test owns, with helpers to fill it.

    Yields:
        A callable placing binary content, optionally recorded as a release
    """
    binary_path = str(tmp_path / 'sysml-grpc')
    monkeypatch.setattr('pysysml.binary.get_binary_path', lambda: binary_path)
    monkeypatch.delenv('PYSYSML_GRPC_VERSION', raising=False)

    def place(content=b'cached binary', version=None):
        with open(binary_path, 'wb') as f:
            f.write(content)
        os.chmod(binary_path, 0o755)
        if version is not None:
            write_metadata(version, hashlib.sha256(content).hexdigest())
        return binary_path

    return place


def test_detect_platform():
    """Test platform detection returns valid tuple."""
    os_name, arch = detect_platform()
    assert os_name in ('linux', 'darwin', 'windows')
    assert arch in ('amd64', 'arm64')


def test_get_binary_path():
    """Test binary path construction."""
    path = get_binary_path()
    assert path.startswith(os.path.expanduser('~/.pysysml/bin/'))
    assert path.endswith('sysml-grpc') or path.endswith('sysml-grpc.exe')


def test_download_binary():
    """Test binary download from GitHub releases."""
    # Mock binary download
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    
    # Mock checksum file download
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        # First call: checksum file
        # Second call: binary
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
        ]
        
        with patch('builtins.open', mock_open(read_data=mock_binary_data)):
            with patch('os.makedirs'):
                with patch('os.chmod'):
                    with patch('os.replace'):
                        result = download_binary(version='v0.1.0')
                        
                        expected_path = get_binary_path()
                        assert result == expected_path
                        assert mock_urlopen.call_count == 2
                        # Verify URL format
                        call_args = mock_urlopen.call_args_list[1][0][0]
                        assert 'github.com/Open-MBEE/Systemica/releases/download/v0.1.0' in call_args


def test_verify_checksum():
    """Test SHA-256 checksum verification."""
    data = b"test binary content"
    expected = hashlib.sha256(data).hexdigest()
    
    with patch('builtins.open', mock_open(read_data=data)):
        assert verify_checksum('/fake/path', expected) == True
        assert verify_checksum('/fake/path', 'wrong_hash') == False


def test_ensure_binary_exists():
    """Test ensure_binary returns path when binary exists."""
    with patch('os.path.exists', return_value=True):
        with patch('os.access', return_value=True):
            path = ensure_binary()
            expected_path = get_binary_path()
            assert path == expected_path


def test_ensure_binary_downloads():
    """Test ensure_binary downloads if binary missing and version provided."""
    with patch('os.path.exists', return_value=False):
        with patch('pysysml.binary.download_binary') as mock_download:
            mock_download.return_value = '/fake/path/sysml-grpc'
            path = ensure_binary(version='v0.1.0')
            assert path == '/fake/path/sysml-grpc'
            mock_download.assert_called_once_with(version='v0.1.0')


def test_ensure_binary_raises_without_version():
    """Test ensure_binary raises ConnectionError when binary missing and no version."""
    from pysysml.errors import ConnectionError
    with patch('os.path.exists', return_value=False):
        with pytest.raises(ConnectionError, match="Binary not found.*auto-download disabled"):
            ensure_binary()


def test_download_binary_verifies_checksum():
    """Test that download_binary fetches and verifies checksum."""
    import pytest
    version = 'v0.1.0'
    github_repo = 'Open-MBEE/Systemica'
    
    # Mock binary download
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    
    # Mock checksum file download
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        # First call: checksum file
        # Second call: binary
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
        ]
        
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with patch('builtins.open', mock_open(read_data=mock_binary_data)):
                with patch('os.makedirs'):
                    with patch('os.chmod'):
                        with patch('os.replace'):
                            result = download_binary(version, github_repo)
                            
                            # Should have called urlopen twice (checksum + binary)
                            assert mock_urlopen.call_count == 2
                            
                            # Verify checksum URL was constructed correctly
                            checksum_url = str(mock_urlopen.call_args_list[0][0][0])
                            assert '.sha256' in checksum_url


def test_download_binary_fails_on_checksum_mismatch():
    """Test that download fails if checksum doesn't match."""
    import pytest
    version = 'v0.1.0'
    github_repo = 'Open-MBEE/Systemica'
    
    # Mock binary download
    mock_binary_data = b'fake binary content'
    
    # Wrong checksum
    wrong_checksum = 'deadbeef' * 8
    mock_checksum_data = f"{wrong_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
        ]
        
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with patch('builtins.open', mock_open(read_data=mock_binary_data)):
                with patch('os.makedirs'):
                    with patch('os.chmod'):
                        with patch('os.remove'):
                            from pysysml.errors import ConnectionError
                            with pytest.raises(ConnectionError, match="Checksum mismatch"):
                                download_binary(version, github_repo)


def test_default_github_repo_env_override(monkeypatch):
    """Test $PYSYSML_GITHUB_REPO overrides the default repository."""
    monkeypatch.delenv('PYSYSML_GITHUB_REPO', raising=False)
    assert default_github_repo() == 'Open-MBEE/Systemica'

    monkeypatch.setenv('PYSYSML_GITHUB_REPO', 'JPL-Devin/Systemica')
    assert default_github_repo() == 'JPL-Devin/Systemica'


def test_resolve_latest_version():
    """Test latest release tag is read from the GitHub API."""
    payload = b'{"tag_name": "v0.0.4"}'
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.return_value = Mock(
            __enter__=Mock(return_value=Mock(read=Mock(return_value=payload))),
            __exit__=Mock(return_value=False))
        assert resolve_latest_version('Open-MBEE/Systemica') == 'v0.0.4'
        url = str(mock_urlopen.call_args_list[0][0][0])
        assert url == 'https://api.github.com/repos/Open-MBEE/Systemica/releases/latest'


def test_resolve_latest_version_without_tag():
    """Test a release carrying no tag is reported rather than returned."""
    from pysysml.errors import ConnectionError
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.return_value = Mock(
            __enter__=Mock(return_value=Mock(read=Mock(return_value=b'{}'))),
            __exit__=Mock(return_value=False))
        with pytest.raises(ConnectionError, match="no tag name"):
            resolve_latest_version('Open-MBEE/Systemica')


def test_download_binary_latest_resolves_tag():
    """Test version='latest' downloads from the resolved tag."""
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()

    with patch('pysysml.binary.resolve_latest_version', return_value='v0.0.4') as mock_resolve:
        with patch('urllib.request.urlopen') as mock_urlopen:
            mock_urlopen.side_effect = [
                Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
                Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
            ]
            with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
                with patch('builtins.open', mock_open(read_data=mock_binary_data)):
                    with patch('os.makedirs'), patch('os.chmod'), patch('os.replace'):
                        download_binary(version='latest')

            mock_resolve.assert_called_once_with('Open-MBEE/Systemica')
            for call in mock_urlopen.call_args_list:
                assert 'releases/download/v0.0.4/sysml-grpc-linux-amd64' in str(call[0][0])


def test_ensure_binary_downloads_version_from_env(monkeypatch):
    """Test $PYSYSML_GRPC_VERSION enables auto-download when no binary is present."""
    monkeypatch.setenv('PYSYSML_GRPC_VERSION', 'v0.0.4')
    with patch('os.path.exists', return_value=False):
        with patch('pysysml.binary.download_binary') as mock_download:
            mock_download.return_value = '/fake/path/sysml-grpc'
            assert ensure_binary() == '/fake/path/sysml-grpc'
            mock_download.assert_called_once_with(version='v0.0.4')


def test_download_binary_records_the_release(cache):
    """Test a download records which release the cache now holds."""
    data = b'fake binary content'
    checksum = hashlib.sha256(data).hexdigest()
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=data))),
             __exit__=Mock(return_value=False)),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            download_binary(version='v0.0.7')

    with open(metadata_path()) as f:
        assert json.load(f) == {'version': 'v0.0.7', 'sha256': checksum}
    assert cached_release() == 'v0.0.7'


def test_download_binary_overwrites_the_cache_it_replaces(cache):
    """Test a download installs over an existing cache, as replacing one must."""
    cache(b'the release before', version='v0.0.5')
    data = b'the release asked for'
    checksum = hashlib.sha256(data).hexdigest()
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=data))),
             __exit__=Mock(return_value=False)),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            path = download_binary(version='v0.0.7')

    with open(path, 'rb') as f:
        assert f.read() == data
    assert cached_release() == 'v0.0.7'
    assert not os.path.exists(path + '.tmp')


def test_download_binary_reports_a_cache_it_cannot_install_over(cache):
    """Test a binary held open by a running service is reported, not raised raw."""
    from pysysml.errors import ConnectionError
    binary_path = cache(version='v0.0.5')
    data = b'the release asked for'
    checksum = hashlib.sha256(data).hexdigest()
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=data))),
             __exit__=Mock(return_value=False)),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('pysysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with patch('os.replace', side_effect=PermissionError('in use')):
                with pytest.raises(ConnectionError, match='could not install it'):
                    download_binary(version='v0.0.7')

    assert not os.path.exists(binary_path + '.tmp')
    assert cached_release() == 'v0.0.5'


def test_cached_release_unknown_when_binary_was_replaced(cache):
    """Test a binary swapped under a recorded name is not read as that release."""
    binary_path = cache(b'downloaded', version='v0.0.7')
    with open(binary_path, 'wb') as f:
        f.write(b'something else entirely')
    assert cached_release() is None


def test_stale_cache_reason_accepts_the_release_asked_for(cache):
    """Test the cache is reused when it is the release asked for."""
    cache(version='v0.0.7')
    assert stale_cache_reason('v0.0.7') is None
    assert stale_cache_reason(None) is None


def test_stale_cache_reason_names_another_release(cache):
    """Test a cache from another release is reported, naming both tags."""
    cache(version='v0.0.5')
    reason = stale_cache_reason('v0.0.7')
    assert 'v0.0.5' in reason and 'v0.0.7' in reason


def test_stale_cache_reason_for_an_unidentifiable_binary(cache):
    """Test a cache this client did not download cannot answer for a release."""
    cache()  # No record beside it: a hand-placed or pre-existing binary.
    assert 'cannot be told' in stale_cache_reason('v0.0.7')


def test_stale_cache_reason_keeps_the_cache_when_offline(cache):
    """Test version='latest' keeps a working cache when releases are unreachable."""
    from pysysml.errors import ConnectionError
    cache(version='v0.0.7')
    with patch('pysysml.binary.resolve_latest_version',
               side_effect=ConnectionError('no network')):
        assert stale_cache_reason('latest') is None


def test_ensure_binary_reuses_the_release_asked_for(cache):
    """Test no download happens when the cache is already that release."""
    binary_path = cache(version='v0.0.7')
    with patch('pysysml.binary.download_binary') as mock_download:
        assert ensure_binary(version='v0.0.7') == binary_path
        mock_download.assert_not_called()


def test_ensure_binary_replaces_a_cache_from_another_release(cache):
    """Test a stale cache is replaced rather than served, with a warning saying so."""
    cache(version='v0.0.5')
    with patch('pysysml.binary.download_binary') as mock_download:
        mock_download.return_value = '/downloaded/sysml-grpc'
        with pytest.warns(UserWarning, match='v0.0.5'):
            assert ensure_binary(version='v0.0.7') == '/downloaded/sysml-grpc'
        mock_download.assert_called_once_with(version='v0.0.7')


def test_ensure_binary_keeps_a_cache_when_no_version_is_asked_for(cache):
    """Test a locally built binary is left alone when nothing names a release."""
    binary_path = cache()
    with patch('pysysml.binary.download_binary') as mock_download:
        assert ensure_binary() == binary_path
        mock_download.assert_not_called()
