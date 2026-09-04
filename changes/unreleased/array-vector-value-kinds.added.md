- **An Array, a vector and a vector quantity are runtime values of their own.** A
  `Collections::Array` usage shaped by its `dimensions` and `elements` evaluates to an Array
  value printed `Array(2, 3)[1, 2, 3, 4, 5, 6]`, whose `rank`, `flattenedSize`, `dimensions`
  and `elements` read out and which `CollectionFunctions::'array#'(a, (2, 1))` and
  `a#(2, 1)` index in row-major order, one-based, as the pilot evaluator does; an index count
  other than the rank, an index outside its dimension and an `elements` list that does not
  fill the `dimensions` are typed errors. `VectorFunctions::VectorOf`, `CartesianVectorOf`,
  `CartesianThreeVectorOf` and every vector operation answer a vector printed `⟨1.0, 2.0⟩`,
  distinct from the sequence `[1.0, 2.0]`, so `VectorFunctions::sum`/`sum0` over a sequence
  of vectors and the library feature `cartesianZeroVector` — the 1-, 2- and 3-dimensional
  zero vectors — now evaluate instead of flattening or failing to write a Real into a
  `CartesianVectorValue`. `VectorCalculations::scalarQuantityVectorMult`,
  `vectorScalarQuantityMult` and `vectorScalarQuantityDiv`, and the `*`/`/` operators between
  a scalar quantity and a vector, answer a vector quantity printed `⟨2.0, 4.0⟩ [m]` whose unit
  is composed by the same rule as the scalar quantities'; a vector of no components takes no
  unit, as a quantity's `num` is `Number[1..*]`. `inner`, `norm` and `angle` over vector
  quantities answer the `Number` the library declares — the magnitude over the components, the
  unit dropped by declaration — so a `Number` feature takes them, as the checker already allowed. A vector binds to a feature typed by any
  `NumericalVectorValue` specialization whose fixed dimension and element type it fits — a
  model's own `:> NumericalVectorValue { :>> elements : Integer; }` as much as
  `CartesianThreeVectorValue` — and the refusal names the declaration it fails; an object of
  such a specialization reads as a vector that keeps its own members (`t.tag`), directly, through a
  calc parameter and on a calc's result. Each new kind is handled wherever
  the runtime, REPL, traces, solver and gRPC bridge inspect a value's kind, and a test
  enumerates the kinds so a future one cannot be left out. Tensors, coordinate
  transformations and a measurement reference passed as an argument value stay typed
  unevaluable, each with the reason.
