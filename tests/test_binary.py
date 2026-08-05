"""Tests for binary management module."""

import hashlib
import os
import platform
from unittest.mock import patch, Mock, mock_open
from pysysml.binary import (
    detect_platform,
    get_binary_path,
    download_binary,
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
    with patch('urllib.request.urlopen') as mock_urlopen:
        # Mock HTTP response with context manager support
        mock_response = Mock()
        mock_response.read.return_value = b'fake binary content'
        mock_response.__enter__ = Mock(return_value=mock_response)
        mock_response.__exit__ = Mock(return_value=False)
        mock_urlopen.return_value = mock_response
        
        with patch('builtins.open', mock_open()) as mock_file:
            with patch('os.makedirs'):
                with patch('os.chmod'):
                    result = download_binary(version='v0.1.0')
                    
                    expected_path = get_binary_path()
                    assert result == expected_path
                    mock_urlopen.assert_called_once()
                    # Verify URL format
                    call_args = mock_urlopen.call_args[0][0]
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
    """Test ensure_binary downloads if binary missing."""
    with patch('os.path.exists', return_value=False):
        with patch('pysysml.binary.download_binary') as mock_download:
            mock_download.return_value = '/fake/path/sysml-grpc'
            path = ensure_binary()
            assert path == '/fake/path/sysml-grpc'
            mock_download.assert_called_once()
