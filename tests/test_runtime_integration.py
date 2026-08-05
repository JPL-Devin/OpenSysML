"""Integration tests for runtime operations against real service."""
import pytest
from pysysml import Connection
from pysysml.errors import RuntimeError

@pytest.mark.integration
class TestRuntimeIntegration:
    """Integration tests requiring live sysml-grpc service."""
    
    def setup_method(self):
        """Check if service is running."""
        import grpc
        try:
            self.conn = Connection(auto_start=False)
            # Probe health
            from pysysml.proto import sysml_pb2
            req = sysml_pb2.DiagnosticsRequest(model_hash="")
            self.conn._stub.GetDiagnostics(req)
        except grpc.RpcError as e:
            # NOT_FOUND is expected for invalid hash - means server is working
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return  # Service is healthy, self.conn already set
            self.conn = None  # Failed, clear for teardown safety
            pytest.skip("sysml-grpc service not running")
        except Exception:
            self.conn = None  # Failed, clear for teardown safety
            pytest.skip("sysml-grpc service not running")
    
    def teardown_method(self):
        """Clean up connection after each test."""
        if hasattr(self, 'conn'):
            self.conn.close()
    
    def test_eval_arithmetic(self):
        """Test evaluating simple arithmetic expression."""
        # Parse a model with a calc
        src = 'package Test { calc result { 2 + 2 } }'
        model = self.conn.load_from_content(src)
        
        # Evaluate expression
        result = self.conn.eval("2 + 2", model.hash)
        assert result == 4
    
    def test_eval_boolean(self):
        """Test evaluating boolean expression."""
        src = 'package Test { }'
        model = self.conn.load_from_content(src)
        
        result = self.conn.eval("true and false", model.hash)
        assert result is False
    
    def test_instantiate_simple_part(self):
        """Test instantiating a part definition."""
        src = '''
        package Test {
            part def SimplePart {
                attribute mass : Integer = 100;
            }
        }
        '''
        model = self.conn.load_from_content(src)
        
        # Instantiate
        instance = self.conn.instantiate("Test::SimplePart", model.hash)
        
        assert instance is not None
        assert instance.type_symbol_id == "Test::SimplePart"
        assert instance.id > 0
    
    def test_eval_invalid_expression_raises(self):
        """Test that invalid expression raises RuntimeError."""
        src = 'package Test { }'
        model = self.conn.load_from_content(src)
        
        with pytest.raises(RuntimeError):
            self.conn.eval("invalid syntax (((", model.hash)
    
    def test_instantiate_nonexistent_symbol_raises(self):
        """Test that instantiating missing symbol raises RuntimeError."""
        src = 'package Test { }'
        model = self.conn.load_from_content(src)
        
        with pytest.raises(RuntimeError, match="not found"):
            self.conn.instantiate("Test::DoesNotExist", model.hash)
    
    def test_load_from_content_with_syntax_error(self):
        """Test that load_from_content returns diagnostics for invalid syntax."""
        src = 'package Test { invalid syntax ((( }'
        model = self.conn.load_from_content(src)
        assert len(model.diagnostics) > 0

    def test_execute_action_binds_inputs(self):
        """Supplied inputs must flow into the action and affect its outputs.

        The action seeds attribute `result` (default 0) then adds 5. Passing
        result=10 must yield 15, proving inputs are not discarded server-side.
        """
        src = '''
        package Test {
            action addFive {
                attribute result : Integer = 0;
                first start;
                action inner {
                    assign result := result + 5;
                }
                done end;
                then start inner;
                then inner end;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        # Baseline: default 0 + 5 = 5.
        base = self.conn.execute_action("Test::addFive", model.hash)
        assert base.get("result") == 5

        # With input result=10 -> 10 + 5 = 15.
        out = self.conn.execute_action(
            "Test::addFive", model.hash, inputs={"result": 10}
        )
        assert out.get("result") == 15

    def test_execute_state_returns_real_trace(self):
        """states_visited must reflect the actual states, not a placeholder."""
        src = '''
        package Test {
            state Machine {
                initial init;
                state Running;
                final done;

                init then Running;
                Running then done;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        result = self.conn.execute_state("Test::Machine", model.hash)
        assert result["states_visited"] == ["init", "Running", "done"]
