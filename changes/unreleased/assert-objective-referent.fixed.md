- **An `assert` or `satisfy` may reference a case's objective.** `assert obj;`, `assert not
  uc.obj;`, `satisfy uc.obj;` and `requirement r :> uc.obj;` used to report `assert target must be
  a constraint usage, found partUsage`: an objective is a requirement usage and is now judged as
  one, directly, negated, through a feature chain, through an alias and
  inside a constraint body. A `subject`, `actor` or `stakeholder` referent stays a part and stays
  rejected, as the pinned pilot rejects it.
- **A feature chain through the asserting usage's own owner names the owner's member.** An
  assertion borrows the name of the feature it references, so `part h : H { assert h.q; }` used
  to resolve `h.q` to the assertion itself and accept it; it now reaches `H::q` and reports
  `assert target must be a constraint usage, found partUsage`, where the pinned pilot reports
  `Must reference a constraint.`
