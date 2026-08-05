"""Tests for Connection class."""

import grpc
from unittest.mock import Mock, patch
from pysysml.connection import Connection
from pysysml.proto import sysml_pb2


def test_connection_init():
    with patch('grpc.insecure_channel') as mock_channel:
        mock_stub = Mock()
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(port=50051)
            
            assert conn.port == 50051
            mock_channel.assert_called_once_with('localhost:50051')


def test_connection_custom_host():
    with patch('grpc.insecure_channel') as mock_channel:
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(host='example.com', port=9000)
            
            mock_channel.assert_called_once_with('example.com:9000')


def test_connection_load():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        # Mock ParseFile RPC response
        pb_root = sysml_pb2.SymbolInfo(
            id="TestModel",
            name="TestModel",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="abc123",
            root=pb_root,
            diagnostics=[],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            model = conn.load("test.sysml")
            
            # Verify ParseFile was called with file path
            call_args = mock_stub.ParseFile.call_args
            request = call_args[0][0]
            assert request.file_path == "test.sysml"
            
            # Verify Model object returned
            assert model.hash == "abc123"
            assert model.root.name == "TestModel"


def test_connection_load_with_diagnostics():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_root = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_diag = sysml_pb2.Diagnostic(
            severity="error",
            message="Parse error",
            span=sysml_pb2.Span(file="bad.sysml", start_line=1, start_col=1, end_line=1, end_col=1),
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash",
            root=pb_root,
            diagnostics=[pb_diag],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            model = conn.load("bad.sysml")
            
            assert len(model.diagnostics) == 1
            assert model.diagnostics[0].message == "Parse error"


def test_connection_load_grpc_error():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        # Simulate gRPC error (e.g., file not found)
        mock_stub.ParseFile.side_effect = grpc.RpcError()
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            
            try:
                conn.load("missing.sysml")
                assert False, "Expected exception"
            except grpc.RpcError:
                pass  # Expected


def test_connection_get_symbol():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_sym = sysml_pb2.SymbolInfo(
            id="Vehicle::Engine",
            name="Engine",
            kind="PartUsage",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=pb_sym,
            error="",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            pb_result = conn.get_symbol("model_hash", "Vehicle::Engine")
            
            # Verify GetSymbol was called correctly
            call_args = mock_stub.GetSymbol.call_args
            request = call_args[0][0]
            assert request.model_hash == "model_hash"
            assert request.symbol_id == "Vehicle::Engine"
            
            # Verify SymbolInfo returned
            assert pb_result.id == "Vehicle::Engine"
            assert pb_result.name == "Engine"


def test_connection_get_symbol_not_found():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=None,
            error="Symbol not found: NonExistent",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection()
            pb_result = conn.get_symbol("hash", "NonExistent")
            
            # Should return None when error is set
            assert pb_result is None
