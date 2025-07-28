# Analysis: Chi Middleware Compression Implementation

## Agent Analysis 1: Current Middleware Structure

Based on my comprehensive analysis of the keystone-gateway codebase, here's what I found regarding middleware implementation, Chi router configuration, and existing patterns:

### 1. How Middleware is Currently Implemented and Integrated with Chi Router

#### Base Middleware Setup (main.go:130-142)
The application uses Chi's built-in middleware stack configured in `setupBaseMiddleware()`:
```go
r.Use(middleware.Logger)
r.Use(middleware.Recoverer) 
r.Use(middleware.RealIP)
r.Use(middleware.RequestID)
r.Use(middleware.Timeout(DefaultRequestTimeout))
```

#### Lua-Based Dynamic Middleware (chi_bindings.go:158-226)
The system supports dynamic middleware registration through Lua scripts:
- **Pattern-based middleware**: Applied to routes matching specific patterns (e.g., `/api/*`)
- **Group-scoped middleware**: Applied only within route groups
- **Global middleware**: Applied to all routes
- **Tenant-scoped middleware**: Isolated by tenant

Key implementation details:
- Middleware logic is cached for performance (`MiddlewareCache`)
- Pattern matching supports wildcards (`/api/*`)
- Middleware is applied in reverse registration order (LIFO)

### 2. Where Middleware is Configured and Applied

#### Configuration Locations:
1. **Base middleware**: `/home/dkremer/keystone-gateway/cmd/main.go:130-142`
2. **Lua middleware registration**: `/home/dkremer/keystone-gateway/internal/lua/chi_bindings.go:158-226`
3. **Middleware application logic**: `/home/dkremer/keystone-gateway/internal/routing/lua_routes.go:322-339`
4. **Host-based routing middleware**: `/home/dkremer/keystone-gateway/cmd/main.go:218-246`

#### Application Points:
- **Global**: Applied to all requests at router level
- **Tenant-specific**: Applied within tenant submux routes
- **Pattern-based**: Applied to routes matching specific patterns
- **Group-based**: Applied within route groups

### 3. Existing Middleware Patterns

#### Current Patterns Used:
1. **Security headers** (global-security.lua):
   ```lua
   chi_middleware("/*", function(request, response, next)
       response:header("X-Content-Type-Options", "nosniff")
       response:header("X-Frame-Options", "DENY")
       -- etc.
   end)
   ```

2. **Tenant identification** (development-routes.lua):
   ```lua
   chi_middleware("/*", function(request, response, next)
       response:header("X-Tenant", "v2")
       response:header("X-API-Version", "2.0")
       next()
   end)
   ```

3. **Built-in Chi middleware**: Logger, Recoverer, RealIP, RequestID, Timeout

### 4. Chi Router Setup and Configuration

#### Router Initialization:
- **Main router**: Created in `/home/dkremer/keystone-gateway/cmd/main.go:122-128`
- **Tenant submux**: Created per tenant in `/home/dkremer/keystone-gateway/internal/routing/lua_routes.go:189-211`
- **Route registry**: Manages dynamic routes in `/home/dkremer/keystone-gateway/internal/routing/lua_routes.go`

#### Router Structure:
```
Main Router (chi.Mux)
├── Base Middleware (Logger, Recoverer, etc.)
├── Host-based Routing Middleware
├── Admin Routes (/.../health, /.../tenants)
└── Tenant Routes
    ├── Path-based mounting (/tenant-prefix/*)
    └── Host-based routing (via middleware)
```

### 5. Existing Compression/Similar Middleware Implementations

#### Current State:
- **No compression middleware found** in the current implementation
- **Base middleware only**: Chi's standard middleware (Logger, Recoverer, RealIP, RequestID, Timeout)
- **Custom middleware framework**: Exists for Lua-based dynamic middleware
- **Connection pooling**: Implemented in gateway.go with optimized HTTP transport

#### Related Infrastructure:
- **HTTP Transport optimization**: `/home/dkremer/keystone-gateway/internal/routing/gateway.go:58-73`
  ```go
  transport: &http.Transport{
      MaxIdleConns:        100,
      MaxIdleConnsPerHost: 20,
      IdleConnTimeout:     90 * time.Second,
      DisableKeepAlives:   false,
  }
  ```

## Key File Locations and Line Numbers:

- **Main middleware setup**: `/home/dkremer/keystone-gateway/cmd/main.go:130-142`
- **Lua middleware bindings**: `/home/dkremer/keystone-gateway/internal/lua/chi_bindings.go:74-226`
- **Route registry**: `/home/dkremer/keystone-gateway/internal/routing/lua_routes.go:16-400`
- **Gateway proxy setup**: `/home/dkremer/keystone-gateway/internal/routing/gateway.go:48-77`
- **Middleware tests**: `/home/dkremer/keystone-gateway/tests/unit/middleware_security_test.go`
- **Configuration structure**: `/home/dkremer/keystone-gateway/internal/config/config.go:14-46`

## Dependencies:
- **Chi router**: `github.com/go-chi/chi/v5 v5.2.2`
- **Gopher Lua**: `github.com/yuin/gopher-lua v1.1.1`

The system is well-architected for adding compression middleware, with clear patterns for both Go-native middleware (like the base middleware stack) and Lua-scriptable middleware (for dynamic, tenant-specific behavior).

---

## Agent Analysis 2: Chi Compression Options Research

Based on my comprehensive research, here's a detailed analysis of Chi router compression middleware options for your Keystone Gateway:

### 1. Built-in Chi Compression Middleware

**Import Path:** `github.com/go-chi/chi/v5/middleware`

**Available Functions:**
- `middleware.Compress(level int, types ...string)` - Basic compression with level and content types
- `middleware.NewCompressor(level int, types ...string)` - Advanced configuration
- `compressor.SetEncoder(encoding string, fn EncoderFunc)` - Custom encoder registration

**Supported Encodings:**
- Gzip (primary)
- Deflate (secondary)

**Usage Example:**
```go
import "github.com/go-chi/chi/v5/middleware"

// Basic usage
r.Use(middleware.Compress(5, "text/html", "text/css", "text/javascript", "application/json"))

// Advanced usage with custom configuration
compressor := middleware.NewCompressor(5, "text/html", "application/json")
r.Use(compressor.Handler)
```

**Current Dependencies Available:**
Your `go.mod` already includes:
- `github.com/go-chi/chi/v5 v5.2.2` ✅

### 2. Popular Third-Party Compression Libraries

#### A. CAFxX/httpcompression (Recommended)
**Import Path:** `github.com/CAFxX/httpcompression`

**Features:**
- Supports gzip, deflate, brotli, zstandard, XZ/LZMA2, LZ4
- Custom dictionary compression
- Extensible architecture
- Chi-compatible middleware interface

**Usage Example:**
```go
import "github.com/CAFxX/httpcompression"

compress, _ := httpcompression.DefaultAdapter()
r.Use(compress)
```

#### B. Klaus Post Enhanced Compression
**Import Path:** `github.com/klauspost/compress/gzhttp`

**Features:**
- Faster gzip implementation than standard library
- Drop-in replacement for standard gzip
- Better performance for high-traffic applications

**Usage Example:**
```go
import "github.com/klauspost/compress/gzhttp"

r.Use(gzhttp.GzipHandler)
```

#### C. NYTimes gziphandler (Legacy)
**Import Path:** `github.com/NYTimes/gziphandler`

**Features:**
- Well-tested in production
- Simple gzip-only compression
- Now superseded by more modern alternatives

### 3. Current Codebase Analysis

**Existing Middleware Stack (in `/home/dkremer/keystone-gateway/cmd/main.go`):**
```go
func (app *Application) setupBaseMiddleware(r *chi.Mux) {
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.RealIP)
    r.Use(middleware.RequestID)
    r.Use(middleware.Timeout(DefaultRequestTimeout))
    
    // Compression middleware would fit here
    
    if app.luaEngine != nil {
        r.Use(app.hostBasedRoutingMiddleware())
    }
}
```

**No Current Compression:** Search results show no existing compression middleware is currently implemented.

### 4. Standard Go Compression Patterns

**Using Standard Library (compress/gzip):**
```go
import (
    "compress/gzip"
    "net/http"
)

func compressionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
            w.Header().Set("Content-Encoding", "gzip")
            gz := gzip.NewWriter(w)
            defer gz.Close()
            gzw := &gzipResponseWriter{Writer: gz, ResponseWriter: w}
            next.ServeHTTP(gzw, r)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### 5. Implementation Recommendations

**For Immediate Implementation:**
1. **Use Chi's built-in compression** - Already available, no new dependencies
2. **Add to setupBaseMiddleware** - Insert after RealIP, before Timeout

**For Enhanced Performance:**
1. **Consider CAFxX/httpcompression** - Modern, multi-algorithm support
2. **Add brotli support** - Better compression ratios for modern browsers

**Example Implementation:**
```go
// In setupBaseMiddleware function
r.Use(middleware.RealIP)
r.Use(middleware.RequestID)

// Add compression middleware
r.Use(middleware.Compress(5, 
    "text/html", 
    "text/css", 
    "text/javascript", 
    "application/json", 
    "application/xml",
    "text/plain",
))

r.Use(middleware.Timeout(DefaultRequestTimeout))
```

### 6. Performance Considerations

**Compression Levels:**
- Level 1: Fastest, least compression
- Level 5: Balanced (recommended)
- Level 9: Best compression, slowest

**Content Types to Compress:**
- Text-based: HTML, CSS, JavaScript, JSON, XML
- Avoid: Images (JPEG, PNG), videos, already compressed files

**Headers Managed Automatically:**
- `Content-Encoding: gzip`
- `Vary: Accept-Encoding`
- Content-Length (recalculated)

This research provides you with multiple pathways to implement compression middleware in your Keystone Gateway, from the simple built-in Chi solution to more advanced third-party libraries with modern algorithms like brotli and zstandard.