- **A body's result expression converts to RDF and back.** `sysml -convert ttl` refused any
  calculation, analysis, verification or case whose body ends in a bare expression
  (`calc def Double { in x : Real; x * 2 }`), which is how the OMG corpora write most of theirs.
  The expression now converts as the metamodel states it: a `sysx:ResultExpressionMember` at its
  `sysx:memberIndex`, owned through a standard `sysml:ResultExpressionMembership` that carries the
  expression tree as `sysml:ownedResultExpression`, so a graph without `sysx:sourceText` still
  writes the notation back. Expression bodies — `{ in y : Real; y + x }` as a result, nested, or
  as the body of an `in expr` parameter — carry their parameters and result structurally as
  `sysx:bodyParameter` and `sysx:resultExpression`, and a `doc` opening one as a
  `sysml:Documentation` node (the parser now keeps it); any other declaration inside one stays a
  `sysx:BodyMember` with its text, and is refused by name when the text is absent. Across the 345
  example files, none of the 13 refused for this reason is any longer: 12 convert, 11 of them to
  a graph that round-trips equal, and the 13th is refused for an unrelated `feature` declaration;
  80 of the 81 calculation conformance models convert, up from 62. See
  [the mapping](docs/reference/rdf-mapping.md).
