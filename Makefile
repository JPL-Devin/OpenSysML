.PHONY: all build build-sysml build-lsp build-grpc test clean install help python-test python-install python-proto

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

# Build output directory
BIN_DIR := bin
PYTHON_DIR := python

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
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

test-short: ## Run tests without race detector (faster)
	@echo "Running tests (short)..."
	go test -v ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.txt
	rm -f sysml sysml-lsp sysml-grpc
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
	@command -v protoc >/dev/null 2>&1 || { echo "Error: protoc not found. Install protobuf compiler."; exit 1; }
	protoc --proto_path=api/proto \
	       --python_out=$(PYTHON_DIR)/pysysml/proto \
	       --grpc_python_out=$(PYTHON_DIR)/pysysml/proto \
	       api/proto/sysml.proto
	@echo "✓ Regenerated Python stubs"

python-install: ## Install Python package in editable mode
	@echo "Installing pysysml..."
	cd $(PYTHON_DIR) && pip install -e .
	@echo "✓ Installed pysysml"

python-test: ## Run Python tests
	@echo "Running Python tests..."
	cd $(PYTHON_DIR) && pytest tests/ -v
	@echo "✓ Python tests passed"

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
