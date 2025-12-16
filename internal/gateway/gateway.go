// Package gateway provides the core multi-tenant HTTP reverse proxy.
// Deep module: Simple interface, complex implementation hidden.
package gateway

import (
	"encoding/json"
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

// backend represents a backend server for proxying
type backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy
}

// Gateway is the main entry point for the reverse proxy.
// Deep module: Hides Chi routing complexity, Lua engine management, backend pooling.
type Gateway struct {
	config    *config.Config
	router    *chi.Mux
	backends  map[string]*backend
	transport *http.Transport
	luaEngine *lua.Engine
	mu        sync.RWMutex
}

// New creates and initializes a new Gateway instance.
// Sets up middleware, Lua engine (if enabled), and tenant routes.
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

	// Setup global middleware
	gw.setupMiddleware()

	// Initialize Lua engine if enabled
	if cfg.LuaRouting.Enabled {
		scriptsDir := cfg.LuaRouting.ScriptsDir
		if scriptsDir == "" {
			scriptsDir = "./scripts"
		}
		gw.luaEngine = lua.NewEngine(
			scriptsDir,
			cfg.LuaRouting.StatePoolSize,
			cfg.LuaRouting.ModulePaths,
			cfg.LuaRouting.ModuleCPaths,
		)

		// Execute global scripts (initialization)
		if len(cfg.LuaRouting.GlobalScripts) > 0 {
			slog.Info("lua_global_scripts_starting",
				"count", len(cfg.LuaRouting.GlobalScripts),
				"component", "lua")
			if err := gw.luaEngine.ExecuteGlobalScripts(cfg.LuaRouting.GlobalScripts); err != nil {
				slog.Error("lua_global_scripts_failed", "error", err, "component", "lua")
				return nil, fmt.Errorf("failed to execute global Lua scripts: %w", err)
			}
			slog.Info("lua_global_scripts_completed", "component", "lua")
		}
	}

	// Health check endpoint
	gw.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Lua pool stats endpoint (if Lua routing is enabled)
	if cfg.LuaRouting.Enabled && gw.luaEngine != nil {
		gw.router.Get("/debug/lua-pool", func(w http.ResponseWriter, r *http.Request) {
			stats := gw.luaEngine.Stats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
		})
	}

	// Setup tenant routes
	for _, tenant := range cfg.Tenants {
		if err := gw.setupTenantRoutes(tenant); err != nil {
			slog.Error("tenant_setup_failed",
				"tenant", tenant.Name,
				"error", err,
				"component", "gateway")
			return nil, fmt.Errorf("failed to setup tenant %s: %w", tenant.Name, err)
		}

		slog.Info("tenant_initialized",
			"tenant", tenant.Name,
			"routes", len(tenant.Routes),
			"route_groups", len(tenant.RouteGroups),
			"component", "gateway")
	}

	return gw, nil
}

// Handler returns the HTTP handler for the gateway.
func (gw *Gateway) Handler() http.Handler {
	return gw.router
}

// Stop performs graceful shutdown of the gateway.
func (gw *Gateway) Stop() {
	if gw.luaEngine != nil {
		gw.luaEngine.Close()
	}
}

// setupMiddleware configures global middleware
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

// setupTenantRoutes configures all routes for a tenant (Go-owned routing)
func (gw *Gateway) setupTenantRoutes(tenant config.Tenant) error {
	var tenantRouter chi.Router

	// Create sub-router for tenant with path prefix
	if tenant.PathPrefix != "" {
		tenantRouter = chi.NewRouter()
		gw.router.Mount(tenant.PathPrefix, tenantRouter)
	} else {
		tenantRouter = gw.router
	}

	// Setup explicit routes
	for _, route := range tenant.Routes {
		if err := gw.setupRoute(tenantRouter, tenant, route); err != nil {
			return fmt.Errorf("failed to setup route %s %s: %w", route.Method, route.Pattern, err)
		}
	}

	// Setup route groups
	for _, group := range tenant.RouteGroups {
		if err := gw.setupRouteGroup(tenantRouter, tenant, group); err != nil {
			return fmt.Errorf("failed to setup route group %s: %w", group.Pattern, err)
		}
	}

	// Setup error handlers
	if tenant.ErrorHandlers.NotFound != "" {
		tenantRouter.NotFound(gw.createLuaHandler(tenant.ErrorHandlers.NotFound))
	}
	if tenant.ErrorHandlers.MethodNotAllowed != "" {
		tenantRouter.MethodNotAllowed(gw.createLuaHandler(tenant.ErrorHandlers.MethodNotAllowed))
	}

	// DEPRECATED: Support old LuaRoutes for backward compatibility
	if len(tenant.LuaRoutes) > 0 {
		slog.Warn("deprecated_lua_routes",
			"tenant", tenant.Name,
			"message", "LuaRoutes is deprecated, use Routes with handler instead",
			"component", "gateway")
	}

	return nil
}

// setupRoute configures a single route
func (gw *Gateway) setupRoute(r chi.Router, tenant config.Tenant, route config.Route) error {
	// Build middleware chain
	var middlewares []func(http.Handler) http.Handler
	for _, mwName := range route.Middleware {
		mw := gw.createLuaMiddleware(mwName)
		middlewares = append(middlewares, mw)
	}

	// Create handler
	var handler http.Handler
	if route.Handler != "" {
		// Lua handler
		handler = gw.createLuaHandler(route.Handler)
	} else if route.Backend != "" {
		// Proxy to backend
		backend, err := gw.getBackend(tenant, route.Backend)
		if err != nil {
			return err
		}
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backend.Proxy.ServeHTTP(w, r)
		})
	} else {
		return fmt.Errorf("route must have either handler or backend")
	}

	// Register route with middleware
	if len(middlewares) > 0 {
		r.With(middlewares...).Method(route.Method, route.Pattern, handler)
	} else {
		r.Method(route.Method, route.Pattern, handler)
	}

	return nil
}

// setupRouteGroup configures a route group (Chi's Route pattern)
func (gw *Gateway) setupRouteGroup(r chi.Router, tenant config.Tenant, group config.RouteGroup) error {
	r.Route(group.Pattern, func(subRouter chi.Router) {
		// Apply group-level middleware
		for _, mwName := range group.Middleware {
			mw := gw.createLuaMiddleware(mwName)
			subRouter.Use(mw)
		}

		// Setup nested routes
		for _, route := range group.Routes {
			if err := gw.setupRoute(subRouter, tenant, route); err != nil {
				slog.Error("failed to setup group route",
					"group", group.Pattern,
					"route", route.Pattern,
					"error", err)
			}
		}
	})
	return nil
}

// createLuaHandler creates an HTTP handler that executes a Lua function
func (gw *Gateway) createLuaHandler(handlerName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if gw.luaEngine == nil {
			http.Error(w, "Lua engine not initialized", http.StatusInternalServerError)
			return
		}

		if err := gw.luaEngine.ExecuteHandler(handlerName, w, r); err != nil {
			slog.Error("lua_handler_error",
				"handler", handlerName,
				"error", err,
				"component", "lua")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// createLuaMiddleware creates middleware that executes a Lua function
func (gw *Gateway) createLuaMiddleware(middlewareName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if gw.luaEngine == nil {
				http.Error(w, "Lua engine not initialized", http.StatusInternalServerError)
				return
			}

			if err := gw.luaEngine.ExecuteMiddleware(middlewareName, w, r, next); err != nil {
				slog.Error("lua_middleware_error",
					"middleware", middlewareName,
					"error", err,
					"component", "lua")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		})
	}
}

// getBackend retrieves or creates a backend for proxying
func (gw *Gateway) getBackend(tenant config.Tenant, backendName string) (*backend, error) {
	// Check if backend already exists
	gw.mu.RLock()
	if back, ok := gw.backends[backendName]; ok {
		gw.mu.RUnlock()
		return back, nil
	}
	gw.mu.RUnlock()

	// Find service in tenant config
	var svc *config.Service
	for i := range tenant.Services {
		if tenant.Services[i].Name == backendName {
			svc = &tenant.Services[i]
			break
		}
	}

	if svc == nil {
		return nil, fmt.Errorf("backend service not found: %s", backendName)
	}

	// Parse URL and create proxy
	u, err := url.Parse(svc.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid service URL for %s: %w", backendName, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = gw.transport
	proxy.ErrorHandler = gw.proxyErrorHandler

	back := &backend{
		URL:   u,
		Proxy: proxy,
	}

	// Store backend
	gw.mu.Lock()
	gw.backends[backendName] = back
	gw.mu.Unlock()

	slog.Info("backend_created",
		"name", backendName,
		"url", svc.URL,
		"component", "gateway")

	return back, nil
}

// proxyErrorHandler handles proxy errors
func (gw *Gateway) proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("proxy_error",
		"error", err,
		"path", r.URL.Path,
		"component", "gateway")
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}
