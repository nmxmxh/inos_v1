.PHONY: setup build test proto clean help install-tools deps lint all
.PHONY: kernel-build kernel-dev kernel-test kernel-proto
.PHONY: modules-build modules-test
.PHONY: frontend-build frontend-dev frontend-install
.PHONY: docker-build docker-up docker-down docker-clean
.PHONY: deploy-setup deploy

# ============================================================================
# INOS v1.9 - Root Makefile
# The Distributed Runtime Build System
# ============================================================================

# Variables
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.9-dev")
PROJECT_NAME=inos
DOCKER_IMAGE=$(PROJECT_NAME)
DOCKER_COMPOSE=docker compose -f deployment/docker/docker-compose.yml

# Include .env file if it exists
-include .env
export $(shell sed 's/=.*//' .env 2>/dev/null)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# WASM build parameters (CRITICAL: Multithreading support)
GOOS=js
GOARCH=wasm
# Enable SharedArrayBuffer and threading support
WASM_BUILD_FLAGS=-ldflags="-s -w" -tags="wasm,threads"
# For development (with debug symbols)
WASM_DEV_FLAGS=-tags="wasm,threads"

# Rust parameters
CARGO=cargo +nightly
RUST_TARGET=wasm32-unknown-unknown
# Enable shared memory with atomics - requires nightly + build-std
RUST_BUILD_FLAGS=--target $(RUST_TARGET) --release -Z build-std=std,panic_abort
RUSTFLAGS=-C target-feature=+atomics,+bulk-memory,+mutable-globals -C link-arg=--import-memory -C link-arg=--shared-memory -C link-arg=--max-memory=2147483648

# Cap'n Proto parameters (consumer-relative outputs)
CAPNP_PATH=protocols/schemas
CAPNP_GO_OUT=kernel/gen
CAPNP_RUST_OUT=modules/gen

# Optimization tools
WASM_OPT=wasm-opt
BROTLI=brotli

# Default target
.DEFAULT_GOAL := help

# ============================================================================
# Setup & Dependencies
# ============================================================================

setup: install-tools
	@echo "🚀 Setting up INOS development environment..."
	@$(MAKE) deps
	@$(MAKE) proto
	@echo "✅ Setup complete"

install-tools:
	@echo "📦 Installing required tools..."
	@echo "Installing Go tools..."
	@$(GOGET) zombiezen.com/go/capnproto2/...@latest || true
	@$(GOGET) github.com/google/uuid@latest || true
	@$(GOGET) golang.org/x/sync/errgroup@latest || true
	@echo ""
	@echo "⚠️  System dependencies required:"
	@echo "  1. Cap'n Proto compiler:"
	@echo "     macOS:   brew install capnp"
	@echo "     Ubuntu:  apt-get install capnproto"
	@echo ""
	@echo "  2. WASM optimization tools (optional but recommended):"
	@echo "     macOS:   brew install binaryen brotli"
	@echo "     Ubuntu:  apt-get install binaryen brotli"
	@echo ""
	@echo "  3. Rust toolchain with WASM target:"
	@echo "     curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
	@echo "     rustup target add wasm32-unknown-unknown"
	@echo ""
	@echo "✅ Go tools installed"

deps:
	@echo "📚 Installing dependencies..."
	@cd kernel && $(GOMOD) download && $(GOMOD) tidy
	@cd modules && $(CARGO) fetch
	@echo "✅ Dependencies installed"

# ============================================================================
# Protocol Generation (Cap'n Proto)
# ============================================================================

proto: proto-go proto-rust proto-ts
	@echo "✅ All protocol code generated"

proto-go:
	@./scripts/gen-proto-go.sh $(CAPNP_GO_OUT)

proto-rust:
	@echo "🔧 Generating Rust code from Cap'n Proto schemas..."
	@mkdir -p $(CAPNP_RUST_OUT)
	@echo "⚠️  Note: Rust Cap'n Proto code generation is handled by build.rs"
	@echo "   Location: modules/build.rs"
	@echo "   Generated code will be in: modules/target/<profile>/build/*/out/"
	@echo "   Rust modules import generated code automatically"
	@echo "✅ Rust protocol generation configured"

proto-ts:
	@echo "🔧 Generating TypeScript code from Cap'n Proto schemas..."
	@mkdir -p frontend/bridge/generated
	@for schema_file in $$(find $(CAPNP_PATH) -name '*.capnp'); do \
		echo "Processing $$schema_file..."; \
		capnp compile -I$(CAPNP_PATH) -o ts:frontend/bridge/generated $$schema_file || true; \
	done
	@echo "✅ TypeScript protocol code generated in frontend/bridge/generated"
 
gen-context:
	@echo "📋 Generating Codebase Context Registry..."
	@go run scripts/gen_context.go > inos_context.json
	@echo "✅ Context generated: inos_context.json"
 
 
 
 
# ============================================================================
# Kernel Build (Go WASM - Layer 2)
# ============================================================================

kernel-build: proto-go
	@echo "🧠 Building INOS Kernel (Go WASM with multithreading)..."
	@mkdir -p frontend/public
	@cd kernel && GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(WASM_BUILD_FLAGS) -o ../frontend/public/kernel.wasm .
	@# Copy wasm_exec.js (required for Go WASM)
	@GOROOT=$$(go env GOROOT) && \
	WASM_EXEC_PATH_NEW="$$GOROOT/lib/wasm/wasm_exec.js" && \
	WASM_EXEC_PATH_OLD="$$GOROOT/misc/wasm/wasm_exec.js" && \
	if [ -f "$$WASM_EXEC_PATH_NEW" ]; then cp "$$WASM_EXEC_PATH_NEW" frontend/public/; \
	elif [ -f "$$WASM_EXEC_PATH_OLD" ]; then cp "$$WASM_EXEC_PATH_OLD" frontend/public/; \
	else echo "❌ Error: wasm_exec.js not found in GOROOT ($$GOROOT)." >&2; exit 1; fi
	@# Optimize with wasm-opt if available
	@# NOTE: Disabled - Go WASM has UTF-8 parsing issues with wasm-opt
	@echo "⚠️  Skipping wasm-opt for kernel (Go WASM incompatibility)"
	@# @if command -v $(WASM_OPT) > /dev/null 2>&1; then \
	@# 	echo "⚡ Optimizing kernel.wasm with wasm-opt..."; \
	@# 	$(WASM_OPT) -O3 --enable-threads --enable-bulk-memory --strip-debug \
	@# 		-o frontend/public/kernel.wasm frontend/public/kernel.wasm; \
	@# 	echo "✅ WASM optimized"; \
	@# else \
	@# 	echo "⚠️  wasm-opt not installed. Skipping optimization."; \
	@# fi
	@# Patch AFTER optimization
	@# NOTE: Disabled - Go WASM doesn't support SharedArrayBuffer natively
	@# Binary patching breaks the WASM structure
	@echo "ℹ️  Kernel will use JS-created SharedArrayBuffer (provided during instantiation)"
	@# @echo "🔧 Patching kernel for SharedArrayBuffer (post-optimization)..."
	@# @node scripts/patch_wasm_memory.js frontend/public/kernel.wasm
	@# Compress with brotli if available
	@if command -v $(BROTLI) > /dev/null 2>&1; then \
		echo "📦 Compressing kernel.wasm with brotli..."; \
		$(BROTLI) -q 11 -f frontend/public/kernel.wasm -o frontend/public/kernel.wasm.br; \
		echo "✅ WASM compressed ($(shell ls -lh frontend/public/kernel.wasm.br | awk '{print $$5}'))"; \
	else \
		echo "⚠️  brotli not installed. Skipping compression."; \
	fi
	@echo "✅ Kernel build complete: frontend/public/kernel.wasm"

kernel-dev: proto-go
	@echo "🧠 Building INOS Kernel (Development mode - with debug symbols)..."
	@mkdir -p frontend/public
	@cd kernel && GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(WASM_DEV_FLAGS) -o ../frontend/public/kernel.wasm .
	@GOROOT=$$(go env GOROOT) && \
	WASM_EXEC_PATH_NEW="$$GOROOT/lib/wasm/wasm_exec.js" && \
	WASM_EXEC_PATH_OLD="$$GOROOT/misc/wasm/wasm_exec.js" && \
	if [ -f "$$WASM_EXEC_PATH_NEW" ]; then cp "$$WASM_EXEC_PATH_NEW" frontend/public/; \
	elif [ -f "$$WASM_EXEC_PATH_OLD" ]; then cp "$$WASM_EXEC_PATH_OLD" frontend/public/; \
	else echo "❌ Error: wasm_exec.js not found." >&2; exit 1; fi
	@echo "✅ Kernel build complete (Development): frontend/public/kernel.wasm"

kernel-test:
	@echo "🧪 Running Kernel tests (native Go)..."
	@cd kernel && $(GOTEST) -v -race ./...
	@echo "✅ Kernel tests complete"

kernel-proto: proto-go
	@echo "✅ Kernel protocols generated"

# ============================================================================
# Modules Build (Rust WASM - Layer 3)
# ============================================================================

modules-build: proto-rust
	@echo "💪 Building INOS Modules (Rust WASM)..."
	@cd modules && RUSTFLAGS="$(RUSTFLAGS)" $(CARGO) build $(RUST_BUILD_FLAGS)
	@echo "📦 Copying WASM modules to frontend..."
	@mkdir -p frontend/public/modules
	@for module in compute science ml mining vault drivers; do \
		if [ -f "modules/target/$(RUST_TARGET)/release/$$module.wasm" ]; then \
			cp "modules/target/$(RUST_TARGET)/release/$$module.wasm" "frontend/public/modules/$$module.wasm"; \
			echo "  ✅ Copied $$module.wasm"; \
		fi \
	done
	@# Optimize each module
	@if command -v $(WASM_OPT) > /dev/null 2>&1; then \
		echo "⚡ Optimizing Rust modules..."; \
		for wasm_file in frontend/public/modules/*.wasm; do \
			if [ -f "$$wasm_file" ]; then \
				echo "Optimizing $$wasm_file..."; \
				$(WASM_OPT) -O3 --enable-threads --enable-bulk-memory --enable-simd --strip-debug \
					-o "$$wasm_file" "$$wasm_file"; \
			fi \
		done; \
		echo "✅ Modules optimized"; \
	fi
	@# Patch module imports AFTER optimization
	@# TEMPORARILY DISABLED FOR DEBUGGING
	@echo "⚠️  Skipping module patches (debugging)"
	@# @echo "🔧 Patching module imports for SharedArrayBuffer (post-optimization)..."
	@# @for wasm_file in frontend/public/modules/*.wasm; do \
	@# 	if [ -f "$$wasm_file" ]; then \
	@# 		node scripts/patch_wasm_import.js "$$wasm_file"; \
	@# 	fi \
	@# done
	@# Compress modules
	@if command -v $(BROTLI) > /dev/null 2>&1; then \
		echo "📦 Compressing modules..."; \
		for wasm_file in frontend/public/modules/*.wasm; do \
			if [ -f "$$wasm_file" ]; then \
				$(BROTLI) -q 11 -f "$$wasm_file" -o "$$wasm_file.br"; \
			fi \
		done; \
		echo "✅ Modules compressed"; \
	fi
	@echo "✅ Modules build complete"


modules-test:
	@echo "🧪 Running Modules tests..."
	@cd modules && $(CARGO) test
	@echo "✅ Modules tests complete"

# Usage: make check-module MODULE=ml
check-module:
	@if [ -z "$(MODULE)" ]; then echo "❌ Error: MODULE argument required. Usage: make check-module MODULE=<name>"; exit 1; fi
	@echo "🔍 Checking module: $(MODULE)..."
	@cd modules && $(CARGO) check -p $(MODULE)
	@echo "✅ Module $(MODULE) checked"

# Usage: make test-module MODULE=ml
test-module:
	@if [ -z "$(MODULE)" ]; then echo "❌ Error: MODULE argument required. Usage: make test-module MODULE=<name>"; exit 1; fi
	@echo "🧪 Testing module: $(MODULE)..."
	@cd modules && $(CARGO) test -p $(MODULE)
	@echo "✅ Module $(MODULE) tested"

# ============================================================================
# Frontend Build (React + Vite - Layer 1)
# ============================================================================

frontend-install:
	@echo "📦 Installing frontend dependencies..."
	@cd frontend && npm install
	@echo "✅ Frontend dependencies installed"

frontend-dev: kernel-dev
	@echo "🎨 Starting frontend dev server..."
	@echo "⚠️  Note: Vite must be configured with COOP/COEP headers for SharedArrayBuffer"
	@cd frontend && npm run dev

frontend-build:
	@echo "🎨 Building frontend for production..."
	@echo "📦 Ensuring WASM artifacts are in place..."
	@mkdir -p frontend/public/modules
	@ls -lh frontend/public/kernel.wasm
	@ls -lh frontend/public/modules/*.wasm
	@cd frontend && npm run build
	@echo "✅ Frontend build complete"

# ============================================================================
# Build All
# ============================================================================

all: proto kernel-build modules-build frontend-build
	@echo "✅ All components built successfully"
	@echo ""
	@echo "📊 Build Summary:"
	@echo "  Kernel:  frontend/public/kernel.wasm"
	@echo "  Modules: frontend/public/modules/*.wasm"
	@echo "  Frontend: frontend/dist/"

build: all

# ============================================================================
# Testing
# ============================================================================

test: kernel-test modules-test
	@echo "✅ All tests complete"

# ============================================================================
# Linting
# ============================================================================

lint:
	@echo "🔍 Running linters..."
	@echo "Linting Go code..."
	@cd kernel && go vet ./...
	@cd kernel && gofmt -l . | grep . && echo "❌ Go files need formatting" && exit 1 || echo "✅ Go code formatted"
	@echo "Linting Rust code..."
	@cd modules && $(CARGO) clippy -- -D warnings
	@echo "✅ Linting complete"

# ============================================================================
# Docker
# ============================================================================

docker-build:
	@echo "🐳 Building Docker images..."
	@$(DOCKER_COMPOSE) build
	@echo "✅ Docker build complete"

docker-up:
	@echo "🚀 Starting Docker containers..."
	@$(DOCKER_COMPOSE) up -d
	@echo "✅ Docker containers started"

docker-down:
	@echo "🛑 Stopping Docker containers..."
	@$(DOCKER_COMPOSE) down
	@echo "✅ Docker containers stopped"

docker-logs:
	@$(DOCKER_COMPOSE) logs -f

docker-clean:
	@echo "🧹 Cleaning Docker resources..."
	@docker builder prune -a -f
	@docker image prune -a -f
	@docker container prune -f
	@docker volume prune -f
	@echo "✅ Docker cleanup complete"

# ============================================================================
# Deployment
# ============================================================================

deploy-setup:
	@echo "🚀 Setting up deployment environment..."
	@echo "⚠️  Configure deployment in deployment/ directory"

deploy:
	@echo "🚀 Deploying INOS..."
	@echo "⚠️  Deployment strategy to be implemented"

# ============================================================================
# Clean
# ============================================================================

clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf frontend/public/kernel.wasm
	@rm -rf frontend/public/kernel.wasm.br
	@rm -rf frontend/public/wasm_exec.js
	@rm -rf frontend/public/modules/
	@rm -rf frontend/dist/
	@cd kernel && $(GOCLEAN)
	@cd modules && $(CARGO) clean
	@find $(CAPNP_GO_OUT) -type f -delete 2>/dev/null || true
	@echo "✅ Clean complete"

# ============================================================================
# Help
# ============================================================================

help:
	@echo "INOS v1.9 - The Distributed Runtime Build System"
	@echo "=================================================="
	@echo ""
	@echo "Setup & Dependencies:"
	@echo "  setup              - Set up development environment (install tools + deps + proto)"
	@echo "  install-tools      - Install required build tools"
	@echo "  deps               - Install Go and Rust dependencies"
	@echo ""
	@echo "Protocol Generation:"
	@echo "  proto              - Generate all protocol code (Go + Rust + TypeScript)"
	@echo "  proto-go           - Generate Go code from Cap'n Proto schemas"
	@echo "  proto-rust         - Configure Rust Cap'n Proto generation"
	@echo "  proto-ts           - Generate TypeScript code from Cap'n Proto schemas"
	@echo ""
	@echo "Kernel (Layer 2 - Go WASM):"
	@echo "  kernel-build       - Build kernel with optimization + compression"
	@echo "  kernel-dev         - Build kernel in development mode (with debug symbols)"
	@echo "  kernel-test        - Run kernel tests"
	@echo ""
	@echo "Modules (Layer 3 - Rust WASM):"
	@echo "  modules-build      - Build all Rust modules with optimization"
	@echo "  modules-test       - Run module tests"
	@echo ""
	@echo "Frontend (Layer 1 - React + Vite):"
	@echo "  frontend-install   - Install frontend dependencies"
	@echo "  frontend-dev       - Start frontend dev server"
	@echo "  frontend-build     - Build frontend for production"
	@echo ""
	@echo "Build All:"
	@echo "  all / build        - Build all components (kernel + modules + frontend)"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  test               - Run all tests (kernel + modules)"
	@echo "  lint               - Run all linters"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build       - Build Docker images"
	@echo "  docker-up          - Start Docker containers"
	@echo "  docker-down        - Stop Docker containers"
	@echo "  docker-logs        - Show Docker logs"
	@echo "  docker-clean       - Clean Docker resources"
	@echo ""
	@echo "Utilities:"
	@echo "  clean              - Clean all build artifacts"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Architecture:"
	@echo "  Layer 1 (Host):    Nginx + JS Bridge (frontend/)"
	@echo "  Layer 2 (Kernel):  Go WASM Orchestrator (kernel/)"
	@echo "  Layer 3 (Modules): Rust WASM Compute (modules/)"
	@echo ""
	@echo "Key Features:"
	@echo "  ✅ Multithreading support (SharedArrayBuffer + Web Workers)"
	@echo "  ✅ Cap'n Proto zero-copy serialization"
	@echo "  ✅ WASM optimization (wasm-opt + brotli compression)"
	@echo "  ✅ Hierarchical threading model"
	@echo ""
	@echo "Version: $(VERSION)"
