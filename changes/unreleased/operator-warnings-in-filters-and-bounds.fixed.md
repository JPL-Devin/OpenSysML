- **Cast, bracket and quantity warnings reach filter conditions and multiplicity bounds.** The
  operator-expression checks were only applied to values and behavior bodies; an
  `x[i]` in a KerML filter, an unrelated `as` target or a non-reference unit in `filter` or `[lo..hi]`
  is now reported once, like the same expression elsewhere.
