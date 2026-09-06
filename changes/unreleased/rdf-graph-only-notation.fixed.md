- **Notation written from an RDF graph alone parses again for connectors, guarded successions
  and nested expressions.** With `sysx:sourceText` stripped, `sysml -convert sysml|kerml` wrote
  an anonymous connector's own multiplicity after its last end (`succession first a then b[n]`),
  where the grammar reads it as the end's; it is now written in the declaration slot
  (`succession [n] first a then b`, `bind [1] a = [1] b`), and a KerML `binding`/`succession`
  with a connector multiplicity always writes `of`/`first` (`binding [1] of a = b`), since
  without the verb the grammar gives the multiplicity to the first end.
- **The expression writer places parentheses by the parser's precedence table.** A
  conditional or any other loosely binding form used as an operand of a tighter operator is
  parenthesized (`size(ae) == (if isEmpty(af) ? 0 else 2) and …`, `(p ?? q) implies r`,
  `(a + b)[1]`, `(x as T).f`, `not (p and q)`), and redundant parentheses are no longer written
  around every operator (`x * 2`, `if x > 0 ? x else - x`); before, a nested conditional came
  back bare and did not parse, while every other operator was wrapped unconditionally.
- **A guarded succession records its syntax from the AST, not from the words ahead of it.**
  `public succession S first A1 if x == 0 then A2;` recorded the bare-source transition form
  because `first` was not among the first tokens, and came back as `transition S A1 if …`,
  which does not parse; the encoder now derives the form from where the AST places the source
  and keeps the `succession` keyword, and the decoder writes a named transition or succession
  with `first` and its visibility.
