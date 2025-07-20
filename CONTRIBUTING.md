# Contributing to Keystone Gateway

Thank you for your interest in contributing to Keystone Gateway! This document provides guidelines and information for contributors.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Standards](#code-standards)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Project Structure](#project-structure)
- [Lua Script Development](#lua-script-development)
- [Documentation Guidelines](#documentation-guidelines)
- [Issue Reporting](#issue-reporting)

## Getting Started

### Prerequisites

- **Go 1.21 or later**
- **Git** for version control
- **Basic knowledge of Go** for core gateway development
- **Basic knowledge of Lua** for routing script development
- **Docker** (optional, for containerized testing)

### First-Time Setup

1. **Fork and clone the repository:**
   ```bash
   git clone https://github.com/your-username/keystone-gateway.git
   cd keystone-gateway
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   go mod verify
   ```

3. **Verify your setup:**
   ```bash
   go build -o keystone-gateway ./cmd/
   ./keystone-gateway --help
   ```

4. **Run example configuration:**
   ```bash
   # Create a simple test config
   cp configs/examples/simple.yaml test-config.yaml
   
   # Start the gateway
   ./keystone-gateway -config test-config.yaml
   ```

## Development Setup

### Building the Project

```bash
# Build for development
go build -o keystone-gateway ./cmd/

# Build with race detection (for testing)
go build -race -o keystone-gateway ./cmd/

# Cross-platform build (Linux)
GOOS=linux GOARCH=amd64 go build -o keystone-gateway-linux ./cmd/
```

### Development Workflow

```bash
# Format code
go fmt ./...

# Lint code
go vet ./...

# Run tests (when implemented)
go test ./...

# Run with race detection
go test -race ./...
```

### Local Development Environment

Create a development configuration:

```yaml
# dev-config.yaml
admin_base_path: "/admin"

lua_routing:
  enabled: true
  scripts_dir: "./scripts"

tenants:
  - name: "dev"
    domains: ["localhost", "127.0.0.1"]
    lua_routes: "development-routes.lua"
    health_interval: 60
    services:
      - name: "mock-backend"
        url: "http://localhost:3001"
        health: "/health"
```

### Mock Backend for Testing

Create a simple mock backend for development:

```go
// mock-backend.go
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"
)

func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
    })
    
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"message": "Mock backend", "path": "%s", "method": "%s"}`, r.URL.Path, r.Method)
    })
    
    log.Println("Mock backend starting on :3001")
    log.Fatal(http.ListenAndServe(":3001", nil))
}
```

Run the mock backend: `go run mock-backend.go`

## Code Standards

### Go Code Style

Follow standard Go conventions:

1. **Formatting:**
   - Use `go fmt` for consistent formatting
   - Use `gofmt -s` for simplified code

2. **Naming:**
   - Use camelCase for variables and functions
   - Use PascalCase for exported functions and types
   - Use descriptive names (avoid abbreviations)

3. **Package structure:**
   - Keep packages focused and cohesive
   - Use internal/ for non-exported packages
   - Avoid circular dependencies

4. **Error handling:**
   ```go
   // Good: Explicit error handling
   result, err := someFunction()
   if err != nil {
       return fmt.Errorf("failed to process: %w", err)
   }
   
   // Avoid: Ignoring errors
   result, _ := someFunction()
   ```

5. **Documentation:**
   ```go
   // Package config provides configuration management for Keystone Gateway.
   package config
   
   // LoadConfig reads and validates a configuration file.
   // It returns an error if the file is invalid or cannot be read.
   func LoadConfig(path string) (*Config, error) {
       // Implementation...
   }
   ```

### Lua Code Style

For Lua routing scripts:

1. **Consistent indentation** (4 spaces)
2. **Descriptive function names**
3. **Comment complex logic**
4. **Use local variables** when possible

```lua
-- Good: Well-structured Lua code
local function validate_auth_token(token)
    if not token or string.len(token) < 10 then
        return false
    end
    -- Validation logic...
    return true
end

chi_route("GET", "/api/users", function(request, response)
    local auth_token = request.headers["Authorization"]
    
    if not validate_auth_token(auth_token) then
        response:status(401)
        response:write('{"error": "Invalid token"}')
        return
    end
    
    -- Route logic...
end)
```

## Testing Guidelines

### Test Structure

**Note:** The project currently lacks a comprehensive testing framework. Contributing test infrastructure is highly encouraged!

Proposed test structure:
```
test/
├── unit/           # Unit tests for individual packages
├── integration/    # Integration tests for components
├── e2e/           # End-to-end tests
├── fixtures/      # Test data and configurations
└── mocks/         # Mock implementations
```

### Writing Tests

When contributing tests, follow these guidelines:

1. **Unit tests** for individual functions and methods
2. **Integration tests** for component interactions
3. **End-to-end tests** for complete workflows

```go
// Example unit test structure
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name        string
        config      Config
        expectError bool
    }{
        {
            name: "valid config",
            config: Config{
                LuaRouting: LuaConfig{Enabled: true},
                Tenants: []TenantConfig{
                    {Name: "test", Domains: []string{"localhost"}},
                },
            },
            expectError: false,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.expectError && err == nil {
                t.Error("expected error but got none")
            }
            if !tt.expectError && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
        })
    }
}
```

### Lua Script Testing

Test Lua scripts by creating test scenarios:

```lua
-- test-routes.lua
-- Test routing script with validation

function test_auth_validation()
    local valid_token = "valid-token-12345"
    local invalid_token = "invalid"
    
    assert(validate_auth_token(valid_token) == true, "Valid token should pass")
    assert(validate_auth_token(invalid_token) == false, "Invalid token should fail")
    
    log("Auth validation tests passed")
end

-- Run tests if in test mode
if os.getenv("RUN_TESTS") == "true" then
    test_auth_validation()
end
```

## Pull Request Process

### Before Submitting

1. **Ensure your code follows the style guidelines**
2. **Add or update tests** for your changes
3. **Update documentation** if needed
4. **Test your changes** thoroughly
5. **Verify no breaking changes** unless intentional

### Pull Request Checklist

- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Tests added or updated
- [ ] Documentation updated
- [ ] No breaking changes (or clearly documented)
- [ ] Commit messages are clear and descriptive

### Commit Message Format

Use clear, descriptive commit messages:

```bash
# Good commit messages
git commit -m "feat: add JWT authentication middleware for Lua scripts"
git commit -m "fix: handle empty request body in routing logic"
git commit -m "docs: add configuration examples for multi-tenant setup"
git commit -m "refactor: simplify tenant validation logic"

# Avoid vague messages
git commit -m "fix stuff"
git commit -m "updates"
git commit -m "changes"
```

### Pull Request Description Template

```markdown
## Description
Brief description of what this PR does.

## Type of Change
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that causes existing functionality to change)
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] No breaking changes (or documented)
```

## Project Structure

Understanding the project structure helps with contributions:

```
keystone-gateway/
├── cmd/                    # Main application entry points
│   └── main.go            # Gateway executable
├── internal/              # Private packages
│   ├── config/           # Configuration management
│   ├── routing/          # Core routing logic
│   └── lua/              # Lua engine integration
├── configs/              # Configuration files
│   └── examples/         # Example configurations
├── scripts/              # Lua routing scripts
│   └── examples/         # Example Lua scripts
├── docs/                 # Documentation
├── test/                 # Test files (to be implemented)
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── README.md            # Project overview
├── CONTRIBUTING.md      # This file
├── CHANGELOG.md         # Version history
└── LICENSE              # License information
```

### Key Packages

- **`cmd/`**: Application entry points and CLI handling
- **`internal/config/`**: YAML configuration parsing and validation
- **`internal/routing/`**: HTTP routing, load balancing, health checks
- **`internal/lua/`**: Lua scripting engine and Chi router bindings

## Lua Script Development

### Adding New Lua Functions

When adding new Lua functions to the gateway:

1. **Define the function** in the appropriate Go package
2. **Register it** with the Lua state
3. **Document it** in the Lua scripting guide
4. **Add examples** showing usage

```go
// Example: Adding a new Lua function
func registerCustomFunctions(L *lua.LState) {
    L.SetGlobal("custom_function", L.NewFunction(luaCustomFunction))
}

func luaCustomFunction(L *lua.LState) int {
    // Implementation...
    return 1 // Number of return values
}
```

### Lua Best Practices

1. **Error handling** in Lua functions
2. **Performance considerations** for frequently called functions
3. **Security** - validate all inputs
4. **Documentation** - comment complex logic

## Documentation Guidelines

### Types of Documentation

1. **Code comments** - Explain complex logic
2. **Package documentation** - Describe package purpose
3. **API documentation** - Document public interfaces
4. **User guides** - Help users accomplish tasks
5. **Examples** - Show practical usage

### Documentation Standards

1. **Keep it current** - Update docs with code changes
2. **Be clear and concise** - Avoid unnecessary complexity
3. **Use examples** - Show practical usage
4. **Follow KISS principles** - Keep It Simple, Stupid

### Adding Examples

When adding new features, include examples:

1. **Configuration examples** in `configs/examples/`
2. **Lua script examples** in `scripts/examples/`
3. **Documentation examples** in relevant docs

## Issue Reporting

### Bug Reports

Include the following information:

1. **Gateway version** and Go version
2. **Operating system** and architecture
3. **Configuration file** (sanitized)
4. **Lua scripts** involved (if applicable)
5. **Steps to reproduce** the issue
6. **Expected vs actual behavior**
7. **Logs** with error messages

### Feature Requests

Describe:

1. **Use case** - What problem does this solve?
2. **Proposed solution** - How should it work?
3. **Alternatives considered** - Other approaches?
4. **Additional context** - Examples, mockups, etc.

### Security Issues

For security-related issues:

1. **Do not** create public issues
2. **Email** maintainers directly
3. **Provide** detailed information privately
4. **Allow time** for responsible disclosure

## Getting Help

- **Documentation**: Check the [docs/](docs/) directory
- **Examples**: See [configs/examples/](configs/examples/) and [scripts/examples/](scripts/examples/)
- **Issues**: Search existing issues on GitHub
- **Discussions**: Use GitHub Discussions for questions

## Recognition

Contributors will be recognized in:

- Release notes for significant contributions
- CONTRIBUTORS file (when created)
- GitHub contributor statistics

Thank you for contributing to Keystone Gateway!