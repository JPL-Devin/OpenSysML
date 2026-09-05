- **KerML `const` is a variable feature.** `const feature k : C;` is a variable feature in KerML,
  so `portion const feature` now reports `A portion cannot be variable` and a `const feature` owned
  by a package or by a non-occurrence type reports `Must be owned by an occurrence type`, as
  `var feature` already did and as the pinned pilot reports.
- **`var feature` parses at namespace level.** A KerML `var` prefix on a package or root member
  used to stop the parser with `expected a namespace member`; the declaration now parses and the
  owner rule reports it.
