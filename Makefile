# Keystone Gateway - Streamlined Development Workflow
# Centralized Makefile for all development, testing, and deployment operations

APP_NAME := keystone-gateway
VERSION := 1.2.1
GO_VERSION := 1.21
DOCKER_IMAGE := chi-stone:$(VERSION)

# Default Go build settings
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
BUILD_FLAGS := -ldflags "-w -s -X main.version=$(VERSION)"

# Configuration
CONFIG_DEV := configs/examples/config.yaml
CONFIG_PROD := configs/environments/production.yaml
PORT := 8080

# Colors for output
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

.PHONY: help dev test build docker run stop clean deps lint fmt check deploy-prod logs

# Default target
help: ## Show this help message
	@echo "$(CYAN)Keystone Gateway Development Workflow$(NC)"
	@echo "======================================"
	@echo ""
	@echo "$(GREEN)Development Commands:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-12s$(NC) %s\n", $$1, $$2}'
	@echo ""

# Development workflow
dev: deps fmt lint test build ## Complete development workflow: deps → fmt → lint → test → build

# Dependencies and setup
deps: ## Download and verify dependencies
	@echo "$(CYAN)→ Installing dependencies...$(NC)"
	@go mod tidy
	@go mod verify
	@echo "$(GREEN)✓ Dependencies ready$(NC)"

# Code quality
fmt: ## Format Go code
	@echo "$(CYAN)→ Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint: ## Lint Go code
	@echo "$(CYAN)→ Linting code...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ Code linted$(NC)"

check: fmt lint ## Run code quality checks

# Testing
test: ## Run unit and integration tests
	@echo "$(CYAN)→ Running tests...$(NC)"
	@go test -v ./internal/... ./pkg/...
	@echo "$(GREEN)✓ Core tests passed$(NC)"

test-all: ## Run all tests (including legacy tests that need updating)
	@echo "$(CYAN)→ Running all tests...$(NC)"
	@go test -v ./...
	@echo "$(GREEN)✓ All tests completed$(NC)"

test-race: ## Run tests with race detection
	@echo "$(CYAN)→ Running tests with race detection...$(NC)"
	@go test -race -v ./...
	@echo "$(GREEN)✓ Race tests passed$(NC)"

test-coverage: ## Run tests with coverage report
	@echo "$(CYAN)→ Running tests with coverage...$(NC)"
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

# Building
build: ## Build the gateway binary
	@echo "$(CYAN)→ Building chi-stone...$(NC)"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o chi-stone ./cmd/chi-stone
	@echo "$(GREEN)✓ chi-stone binary ready$(NC)"

build-all: ## Build all binaries
	@echo "$(CYAN)→ Building all binaries...$(NC)"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o chi-stone ./cmd/chi-stone
	@echo "$(GREEN)✓ chi-stone binary ready$(NC)"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o lua-stone ./cmd/lua-stone
	@echo "$(GREEN)✓ lua-stone binary ready$(NC)"

build-all-platforms: ## Build for multiple platforms
	@echo "$(CYAN)→ Building for multiple platforms...$(NC)"
	@mkdir -p dist
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/chi-stone-linux-amd64 ./cmd/chi-stone
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/chi-stone-darwin-amd64 ./cmd/chi-stone
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/chi-stone-windows-amd64.exe ./cmd/chi-stone
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/lua-stone-linux-amd64 ./cmd/lua-stone
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/lua-stone-darwin-amd64 ./cmd/lua-stone
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/lua-stone-windows-amd64.exe ./cmd/lua-stone
	@echo "$(GREEN)✓ Multi-platform builds completed$(NC)"

# Docker operations
docker: ## Build Docker image
	@echo "$(CYAN)→ Building Docker image...$(NC)"
	@docker build -t $(DOCKER_IMAGE) .
	@docker tag $(DOCKER_IMAGE) chi-stone:latest
	@echo "$(GREEN)✓ Docker image built: $(DOCKER_IMAGE)$(NC)"

# Local development server
run: build ## Run the gateway locally with development config
	@echo "$(CYAN)→ Starting chi-stone on :$(PORT)...$(NC)"
	@echo "$(YELLOW)Press Ctrl+C to stop$(NC)"
	@./chi-stone -config $(CONFIG_DEV) -addr :$(PORT)

run-docker: docker ## Run the gateway in Docker
	docker-run: ## Run the gateway in Docker
	@echo "$(CYAN)→ Running chi-stone in Docker...$(NC)"
	@docker run --rm -p 8080:8080 --name chi-stone-dev $(DOCKER_IMAGE)
	@echo "$(GREEN)✓ chi-stone started$(NC)"

# Quick testing with minimal backends
test-local: build ## Run local test with minimal mock backends
	@echo "$(CYAN)→ Starting minimal test environment...$(NC)"
	@echo "$(YELLOW)Starting mock backends...$(NC)"
	@docker run -d --name test-api --rm -p 3002:80 \
		-v $(PWD)/mock-backends/demo:/usr/share/nginx/html:ro nginx:alpine || true
	@sleep 2
	@echo "$(YELLOW)Starting gateway...$(NC)"
	@./chi-stone -config $(CONFIG_DEV) -addr :$(PORT) &
	@sleep 3
	@echo "$(CYAN)→ Testing endpoints...$(NC)"
	@curl -s http://localhost:$(PORT)/admin/health | grep -q healthy && echo "$(GREEN)✓ Health check passed$(NC)" || echo "$(RED)✗ Health check failed$(NC)"
	@echo "$(CYAN)→ Cleaning up...$(NC)"
	@pkill -f chi-stone || true
	@docker stop test-api || true
	@echo "$(GREEN)✓ Local test completed$(NC)"

# Production deployment (simplified)
deploy-prod: build docker ## Deploy to production environment
	@echo "$(CYAN)→ Deploying to production...$(NC)"
	@echo "$(YELLOW)Warning: This will replace the current production deployment$(NC)"
	@read -p "Continue? [y/N] " -n 1 -r; echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose -f deployments/docker/docker-compose.simple.yml down --remove-orphans || true; \
		docker-compose -f deployments/docker/docker-compose.simple.yml up -d --build; \
		echo "$(GREEN)✓ Production deployment completed$(NC)"; \
	else \
		echo "$(YELLOW)Deployment cancelled$(NC)"; \
	fi

# Utilities
logs: ## Show application logs (Docker)
	@docker-compose -f deployments/docker/docker-compose.simple.yml logs -f keystone-gateway || echo "$(RED)No running containers found$(NC)"

stop: ## Stop all running services
	@echo "$(CYAN)→ Stopping services...$(NC)"
	@docker-compose -f deployments/docker/docker-compose.simple.yml down --remove-orphans || true
	@pkill -f chi-stone || true
	@docker stop test-api 2>/dev/null || true
	@echo "$(GREEN)✓ Services stopped$(NC)"

status: ## Show running services status
	@echo "$(CYAN)→ Service Status:$(NC)"
	@docker-compose -f deployments/docker/docker-compose.simple.yml ps 2>/dev/null || echo "No Docker services running"
	@echo ""
	@echo "$(CYAN)→ Local Process:$(NC)"
	@pgrep -f chi-stone && echo "Gateway process running" || echo "No local gateway process"

clean: stop ## Clean up build artifacts and containers
	@echo "$(CYAN)→ Cleaning up...$(NC)"
	@rm -f chi-stone lua-stone
	@rm -rf dist/
	@rm -f coverage.out coverage.html
	@docker image rm $(DOCKER_IMAGE) chi-stone:latest 2>/dev/null || true
	@docker system prune -f
	@echo "$(GREEN)✓ Cleanup completed$(NC)"

# Performance testing (simplified)
perf: build ## Run simplified performance test
	@echo "$(CYAN)→ Running performance test...$(NC)"
	@echo "$(YELLOW)Starting test environment...$(NC)"
	@docker run -d --name perf-backend --rm -p 3001:80 nginx:alpine
	@./chi-stone -config $(CONFIG_DEV) -addr :$(PORT) &
	@sleep 3
	@echo "$(CYAN)→ Testing with Apache Bench...$(NC)"
	@ab -n 1000 -c 10 -q http://localhost:$(PORT)/admin/health && echo "$(GREEN)✓ Performance test completed$(NC)" || echo "$(RED)✗ Performance test failed$(NC)"
	@pkill -f chi-stone || true
	@docker stop perf-backend || true

# Development convenience targets
watch: ## Watch for changes and rebuild (requires entr)
	@echo "$(CYAN)→ Watching for changes...$(NC)"
	@echo "$(YELLOW)Install entr with: apt-get install entr$(NC)"
	@find . -name "*.go" | entr -r make build

quick: fmt build run ## Quick development cycle: format → build → run

# Installation and setup
install-deps: ## Install development dependencies
	@echo "$(CYAN)→ Installing development dependencies...$(NC)"
	@command -v go >/dev/null 2>&1 || (echo "$(RED)Go not installed$(NC)" && exit 1)
	@command -v docker >/dev/null 2>&1 || (echo "$(RED)Docker not installed$(NC)" && exit 1)
	@go version | grep -q "go1.21" || echo "$(YELLOW)Consider upgrading to Go 1.21+$(NC)"
	@echo "$(GREEN)✓ Development environment ready$(NC)"

# Information
info: ## Show project information
	@echo "$(CYAN)Keystone Gateway v$(VERSION)$(NC)"
	@echo "=================================="
	@echo "Go version: $$(go version)"
	@echo "Docker: $$(docker --version 2>/dev/null || echo 'Not installed')"
	@echo "Build target: $(GOOS)/$(GOARCH)"
	@echo "Config (dev): $(CONFIG_DEV)"
	@echo "Config (prod): $(CONFIG_PROD)"
	@echo "Port: $(PORT)"