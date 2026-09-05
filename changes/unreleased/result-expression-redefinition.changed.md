- **A specialization or redefinition that inherits a result expression may not state a second.**
  `constraint def Sub :> Base { x > 1 }`, `require constraint :>> c { x > 1 }`,
  `calc def D :> C { x + 2 }` and a calculation or constraint usage typed by, subsetting or
  reference-subsetting (`constraint d ::> c { x > 1 }`) one that owns a body are rejected with
  `Only one (owned or inherited) result expression is allowed`, on the newly stated body, as the
  reference validators reject them; two generals each owning a
  result expression are reported on the declaration that inherits both, and a calculation,
  function or expression body listing two bare expressions (`calc c { 1 2 }`) is reported on
  the second, which the reference grammar does not admit. An empty or
  documentation-only redefinition (`:>> c;`, `:>> c { }`, `:>> c { doc /* … */ }`) keeps the
  inherited expression, and a nested `assert constraint { … }` remains a separate constraint, so
  a tighter requirement is written as a new or nested constraint rather than by replacing the
  inherited body. The runtime agrees: a calculation or constraint that states or inherits more
  than one result expression is refused with a typed error, naming each owner, instead of being
  evaluated with a silently chosen body.
