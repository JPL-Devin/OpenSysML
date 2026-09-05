- **The state machine executor refuses the time trigger argument validation refuses.**
  `accept after 5` and `accept at 2 [min]` were reported by validation (`trigger-after-duration`,
  `trigger-at-time-instant`) yet scheduled by the runtime as five seconds and as an instant. The
  executor now makes the same judgement validation does, from the same declarations, and refuses
  such an argument as `ErrTimeTriggerType` when the state is entered, before the argument is
  evaluated or anything is scheduled; only an argument the declarations leave open — a feature
  whose type does not resolve — is read from its value, and a value there that is no time is
  still `ErrIncommensurableUnits`.
  Write `after 5 [s]` and `at` a `TimeInstantValue` feature, as the shipped conformance fixtures
  now do.
