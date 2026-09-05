- **Resolving a name through a wildcard import costs the matches, not the namespace.** Each
  unqualified name reaching `import ISQ::*` or another large library namespace used to be
  compared against every member the import surfaces, so a model leaning on the quantity and
  unit libraries spent much of its load time in that scan. The index now answers a wildcard
  import's members by name; the public Apollo 11 model (28 files, 7.2k lines) validates in
  about 0.33 s instead of 0.54 s, allocating 168 MiB instead of 218 MiB, with identical
  diagnostics.
