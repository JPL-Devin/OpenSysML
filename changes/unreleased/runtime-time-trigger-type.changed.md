- **The state machine executor refuses the time trigger argument validation refuses.**
  `accept after 5` and `accept at 2 [min]` were reported by validation (`trigger-after-duration`,
  `trigger-at-time-instant`) yet scheduled by the runtime as five seconds and as an instant. The
  executor now makes the same judgement validation does, from the same declarations, and refuses
  such an argument as `ErrTimeTriggerType` when the state is entered, before anything is scheduled;
  an argument the declarations leave open — an untyped feature — is still read from its value.
  Write `after 5 [s]` and `at` a `TimeInstantValue` feature, as the shipped conformance fixtures
  now do.
