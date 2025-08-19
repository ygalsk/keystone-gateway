# Keystone Gateway - Lua Scripting Best Practices

**Version:** 1.0  
**Target:** Dynamic routing and middleware management via Lua scripts  
**Architecture:** Production-ready patterns for Chi router integration

---

## 🎯 **Overview**

This guide establishes best practices for writing Lua scripts that dynamically control the Keystone Gateway's Chi router. These patterns ensure **production safety**, **performance excellence**, and **maintainable code** aligned with Go best practices.

### **Core Principles**
1. **Follow Chi's Mental Model** - Scripts should feel natural to Go/Chi developers
2. **Production Safety** - Proper error handling, timeouts, and resource management
3. **Multi-Tenant Aware** - Built-in tenant isolation and security
4. **Performance First** - Atomic operations, minimal allocations, efficient patterns
5. **Observability** - Structured logging and metrics integration

---

## 📋 **Table of Contents**

- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Best Practices](#best-practices)
- [Common Patterns](#common-patterns)
- [Error Handling](#error-handling)
- [Performance Guidelines](#performance-guidelines)
- [Multi-Tenant Patterns](#multi-tenant-patterns)
- [Testing Strategy](#testing-strategy)
- [Project Organization](#project-organization)
- [Security Guidelines](#security-guidelines)

---

## 🚀 **Quick Start**

### **Basic Route Registration**
```lua
-- Register a simple GET route
local success, err = chi_route("GET", "/api/health", function(req, res)
    res.json({
        status = "ok",
        timestamp = os.time(),
        version = "1.0.0"
    })
end)

if not success then
    log.error("Failed to register health route: " .. tostring(err))
    return false
end
```

### **Middleware Registration**
```lua
-- Add authentication middleware
local auth_success = chi_middleware("/api/v1/*", function(req, res, next)
    local token = req.headers["authorization"]
    
    if not token or not validate_jwt(token) then
        res.status_code = 401
        res.json({error = "Unauthorized"})
        return
    end
    
    -- Continue to next handler
    next()
end)
```

### **Route Groups**
```lua
-- Organize routes into logical groups
chi_group("/api/v1", function(r)
    -- Group-level middleware
    r.use(auth_middleware)
    r.use(rate_limit_middleware)
    
    -- User routes
    r.get("/users", list_users)
    r.get("/users/{id}", get_user)
    r.post("/users", create_user)
    r.put("/users/{id}", update_user)
end)
```

---

## 📚 **API Reference**

### **Route Management**

#### `chi_route(method, pattern, handler)`
Register a new HTTP route.

**Parameters:**
- `method` (string): HTTP method ("GET", "POST", "PUT", "DELETE", etc.)
- `pattern` (string): Chi-compatible route pattern (e.g., "/users/{id}")
- `handler` (function): Request handler function

**Returns:** `success (boolean), error (string|nil)`

```lua
local success, err = chi_route("GET", "/api/users/{id}", function(req, res)
    local user_id = chi_param(req, "id")
    -- Handler logic here
end)
```

#### `chi_remove_route(method, pattern)`
Remove an existing route.

**Parameters:**
- `method` (string): HTTP method
- `pattern` (string): Route pattern to remove

**Returns:** `success (boolean), error (string|nil)`

### **Middleware Management**

#### `chi_middleware(pattern, middleware_func)`
Register middleware for matching routes.

**Parameters:**
- `pattern` (string): Route pattern to match
- `middleware_func` (function): Middleware handler

**Returns:** `success (boolean), error (string|nil)`

```lua
chi_middleware("/api/*", function(req, res, next)
    -- Middleware logic
    req.start_time = os.clock()
    
    next() -- Continue to next handler
    
    -- Post-processing
    local duration = os.clock() - req.start_time
    log.info("Request completed", {duration = duration})
end)
```

### **Group Management**

#### `chi_group(pattern, setup_function)`
Create a route group with shared middleware.

**Parameters:**
- `pattern` (string): Base path for the group
- `setup_function` (function): Function that receives chi.Router

**Returns:** `success (boolean), error (string|nil)`

```lua
chi_group("/api/admin", function(r)
    r.use(admin_auth_middleware)
    r.get("/stats", admin_stats)
    r.post("/config", update_config)
end)
```

### **Utility Functions**

#### `chi_param(request, param_name)`
Extract URL parameter from request.

```lua
local user_id = chi_param(req, "id")
local category = chi_param(req, "category")
```

#### `chi_get_routes()`
List all registered routes (for debugging/monitoring).

```lua
local routes = chi_get_routes()
for _, route in ipairs(routes) do
    log.info("Route: " .. route.method .. " " .. route.pattern)
end
```

---

## ✅ **Best Practices**

### **1. Error Handling**

**✅ Always check operation results:**
```lua
-- Good
local success, err = chi_route("GET", "/api/users", handler)
if not success then
    log.error("Route registration failed", {error = err, pattern = "/api/users"})
    return false
end

-- Bad - ignoring errors
chi_route("GET", "/api/users", handler) -- Could fail silently
```

**✅ Provide fallback strategies:**
```lua
-- Good - graceful degradation
local auth_registered = chi_middleware("/api/*", jwt_auth_middleware)
if not auth_registered then
    log.warn("JWT auth failed, using basic auth")
    chi_middleware("/api/*", basic_auth_middleware)
end
```

### **2. Resource Management**

**✅ Cache expensive operations:**
```lua
-- Good - cache compiled patterns
local email_pattern = compile_regex("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$")

local function validate_email(email)
    return email_pattern:match(email)
end

-- Bad - recompiling every time
local function validate_email(email)
    local pattern = compile_regex("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$")
    return pattern:match(email)
end
```

**✅ Minimize allocations in hot paths:**
```lua
-- Good - reuse tables
local response_template = {
    status = "ok",
    data = nil,
    timestamp = 0
}

local function success_response(data)
    response_template.data = data
    response_template.timestamp = os.time()
    return response_template
end

-- Bad - new table every time
local function success_response(data)
    return {
        status = "ok",
        data = data,
        timestamp = os.time()
    }
end
```

### **3. Structured Logging**

**✅ Use structured, contextual logging:**
```lua
-- Good
log.info("Route registered successfully", {
    method = "GET",
    pattern = "/api/users",
    tenant = tenant_name,
    handler = "list_users",
    timestamp = os.time()
})

-- Bad
log.info("Registered GET /api/users for " .. tenant_name)
```

**✅ Log levels appropriately:**
```lua
-- Error: System problems
log.error("Failed to connect to database", {error = err})

-- Warn: Unusual but handled conditions  
log.warn("Rate limit exceeded", {client_ip = req.remote_addr})

-- Info: Important business events
log.info("User authenticated", {user_id = user.id, method = "jwt"})

-- Debug: Detailed diagnostic info
log.debug("Request processing", {path = req.path, duration = elapsed})
```

### **4. Security Practices**

**✅ Validate all inputs:**
```lua
local function create_user(req, res)
    -- Validate required fields
    if not req.body.email or not req.body.name then
        res.status_code = 400
        res.json({error = "Email and name are required"})
        return
    end
    
    -- Validate email format
    if not validate_email(req.body.email) then
        res.status_code = 400
        res.json({error = "Invalid email format"})
        return
    end
    
    -- Sanitize inputs
    local clean_email = sanitize_email(req.body.email)
    local clean_name = sanitize_name(req.body.name)
    
    -- Process safely...
end
```

**✅ Implement proper authentication:**
```lua
local function jwt_auth_middleware()
    return function(req, res, next)
        local auth_header = req.headers["authorization"]
        
        if not auth_header then
            res.status_code = 401
            res.json({error = "Authorization header required"})
            return
        end
        
        local token = auth_header:match("Bearer%s+(.+)")
        if not token then
            res.status_code = 401
            res.json({error = "Invalid authorization format"})
            return
        end
        
        local user, err = validate_jwt_token(token)
        if not user then
            res.status_code = 401
            res.json({error = "Invalid token: " .. tostring(err)})
            return
        end
        
        req.user = user
        next()
    end
end
```

---

## 🏗️ **Common Patterns**

### **RESTful API Pattern**
```lua
local function setup_user_api(r)
    -- List users with pagination
    r.get("/", function(req, res)
        local page = tonumber(req.query.page) or 1
        local limit = tonumber(req.query.limit) or 10
        
        local users, total = get_users_paginated(page, limit)
        res.json({
            users = users,
            pagination = {
                page = page,
                limit = limit,
                total = total,
                pages = math.ceil(total / limit)
            }
        })
    end)
    
    -- Get single user
    r.get("/{id}", function(req, res)
        local user_id = chi_param(req, "id")
        if not user_id then
            res.status_code = 400
            res.json({error = "User ID required"})
            return
        end
        
        local user = get_user_by_id(user_id)
        if not user then
            res.status_code = 404
            res.json({error = "User not found"})
            return
        end
        
        res.json({user = user})
    end)
    
    -- Create user
    r.post("/", function(req, res)
        local user_data = req.body
        
        -- Validation
        local errors = validate_user_data(user_data)
        if #errors > 0 then
            res.status_code = 400
            res.json({errors = errors})
            return
        end
        
        -- Create user
        local user, err = create_user(user_data)
        if not user then
            res.status_code = 500
            res.json({error = "Failed to create user: " .. tostring(err)})
            return
        end
        
        res.status_code = 201
        res.json({user = user})
    end)
end

-- Register the API
chi_group("/api/v1/users", setup_user_api)
```

### **Health Check Pattern**
```lua
local function health_check(req, res)
    local health = {
        status = "ok",
        timestamp = os.time(),
        version = "1.0.0",
        checks = {}
    }
    
    -- Database connectivity
    local db_ok, db_err = check_database()
    health.checks.database = {
        status = db_ok and "healthy" or "unhealthy",
        error = db_err
    }
    
    -- External service connectivity
    local api_ok, api_err = check_external_api()
    health.checks.external_api = {
        status = api_ok and "healthy" or "unhealthy", 
        error = api_err
    }
    
    -- Overall health
    local all_healthy = db_ok and api_ok
    health.status = all_healthy and "ok" or "degraded"
    
    res.status_code = all_healthy and 200 or 503
    res.json(health)
end

chi_route("GET", "/health", health_check)
chi_route("GET", "/api/health", health_check)
```

### **Rate Limiting Pattern**
```lua
local rate_limiter = {}

local function rate_limit_middleware(requests_per_minute)
    return function(req, res, next)
        local client_ip = req.remote_addr
        local current_time = os.time()
        local window_start = current_time - (current_time % 60) -- Start of current minute
        
        if not rate_limiter[client_ip] then
            rate_limiter[client_ip] = {count = 0, window = window_start}
        end
        
        local client_data = rate_limiter[client_ip]
        
        -- Reset counter if new window
        if client_data.window < window_start then
            client_data.count = 0
            client_data.window = window_start
        end
        
        -- Check limit
        if client_data.count >= requests_per_minute then
            res.status_code = 429
            res.headers["retry-after"] = "60"
            res.json({
                error = "Rate limit exceeded",
                limit = requests_per_minute,
                reset = window_start + 60
            })
            return
        end
        
        -- Increment counter and continue
        client_data.count = client_data.count + 1
        next()
    end
end

-- Apply rate limiting
chi_middleware("/api/*", rate_limit_middleware(100)) -- 100 requests per minute
```

---

## 🏢 **Multi-Tenant Patterns**

### **Tenant Resolution**
```lua
local function resolve_tenant(req)
    -- Check subdomain
    local host = req.headers["host"]
    if host then
        local subdomain = host:match("^([^%.]+)%.")
        if subdomain and subdomain ~= "www" then
            return subdomain
        end
    end
    
    -- Check header
    local tenant_header = req.headers["x-tenant"]
    if tenant_header then
        return tenant_header
    end
    
    -- Check path
    local tenant_path = req.path:match("^/tenant/([^/]+)")
    if tenant_path then
        return tenant_path
    end
    
    return "default"
end
```

### **Tenant-Specific Routing**
```lua
local function setup_tenant_routes()
    chi_middleware("/*", function(req, res, next)
        -- Resolve tenant
        local tenant = resolve_tenant(req)
        req.tenant = tenant
        
        -- Add tenant to response headers
        res.headers["x-tenant"] = tenant
        
        next()
    end)
    
    -- Tenant-specific API groups
    for _, tenant in ipairs(get_active_tenants()) do
        chi_group("/api/" .. tenant, function(r)
            -- Tenant-specific middleware
            r.use(tenant_auth_middleware(tenant))
            r.use(tenant_rate_limit_middleware(tenant))
            
            -- Tenant-specific routes
            setup_tenant_api_routes(r, tenant)
        end)
    end
end
```

---

## 🧪 **Testing Strategy**

### **Unit Testing**
```lua
-- test/user_routes_test.lua
local test = require("test_framework")

local function test_create_user()
    -- Setup
    local req = {
        method = "POST",
        body = {
            name = "John Doe",
            email = "john@example.com"
        }
    }
    local res = {}
    
    -- Execute
    create_user_handler(req, res)
    
    -- Assert
    test.assert_equals(res.status_code, 201)
    test.assert_not_nil(res.body.user)
    test.assert_equals(res.body.user.email, "john@example.com")
end

local function test_create_user_invalid_email()
    local req = {
        method = "POST",
        body = {
            name = "John Doe", 
            email = "invalid-email"
        }
    }
    local res = {}
    
    create_user_handler(req, res)
    
    test.assert_equals(res.status_code, 400)
    test.assert_contains(res.body.error, "Invalid email")
end

-- Run tests
test.run({
    test_create_user,
    test_create_user_invalid_email
})
```

### **Integration Testing**
```lua
-- test/integration_test.lua
local function test_full_user_workflow()
    -- Register routes
    local success = setup_user_routes()
    test.assert_true(success, "Failed to setup user routes")
    
    -- Test route registration
    local routes = chi_get_routes()
    local user_routes = filter(routes, function(r) 
        return r.pattern:match("/api/users") 
    end)
    
    test.assert_equals(#user_routes, 4) -- GET, POST, PUT, DELETE
    
    -- Test actual HTTP requests (if test framework supports)
    local response = http_request("GET", "/api/users")
    test.assert_equals(response.status, 200)
end
```

---

## 📁 **Project Organization**

### **Recommended Directory Structure**
```
/lua-scripts/
├── /common/                    # Shared utilities
│   ├── auth.lua               # Authentication middleware
│   ├── logging.lua            # Logging utilities
│   ├── metrics.lua            # Metrics collection
│   ├── validation.lua         # Input validation
│   └── utils.lua              # General utilities
│
├── /middleware/               # Reusable middleware
│   ├── cors.lua               # CORS handling
│   ├── rate_limiting.lua      # Rate limiting
│   ├── compression.lua        # Response compression
│   └── security.lua           # Security headers
│
├── /routes/                   # Route definitions
│   ├── health.lua             # Health check routes
│   ├── api_v1.lua             # API v1 routes
│   ├── api_v2.lua             # API v2 routes
│   └── admin.lua              # Admin routes
│
├── /tenants/                  # Tenant-specific scripts
│   ├── /tenant1/
│   │   ├── routes.lua         # Tenant-specific routes
│   │   ├── middleware.lua     # Tenant middleware
│   │   └── config.lua         # Tenant configuration
│   └── /tenant2/
│       └── ...
│
├── /test/                     # Test scripts
│   ├── /unit/
│   └── /integration/
│
└── main.lua                   # Main entry point
```

### **Module Loading Pattern**
```lua
-- main.lua
local auth = require("common.auth")
local logging = require("common.logging")
local user_routes = require("routes.api_v1.users")
local admin_routes = require("routes.admin")

-- Setup logging
logging.setup({
    level = "info",
    format = "json"
})

-- Setup global middleware
chi_middleware("/*", logging.request_middleware)
chi_middleware("/*", auth.jwt_middleware)

-- Setup routes
user_routes.setup()
admin_routes.setup()

log.info("Keystone Gateway Lua scripts loaded successfully")
```

---

## 🔒 **Security Guidelines**

### **Input Sanitization**
```lua
local function sanitize_input(input, input_type)
    if input_type == "email" then
        return input:lower():gsub("[^%w@%.%-_]", "")
    elseif input_type == "name" then
        return input:gsub("[^%w%s%-_]", "")
    elseif input_type == "id" then
        return input:gsub("[^%w%-]", "")
    end
    return input
end
```

### **SQL Injection Prevention**
```lua
-- Good - parameterized queries
local function get_user_by_email(email)
    local query = "SELECT * FROM users WHERE email = ?"
    return db.execute(query, {email})
end

-- Bad - string concatenation
local function get_user_by_email(email)
    local query = "SELECT * FROM users WHERE email = '" .. email .. "'"
    return db.execute(query)
end
```

### **XSS Prevention**
```lua
local function escape_html(str)
    return str:gsub("&", "&amp;")
              :gsub("<", "&lt;")
              :gsub(">", "&gt;")
              :gsub("\"", "&quot;")
              :gsub("'", "&#39;")
end
```

---

## 📊 **Performance Guidelines**

### **Memory Management**
- Reuse tables and objects where possible
- Use local variables instead of globals
- Clear large data structures when done
- Avoid creating functions in loops

### **CPU Optimization**
- Cache expensive computations
- Use appropriate data structures (arrays vs tables)
- Minimize string concatenation in loops
- Profile and measure performance

### **Network Efficiency**
- Implement connection pooling
- Use appropriate timeout values
- Compress large responses
- Implement proper caching headers

---

## 📈 **Monitoring and Observability**

### **Metrics Collection**
```lua
-- Custom metrics
metrics.counter("requests_total"):inc(1)
metrics.histogram("request_duration_seconds"):observe(duration)
metrics.gauge("active_connections"):set(connection_count)

-- Route-specific metrics
local route_counter = metrics.counter("route_requests_total", {"method", "route"})
route_counter:inc(1, {req.method, req.route})
```

### **Health Monitoring**
```lua
local function detailed_health_check()
    local checks = {}
    
    -- Component health checks
    checks.database = check_database_health()
    checks.cache = check_cache_health()  
    checks.external_apis = check_external_apis_health()
    
    -- Performance metrics
    checks.memory_usage = get_memory_usage()
    checks.cpu_usage = get_cpu_usage()
    checks.active_connections = get_active_connections()
    
    return checks
end
```

---

## 🔧 **Troubleshooting**

### **Common Issues**

**Route not registering:**
- Check for syntax errors in handler function
- Verify route pattern format
- Check for conflicts with existing routes

**Middleware not executing:**
- Verify middleware pattern matches request path
- Check middleware function signature
- Ensure `next()` is called when appropriate

**Performance issues:**
- Profile script execution time
- Check for memory leaks
- Monitor garbage collection frequency
- Review database query efficiency

### **Debugging Tools**
```lua
-- Enable debug logging
log.level = "debug"

-- Trace request flow
chi_middleware("/*", function(req, res, next)
    log.debug("Request start", {path = req.path, method = req.method})
    local start_time = os.clock()
    
    next()
    
    local duration = os.clock() - start_time
    log.debug("Request end", {path = req.path, duration = duration})
end)

-- Monitor route registration
local original_chi_route = chi_route
chi_route = function(method, pattern, handler)
    log.debug("Registering route", {method = method, pattern = pattern})
    return original_chi_route(method, pattern, handler)
end
```

---

## 📋 **Checklist**

Before deploying Lua scripts to production:

**✅ Code Quality**
- [ ] All functions have proper error handling
- [ ] Input validation implemented
- [ ] Logging added for important operations
- [ ] No hardcoded credentials or secrets
- [ ] Code follows consistent style

**✅ Security**
- [ ] Authentication implemented where needed
- [ ] Input sanitization in place
- [ ] SQL injection prevention
- [ ] XSS prevention for HTML output
- [ ] Rate limiting configured

**✅ Performance**
- [ ] No expensive operations in hot paths
- [ ] Resources properly cached
- [ ] Memory usage optimized
- [ ] Database queries optimized

**✅ Testing**
- [ ] Unit tests written and passing
- [ ] Integration tests covering main flows
- [ ] Load testing performed
- [ ] Error scenarios tested

**✅ Monitoring**
- [ ] Metrics collection implemented
- [ ] Health checks configured
- [ ] Alerting rules defined
- [ ] Log aggregation configured

---

*This document is part of the Keystone Gateway project. For questions or contributions, please refer to the project repository.*