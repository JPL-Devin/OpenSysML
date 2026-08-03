# Contributing to Systemica

Thank you for your interest in contributing to Systemica!

## Development Setup

### Prerequisites

- Go 1.23 or later
- Git
- Make (recommended for build automation)

### Clone and Build

```bash
git clone https://github.com/Open-MBEE/Systemica.git
cd Systemica
make build  # builds bin/sysml and bin/sysml-lsp with version info
make test   # runs all tests
```

## Development Workflow

### Building

```bash
# Build all binaries with version info
make build

# Build specific binary
make build-sysml
make build-lsp

# Install to $GOPATH/bin
make install

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

## Project Structure

```
github.com/Open-MBEE/Systemica
├── cmd/                    # Binaries (sysml, sysml-lsp)
├── internal/core/          # Core implementation
│   ├── source/            # Source file handling
│   ├── lexer/             # Tokenization
│   ├── parser/            # Parsing
│   ├── ast/               # AST nodes
│   ├── symbols/           # Symbol tables
│   ├── resolve/           # Name resolution
│   ├── semantics/         # Type system
│   ├── passes/            # Validation
│   ├── runtime/           # Execution
│   └── model/             # Workspace
├── internal/lsp/          # LSP implementation
├── internal/repl/         # REPL implementation
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
