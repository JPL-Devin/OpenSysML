- **A control node's successions are validated statically.** The nine SysML v2 constraints on
  `ControlNode`, `ForkNode`, `JoinNode`, `MergeNode` and `DecisionNode` (§8.3.17) are now
  errors at validation time rather than a runtime failure or silence: a fork or decision with
  two incoming successions, a join or merge with two outgoing, a succession end whose written
  multiplicity is not the `1..1` every control node requires (`0..1` into a merge or out of a
  decision), and a control node declared outside an action definition or usage. Successions
  are counted however they are written — `first a then b;`, a member-attached `then b;`, a
  `succession s first a then b;`, a guarded or default branch out of a decision — and include
  those an action inherits from the definition it specializes, with a redefinition replacing
  the succession it redefines; a `connect`, `bind` or `flow` is not a succession and does not
  count. Each diagnostic names the node and the count or multiplicity it found and says what
  the rule requires; the runtime keeps its own structural checks and their timing. The pinned
  pilot implements only the owning-type rule, so the other eight are refereed against the
  specification and recorded as pilot gaps.
  A constraint body now parses the action statements the specification's calculation body
  allows (`assign`, `if`, loops, `send`), so a control node inside one reaches the rule; checking
  or solving a constraint that states such a statement refuses — `statement in a constraint body
  is not executed by OpenSysML` — rather than reporting a verdict that ignored it.
