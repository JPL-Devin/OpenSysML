- **Overriding a binding feature value is an error.** A value written with `=` is a binding,
  fixed for every feature that redefines the valued feature; only a `default =` value may be
  overridden (KerML 8.3.4.10.2 `validateFeatureValueOverriding`). `attribute :>> a = 2;` under an
  `attribute a : Integer = 1;` — on a redefining usage, a specializing definition, or through a
  chain of redefinitions, including the implicit redefinition of a parameter, connector end or
  case subject — now reports `cannot override the binding value of …` at the value, naming the
  bound feature and the fix (`default =` on it, or no value here), at the positions the pinned
  pilot reports. A `:=` initial value over a binding is reported the same way; a `:=` on a
  non-variable feature is a separate rule and unchanged. The parser records the value operator
  (`=`, `:=`, `default =`, `default :=`) on usages, subjects and named `assume`/`require
  constraint` declarations so the rule can tell them apart, and the RDF mapping carries it as
  `sysml:isDefault` and `sysml:isInitial` beside `sysml:value`, so a round trip no longer turns
  an overridable `default = 1` into the binding `= 1`; a graph stating either flag without a
  `sysml:value`, or stating one both true and false, is refused rather than read as one value.
  A named `require constraint c : C = c0;` or `assume constraint a : C` is now a member of its
  requirement: `:>> c` in a specializing requirement resolves, is checked by this rule and by the
  constraint-usage declaration rules, is found by the language server, and is checked at runtime
  by qualified name and through its requirement like any constraint usage, a redefinition owning a
  result expression replacing the one it redefines and one owning none inheriting it. Examples,
  fixtures and guide snippets that overrode a bound attribute now declare the base value as
  `default =`; the solver's `attribute :>> best = <expression>` objective over the library's bound
  `TradeStudies` `best` is reported and recorded as a known gap until that contract is restated.
