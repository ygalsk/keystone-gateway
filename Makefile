# Keystone Gateway - Simple Build System
# Keep It Simple, Stupid

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Start development - build and run locally
	go run ./cmd

.PHONY: build
build: ## Build binary
	go build -o keystone-gateway ./cmd

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: test-short
test-short: ## Run tests (short)
	go test -short ./...

.PHONY: lint
lint: ## Run linter
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

.PHONY: security
security: ## Run security scan
	gosec -exclude=G304 ./...

.PHONY: clean
clean: ## Clean build artifacts
	rm -f keystone-gateway
	go clean

.PHONY: docker
docker: ## Build Docker image
	docker build -t keystone-gateway .

.PHONY: all
all: fmt lint test build ## Format, lint, test, and build
