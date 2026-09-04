- **`send Def(args)` on an item or attribute definition is an error, as the specification and
  the pinned pilot say.** The runtime used to read that invocation as "send an instance of
  `Def`", the shape the conformance fixtures and the relay-probe demo were written in; KerML's
  `validateInvocationExpressionInstantiatedType` allows an invocation only of a behavior or a
  behavioral feature. Write the constructor instead: `send new Def(args)`. The fixtures, the
  demo and the examples are migrated; invoking a behavioral feature (`send shutDown() to self`
  over an action) is unchanged.
