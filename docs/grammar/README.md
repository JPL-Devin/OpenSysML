# Grammar Reference

**This directory does not contain grammar files.** The parser in `internal/core/parser/` is hand-written recursive descent.

## OMG Grammar Reference

For reference, the official Xtext grammar files from OMG are available at:

**KerML Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext

**SysML v2 Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext

These files are licensed under Eclipse Public License 2.0 (EPL-2.0) and are not included in this repository to avoid license mixing.

## State Machine Notation Beyond the OMG Grammar

The OMG textual notation covers state definitions/usages, `entry`/`do`/`exit`
subactions and transitions (`SysML.xtext`, `StateDefinition` … `TransitionUsage`,
around the `/* STATES */` section), and nothing else about state machines: there
is no production for a pseudostate of any kind and none for event deferral.
UML 2.5.1 §14.2.3.4 (Pseudostates) and the `State::deferrableTrigger` property of
§14.2.3 (StateMachines) define the semantics, but their notation is diagrammatic:
neither has a textual surface syntax to borrow.

Systemica therefore defines its own keywords for them, in a state body only.
They are a documented extension, not an OMG notation:

| Form | Meaning | UML concept |
|------|---------|-------------|
| `choice <name>;` | dynamic conditional branch | `choice` pseudostate |
| `junction <name>;` | static branch/merge | `junction` pseudostate |
| `fork <name>;` | parallel split | `fork` pseudostate |
| `join <name>;` | parallel synchronization | `join` pseudostate |
| `region <name> { … }` | orthogonal region | `Region` |
| `history <name>;` | shallow history (UML `H`) | `shallowHistory` pseudostate |
| `shallow history <name>;` | shallow history, spelled out | `shallowHistory` pseudostate |
| `deep history <name>;` | deep history (UML `H*`) | `deepHistory` pseudostate |
| `entry point <name>;` | entry point | `entryPoint` pseudostate |
| `exit point <name>;` | exit point | `exitPoint` pseudostate |
| `defer <event> [, <event>]*;` | events the state retains while active | `State::deferrableTrigger` |

Notes:

- `point` is **not** a reserved word: it is matched contextually after `entry`
  or `exit` and only when a pseudostate name and `;` follow, because models
  routinely declare features named `point`. `entry <action>` keeps its OMG
  meaning.
- A deferred event is parsed exactly like a transition trigger, so both a signal
  name (`defer Ping;`) and a call event (`defer setSpeed(value);`) are accepted;
  time and change events cannot be deferred and are reported at lowering.
- `defer` is only meaningful inside a state: one in the machine's own body is
  reported by `lower.ToStateGraph`.

## Validation

Grammar conformance is validated through parsing **OMG's own files**:

1. **Stdlib Conformance Gate** - all 95 bundled library files (94 OMG standard library files and 1 Systemica extension) must parse with zero diagnostics
   - See: `internal/core/libs/stdlib_conformance_test.go`
   - These files are the **source of truth** for correct parsing

2. **Training Examples** - 100 OMG training files (97/100 clean)
   - See: `docs/TRAINING_EXAMPLES.md`

3. **Golden AST Tests** - 33 fixtures with expected AST output
   - See: `internal/core/parser/testdata/parse/`

4. **Negative Tests** - 36 test cases for error recovery
   - See: `internal/core/parser/negative_test.go`

## Hand-Written Parser

The parser is hand-written for:
- **Performance** - 10-100x faster than generated parsers
- **Error Recovery** - Custom ErrorNode insertion for fault tolerance
- **Control** - Full control over diagnostic messages
- **Incremental Parsing** - Future LSP support
