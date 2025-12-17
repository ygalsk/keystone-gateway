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
build: ## Build binary (without LuaJIT)
	go build -o keystone-gateway ./cmd

.PHONY: build-luajit
build-luajit: ## Build with LuaJIT support
	CGO_CFLAGS="$$(pkg-config luajit --cflags)" CGO_LDFLAGS="$$(pkg-config luajit --libs)" go build -tags luajit -o keystone-gateway ./cmd

.PHONY: run-luajit
run-luajit: build-luajit ## Build and run with LuaJIT + example config
	./keystone-gateway -config examples/configs/config-golua.yaml

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
docker: ## Build Docker image with LuaJIT support
	docker build -t keystone-gateway:latest .

.PHONY: docker-luajit
docker-luajit: docker ## Alias for docker target (LuaJIT is default)

.PHONY: docker-luarocks
docker-luarocks: docker ## Build Docker image with LuaJIT + LuaRocks
	docker build -f Dockerfile.luajit -t keystone-gateway:luarocks .

.PHONY: all
all: fmt lint test build ## Format, lint, test, and build
