# Keystone Lua Engine Makefile

.PHONY: help build run test clean docker-build docker-run deps fmt lint

# Colors for output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

# Variables
BINARY_NAME := lua-engine
DOCKER_IMAGE := keystone-lua-engine
PORT := 8081

help: ## Show this help message
	@echo "$(CYAN)Keystone Lua Engine$(RESET)"
	@echo "$(CYAN)==================$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-15s$(RESET) %s\n", $$1, $$2}'

deps: ## Download dependencies
	@echo "$(CYAN)⬇️  Downloading dependencies...$(RESET)"
	go mod download
	go mod tidy

build: deps ## Build the lua engine binary
	@echo "$(CYAN)🔨 Building lua engine...$(RESET)"
	go build -o $(BINARY_NAME) .
	@echo "$(GREEN)✅ Build completed: $(BINARY_NAME)$(RESET)"

run: build ## Run the lua engine locally
	@echo "$(CYAN)🚀 Starting lua engine on port $(PORT)...$(RESET)"
	./$(BINARY_NAME) -addr :$(PORT) -scripts ../scripts

dev: ## Run in development mode with auto-reload
	@echo "$(CYAN)🔧 Starting development mode...$(RESET)"
	@which air > /dev/null || (echo "$(YELLOW)Installing air for hot reload...$(RESET)" && go install github.com/cosmtrek/air@latest)
	air -c .air.toml

test: ## Run tests
	@echo "$(CYAN)🧪 Running tests...$(RESET)"
	go test -v ./...

fmt: ## Format code
	@echo "$(CYAN)📝 Formatting code...$(RESET)"
	go fmt ./...

lint: ## Run linter
	@echo "$(CYAN)🔍 Running linter...$(RESET)"
	@which golangci-lint > /dev/null || (echo "$(YELLOW)Installing golangci-lint...$(RESET)" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

docker-build: ## Build Docker image
	@echo "$(CYAN)🐳 Building Docker image...$(RESET)"
	docker build -t $(DOCKER_IMAGE) .
	@echo "$(GREEN)✅ Docker image built: $(DOCKER_IMAGE)$(RESET)"

docker-run: docker-build ## Run in Docker container
	@echo "$(CYAN)🐳 Running in Docker container...$(RESET)"
	docker run -p $(PORT):$(PORT) -v $(PWD)/../scripts:/app/scripts $(DOCKER_IMAGE)

docker-shell: ## Get shell in Docker container
	@echo "$(CYAN)🐳 Opening shell in container...$(RESET)"
	docker run -it --entrypoint /bin/sh $(DOCKER_IMAGE)

benchmark: ## Run performance benchmarks
	@echo "$(CYAN)📊 Running benchmarks...$(RESET)"
	go test -bench=. -benchmem ./...

scripts-validate: ## Validate all Lua scripts
	@echo "$(CYAN)✅ Validating Lua scripts...$(RESET)"
	@for script in ../scripts/*.lua; do \
		echo "Checking $$script..."; \
		lua -l $$script 2>/dev/null || echo "$(RED)❌ Syntax error in $$script$(RESET)"; \
	done
	@echo "$(GREEN)✅ Script validation completed$(RESET)"

clean: ## Clean build artifacts
	@echo "$(CYAN)🧹 Cleaning...$(RESET)"
	rm -f $(BINARY_NAME)
	docker rmi $(DOCKER_IMAGE) 2>/dev/null || true
	@echo "$(GREEN)✅ Cleanup completed$(RESET)"

health: ## Check if lua engine is running
	@echo "$(CYAN)🏥 Checking health...$(RESET)"
	@curl -s http://localhost:$(PORT)/health | jq . || echo "$(RED)❌ Lua engine not running$(RESET)"

reload: ## Reload scripts in running engine
	@echo "$(CYAN)🔄 Reloading scripts...$(RESET)"
	@curl -s -X POST http://localhost:$(PORT)/reload | jq . || echo "$(RED)❌ Failed to reload scripts$(RESET)"

logs: ## Show Docker container logs
	@echo "$(CYAN)📋 Container logs:$(RESET)"
	docker logs $$(docker ps -q --filter ancestor=$(DOCKER_IMAGE)) -f

# Default target
.DEFAULT_GOAL := help
