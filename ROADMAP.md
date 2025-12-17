# Keystone Gateway - Roadmap

**Purpose:** This document chronicles the evolution of Keystone Gateway from its initial experimental releases to the current stable v1.0.0, explaining the decisions, refactorings, and lessons learned along the way.

For philosophy and principles, see [MANIFEST.md](MANIFEST.md).
For technical architecture, see [DESIGN.md](DESIGN.md).

---

## Version Reset (December 2025)

**Note:** This project was reset to v1.0.0 in December 2025. The journey described below represents the **experimental phase** (archived as `archive/v1.0.0-experimental` through `archive/v4.0.0-experimental`). The current v1.0.0 represents the **first stable release**, incorporating all the lessons learned during this evolution.

What was originally planned as v5.0.0 is now v1.0.0 - reflecting that this is truly the first production-ready release.

---

## The Journey: Experimental Phase (July 2025 - December 2025)

**Theme:** From complex to simple. From opinionated to primitive. From stateful to stateless.

---

## Timeline Overview

```
July 2025        September 2025       December 2025
    |                    |                    |
 archive/v1.x        archive/v2.x    archive/v3.x → archive/v4.x → v1.0.0
    ↓                    ↓                    ↓
Initial Release    Performance       The Great Simplification
```

---

## Version History

### v1.0.0 (July 18, 2025) - Birth of Keystone

**What we built:**
- Multi-tenant reverse proxy with Go
- Path-based routing for tenant isolation
- Chi router for HTTP handling
- Lua scripting for custom routes and middleware
- Built-in health checking and load balancing
- Docker support with hardened Alpine image

**Architecture decisions:**
- Health checking: Gateway actively monitors backends, marks them healthy/unhealthy
- Load balancing: Round-robin across multiple backends per tenant
- Routing: Path-based only (e.g., `/api/*`, `/admin/*`)
- Lua: Embedded scripting for tenant-specific logic

**Why these choices:**
At launch, we thought the gateway should "own" reliability concerns like health checking and load balancing. This seemed like a complete, batteries-included solution.

**What we learned:**
This initial vision was functional but contained the seeds of complexity we'd later address.

---

### v1.1.0 (July 18, 2025) - Multi-Tenant Routing

**What changed:**
- Added multi-tenant support with host and header detection
- Improved tenant isolation

**Impact:**
- Foundation for true multi-tenancy
- First steps toward flexible routing

---

### v1.2.0 (July 31, 2025) - Host-Based Routing

**What changed:**
- Added domain-based routing using `hostrouter` library
- Supported `domains` field in tenant configuration
- Hybrid routing: host + path combination
- Routing priority: hybrid > host-only > path-only

**Why we added it:**
Users wanted to route by domain (`api.example.com`, `admin.example.com`) without path prefixes. Host-based routing seemed like the natural evolution.

**Configuration example:**
```yaml
tenants:
  - name: "api"
    domains: ["api.example.com"]
    services:
      - url: "http://backend:3000"
```

**What we learned later:**
This introduced dual routing complexity. The `Handler()` method returned either `gw.router` OR `gw.hostRouter` depending on tenant config. Lua routes registered on `gw.router` became unreachable when host routing was enabled. This bug would haunt us until v5.0.0.

---

### v1.3.0 (July 31, 2025) - Project Structure

**What changed:**
- Adopted Go 2025 best practices
- Added `pkg/` directory for reusable packages
- Enhanced `configs/` organization
- Improved `scripts/lua/` structure

**Impact:**
- Better code organization
- Standard Go project layout

---

### v1.4.0 (August 9, 2025) - KISS/DRY Refactoring

**What changed:**
- Consolidated Lua-stone service into single binary
- Redesigned middleware system
- Major KISS (Keep It Simple, Stupid) / DRY (Don't Repeat Yourself) refactoring
- Added comprehensive test suite with fixture-based architecture
- Created Lua scripting examples

**Why we did it:**
The codebase had grown organically with duplication and complexity. This was the first major simplification pass.

**Impact:**
- Simpler deployment (single binary)
- Cleaner middleware architecture
- Better test coverage

**Lesson learned:**
Simplification isn't a one-time event. It's an ongoing discipline.

---

### v2.0.0 (September 17, 2025) - Performance Breakthrough

**BREAKING CHANGES**

**What changed:**
- Unified compiler cache for all Lua scripts
- Bytecode compilation: 50-70% memory reduction (per gopher-lua docs)
- HTTP/2 support with optimized timeouts
- Added `http_get()` and `http_post()` functions with context propagation
- Request limits with `max_body_size` enforcement
- Immutable context caching in Lua bindings

**Breaking Lua API changes:**
```lua
-- Old (v1.x)
response:write("Hello")
response:header("Content-Type", "text/plain")

-- New (v2.0.0)
response_write("Hello")
response_header("Content-Type", "text/plain")
```

**Why we did it:**
Performance was becoming a concern. Lua scripts were compiled on every request. Memory usage was higher than necessary.

**Impact:**
- **Massive performance improvement**: Bytecode compilation once at startup, execute many times
- **Memory reduction**: 50-70% less memory per script
- **HTTP/2 support**: Modern protocol support
- **Breaking change**: Users had to update Lua scripts

**Lesson learned:**
Breaking changes are acceptable when they provide significant value. But we needed better migration guidance (which we improved in later versions).

---

### v3.0.0 (December 2025) - The Deep Modules Refactoring

**Commit:** `287bc80` - "use gopher-luar for automatic type conversion and make lua modules DEEP"

**What changed:**
- Adopted `gopher-luar` library for automatic Go ↔ Lua type conversion
- Created `internal/lua/modules/` directory with deep modules:
  - `request.go` - Request wrapper with caching, Chi URL params, context management
  - `response.go` - Response writer with proper headers and status codes
  - `http.go` - HTTP client with connection pooling, HTTP/2, timeout management
- **90% code reduction**: `chi_bindings.go` from 598 lines → ~50 lines

**Lua API improvements:**
```lua
-- Properties instead of methods (more discoverable)
print(req.Method)  -- Instead of req:GetMethod()
print(req.URL)
print(req.Path)

-- Methods for dynamic access
local auth = req:Header("Authorization")
local body = req:Body()  -- Automatically cached
local id = req:Param("id")  -- Chi URL parameters

-- HTTP client (unchanged interface, better internals)
local resp = HTTP:Get("https://api.example.com/data", {
    Authorization = "Bearer token"
})
```

**Why we did it:**
The `chi_bindings.go` file had 598 lines of manual Lua binding code. Each function required:
- Type checking (is this actually a request?)
- Lua stack manipulation (push, pop, check types)
- Error handling (pcall wrapping, error messages)
- Boilerplate for every single method

This was **shallow module design** - many small functions, each exposing complexity.

**Implementation approach:**
1. Created deep Go modules (simple interface, complex implementation)
2. Used gopher-luar to automatically expose them to Lua
3. Eliminated 90% of glue code

**Impact:**
- **Maintainability**: 598 → 50 lines (92% reduction) in chi_bindings.go
- **Discoverability**: Lua API now matches Go struct properties
- **Deep modules**: Request, Response, HTTP all hide complexity behind simple interfaces
- **No performance cost**: gopher-luar uses reflection but gateway is I/O bound anyway

**Lesson learned:**
This was a **massive win**. It proved the "deep modules" philosophy from MANIFEST.md. The code became dramatically simpler without sacrificing functionality.

**Quote from commit:**
> "use gopher-luar for automatic type conversion and make lua modules DEEP"

This refactoring was inspired by John Ousterhout's "A Philosophy of Software Design" and became the template for all future modules.

---

### v4.0.0 (December 14, 2025) - Stateless Revolution

**BREAKING CHANGES**

**Commit:** `609d89a` - "feat(gateway)!: remove health checking, make gateway stateless"

**What changed:**
- Removed **all health checking code** (~150 lines)
- Removed health check goroutines, context management, state synchronization
- Changed from **multiple backends per tenant** to **single backend URL**
- Removed `Backend.Healthy` field
- Removed `HasHealthyBackends()` method
- Removed round-robin load balancing
- Simplified `/health` endpoint to basic liveness check (not backend health aggregation)

**Configuration changes:**
```yaml
# Before (v3.x) - Multiple backends with health checking
tenants:
  - name: "api"
    services:
      - name: "backend-1"
        url: "http://backend1:3000"
      - name: "backend-2"
        url: "http://backend2:3000"

# After (v4.0.0) - Single backend (point to external LB)
tenants:
  - name: "api"
    services:
      - name: "backend"
        url: "http://load-balancer:3000"  # External LB handles health/failover
```

**Why we removed health checking:**

This was a **fundamental architectural decision**. We realized:

1. **Stateful complexity**: Health checking required:
   - Goroutines per tenant to probe backends
   - Context management for graceful shutdown
   - Mutex-protected state (which backends are healthy)
   - Coordination across gateway instances

2. **Infrastructure concerns**: Health checking is an infrastructure responsibility:
   - HAProxy, Nginx, AWS ELB, K8s Ingress all provide superior health checking
   - They have more sophisticated algorithms (exponential backoff, circuit breakers)
   - They can make routing decisions based on real-time metrics

3. **Gateway is dumb principle**: From MANIFEST.md:
   > "The gateway is dumb. Tenants are smart."

   Health checking is **reliability infrastructure**, not a routing primitive.

4. **Cloud-native alignment**: Modern platforms provide load balancing with health checks:
   - Kubernetes Ingress Controllers
   - AWS Application Load Balancer
   - GCP Load Balancer
   - HAProxy, Nginx

5. **Horizontal scalability**: Stateless gateway = no state synchronization across instances

**Migration guide:**
Users should:
1. Deploy external load balancer (HAProxy, Nginx, AWS ELB, K8s Ingress)
2. Configure health checks in the load balancer
3. Point gateway tenant backends to the load balancer URL
4. Deploy multiple gateway instances for HA

**Alternative for simple deployments:**
- Implement custom health checking in Lua scripts if needed
- Use DNS-based load balancing with health checks

**Impact:**
- ✅ **Stateless**: Gateway can now scale horizontally without state
- ✅ **Simpler**: Removed ~150 lines of complex health checking code
- ✅ **Cloud-native**: Leverages platform capabilities
- ✅ **Single responsibility**: Gateway focuses on routing, infrastructure handles reliability
- ⚠️ **Breaking change**: Users must use external load balancers

**Lesson learned:**
This was **controversial but correct**. Some users initially resisted ("why remove a useful feature?"), but it:
- Dramatically simplified the codebase
- Aligned with cloud-native best practices
- Made the gateway easier to reason about
- Forced proper architectural separation of concerns

**References:**
- See DESIGN.md "Design Decisions Record" for full rationale
- See CHANGELOG.md v4.0.0 for migration guide

---

### Current v1.0.0 (formerly planned as v5.0.0) - Path-Only Routing

**BREAKING CHANGES**

**What changed:**
- Removed `hostrouter` dependency completely
- Removed `Domains []string` field from Tenant configuration
- Removed domain validation logic (`isValidDomain()` function)
- Simplified `Handler()` to always return `gw.router` (no more conditional routing)
- Gateway now uses **path-based routing only**
- `PathPrefix` is now optional (defaults to catch-all `/*`)

**Configuration changes:**
```yaml
# Before (v4.x) - Host-based routing
tenants:
  - name: "api"
    domains: ["api.example.com"]
    services:
      - url: "http://backend:3000"
  - name: "admin"
    domains: ["admin.example.com"]
    services:
      - url: "http://admin-backend:4000"

# After (v5.0.0) - Path-based routing with external reverse proxy
tenants:
  - name: "api"
    path_prefix: "/api"
    services:
      - url: "http://backend:3000"
  - name: "admin"
    path_prefix: "/admin"
    services:
      - url: "http://admin-backend:4000"
```

**External reverse proxy configuration:**

**Nginx example:**
```nginx
server {
    listen 80;
    server_name api.example.com;
    location / {
        proxy_pass http://gateway:8080/api;
    }
}

server {
    listen 80;
    server_name admin.example.com;
    location / {
        proxy_pass http://gateway:8080/admin;
    }
}
```

**Kubernetes Ingress example:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
spec:
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gateway
            port:
              number: 8080
```

**Why we removed host-based routing:**

1. **Dual routing complexity**: The gateway had two routing mechanisms:
   - `gw.router` (Chi router for path-based routing)
   - `gw.hostRouter` (hostrouter for domain-based routing)

   The `Handler()` method conditionally returned one or the other:
   ```go
   // Before - confusing!
   func (gw *Gateway) Handler() http.Handler {
       if gw.hostRouter != nil {
           return gw.hostRouter  // Some tenants
       }
       return gw.router  // Other tenants
   }
   ```

2. **Lua route bug**: Lua routes were registered on `gw.router` via Chi. When host-based routing was enabled, `Handler()` returned `gw.hostRouter` instead, making Lua routes **unreachable**. This was a subtle bug that confused users.

3. **Gateway is dumb principle**: Domain routing is an **infrastructure concern**:
   - Nginx, HAProxy, K8s Ingress are designed for domain routing
   - They have more features (TLS termination, domain wildcards, regex matching)
   - They're the **correct layer** for this responsibility

4. **Simpler mental model**: One router (`chi.Mux`), one routing mechanism, clear semantics

5. **Cloud-native alignment**: Like health checking (v4.0.0), this delegates infrastructure concerns to infrastructure layer

**Implementation:**
- Removed ~50 lines of domain routing code
- Simplified tenant validation: `PathPrefix` now optional (defaults to `/*`)
- Updated all tests to use path-based routing
- Fixed the Lua route reachability bug

**Impact:**
- ✅ **Simpler**: One router, one routing mechanism
- ✅ **Bug fix**: Lua routes now always work
- ✅ **Clearer**: No hidden router switching
- ✅ **Aligns with philosophy**: "Gateway is dumb"
- ⚠️ **Breaking change**: Users need external reverse proxy for domain routing

**Lesson learned:**
This completed the architectural simplification started in archive/v4.0.0:
- archive/v4.0.0: Removed health checking (stateless)
- Current v1.0.0: Removed host routing (path-only)

Both changes delegate infrastructure concerns to the infrastructure layer, making the gateway **purely a routing primitive**.

**References:**
- See DESIGN.md "Design Decisions Record" for full rationale
- See examples in `examples/nginx-ingress.conf` for migration

---

## Key Achievements

### Code Reduction
```
Initial (archive/v1.0.0):  ~3,500 lines
Current (v1.0.0):          ~1,500 lines
Reduction:                 ~57% smaller codebase
```

### Module Simplification
```
chi_bindings.go:   598 → 50 lines (92% reduction)
Health checking:   ~150 lines removed
Host routing:      ~50 lines removed
Total removed:     ~900 lines
```

### Architecture Evolution
```
archive/v1.0.0:  Stateful, opinionated, complex
         ↓
archive/v2.0.0:  Performance optimized
         ↓
archive/v3.0.0:  Deep modules, clean APIs
         ↓
archive/v4.0.0:  Stateless, cloud-native
         ↓
v1.0.0:          Path-only, primitives-focused (first stable release)
```

### Current State (v1.0.0)
- **Stateless**: No backend state tracking, horizontal scaling
- **Simple**: One router, clear routing semantics
- **Lua scripting**: Powered by golua with LuaJIT for high performance
- **Cloud-native**: Delegates health checking and domain routing to infrastructure
- **Primitives-focused**: Core routing operations only, no opinions

---

## Lessons Learned

### What Worked ✅

#### 1. Deep Modules Pattern
**The win:** archive/v3.0.0 refactoring reduced chi_bindings.go by 92%.

**Why it worked:**
- Simple interfaces hide complex implementations
- Automatic binding eliminated manual glue code
- Lua API became more discoverable

**Takeaway:** Invest in proper module design upfront. The payoff is massive.

#### 2. Stateless Design
**The win:** archive/v4.0.0 removed health checking, making gateway horizontally scalable.

**Why it worked:**
- Aligned with cloud-native infrastructure patterns
- Removed ~150 lines of complex state management
- Simplified deployment and scaling

**Takeaway:** Push state management to external systems. Stateless services are simpler and more scalable.

#### 3. "Gateway is Dumb" Principle
**The win:** archive/v4.0.0 and current v1.0.0 removed opinionated features (health checking, host routing).

**Why it worked:**
- Forced clear separation of concerns
- Made gateway a **primitive** instead of a framework
- Users compose primitives into solutions

**Takeaway:** General-purpose tools are more useful than opinionated frameworks.

---

### What Didn't Work ❌

#### 1. Built-in Health Checking
**The problem:** Health checking added complexity and state management.

**Why it failed:**
- Stateful design prevented horizontal scaling
- External load balancers do this better
- Not a routing primitive

**Fix:** archive/v4.0.0 removed it entirely. Delegate to infrastructure.

#### 2. Host-Based Routing
**The problem:** Dual routing system (Chi + hostrouter) created complexity and bugs.

**Why it failed:**
- Two routers with different semantics
- Lua routes became unreachable with host routing enabled
- Infrastructure concern, not gateway responsibility

**Fix:** Current v1.0.0 removed it. Path-only routing with external reverse proxy for domains.

#### 3. Manual Lua Bindings
**The problem:** chi_bindings.go had 598 lines of repetitive glue code.

**Why it failed:**
- High maintenance burden (every API change = 20+ lines)
- Error-prone (easy to forget null checks, type checks)
- Shallow module design (many small functions)

**Fix:** archive/v3.0.0 refactored with better binding approach. 92% code reduction.

---

### Why Changes Matter

#### Cloud-Native Alignment
Modern platforms provide:
- **Load balancers** (health checking, failover, sophisticated routing)
- **Ingress controllers** (domain routing, TLS termination, path rewriting)
- **Service meshes** (mTLS, observability, traffic management)

Keystone Gateway now **complements** these tools instead of **competing** with them.

#### Simplicity Enables Understanding
From ~3,500 lines to ~1,500 lines. From complex state management to stateless primitives.

**Result:** New contributors can understand the codebase faster. Bugs are easier to find and fix.

#### Principles Over Features
We removed features (health checking, host routing) to **honor principles** (stateless, general-purpose, deep modules).

**Result:** A tool that does less but does it better.

---

## Evolution Philosophy

Looking back at this journey, several themes emerge:

### 1. Simplification is a Journey
- archive/v1.0.0: Built with good intentions
- archive/v1.4.0: First major refactoring (KISS/DRY)
- archive/v3.0.0: Deep modules refactoring (92% reduction)
- archive/v4.0.0: Removed health checking (stateless)
- Current v1.0.0: Removed host routing (primitives-focused)

**Lesson:** Simplification isn't a one-time event. It's continuous refinement.

### 2. Break Things to Make Progress
- archive/v2.0.0: Breaking Lua API changes for performance
- archive/v4.0.0: Removed health checking (users needed external LBs)
- Current v1.0.0: Removed host routing (users needed reverse proxy)

**Lesson:** Breaking changes are acceptable when they provide significant architectural improvement. Document migration paths clearly.

### 3. Principles Guide Decisions
Every major decision was guided by principles from MANIFEST.md:
- **Deep modules** → archive/v3.0.0 refactoring
- **Stateless** → archive/v4.0.0 health checking removal
- **General-purpose** → archive/v4.0.0 and current v1.0.0 infrastructure delegation
- **Gateway is dumb** → All removal decisions

**Lesson:** Explicit principles make tough decisions easier.

### 4. Less is More
We removed:
- ~900 lines of code
- Two major features (health checking, host routing)
- Complexity and state management

**Result:** A simpler, more focused tool that does one thing well.

---

## Looking Forward

For planned features and evolution guidelines, see [MANIFEST.md - Future Evolution](MANIFEST.md#future-evolution).

**Current focus:**
- Maintain simplicity and focus
- Add primitives (WebSocket, SSE) without opinions
- Continue cloud-native alignment
- Honor the "gateway is dumb" principle

---

## References

- [MANIFEST.md](MANIFEST.md) - Philosophy, principles, and guidelines
- [DESIGN.md](DESIGN.md) - Technical architecture and implementation
- [CHANGELOG.md](CHANGELOG.md) - Detailed version history and migration guides
- ["A Philosophy of Software Design" by John Ousterhout](https://web.stanford.edu/~ouster/cgi-bin/book.php) - Inspiration for our principles

---

**This roadmap is a living document. It will be updated as we continue evolving Keystone Gateway.**

Last updated: December 2025 (v1.0.0 - First Stable Release)
