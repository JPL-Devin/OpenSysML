- **Solving an analysis case that states or inherits more than one result expression is refused.**
  `analysis def Stated :> Base { size <= 4 }` over a `Base` that owns a result expression, or
  `analysis def Inherited :> Base, Other;` inheriting one from each, is rejected by validation
  with `Only one (owned or inherited) result expression is allowed`; the runtime's case
  conditions now record the same conflict, so `solve` refuses the analysis at the offending body
  or declaration instead of solving it with a silently chosen result. A case inheriting a single
  result expression is unaffected.
