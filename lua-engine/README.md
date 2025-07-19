# Keystone Lua Engine

Lua scripting engine for Keystone Gateway - enables advanced routing logic through Lua scripts.

## Quick Start

### Build & Run

```bash
# Build the engine
cd lua-engine
go mod tidy
go build -o lua-engine

# Run with default settings
./lua-engine

# Or with custom settings
./lua-engine -addr :8081 -scripts ./scripts
```

### Docker

```bash
# Build Docker image
docker build -t keystone-lua-engine .

# Run container
docker run -p 8081:8081 -v $(pwd)/scripts:/app/scripts keystone-lua-engine
```

### Docker Compose (Full Stack)

```bash
# Run Gateway + Lua Engine + Test Backends
docker-compose -f docker-compose.lua.yml up
```

## API Endpoints

### Route Request
```bash
POST /route/{tenant}
Content-Type: application/json

{
  "method": "GET",
  "path": "/api/users",
  "host": "api.example.com",
  "headers": {
    "X-Canary": "true",
    "User-Agent": "Mozilla/5.0"
  },
  "backends": [
    {
      "name": "api-stable",
      "url": "http://backend1:8080",
      "health": true
    },
    {
      "name": "api-canary", 
      "url": "http://backend2:8080",
      "health": true
    }
  ]
}
```

### Health Check
```bash
GET /health
```

### Reload Scripts
```bash
POST /reload
```

## Lua Script Format

Scripts must implement the `on_route_request` function:

```lua
function on_route_request(request, backends)
    -- Your routing logic here
    
    return {
        selected_backend = "api-stable",
        modified_headers = {
            ["X-Routed-By"] = "lua-engine"
        },
        modified_path = "/v2/api/users",  -- optional
        reject = false,                    -- optional
        reject_reason = "reason"           -- optional
    }
end
```

## Available Scripts

- **canary.lua**: Canary deployment routing
- **blue-green.lua**: Blue/green deployment routing  
- **ab-testing.lua**: A/B testing with user segmentation

## Security Features

- **Isolated Execution**: Each script runs in a separate Lua state
- **Memory Limits**: 10MB memory limit per script execution
- **Timeout Protection**: 5-second execution timeout
- **Hardened Container**: Runs as non-root user in scratch container
- **Limited API**: Only safe Lua functions are exposed

## Integration with Gateway

Add to your tenant configuration:

```yaml
tenants:
  - name: "my-api"
    domains: ["api.example.com"]
    lua_script: "canary"  # references canary.lua
    lua_engine_url: "http://lua-engine:8081"
    services:
      - name: "api-stable"
        url: "http://api-v1:8080"
      - name: "api-canary"
        url: "http://api-v2:8080"
```

## Performance

- **Execution Time**: <5ms per request
- **Memory Usage**: ~10MB per engine instance
- **Throughput**: 1000+ req/sec
- **Startup Time**: <1 second

## Monitoring

Check engine status:
```bash
curl http://localhost:8081/health
```

View logs:
```bash
docker logs keystone-lua-engine
```
