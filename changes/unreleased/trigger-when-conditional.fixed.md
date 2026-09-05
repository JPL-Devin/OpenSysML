- **A `when` trigger written as a conditional is refused like the other triggers.**
  `accept when (if flag ? true else false)` was accepted because both branches are Boolean,
  while `after` and `at` already refused a conditional argument as the `Anything` its library
  function returns, and the reference validator refuses all three. The `when` argument is now
  judged the same way (`trigger-when-boolean`, naming the result of `if`); an ordinary
  condition — a guard, a constraint, an `if` — still leaves a conditional to evaluation.
