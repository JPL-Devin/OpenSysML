- **Every reader of a call selects the overload the checker selects.** Document queries and
  their `Column(...)` projections, and a `send` telling a calc call from a signal, resolved a
  name to the first declaration in view rather than to the overload its arguments fit — a
  query called with the other overload's arguments was refused as binding the wrong type, a
  same-named `Column` elsewhere hid the library's, and a signal and a calc sharing a name
  were told apart by import order. They now share the invocation selection, so a genuine tie
  is reported (`ambiguous-invocation` for a query) instead of picked silently. An expression
  whose name is also an action's selects the calc: an action has no result to evaluate, so
  `attribute v = tag(3);` no longer fails at run time when an `Integer` action beside a `Real`
  calc fits its argument more closely. `action call = tag(3);` and `perform tag(3);` keep
  selecting among actions only.
