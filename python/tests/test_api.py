"""Tests for pysysml module-level convenience API."""

import pytest
from unittest.mock import patch, MagicMock, Mock
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
            
            # Should delegate to Connection.load(), not asking for strict loading
            mock_conn.load.assert_called_once_with("test.sysml", strict=False)
            
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
            
            mock_conn.load.assert_called_once_with("test.sysml", strict=False)
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
            mock_conn1.load.assert_called_once_with("test1.sysml", strict=False)
            mock_conn2.load.assert_called_once_with("test2.sysml", strict=False)
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
            MockConnection.assert_called_once_with(
                'localhost', 50051, auto_start=True, version=None,
                require_capabilities=None,
            )
            
            assert result == mock_conn
    
    def test_pysysml_connect_with_custom_params(self):
        """Test pysysml.connect() with custom host, port, auto_start."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            MockConnection.return_value = mock_conn
            
            # Call connect with custom params
            result = pysysml.connect(host="example.com", port=9999, auto_start=False)
            
            # Should create connection with custom params
            MockConnection.assert_called_once_with(
                'example.com', 9999, auto_start=False, version=None,
                require_capabilities=None,
            )
            
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
    
    def test_pysysml_eval_with_file(self):
        """Test module-level eval() loads file."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            mock_model = Mock()
            mock_model.hash = "model-abc"
            mock_conn.load.return_value = mock_model
            mock_conn.eval.return_value = 42
            
            result = pysysml.eval("6 * 7", file_path="test.sysml")
            
            assert result == 42
            mock_conn.load.assert_called_once_with("test.sysml")
            mock_conn.eval.assert_called_once_with("6 * 7", "model-abc", None)
    
    def test_pysysml_instantiate_with_hash(self):
        """Test module-level instantiate() with model_hash."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            mock_instance = Mock()
            mock_instance.id = 999
            mock_conn.instantiate.return_value = mock_instance
            
            result = pysysml.instantiate("Part", model_hash="hash-xyz")
            
            assert result.id == 999
            mock_conn.instantiate.assert_called_once_with("Part", "hash-xyz")
    
    def test_pysysml_eval_missing_both_params(self):
        """Test eval() raises ValueError if neither file_path nor model_hash."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            with pytest.raises(ValueError, match="Must provide either"):
                pysysml.eval("2 + 2")
    
    def test_pysysml_eval_with_both_params(self):
        """Test eval() raises ValueError when both file_path and model_hash provided."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            with pytest.raises(ValueError, match="Provide either file_path or model_hash, not both"):
                pysysml.eval("2 + 2", file_path="test.sysml", model_hash="hash-abc")
    
    def test_pysysml_eval_with_context(self):
        """Test eval() passes context_symbol_id through to conn.eval()."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            mock_model = Mock()
            mock_model.hash = "model-xyz"
            mock_conn.load.return_value = mock_model
            mock_conn.eval.return_value = 100
            
            result = pysysml.eval("x + y", file_path="test.sysml", context_symbol_id="ctx-123")
            
            assert result == 100
            mock_conn.load.assert_called_once_with("test.sysml")
            # Verify context_symbol_id passes through
            mock_conn.eval.assert_called_once_with("x + y", "model-xyz", "ctx-123")
    
    def test_pysysml_instantiate_missing_both_params(self):
        """Test instantiate() raises ValueError if neither file_path nor model_hash."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            with pytest.raises(ValueError, match="Must provide either"):
                pysysml.instantiate("Part")
    
    def test_pysysml_instantiate_with_both_params(self):
        """Test instantiate() raises ValueError when both file_path and model_hash provided."""
        with patch('pysysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            # Clear cached connection
            pysysml._default_connection = None
            pysysml._default_connection_params = None
            
            with pytest.raises(ValueError, match="Provide either file_path or model_hash, not both"):
                pysysml.instantiate("Part", file_path="test.sysml", model_hash="hash-abc")
