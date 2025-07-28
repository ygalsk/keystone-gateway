# Keystone Gateway Test Suite

This document provides a comprehensive guide to the test architecture, organization, and best practices for the Keystone Gateway project.

## 📋 Table of Contents

- [Test Architecture Overview](#test-architecture-overview)
- [Test Organization](#test-organization)
- [Running Tests](#running-tests)
- [Test Types](#test-types)
- [Test Patterns and Best Practices](#test-patterns-and-best-practices)
- [Performance Testing](#performance-testing)
- [Fixtures and Utilities](#fixtures-and-utilities)
- [CI/CD Integration](#cicd-integration)
- [Troubleshooting](#troubleshooting)

## 🏗️ Test Architecture Overview

The Keystone Gateway test suite follows a **pyramidal testing architecture** with comprehensive coverage across multiple layers:

```
              🔺 E2E Tests (Real Systems)
            🔺🔺 Integration Tests (Component Interaction)
          🔺🔺🔺 Unit Tests (Individual Functions)
        🔺🔺🔺🔺 Performance Tests (Benchmarks & Load)
```

### Design Principles

- **KISS (Keep It Simple, Stupid)**: Clean, readable test code
- **DRY (Don't Repeat Yourself)**: Reusable fixtures and utilities
- **Go Best Practices**: Table-driven tests, proper error handling
- **Production Readiness**: Real-world scenarios and performance validation

## 📁 Test Organization

```
tests/
├── README.md                          # This documentation
├── unit/                              # Unit tests (individual components)
├── integration/                       # Integration tests (component interaction)
│   ├── basic_test.go                 # Core integration scenarios
│   ├── component_integration_test.go  # Config-to-gateway integration
│   ├── multitenant_integration_test.go # Multi-tenant routing
│   └── backend_health_integration_test.go # Health monitoring
├── e2e/                              # End-to-end tests (full system)
│   ├── README.md                     # E2E specific documentation
│   ├── gateway_e2e_test.go          # Core gateway E2E tests
│   ├── multitenant_e2e_test.go      # Real-world multi-tenant scenarios
│   ├── lua_integration_e2e_test.go  # Lua middleware execution
│   └── fixtures/                    # E2E test utilities
│       ├── e2e_gateway.go           # Real gateway server management
│       ├── e2e_backends.go          # Real backend servers
│       └── e2e_client.go            # HTTP client utilities
├── fixtures/                         # Shared test utilities
│   ├── backends.go                  # Backend server fixtures
│   ├── configs.go                   # Configuration fixtures
│   ├── common.go                    # Common test utilities
│   └── performance_fixtures.go     # Performance testing fixtures
├── benchmark_test.go                # Performance benchmarks
├── load_test.go                     # Load and concurrency testing
├── performance_regression_test.go   # Performance regression tracking
├── performance_baselines.json      # Performance baseline data
└── performance_history.json        # Historical performance data
```

## 🚀 Running Tests

### Quick Start

```bash
# Run core tests (unit, integration, e2e)
make test

# Run all tests including performance tests
make test-all

# Show all available test targets
make help
```

### Test Categories

#### Core Testing
```bash
# Unit tests only
make test-unit

# Integration tests only
make test-integration

# End-to-end tests only
make test-e2e

# Quick tests (excludes E2E)
make test-short
```

#### Performance Testing
```bash
# Run benchmark tests (3s duration)
make test-bench

# Run quick benchmarks (1s duration)
make test-bench-quick

# Run load and concurrency tests
make test-load

# Run performance regression tests
make test-perf
```

#### Coverage Analysis
```bash
# Generate HTML coverage report
make coverage

# Show coverage in terminal
make coverage-text

# Generate comprehensive coverage report
make coverage-full
```

### Direct Go Commands

```bash
# Run specific test patterns
go test -run TestGatewayRouting ./tests/integration/...
go test -run TestLuaMiddleware ./tests/e2e/...

# Run benchmarks with custom parameters
go test -bench=BenchmarkGateway -benchtime=5s -benchmem ./tests

# Run tests with race detection
go test -race ./tests/...

# Run tests with verbose output
go test -v ./tests/...
```

## 🧪 Test Types

### 1. Unit Tests (`tests/unit/`)

**Purpose**: Test individual functions and methods in isolation.

**Characteristics**:
- Fast execution (< 1ms per test)
- No external dependencies
- Mocked dependencies
- High code coverage

**Example**:
```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *config.Config
        wantErr bool
    }{
        {"valid_config", validConfig(), false},
        {"invalid_config", invalidConfig(), true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 2. Integration Tests (`tests/integration/`)

**Purpose**: Test component interactions and data flow.

**Characteristics**:
- Real component integration
- Test fixtures for backends
- Multi-component scenarios
- Medium execution time (< 100ms per test)

**Key Files**:
- `basic_test.go`: Core proxy functionality
- `component_integration_test.go`: Config-to-gateway integration
- `multitenant_integration_test.go`: Multi-tenant routing scenarios
- `backend_health_integration_test.go`: Health monitoring integration

**Example**:
```go
func TestMultiTenantRouting(t *testing.T) {
    // Setup multiple backends
    apiBackend := fixtures.CreateAPIBackend(t)
    defer apiBackend.Close()
    
    webBackend := fixtures.CreateSimpleBackend(t)
    defer webBackend.Close()
    
    // Create multi-tenant configuration
    cfg := fixtures.CreateMultiTenantConfig(apiBackend.URL, webBackend.URL)
    
    // Test routing logic
    gateway := routing.NewGateway(cfg)
    // ... test assertions
}
```

### 3. End-to-End Tests (`tests/e2e/`)

**Purpose**: Test complete system functionality with real HTTP servers.

**Characteristics**:
- Real HTTP servers and network requests
- Complete request lifecycle testing
- Production-like scenarios
- Slower execution (< 1s per test)

**Key Files**:
- `gateway_e2e_test.go`: Full gateway request lifecycle
- `multitenant_e2e_test.go`: Real-world multi-tenant scenarios
- `lua_integration_e2e_test.go`: Lua middleware execution

**Example**:
```go
func TestGatewayE2E(t *testing.T) {
    // Start real backend server
    backend := fixtures.StartRealBackend(t, "api")
    defer backend.Stop()
    
    // Start real gateway server
    gateway := fixtures.StartRealGateway(t, cfg)
    defer gateway.Stop()
    
    // Create HTTP client
    client := fixtures.NewE2EClient()
    client.SetBaseURL(gateway.URL)
    
    // Test real HTTP requests
    resp, err := client.GetResponse("/api/users")
    // ... assertions
}
```

### 4. Performance Tests

#### Benchmarks (`benchmark_test.go`)

**Purpose**: Measure performance characteristics and resource usage.

**Available Benchmarks**:
- `BenchmarkGatewayRouting`: Core routing performance
- `BenchmarkLoadBalancing`: Load balancing across backends
- `BenchmarkMultiTenantRouting`: Multi-tenant routing performance
- `BenchmarkLuaScriptExecution`: Lua middleware performance
- `BenchmarkMemoryUsage`: Memory allocation patterns

**Example**:
```go
func BenchmarkGatewayRouting(b *testing.B) {
    // Setup test environment
    backend := createBenchmarkBackend()
    defer backend.Close()
    
    cfg := fixtures.CreateTestConfig("bench-tenant", "/api/")
    gateway := routing.NewGateway(cfg)
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        // Benchmark code
    }
}
```

#### Load Tests (`load_test.go`)

**Purpose**: Test system behavior under concurrent load and memory pressure.

**Test Categories**:
- `TestConcurrentRequests`: Concurrent load testing
- `TestMemoryUsage`: Memory allocation and leak detection

**Example Results**:
```
Concurrent Load Test Results:
  Total Requests: 1000 (expected 1000)
  Successful: 1000
  Errors: 0
  Duration: 81.785707ms
  Requests/sec: 12227.08
```

#### Performance Regression (`performance_regression_test.go`)

**Purpose**: Detect performance regressions against established baselines.

**Features**:
- JSON-based baseline management
- Automated regression detection
- Historical performance tracking
- Configurable thresholds

## 🎯 Test Patterns and Best Practices

### Table-Driven Tests

**Use for**: Multiple test cases with similar structure.

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
        wantErr  bool
    }{
        {"valid_input", "valid", true, false},
        {"invalid_input", "invalid", false, true},
        {"empty_input", "", false, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Validate(tt.input)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if result != tt.expected {
                t.Errorf("Validate() = %v, expected %v", result, tt.expected)
            }
        })
    }
}
```

### Fixture Usage

**Purpose**: Provide reusable test components and data.

```go
// Create test backends
backend := fixtures.CreateSimpleBackend(t)
defer backend.Close()

// Create test configurations
cfg := fixtures.CreateTestConfig("tenant-name", "/api/")

// Create test gateways
gateway := fixtures.CreateTestGateway(t, cfg)
```

### Subtests for Organization

```go
func TestGatewayFunctionality(t *testing.T) {
    t.Run("routing", func(t *testing.T) {
        // Routing tests
    })
    
    t.Run("load_balancing", func(t *testing.T) {
        // Load balancing tests
    })
    
    t.Run("health_checks", func(t *testing.T) {
        // Health check tests
    })
}
```

### Error Testing

```go
func TestErrorHandling(t *testing.T) {
    // Test expected errors
    _, err := ProcessInvalidInput("invalid")
    if err == nil {
        t.Error("Expected error for invalid input")
    }
    
    // Test specific error types
    if !errors.Is(err, ErrInvalidInput) {
        t.Errorf("Expected ErrInvalidInput, got %v", err)
    }
}
```

## ⚡ Performance Testing

### Benchmark Guidelines

1. **Use realistic data**: Test with production-like configurations
2. **Isolate what you're measuring**: Use `b.ResetTimer()` after setup
3. **Report allocations**: Use `b.ReportAllocs()` for memory analysis
4. **Consistent environment**: Run on dedicated hardware when possible

### Performance Baselines

Default performance expectations:

```json
{
  "gateway_routing": {
    "max_request_duration": "200ms",
    "max_memory_per_request": 50000,
    "min_requests_per_second": 100
  },
  "load_balancing": {
    "max_request_duration": "300ms", 
    "max_memory_per_request": 60000,
    "min_requests_per_second": 80
  }
}
```

### Running Performance Tests

```bash
# Run all benchmarks
make test-bench

# Run specific benchmark patterns
go test -bench=BenchmarkGateway -benchtime=5s ./tests

# Generate performance profiles
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof ./tests

# Update performance baselines (when intentionally improving performance)
UPDATE_PERFORMANCE_BASELINES=true go test -run TestPerformanceRegression ./tests
```

## 🛠️ Fixtures and Utilities

### Backend Fixtures (`fixtures/backends.go`)

```go
// Simple HTTP backend
backend := fixtures.CreateSimpleBackend(t)

// API backend with JSON responses
backend := fixtures.CreateAPIBackend(t)

// Error-generating backend
backend := fixtures.CreateErrorBackend(t)

// Slow backend with configurable delay
backend := fixtures.CreateSlowBackend(t, 100*time.Millisecond)
```

### Configuration Fixtures (`fixtures/configs.go`)

```go
// Single tenant configuration
cfg := fixtures.CreateTestConfig("tenant", "/api/")

// Multi-tenant configuration
cfg := fixtures.CreateMultiTenantConfig()

// Custom configuration
cfg := fixtures.CreateCustomConfig(tenants, services)
```

### E2E Fixtures (`e2e/fixtures/`)

```go
// Real gateway server
gateway := fixtures.StartRealGateway(t, cfg)
defer gateway.Stop()

// Real backend servers
backend := fixtures.StartRealBackend(t, "api")
defer backend.Stop()

// HTTP client for testing
client := fixtures.NewE2EClient()
client.SetBaseURL(gateway.URL)
```

### Performance Fixtures (`fixtures/performance_fixtures.go`)

```go
// Performance test suite
suite := fixtures.NewPerformanceTestSuite(t)
defer suite.Cleanup()

// Specialized backends
fastBackend := fixtures.CreatePerfFastBackend(t)
slowBackend := fixtures.CreatePerfSlowBackend(t)
```

## 🔄 CI/CD Integration

### GitHub Actions Example

```yaml
name: Test Suite
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run tests
        run: make test-all
      
      - name: Generate coverage
        run: make coverage-full
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

### Docker Testing

```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go mod download
RUN make test-all
```

## 🐛 Troubleshooting

### Common Issues

#### 1. Test Timeouts
```bash
# Increase timeout for slow tests
go test -timeout 5m ./tests/e2e/...

# Or use make target with built-in timeout
make test-load  # Has 2m timeout
```

#### 2. Port Conflicts
```go
// E2E tests use random ports to avoid conflicts
port := fixtures.GetRandomPort()
```

#### 3. Memory Issues
```bash
# Run with race detection
go test -race ./tests/...

# Check for memory leaks
go test -run TestMemoryUsage -v ./tests
```

#### 4. Flaky Tests
```bash
# Run tests multiple times
go test -count=10 ./tests/integration/...

# Run specific flaky test
go test -run TestSpecificTest -count=100 ./tests/...
```

### Debug Output

```bash
# Enable verbose logging
go test -v ./tests/...

# Run single test with debugging
go test -run TestSpecificTest -v ./tests/integration/...
```

### Performance Debugging

```bash
# Generate CPU profile
go test -bench=. -cpuprofile=cpu.prof ./tests

# Generate memory profile  
go test -bench=. -memprofile=mem.prof ./tests

# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

## 📊 Test Metrics and Coverage

### Coverage Targets

- **Unit Tests**: > 80% line coverage
- **Integration Tests**: > 70% line coverage  
- **E2E Tests**: > 60% line coverage
- **Overall**: > 75% line coverage

### Performance Targets

- **Gateway Routing**: < 200ms per request, > 100 RPS
- **Load Balancing**: < 300ms per request, > 80 RPS
- **Multi-tenant**: < 250ms per request, > 90 RPS
- **Memory Usage**: < 50KB per request

### Test Execution Time

- **Unit Tests**: < 5 seconds total
- **Integration Tests**: < 30 seconds total
- **E2E Tests**: < 2 minutes total
- **Benchmarks**: < 5 minutes total (3s per benchmark)

## 🔗 Additional Resources

- [Go Testing Package Documentation](https://pkg.go.dev/testing)
- [Go Benchmark Guide](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Keystone Gateway Architecture](../README.md)

---

**Contributing**: When adding new tests, follow the established patterns and update this documentation accordingly. Ensure all tests follow KISS/DRY principles and include appropriate fixtures.

**Questions**: For questions about the test suite, please check the existing test files for examples or create an issue with the `testing` label.