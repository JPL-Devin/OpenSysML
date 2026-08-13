"""Tests for binary management module."""

import hashlib
import os
import platform
import pytest
from unittest.mock import patch, Mock, mock_open
from pysysml.binary import (
    default_github_repo,
    detect_platform,
    get_binary_path,
    download_binary,
    resolve_latest_version,
    verify_checksum,
    ensure_binary
)


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
                    with patch('os.rename'):
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
                        with patch('os.rename'):
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
                    with patch('os.makedirs'), patch('os.chmod'), patch('os.rename'):
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
