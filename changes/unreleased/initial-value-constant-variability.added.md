- **An initial value or `constant` requires a variable feature.** A feature is variable when KerML
  declares it `var` (or `const`), or when a SysML usage may time-vary: owned by an occurrence type,
  not a portion, and not a composite action (KerML 8.3.3.1 `Feature::isVariable`,
  `validateFeatureValueIsInitial`, `validateFeatureConstantIsVariable`). A `:=` initial value on any
  other feature — an attribute of a data type or `attribute def`, a behavior parameter, a timeslice,
  a root usage — now reports `Initialized feature must be variable` at the value, and a `constant`
  prefix on one reports `Only a variable feature can be constant` at the usage, at the positions the
  pinned pilot reports. `var attribute x : Integer := 1;` in an `item def`, `constant attribute c = 1;`
  on a part, and `:=` anywhere inside an occurrence stay silent, as does every model under `examples/`
  and the OMG corpora.
