// Package runtime provides the SysML v2 execution runtime: expression
// evaluation, instance materialization, and KerML operator library.
//
// This package implements Tiers 1–3 of the SysML v2 runtime:
//   - Tier 1: Feature flattening (effective-feature lists per type)
//   - Tier 2: Instance model (lazy slot materialization, multiplicity-driven collections)
//   - Tier 3: Expression evaluator (literals, operators, feature access, calc invocation, KerML builtins)
//
// Key types:
//   - Context: Runtime execution context (ID allocator, instance registry, memoization)
//   - Value: Runtime-evaluable value (int/real/bool/string/null/instance/Sequence/Set)
//   - EffectiveFeature: One entry in a type's effective feature list (Tier 1 schema)
//   - Instance: Runtime-materialized object with typed slots
//   - EvalContext: Lexical environment for evaluation (frame stack)
//
// Integration:
//   - Consumes semantics.Model (inherits features, multiplicity, constant folding)
//   - Gates on pass-validated models (LevelConstraint success)
//   - One Context per workspace session (LSP/REPL lifetime)
//
// Behavioral simulation (actions, state machines) is out of scope (future Tiers 4–5).
package runtime
