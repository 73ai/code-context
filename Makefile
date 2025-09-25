# Simple Makefile for codegrep
PROJECT_NAME := codegrep
MAIN_PACKAGE := ./cmd/codegrep

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-X 'main.version=$(VERSION)'"

.DEFAULT_GOAL := build

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean build artifacts
	go clean -cache
	rm -rf bin build

.PHONY: deps
deps: ## Download dependencies
	go mod download
	go mod tidy

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

.PHONY: lint
lint: ## Run linter
	golangci-lint run ./...

.PHONY: test
test: ## Run tests
	go test -v -race ./... 2>&1 | grep -v "has malformed LC_DYSYMTAB" || true

.PHONY: build
build: deps ## Build binary
	mkdir -p bin
	go build $(LDFLAGS) -o bin/$(PROJECT_NAME) $(MAIN_PACKAGE)

.PHONY: install
install: ## Install to GOPATH/bin
	go install $(LDFLAGS) $(MAIN_PACKAGE)

.PHONY: run
run: build ## Build and run with help
	./bin/$(PROJECT_NAME) --help

.PHONY: check
check: fmt lint test ## Run all checks