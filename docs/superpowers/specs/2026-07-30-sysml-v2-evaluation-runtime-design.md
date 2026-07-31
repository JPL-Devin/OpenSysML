# SysML v2 Evaluation Runtime Design

**Date:** 2026-07-30  
**Status:** Draft  
**Scope:** Tiers 1–3 (Feature Flattening, Instance Model, Expression Evaluator)

---

## 0. Executive Summary

This design spec defines the **SysML v2 Evaluation Runtime** — a new package (`internal/core/runtime`) that enables SysML v2 models to **execute**, not merely parse and validate. It adds three capability tiers atop the existing static analysis pipeline:

1. **Tier 1 — Feature Flattening:** Produce a stable, ordered effective-feature list per type (own + inherited − redefined/masked), each entry carrying type, multiplicity, and default-value expression. This is the "type schema" every instance reads.

2. **Tier 2 — Instance Model:** Materialize typed instances with one slot per effective feature. Instance identity via explicit `int64` IDs. Lazy instantiation: composite features (nested parts) materialize on first access, not eagerly. Supports multiplicity-driven collections (Sequence/Set).

3. **Tier 3 — Expression Evaluator:** Evaluate expressions over instances. Handles literals, operators, feature access (`inst.x.y`), calc invocation (parameter binding + frame stack), and KerML collection operators (`ControlFunctions::select`, `SequenceFunctions::size`, etc.) as hardcoded Go builtins. Step-counter runaway guard (default 100k steps).

**End goal unlocked:** Evaluate `calc` and `analysis` cases against concrete values, check constraints/requirements against real instances, compute derived attributes. Behavioral simulation (actions, state machines) deferred to future Tiers 4–5 (requires parser/AST extensions).

**Integration:** Runtime consumes pass-validated models (constraint-clean). Plugs into LSP (hover eval, code actions) and REPL (`:eval`, `:instantiate`, `:run` commands). One `Context` per workspace session.

---

## 1. Goals & Scope

### 1.1 What This Adds

**Core capability:** SysML v2 models can **execute** — not merely be statically analyzed. A model author can:

- Instantiate `part` / `item` usages into concrete instances with typed slots
- Evaluate expressions (arithmetic, boolean, feature access, calc invocation) over instance graphs
- Run `calc` / `analysis` cases against concrete values and retrieve results
- Check `constraint` / `requirement` predicates against real instances (pass/fail)
- Compute derived attributes (`attribute mass = volume * density;`)

**What this enables:**
- LSP: "Evaluate expression" hover/code-action; show calc results inline
- REPL: `:eval <expr>`, `:instantiate <part>`, `:run <calc>` commands
- Foundation for verification cases (Tier 6): evaluate requirements against test instances

### 1.2 What This Builds On

The runtime is a **pure consumer** of the existing pipeline. It reuses, does not replace:

- **Lexer/Parser** (`internal/core/lexer`, `internal/core/parser`) — Expression AST already exists (`FeatureReference`, `InvocationExpr`, `FeatureChainExpr`, etc.). Tier 3 implements evaluation semantics for nodes that already parse.
- **Symbol index + resolver** (`internal/core/symbols`, `internal/core/resolve`) — Runtime resolves feature names, types, qualified names via the existing `resolve.Resolver`.
- **Semantic model** (`internal/core/semantics`) — Runtime directly calls `Model.MembersOf()` (inherited features), `Model.MultiplicityOf()` (bounds), `Model.Eval()` (constant folding), `Model.AllSupertypes()` (type hierarchy).
- **Passes** (`internal/core/passes`) — Runtime assumes constraint-valid models (all passes green). Execution gates on `LevelConstraint` success.
- **Workspace/Model** (`internal/core/model`) — One `runtime.Context` per workspace session, backed by the workspace's `semantics.Model`.

**Key architectural rule (inherited from AST design):** No mutation of the AST. Runtime state (instances, evaluated values, execution frames) lives in **side tables** keyed by symbol/instance-ID, never on AST nodes.

### 1.3 Out of Scope (Tiers 4–5)

**Behavioral simulation is NOT part of this design.** Actions (fork/join/merge/decision nodes, succession edges, token flows) and state machines (transitions, guards, entry/exit/do behaviors) require:

- New AST nodes (action-graph nodes, control/flow edges, state/transition structures)
- Parser grammar extensions (`action` / `state` body parsing currently undifferentiated)
- A resolution/type pass over behavioral constructs

Tiers 4–5 (behavioral runtime) are **future work** following the additive Tier-B pattern (see AGENTS.md §2.4). This spec scopes **evaluation runtime only** (Tiers 1–3): expressions, instances, calcs.

---

## 2. Design Principles

### 2.1 Spec Compliance

**In all cases of ambiguity, prefer SysML v2.0 spec compliance over convenience.**

- **Authoritative references:**
  - SysML v2 Specification (OMG 2025-02-01): `https://www.omg.org/spec/SysML/20250201`
  - KerML Standard Library: `SysML-v2-Pilot-Implementation/sysml.library/Kernel Libraries/`
  - Pilot Xtext grammar: `SysML-v2-Pilot-Implementation/org.omg.kerml.xtext/src/org/omg/kerml/xtext/`

- **Concrete applications:**
  - KerML builtins use **fully-qualified spec names** (`ControlFunctions::select`, not short-name `select`)
  - Collection semantics follow `ordered`/`nonunique` modifiers per KerML `Collections.kerml` taxonomy
  - Multiplicity bounds match KerML semantics (single-bound `[n]` means lower=upper=n)
  - Evaluation order, null-handling, type coercion follow the spec where defined

- **When spec is silent:** Follow the pilot implementation's precedent (Jupyter kernel for expression evaluation, Xtext semantics validator for type rules). Document the choice.

### 2.2 Architectural Constraints

**Never mutate the AST.** Runtime state lives in side tables keyed by symbol/instance-ID, per the project-wide immutability rule (inherited from `internal/core/ast` design). Instances, evaluated values, frame stacks — all external to AST nodes.

**Reuse `semantics.Model` maximally.** Don't re-derive inherited features, multiplicity, conformance, or constant evaluation. The semantic model is the substrate; runtime adds execution on top.

**Additive extension only.** If behavioral tiers need new AST fields, follow the Tier-B pattern (`Usage.FlowEnds`, `Usage.ConnectorEnds` — fields added to existing nodes, not new node types unless unavoidable).

**Tier dependencies are logical, not package boundaries.** Tiers 1–3 live in one `runtime` package. Tier 2 depends on Tier 1 (needs `FeaturesOf`), Tier 3 depends on Tier 2 (needs instances). Enforced by call graph, not import structure.

---

## 3. Package Structure

### 3.1 Location & Files

**Package:** `internal/core/runtime/` (new, lives beside `semantics`)

**Files:**
- `context.go` — `Context` (ID allocator, step counter, semantic model ref, instance registry, feature cache)
- `value.go` — `Value`, `Sequence`, `Set` (runtime value types)
- `shape.go` — `EffectiveFeature`, `FeaturesOf()` (Tier 1: feature flattening)
- `instance.go` — `Instance`, `Slot`, `Instantiate()` (Tier 2: instance model)
- `eval.go` — `EvalContext`, `Eval()` (Tier 3: evaluator core, frame stack)
- `builtins.go` — KerML collection/string operators (Tier 3: hardcoded library)

**Tests:**
- `shape_test.go` — Tier 1 unit tests
- `instance_test.go` — Tier 2 unit tests
- `eval_test.go` — Tier 3 evaluator unit tests
- `builtins_test.go` — Tier 3 builtin function tests
- `runtime_integration_test.go` — End-to-end scenarios

### 3.2 Core Types Overview

```go
// Context carries runtime execution state. One per workspace session.
type Context struct {
    model      *semantics.Model                           // semantic substrate
    nextID     int64                                      // instance ID allocator
    steps      int64                                      // eval step counter
    maxSteps   int64                                      // runaway guard (default 100_000)
    instances  map[int64]*Instance                        // ID → instance registry
    features   map[*symbols.Symbol][]EffectiveFeature     // memoized FeaturesOf results
}

// Value is a runtime-evaluable value.
type Value struct {
    Kind      ValueKind
    Const     semantics.Value  // ValConst: reuse static evaluator's int/real/bool/infinity
    Str       string           // ValString
    Instance  int64            // ValInstance: instance ID
    Sequence  *Sequence        // ValSequence
    Set       *Set             // ValSet
}

type ValueKind int
const (
    ValInvalid ValueKind = iota
    ValConst   // wraps semantics.Value (int, real, bool, infinity)
    ValNull
    ValString
    ValInstance
    ValSequence
    ValSet
)

// Sequence is an ordered collection (slice-backed).
type Sequence struct {
    elements []Value
}

// Set is an unordered unique collection (map-backed).
// Requires Value equality/hashing (see §11.4).
type Set struct {
    elements map[Value]struct{}
}

// EffectiveFeature is one entry in a type's effective feature list (Tier 1).
type EffectiveFeature struct {
    Feature      *symbols.Symbol    // feature symbol
    Type         *symbols.Symbol    // resolved type (via typing relationship), nil if untyped
    Multiplicity semantics.Range    // bounds (via MultiplicityOf), zero if no multiplicity
    DefaultValue ast.Node           // Usage.Value expression, nil if no default
}

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
    ID    int64                 // unique identity (for reference equality, debugging)
    Type  *symbols.Symbol       // the def/usage symbol this instantiates
    Slots map[string]*Slot      // feature name → slot
}

// Slot holds the runtime value(s) for one feature.
type Slot struct {
    Feature      *EffectiveFeature
    Value        Value   // scalar slot (multiplicity [1])
    Values       Value   // collection slot (Sequence or Set)
    Materialized bool    // lazy flag: composite features instantiate on first access
}

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
    ctx    *Context              // runtime context
    frames []map[string]Value    // stack of local bindings (calc params, lambda params)
}
```

**Design rationale:** Single package (not sub-packages per tier) mirrors `semantics` structure. Tiers 1–3 are one conceptual unit ("evaluation runtime"); package boundaries would add import churn without enforcement benefit.

---

## 4. Tier 1: Feature Flattening (Type Shape)

### 4.1 Goal

Produce a stable, ordered **effective-feature list per type**: own + inherited − redefined/masked. Each entry carries:
- Feature symbol
- Resolved type (via `typing` / `:` relationship)
- Multiplicity bounds (via `MultiplicityOf`)
- Default-value expression (via `Usage.Value`)

This is the "schema" every instance reads when allocating slots. Tier 2 instantiation walks this list; Tier 3 feature access resolves against it.

### 4.2 Types

```go
// EffectiveFeature is one slot in a type's effective feature list.
type EffectiveFeature struct {
    Feature      *symbols.Symbol    // the feature symbol (from MembersOf)
    Type         *symbols.Symbol    // resolved type, nil if untyped
    Multiplicity semantics.Range    // extracted multiplicity, zero if none
    DefaultValue ast.Node           // Usage.Value expression, nil if no default
}
```

### 4.3 API

```go
// FeaturesOf returns the effective feature list for a type symbol: all visible
// features (own + inherited with masking), in stable order (local first, then
// supertypes breadth-first). Result is memoized in ctx.features.
func (ctx *Context) FeaturesOf(sym *symbols.Symbol) []EffectiveFeature
```

### 4.4 Implementation Strategy

1. **Get members:** Call `ctx.model.MembersOf(sym)` → `[]*symbols.Symbol` (dedupe short+primary aliases by pointer).
2. **For each member:**
   - **Type:** Walk `semantics.RelationshipsOf(member)`, find first `RelTyping`, resolve target via `ctx.model.resolver.ResolveQualified(scope, target)`.
   - **Multiplicity:** Call `ctx.model.MultiplicityOf(member)` (only populated for `*ast.Usage` decls with non-nil `.Multiplicity`).
   - **Default value:** If `member.Decl` is `*ast.Usage`, read `.Value` field (expression node or nil).
3. **Build `EffectiveFeature` slice, memoize in `ctx.features[sym]`.**

**Memoization:** First call computes, subsequent calls return cached. Stable across workspace session (no invalidation needed unless model changes, which triggers full workspace reindex anyway).

**Testing:** Unit tests against fixture types with inheritance chains, redefinition, multiplicity overrides. Verify feature order, masking correctness, type/multiplicity/default extraction.

---

## 5. Tier 2: Instance Model

### 5.1 Goal

Materialize **instances** from usage declarations: typed objects with one slot per effective feature. Instances hold runtime values (scalar or collection), support lazy instantiation of composite features (nested parts materialize on first access, not eagerly).

### 5.2 Instance & Slot Types

```go
// Instance is a runtime-materialized object.
type Instance struct {
    ID    int64                 // unique identity (explicit int64, not pointer identity)
    Type  *symbols.Symbol       // the def/usage symbol this instantiates
    Slots map[string]*Slot      // feature name → slot
}

// Slot holds the runtime value(s) for one feature.
type Slot struct {
    Feature      *EffectiveFeature
    Value        Value              // scalar slot (multiplicity [1])
    Values       Value              // collection slot (Sequence or Set)
    Materialized bool               // lazy flag: has this slot been instantiated?
}
```

### 5.3 Instance Identity (Explicit ID)

**Choice:** Instances carry an explicit `int64` ID (not pointer identity).

**Rationale:**
- **Serialization-friendly:** Can snapshot/load instance graphs, compare runs, persist state.
- **Debuggable:** IDs are stable, meaningful (`instance #42` in traces, not `0x14a2c40`).
- **Test-friendly:** Tests can construct expected instances with known IDs, compare via ID equality.

**ID allocation:** `Context` holds `nextID int64`; each `Instantiate()` call increments: `inst.ID = ctx.nextID; ctx.nextID++`.

**Instance registry:** `Context` holds `instances map[int64]*Instance` for ID → instance lookup (needed when evaluating `ValInstance` references in expressions).

### 5.4 Lazy Instantiation

**Goal:** Don't materialize composite features (nested parts) until accessed. A `part wheels[4]` feature doesn't create 4 wheel instances at parent-instantiation time; they're created on first `GetSlot("wheels")` call.

**Implementation:**
- **At instantiation time:** For each feature in `FeaturesOf(type)`:
  - Create `Slot{Feature: &feat}`
  - If scalar (multiplicity [1]): evaluate `feat.DefaultValue` (if present) → store in `Slot.Value`
  - If collection: leave `Materialized = false`
- **On access (`Instance.GetSlot(ctx, name)`):**
  - If `Slot.Materialized == false` and feature is composite (type is a part/item def): recursively `Instantiate(feat.Type)` up to multiplicity bounds, store in `Slot.Values` (Sequence or Set depending on `ordered`/`nonunique` modifiers)
  - Mark `Materialized = true`

**Multiplicity bounds for lazy instantiation:**
- Lower bound (e.g., `[2..*]`) → materialize `lower` instances initially
- Access beyond materialized count → grow collection up to `upper` (or error if `upper` finite and exceeded)
- Infinite upper (`[0..*]`) → grow on demand, bounded by access pattern

**Collection type (Sequence vs Set):** Check `feat.Feature.Decl.(*ast.Usage).IsOrdered` and `.IsNonunique`:
- `ordered` + `unique` → Sequence (spec default; unique enforcement TBD — may error on duplicates or dedup silently)
- `ordered` + `nonunique` → Sequence
- `!ordered` + `unique` → Set
- `!ordered` + `nonunique` → Bag (unsupported in Tier 3; use Sequence with warning)

### 5.5 API

```go
// Instantiate materializes an instance of the given usage/definition symbol.
// Allocates ID, creates slots per FeaturesOf(sym), evaluates default values,
// leaves composite features lazy. Returns the instance or an error.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error)

// GetSlot retrieves the slot for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet.
func (inst *Instance) GetSlot(ctx *Context, name string) (*Slot, error)
```

**Testing:** Unit tests for simple part defs, nested composite features, multiplicity `[0..*]`, `[2..5]`, default-value evaluation. Verify ID allocation, lazy instantiation (composite slots not materialized until `GetSlot` called), collection type selection.

---

## 6. Value Model (Tier 2 Foundation)

### 6.1 Goal

Extend value representation beyond `semantics.Value` (int/real/bool/infinity — the constant subset) to include null, strings, instance references, and collections. Keep `semantics.Value` immutable/constant-only; runtime adds execution state.

### 6.2 runtime.Value Type

```go
// Value is a runtime-evaluable value.
type Value struct {
    Kind      ValueKind
    Const     semantics.Value  // ValConst: reuse static evaluator's result
    Str       string           // ValString
    Instance  int64            // ValInstance: instance ID (not pointer)
    Sequence  *Sequence        // ValSequence
    Set       *Set             // ValSet
}

type ValueKind int
const (
    ValInvalid ValueKind = iota
    ValConst   // int/real/bool/infinity from semantics.Value
    ValNull
    ValString
    ValInstance
    ValSequence
    ValSet
)
```

**Design rationale:**
- **Wraps `semantics.Value`** in the `Const` field → reuses constant folder (`semantics.Eval`) without duplicating int/real/bool/infinity handling.
- **Instance references store ID** (int64), not pointer → serialization-friendly, debuggable, test-friendly (per §5.3).
- **Collections are pointers** (`*Sequence`, `*Set`) → mutable (can grow during lazy instantiation), reference-semantics (avoid copying large collections).

### 6.3 Collections (Sequence vs Set)

**Distinct types** (not a union + flags):

```go
// Sequence is an ordered collection (slice-backed).
type Sequence struct {
    elements []Value
}

// Set is an unordered unique collection (map-backed).
type Set struct {
    elements map[Value]struct{}
}
```

**Rationale:**
- **Type safety:** Sequence uses slice (ordered), Set uses map (unique + O(1) membership). Right data structure per semantics.
- **Clear intent:** Code that needs ordering uses `Sequence`, code that needs uniqueness uses `Set`. No runtime flag checks.

**Operations:**
- **Sequence:** `Append(val)`, `At(index)`, `Size()`, `Elements()` → `[]Value`
- **Set:** `Add(val)`, `Contains(val)`, `Size()`, `Elements()` → `[]Value` (arbitrary order)

### 6.4 Value Equality & Hashing (for Set)

**Problem:** Go structs with slices/maps are not comparable by default. `Set` uses `map[Value]struct{}` → requires `Value` to be comparable.

**Solution options:**

**A. Implement custom equality/hashing:**
```go
func (v Value) Hash() uint64 { ... }
func (v Value) Equal(other Value) bool { ... }
```
Set internally uses a wrapper map: `map[valueKey]*Value` where `valueKey` is a comparable projection (e.g., `(Kind, Int, Str, Instance)` tuple for scalars; hash for collections).

**B. Use comparable projection as map key:**
```go
type valueKey struct {
    kind     ValueKind
    intVal   int64    // for ValConst int
    realVal  float64  // for ValConst real (caveat: float equality fragile)
    boolVal  bool     // for ValConst bool
    strVal   string   // for ValString
    instID   int64    // for ValInstance
    seqHash  uint64   // for ValSequence (hash of elements)
    setHash  uint64   // for ValSet (hash of elements, order-invariant)
}
```
Extract `valueKey(v Value)` → use as map key. Collections hash recursively (content-based equality).

**Recommendation:** Option B (comparable projection). Simpler than custom interface; Go's built-in map handles hashing. Deep equality for collections (recursive hash) ensures `(1, 2, 3)` and `(1, 2, 3)` are the same set element.

**Note:** Float equality in `realVal` is fragile (IEEE 754 precision). Document that `Set` containing reals may have unexpected equality behavior (spec-compliant: KerML allows reals in sets, but float equality is implementation-defined).

---

## 7. Tier 3: Expression Evaluator

### 7.1 Goal

Evaluate expressions over the instance model. Handle literals, operators (reuse `semantics.Eval` for constants), feature access (`inst.x.y`), calc invocation (parameter binding + frame stack), collection operators (`ControlFunctions::select`, etc.).

### 7.2 Evaluation Context (Frame Stack)

**Lexical environment:** Stack of local bindings for calc parameters, lambda params, loop variables.

```go
// EvalContext is the lexical environment during evaluation.
type EvalContext struct {
    ctx    *Context              // runtime context (instances, ID allocator, step counter)
    frames []map[string]Value    // stack of local bindings (innermost = frames[len-1])
}

// Push a new frame (on calc invocation, lambda entry).
func (ec *EvalContext) Push(bindings map[string]Value)

// Pop the top frame (on return, lambda exit).
func (ec *EvalContext) Pop()

// Lookup a name in the frame stack (innermost first).
func (ec *EvalContext) Lookup(name string) (Value, bool)
```

**Frame stack usage:**
- **Calc invocation:** Eval args → `Push({param1: arg1Val, param2: arg2Val})` → eval body → `Pop()` → return result
- **Lambda (`BodyExpr`):** Eval operand → iterate elements → for each: `Push({paramName: element})` → eval body → `Pop()`

### 7.3 Eval API

```go
// Eval evaluates an expression node in the given context. Returns a Value or
// an error (unresolved reference, type mismatch, step limit exceeded).
// Increments ctx.steps on each eval call; errors when ctx.steps >= ctx.maxSteps.
func (ec *EvalContext) Eval(node ast.Node) (Value, error)
```

**Top-level entry point (for LSP/REPL):**
```go
// Eval evaluates an expression in an empty environment (no local bindings).
// For instance-relative expressions, caller must set up an EvalContext with
// appropriate frames.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
    ec := &EvalContext{ctx: ctx, frames: nil}
    return ec.Eval(node)
}
```

### 7.4 Evaluation Dispatch (by AST Node Type)

**Core dispatch loop (`Eval(node ast.Node)`:** switch on node type:

**1. Literals:**
- `*ast.LiteralInteger` → parse via `strconv.ParseInt(e.Value, 10, 64)` → `Value{Kind: ValConst, Const: semantics.Value{Kind: ValInt, Int: ...}}`
- `*ast.LiteralReal` → parse via `strconv.ParseFloat(e.Value, 64)` → `ValConst` with `ValReal`
- `*ast.LiteralBool` → `ValConst` with `ValBool`
- `*ast.LiteralInfinity` → `ValConst` with `ValInfinity`
- `*ast.LiteralString` → `Value{Kind: ValString, Str: e.Value}` (strip quotes)
- `*ast.NullExpr` → `Value{Kind: ValNull}`

**2. OperatorExpr (arithmetic, boolean, comparison):**
- **Try constant folding first:** `val, ok := ec.ctx.model.Eval(node)`; if `ok`, wrap in `Value{Kind: ValConst, Const: val}`
- **If constant folder returns `ok=false`:** Recursively eval operands → apply operator over `runtime.Value`:
  - Arithmetic on `ValConst` int/real (reuse semantics logic or inline)
  - Comparison: handle `ValNull` (null == null → true, null == anything-else → false per KerML)
  - Equality (`==`, `!=`, `===`, `!==`): instance references compare by ID, collections compare deep-equal (recursive)
  - Null-coalesce (`??`): `left ?? right` → eval left; if `ValNull`, eval right

**3. FeatureReference (`x` or `Foo::bar`):**
- **Resolve name:** `ec.ctx.model.resolver.ResolveQualified(scope, name)`
- **Check frame stack first:** `val, ok := ec.Lookup(name.String())` → if found, return (local binding)
- **Otherwise:** Unbound reference → error (instance-relative access requires `FeatureChainExpr`)

**4. FeatureChainExpr (`x.y.z`):**
- **Eval operand:** must be `ValInstance`
- **Lookup instance:** `inst := ec.ctx.instances[operand.Instance]`
- **Get slot:** `slot, err := inst.GetSlot(ec.ctx, member.Name)` (materializes lazily if composite)
- **Return slot value:** `slot.Value` (scalar) or `slot.Values` (collection)

**5. InvocationExpr (calc calls, KerML operators):**
- **Resolve target:** `targetSym, ok := ec.ctx.model.resolver.ResolveQualified(scope, inv.Type)`
- **Get qualified name:** `qualName := symbolQualifiedName(targetSym)` (e.g., `"ControlFunctions::select"`)
- **Check builtin registry:** `if fn, ok := builtins[qualName]; ok { ... }` (see §8)
- **Otherwise, user-defined calc:**
  1. Eval args: `argVals := []Value{...}`
  2. Extract param names from calc body (scan `.Members` for param usages or use convention)
  3. Build bindings: `map[string]Value{param1: argVals[0], ...}`
  4. Push frame: `ec.Push(bindings)`
  5. Eval body: extract result expression (see §11.1 for heuristic)
  6. Pop frame: `ec.Pop()`
  7. Return result

**6. CollectExpr (`col.{expr}`):**
- Eval operand → must be `ValSequence` or `ValSet`
- Iterate elements: for each `elem`, `ec.Push({it: elem})` (or extract param from body), eval body, collect results
- `ec.Pop()` per iteration
- Return new Sequence of results

**7. SelectExpr (`col.?{pred}`):**
- Eval operand → must be collection
- Iterate elements: for each `elem`, `ec.Push({it: elem})`, eval body (must be bool), if true → keep elem
- `ec.Pop()` per iteration
- Return filtered Sequence

**8. SequenceExpr (`(a, b, c)`):**
- Eval each element → build `Sequence{elements: [val1, val2, val3]}`
- Return `Value{Kind: ValSequence, Sequence: &seq}`

**9. ConstructorExpr (`new Type(args)`):**
- Resolve `Type` → must be a part/item def
- Call `ec.ctx.Instantiate(typeSym)` → returns `*Instance`
- If args present: set slots via arg bindings (TBD: constructor semantics not fully defined in spec — defer to implementation phase)

**10. Unsupported (Tier 3):**
- `IndexExpr` (`operand#(index)`) — future
- `MetadataAccessExpr` — future
- Behavioral constructs (action nodes, state transitions) — out of scope (Tier 4–5)

### 7.5 Step Counter (Runaway Guard)

**Goal:** Prevent infinite loops, deep recursion, or large allocations from hanging eval.

**Mechanism:** Step counter in `Context`:
```go
type Context struct {
    steps    int64
    maxSteps int64  // default 100_000
}
```

**Enforcement:** At the start of every `Eval` call:
```go
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
    ec.ctx.steps++
    if ec.ctx.steps >= ec.ctx.maxSteps {
        return Value{}, fmt.Errorf("evaluation step limit exceeded (%d steps)", ec.ctx.maxSteps)
    }
    // ... dispatch
}
```

**Step definition:** One step = one `Eval` call (one AST node evaluation). A calc invocation with 10 subexpressions → 11 steps (invocation + 10 operands). A `select` over 100 elements → ~200 steps (operand eval + 100 iterations × 2 evals each).

**Configuration:** `NewContext(model, maxSteps int64)` allows caller to override default. LSP might use lower limit (10k for hover eval), REPL might use higher (1M for user scripts), tests might disable (set to `math.MaxInt64`).

**Testing:** Unit test that constructs a recursive calc (infinite loop) and verifies step-limit error triggers.

---

## 8. KerML Builtins (Tier 3)

### 8.1 Goal

Provide essential KerML collection/string operators as hardcoded Go functions. Use **spec-compliant qualified names** from the KerML standard library (`ControlFunctions::select`, `SequenceFunctions::size`, etc.).

### 8.2 Builtin Functions (Spec-Compliant Names)

**Minimal set for Tier 3:**

**From `SequenceFunctions` (`sysml.library/Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml`):**
- `SequenceFunctions::size` → int (collection length)
- `SequenceFunctions::isEmpty` → bool
- `SequenceFunctions::includes` → bool (seq1 includes seq2)

**From `ControlFunctions` (`sysml.library/Kernel Libraries/Kernel Function Library/ControlFunctions.kerml`):**
- `ControlFunctions::select` → filtered collection (filter via predicate lambda)
- `ControlFunctions::collect` → mapped collection (map via lambda)

**From `CollectionFunctions` (`sysml.library/Kernel Libraries/Kernel Function Library/CollectionFunctions.kerml`):**
- `CollectionFunctions::size` → int
- `CollectionFunctions::isEmpty` → bool

**String operations (if `ScalarFunctions` / `StringFunctions` exist in stdlib):**
- `StringFunctions::size` → int (string length)
- `StringFunctions::substring` → string
- `StringFunctions::toUpperCase` / `toLowerCase` → string

**Note:** Arithmetic/comparison operators already covered by `OperatorExpr` dispatch (§7.4).

### 8.3 Builtin Registry & Dispatch

**Registry (`builtins.go`):**
```go
var builtins = map[string]func(*EvalContext, []Value) (Value, error){
    "SequenceFunctions::size":    builtinSequenceSize,
    "SequenceFunctions::isEmpty": builtinSequenceIsEmpty,
    "SequenceFunctions::includes": builtinSequenceIncludes,
    "ControlFunctions::select":   builtinSelect,
    "ControlFunctions::collect":  builtinCollect,
    "CollectionFunctions::size":  builtinCollectionSize,
    // ...
}
```

**Dispatch (in `evalInvocation`):**
```go
func (ec *EvalContext) evalInvocation(inv *ast.InvocationExpr) (Value, error) {
    // Resolve target via resolver
    targetSym, ok := ec.ctx.model.resolver.ResolveQualified(scope, inv.Type)
    if !ok {
        return Value{}, fmt.Errorf("unresolved invocation: %s", inv.Type)
    }
    
    // Get fully-qualified name
    qualName := symbolQualifiedName(targetSym) // e.g., "ControlFunctions::select"
    
    // Check builtin registry
    if fn, ok := builtins[qualName]; ok {
        args, err := ec.evalArgs(inv.Args)
        if err != nil {
            return Value{}, err
        }
        return fn(ec, args)
    }
    
    // Otherwise: user-defined calc (push frame, eval body)
    return ec.evalCalc(targetSym, args)
}
```

**Example builtin implementation:**
```go
func builtinSequenceSize(ec *EvalContext, args []Value) (Value, error) {
    if len(args) != 1 {
        return Value{}, errors.New("SequenceFunctions::size: expected 1 argument")
    }
    col := args[0]
    switch col.Kind {
    case ValSequence:
        sz := int64(len(col.Sequence.elements))
        return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: sz}}, nil
    case ValSet:
        sz := int64(len(col.Set.elements))
        return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: sz}}, nil
    default:
        return Value{}, errors.New("SequenceFunctions::size: expected collection")
    }
}

func builtinSelect(ec *EvalContext, args []Value) (Value, error) {
    if len(args) != 2 {
        return Value{}, errors.New("ControlFunctions::select: expected 2 arguments (collection, predicate)")
    }
    col := args[0]
    pred := args[1] // must be a lambda (BodyExpr) or error
    
    // Extract elements
    var elements []Value
    switch col.Kind {
    case ValSequence:
        elements = col.Sequence.elements
    case ValSet:
        elements = col.Set.Elements() // arbitrary order
    default:
        return Value{}, errors.New("ControlFunctions::select: first argument must be collection")
    }
    
    // Iterate, filter
    var result []Value
    for _, elem := range elements {
        // Push frame with element binding (TBD: extract param name from pred body)
        ec.Push(map[string]Value{"it": elem})
        predVal, err := ec.Eval(pred) // eval predicate body
        ec.Pop()
        if err != nil {
            return Value{}, err
        }
        if predVal.Kind == ValConst && predVal.Const.Kind == semantics.ValBool && predVal.Const.Bool {
            result = append(result, elem)
        }
    }
    
    return Value{Kind: ValSequence, Sequence: &Sequence{elements: result}}, nil
}
```

### 8.4 KerML Library Symbol Resolution

**Requirement:** Builtins require qualified names (`ControlFunctions::select`) to resolve via the symbol index.

**Options:**

**A. Require models to import KerML library:**
```sysml
import ControlFunctions::*;
import SequenceFunctions::*;

calc myCalc {
    in col: Integer[*];
    return col->select {in x; x > 0};
}
```
**Pro:** Spec-compliant, clean. **Con:** User burden (must know to import).

**B. Runtime injects synthetic symbols for KerML library on `Context` init:**
- On `NewContext(model)`, populate `model.index` with synthetic symbols for `ControlFunctions`, `SequenceFunctions`, etc.
- Resolver then finds these names even if model didn't import.
**Pro:** User-friendly, "just works". **Con:** Pollutes symbol index with synthetic entries.

**Recommendation (for Tier 3):** Start with **Option A** (require imports). Document in user guide. If too cumbersome, add Option B in a later increment (synthetic symbol injection is ~50 LOC, low risk).

---

## 9. Testing Strategy

### 9.1 Tier 1 Tests

**File:** `shape_test.go`

**Coverage:**
- `FeaturesOf()` on types with inheritance chains (A :> B :> C)
- Redefinition masking (child redefines parent feature → only child version appears)
- Multiplicity extraction (`[1]`, `[0..*]`, `[2..5]`)
- Type extraction (typing relationships `:` / `defined by`)
- Default-value wiring (`Usage.Value` expression)

**Fixtures:**
- Simple def with 3 features
- Def with specialization chain (3 levels)
- Def with redefined feature
- Usage with multiplicity + default value

**Assertions:**
- Feature order (local first, then supertypes breadth-first)
- Correct type symbol per feature
- Multiplicity bounds match AST
- Default-value node pointer matches `Usage.Value`

### 9.2 Tier 2 Tests

**File:** `instance_test.go`

**Coverage:**
- `Instantiate()` on simple part def (no nesting)
- ID allocation (unique IDs per instance)
- Slot creation (one per effective feature)
- Default-value evaluation (scalar slots)
- Lazy instantiation (composite features not materialized until `GetSlot` called)
- Multiplicity-driven collection instantiation (`[0..*]`, `[2..5]`)
- Ordered/nonunique modifiers (Sequence vs Set selection)

**Fixtures:**
- `part def Wheel { attribute diameter: Real; }`
- `part def Car { part wheels: Wheel[4]; }` (composite, fixed multiplicity)
- `part def Inventory { part items: Item[0..*]; }` (composite, unbounded)
- Usage with default value: `attribute mass: Real = 100.0;`

**Assertions:**
- Inst.ID unique across multiple `Instantiate()` calls
- Slot count matches `FeaturesOf(type).length`
- Scalar slot with default → `Slot.Value` populated
- Composite slot → `Slot.Materialized == false` initially
- After `GetSlot("wheels")` → `Slot.Materialized == true`, `Slot.Values` is Sequence with 4 instances
- Ordered+unique → Sequence; !ordered+unique → Set

### 9.3 Tier 3 Tests

**File:** `eval_test.go`

**Coverage:**
- Literals (int, real, bool, string, null, infinity) → correct Value kind
- Operators (arithmetic, boolean, comparison) → correct results
- Null handling (`null == null` → true, `null + 1` → error or null per spec)
- FeatureReference in frame stack (calc parameter lookup)
- FeatureChainExpr (`inst.x.y`) → slot access, lazy materialization
- Calc invocation (param binding, body eval, return)
- CollectExpr / SelectExpr (iteration, lambda eval)
- SequenceExpr (`(1, 2, 3)`) → Sequence
- Step counter (construct recursive calc → verify step-limit error)

**Fixtures:**
- Simple expressions: `1 + 2`, `3.5 * 2.0`, `true and false`
- Feature chain: inst of `part def A { part b: B; }` → eval `a.b`
- Calc: `calc sum {in x: Integer; in y: Integer; return x + y;}` → invoke with (3, 5) → 8
- Recursive calc (infinite loop) → step-limit error

**Assertions:**
- Literal evals to correct Value
- Operators match `semantics.Eval` for constants
- Feature chain returns slot value (correct instance ID if composite)
- Calc invocation returns correct result
- Step counter increments; exceeding limit triggers error

**File:** `builtins_test.go`

**Coverage:**
- Each KerML builtin function against hand-constructed Values
- `SequenceFunctions::size` on Sequence, Set
- `ControlFunctions::select` with predicate lambda
- `ControlFunctions::collect` with mapper lambda
- Error cases (wrong argument count, type mismatch)

**Fixtures:**
- `Sequence{[1, 2, 3]}` → `size` → 3
- `Sequence{[1, 2, 3]}` → `select {in x; x > 1}` → `[2, 3]`
- `Set{[a, b, c]}` → `size` → 3

**Assertions:**
- Correct result Value per KerML spec semantics
- Errors on invalid inputs

### 9.4 Integration Tests

**File:** `runtime_integration_test.go`

**Coverage:**
- End-to-end: parse model → validate → instantiate → eval expression
- Realistic model snippets (vehicle with mass calc, inventory with constraint)

**Fixtures (in `testdata/runtime/`):**
- `simple_calc.sysml`: calc that sums attributes of an instance
- `nested_parts.sysml`: part with composite features (2 levels deep) → instantiate → access nested slots
- `constraint_check.sysml`: constraint over an instance → eval → verify pass/fail

**Assertions:**
- Parse → passes → `Instantiate` succeeds → instance has expected slot count
- Eval calc over instance → result matches hand-computed value
- Eval constraint → boolean result correct

**Fixture example:**
```sysml
part def Wheel {
    attribute diameter: Real;
}

part def Car {
    part wheels: Wheel[4];
    attribute mass: Real = 1500.0;
    
    calc totalWheelDiameter {
        return wheels->collect {in w; w.diameter}->sum;
    }
}

// Integration test:
// 1. Instantiate Car
// 2. Set wheels[0..3].diameter = [0.5, 0.5, 0.5, 0.5]
// 3. Eval totalWheelDiameter → expect 2.0
```

---

## 10. Integration Points

### 10.1 Relationship to Passes

**Runtime consumes pass-validated models.** It assumes:
- Syntax valid (`LevelSyntax` passed)
- Names resolved (`LevelNameResolution` passed)
- Types checked (`LevelType` passed)
- Constraints validated (`LevelConstraint` passed)

**Execution gate:** Only run runtime operations on models with **no errors from `LevelConstraint` or higher**. LSP/REPL should check `workspace.Diagnostics()` before calling `runtime.Instantiate()` or `runtime.Eval()`.

**Not a Pass:** Runtime is **not** a `passes.Pass`. Execution is stateful (instances, frame stack), iterative (eval loops), and value-producing — fundamentally different shape than diagnostic emission. Runtime is a separate subsystem that consumes pass output.

### 10.2 LSP Integration

**Location:** `internal/lsp/`

**Features to add:**

**1. Hover on calc usage → show evaluated result (if side-effect-free):**
- User hovers over `totalMass` calc usage
- LSP calls `runtime.Eval(calcBodyExpr)` (if no instance context needed, or with a synthetic instance)
- Show result in hover: `"Result: 1500.0 kg"`

**2. "Evaluate expression" code action:**
- User selects an expression (e.g., `wheels->size`)
- LSP offers code action "Evaluate expression"
- On invoke: call `runtime.Eval(selectedExpr)`, show result in message/output panel

**3. "Instantiate part" command:**
- User invokes command on a part usage symbol (via right-click / command palette)
- LSP calls `runtime.Instantiate(partSym)`, formats instance tree as JSON/outline
- Show in custom view or output panel: `Instance #42: Car { wheels: [...], mass: 1500.0 }`

**API entry points (add to `internal/lsp/server.go` or new `internal/lsp/runtime.go`):**
```go
func (s *Server) evaluateExpression(params EvalParams) (Value, error) {
    ctx := runtime.NewContext(s.workspace.Model(), 10000) // 10k step limit for LSP
    return ctx.Eval(params.Expression)
}

func (s *Server) instantiatePart(params InstantiateParams) (*runtime.Instance, error) {
    ctx := runtime.NewContext(s.workspace.Model(), 10000)
    return ctx.Instantiate(params.PartSymbol)
}
```

### 10.3 REPL Integration

**Location:** `internal/repl/`

**New meta commands (add to `internal/repl/commands.go`):**

**1. `:eval <expr>` — Evaluate an expression:**
```
sysml> :eval 1 + 2
3

sysml> :eval (1, 2, 3)->size
3
```
**Implementation:** Parse `<expr>` → call `runtime.Eval()` → format result → print.

**2. `:run <calc>` — Evaluate a calc definition:**
```
sysml> :run totalMass
Result: 1500.0
```
**Implementation:** Resolve `<calc>` symbol → if calc requires instance context, prompt for subject or use synthetic instance → call `runtime.Eval(calcBodyExpr)` → print.

**3. `:instantiate <part>` — Materialize an instance:**
```
sysml> :instantiate myCar
Instance #1: Car
  wheels: [Instance #2, Instance #3, Instance #4, Instance #5]
  mass: 1500.0
```
**Implementation:** Resolve `<part>` symbol → call `runtime.Instantiate()` → format instance tree (recursive slot dump) → print.

**4. `:check <constraint>` — Evaluate a constraint over an instance:**
```
sysml> :check massConstraint on myCar
PASS: massConstraint
```
**Implementation:** Resolve constraint + subject instance → eval constraint body (boolean expr) → print PASS/FAIL.

**API entry points (add to `internal/repl/repl.go`):**
```go
func (r *REPL) handleEval(line string) error {
    expr := parseExpression(line) // reuse parser
    ctx := runtime.NewContext(r.workspace.Model(), 100000) // 100k steps for REPL
    val, err := ctx.Eval(expr)
    if err != nil {
        return err
    }
    fmt.Println(formatValue(val))
    return nil
}

func (r *REPL) handleInstantiate(line string) error {
    partName := strings.TrimSpace(line)
    sym := resolveSymbol(r.workspace, partName)
    ctx := runtime.NewContext(r.workspace.Model(), 100000)
    inst, err := ctx.Instantiate(sym)
    if err != nil {
        return err
    }
    fmt.Println(formatInstance(inst))
    return nil
}
```

### 10.4 Workspace/Model Integration

**Runtime `Context` lifecycle:**
- **One `Context` per workspace session** (LSP server lifetime or REPL session lifetime)
- Context holds reference to `semantics.Model` (built from workspace's resolver)
- Context caches:
  - `features map[*symbols.Symbol][]EffectiveFeature` — memoized `FeaturesOf` results
  - `instances map[int64]*Instance` — instance registry (ID → instance lookup)

**Context creation (on workspace init):**
```go
// In internal/lsp/server.go or internal/repl/repl.go:
func newRuntimeContext(ws *model.Workspace) *runtime.Context {
    model := semantics.NewModel(ws.Resolver())
    return runtime.NewContext(model, 100000) // configurable step limit
}
```

**Context invalidation:** If workspace reindexes (source file changed), discard old `Context`, create new. Instances are **not persistent across reindex** (runtime state is session-local, not workspace-persistent).

**Entry point API (for LSP/REPL):**
```go
package runtime

// NewContext creates a runtime context backed by the given semantic model.
// maxSteps sets the runaway guard (step counter limit).
func NewContext(model *semantics.Model, maxSteps int64) *Context

// Instantiate materializes an instance of the given usage/definition symbol.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error)

// Eval evaluates an expression in an empty environment (no local bindings).
// For instance-relative expressions, use EvalContext with appropriate frames.
func (ctx *Context) Eval(node ast.Node) (Value, error)

// FeaturesOf returns the effective feature list for a type symbol (memoized).
func (ctx *Context) FeaturesOf(sym *symbols.Symbol) []EffectiveFeature
```

---

## 11. Open Issues & Implementation Notes

### 11.1 Calc Body Result Extraction

**Problem:** SysML `calc` / `constraint` bodies are undifferentiated `Members []Node` (no dedicated result-expression field in AST). How does the evaluator extract the return value?

**Options:**

**A. Last expression in `Members`:**
- Scan `Members` backwards, find first expression node → that's the result.
- Matches KerML "expression-oriented" semantics (last expr is implicit return).

**B. Usage named `result` or `return` with a `Value`:**
- Scan `Members` for `*ast.Usage` with name `"result"` or `"return"`, eval its `.Value` expression.
- Matches pilot convention (Jupyter kernel uses `result` feature).

**C. Hybrid:**
- Try B first (look for `result` usage); if not found, fall back to A (last expression).

**Recommendation:** **Option C (hybrid).** Check pilot's Jupyter kernel implementation for precedent. Document the heuristic in code comments.

**Implementation:** Add helper `extractCalcResult(members []ast.Node) ast.Node` in `eval.go`.

### 11.2 Ordered/Nonunique Modifiers

**Problem:** `ast.Usage` has `IsOrdered bool`, `IsNonunique bool` fields. How does instantiation select collection type (Sequence vs Set)?

**Mapping (per KerML `Collections.kerml` taxonomy):**
- `ordered` + `unique` (default) → Sequence (with uniqueness enforcement — error on duplicate add, or silent dedup?)
- `ordered` + `nonunique` → Sequence
- `!ordered` + `unique` → Set
- `!ordered` + `nonunique` → Bag (unsupported in Tier 3)

**Bag handling:** Tier 3 has no Bag type. If `!ordered` + `nonunique`, use Sequence with a warning/comment (document that Bag semantics not fully implemented).

**Uniqueness enforcement in Sequence:** When `ordered` + `unique`, should adding a duplicate element error or silently dedup? **Recommendation:** Error (fail-fast; user can see the issue). Check spec for guidance.

**Implementation:** Add `collectionType(feat *EffectiveFeature) collectionKind` helper in `instance.go`; returns enum `{SeqUnique, SeqNonunique, SetType}`.

### 11.3 KerML Library Symbol Injection

**Problem:** Builtins require qualified names (`ControlFunctions::select`) to resolve. If model doesn't `import ControlFunctions::*;`, resolution fails.

**Options:**

**A. Require imports (Tier 3 initial approach):**
- Document in user guide: "Models must import `ControlFunctions`, `SequenceFunctions`, etc. to use collection operators."
- **Pro:** Spec-compliant, clean.
- **Con:** User friction (must know to add imports).

**B. Synthetic symbol injection:**
- On `NewContext(model)`, inject synthetic symbols for `ControlFunctions`, `SequenceFunctions`, `CollectionFunctions` into the resolver's symbol index.
- Resolver then finds these even if model didn't import.
- **Pro:** User-friendly, "just works".
- **Con:** Pollutes symbol index with non-model entries.

**Recommendation:** Start with **A** for Tier 3 (require imports). If user feedback indicates friction, add **B** in a follow-up increment (~50 LOC, low risk).

**Implementation (Option B, if pursued):** Add `injectKerMLLibrary(index *symbols.Index)` in `context.go`; called from `NewContext`. Populate synthetic symbols with stub scopes (no members, just the top-level function names).

### 11.4 Value Equality for Set

**Problem:** `Set` uses `map[Value]struct{}` → requires `Value` to be comparable. Go structs with slices/maps are not comparable by default.

**Solution (per §6.4 design decision):** Use **comparable projection** as map key:

```go
type valueKey struct {
    kind     ValueKind
    intVal   int64    // for ValConst int
    realVal  float64  // for ValConst real (IEEE 754 equality — fragile)
    boolVal  bool     // for ValConst bool
    strVal   string   // for ValString
    instID   int64    // for ValInstance
    seqHash  uint64   // for ValSequence (hash of elements)
    setHash  uint64   // for ValSet (hash of elements, order-invariant)
}

func valueKey(v Value) valueKey { /* ... */ }
```

**Set implementation:**
```go
type Set struct {
    elements map[valueKey]Value  // key = comparable projection, value = original Value
}

func (s *Set) Add(val Value) {
    s.elements[valueKey(val)] = val
}

func (s *Set) Contains(val Value) bool {
    _, ok := s.elements[valueKey(val)]
    return ok
}
```

**Hash function for collections:** Use a content-based hash (FNV-1a or similar). For Sequence, hash elements in order; for Set, hash elements in sorted order (to ensure `{1, 2, 3}` and `{3, 2, 1}` have same hash).

**Float equality caveat:** `realVal` uses `float64` equality, which is fragile (0.1 + 0.2 != 0.3 in IEEE 754). Document that `Set` containing reals may have unexpected equality behavior. This is spec-compliant: KerML allows reals in sets, but float equality is implementation-defined.

**Implementation:** Add `value_equality.go` with `valueKey()`, hash functions, and `Value.Equal()` / `Value.Hash()` methods (even if not exported, used internally by Set).

---

## 12. Future Work (Out of Scope)

### 12.1 Behavioral Simulation (Tiers 4–5)

**Not part of this design.** Requires parser/AST extensions (see AGENTS.md §2.4):

**Tier 4 — Behavioral AST:**
- Parse action bodies: fork/join/merge/decision nodes, initial/final nodes
- Succession edges (`first then second`), control flows
- Item flows, token payloads
- State machine structures: states, transitions (trigger/guard/effect), entry/exit/do behaviors

**New AST nodes needed (additive, following Tier-B pattern):**
- `ActionNode` (fork, join, merge, decision, initial, final)
- `Succession`, `ControlFlow` edges
- `StateNode`, `Transition`
- `ActionBody` / `StateBody` (structured, not undifferentiated `Members`)

**Resolution/type pass over behavioral constructs:** Validate node connectivity, check guard/effect expression types.

**Tier 5 — Behavioral Interpreter:**
- Token-flow execution (Petri-net-like semantics)
- Event-driven state machine stepping
- Scheduler (step to quiescence, bounded step count, optional time model)
- Deterministic where spec allows; document non-determinism (concurrent forks, event arrival order)

**Estimated scope:** +1500 LOC (Tier 4 AST extensions), +2000 LOC (Tier 5 interpreter). Future design doc required.

### 12.2 Analysis/Verification Case Drivers (Tier 6)

**Analysis case execution:**
- Subject (part/item instance) + calc chain → result values
- Requires Tier 3 (calc evaluation) + case-specific orchestration (subject binding, result collection)

**Verification case execution:**
- Subject instance + requirement constraints → evaluate predicates → PASS/FAIL
- Requires Tier 3 (constraint evaluation) + test harness (run against multiple instances, aggregate results)

**Case orchestration:**
- REPL `:run-case <analysisCaseName>` command
- LSP "Run verification case" code action
- Result reporting (JSON export, trace log, instance snapshots)

**Estimated scope:** +500 LOC (case drivers), +300 LOC (result formatting). Can be added incrementally after Tier 3 ships.

### 12.3 Observability & Debugging

**Trace logging:**
- Eval step trace: log every `Eval()` call with node type, input values, result
- Invocation stack: log calc invocation depth, parameters
- Slot access: log `GetSlot()` calls, lazy instantiation triggers

**Step-by-step debugger:**
- REPL `:debug <expr>` command → step through eval, inspect frame stack, print intermediate values
- LSP debug adapter protocol (DAP) integration (distant future)

**Result export:**
- Serialize instance graph to JSON: `{"id": 42, "type": "Car", "slots": {"mass": 1500.0, ...}}`
- Dump execution trace to file: `eval_trace.jsonl` (one JSON object per step)

**Estimated scope:** +800 LOC (trace infrastructure), +600 LOC (debugger REPL commands), +400 LOC (JSON export). Can be added post-Tier-3.

---

## References

- SysML v2 Specification: OMG 2025-02-01
- KerML Standard Library: `SysML-v2-Pilot-Implementation/sysml.library/`
- AGENTS.md: `/home/han/IdeaProjects/Systems-Modeling/runtime/AGENTS.md`
- Existing semantics package: `internal/core/semantics/`
