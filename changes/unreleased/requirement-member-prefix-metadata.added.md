- **A `#M` prefix on a `subject`, `assume` or `require` member now means what it means on any
  usage.** The prefix was parsed and carried through the RDF mapping but the semantic engine
  never saw it. `MetadataAnnotationsOf` now reports it, a semantic metadata keyword
  (`Metaobjects::SemanticMetadata`) makes the member specialize the keyword's `baseType`, a
  metadata definition that restricts `annotatedElement` is checked against the member
  (`Cannot annotate ConstraintUsage` on `assume #OnDefinitions constraint c : C;`, silence on
  one that admits usages), and an abstract metadata type is reported there as elsewhere.
- **The constraint usage an `assume` or `require` member owns is a declaration in its own
  right.** `assume constraint c : C;` and `require constraint r : C[0..1] { … }` now declare
  `c` and `r` in the requirement: they are found by name, typed by `: C`, bounded by their
  multiplicity, redefine what `:>>` names (and their bodies see names nested under it),
  take a constraint usage's implicit base, and go-to-definition on a `:>> c` lands on the
  member. An anonymous `require constraint { … }` still declares nothing and its conditions
  still belong to the requirement.
