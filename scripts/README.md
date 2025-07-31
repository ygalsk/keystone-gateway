# Scripts Directory

This directory contains scripts and tools for Keystone Gateway development and operations.

## 📁 Structure

```
scripts/
├── lua/               # Lua routing scripts
│   └── api-routes.lua # Main API routes configuration
├── tools/             # Development and operational tools
│   └── validate-branching-strategy.sh # Repository validation
└── README.md          # This file
```

## 🐢 Lua Scripts (`scripts/lua/`)

Lua scripts define the routing logic and middleware for the gateway.

### `api-routes.lua`
Main routing configuration that defines:
- API endpoints and routes
- Middleware functions
- Request/response handling
- Authentication logic

**Usage in configuration:**
```yaml
lua_routing:
  enabled: true
  scripts_dir: "./scripts/lua"

tenants:
  - name: "api"
    lua_routes: "api-routes.lua"  # References scripts/lua/api-routes.lua
```

## 🔧 Tools (`scripts/tools/`)

Development and operational tools for the project.

### `validate-branching-strategy.sh`
Validates the repository setup and branching strategy implementation.

**Usage:**
```bash
# Via Makefile (recommended)
make validate

# Direct execution
./scripts/tools/validate-branching-strategy.sh
```

**Checks:**
- Branch structure (main, staging)
- Configuration files
- Deployment infrastructure
- Documentation completeness
- CI/CD pipeline setup

## 🚀 Adding New Scripts

### Lua Scripts
1. Create your script in `scripts/lua/`
2. Reference it in your tenant configuration:
   ```yaml
   tenants:
     - name: "my-tenant"
       lua_routes: "my-script.lua"
   ```

### Tools
1. Create executable scripts in `scripts/tools/`
2. Add appropriate shebang (`#!/bin/bash`)
3. Make executable: `chmod +x scripts/tools/my-tool.sh`
4. Consider adding Makefile targets for common tools

## 📋 Best Practices

### Lua Scripts
- Use descriptive function names
- Comment complex logic
- Follow consistent indentation (4 spaces)
- Test scripts locally before deployment
- Use local variables when possible

### Shell Scripts
- Always use `set -e` for error handling
- Provide descriptive error messages
- Use colors for output readability
- Include usage help text
- Validate input parameters

## 🔗 Related Documentation

- [Lua Scripting Guide](../docs/lua-scripting.md)
- [Configuration Reference](../docs/configuration.md)
- [Branching Strategy](../docs/branching-strategy.md)