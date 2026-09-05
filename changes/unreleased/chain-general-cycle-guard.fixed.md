- **A generalization written as a feature chain is kept when resolving it re-enters an
  active lookup.** `feature b subsets x.f` declared inside a feature that itself subsets
  `a::b` used to lose `f` as a supertype for good: the chain's lookup was cut short by the
  resolver's cycle guard and the incomplete answer was memoized, so `b` inherited nothing
  from `f`. The answer is now provisional, as it already was for a qualified-name target,
  and the next query resolves the chain.
- **A generalization that fell back to an outer name while its owner's supertypes were
  still being computed is no longer memoized.** A member's `specializes X` resolved while
  the owner was mid-way through its own supertype query could not yet see the `X` the
  owner inherits and settled for a same-named `X` in an enclosing namespace; that answer
  was cached by both the resolver and the semantic model, so the inherited general was
  lost for good. Such an answer now holds for the query that made it only, and the next
  query finds the inherited one.
