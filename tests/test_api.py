"""Tests for pysysml module-level convenience API."""

import pytest
from unittest.mock import patch, MagicMock
import pysysml


class TestModuleLevelAPI:
    """Test module-level convenience functions."""
    
    def test_pysysml_load(self):
        """Test pysysml.load() uses default connection."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model = MagicMock()
            mock_conn.load.return_value = mock_model
            MockConnection.return_value = mock_conn
            
            # Clear any cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            # Call load
            result = pysysml.load("test.sysml")
            
            # Should create default connection with auto_start=True
            MockConnection.assert_called_once_with('localhost', 50051, auto_start=True)
            
            # Should delegate to Connection.load()
            mock_conn.load.assert_called_once_with("test.sysml")
            
            assert result == mock_model
    
    def test_pysysml_load_with_custom_host_port(self):
        """Test pysysml.load() with custom host and port."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model = MagicMock()
            mock_conn.load.return_value = mock_model
            MockConnection.return_value = mock_conn
            
            # Clear any cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            # Call load with custom params
            result = pysysml.load("test.sysml", host="example.com", port=9999)
            
            # Should create connection with custom params
            MockConnection.assert_called_once_with('example.com', 9999, auto_start=True)
            
            mock_conn.load.assert_called_once_with("test.sysml")
            assert result == mock_model
    
    def test_pysysml_load_reuses_default_connection(self):
        """Test pysysml.load() reuses default connection for same host/port."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model1 = MagicMock()
            mock_model2 = MagicMock()
            mock_conn.load.side_effect = [mock_model1, mock_model2]
            MockConnection.return_value = mock_conn
            
            # Clear any cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            # Call load twice with same host/port
            result1 = pysysml.load("test1.sysml")
            result2 = pysysml.load("test2.sysml")
            
            # Should create connection only once
            MockConnection.assert_called_once()
            
            # Both loads should use same connection
            assert mock_conn.load.call_count == 2
            assert result1 == mock_model1
            assert result2 == mock_model2
    
    def test_pysysml_load_recreates_connection_on_param_change(self):
        """Test pysysml.load() creates new connection when host/port changes."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn1 = MagicMock()
            mock_conn2 = MagicMock()
            mock_model1 = MagicMock()
            mock_model2 = MagicMock()
            mock_conn1.load.return_value = mock_model1
            mock_conn2.load.return_value = mock_model2
            MockConnection.side_effect = [mock_conn1, mock_conn2]
            
            # Clear any cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            # Call load with different host/port
            result1 = pysysml.load("test1.sysml", host="h1", port=1111)
            result2 = pysysml.load("test2.sysml", host="h2", port=2222)
            
            # Should create two connections
            assert MockConnection.call_count == 2
            MockConnection.assert_any_call('h1', 1111, auto_start=True)
            MockConnection.assert_any_call('h2', 2222, auto_start=True)
            
            # Each load uses its own connection
            mock_conn1.load.assert_called_once_with("test1.sysml")
            mock_conn2.load.assert_called_once_with("test2.sysml")
            assert result1 == mock_model1
            assert result2 == mock_model2
    
    def test_pysysml_connect(self):
        """Test pysysml.connect() creates new Connection."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            MockConnection.return_value = mock_conn
            
            # Call connect
            result = pysysml.connect()
            
            # Should create new connection with defaults
            MockConnection.assert_called_once_with('localhost', 50051, auto_start=True)
            
            assert result == mock_conn
    
    def test_pysysml_connect_with_custom_params(self):
        """Test pysysml.connect() with custom host, port, auto_start."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            MockConnection.return_value = mock_conn
            
            # Call connect with custom params
            result = pysysml.connect(host="example.com", port=9999, auto_start=False)
            
            # Should create connection with custom params
            MockConnection.assert_called_once_with('example.com', 9999, auto_start=False)
            
            assert result == mock_conn
    
    def test_pysysml_connect_creates_new_instance(self):
        """Test pysysml.connect() creates new instance each time."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn1 = MagicMock()
            mock_conn2 = MagicMock()
            MockConnection.side_effect = [mock_conn1, mock_conn2]
            
            # Call connect twice
            result1 = pysysml.connect()
            result2 = pysysml.connect()
            
            # Should create two instances
            assert MockConnection.call_count == 2
            
            # Should return different instances
            assert result1 == mock_conn1
            assert result2 == mock_conn2
            assert result1 != result2
