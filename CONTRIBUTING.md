# Contributing to Systemica

Thank you for your interest in contributing to Systemica!

## Development Setup

### Prerequisites

- Go 1.25 or later
- Git
- Make (recommended for build automation)

### Clone and Build

```bash
git clone https://github.com/Open-MBEE/Systemica.git
cd Systemica
make build  # builds bin/sysml, bin/sysml-lsp, and bin/sysml-grpc with version info
make test   # runs all tests
make lint   # runs staticcheck and gosec, as CI does
```

## Development Workflow

### Building

```bash
# Build all binaries with version info
make build

# Build specific binary
make build-sysml
make build-lsp
make build-grpc

# Install to $GOPATH/bin
make install

# Python gRPC bindings
make python-proto    # regenerate protobuf stubs
make python-install  # install pysysml package
make python-test     # run Python binding tests

# Clean build artifacts
make clean
```

### Running Tests

```bash
# All tests (using Makefile)
make test

# All tests (direct)
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# With race detector
go test -race ./...

# Short tests (faster, no race detector)
make test-short

# Specific package
go test ./internal/core/parser
```

**Parser-specific tests:** When modifying the parser, ensure the four-layer test contract passes:

1. **Conformance gate:** `go test -run TestStdlibConformance ./internal/core/libs`
2. **Golden ASTs:** `go test -run TestGolden ./internal/core/parser`
3. **Negative tests:** `go test -run TestNegative ./internal/core/parser`
4. **Update goldens** (after intentional changes): `go test -run TestGolden -update ./internal/core/parser`

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#parser-test-contract) for full details on the parser testing contract.

### Static Analysis

`make lint` runs the same two checks CI gates on, at the versions pinned in the
Makefile:

- **staticcheck** — must report nothing. Unused code is deleted rather than left
  behind; if a helper is genuinely needed by work in flight, land it with its
  caller.
- **gosec** — must report nothing. Generated protobuf code is excluded
  (`-exclude-generated`). Suppress a finding with `#nosec <rule>` **only** with a
  comment saying why it is safe; do not widen the exclusion list.

### Code Style

- Use `gofmt` (enforced by CI)
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Write tests for new functionality
- Document exported types and functions

### Commit Messages

Follow conventional commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions/changes
- `refactor`: Code refactoring
- `chore`: Build/tooling changes

**Examples:**
```
feat(parser): add support for state machine transitions
fix(runtime): handle null values in expression evaluation
docs: update Quick Start guide with new commands
test(semantics): add conformance checking test cases
```

## Pull Request Process

1. **Fork** the repository
2. **Create a branch** from `main`: `git checkout -b feat/my-feature`
3. **Make changes** with clear commit messages
4. **Run tests**: `go test ./...`
5. **Push** to your fork
6. **Open a Pull Request** targeting `main`

### PR Guidelines

- Include tests for new features
- Update documentation as needed
- Ensure CI passes (build + tests)
- Keep PRs focused (one feature/fix per PR)
- Respond to review feedback

## Release Process

**For maintainers:**

### Creating a Release

1. **Update version** (if needed in code)
2. **Tag the release:**
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0: Initial public release"
   git push origin v0.1.0
   ```
3. **CI automatically:**
   - Builds binaries for all platforms
   - Publishes to GitHub Releases

### Release Checklist

- [ ] All tests pass
- [ ] Documentation updated
- [ ] CHANGELOG updated (if maintained)
- [ ] Version tag follows semver (`vX.Y.Z`)
- [ ] Release notes prepared

### Versioning

We use [Semantic Versioning](https://semver.org/):

- **MAJOR** (v1.0.0 → v2.0.0): Breaking changes
- **MINOR** (v0.1.0 → v0.2.0): New features (backward compatible)
- **PATCH** (v0.1.0 → v0.1.1): Bug fixes (backward compatible)

**Current status:** Pre-1.0, APIs subject to change

## CI/CD

### CircleCI

All commits and tags trigger CI:

**On every commit:**
- Build for Linux/macOS/Windows
- Run full test suite
- Run race detector

**On tags (`v*`):**
- Build release binaries (all platforms)
- Create GitHub Release
- Upload binaries

### Required Checks

PRs must pass:
- [ ] Build succeeds
- [ ] All tests pass
- [ ] No race conditions
- [ ] Code formatted (`gofmt`)
- [ ] Static analysis clean (`make lint`: staticcheck and gosec)

## Project Structure

```
github.com/Open-MBEE/Systemica
├── cmd/                    # Binaries (sysml, sysml-lsp, sysml-grpc)
├── internal/core/          # Core implementation
│   ├── source/            # Source file handling
│   ├── lexer/             # Tokenization
│   ├── parser/            # Parsing
│   ├── ast/               # AST nodes
│   ├── symbols/           # Symbol tables
│   ├── resolve/           # Name resolution
│   ├── semantics/         # Type system
│   ├── passes/            # Validation
│   ├── lower/             # AST → execution IR (ActionGraph/StateGraph)
│   ├── runtime/           # Execution
│   ├── model/             # Workspace
│   └── libs/              # Standard library bundling
├── internal/lsp/          # LSP implementation
├── internal/grpc/         # gRPC service implementation
├── internal/repl/         # REPL implementation
├── python/                # Python client bindings (pysysml)
├── docs/                  # Documentation
├── testdata/              # Test fixtures
└── .circleci/             # CI configuration
```

## Architecture

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed design.

**Key principles:**
- **Immutable AST:** Syntax-only, never mutated
- **Side tables:** Semantic info keyed by node/symbol
- **Lazy evaluation:** Compute on-demand, memoize
- **Incremental:** Invalidate only affected parts

## Testing Philosophy

- **Unit tests:** Per-package (`*_test.go`)
- **Integration tests:** Cross-package scenarios
- **Fixtures:** Real SysML v2 models in `testdata/`
- **Golden files:** Expected outputs (where applicable)

## Getting Help

- **GitHub Issues:** Bug reports, feature requests
- **Discussions:** Questions, ideas, help
- **Pull Requests:** Code contributions

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Follow project conventions

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see [LICENSE](LICENSE)).

---

**Thank you for contributing to Systemica!** 🚀
