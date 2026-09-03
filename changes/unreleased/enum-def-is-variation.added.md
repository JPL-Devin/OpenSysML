- **Every enumeration definition is checked as a variation.** `enum def F :> E;` is now
  rejected as a variation specializing another variation, as are an `enum def` specializing
  a `variation`, a `variation` specializing or typed by an `enum def`, and a member of an
  `enum def` that is not an enumerated value; the messages name the implicit variation and the
  fix. The reflective `isVariation`/`isVariant` read by element filters derive `true` for
  enumeration definitions and their values. A declaration an enumeration body cannot own
  (a nested definition, package, import or alias) is reported by `enumeration-body-member`,
  and every `EnumeratedValue` form the grammar admits — typed, anonymous, redefining, with
  a default or initial value, behind visibility or prefix metadata such as `#$::P::M` — parses
  as an enumerated value rather than an attribute.
