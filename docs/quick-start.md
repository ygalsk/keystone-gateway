# Quick Start

## Install & Run

```bash
# Clone and build
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway
make dev

# Gateway runs on :8080
curl http://localhost:8080/admin/health
```

## Basic Config

Create `config.yaml`:

```yaml
tenants:
  - name: "api"
    domains: ["localhost"]
    lua_routes: "routes.lua"
    services:
      - name: "backend"
        url: "http://localhost:3001"
```

Create `scripts/routes.lua`:

```lua
chi_route("GET", "/hello", function(request, response)
    response:write("Hello World")
end)
```

Start: `./keystone-gateway -config config.yaml`

Done.
