# Keystone Gateway - Manifest

**Purpose:** This document defines the philosophy, principles, and guidelines that govern Keystone Gateway's development. Read this to understand **WHY** we make certain decisions.

For technical architecture details, see [DESIGN.md](DESIGN.md).
For the evolution history, see [ROADMAP.md](ROADMAP.md).

---

## Table of Contents

1. [Vision & Principles](#vision--principles)
2. [Core Philosophy](#core-philosophy)
3. [Anti-Patterns to Avoid](#anti-patterns-to-avoid)
4. [Development Guidelines](#development-guidelines)
5. [Future Evolution](#future-evolution)
6. [Principles Checklist](#principles-checklist)

---

## Vision & Principles

### What Keystone Gateway IS

**A general-purpose HTTP routing primitive with embedded Lua scripting.**

Keystone Gateway is a high-performance reverse proxy that provides:
- Multi-tenant HTTP routing (by domain, path, or both)
- Embedded Lua scripting for route definition
- Stateless request forwarding to backend services

### What Keystone Gateway IS NOT

- ❌ An API gateway with opinions (auth, rate limiting, etc.)
- ❌ An OAuth provider or authentication system
- ❌ A service mesh or distributed system
- ❌ A configuration management system
- ❌ A specialized tool for one use case

### Core Design Principle

> **The gateway is dumb. Tenants are smart.**

The gateway provides **powerful, general-purpose primitives**. Tenants compose these primitives into specific solutions for their needs.

---

## Core Philosophy

Based on "A Philosophy of Software Design" by John Ousterhout

### 1. Complexity is the Enemy

**From the book (Chapter 2):**
> "Complexity is anything related to the structure of a software system that makes it hard to understand and modify the system."

**Our application:**
- Minimize the number of concepts a developer must understand
- Hide complexity inside deep modules
- Keep interfaces simple and obvious

### 2. Deep Modules

**From the book (Chapter 4):**
> "The best modules are those whose interfaces are much simpler than their implementations."

**Deep modules in Keystone:**
```
┌─────────────────────────────────────┐
│      Simple Interface (small)       │
├─────────────────────────────────────┤
│                                     │
│                                     │
│    Complex Implementation (large)   │
│                                     │
│                                     │
└─────────────────────────────────────┘
```

**Examples:**
- **Lua Engine**: Simple API (`ExecuteRouteScript`), complex internals (state pooling, bytecode compilation, caching)
- **HTTP Client**: Simple API (`Get`, `Post`), complex internals (connection pooling, HTTP/2, timeouts)
- **Request Wrapper**: Simple property access (`req.Method`), complex internals (caching, parsing, validation)

### 3. Information Hiding

**From the book (Chapter 5):**
> "Each module should encapsulate a few pieces of knowledge, which represent design decisions."

**What we hide:**
- Lua state pool implementation
- Bytecode compilation strategy
- HTTP connection pooling details
- Request body caching mechanism

**What we expose:**
- Simple route registration
- Request/response properties
- HTTP client methods
- Configuration structure

### 4. Pull Complexity Downward

**From the book (Chapter 8):**
> "It is more important for a module to have a simple interface than a simple implementation."

**Application:**
- Complex logic belongs in **Go**, not **Lua scripts**
- Request body caching: automatic, not manual
- State management: hidden in pools, not exposed to users
- Error handling: define errors out of existence where possible

### 5. General-Purpose Modules

**From the book (Chapter 6):**
> "A somewhat general-purpose approach can be simpler than a special-purpose approach."

**Application:**
- HTTP client works for ANY HTTP request (not specialized for OAuth, REST, etc.)
- Request wrapper works for ANY request type
- Routing works for ANY tenant configuration
- No special cases baked into core

---

## Anti-Patterns to Avoid

### ❌ Shallow Modules (Too Many Small Functions)

**Bad (example from chi_bindings.go before refactoring):**
```go
L.SetGlobal("request_method", ...)      // 20 lines
L.SetGlobal("request_url", ...)         // 20 lines
L.SetGlobal("request_header", ...)      // 20 lines
L.SetGlobal("request_body", ...)        // 30 lines
// ... 20 more functions
```

**Good (with gopher-luar):**
```go
// Request table passed to Lua handlers
// Internal implementation uses optimized table construction

// Usage in Lua handler:
function handler(req)
    local method = req.method       -- Properties
    local auth = req.headers["Authorization"]
    local body = req.body           -- Auto-cached
    return {status = 200, body = "OK"}
end
```

**Why bad:** Each function requires type checking, error handling, Lua stack manipulation. 500 lines of boilerplate.

**Why good:** One module, many capabilities. 50 lines total with gopher-luar.

---

### ❌ Information Leakage

**Bad (OAuth in gateway core):**
```go
// Gateway knows about OAuth tokens
type OAuthConfig struct {
    TokenFile   string
    TokenFormat string  // Leaked implementation detail
    ExpiryBuffer int
}
```

**Good (OAuth in tenant code):**
```lua
-- Tenant's OAuth module uses gateway primitives
local OAuth = require("oauth_proxy")
local token = OAuth.get_token()  -- Implementation hidden

-- Gateway only provides HTTP primitives
local resp, err = http_get(url)
```

**Why bad:** Gateway is coupled to OAuth implementation details.

**Why good:** Gateway provides HTTP primitive, tenant composes auth logic.

---

### ❌ Pass-Through Variables

**Bad:**
```go
// cmd/main.go
cfg := config.LoadConfig(path)

// app/application.go
func New(cfg *config.Config) {
    engine := lua.NewEngine(cfg)  // Just passing through
}

// lua/engine.go
func NewEngine(cfg *config.Config) {
    limits := cfg.RequestLimits  // Only needs this
}
```

**Good:**
```go
// cmd/main.go
cfg := config.LoadConfig(path)

// app/application.go
func New(cfg *config.Config) {
    engine := lua.NewEngine(cfg.RequestLimits)  // Pass what's needed
}

// lua/engine.go
func NewEngine(limits RequestLimits) {
    // Only depends on what it uses
}
```

**Why bad:** `cfg` travels through multiple layers unchanged. Creates coupling.

**Why good:** Each layer takes only what it needs. Reduces coupling.

---

### ❌ Classitis (Too Many Empty Files)

**Bad:**
```
internal/routing/
  ├── gateway.go           # 200 lines
  ├── circuit_breaker.go   # EMPTY
  ├── health_checker.go    # EMPTY
  └── load_balancer.go     # EMPTY
```

**Good:**
```
internal/routing/
  └── gateway.go  # 250 lines (everything consolidated)
```

**Why bad:** Empty files created "in anticipation" of features. Adds cognitive load.

**Why good:** Code lives where it's needed. Add files when they're actually needed.

---

### ❌ Temporal Decomposition

**Bad (splitting by when code runs):**
```go
// Phase 1 functions
func ParseConfig() {}
func ValidateConfig() {}

// Phase 2 functions
func SetupRouting() {}
func StartServer() {}

// Phase 3 functions
func HandleRequest() {}
```

**Good (splitting by knowledge/capability):**
```go
// Config module (encapsulates config knowledge)
type Config struct {}
func LoadConfig() {}
func (c *Config) Validate() {}

// Gateway module (encapsulates routing knowledge)
type Gateway struct {}
func NewGateway(cfg Config) {}
func (g *Gateway) Handler() http.Handler
```

**Why bad:** Groups code by execution order. Changes spread across multiple places.

**Why good:** Groups code by related knowledge. Changes are localized.

---

### ❌ Special Cases in Core

**Bad:**
```go
// Gateway knows about HTML, CSV, ZIP formats
if strings.Contains(url, "format=html") {
    // HTML redirect logic
} else if strings.Contains(url, "format=csv") {
    // CSV download logic
} else if strings.Contains(url, "zipexport") {
    // ZIP logic
}
```

**Good:**
```go
// Gateway provides general-purpose HTTP proxy
func (gw *Gateway) Proxy(w, r, targetURL) {
    // Standard HTTP proxying
}

// Tenant handles special cases in their code
// scripts/lua/data_transforms.lua (tenant's file)
local Transforms = require("transforms")
if url:match("format=html") then
    Transforms.redirect_to_html(req, res)
end
```

**Why bad:** Gateway is coupled to specific data formats. Not general-purpose.

**Why good:** Gateway stays general. Tenants add their own transforms.

---

## Development Guidelines

### 1. When to Create a New Module

**Create a module when:**
- ✅ It encapsulates a distinct piece of knowledge (e.g., "how to manage Lua states")
- ✅ It can have a simple interface hiding complex implementation
- ✅ It's reusable across multiple parts of the codebase
- ✅ It has a clear, single responsibility

**Don't create a module when:**
- ❌ It's just one function (keep it in the parent module)
- ❌ It's empty or "planned for the future"
- ❌ It's only used in one place and tightly coupled to that place
- ❌ It's just grouping code by execution phase

**Example - Good module:**
```go
// internal/lua/compiler.go
// Clear responsibility: Compile Lua scripts to bytecode
type ScriptCompiler struct {
    cache map[string]*CompiledScript
}

func (c *ScriptCompiler) CompileScript(name, content string) (*CompiledScript, error)
func (c *ScriptCompiler) GetScript(name string) (*CompiledScript, bool)
```

---

### 2. Interface Design Checklist

Before exposing a function/method to Lua, ask:

- [ ] **Is this a primitive capability or business logic?**
  - Primitive → Gateway (e.g., HTTP request)
  - Business logic → Tenant (e.g., OAuth flow)

- [ ] **Can this be a property instead of a method?**
  - `req.Method` > `req:GetMethod()`
  - Properties are simpler and more discoverable

- [ ] **Does this leak implementation details?**
  - Bad: `GetTokenFromFile(path)` (leaks file storage)
  - Good: `GetToken()` (hides storage mechanism)

- [ ] **Is the interface minimal?**
  - Expose only what's necessary
  - Can't remove functions later (breaking change)
  - Can always add functions later (non-breaking)

- [ ] **Is it obvious what this does?**
  - Good: `req.body` - clearly contains request body
  - Bad: `req.data` - what data?

---

### 3. Error Handling Strategy

**Prefer defining errors out of existence:**

**Bad (errors as control flow):**
```go
func ValidatePath(path string) error {
    if !strings.HasPrefix(path, "/") {
        return errors.New("path must start with /")
    }
    if !strings.HasSuffix(path, "/") {
        return errors.New("path must end with /")
    }
    return nil
}
```

**Good (make it impossible to construct invalid state):**
```go
type PathPrefix string

func NewPathPrefix(s string) PathPrefix {
    // Auto-fix, can't be invalid
    s = strings.TrimSpace(s)
    if !strings.HasPrefix(s, "/") {
        s = "/" + s
    }
    if !strings.HasSuffix(s, "/") {
        s += "/"
    }
    return PathPrefix(s)
}
```

**When to use errors:**
- External I/O failures (file not found, network error)
- User input that can't be auto-corrected
- Truly exceptional conditions

**When to avoid errors:**
- Configuration that can be normalized
- Optional values (use nil/empty instead)
- Conditions you can prevent at construction time

---

### 4. Adding New Lua Primitives

**Process:**

1. **Identify the primitive capability**
   - Is this a general-purpose operation?
   - Or is it business logic that belongs in tenant code?

2. **Design the Go module (deep!)**
   ```go
   // internal/lua/modules/new_thing.go
   type NewThing struct {
       // Private fields (hidden complexity)
   }

   // Simple public methods
   func (t *NewThing) DoSomething() Result
   ```

3. **Expose via gopher-luar**
   ```go
   // internal/lua/chi_bindings.go
   L.SetGlobal("NewThing", luar.New(L, modules.NewThing()))
   ```

4. **Document in docs/lua.md**
   ```markdown
   ### NewThing

   ```lua
   local result = NewThing:DoSomething()
   ```

   Description of what it does...
   ```

5. **Create example in examples/scripts/**
   ```lua
   -- examples/scripts/new_thing_demo.lua
   function demo_handler(req)
       local result = NewThing:DoSomething()
       return {
           status = 200,
           body = result,
           headers = {["Content-Type"] = "text/plain"}
       }
   end
   ```

   And add to config:
   ```yaml
   routes:
     - method: "GET"
       pattern: "/demo"
       handler: "demo_handler"
   ```

**Checklist before committing:**
- [ ] Module has simple interface, complex implementation (deep)
- [ ] Used gopher-luar to avoid glue code
- [ ] Documented in docs/lua.md
- [ ] Example script created
- [ ] No business logic leaked into gateway core

---

### 5. Refactoring Guidelines

**When to refactor:**
- ✅ Code is duplicated in 3+ places
- ✅ Function is longer than 50 lines and doing multiple things
- ✅ You're adding a feature and current structure makes it hard
- ✅ Interface is confusing and you keep making mistakes

**When NOT to refactor:**
- ❌ "It could be prettier" (aesthetics alone)
- ❌ "We might need X in the future" (speculation)
- ❌ "I want to try pattern Y" (resume-driven development)
- ❌ Code works fine and changes are rare

**Refactoring process:**
1. Write a failing test for new behavior OR
2. Document current behavior with tests
3. Make the change
4. Verify tests still pass
5. Update documentation
6. Commit with clear message explaining WHY

**Red flags during refactoring:**
- Increasing number of interfaces/abstractions
- More layers added "for flexibility"
- Splitting one file into many small files
- Adding empty files for "future features"

**Good refactoring:**
- Consolidating duplicate code
- Extracting complex implementation behind simple interface
- Removing dead code
- Simplifying confusing interfaces

---

### 6. Code Review Checklist

Before submitting PR:

**Architecture:**
- [ ] Does this follow the "dumb gateway, smart tenant" principle?
- [ ] Are new modules deep (simple interface, complex implementation)?
- [ ] Is complexity pulled downward (in Go, not Lua)?
- [ ] Are we avoiding information leakage?

**Code Quality:**
- [ ] No empty files created "for the future"
- [ ] No pass-through variables (cfg passed through 3+ layers)
- [ ] Functions are <50 lines (or have good reason to be longer)
- [ ] Clear, obvious naming (no abbreviations unless standard)

**Lua Integration:**
- [ ] Using gopher-luar for new bindings (avoid manual glue code)
- [ ] Primitives only (no business logic in gateway)
- [ ] Properties preferred over methods where appropriate
- [ ] Documented in docs/lua.md

**Testing:**
- [ ] Happy path tested
- [ ] Error cases considered (or designed out)
- [ ] Example script provided if new feature

**Documentation:**
- [ ] MANIFEST.md updated if philosophy changed
- [ ] DESIGN.md updated if architecture changed
- [ ] docs/lua.md updated if Lua API changed
- [ ] Inline comments for complex algorithms
- [ ] No comments explaining obvious code

---

## Future Evolution

### Planned Features (Aligned with Design)

#### 1. Enhanced Lua Primitives
**Status:** Planned

**What:**
- WebSocket support in Lua
- Server-Sent Events (SSE) support
- Streaming response handling

**Why this fits:**
- ✅ General-purpose primitives (not business logic)
- ✅ Extends HTTP client capabilities
- ✅ Maintains deep module design

**Implementation sketch:**
```go
// internal/lua/modules/websocket.go
type WebSocket struct {
    conn *websocket.Conn
}

func (ws *WebSocket) Send(message string) error
func (ws *WebSocket) Receive() (string, error)
func (ws *WebSocket) Close() error
```

```lua
-- Tenant usage
function websocket_handler(req)
    local ws = WebSocket.Upgrade(req)
    while true do
        local msg = ws:Receive()
        ws:Send("Echo: " .. msg)
    end
end

-- In config.yaml:
routes:
  - method: "GET"
    pattern: "/ws"
    handler: "websocket_handler"
```

---

#### 2. Metrics and Observability
**Status:** Under consideration

**What:**
- Prometheus metrics exposure
- Request tracing integration
- Structured logging helpers

**Why this fits:**
- ✅ Infrastructure concern (gateway responsibility)
- ✅ Doesn't impose business logic
- ✅ Optional (can be disabled)

**Design principle:**
- Metrics are **opt-in** per tenant
- Tenants define **what** to measure (not how)
- Gateway handles collection/export (Prometheus, OpenTelemetry)

**Example:**
```lua
-- Tenant defines what to measure
function metrics_middleware(req, next)
    local start = os.clock()
    next()
    local duration = os.clock() - start

    -- Gateway primitive for metrics
    Metrics:RecordLatency("api_requests", duration, {
        method = req.method,
        path = req.path
    })
    return nil
end

-- In config.yaml:
routes:
  - method: "GET"
    pattern: "/api/*"
    handler: "api_handler"
    middleware:
      - "metrics_middleware"
```

---

#### 3. Request/Response Transformation Helpers
**Status:** Research phase

**What:**
- JSON parsing/generation in Lua
- XML parsing
- Base64 encoding/decoding
- URL encoding/decoding

**Why this fits:**
- ✅ General-purpose utilities
- ✅ Common need across tenants
- ✅ Better performance than pure Lua implementations

**Design decision:**
- Provide as optional Lua libraries (not core)
- Tenants `require` what they need
- No automatic inclusion (keeps core lean)

**Example:**
```lua
local JSON = require("json")  -- Gateway-provided utility

function process_data(req)
    local data = JSON.decode(req.body)
    data.processed = true
    return {
        status = 200,
        body = JSON.encode(data),
        headers = {["Content-Type"] = "application/json"}
    }
end

-- In config.yaml:
routes:
  - method: "POST"
    pattern: "/data"
    handler: "process_data"
```

---

### Features We Will NOT Add

#### ❌ Built-in Authentication
**Why not:**
- Every tenant has different auth needs (OAuth, mTLS, API keys, custom)
- Gateway would need to handle sessions, tokens, etc. (state)
- Violates "general-purpose" principle

**Alternative:**
- Tenants implement auth in Lua using HTTP primitives
- Or use external auth service (AuthN/AuthZ gateway)

---

#### ❌ Rate Limiting in Core
**Why not:**
- Rate limiting strategies vary (per IP, per user, per tenant, sliding window, token bucket)
- Requires state (counters, timestamps)
- Not a primitive HTTP operation

**Alternative:**
- Tenants implement rate limiting in Lua
- Or use external service (Redis-based rate limiter)
- Gateway provides primitives (timers, storage access if needed)

---

#### ❌ Service Discovery Integration
**Why not:**
- Too many service discovery systems (Consul, etcd, Kubernetes, DNS)
- Gateway would need to support all or pick one (opinion)
- Configuration via YAML works fine for most cases

**Alternative:**
- Use external tool to generate YAML config
- Kubernetes can template config via ConfigMaps
- Simple cron job to regenerate config from discovery system

---

#### ❌ GraphQL Gateway
**Why not:**
- GraphQL is a specific protocol, not a primitive
- Requires schema management, query parsing, validation
- Many good GraphQL gateways exist already

**Alternative:**
- Tenants can proxy to GraphQL backend
- Or implement GraphQL handling in Lua if needed
- Gateway provides HTTP primitives, tenants compose

---

### Evolution Guidelines

**When considering new features, ask:**

1. **Is this a primitive or an opinion?**
   - Primitive: HTTP client, WebSocket, file I/O → ✅ Consider
   - Opinion: OAuth, rate limiting, auth → ❌ Leave to tenants

2. **Does this make the interface simpler or more complex?**
   - Simpler: Consolidates existing complexity → ✅ Good
   - More complex: Adds new concepts to learn → ❌ Bad

3. **Is this general-purpose or special-purpose?**
   - General: Works for 80%+ of use cases → ✅ Good
   - Special: Solves one specific problem → ❌ Tenant code

4. **Can this be a library instead of core?**
   - If yes → ✅ Make it a library (require-able)
   - If no → Maybe belongs in core

5. **Does this hide complexity or expose it?**
   - Hides: Deep module with simple interface → ✅ Good
   - Exposes: Many functions, config options → ❌ Reconsider

**Process for adding features:**
1. Prototype as Lua library first (tenant code)
2. If useful across tenants → Move to gateway-provided library
3. If essential and can't be library → Add to core
4. Update MANIFEST.md with rationale

---

## Principles Checklist

Before writing any code, ask:

- [ ] **Deep modules**: Does this have a simple interface hiding complex implementation?
- [ ] **Information hiding**: Are implementation details hidden from users?
- [ ] **Pull complexity down**: Is complexity in Go, not Lua?
- [ ] **General-purpose**: Does this work for many use cases, not just one?
- [ ] **Define errors out**: Can I prevent this error at construction time?
- [ ] **Gateway is dumb**: Am I adding primitives, not opinions?
- [ ] **Obvious code**: Is it clear what this does without comments?

**Remember:**
> "Working code isn't enough. The goal is not just to make something work, but to create a system that is simple and obvious."
> — John Ousterhout

---

## Further Reading

- [DESIGN.md](DESIGN.md) - Technical architecture and implementation details
- [ROADMAP.md](ROADMAP.md) - Evolution from v1.0.0 to v4.0.0
- [CHANGELOG.md](CHANGELOG.md) - Detailed version history
- ["A Philosophy of Software Design" by John Ousterhout](https://web.stanford.edu/~ouster/cgi-bin/book.php) - The book that inspired our principles
