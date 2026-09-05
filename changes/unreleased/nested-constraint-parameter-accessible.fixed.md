- **A `require`/`assume constraint` body reaches the parameters it declares.**
  `require constraint q { in y : Integer; y > 0 }` used to report `y` as
  `Must be an accessible feature`, because the body was judged from the requirement rather
  than from the constraint usage it states. The body is now checked from that usage, so its
  own parameters and the requirement's features are both reachable, while a feature named
  through another type's namespace is still reported, as the pilot implementation does.
