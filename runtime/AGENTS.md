# SysML v2 Execution Runtime — Project Guide (AGENTS.md)

Status: Planning
Date: 2026-07-30
Module: `github.com/Open-MBEE/Systemica` (Go 1.25.10)

## 0. Purpose of This Document

This is the authoritative onboarding + planning brief for building a **SysML v2
execution runtime** on top of the existing Go SysML v2 implementation. It
captures the end goal, the current state of the codebase it builds on, the
tiered architecture, the concrete near-term scope, and the constraints an agent
must respect. Read this fully before planning or writing code.

## 1. Project Goal

**End goal:** SysML v2 models can actually *execute* — not merely be parsed and
statically validated. A model author should be able to evaluate expressions,
run `calc` and `analysis` cases against concrete values, check constraints/
requirements against real instances, and (eventually) simulate behavior
(actions, state machines).

**This is a scope expansion.** The parent project (see
`docs/superpowers/specs/2026-07-25-sysml-v2-go-design.md`) was framed as a
*Language Server & REPL* — static tooling whose declared finish line was
"validation depth C" (full constraint parity). That work is essentially
complete. The runtime is the new north star beyond it.

**Near-term committed scope: the Evaluation Runtime (Tiers 1–3 below).**
Behavioral simulation (Tiers 4–5) is documented as future vision, NOT current
scope — it requires parser/AST extensions that do not yet exist.

## 2. What Already Exists (the foundation you build on)

The runtime consumes the existing static pipeline; it does **not** re-parse or
re-resolve. All existing packages live under `internal/core/`.

### 2.1 Pipeline (all green: `go build ./...`, `go vet ./...`, `go test ./...`)

`source → lexer → parser → AST → symbol index → resolve → passes`

- `internal/core/source/` — `SourceFile`, spans, line index.
- `internal/core/lexer/` — hand-written scanner. NOTE: SysML has ~200 keywords;
  `all` is a RESERVED KEYWORD (breaks fixtures if used as an identifier).
- `internal/core/parser/` — recursive-descent. Entry:
  `parser.New(source.New(name, bytes)).ParseFile() -> *ast.RootNamespace`.
  Always produces a tree (ErrorNodes on bad input). Parse order in `parseUsage`
  (parser/defusage.go): relationships FIRST, then multiplicity — surface syntax
  is `part few subsets cap [0..10];` (multiplicity after the subsets clause).
- `internal/core/ast/` — syntax-only, **immutable after parse**. `Node`
  interface = `{Span() source.Span; LeadingTrivia()/TrailingTrivia() []Trivia}`,
  `NodeBase` embedded by all nodes. KEY ARCHITECTURAL RULE: no semantic info in
  the AST; all derived data lives in **side tables keyed by node/symbol**. The
  runtime follows this rule — instance state / evaluated values go in new side
  tables, never on AST nodes.
- `internal/core/symbols/` — scope trees + qualified-name index.
  `symbols.Symbol` (symbol.go:152): `Name`, `Kind SymbolKind`, `Decl ast.Node`,
  `Visibility`, `DeclSpan`, `Scope *Scope` (child scope this decl owns, nil for
  leaves), `OwnerScope *Scope`. A decl with short+primary names registers the
  SAME `*Symbol` under two keys — **dedupe by pointer when walking**.
  `symbols.Scope`: `Parent()`, `Node()`, `Children()`, `LookupLocal(name)`,
  `LookupLocalAll(name)`, `MemberNames()` (decl order). `symbols.Index`:
  `DocumentRoot(name) *Scope`.
- `internal/core/resolve/` — lazy name resolution, memoized.
  `resolve.New(idx)`; `(*Resolver).ResolveQualified(scope, *ast.QualifiedName)
  (*symbols.Symbol, bool)`.
- `internal/core/passes/` — pluggable validation. `PassLevel` tiers
  {LevelSyntax, LevelNameResolution, LevelType, LevelConstraint}. `Pass` =
  `{Level() PassLevel; Run(ctx, name, root) []Diagnostic}`. `Context` exposes
  `Resolver() *resolve.Resolver` and `Model() *semantics.Model` (both lazy,
  memoized). `DefaultRegistry()` registers SyntaxPass, NameResolutionPass,
  TypeCheckPass, ConstraintPass. Higher tiers skipped if a lower tier errored.
- `internal/core/model/` — `Workspace` (workspace.go): single source of truth,
  owns document set + global index + diagnostic cache; `Document` holds
  source/AST/scope/version. One `Workspace` per LSP/REPL session.
- `internal/lsp/` + `cmd/sysml-lsp/` — LSP server (static analysis only).
- `internal/repl/` + `cmd/sysml-repl/` — interactive scratchpad (static only).

### 2.2 The Semantic Model — `internal/core/semantics/`

This is the runtime's primary substrate. Built via `NewModel(*resolve.Resolver)`.
All results memoized in side tables keyed by `*symbols.Symbol`.

- `model.go`:
  - `DirectSupertypes(sym) []*symbols.Symbol` — resolved generalization edges
    (specializes/subsets/redefines/typing).
  - `AllSupertypes(sym)` — transitive, cycle-safe.
  - `Conforms(a, b) bool`.
  - `HasSpecializationCycle(sym) bool`.
  - `generalizationKind`, `relationshipsOf` — UNEXPORTED (copy locally if needed
    in another package, as passes/constraint.go does).
- `members.go`: `MembersOf(sym)`, `LookupMember(sym, name)` — local + inherited
  with masking. This is the substrate for "effective feature list per type".
- `multiplicity.go`: `MultiplicityOf(sym) (Range, bool)` (ok only for `*ast.Usage`
  with non-nil `.Multiplicity`). `Range{Lower, Upper Bound}`;
  `Bound{Value int64, Infinite bool, Known bool}`; `Range.LowerLeUpper()`.
- `eval.go`: **`Eval(n ast.Node) (Value, bool)` — the seed of the runtime.**
  Currently a CONSTANT-FOLDER only. `Value{Kind ValueKind, Int int64,
  Real float64, Bool bool}`; `ValueKind` ∈ {ValInvalid, ValInt, ValReal,
  ValBool, ValInfinity}. Handles literals + unary/binary/conditional operators
  over int/real/bool/infinity. Returns `ok=false` for feature references,
  strings, null, invocations, collections. Exists only to evaluate multiplicity
  bounds + simple guards. **Tier 3 grows this into a real evaluator.**

### 2.3 Expression AST — RUNTIME-READY (no parser work needed for Tier 3)

`internal/core/ast/expr.go` already models far more than `Eval` handles:

- `FeatureReference{Name *QualifiedName}`
- `FeatureChainExpr{Operand Node, Member *QualifiedName}` — `x.y.z`
- `InvocationExpr{Operand, Type, Args, NamedArgs}` — calc/function invocation
- `CollectExpr` (`.`), `SelectExpr` (`.?`), `IndexExpr`
- `SequenceExpr{Elements}`, `ConstructorExpr{Type, Args}` (`new`)
- `BodyExpr{Params []BodyParam, Result Node}` — lambda
- `MetadataAccessExpr`, `NullExpr`
- `LiteralBool/String/Integer/Real/Infinity` — literals store RAW TEXT strings;
  the evaluator must parse them to numbers (as `Eval` does with `strconv`).
- Full `OperatorKind` ladder: OpConditional, OpNullCoalesce, OpImplies, boolean
  ops, OpEq..OpGe, OpHasType/OpIsType/OpAs, OpAt, OpRange, arithmetic, OpNeg,
  OpNot, OpAll, OpIndex.

Implication: **Tier 3 = implement behavior for AST that already exists.** The
parser needs no changes for the evaluation runtime.

### 2.4 The Behavioral Gap — NOT reusable (why Tiers 4–5 are future)

`ast.Definition` and `ast.Usage` (defusage.go) carry structural fields only.
`Usage` has Tier-B additions (`ConnectorEnds`, `FlowEnds{From,To,Payload}`,
`IsConjugated`) showing the additive extension pattern. But:

- NO action-node graph (fork/join/merge/decision/initial/final).
- NO succession/control edges, NO item/token flows.
- NO state transitions (trigger/guard/effect), NO entry/exit/do behaviors.
- `action`/`state` bodies are undifferentiated `Members []Node`.

Behavioral simulation therefore REQUIRES new AST nodes + parser grammar FIRST,
following the additive Tier-B pattern. Out of near-term scope.

## 3. Runtime Architecture — Tiers

Built in order; each tier is a prerequisite for the next. Tiers 1–3 reuse
`semantics`/`model`; they live in a NEW package (proposed `internal/core/runtime/`)
beside `semantics`, NOT as a `passes.Pass` (execution is stateful/iterative/
value-producing — a fundamentally different shape than a diagnostic-emitting
pass).

### Tier 1 — Feature flattening → "type shape" (near-term)
Harden `MembersOf` into a stable, ordered **effective-feature list** per type:
own + inherited − redefined/masked, each entry carrying type + multiplicity +
default-value expression. This is the schema every later tier reads.

### Tier 2 — Instance / value model (near-term, the missing core)
New `runtime` package:
- **Value**: extend beyond `Eval`'s int/real/bool/infinity to include `null`,
  strings, **instance references**, and **collections** (multiplicity-aware
  sequences/sets). Decide: extend `semantics.Value` vs. a new richer runtime
  Value type (recommend a new type; keep `semantics.Value` as the constant
  subset).
- **Instance**: a typed object with one slot per effective feature (Tier 1),
  each slot holding a Value or unset.
- **Instantiation**: materialize an instance graph from a `part`/`item` usage —
  recursively instantiate composite features, wire references; multiplicity
  governs slot cardinality.

### Tier 3 — Expression evaluator (near-term, highest value-per-effort)
Grow evaluation from constant-folder to full evaluator over the instance model:
- Feature access `x.y.z` resolved against instance slots (uses resolver + Tier-1
  shape).
- KerML operator/collection library (`->select`, `->collect`, `size`, string
  ops, `null`/empty-collection semantics).
- `calc` invocation: bind args → evaluate body → return.
- Unlocks: constraint checking against concrete values; `analysis`/`calc`
  evaluation (Tier 6 analysis driver).

### Tier 4 — Behavioral AST (FUTURE — needs parser/AST work)
Parse + model action bodies (nodes, succession/control edges, token flows,
param directions) and state machines (states, transitions, entry/exit/do).
New AST nodes + parser grammar + a resolution/type pass over them.

### Tier 5 — Behavioral interpreter + scheduler (FUTURE)
Token-flow execution for actions (Petri-net-like), event-driven stepping for
state machines, a scheduler to quiescence/step-bound, optional occurrence/time
model. Deterministic where the spec allows.

### Tier 6 — Verification & analysis drivers, execution driver, observability
- Analysis case: subject/instances → calc chain → result values (needs Tier 3).
- Verification case: run against instance → evaluate requirement constraints →
  pass/fail (needs Tier 3; richer with Tier 5).
- Entry point: REPL/LSP command ("run this case"/"simulate this action").
- Trace + result reporting; bounded steps / termination guards (mirror `Eval`'s
  existing bounding).

## 4. Guardrails & Conventions

- **Never mutate the AST.** Runtime state lives in side tables keyed by
  node/symbol, per §2.1.
- **Dedupe symbols by pointer** when walking scopes (short+primary alias the
  same `*Symbol`).
- **Runtime is not a Pass.** Do not shoehorn execution into `passes.Pass`.
  Execution gates on a constraint-valid model but is a separate subsystem
  consuming pass output.
- **Reuse `semantics.Model`** (resolver, MembersOf/LookupMember, MultiplicityOf,
  Eval) rather than re-deriving.
- **Additive extension** for any behavioral AST work (follow the Tier-B
  `FlowEnds`/`ConnectorEnds` pattern on `Usage`).
- **Fixtures:** avoid reserved keywords (`all`, etc.) as identifiers; put
  multiplicity after subsets clauses.
- **Verification before "done":** `go build ./...`, `go vet ./...`,
  `go test ./...` must all be green; `gofmt` your files. Do not commit unless
  explicitly asked.
- **Grammar/semantics reference:** pilot Xtext (`SysML.xtext` +
  `KerMLExpressions`), metamodel `https://www.omg.org/spec/SysML/20250201`, and
  the vendored `SysML-v2-Pilot-Implementation/` (incl. its Jupyter kernel, which
  demonstrates expression evaluation — the only "runtime" precedent in-tree).

## 5. Immediate Next Step

Run the brainstorming → writing-plans flow to turn Tiers 1–3 into a concrete,
staged implementation plan (files, types, package layout under
`internal/core/runtime/`, test strategy) against the current `semantics`/`model`
packages. Start with Tier 1 (feature flattening) since Tiers 2–3 depend on it.
