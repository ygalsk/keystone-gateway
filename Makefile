# Keystone Gateway Production Setup
# ===================================

# Variables
COMPOSE_FILE := docker-compose.production.yml
PROJECT_NAME := keystone-gateway
GATEWAY_PORT := 8080
NGINX_PORT := 80
DOMAIN := keystone-gateway.dev

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

.DEFAULT_GOAL := help

# =============================================================================
# HELP & INFO
# =============================================================================

.PHONY: help
help: ## Show this help message
	@echo "$(GREEN)Keystone Gateway - Production Setup$(NC)"
	@echo "===================================="
	@echo ""
	@echo "$(YELLOW)Prerequisites:$(NC)"
	@echo "  - Docker & Docker Compose installed"
	@echo "  - $(COMPOSE_FILE) file present"
	@echo "  - config/ directory with production.yaml"
	@echo ""
	@echo "$(YELLOW)Quick Start:$(NC)"
	@echo "  make up        # Start all services"
	@echo "  make test      # Run load tests"
	@echo "  make status    # Check service status"
	@echo ""
	@echo "$(YELLOW)Available commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: check-requirements
check-requirements: ## Check if all required files exist
	@echo "$(YELLOW)🔍 Checking requirements...$(NC)"
	@test -f $(COMPOSE_FILE) || (echo "$(RED)❌ $(COMPOSE_FILE) not found$(NC)" && exit 1)
	@test -f Dockerfile || (echo "$(RED)❌ Dockerfile not found$(NC)" && exit 1)
	@test -f configs/production.yaml || (echo "$(RED)❌ configs/production.yaml not found$(NC)" && exit 1)
	@test -d scripts || (echo "$(RED)❌ scripts/ directory not found$(NC)" && exit 1)
	@command -v docker >/dev/null || (echo "$(RED)❌ Docker not installed$(NC)" && exit 1)
	@command -v docker-compose >/dev/null || (echo "$(RED)❌ Docker Compose not installed$(NC)" && exit 1)
	@echo "$(GREEN)✅ All requirements satisfied$(NC)"

.PHONY: status
status: ## Show current status
	@echo "$(YELLOW)🔍 Service Status:$(NC)"
	@docker-compose -f $(COMPOSE_FILE) ps
	@echo ""
	@echo "$(YELLOW)📊 Resource Usage:$(NC)"
	@docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" 2>/dev/null | head -6 || echo "No containers running"

# =============================================================================
# DOCKER OPERATIONS
# =============================================================================

.PHONY: build
build: check-requirements ## Build all Docker images
	@echo "$(YELLOW)🔨 Building Docker images...$(NC)"
	@docker-compose -f $(COMPOSE_FILE) build

.PHONY: up
up: check-requirements ## Start all services
	@echo "$(YELLOW)🚀 Starting all services...$(NC)"
	@docker-compose -f $(COMPOSE_FILE) up -d
	@echo "$(YELLOW)⏳ Waiting for services to be ready...$(NC)"
	@sleep 15
	@$(MAKE) -s health-check

.PHONY: down
down: ## Stop all services
	@echo "$(YELLOW)⏹️  Stopping all services...$(NC)"
	@docker-compose -f $(COMPOSE_FILE) down

.PHONY: restart
restart: down up ## Restart all services

.PHONY: rebuild
rebuild: down build up ## Rebuild and restart all services

.PHONY: logs
logs: ## Show logs from all services
	@docker-compose -f $(COMPOSE_FILE) logs -f

.PHONY: logs-gateway
logs-gateway: ## Show gateway logs only
	@docker-compose -f $(COMPOSE_FILE) logs -f keystone-gateway

.PHONY: logs-nginx
logs-nginx: ## Show nginx logs only
	@docker-compose -f $(COMPOSE_FILE) logs -f nginx

# =============================================================================
# TESTING & MONITORING
# =============================================================================

.PHONY: health-check
health-check: ## Run health checks
	@echo "$(YELLOW)🏥 Running health checks...$(NC)"
	@curl -sf http://localhost/admin/health > /dev/null && echo "$(GREEN)✅ Gateway: healthy$(NC)" || echo "$(RED)❌ Gateway: unhealthy$(NC)"
	@curl -sf http://localhost/api/time > /dev/null && echo "$(GREEN)✅ API: healthy$(NC)" || echo "$(RED)❌ API: unhealthy$(NC)"
	@curl -sf http://localhost:9090/-/healthy > /dev/null 2>&1 && echo "$(GREEN)✅ Prometheus: healthy$(NC)" || echo "$(RED)❌ Prometheus: not available$(NC)"

.PHONY: test
test: ## Run comprehensive load tests
	@echo "$(YELLOW)🧪 Running load tests...$(NC)"
	@docker-compose -f $(COMPOSE_FILE) run --rm wrk-tester /tests/run-all-tests.sh
	@echo "$(GREEN)✅ Load tests completed!$(NC)"
	@$(MAKE) -s show-results

.PHONY: test-quick
test-quick: ## Run quick performance test (30s)
	@echo "$(YELLOW)⚡ Quick performance test...$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t4 -c50 -d30s --latency http://keystone-gateway:8080/admin/health

.PHONY: test-api
test-api: ## Test API endpoints specifically
	@echo "$(YELLOW)🌐 Testing API endpoints...$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t4 -c100 -d60s --latency http://nginx/api/time

.PHONY: test-stress
test-stress: ## Run stress test (high load)
	@echo "$(YELLOW)💪 Stress testing (200 connections)...$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t8 -c200 -d60s --timeout 10s --latency http://nginx/api/time

.PHONY: test-sustained
test-sustained: ## Run sustained load test (5 minutes)
	@echo "$(YELLOW)⏱️  Sustained load test (5 minutes)...$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t6 -c150 -d300s --latency http://nginx/lb/status/200

.PHONY: benchmark
benchmark: ## Run comprehensive benchmarks
	@echo "$(YELLOW)📊 Running benchmarks...$(NC)"
	@echo ""
	@echo "$(YELLOW)Test 1: Health endpoint$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t2 -c10 -d10s --latency http://nginx/admin/health | grep -E "(Requests/sec|Latency)"
	@echo ""
	@echo "$(YELLOW)Test 2: API endpoint$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t4 -c50 -d30s --latency http://nginx/api/time | grep -E "(Requests/sec|Latency)"
	@echo ""
	@echo "$(YELLOW)Test 3: Load balancing$(NC)"
	@docker run --rm --network $(PROJECT_NAME)_gateway-network williamyeh/wrk \
		-t4 -c100 -d30s --latency http://nginx/lb/status/200 | grep -E "(Requests/sec|Latency)"
	@echo ""
	@echo "$(GREEN)✅ Benchmarks completed!$(NC)"

.PHONY: show-results
show-results: ## Show test results summary
	@echo "$(YELLOW)📈 Test Results Summary:$(NC)"
	@if [ -f logs/load-tests/summary.txt ]; then \
		cat logs/load-tests/summary.txt; \
	else \
		echo "No test results found. Run 'make test' first."; \
	fi

# =============================================================================
# DEVELOPMENT & DEBUGGING
# =============================================================================

.PHONY: shell
shell: ## Get shell in gateway container
	@docker-compose -f $(COMPOSE_FILE) exec keystone-gateway sh

.PHONY: shell-nginx
shell-nginx: ## Get shell in nginx container
	@docker-compose -f $(COMPOSE_FILE) exec nginx sh

.PHONY: debug
debug: ## Show debug information
	@echo "$(YELLOW)🐛 Debug Information:$(NC)"
	@echo "Docker version: $(shell docker --version 2>/dev/null || echo 'Not installed')"
	@echo "Docker Compose version: $(shell docker-compose --version 2>/dev/null || echo 'Not installed')"
	@echo ""
	@echo "Project: $(PROJECT_NAME)"
	@echo "Compose file: $(COMPOSE_FILE)"
	@echo ""
	@echo "Services:"
	@docker-compose -f $(COMPOSE_FILE) ps 2>/dev/null || echo "No services running"
	@echo ""
	@echo "Networks:"
	@docker network ls 2>/dev/null | grep $(PROJECT_NAME) || echo "No project networks found"
	@echo ""
	@echo "Volumes:"
	@docker volume ls 2>/dev/null | grep $(PROJECT_NAME) || echo "No project volumes found"

.PHONY: metrics
metrics: ## Show performance metrics
	@echo "$(YELLOW)📊 Performance Metrics:$(NC)"
	@echo ""
	@echo "$(YELLOW)Gateway Health:$(NC)"
	@curl -s http://localhost/admin/health 2>/dev/null | jq . 2>/dev/null || curl -s http://localhost/admin/health 2>/dev/null || echo "Gateway not accessible"
	@echo ""
	@echo "$(YELLOW)Container Resources:$(NC)"
	@docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}" 2>/dev/null || echo "No containers running"

.PHONY: endpoints
endpoints: ## Test all available endpoints
	@echo "$(YELLOW)🌐 Testing all endpoints:$(NC)"
	@echo ""
	@echo "$(YELLOW)Gateway Health:$(NC)"
	@curl -s -w "Status: %{http_code}, Time: %{time_total}s\n" -o /dev/null http://localhost/admin/health 2>/dev/null || echo "❌ Failed"
	@echo ""
	@echo "$(YELLOW)API Time:$(NC)"
	@curl -s -w "Status: %{http_code}, Time: %{time_total}s\n" -o /dev/null http://localhost/api/time 2>/dev/null || echo "❌ Failed"
	@echo ""
	@echo "$(YELLOW)Load Balancer:$(NC)"
	@curl -s -w "Status: %{http_code}, Time: %{time_total}s\n" -o /dev/null http://localhost/lb/status/200 2>/dev/null || echo "❌ Failed"
	@echo ""
	@echo "$(YELLOW)Web Interface:$(NC)"
	@curl -s -w "Status: %{http_code}, Time: %{time_total}s\n" -o /dev/null http://localhost/web/ 2>/dev/null || echo "❌ Failed"

# =============================================================================
# CLEANUP
# =============================================================================

.PHONY: clean
clean: ## Clean up Docker resources
	@echo "$(YELLOW)🧹 Cleaning up Docker resources...$(NC)"
	@docker-compose -f $(COMPOSE_FILE) down -v --remove-orphans 2>/dev/null || true
	@docker system prune -f

.PHONY: clean-images
clean-images: ## Remove project Docker images
	@echo "$(YELLOW)🗑️  Removing project images...$(NC)"
	@docker images | grep $(PROJECT_NAME) | awk '{print $$3}' | xargs docker rmi -f 2>/dev/null || true

.PHONY: clean-all
clean-all: down clean clean-images ## Complete cleanup (containers, volumes, images)
	@echo "$(GREEN)✅ Complete cleanup finished$(NC)"

# =============================================================================
# MONITORING & DASHBOARDS
# =============================================================================

.PHONY: dashboard
dashboard: ## Open monitoring dashboards
	@echo "$(YELLOW)📊 Available dashboards:$(NC)"
	@echo "  • Prometheus: http://localhost:9090"
	@echo "  • Grafana: http://localhost:3000 (admin/admin)"
	@echo "  • Gateway Admin: http://localhost/admin/health"
	@echo ""
	@if command -v open >/dev/null 2>&1; then \
		echo "Opening Grafana..."; \
		open http://localhost:3000; \
	elif command -v xdg-open >/dev/null 2>&1; then \
		echo "Opening Grafana..."; \
		xdg-open http://localhost:3000; \
	else \
		echo "Please open http://localhost:3000 manually"; \
	fi

.PHONY: prometheus
prometheus: ## Show Prometheus targets status
	@echo "$(YELLOW)🎯 Prometheus Targets:$(NC)"
	@curl -s http://localhost:9090/api/v1/targets 2>/dev/null | jq -r '.data.activeTargets[] | "\(.labels.job): \(.health) \(.lastError // "")"' 2>/dev/null || echo "Prometheus not accessible"

# =============================================================================
# PRODUCTION HELPERS
# =============================================================================

.PHONY: deploy
deploy: check-requirements build up health-check ## Full deployment workflow
	@echo "$(GREEN)🚀 Deployment completed successfully!$(NC)"
	@echo ""
	@echo "$(YELLOW)Available services:$(NC)"
	@echo "  • Gateway: http://localhost:$(NGINX_PORT)"
	@echo "  • Admin: http://localhost:$(NGINX_PORT)/admin/health"
	@echo "  • API: http://localhost:$(NGINX_PORT)/api/time"
	@echo "  • Monitoring: http://localhost:9090"

.PHONY: smoke-test
smoke-test: ## Run smoke tests to verify deployment
	@echo "$(YELLOW)💨 Running smoke tests...$(NC)"
	@$(MAKE) -s endpoints
	@$(MAKE) -s test-quick
	@echo "$(GREEN)✅ Smoke tests passed!$(NC)"

.PHONY: production-check
production-check: ## Comprehensive production readiness check
	@echo "$(YELLOW)🔍 Production Readiness Check:$(NC)"
	@echo ""
	@$(MAKE) -s check-requirements
	@$(MAKE) -s health-check
	@$(MAKE) -s endpoints
	@echo ""
	@echo "$(YELLOW)Performance baseline:$(NC)"
	@$(MAKE) -s test-quick
	@echo ""
	@echo "$(GREEN)✅ Production check completed!$(NC)"

# =============================================================================
# UTILITY TARGETS
# =============================================================================

.PHONY: ps
ps: ## Show running containers (alias for status)
	@$(MAKE) -s status

.PHONY: top
top: ## Show real-time container stats
	@docker stats

.PHONY: inspect
inspect: ## Show detailed container information
	@docker-compose -f $(COMPOSE_FILE) config

.PHONY: network
network: ## Show network information
	@echo "$(YELLOW)🌐 Network Information:$(NC)"
	@docker network ls | grep $(PROJECT_NAME) || echo "No project networks found"
	@echo ""
	@docker network inspect $(PROJECT_NAME)_gateway-network 2>/dev/null | jq '.[0].Containers' 2>/dev/null || echo "Network details not available"

# =============================================================================
# PERFORMANCE TESTING SUITE
# =============================================================================

.PHONY: perf-suite
perf-suite: ## Run complete performance testing suite
	@echo "$(YELLOW)🏁 Complete Performance Suite$(NC)"
	@echo "=============================="
	@echo ""
	@$(MAKE) -s test-quick
	@echo ""
	@$(MAKE) -s test-api
	@echo ""
	@$(MAKE) -s benchmark
	@echo ""
	@echo "$(GREEN)✅ Performance suite completed!$(NC)"