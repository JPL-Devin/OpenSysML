- **Each nested action node performs in a frame of its own.** An action node is a performance
  (`Actions::Action :> Performance`, `subactions :> subperformances`), so the parameters and
  attributes it declares, and those of the action it performs, now live in a frame the node's
  performance holds rather than in the enclosing action's one feature space. Two nodes each
  declaring `out v` no longer overwrite each other; `assign total := p.v + q.v;` reads each
  node's pin, and so does `leg.inner.v` through two levels; `bind add.a = x;` and
  `flow p.v to q.w;` address the pins they name; a typed node's body-local `in a = 3;` seeds its
  own input; `action add = Adder(3, 4)` and `Adder(a = 3, b = 4)` bind the callee's inputs by the
  callee's own parameter order and names, inherited and redefined parameters in their effective
  order — never by what the caller happens to name alike — and the untyped `add` read as a value
  is the callee's `return` parameter, or its `out result`. The callee binds the supplied inputs
  before it evaluates its defaults in declaration order, so `in b : Integer = a * 2;` reads the
  `a` the caller passed; likewise a `bind` at a node's input pin takes precedence over the value
  the node's own declaration states. An invocation's arguments are the enclosing action's
  expressions, so `Adder(a, 1)` reads the caller's `a` even when the node's own pin `a` already
  holds a bound value. A binding at an undirected attribute of a node is kept at both ends: the
  node reads the other end's value as it begins, and what it changed is carried back as it ends,
  to the enclosing attribute or on to a downstream node's pin. A binding end that chains through
  an object, `bind add.sum = holder.inner.mark`, writes the feature of the object the chain
  reaches, typed as an assignment through it is. A performance's bindings and
  defaults are one evaluation of their own: a calc usage two of them read answers once, and the
  next performance of the node evaluates it anew. Two tokens performing one node at once each hold a frame of their own,
  take what flows delivered to its pins oldest first and send their own outputs on. A nested
  body still resolves a name it does not declare lexically to the enclosing action's feature and
  writes it in place, so a grandchild writing `legs` keeps working, and a perform usage on a
  part keeps its occurrence slot. Reading a pin before its node has run, a pin the node does not
  declare, a surplus, missing, unknown or repeated argument, a binding at a non-parameter or
  into a feature no enclosing action holds, and two bindings at one input pin whose other ends
  disagree are typed errors
  (`ErrNodeNotPerformed`, `ErrNodePin`, `ErrActionArity`, `ErrUnboundParameter`,
  `ErrUnknownParameter`, `ErrDuplicateArgument`, `ErrBindingEnd`, `ErrBindingConflict`). `Results()`, the REPL's
  `%continue`/`%tokens` and a gRPC execution response report a node's pins under its path
  (`p.v`); `Data()` stays the action's own performance. Kept for compatibility: a bare typed usage `action call : Callee;`
  still reads an unbound `in` from the same-named enclosing feature — `Callee()` passes nothing
  and lets the callee's defaults apply — and every invocation form still returns its `out`
  values into same-named enclosing features that exist. Not yet: `n.pin` on an untyped
  `action n = Callee(args)` is refused by name resolution, which does not type `n` by the
  invocation; read `n` itself, or type the usage. An action declared in an `if` branch or a
  loop body is a performance of its own like any other node, with the block's locals (a loop
  variable) in reach, so a sibling in the branch reads its pins as `p.v` and `Results()`
  reports them under its path (`iterate.square.s`), the latest iteration's standing for it; a
  `bind` or `flow` written in the branch or body at such a node's pin is applied per
  performance (`bind dbl.a = i` seeds each iteration's node from its loop variable) where
  before it failed as a statement a body cannot run; where both branches declare a `p`, a read
  of `p.v` in a branch is of that branch's node. A typed or invoked node adopts the subactions
  of the action it performed, so `call.inner.v` reads a pin inside the callee through it and
  `Results()` reports it under `call.inner.v`. A node in a state's entry/do/exit or transition
  body performs in a frame of its own as one in an action body does, with the machine's data and
  the enclosing states' attributes in reach, so a sibling reads its pins as `p.v`, two nodes'
  same-named pins keep apart, and a `bind` or `flow` the body states at its pins — from a state
  attribute into a pin, between two nodes' pins, or an output back to a state attribute — is
  applied; before, such a connector was reported as a statement a body cannot run and `p.v`
  found no `p`. A typed action node in a branch or loop of a
  state's entry/do/exit body performs the action it names with the `in` values and arguments
  it states, holds every pin of the callee as it ended (an argument overriding the node's own
  default included) for its body to read, and returns its `out` values to a same-named block
  local, state attribute or state datum that exists — before, the node's `in x = i` and body
  statements were skipped and the callee saw only state data. A pin such a node in a state's or a
  calc's body declares without a value (`out v : Integer;`) is the node's own too: its body's
  `assign v := 1` writes that pin, not a same-named state attribute or calc local — before, the
  write reached the attribute, or was refused in a calc as a name it never declared. In an action
  body as in a state's,
  those `out` values return once the node's own body has run, so a body that rewrites an output
  returns what it wrote rather than what the callee produced. A typed or invoked node a derived
  action inherits resolves its callee where the node was declared, so one visible only to the
  general action is found rather than reported unresolved, and a `bind` or `flow` the general
  action states at that node's pins applies to the derived action's performance of it, in the
  general action's scope — before, only the derived action's own connectors were lowered and the
  inherited node ran with its input unbound. Such a connector follows the node's declaration: a
  node the derived action declares of its own under the inherited node's name does not take it
  (the connector lowers to nothing, and the replacement's same-named pin stays unbound), while
  a node redefining the inherited one (`action add :>> add`) does. A binding between two of the
  general action's nodes holds at both or at neither: when the derived action replaces the node
  at one end (`bind add.a = src.n` with a `src` of its own), the other end no longer reads the
  replacement's pin by name. An action declared in a branch or loop body that
  states a flow of its own (`first`, successions, forks and joins among its nodes) now runs that
  flow to completion in its frame, and its steps spend the action's own token-flow budget
  (`OPENSYSML_MAX_ACTION_STEPS`, `ErrActionStepLimitExceeded`) as the enclosing flow's do;
  before, its `first` was reported as not executable. The
  arguments of `action n = Callee(a = 3) { in a = ...; }` bind the node's pins before its own
  defaults are evaluated, so the default an argument replaces is never evaluated and one the
  node keeps (`in b = a * 10;`) reads the argument — before, the replaced default ran first and
  its failure was reported. A
  `perform` in statement form and a
  state's entry/do/exit action now refuse an `in` without a default that nothing binds
  (`ErrUnboundParameter`) instead of failing later inside the callee. A
  default an action inherits from a generalization (`action def Derived :> Base` with Base's
  `in x : Integer = 3;`) is seeded when the performance starts, as an owned default is, evaluated
  where it was declared and reading a parameter the action redefines as redefined — before the
  fix it was recomputed on every read, so a body that reassigned `x` saw `z = x * 2` change with
  it, and the inherited feature never appeared in `Results()`. A `bind` at a pin two levels
  down, `bind leg.inner.w = x;`, is lowered with the whole path it names, so it seeds `inner`'s
  `w` and not a `w` of `leg`; before, the path collapsed to `leg.w`. A binding between a nested
  pin and a pin of the node around it or of another node under it, `bind leg.inner.v = leg.v` or
  `bind leg.inner.v = leg.rest.n`, holds within the one performance of `leg` the nested node
  runs in: `leg.v` takes `inner`'s value as `inner` ends, and two tokens performing `leg` at
  once each hand their own `inner`'s value to their own `rest` — before, the value went to the
  latest performance of `leg`, or was queued for one yet to come, so `leg.v` ended unvalued and
  concurrent performances swapped values. A debugger breakpoint on a node an `if` branch or a loop body declares now pauses the run before the node performs, once
  per performance, and `%step`/`Step` resumes it; `NodeNames()` lists such nodes, so `%break add`
  on a loop body's node is accepted — before, the name was refused and the node never paused.
  Such a pause ends the step at once: a token forked alongside the paused one takes no step of
  its own in that call, and is stepped first by the next one. A REPL session ended while paused
  there — by `%stop`, another `%action`, or a redeclaration of what it debugs — releases its
  executor (`ActionExecutor.Release()`), ending the paused work rather than holding it suspended.
