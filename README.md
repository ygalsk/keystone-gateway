# Keystone Gateway

A high-performance, programmable reverse proxy and API gateway written in Go with embedded Lua scripting for dynamic routing in multi-tenant environments.

## Features

- **Multi-tenant routing**: Host-based, path-based, and hybrid routing strategies
- **Embedded Lua scripting**: Dynamic route definition and middleware without recompilation
- **HTTP compression**: Configurable gzip compression for improved performance
- **Load balancing**: Round-robin load balancing with health checking
- **Admin API**: Health monitoring and tenant management endpoints
- **Thread-safe architecture**: Lua state pools and atomic operations for concurrent safety

## 🚀 Quick Start

### Prerequisites

- **Go 1.22 or later**
- **Docker and Docker Compose** for development and deployment
- **Make** for build automation (cross-platform)

### Installation & Development

```bash
# Clone the repository
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway

# View all available commands
make help

# Start development environment
make dev

# Run tests
make test

# Deploy to staging
make staging
```

### Makefile System

Keystone Gateway uses a comprehensive **Makefile system** for all operations:

```bash
# 🏗️  Development
make dev             # Start development environment
make dev-health      # Check development health
make feature-start FEATURE=my-feature  # Start new feature

# 🧪 Testing & Quality
make test            # Run comprehensive tests
make lint            # Code quality checks
make fmt             # Format code

# 🚀 Deployment
make staging         # Deploy to staging
make production      # Deploy to production (with confirmation)
make health          # Check all environment health

# 🔧 Maintenance
make clean           # Clean up resources
make validate        # Validate repository setup
make info            # Show project information
```

### Configuration Examples

See the `configs/` directory for configuration examples:
- **`configs/examples/simple.yaml`** - Basic single-tenant setup
- **`configs/examples/multi-tenant.yaml`** - Multi-tenant configuration
- **`configs/environments/staging.yaml`** - Staging environment
- **`configs/environments/production-high-load.yaml`** - Production setup

## 📁 Project Structure

```
keystone-gateway/
├── 📂 cmd/                     # Application entry points
├── 📂 internal/                # Private Go packages
│   ├── config/                 # Configuration management  
│   ├── lua/                    # Lua engine integration
│   └── routing/                # HTTP routing and load balancing
├── 📂 configs/                 # Configuration files
│   ├── environments/           # Environment-specific configs
│   └── examples/               # Example configurations
├── 📂 scripts/                 # Scripts and tools
│   ├── lua/                    # Lua routing scripts
│   └── tools/                  # Development tools
├── 📂 tests/                   # Comprehensive test suite
│   ├── unit/                   # Unit tests
│   ├── integration/            # Integration tests
│   └── e2e/                    # End-to-end tests
├── 📂 deployments/             # Deployment configurations
│   └── docker/                 # Docker Compose files
├── 📂 docs/                    # Documentation
├── 🐳 docker-compose.production.yml  # Production deployment
├── 🔨 Makefile                 # Unified build system
└── 📋 README.md                # This file
```

## Configuration

### Basic Structure

```yaml
# Optional admin API configuration
admin_base_path: "/admin"    # Default: "/admin"

# Lua routing configuration (required)
lua_routing:
  enabled: true              # Must be true
  scripts_dir: "./scripts"   # Default: "./scripts"

# Tenant definitions
tenants:
  - name: "tenant-name"      # Required: unique identifier
    
    # Routing strategy (choose one):
    domains: ["api.example.com"]           # Host-based routing
    # OR
    path_prefix: "/api/"                   # Path-based routing
    
    lua_routes: "auth-routes.lua"          # Lua script file
    health_interval: 30                    # Health check interval (seconds)
    
    # Backend services
    services:
      - name: "backend1"
        url: "http://localhost:3001"
        health: "/health"                  # Health check endpoint
```

### Configuration Examples

See [configs/examples/](configs/examples/) for complete configuration examples:
- `simple.yaml` - Basic single-tenant setup
- `multi-tenant.yaml` - Multi-tenant with different routing strategies
- `production.yaml` - Production-ready configuration

## Lua Scripting

Keystone Gateway's power comes from embedded Lua scripting for dynamic routing:

### Core Functions

```lua
-- Route registration
chi_route("GET", "/api/users", handler_function)

-- Middleware
chi_middleware("/api/*", auth_middleware)

-- Route groups
chi_group("/api/v1", function()
    chi_route("GET", "/users", users_handler)
    chi_route("POST", "/users", create_user_handler)
end)
```

### Examples

See [scripts/examples/](scripts/examples/) for complete examples:
- `auth-routes.lua` - Authentication and authorization patterns
- `ab-testing-routes.lua` - A/B testing implementation
- `canary-routes.lua` - Canary deployment strategies

For detailed Lua API documentation, see [docs/lua-scripting.md](docs/lua-scripting.md).

## Admin API

Monitor your gateway using the admin endpoints:

```bash
# Gateway health
curl http://localhost:8080/admin/health

# Tenant information
curl http://localhost:8080/admin/tenants

# Individual tenant health
curl http://localhost:8080/admin/tenants/api/health
```

## Development

### Local Setup

```bash
# Clone the repository
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway

# Install dependencies
go mod download

# Run with example configuration
go run ./cmd/ -config configs/examples/simple.yaml
```

### Testing Your Changes

```bash
# Run tests (when implemented)
go test ./...

# Format code
go fmt ./...

# Lint code
go vet ./...
```

For development guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Documentation

- [Getting Started Guide](docs/getting-started.md) - Step-by-step tutorial
- [Configuration Reference](docs/configuration.md) - Complete configuration options
- [Lua Scripting Guide](docs/lua-scripting.md) - Comprehensive Lua API documentation
- [Examples](configs/examples/) - Configuration examples for different scenarios

## 🏗️ Architecture

Keystone Gateway uses a **clean, layered architecture** with embedded Lua scripting:

### Core Components
- **🌐 HTTP Layer**: Chi router for high-performance request handling
- **🚀 Application Layer**: Gateway logic with embedded Lua engine  
- **🏢 Business Logic**: Multi-tenant routing and load balancing
- **🐳 Deployment Layer**: Docker-first with Makefile automation

### Key Features
- **Thread-safe Lua state pools** for concurrent safety
- **Zero-downtime deployments** with health checking
- **Environment-based configuration** (dev, staging, production)
- **Comprehensive testing** (unit, integration, e2e, load)
- **Simple Docker deployment** focused purely on the gateway

## Performance Optimizations

Keystone Gateway includes several built-in performance optimizations:

- **HTTP/2 Support**: Automatic HTTP/2 multiplexing for supported backends
- **Connection Pooling**: Optimized connection reuse with configurable pool sizes
- **Garbage Collection Tuning**: GOGC=200 for reduced GC overhead in high-throughput scenarios
- **Response Compression**: Configurable gzip compression for text-based content
- **Request Caching**: Proxy object caching to eliminate per-request allocations

These optimizations provide excellent performance for lightweight gateway use cases while maintaining simplicity and extensibility.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:
- Development setup
- Code standards
- Testing requirements
- Pull request process

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: Check the [docs/](docs/) directory
- **Examples**: See [configs/examples/](configs/examples/) and [scripts/examples/](scripts/examples/)
- **Issues**: Report bugs and request features on GitHub Issues