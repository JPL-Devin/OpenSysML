- **`RationalFunctions::rat`, `numer` and `denom` compute.** `rat(n, d)` is the binary64
  quotient the `/` operator computes (`rat(1, 3)` is `0.3333333333333333`, `rat(6, 4)` is
  `1.5`), and `rat(n, 0)` is the division-by-zero error `1 / 0` reports, never an infinity.
  `numer(x)` and `denom(x)` read the exact numerator and positive denominator, in lowest
  terms, of the binary64 a Rational is here — `numer(0.75)` is `3`, `denom(0.75)` is `4`,
  `numer(2)` is `2` over `1` — so `rat(numer(x), denom(x)) == x` holds for every finite `x`
  whose terms are Integers; a term past the Integer range (`denom(0.0001)` is 2^66) is an
  overflow error, an infinity a domain error. Because the value is the double nearest what
  was written, `numer(0.1)` is `3602879701896397` over `2^55` rather than `1` over `10` — the
  same class of artifact as `0.1 + 0.2 != 0.3`, matching the pinned pilot's `double`
  storage; the pilot itself evaluates none of the three, so they are self-assessed
  (`docs/project/exact-rational-evaluation.md`). Before, all three reported themselves as
  unevaluable.
