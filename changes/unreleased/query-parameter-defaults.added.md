- **A document-query parameter default is used when the caller leaves it unbound.** A query
  could declare `in root : Element = telescope;` or `in pattern : String default "m";`, but
  the plan recorded only that a default existed and execution refused the unbound parameter
  with `relies on a default not retained in the plan`. The plan now retains each default as a
  compiled expression together with the query that declared it, under the rule `%run-query`
  already applied to bindings: a default naming a model element binds that element, any other
  default is evaluated — once per query execution, before any row is produced, in the scope
  of the query that declared it, within the same visit and invocation budgets as the query
  body. Inherited defaults apply, a redefining parameter's default replaces the inherited one,
  the value passes the same type and multiplicity checks as an explicit binding, and an
  explicit binding still overrides. `%run-query`, `-run-query`, `RunDocumentQuery`,
  a document content block that leaves the parameter unbound, and a query invoked by another
  query all take the default through the one executor. A default written in a form the query
  expression language has no operation for is refused when the query is planned
  (`document-query-unsupported-default`, naming the parameter) rather than at execution;
  the `default-unavailable` execution failure no longer exists.

- **A query expression may name a model element wherever a value is expected.** A name in
  a default, a list, or an operation or named-query argument that is not one of the query's
  parameters now binds the element it denotes — `OwnedElements(source = telescope)` starts
  from that part, in a default or in the query body alike — where planning previously refused
  it as an unknown parameter. The element is checked against the receiving parameter's type
  and multiplicity when the query is planned — an element is never a data value, so an
  attribute named where a `String`, a `ScalarValue` or any `attribute def` is due is refused
  rather than passed by name, while an enumeration literal (`Color::red`) is a value of its
  enumeration and binds an `enum def`-typed parameter — with the same typed `argument-type` and
  `argument-multiplicity` failures a mismatched parameter reference gets.
