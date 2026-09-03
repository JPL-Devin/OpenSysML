- **A reference written back from RDF resolves to the element the graph names.** The notation
  writer spelled every reference as the name the source had used, read against the writing scope
  by a walk of its own; where a nearer declaration bore the same name — a redefining attribute
  named after its target, a subsetting part named after the part it subsets, a nested `part def`
  shadowing an outer one — the written name re-resolved to that declaration and the second
  conversion recorded a different graph (`redefines <itself>`, for the pilot `Packets.sysml`). The
  encoder now links each reference through the resolver's own reading of that occurrence
  (redefinition and subsetting targets looked up past the declaring feature, transition ends as
  vertices of the machine, feature-chain members in the operand's scope, `first x` labels and loop
  variables not links at all), and the writer spells each link as the short name only when the
  resolver reads it there as the linked element, else the shortest qualified suffix that it does,
  else the global form, and refuses a reference no spelling reaches. An unnamed usage that
  redefines or references a feature takes that feature's name, so a reference to it is a graph
  link written back by that name rather than a literal. An unnamed transition's effect now has a
  scope of its own, as a named one's does, so a succession between its members links both ends and
  the trigger's parameters reach however deeply the effect nests. A `then` after `first x` now
  sequences from the member `x` names, as the initial node itself does, so the pilot use-case model
  that redefines `start` comes back from its graph. A named multiplicity now owns the members its
  body declares, so a reference made there resolves from the body and is a link rather than a
  literal. The corpus gate writes each file back from the
  source text it carries, so its verdicts do not move; written from the graph alone, `Packets.sysml`
  now comes back as the same graph rather than a different one, and no file regresses.
  `TestRoundTripIsLossless` now also writes every
  fixture back from its graph with the source text removed and requires the same graph again, and
  a fixture reproduces each shadowing
  ([docs/reference/rdf-mapping.md](docs/reference/rdf-mapping.md#limitations)).
