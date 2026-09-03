- **A literal of the wrong datatype is no longer read as a name.** The Turtle reader took every
  literal by its lexical form, so `sysml:declaredName "3"^^xsd:integer` or a `sysx:bodyParameter`
  stated as `"x"@en` came back as the name `3` or `x`. Every metamodel property the mapping
  reads as text is a `String`; a language-tagged literal, or one whose datatype its property does
  not take, is now refused before anything is read, naming the literal and the subject that
  states it. Plain and `xsd:string` literals read as before, and each property takes the
  datatypes the ontology gives it: `xsd:boolean` for a flag, `xsd:integer` or `xsd:int` for an
  index or a `LiteralInteger`, `xsd:decimal`, `owl:real`, `xsd:double` or `xsd:float` for a
  `LiteralRational`. The text is checked against the datatype's lexical space too, so
  `"false"^^xsd:int` or `"yes"^^xsd:boolean` is refused rather than read as the text it spells.
  A `sysx:` index that is negative or too large for `int` is refused too, where it was read as 0
  and moved its member to the front, and so is a subject stating a single-valued `sysx:`
  property twice (a body with two `sysx:resultExpression`s), where the first was kept and the
  rest dropped. A subject stating several `rdf:type`s is read as the one that is a subclass of all
  the others, whichever is written first, where the first one was read; a set of classes with no
  such member is refused naming the subject. A property a known metaclass does not declare, such
  as `sysml:value` on an `AttributeUsage`, takes the datatypes this mapping writes there rather
  than the union of every metaclass declaring the name, so `"2"^^xsd:integer` is refused as a
  feature's value where it was read as expression text.
- **A result expression rebuilt from its graph is spelled as the grammar requires.** A
  `sysml:LiteralString` whose value holds a quote, a backslash or a line break is written back
  as the escaped string token; a `LiteralRational` value is written as a real token (`"3"^^xsd:decimal`
  comes back as `3.0`) and a `LiteralBoolean` as `true` or `false`; a value no token spells — a
  signed number, `INF`, `NaN` — is refused naming the node instead of becoming a name. A real
  literal with an exponent is now written as `xsd:double`, whose lexical space holds it, where
  `xsd:decimal`'s does not. An empty expression body `{}` states `sysx:hasBody` so it comes
  back without its `sysx:sourceText`, and a named invocation argument whose name is not a basic
  name (`f('the value' = x)`) keeps its quotes.
