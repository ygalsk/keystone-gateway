# Development Documentation

This directory contains comprehensive development documentation for Keystone Gateway.

## Available Guides

### Core Development
- **[Getting Started](getting-started.md)** - Step-by-step setup and first steps
- **[Configuration](configuration.md)** - Complete configuration reference
- **[Lua Scripting](lua-scripting.md)** - Comprehensive Lua API documentation

### Architecture & Design
- **[Project Structure](#project-structure)** - Understanding the codebase organization
- **[Testing Strategy](#testing-strategy)** - How to test your changes effectively

## Project Structure

```
keystone-gateway/
├── cmd/                     # Application entry points
├── internal/                # Private Go packages (gateway-specific)
│   ├── config/                # Configuration management
│   ├── lua/                   # Lua engine integration
│   └── routing/               # HTTP routing and load balancing
├── pkg/                     # Public Go packages (potentially reusable)
│   ├── config/                # Reusable configuration utilities
│   ├── gateway/               # Core gateway functionality
│   └── luaengine/             # Lua engine wrapper
├── configs/                 # Configuration files
│   ├── defaults/              # Default configuration values
│   ├── environments/          # Environment-specific configs
│   └── examples/              # Example configurations
├── scripts/                 # Scripts and tools
│   └── lua/                   # Lua routing scripts
│       ├── examples/          # Example Lua scripts
│       └── utils/             # Lua utility functions
├── tests/                   # Comprehensive test suite
│   ├── unit/                  # Unit tests
│   ├── integration/           # Integration tests
│   └── e2e/                   # End-to-end tests
├── docs/                    # Documentation
│   ├── api/                   # API documentation
│   ├── deployment/            # Deployment guides
│   ├── development/           # Development guides (this directory)
│   └── examples/              # Usage examples
├── tools/                   # Development tools
└── deployments/             # Deployment configurations
```

## Testing Strategy

### Test Types
1. **Unit Tests** (`tests/unit/`) - Test individual functions and methods
2. **Integration Tests** (`tests/integration/`) - Test component interactions
3. **E2E Tests** (`tests/e2e/`) - Test complete user scenarios
4. **Load Tests** (`tests/`) - Performance and scalability testing

### Running Tests
```bash
# Run all tests
make test

# Run specific test type
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/e2e/...

# Run with coverage
go test -cover ./...
```

## Development Workflow

1. **Create feature branch** from `main`
2. **Implement changes** following Go best practices
3. **Add/update tests** for your changes
4. **Update documentation** if needed
5. **Run tests and linting** locally
6. **Submit pull request** with clear description

## Code Standards

- Follow standard Go conventions and idioms
- Use `gofmt` for code formatting
- Handle errors explicitly
- Write clear, descriptive comments
- Prefer composition over inheritance
- Keep packages small and focused

## Getting Help

- Check existing documentation in `docs/`
- Look at examples in `configs/examples/` and `scripts/lua/examples/`
- Review test files for usage patterns
- Consult the main [README.md](../../README.md) for overview