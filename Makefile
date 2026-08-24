.PHONY: all build build-sysml build-lsp build-grpc test lint clean install help python-test python-install python-proto vscode-grammar vscode-build vscode-package docs docs-install docs-serve docs-counts

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

# Build output directory
BIN_DIR := bin
PYTHON_DIR := python
VSCODE_DIR := editors/vscode
PYTHON ?= python3
SITE_DIR := site

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

test: ## Run all tests
	@echo "Running tests..."
	@# Per-package timeout: under -race, passes and model run within 1% of go's 10m default.
	go test -v -race -timeout 30m -coverprofile=coverage.txt -covermode=atomic ./...

lint: ## Run static analysis (staticcheck + gosec), as CI does
	@echo "Running staticcheck..."
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	@echo "Running gosec..."
	@# Generated protobuf code is excluded: its unsafe.Pointer use (G103) comes
	@# from protoc-gen-go and is not ours to change.
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -quiet -exclude-generated ./...
	@echo "✓ Lint passed"

test-short: ## Run tests without race detector (faster)
	@echo "Running tests (short)..."
	go test -v ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.txt
	rm -f sysml sysml-lsp sysml-grpc
	rm -rf $(SITE_DIR)
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

python-proto: ## Regenerate Python protobuf stubs
	@echo "Regenerating Python protobuf stubs..."
	@$(PYTHON) -c "import grpc_tools.protoc" >/dev/null 2>&1 || { echo "Error: grpcio-tools not installed. Run: $(PYTHON) -m pip install grpcio-tools"; exit 1; }
	$(PYTHON) -m grpc_tools.protoc --proto_path=api/proto \
	       --python_out=$(PYTHON_DIR)/opensysml/proto \
	       --pyi_out=$(PYTHON_DIR)/opensysml/proto \
	       --grpc_python_out=$(PYTHON_DIR)/opensysml/proto \
	       api/proto/sysml.proto
	@# generated stubs import each other by top-level name; make it package-relative
	sed -i.bak 's/^import sysml_pb2 as sysml__pb2$$/from . import sysml_pb2 as sysml__pb2/' $(PYTHON_DIR)/opensysml/proto/sysml_pb2_grpc.py
	@rm -f $(PYTHON_DIR)/opensysml/proto/sysml_pb2_grpc.py.bak
	@echo "✓ Regenerated Python stubs"

python-install: ## Install Python package in editable mode
	@echo "Installing opensysml..."
	cd $(PYTHON_DIR) && pip install -e .
	@echo "✓ Installed opensysml"

python-test: ## Run Python tests
	@echo "Running Python tests..."
	cd $(PYTHON_DIR) && pytest tests/ -v
	@echo "✓ Python tests passed"

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

docs-counts: ## Regenerate and verify all derived documentation counts
	@echo "Regenerating the documentation count lines and refereed figures..."
	go run ./cmd/doc-counts
	go run ./cmd/doc-counts -check
	go test -count=1 ./cmd/pilot-diff ./cmd/pilot-reject ./cmd/doc-counts
	@echo "✓ Documentation counts and refereed figures are current"

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
