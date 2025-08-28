# Keystone Gateway: Project Analysis & Clean Slate Blueprint

## 1) Problem + Users

### Core Problem & Vision
- **Primary**: Multi-tenant reverse proxy with embedded scripting capability
- **Vision**: Basic gateway/reverse proxy written in Go, configured with YAML, with advanced business logic and routing possible through Lua scripts
- **Goal**: Easy to use and powerful gateway in the right hands - out of the box a solid foundation with extensive extensibility via simple Lua scripts
- **Key Value**: Route requests dynamically using Lua scripts without recompilation
- **Differentiator**: One binary, YAML config + Lua scripts (no complex setup/dependencies)

### Users/Clients
- **Internal services**: Backend microservices (API services, user services, payment services)
- **External traffic**: Web browsers, mobile apps, partner APIs
- **Multi-tenant**: Different domains/paths route to different backend clusters
- **Admin users**: Health monitoring via `/admin/health` and `/admin/tenants`

### Hello World Success Metric
- Single request `curl http://localhost:8080/admin/health` returns JSON with tenant status
- Lua script route `GET /hello` returns "Hello World" 
- Load balancing across multiple backends works automatically

## 2) Traffic + Scale

### Current Capacity (from code analysis)
- **Architecture**: Single Go binary, HTTP/2 support, connection pooling
- **RPS**: No explicit limits found, benchmarks focus on internal processing speed
- **Expected scale**: **UNKNOWN** - needs user input for production planning
- **Payload patterns**: JSON APIs (compression enabled for text/html/css/json)
- **Latency SLO**: **UNKNOWN** - default 60s request timeout, needs realistic target

### Performance Features
- Thread-safe Lua state pools
- HTTP/2 support
- Connection pooling 
- Gzip compression (configurable levels 1-9, default 5)
- Health checks (15-120s intervals)

## 3) Protocols + Features

### Inbound Protocols
- **HTTP/1.1, HTTP/2**: ✅ Supported
- **HTTP/3/QUIC**: ❌ Not implemented 
- **TLS**: ✅ Optional TLS termination (cert/key files)

### Outbound to Upstreams  
- **HTTP**: ✅ Primary protocol to backends
- **gRPC**: ❌ Not mentioned
- **Health checks**: ✅ HTTP health endpoints

### Gateway Features Implemented
- ✅ **Routing**: Host-based, path-based, hybrid routing
- ✅ **Load balancing**: Round-robin with health checks
- ✅ **Health checks**: Configurable intervals (15-120s)
- ✅ **Circuit breaking**: Half-open state, configurable thresholds
- ✅ **Request/response transforms**: Via Lua scripting
- ✅ **Compression**: Gzip for JSON/HTML/text
- ✅ **Multi-tenant**: Domain + path-based tenant isolation
- ✅ **Graceful shutdown**: SIGTERM handling
- ⚠️  **Rate limiting**: Example Lua script only (in-memory, not production-ready)
- ⚠️  **Auth**: Example Lua middleware, no built-in OAuth/JWT
- ❌ **Caching**: Not implemented
- ❌ **Websockets**: Not mentioned
- ❌ **mTLS**: TLS termination only, no mutual TLS
- ❌ **Retries**: Not implemented
- ❌ **Canary/blue-green**: Not implemented

## 4) Security + Compliance

### Current Security
- **TLS termination**: ✅ Optional (cert/key files)
- **Auth source**: ⚠️ Lua middleware examples only, no built-in IdP integration
- **Compliance**: **UNKNOWN** - no mention of GDPR, audit logging, WAF
- **Headers**: Basic security headers via Lua middleware possible

### Security Gaps
- No built-in authentication/authorization
- No WAF capabilities
- No audit logging
- No IP allow/deny lists
- No HSTS/CSP header enforcement

## 5) Upstream Services

### Service Discovery
- **Static config**: ✅ YAML-based service URLs
- **Dynamic discovery**: ❌ No Consul/K8s/AWS integration
- **Current scale**: 3-service clusters in production example
- **Protocol consistency**: HTTP-only to backends

### Load Balancing
- **Strategy**: Round-robin only
- **Health checks**: HTTP endpoint polling
- **Circuit breaking**: Basic implementation with half-open state

## 6) Deployment Environment

### Current Setup
- **Runtime**: Single Go binary or Docker (Docker preferred)
- **Platform**: **USER CONFIRMED** - Docker preferred, single binary fallback
- **Config**: YAML files + Lua scripts directory
- **CI/CD**: GitHub Actions with test coverage and releases

### Environment Examples
- `development.yaml`: Local development, longer health intervals
- `staging.yaml`: Staging environment setup  
- `production.yaml`: Multi-service production cluster
- `production-high-load.yaml`: High-performance production

## 7) Observability

### Current Logging
- **Format**: **USER CONFIRMED** - Structured JSON logging via slog
- **Content**: Request IDs, component tagging, health status
- **Metrics**: **UNKNOWN** - no Prometheus/OTel integration found
- **Tracing**: **UNKNOWN** - no distributed tracing

### Monitoring Gaps
- No metrics collection (Prometheus/OTel)
- No distributed tracing
- No SLI/SLO monitoring
- Basic health checks only

## 8) Constraints

### Project Context
- **Team size**: **USER CONFIRMED** - Solo developer
- **Timeline**: **USER CONFIRMED** - 1-2 months for v1
- **Origin**: **USER CONFIRMED** - Started as idea, evolved organically without upfront planning
- **Goal**: Extract lessons learned for clean slate rebuild
- **Must-use tech**: Go (confirmed), Chi router, Lua scripting

### Technical Constraints
- Keep it simple (philosophy: "One binary, YAML config, Lua scripts")
- No external dependencies beyond Go stdlib + 3 modules
- Docker-first deployment strategy

## 9) Non-Functionals  

### Current Targets
- **Availability**: Basic health checks, graceful shutdown
- **Configuration**: Hot-reload not implemented (requires restart)
- **Data durability**: Stateless (no persistent data)
- **Performance**: HTTP/2, connection pooling, Lua state pools

### Missing SLOs
- **Availability target**: **UNKNOWN** (no specific target defined)
- **Latency targets**: **UNKNOWN** (60s timeout is too high for production)
- **Error rate**: **UNKNOWN** (no SLI monitoring)

## 10) Nice-to-Haves vs V1 Core

### V1 Core (Already Implemented + Target)
- Multi-tenant routing (host + path based)
- Lua scripting for static route definition
- Basic load balancing + health checks
- Docker deployment
- Structured logging
- TLS termination
- **Hot reload for static routes**: File-based script reloading without restart

### V2 Candidates (Current Gaps + Runtime Enhancement)
- **Dynamic Runtime API**: Lua functions to add/remove routes during runtime
- **Runtime Route Introspection**: Query/modify existing routes from Lua
- **Service Discovery**: Consul/K8s integration
- **Advanced Auth**: OAuth2/JWT/OIDC built-in
- **Observability**: Prometheus metrics, distributed tracing
- **Advanced LB**: Weighted, least-connections algorithms  
- **Caching**: Response caching with TTL
- **Rate Limiting**: Production-ready (Redis-backed)
- **Security**: WAF, IP filtering, audit logging
- **Reliability**: Retries, timeouts, canary deployments

## Project Philosophy & Vision

**Core Philosophy**: Keep it simple. Get it working. Make it fast.

**Product Vision**: 
- **Foundation Layer**: Solid Go-based reverse proxy/gateway configured via YAML
- **Extensibility Layer**: Lua bindings to Chi router making advanced routing easily applicable in Lua scripts
- **User Experience**: Easy to use out-of-the-box, powerful in the right hands
- **Deployment**: One binary, YAML config, Lua scripts. No external dependencies, no complex setup, no microservice hell
- **Scalability**: Chi router functionality exposed through simple, accessible Lua bindings

**Target Users**:
- **Beginners**: Get a working gateway with basic YAML configuration
- **Power Users**: Access full Chi router capabilities (routes, middleware, groups) through simple Lua bindings
- **Operations Teams**: Simple deployment model with powerful observability hooks

## Key Insights for Clean Slate

### What Worked Well
1. **Simplicity**: Single binary + YAML + Lua is powerful and simple
2. **Multi-tenancy**: Host + path routing covers most use cases
3. **Lua Integration**: Flexible routing without recompilation
4. **Performance**: HTTP/2, connection pooling, good foundations
5. **Docker-First**: Deployment model is solid

### What Needs Better Planning
1. **Observability Strategy**: Metrics, tracing, SLI monitoring from day 1
2. **Security Architecture**: Built-in auth, not just Lua examples
3. **Service Discovery**: Static config doesn't scale beyond small clusters
4. **Rate Limiting**: In-memory approach won't work in production
5. **Hot Reload Implementation**: Complete the fsnotify integration for V1
6. **SLO Definition**: Need concrete latency/availability targets
7. **V2 Runtime API Design**: Plan dynamic routing boundaries early

### Architecture Strengths to Preserve
- Go + Chi router performance
- Lua scripting flexibility  
- Multi-tenant isolation
- Health check automation
- Circuit breaker pattern
- Graceful shutdown handling

### Technical Debt to Address
#### V1 Priority
- **Hot reload completion**: Finish fsnotify integration for static route updates
- Static service discovery limits scalability  
- No production-ready rate limiting
- Missing observability integration
- Security is mostly examples, not built-in

#### V2 Architectural Evolution
- **Dynamic routing boundaries**: Define scope of runtime route modifications
- **Route state management**: How to track/audit runtime route changes
- **Runtime safety**: Prevent Lua scripts from breaking live traffic
- **Performance impact**: Ensure dynamic routing doesn't hurt static route performance