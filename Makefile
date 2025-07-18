# Keystone Gateway
APP_NAME := keystone-gateway
STACK_NAME := keystone

.PHONY: help build start test stop logs status clean

help:
	@echo "Keystone Gateway Commands:"
	@echo "  build   - Build the application"
	@echo "  start   - Deploy gateway stack"
	@echo "  test    - Deploy and test endpoints"
	@echo "  stop    - Remove the stack"
	@echo "  logs    - Show service logs"
	@echo "  status  - Show stack status"
	@echo "  clean   - Clean up everything"

build:
	docker build -t $(APP_NAME):latest .

start: build
	docker swarm init 2>/dev/null || true
	docker stack deploy -c docker-compose.yml $(STACK_NAME)

test: build
	docker swarm init 2>/dev/null || true
	docker stack deploy -c docker-compose.yml $(STACK_NAME)
	@echo "Waiting for services to start..."
	@sleep 15
	@echo ""
	@echo "=== Testing Keystone Gateway ==="
	@echo "Root (should return 404):"
	@curl -s -o /dev/null -w "  Status: %{http_code}\n" http://localhost:8080/ || echo "  Connection failed"
	@echo ""
	@echo "ACME tenant:"
	@echo -n "  Service: "
	@curl -s http://localhost:8080/acme/ 2>/dev/null || echo "Not responding"
	@echo -n "  Health: "
	@curl -s http://localhost:8080/acme/health 2>/dev/null || echo "Not responding"
	@echo ""
	@echo "Beta tenant:"
	@echo -n "  Service: "
	@curl -s http://localhost:8080/beta/ 2>/dev/null || echo "Not responding"
	@echo -n "  Status: "
	@curl -s http://localhost:8080/beta/status 2>/dev/null || echo "Not responding"
	@echo ""
	@echo "✅ End-to-end test completed"

stop:
	docker stack rm $(STACK_NAME)

logs:
	docker service logs -f $(STACK_NAME)_keystone-gateway

status:
	docker stack services $(STACK_NAME)

clean: stop
	docker image rm $(APP_NAME):latest 2>/dev/null || true
	docker system prune -f
