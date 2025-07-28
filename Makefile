# Keystone Gateway Makefile

.PHONY: build test test-unit test-integration test-e2e test-bench test-load test-perf test-all coverage clean help

# Build the application
build:
	go build -o bin/keystone-gateway cmd/main.go

# Run core tests (unit, integration, e2e)
test: test-unit test-integration test-e2e

# Run all tests including performance tests
test-all: test test-bench test-load test-perf

# Run unit tests
test-unit:
	go test -v ./tests/unit/...

# Run integration tests  
test-integration:
	go test -v ./tests/integration/...

# Run E2E tests
test-e2e:
	go test -v ./tests/e2e/...

# Run benchmark tests
test-bench:
	go test -bench=. -benchmem -benchtime=3s ./tests

# Run load tests
test-load:
	go test -run="TestConcurrentRequests|TestMemoryUsage" -timeout=2m ./tests

# Run performance regression tests
test-perf:
	go test -run="TestPerformanceRegression|TestPerformanceHistory" -timeout=1m ./tests

# Run quick benchmarks (shorter duration)
test-bench-quick:
	go test -bench=. -benchmem -benchtime=1s ./tests

# Run tests with coverage
coverage:
	go test -v -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/... ./tests/integration/... ./tests
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with coverage and display in terminal
coverage-text:
	go test -v -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/... ./tests/integration/... ./tests
	go tool cover -func=coverage.out

# Generate comprehensive coverage including all test types
coverage-full:
	go test -v -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/... ./tests/integration/... ./tests
	go tool cover -html=coverage.out -o coverage-full.html
	go tool cover -func=coverage.out
	@echo "Full coverage report generated: coverage-full.html"

# Clean build artifacts
clean:
	rm -f bin/keystone-gateway
	rm -f coverage.out coverage.html coverage-full.html
	rm -f tests/performance_baselines.json tests/performance_history.json
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
	@echo ""
	@echo "Building:"
	@echo "  build           - Build the application binary"
	@echo "  run             - Run with example config"
	@echo ""
	@echo "Core Testing:"
	@echo "  test            - Run core tests (unit, integration, e2e)"
	@echo "  test-all        - Run all tests including performance tests"
	@echo "  test-unit       - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-e2e        - Run end-to-end tests only"
	@echo "  test-short      - Run tests in short mode (no E2E)"
	@echo ""
	@echo "Performance Testing:"
	@echo "  test-bench      - Run benchmark tests (3s duration)"
	@echo "  test-bench-quick - Run quick benchmarks (1s duration)"
	@echo "  test-load       - Run load and concurrency tests"
	@echo "  test-perf       - Run performance regression tests"
	@echo ""
	@echo "Coverage:"
	@echo "  coverage        - Generate HTML coverage report"
	@echo "  coverage-text   - Show coverage in terminal"
	@echo "  coverage-full   - Generate comprehensive coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt             - Format Go code"
	@echo "  lint            - Run linter"
	@echo "  deps            - Install and tidy dependencies"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean           - Clean build artifacts and test files"
	@echo "  help            - Show this help message"