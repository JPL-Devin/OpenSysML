- **A literal reference must be a name.** A graph stating a reference the graph does not define —
  a metadata usage's `sysml:type`, a specialization, an `about` target — as a literal that is no
  name (a number, a boolean, a language-tagged or expression-typed string, an empty or broken
  qualified name) was written into the notation as it stood, as `@42`. `sysml -convert` now refuses
  it with an error naming the element and the literal; a plain string spelling a qualified name is
  written as before.
