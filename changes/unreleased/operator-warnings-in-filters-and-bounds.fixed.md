- **Cast, bracket and quantity warnings reach filter conditions and multiplicity bounds.** The
  operator-expression checks were only applied to values and behavior bodies; an
  `x[i]` in a KerML filter, an unrelated `as` target or a non-reference unit in `filter` or `[lo..hi]`
  is now reported once, like the same expression elsewhere. The declarations an expression body
  `{ … }` makes are reached too, and a bound's operators are judged by the type tier, so an
  unrelated error elsewhere in the document no longer silences them.
