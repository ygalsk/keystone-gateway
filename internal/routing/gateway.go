// Package routing provides simplified routing logic for Keystone Gateway.
//
// Design Note: This package deliberately does NOT include health checking or load balancing logic.
// Health checking is handled by external infrastructure (load balancers like HAProxy, Nginx, AWS ELB, K8s Ingress, etc.).
// This keeps the gateway stateless and follows the "gateway is dumb" design principle.
package routing

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"keystone-gateway/internal/config"
	httputil2 "keystone-gateway/internal/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/hostrouter"
)

// Backend represents a simple backend server
type Backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy
}

// Gateway provides simplified HTTP routing to tenant backends.
// Each tenant routes to a single backend URL (which may be a load balancer).
// The gateway is stateless - no health checking or load balancing logic.
type Gateway struct {
	config     *config.Config
	router     *chi.Mux
	hostRouter hostrouter.Routes
	backends   map[string]*Backend // One backend per tenant
	transport  *http.Transport
	mu         sync.RWMutex
}

// NewGateway creates a new gateway with the provided router
func NewGateway(cfg *config.Config, router *chi.Mux) *Gateway {
	return &Gateway{
		config:     cfg,
		router:     router,
		hostRouter: hostrouter.New(),
		backends:   make(map[string]*Backend),
		transport:  httputil2.CreateTransport(),
	}
}

// SetupRoutes configures all tenant routes
func (gw *Gateway) SetupRoutes() {
	for _, tenant := range gw.config.Tenants {
		if err := gw.setupTenantRoutes(tenant); err != nil {
			slog.Error("tenant_setup_failed",
				"tenant", tenant.Name,
				"error", err,
				"component", "gateway")
			continue
		}

		slog.Info("tenant_initialized",
			"tenant", tenant.Name,
			"backend", tenant.Services[0].URL,
			"component", "gateway")
	}
}

// setupTenantRoutes sets up routes for a specific tenant
func (gw *Gateway) setupTenantRoutes(tenant config.Tenant) error {
	// Each tenant gets one backend (first service configured)
	if len(tenant.Services) == 0 {
		return fmt.Errorf("tenant %s has no services configured", tenant.Name)
	}

	// Use first service (if multiple configured, log warning)
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

	backend := &Backend{
		URL:   u,
		Proxy: proxy,
	}

	gw.mu.Lock()
	gw.backends[tenant.Name] = backend
	gw.mu.Unlock()

	// Setup routing based on tenant configuration
	handler := gw.createTenantHandler(tenant.Name)

	if len(tenant.Domains) > 0 {
		// Host-based routing - create single router for all domains of this tenant
		var pattern string
		if tenant.PathPrefix != "" {
			pattern = tenant.PathPrefix + "*"
		} else {
			pattern = "/*"
		}

		subrouter := chi.NewRouter()
		subrouter.HandleFunc(pattern, handler)

		// Assign same router to all domains
		for _, domain := range tenant.Domains {
			gw.hostRouter[domain] = subrouter
		}
	} else if tenant.PathPrefix != "" {
		// Path-only routing
		gw.router.HandleFunc(tenant.PathPrefix+"*", handler)
	}

	return nil
}

// createTenantHandler creates an HTTP handler that proxies requests to the tenant's backend.
func (gw *Gateway) createTenantHandler(tenantName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gw.mu.RLock()
		backend := gw.backends[tenantName]
		gw.mu.RUnlock()

		if backend == nil {
			http.Error(w, "No backend configured", http.StatusBadGateway)
			return
		}

		backend.Proxy.ServeHTTP(w, r)
	}
}

// misterious catch all ?!
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

// Stop performs cleanup (placeholder for future cleanup needs)
func (gw *Gateway) Stop() {
	// No health checks to stop
	// Keep method for future cleanup needs (transport close, etc.)
}
