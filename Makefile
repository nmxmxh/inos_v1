# INOS Core Makefile

# Proto versioning and partitioning
PROTO_VERSION=v1


# Go tools
PROTOC_GEN_GO := $(shell which protoc-gen-go)
PROTOC_GEN_GO_GRPC := $(shell which protoc-gen-go-grpc)

# Ensure proto output dir exists
$(PROTO_OUT_DIR):
	@mkdir -p $(PROTO_OUT_DIR)

# Compile protos
protos: $(PROTO_OUT_DIR)
	protoc -I $(PROTO_SRC) --go_out=$(PROTO_OUT_DIR) --go_opt=paths=source_relative \
		$(PROTO_SRC)/*.proto

# Clean generated files
clean-protos:
	rm -rf $(PROTO_OUT_DIR)/*.pb.go

# Install tools
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

.PHONY: setup build test proto proto-ts docker-* migrate-* frontend-* wasm-* clean help install-tools deps lint generate-testdata

# Variables
BINARY_NAME=inos-core
DOCKER_IMAGE=inos-core
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")


# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Default target
.DEFAULT_GOAL := help

setup: install-tools
	@echo "Setting up development environment..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ Setup complete"

install-tools:
	@echo "Installing Go tools..."
	@$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go@latest || true
	@echo "✅ Tools installed"

proto:
	@echo "Generating Go protobuf code..."
	@for proto_file in $$(find proto -name '*.proto'); do \
		proto_dir=$$(dirname $$proto_file); \
		echo "Processing $$proto_file in $$proto_dir..."; \
		protoc -I=$$proto_dir --go_out=$$proto_dir --go_opt=paths=source_relative $$proto_file; \
	done
	@echo "✅ Go protobuf code generation complete"

clean-protos:
	 rm -rf $(PROTO_GO_OUT)/*.pb.go

build: proto
	@echo "Building backend..."
	@mkdir -p bin
	$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/inos-node
	@echo "✅ Build complete: bin/$(BINARY_NAME)"

clean:
	@echo "Cleaning build files..."
	$(GOCLEAN)
	@rm -f bin/$(BINARY_NAME)
	@find $(PROTO_PATH) -name "*.pb.go" -delete
	@echo "✅ Clean complete"

help:
	@echo "INOS Core Build System"
	@echo "Usage: make [target]"
	@echo "  setup         - Set up development environment"
	@echo "  build         - Build the application"
	@echo "  proto         - Generate Go protobuf code"
	@echo "  clean         - Clean build files"
	@echo "  install-tools - Install Go tools for protobuf"
