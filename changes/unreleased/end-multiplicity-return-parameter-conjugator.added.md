- **End features, return parameters and conjugation are checked the way the reference does.** An
  end feature none of whose multiplicities — its own, or one it takes through subsetting,
  redefinition, typing, a reference, a feature chain or an implicit end — is exactly `1..1` is
  reported with the warning `End feature must have multiplicity 1`; a SysML end usage defaults to
  `1..1`, so only a declared own multiplicity other than `[1]` warns there, and the multiplicity
  written between `end` and the keyword (`end [0..*] item x : A`) belongs to the cross feature
  and is silent. A `return` parameter whose owner is no function, expression, calculation,
  constraint, requirement or case is the error `Return parameter membership not allowed`, and a
  type that declares a second conjugation (`classifier C ~A ~B;`) reports `Cannot have more than
  one conjugator` on each `~` past the first.
- **The multiplicity between `end` and the keyword is the cross feature's.** `end [m] item x : A`
  now declares an anonymous cross feature carrying `[m]`, as the grammar reads it, instead of
  copying `[m]` onto the end itself; an end that also declares its own `[n]` keeps both. An end
  that declares its cross feature this way and also `crosses` another feature is reported
  `Must be the cross feature`, as the reference does.
