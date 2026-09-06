- **A KerML `binding`/`succession` written without `of`/`first` puts a leading multiplicity on
  its first end.** `binding [1] a = [1] b;` and `succession [1] a then [*] b;` declare no
  connector of their own, so the grammar reads `[1]` as the first end's crossing multiplicity;
  the parser used to record it as the connector's multiplicity, which the RDF mapping then wrote
  back behind the second end and could not read again. Both ends now carry their multiplicities,
  the connector carries none, the RDF graph states each end's bounds on the end node, and the
  Kernel Semantic Library's `Occurrences.kerml` written back from its graph alone keeps every such
  site as written. `binding [1] of a = b;` and `succession [1] first a then b;` still give `[1]`
  to the connector.
- **A parameter's specializations may follow its multiplicity.** `in x : Integer[1] redefines
  A::x;`, `in y : Integer[1] :>> A::x;` and `return : Integer[1] ordered :>> C::r;` are accepted
  for `in`, `out`, `inout` and `return` parameters, with `ordered`/`nonunique` between, as
  `FeatureSpecializationPart` allows on any feature; they were reported `expected ';' or '{'
  after parameter`. The parameter path now shares the ordinary usage's specialization loop.
