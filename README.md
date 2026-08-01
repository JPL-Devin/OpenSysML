# SysML v2 Execution Environment

A complete, production-grade SysML v2 implementation in Go—spanning the full lifecycle from authoring to execution, delivering the integrated tooling experience systems engineers expect from modern language ecosystems.

## What is This?

**Think Python/Rust/Go tooling, but for SysML v2:**

- **Language Server** — First-class IDE support (VS Code, IntelliJ, Emacs, etc.) with live diagnostics, semantic hover, go-to-definition, intelligent completion, and workspace-wide symbol search.
- **Interactive REPL** — Exploratory modeling environment: define models incrementally, evaluate expressions on-the-fly, instantiate parts, run calculations, inspect runtime state—like IPython/Jupyter for systems engineering.
- **Execution Runtime** — Not just a validator: instantiate parts, evaluate constraints against concrete values, execute calc/analysis cases, simulate behavioral models (actions, state machines).
- **Modern Toolchain** — Dependency management (local + remote git), incremental compilation, bundled standard library, persistent semantic caches—`cargo`/`go mod` ergonomics for systems modeling.

## Goals

- **Performance:** Sub-millisecond parsing, single static binary, no JVM/Eclipse runtime
- **Completeness:** Full OMG SysML v2 spec compliance (textual notation + KerML)
- **Executable models:** Instantiate, evaluate, simulate—turn specifications into running systems
- **Real-world ergonomics:** Multi-file projects, incremental analysis, rich diagnostics

## Status

**Active development. Core infrastructure operational:**

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral grammar) | ✅ Complete |
| Symbol resolution & type system | ✅ Complete |
| Validation passes (syntax → constraints) | ✅ Complete |
| Expression evaluator & instance model (runtime Tiers 1-3) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (actions/states) | ✅ Phase C3 integrated |
| Standard library bundling & caching | 🚧 In progress |
| LSP server implementation | ⏳ Planned |
| REPL implementation | ⏳ Planned |
| Behavioral execution (Tiers 4-5) | 🔮 Future |

**Current commit:** All tests pass (`go test ./...`), builds clean (`go build ./...`).

## Architecture

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

**Key design principles:**
- **Incremental & lazy:** Parse immediately, resolve semantics on-demand (gopls/rust-analyzer precedent)
- **Immutable AST:** All semantic state lives in side tables keyed by node/symbol
- **Pluggable validation:** Tiered passes (syntax → names → types → constraints)
- **Separated concerns:** Static analysis pipeline feeds execution runtime

## Module Structure

```
github.com/Open-MBEE/Systemica
├── cmd/
│   ├── sysml-lsp/          # LSP server binary
│   └── sysml/              # Interactive REPL binary
├── internal/core/
│   ├── source/             # Source files, spans, line indexing
│   ├── lexer/              # Hand-written scanner
│   ├── parser/             # Recursive-descent parser
│   ├── ast/                # Syntax tree nodes
│   ├── symbols/            # Symbol tables, scope trees
│   ├── resolve/            # Name resolution (lazy, memoized)
│   ├── semantics/          # Type system, conformance, multiplicity
│   ├── passes/             # Validation passes (syntax → constraints)
│   ├── runtime/            # Execution engine (eval, instances, builtins)
│   ├── model/              # Workspace, document management
│   └── libs/               # Standard library bundling & caching
├── internal/lsp/           # LSP protocol implementation
├── internal/repl/          # REPL loop implementation
├── docs/                   # Design specs, architecture docs
└── testdata/               # Test fixtures (.sysml, .kerml)
```

## Technology

- **Language:** Go 1.25+ (goroutines for concurrency, single static binary, proven LSP track record)
- **Parser:** Hand-written recursive descent (zero overhead, full error recovery, sub-ms parses)
- **Grammar source:** OMG pilot Xtext grammars (`SysML.xtext` + `KerMLExpressions`)
- **Spec reference:** [OMG SysML v2 (2025-02-01)](https://www.omg.org/spec/SysML/20250201)

## Building

```bash
# Build all binaries
go build ./...

# Run tests
go test ./...

# Build LSP server
go build -o bin/sysml-lsp ./cmd/sysml-lsp

# Build REPL
go build -o bin/sysml ./cmd/sysml
```

## Documentation

- **Design spec:** [`docs/superpowers/specs/2026-07-25-sysml-v2-go-design.md`](docs/superpowers/specs/2026-07-25-sysml-v2-go-design.md)
- **Runtime architecture:** [`runtime/AGENTS.md`](runtime/AGENTS.md)
- **Behavioral grammar design:** [`docs/superpowers/specs/2026-07-30-tier-c-behavioral-grammar-design.md`](docs/superpowers/specs/2026-07-30-tier-c-behavioral-grammar-design.md)
- **Validation design:** [`docs/superpowers/specs/2026-07-30-validation-depth-c-design.md`](docs/superpowers/specs/2026-07-30-validation-depth-c-design.md)

## License

[To be determined]

## Contributing

Project currently in active development. Contribution guidelines forthcoming.

## Contact

[Contact information to be added]
