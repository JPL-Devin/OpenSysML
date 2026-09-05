- **Six more KerML structural rules are checked the way the reference does.** A binding
  connector whose effective ends — declared, positional, bound `references` targets and
  inherited alike — are not exactly two reports `Binding connector must be binary`. A feature
  chain whose later link is not a feature of the type the earlier link reaches, written in a
  usage header, a `references`, a connector end or a flow, reports it as not featured within
  that type, aliases and inherited members followed. An annotating element that annotates
  itself reports `Must own its annotating element`. An `end` feature with a direction reports
  `End feature cannot have direction`, and one that is derived, abstract, variation, composite
  or portion reports `End feature cannot be derived, abstract, composite or portion`. A
  conjugated classifier, which owns no specialization of its own, must still reach its kind's
  default supertype through the type it conjugates (`Must directly or indirectly specialize
  Objects::Object`, say), and a conjugated feature that reaches no type at all reports
  `Features must have at least one type`. Anonymous usages now keep their `abstract`,
  `variation`, `ref`, `derived`, `constant`, `var` and portion modifiers, so these checks see
  them.
