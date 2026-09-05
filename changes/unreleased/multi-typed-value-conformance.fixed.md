- **A value typed by several types conforms when any one of them does.** `Bound features
  should have conforming types` and `cast-conformance` judged a multi-typed value (`part ab : A, B`,
  a calculation returning `A, B`, an element `abs#(1)` or `abs.?{…}` of such a sequence, a feature
  typed only by such a value) by its first type alone, so a result expression, a bound subject, a
  `satisfy … by` operand or a cast that conformed through the second type was reported. Every
  statically known type is now kept and the reference implementation's existential rule applied;
  arithmetic and conditional results, which are not statically known, stay silent as before.
