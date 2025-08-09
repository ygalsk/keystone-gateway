// Package main implements the Keystone Gateway chi-stone binary.
// This is the main entry point for the gateway service.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

// Constants for the application
const (
	DefaultRequestTimeout = 60 * time.Second
	DefaultListenAddress  = ":8080"
	Version               = "1.2.1"
)

func init() {
	// Simple structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

// HealthStatus represents the current health status of the gateway and all tenants.
type HealthStatus struct {
	Status  string            `json:"status"`
	Tenants map[string]string `json:"tenants"`
	Uptime  string            `json:"uptime"`
	Version string            `json:"version"`
}

// Application holds the main application components
type Application struct {
	gateway   *routing.Gateway
	luaEngine *lua.Engine    // Embedded Lua engine for route definition
	config    *config.Config // Configuration for the application
}

// NewApplicationWithLuaRouting creates an application with embedded Lua routing
func NewApplicationWithLuaRouting(cfg *config.Config, router *chi.Mux) *Application {
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Initialize Lua engine if lua_routing is enabled
	var luaEngine *lua.Engine
	if cfg.LuaRouting != nil && cfg.LuaRouting.Enabled {
		scriptsDir := cfg.LuaRouting.ScriptsDir
		if scriptsDir == "" {
			scriptsDir = "./scripts"
		}
		luaEngine = lua.NewEngine(scriptsDir, router)
		slog.Info("lua_routing_enabled",
			"scripts_directory", scriptsDir,
			"component", "lua_engine")
	}

	return &Application{
		gateway:   gateway,
		luaEngine: luaEngine,
		config:    cfg,
	}
}

// HealthHandler handles health check requests
func (app *Application) HealthHandler(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:  "healthy",
		Tenants: make(map[string]string),
		Version: Version,
		Uptime:  time.Since(app.gateway.GetStartTime()).String(),
	}

	// Get tenant health status
	cfg := app.gateway.GetConfig()
	for _, tenant := range cfg.Tenants {
		if router := app.gateway.GetTenantRouter(tenant.Name); router != nil {
			healthyCount := 0
			for _, backend := range router.Backends {
				if backend.Alive.Load() {
					healthyCount++
				}
			}
			status.Tenants[tenant.Name] = fmt.Sprintf("%d/%d healthy", healthyCount, len(router.Backends))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode health status", http.StatusInternalServerError)
		return
	}
}

// TenantsHandler handles tenant listing requests
func (app *Application) TenantsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := app.gateway.GetConfig()
	if err := json.NewEncoder(w).Encode(cfg.Tenants); err != nil {
		http.Error(w, "Failed to encode tenants data", http.StatusInternalServerError)
		return
	}
}

// ProxyHandler handles proxy requests
func (app *Application) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	router, stripPrefix := app.gateway.MatchRoute(r.Host, r.URL.Path)
	if router == nil {
		http.NotFound(w, r)
		return
	}

	backend := router.NextBackend()
	if backend == nil {
		http.Error(w, "No backend available", http.StatusBadGateway)
		return
	}

	proxy := app.gateway.CreateProxy(backend, stripPrefix)
	proxy.ServeHTTP(w, r)
}

// SetupRouter configures and returns the main router
func (app *Application) SetupRouter() *chi.Mux {
	r := chi.NewRouter()
	app.setupBaseMiddleware(r)
	app.setupAdminRoutes(r)
	app.setupTenantRouting(r)
	return r
}

// setupBaseMiddleware configures the core middleware stack
func (app *Application) setupBaseMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)

	// Add compression middleware for better performance on text content
	compressionConfig := app.config.GetCompressionConfig()
	if compressionConfig.Enabled {
		r.Use(middleware.Compress(compressionConfig.Level, compressionConfig.ContentTypes...))
	}

	r.Use(middleware.Timeout(DefaultRequestTimeout))

	// Add host-based routing middleware if we have host-based tenants
	if app.hasHostBasedTenants() {
		r.Use(app.hostBasedRoutingMiddleware())
	}
}

// setupAdminRoutes configures the admin API endpoints
func (app *Application) setupAdminRoutes(r *chi.Mux) {
	cfg := app.gateway.GetConfig()
	basePath := cfg.AdminBasePath
	if basePath == "" {
		basePath = "/"
	}

	r.Route(basePath, func(r chi.Router) {
		r.Get("/health", app.HealthHandler)
		r.Get("/tenants", app.TenantsHandler)
	})
}

// setupTenantRouting configures tenant-specific routing
func (app *Application) setupTenantRouting(r *chi.Mux) {
	// Use Lua-based routing if available
	if app.luaEngine != nil {
		app.setupLuaBasedRouting(r)
		return
	}

	// Setup basic proxy routing for tenants without Lua
	app.setupBasicProxyRouting(r)
}

// setupBasicProxyRouting sets up basic proxy routing without Lua
func (app *Application) setupBasicProxyRouting(r *chi.Mux) {
	cfg := app.gateway.GetConfig()

	// Setup path-based tenant routes
	for _, tenant := range cfg.Tenants {
		if tenant.PathPrefix != "" {
			slog.Info("tenant_routing_setup",
				"tenant", tenant.Name,
				"path_prefix", tenant.PathPrefix,
				"routing_type", "path_based",
				"component", "routing")
			r.Handle(tenant.PathPrefix+"*", http.HandlerFunc(app.ProxyHandler))
		}
	}

	// Add catch-all handlers for fallback
	r.HandleFunc("/", app.ProxyHandler)
	r.HandleFunc("/*", app.ProxyHandler)
}

// setupLuaBasedRouting sets up routing using Lua scripts
func (app *Application) setupLuaBasedRouting(r *chi.Mux) {
	cfg := app.gateway.GetConfig()

	// Execute global Lua scripts first (applies to all tenants)
	if err := app.luaEngine.ExecuteGlobalScripts(); err != nil {
		slog.Error("global_scripts_failed",
			"error", err,
			"component", "lua_engine")
	}

	// Execute Lua route scripts for all tenants
	for _, tenant := range cfg.Tenants {
		if tenant.LuaRoutes != "" {
			slog.Info("lua_route_script_executing",
				"script", tenant.LuaRoutes,
				"tenant", tenant.Name,
				"component", "lua_engine")
			if err := app.luaEngine.ExecuteRouteScript(tenant.LuaRoutes, tenant.Name); err != nil {
				slog.Error("lua_route_script_failed",
					"script", tenant.LuaRoutes,
					"tenant", tenant.Name,
					"error", err,
					"component", "lua_engine")
				continue
			}
		}
	}

	// Mount path-based tenant routes only (host-based handled by middleware)
	for _, tenant := range cfg.Tenants {
		if tenant.LuaRoutes != "" {
			if len(tenant.Domains) > 0 {
				// Host-based routing: handled by middleware, just log
				slog.Info("tenant_host_routing_configured",
					"tenant", tenant.Name,
					"domains", tenant.Domains,
					"routing_type", "host_based",
					"component", "routing")
			} else if tenant.PathPrefix != "" {
				// Path-based routing: mount at specific path prefix
				slog.Info("tenant_path_routing_mounting",
					"tenant", tenant.Name,
					"path_prefix", tenant.PathPrefix,
					"routing_type", "path_based",
					"component", "routing")
				if err := app.luaEngine.RouteRegistry().MountTenantRoutes(tenant.Name, tenant.PathPrefix); err != nil {
					slog.Error("tenant_route_mount_failed",
						"tenant", tenant.Name,
						"path_prefix", tenant.PathPrefix,
						"error", err,
						"component", "routing")
				}
			} else {
				// Fallback: mount at root with warning about potential collisions
				slog.Warn("tenant_root_mount_warning",
					"tenant", tenant.Name,
					"message", "no path prefix or domains - mounting at root may cause route collisions",
					"component", "routing")
				if err := app.luaEngine.RouteRegistry().MountTenantRoutes(tenant.Name, "/"); err != nil {
					slog.Error("tenant_route_mount_failed",
						"tenant", tenant.Name,
						"path_prefix", "/",
						"error", err,
						"component", "routing")
				}
			}
		}
	}

	// Add catch-all handlers for fallback
	r.HandleFunc("/", app.ProxyHandler)
	r.HandleFunc("/*", app.ProxyHandler)
}

// hasHostBasedTenants checks if any tenants use host-based routing
func (app *Application) hasHostBasedTenants() bool {
	cfg := app.gateway.GetConfig()
	for _, tenant := range cfg.Tenants {
		if len(tenant.Domains) > 0 {
			return true
		}
	}
	return false
}

// hostBasedRoutingMiddleware creates middleware for host-based tenant routing
func (app *Application) hostBasedRoutingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg := app.gateway.GetConfig()

			// Check if this is a host-based tenant request
			for _, tenant := range cfg.Tenants {
				if len(tenant.Domains) > 0 {
					for _, domain := range tenant.Domains {
						if r.Host == domain || r.Host == domain+":"+app.port() {
							slog.Info("host_routing_match",
								"host", r.Host,
								"tenant", tenant.Name,
								"domain", domain,
								"component", "routing")

							// If we have Lua routing, use the registry
							if app.luaEngine != nil {
								registry := app.luaEngine.RouteRegistry()
								if submux := registry.GetTenantRoutes(tenant.Name); submux != nil {
									submux.ServeHTTP(w, r)
									return
								}
							}

							// Fallback to basic proxy handling
							app.ProxyHandler(w, r)
							return
						}
					}
				}
			}

			// No host-based match, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// port extracts the port from the configuration
func (app *Application) port() string {
	return app.config.GetPort()
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config")
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	flag.Parse()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("config_load_failed", "error", err, "path", *cfgPath, "component", "config")
		os.Exit(1)
	}

	// Allow gateway to run without Lua routing for pure proxying
	if cfg.LuaRouting != nil && cfg.LuaRouting.Enabled {
		slog.Info("mode_lua_enabled", "component", "config")
	} else {
		slog.Info("mode_pure_proxy", "component", "config")
	}

	// Create application (with or without Lua routing)
	router := chi.NewRouter()
	app := NewApplicationWithLuaRouting(cfg, router)
	router = app.SetupRouter()

	// Create HTTP server
	server := &http.Server{
		Addr:              *addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server_starting",
			"version", Version,
			"address", *addr,
			"router", "chi",
			"component", "server")

		// Start server with TLS support if configured
		if cfg.TLS != nil && cfg.TLS.Enabled {
			slog.Info("tls_enabled",
				"cert_file", cfg.TLS.CertFile,
				"key_file", cfg.TLS.KeyFile,
				"component", "server")
			if err := server.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				slog.Error("server_failed", "error", err, "component", "server")
				os.Exit(1)
			}
		} else {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("server_failed", "error", err, "component", "server")
				os.Exit(1)
			}
		}
	}()

	// Wait for shutdown signal
	<-stop
	slog.Info("shutdown_initiated", "component", "server")

	// Stop health checks first
	app.gateway.StopHealthChecks()

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server_shutdown_forced", "error", err, "component", "server")
	} else {
		slog.Info("server_shutdown_graceful", "component", "server")
	}
}
