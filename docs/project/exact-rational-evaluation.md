# Exact-rational evaluation: adjudicated and declined

The solver work left one asymmetry standing: the evaluator computes `Real`/`Rational`
arithmetic in IEEE 754 binary64 while the SMT translation reasons over the exact `Real`
(rational) sort, so the two can disagree wherever a value is not float64-representable.
The [solver soundness record](spec-compliance.md#exact-reals-against-a-rounding-evaluator--what-agreement-is-claimed) made the
*verdict* sound rather than exact — sat witnesses are replayed through the evaluator's
own arithmetic, and a query the evaluator rounds does not report an exact-real `unsat`
as an evaluator verdict — at the cost of completeness: rounded queries answer undecided
where a float64-aware solver might have decided them.

The remaining way to close that gap would be to make the evaluator itself exact — a
`big.Rat`-backed value representation, so there is nothing for the solver to disagree
with. This record adjudicates that change: what the reference implementation actually
computes, what the specification actually requires, what the change would touch, what
it would cost, and why it is **declined**.

## What the reference computes: binary64, observably and structurally

The pinned pilot (`jupyter-sysml-kernel` 0.60.1, provisioned by
`scripts/download-pilot-validator.sh` + `scripts/download-pilot-evaluator.sh`, driven
by `build/pilot-evaluator/eval-sysml --cases`) was probed with cases chosen to
distinguish exact-rational from binary64 evaluation. Its answers, verbatim:

| Case | Pilot answer | Exact-rational answer would be |
|------|--------------|--------------------------------|
| `0.1 + 0.2` | `LiteralRational 0.30000000000000004` | `0.3` |
| `0.1 + 0.2 == 0.3` | `LiteralBoolean false` | `true` |
| `0.1 + 0.2 <= 0.3` | `LiteralBoolean false` | `true` |
| `0.3 < 0.1 + 0.2` | `LiteralBoolean true` | `false` |
| `0.1 + 0.2 - 0.3` | `LiteralRational 5.551115123125783E-17` | `0` |
| `(1.0 / 49.0) * 49.0 == 1.0` | `LiteralBoolean false` | `true` |
| `(1.0 / 3.0) * 3.0 == 1.0` | `LiteralBoolean true` (double rounding happens to land on 1.0) | `true` |
| `0.1` ten-fold sum `== 1.0` | `LiteralBoolean false` | `true` |
| `1.0 / 3.0`, `1 / 3` | `LiteralRational 0.3333333333333333` | an exact third |

`5.551115123125783E-17` is exactly the binary64 value of `0.1 + 0.2 - 0.3`; every row is
the IEEE 754 double answer, none is the exact-rational one. The evidence is structural
too: in the pinned jar, `LiteralRationalImpl.value` is a Java `double`
(`javap`: `protected double value; public double getValue(); public void setValue(double)`),
so a rational literal is rounded to binary64 at parse time, before any arithmetic runs.
(Its integer literals are narrower still: `9007199254740993` answers
`ERROR:For input string: "9007199254740993"` — a Java 32-bit `parseInt`, the limit the
division work had already recorded.)

OpenSysML today answers **identically on every one of these probes** (`0.30000000000000004`,
`false`, `false`, `true`, `5.551115123125783e-17`, `false`, `true`, `false`,
`0.3333333333333333`). An exact-rational evaluator would therefore not close a gap with
the reference — it would **open one**, flipping the observable answer of every probe row
above against the pilot, and `cmd/pilot-exec-diff` would report each as a disagreement.
This inverts the premise of the change: the evaluator's binary64 arithmetic *is* the
reference behavior.

## What the specification requires: nothing about precision

KerML 1.0 (formal/2025-02-01; the 1.1 RTF has not published changes to these clauses)
describes the data types mathematically:

- §9.3.2.2.8 Rational: "Rational is the type of rational numbers, extended with values
  for positive and negative infinity."
- §9.3.2.2.9 Real: "Real is the type of mathematical (extended) real numbers. This
  includes both rational and irrational numbers, and values for positive and negative
  infinity."
- §8.3.4.8.13 LiteralRational: the abstract-syntax `value` attribute is typed `Real`
  ("The value whose rational approximation is the result of evaluating this
  LiteralRational"), and §8.4.4.9.2 notes that "only the rational-number subset of the
  real numbers can be represented using a finite literal. So the result of a
  LiteralRational is actually always classified in the KerML DataType Rational."

That is a statement about what the *values are*, not about what arithmetic an
implementation must perform. The Kernel Function Library (§9.4; `RealFunctions.kerml`,
`RationalFunctions.kerml`) declares only signatures — `function '+' … in x: Real[1]; in
y: Real[0..1]; return : Real[1];` — with no precision, rounding, or exactness clause,
and no conformance clause elsewhere in the specification constrains numeric precision.
The spec is **silent on precision**; the reference implementation chose binary64. There
is no clause an exact-rational evaluator could point to as mandating it, and the only
executable oracle contradicts it.

## What the change would touch

For completeness of the adjudication, the blast radius of a `big.Rat`-backed (or
exact-until-formatted) `Real`/`Rational` value, mapped concretely:

- `internal/core/semantics`: `Value` carries `Real float64` (`eval.go`); the constant
  folder's `evalRealArith`/`RealArith`, `IntQuotient`, `Pow`, comparisons and equality,
  and the numeric-widening lattice all move to a rational representation.
- `internal/core/runtime`: the evaluator (`eval.go`, `toReal`), `value.go`
  (`FormatReal` and all printing), `library_functions.go` (34 `math.*` call sites —
  `sqrt`, trig, `floor`/`round`, `exp`/`ln` — which have no exact form), quantities and
  unit scaling, collections, overflow handling; 69 `float64` sites in the package.
- Wire and clients: `api/proto/sysml.proto` carries `double real_value`,
  `double real_magnitude`, unit `scale_num`/`scale_den`, `double exponent`; an exact
  value needs a new wire form and migrations in the Go service and the Python client
  (`Real, Rational → float` is a documented mapping in the Python API reference).
- Fixtures: conformance `.expected.json` files and golden traces that print reals, the
  REPL output contract, and the pilot execution referee's normalization (which today
  matches the pilot digit-for-digit because both sides are binary64).
- The solver seam: `solve/replay.go` — the witness replay and `Query.Rounded` marking —
  encodes the evaluator's rounding; an exact evaluator rewrites that contract and its
  tests (`TestSolvedWitnessRejectedByEvaluatorIsUndecided`,
  `TestRoundedMarksFloatComputingQueries`, …), the very behavior the soundness work
  just pinned down.
- The irrational remainder: `sqrt`, trig, `**` with a fractional exponent and the
  transcendentals cannot be exact, so the result is necessarily a **hybrid** — exact
  for the field operations, rounded for irrational functions — and any query touching
  an irrational operation re-enters exactly the rounded-query incompleteness this
  change was meant to remove. Exactness only ever covers the rational-closed fragment.

## What it would cost, measured

Microbenchmarks on this machine (Go 1.23, `math/big`; float64 loop vs the equivalent
`big.Rat` loop, reusing allocated `Rat`s):

| Workload | float64 | big.Rat | Ratio |
|----------|---------|---------|-------|
| mul + quo + add per iteration | 0.33 ns/op | 731 ns/op | ~2,200× |
| accumulating sums of distinct small fractions | 0.30 ns/op | 10,380 ns/op | ~35,000× |

The second row is the structural problem, not just a constant factor: exact rational
accumulation grows denominators without bound (the running sum's denominator tends
toward the lcm of everything added), so operand size — and per-operation cost and
memory — grows with the computation. A simulation loop accumulating quantities is the
runtime's hot path.

## The options, compared

| Option | Solver agreement bought | Pilot agreement | Cost |
|--------|------------------------|-----------------|------|
| (a) Full exact-rational values | Closes the gap on the rational-closed fragment only; irrational ops re-open it | **Diverges** on every probe row above | Every package listed above, a wire-format migration, fixture rewrites, 3–4 orders of magnitude on numeric hot paths |
| (b) Exact-rational only where it closes the solver gap, binary64 elsewhere | Same fragment as (a) | Diverges wherever the exact path is used | The same expression yields different values depending on whether a solver looks at it — a worse contract than either uniform choice |
| (c) Keep binary64 + sound-but-incomplete verdicts (status quo) | Sound verdicts; rounded queries stay undecided | **Agrees** (verified digit-for-digit on the probes) | Zero; already landed and tested |
| (d) Narrow `Query.Rounded` by proving exactness per term | Recovers decided verdicts for provably-exact float64 computations (dyadic constants, small-magnitude sums) without touching the value representation | Agrees (no evaluator change) | Contained in `solve/replay.go` `roundedTerm`; subtle — a term over free Real variables can round on some witness values and not others, so unmarking is only sound for terms whose *whole value set* is exact |

## Decision

**Declined — the evaluator stays binary64; option (c) stands.** The reference
implementation computes in binary64 (behaviorally and structurally verified above), the
specification is silent on precision, and this project's conformance posture is
agreement with the pinned pilot wherever it can speak. An exact-rational evaluator
would diverge from the reference on observable answers, cost measured orders of
magnitude on hot paths, force a hybrid contract that re-admits the same solver gap at
every irrational operation, and rewrite the wire format and the just-landed soundness
seam. The incompleteness it would remove is narrow (rounded queries answer undecided
rather than wrong) and already documented as the deliberate trade.

Option (d) is recorded as the one refinement worth holding open: it narrows the
undecided surface without moving the value contract or the pilot agreement, and it is
contained in one function — but it is subtle enough (exactness must hold for a term's
whole value set, not one witness) that it should wait until the undecided verdicts are
observed to bite in practice.
