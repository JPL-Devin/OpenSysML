"""Wire compatibility of the message types that existed before verification.

Adding RPCs and messages to sysml.proto must not move a field of a type already
on the wire: a client built against the older schema has to keep reading what a
newer service writes, and vice versa. These tests pin the field numbers of the
pre-existing types and check that bytes written by one schema parse back
unchanged.
"""

from pysysml.proto import sysml_pb2


# Field numbers as of the schema before the verification RPCs were added. A
# change here is a wire break, not a test to update.
LEGACY_FIELD_NUMBERS = {
    "ParseFileRequest": {"file_path": 1, "content": 2, "content_hash": 3},
    "ParseFileResponse": {"model_hash": 1, "root": 2, "diagnostics": 3},
    "GetSymbolRequest": {"model_hash": 1, "symbol_id": 2},
    "SymbolResponse": {"symbol": 1, "error": 2},
    "DiagnosticsRequest": {"model_hash": 1},
    "DiagnosticsResponse": {"diagnostics": 1},
    "EvaluateRequest": {
        "model_hash": 1,
        "expression": 2,
        "context_symbol_id": 3,
    },
    "EvaluateResponse": {"result": 1, "error": 2, "diagnostics": 3},
    "Instance": {"id": 1, "type_symbol_id": 2, "slots": 3},
    "SlotValue": {
        "feature_name": 1,
        "value": 2,
        "values": 3,
        "materialized": 4,
        "error": 5,
    },
    "InstantiateRequest": {"model_hash": 1, "symbol_id": 2},
    "ExecuteActionRequest": {"model_hash": 1, "action_symbol_id": 2, "inputs": 3},
    "ExecuteStateRequest": {
        "model_hash": 1,
        "state_machine_symbol_id": 2,
        "events": 3,
    },
    "Value": {
        "int_value": 1,
        "real_value": 2,
        "bool_value": 3,
        "string_value": 4,
        "instance_id": 5,
        "sequence": 6,
        "null": 7,
    },
    "ValueSequence": {"elements": 1},
    "Diagnostic": {"severity": 1, "message": 2, "span": 3},
    "Span": {
        "file": 1,
        "start_line": 2,
        "start_col": 3,
        "end_line": 4,
        "end_col": 5,
    },
    "ServerInfoResponse": {"version": 1, "capabilities": 2},
}


def test_legacy_field_numbers_are_unchanged():
    for message_name, fields in LEGACY_FIELD_NUMBERS.items():
        descriptor = getattr(sysml_pb2, message_name).DESCRIPTOR
        got = {
            name: descriptor.fields_by_name[name].number
            for name in fields
            if name in descriptor.fields_by_name
        }
        assert got == fields, f"{message_name} field numbers moved"


def test_legacy_rpcs_are_still_declared():
    service = sysml_pb2.DESCRIPTOR.services_by_name["SysMLService"]
    names = {method.name for method in service.methods}
    assert {
        "GetServerInfo",
        "ParseFile",
        "GetSymbol",
        "GetDiagnostics",
        "Evaluate",
        "Instantiate",
        "ExecuteAction",
        "ExecuteState",
        "Convert",
    } <= names


def test_parse_file_response_round_trips():
    response = sysml_pb2.ParseFileResponse(
        model_hash="abc123",
        root=sysml_pb2.SymbolInfo(
            id="Demo",
            name="Demo",
            kind="Package",
            child_ids=["Demo::Vehicle"],
            metadata={"declared": "true"},
        ),
        diagnostics=[
            sysml_pb2.Diagnostic(
                severity="error",
                message="expected a namespace member",
                span=sysml_pb2.Span(
                    file="demo.sysml",
                    start_line=3,
                    start_col=2,
                    end_line=3,
                    end_col=11,
                ),
            )
        ],
    )

    again = sysml_pb2.ParseFileResponse()
    again.ParseFromString(response.SerializeToString())
    assert again == response


def test_instance_graph_round_trips():
    response = sysml_pb2.InstantiateResponse(
        instance=sysml_pb2.Instance(
            id=1,
            type_symbol_id="Demo::Vehicle",
            slots={
                "mass": sysml_pb2.SlotValue(
                    feature_name="mass",
                    value=sysml_pb2.Value(real_value=1200.0),
                    materialized=True,
                ),
                "wheels": sysml_pb2.SlotValue(
                    feature_name="wheels",
                    values=[
                        sysml_pb2.Value(instance_id=2),
                        sysml_pb2.Value(instance_id=3),
                    ],
                    materialized=True,
                ),
                "broken": sysml_pb2.SlotValue(
                    feature_name="broken",
                    error="feature is unbound",
                ),
            },
        ),
        instances=[
            sysml_pb2.Instance(id=2, type_symbol_id="Demo::Wheel"),
            sysml_pb2.Instance(id=3, type_symbol_id="Demo::Wheel"),
        ],
    )

    again = sysml_pb2.InstantiateResponse()
    again.ParseFromString(response.SerializeToString())
    assert again == response
    assert again.instance.slots["mass"].value.real_value == 1200.0


def test_every_value_kind_round_trips():
    for value in (
        sysml_pb2.Value(int_value=-7),
        sysml_pb2.Value(real_value=1.5),
        sysml_pb2.Value(bool_value=True),
        sysml_pb2.Value(string_value="hello"),
        sysml_pb2.Value(instance_id=42),
        sysml_pb2.Value(null=""),
        sysml_pb2.Value(
            sequence=sysml_pb2.ValueSequence(
                elements=[
                    sysml_pb2.Value(int_value=1),
                    sysml_pb2.Value(string_value="two"),
                ]
            )
        ),
    ):
        again = sysml_pb2.Value()
        again.ParseFromString(value.SerializeToString())
        assert again == value
        assert again.WhichOneof("kind") == value.WhichOneof("kind")


def test_execute_responses_round_trip():
    action = sysml_pb2.ExecuteActionResponse(
        outputs={"total": sysml_pb2.Value(int_value=9)},
        error="",
        diagnostics=[sysml_pb2.Diagnostic(severity="warning", message="slow")],
    )
    again = sysml_pb2.ExecuteActionResponse()
    again.ParseFromString(action.SerializeToString())
    assert again == action

    state = sysml_pb2.ExecuteStateResponse(
        states_visited=["Idle", "Running"],
        final_context={"count": sysml_pb2.Value(int_value=2)},
    )
    again_state = sysml_pb2.ExecuteStateResponse()
    again_state.ParseFromString(state.SerializeToString())
    assert again_state == state


def test_unknown_verification_fields_survive_an_older_reader():
    """A verification response is opaque, not corrupting, to an older client.

    An older client parses what it does not know as unknown fields and can write
    them back out byte-for-byte, which is what makes adding messages safe.
    """
    response = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[sysml_pb2.Verdict(kind="satisfy", holds=True, element="satisfy r by p")],
    )
    payload = response.SerializeToString()

    # ServerInfoRequest declares no fields, so it stands in for a client whose
    # schema knows nothing of this message: everything lands in unknown fields.
    older = sysml_pb2.ServerInfoRequest()
    older.ParseFromString(payload)
    assert older.SerializeToString() == payload
