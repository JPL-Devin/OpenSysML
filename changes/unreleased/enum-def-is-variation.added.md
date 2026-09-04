- **Every enumeration definition is a variation, and its enumerated values are its variants.**
  `enum def F :> E;` is now rejected as a variation specializing another variation, as are an
  `enum def` specializing a `variation`, a `variation` specializing or typed by an `enum def`, and
  a member of an `enum def` that is not an enumerated value; the messages name the implicit
  variation and the fix. The reflective `isVariation`/`isVariant` read by element filters, the
  variant queries (`VariantsOf`, `SelectsVariantOf`) behind the runtime and the solver, and the
  RDF export (`sysml:isVariation`, `sysml:isVariant`, `sysml:variant`) all derive the same facts
  for an enumeration definition and its values, and the RDF import reads them back without
  inventing a `variation` or `variant` keyword the enumeration grammar has no place for. A usage
  typed by an enumeration still holds one of its values, as any attribute does, and an enumerated
  value still evaluates to its literal. A declaration an enumeration body cannot own (a nested
  definition, package, import or alias) is reported by `enumeration-body-member`, and every
  `EnumeratedValue` form the grammar admits — typed, anonymous, redefining, with a default or
  initial value, behind visibility or prefix metadata such as `#$::P::M` — parses as an
  enumerated value rather than an attribute.
