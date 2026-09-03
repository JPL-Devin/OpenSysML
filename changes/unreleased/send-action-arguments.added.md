- **A send's arguments are validated.** The payload, `via` and `to` arguments of a send were
  never typed, so `send Sig() to target` with `attribute def Sig` — an invocation of something
  that is not a behavior, which the pinned OMG pilot rejects with `Must invoke a behavior or a
  behavioral feature` — passed silently. A new type-tier pass infers every send argument, in
  action bodies, state entry/do/exit actions, transition effects and nested forms alike, and
  reports the SysML v2 `SendActionUsage` constraints on them: a state subaction or transition
  effect that sends no payload is an error (`send-payload-missing`), sending `to` a port warns
  that `via` is the routing form (`send-to-port`), and a `via` or `to` argument whose types are
  disjoint from `Occurrence` warns at the argument (`send-sender-not-occurrence`,
  `send-receiver-not-occurrence`). The pass is in the shared registry, so the LSP reports the
  same diagnostics. Two refereed cases join the rejection corpus under
  `cmd/pilot-reject/testdata/negative/semantic/`.
- **`send new Def(args)` constructs the message it sends.** The notation's constructor keeps its
  named arguments (`send new Telemetry(frames = 3.0) via antenna;`) through the AST, the RDF
  export and the runtime, which builds the message from the constructed definition and its
  positional or named arguments. An accept binds the constructed occurrence, whatever the
  argument count: `accept d : Data` of a `send new Data(7)` binds a `Data` whose first feature
  is 7, so read `d.value`, not `d`. An accept subsetting a
  declared event (`accept :> shutDown`) now takes a message sent from that event feature
  (`send shutDown to interrupt`), not only one of its type.
- **A constructor's arguments are checked against the type it instantiates.** `new T(…)` binds
  the type's features — its own first, then the inherited ones — by position or by label, and the
  type tier now reports a positional argument beyond them, a label bound twice, a qualified label
  naming another type's feature, and an argument whose scalar type cannot bind its feature, each at
  the offending argument. A simple label resolves as a feature of the constructed type rather than
  of the surrounding scope, so an unknown one is reported where it is written and renaming the
  feature rewrites its labels.
