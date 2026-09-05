- **Association arity, binary-link end counts and multiplicity bound types are checked the way
  the reference does.** A concrete KerML `assoc`, `assoc struct` or `interaction` with fewer than
  two ends is reported `Must have at least two related elements`, as a SysML `connection def`
  already was; an interaction now implicitly specializes `Links::Link`, as any association does.
  A connector, binding, succession, flow or association that conforms to `Links::BinaryLink` yet
  has more than two ends — positional `(x, y, z)` ends, declared `end` features and inherited
  ends counted alike — reports each end past the second, with the redefinition check no longer
  masking it. A multiplicity bound whose result type resolves to anything but an Integer-conforming
  data type — a feature typed by a class, a call to a function whose result is a class, or a
  quantity such as `3 [kg]`, say — is rejected with `Must have a Natural value`; an unresolved or
  untyped bound stays silent.
- **KerML binary connectors parse as the grammar reads them.** `connector a to b`,
  `connector [0..1] a to [1..*] b`, `connector e ::> a.x to b.y`, `connector e references x to y`
  and `connector $::P::a to b` declare an anonymous connector with two ends; only `connector c from a to b` names one. The
  first end is no longer mistaken for the connector's name, so a model with two such connectors
  no longer reports a duplicate declaration.
