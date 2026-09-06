- **A name written ahead of a kind keyword is a syntax error, not a renamed member.** `part def B
  { foo attribute bar : A; }` was accepted silently as an attribute named `foo`, and the declared
  name `bar` was dropped; likewise `x part def P;` became a definition named `x`. Neither the SysML
  nor the KerML grammar has such a form — a name always follows its keyword — and the reference
  implementation rejects it. The stray name is now reported (`expected a body member`, or `expected
  a namespace member` at package level), skipped, and the members after it still parse.
