- **Cast, bracket and quantity warnings reach filter conditions and multiplicity bounds.** The
  operator-expression checks were only applied to values and behavior bodies; an
  `x[i]` in a KerML filter, an unrelated `as` target or a non-reference unit in `filter` or `[lo..hi]`
  is now reported once, like the same expression elsewhere. The declarations an expression body
  `{ … }` makes are reached too, and a bound's operators are judged by the type tier, so an
  unrelated error elsewhere in the document no longer silences them. Every bound the notation
  writes is covered: a `multiplicity m [lo..hi]` declaration, a body parameter's `in x : T[lo..hi]`,
  a cast's, a connector end's, a cross feature's and a subject's or assume/require's. The members
  a `multiplicity` or `specialization` declaration owns in its body are type-checked like any other
  body's, so an invalid cast or value in them is reported too, also when the declaration sits under
  a definition or package that a filter's expression body declares.
