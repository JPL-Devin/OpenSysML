- **A generalization written as a feature chain is kept when resolving it re-enters an
  active lookup.** `feature b subsets x.f` declared inside a feature that itself subsets
  `a::b` used to lose `f` as a supertype for good: the chain's lookup was cut short by the
  resolver's cycle guard and the incomplete answer was memoized, so `b` inherited nothing
  from `f`. The answer is now provisional, as it already was for a qualified-name target,
  and the next query resolves the chain.
