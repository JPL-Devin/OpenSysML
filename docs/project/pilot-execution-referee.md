# The pilot as a referee for *behavioral* conformance

`cmd/pilot-diff` uses the pinned OMG pilot as an external referee for parsing, resolution and
validation. This document answers a different question: **how far does the pinned pilot's
*execution* surface reach, and which of the behavior rows in
[spec compliance](spec-compliance.md) can it adjudicate?**

The answer is narrow, and deliberately stated as such. The pinned artifact evaluates
*expressions* over a model's declarations. It does not execute actions, does not run state
machines, and has no notion of a step, a token or a trace. Three of the four behavior areas
are therefore **out of its reach**, and no amount of harness work changes that.

Pin: tag `2026-05`, artifact `jupyter-sysml-kernel 0.60.1` (`scripts/pilot-pin.sh`). Every
command below was run against the shaded jar that `scripts/download-pilot-validator.sh`
unpacks, at
`build/pilot-validator/target/sysml-download/sysml/jupyter-sysml-kernel-0.60.1-all.jar`.

## Capability map

| Behavior area | Verdict | Why |
|---------------|---------|-----|
| **Expression evaluation** | **Can adjudicate** — for model-level expressions | `SysMLInteractive.eval` (the `%eval` magic) evaluates literals, operators, library functions, `calc` invocations and feature *default values*, headlessly, deterministically. This is a genuine second opinion, and `cmd/pilot-exec-diff` uses it |
| **Action / token-flow execution** | **Cannot speak to it** | No interpreter exists in the artifact. `%eval` refuses an action def or usage as a target |
| **State-machine execution** | **Cannot speak to it** | Same: no state execution, no transition firing, no trace output. `%eval` refuses a state def or an `exhibit`ed state |
| **Classifier behaviors (`exhibit`/`perform`)** | **Cannot speak to it** for execution; **can corroborate** the *declared* value of a performed action's `out` parameter | `%eval` reads `machine.p.n` as the model-level value of the parameter's default expression, which is not an execution: no performer object exists, nothing is stepped |
| *(cross-cutting)* **Scope of an expression in a behavior body** | **Can corroborate only** | The pilot resolves names for a *model-level* evaluation of a declaration written in a behavior body. It says nothing about frames, shadowing or the values a running body writes — the substance of those rows |

## Evidence

### The artifact's whole execution surface is expression evaluation

The magics in the pinned jar:

```
$ unzip -Z1 build/pilot-validator/.../jupyter-sysml-kernel-0.60.1-all.jar \
    | grep -i 'jupyter/kernel/magic/[A-Za-z]*\.class'
org/omg/sysml/jupyter/kernel/magic/Load.class
org/omg/sysml/jupyter/kernel/magic/Projects.class
org/omg/sysml/jupyter/kernel/magic/View.class
org/omg/sysml/jupyter/kernel/magic/Viz.class
org/omg/sysml/jupyter/kernel/magic/Show.class
org/omg/sysml/jupyter/kernel/magic/Publish.class
org/omg/sysml/jupyter/kernel/magic/Repo.class
org/omg/sysml/jupyter/kernel/magic/Eval.class
org/omg/sysml/jupyter/kernel/magic/Help.class
org/omg/sysml/jupyter/kernel/magic/MyMagicParser.class
org/omg/sysml/jupyter/kernel/magic/Export.class
org/omg/sysml/jupyter/kernel/magic/Listing.class
```

`Eval` is the only one that computes anything: `Load`/`Projects`/`Repo`/`Publish` talk to a
model repository, `Viz`/`View`/`Show`/`Listing`/`Export` render or serialize. The kernel's own
help text for it, verbatim from `SysMLInteractiveHelp`:

```
Usage: %eval [--target=<NAME>] <EXPR>
Print the results of evaluating <EXPR> on the target given by <NAME>, which must be fully qualified.
If a target is not given, then evaluate <EXPR> in global scope.
```

The `org/omg/sysml/execution/` package in the jar contains exactly one thing —
`execution/expressions/`, an `ExpressionEvaluator` plus library function implementations
(`SizeFunction`, `SelectFunction`, `SumFunction`, the trig functions, …). There is no
interpreter, simulator, scheduler, token or trace class under `org/omg/sysml/` at all; a
search for those names in the jar returns only Guava/ICU/Xtext infrastructure. Action and
state semantics are present in the artifact **as a metamodel and as adapters**
(`ActionUsageImpl`, `StateUsageImpl`, `TransitionUsageImpl`, `ExhibitStateUsageAdapter`,
`plantuml/VStateMachine` for *drawing* a machine) — representation and rendering, not
execution.

### It runs headlessly

`scripts/pilot-evaluator/EvalSysML.java` drives `SysMLInteractive` directly — the same class
the kernel drives — with no notebook and no kernel protocol:
`SysMLInteractive.createInstance()`, `loadLibrary(dir)`, `process(modelText)` per model, then
`eval(expr, target, List.of())` per case. `scripts/download-pilot-evaluator.sh` compiles it
against the pinned jar and writes the launcher `build/pilot-evaluator/eval-sysml`. So the
expression surface is usable as a referee exactly the way the two validators are.

### What it emits

Output is **plain text, one line per resulting value**, each line a metamodel node kind, the
value, and the element's UUID:

```
== case int
LiteralInteger 7 (fb98f88c-9172-45bc-8ed8-b78fe546719b)
== end int
== case seqattr
LiteralInteger 3 (938afbeb-0cea-48ec-9c99-b5eadc1ab9b9)
LiteralInteger 1 (390142ba-ef95-4e34-9f78-8381a269be31)
LiteralInteger 2 (1b8b3471-f721-4eab-a377-6d6da070fb36)
== end seqattr
```

(`== case`/`== end` are our driver's framing; the lines between them are the pilot's own
output, verbatim.) There is **no machine-readable protocol** for `eval` — no JSON, no exit
code per case. A sequence is a run of value lines in order; an empty result is *zero* lines,
which is the same rendering an unevaluable expression gets (`1 / 0` produces no lines and no
diagnostic). Diagnostics come back as `ERROR:…` / `WARNING:…` lines in the same stream.

An expression the evaluator cannot reduce comes back as the **unevaluated node**, which is how
quantities appear:

```
== case quant
OperatorExpression [ (3009c3ad-7f3b-4665-a111-d99b4e256305)
== end quant
== case quantcalc
OperatorExpression + (e2642709-e92b-484f-bb0c-03331ad7ca9e)
== end quantcalc
```

`Probe::q` is `3.0 [SI::kg]` and `Probe::Quant()` is `3.0 [SI::kg] + 1.0 [SI::kg]`; OpenSysML
answers `3.00 [SI::kg]` and `4.00 [SI::kg]`. The pilot is not disagreeing — it is not
evaluating. Unit-carrying values are therefore out of the referee's reach, and
`cmd/pilot-exec-diff` buckets them `pilot-unevaluated` rather than as a disagreement.

It is **deterministic**: two runs of the same cases in separate JVMs differ only in the UUIDs.

```
$ diff <(sed -E 's/\([0-9a-f-]{36}\)//' run1.txt) <(sed -E 's/\([0-9a-f-]{36}\)//' run2.txt)
IDENTICAL
```

### What it accepts

A model is `process`ed as text into an accumulating session, as a notebook cell is. **A model
with any error registers nothing**: while `Probe` had one unresolved function name, every
later case failed with `Couldn't resolve reference to Element 'Probe::n'` — a whole-file
verdict, not a per-case one. The harness reports the model block so this cannot be mistaken
for a semantic disagreement.

A `--target` must be a **namespace**. A package works; a part, an action, a state or a feature
does not:

```
== case action-def         (target -, expr Behave::Bump)
ERROR:Must be a valid feature
== case action-usage       (target -, expr Behave::Flow::a)
ERROR:Must be an accessible feature (use dot notation for nesting)
== case state-def          (target -, expr Behave::Modes)
ERROR:Must be a valid feature
== case exhibit            (target -, expr Behave::machine::s)
ERROR:Must be an accessible feature (use dot notation for nesting)
== case perform            (target -, expr Behave::machine::p)
ERROR:Must be an accessible feature (use dot notation for nesting)
== case part-attr          (target Behave::cc, expr count)
ERROR:Must be an accessible feature (use dot notation for nesting)
```

Naming a behavior is how one would ask for its execution, and that is precisely what is
refused. Nothing in the surface takes a behavior and runs it.

Feature *values* are readable through dot notation, and this is the corroboration the surface
does offer:

```
== case dot-part-attr      (expr Behave::cc.count)
LiteralInteger 0 (d36e4833-0c8d-480c-b7e4-3de1b0f94d6d)
== case dot-perform-out    (expr Behave::machine.p.n)
LiteralInteger 1 (1a672f09-0b90-4137-a358-c8f5ea36164d)
```

`Behave::Bump` declares `out n : Integer = c.count + 1`, so `1` is the default expression
evaluated over declarations — no object was materialized and no action ran.

## What this means for the 23 behavior rows

- **Action (all rows), State Machine (all rows), Classifier Behaviors (execution rows).**
  Out of reach. The two named deliberate deviations — concurrent fork branches writing a
  shared feature space in step order, and variant selection not being ordering-sensitive —
  **cannot be adjudicated by the pinned artifact**, because ordering is a property of an
  execution the artifact never performs. Their status stays self-assessed against our golden
  traces; this document is not evidence for or against them.
- **Scope of an expression in a behavior body.** Corroboration only, and only for the
  model-level slice: whether a name written in a behavior body resolves at all, and what a
  declaration's default expression evaluates to. Frames, shadowing and values written by a
  running body are unobservable to the pilot.
- **Expression Evaluation rows.** Genuinely adjudicable, within the limits below; this is what
  `cmd/pilot-exec-diff` compares.

No compliance row's status flag is changed on the strength of this work.

## Limits of the comparison (read before trusting a bucket)

- **Reals are compared to two decimal places**, because that is OpenSysML's display precision:
  the pilot reports `LiteralRational 0.3333333333333333` for `1.0 / 3.0` where we print
  `0.33`. A divergence below 2dp is invisible to this harness.
- **Integer vs Rational is reported, never normalized away.** `2 ** 40` gives the pilot
  `LiteralRational 1.099511627776E12` and gives us `1099511627776`; the harness buckets that
  `kind-only`, not `agree`.
- **Quantities are out of reach** (see above), as is anything else the pilot returns as an
  unevaluated node.
- **An empty result is ambiguous** on the pilot side: no value, and an expression it declined
  to evaluate, render identically.
- **Collection order** is compared as order; a same-multiset/different-order result is its own
  bucket, so a real ordering difference is never hidden by sorting.
- **Scalar vs one-element sequence is unobservable.** We print `[2]` where the pilot prints a
  single value line, and its rendering has no way to distinguish the two (in the notation every
  value is a sequence), so single-element sequences are unwrapped on both sides. A genuine
  scalar/singleton difference, if one exists, would be invisible here.
- **UUID identity is the only thing normalized away** on the pilot side.

Run it with `go run ./cmd/pilot-exec-diff` after `./scripts/download-pilot-evaluator.sh`; with the
execution artifact absent it prints a provisioning instruction, exits 0 and writes nothing, so
`cmd/pilot-diff` and its committed baseline are untouched. Current state of the 32 committed cases:

```
agree: 21 · kind-only: 1 · order-only: 0 · disagree: 0
pilot-unevaluated: 2 · pilot-error: 2 · ours-error: 3 · both-error: 3 · nondeterministic: 0
```

No case disagrees on a value both tools evaluate. The one `kind-only` is `2 ** 40` (above); the
`pilot-error` and `pilot-unevaluated` cases are the pilot's limits, not disagreements; the
`ours-error` cases are finding 1 below plus `1 / 0`, where we raise `division by zero` and the pilot
returns nothing at all.

## Findings to carry forward

1. **`Behave::machine.p.n` — a divergence in a `perform action` binding's scope.** Both tools
   validate the model clean. The pilot evaluates the performed action's `out` parameter
   default; our runtime fails to resolve the package-level part named in the binding:

   ```
   $ ./bin/sysml -validate behavior.sysml
   ✓ /tmp/ev/behavior.sysml: no errors
   $ ./bin/sysml -e "Behave::machine.p.n" behavior.sysml
   sysml: evaluation failed: usage machine: performed action p of machine: bind c: unresolved reference: cc
   ```

   against the pilot's `LiteralInteger 1`, on `perform action p : Bump { in c = cc; }` inside
   `part machine`, where `cc` is a sibling member of the enclosing package. Reported here, not
   fixed: the fix belongs in `internal/core/**` with the AGENTS §5.2 contract.
2. **The pilot's evaluation context is narrower than ours, so the scope rows get no referee.**
   With `--target=Behave::Flow` the pilot cannot see the enclosing package's members
   (`cc.count` → `Couldn't resolve reference to Element 'cc'`) where we answer `0`; and
   `--target=Behave::Bump` with `n` is refused outright (`Must be an accessible feature`) where we
   answer `1`. Both are limits of the pilot's `%eval` context rather than statements about scope
   semantics — which is exactly why the "scope of an expression in a behavior body" rows get
   corroboration only, and why these two cases are bucketed `pilot-error` rather than as our
   disagreement.

## What a real behavioral referee would take

Nothing short of a second *executing* implementation. Options, in order of cost:

1. **A pilot version that executes.** The OMG pilot's execution work lives outside this
   artifact; adopting it means moving off the pin, and the pin is what makes the static
   differential meaningful. It would have to be a *second*, separately pinned artifact.
2. **A different tool** (e.g. an fUML/Alf-based executor, or SysIDE's runtime if it grows one),
   which brings its own conformance question: a disagreement then needs adjudication against
   the specification anyway.
3. **Specification-derived traces.** Hand-adjudicated traces from KerML `Performances`/
   `Occurrences` and the Systems Library, reviewed as evidence in their own right. Slower per
   row, but it is the only route that answers the ordering questions the deviations raise.
