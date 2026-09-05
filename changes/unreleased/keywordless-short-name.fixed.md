- **A kind-less feature may be identified by a short name.** `<a> alpha = 1;`, `<b> :> alpha = 2;`
  and `<t> twice = n * 2;` used to be rejected with `expected a namespace member` or `expected a
  body member` in every namespace, definition body, calculation body and nested statement body;
  they now declare the feature under its short name (SysML `DefaultReferenceUsage` over an
  `Identification`), as the pilot implementation reads them. A keyword of the other language is
  an ordinary name there too: `<chains> links = 3;` and `<s> featured = 1;` in a `.sysml` file,
  `<s> part = 1;` and `<attribute> y = 2;` in a `.kerml` file.
