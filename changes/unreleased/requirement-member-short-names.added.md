- **A requirement's `subject`, `assume constraint` and `require constraint` members take a
  short name.** `subject <s> x : T;`, `assume constraint <a> ac : C;`, `require constraint <r> : C;`
  and the other short-name-only forms used to be parse errors, although the grammar allows a
  short name wherever a name is declared. They now parse, with or without a name, a `#M` prefix,
  typing, multiplicity, a value or a body; the short name resolves (`Req::s`, `:>> s`, from an
  expression), is checked for distinguishability against the requirement's other members, is
  exported as `sysml:declaredShortName` and read back into `<s>`, and is what the REPL and the
  editor show, navigate to and rename when the member declares no other name. A malformed short
  name (`subject <> x;`, `subject <s x;`) is a diagnostic.
