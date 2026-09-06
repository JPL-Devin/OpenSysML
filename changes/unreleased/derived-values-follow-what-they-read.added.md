- **A derived `=` value follows the features it read.** `attribute a : Integer default 3;
  attribute d : Integer = a * 2;` read `d` as `6` once and kept it after `a` was assigned `9`;
  the runtime now records what a `=` value reads while it is derived and, when one of those
  features changes — an `assign` in an action or state body, a `SetFeatureValue` from the REPL or
  the gRPC service, a binding propagating a new value, a classifier's subsetter superseding a
  `default null` collection a roll-up had summed while empty — drops the derived value so the
  next read derives it again, transitively through the values that read it. Nothing is
  recomputed before it is read, a value a run assigned keeps the assignment, a probe or
  transaction that wrote such a feature is rolled back with the values that read it, and a value
  derived from itself is still `ErrCyclicFeatureValue`. The `in` parameters of a calc or action
  usage are unchanged: bound once per invocation, they stay bound while its outputs are read.
