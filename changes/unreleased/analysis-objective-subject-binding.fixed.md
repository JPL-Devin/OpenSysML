- **An analysis objective binds its requirement's subject by keyword alone.** An objective typed by
  a requirement definition (`objective : MassLimit { subject = ship; }`) was always `undecided: no
  value for feature s`, because only the requirement engine knew the keyword-only form. An
  objective's own members are now bound the way `-requirement` binds a requirement usage's, so
  `subject = ship;`, `subject s = ship;` and `subject :>> s = ship;` all decide the verdict, in a
  definition, a usage and through a nested analysis step, and the binding may read the case's steps'
  outputs, a nested case's or an action's (`subject = weigh.m;`), as may an `assert constraint` of
  the body. An objective that binds no subject takes the library's default, the case's result
  (`Cases::Case::obj` declares `subject subj default Case::result`); a result of the wrong type is
  `undecided` saying so (`subject s defaults to the case's result (Cases::Case::obj): type
  mismatch: 1000.0 (a Real) is not a Ship`), one of the wrong multiplicity (one `Ship` for a
  `Ship[2]` subject) is `undecided` as a multiplicity violation — an objective redeclaring the
  subject without one (`subject :>> pair;`) keeps the `[2]` — and a case returning none says to
  bind it or return one. An object bound to a subject, by the default or by an expression, in an
  objective or a requirement usage, is held as a value of it: a `Ship` bound to a `subject t :
  Tanker` gains `Tanker`'s features, so `t.cargo` answers where it read `member cargo not found`,
  and a value the subject cannot hold at all is refused as a `type mismatch` instead; an expression
  yielding more or fewer values than the subject declares (one `Ship` for `Ship[2]`, or none) is
  refused as a multiplicity violation, as the default already was. The object a satisfaction
  assertion supplies with `by` is held to the subject the same way: `satisfy laden by ship` reads
  `t.cargo` of a `Ship` bound to a `Tanker` subject, and a `Buoy` supplied for a `Ship` is refused
  (`subject: type mismatch: Buoy #1 (buoy) is not a Ship`) rather than checked.
- **A case's result is readable by its qualified name.** `MassCase::result` — the form the OMG
  examples use, `objective : MassAnalysisObjective { subject = MassAnalysisCase::result; }` — read as
  an empty sequence when the case's result was unnamed (a trailing expression or `return : Real`),
  leaving the objective `undecided: comparison operands must be constants`. A qualified feature the
  running case declares, or inherits from the library (`Cases::Case::result`), now reads the run's
  binding for it: in the objective's subject, in an `assert constraint` of the body, and as
  `inner.result` from the case performing `inner` as a step.
- **A recursive analysis step reports one line, not one per frame.** An analysis performing itself
  as a nested step, or a `calc def` recursing through its own `calc` usage member, hit the recursion
  limit with a message repeating `node again:` ten thousand times (hundreds of kilobytes). Those
  frames now collapse as a calc calling itself does — `analysis An::Rec::again: … 9999 frames:` —
  keeping the typed error and the hint to raise `OPENSYSML_MAX_CALC_DEPTH`.
