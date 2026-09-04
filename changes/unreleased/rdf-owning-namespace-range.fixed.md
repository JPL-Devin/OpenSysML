- **`sysml:owningNamespace` names a namespace only.** An element a relationship owns — a
  state's entry action, a `#M` prefix on a dependency or a subject — stated the relationship as
  its `sysml:owningNamespace`, outside the property's range. It now states `sysml:owner` and the
  membership wiring alone, as the metamodel does; `sysml:owningNamespace` is written for an
  element a namespace owns, as before.
