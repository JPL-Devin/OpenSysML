- **Metadata annotations are checked against the metaclass they name.** A KerML metadata
  feature (`@M`, `@M about …`, `metadata m : M`) must be typed by exactly one metaclass — an
  ordinary class, structure or data type is reported `Must have exactly one metaclass` (KerML
  `validateMetadataFeatureMetadata`) — and a SysML metadata usage by exactly one metadata
  definition (`A metadata usage must be typed by one metadata definition.`,
  `validateMetadataUsageType`), where a part definition was accepted before. The elements an
  annotation may be applied to are now read from the metaclass's own `annotatedElement`
  features — declared, inherited, redefined or subsetted, resolved through the reflective `KerML`
  library rather than a fixed list of kinds — and each annotated element, in the `@M` and the
  `@M about …` forms alike, is reported `Cannot annotate <Metaclass>` when its metaclass does not
  conform (`validateMetadataFeatureAnnotatedElement`). A feature written in an annotation body,
  in KerML as in SysML and at any nesting depth, must redefine a feature of the metaclass or of
  one it specializes: an explicit `:>> g` naming a feature elsewhere is reported `Must redefine an
  owning-type feature` (`validateMetadataFeatureBody`), which previously applied only to the SysML
  `metadata … : M` usage form. Model-level evaluability of a metadata value follows the pinned
  pilot: an unfeatured feature is as evaluable as its own value, a feature of another type is not,
  and a metadata feature always is.
