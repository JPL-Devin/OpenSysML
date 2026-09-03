- **`%run-query` and `-run-query` accept a value of a type the session declares.** The REPL
  reads names from the document's own scope tree while the runtime model reads the index, so
  a parameter typed by a `part def` or `enum def` of the loaded model refused that model's
  own elements and literals (`binding site has type element, expected Site::Telescope`).
  Type conformance now compares symbols as elements, so one declaration reached through
  either tree conforms to itself and to its supertypes whichever tree those came through.
