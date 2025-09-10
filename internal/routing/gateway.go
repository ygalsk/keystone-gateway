// Package routing provides simplified routing logic for Keystone Gateway.
package routing

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"keystone-gateway/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/hostrouter"
)

// Backend represents a simple backend server
type Backend struct {
	URL     *url.URL
	Healthy bool
	Proxy   *httputil.ReverseProxy
}

// Gateway provides simplified routing using go-chi and standard library
type Gateway struct {
	config     *config.Config
	router     *chi.Mux
	hostRouter hostrouter.Routes
	backends   map[string][]*Backend
	healthCtx  context.Context
	healthStop context.CancelFunc
	healthWG   sync.WaitGroup
	mu         sync.RWMutex
}

// NewGateway creates a new simplified gateway
func NewGateway(cfg *config.Config) *Gateway {
	healthCtx, healthStop := context.WithCancel(context.Background())

	return &Gateway{
		config:     cfg,
		router:     chi.NewRouter(),
		hostRouter: hostrouter.New(),
		backends:   make(map[string][]*Backend),
		healthCtx:  healthCtx,
		healthStop: healthStop,
	}
}

// NewGatewayWithRouter creates a gateway with existing router
func NewGatewayWithRouter(cfg *config.Config, router *chi.Mux) *Gateway {
	healthCtx, healthStop := context.WithCancel(context.Background())

	gw := &Gateway{
		config:     cfg,
		router:     router,
		hostRouter: hostrouter.New(),
		backends:   make(map[string][]*Backend),
		healthCtx:  healthCtx,
		healthStop: healthStop,
	}

	gw.setupRoutes()
	gw.startHealthChecks()
	return gw
}

// setupRoutes configures all tenant routes
func (gw *Gateway) setupRoutes() {
	for _, tenant := range gw.config.Tenants {
		gw.setupTenantRoutes(tenant)
		slog.Info("tenant_initialized",
			"tenant", tenant.Name,
			"backend_count", len(tenant.Services),
			"component", "gateway")
	}
}

// setupTenantRoutes sets up routes for a specific tenant
func (gw *Gateway) setupTenantRoutes(tenant config.Tenant) {
	// Initialize backends for this tenant
	var backends []*Backend
	for _, svc := range tenant.Services {
		u, err := url.Parse(svc.URL)
		if err != nil {
			slog.Warn("invalid_service_url", "service", svc.Name, "url", svc.URL, "error", err)
			continue
		}

		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.ErrorHandler = gw.proxyErrorHandler

		backends = append(backends, &Backend{
			URL:     u,
			Healthy: true, // Start optimistic
			Proxy:   proxy,
		})
	}

	gw.mu.Lock()
	gw.backends[tenant.Name] = backends
	gw.mu.Unlock()

	// Setup routing based on tenant configuration
	handler := gw.createTenantHandler(tenant.Name)

	if len(tenant.Domains) > 0 {
		// Host-based routing
		for _, domain := range tenant.Domains {
			if tenant.PathPrefix != "" {
				// Host + path routing - mount router to hostrouter
				subrouter := chi.NewRouter()
				subrouter.HandleFunc(tenant.PathPrefix+"*", handler)
				gw.hostRouter[domain] = subrouter
			} else {
				// Host-only routing - create simple router
				subrouter := chi.NewRouter()
				subrouter.HandleFunc("/*", handler)
				gw.hostRouter[domain] = subrouter
			}
		}
	} else if tenant.PathPrefix != "" {
		// Path-only routing
		gw.router.HandleFunc(tenant.PathPrefix+"*", handler)
	}
}

// createTenantHandler creates a handler function for a tenant
func (gw *Gateway) createTenantHandler(tenantName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gw.mu.RLock()
		backends := gw.backends[tenantName]
		gw.mu.RUnlock()

		if len(backends) == 0 {
			http.Error(w, "No backends available", http.StatusBadGateway)
			return
		}

		// Simple round-robin (could be improved with better load balancing)
		backend := gw.selectHealthyBackend(backends)
		if backend == nil {
			http.Error(w, "No healthy backends", http.StatusBadGateway)
			return
		}

		backend.Proxy.ServeHTTP(w, r)
	}
}

// selectHealthyBackend picks the first healthy backend (simple strategy)
func (gw *Gateway) selectHealthyBackend(backends []*Backend) *Backend {
	for _, backend := range backends {
		if backend.Healthy {
			return backend
		}
	}
	return nil
}

// proxyErrorHandler handles proxy errors
func (gw *Gateway) proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("proxy_error", "error", err, "path", r.URL.Path)
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}

// Handler returns the main HTTP handler
func (gw *Gateway) Handler() http.Handler {
	if len(gw.config.Tenants) == 0 {
		return gw.router
	}

	// Check if any tenants use host-based routing
	hasHostRouting := false
	for _, tenant := range gw.config.Tenants {
		if len(tenant.Domains) > 0 {
			hasHostRouting = true
			break
		}
	}

	if hasHostRouting {
		return gw.hostRouter
	}

	return gw.router
}

// startHealthChecks starts health checking for all backends
func (gw *Gateway) startHealthChecks() {
	interval := 30 * time.Second

	for tenantName, backends := range gw.backends {
		for _, backend := range backends {
			gw.healthWG.Add(1)
			go gw.healthCheckWorker(tenantName, backend, interval)
		}

		slog.Info("health_checks_started",
			"tenant", tenantName,
			"backend_count", len(backends),
			"interval", interval,
			"component", "health_checker")
	}
}

// healthCheckWorker runs health checks for a single backend
func (gw *Gateway) healthCheckWorker(tenantName string, backend *Backend, interval time.Duration) {
	defer gw.healthWG.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-gw.healthCtx.Done():
			return
		case <-ticker.C:
			gw.checkBackendHealth(tenantName, backend)
		}
	}
}

// checkBackendHealth performs a health check on a backend
func (gw *Gateway) checkBackendHealth(tenantName string, backend *Backend) {
	healthURL := backend.URL.String() + "/health"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)

	healthy := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300

	if resp != nil {
		resp.Body.Close()
	}

	wasHealthy := backend.Healthy
	backend.Healthy = healthy

	if !healthy && wasHealthy {
		slog.Error("health_check_failed",
			"backend", backend.URL.String(),
			"error", err,
			"component", "health_checker")
	} else if healthy && !wasHealthy {
		slog.Info("health_check_recovered",
			"backend", backend.URL.String(),
			"component", "health_checker")
	}
}

// Stop stops all health checks and cleanup
func (gw *Gateway) Stop() {
	gw.healthStop()
	gw.healthWG.Wait()
}

// StopHealthChecks is an alias for Stop for backward compatibility
func (gw *Gateway) StopHealthChecks() {
	gw.Stop()
}

// GetConfig returns the gateway configuration
func (gw *Gateway) GetConfig() *config.Config {
	return gw.config
}

// GetRouteRegistry returns the Lua route registry (for compatibility)
func (gw *Gateway) GetRouteRegistry() *LuaRouteRegistry {
	return NewLuaRouteRegistry(gw.router)
}
