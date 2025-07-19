# Contributing Guide

**How to contribute to Keystone Gateway**

## Quick Start

```bash
# Fork and clone the repository
git clone https://github.com/your-username/keystone-gateway
cd keystone-gateway

# Set up development environment
make install-deps

# Run complete development workflow
make dev
```

## Development Workflow

### 1. Setup
```bash
# Install dependencies
make deps

# Verify everything works
make test
```

### 2. Make Changes
```bash
# Create feature branch
git checkout -b feature/your-feature-name

# Make your changes
# ... edit code ...

# Test changes
make dev
```

### 3. Validate
```bash
# Format code
make fmt

# Run linting
make lint

# Run all tests
make test

# Test with race detection
make test-race
```

### 4. Submit
```bash
# Commit changes
git add .
git commit -m "feat: add your feature description"

# Push and create PR
git push origin feature/your-feature-name
```

## Code Standards

### Go Code Style
- **Format**: Use `go fmt` (run `make fmt`)
- **Linting**: Pass `go vet` (run `make lint`)
- **Naming**: Follow Go naming conventions
- **Comments**: Document all exported functions and types
- **Errors**: Handle all errors explicitly

### Commit Messages
Follow conventional commit format:
```
type(scope): description

feat(routing): add hybrid routing support
fix(health): resolve timeout issue
docs(api): update health endpoint docs
test(proxy): add integration tests
refactor(config): simplify YAML structure
```

### Code Review Checklist
- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] No linting errors (`make lint`)
- [ ] Documentation updated
- [ ] Commit messages follow convention
- [ ] No breaking changes (unless major version)

## Testing

### Unit Tests
```bash
# Run all tests
make test

# Run specific test
go test -run TestSpecificFunction

# Run with verbose output
go test -v ./...
```

### Integration Tests
```bash
# Run local integration test
make test-local

# Test Docker deployment
make docker
docker run --rm keystone-gateway:latest -help
```

### Performance Tests
```bash
# Simple performance test
make perf

# Manual load testing (if ab available)
ab -n 1000 -c 10 http://localhost:8080/admin/health
```

## Making Changes

### Adding Features

1. **Design first**: Open an issue to discuss the feature
2. **Start small**: Create minimal working implementation
3. **Add tests**: Ensure feature is well-tested
4. **Update docs**: Document new functionality
5. **Consider backwards compatibility**: Avoid breaking changes

### Fixing Bugs

1. **Reproduce**: Create test that demonstrates the bug
2. **Fix**: Implement the minimal fix
3. **Verify**: Ensure fix resolves the issue
4. **Test**: Add regression test if possible

### Improving Documentation

1. **User-focused**: Write for the target audience
2. **Examples**: Include practical examples
3. **Clear structure**: Use consistent formatting
4. **Test examples**: Verify all code examples work

## Project Structure

```
keystone-gateway/
├── cmd/chi-stone/       # Main gateway binary entry point
├── internal/            # Internal packages (config, routing, health, proxy)
├── pkg/client/          # Public API packages
├── test/               # All test files (unit, integration, e2e)
├── Makefile            # Development workflow
├── Dockerfile          # Container build
├── configs/            # Configuration examples
├── docs/               # Documentation
├── mock-backends/      # Testing backends
└── todos/              # Development tracking
```

### Key Files

- **`cmd/chi-stone/main.go`**: Main application entry point (~280 lines)
- **`internal/config/config.go`**: Configuration management
- **`internal/routing/gateway.go`**: Core routing logic
- **`test/integration/gateway_test.go`**: Integration tests  
- **`Makefile`**: All development commands
- **`DEVELOPMENT.md`**: Development workflow guide
- **`configs/config.yaml`**: Development configuration

## Architecture Guidelines

### Design Principles

1. **Simplicity**: Keep it simple and focused
2. **Performance**: Optimize for speed and efficiency  
3. **Reliability**: Handle errors gracefully
4. **Maintainability**: Write clear, understandable code
5. **Testability**: Make code easy to test

### Code Organization

- **Modular packages**: Core logic separated into internal packages
- **Clear separation**: Distinct packages for different concerns (config, routing, health, proxy)
- **Minimal dependencies**: Only essential external packages
- **Standard library first**: Prefer standard library when possible

### Configuration

- **YAML-based**: Simple, human-readable configuration
- **Validation**: Validate configuration at startup
- **Defaults**: Provide sensible defaults
- **Environment**: Support environment variable overrides

## Common Tasks

### Adding a New Configuration Option

1. **Update structs**: Add field to appropriate struct in `internal/config/config.go`
2. **Add validation**: Include validation in `validateTenant()` in config package
3. **Update docs**: Document in `docs/configuration.md`
4. **Add tests**: Test new configuration option
5. **Update examples**: Include in example configs

### Adding a New Endpoint

1. **Define handler**: Create handler function
2. **Add route**: Register route in `SetupRouter()`
3. **Add tests**: Create integration test
4. **Update docs**: Document in `docs/api-reference.md`
5. **Security review**: Consider security implications

### Improving Performance

1. **Benchmark first**: Establish baseline performance
2. **Profile**: Use Go profiling tools to identify bottlenecks
3. **Optimize**: Make targeted improvements
4. **Measure**: Verify improvements with benchmarks
5. **Document**: Update performance documentation

## Release Process

### Version Strategy

- **Semantic versioning**: MAJOR.MINOR.PATCH
- **Backwards compatibility**: Maintain within major versions
- **Feature releases**: Increment minor version
- **Bug fixes**: Increment patch version

### Release Checklist

- [ ] All tests pass
- [ ] Documentation updated
- [ ] Performance tested
- [ ] Security reviewed
- [ ] Example configurations updated
- [ ] CHANGELOG.md updated

## Getting Help

### Development Questions

- **GitHub Discussions**: Ask questions about development
- **Code Review**: Request feedback on implementation approach
- **Design Discussions**: Discuss architectural decisions

### Issue Guidelines

**Bug Reports:**
- Include reproduction steps
- Provide configuration (sanitized)
- Include error logs
- Specify environment details

**Feature Requests:**
- Describe use case and problem
- Explain proposed solution
- Consider implementation complexity
- Discuss alternatives

### Communication

- **Be respectful**: Maintain professional, friendly communication
- **Be clear**: Provide specific, actionable feedback
- **Be patient**: Allow time for review and discussion
- **Be helpful**: Support other contributors

## Code Examples

### Adding a New Route

```go
func (gw *Gateway) SetupRouter() *chi.Mux {
    r := chi.NewRouter()
    
    // ... existing middleware ...
    
    r.Route("/admin", func(r chi.Router) {
        r.Get("/health", gw.HealthHandler)
        r.Get("/tenants", gw.TenantsHandler)
        r.Get("/metrics", gw.MetricsHandler) // New endpoint
    })
    
    return r
}

func (gw *Gateway) MetricsHandler(w http.ResponseWriter, r *http.Request) {
    metrics := gw.collectMetrics()
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(metrics); err != nil {
        http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
        return
    }
}
```

### Adding Configuration Validation

```go
func validateTenant(tenant Tenant) error {
    if tenant.Name == "" {
        return fmt.Errorf("tenant name is required")
    }
    
    // New validation
    if tenant.NewField < 0 {
        return fmt.Errorf("new_field must be positive")
    }
    
    return nil
}
```

### Adding Tests

```go
func TestNewFeature(t *testing.T) {
    // Test setup
    config := &Config{
        Tenants: []Tenant{{
            Name: "test",
            // ... config ...
        }},
    }
    
    gateway := NewGateway(config)
    
    // Test execution
    result := gateway.NewFeature()
    
    // Assertions
    assert.Equal(t, expectedResult, result)
}
```

## Thank You!

Thank you for contributing to Keystone Gateway! Your contributions help make this project better for everyone.