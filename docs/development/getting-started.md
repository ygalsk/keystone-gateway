# Getting Started with Keystone Gateway

This guide will walk you through setting up Keystone Gateway from scratch, creating your first routing configuration, and testing your setup.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Your First Gateway](#your-first-gateway)
- [Testing Your Setup](#testing-your-setup)
- [Adding More Routes](#adding-more-routes)
- [Multi-Tenant Setup](#multi-tenant-setup)
- [Production Considerations](#production-considerations)
- [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have:

- **Go 1.21 or later** installed
- **Basic knowledge of YAML** for configuration
- **Basic knowledge of Lua** for routing scripts (optional but helpful)
- A backend service to route to (we'll show you how to create a simple one)

### Verify Go Installation

```bash
go version
# Should output: go version go1.21.x ...
```

## Installation

### Option 1: Install from Source

```bash
# Clone the repository
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway

# Build the gateway
go build -o keystone-gateway ./cmd/

# Verify installation
./keystone-gateway --help
```

### Option 2: Direct Install (when available)

```bash
go install github.com/your-org/keystone-gateway/cmd@latest
```

## Your First Gateway

Let's create a simple gateway that routes requests to a local backend service.

### Step 1: Create a Simple Backend Service

First, let's create a simple backend service to route to:

```bash
# Create a simple HTTP server (save as backend.go)
cat > backend.go << 'EOF'
package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"status": "healthy", "service": "backend"}`)
    })
    
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"users": [{"id": 1, "name": "John Doe"}]}`)
    })
    
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from backend! Path: %s", r.URL.Path)
    })
    
    log.Println("Backend server starting on :3001")
    log.Fatal(http.ListenAndServe(":3001", nil))
}
EOF

# Run the backend service
go run backend.go &
BACKEND_PID=$!
echo "Backend running with PID: $BACKEND_PID"
```

### Step 2: Create Your Gateway Configuration

Create a configuration file for your gateway:

```bash
# Create config.yaml
cat > config.yaml << 'EOF'
admin_base_path: "/admin"

lua_routing:
  enabled: true
  scripts_dir: "./scripts"

tenants:
  - name: "my-api"
    domains: ["localhost"]
    lua_routes: "my-first-routes.lua"
    health_interval: 30
    services:
      - name: "backend"
        url: "http://localhost:3001"
        health: "/health"
EOF
```

### Step 3: Create Your First Lua Routing Script

Create the scripts directory and your first routing script:

```bash
# Create scripts directory
mkdir -p scripts

# Create your first Lua routing script
cat > scripts/my-first-routes.lua << 'EOF'
-- My First Routes
-- Simple routing script to get started

-- Health check endpoint
chi_route("GET", "/gateway/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "keystone-gateway"}')
end)

-- Simple API route that forwards to backend
chi_route("GET", "/api/users", function(request, response)
    -- This will be handled by the load balancer
    -- and forwarded to the backend service
    response:header("X-Gateway", "Keystone")
end)

-- Welcome route
chi_route("GET", "/", function(request, response)
    response:header("Content-Type", "text/html")
    response:write('<h1>Welcome to Keystone Gateway!</h1><p>Your gateway is working!</p>')
end)

-- Add logging middleware for all requests
chi_middleware("/*", function(request, response, next)
    log("Request: " .. request.method .. " " .. request.path)
    response:header("X-Powered-By", "Keystone Gateway")
    next()
end)
EOF
```

### Step 4: Start Your Gateway

```bash
# Start the gateway
./keystone-gateway -config config.yaml
```

You should see output similar to:
```
2024/01/20 10:00:00 Starting Keystone Gateway...
2024/01/20 10:00:00 Loading configuration from config.yaml
2024/01/20 10:00:00 Tenant 'my-api' loaded with 1 services
2024/01/20 10:00:00 Gateway listening on :8080
```

## Testing Your Setup

Now let's test that everything is working correctly.

### Test the Gateway Endpoints

Open a new terminal and run these tests:

```bash
# Test the welcome route
curl http://localhost:8080/
# Expected: <h1>Welcome to Keystone Gateway!</h1>...

# Test the gateway health check
curl http://localhost:8080/gateway/health
# Expected: {"status": "healthy", "service": "keystone-gateway"}

# Test the API route (forwarded to backend)
curl http://localhost:8080/api/users
# Expected: {"users": [{"id": 1, "name": "John Doe"}]}

# Test the admin endpoints
curl http://localhost:8080/admin/health
# Expected: Gateway health information

curl http://localhost:8080/admin/tenants
# Expected: List of configured tenants
```

### Verify Backend Health

```bash
# Check that the gateway can reach your backend
curl http://localhost:8080/admin/tenants/my-api/health
# Expected: Health status of the my-api tenant
```

If all tests pass, congratulations! Your gateway is working correctly.

## Adding More Routes

Let's expand your routing script with more functionality:

```bash
# Update scripts/my-first-routes.lua
cat > scripts/my-first-routes.lua << 'EOF'
-- Enhanced routing script with more features

-- Health check endpoint
chi_route("GET", "/gateway/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "keystone-gateway", "timestamp": "' .. os.date() .. '"}')
end)

-- API routes group
chi_group("/api", function()
    -- Middleware for all API routes
    chi_middleware("/*", function(request, response, next)
        response:header("Content-Type", "application/json")
        response:header("API-Version", "v1")
        next()
    end)
    
    -- Users endpoint (forwards to backend)
    chi_route("GET", "/users", function(request, response)
        -- Backend will handle this
    end)
    
    -- User by ID endpoint
    chi_route("GET", "/users/{id}", function(request, response)
        local user_id = chi_param(request, "id")
        log("Fetching user: " .. user_id)
        -- Forward to backend with parameter
    end)
    
    -- Echo endpoint for testing
    chi_route("POST", "/echo", function(request, response)
        local body = request.body or ""
        response:write('{"received": "' .. body .. '", "timestamp": "' .. os.date() .. '"}')
    end)
end)

-- Static content route
chi_route("GET", "/", function(request, response)
    response:header("Content-Type", "text/html")
    response:write([[
        <h1>Welcome to Keystone Gateway!</h1>
        <p>Your gateway is working!</p>
        <h2>Available Endpoints:</h2>
        <ul>
            <li><a href="/gateway/health">Gateway Health</a></li>
            <li><a href="/api/users">API Users</a></li>
            <li><a href="/admin/health">Admin Health</a></li>
        </ul>
    ]])
end)

-- Global middleware for logging and headers
chi_middleware("/*", function(request, response, next)
    log("Request: " .. request.method .. " " .. request.path)
    response:header("X-Powered-By", "Keystone Gateway")
    next()
end)
EOF
```

Restart your gateway to load the new routes:

```bash
# Stop the gateway (Ctrl+C), then restart
./keystone-gateway -config config.yaml
```

Test the new routes:

```bash
# Test the enhanced welcome page
curl http://localhost:8080/

# Test the echo endpoint
curl -X POST http://localhost:8080/api/echo -d '{"test": "data"}'

# Test user by ID
curl http://localhost:8080/api/users/123
```

## Multi-Tenant Setup

Now let's create a more complex setup with multiple tenants:

```bash
# Create a multi-tenant configuration
cat > multi-tenant-config.yaml << 'EOF'
admin_base_path: "/admin"

lua_routing:
  enabled: true
  scripts_dir: "./scripts"

tenants:
  # API tenant - host-based routing
  - name: "api"
    domains: ["localhost"]
    lua_routes: "api-routes.lua"
    health_interval: 30
    services:
      - name: "api-backend"
        url: "http://localhost:3001"
        health: "/health"

  # App tenant - path-based routing
  - name: "app"
    path_prefix: "/app/"
    lua_routes: "app-routes.lua"
    health_interval: 30
    services:
      - name: "app-backend"
        url: "http://localhost:3001"
        health: "/health"
EOF
```

Create routing scripts for each tenant:

```bash
# API tenant routes
cat > scripts/api-routes.lua << 'EOF'
-- API tenant routes

chi_group("/api", function()
    chi_route("GET", "/users", function(request, response)
        response:header("Content-Type", "application/json")
        -- Forwarded to backend
    end)
    
    chi_route("GET", "/health", function(request, response)
        response:header("Content-Type", "application/json")
        response:write('{"service": "api", "status": "healthy"}')
    end)
end)

chi_middleware("/api/*", function(request, response, next)
    response:header("X-Tenant", "API")
    next()
end)
EOF

# App tenant routes
cat > scripts/app-routes.lua << 'EOF'
-- App tenant routes

chi_route("GET", "/", function(request, response)
    response:header("Content-Type", "text/html")
    response:write('<h1>App Tenant</h1><p>This is the app tenant!</p>')
end)

chi_route("GET", "/dashboard", function(request, response)
    response:header("Content-Type", "text/html")
    response:write('<h1>Dashboard</h1><p>Welcome to your dashboard!</p>')
end)

chi_middleware("/*", function(request, response, next)
    response:header("X-Tenant", "App")
    next()
end)
EOF
```

Test the multi-tenant setup:

```bash
# Start with multi-tenant config
./keystone-gateway -config multi-tenant-config.yaml

# Test API tenant (host-based)
curl -H "Host: localhost" http://localhost:8080/api/users

# Test App tenant (path-based)
curl http://localhost:8080/app/
curl http://localhost:8080/app/dashboard
```

## Production Considerations

When moving to production, consider these important factors:

### Security Configuration

```lua
-- Add authentication middleware
chi_middleware("/api/*", function(request, response, next)
    local auth_header = request.headers["Authorization"]
    if not auth_header then
        response:status(401)
        response:write('{"error": "Authorization required"}')
        return
    end
    
    -- Validate token here
    if not validate_token(auth_header) then
        response:status(401)
        response:write('{"error": "Invalid token"}')
        return
    end
    
    next()
end)
```

### Health Check Configuration

```yaml
tenants:
  - name: "prod-api"
    domains: ["api.production.com"]
    health_interval: 15  # More frequent checks in production
    services:
      - name: "api-1"
        url: "http://api-1.internal:3001"
        health: "/health"
      - name: "api-2"
        url: "http://api-2.internal:3001"
        health: "/health"
```

### Logging and Monitoring

```lua
-- Enhanced logging
chi_middleware("/*", function(request, response, next)
    local start_time = os.clock()
    
    -- Add request ID
    local request_id = generate_request_id()
    response:header("X-Request-ID", request_id)
    
    next()
    
    -- Log timing
    local duration = os.clock() - start_time
    log("Request " .. request_id .. ": " .. request.method .. " " .. request.path .. " - " .. duration .. "s")
end)
```

### Environment-Specific Configuration

```bash
# Production environment variables
export GATEWAY_PORT=80
export GATEWAY_CONFIG=/etc/keystone/production.yaml
export LUA_SCRIPTS_DIR=/opt/keystone/scripts
export LOG_LEVEL=info

./keystone-gateway
```

## Next Steps

Now that you have a working Keystone Gateway setup, here are some next steps:

### Learn More About Lua Scripting

- Read the [Lua Scripting Guide](lua-scripting.md) for advanced patterns
- Explore the [example scripts](../scripts/examples/) in the repository
- Learn about middleware patterns and authentication

### Explore Advanced Configuration

- Read the [Configuration Reference](configuration.md) for all available options
- Set up different routing strategies (host-based, path-based, hybrid)
- Configure multiple backend services with load balancing

### Production Deployment

- Set up proper logging and monitoring
- Configure SSL/TLS termination
- Set up health checks and alerting
- Consider container deployment with Docker

### Development Workflow

- Read [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines
- Set up automated testing for your Lua scripts
- Use configuration templates for different environments

## Troubleshooting

### Gateway Won't Start

1. **Check configuration syntax:**
   ```bash
   ./keystone-gateway -config config.yaml --validate
   ```

2. **Verify Lua scripts exist:**
   ```bash
   ls -la scripts/
   ```

3. **Check logs for specific errors**

### Routes Not Working

1. **Verify tenant configuration matches request:**
   - Check domain names for host-based routing
   - Check path prefixes for path-based routing

2. **Test admin endpoints:**
   ```bash
   curl http://localhost:8080/admin/tenants
   ```

3. **Check Lua script syntax**

### Backend Connection Issues

1. **Test backend directly:**
   ```bash
   curl http://localhost:3001/health
   ```

2. **Check tenant health:**
   ```bash
   curl http://localhost:8080/admin/tenants/my-api/health
   ```

3. **Verify service URLs in configuration**

For more help, check the project documentation or create an issue on GitHub.

## Cleanup

When you're done experimenting, clean up the test processes:

```bash
# Stop the backend service
kill $BACKEND_PID

# Remove test files (optional)
rm backend.go config.yaml multi-tenant-config.yaml
rm -rf scripts/
```

Congratulations! You now have a working knowledge of Keystone Gateway and can start building your own routing configurations.