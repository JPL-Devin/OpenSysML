# Execution Conformance Schema

This directory contains behavioral execution conformance tests. Each test consists of:

1. **`<case>.sysml`** - The behavioral model (action/state/calc/constraint/requirement)
2. **`<case>.expected.json`** - Expected execution outcome

## Schema Format

### For Actions (`ExecuteAction`)

```json
{
  "type": "action",
  "outputs": {
    "paramName": {"type": "Integer", "value": 42},
    "result": {"type": "Real", "value": 3.14}
  },
  "tokenCount": 5
}
```

- `outputs`: map of output parameter names to their final values
- `tokenCount`: number of tokens processed (optional, for regression detection)

### For States (`ExecuteState`)

```json
{
  "type": "state",
  "events": ["sigB"],
  "finalState": "Active.Cruising",
  "stateVisits": ["Off", "Idle", "Active", "Active.Accelerating", "Active.Cruising"],
  "outputs": {
    "stateData": {"type": "String", "value": "cruising"}
  }
}
```

- `events`: ordered list of signal names to inject into the machine (drives
  `AcceptEvent`-triggered transitions). Each name is delivered in order after the
  machine reaches a stable configuration. Optional; omit for autonomous
  (time/completion-driven) machines.
- `finalState`: qualified name of final reached state
- `stateVisits`: ordered list of states visited (optional, for golden trace verification)
- `outputs`: map of state machine outputs

### For Calculations (`InvokeCalc`)

```json
{
  "type": "calc",
  "inputs": [
    {"type": "Real", "value": 10.0},
    {"type": "Real", "value": 2.0}
  ],
  "result": {"type": "Real", "value": 12.0}
}
```

- `inputs`: ordered list of input arguments
- `result`: returned value

### For Constraints (`EvaluateConstraint`)

```json
{
  "type": "constraint",
  "bindings": {
    "pressure": {"type": "Real", "value": 50.0},
    "temp": {"type": "Real", "value": 100.0}
  },
  "satisfied": true
}
```

- `bindings`: variable bindings for constraint evaluation
- `satisfied`: boolean, whether constraint is satisfied

### For Requirements (`EvaluateRequirement`)

```json
{
  "type": "requirement",
  "bindings": {
    "vehicle.speed": {"type": "Real", "value": 120.0}
  },
  "satisfied": true
}
```

- `bindings`: variable bindings for requirement evaluation
- `satisfied`: boolean, whether requirement is satisfied

## Value Format

All values use this format:

```json
{"type": "TypeName", "value": <JSON-serializable>}
```

Supported types:
- `Integer`: JSON number (no decimals)
- `Real`: JSON number (may have decimals)
- `Boolean`: JSON boolean
- `String`: JSON string
- `Null`: JSON null

## Adding New Cases

1. Create `<case>.sysml` with the behavioral model
2. Create `<case>.expected.json` with expected outcome
3. Run `go test ./internal/core/runtime/ -run TestExecutionConformance -v`
4. If test fails but behavior is correct, verify and update expected file
5. If behavior is unimplemented, add case name to `known_failures.txt`

## Known Failures

Cases in `known_failures.txt` are skipped (logged as `SKIP`). Remove from file when implemented.

## Provenance

Where possible, expected outcomes are derived from the OMG SysML v2 Pilot Implementation (commit 4c289b926). Cases with pilot-derived expectations are marked in comments.
