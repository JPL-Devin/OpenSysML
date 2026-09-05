- **A kind-less `x = e;` or `x := e;` in a behavioral body declares a feature; an assignment
  is spelled `assign x := e;`.** In a calculation body, a constraint body, a `while`/`loop`/`for`/`if`
  body, a state's entry/`do`/exit block or a transition effect, `x = e;` used to be OpenSysML's
  own shorthand for assigning `x`. It now reads as the standard notation does — a member of the
  body declared only by its name and value (SysML.xtext `DefaultReferenceUsage`), the reading the
  pilot implementation gives it — so `calc def c { in n : Integer; twice = n * 2; twice + 1 }`
  declares a local `twice` that the trailing expression reads, and
  `assert constraint { flag = true; }` declares `flag` rather than writing it. A model that
  relied on the shorthand must write `assign x := e;`: an `x = e;` in such a body no longer
  updates an output, a local or a state attribute, and a calc whose outputs were written that
  way reports them as never assigned. The bundled fixtures have been migrated.
