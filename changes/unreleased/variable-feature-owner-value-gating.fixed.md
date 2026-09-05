- **An error in an owner's value no longer hides its members' variability diagnostics.**
  `Initialized feature must be variable` and `Only a variable feature can be constant` on a feature
  declared inside a usage's body used to go unreported when the usage's own value failed a lower
  tier (`part p : PD = missing { attribute a : Integer := 1; }`). A value does not decide whether
  the members are variable, so only the owner's head before its value gates them now, as the
  feature's own head already did.
