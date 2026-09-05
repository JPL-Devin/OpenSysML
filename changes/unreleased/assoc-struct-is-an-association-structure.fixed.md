- **A KerML `assoc struct` is an association structure.** The parser recorded it as a plain
  `struct`, so it specialized `Objects::Object` instead of `Objects::LinkObject`, its features
  subset `Objects::objects` instead of `Objects::linkObjects`, and its metaclass was `Structure`
  instead of `AssociationStructure`. It now keeps the compound keyword through the parser, the
  implicit-specialization tables, the metaclass table and the Xpect export.
