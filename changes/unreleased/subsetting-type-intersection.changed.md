- **Why an unrelated type on a subsetting feature is not a diagnostic is now recorded.** The
  compliance record explains that `feature f : B subsets g;` under `feature g : A;` is
  well-formed because subsetting adds `A` to `f`'s types rather than requiring `B` to conform
  to it (KerML §8.3.3.3.4, §7.3.4.4) — the shape the OMG training corpus's
  `Model Library Example` uses and the reference validator accepts — while a redefinition,
  which replaces the redefined feature, is still held to type conformance.
