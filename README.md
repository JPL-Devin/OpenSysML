# Open Source SysML v2 Implementation

A complete, production-grade SysML v2 implementation in Go—providing language server, interactive REPL, execution runtime, and Python client library. Spanning the full lifecycle from authoring to execution, delivering the integrated tooling experience systems engineers expect from modern language ecosystems.

## Quick Start

**Get started in 5 minutes:** [the guide](docs/guide/)

### Install

**Download pre-built binaries:**
```bash
# Linux x64 (use systemica-linux-arm64.tar.gz on arm64)
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/systemica-linux-amd64.tar.gz
tar xzf systemica-linux-amd64.tar.gz && sudo mv sysml sysml-lsp /usr/local/bin/

# macOS (Intel or Apple Silicon) — see the note below
brew install Open-MBEE/tap/systemica
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
> [the guide](docs/guide/01-install.md#macos-gatekeeper). Signing/notarization is the eventual
> fix — [docs/project/macos-distribution.md](docs/project/macos-distribution.md).
>
> Install by the **fully-qualified** name. Homebrew 6 requires third-party taps to be trusted
> before their Ruby is loaded, and `brew install Open-MBEE/tap/systemica` trusts just that
> formula. `brew tap Open-MBEE/tap && brew install systemica` needs
> `brew trust --formula Open-MBEE/tap/systemica` in between.

### Try it

**Interactive modeling:**
```bash
$ sysml
sysml> part def Wheel { attribute diameter = 16.0; }
✓ part def Wheel

sysml> %instantiate Wheel
✓ Created instance of Wheel
  ID: 1
  Use %features Wheel to inspect

sysml> %features Wheel
Instance: Wheel (ID: 1)
Features:
  diameter = 16.00
```

**Behavioral execution:**
```bash
sysml> calc add { in x; in y; return x + y; }
✓ calc add

sysml> %calc add 10 20
✓ add(10, 20)
  = 30

sysml> constraint ValidSpeed { assert 65 <= 120; }
✓ constraint ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Action & state debugging:**
```bash
sysml> %action MyWorkflow
✓ Started action executor for "MyWorkflow"
  State: Running
  Tokens: 1

Use %step to advance, %tokens to inspect, %continue to run to completion

sysml> %break compute
✓ Breakpoint set at node "compute"
  %continue runs until a token reaches it

sysml> %continue
⏸ Paused at breakpoint "compute"
  State: Suspended
  Tokens: 1

Use %tokens to inspect, %step or %continue to resume

sysml> %tokens
Active tokens (1):
  Token 1 @ compute
    result = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    result = 42

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: red
  Time: 0.00
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

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

- **Language Server** — A standard LSP server (`sysml-lsp`) with live diagnostics, semantic hover, go-to-definition, find references, completion, workspace-wide symbol search, formatting, rename, semantic tokens and quick fixes. A VS Code extension with TextMate grammars for `.sysml` and `.kerml` ships in [editors/vscode](editors/vscode), and any editor with a generic LSP client can drive the server directly — [guide chapter 8](docs/guide/08-editors.md) walks through both. *Not yet:* the extension is built from source rather than published to a marketplace, and the server answers no semantic token delta requests or signature help.
- **Interactive REPL** — Exploratory modeling environment: define models incrementally, evaluate expressions on-the-fly, instantiate parts, run calculations, inspect runtime state—like IPython/Jupyter for systems engineering.
- **Execution Runtime** — Not just a validator: instantiate parts, evaluate constraints against concrete values, execute calc/analysis cases. Action/state executor infrastructure complete (activity fork/join parallelism, decision guards, hierarchical/orthogonal states, choice/junction pseudostates, TimeEvent/ChangeEvent/AcceptEvent, sourceless transitions). See [spec compliance](docs/project/spec-compliance.md) for measured behavioral coverage.
- **Python Client Library** — gRPC-based Python bindings for programmatic access: parse models, resolve symbols, evaluate expressions, instantiate parts, execute actions/state machines. Includes IPython display hooks for Jupyter notebooks and pandas DataFrame integration. Constraint, requirement, satisfaction and calc verdicts are available as RPCs (`verify_constraint`, `verify_requirement`, `verify_satisfaction`, `calc`).
- **Modern Toolchain** — Incremental compilation, bundled standard library, persistent semantic caches. A model is a set of files, named on the command line or opened by the editor.

## Goals

- **Performance:** Sub-millisecond parsing, single static binary, no JVM/Eclipse runtime
- **Completeness:** SysML v2 textual notation support (95/95 stdlib files parse clean: 94 vendored OMG files and 1 Systemica extension)
- **Executable models:** Instantiate, evaluate, simulate—turn specifications into running systems
- **Real-world ergonomics:** Multi-file workspaces, incremental analysis, rich diagnostics

## Status

**Active development. Core infrastructure operational:**

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral grammar) | ✅ Operational (95/95 stdlib clean - see [conformance gate](internal/core/libs/stdlib_conformance_test.go)) |
| Symbol resolution & type system | ✅ Complete |
| Semantic layer (operators, builtins, validation) | ✅ Complete |
| Feature chain resolution (member access) | ✅ Complete |
| Validation passes (typing conformance, redefinition) | ✅ Complete |
| Expression evaluator & instance model (runtime Tiers 1-3) | ✅ Complete |
| Runtime operators (equality, logical, negation) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (unified grammar with graceful fallback) | ✅ Complete (86 golden ASTs, 129 negative tests) |
| Calc invocation, constraint & requirement evaluation | ✅ Complete (conformance gate: 104 calc/constraint/requirement/satisfy cases passing) |
| Action execution engine (Tier 5) | ✅ Complete (56 conformance cases passing) |
| State machine runtime (Tier 5) | ✅ Complete (64 conformance cases: transitions, accept events, sourceless) |
| REPL debugging commands | ✅ Complete — `%constraint`, `%requirement`, `%satisfy` and `%calc` also answer from the command line (`-constraint`, `-requirement`, `-satisfy`, `-calc`) and over gRPC, on one evaluation |
| Model save to notation (`%save model.sysml`, `sysml -convert sysml`) | ✅ Complete — writes the source through the formatter, so comments and spacing survive |
| SysML ↔ RDF Turtle conversion (`%save model.ttl`, `sysml -convert ttl`) | 🧪 **Experimental** — packages, definitions, usages, ports, connections, values, documentation, and the nodes an action or state body states (102 of 120 `examples/` models convert; what is not mapped is refused with the construct named), but the vocabulary may change without a compatibility path. Every run says so; see [the RDF mapping's status](docs/reference/rdf-mapping.md#status-experimental) and [worked example](examples/rdf-interop-demo.sysml) |
| Standard library bundling | ✅ Complete |
| LSP server implementation | ✅ Diagnostics, hover, go-to-definition, references, symbols, completion, formatting, rename, semantic tokens (full + range), code actions (quick fixes) — semantic token deltas and signature help not implemented |
| gRPC service layer | ✅ Complete (parse, symbols, diagnostics, runtime, verification, conversion and Query RPCs) |
| Python client library | ✅ Complete for the RPCs that exist (connection lifecycle, parse/symbols/eval/instantiate/execute, constraint/requirement/satisfaction/calc verification, conversion, Query, IPython hooks, DataFrame) |

**Current commit:** All tests pass (`go test -race ./...`), builds clean (`go build ./...`).
**Test coverage:** 4,447 tests and subtests (4,440 pass, 7 skip themselves; 2,351 top-level `Test` functions) covering parsers, semantics, runtime (actions, states, instances, operators, validation). Behavioral robustness: 86 golden ASTs, 129 negatives, 297 conformance cases, 98 golden traces, 165 runtime robustness cases and 8 gRPC ones.
**Parser coverage:** 95/95 bundled library files parse cleanly — the 94 official SysML v2 standard library files and the non-normative `Systemica Libraries/SystemicaMathFunctions.kerml` extension. Conformance verified by [stdlib_conformance_test.go](internal/core/libs/stdlib_conformance_test.go). Grammar reference: [OMG Xtext grammar](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext).
**Behavioral execution:** Calc/constraint/requirement/satisfy fully functional. Action/state executors complete with nested invocation, control flow keywords, loop and conditional statements, send statement (297/297 conformance tests passing). See [spec compliance](docs/project/spec-compliance.md) for measured compliance (~98% faithful implementation).
**Training examples:** 98/100 files clean (2 files, 4 errors), gated by `internal/core/model/testdata/training_examples_expected.txt`. Download with `./scripts/download-training-examples.sh` (from the [OMG training directory](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training)). See [training examples](docs/project/training-examples.md) for analysis.
**Semantic layer:** Complete implementation of runtime operators, feature chains, and validation rules. See [examples/semantic-layer/](examples/semantic-layer/) for comprehensive demo.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Frontends: LSP Server │ Interactive REPL               │
├─────────────────────────────────────────────────────────┤
│  Workspace: Multi-file documents, incremental reindex   │
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
- **Standard library:** 94 files from [SysML v2 Pilot Implementation 2026-05](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/releases/tag/2026-05), byte-identical, plus the non-normative `Systemica Libraries/SystemicaMathFunctions.kerml` extension
- **CI/CD:** CircleCI for automated builds, tests, and releases

## Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases page](https://github.com/Open-MBEE/Systemica/releases).

**Supported platforms:**
- Linux (x64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x64)

**Release process:**
- Every commit: Build + test
- Tagged releases (`v*`): the suite runs again on the tagged commit, then multi-platform
  binaries are published to GitHub Releases. Maintainer procedure:
  [docs/project/releasing.md](docs/project/releasing.md); what changed per release:
  [CHANGELOG.md](CHANGELOG.md)
- The Python client is released on its own tag (`pysysml-v*`), which uploads `pysysml` to
  PyPI — its version is not coupled to the core's, since it resolves a `sysml-grpc` binary
  at runtime from whichever release the caller names

**Release artifacts:** per-binary archives (`sysml-<os>-<arch>.tar.gz`,
`sysml-lsp-<os>-<arch>.tar.gz`), `systemica-<os>-<arch>.tar.gz` bundles containing both
binaries, and `SHA256SUMS.txt`. macOS binaries are not Developer ID signed or notarized and
Windows binaries are not Authenticode signed — see
[docs/project/macos-distribution.md](docs/project/macos-distribution.md).

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
pip install pysysml          # from PyPI, once the first release is published

# Or from a checkout, in development mode
pip install -e python/
```

**Quick example:**
```python
import pysysml

# Load and parse a SysML model
model = pysysml.load("vehicle.sysml")

# Evaluate expressions
result = model.eval("2 + 2")
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

- **[The guide](docs/guide/)** — install, first model, CLI, REPL, checks, behavior, saving, editors, Python
- **[Reference](docs/reference/)** — CLI flags, REPL commands, environment, Go and Python APIs, RDF mapping
- **[Internals](docs/internals/architecture.md)** — the pipeline, the tiers, testing and performance
- **[Project status](docs/project/spec-compliance.md)** — spec compliance, roadmap, releasing
- **[Examples](examples/)** — Runtime demos and behavioral model examples

The full map is [docs/README.md](docs/README.md).

## License

Apache 2.0

## Contributing

Project currently in active development. See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, and contribution guidelines.

## Contact

[Contact information to be added]
