"""Integration tests for the model surface against a real service.

Covers evaluation on the model itself and the wrong-kind requests the service
now classifies, since both are about what the service actually answers rather
than about how the client wraps a canned response.
"""

import pytest

from pysysml import Connection
from pysysml.errors import ExecutionError, ModelNotFoundError, WrongKindError
from pysysml.model import Model
from pysysml.proto import sysml_pb2

MODEL_SOURCE = '''
package Demo {
    part def Vehicle {
        attribute mass = 1500.0;
        constraint massPositive {
            assert mass > 0.0;
        }
        constraint massLight {
            assert mass < 100.0;
        }
        requirement lightEnough {
            require constraint { mass < 2000.0 }
        }
    }

    part sedan : Vehicle {
        attribute :>> mass = 1200.0;
    }

    calc add {
        in x;
        in y;
        return x + y;
    }
}
'''


@pytest.mark.integration
class TestModelSurfaceIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_eval_on_the_model(self):
        assert self.model.eval("1+1") == 2

    def test_eval_in_a_context(self):
        assert self.model.eval("mass", context_symbol_id="Demo::sedan") == 1200.0

    def test_eval_against_a_subject_reads_that_object(self):
        # The object's redefinition wins over the definition's default, the way
        # %eval does after %instantiate.
        assert self.model.eval("mass", context_symbol_id="Demo::Vehicle") == 1500.0
        assert self.model.eval("mass", subject="Demo::sedan") == 1200.0
        assert self.model.eval("mass * 2", subject="Demo::sedan") == 2400.0

    def test_eval_against_a_subject_in_a_named_context(self):
        assert (
            self.model.eval(
                "mass",
                context_symbol_id="Demo::Vehicle",
                subject="Demo::sedan",
            )
            == 1200.0
        )

    def test_eval_raises_for_an_unknown_subject(self):
        with pytest.raises(ExecutionError):
            self.model.eval("mass", subject="Demo::nope")

    @pytest.mark.parametrize("expression", ["1/0", "nope", '1 + "a"'])
    def test_eval_raises_for_an_expression_it_cannot_evaluate(self, expression):
        with pytest.raises(ExecutionError):
            self.model.eval(expression)

    def test_eval_raises_when_the_service_no_longer_holds_the_model(self):
        # A model whose hash the service's bounded cache has evicted.
        evicted = Model(
            sysml_pb2.ParseFileResponse(
                model_hash="0" * 64,
                root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
            ),
            self.conn,
        )
        with pytest.raises(ModelNotFoundError):
            evicted.eval("1+1")

    def test_a_verdict_is_still_a_verdict(self):
        assert self.model.verify_constraint(
            "Demo::Vehicle::massPositive", subject="Demo::sedan"
        ).holds
        assert self.model.verify_constraint(
            "Demo::Vehicle::massLight", subject="Demo::sedan"
        ).holds is False
        assert self.model.verify_requirement(
            "Demo::Vehicle::lightEnough", subject="Demo::sedan"
        ).holds

    def test_a_wrong_kind_verification_raises(self):
        for call in (
            lambda: self.model.verify_constraint("Demo::Vehicle"),
            lambda: self.model.verify_requirement("Demo::Vehicle"),
            lambda: self.model.calc("Demo::Vehicle", arguments=[1]),
        ):
            with pytest.raises(WrongKindError):
                call()

    def test_an_unknown_symbol_still_raises(self):
        for call in (
            lambda: self.model.verify_constraint("Demo::Nope"),
            lambda: self.model.verify_requirement("Demo::Nope"),
            lambda: self.model.verify_satisfaction("Demo::Nope"),
            lambda: self.model.calc("Demo::Nope", arguments=[1]),
        ):
            with pytest.raises(ExecutionError):
                call()

    def test_an_element_stating_no_assertion_still_answers_with_none(self):
        assert self.model.verify_satisfaction("Demo::Vehicle") == []
