"""Integration tests for running analysis cases against a real service.

These are about what the service actually computes and decides for a case —
its outputs, its objective's verdict, how the subject and inputs bind — rather
than about how the client wraps a canned response.
"""

import pytest

from opensysml import Connection
from opensysml.errors import ExecutionError, WrongKindError
from opensysml.verdict import AnalysisResult

MODEL_SOURCE = '''
package An {
    private import ScalarValues::*;

    part def Ship {
        attribute cost : Real default = 5.0;
        attribute other : Real default = 7.0;
    }

    calc def Sum {
        in a : Real;
        in b : Real;
        return : Real = a + b;
    }

    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = Sum(s.cost, s.other);
        objective affordable {
            require constraint { total <= limit }
        }
    }

    part ship : Ship;
    part barge : Ship {
        attribute :>> cost = 30.0;
    }

    analysis shipCost : CostAnalysis {
        subject s = ship;
    }

    analysis plain {
        out x : Real = 1.0 + 2.0;
    }
}
'''


@pytest.mark.integration
class TestAnalysisIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_a_usage_binding_its_subject_computes_its_outputs(self):
        result = self.model.run_analysis("An::shipCost")
        assert isinstance(result, AnalysisResult)
        assert result.outputs == {"total": 12.0}
        assert [v.kind for v in result.verdicts] == ["objective"]
        assert result.verdicts[0].holds
        assert result.verdicts[0].element_id == "An::CostAnalysis::affordable"
        assert result.satisfied

    def test_a_case_without_objective_has_no_verdict(self):
        result = self.model.run_analysis("An::plain")
        assert result.outputs == {"x": 3.0}
        assert result.verdicts == []
        assert result.satisfied

    def test_a_definition_runs_on_the_subject_named(self):
        result = self.model.run_analysis("An::CostAnalysis", subject="An::barge")
        assert result.outputs == {"total": 37.0}
        verdict = result.verdicts[0]
        assert not verdict.holds
        assert verdict.evaluated
        assert verdict.condition == "total <= limit"
        assert verdict.instance_id
        assert [inst.id for inst in result.instances][0] == verdict.instance_id
        assert not result

    def test_arguments_bind_the_inputs(self):
        by_name = self.model.run_analysis(
            "An::CostAnalysis", subject="An::barge", named_arguments={"limit": 50.0}
        )
        assert by_name.satisfied
        positional = self.model.run_analysis(
            "An::CostAnalysis", subject="An::barge", arguments=[10.0]
        )
        assert not positional.satisfied
        assert positional.outputs == {"total": 37.0}

    def test_an_unbound_subject_raises(self):
        with pytest.raises(ExecutionError) as exc_info:
            self.model.run_analysis("An::CostAnalysis")
        assert "subject" in str(exc_info.value)
        assert not isinstance(exc_info.value, WrongKindError)

    def test_a_wrong_kind_raises(self):
        with pytest.raises(WrongKindError):
            self.model.run_analysis("An::ship")

    def test_an_unknown_symbol_raises(self):
        with pytest.raises(ExecutionError):
            self.model.run_analysis("An::Nope")
