# Project Structure Guide

This document describes the updated project structure for Keystone Gateway, following Go best practices and 2025 standards.

## Directory Structure

```
keystone-gateway/
├── 📂 cmd/                          # Application entry points
│   └── main.go                      # Gateway application main
├── 📂 internal/                     # Private Go packages (gateway-specific)
│   ├── config/                      # Configuration management
│   ├── lua/                         # Lua engine integration
│   └── routing/                     # HTTP routing and load balancing
├── 📂 pkg/                          # Public Go packages (potentially reusable)
│   ├── config/                      # Reusable configuration utilities
│   ├── gateway/                     # Core gateway functionality
│   └── luaengine/                   # Lua engine wrapper
├── 📂 configs/                      # Configuration files
│   ├── defaults/                    # Default configuration values
│   │   └── gateway.yaml             # Default gateway settings
│   ├── environments/                # Environment-specific configs
│   │   ├── production-high-load.yaml
│   │   └── staging.yaml
│   └── examples/                    # Example configurations
│       ├── development.yaml
│       ├── multi-tenant.yaml
│       ├── production.yaml
│       └── simple.yaml
├── 📂 scripts/                      # Scripts and automation
│   ├── lua/                         # Lua routing scripts
│   │   ├── examples/                # Example Lua scripts
│   │   │   ├── api-routes.lua       # Basic API routes
│   │   │   ├── auth-routes.lua      # Authentication example
│   │   │   └── rate-limiting.lua    # Rate limiting example
│   │   ├── utils/                   # Lua utility functions
│   │   │   └── common.lua           # Common utilities
│   │   ├── api-routes.lua           # Active API routes
│   │   └── README.md                # Lua scripting guide
│   └── README.md                    # Scripts documentation
├── 📂 tests/                        # Comprehensive test suite
│   ├── unit/                        # Unit tests
│   ├── integration/                 # Integration tests
│   ├── e2e/                         # End-to-end tests
│   ├── fixtures/                    # Test fixtures
│   └── README.md                    # Testing documentation
├── 📂 docs/                         # Documentation
│   ├── api/                         # API documentation
│   ├── deployment/                  # Deployment guides
│   ├── development/                 # Development guides
│   │   ├── getting-started.md       # Development setup
│   │   ├── configuration.md         # Configuration guide
│   │   ├── lua-scripting.md         # Lua API reference
│   │   └── README.md                # Development overview
│   ├── examples/                    # Usage examples
│   ├── project-structure.md         # This file
│   └── README.md                    # Documentation index
├── 📂 tools/                        # Development tools
│   ├── validate-branching-strategy.sh # Branch validation
│   └── README.md                    # Tools documentation
├── 📂 deployments/                  # Deployment configurations
│   └── docker/                      # Docker deployment files
├── 🐳 docker-compose.production.yml # Production deployment
├── 🔨 Makefile                      # Unified build system
├── 📋 README.md                     # Project overview
└── 📄 go.mod                        # Go module definition
```

## Directory Purposes

### `/cmd/`
Application entry points. Contains the main package for the gateway application.

### `/internal/`
Private packages specific to Keystone Gateway. Code here cannot be imported by other projects.
- **`config/`** - Configuration loading and validation
- **`lua/`** - Lua engine integration and state management
- **`routing/`** - HTTP routing, load balancing, and gateway logic

### `/pkg/` (Future)
Public packages that could potentially be reused by other projects. Currently prepared but not populated.
- **`config/`** - Reusable configuration utilities
- **`gateway/`** - Core gateway functionality that could be reused
- **`luaengine/`** - Lua engine wrapper for general use

### `/configs/`
Configuration files organized by purpose:
- **`defaults/`** - Default configuration values and templates
- **`environments/`** - Environment-specific configuration overrides
- **`examples/`** - Example configurations for different use cases

### `/scripts/`
Automation scripts and tools:
- **`lua/`** - Lua scripts for dynamic routing
  - **`examples/`** - Example scripts for common patterns
  - **`utils/`** - Reusable Lua utility functions

### `/tests/`
Comprehensive test suite with clear separation:
- **`unit/`** - Unit tests for individual functions/methods
- **`integration/`** - Integration tests for component interaction
- **`e2e/`** - End-to-end tests for complete scenarios
- **`fixtures/`** - Shared test fixtures and utilities

### `/docs/`
Documentation organized by audience and purpose:
- **`api/`** - API documentation and references
- **`deployment/`** - Deployment guides and configuration
- **`development/`** - Development guides and setup instructions
- **`examples/`** - Usage examples and tutorials

### `/tools/`
Development and maintenance tools:
- Validation scripts
- Build utilities
- Development helpers

### `/deployments/`
Deployment-specific configurations:
- Docker configurations
- Kubernetes manifests (future)
- Environment-specific deployment files

## Design Principles

### 1. **Clear Separation of Concerns**
Each directory has a specific purpose and contains related functionality.

### 2. **Progressive Disclosure**
Structure supports both simple and complex use cases, allowing users to start simple and add complexity as needed.

### 3. **Developer Experience**
Tools, documentation, and examples are easily discoverable and well-organized.

### 4. **Maintainability**
Code is organized to minimize coupling and make changes easy to implement and test.

### 5. **Standard Compliance**
Follows Go community standards and conventions for project layout.

## Migration Benefits

This reorganized structure provides:

1. **Better Developer Onboarding** - Clear structure helps new developers understand the codebase
2. **Improved Maintainability** - Related functionality is grouped together
3. **Enhanced Reusability** - `pkg/` directory prepared for future code sharing
4. **Better Documentation** - Organized by audience and purpose
5. **Clearer Testing Strategy** - Test types are clearly separated
6. **Tool Consolidation** - All development tools in one location
7. **Example-Driven Learning** - Comprehensive examples for common use cases

## Future Considerations

- **`/pkg/` Population** - Move appropriate code from `internal/` to `pkg/` when ready for external use
- **API Documentation** - Generate OpenAPI/Swagger documentation in `docs/api/`
- **Kubernetes Support** - Add Kubernetes manifests to `deployments/k8s/`
- **Plugin Architecture** - Consider `plugins/` directory for extensibility