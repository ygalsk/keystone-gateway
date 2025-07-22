// Package main implements the Keystone Gateway chi-stone binary.
// This is the main entry point for the gateway service.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
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
	luaEngine *lua.Engine // New: embedded Lua engine for route definition
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
		log.Printf("Embedded Lua routing enabled with scripts directory: %s", scriptsDir)
	}

	return &Application{
		gateway:   gateway,
		luaEngine: luaEngine,
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
	r.Use(middleware.Timeout(DefaultRequestTimeout))

	// Add host-based routing middleware if we have host-based tenants
	if app.luaEngine != nil {
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

	// Fallback to catch-all handlers
	r.HandleFunc("/", app.ProxyHandler)
	r.HandleFunc("/*", app.ProxyHandler)
}

// setupLuaBasedRouting sets up routing using Lua scripts
func (app *Application) setupLuaBasedRouting(r *chi.Mux) {
	cfg := app.gateway.GetConfig()

	// Execute global Lua scripts first (applies to all tenants)
	if err := app.luaEngine.ExecuteGlobalScripts(); err != nil {
		log.Printf("Failed to execute global scripts: %v", err)
	}

	// Execute Lua route scripts for all tenants
	for _, tenant := range cfg.Tenants {
		if tenant.LuaRoutes != "" {
			log.Printf("Executing Lua route script '%s' for tenant: %s", tenant.LuaRoutes, tenant.Name)
			if err := app.luaEngine.ExecuteRouteScript(tenant.LuaRoutes, tenant.Name); err != nil {
				log.Printf("Failed to execute Lua route script '%s' for tenant %s: %v", tenant.LuaRoutes, tenant.Name, err)
				continue
			}
		}
	}

	// Mount path-based tenant routes only (host-based handled by middleware)
	for _, tenant := range cfg.Tenants {
		if tenant.LuaRoutes != "" {
			if len(tenant.Domains) > 0 {
				// Host-based routing: handled by middleware, just log
				log.Printf("Tenant %s configured for host-based routing with domains: %v", tenant.Name, tenant.Domains)
			} else if tenant.PathPrefix != "" {
				// Path-based routing: mount at specific path prefix
				log.Printf("Mounting tenant %s routes at path prefix: %s", tenant.Name, tenant.PathPrefix)
				if err := app.luaEngine.RouteRegistry().MountTenantRoutes(tenant.Name, tenant.PathPrefix); err != nil {
					log.Printf("Failed to mount routes for tenant %s: %v", tenant.Name, err)
				}
			} else {
				// Fallback: mount at root with warning about potential collisions
				log.Printf("Warning: Tenant %s has no path prefix or domains - mounting at root may cause route collisions", tenant.Name)
				if err := app.luaEngine.RouteRegistry().MountTenantRoutes(tenant.Name, "/"); err != nil {
					log.Printf("Failed to mount routes for tenant %s: %v", tenant.Name, err)
				}
			}
		}
	}

	// Add catch-all handlers for fallback
	r.HandleFunc("/", app.ProxyHandler)
	r.HandleFunc("/*", app.ProxyHandler)
}

// hostBasedRoutingMiddleware creates middleware for host-based tenant routing
func (app *Application) hostBasedRoutingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a host-based tenant request
			cfg := app.gateway.GetConfig()
			registry := app.luaEngine.RouteRegistry()

			for _, tenant := range cfg.Tenants {
				if len(tenant.Domains) > 0 {
					for _, domain := range tenant.Domains {
						if r.Host == domain || r.Host == domain+":"+app.port() {
							// Found matching host-based tenant
							submux := registry.GetTenantRoutes(tenant.Name)
							if submux != nil {
								log.Printf("Host-based routing: %s -> tenant %s", r.Host, tenant.Name)
								submux.ServeHTTP(w, r)
								return
							}
						}
					}
				}
			}

			// No host-based match, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// port extracts the port from the listen address (defaults to 8080)
func (app *Application) port() string {
	return "8080" // Default port - could be enhanced to parse from listen address
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config")
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	flag.Parse()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.LuaRouting == nil || !cfg.LuaRouting.Enabled {
		log.Fatal("Lua routing must be enabled")
	}

	// Create application with Lua routing
	router := chi.NewRouter()
	app := NewApplicationWithLuaRouting(cfg, router)
	router = app.SetupRouter()

	log.Printf("Keystone Gateway v%s (Chi Router) listening on %s", Version, *addr)

	// Start server with TLS support if configured
	if cfg.TLS != nil && cfg.TLS.Enabled {
		log.Printf("Starting server with TLS enabled")
		if err := http.ListenAndServeTLS(*addr, cfg.TLS.CertFile, cfg.TLS.KeyFile, router); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := http.ListenAndServe(*addr, router); err != nil {
			log.Fatal(err)
		}
	}
}
