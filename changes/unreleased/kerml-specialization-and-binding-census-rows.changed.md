- **Specialization and binding-conformance validation follows the reference more closely.** A
  class, data type or SysML definition specializing the wrong classifier family is reported
  through `:>` as well as `specializes`, in SysML with the reference's wording (`Cannot specialize
  attribute definition`, `Cannot specialize item definition`) in place of the former kind-mismatch
  message; a conjugated feature at the specific end of a standalone `specialization subset`,
  `redefinition` or `typing` is reported like a conjugated subclassifier. `Bound features should
  have conforming types` now also covers the bindings the language implies — a result expression
  against its result parameter, a `satisfy … by` operand and a nested requirement's or case's
  subject against the subject they fill. An invocation argument that corresponds to no input
  parameter is headed by the reference's `Must correspond to one input parameter of the invoked
  type`, at the argument itself.
