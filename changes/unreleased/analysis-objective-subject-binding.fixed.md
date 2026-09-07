- **An analysis objective binds its requirement's subject by keyword alone.** An objective typed by
  a requirement definition (`objective : MassLimit { subject = ship; }`) was always `undecided: no
  value for feature s`, because only the requirement engine knew the keyword-only form. An
  objective's own members are now bound the way `-requirement` binds a requirement usage's, so
  `subject = ship;`, `subject s = ship;` and `subject :>> s = ship;` all decide the verdict, in a
  definition, a usage and through a nested analysis step, and the binding may read the case's steps'
  outputs. An objective that binds no subject takes the library's default, the case's result
  (`Cases::Case::obj` declares `subject subj default Case::result`); a result of the wrong type is
  `undecided` saying so (`subject s defaults to the case's result (Cases::Case::obj): type
  mismatch: 1000.0 (a Real) is not a Ship`), and a case returning none says to bind it or return one.
- **A recursive analysis step reports one line, not one per frame.** An analysis performing itself
  as a nested step, or a `calc def` recursing through its own `calc` usage member, hit the recursion
  limit with a message repeating `node again:` ten thousand times (hundreds of kilobytes). Those
  frames now collapse as a calc calling itself does — `analysis An::Rec::again: … 9999 frames:` —
  keeping the typed error and the hint to raise `OPENSYSML_MAX_CALC_DEPTH`.
