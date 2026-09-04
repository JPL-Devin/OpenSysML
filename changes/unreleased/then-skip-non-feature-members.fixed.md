- **A positional `then` sequences past every member that is not a feature.** The parser read
  `action a; doc /* … */ then action b;` as a succession from the documentation, and the same for
  a `comment`, a `rep`, an `import`, an `alias`, a nested definition or `package`, a `multiplicity`
  declaration, a `defer` written before the `then`: the runtime then refused to lower an action body (`succession edge references
  undefined source node`) and a state machine stopped at the state before the `doc`. The rule the
  parser and the RDF writer share is now `ast.IsSuccessionSource` — a feature that is not an edge
  (`ast.UsageKind.IsEdge`) — so a `then` sequences from the nearest feature before it, as the pilot
  implementation resolves it (`UsageUtil.getPreviousFeature`) and as SysML v2 §7.17.4 reads. A
  `then` with only non-feature members before it is diagnosed as having nothing to sequence from.
  The writer folds a succession back into `then` past the same members and refuses a graph whose
  source is one of them; `docs/reference/rdf-mapping.md` records the rule and its two known gaps
  (an end-less `flow`/`message`, and an `alias` of a feature, which the pilot keeps as the source).
