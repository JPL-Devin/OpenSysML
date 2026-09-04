- **A member is rejected outside the body kind that owns it.** `subject` and `actor` belong to a
  requirement or case body, `stakeholder` to a requirement body, `objective` to a case body,
  `entry`, `do` and `exit` to a state body and `render` to a view body (SysML v2 `RequirementBody`,
  `CaseBody`, `StateBody` and `ViewBody`). Written anywhere else — a part, an action, a package, a
  nested usage of another kind — the parser now reports an error naming the owning body and the
  fix (`'actor' declares an actor of a requirement or case and is only allowed in a requirement or
  case body; move it into the requirement or case it belongs to`) where it used to accept the
  member silently, or, for `entry action init;`, read it as a plain action. The OMG pilot rejects
  the same models at the same tier. Every legitimate state form is unchanged, including `entry;`,
  `entry; then s;`, inline and braced entry/do/exit actions, transitions and nested or parallel
  states, and a member inside an `include`, `perform`, `exhibit`, `frame` or `satisfy` body is
  judged by that body's kind.
