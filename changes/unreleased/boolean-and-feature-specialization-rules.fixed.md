- **A `bool def` is checked like the other behavior definitions.** A Boolean expression
  definition specializing an attribute, item or part definition now draws the same `Cannot
  specialize …` report as a `calc def` or `constraint def` would; it was skipped by the family
  check because the kind had no entry.
- **Every KerML feature that subsets a classifier is reported, not only `feature`.** A `bool`,
  `expr`, `step` or other feature whose `:>` names a data type, class, structure or
  behavior is now reported (`subsets target must be a feature, found …`), as the reference
  implementation does when it fails to resolve the classifier at that position; only
  `feature f :> D` was checked.
