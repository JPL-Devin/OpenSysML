# gRPC execution conformance cases

Layer 2 of the AGENTS.md §5.2 test contract for the gRPC service. Each case is a pair:

- `<name>.sysml` — a real model, parsed through the `ParseFile` RPC (stdlib loaded, semantic
  passes run). A case whose model reports a diagnostic *error* fails; import what you use
  (`import ScalarValues::*;`) rather than relying on an implicit library import.
- `<name>.expected.json` — the RPC to drive and the expected response.

`TestGRPCConformance` in `internal/grpc/conformance_test.go` discovers every `.expected.json`
in this directory, so adding a case is a data-only change.

## Expectation schema

| Field | Applies to | Meaning |
|---|---|---|
| `rpc` | all | `Evaluate`, `Instantiate`, `ExecuteAction` or `ExecuteState` |
| `expression` | Evaluate | expression source to evaluate |
| `context_symbol_id` | Evaluate | optional FQN whose scope the expression is evaluated in |
| `symbol_id` | Instantiate, ExecuteAction, ExecuteState | FQN of the subject |
| `inputs` | ExecuteAction | parameter name → value, bound before execution |
| `events` | ExecuteState | event names injected, in order |
| `expected_result` | Evaluate | expected `Value` |
| `expected_slots` | Instantiate | slot name → `{materialized, value_kind, value, error}` |
| `expected_instance_count` | Instantiate | number of reachable instances in the response graph |
| `expected_outputs` | ExecuteAction | output name → expected `Value` |
| `expected_states_visited` | ExecuteState | full ordered state-visit trace |
| `expected_final_context` | ExecuteState | context entry name → expected `Value` |
| `expected_error` | all | substring the RPC's in-band `error` must contain |

A case without `expected_error` requires an empty `error` field. A case with `expected_error`
asserts only the error, and is how failure modes (for example an action with no initial node)
are pinned.

A slot's `error` is a substring its `SlotValue.error` must contain; a slot without one must
carry no error.

A value is `{"kind": <oneof field of pb.Value>, "value": <literal>}`, where `kind` is one of
`int_value`, `real_value`, `bool_value`, `string_value`, `instance_id` or `null`. The
assertion checks the oneof arm as well as the payload, so a value returned with the wrong
type fails; `instance_id` and `null` assert the arm only, since instance ids are assigned at
runtime.
