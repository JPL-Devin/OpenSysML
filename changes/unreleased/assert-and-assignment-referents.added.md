- **An `assert` must reference a constraint, and an `assign` must name a feature.** `assert c`,
  `assert not c` and `assert constraint c` now report `assert target must be a constraint usage,
  found partUsage` when `c` is not a constraint usage (a requirement usage counts, and a feature
  chain is judged by its last feature), sharing the referent-kind check `satisfy` already had.
  `assign PD := 1;` where `PD` is a part definition, a package, a datatype or any other
  non-feature now reports `An assignment must have a referent.` followed by what the target is
  declared as, instead of being accepted; an unresolved target keeps its name-resolution error as
  the first and only diagnostic, and only a feature reaches the time-varying rule. Both rules
  are refereed against the pinned pilot, which rejects the same models at the same positions.
