- **Metadata annotations convert to RDF structurally, bodies and prefixes included.** The
  encoder used to carry a `#Safety` prefix as the text it was written as and to refuse an
  annotation with a body (`@Safety { level = 2; }`), the most common refusal across the corpus.
  Every annotation — `@M;`, `@M { … }`, `metadata m : M about a, b;` and the prefix
  `#M part def P;` — is now a `sysml:MetadataUsage` owned by the element it is written in or
  ahead of, carrying its type, one `sysml:annotatedElement` per target, `sysx:hasBody`, the sigil
  it was written with as `sysx:declaredKeyword` (`@`, `#`, or none for `metadata`) and its
  body's members as owned members with their `sysml:value` expression trees, so the notation is
  written back from the graph alone, without `sysx:sourceText`. `sysx:prefixMetadata` and
  `sysml:annotates` are gone; a `.ttl` file written by 0.4.3 that states either is refused naming
  the property rather than read without the annotation — re-export it from its notation source.
  Of the 19 files refused for this reason, 17 now convert and 13 of those come back as an equal
  graph without `sysx:sourceText`; the other four run into older gaps the refusal had hidden
  (an `isEnd` and an `isNamespaceImport` flag the writer drops, an invocation expression it
  refuses, an n-ary `connect (…)` head with no `sysx:endForm`), and the remaining two fall to
  an older refusal, an unnamed `feature` or `event` declaration. The parser now reads
  `metadata M about x;` as typed by `M` and unnamed, as the grammar's
  `MetadataUsageDeclaration` requires, rather than naming the usage `M` — which had made the
  training corpus's `Metadata Example-1.sysml` a duplicate declaration under conversion
  (`docs/reference/rdf-mapping.md`).
- **An `assume`/`require` member's constraint declaration converts to RDF.** The encoder carried
  only the condition of a requirement's `assume`/`require` members, so
  `assume constraint c : C;` and `require constraint d [1] = true;` came back as
  `assume constraint { }` and `require constraint { }`. The name, specializations, multiplicity
  and value of the constraint usage the member owns are now carried as they are for any usage,
  and a body-less member comes back with its `;` rather than an empty body.
