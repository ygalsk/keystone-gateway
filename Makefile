# Keystone Gateway Makefile

.PHONY: build test test-unit test-integration test-e2e coverage clean help

# Build the application
build:
	go build -o bin/keystone-gateway cmd/main.go

# Run all tests
test: test-unit test-integration test-e2e

# Run unit tests
test-unit:
	go test -v ./tests/unit/...

# Run integration tests  
test-integration:
	go test -v ./tests/integration/...

# Run E2E tests
test-e2e:
	go test -v ./tests/e2e/...

# Run tests with coverage
coverage:
	go test -v -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/... ./tests/integration/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with coverage and display in terminal
coverage-text:
	go test -v -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/... ./tests/integration/...
	go tool cover -func=coverage.out

# Clean build artifacts
clean:
	rm -f bin/keystone-gateway
	rm -f coverage.out coverage.html
	rm -f main

# Run the application with example config
run:
	go run cmd/main.go -config configs/examples/simple.yaml

# Run tests in short mode (skips E2E)
test-short:
	go test -short -v ./tests/...

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download
	go mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  build           - Build the application binary"
	@echo "  test            - Run all tests (unit, integration, e2e)"
	@echo "  test-unit       - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-e2e        - Run end-to-end tests only"
	@echo "  test-short      - Run tests in short mode (no E2E)"
	@echo "  coverage        - Generate HTML coverage report"
	@echo "  coverage-text   - Show coverage in terminal"
	@echo "  run             - Run with example config"
	@echo "  fmt             - Format Go code"
	@echo "  lint            - Run linter"
	@echo "  deps            - Install and tidy dependencies"
	@echo "  clean           - Clean build artifacts"
	@echo "  help            - Show this help message"