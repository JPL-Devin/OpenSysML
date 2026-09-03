- **Composed units render in one canonical form.** A unit an operation composes is a sorted
  product of powers of the units the operands were written in — `3 [m] * 3 [m]`,
  `(3 [m]) ** 2` and `(3 [m] * 3 [m]) / 3 [m]` print `9 [m**2]`, `9 [m**2]` and `3.0 [m]` rather
  than `9.0 [m*m]`, `9 [(m)**2]` and `3.0 [(m*m)/m]`, and `(m/s) * (kg/s)` prints
  `kg*m/s**2` — while a named derived unit stays as written (`2 [N*m]`, `18.0 [km/h**2]`). A
  product of quantities keeps the magnitude kind bare arithmetic gives, so `l1 * l1` and
  `l1 ** 2` agree. The REPL, the trace and the gRPC `unit` field all render the one text, and
  a quantity sent over gRPC composes as one written locally does: `SI::m/SI::s` times `SI::s`
  is `SI::m`. A unit text the model cannot read as a whole is read name by name, so the units
  it does declare stay factors of their own: `SI::s` composed into `metres per second` is
  `'metres per second'*SI::s`, and dividing by `SI::s` after a round trip gives the opaque unit
  back; text that is no unit name is quoted so the product reads back as itself. Only the
  trigonometric functions take a dimension-one quantity for a number;
  `IntegerFunctions::abs(1 [rad])` is a type mismatch, as its declaration says.
