# Examples

## Single API Backend

```yaml
# config.yaml
tenants:
  - name: "api"
    domains: ["api.example.com"]
    lua_routes: "api"
    services:
      - name: "backend"
        url: "http://api-server:3001"
```

```lua
-- scripts/api.lua
chi_route("GET", "/users", function(request, response)
    -- Forwards to backend automatically
end)
```

## Microservices by Path

```yaml
tenants:
  - name: "users"
    path_prefix: "/users/"
    lua_routes: "users"
    services:
      - name: "user-service"
        url: "http://users:3000"
        
  - name: "orders" 
    path_prefix: "/orders/"
    lua_routes: "orders"
    services:
      - name: "order-service"
        url: "http://orders:3000"
```

## Load Balancing

```yaml
tenants:
  - name: "api"
    domains: ["api.example.com"]
    lua_routes: "api"
    services:
      - name: "api-1"
        url: "http://api-1:3001"
      - name: "api-2"  
        url: "http://api-2:3001"
      - name: "api-3"
        url: "http://api-3:3001"
```

Round-robin happens automatically.

## Authentication Middleware

```lua
-- scripts/auth.lua
chi_middleware("/api/*", function(request, response, next)
    local token = request.headers["Authorization"]
    if not token or not validate_token(token) then
        response:status(401)
        response:write('{"error": "Unauthorized"}')
        return
    end
    next()
end)

function validate_token(token)
    -- Your validation logic
    return token == "Bearer valid-token"
end
```

## Multi-Environment

```yaml
# Production
tenants:
  - name: "prod"
    domains: ["api.company.com"]
    lua_routes: "prod"
    health_interval: 15
    services:
      - name: "prod-api"
        url: "https://prod-api.internal:443"

# Staging  
  - name: "staging"
    domains: ["staging-api.company.com"] 
    lua_routes: "staging"
    services:
      - name: "staging-api"
        url: "http://staging-api.internal:3001"
```

## Docker Deployment

```yaml
# docker-compose.yml
version: '3.8'
services:
  gateway:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs
      - ./scripts:/app/scripts
    environment:
      - GATEWAY_CONFIG=/app/configs/production.yaml
```

```bash
docker-compose up -d
```

## Development

```bash
# Start everything
make dev

# Check health
curl http://localhost:8080/admin/health

# Test route
curl http://localhost:8080/your-route
```

## Production Checklist

1. Use `configs/environments/production-high-load.yaml`
2. Set health check intervals (15-30s)  
3. Configure TLS termination (reverse proxy)
4. Monitor `/admin/health` endpoint
5. Use multiple backend services for HA

That's it. Keep it simple.