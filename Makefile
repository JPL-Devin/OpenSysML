.PHONY: all build build-sysml build-lsp build-grpc conformance conformance-pkg conformance-rust test coverage lint clean install help python-test python-coverage node-coverage python-install proto proto-buf python-proto proto-ts proto-rust proto-lint proto-breaking vscode-grammar vscode-build vscode-package docs docs-install docs-serve docs-counts docs-check self-model

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION ?= $(shell go version | awk '{print $$3}')

# Build flags
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.Commit=$(COMMIT) \
           -X main.BuildTime=$(BUILD_TIME) \
           -X main.GoVersion=$(GO_VERSION)

# Static-analysis tool versions, pinned so CI and local runs agree
STATICCHECK_VERSION := 2025.1.1
GOSEC_VERSION := v2.22.5
BUF_VERSION := v1.57.2

# buf drives all protobuf codegen; override BUF to use an already-installed binary.
BUF ?= go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
# Wire-compatibility baseline: the schema as it stands on the main branch. The
# backslash escapes buf's ref separator, which make would otherwise read as a comment.
BUF_BREAKING_AGAINST ?= .git\#ref=origin/main,subdir=api/proto

# Build output directory
BIN_DIR := bin
PYTHON_DIR := clients/python
NODE_DIR := clients/node
VSCODE_DIR := editors/vscode
PYTHON ?= python3
# buf.gen.python.yaml starts the interpreter this names.
export PYTHON
SITE_DIR := site
# Where make self-model writes the architecture self-model's rendered views.
SELF_MODEL_DIR := examples/self-model
SELF_MODEL_OUT ?= build/self-model

all: build test python-test ## Build and test everything

build: build-sysml build-lsp build-grpc ## Build all binaries

build-sysml: ## Build sysml binary
	@echo "Building sysml..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml ./cmd/sysml
	@echo "✓ Built $(BIN_DIR)/sysml ($(VERSION))"

build-lsp: ## Build sysml-lsp binary
	@echo "Building sysml-lsp..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml-lsp ./cmd/sysml-lsp
	@echo "✓ Built $(BIN_DIR)/sysml-lsp ($(VERSION))"

build-grpc: ## Build sysml-grpc binary
	@echo "Building sysml-grpc..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml-grpc ./cmd/sysml-grpc
	@echo "✓ Built $(BIN_DIR)/sysml-grpc ($(VERSION))"

conformance: ## Run the language-independent conformance suite against sysml-grpc
	@echo "Running the conformance suite..."
	@mkdir -p $(BIN_DIR)
	go run ./cmd/conformance -withhold-capabilities strict_conformance,oslc_query -report $(BIN_DIR)/conformance-report.json -junit $(BIN_DIR)/conformance-report.xml
	@echo "✓ Conformance suite passed ($(BIN_DIR)/conformance-report.json, $(BIN_DIR)/conformance-report.xml)"

conformance-rust: ## Run the conformance suite with the blocking Rust client
	$(MAKE) build
	@mkdir -p $(BIN_DIR)
	OPENSYSML_GRPC_BINARY="$(CURDIR)/$(BIN_DIR)/sysml-grpc" cargo run --manifest-path clients/rust/Cargo.toml -p opensysml-conformance -- -binary "$(CURDIR)/$(BIN_DIR)/sysml-grpc" -report "$(CURDIR)/$(BIN_DIR)/conformance-report-rust.json"

conformance-pkg: ## Run the conformance suite through the public Go API (client/opensysml)
	@echo "Running the conformance suite through client/opensysml..."
	@mkdir -p $(BIN_DIR)
	go run ./cmd/conformance -protocols pkg,pkg-connect -allow-skips -report $(BIN_DIR)/conformance-pkg-report.json
	@echo "✓ Conformance suite passed through client/opensysml ($(BIN_DIR)/conformance-pkg-report.json)"

test: ## Run Go tests with race detection and coverage
	@echo "Running Go race tests..."
	@# Per-package timeout: under -race, passes and model run within 1% of go's 10m default.
	go test -v -race -timeout 30m -coverprofile=coverage.txt -covermode=atomic ./...

coverage: ## Write the coverage profile the SonarCloud scan reads
	@echo "Writing coverage.txt..."
	@# -coverpkg credits a package for the code it exercises elsewhere: without it
	@# ast/dump.go measures 21% though the parser's golden tests run 90% of it.
	@# Instrumenting every package is too slow to combine with -race, which
	@# make test above runs instead.
	go test -timeout 30m -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic ./...
	@# -coverpkg repeats every block once per test binary; see the script's header.
	python3 scripts/dedupe-coverage.py coverage.txt
	@go tool cover -func=coverage.txt | tail -n 1

lint: ## Run static analysis (staticcheck + gosec), as CI does
	@echo "Running staticcheck..."
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	@echo "Running gosec..."
	@# Generated protobuf code is excluded: its unsafe.Pointer use (G103) comes
	@# from protoc-gen-go and is not ours to change.
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -quiet -exclude-generated ./...
	@echo "✓ Lint passed"

test-short: ## Run Go tests without race detection
	@echo "Running Go tests without race detection..."
	go test -v ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.txt coverage-python.xml coverage-node.lcov
	rm -f sysml sysml-lsp sysml-grpc
	rm -rf $(SITE_DIR)
	@# Only the default destination; an overridden SELF_MODEL_OUT is the caller's.
	rm -rf build/self-model
	@echo "✓ Cleaned"

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing to $(shell go env GOPATH)/bin..."
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml-lsp
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml-grpc
	@echo "✓ Installed"

version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Go version: $(GO_VERSION)"

proto: proto-buf python-proto proto-ts proto-rust ## Regenerate all protobuf stubs

# One template, so the Go stubs and the Java client's message classes cannot drift apart.
# The Java plugin is a remote one, so this needs the Buf Schema Registry.
proto-buf: ## Regenerate the Go and Java protobuf stubs
	@echo "Regenerating Go and Java protobuf stubs..."
	$(BUF) generate
	@echo "✓ Regenerated Go and Java stubs"

python-proto: ## Regenerate Python protobuf stubs
	@echo "Regenerating Python protobuf stubs..."
	@$(PYTHON) -c "import grpc_tools.protoc" >/dev/null 2>&1 || { echo "Error: grpcio-tools not installed. Run: $(PYTHON) -m pip install grpcio-tools"; exit 1; }
	$(BUF) generate --template buf.gen.python.yaml
	@echo "✓ Regenerated Python stubs"

proto-ts: ## Regenerate the TypeScript stubs the npm client in clients/node ships
	@echo "Regenerating TypeScript protobuf stubs..."
	$(BUF) generate --template buf.gen.ts.yaml
	@echo "✓ Regenerated TypeScript stubs"

proto-rust: ## Generate Rust stubs and the descriptor for the Rust clients
	$(BUF) generate --template buf.gen.rust.yaml
	$(BUF) build -o clients/rust/conformance/sysml.descriptor.binpb

proto-lint: ## Lint the protobuf schema
	$(BUF) lint
	@echo "✓ Proto lint passed"

proto-breaking: ## Check the protobuf schema for wire-breaking changes against main
	$(BUF) breaking --against '$(BUF_BREAKING_AGAINST)'
	@echo "✓ No breaking schema changes"

python-install: ## Install the Python client in editable mode
	@echo "Installing opensysml..."
	cd $(PYTHON_DIR) && pip install -e .
	@echo "✓ Installed opensysml"

python-test: ## Run Python client tests
	@echo "Running Python client tests..."
	cd $(PYTHON_DIR) && pytest tests/ -v
	@echo "✓ Python client tests passed"

# Run from the repo root so the report records repo-relative paths, which is
# what the SonarCloud scan resolves against.
python-coverage: ## Run Python client tests and write coverage-python.xml
	@echo "Running Python client tests with coverage..."
	pytest $(PYTHON_DIR)/tests --cov=opensysml --cov-report=xml:coverage-python.xml --cov-report=term
	@echo "✓ Wrote coverage-python.xml"

# c8 records paths relative to the client directory, so rewrite them to
# repo-relative before the scan reads the report.
node-coverage: ## Run Node client tests and write coverage-node.lcov
	@echo "Running Node client tests with coverage..."
	cd $(NODE_DIR) && npm run test:coverage
	sed -e 's|^SF:|SF:$(NODE_DIR)/|' $(NODE_DIR)/coverage/lcov.info > coverage-node.lcov
	@echo "✓ Wrote coverage-node.lcov"

vscode-grammar: ## Regenerate the VS Code TextMate grammars from the lexer keywords
	@echo "Generating TextMate grammars..."
	go run ./$(VSCODE_DIR)/tools/gengrammar -out $(VSCODE_DIR)/syntaxes
	@echo "✓ Grammars generated"

vscode-build: ## Type-check and bundle the VS Code extension
	@echo "Building the VS Code extension..."
	cd $(VSCODE_DIR) && npm ci && npm run typecheck && npm run build
	@echo "✓ Built $(VSCODE_DIR)/dist/extension.js"

vscode-package: ## Package the VS Code extension as a .vsix for side-loading
	@echo "Packaging the VS Code extension..."
	cd $(VSCODE_DIR) && npm ci && npm run package
	@echo "✓ Packaged $(VSCODE_DIR)/opensysml-sysml.vsix"

self-model: build-sysml ## Render the architecture self-model's views (see examples/self-model/README.md)
	@echo "Rendering the architecture self-model..."
	@mkdir -p $(SELF_MODEL_OUT)
	@# A renamed or deleted view must not leave its old rendering behind.
	@rm -f $(SELF_MODEL_OUT)/OpenSysMLViews.*.mmd $(SELF_MODEL_OUT)/OpenSysMLViews.*.md
	$(BIN_DIR)/sysml $(SELF_MODEL_DIR)/*.sysml -render-all $(SELF_MODEL_OUT)
	@echo "✓ Rendered the self-model's views into $(SELF_MODEL_OUT)/"

docs-counts: ## Regenerate and verify all derived documentation counts
	@echo "Regenerating the documentation count lines and refereed figures..."
	go run ./cmd/doc-counts
	go run ./cmd/doc-counts -check
	go test -count=1 ./cmd/pilot-diff ./cmd/pilot-reject ./cmd/doc-counts
	@echo "✓ Documentation counts and refereed figures are current"

docs-check: ## Verify documentation links, that reader-facing pages cite no internal label, and that quoted oracle figures name their round
	$(PYTHON) scripts/check-doc-links.py
	$(PYTHON) scripts/check-doc-ids.py
	$(PYTHON) scripts/check-doc-figures.py

docs-install: ## Install the documentation site toolchain
	$(PYTHON) -m pip install -r docs-requirements.txt

docs: ## Build the documentation site, failing on a broken link
	@echo "Building the documentation site..."
	$(PYTHON) -m mkdocs build --strict --site-dir $(SITE_DIR)
	@echo "✓ Built $(SITE_DIR)/"

docs-serve: ## Serve the documentation site with live reload
	$(PYTHON) -m mkdocs serve --strict

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
