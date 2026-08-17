# Extracting the State Machine Engine as a Reusable Statechart Library

Status: assessment / design proposal. No code changes accompany this document.
Companion to `state-engine-pluggability.md`, which assessed importing a foreign
engine. This document assesses the inverse: making *our* engine generic and
usable the way the surveyed candidates are.

Short answer: this is the better direction, and it is achievable because the
engine's dependency on the rest of Systemica is **wide but shallow** — hundreds
of `ast.`/`symbols.` references, but only seven *kinds* of coupling, each of
which has a mechanical replacement (§2). The extraction is verifiable rather than
speculative: 64 state conformance cases and 26 ordering-sensitive trace goldens
must stay byte-identical, which is a stronger acceptance test than most refactors
get. It also subsumes the pluggability seam, and — via semantics profiles (§5) —
delivers the original "support other engines" benefit without a second engine.

---

## 1. What we would be offering

The Go ecosystem has flat FSMs (`looplab/fsm`), hierarchical-only machines
(`qmuntal/stateless`), and SCXML-flavoured statecharts. What none of them
combine, and what this engine already has under test:

| Capability | Status here | Typical candidate |
|---|---|---|
| Orthogonal regions with declaration-ordered, deterministic dispatch | ✅ 69 tracked rules | some |
| Shallow **and** deep history, restored per region | ✅ | some |
| Fork/join, choice, junction, entry/exit points, pseudostate chains with cycle detection | ✅ | rare |
| Deferred events with earliest-transfer recall ordering, deferral by ancestors and across regions | ✅ | rare |
| Do activities scheduled in rounds, interleaved across active regions, abandoned on exit, gating completion | ✅ | rare |
| Virtual clock; time triggers scoped to the declaring state | ✅ | tick loops |
| Resource budgets with typed runaway errors instead of hangs | ✅ | none |
| Debugger-grade stepping (`ProcessNextEvent`, `RunDoRound`, `HasPendingWork`, `HasPendingDoWork`) | ✅ | rare |
| Deterministic, golden-comparable execution traces | ✅ 26 goldens | rare |
| Precise-semantics lineage (KerML `StatePerformances`, PSSM-adjacent) | ✅ documented rule by rule | UML "flavoured" |

The honest differentiator is not the feature list alone — it is that every item
is pinned to a named semantic rule in `docs/project/spec-compliance.md`. A
library that can say *which* statechart semantics it implements, rule by rule,
with tests, is a genuinely scarce thing.

---

## 2. How coupled is the engine, really

Measured on the current tree: 141 `ast.`/`symbols.` references in
`state_executor.go`, 40 in `state_region_transition.go`, 138 in
`lower/state_graph.go`. But the coupling falls into exactly seven kinds, and none
of them is semantic:

| # | Coupling today | What the engine actually needs | Replacement |
|---|---|---|---|
| 1 | `map[*ast.StateNode]…`, `map[*ast.StateRegion]…` | stable identity | opaque `ID` handles allocated by the model builder |
| 2 | `state.Name`, `getNodeName` for traces/visits | a label | `State.Name` on the engine's own model |
| 3 | Trigger inspection: `AcceptEvent.SignalType`/`Subsets`, `CallEvent.Operation`/`Parameters`, `ChangeEvent.Condition != nil` | trigger *shape*: a signal name, an operation name + parameter names, "has a condition" | engine-owned `Trigger` descriptor |
| 4 | Guard/change-condition/time-duration expressions passed to `NewEvalContext(ctx, scope)` | "give me a bool"/"give me a number" | opaque `ExprRef` evaluated by the host |
| 5 | `lower.StateBehavior` run via the action-statement interpreter | "run this behavior to completion" / "run one step of it" | opaque `BehaviorRef` run by the host |
| 6 | `Value` + `semantics.ValBool`/`ValInt`/`ValReal` kind checks; `stateData map[string]Value` | a value type it never interprets, except through the host | type parameter `V`, host-provided predicates |
| 7 | `Message.carriesSignal/arrivedAt/reachedObject`, `Call{Operation, Args}`, `ctx.post`, `TakeMessage`, budgets, `TraceRecorder` (string-only calls) | routing predicate, event source, cost accounting, observation sink | host services |

Two observations that make this tractable:

- The engine never reads a type, resolves a name, or evaluates an expression
  itself. Name resolution and typing happen upstream of `lower.StateGraph`
  already, and evaluation is already a call out to the evaluator.
- The trace calls are string-only (`RecordStateEntry(name string, …)`), so
  observation is trivially an interface.

The dependency edge is therefore enumerable — which is what makes a *library*
extraction different from a rewrite.

---

## 3. Target architecture

```
    ┌──────────────────── statechart (stdlib-only, no SysML) ─────────────────┐
    │  Model: States, Regions, Pseudostates, Transitions, Triggers (by ID)    │
    │  Machine[V]: configuration, selection, RTC, deferral, history,          │
    │              do-rounds, virtual clock, budgets, observation             │
    └──────────────▲───────────────────────────────┬──────────────────────────┘
        Builder    │                               │ Host[V] callbacks
                   │                               ▼
    lower.StateGraph ──► sysml adapter ──► EvalBool / Run / Bind / Route / Now
                             (runtime.StateExecutor becomes this adapter)
```

API sketch — illustrative, not final:

```go
package statechart

type ID uint32
type ExprRef, BehaviorRef any // opaque to the engine; the host's own handles

type TriggerKind int // Signal | Call | Change | Time | Completion

type Trigger struct {
    Kind      TriggerKind
    Signal    string   // accept Ping / accept :> shutDown (already flattened)
    Operation string
    Params    []string // bound into machine data for the guard and effect
    Payload   string   // the name an accept gives its payload, "" if none
    Via       string   // port the occurrence must have arrived at
    Expr      ExprRef  // change condition, or time instant/duration
    Absolute  bool     // `at` vs `after`
}

type State struct {
    ID      ID
    Name    string
    Parent  ID
    Regions []ID
    Entry, Do, Exit []BehaviorRef
    Defer   []Trigger
}

type Transition struct {
    ID             ID
    Source, Target ID          // states or pseudostates
    Trigger        *Trigger    // nil = completion
    Guard          ExprRef
    Effect         []BehaviorRef
}

type Model struct { /* States, Regions, Pseudostates, Transitions, Initial, Semantics */ }

// Validate reports every structurally impossible model — a transition endpoint
// naming no vertex, a routing pseudostate with no way out, a pseudostate cycle,
// a region with no initial state — as a typed error, so a host need not
// re-implement lowering's checks.
func (m *Model) Validate() error

type Host[V any] interface {
    EvalBool(ExprRef) (bool, error)
    EvalNumber(ExprRef) (float64, error)
    Run(BehaviorRef) error                     // entry/exit/effect: to completion
    RunStep(BehaviorRef) (done bool, err error) // do activity: one round
    Bind(name string, v V) (unbind func())     // trigger payload / call arguments
    Route(t Trigger, e Event[V]) (bool, error) // port and addressee matching
    Now() float64
    Charge(Cost) error                          // budgets; returns the host's typed error
    Observe(Record)                             // trace/animation sink
}

type Machine[V any] struct{ /* … */ }

func New[V any](m *Model, h Host[V]) (*Machine[V], error)

// Drive and observe — the surface the REPL debugger and gRPC already use.
func (mc *Machine[V]) Start() error
func (mc *Machine[V]) RunToCompletion() error
func (mc *Machine[V]) ProcessNextEvent() error
func (mc *Machine[V]) RunDoRound() (int, error)
func (mc *Machine[V]) Send(signal string, args map[string]V)
func (mc *Machine[V]) Call(op string, args map[string]V)
func (mc *Machine[V]) Active() []ID
func (mc *Machine[V]) Data() map[string]V
func (mc *Machine[V]) Visits() []string
func (mc *Machine[V]) Now() float64
```

Notes on the choices:

- **`V` as a type parameter, not `any`.** The engine stores values and hands them
  back; it never inspects them. A type parameter keeps that honest and gives
  external users type safety without boxing.
- **`Route` on the host, matching in the engine.** Name/operation/parameter
  matching is semantics and stays in the engine; port and addressee matching is
  the host's model (`arrivedAt`, `reachedObject`), so it stays out.
- **`Charge(Cost)` returns the host's error.** Budgets are the host's policy;
  the engine only declares what it is about to spend. This keeps
  `ErrStepLimitExceeded` and its siblings in Systemica where their messages name
  the `SYSML_*` variable that raises the bound.
- **`Observe(Record)`** replaces the `TraceRecorder` calls; Systemica's recorder
  becomes one implementation, and animation/GUI consumers become another.

### 3.1 What Systemica becomes

`runtime.StateExecutor` shrinks to an adapter: build a `statechart.Model` from
`lower.StateGraph` (keeping an `ID ↔ *ast.StateNode` map so
`ActiveStates() []*ast.StateNode` and the REPL keep working), implement `Host`
over `*Context` (evaluator, statement interpreter, message bus, budgets, trace),
and delegate. Its public methods keep their signatures, so `internal/repl`,
`internal/grpc` and the tests do not change.

**Acceptance test for the whole extraction:** every state conformance case and
all 26 trace goldens pass unchanged, plus the OMG corpus gate run locally. If a
golden moves, the extraction is wrong — not the golden.

---

## 4. Publishing shape

| Option | Import path | Pros | Cons |
|---|---|---|---|
| A. `internal/core/statechart` | not importable | no API promise; unlocks §5 and the pluggability seam immediately | no external users |
| B. `pkg/statechart` in this module | `github.com/Open-MBEE/Systemica/pkg/statechart` | one module, one CI, Apache-2.0 already permissive | library versioning is tied to the tool's release cadence; every tool release is an API release |
| C. Separate module in this repo (`statechart/go.mod`, tags `statechart/vX.Y.Z`) | `github.com/Open-MBEE/Systemica/statechart` | independent semver, still one repo and one CI; the tool consumes it via the module graph | needs a second `go.mod` in build/release scripts and the Homebrew/CI plumbing |
| D. Separate repository | new path | maximum independence | two repos to keep in sync; changes that span engine and tool become two PRs |

Recommendation: A first (it is a prerequisite for everything), then C if we
actually want external users. B is a trap — it makes every `sysml` patch release
an engine API release.

Stdlib-only is achievable and worth committing to: after §2, the engine imports
nothing outside the standard library, which is exactly the property that makes
libraries like these adoptable. Today the module carries readline, gRPC, protobuf,
zap and the LSP stack; a consumer who wants only a statechart engine should not
inherit any of that, which is an argument for option C over B.

### 4.1 Library, or standalone runner? They are two different costs

"Decoupled engine" splits into two products, and it matters which one we mean:

**A Go library, embedded in a host (cheap — this is Phases A–C).** The consumer
supplies `Host`: expression evaluation, behavior execution, event routing.
Systemica is the first such host; a second host could be an app that wants
hierarchy, regions, history and deferral for its own workflow states and evaluates
its own guards as Go closures:

```go
m, _ := statechart.New[any](model, myHost{})     // guards/effects are my funcs
m.Send("payment_captured", nil)
```

This is usable standalone in the sense that it needs no SysML, no parser and no
model file. It is *not* usable standalone in the sense of "run a `.sysml` file" —
because it deliberately never learns what an expression means.

**A standalone runner that executes a KerML model (more expensive, and worth
naming as separate scope).** The blocker is that guards, change conditions, time
durations and effects are today `ast.Node`s evaluated by Systemica's evaluator
against live scopes — they are references into a parsed model, not data. A
standalone runner therefore needs a **serializable behavior/expression IR** plus
an evaluator for it, so that:

```
sysml compile model.sysml --state Vehicle::controller -o controller.scm.json
scrun controller.scm.json --send brakeApplied     # no parser, no symbol table
```

That is real work (expression lowering to a closed instruction set, plus its own
conformance tests) and it is what buys the deployment shapes below. It should be
a deliberate later phase, not smuggled into the extraction.

Once that IR exists, the same engine ships as: an in-process Go library; a
compile-once/run-anywhere CLI; the existing gRPC service minus the compiler (a
slim execution service the `pysysml` client already has a protocol for); and —
because the engine is stdlib-only and allocation-flat — a WASM build, which is the
cheapest path to animating a state machine in a browser diagram viewer without a
server.

---

## 5. Semantics profiles — the original request, inverted

The blocker in `state-engine-pluggability.md` was that our semantics are
KerML-derived while every candidate implements UML/SCXML semantics. Once the
engine owns its own model, that difference becomes a *model option* rather than a
different engine:

```go
type Semantics struct {
    // CrossRegionExit: KerML exits the source only (StatePerformances orders
    // `guard then transitionLinkSource.exit`); UML/SCXML exits and re-enters
    // the least common ancestor.
    CrossRegionExit CrossRegionPolicy // KerML | LCA
    // Junction: evaluated when entered (KerML/here) or resolved statically
    // before the incoming transition (UML).
    Junction JunctionPolicy
    // ChangeEvents: polled (here) or continuously re-evaluated after every
    // microstep (SCXML-style).
    ChangeEvents ChangePolicy
    // DoActivities: one action per round interleaved across regions (here) or
    // run to completion on entry.
    DoActivities DoPolicy
}
```

Each profile axis gets its own tests, and the default profile is the KerML one
that the goldens pin. This gives a user who wants "another engine's behaviour"
the behaviour they actually wanted, from one engine we already understand, and it
makes the library credible to non-SysML users who expect UML/SCXML semantics.

It also bounds the claim we may make: OMG's conformance terms allow a compliance
claim only where the applicable test suites are satisfied, so profile names must
describe intent ("UML LCA exit policy"), and any PSSM/SCXML conformance statement
must be backed by published suite results and a departures table (§6, Phase D).

---

## 6. Phasing and effort

Sessions are my own throughput, and every phase ends at the AGENTS.md §2
definition of done plus the locally-run OMG corpus gate.

**Phase A — decouple in place (~2 sessions).** Introduce `statechart` with the
ID-based model, trigger descriptors, `Host`, and `Observe`; convert the executor
to it; keep `StateExecutor`'s signatures as the adapter. Wide (≈2,850 engine lines
across `internal/core/runtime/state_*.go`, plus the ≈900-line lowering it
consumes) but mechanical, and the goldens make
each step verifiable. Deliverable: identical behavior, `internal/` only.

**Phase B — library-grade boundary (~1–2 sessions).** `Model.Validate` with its
own typed error taxonomy (today's structural checks live partly in lowering and
partly in `passes/state_transition.go`), engine-level unit tests written against
the engine's own model with no SysML in sight, stdlib-only enforced by a build
check, and package documentation with runnable examples.

**Phase C — semantics profiles (~1–2 sessions).** The axes in §5, each with
tests, and the profile reported by every surface that can select it. This is the
phase that answers the original question, and it is also what makes the library
useful outside SysML.

**Phase D — external face (~2–3 sessions).** A non-SysML model front-end
(JSON/YAML, plus SCXML import to reuse existing test corpora), the W3C SCXML test
suite and the OMG PSSM test suite run against the relevant profiles with a
published departures table, benchmarks (the existing harness in
`internal/repl/bench_test.go` measures machine start at 26–334 µs and flat
execution memory, so the numbers to publish exist already), a stability policy, and the module split of option C.

Cumulative: ~6–9 sessions to a credible v0.1 external library. Phase A stands on
its own — it improves the codebase, and it is the seam
`state-engine-pluggability.md` Phase 1 asked for, so the two directions converge
here rather than competing.

**Phase E — standalone runner (demand-driven, ~2–3 sessions, §4.1).** A
serializable behavior/expression IR plus its evaluator, a `compile`/`run` CLI pair,
and the gRPC/WASM shapes that follow. Only worth starting if a §6.1 consumer in
the second group is actually wanted.

### 6.1 Who would use a KerML engine, other than us

Ordered by how little extra scope each needs beyond the extraction. The first
group needs only the library (§4.1); the second needs the serializable IR.

**Needs only the library.**

- **Systemica itself, better factored.** The immediate consumer: one engine, host
  callbacks, and the pluggability seam of the companion document for free.
- **Trace-based V&V and model-based test generation.** Deterministic traces plus
  budgets make the engine a generator of expected behavior: enumerate event
  sequences over a model, emit traces, and use them as test vectors for the
  implementation the model describes. Budgets are what makes bounded exploration
  safe here.
- **Conformance oracle in both directions.** Run the OMG PSSM suite and the W3C
  SCXML suite against the matching profile (§5) — that is how the library earns
  the right to describe its own semantics, and it also tells us where *we* differ.
- **Non-SysML Go users.** Hierarchical states with orthogonal regions, deep
  history, deferral and choice/junction are exactly what application state
  machines (device controllers, long-running workflows, protocol handling) need
  and what `looplab/fsm` does not have. This is the audience that makes it a
  *library* rather than an internal package, and it is served by the UML profile,
  not by KerML semantics.
- **Animation and debugging front-ends.** `Observe(Record)` plus the stepping API
  is a ready-made protocol for a diagram animator or a time-travel debugger; the
  VS Code extension in `editors/vscode` is the obvious first client.

**Needs the serializable behavior/expression IR too (§4.1).**

- **Runtime conformance monitoring.** Execute the model's state machine alongside
  a real system's telemetry and flag where observed behavior departs from the
  specified machine — the model becomes an executable monitor rather than a
  document. This is the use case with the most obvious value for flight software
  and ground-system operations, and the one that most needs "run the model
  without a compiler in the loop".
- **Co-execution against generated or hand-written implementations.** Compile the
  model once, run it in step with the C/C++/Rust implementation, and compare
  traces — a cheap way to keep codegen or a hand port honest.
- **Simulation embedded in other tools.** Any Go, Python (via the existing gRPC
  protocol) or browser (WASM) tool that wants to run behavior from a model it did
  not compile.
- **CI gates over models.** Execute a model's machines in a pipeline with no IDE
  and no full toolchain, asserting reachability, no-deadlock, and expected traces.

What this list does *not* contain is a case for making the engine standalone as an
end in itself. Every entry above is either served by the library today plus
callbacks, or by the library plus a compiled model artifact. That is the argument
for doing Phases A–C first and treating the runner as demand-driven.

---

## 7. Risks and honest limitations

- **API freeze.** An exported engine API is a promise. Mitigated by staying in
  `internal/` through Phases A–C and only then choosing option C.
- **No real-time driver.** Our clock is virtual and queue-advanced; several
  candidates offer tick-based real-time runtimes. External users will ask for
  one. It is a wrapper, not a semantics change, but it is unbuilt scope.
- **Unusual do-activity model.** Round-interleaved do activities are a strength
  for simulation and a surprise for anyone expecting UML "do behaviour runs
  concurrently". It must be documented as a profile axis, not hidden.
- **The trace format is ours.** Useful for goldens, not a standard. Fine, as long
  as `Observe` is the seam and the format is not in the API promise.
- **Conformance claims.** See §5: profile names describe policy; conformance
  language requires suite results.
- **Maintenance.** An external library means issues, versioning, and
  compatibility work that the tool alone does not incur. Option C keeps that in
  one repository, but it does not make it free.

---

## 8. Decisions needed

1. Publishing shape: keep it `internal/` (A) for now, with C as the stated goal,
   or commit to C up front so Phase A lands with the module split already in
   mind?
2. Are semantics profiles (§5, Phase C) in scope for the first external release,
   or is v0.1 KerML-only with profiles as a follow-on?
3. Is a real-time/tick driver (§7) in scope, or explicitly out for v0.1?
4. Do we want the serializable behavior/expression IR and a standalone runner
   (§4.1) on the roadmap at all, or is the library-with-host shape enough? The
   answer follows from whether runtime conformance monitoring (§6.1) is something
   we actually want to offer.
