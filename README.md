# Open Source SysML v2 Implementation

A complete, production-grade SysML v2 implementation in Go—providing language server, interactive REPL, execution runtime, and Python client library. Spanning the full lifecycle from authoring to execution, delivering the integrated tooling experience systems engineers expect from modern language ecosystems.

## Quick Start

**Get started in 5 minutes:** [Quick Start Guide](docs/QUICKSTART.md)

### Install

**Download pre-built binaries:**
```bash
# Linux x64
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/sysml-linux-amd64.tar.gz
tar xzf sysml-linux-amd64.tar.gz && sudo mv sysml-linux-amd64 /usr/local/bin/sysml

# macOS (Intel or Apple Silicon) — see the note below; requires the tap to exist
brew tap Open-MBEE/tap && brew install systemica
```

**With a Go toolchain (no download, never quarantined):**
```bash
go install github.com/Open-MBEE/Systemica/cmd/sysml@latest
go install github.com/Open-MBEE/Systemica/cmd/sysml-lsp@latest
```

**Or build from source:**
```bash
make build
./bin/sysml
```

> **macOS — use Homebrew.** The released binaries are not Developer ID signed or notarized,
> so a tarball downloaded *in a browser* carries `com.apple.quarantine` and Gatekeeper shows
> "cannot be opened because the developer cannot be verified". Homebrew downloads with
> `curl`, which never sets that attribute, so `brew install` avoids the prompt entirely.
> Fallback if you download the tarball directly (`curl -fL ... systemica-darwin-arm64.tar.gz`,
> then `xattr -d com.apple.quarantine`): see
> [Quick Start](docs/QUICKSTART.md#macos-gatekeeper). Signing/notarization is the eventual
> fix — [docs/MACOS_DISTRIBUTION.md](docs/MACOS_DISTRIBUTION.md).
>
> The tap is not published yet: `brew tap Open-MBEE/tap` works once the maintainer creates
> `Open-MBEE/homebrew-tap` ([how](packaging/homebrew/README.md)). Until then use `go install`
> or the direct download.

### Try it

**Interactive modeling:**
```bash
$ sysml
sysml> part Wheel { attribute diameter = 16.0; }
✓ Wheel

sysml> %instantiate Wheel
Created instance: Wheel (ID: 1)

sysml> %slots Wheel
Instance: Wheel (ID: 1)
  diameter: 16.0
```

**Behavioral execution:**
```bash
sysml> calc add { in x; in y; return x + y; }
✓ add

sysml> %calc add 10 20
✓ add(10, 20)
  = 30

sysml> constraint ValidSpeed { assert 65 <= 120; }
✓ ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Action & state debugging:**
```bash
sysml> %action MyWorkflow
Action: MyWorkflow
Tokens: 1
State: Ready

sysml> %step
Tokens: 3  (fork created parallel paths)

sysml> %tokens
Token 1: processA { input: 100 }
Token 2: processB { input: 100 }
Token 3: processC { input: 100 }

sysml> %continue
✓ Completed
Result: 360

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: red
  Time: 0.00
  Events: 1

sysml> %advance 30
✓ Advanced to 30.00 (1 event(s) processed)
  Current state: green
  Last event at: 30.00
  Remaining events: 1
```

**See [examples/repl-behavioral-demo.sysml](examples/repl-behavioral-demo.sysml) for comprehensive demos.**

---

## What is This?

**Think Python/Rust/Go tooling, but for SysML v2:**

- **Language Server** — First-class IDE support (VS Code, IntelliJ, Emacs, etc.) with live diagnostics, semantic hover, go-to-definition, intelligent completion, and workspace-wide symbol search.
- **Interactive REPL** — Exploratory modeling environment: define models incrementally, evaluate expressions on-the-fly, instantiate parts, run calculations, inspect runtime state—like IPython/Jupyter for systems engineering.
- **Execution Runtime** — Not just a validator: instantiate parts, evaluate constraints against concrete values, execute calc/analysis cases. Action/state executor infrastructure complete (activity fork/join parallelism, decision guards, hierarchical/orthogonal states, choice/junction pseudostates, TimeEvent/ChangeEvent/AcceptEvent, sourceless transitions). See [SPEC_COMPLIANCE.md](docs/SPEC_COMPLIANCE.md) for measured behavioral coverage.
- **Python Client Library** — gRPC-based Python bindings for programmatic access: parse models, resolve symbols, evaluate expressions, instantiate parts, execute actions/state machines. Includes IPython display hooks for Jupyter notebooks and pandas DataFrame integration.
- **Modern Toolchain** — Dependency management (local + remote git), incremental compilation, bundled standard library, persistent semantic caches—`cargo`/`go mod` ergonomics for systems modeling.

## Goals

- **Performance:** Sub-millisecond parsing, single static binary, no JVM/Eclipse runtime
- **Completeness:** SysML v2 textual notation support (94/94 stdlib files parse clean)
- **Executable models:** Instantiate, evaluate, simulate—turn specifications into running systems
- **Real-world ergonomics:** Multi-file projects, incremental analysis, rich diagnostics

## Status

**Active development. Core infrastructure operational:**

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral grammar) | ✅ Operational (94/94 stdlib clean - see [conformance gate](internal/core/libs/stdlib_conformance_test.go)) |
| Symbol resolution & type system | ✅ Complete |
| Semantic layer (operators, builtins, validation) | ✅ Complete |
| Feature chain resolution (member access) | ✅ Complete |
| Validation passes (typing conformance, redefinition) | ✅ Complete |
| Expression evaluator & instance model (runtime Tiers 1-3) | ✅ Complete |
| Runtime operators (equality, logical, negation) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (unified grammar with graceful fallback) | ✅ Complete (36 golden ASTs, 49 negative tests) |
| Calc invocation, constraint & requirement evaluation | ✅ Complete (conformance gate: 18/18 passing) |
| Action execution engine (Tier 5) | ✅ Complete (9 conformance cases passing) |
| State machine runtime (Tier 5) | ✅ Complete (26 conformance cases: transitions, accept events, sourceless) |
| REPL debugging commands | ✅ Complete |
| Standard library bundling | ✅ Complete |
| LSP server implementation | ✅ Diagnostics, hover, go-to-definition, references, symbols, completion, formatting, rename (semantic tokens, code actions, signature help not implemented) |
| gRPC service layer | ✅ Complete (parse, symbols, diagnostics, runtime RPCs) |
| Python client library | ✅ Complete (connection lifecycle, runtime APIs, IPython hooks, DataFrame) |

**Current commit:** All tests pass (`go test ./...`), builds clean (`go build ./...`).
**Test coverage:** 1,500+ tests covering parsers, semantics, runtime (actions, states, instances, operators, validation). Behavioral robustness: 36 golden ASTs, 49 negatives, 54 conformance cases, 31 robustness subtests.
**Parser coverage:** 94/94 official SysML v2 standard library files parse cleanly. Conformance verified by [stdlib_conformance_test.go](internal/core/libs/stdlib_conformance_test.go). Grammar reference: [OMG Xtext grammar](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext).
**Behavioral execution:** Calc/constraint/requirement fully functional (18/18 tests). Action/state executors complete with nested invocation, control flow keywords, send statement (54/54 conformance tests passing). See [SPEC_COMPLIANCE.md](docs/SPEC_COMPLIANCE.md) for measured compliance (~98% faithful implementation).
**Training examples:** 98/100 files clean (2 files, 4 errors), gated by `internal/core/model/testdata/training_examples_expected.txt`. Download with `./scripts/download-training-examples.sh` (from the [OMG training directory](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training)). See [docs/TRAINING_EXAMPLES.md](docs/TRAINING_EXAMPLES.md) for analysis.
**Semantic layer:** Complete implementation of runtime operators, feature chains, and validation rules. See [examples/semantic-layer/](examples/semantic-layer/) for comprehensive demo.

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
│   ├── sysml-grpc/         # gRPC server binary (Python bindings)
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
│   ├── lower/              # AST → execution IR (ActionGraph/StateGraph)
│   ├── runtime/            # Execution engine (eval, instances, builtins)
│   ├── model/              # Workspace, document management
│   └── libs/               # Standard library bundling & caching
├── internal/lsp/           # LSP protocol implementation
├── internal/grpc/          # gRPC service implementation
├── internal/repl/          # REPL loop implementation
├── python/                 # Python client bindings (pysysml)
├── docs/                   # Design specs, architecture docs
└── testdata/               # Test fixtures (.sysml, .kerml)
```

## Technology

- **Language:** Go 1.25+ (goroutines for concurrency, single static binary, proven LSP track record)
- **Parser:** Hand-written recursive descent (zero overhead, full error recovery, sub-ms parses)
- **Grammar source:** OMG pilot Xtext grammars (`SysML.xtext` + `KerMLExpressions`)
- **Spec compliance:** [OMG SysML v2.1 Beta 1 / KerML 1.1](https://www.omg.org/spec/SysML/2.0) (2026-05 release)
- **Standard library:** 94 files from [SysML v2 Pilot Implementation 2026-05](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/releases/tag/2026-05)
- **CI/CD:** CircleCI for automated builds, tests, and releases

## Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases page](https://github.com/Open-MBEE/Systemica/releases).

**Supported platforms:**
- Linux (x64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x64)

**Release process:**
- Every commit: Build + test
- Tagged releases (`v*`): Multi-platform binaries published to GitHub Releases

**Release artifacts:** per-binary archives (`sysml-<os>-<arch>.tar.gz`,
`sysml-lsp-<os>-<arch>.tar.gz`), `systemica-<os>-<arch>.tar.gz` bundles containing both
binaries, and `SHA256SUMS.txt`. macOS binaries are not Developer ID signed or notarized and
Windows binaries are not Authenticode signed — see
[docs/MACOS_DISTRIBUTION.md](docs/MACOS_DISTRIBUTION.md).

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

# Build gRPC service
go build -o bin/sysml-grpc ./cmd/sysml-grpc
```

## Python Client

**pysysml** provides a Python client library for programmatic access to Systemica's parsing and runtime capabilities via gRPC.

**Installation:**
```bash
# Install from source (development mode)
pip install -e python/
```

**Quick example:**
```python
import pysysml

# Load and parse a SysML model
model = pysysml.load("vehicle.sysml")

# Evaluate expressions
result = pysysml.eval("2 + 2", model_hash=model.hash)
print(result)  # 4

# Instantiate parts
instance = pysysml.instantiate("Vehicle", model_hash=model.hash)
print(instance.slots["mass"])
```

**Features:**
- Jupyter notebook integration with rich HTML displays
- pandas DataFrame integration for model analysis
- Automatic service lifecycle management
- Full runtime API access (eval, instantiate, execute actions/states)

See [python/INSTALL.md](python/INSTALL.md) for detailed installation and usage instructions.

## Documentation

- **[Quick Start Guide](docs/QUICKSTART.md)** — Get up and running in 5 minutes
- **[Architecture](docs/ARCHITECTURE.md)** — Complete system architecture, core pipeline, runtime tiers
- **[Examples](examples/)** — Runtime demos and behavioral model examples

## License

Apache 2.0

## Contributing

Project currently in active development. See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, and contribution guidelines.

## Contact

[Contact information to be added]
