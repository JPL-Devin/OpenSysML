- **Document queries read quantity-valued attributes.** `attribute :>> mass = 5840 [kg];` —
  the form every Apollo 11 component uses — was refused by `Project`, `WhereFeature`, `OrderBy`
  and `Column` expressions with `cannot evaluate feature mass`, because the constant folder
  behind them knew literals but not the quantity operator. Quantities and the constant
  expressions over them (`2 [kg] * 3`) now fold in semantics into a value that keeps its
  magnitude, the unit the model spelt and the reduced unit term; the runtime shares that
  representation instead of owning its own. Cells render as the REPL prints them
  (`2290000 [kg]`) in `-run-query` listings, Markdown and HTML tables (the HTML span also
  carries `data-magnitude` and `data-unit`) and over gRPC as a `quantity` `DocumentValue`.
  `WhereFeature` compares a bare threshold against the magnitude in the attribute's own unit;
  `OrderBy` converts commensurable units (`500000 [g]` sorts below `119000 [kg]`) and refuses
  different dimensions with an `invalid-order` error naming both units; column arithmetic keeps
  and composes units (`mass / length` is `[kg/m]`) and refuses incommensurable operands with a
  `column-incommensurable` error naming the column and row. An attribute whose value is not a
  constant (`mass = dryMass + propellantMass`) stays a typed `unevaluable-feature` error, now
  naming the row element too.
