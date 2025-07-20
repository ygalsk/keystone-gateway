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

.PHONY: help dev test build docker docker-chi docker-lua deploy-core deploy-full deploy-prod deploy-swarm scale run stop clean deps lint fmt check logs status

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

# Testing - Streamlined and categorized
test: ## Run fast unit tests only
	@echo "$(CYAN)→ Running unit tests...$(NC)"
	@go test -v ./internal/config ./internal/routing
	@echo "$(GREEN)✓ Unit tests passed$(NC)"

test-integration: ## Run integration tests
	@echo "$(CYAN)→ Running integration tests...$(NC)"
	@go test -v ./test/integration/...
	@echo "$(GREEN)✓ Integration tests passed$(NC)"

test-all: ## Run all tests (unit + integration + e2e)
	@echo "$(CYAN)→ Running all tests...$(NC)"
	@go test -v ./internal/... ./test/...
	@echo "$(GREEN)✓ All tests completed$(NC)"

test-race: ## Run tests with race detection
	@echo "$(CYAN)→ Running tests with race detection...$(NC)"
	@go test -race -v ./internal/... ./test/...
	@echo "$(GREEN)✓ Race tests passed$(NC)"

test-coverage: ## Run tests with coverage report
	@echo "$(CYAN)→ Running tests with coverage...$(NC)"
	@go test -coverprofile=coverage.out ./internal/... ./test/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

bench: ## Run benchmark tests
	@echo "$(CYAN)→ Running benchmark tests...$(NC)"
	@go test -bench=. -benchmem ./test/e2e/...
	@echo "$(GREEN)✓ Benchmarks completed$(NC)"

bench-compare: ## Compare benchmark results with baseline
	@echo "$(CYAN)→ Running benchmark comparison...$(NC)"
	@go test -bench=. -benchmem -count=5 ./test/e2e/... | tee bench-current.txt
	@echo "$(GREEN)✓ Benchmark results saved to bench-current.txt$(NC)"

# Building
build: ## Build the gateway binary
	@echo "$(CYAN)→ Building chi-stone...$(NC)"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o chi-stone ./cmd/chi-stone
	@echo "$(GREEN)✓ chi-stone binary ready$(NC)"

build-all: build ## Build all binaries (alias for build)
	@echo "$(GREEN)✓ All binaries built$(NC)"

build-all-platforms: ## Build for multiple platforms
	@echo "$(CYAN)→ Building for multiple platforms...$(NC)"
	@mkdir -p dist
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/keystone-gateway-linux-amd64 ./cmd/chi-stone
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/keystone-gateway-darwin-amd64 ./cmd/chi-stone
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/keystone-gateway-windows-amd64.exe ./cmd/chi-stone
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/keystone-gateway-linux-arm64 ./cmd/chi-stone
	@echo "$(GREEN)✓ Multi-platform builds completed$(NC)"

# Docker operations
docker: ## Build Docker image
	@echo "$(CYAN)→ Building Docker image...$(NC)"
	@docker build -f deployments/docker/chi-stone.Dockerfile -t keystone-gateway:$(VERSION) .
	@docker tag keystone-gateway:$(VERSION) keystone-gateway:latest
	@echo "$(GREEN)✓ Docker image built: keystone-gateway:$(VERSION)$(NC)"


# Local development server
run: build ## Run the gateway locally with development config
	@echo "$(CYAN)→ Starting chi-stone on :$(PORT)...$(NC)"
	@echo "$(YELLOW)Press Ctrl+C to stop$(NC)"
	@./chi-stone -config $(CONFIG_DEV) -addr :$(PORT)

run-docker: docker ## Run the gateway in Docker
	@echo "$(CYAN)→ Running keystone-gateway in Docker...$(NC)"
	@docker run --rm -p 8080:8080 --name keystone-gateway-dev keystone-gateway:$(VERSION)
	@echo "$(GREEN)✓ keystone-gateway started$(NC)"

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
# Docker Compose deployment options
deploy-core: docker ## Deploy core services only (chi-stone)
	@echo "$(CYAN)→ Deploying core services...$(NC)"
	@docker-compose -f deployments/docker/docker-compose.core.yml down --remove-orphans || true
	@docker-compose -f deployments/docker/docker-compose.core.yml up -d --build
	@echo "$(GREEN)✓ Core services deployed$(NC)"

deploy-full: docker ## Deploy full stack with monitoring
	@echo "$(CYAN)→ Deploying full stack...$(NC)"
	@docker-compose -f deployments/docker/docker-compose.full.yml down --remove-orphans || true
	@docker-compose -f deployments/docker/docker-compose.full.yml up -d --build
	@echo "$(GREEN)✓ Full stack deployed$(NC)"

deploy-prod: docker ## Deploy production environment with monitoring
	@echo "$(CYAN)→ Deploying production environment...$(NC)"
	@echo "$(YELLOW)Warning: This will replace the current production deployment$(NC)"
	@read -p "Continue? [y/N] " -n 1 -r; echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose -f deployments/docker/docker-compose.production.yml down --remove-orphans || true; \
		docker-compose -f deployments/docker/docker-compose.production.yml up -d --build; \
		echo "$(GREEN)✓ Production deployment completed$(NC)"; \
		echo "$(CYAN)→ Access points:$(NC)"; \
		echo "  Gateway: http://localhost:8080"; \
		echo "  Prometheus: http://localhost:9090"; \
		echo "  Grafana: http://localhost:3000 (admin/admin)"; \
		echo "  Loki: http://localhost:3100"; \
	else \
		echo "$(YELLOW)Deployment cancelled$(NC)"; \
	fi

# Docker Swarm deployment
deploy-swarm: docker ## Deploy to Docker Swarm cluster
	@echo "$(CYAN)→ Deploying to Docker Swarm...$(NC)"
	@docker swarm init 2>/dev/null || echo "Swarm already initialized"
	@docker stack deploy -c deployments/docker/docker-compose.swarm.yml keystone-gateway
	@echo "$(GREEN)✓ Swarm deployment completed$(NC)"
	@echo "$(CYAN)→ Stack services:$(NC)"
	@docker stack services keystone-gateway

# Simple service management
scale: ## Scale gateway replicas
	@echo "$(CYAN)→ Scaling keystone-gateway...$(NC)"
	@docker-compose -f deployments/docker/docker-compose.full.yml up -d --scale keystone-gateway=3
	@echo "$(GREEN)✓ Keystone-gateway scaled to 3 replicas$(NC)"

# Utilities
logs: ## Show application logs (Docker)
	@echo "$(CYAN)→ Available log sources:$(NC)"
	@echo "1. Core (gateway only)"
	@echo "2. Full (gateway + monitoring)"
	@echo "3. Production (all services + monitoring)"
	@read -p "Select option [1-3]: " -n 1 -r; echo; \
	case $$REPLY in \
		1) docker-compose -f deployments/docker/docker-compose.core.yml logs -f keystone-gateway || echo "$(RED)No core services running$(NC)";; \
		2) docker-compose -f deployments/docker/docker-compose.full.yml logs -f || echo "$(RED)No full stack running$(NC)";; \
		3) docker-compose -f deployments/docker/docker-compose.production.yml logs -f || echo "$(RED)No production stack running$(NC)";; \
		*) echo "$(RED)Invalid option$(NC)";; \
	esac

stop: ## Stop all running services
	@echo "$(CYAN)→ Stopping services...$(NC)"
	@docker-compose -f deployments/docker/docker-compose.core.yml down --remove-orphans 2>/dev/null || true
	@docker-compose -f deployments/docker/docker-compose.full.yml down --remove-orphans 2>/dev/null || true
	@docker-compose -f deployments/docker/docker-compose.production.yml down --remove-orphans 2>/dev/null || true
	@pkill -f chi-stone || true
	@docker stop test-api 2>/dev/null || true
	@echo "$(GREEN)✓ Services stopped$(NC)"

status: ## Show running services status
	@echo "$(CYAN)→ Service Status:$(NC)"
	@echo "Core Services:"
	@docker-compose -f deployments/docker/docker-compose.core.yml ps 2>/dev/null || echo "  No core services running"
	@echo ""
	@echo "Full Stack:"
	@docker-compose -f deployments/docker/docker-compose.full.yml ps 2>/dev/null || echo "  No full stack running"
	@echo ""
	@echo "Production Stack:"
	@docker-compose -f deployments/docker/docker-compose.production.yml ps 2>/dev/null || echo "  No production stack running"
	@echo ""
	@echo "$(CYAN)→ Local Processes:$(NC)"
	@pgrep -f chi-stone && echo "Gateway process running" || echo "No local gateway process"

clean: stop ## Clean up build artifacts and containers
	@echo "$(CYAN)→ Cleaning up...$(NC)"
	@rm -f chi-stone
	@rm -rf dist/
	@rm -f coverage.out coverage.html
	@docker image rm keystone-gateway:$(VERSION) keystone-gateway:latest 2>/dev/null || true
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