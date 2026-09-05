- **A feature whose value calls a function on itself types in finite time.** A rollup such as
  `attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);` — the value of
  `totalMass` calls `sum` on `totalMass` itself — sent the type checker round the same call
  without end, and `sysml -validate` died with a stack overflow on the Apollo 11 model. Typing an
  argument that leads back to the call being typed now selects that call on its argument count
  alone, once, so the model validates and reports its diagnostics.
- **An interface whose ends are named like the parts it connects validates.** With
  `interface def I { end plss : P; end psa : ~P; }` and
  `interface x : I connect plss.port to psa.port;` inside a part with parts `plss` and `psa`,
  the accessibility rule resolved each end against the interface's own inherited ends rather than
  the enclosing part's, and reported `Must be an accessible feature` on a legal model. Ends now
  resolve in the enclosing scope, as name resolution already did.
- **A state definition written `:> StateAction` instantiates.** Spelling out the specialization
  every state definition has implicitly made lowering materialize the library's content, whose
  `ref state self : StateAction` led back to `StateAction`, so `-instantiate` of a part exhibiting
  such states failed with `recursive state typing`. A library type now contributes no content to
  a state machine, as the implicit specialization never did.
