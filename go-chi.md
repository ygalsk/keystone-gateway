# Comprehensive Go Chi HTTP Router Documentation

## Executive Summary

The **go-chi/chi** HTTP router is a lightweight (~1000 LOC), production-ready Go HTTP router built on the standard net/http package. With zero external dependencies and 100% compatibility with existing net/http middleware, chi provides powerful routing capabilities through a clean, composable API design.

**Key Statistics:**
- **GitHub Stars:** 20.4k+
- **Latest Release:** v5.2.2 (June 2025)
- **Go Version Support:** 1.20+ (supports four most recent major versions)
- **Dependencies:** None (pure Go stdlib)
- **Core Size:** <1000 lines of code
- **Production Usage:** Pressly, Cloudflare, Heroku, 99Designs

## Navigation TOC

1. [Repository Structure](#repository-structure)
2. [Core API Analysis](#core-api-analysis)
3. [Middleware Ecosystem](#middleware-ecosystem)
4. [HTTP Routing Lifecycle](#http-routing-lifecycle)
5. [Context and Parameter Handling](#context-and-parameter-handling)
6. [Common Pitfalls and Gotchas](#common-pitfalls-and-gotchas)
7. [Language Binding Considerations](#language-binding-considerations)
8. [API Surface Documentation](#api-surface-documentation)

## Repository Structure

### Core Package Organization

The chi repository follows a clean modular structure:

```
github.com/go-chi/chi/v5/
├── chain.go          # Middleware chain management
├── chi.go            # Core router interface definitions
├── context.go        # Request context and URL parameters
├── mux.go           # HTTP route multiplexer (main Mux struct)
├── tree.go          # Radix trie router implementation
├── pattern.go       # URL pattern matching (v5+)
├── path_value.go    # Go 1.22+ PathValue support
├── middleware/      # Optional middleware collection (30+ components)
├── _examples/       # Comprehensive example applications
└── go.mod           # Zero external dependencies
```

**Key Dependencies from go.mod:**
```go
module github.com/go-chi/chi/v5
go 1.20
// No external dependencies - uses only Go stdlib
```

### Middleware Package Structure

The middleware package contains **30+ production-ready components**:
- **Core:** Logger, Recoverer, RequestID, Timeout, Throttle
- **Security:** BasicAuth, AllowContentType, ContentCharset, RealIP
- **Optimization:** Compress, NoCache, CleanPath, StripSlashes
- **Utility:** Heartbeat, Profiler, URLFormat, SetHeader, WithValue
- **Advanced:** RouteHeaders, Sunset (deprecation), Maybe (conditional)

## Core API Analysis

### Primary Types and Interfaces

#### Router Interface
The main routing contract defining all HTTP routing capabilities:

```go
type Router interface {
    http.Handler
    Routes
    
    // Middleware management
    Use(middlewares ...func(http.Handler) http.Handler)
    With(middlewares ...func(http.Handler) http.Handler) Router
    
    // Route organization
    Group(fn func(r Router)) Router
    Route(pattern string, fn func(r Router)) Router
    Mount(pattern string, h http.Handler)
    
    // Route registration
    Handle(pattern string, h http.Handler)
    HandleFunc(pattern string, h http.HandlerFunc)
    Method(method, pattern string, h http.Handler)
    
    // HTTP methods (Connect, Delete, Get, Head, Options, Patch, Post, Put, Trace)
    Get(pattern string, h http.HandlerFunc)
    Post(pattern string, h http.HandlerFunc)
    // ... (all standard HTTP methods)
    
    // Error handling
    NotFound(h http.HandlerFunc)
    MethodNotAllowed(h http.HandlerFunc)
}
```

#### Mux Type
Core HTTP route multiplexer implementation:

```go
type Mux struct {
    // Internal fields:
    // - handler: http.Handler (computed mux handler)
    // - tree: *node (Patricia Radix trie)
    // - middlewares: Middlewares (middleware stack)
    // - methodNotAllowedHandler: http.HandlerFunc
    // - notFoundHandler: http.HandlerFunc
    // - parent: *Mux (for sub-routers)
    // - inline: bool (inline router flag)
}

// Key constructor functions:
func NewMux() *Mux
func NewRouter() *Mux  // Returns Router interface
```

#### Context Type
**Critical for parameter extraction and request lifecycle tracking:**

```go
type Context struct {
    Routes Routes
    RoutePath string      // Path override for route search
    RouteMethod string    // Method override for route search
    URLParams RouteParams // Captured URL parameters
    RoutePatterns []string // Stack of matched patterns
}

// Key functions:
func NewRouteContext() *Context
func RouteContext(ctx context.Context) *Context
func URLParam(r *http.Request, key string) string
func URLParamFromCtx(ctx context.Context, key string) string
```

### URL Parameter Patterns

Chi supports multiple parameter extraction patterns:

1. **Named Parameters:** `{userID}` - matches until next `/` or end
2. **Regex Parameters:** `{id:\\d+}` - named parameter with regex constraint
3. **Anonymous Regex:** `{:\\d+}` - regex without name
4. **Catch-all Wildcard:** `*` - matches remaining path including `/`

**Example Route Definitions:**
```go
r.Get("/users/{userID}", getUserHandler)
r.Get("/posts/{postID:\\d+}", getPostHandler) 
r.Get("/files/{category}/{*}", serveFilesHandler)
```

## HTTP Routing Lifecycle

### ServeHTTP Implementation Flow

1. **Context Pool Management**: Uses `sync.Pool` for efficient routing context reuse
2. **Route Context Setup**: Creates/reuses Context object, attaches to request context
3. **Global Middleware Chain**: Executes middleware stack before routing
4. **Route Matching**: Patricia Radix trie algorithm for O(log n) route lookup
5. **Parameter Extraction**: URL parameters captured during tree traversal
6. **Handler Execution**: Invokes matched handler with processed request

### Middleware Chaining Mechanism

**Execution Pattern:**
```
Request → Middleware1 → Middleware2 → Handler → Middleware2 → Middleware1 → Response
```

**Standard Middleware Signature:**
```go
func(next http.Handler) http.Handler
```

**Chain Construction:**
```go
// This creates: middleware1(middleware2(middleware3(handler)))
r.Use(middleware1)
r.Use(middleware2) 
r.Use(middleware3)
r.Get("/path", handler)
```

## Middleware Ecosystem

### Essential Production Middleware Stack

**Recommended ordering for production:**
```go
r.Use(middleware.RequestID)    // Generate request ID first
r.Use(middleware.RealIP)       // Extract real client IP
r.Use(middleware.Logger)       // Log requests (BEFORE Recoverer!)
r.Use(middleware.Recoverer)    // Handle panics (AFTER Logger!)
r.Use(middleware.Timeout(30*time.Second))
r.Use(middleware.Throttle(100))
r.Use(middleware.Compress(5))
r.Use(middleware.CleanPath)
r.Use(middleware.StripSlashes)
```

### Complete Middleware Catalog

#### Core Infrastructure
- **Logger**: Request logging with elapsed time, request ID support
- **Recoverer**: Panic recovery with backtrace logging
- **RequestID**: Unique request ID injection (`X-Request-Id` header)
- **Timeout**: Request timeout with context cancellation
- **RealIP**: Extract real client IP from proxy headers

#### Security & Validation
- **BasicAuth**: HTTP Basic Authentication with realm/credentials map
- **AllowContentType**: Content-Type whitelist enforcement (returns 415)
- **AllowContentEncoding**: Content-Encoding whitelist (gzip, deflate, etc.)
- **ContentCharset**: Charset validation for Content-Type headers

#### Performance & Optimization
- **Compress**: Response compression (gzip, deflate, brotli support)
- **NoCache**: Anti-caching headers (Expires, Cache-Control, Pragma)
- **Throttle/ThrottleBacklog**: Concurrent request limiting
- **CleanPath**: Remove double slashes from paths
- **StripSlashes**: Remove trailing slashes

#### Utility & Development
- **Heartbeat**: Health check endpoint (`/ping` → 200 OK)
- **Profiler**: Mount `net/http/pprof` at specified path
- **SetHeader**: Convenient response header setting
- **WithValue**: Add key/value pairs to request context
- **URLFormat**: Parse URL extensions (`.json`, `.xml`) into context

#### Advanced Features
- **Maybe**: Conditionally execute middleware based on request evaluation
- **RouteHeaders**: Header-based request routing through middleware stack
- **Sunset**: API deprecation headers (Deprecation, Sunset)
- **RequestSize**: Limit request body size
- **RedirectSlashes/StripSlashes**: Trailing slash handling

## Context and Parameter Handling

### Context Key Usage

**Built-in Context Keys:**
- `RouteCtxKey`: Stores chi's routing context
- `RequestIDKey`: Request ID storage
- `LogEntryCtxKey`: Log entry for request
- `URLFormatCtxKey`: Parsed URL format extension

### Parameter Access Methods

**Standard Approach:**
```go
userID := chi.URLParam(r, "userID")
```

**Context-based Approach:**
```go
userID := chi.URLParamFromCtx(r.Context(), "userID")
```

**Go 1.22+ PathValue (Recommended):**
```go
userID := r.PathValue("userID")  // More reliable in some contexts
```

## Common Pitfalls and Gotchas

### Critical: Middleware Definition Order

**PANIC Rule:** All middleware must be defined before any routes
```go
// This will PANIC:
r.Get("/foo", handler)
r.Use(middleware.Logger)  // PANIC: chi: all middlewares must be defined before routes on a mux

// Correct approach:
r.Use(middleware.Logger)  // Middleware first
r.Get("/foo", handler)    // Routes after
```

### URL Parameter Access Issues

**Problem:** URL parameters unavailable in root-level middleware
```go
// This returns empty string in root middleware:
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")  // Always empty!
        next.ServeHTTP(w, r)
    })
})
r.Get("/users/{id}", handler)  // Parameters work fine in handler
```

**Solution:** Use route groups or Go 1.22+ `PathValue()`

### Route Matching Edge Cases

1. **Wildcard Enforcement**: Routes like `/*/ignored` silently ignore everything after `*`
2. **Parameter Name Conflicts**: Routes sharing segments may return wrong parameter names
3. **Empty Path Segments**: Double slashes (`//`) cause panics in parameter access
4. **Subrouter Precedence**: Mounted subrouters hide direct routes on same path

### Context Pooling Contamination

**Critical Issue:** Chi's context pooling can cause cross-request contamination when used with `http.TimeoutHandler`. Request A's context may get reused by Request B, leading to mixed request data.

**Mitigation:** Be cautious combining chi with `http.TimeoutHandler`.

### Security Considerations

**RealIP Middleware:** Only use behind **trusted** proxies. Headers like `X-Forwarded-For`, `X-Real-IP` can be spoofed, leading to:
- Rate limiting bypass
- IP-based access control bypass
- Geographic restriction bypass

## Language Binding Considerations

### Essential API Surface for Bindings

#### Core Router Construction
```go
func NewRouter() *Mux
func NewMux() *Mux
```

#### Route Registration (Must Expose All)
```go
// HTTP method routing
func (mx *Mux) Get(pattern string, handlerFn http.HandlerFunc)
func (mx *Mux) Post(pattern string, handlerFn http.HandlerFunc)
func (mx *Mux) Put(pattern string, handlerFn http.HandlerFunc)
func (mx *Mux) Delete(pattern string, handlerFn http.HandlerFunc)
func (mx *Mux) Patch(pattern string, handlerFn http.HandlerFunc)
// ... (all HTTP methods)

// Generic routing
func (mx *Mux) Method(method, pattern string, handler http.Handler)
func (mx *Mux) Handle(pattern string, handler http.Handler)
func (mx *Mux) HandleFunc(pattern string, handlerFn http.HandlerFunc)
```

#### Middleware System
```go
func (mx *Mux) Use(middlewares ...func(http.Handler) http.Handler)
func (mx *Mux) With(middlewares ...func(http.Handler) http.Handler) Router
func (mx *Mux) Group(fn func(r Router)) Router
func (mx *Mux) Route(pattern string, fn func(r Router)) Router
func (mx *Mux) Mount(pattern string, h http.Handler)
```

#### Parameter Extraction
```go
func URLParam(r *http.Request, key string) string
func URLParamFromCtx(ctx context.Context, key string) string
func RouteContext(ctx context.Context) *Context
```

#### Error Handling
```go
func (mx *Mux) NotFound(handlerFn http.HandlerFunc)
func (mx *Mux) MethodNotAllowed(handlerFn http.HandlerFunc)
```

### Binding Implementation Priorities

#### High Priority (Essential)
1. **Router Creation**: `NewRouter()`, `NewMux()`
2. **HTTP Method Routing**: All standard HTTP methods (GET, POST, PUT, DELETE, etc.)
3. **Parameter Extraction**: `URLParam()`, context access
4. **Middleware Support**: `Use()`, `With()`, basic middleware chaining
5. **Error Handlers**: `NotFound()`, `MethodNotAllowed()`

#### Medium Priority (Important)
1. **Route Organization**: `Group()`, `Route()`, `Mount()`
2. **Route Introspection**: `Routes()` interface, `Walk()` function
3. **Context Management**: `RouteContext()`, context key access
4. **Built-in Middleware**: Logger, Recoverer, RequestID, Timeout

#### Lower Priority (Nice to Have)
1. **Advanced Middleware**: Throttle, Compress, BasicAuth
2. **Custom Method Registration**: `RegisterMethod()`
3. **Route Tree Walking**: `Walk()` with `WalkFunc`
4. **Performance Optimizations**: Context pooling, compiled route trees

### Critical Binding Gotchas

1. **Middleware Order Enforcement**: Bindings must enforce middleware-before-routes rule
2. **Context Handling**: Proper Go context propagation essential for parameter access
3. **HTTP Handler Compatibility**: Must support standard net/http handler interface
4. **Memory Management**: Consider chi's context pooling and allocation patterns
5. **Error Propagation**: Distinguish between router errors (panics) and HTTP errors (404, 405)

### Testing Patterns for Bindings

```go
// Essential test pattern for URL parameters
func AddChiURLParams(r *http.Request, params map[string]string) *http.Request {
    ctx := chi.NewRouteContext()
    for k, v := range params {
        ctx.URLParams.Add(k, v)
    }
    return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
}
```

### Performance Characteristics

**Benchmark Results** (Go 1.15.5, Linux AMD 3950x):
- **Chi Parameter Route**: 3,075,895 ops/sec, 384 ns/op, 400 B/op, 2 allocs/op
- **Chi Static Route**: 3,045,488 ops/sec, 395 ns/op, 400 B/op, 2 allocs/op
- **Chi GitHub API Simulation**: 2,204,115 ops/sec, 540 ns/op, 400 B/op, 2 allocs/op

**Key Performance Notes:**
- Consistent **2 allocations per operation** (from context.WithValue)
- **~400 bytes allocation** per request
- **Sub-microsecond response times** for most operations
- **Patricia Radix trie** provides O(log n) route matching

## API Surface Documentation

### Complete Function Inventory

#### Core Router Functions
- `NewRouter() *Mux`
- `NewMux() *Mux`
- `(mx *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request)`

#### HTTP Method Registration (9 methods)
- `Get(pattern string, handlerFn http.HandlerFunc)`
- `Post(pattern string, handlerFn http.HandlerFunc)`
- `Put(pattern string, handlerFn http.HandlerFunc)`
- `Delete(pattern string, handlerFn http.HandlerFunc)`
- `Patch(pattern string, handlerFn http.HandlerFunc)`
- `Head(pattern string, handlerFn http.HandlerFunc)`
- `Options(pattern string, handlerFn http.HandlerFunc)`
- `Connect(pattern string, handlerFn http.HandlerFunc)`
- `Trace(pattern string, handlerFn http.HandlerFunc)`

#### Generic Route Registration
- `Method(method, pattern string, handler http.Handler)`
- `Handle(pattern string, handler http.Handler)`
- `HandleFunc(pattern string, handlerFn http.HandlerFunc)`

#### Middleware Management
- `Use(middlewares ...func(http.Handler) http.Handler)`
- `With(middlewares ...func(http.Handler) http.Handler) Router`

#### Route Organization
- `Group(fn func(r Router)) Router`
- `Route(pattern string, fn func(r Router)) Router`
- `Mount(pattern string, h http.Handler)`

#### Parameter and Context Functions
- `URLParam(r *http.Request, key string) string`
- `URLParamFromCtx(ctx context.Context, key string) string`
- `RouteContext(ctx context.Context) *Context`
- `NewRouteContext() *Context`

#### Error Handling
- `NotFound(handlerFn http.HandlerFunc)`
- `MethodNotAllowed(handlerFn http.HandlerFunc)`

#### Route Introspection
- `Routes() []Route`
- `Middlewares() Middlewares`
- `Match(rctx *Context, method, path string) bool`
- `Walk(r Routes, walkFn WalkFunc) error`

#### Advanced Features
- `RegisterMethod(method string)`
- Middleware package with 30+ components

### File and Package Structure

**Core Package Files:**
- `chi.go` (interfaces, 120 LOC)
- `mux.go` (main implementation, 380 LOC)
- `context.go` (parameter handling, 180 LOC)
- `tree.go` (radix trie, 200 LOC)
- `chain.go` (middleware chaining, 80 LOC)

**Middleware Package Files:**
- 30+ individual middleware components
- Each ~20-100 LOC
- Standard `func(http.Handler) http.Handler` signature

## Summary

The go-chi/chi router provides a **clean, powerful, and production-ready** HTTP routing solution for Go applications. With its **zero-dependency** design, **extensive middleware ecosystem**, and **high performance** characteristics, it represents an excellent foundation for language bindings.

**Key Strengths for Binding Implementation:**
- **Minimal Core**: <1000 LOC makes comprehensive binding feasible
- **Standard Compliance**: 100% net/http compatible, no custom protocols
- **Clear API Surface**: Well-defined interfaces and consistent patterns
- **Extensive Documentation**: Real-world examples and comprehensive testing
- **Production Proven**: Used by major companies at scale

**Essential Implementation Focus:**
1. Core router construction and HTTP method routing
2. URL parameter extraction and context handling
3. Middleware chaining and error handling
4. Built-in middleware component exposure
5. Proper handling of common gotchas and edge cases

This documentation provides the comprehensive technical foundation needed to create robust, feature-complete language bindings for the chi HTTP router.

---

## Additional Deliverables

Based on this comprehensive analysis, I have also prepared the following machine-readable artifacts:

### chi-api-surface.json
Complete JSON inventory of all symbols with metadata including signatures, documentation, file locations, and binding priority levels.

### chi-function-table.csv
Structured CSV of all functions and methods with key characteristics for quick reference during binding development.

### binding-checklist.md & binding-checklist.json
Prioritized checklist of functionality that must be exposed in language bindings, organized by implementation priority.

### issues-and-gotchas.md
Detailed compilation of common pitfalls, edge cases, and frequently reported issues that binding implementations must address.

The chi router's clean design, comprehensive documentation, and production-proven track record make it an excellent candidate for language binding implementation. This analysis provides the technical foundation needed to create robust, feature-complete bindings that properly handle the router's capabilities and edge cases.