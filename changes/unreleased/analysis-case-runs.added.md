- **An analysis case runs.** An `analysis` definition or usage is the calculation it is: `-analysis
  "Pkg::Case[(args)] [object]"` and `%analysis` run it and print its `out` and `return` values with
  their units, then the verdict of its `objective` and of every `assert constraint` in its body —
  `satisfied`, `not satisfied` with the condition that failed, or `undecided` with the reason — and
  exit 0, 1 or 2 accordingly. The `subject` is an `in` parameter: a usage binding it (`subject s =
  ship;`) needs nothing more, a definition or an unbinding usage takes the object the run names (one
  `-instantiate`/`%instantiate` created) and a case nested in another runs on the enclosing case's
  subject; a case run with no subject is refused naming it rather than run empty. Arguments bind the
  other `in` parameters positionally or by name, as `-calc` takes them. The body's `action`,
  `perform` and nested `analysis` steps run through the action executor as one flow over the
  successions they state (`then`, `first`, forks, joins, decisions and merges) or in declaration
  order where they state none, each a subperformance whose outputs later steps and the case's
  outputs read by `step.pin`; a step that fails, a body that deadlocks or exceeds its step budget, a
  case that runs itself and an `in` parameter left without a value are typed refusals naming the
  case. Reading an analysis usage's output as a feature — `An::shipCost.total` on a package-level
  usage, `holder.inner.total` on a usage a part owns, `attribute :>> x = a.result;` — runs the case
  the way a `calc` usage's output does, memoized until a value it read changes. The gRPC service
  gains `RunAnalysis` (symbol, optional subject, positional and named arguments; outputs, verdicts,
  the subject's instances, and a typed `failure_reason`), the Connect adapter and the Go client gain
  `RunAnalysis`, and the Python client gains `Model.run_analysis` answering an `AnalysisResult`.
  `-calc`/`%calc`/`EvaluateCalc` still refuse an analysis by kind and now say to run it as one;
  `%optimize` is unchanged. A verification case body shares the grammar and lowers the same way but
  is not yet run: its verdict stays what `-requirement`/`-satisfy` compute.
