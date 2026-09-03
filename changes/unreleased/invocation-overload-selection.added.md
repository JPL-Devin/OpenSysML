- **A call selects the overload its arguments fit, and the checker and the runtime select the
  same one.** A name visible as several function or calc declarations — owned, inherited,
  imported or re-exported, a library function among them only where its package is imported — no
  longer resolves to whichever declaration is found first: the candidates are filtered by arity,
  by positional or named binding, and by argument-type conformance (the `ScalarValues` lattice,
  strings, booleans, collections, quantities and declared types), and the most specific fit is
  selected. `ToInteger("7")` is `IntegerFunctions::ToInteger`, `abs(-2)` answers `2` and
  `abs(rect(3.0, 4.0))` answers `5.0` through `ComplexFunctions::abs`, where the unqualified
  name used to be rejected with `expects Real, found String` or `requires a numeric value`. A
  genuine tie is reported as `invocation-ambiguous`, naming the tied candidates, and refused at
  runtime as `ErrAmbiguousInvocation` rather than dispatched silently; a call no candidate fits
  keeps its argument diagnostic and names the declarations considered. The selection is
  memoized in a side table keyed by the invocation node and scope, and the evaluator dispatches
  the declaration recorded there. A model's own `calc` of the name still shadows the library,
  and an argument whose type is statically unknown keeps the previous selection with no new
  diagnostic. `ComplexFunctions::sum` and `product`, which a Real collection now selects where
  `ComplexFunctions` is imported and `RealFunctions` is not, fold Real elements as the library's
  `reduce '+'` does — on the real axis — so `sum((1.0, 2.0))` stays the Real `3.0` rather than
  becoming `3.0 + 0.0i`.
