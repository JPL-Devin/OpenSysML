- **A collection-valued property in a Turtle graph is also written as the JSON literal
  Flexo reads it from.** The `flexo-mms-sysmlv2` reader skips a `sysml:` predicate that has
  several objects and takes the property from the literal at
  `urn:sysmlv2:annotation:json:<key>` instead, so every `ownedMember`, `specializes`,
  `argument`, … a graph stated as bare repeated triples was silently dropped on read: the live
  harness measured 0 of 14 multi-valued standard properties delivered. The encoder now keeps the
  typed triples and adds one `json:<key>` literal per collection holding the whole array in the
  shape that service's own commit path stores — `[{"@id":"…"},…]` for references, JSON
  strings, numbers and booleans for literals — in the graph's deterministic order, declared
  under a `json:` prefix (`docs/reference/rdf-mapping.md` § Collections). Reading a graph
  accepts a collection stated by the annotation alone (a Flexo-produced graph), by typed
  triples alone (a graph an earlier release wrote) or by both; two spellings that disagree, or
  an annotation that is not one literal holding a JSON array, are refused with an error naming
  the subject and the key rather than one being picked. `-sync-diff` treats the annotation as
  the restatement it is, and minting ids rewrites the references inside it with the typed ones.
  Re-recorded against the live stack, the graph-load side of the harness delivers 14 of 14
  multi-valued standard properties and 369 of 452 properties overall (was 355 of 424); every
  remaining loss is a `sysx:` property.
