# SysML v2 Execution Environment — Architecture

**Status:** Active Development  
**Module:** `github.com/Open-MBEE/Systemica`  
**Language:** Go 1.25+

## Overview

A complete, production-grade SysML v2 implementation delivering the integrated tooling experience systems engineers expect from modern language ecosystems (Python, Rust, Go).

### Core Components

1. **Language Server (`sysml-lsp`)** — IDE support with live diagnostics, semantic hover, go-to-definition, intelligent completion, and workspace-wide symbol search
2. **Interactive REPL (`sysml`)** — Exploratory modeling: define models incrementally, evaluate expressions, instantiate parts, inspect runtime state
3. **Execution Runtime** — Instantiate parts, evaluate constraints, execute calc/analysis cases, simulate behavioral models
4. **Toolchain** — Workspace management, dependency resolution, incremental compilation, bundled stdlib, persistent caches

### Design Principles

- **Performance:** Sub-millisecond parsing, single static binary, no JVM/Eclipse runtime
- **Completeness:** Full OMG SysML v2 spec compliance (textual notation + KerML)
- **Executable models:** Not just validation—runtime that instantiates, evaluates, simulates
- **Incremental & lazy:** Parse immediately, resolve semantics on-demand (gopls/rust-analyzer precedent)
- **Immutable AST:** All semantic state lives in side tables keyed by node/symbol

---

## Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│  Frontends: LSP Server │ Interactive REPL               │
├─────────────────────────────────────────────────────────┤
│  Workspace: Multi-file projects, dependency management  │
├─────────────────────────────────────────────────────────┤
│  Semantic Engine: Types, resolution, validation         │
├─────────────────────────────────────────────────────────┤
│  Execution Runtime: Expressions, instances, behaviors   │
├─────────────────────────────────────────────────────────┤
│  Parser/Lexer: Hand-written recursive descent           │
├─────────────────────────────────────────────────────────┤
│  AST: Syntax-only, immutable (semantics in side tables) │
└─────────────────────────────────────────────────────────┘
```

---

## Module Structure

```
github.com/Open-MBEE/Systemica
├── cmd/
│   ├── sysml-lsp/          # LSP server binary
│   └── sysml/              # Interactive REPL binary
├── internal/core/
│   ├── source/             # Source files, spans, line indexing
│   ├── lexer/              # Hand-written scanner (~200 keywords)
│   ├── parser/             # Recursive-descent parser
│   ├── ast/                # Syntax tree nodes (immutable)
│   ├── symbols/            # Symbol tables, scope trees
│   ├── resolve/            # Name resolution (lazy, memoized)
│   ├── semantics/          # Type system, conformance, multiplicity
│   ├── passes/             # Validation passes (syntax → constraints)
│   ├── runtime/            # Execution engine (eval, instances, builtins)
│   ├── model/              # Workspace, document management
│   ├── libs/               # Standard library bundling & caching
│   └── deps/               # Dependency resolution
├── internal/lsp/           # LSP protocol implementation
├── internal/repl/          # REPL loop implementation
├── testdata/               # Test fixtures (.sysml, .kerml)
├── examples/               # Example models and demos
└── docs/                   # Documentation
```

---

## Core Pipeline

**Static Analysis Path:**

```
source → lexer → parser → AST → symbol index → resolve → passes
```

### 1. Source & Lexer (`internal/core/source`, `internal/core/lexer`)

- **SourceFile:** Input file (.sysml or .kerml) with byte content
- **Lexer:** Hand-written scanner producing tokens with full position tracking
- **Trivia:** Comments and whitespace tracked as leading/trailing trivia
- **Keywords:** ~200 SysML keywords (case-sensitive, pre-registered)

### 2. Parser (`internal/core/parser`)

- **Hand-written recursive descent** (chosen over ANTLR4/yacc/JNI bridge)
- **Rationale:** Zero overhead, full error recovery, sub-ms parses for keystroke-latency feedback
- **Entry:** `parser.New(source).ParseFile() → *ast.RootNamespace`
- **Always produces tree:** ErrorNodes on bad input, parsing never fails
- **Grammar source:** OMG pilot Xtext grammars (SysML.xtext + KerMLExpressions)

### 3. AST (`internal/core/ast`)

**Key architectural rule:** AST is syntax-only, **immutable after parse**

- **Node interface:** `{Span() source.Span; LeadingTrivia()/TrailingTrivia() []Trivia}`
- **NodeBase:** Embedded by all nodes
- **No semantic info in AST:** All derived data lives in **side tables keyed by node/symbol**
- **Expression AST:** Full SysML v2 expression grammar (literals, operators, feature refs, invocations, collections, lambdas)
- **Behavioral AST:** Action control-flow nodes (InitialNode, FinalNode, ForkNode, JoinNode, MergeNode, DecisionNode, ActionExecutionNode), succession edges with guards

### 4. Symbols & Resolution (`internal/core/symbols`, `internal/core/resolve`)

- **Symbol:** `{Name, Kind, Decl ast.Node, Visibility, Scope, OwnerScope}`
- **Scope:** `{Parent(), Node(), Children(), LookupLocal(name), MemberNames()}`
- **Index:** `DocumentRoot(name) *Scope` — global qualified-name index
- **Resolver:** Lazy name resolution, memoized, `ResolveQualified(scope, *ast.QualifiedName) (*Symbol, bool)`
- **Deduplication:** Short+primary names alias same `*Symbol` — dedupe by pointer when walking

### 5. Semantic Model (`internal/core/semantics`)

**Runtime's primary substrate. Built via `NewModel(*resolve.Resolver)`. All results memoized in side tables.**

- **`model.go`:**
  - `DirectSupertypes(sym)` — resolved generalization edges (specializes/subsets/redefines/typing)
  - `AllSupertypes(sym)` — transitive, cycle-safe
  - `Conforms(a, b) bool` — conformance checking
  - `HasSpecializationCycle(sym) bool`
- **`members.go`:**
  - `MembersOf(sym)` — local + inherited members with masking
  - `LookupMember(sym, name)` — member lookup
  - **Effective feature list per type** (substrate for runtime instantiation)
- **`multiplicity.go`:**
  - `MultiplicityOf(sym) (Range, bool)` — parse multiplicity bounds
  - `Range{Lower, Upper Bound}`; `Bound{Value int64, Infinite bool, Known bool}`
- **`eval.go`:**
  - `Eval(n ast.Node) (Value, bool)` — **constant-folder** (seed of runtime)
  - `Value{Kind ValueKind, Int, Real, Bool}` — int/real/bool/infinity only
  - Returns `ok=false` for feature refs, strings, null, invocations, collections
  - **Runtime Tier 3 extends this to full evaluator**

### 6. Validation Passes (`internal/core/passes`)

**Pluggable validation tiers:**

- **PassLevel:** `{LevelSyntax, LevelNameResolution, LevelType, LevelConstraint}`
- **Pass:** `{Level() PassLevel; Run(ctx, name, root) []Diagnostic}`
- **Context:** Exposes `Resolver()` + `Model()` (both lazy, memoized)
- **DefaultRegistry:** SyntaxPass, NameResolutionPass, TypeCheckPass, ConstraintPass
- **Tiered execution:** Higher tiers skipped if lower tier errors

### 7. Workspace (`internal/core/model`)

- **Single source of truth:** Owns document set + global index + diagnostic cache
- **Document:** `{source, AST, scope, version}`
- **One Workspace per session** (LSP/REPL)

---

## Execution Runtime Architecture

**Package:** `internal/core/runtime`  
**Not a Pass:** Execution is stateful/iterative/value-producing (different shape than diagnostic-emitting pass)

### Tier 1 — Feature Flattening ✅

Harden `MembersOf` into stable, ordered **effective-feature list** per type:
- Own + inherited − redefined/masked
- Each entry: type + multiplicity + default-value expression
- **Schema for instance materialization**

### Tier 2 — Instance Model ✅

- **Value:** Extends `semantics.Value` → `null`, strings, **instance references**, **collections** (sequences/sets)
- **Instance:** Typed object with one slot per effective feature (Tier 1)
- **Instantiation:** Materialize instance graph from `part`/`item` usage
  - Recursively instantiate composite features
  - Multiplicity governs slot cardinality
  - Lazy slot materialization

### Tier 3 — Expression Evaluator ✅

Full evaluator with **user-defined calc invocation** and **constraint evaluation**:
- Feature access `x.y.z` resolved against instance slots
- KerML operator library (`->select`, `->collect`, `size`, string ops)
- **Calc invocation:** Resolve calc symbol → bind args to parameters → evaluate return expression
- **Constraint evaluation:** Extract `assert`/`assume` members → evaluate → check satisfaction
- **Scoped evaluation:** `EvalContext.scope` for name resolution, frame stack for parameter bindings
- **Unlocks:** Constraint checking against concrete values, `calc` execution, runtime validation

### Tier 4 — Behavioral AST ✅ (Phase C complete)

Parse + model all behavioral bodies:
- **C1: Calc bodies** — `return` expressions + mixed parameter declarations
- **C2: Constraint bodies** — `assert`/`assume` with optional `not` negation
- **C3: Requirement bodies** — `subject`/`assume`/`require`/`actor` declarations
- **C4: Action bodies** — Control flow nodes (initial/final/fork/join/merge/decision) + action execution nodes + succession edges
- **C5: State bodies** — Entry/do/exit behaviors, substates, transitions with triggers/guards/effects
- **Dispatcher:** Lookahead detects specialized vs generic bodies (e.g., `return` → calc, generic → fallback)
- **Status:** Parsed, calc/constraint **executable**, action/state **not yet executable**

### Tier 5 — Behavioral Interpreter ⏳ (Future)

Token-flow execution for actions (Petri-net-like), event-driven state machine stepping, deterministic scheduling.

### Tier 6 — Analysis & Verification Drivers ⏳ (Future)

- Analysis case: subject → calc chain → result values
- Verification case: evaluate requirements → pass/fail
- Entry points: REPL/LSP commands (`%run`, `%verify`)

---

## REPL Integration

**Package:** `internal/repl`  
**Binary:** `cmd/sysml`

### Commands

**Document management:**
- `%help` — Show help
- `%list` — List current session declarations
- `%clear` — Reset session
- `%load <file>` — Load .sysml file

**Runtime execution:**
- `%instantiate <name>` — Create instance from part def
- `%eval <expr>` — Evaluate expression (feature refs + literals)
- `%slots <name>` — Show instance slots with values
- `%instances` — List all created instances

### Implementation

- **Session:** Manages document + runtime context + instances
- **getOrCreateRuntime():** Lazy init, builds index from current document
- **Runtime commands wire to:** `runtime.Context.Instantiate()`, `runtime.Context.Eval()`, `runtime.Context.GetSlot()`

---

## Technology Choices

### Why Go?

- **Goroutines:** Concurrent reindex/query handling
- **Single binary:** Cross-platform, no JVM/runtime dependencies
- **LSP track record:** gopls demonstrates Go's suitability for language servers
- **Performance:** Fast compilation, efficient memory model

### Why Hand-Written Parser?

**Alternatives rejected:** ANTLR4-Go, goyacc, JNI/gRPC bridge to pilot

**Rationale:**
- Zero runtime overhead
- Full control over error recovery
- Sub-millisecond parses (keystroke-latency diagnostics)
- **Trade-off accepted:** Manual grammar translation from Xtext

### Incremental & Lazy Analysis

**Precedents:** gopls, rust-analyzer

- Parse immediately (syntax errors visible instantly)
- Defer name resolution / type checking until requested
- Memoize all semantic queries
- **Result:** Interactive performance even on large workspaces

---

## Development Status

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral) | ✅ Complete |
| Symbol resolution & type system | ✅ Complete |
| Validation passes (syntax → constraints) | ✅ Complete |
| Expression evaluator & instance model (Tiers 1-3) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (Phase C1-5: all behavioral bodies) | ✅ Complete |
| **Calc invocation & constraint evaluation** | ✅ **Complete** |
| REPL implementation | ✅ Complete |
| Standard library bundling | 🚧 In progress |
| LSP server implementation | ⏳ Planned |
| Action execution engine (Tier 5) | 🔮 Future |
| State machine runtime (Tier 5) | 🔮 Future |

---

## Testing Strategy

- **Unit tests:** Per-package test coverage (lexer, parser, semantics, runtime)
- **Integration tests:** End-to-end REPL/runtime scenarios
- **Test fixtures:** `testdata/*.sysml`, `testdata/*.kerml`
- **Golden files:** Expected parse/resolve/diagnostic outputs
- **Verification:** `go test ./...` (all tests pass), `go build ./...` (clean build)

---

## References

- **OMG SysML v2.0 Spec:** [https://www.omg.org/spec/SysML/2.0](https://www.omg.org/spec/SysML/2.0)
- **Pilot Xtext Grammar:** `SysML.xtext` + `KerMLExpressions` (OMG reference implementation)
- **Metamodel:** OMG SysML v2 metamodel (semantic foundation)
- **Precedents:** gopls (Go LSP), rust-analyzer (Rust LSP), IPython/Jupyter (REPL design)
