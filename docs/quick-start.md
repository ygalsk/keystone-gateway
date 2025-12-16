# Quick Start Guide

Get Keystone Gateway running in 5 minutes.

## Installation

```bash
git clone https://github.com/ygalsk/keystone-gateway.git
cd keystone-gateway
make build
```

## Minimal Configuration

Create `config.yaml`:

```yaml
lua_routing:
  enabled: true
  scripts_dir: "./examples/scripts"
  global_scripts:
    - "init"
    - "handlers"

tenants:
  - name: "api"
    path_prefix: "/api"
    routes:
      - method: "GET"
        pattern: "/hello"
        handler: "hello_handler"
```

## Create Your First Handler

Create `examples/scripts/handlers.lua`:

```lua
function hello_handler(req)
    return {
        status = 200,
        body = "Hello from Keystone!",
        headers = {["Content-Type"] = "text/plain"}
    }
end
```

## Run the Gateway

```bash
./keystone-gateway -config config.yaml
```

Test it:

```bash
curl http://localhost:8080/api/hello
# Output: Hello from Keystone!
```

## What Just Happened?

1. **Configuration loaded**: Gateway read `config.yaml`
2. **Lua scripts executed**: `init.lua` and `handlers.lua` loaded into state pool
3. **Routes registered**: `/api/hello` → `hello_handler` function
4. **Request processed**:
   - HTTP GET → Chi router → Lua handler
   - Handler returned response table
   - Gateway wrote HTTP response

## Next Steps

- [Configuration Guide](configuration.md) - Full YAML reference
- [Lua API Reference](lua-api.md) - Handler and middleware interfaces
- [Examples](examples.md) - Working code samples
