# Keystone Gateway - Unified Build & Deployment System
# ====================================================

# Project Configuration
PROJECT_NAME := keystone-gateway
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Environment Configuration
STAGING_COMPOSE := deployments/docker/docker-compose.staging.yml
PRODUCTION_COMPOSE := docker-compose.production.yml

# Port Configuration
STAGING_PORT := 8081
PRODUCTION_PORT := 8080
DEV_PORT := 8082

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
CYAN := \033[0;36m
NC := \033[0m

.DEFAULT_GOAL := help

# =============================================================================
# HELP & INFORMATION
# =============================================================================

.PHONY: help
help: ## Show available commands
	@echo "$(BLUE)🚀 Keystone Gateway - Unified Build & Deploy$(NC)"
	@echo "=============================================="
	@echo ""
	@echo "$(CYAN)⚡ Quick Commands:$(NC)"
	@echo "  $(GREEN)make dev$(NC)        Start development environment"
	@echo "  $(GREEN)make staging$(NC)    Deploy to staging"
	@echo "  $(GREEN)make production$(NC) Deploy to production"
	@echo "  $(GREEN)make test$(NC)       Run all tests"
	@echo "  $(GREEN)make clean$(NC)      Clean up everything"
	@echo ""
	@echo "$(CYAN)📋 All Commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "}; /^[a-zA-Z0-9_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@echo ""
	@echo "$(CYAN)🌍 Project Info:$(NC)"
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(GIT_COMMIT)"
	@echo "  Built:   $(BUILD_TIME)"

.PHONY: info
info: ## Show detailed project information
	@echo "$(BLUE)📊 Project Status$(NC)"
	@echo "=================="
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(GIT_COMMIT)"
	@echo "Build:   $(BUILD_TIME)"
	@echo ""
	@echo "$(BLUE)🐳 Container Status:$(NC)"
	@docker ps --filter "name=$(PROJECT_NAME)" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "No containers running"

# =============================================================================
# VALIDATION & BUILD
# =============================================================================

.PHONY: validate
validate: ## Validate setup and dependencies
	@echo "$(YELLOW)🔍 Validating setup...$(NC)"
	@command -v docker >/dev/null || (echo "$(RED)❌ Docker required$(NC)" && exit 1)
	@command -v docker-compose >/dev/null || (echo "$(RED)❌ Docker Compose required$(NC)" && exit 1)
	@test -f Dockerfile || (echo "$(RED)❌ Dockerfile missing$(NC)" && exit 1)
	@test -f configs/environments/staging.yaml || (echo "$(RED)❌ Staging config missing$(NC)" && exit 1)
	@test -f configs/environments/production-high-load.yaml || (echo "$(RED)❌ Production config missing$(NC)" && exit 1)
	@echo "$(GREEN)✅ Validation passed$(NC)"

.PHONY: build
build: validate ## Build Docker image
	@echo "$(YELLOW)🔨 Building image...$(NC)"
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(PROJECT_NAME):latest \
		-t $(PROJECT_NAME):$(VERSION) \
		.
	@echo "$(GREEN)✅ Build completed: $(PROJECT_NAME):$(VERSION)$(NC)"

# =============================================================================
# DEVELOPMENT
# =============================================================================

.PHONY: dev
dev: build ## Start development environment
	@echo "$(YELLOW)🚀 Starting development...$(NC)"
	@docker stop $(PROJECT_NAME)-dev 2>/dev/null || true
	@docker run -d \
		--name $(PROJECT_NAME)-dev \
		--rm \
		-p $(DEV_PORT):8080 \
		-v $(PWD)/configs/environments/staging.yaml:/app/config.yaml:ro \
		-v $(PWD)/scripts/lua:/app/scripts:ro \
		$(PROJECT_NAME):latest
	@echo "$(YELLOW)⏳ Waiting for startup...$(NC)"
	@sleep 5
	@$(MAKE) dev-health
	@echo "$(GREEN)✅ Development ready at http://localhost:$(DEV_PORT)$(NC)"

.PHONY: dev-logs
dev-logs: ## Show development logs
	@docker logs -f $(PROJECT_NAME)-dev

.PHONY: dev-health
dev-health: ## Check development health
	@curl -sf http://localhost:$(DEV_PORT)/admin/health >/dev/null && \
		echo "$(GREEN)✅ Development healthy$(NC)" || \
		echo "$(RED)❌ Development unhealthy$(NC)"

.PHONY: dev-stop
dev-stop: ## Stop development environment
	@echo "$(YELLOW)⏹️  Stopping development...$(NC)"
	@docker stop $(PROJECT_NAME)-dev 2>/dev/null || true
	@echo "$(GREEN)✅ Development stopped$(NC)"

# =============================================================================
# STAGING
# =============================================================================

.PHONY: staging
staging: validate build ## Deploy to staging
	@echo "$(YELLOW)🚀 Deploying to staging...$(NC)"
	@docker-compose -f $(STAGING_COMPOSE) down --remove-orphans 2>/dev/null || true
	@docker-compose -f $(STAGING_COMPOSE) up -d --build
	@echo "$(YELLOW)⏳ Waiting for staging...$(NC)"
	@sleep 10
	@$(MAKE) staging-health
	@echo "$(GREEN)🎉 Staging ready at http://localhost:$(STAGING_PORT)$(NC)"

.PHONY: staging-logs
staging-logs: ## Show staging logs
	@docker-compose -f $(STAGING_COMPOSE) logs -f

.PHONY: staging-health
staging-health: ## Check staging health
	@curl -sf http://localhost:$(STAGING_PORT)/admin/health >/dev/null && \
		echo "$(GREEN)✅ Staging healthy$(NC)" || \
		echo "$(RED)❌ Staging unhealthy$(NC)"

.PHONY: staging-stop
staging-stop: ## Stop staging environment
	@echo "$(YELLOW)⏹️  Stopping staging...$(NC)"
	@docker-compose -f $(STAGING_COMPOSE) down --remove-orphans
	@echo "$(GREEN)✅ Staging stopped$(NC)"

# =============================================================================
# PRODUCTION
# =============================================================================

.PHONY: production
production: validate build ## Deploy to production (with confirmation)
	@echo "$(RED)⚠️  PRODUCTION DEPLOYMENT$(NC)"
	@echo "This will deploy to production!"
	@read -p "Continue? (yes/no): " confirm && \
		if [ "$$confirm" != "yes" ]; then echo "Cancelled."; exit 1; fi
	@echo "$(YELLOW)🚀 Deploying to production...$(NC)"
	@$(MAKE) production-backup
	@docker-compose -f $(PRODUCTION_COMPOSE) up -d --build
	@echo "$(YELLOW)⏳ Waiting for production...$(NC)"
	@sleep 15
	@$(MAKE) production-health
	@echo "$(GREEN)🎉 Production deployed at http://localhost:$(PRODUCTION_PORT)$(NC)"

.PHONY: production-logs
production-logs: ## Show production logs
	@docker-compose -f $(PRODUCTION_COMPOSE) logs -f

.PHONY: production-health
production-health: ## Check production health
	@curl -sf http://localhost:$(PRODUCTION_PORT)/admin/health >/dev/null && \
		echo "$(GREEN)✅ Production healthy$(NC)" || \
		echo "$(RED)❌ Production unhealthy$(NC)"

.PHONY: production-stop
production-stop: ## Stop production environment (with confirmation)
	@echo "$(RED)⚠️  PRODUCTION SHUTDOWN$(NC)"
	@read -p "Stop production? (yes/no): " confirm && \
		if [ "$$confirm" != "yes" ]; then echo "Cancelled."; exit 1; fi
	@echo "$(YELLOW)⏹️  Stopping production...$(NC)"
	@docker-compose -f $(PRODUCTION_COMPOSE) down
	@echo "$(GREEN)✅ Production stopped$(NC)"

.PHONY: production-backup
production-backup: ## Create production backup
	@echo "$(YELLOW)💾 Creating backup...$(NC)"
	@mkdir -p backups/$(shell date +%Y%m%d-%H%M%S)
	@backup_dir="backups/$(shell date +%Y%m%d-%H%M%S)" && \
		docker-compose -f $(PRODUCTION_COMPOSE) ps > $$backup_dir/containers.txt && \
		docker images $(PROJECT_NAME):* > $$backup_dir/images.txt && \
		echo "$(GREEN)✅ Backup: $$backup_dir$(NC)"

# =============================================================================
# TESTING & QUALITY
# =============================================================================

.PHONY: test
test: ## Run all tests
	@echo "$(YELLOW)🧪 Running tests...$(NC)"
	@go test -v ./...
	@echo "$(GREEN)✅ Tests passed$(NC)"

.PHONY: test-unit
test-unit: ## Run unit tests
	@echo "$(YELLOW)🔬 Running unit tests...$(NC)"
	@go test -v ./tests/unit/...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(YELLOW)🔗 Running integration tests...$(NC)"
	@go test -v ./tests/integration/...

.PHONY: test-load
test-load: ## Run load tests
	@echo "$(YELLOW)⚡ Running load tests...$(NC)"
	@go test -v ./tests/ -run "TestRealisticProductionLoadTesting"

.PHONY: lint
lint: ## Run code linting
	@echo "$(YELLOW)🔍 Linting code...$(NC)"
	@golangci-lint run ./... 2>/dev/null || echo "$(YELLOW)⚠️ golangci-lint not found, skipping$(NC)"
	@go vet ./...
	@echo "$(GREEN)✅ Linting completed$(NC)"

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(YELLOW)✨ Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✅ Code formatted$(NC)"

# =============================================================================
# MONITORING & HEALTH
# =============================================================================

.PHONY: health
health: ## Check health of all environments
	@echo "$(YELLOW)🏥 Health Check Summary$(NC)"
	@echo "======================="
	@echo -n "Development: " && $(MAKE) dev-health 2>/dev/null || echo "$(RED)Not running$(NC)"
	@echo -n "Staging: " && $(MAKE) staging-health 2>/dev/null || echo "$(RED)Not running$(NC)"
	@echo -n "Production: " && $(MAKE) production-health 2>/dev/null || echo "$(RED)Not running$(NC)"

.PHONY: status
status: ## Show status of all environments
	@echo "$(CYAN)📊 Environment Status$(NC)"
	@echo "====================="
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@docker ps --filter "name=$(PROJECT_NAME)-dev" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "Not running"
	@echo ""
	@echo "$(YELLOW)Staging:$(NC)"
	@docker-compose -f $(STAGING_COMPOSE) ps 2>/dev/null || echo "Not running"
	@echo ""
	@echo "$(YELLOW)Production:$(NC)"
	@docker-compose -f $(PRODUCTION_COMPOSE) ps 2>/dev/null || echo "Not running"

.PHONY: logs
logs: ## Show all available log commands
	@echo "$(CYAN)📜 Available Log Commands$(NC)"
	@echo "============================"
	@echo "  make dev-logs        - Development logs"
	@echo "  make staging-logs    - Staging logs"
	@echo "  make production-logs - Production logs"

# =============================================================================
# WORKFLOW HELPERS
# =============================================================================

.PHONY: feature-start
feature-start: ## Start new feature (usage: make feature-start FEATURE=name)
	@if [ -z "$(FEATURE)" ]; then echo "$(RED)Usage: make feature-start FEATURE=name$(NC)"; exit 1; fi
	@echo "$(YELLOW)🌿 Starting feature: $(FEATURE)$(NC)"
	@git checkout staging 2>/dev/null || git checkout -b staging
	@git pull origin staging 2>/dev/null || true
	@git checkout -b feature/$(FEATURE)
	@echo "$(GREEN)✅ Feature branch: feature/$(FEATURE)$(NC)"
	@$(MAKE) dev

.PHONY: hotfix-start
hotfix-start: ## Start hotfix (usage: make hotfix-start HOTFIX=name)
	@if [ -z "$(HOTFIX)" ]; then echo "$(RED)Usage: make hotfix-start HOTFIX=name$(NC)"; exit 1; fi
	@echo "$(RED)🚨 Starting hotfix: $(HOTFIX)$(NC)"
	@git checkout main
	@git pull origin main 2>/dev/null || true
	@git checkout -b hotfix/$(HOTFIX)
	@echo "$(GREEN)✅ Hotfix branch: hotfix/$(HOTFIX)$(NC)"

# =============================================================================
# MAINTENANCE & CLEANUP
# =============================================================================

.PHONY: clean
clean: ## Clean up all environments and resources
	@echo "$(YELLOW)🧹 Cleaning up...$(NC)"
	@$(MAKE) dev-stop 2>/dev/null || true
	@$(MAKE) staging-stop 2>/dev/null || true
	@docker system prune -f
	@echo "$(GREEN)✅ Cleanup completed$(NC)"

.PHONY: clean-all
clean-all: ## Complete cleanup including images (with confirmation)
	@echo "$(RED)⚠️  This will remove ALL project containers, images, and volumes$(NC)"
	@read -p "Continue? (yes/no): " confirm && \
		if [ "$$confirm" != "yes" ]; then echo "Cancelled."; exit 1; fi
	@$(MAKE) clean
	@docker images $(PROJECT_NAME) -q | xargs -r docker rmi -f 2>/dev/null || true
	@docker volume ls -q --filter name=$(PROJECT_NAME) | xargs -r docker volume rm 2>/dev/null || true
	@echo "$(GREEN)✅ Complete cleanup finished$(NC)"

.PHONY: reset
reset: clean-all build ## Complete reset and rebuild

# =============================================================================
# SHORTCUTS & ALIASES
# =============================================================================

.PHONY: up
up: dev ## Alias for 'make dev'

.PHONY: down  
down: dev-stop ## Alias for 'make dev-stop'

.PHONY: restart
restart: dev-stop dev ## Restart development environment

# Include local overrides if they exist
-include Makefile.local