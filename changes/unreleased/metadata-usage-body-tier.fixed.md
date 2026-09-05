- **A `metadata m : A { … }` usage body is checked at the same tier as an `@A { … }` annotation body.**
  The usage form's `Must redefine an owning-type feature` and `Must be model-level evaluable`
  reports used to be skipped whenever the document carried any type-tier error — including the
  very same violation written in a sibling `@A { … }` annotation — so only one of two identical
  bodies was reported. Both spellings are now reported together, as the pilot implementation
  reports them.
