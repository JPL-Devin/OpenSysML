- **A calculation's `return` may specialize without a name or a typing.**
  `return :> ISQ::power = force * speed;`, `return deltaV :> ISQ::speed = v1 - v0;` and
  `return :> ISQ::length[*] = xs;` used to be rejected with `expected '{' or ';' after return
  parameter` followed by a cascade of `expected a body member`; they now declare the result
  parameter with its subsetting, as the pilot implementation reads them. A `return :>` that
  names no target is reported once, without a cascade.
