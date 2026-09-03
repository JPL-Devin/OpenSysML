- **An interface, connection, flow, allocation, binding or succession usage keeps the members
  of its body in RDF.** `sysml -convert ttl` used to fold the whole declaration of a usage whose
  head binds ends (`interface seam connect w.outp to r.inp { attribute coupling : C = C::x; }`)
  into its `sysx:sourceText`, so `coupling` had no node of its own — no `sysml:ownedMember`, no
  `sysml:ownedFeature`, nothing to query — and a graph without the text came back without the
  body. The same held for a `first a then b { … }` or `then b { … }` in an action body. The text
  now carries the head alone, and the body's members convert like those of any part or interface
  definition body: each an element with its name, type, value, `sysx:memberIndex` and membership,
  written back from the structure whether or not the graph carries the text
  (Open-MBEE/OpenSysML#89).
