"""Tests for the verification wrappers: constraints, requirements, satisfy, calc.

These are the answers an engineer scripts ("does this model satisfy its
requirements?"), so what matters here is that a condition evaluating to false
comes back as a verdict while a failure to evaluate comes back as an exception.
"""

import grpc
import pytest
from unittest.mock import Mock, patch

from pysysml.capabilities import CAPABILITY_VERIFICATION, MissingCapabilityError
from pysysml.connection import Connection
from pysysml.errors import ExecutionError, ModelNotFoundError
from pysysml.proto import sysml_pb2
from pysysml.verdict import CalcResult, Verdict


def make_connection(stub):
    """Build a Connection over a mock stub that reports verification support."""
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test",
        capabilities=[CAPABILITY_VERIFICATION],
    )
    with patch('grpc.insecure_channel'):
        with patch(
            'pysysml.proto.sysml_pb2_grpc.SysMLServiceStub',
            return_value=stub,
        ):
            return Connection(auto_start=False)


def rpc_error(code, details=""):
    """A gRPC failure carrying a status, as the service's would."""

    class Failure(grpc.RpcError, grpc.Call):
        def code(self):
            return code

        def details(self):
            return details

        def trailing_metadata(self):
            return ()

    return Failure()


def test_verify_constraint_holds():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::Vehicle::massOK",
            holds=True,
            instance_id=1,
            instance_type_id="Demo::Vehicle",
        ),
        instances=[sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle")],
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint(
        "Demo::Vehicle::massOK", "hash1", subject_symbol_id="Demo::sedan"
    )

    request = stub.VerifyConstraint.call_args[0][0]
    assert request.model_hash == "hash1"
    assert request.symbol_id == "Demo::Vehicle::massOK"
    assert request.subject_symbol_id == "Demo::sedan"

    assert isinstance(verdict, Verdict)
    assert verdict.holds
    assert bool(verdict) is True
    assert verdict.kind == "constraint"
    assert verdict.element == "Demo::Vehicle::massOK"
    assert verdict.instance_id == 1
    assert verdict.evaluated
    assert [inst.id for inst in verdict.instances] == [1]
    # A holding verdict raises nothing, so this can guard a script.
    assert verdict.raise_for_error() is verdict


def test_verify_constraint_false_is_a_verdict_not_an_exception():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::Vehicle::massOK",
            holds=False,
            condition="mass < 100.0",
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint("Demo::Vehicle::massOK", "hash1")

    assert not verdict
    assert verdict.evaluated
    assert verdict.condition == "mass < 100.0"
    assert "mass < 100.0" in verdict.explain()
    verdict.raise_for_error()


def test_verify_constraint_evaluation_failure_raises_on_request():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::Vehicle::massOK",
            holds=False,
            error="feature mass is unbound",
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint("Demo::Vehicle::massOK", "hash1")

    assert not verdict.evaluated
    assert verdict.error == "feature mass is unbound"
    with pytest.raises(ExecutionError) as exc_info:
        verdict.raise_for_error()
    assert "unbound" in str(exc_info.value)


def test_verify_constraint_unanswerable_request_raises():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        error="symbol not found: Demo::Nope",
        diagnostics=[sysml_pb2.Diagnostic(severity="error", message="boom")],
    )
    conn = make_connection(stub)

    with pytest.raises(ExecutionError) as exc_info:
        conn.verify_constraint("Demo::Nope", "hash1")
    assert "Demo::Nope" in str(exc_info.value)
    assert [d.message for d in exc_info.value.diagnostics] == ["boom"]


def test_verify_requirement_holds():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(
            kind="requirement",
            element_id="Demo::Vehicle::lightEnough",
            holds=True,
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_requirement(
        "Demo::Vehicle::lightEnough", "hash1", subject_symbol_id="Demo::sedan"
    )

    request = stub.VerifyRequirement.call_args[0][0]
    assert request.symbol_id == "Demo::Vehicle::lightEnough"
    assert request.subject_symbol_id == "Demo::sedan"
    assert verdict.kind == "requirement"
    assert verdict.holds


def test_verify_satisfaction_reports_one_verdict_per_assertion():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massLimit by sedan",
                holds=True,
                instance_id=1,
            ),
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massTiny by sedan",
                holds=False,
                condition="vehicle.mass <= maxMass",
                instance_id=2,
            ),
        ],
        instances=[
            sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle"),
            sysml_pb2.Instance(id=2, type_symbol_id="Demo::Vehicle"),
        ],
        diagnostics=[sysml_pb2.Diagnostic(severity="warning", message="heads up")],
    )
    conn = make_connection(stub)

    verdicts = conn.verify_satisfaction("hash1")

    assert stub.VerifySatisfaction.call_args[0][0].symbol_id == ""
    assert [v.holds for v in verdicts] == [True, False]
    assert verdicts[1].element == "satisfy massTiny by sedan"
    assert verdicts[1].condition == "vehicle.mass <= maxMass"
    # The diagnostics of the run are readable from any of its verdicts.
    assert [d.message for d in verdicts[0].diagnostics] == ["heads up"]
    assert [inst.id for inst in verdicts[0].instances] == [1, 2]


def test_verify_satisfaction_narrowed_to_a_symbol():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse()
    conn = make_connection(stub)

    assert conn.verify_satisfaction("hash1", symbol_id="Demo::analysis") == []
    assert stub.VerifySatisfaction.call_args[0][0].symbol_id == "Demo::analysis"


def test_calc_invocation_returns_its_value():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(real_value=6.5),
    )
    conn = make_connection(stub)

    result = conn.calc("Demo::add", "hash1", arguments=[2.5, 4.0])

    request = stub.EvaluateCalc.call_args[0][0]
    assert request.symbol_id == "Demo::add"
    assert [arg.real_value for arg in request.arguments] == [2.5, 4.0]
    assert isinstance(result, CalcResult)
    assert result.value == 6.5
    assert result.outputs == {}


def test_calc_usage_returns_its_outputs():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        outputs=[
            sysml_pb2.CalcOutput(name="a", value=sysml_pb2.Value(int_value=6)),
            sysml_pb2.CalcOutput(name="b", value=sysml_pb2.Value(int_value=10)),
        ],
    )
    conn = make_connection(stub)

    result = conn.calc("Demo::c", "hash1")

    assert stub.EvaluateCalc.call_args[0][0].arguments == []
    assert result.outputs == {"a": 6, "b": 10}
    assert result.value is None
    assert "a = 6" in str(result)


def test_calc_failure_raises_with_diagnostics():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        error="calc invocation failed: unbound input y",
        diagnostics=[sysml_pb2.Diagnostic(severity="error", message="unbound input y")],
    )
    conn = make_connection(stub)

    with pytest.raises(ExecutionError) as exc_info:
        conn.calc("Demo::add", "hash1", arguments=[1.0])
    assert "unbound input y" in str(exc_info.value)
    assert len(exc_info.value.diagnostics) == 1
    # ExecutionError is a builtin RuntimeError too, so `except RuntimeError`
    # catches it.
    assert isinstance(exc_info.value, RuntimeError)


def test_verification_requires_the_capability():
    stub = Mock()
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="old", capabilities=["typefacts"]
    )
    with patch('grpc.insecure_channel'):
        with patch(
            'pysysml.proto.sysml_pb2_grpc.SysMLServiceStub',
            return_value=stub,
        ):
            conn = Connection(auto_start=False)

    for call in (
        lambda: conn.verify_constraint("Demo::c", "hash1"),
        lambda: conn.verify_requirement("Demo::r", "hash1"),
        lambda: conn.verify_satisfaction("hash1"),
        lambda: conn.calc("Demo::add", "hash1"),
    ):
        with pytest.raises(MissingCapabilityError):
            call()
    stub.VerifyConstraint.assert_not_called()
    stub.EvaluateCalc.assert_not_called()


def test_evicted_model_is_a_model_not_found_error():
    stub = Mock()
    stub.VerifySatisfaction.side_effect = rpc_error(
        grpc.StatusCode.NOT_FOUND, "model not found: hash1"
    )
    conn = make_connection(stub)

    with pytest.raises(ModelNotFoundError) as exc_info:
        conn.verify_satisfaction("hash1")
    assert isinstance(exc_info.value.__cause__, grpc.RpcError)


def test_model_verification_helpers_pass_the_models_hash():
    stub = Mock()
    stub.ParseFile.return_value = sysml_pb2.ParseFileResponse(
        model_hash="hash1",
        root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
    )
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(kind="constraint", holds=True),
    )
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(kind="requirement", holds=True),
    )
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(kind="satisfy", holds=True),
            sysml_pb2.Verdict(kind="satisfy", holds=False),
        ],
    )
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(int_value=4),
    )
    conn = make_connection(stub)
    model = conn.load("demo.sysml")

    assert model.verify_constraint("Demo::c", subject="Demo::sedan").holds
    assert stub.VerifyConstraint.call_args[0][0].model_hash == "hash1"
    assert stub.VerifyConstraint.call_args[0][0].subject_symbol_id == "Demo::sedan"

    assert model.verify_requirement("Demo::r").holds
    assert stub.VerifyRequirement.call_args[0][0].model_hash == "hash1"

    assert len(model.verify_satisfaction()) == 2
    # One failing assertion is enough for the model not to satisfy them.
    assert model.satisfied() is False

    assert model.calc("Demo::add", arguments=[2, 2]).value == 4
    assert stub.EvaluateCalc.call_args[0][0].model_hash == "hash1"


def test_satisfied_is_false_when_an_assertion_could_not_be_evaluated():
    stub = Mock()
    stub.ParseFile.return_value = sysml_pb2.ParseFileResponse(
        model_hash="hash1",
        root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
    )
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(kind="satisfy", holds=False, error="unbound feature"),
        ],
    )
    conn = make_connection(stub)
    model = conn.load("demo.sysml")

    assert model.satisfied() is False


class TestVerdictLines:
    def test_an_assertions_line_does_not_repeat_the_kind_it_is(self):
        # The assertion's own text begins "satisfy …", so "satisfy satisfy …"
        # is not what a reader is shown.
        verdict = Verdict(
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massLimit by sedan",
                holds=False,
                condition="mass <= maxMass",
            )
        )
        assert str(verdict) == (
            "\u2717 satisfy massLimit by sedan fails: condition evaluated to "
            "false: mass <= maxMass"
        )

    def test_a_named_element_is_still_said_to_be_a_constraint(self):
        verdict = Verdict(
            sysml_pb2.Verdict(
                kind="constraint", element="Demo::Vehicle::massOK", holds=True
            )
        )
        assert str(verdict) == "\u2713 constraint Demo::Vehicle::massOK holds"
