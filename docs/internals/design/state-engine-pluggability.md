# Pluggable State Machine Engines — Assessment and Design

Status: assessment / design proposal. No code changes accompany this document.

Question asked: pre-existing state machine engines exist; could Systemica support
additional state machine engines by configuration, and what would that take?

Short answer: the *seam* is affordable and worth building — roughly one focused
change extracting an engine interface, a registry and a configuration surface,
with the existing engine registered as `native` and still the default. What is
**not** affordable, and what this document argues against planning for, is a
second engine that is semantically interchangeable with the current one: the 69
semantic rules the state machine section of `docs/project/spec-compliance.md`
tracks are derived from KerML `StatePerformances`, and no off-the-shelf engine
implements them. An imported engine can be offered as an *experimental,
restricted-subset* alternative, and pre-existing engines can be used as
*oracles* and *interoperability targets*, but not as a drop-in replacement.

---

## 1. What the current engine actually is

### 1.1 Inventory

| Part | Location | Size |
|---|---|---|
| Engine (dispatch, RTC, hierarchy, regions, pseudostates, history, do activities) | `internal/core/runtime/state_executor.go`, `state_region_transition.go`, `state_statements.go`, `routing.go` | ~3,090 lines |
| Lowering to the execution IR | `internal/core/lower/state_graph.go`, `state_behavior.go` | ~1,005 lines |
| Event queue, execution state, event types | `internal/core/runtime/executor_common.go` | shared with the action executor |
| Budgets | `internal/core/runtime/budget.go` | shared |
| Tests | 89 Go test functions in the state-focused `*_test.go` files (~4,190 lines), 64 state conformance models, 26 state trace goldens | |
| Semantic rule ledger | `docs/project/spec-compliance.md` §"State Machine" | 69 rules |

That test corpus *is* the engine's specification. Any engine-swap discussion is
really a discussion about who satisfies those 69 rules, 64 conformance cases and
26 ordering-sensitive trace goldens.

### 1.2 It is not a self-contained state machine library

The current executor is the control layer of a *model interpreter*, not a
standalone statechart runtime. Its inputs, callbacks and outputs are all
Systemica types:

- **Input**: `lower.StateGraph`, whose maps are keyed by `*ast.StateNode`,
  `*ast.StateRegion` and `*ast.PseudostateNode`, and which carries a
  `*symbols.Scope` per state and per transition so that a guard or effect
  resolves names where it was written.
- **Guards and triggers**: evaluated through `NewEvalContext(ctx, scope)` — the
  full SysML expression layer, including units, collections and `calc`
  invocation.
- **Entry/do/exit/effect behaviors**: `lower.StateBehavior` run through
  `state_statements.go`, i.e. the action-statement interpreter, including
  `perform` of a named action via `invokeAction`.
- **Messaging**: `ctx.post(graph.Connections, msg, …)`, `ctx.TakeMessage`,
  `ctx.PendingMessages` — sends route through the performing `*Instance`'s
  connectors and variant selections.
- **Time**: a virtual scalar clock (`currentTime`) advanced by the event queue,
  not wall clock.
- **Budgets**: `ctx.maxStateEvents`, `ctx.maxDoSteps`, plus the shared step and
  element budgets, so a runaway machine reports a typed error instead of
  hanging.
- **Observation**: `TraceRecorder`, `GetStateVisits`, and the debug drive API.

So an "engine" in Systemica's sense owns only: active-configuration bookkeeping,
transition selection and conflict resolution, entry/exit ordering, the
run-to-completion step, event queueing/deferral, and do-activity scheduling.
Everything a general-purpose FSM library would ask you to supply as callbacks —
guard evaluation, actions, timers, messaging — is the part Systemica already
owns and cannot delegate.

### 1.3 The drive/observe API consumers already depend on

`internal/repl` (`%state`, `%current`, `%events`, `%advance`, `%signal`),
`internal/grpc` (`ExecuteState`, `ExecuteStateWithEvents`) and the runtime tests
consume this surface today:

```
RunToCompletion() error
ProcessNextEvent() error
RunDoRound() (int, error)
HasPendingWork() bool            HasPendingDoWork() bool
SendSignal(type string, args map[string]Value)
InvokeOperation(op string, args map[string]Value)
HasPendingSignal() bool
CurrentState() ast.Node          ActiveStates() []*ast.StateNode
StateStack() []*ast.StateNode    StateData() map[string]Value
GetStateVisits() []string
EventQueue() *EventQueue         CurrentTime() float64
State() ExecutionState           StateMachineSymbol() *symbols.Symbol
SetTrace(*TraceRecorder)
```

That is ~17 methods, and it is the natural interface to extract. Note how much
of it is AST-typed: `ActiveStates() []*ast.StateNode` means a foreign engine
must map its own state identity back to the model's nodes, not merely to names.

---

## 2. Candidate engines

| Candidate | Language / mode | Hierarchy | Orthogonal regions | History | Fork/join | Deferred events | Fit verdict |
|---|---|---|---|---|---|---|---|
| `looplab/fsm` | Go, in-process | no | no | no | no | no | flat FSM only; not a candidate |
| `qmuntal/stateless` | Go, in-process | yes | no | no | no | no | needs mutually-exclusive guards; no regions/history — cannot host the corpus |
| Go statechart libraries (e.g. `comalice/statechartx`, `comalice/statechart`) | Go, in-process | yes | yes | yes | partial | varies | closest in-process fit; semantics are SCXML/UML-flavoured, not KerML |
| W3C SCXML engines (Apache Commons SCXML, uSCXML, Sismic, SCION/XState) | JVM/C++/Python/JS, out-of-process | yes | yes | yes | yes | yes (SCXML has no `defer`, but has internal queues) | strong engines, wrong process and wrong semantics baseline; excellent as an interop target |
| OMG PSSM implementations (Papyrus **Moka**: fUML 1.4 + PSCS 1.2 + PSSM 1.0) | Java/Eclipse, out-of-process | yes | yes | yes | yes | yes | the only candidate whose semantics are a *normative* precise-semantics standard; usable as an oracle, not embeddable |
| Commercial simulators (Cameo Simulation Toolkit, itemis CREATE) | out-of-process / codegen | yes | yes | yes | yes | yes | licensing and integration cost; not embeddable |

Two facts matter more than the matrix:

1. **Every in-process Go candidate is a control-layer-only library that expects
   the host to supply guards and actions as closures.** That is exactly the
   division of labour §1.2 describes, so the integration is *mechanically*
   plausible: translate `lower.StateGraph` into the library's model, and hand it
   closures that call back into Systemica's evaluator. Cost is in translation
   and in divergence (§3), not in plumbing.
2. **The out-of-process candidates cannot be engines here at all.** Every guard
   and every effect would cross a process boundary and would need the SysML
   expression layer on the other side. Their real value is as *oracles* and
   *export targets* (§5.3): PSSM ships a normative machine-readable test suite
   (`PSSM_TestSuite.xmi`, OMG file ptc/18-11-06), and SCXML has the W3C test
   suite.

---

## 3. Where any imported engine will disagree

These are not stylistic differences; each is a rule with a test and a golden in
this repository. They are the reason "configure a different engine" cannot mean
"get the same answers".

1. **Cross-region transitions exit the source only.** KerML
   `StatePerformances::StateTransitionPerformance` orders
   `guard then transitionLinkSource.exit`, so a transition between two regions
   of one composite state neither exits nor re-enters the composite, leaves the
   source region with no active state, and leaves sibling regions untouched.
   UML/SCXML LCA semantics — what every candidate implements — exit and re-enter
   the composite. Pinned by `cross_region_transition_test.go` and the
   `state_region_*` trace goldens.
2. **Do activities interleave one action per round**, in region declaration
   order, and an inline `entry action { … }` body is atomic while the
   statement-per-action `do { … }` form interleaves. No candidate models
   do-activity granularity this way.
3. **Completion-event and recall priority.** `Occurrence::incomingTransferSort`
   defaults to `earlierFirstIncomingTransferSort`: a recalled deferred event
   precedes later arrivals, and a completion event precedes both
   (`executor_common.go` `eventHeap.Less`).
4. **A state completes only after its do behavior finishes**, so completion
   transitions are scheduled against do-activity state.
5. **Time triggers are scoped to the state that declares them** — a composite
   state's timer starts on entry, survives substate movement, and is destroyed on
   exit — over a virtual clock advanced by the queue.
6. **Change events are polled**, not continuously watched (already flagged
   ⚠️ Approximate); an engine with its own eventing model would change *when*
   conditions are observed.
7. **Junction and choice are both evaluated when entered**, rather than junction
   being resolved statically before the incoming transition.
8. **Budgets and typed failures**: runaway dispatch must report
   `ErrStepLimitExceeded`-class errors naming the variable that raises the bound,
   never hang. A foreign engine's event loop must be driven under that accounting.
9. **Determinism of the trace**: 26 goldens fix the exact order of entry, exit,
   effect and do-round records.

Consequence for planning: a second engine cannot share the conformance
expectations. It needs per-engine expectations (§4.4), and its divergences must
be *declared* — a typed "this engine does not implement X" error — never a silent
different answer. That is the AGENTS.md rule against faking completeness applied
to engines.

---

## 4. Proposed design

### 4.1 Layering

Nothing changes upstream of `lower.StateGraph`. The IR stays the single source of
truth and the only engine input; engines that need a foreign model translate
*from* it.

```
symbols/resolve ─► lower.StateGraph ─┬─► engine "native"   (today's StateExecutor)
                                     └─► engine "<other>"  (translator + adapter)
                                            │
                                            └─ callbacks ─► EngineHost
                                                            (eval, behaviors,
                                                             messaging, clock,
                                                             budgets, trace)
```

### 4.2 Three new interfaces, in `internal/core/runtime`

- **`StateMachine`** — the drive/observe interface, exactly the surface in §1.3.
  `*StateExecutor` satisfies it unchanged. REPL, gRPC and tests move to the
  interface.
- **`StateEngine`** — the factory: a `Name() string`, a
  `New(host EngineHost, graph *lower.StateGraph, machine *symbols.Symbol, self *Instance) (StateMachine, error)`,
  and a `Capabilities()` report so a surface can say up front which constructs
  an engine refuses.
- **`EngineHost`** — what an engine is allowed to ask of Systemica:
  `EvalGuard(expr ast.Node, scope *symbols.Scope) (bool, error)`,
  `RunBehavior(b lower.StateBehavior, data map[string]Value) error`,
  `Post(msg Message) error` / `TakeMessage(...)`, `Now() float64`,
  `ChargeEvent()` / `ChargeDoStep()`, and `Trace() *TraceRecorder`.
  The host implementation is a thin wrapper over `*Context` — the native engine
  keeps calling `Context` directly, so the wrapper adds no indirection to the
  default path.

A registry (`runtime.RegisterStateEngine`) maps name → `StateEngine`, with
`native` registered by the package itself.
`Context.CreateStateExecutorFor` keeps its signature and gains an
engine-selecting sibling, so no existing caller changes behavior.

### 4.3 Configuration

Follow the precedent already in the tree (`budget.go`'s `SYSML_*` variables,
`SYSML_LIBRARY_PATH`), resolved in this order — most specific wins:

1. Per-call/API: `ExecuteStateRequest.engine` (new optional gRPC field),
   `%state <machine> --engine <name>` in the REPL.
2. Session: REPL `%engine <name>`, LSP setting `systemica.stateEngine`.
3. Process: `sysml --state-engine <name>`.
4. Environment: `SYSML_STATE_ENGINE=<name>`.
5. Default: `native`.

Rules, mirroring how budgets already treat a bad value: an unknown name is an
error listing the registered engines, never a silent fallback; every surface that
selected a non-default engine says so in its output (`%current`, trace header,
gRPC response field); a non-native engine carries an experimental notice using
the same pattern as `export.ExperimentalNotice` for RDF.

### 4.4 Test contract for a second engine

- The conformance runner gains an engine dimension: each case declares which
  engines it is expected to pass, defaulting to `native` only.
- Each non-native engine gets a `known_failures` list with a *reason* per case,
  reviewed like the training-examples baseline — a case is not silently dropped.
- Trace goldens stay native-only unless an engine claims trace fidelity.
- `Capabilities()` is tested against the corpus: a construct an engine declares
  unsupported must produce a typed error on every case using it.

---

## 5. Options and effort

Estimates are in focused work sessions, and assume the definition of done in
AGENTS.md §2 (build, vet, gofmt, full suite, plus the local OMG corpus gate).

### 5.1 Phase 1 — the seam only (recommended, ~1 session)

Extract `StateMachine`, `StateEngine`, `EngineHost`; add the registry; register
`native`; plumb the configuration surfaces of §4.3; migrate REPL/gRPC/tests to
the interface. Zero semantic change, all goldens untouched.

Risk: the interface is wide (~17 methods) and AST-typed. Mitigation: it is
`internal/`, so it is not a public contract, and the alternative — a narrower
interface — would hide the debugger stepping the REPL depends on.

### 5.2 Phase 2 — a reference second engine (experimental, 2–3 sessions + carrying cost)

Vendor one in-process Go statechart library, write the
`lower.StateGraph` → library-model translator, wire guards/effects back through
`EngineHost`, declare the divergences of §3 as unsupported capabilities, and land
it experimental behind the configuration flag with its own `known_failures`.

What this buys: proof the seam is real, and a second opinion on the machines that
fall inside the intersection of both semantics. What it costs: a permanent second
semantics to maintain, and the near-certainty that it can never become the
default. Worth doing only if the motivation is *validation* or *user choice*, not
*less code to maintain*.

### 5.3 Phase 3 — pre-existing engines as oracles and interop targets (recommended, 2–3 sessions)

Higher value per unit of risk than Phase 2, and it does not fork the semantics:

- **SCXML export**, alongside the existing RDF export in `internal/core/export`
  and gated with the same experimental notice, so a Systemica machine can be run
  in any W3C-conformant engine and compared.
- **Import the OMG PSSM test suite** (`PSSM_TestSuite.xmi`) as an external
  semantic oracle, adjudicated per test the way `docs/project/training-examples.md`
  adjudicates the OMG corpus — where PSSM and KerML `StatePerformances` genuinely
  differ, that difference gets recorded as a ruling instead of being discovered
  later as a bug.
- Optionally the same for the W3C SCXML test suite, restricted to the constructs
  the export covers.

### 5.4 Not recommended

Out-of-process delegation to Moka/Commons SCXML as a *runtime* engine: every
guard and effect would cross the boundary and would need the SysML expression
layer on the far side, which is the bulk of Systemica.

---

## 6. Recommendation

1. Do Phase 1. It is cheap, reversible, and turns "could we?" into a question
   answerable by experiment rather than by argument.
2. Do Phase 3 next if the motivation behind the question is trusting our
   semantics — external suites give the assurance an external engine is being
   asked for, without dual-maintaining a second engine.
3. Treat Phase 2 as an experiment behind the flag. Do not plan for a foreign
   engine to become the default: 69 rules, 64 conformance cases and 26 trace
   goldens encode KerML-derived semantics that no surveyed engine implements.

## 7. Open question

Which benefit is being sought — external validation of our semantics,
interoperability with other tools, execution performance, or reducing the amount
of engine code we maintain? Phases 1+3 serve the first two; performance is not
addressed by any candidate (the native engine already runs ~1.9M events/s per
`budget.go`); and the fourth is not achievable, because the divergences in §3
mean an imported engine adds semantics rather than replacing them.
