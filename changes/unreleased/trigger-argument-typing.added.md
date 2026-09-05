- **The argument of an `accept after`, `at` or `when` trigger is typed.** An `after` delay must
  be a `DurationValue`, an `at` time a `TimeInstantValue` and a `when` condition a Boolean, as
  the SysML v2 `TriggerInvocationExpression` constraints require and the reference validator
  enforces; `accept after 5` is reported as `trigger-after-duration` with the unit-bearing
  spelling suggested (`after 5 [s]`), and `trigger-at-time-instant` and `trigger-when-boolean`
  name the other two. The judgement is semantic rather than syntactic: a quantity literal is
  typed by the unit it is written in, a feature by its declared type through inheritance,
  redefinition, aliases and feature chains (a feature declared by nothing but a value takes the
  value's type), a call by the result of the overload its arguments select, and arithmetic by
  the dimension of its value — so `after Twice(d) + 5 [s]` is silent and `after Len() * Len()`
  is reported as a value of dimension L². Triggers nested in action, state and transition
  bodies, including the body an action-target succession carries, are checked, and a body
  declared there now gets its own scope. An argument whose type only evaluation determines is
  left to it, and an unresolved name is reported by name resolution alone. A body `{ … }`
  written as a value is the expression itself, not its result, wherever a value is typed: it is
  reported as a trigger argument, bound to a typed feature (`attribute b : Boolean = { true }`),
  passed to a typed parameter or given to an operator, as the reference validator reports it,
  while its members are still checked. The check is gated
  per trigger, so an unresolved name elsewhere in the document does not hide an invalid trigger,
  and a workspace document that redeclares a library type's qualified name does not disable it.
