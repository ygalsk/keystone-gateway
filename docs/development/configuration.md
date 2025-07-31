# Configuration Reference

This document provides a complete reference for configuring Keystone Gateway using YAML configuration files.

## Table of Contents

- [Configuration File Structure](#configuration-file-structure)
- [Global Settings](#global-settings)
- [Compression Configuration](#compression-configuration)
- [Lua Routing Configuration](#lua-routing-configuration)
- [Tenant Configuration](#tenant-configuration)
- [Service Configuration](#service-configuration)
- [Routing Strategies](#routing-strategies)
- [Validation Rules](#validation-rules)
- [Environment Variables](#environment-variables)
- [Examples](#examples)

## Configuration File Structure

A Keystone Gateway configuration file has the following top-level structure:

```yaml
# Global gateway settings
admin_base_path: "/admin"

# HTTP response compression
compression:
  enabled: true
  level: 5
  content_types: ["application/json", "text/html"]

# Lua routing configuration
lua_routing:
  enabled: true
  scripts_dir: "./scripts"

# Tenant definitions
tenants:
  - name: "tenant1"
    # Tenant configuration...
  - name: "tenant2"
    # Tenant configuration...
```

## Global Settings

### `admin_base_path`

**Type:** `string`  
**Default:** `"/admin"`  
**Optional:** Yes

Base path for admin API endpoints. Admin endpoints will be available under this path.

```yaml
admin_base_path: "/admin"
```

**Admin endpoints:**
- `GET {admin_base_path}/health` - Gateway health status
- `GET {admin_base_path}/tenants` - List all tenants
- `GET {admin_base_path}/tenants/{name}/health` - Individual tenant health

**Examples:**
```yaml
# Default admin path
admin_base_path: "/admin"
# Access: http://localhost:8080/admin/health

# Custom admin path
admin_base_path: "/management"
# Access: http://localhost:8080/management/health

# Root level admin (not recommended)
admin_base_path: "/"
# Access: http://localhost:8080/health
```

## Compression Configuration

### `compression`

**Type:** `object`  
**Optional:** Yes

Configuration for HTTP response compression to improve performance and reduce bandwidth usage.

#### `compression.enabled`

**Type:** `boolean`  
**Default:** `true`  
**Optional:** Yes

Enables or disables HTTP response compression globally.

```yaml
compression:
  enabled: true   # Enable compression (default)
  enabled: false  # Disable compression completely
```

#### `compression.level`

**Type:** `integer`  
**Default:** `5`  
**Range:** `1-9`  
**Optional:** Yes

Compression level for gzip encoding:
- `1`: Fastest compression, largest file size
- `5`: Balanced compression and speed (recommended)
- `9`: Best compression, slowest speed

```yaml
compression:
  level: 1  # Fast compression for high-traffic scenarios
  level: 5  # Balanced (default)
  level: 9  # Maximum compression for bandwidth-limited environments
```

#### `compression.content_types`

**Type:** `array[string]`  
**Default:** `["text/html", "text/css", "text/javascript", "application/json", "application/xml", "text/plain"]`  
**Optional:** Yes

MIME types that should be compressed. Only responses with these content types will be compressed.

```yaml
compression:
  content_types:
    - "application/json"      # API responses
    - "text/html"            # HTML pages
    - "text/css"             # Stylesheets
    - "text/javascript"      # JavaScript files
    - "application/xml"      # XML documents
    - "text/plain"           # Plain text
```

**Note:** Binary content types (images, videos, archives) are typically not included as they don't benefit from compression or are already compressed.

### Compression Examples

#### Default Configuration

When no compression configuration is specified, the gateway uses these defaults:

```yaml
# Implicit default configuration (no need to specify)
compression:
  enabled: true
  level: 5
  content_types:
    - "text/html"
    - "text/css" 
    - "text/javascript"
    - "application/json"
    - "application/xml"
    - "text/plain"
```

#### API-Only Compression

For API gateways handling primarily JSON responses:

```yaml
compression:
  enabled: true
  level: 6
  content_types:
    - "application/json"
    - "application/xml"
```

#### High-Performance Configuration

For high-traffic scenarios where CPU usage should be minimized:

```yaml
compression:
  enabled: true
  level: 1  # Fastest compression
  content_types:
    - "application/json"  # Only compress JSON APIs
```

#### Maximum Bandwidth Savings

For environments with limited bandwidth:

```yaml
compression:
  enabled: true
  level: 9  # Maximum compression
  content_types:
    - "text/html"
    - "text/css"
    - "text/javascript"
    - "application/json"
    - "application/xml"
    - "text/plain"
    - "application/javascript"
    - "text/xml"
```

#### Disabled Compression

To disable compression entirely:

```yaml
compression:
  enabled: false
```

### Performance Considerations

**Compression Level Trade-offs:**
- **Level 1-3**: Fast compression, lower CPU usage, moderate file size reduction
- **Level 4-6**: Balanced compression, reasonable CPU usage, good file size reduction
- **Level 7-9**: Slower compression, higher CPU usage, maximum file size reduction

**Content Type Recommendations:**
- **Always compress:** JSON, XML, HTML, CSS, JavaScript, plain text
- **Never compress:** Images (JPEG, PNG, GIF), videos (MP4, WebM), archives (ZIP, GZIP)
- **Consider carefully:** Large binary APIs, already-compressed formats

**Bandwidth Savings:**
- JSON responses: 60-80% size reduction
- HTML pages: 70-85% size reduction
- CSS/JavaScript: 70-90% size reduction

## Lua Routing Configuration

### `lua_routing`

**Type:** `object`  
**Required:** Yes

Configuration for the embedded Lua scripting engine.

#### `lua_routing.enabled`

**Type:** `boolean`  
**Required:** Yes  
**Must be:** `true`

Enables Lua routing. Currently must be set to `true` for the gateway to function.

#### `lua_routing.scripts_dir`

**Type:** `string`  
**Default:** `"./scripts"`  
**Optional:** Yes

Directory containing Lua routing scripts. Can be relative or absolute path.

```yaml
lua_routing:
  enabled: true
  scripts_dir: "./scripts"              # Relative path
  # scripts_dir: "/opt/gateway/scripts" # Absolute path
```

## Tenant Configuration

### `tenants`

**Type:** `array`  
**Required:** Yes

Array of tenant configurations. Each tenant represents a routing target with its own configuration.

### Tenant Object

Each tenant must have the following structure:

#### `name`

**Type:** `string`  
**Required:** Yes

Unique identifier for the tenant. Used in admin endpoints and logging.

```yaml
tenants:
  - name: "api-service"
  - name: "web-app"
  - name: "legacy-system"
```

#### Routing Configuration

Each tenant must specify **exactly one** routing strategy:

##### `domains` (Host-based routing)

**Type:** `array[string]`  
**Required:** No (but either `domains` or `path_prefix` required)

Array of domain names for host-based routing.

**Validation:**
- Each domain must contain at least one dot (`.`)
- No spaces allowed in domain names
- Case-insensitive matching

```yaml
tenants:
  - name: "api"
    domains: ["api.example.com", "api.mysite.org"]
```

##### `path_prefix` (Path-based routing)

**Type:** `string`  
**Required:** No (but either `domains` or `path_prefix` required)

URL path prefix for path-based routing.

**Validation:**
- Must start with `/`
- Must end with `/`
- Cannot be just `/`

```yaml
tenants:
  - name: "app"
    path_prefix: "/app/"
  - name: "api"
    path_prefix: "/api/v1/"
```

#### `lua_routes`

**Type:** `string`  
**Required:** Yes

Filename of the Lua script to load for this tenant. The file must exist in the `scripts_dir`.

```yaml
tenants:
  - name: "api"
    lua_routes: "api-routes.lua"
  - name: "auth"
    lua_routes: "auth-routes.lua"
```

#### `health_interval`

**Type:** `integer`  
**Default:** `30`  
**Optional:** Yes  
**Unit:** seconds

Interval between health checks for backend services.

```yaml
tenants:
  - name: "api"
    health_interval: 30   # Check every 30 seconds
    health_interval: 60   # Check every minute
    health_interval: 10   # Check every 10 seconds (frequent)
```

#### `services`

**Type:** `array`  
**Required:** Yes

Array of backend services for this tenant.

## Service Configuration

### Service Object

Each service in the `services` array has the following structure:

#### `name`

**Type:** `string`  
**Required:** Yes

Unique name for the service within the tenant.

#### `url`

**Type:** `string`  
**Required:** Yes

Base URL of the backend service.

**Format:** `http://host:port` or `https://host:port`

#### `health`

**Type:** `string`  
**Default:** `"/health"`  
**Optional:** Yes

Health check endpoint path on the backend service.

```yaml
services:
  - name: "api-backend-1"
    url: "http://api-service-1:3001"
    health: "/health"
  
  - name: "api-backend-2" 
    url: "http://api-service-2:3001"
    health: "/status"
  
  - name: "database-api"
    url: "https://db-api.internal:8443"
    health: "/api/health"
```

### Complete Tenant Example

```yaml
tenants:
  - name: "production-api"
    domains: ["api.example.com", "api.company.org"]
    lua_routes: "production-auth-routes.lua"
    health_interval: 15
    services:
      - name: "api-primary"
        url: "http://api-1.internal:3001"
        health: "/health"
      - name: "api-secondary"
        url: "http://api-2.internal:3001"
        health: "/health"
      - name: "api-backup"
        url: "http://api-backup.internal:3001"
        health: "/status"
```

## Routing Strategies

Keystone Gateway supports three routing strategies with a specific priority order:

### 1. Hybrid Routing (Highest Priority)

Tenants that specify **both** `domains` and `path_prefix` use hybrid routing.

```yaml
tenants:
  - name: "api-v2"
    domains: ["api.example.com"]
    path_prefix: "/v2/"
    # This tenant handles: api.example.com/v2/*
```

### 2. Host-based Routing (Medium Priority)

Tenants that specify only `domains` use host-based routing.

```yaml
tenants:
  - name: "api"
    domains: ["api.example.com", "api.company.org"]
    # This tenant handles: api.example.com/* and api.company.org/*
```

### 3. Path-based Routing (Lowest Priority)

Tenants that specify only `path_prefix` use path-based routing.

```yaml
tenants:
  - name: "legacy"
    path_prefix: "/legacy/"
    # This tenant handles: {any-host}/legacy/*
```

### Routing Resolution Example

Given this configuration:

```yaml
tenants:
  - name: "hybrid"
    domains: ["api.example.com"]
    path_prefix: "/v2/"
  
  - name: "host-only"
    domains: ["api.example.com"]
  
  - name: "path-only"
    path_prefix: "/v2/"
```

Request routing:
- `api.example.com/v2/users` → `hybrid` tenant (highest priority)
- `api.example.com/v1/users` → `host-only` tenant
- `other.com/v2/users` → `path-only` tenant
- `other.com/v1/users` → No match (404)

## Validation Rules

### Configuration Validation

1. **Lua routing must be enabled:**
   ```yaml
   lua_routing:
     enabled: true  # Must be true
   ```

2. **Each tenant must have exactly one routing strategy:**
   ```yaml
   # Valid: domains only
   - name: "api"
     domains: ["api.example.com"]
   
   # Valid: path_prefix only  
   - name: "app"
     path_prefix: "/app/"
   
   # Valid: both (hybrid)
   - name: "hybrid"
     domains: ["api.example.com"]
     path_prefix: "/v2/"
   
   # Invalid: neither specified
   - name: "invalid"
     # Missing routing configuration
   ```

3. **Domain validation:**
   ```yaml
   # Valid domains
   domains: ["api.example.com", "sub.domain.org", "localhost"]
   
   # Invalid domains
   domains: ["example", "api example.com", ""]
   ```

4. **Path prefix validation:**
   ```yaml
   # Valid path prefixes
   path_prefix: "/api/"
   path_prefix: "/app/v1/"
   path_prefix: "/legacy-system/"
   
   # Invalid path prefixes
   path_prefix: "api/"      # Must start with /
   path_prefix: "/api"      # Must end with /
   path_prefix: "/"         # Cannot be just /
   ```

5. **Service validation:**
   ```yaml
   services:
     # Valid service
     - name: "backend"
       url: "http://localhost:3001"
       health: "/health"
   
     # Invalid: missing required fields
     - url: "http://localhost:3001"  # Missing name
   ```

### Runtime Validation

The gateway performs additional validation at startup:

1. **Lua script files must exist** in the specified `scripts_dir`
2. **Tenant names must be unique** across all tenants
3. **Service names must be unique** within each tenant
4. **URLs must be valid** HTTP/HTTPS URLs

## Environment Variables

Configuration can be supplemented with environment variables:

### Gateway Configuration

```bash
# Override default port
export GATEWAY_PORT=8080

# Override config file path
export GATEWAY_CONFIG=/path/to/config.yaml

# Enable debug logging
export DEBUG=true

# Override scripts directory
export LUA_SCRIPTS_DIR=/opt/scripts
```

### Service URLs

Environment variables can be used in service URLs:

```yaml
services:
  - name: "api-backend"
    url: "${API_BACKEND_URL:-http://localhost:3001}"
    health: "/health"
```

```bash
export API_BACKEND_URL=http://api-prod.internal:3001
```

## Examples

### Development Configuration

```yaml
admin_base_path: "/admin"

lua_routing:
  enabled: true
  scripts_dir: "./scripts"

tenants:
  - name: "dev-api"
    domains: ["localhost", "127.0.0.1"]
    lua_routes: "development-routes.lua"
    health_interval: 60
    services:
      - name: "local-backend"
        url: "http://localhost:3001"
        health: "/health"
```

### Production Multi-Tenant Configuration

```yaml
admin_base_path: "/admin"

lua_routing:
  enabled: true
  scripts_dir: "/opt/keystone/scripts"

tenants:
  # Production API
  - name: "prod-api"
    domains: ["api.production.com"]
    lua_routes: "production-auth-routes.lua"
    health_interval: 15
    services:
      - name: "api-1"
        url: "http://api-1.internal:3001"
        health: "/health"
      - name: "api-2"
        url: "http://api-2.internal:3001"
        health: "/health"

  # Staging environment
  - name: "staging"
    domains: ["staging.production.com"]
    lua_routes: "staging-routes.lua"
    health_interval: 30
    services:
      - name: "staging-backend"
        url: "http://staging.internal:3001"
        health: "/health"

  # Legacy system
  - name: "legacy"
    path_prefix: "/legacy/"
    lua_routes: "legacy-routes.lua"
    health_interval: 120
    services:
      - name: "legacy-system"
        url: "http://legacy.internal:8080"
        health: "/status"
```

### Microservices Configuration

```yaml
admin_base_path: "/admin"

lua_routing:
  enabled: true
  scripts_dir: "./scripts"

tenants:
  # User service
  - name: "users"
    path_prefix: "/users/"
    lua_routes: "user-service-routes.lua"
    health_interval: 20
    services:
      - name: "user-service-1"
        url: "http://user-service-1:3000"
        health: "/health"
      - name: "user-service-2"
        url: "http://user-service-2:3000"
        health: "/health"

  # Order service
  - name: "orders"
    path_prefix: "/orders/"
    lua_routes: "order-service-routes.lua"
    health_interval: 20
    services:
      - name: "order-service"
        url: "http://order-service:3000"
        health: "/health"

  # Payment service
  - name: "payments"
    path_prefix: "/payments/"
    lua_routes: "payment-service-routes.lua"
    health_interval: 10  # More frequent for critical service
    services:
      - name: "payment-service"
        url: "http://payment-service:3000"
        health: "/health"
```

For more configuration examples, see the [configs/examples/](../configs/examples/) directory.