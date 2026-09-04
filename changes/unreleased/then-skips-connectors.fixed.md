- **A positional `then` sequences past a `connect`, `connection`, `interface`, `allocate` or
  `allocation`.** The parser read `action a; connect p to q; then action b;` as a succession
  from the connection, so the runtime refused to lower it (`succession edge references undefined
  source node written as anonymous connection usage`) and the RDF writer folded `then` back only
  past a `flow`, `bind`, `succession` or `transition`. The rule the parser and the writer share,
  `ast.UsageKind.IsEdge`, now covers every connector kind: a `then` sequences from the nearest
  member before it that is not a connector or a transition, as the pilot implementation resolves
  it (`UsageUtil.getPreviousFeature`). `docs/reference/rdf-mapping.md` records the rule, its basis
  and the one known gap — the pilot keeps a `flow` or `message` written with no ends as the
  source, this implementation reads past it.
