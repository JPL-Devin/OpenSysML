- **Every Kernel Function Library declaration is dispatchable by name.** All 17 vendored
  packages, and `OpenSysMLMathFunctions`, are gated: each calc or function declaration —
  the operator-named ones included — either computes or names itself as unevaluable with the
  reason, so `RealFunctions::ToReal("1.5")` is `1.5` rather than "calc has no return
  expression" and `NumericalFunctions::sum0((1, 2, 3), 0)` is `6` rather than an unresolved
  `+`. New: the conversions `ToString`, `ToBoolean`, `ToInteger`, `ToNatural`, `ToRational`
  and `ToReal` of every package that declares them (a String that is not a notation of the
  type, a negative given to `ToNatural` and a value outside the Integer range are typed
  errors; `ToString` of a Real is the shortest decimal that reads back as the same Real, so
  `ToReal(ToString(x)) == x`); `RationalFunctions::floor`, `round` and `gcd`
  (`gcd(0, 0)` is `0`, a negative operand is taken by magnitude); `RealFunctions::re`, `im`
  and `arg`; `NumericalFunctions::sum0`/`product1`, which answer the identity they are given
  for an empty collection; `DataFunctions`/`ScalarFunctions::max`/`min` over numbers,
  strings and quantities; every operator as a function — `IntegerFunctions::'+'(1, 2)`,
  `DataFunctions::'=='(1, 1)`, `BaseFunctions::'#'(xs, 2)`, `ScalarFunctions::'..'(1, 3)`,
  `BooleanFunctions::'not'`/`'xor'` — each evaluated by the operator's own code, with each
  package's parameter types imposed (`IntegerFunctions::'=='(2, 2.0)` is a type mismatch where
  `BaseFunctions::'=='` answers `true`) and `NaturalFunctions::'/'` answering the Natural it
  declares (`'/'(6, 3)` is `2`; `'/'(7, 2)` is a domain error, not `3.5`); and
  `ControlFunctions::'if'`, `'and'`, `'or'`, `'implies'` and `'??'`, which evaluate only the
  operand they select and accept an omitted second operand when the first decides. Built-in
  functions bind named arguments (`sum0(zero = 0, collection = xs)`), bind null to every
  `[0..1]` parameter a call leaves out, trailing ones included (`size()` is `0`, `'if'(false)`
  null, `subsequence(seq, 2)` runs to the end), and a model's own calc
  named like a library function — a collection built-in, a conversion, an operator form or
  `sqrt` alike — is no longer answered by the implementation of that name, with or without a
  body of its own. A body
  passed on through an `expr` parameter (`Keep(xs, { in x; x > threshold })` with `Keep`
  doing `xs->select pred`) is applied in the scope it was written in, so it reads its writer's
  `threshold`, and one a control function selects is applied rather than answered as a body;
  a body a calc returns keeps the parameter it closes over after that calc has returned, and
  one that names an output of that calc (`out threshold = n; out pred : expr = { in x; x >
  threshold }; bind result = pred;`, or a usage nested in its body) still works it out from
  the invocation's own parameters once the frame that invocation ran in has been reused.
  A nonzero Real notation too small for a Real (`1e-400`, as a literal or through `ToReal`)
  is an overflow error rather than `0.0`, and only decimal notation is a Real at all (`NaN`,
  `Inf` and a hexadecimal float are invalid notation wherever a Real is read, a compiled
  calculation's command-line arguments included). A `RealFunctions`
  operator form binds an Integer argument as the Real it equals and answers a Real
  (`RealFunctions::'+'(1, 2)` is `3.0`, `RealFunctions::ToRational(2)` is `2.0`; a product too
  large for an Integer stays finite), while
  `RationalFunctions` keep an Integer's kind as their `abs`/`max`/`min` do. A direct
  invocation of a built-in through the runtime API (`InvokeCalc`, `InvokeCalcNamed`) binds
  and computes as the written call does, a body value handed to an `expr` parameter applied
  only when selected.
  `DataFunctions::'=='`/`'==='` take DataValues only: a part or other occurrence is a type
  mismatch, where `BaseFunctions::'=='` compares anything. The equality and identity forms
  hold their `[0..1]` operands to one value: an empty collection is null (`'=='((), null)` is
  `true`, as `() == null` is) and two or more values are a multiplicity violation; `??` in
  either notation falls back over an empty collection, not only over `null`.
  `BaseFunctions::'#'` selects by one index and reports several, which address an Array.
  `RationalFunctions::rat`/`numer`/`denom` (a Rational is a float64 here),
  `CollectionFunctions::'array#'`, `BaseFunctions::'['` and the several-index
  `BaseFunctions::'#'` (no Array value kind),
  `BaseFunctions::all`/`as`/`meta`/`istype`/`hastype`/`'@'`/`'@@'` and `ControlFunctions::'.'`
  (evaluated from their own notation, not as functions), `DataFunctions`/`ScalarFunctions::'~'`
  and every `OccurrenceFunctions` declaration report themselves by name, each holding its
  declared multiplicities (`addNew(occ = o)` omits the `[0..*]` group; a call missing a
  required parameter is an arity error first). An operator-named
  function reports itself as the model writes it (`IntegerFunctions::'+'`).
