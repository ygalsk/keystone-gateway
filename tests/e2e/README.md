# End-to-End (E2E) Tests

This directory contains end-to-end tests for the Keystone Gateway project. E2E tests verify complete user workflows and real-world scenarios by testing the full request lifecycle from client to backend.

## Directory Structure

```
tests/e2e/
├── README.md                          # This file - E2E testing documentation
├── fixtures/                          # E2E-specific test fixtures and utilities
│   ├── e2e_gateway.go                 # E2E gateway setup with real server instances
│   ├── e2e_backends.go                # Real backend server management for E2E
│   ├── e2e_client.go                  # HTTP client utilities for E2E testing
│   └── e2e_scenarios.go               # Common E2E test scenario builders
├── gateway_e2e_test.go                # Core gateway E2E tests
├── multitenant_e2e_test.go            # Multi-tenant E2E scenarios
├── lua_integration_e2e_test.go        # Lua script execution E2E tests
└── performance_e2e_test.go            # Performance and load E2E tests
```

## Test Categories

### 1. Core Gateway E2E Tests (`gateway_e2e_test.go`)
- Full request lifecycle testing
- Real HTTP client → Gateway → Backend flows
- Configuration loading and application
- Error handling across the full stack

### 2. Multi-Tenant E2E Tests (`multitenant_e2e_test.go`)
- Real-world multi-tenant scenarios
- Host-based and path-based routing E2E
- Tenant isolation verification
- Load balancing across tenants

### 3. Lua Integration E2E Tests (`lua_integration_e2e_test.go`)
- Lua script execution in real request context
- Middleware application E2E
- Dynamic routing through Lua
- Performance impact of Lua processing

### 4. Performance E2E Tests (`performance_e2e_test.go`)
- Load testing with real traffic
- Concurrent request handling
- Memory usage monitoring
- Performance regression detection

## E2E vs Integration Tests

**Integration Tests** (`../integration/`):
- Test component interactions using test fixtures
- Use `httptest.Server` for mock backends
- Fast execution, isolated environments
- Focus on component integration correctness

**E2E Tests** (`./`):
- Test complete user workflows with real servers
- Use actual HTTP clients and real network requests
- Slower execution, real-world scenarios
- Focus on end-user experience and system behavior

## Test Patterns

### E2E Gateway Setup
```go
// Start real gateway server
gateway := fixtures.StartRealGateway(t, config)
defer gateway.Stop()

// Start real backend servers
backend := fixtures.StartRealBackend(t, ":8080")
defer backend.Stop()

// Use real HTTP client
client := fixtures.NewE2EClient()
resp, err := client.Get(gateway.URL + "/api/test")
```

### E2E Test Structure
```go
func TestE2EScenario(t *testing.T) {
    // 1. Setup: Start real servers
    // 2. Execute: Real HTTP requests
    // 3. Verify: End-to-end behavior
    // 4. Cleanup: Stop servers
}
```

## Running E2E Tests

```bash
# Run all E2E tests
go test ./tests/e2e/ -v

# Run specific E2E test file
go test ./tests/e2e/gateway_e2e_test.go -v

# Run E2E tests with timeout (for longer scenarios)
go test ./tests/e2e/ -v -timeout 5m

# Run E2E tests with race detection
go test ./tests/e2e/ -v -race
```

## E2E Test Guidelines

1. **Real Servers**: Use actual HTTP servers, not test fixtures
2. **Real Clients**: Use standard HTTP clients for realistic requests
3. **Full Lifecycle**: Test complete request/response cycles
4. **Cleanup**: Always stop servers and clean up resources
5. **Realistic Data**: Use production-like configurations and payloads
6. **Performance Aware**: Monitor test execution time and resource usage
7. **Flaky Test Prevention**: Handle timing issues and race conditions

## Configuration

E2E tests use temporary configurations and random ports to avoid conflicts:

```go
config := &config.Config{
    Port: fixtures.GetRandomPort(),
    Tenants: []config.Tenant{
        // Realistic tenant configurations
    },
}
```

## Debugging E2E Tests

- Use `t.Logf()` for detailed request/response logging
- Enable verbose HTTP client logging for troubleshooting
- Check server logs for backend interaction details
- Use network inspection tools for complex scenarios

## Best Practices

1. **Test Independence**: Each E2E test should be completely independent
2. **Resource Management**: Always clean up servers and connections
3. **Realistic Scenarios**: Model real-world usage patterns
4. **Error Handling**: Test both success and failure scenarios
5. **Performance Monitoring**: Track execution time and resource usage
6. **Documentation**: Document complex E2E scenarios and their purpose