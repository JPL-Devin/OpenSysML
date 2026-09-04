- **A redefining requirement subject or actor is bound under every name of the feature it
  redefines.** `requirement r : Req { subject renamed :>> truck = loaded; }` used to leave a
  condition inherited from `Req` that reads `truck` (or its short name) unbound, reporting
  `truck subject is unbound` although the redefinition supplies the value. The binding now
  reaches the subject under its own name and short name, the names of every feature it
  redefines — explicitly, by short name, implicitly by role, or through a redefinition of a
  redefinition — and a redefinition that values nothing reads what the redefined feature binds.
  A subject `by` a `satisfy` assertion overrides the declaration under all of those names, and
  the same declaration written without a name (`subject <s> :>> x;`) is one member under `s`
  and `x`, as `part <p> :>> x;` is, so both names resolve, are shown and rename together.
