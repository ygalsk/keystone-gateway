# Keystone Gateway

A high-performance, programmable reverse proxy and API gateway written in Go with embedded Lua scripting for dynamic routing in multi-tenant environments.

## Features

- **Multi-tenant routing**: Host-based, path-based, and hybrid routing strategies
- **Embedded Lua scripting**: Dynamic route definition and middleware without recompilation
- **Load balancing**: Round-robin load balancing with health checking
- **Admin API**: Health monitoring and tenant management endpoints
- **Thread-safe architecture**: Lua state pools and atomic operations for concurrent safety

## Quick Start

### Prerequisites

- Go 1.21 or later
- Basic knowledge of YAML configuration

### Installation

**From source:**
```bash
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway
go build -o keystone-gateway ./cmd/
```

**Or install directly:**
```bash
go install github.com/your-org/keystone-gateway/cmd@latest
```

### Basic Usage

1. **Create a basic configuration** (`config.yaml`):
```yaml
admin_base_path: "/admin"
lua_routing:
  enabled: true
  scripts_dir: "./scripts"

tenants:
  - name: "api"
    domains: ["localhost"]
    lua_routes: "basic-routes.lua"
    services:
      - name: "backend"
        url: "http://localhost:3001"
        health: "/health"
```

2. **Create a basic Lua routing script** (`scripts/basic-routes.lua`):
```lua
-- Register a simple route
chi_route("GET", "/api/hello", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Hello from Keystone Gateway!"}')
end)

-- Add middleware
chi_middleware("/api/*", function(request, response, next)
    response:header("X-Gateway", "Keystone")
    next()
end)
```

3. **Run the gateway**:
```bash
./keystone-gateway -config config.yaml
```

4. **Test your setup**:
```bash
curl http://localhost:8080/api/hello
# {"message": "Hello from Keystone Gateway!"}
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

## Architecture

Keystone Gateway uses a layered architecture with embedded Lua scripting:

- **HTTP Layer**: Chi router for high-performance request handling
- **Application Layer**: Gateway logic with embedded Lua engine
- **Business Logic**: Multi-tenant routing and load balancing

Key components interact through thread-safe Lua state pools, ensuring concurrent safety while maintaining the flexibility of dynamic scripting.

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