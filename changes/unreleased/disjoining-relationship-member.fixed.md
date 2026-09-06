- **A KerML `disjoint X from Y;` member keeps which end is disjoined from which.** The
  keyword-first Disjoining is now a relationship element of its own, like `subtype A specializes B`
  and `inverse f of g`, with ordered ends, an optional `disjoining <id>` identification, a
  visibility prefix and a relationship body; both ends resolve, including feature-chain ends such
  as `disjoint earlierOccurrence.successors from laterOccurrence.predecessors;`. The RDF export
  writes it as a `sysml:Disjoining` with `typeDisjoined` and `disjoiningType`, and a graph
  without source text converts back to the same notation — previously it was an anonymous feature
  with two `disjointFrom` objects that came back as `disjoint from X, Y;`, which does not parse.
  The declaration clause `class C specializes A disjoint from B;` is unchanged.
