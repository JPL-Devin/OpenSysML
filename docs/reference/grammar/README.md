# Grammar Reference

**This directory does not contain grammar files.** The parser in `internal/core/parser/` is hand-written recursive descent.

## OMG Grammar Reference

For reference, the official Xtext grammar files from OMG are available at:

**KerML Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext

**SysML v2 Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext

These files are licensed under Eclipse Public License 2.0 (EPL-2.0) and are not included in this repository to avoid license mixing. `./scripts/download-pilot-grammars.sh` fetches them at the pinned release into `build/pilot-grammars/` when they are needed.

Which of their productions our test inputs have ever exercised is measured, on input-presence evidence rather than execution coverage, in [grammar-coverage.md](../../project/grammar-coverage.md).

## Conformance Audit

[conformance-audit.md](conformance-audit.md) records, with `file:line` citations
at the pinned grammars, which words and constructs OpenSysML accepts are standard
(silent), which are our own extensions (warned as `nonstandard-notation`), and
which are KerML-only in a `.sysml` file (warned as `kerml-notation`).

## State Machine Notation Beyond the OMG Grammar

The OMG textual notation covers state definitions/usages, `entry`/`do`/`exit`
subactions and transitions (`SysML.xtext`, `StateDefinition` … `TransitionUsage`,
around the `/* STATES */` section), and nothing else about state machines: there
is no production for a pseudostate of any kind and none for event deferral.
Some of these concepts nevertheless have semantics in the bundled KerML semantic
library or the Systems Library, and those are the governing reference; UML 2.5.1
§14.2.3.4 (Pseudostates) is cited only for the ones that have neither, whose
notation there is diagrammatic and so has no textual surface syntax to borrow.

OpenSysML therefore defines its own keywords for them, in a state body only.
They are a documented extension, not an OMG notation, and using one draws a
`nonstandard-notation` warning:

| Form | Meaning | Semantic reference |
|------|---------|-------------|
| `choice <name>;` | dynamic conditional branch | KerML `ControlPerformances::DecisionPerformance` — selects one of the successions leaving it, `outgoingHBLink: HappensBefore[1]` (notation is an OpenSysML invention) |
| `junction <name>;` | static branch/merge | KerML `DecisionPerformance::outgoingHBLink[1]` / `MergePerformance::incomingHBLink[1]` (notation is an OpenSysML invention) |
| `fork <name>;` | parallel split | UML `fork` pseudostate (a state-body fork has no SysML v2 or KerML counterpart; the action-level one is `Actions::ForkAction`) |
| `join <name>;` | parallel synchronization | UML `join` pseudostate (a state-body join has no SysML v2 or KerML counterpart; the action-level one is `Actions::JoinAction`) |
| `region <name> { … }` | orthogonal region | UML `Region` |
| `history <name>;` | shallow history (UML `H`) | UML `shallowHistory` pseudostate |
| `shallow history <name>;` | shallow history, spelled out | UML `shallowHistory` pseudostate |
| `deep history <name>;` | deep history (UML `H*`) | UML `deepHistory` pseudostate |
| `entry point <name>;` | entry point | UML `entryPoint` pseudostate |
| `exit point <name>;` | exit point | UML `exitPoint` pseudostate |
| `defer <event> [, <event>]*;` | events the state retains while active | KerML `StatePerformances::StatePerformance::deferrable: Transfer[0..*] subsets acceptable` — "transfers … can be considered for acceptance more than once"; dispatch order is `Occurrences::Occurrence::incomingTransferSort`, defaulting to `earlierFirstIncomingTransferSort` |

The action-level `fork`/`join` control nodes are SysML v2's `Actions::ForkAction`
and `JoinAction`, whose behavior "results from requiring that the target
[respectively source] multiplicity of all outgoing [incoming] succession
connectors be 1..1" — the library states no behavior of their own, a
`ControlAction` having "no inherent behavior".

Notes:

- `fork` and `join` are the exception in that table: both are action node
  literals a state body admits (`SysML.xtext:1684`, `:1678`, `:1761-1763`), so
  they are read as standard and are not warned about.
- None of `choice`, `decision`, `deep`, `defer`, `done`, `final`, `history`,
  `initial`, `junction`, `region` or `shallow` is reserved: each is a literal in
  none of the pinned grammars, so all are ordinary names, matched contextually
  where the notation above needs them.
- `point` is **not** a reserved word: it is matched contextually after `entry`
  or `exit` and only when a pseudostate name and `;` follow, because models
  routinely declare features named `point`. `entry <action>` keeps its OMG
  meaning.
- `on` and `var` are **not** reserved either, for the same reason and on the
  pilot implementation's authority: `on` is a literal in none of its grammars,
  and `var` only in `KerML.xtext` `BasicFeaturePrefix` (`isVariable ?= 'var'`).
  So `state on { … }` and `then on;` (the OMG training corpus writes both) and
  `attribute var : Integer;` declare and name features, while `var` before a
  kind keyword (`var feature x`, `var attribute total : Integer;`) still marks a
  variable feature. `var` with the kind keyword left out is not supported and is
  reported. See [pilot-differential.md](../../project/pilot-differential.md).
- A deferred event is parsed exactly like a transition trigger, so both a signal
  name (`defer Ping;`) and a call event (`defer setSpeed(value);`) are accepted;
  time and change events cannot be deferred and are reported at lowering.
- `defer` is only meaningful inside a state: one in the machine's own body is
  reported by `lower.ToStateGraph`.

## Validation

Grammar conformance is validated through parsing **OMG's own files**:

1. **Stdlib Conformance Gate** - all 95 bundled library files (94 OMG standard library files and 1 OpenSysML extension) must parse with zero diagnostics
   - See: `internal/core/libs/stdlib_conformance_test.go`
   - These files are the **source of truth** for correct parsing

2. **Training Examples** - 100 OMG training files (current result in the page below)
   - See: `docs/project/training-examples.md`

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
