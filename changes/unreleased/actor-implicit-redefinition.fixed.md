- **An actor bound without `:>>` is held to the actor it inherits.** A requirement or
  objective usage's `actor pilots = pilot;` did not redefine the definition's `actor pilots :
  Pilot[2]` — only `actor :>> pilots = pilot;` did — so one pilot, or a buoy, passed unchecked. An
  actor or stakeholder now implicitly redefines the general's actor at its position, as a subject
  redefines the subject and a parameter the parameter it follows (KerML §7.4.7.3), keeping or not
  the inherited name: it takes that actor's type and multiplicity, so one pilot is refused as
  `actor binding: multiplicity violation: 1 value(s) bound to a feature with multiplicity lower
  bound 2`, two buoys as a `type mismatch`, and two pilots satisfy the requirement; in an
  objective the refusal leaves the verdict `undecided` with that detail. An explicit `:>>` binding
  is unchanged. The redefined actor contributes its members and conformance wherever the
  redefining one is read, in the language server and the passes as at run time.
