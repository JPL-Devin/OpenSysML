- **An unrelated error in a package no longer hides its features' variability diagnostics.**
  `Initialized feature must be variable` and `Only a variable feature can be constant` on a feature
  declared directly in a package or namespace used to go unreported whenever any sibling member
  later in that package failed a lower tier (an unresolved typing, say). A package has no typing of
  its own to fail, so it now gates nothing: only the feature's own head, and the head of a
  definition or usage that owns it, silence the rule.
