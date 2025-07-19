// Package main implements the Keystone Gateway chi-stone binary.
// This is the main entry point for the gateway service.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

// Constants for the application
const (
	// Default timeouts
	DefaultHealthCheckInterval = 10 * time.Second
	DefaultHealthCheckTimeout  = 3 * time.Second
	DefaultRequestTimeout      = 60 * time.Second

	// Default server settings
	DefaultListenAddress = ":8080"

	// Version
	Version = "1.2.1"
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

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) *Application {
	return &Application{
		gateway: routing.NewGateway(cfg),
	}
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

	proxy := app.createProxy(backend, stripPrefix)
	proxy.ServeHTTP(w, r)
}

// createProxy creates a reverse proxy for the given backend
func (app *Application) createProxy(backend *routing.GatewayBackend, stripPrefix string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = backend.URL.Scheme
		req.URL.Host = backend.URL.Host

		if stripPrefix != "" {
			newPath := strings.TrimPrefix(req.URL.Path, stripPrefix)
			if newPath == "" {
				newPath = "/"
			}
			req.URL.Path = newPath
		}

		// Merge query parameters
		if backend.URL.RawQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = backend.URL.RawQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = backend.URL.RawQuery + "&" + req.URL.RawQuery
		}
	}

	return proxy
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
	// Check if we should use Lua-based routing
	if app.luaEngine != nil {
		app.setupLuaBasedRouting(r)
		return
	}

	// ...removed legacy static routing fallback...
}

// setupLuaBasedRouting sets up routing using Lua scripts
func (app *Application) setupLuaBasedRouting(r *chi.Mux) {
	cfg := app.gateway.GetConfig()

	for _, tenant := range cfg.Tenants {
		// Execute Lua route scripts if specified
		if tenant.LuaRoutes != "" {
			log.Printf("Executing Lua route script '%s' for tenant: %s", tenant.LuaRoutes, tenant.Name)
			if err := app.luaEngine.ExecuteRouteScript(tenant.LuaRoutes, tenant.Name); err != nil {
				log.Printf("Failed to execute Lua route script '%s' for tenant %s: %v", tenant.LuaRoutes, tenant.Name, err)
			}
		}
	}

	// Add catch-all handlers for fallback
	r.HandleFunc("/", app.ProxyHandler)
	r.HandleFunc("/*", app.ProxyHandler)
}

// setupStaticRouting sets up the original static routing for all tenants
func (app *Application) setupStaticRouting(r *chi.Mux) {
	cfg := app.gateway.GetConfig()
	for _, tenant := range cfg.Tenants {
		router := app.gateway.GetTenantRouter(tenant.Name)
		if router == nil {
			continue
		}

		if len(tenant.Domains) > 0 && tenant.PathPrefix != "" {
			// Hybrid routing
			r.Route(tenant.PathPrefix, func(r chi.Router) {
				r.Use(app.gateway.HostMiddleware(tenant.Domains))
				r.Use(app.gateway.ProxyMiddleware(router, tenant.PathPrefix))
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		} else if len(tenant.Domains) > 0 {
			// Host-only routing
			r.Group(func(r chi.Router) {
				r.Use(app.gateway.HostMiddleware(tenant.Domains))
				r.Use(app.gateway.ProxyMiddleware(router, ""))
				r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		} else if tenant.PathPrefix != "" {
			// Path-only routing
			r.Route(tenant.PathPrefix, func(r chi.Router) {
				r.Use(app.gateway.ProxyMiddleware(router, tenant.PathPrefix))
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		}
	}

	// Catch-all handlers for tenant routing
	r.HandleFunc("/", app.ProxyHandler)
	r.HandleFunc("/*", app.ProxyHandler)
}

// setupStaticRoutingForTenant sets up static routing for a single tenant
func (app *Application) setupStaticRoutingForTenant(r *chi.Mux, tenant config.Tenant) {
	router := app.gateway.GetTenantRouter(tenant.Name)
	if router == nil {
		return
	}

	if len(tenant.Domains) > 0 && tenant.PathPrefix != "" {
		// Hybrid routing
		r.Route(tenant.PathPrefix, func(r chi.Router) {
			r.Use(app.gateway.HostMiddleware(tenant.Domains))
			r.Use(app.gateway.ProxyMiddleware(router, tenant.PathPrefix))
			r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				// Middleware handles everything
			})
		})
	} else if len(tenant.Domains) > 0 {
		// Host-only routing
		r.Group(func(r chi.Router) {
			r.Use(app.gateway.HostMiddleware(tenant.Domains))
			r.Use(app.gateway.ProxyMiddleware(router, ""))
			r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				// Middleware handles everything
			})
			r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				// Middleware handles everything
			})
		})
	} else if tenant.PathPrefix != "" {
		// Path-only routing
		r.Route(tenant.PathPrefix, func(r chi.Router) {
			r.Use(app.gateway.ProxyMiddleware(router, tenant.PathPrefix))
			r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				// Middleware handles everything
			})
		})
	}
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config")
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	flag.Parse()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Choose routing mode based on configuration
	var app *Application
	var router *chi.Mux

	if cfg.LuaRouting != nil && cfg.LuaRouting.Enabled {
		// Use embedded Lua routing
		router = chi.NewRouter()
		app = NewApplicationWithLuaRouting(cfg, router)
		app.setupBaseMiddleware(router)
		app.setupAdminRoutes(router)
		app.setupTenantRouting(router)
		log.Printf("Using embedded Lua routing mode")
	}

	log.Printf("Keystone Gateway v%s (Chi Router) listening on %s", Version, *addr)
	if err := http.ListenAndServe(*addr, router); err != nil {
		log.Fatal(err)
	}
}
