// Package gateway provides the core multi-tenant HTTP reverse proxy.
// It merges routing and initialization logic into a single deep module.
package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
	httputil2 "keystone-gateway/internal/http"
	"keystone-gateway/internal/lua"
)

// backend represents a simple backend server
type backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy
}

// Gateway is the main entry point for the reverse proxy.
// It handles all HTTP routing including both static tenant routes and Lua-scripted routes.
// Gateway uses path-based routing only. Domain-based routing should be handled by
// external infrastructure (reverse proxies, ingress controllers, load balancers).
type Gateway struct {
	config    *config.Config
	router    *chi.Mux
	backends  map[string]*backend
	transport *http.Transport
	luaEngine *lua.Engine
	mu        sync.RWMutex
}

// New creates and initializes a new Gateway instance.
// It sets up middleware, Lua routing (if configured), and tenant routes.
func New(cfg *config.Config, version string) (*Gateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if len(cfg.Tenants) == 0 {
		return nil, fmt.Errorf("no tenants configured")
	}

	router := chi.NewRouter()

	gw := &Gateway{
		config:    cfg,
		router:    router,
		backends:  make(map[string]*backend),
		transport: httputil2.CreateTransport(),
	}

	// Setup middleware FIRST
	gw.setupMiddleware()

	// Initialize Lua engine if enabled
	if cfg.LuaRouting.Enabled {
		scriptsDir := cfg.LuaRouting.ScriptsDir
		if scriptsDir == "" {
			scriptsDir = "./scripts"
		}
		gw.luaEngine = lua.NewEngine(scriptsDir, router, cfg.RequestLimits.MaxBodySize)
	}

	// Setup Lua routing (registers routes via Lua scripts)
	if gw.luaEngine != nil {
		gw.setupLuaRouting()
	}

	// Health check endpoint
	gw.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Finally, setup tenant proxy routes
	gw.setupTenantRoutes()

	return gw, nil
}

// Handler returns the HTTP handler for the gateway.
// This is the main entry point for all HTTP requests.
// All routing is path-based. For domain-based routing, use an external
// reverse proxy (Nginx, HAProxy) or ingress controller (Kubernetes).
func (gw *Gateway) Handler() http.Handler {
	return gw.router
}

// Stop performs graceful shutdown of the gateway.
// Currently a no-op but reserved for future cleanup (connection draining, etc.)
func (gw *Gateway) Stop() {
	// Placeholder for future cleanup
	// Could close connections, stop background workers, etc.
}

// setupMiddleware configures middleware based on config
func (gw *Gateway) setupMiddleware() {
	if gw.config.Middleware.RequestID {
		gw.router.Use(middleware.RequestID)
	}
	if gw.config.Middleware.RealIP {
		gw.router.Use(middleware.RealIP)
	}
	if gw.config.Middleware.Logging {
		gw.router.Use(middleware.Logger)
	}
	if gw.config.Middleware.Recovery {
		gw.router.Use(middleware.Recoverer)
	}
	if gw.config.Middleware.Timeout > 0 {
		gw.router.Use(middleware.Timeout(time.Duration(gw.config.Middleware.Timeout) * time.Second))
	}
	if gw.config.Middleware.Throttle > 0 {
		gw.router.Use(middleware.Throttle(gw.config.Middleware.Throttle))
	}

	// Request size limits
	gw.router.Use(middleware.RequestSize(gw.config.RequestLimits.MaxBodySize))

	// Compression
	if gw.config.Compression.Enabled {
		gw.router.Use(middleware.Compress(gw.config.Compression.Level, gw.config.Compression.ContentTypes...))
	}

	gw.router.Use(middleware.CleanPath)
	gw.router.Use(middleware.StripSlashes)
}

// setupLuaRouting executes Lua scripts to register routes
func (gw *Gateway) setupLuaRouting() {
	// Execute global Lua scripts first
	if len(gw.config.LuaRouting.GlobalScripts) > 0 {
		slog.Info("lua_global_scripts_starting", "count", len(gw.config.LuaRouting.GlobalScripts), "component", "lua")
		if err := gw.luaEngine.ExecuteGlobalScripts(); err != nil {
			slog.Error("lua_global_scripts_failed", "error", err, "component", "lua")
		} else {
			slog.Info("lua_global_scripts_completed", "component", "lua")
		}
	}

	// Execute tenant-specific Lua route scripts
	luaTenantsCount := 0
	for _, tenant := range gw.config.Tenants {
		if len(tenant.LuaRoutes) > 0 {
			luaTenantsCount++
			slog.Info("lua_tenant_routes_starting", "tenant", tenant.Name, "scripts", tenant.LuaRoutes, "count", len(tenant.LuaRoutes), "component", "lua")
			for _, script := range tenant.LuaRoutes {
				slog.Info("lua_tenant_script_executing", "tenant", tenant.Name, "script", script, "component", "lua")
				if err := gw.luaEngine.ExecuteRouteScript(script); err != nil {
					slog.Error("lua_tenant_script_failed", "tenant", tenant.Name, "script", script, "error", err, "component", "lua")
				} else {
					slog.Info("lua_tenant_script_completed", "tenant", tenant.Name, "script", script, "component", "lua")
				}
			}
			slog.Info("lua_tenant_routes_completed", "tenant", tenant.Name, "component", "lua")
		}
	}

	if luaTenantsCount > 0 {
		slog.Info("lua_routing_initialized", "tenants_with_lua", luaTenantsCount, "component", "lua")
	}
}

// setupTenantRoutes configures all tenant proxy routes
func (gw *Gateway) setupTenantRoutes() {
	for _, tenant := range gw.config.Tenants {
		if err := gw.setupSingleTenantRoutes(tenant); err != nil {
			slog.Error("tenant_setup_failed",
				"tenant", tenant.Name,
				"error", err,
				"component", "gateway")
			continue
		}

		if len(tenant.Services) > 0 {
			slog.Info("tenant_initialized",
				"tenant", tenant.Name,
				"backend", tenant.Services[0].URL,
				"component", "gateway")
		}
	}
}

// setupSingleTenantRoutes sets up routes for a specific tenant
// All routing is path-based using PathPrefix. Tenants must specify a PathPrefix.
func (gw *Gateway) setupSingleTenantRoutes(tenant config.Tenant) error {
	// Skip if tenant has no services (may be Lua-only tenant)
	if len(tenant.Services) == 0 {
		return nil
	}

	// Use first service
	svc := tenant.Services[0]
	if len(tenant.Services) > 1 {
		slog.Warn("tenant_multiple_services",
			"tenant", tenant.Name,
			"configured", len(tenant.Services),
			"using", svc.Name,
			"note", "Only first service is used. For load balancing, point to external LB.",
			"component", "gateway")
	}

	u, err := url.Parse(svc.URL)
	if err != nil {
		return fmt.Errorf("invalid service URL for tenant %s: %w", tenant.Name, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = gw.transport
	proxy.ErrorHandler = gw.proxyErrorHandler

	back := &backend{
		URL:   u,
		Proxy: proxy,
	}

	gw.mu.Lock()
	gw.backends[tenant.Name] = back
	gw.mu.Unlock()

	// Setup path-based routing
	handler := gw.createTenantHandler(tenant.Name)

	if tenant.PathPrefix != "" {
		// Path-based routing
		gw.router.HandleFunc(tenant.PathPrefix+"*", handler)
	} else {
		// Default catch-all route if no path prefix specified
		gw.router.HandleFunc("/*", handler)
	}

	return nil
}

// createTenantHandler creates an HTTP handler that proxies requests to the tenant's backend
func (gw *Gateway) createTenantHandler(tenantName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gw.mu.RLock()
		back := gw.backends[tenantName]
		gw.mu.RUnlock()

		if back == nil {
			http.Error(w, "No backend configured", http.StatusBadGateway)
			return
		}

		back.Proxy.ServeHTTP(w, r)
	}
}

// proxyErrorHandler handles proxy errors
func (gw *Gateway) proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("proxy_error", "error", err, "path", r.URL.Path)
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}
