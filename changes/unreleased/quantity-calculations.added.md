- **The Quantities and Units domain library's calculations compute over quantities.** Every
  `QuantityCalculations` declaration dispatches to the runtime's unit-aware arithmetic:
  `sqrt(9 [m**2])` is `3.0 [m]`, while `sqrt(9 [m])` and `sqrt(9 [rad])` — an angle is
  dimension one, but a named unit all the same — are a typed error (`unit has no root`)
  rather than a magnitude in a fractional unit; `abs`, `floor` and `round` keep the unit;
  `max`/`min` convert to compare and answer the winning operand as written; `sum`/`product`
  fold in the first element's unit; the operator, comparison, predicate and conversion forms
  delegate to the code the operators already use. `import QuantityCalculations::*;` — which the
  ISQ examples do — no longer breaks `(1 [m], 2 [m])->sum()`, which computes `3 [m]` with the
  import and without it (where `sum` is the `NumericalFunctions::sum` the model imports). The
  `TrigFunctions` take an angle quantity (`sin(90 ['°'])`, `cos(0 [rad])`) through its declared
  scale, and only an angle: a bit, a byte, a steradian or `one` is dimension one but no angle,
  and is a type mismatch. `VectorCalculations` over a numeric vector
  compute as the Kernel `VectorFunctions` do; the quantity-scaled vector forms, `outer`, and
  every `MeasurementRefCalculations` and `TensorCalculations` declaration report themselves by
  name with the reason instead of `no result expression`. A parameter these libraries declare
  as `in : Type` binds by the name of the general's parameter it implicitly redefines
  (`VectorCalculations::angle(v = a, w = b)`), and where there is no general it is anonymous:
  it binds by position only, a named call is `ErrUnknownParameter` listing it as `#1`, and the
  registry publishes no name for it. A gate asserts every declaration of the four packages is
  either computed or named, parameters by effective name in declared order.
