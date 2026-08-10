# Execution Conformance Schema

This directory contains behavioral execution conformance tests. Each test consists of:

1. **`<case>.sysml`** - The behavioral model (action/state/calc/constraint/requirement)
2. **`<case>.expected.json`** - Expected execution outcome
3. **`<case>.trace.golden`** - Optional ordered execution trace (see [Golden Traces](#golden-traces))

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
  "events": [{"signal": "sigB"}],
  "finalState": "Active.Cruising",
  "stateVisits": ["Off", "Idle", "Active", "Active.Accelerating", "Active.Cruising"],
  "outputs": {
    "stateData": {"type": "String", "value": "cruising"}
  }
}
```

- `events`: ordered list of events to inject into the machine. Each entry names
  either a signal (`{"signal": "sigB", "args": {...}}`, driving
  `AcceptEvent`-triggered transitions) or an operation invocation
  (`{"call": "setSpeed", "args": {"value": {"type": "Integer", "value": 55}}}`,
  driving `CallEvent`-triggered transitions), with `args` optional. Events are
  delivered in order. Optional; omit for autonomous (time/completion-driven)
  machines.
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

### For Instances (`Instantiate`)

```json
{
  "type": "instance",
  "instantiate": "test::Vehicle",
  "slots": {
    "mass": {"type": "Real", "value": 1500.0},
    "doubled": {"type": "Real", "value": 3000.0}
  },
  "constraints": {
    "withinLimit": true,
    "overLimit": false
  }
}
```

- `instantiate`: qualified name of the type to instantiate
- `slots`: expected values of the instance's own slots, materialized on demand
  — a default expression that reads sibling features (including through a
  nested part, `mass + engine.derated`) is evaluated against this object rather
  than constant-folded. Keys are slot names, not paths.
- `constraints`: expected verdict per constraint feature the instance carries,
  evaluated bound to the instance. `false` means the assertion evaluated to
  false (`ErrConstraintViolated`), not that evaluation failed.

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

## Golden Traces

A case may also carry a `<case>.trace.golden` recording the order in which the
case executes, checked by `TestExecutionTrace`. Calc and constraint cases record
calc evaluation: parameter binding, every sub-expression, and results.

```
enter calc test::scale
  bind x = 3 [argument]        # argument, or default when none is passed
  bind factor = 4 [default]
    eval feature x -> 3        # indentation is sub-expression nesting
    eval feature factor -> 4
  eval operator * -> 12
exit calc test::scale -> 12    # or `-> error: <typed error>`
```

Entries are canonical, never positions or addresses: parameters bind in
declaration order (inherited parameters first, at the position the declaring calc
gives them), and an unordered value such as a set renders sorted. Regenerate with
`go test ./internal/core/runtime/ -run TestExecutionTrace -update-traces` and
review every diff — a reordered entry is a behavior change, not noise.

Only cases that have a golden are trace-checked. State cases that broadcast an
event over orthogonal regions have no order-stable trace yet, so they carry no
golden rather than a flaky one.

## Known Failures

Cases in `known_failures.txt` are skipped (logged as `SKIP`). Remove from file when implemented.

## Provenance

Where possible, expected outcomes are derived from the OMG SysML v2 Pilot Implementation (commit 4c289b926). Cases with pilot-derived expectations are marked in comments.
