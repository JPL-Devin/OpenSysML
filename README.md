# Open Source SysML v2 Implementation

A SysML v2 and KerML 1.1 implementation in Go—providing language server, interactive REPL, execution runtime, and Python client library. Spanning the lifecycle from authoring to execution, delivering the integrated tooling experience systems engineers expect from modern language ecosystems.

What that is measured against, and what it is not, is in [spec compliance](docs/project/spec-compliance.md) and the [pilot differential](docs/project/pilot-differential.md): we compare every diagnostic against the pinned OMG pilot implementation over its own corpora, and we do not claim conformance certification.

## Quick Start

**Get started in 5 minutes:** [the guide](docs/guide/)

**All documentation, searchable:** <https://open-mbee.github.io/OpenSysML/> — the same
pages as [docs/](docs/), rendered from `main`.

### Install

**Download pre-built binaries:**
```bash
# Linux x64 (use opensysml-linux-arm64.tar.gz on arm64)
wget https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-linux-amd64.tar.gz
tar xzf opensysml-linux-amd64.tar.gz && sudo mv sysml sysml-lsp /usr/local/bin/

# macOS (Intel or Apple Silicon) — see the note below
brew install Open-MBEE/tap/opensysml
```

**With a Go toolchain (no download, never quarantined):**
```bash
go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest
go install github.com/Open-MBEE/OpenSysML/cmd/sysml-lsp@latest
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
> Fallback if you download the tarball directly (`curl -fL ... opensysml-darwin-arm64.tar.gz`,
> then `xattr -d com.apple.quarantine`): see
> [the guide](docs/guide/01-install.md#macos-gatekeeper). Signing/notarization is the eventual
> fix — [docs/project/macos-distribution.md](docs/project/macos-distribution.md).
>
> Install by the **fully-qualified** name. Homebrew 6 requires third-party taps to be trusted
> before their Ruby is loaded, and `brew install Open-MBEE/tap/opensysml` trusts just that
> formula. `brew tap Open-MBEE/tap && brew install opensysml` needs
> `brew trust --formula Open-MBEE/tap/opensysml` in between.

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
- **Constraint Solving** *(experimental)* — Beyond evaluating what holds of an object: an external SMT solver answers whether a constraint, requirement or satisfaction assertion *can* hold, which conditions conflict when it cannot, what values would satisfy it, which variants a model permits, and what optimizes an `analysis def`'s objectives. The solver is optional and discovered at runtime — [the REPL command reference](docs/reference/repl-commands.md) documents each command and [installing a solver](docs/guide/01-install.md#installing-a-solver-optional) how to get one.
- **Execution Runtime** — Not just a validator: instantiate parts, evaluate constraints against concrete values, execute calc/analysis cases. Action/state executor infrastructure complete (activity fork/join parallelism, decision guards, hierarchical/orthogonal states, choice/junction pseudostates, TimeEvent/ChangeEvent/AcceptEvent, sourceless transitions). See [spec compliance](docs/project/spec-compliance.md) for measured behavioral coverage.
- **Python Client Library** — gRPC-based Python bindings for programmatic access: parse models, resolve symbols, evaluate expressions, instantiate parts, execute actions/state machines. Includes IPython display hooks for Jupyter notebooks and pandas DataFrame integration. Constraint, requirement, satisfaction and calc verdicts are available as RPCs (`verify_constraint`, `verify_requirement`, `verify_satisfaction`, `calc`).
- **Modern Toolchain** — Incremental compilation, bundled standard library, persistent semantic caches. A model is a set of files, named on the command line or opened by the editor.

## Goals

- **Performance:** Sub-millisecond parsing, single static binary, no JVM/Eclipse runtime
- **Completeness:** SysML v2 textual notation support (95/95 stdlib files parse clean: 94 vendored OMG files and 1 OpenSysML extension)
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
| Behavioral parser (unified grammar with graceful fallback) | ✅ Complete (107 golden ASTs, 167 negative tests) |
| Calc invocation, constraint & requirement evaluation | ✅ Complete (conformance gate: 104 calc/constraint/requirement/satisfy cases passing) |
| Action execution engine (Tier 5) | ✅ Complete (62 conformance cases passing) |
| State machine runtime (Tier 5) | ✅ Complete (71 conformance cases: transitions, accept events, sourceless) |
| REPL debugging commands | ✅ Complete — `%constraint`, `%requirement`, `%satisfy` and `%calc` also answer from the command line (`-constraint`, `-requirement`, `-satisfy`, `-calc`) and over gRPC, on one evaluation |
| Model save to notation (`%save model.sysml`, `sysml -convert sysml`) | ✅ Complete — writes the source through the formatter, so comments and spacing survive |
| SysML ↔ RDF Turtle conversion (`%save model.ttl`, `sysml -convert ttl`) | 🧪 **Experimental** — packages, definitions, usages, ports, connections, values, documentation, and the nodes an action or state body states (102 of 120 `examples/` models convert; what is not mapped is refused with the construct named), but the vocabulary may change without a compatibility path. Every run says so; see [the RDF mapping's status](docs/reference/rdf-mapping.md#status-experimental) and [worked example](examples/rdf-interop-demo.sysml) |
| View rendering (`%render <view>`, `sysml -render`) | ✅ Complete for the kinds produced — containment tree, interconnection diagram, state machine, action flow and table, as indented text or in the kind's machine-readable form (Mermaid, Markdown). State and action renderings read the graph the runtime executes; the notation itself is tool-defined ([SysML v2 §10.2](docs/project/spec-compliance.md)) |
| Constraint solving (`%check`, `%explain`, `%solve`, `%configure`, `%optimize`) | 🧪 **Experimental** — an external SMT-LIB 2 solver decides whether conditions *can* be satisfied, explains an `unsat` with a minimal unsat core, synthesises satisfying values, enumerates the variant selections a model permits and optimizes an `analysis def`'s objectives (optimization needs z3, which implements it). The solver is optional and discovered on `PATH` or through `OPENSYSML_SMT`; a build with none reports that rather than a verdict — see [installing a solver](docs/guide/01-install.md#installing-a-solver-optional) |
| Source-preserving model edits (`ApplyEdits`, `model.edit()`) | ✅ Complete for four operations — set a feature's value, rename a declaration, add a member, and delete a declaration — rewriting the bytes of the model's own source so every untouched byte is identical. A rename rewrites the references to the renamed element too, and a non-cascade deletion of a referenced element is refused rather than approximated |
| Standard library bundling | ✅ Complete |
| LSP server implementation | ✅ Diagnostics, hover, go-to-definition, references, symbols, completion, formatting, rename, semantic tokens (full + range), code actions (quick fixes) — semantic token deltas and signature help not implemented |
| gRPC service layer | ✅ Complete (parse, symbols, diagnostics, runtime, verification, conversion, edit and Query RPCs) |
| Python client library | ✅ Complete for the RPCs that exist (connection lifecycle, parse/symbols/eval/instantiate/execute, constraint/requirement/satisfaction/calc verification, conversion, edits, Query, IPython hooks, DataFrame) |

**Measured against the pinned reference** (`PILOT_TAG=2026-05`, artifact `0.60.1`). Every number below is re-derived from a committed baseline by the doc-count guard in `cmd/pilot-diff`; none of them is typed in by hand.

- **Corpus agreement:** 309 of 353 files agree diagnostic-by-diagnostic; 119 diagnostics are ours alone and 85 the reference's alone ([differential](docs/project/pilot-differential.md), `go run ./cmd/pilot-diff`).
- **Declared-diagnostic silence:** of the 513 declared `errors` rows in the reference's own Xpect suites, we report nothing for 49. 95 we report word-for-word; 232 wording-only and 62 location-only differences are agreement in substance and are not counted as gaps; 24 more we report as a warning and 51 elsewhere in the file ([Xpect oracle](docs/project/pilot-xpect.md), `go run ./cmd/pilot-xpect`).
- **Scope agreement:** 212 of 230 declared scope assertions match exactly (same source).
- **Permissiveness gaps:** of 119 invalid models we wrote ourselves, the reference rejects 21 that we accept by default, and 98 both reject; 5 further cases agree only when we are asked strictly. We authored every one of these cases ourselves, so the denominator measures the reach of our own corpus and not our conformance; agreement reached only under an opt-in strict mode is weaker evidence than agreement by default ([rejection oracle](docs/project/pilot-rejection.md), `go run ./cmd/pilot-reject`).
- **Self-assessed surface:** 125 of the tracked rules have no external referee at all — the action, state-machine and classifier-behavior rows, which the four numbers above cannot see, because the pinned artifact evaluates expressions but executes neither actions nor state machines.

What these numbers cannot show: the OMG corpora are demonstrations rather than an official conformance suite; the differential is one-directional, comparing the diagnostics the two implementations report on the same files; the Xpect suites are the pilot authors' test intent rather than a certification oracle; and none of these is a percentage of the specification — no global compliance figure is claimed anywhere.

**Row bookkeeping:** the ✅/⚠️/❌/⛔ status of each of the 690 tracked rules stays in [spec compliance](docs/project/spec-compliance.md) as a census of our own row list. It moves when rows are rewritten and does not move when an oracle does, so it is not the progress measure.

**Current commit:** All tests pass (`go test -race ./...`), builds clean (`go build ./...`).
**Test coverage:** 5,884 tests and subtests (5,870 pass, 14 skip themselves; 3,095 top-level `Test` functions) covering parsers, semantics, runtime (actions, states, instances, operators, validation). Behavioral robustness: 107 golden ASTs, 167 negatives, 344 conformance cases, 109 golden traces, 195 runtime robustness cases, 15 gRPC conformance cases and 8 gRPC robustness cases.
**Parser coverage:** 95/95 bundled library files parse cleanly — the 94 official SysML v2 standard library files and the non-normative `OpenSysML Libraries/OpenSysMLMathFunctions.kerml` extension. Conformance verified by [stdlib_conformance_test.go](internal/core/libs/stdlib_conformance_test.go). Grammar reference: [OMG Xtext grammar](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext).
**Behavioral execution:** Calc/constraint/requirement/satisfy functional. Action/state executors handle nested invocation, control flow keywords, loop and conditional statements and the send statement (344/344 conformance cases passing). Coverage is self-assessed against the specification text and the normative library: the pinned OMG pilot implementation evaluates expressions but does not execute actions or state machines headlessly, so no external implementation currently adjudicates these rows. See [spec compliance](docs/project/spec-compliance.md).
**Reference differential:** 353 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (`2026-05`), 309 in full agreement; every divergence is enumerated and adjudicated in [the differential](docs/project/pilot-differential.md), reproducible with `go run ./cmd/pilot-diff`.
**Rejection oracle:** the reverse direction — do we reject what the reference rejects? 119 hand-written invalid models validated by both implementations, 103 rejected by both, 16 the pinned pilot rejects and we accept; every permissiveness gap is enumerated with a reproducer and likely root cause in [the rejection oracle](docs/project/pilot-rejection.md), reproducible with `go run ./cmd/pilot-reject`. Five of those agreements hold only under the opt-in [strict conformance mode](docs/guide/03-command-line.md#strict-conformance), which reports OpenSysML's notation extensions as errors; the default mode accepts them on purpose and agrees on 98. We wrote all 119 cases, so the count measures our coverage of the rejection surface, not our conformance — a sample, not a proof.
**Training examples:** 100/100 files clean, gated by `internal/core/model/testdata/training_examples_expected.txt`. Download with `./scripts/download-training-examples.sh` (from the [OMG training directory](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training)). See [training examples](docs/project/training-examples.md) for analysis.
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
github.com/Open-MBEE/OpenSysML
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
├── python/                 # Python client bindings (opensysml)
├── docs/                   # Design specs, architecture docs
└── testdata/               # Test fixtures (.sysml, .kerml)
```

## Technology

- **Language:** Go 1.25+ (goroutines for concurrency, single static binary, proven LSP track record)
- **Parser:** Hand-written recursive descent (zero overhead, full error recovery, sub-ms parses)
- **Grammar source:** OMG pilot Xtext grammars (`SysML.xtext` + `KerMLExpressions`)
- **Spec compliance:** [OMG SysML v2.1 Beta 1 / KerML 1.1](https://www.omg.org/spec/SysML/2.0) (2026-05 release)
- **Standard library:** 94 files from [SysML v2 Pilot Implementation 2026-05](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/releases/tag/2026-05), byte-identical, plus the non-normative `OpenSysML Libraries/OpenSysMLMathFunctions.kerml` extension
- **CI/CD:** CircleCI for automated builds, tests, and releases

## Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases page](https://github.com/Open-MBEE/OpenSysML/releases).

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
- The Python client is released on its own tag (`opensysml-v*`), which uploads `opensysml` to
  PyPI — its version is not coupled to the core's, since it resolves a `sysml-grpc` binary
  at runtime from whichever release the caller names

**Release artifacts:** per-binary archives (`sysml-<os>-<arch>.tar.gz`,
`sysml-lsp-<os>-<arch>.tar.gz`), `opensysml-<os>-<arch>.tar.gz` bundles containing both
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

**opensysml** provides a Python client library for programmatic access to OpenSysML's parsing and runtime capabilities via gRPC.

**Installation:**
```bash
pip install opensysml          # from PyPI, once the first release is published

# Or from a checkout, in development mode
pip install -e python/
```

**Quick example:**
```python
import opensysml

# Load and parse a SysML model
model = opensysml.load("vehicle.sysml")

# Evaluate expressions
result = model.eval("2 + 2")
print(result)  # 4

# Instantiate parts
instance = opensysml.instantiate("Vehicle", model_hash=model.hash)
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
