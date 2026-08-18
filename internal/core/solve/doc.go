// Package solve translates the conditions a constraint, requirement or
// satisfaction assertion states into a solver-independent term IR and writes
// that IR as an SMT-LIB2 script.
//
// It also runs an external solver over that script — z3 or cvc5, found on PATH
// or named by OPENSYSML_SMT — as a process speaking SMT-LIB2 on standard input,
// so no library is linked in and releases stay pure Go. The verdicts sat, unsat
// and unknown stay distinct: a timeout or arithmetic the solver gave up on is
// unknown, and a solver that crashes, is absent, or replies unusably is a typed
// error rather than a verdict.
//
// The runtime evaluator remains the normative semantics; SysML v2 defines no
// solving semantics, so this is an advertised extension, not a conformance
// claim.
//
// Conditions come from the evaluator's own collection
// (runtime.Context.ConditionsOf), keeping its order and its distinctions:
// `require` versus `assume`, negation, and a body meaning the conjunction of
// its conditions.
//
// # Translatable subset
//
// A condition is translatable when every part of it is:
//
//   - Boolean operators: `not`, `and`, `&`, `or`, `|`, `xor`, `implies`, and the
//     conditional expression `if c ? a else b`.
//   - Equality `==` and `!=` between two values of the same sort.
//   - Comparisons `<`, `<=`, `>`, `>=` between numbers of the same dimension.
//   - Arithmetic `+`, `-`, `*`, unary `-` and `+`.
//   - Division `/` and remainder `%`. Integer division truncates toward zero as
//     the evaluator does, encoded as ite(a >= 0, div(a, b), -div(-a, b)), and the
//     remainder as a - b*tdiv(a, b), which takes the dividend's sign. A literal
//     divisor keeps this linear; a variable divisor sets Query.Nonlinear, so
//     unknown is an expected verdict rather than a surprise.
//   - Division by zero, which SMT-LIB leaves underspecified while the evaluator
//     refuses it: a literal zero divisor refuses translation, and any other
//     divisor, integer or real, is asserted non-zero as a RoleDefined side
//     condition. That assertion constrains the whole query, so it is only made
//     where the division is always evaluated and read unnegated; a computed
//     divisor under `not`, `or`, `xor`, `implies`, a conditional branch or a
//     denied element refuses instead, since the evaluator may never divide there.
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
// A variable stands for the value a feature may take, constrained only by its
// sort: declared values are not asserted, so a query asks what the conditions
// permit rather than what one object holds.
//
// # Deliberately out of subset
//
// Everything else refuses with ErrNotTranslatable, and one refused conjunct
// fails the whole query, so no partial script exists:
//
//   - Collections and quantifiers: sequences, sets, `->select`, `->collect`,
//     `->forAll`, `->exists`, `->size`, indexing `#(i)`, ranges `a..b`, and
//     collection-valued features. Bounded expansion is not implemented.
//   - Invocations of any kind, calc usages included: a calc body may be
//     iterative or read state, and constant folding it is the evaluator's job.
//   - Euclidean `div`/`mod` themselves, as SMT-LIB defines them: only the
//     evaluator's truncating semantics are encoded.
//   - Real remainder `%`, which the evaluator answers by floating-point
//     remainder.
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
