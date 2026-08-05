"""Tests for runtime methods (eval, instantiate, etc.)."""
import pytest
from unittest.mock import Mock, patch
from pysysml.connection import Connection
from pysysml.proto import sysml_pb2
from pysysml.errors import RuntimeError


def test_eval_simple_expression():
    """Test eval() with mocked RPC."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub') as mock_stub_cls:
            mock_stub = Mock()
            mock_stub_cls.return_value = mock_stub
            
            # Mock Evaluate response
            mock_response = sysml_pb2.EvaluateResponse(
                result=sysml_pb2.Value(int_value=4),
                error=""
            )
            mock_stub.Evaluate.return_value = mock_response
            
            conn = Connection(auto_start=False)
            result = conn.eval("2 + 2", "model-hash")
            
            assert result == 4
            mock_stub.Evaluate.assert_called_once()


def test_instantiate_returns_instance():
    """Test instantiate() with mocked RPC."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub') as mock_stub_cls:
            mock_stub = Mock()
            mock_stub_cls.return_value = mock_stub
            
            # Mock Instantiate response
            mock_response = sysml_pb2.InstantiateResponse(
                instance=sysml_pb2.Instance(
                    id=123,
                    type_symbol_id="Test::Part",
                    slots={}
                ),
                error=""
            )
            mock_stub.Instantiate.return_value = mock_response
            
            conn = Connection(auto_start=False)
            instance = conn.instantiate("Test::Part", "model-hash")
            
            assert instance.id == 123
            assert instance.type_symbol_id == "Test::Part"


def test_eval_raises_on_error():
    """Test eval() raises RuntimeError on failure."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub') as mock_stub_cls:
            mock_stub = Mock()
            mock_stub_cls.return_value = mock_stub
            
            # Mock error response
            mock_response = sysml_pb2.EvaluateResponse(
                error="Parse error: unexpected token"
            )
            mock_stub.Evaluate.return_value = mock_response
            
            conn = Connection(auto_start=False)
            
            with pytest.raises(RuntimeError, match="Parse error"):
                conn.eval("invalid(((", "model-hash")


def test_execute_action_with_inputs():
    """Test execute_action() returns outputs correctly."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub') as mock_stub_cls:
            mock_stub = Mock()
            mock_stub_cls.return_value = mock_stub
            
            # Mock ExecuteAction response with outputs
            mock_response = sysml_pb2.ExecuteActionResponse(
                outputs={
                    'result': sysml_pb2.Value(int_value=42),
                    'status': sysml_pb2.Value(string_value='ok')
                },
                error=""
            )
            mock_stub.ExecuteAction.return_value = mock_response
            
            conn = Connection(auto_start=False)
            outputs = conn.execute_action("Test::MyAction", "model-hash", inputs={'x': 10})
            
            assert outputs['result'] == 42
            assert outputs['status'] == 'ok'
            mock_stub.ExecuteAction.assert_called_once()


def test_execute_state_visits_states():
    """Test execute_state() returns trace of states_visited."""
    with patch('grpc.insecure_channel'):
        with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub') as mock_stub_cls:
            mock_stub = Mock()
            mock_stub_cls.return_value = mock_stub
            
            # Mock ExecuteState response with states_visited
            mock_response = sysml_pb2.ExecuteStateResponse(
                states_visited=['Initial', 'Active', 'Done'],
                final_context={},
                error=""
            )
            mock_stub.ExecuteState.return_value = mock_response
            
            conn = Connection(auto_start=False)
            result = conn.execute_state("Test::StateMachine", "model-hash")
            
            # execute_state returns dict with states_visited and final_context
            assert result['states_visited'] == ['Initial', 'Active', 'Done']
            assert result['final_context'] == {}
            mock_stub.ExecuteState.assert_called_once()
