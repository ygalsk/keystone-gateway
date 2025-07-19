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
	gateway *routing.Gateway
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) *Application {
	return &Application{
		gateway: routing.NewGateway(cfg),
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

// extractHost extracts the hostname from a host header (removing port if present)
func extractHost(hostHeader string) string {
	if colonIndex := strings.Index(hostHeader, ":"); colonIndex != -1 {
		return hostHeader[:colonIndex]
	}
	return hostHeader
}

// HostMiddleware validates that the request host matches one of the allowed domains
func (app *Application) HostMiddleware(domains []string) func(http.Handler) http.Handler {
	domainMap := make(map[string]bool, len(domains))
	for _, domain := range domains {
		domainMap[domain] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := extractHost(r.Host)
			if domainMap[host] {
				next.ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
		})
	}
}

// ProxyMiddleware handles proxying for a specific tenant router
func (app *Application) ProxyMiddleware(tr *routing.TenantRouter, stripPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backend := tr.NextBackend()
			if backend == nil {
				http.Error(w, "No backend available", http.StatusBadGateway)
				return
			}

			proxy := app.createProxy(backend, stripPrefix)
			proxy.ServeHTTP(w, r)
		})
	}
}

// SetupRouter configures and returns the main router
func (app *Application) SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Core middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(DefaultRequestTimeout))

	// Admin endpoints
	cfg := app.gateway.GetConfig()
	basePath := cfg.AdminBasePath
	if basePath == "" {
		basePath = "/"
	}

	r.Route(basePath, func(r chi.Router) {
		r.Get("/health", app.HealthHandler)
		r.Get("/tenants", app.TenantsHandler)
	})

	// Setup tenant routing
	app.setupTenantRouting(r)

	return r
}

// setupTenantRouting configures tenant-specific routing
func (app *Application) setupTenantRouting(r *chi.Mux) {
	cfg := app.gateway.GetConfig()
	for _, tenant := range cfg.Tenants {
		router := app.gateway.GetTenantRouter(tenant.Name)
		if router == nil {
			continue
		}

		if len(tenant.Domains) > 0 && tenant.PathPrefix != "" {
			// Hybrid routing
			r.Route(tenant.PathPrefix, func(r chi.Router) {
				r.Use(app.HostMiddleware(tenant.Domains))
				r.Use(app.ProxyMiddleware(router, tenant.PathPrefix))
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		} else if len(tenant.Domains) > 0 {
			// Host-only routing
			r.Group(func(r chi.Router) {
				r.Use(app.HostMiddleware(tenant.Domains))
				r.Use(app.ProxyMiddleware(router, ""))
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
				r.Use(app.ProxyMiddleware(router, tenant.PathPrefix))
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

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config")
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	flag.Parse()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	app := NewApplication(cfg)
	router := app.SetupRouter()

	log.Printf("Keystone Gateway v%s (Chi Router) listening on %s", Version, *addr)
	if err := http.ListenAndServe(*addr, router); err != nil {
		log.Fatal(err)
	}
}
