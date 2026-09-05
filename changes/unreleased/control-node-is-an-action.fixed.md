- **A control node's members are an action's.** `fork f { attribute a : Integer := 1; }` (and
  `join`, `merge`, `decide`) used to report `Initialized feature must be variable`: the node
  had no implicit base, so nothing made it an occurrence. A control node now specializes its
  control action (`Actions::ForkAction`, `JoinAction`, `MergeAction`, `DecisionAction`), so its
  members are an occurrence's and may be variable.
