- **A nested library shape's edges, vertices and per-face dimensions evaluate.** A `Rectangle`
  answers `rect.edges` with four `Line`s, `rect.e1.length`/`rect.e3.length` with its `length`
  and `rect.e2.length`/`rect.e4.length` with its `width`, `rect.vertices` with eight objects
  and `rect.v12`…`rect.v41` with two each; a `Box` answers `box.edges` with the twenty-four
  edges of its six faces and `box.tf.length`/`box.tf.width` with the cuboid's `length` and
  `width`; a `Triangle` values `base`, `e2` and `xoffset`, and `RightTriangle::hypotenuse`
  reports the library's squared length as the dimension mismatch it is. Four general rules do
  this, and each holds for a model of your own: an object listed in a typed collection
  (`item :>> edges : Line = (e1, e2, e3, e4)`) is classified by the collection's type rather
  than refused, keeping its identity, while a value that cannot be so classified — an
  `Integer` into `edges : Line` — stays `type mismatch`; a qualified name of an enclosing
  type's feature read inside a nested usage (`attribute :>> length = Rectangle::length;`) is
  that feature of the enclosing object, and a sibling chain (`e3.length = e1.length`) the
  sibling's; a feature chain valued over a collection (`edges = faces.edges`) collects across
  it, and a required lower bound the named subsetting features fall short of is filled from an
  optional subsetting feature before an anonymous object is made up; and a usage declared with
  no kind keyword (`doubled = span * 2.0;`) is a reference usage that materializes and evaluates
  like any attribute. A binding connector's end multiplicities now reach the runtime, so two
  `[0..*]` bindings of one collection agree or are a `binding conflict`, while a `[0..1]`
  binding that links one unspecified value of each end (`bind [0..1] tf.edges = [0..1] tfe`)
  is reported as the binding end it cannot resolve rather than answered with a witness — so
  `box.vertices` and `box.tfe`…`box.urre` are typed errors naming the binding. An optional
  feature holding nothing is the empty sequence on every surface: `%features box` prints
  `shape = []` as `%eval box.shape` does, while a required feature holding nothing is still
  uninitialized and a valueless `Real` still `<unset>`. A read or write naming no feature of
  the object is the typed `object has no such feature` error, which is what
  `box.matingOccurrences` and `box.spaceBoundary` — Kernel frame features, not features of the
  shape — now report; the vertex-mating `assert constraint` bodies of `Path`/`Polygon` and the
  curved `Cylinder`/`Cone` edge graph remain documented limitations with the same typed errors.
