- **A calculation's `return` may specialize without a name or a typing.**
  `return :> ISQ::power = force * speed;`, `return deltaV :> ISQ::speed = v1 - v0;` and
  `return :> ISQ::length[*] = xs;` used to be rejected with `expected '{' or ';' after return
  parameter` followed by a cascade of `expected a body member`; they now declare the result
  parameter with its subsetting, as the pilot implementation reads them. A `return :>` that
  names no target is reported once, without a cascade.
- **A calculation's result may be identified by a short name.** `return <r> result : Real = x;`,
  `return <r> :> ISQ::length = x;`, `return <r> = x;` and `return <r>;` used to be refused as a
  return expression; they now declare the result parameter under its short name, as the pilot
  implementation reads them.
- **A calculation's result may open with a multiplicity, or carry a body right after its name.**
  `return [*] = xs;`, `return [*] :> xs;`, `return r { doc /* … */ }` and `return <r> { … }` used to
  be refused as a return expression; they now declare the result parameter, as the pilot
  implementation reads them. A `return` followed only by a value or a body (`return = e;`,
  `return { … }`) declares nothing and is reported once, without a cascade. A kind-less member
  followed directly by a body (`twice { doc /* … */ }`) in a calculation or constraint body is
  likewise the declaration it is, not a trailing expression.
