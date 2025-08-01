# Development

## Setup

```bash
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway
make dev
```

Requirements: Go 1.22+, Docker, Make

## Commands

```bash
make dev        # Start development 
make test       # Run all tests
make lint       # Code quality
make clean      # Cleanup

# Testing specific parts
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/e2e/...
```

## Project Structure

```
cmd/           # Main application
internal/      # Private Go packages
  config/      # Configuration management
  lua/         # Lua engine integration  
  routing/     # HTTP routing & load balancing
configs/       # YAML configurations
scripts/lua/   # Lua routing scripts
tests/         # Test suites
```

## Making Changes

1. Create feature branch
2. Make changes
3. Run `make test`
4. Submit PR

## Adding Routes

1. Edit Lua script in `scripts/lua/`
2. Update config to reference script
3. Restart gateway: `make dev`

## Adding Config Options

1. Update structs in `internal/config/`
2. Add validation
3. Update example configs
4. Add tests

## Debugging

```bash
# Verbose logging
DEBUG=true ./keystone-gateway -config config.yaml

# Lua script debugging  
log("Debug: " .. request.path)
```

## Testing Your Changes

```bash
# Start gateway
make dev

# Test endpoints
curl http://localhost:8080/admin/health
curl http://localhost:8080/your-route

# Stop
make clean
```

That's it. Keep it simple.