- **A metadata type's `annotatedElement` alternatives are found by specialization, not by name.**
  A body feature merely named `annotatedElement` that specializes nothing is a duplicate member,
  not a restriction on what the metadata type may annotate; `@Named about Q` and `#Named part p`
  used to be reported `Cannot annotate …` against its type and are now accepted, as the pilot
  implementation accepts them. Only features that redefine or subset
  `Metaobjects::Metaobject::annotatedElement`, at any distance, are read.
