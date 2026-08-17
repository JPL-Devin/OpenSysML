// Package solve translates the conditions a constraint, requirement or
// satisfaction assertion states into a solver-independent term IR and writes
// that IR as an SMT-LIB2 script.
//
// This is an optional, additive extension: the runtime evaluator
// (internal/core/runtime) remains the normative semantics of Invariant,
// RequirementUsage and `assert satisfy`, and nothing here changes a verdict it
// reaches. SysML v2 defines no solving semantics, so a translation is an
// advertised extension rather than a conformance claim. No solver is invoked —
// the package produces a script, and running one is a caller's business.
//
// Conditions come from the evaluator's own collection
// (runtime.Context.ConditionsOf), so the terms encode exactly what the evaluator
// checks, with the same order and the same distinctions: `require` versus
// `assume`, negation, and a body meaning the conjunction of its conditions
// (negating a body negates that conjunction, not each condition).
//
// # Translatable subset
//
// A condition is translatable when every part of it is:
//
//   - Boolean operators: `not`, `and`, `&`, `or`, `|`, `xor`, `implies`, and the
//     conditional expression `if c ? a else b`.
//   - Equality `==` and `!=` between two values of the same sort.
//   - Comparisons `<`, `<=`, `>`, `>=` between numbers of the same dimension.
//   - Arithmetic `+`, `-`, `*`, unary `-` and `+`, and `/` where the result is a
//     real number.
//   - Literals: boolean, integer, real, string.
//   - Quantity expressions (`450.0 [km/h]`), normalized to the base units their
//     unit reduces to through semantics.UnitTermOf — magnitudes are exact
//     rationals, so a scale factor introduces no rounding.
//   - References to scalar-valued features, resolved through the same names the
//     evaluator resolves: Boolean, String, Natural (declared non-negative),
//     Integer, Rational, Real and Number features, features typed by a quantity
//     value type, enumeration-typed features and variation points.
//   - Feature chains that ground in such a feature (`lander.verticalSpeed`).
//   - Enumeration literals and variants, as constructors of a finite datatype
//     sort declared per enumeration definition or variation point.
//
// A variable stands for the value a feature may take, unconstrained except by
// its sort: values a model declares are not asserted, so a query asks what the
// conditions permit rather than what one object holds. Two feature chains
// reaching the same feature through different objects are two variables.
//
// # Deliberately out of subset
//
// Everything else refuses with ErrNotTranslatable rather than being dropped, and
// one refused conjunct fails the whole query — a partial script would answer
// sat/unsat about conditions it does not contain:
//
//   - Collections and quantifiers: sequences, sets, `->select`, `->collect`,
//     `->forAll`, `->exists`, `->size`, indexing `#(i)`, ranges `a..b`, and
//     collection-valued features. Bounded expansion is not implemented.
//   - Invocations of any kind, calc usages included: a calc body may be
//     iterative or read state, and constant folding it is the evaluator's job.
//   - Integer division and `%`: SMT-LIB's `div`/`mod` are Euclidean while the
//     evaluator truncates toward zero, so encoding them would change answers.
//   - `**` and `^`: exponentiation is outside linear and polynomial arithmetic
//     as encoded here.
//   - Classification and metadata operators: `hastype`, `istype`, `@`, `@@`,
//     `as`, `meta`, `all`, `===`, `!==`, `??`, `~`, and `null`.
//   - Complex numbers, string operations other than equality, and features whose
//     type determines no scalar sort.
//   - Comparing or adding magnitudes of different dimensions, which the
//     evaluator reports as incommensurable units.
//   - Unresolved names and feature chains that ground in nothing.
package solve
