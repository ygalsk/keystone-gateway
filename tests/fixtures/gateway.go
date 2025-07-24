// Package fixtures provides organized test fixtures following KISS and DRY principles
package fixtures

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

// GatewayTestEnv represents a complete gateway testing environment
type GatewayTestEnv struct {
	Gateway  *routing.Gateway
	Router   *chi.Mux
	Config   *config.Config
	Backends []*httptest.Server // Mock backend servers for cleanup
}

// SetupGateway creates a basic gateway test environment
func SetupGateway(t *testing.T, cfg *config.Config) *GatewayTestEnv {
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	
	// Set up proxy handler for routing (similar to main.go)
	proxyHandler := func(w http.ResponseWriter, r *http.Request) {
		// Validate headers for malformed content
		for name := range r.Header {
			for _, char := range name {
				if char == 0 { // null byte
					http.Error(w, "Bad Request: Invalid header name", http.StatusBadRequest)
					return
				}
			}
		}
		
		// Validate path for null bytes and excessive length
		if len(r.URL.Path) > 1024 { // Reject paths longer than 1KB
			http.NotFound(w, r)
			return
		}
		for _, char := range r.URL.Path {
			if char == 0 { // null byte in path
				http.NotFound(w, r)
				return
			}
		}
		
		tenantRouter, stripPrefix := gateway.MatchRoute(r.Host, r.URL.Path)
		if tenantRouter == nil {
			http.NotFound(w, r)
			return
		}

		backend := tenantRouter.NextBackend()
		if backend == nil {
			http.Error(w, "No backend available", http.StatusBadGateway)
			return
		}

		proxy := gateway.CreateProxy(backend, stripPrefix)
		proxy.ServeHTTP(w, r)
	}
	
	// Register the proxy handler as catch-all routes
	router.HandleFunc("/", proxyHandler)
	router.HandleFunc("/*", proxyHandler)
	
	return &GatewayTestEnv{
		Gateway: gateway,
		Router:  router,
		Config:  cfg,
	}
}

// SetupSimpleGateway creates a gateway with a single tenant for simple tests
func SetupSimpleGateway(t *testing.T, tenantName, pathPrefix string) *GatewayTestEnv {
	// Create a real mock backend
	backend := CreateSimpleBackend(t)
	
	cfg := CreateConfigWithBackend(tenantName, pathPrefix, backend.URL)
	env := SetupGateway(t, cfg)
	
	// Store backend reference for cleanup
	env.Backends = []*httptest.Server{backend}
	
	// Mark backend as alive for testing
	if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
		for _, gtwBackend := range tenantRouter.Backends {
			gtwBackend.Alive.Store(true)
		}
	}
	
	return env
}

// SetupMultiTenantGateway creates a gateway with multiple tenants for complex tests
func SetupMultiTenantGateway(t *testing.T) *GatewayTestEnv {
	// Create mock backends for each tenant
	apiBackend := CreateSimpleBackend(t)
	webBackend := CreateSimpleBackend(t) 
	mobileBackend := CreateSimpleBackend(t)
	adminBackend := CreateSimpleBackend(t)
	apiPathBackend := CreateSimpleBackend(t)
	hybridBackend := CreateSimpleBackend(t)

	// Create configuration with real backend URLs
	cfg := &config.Config{
		Tenants: []config.Tenant{
			// Host-based tenants
			{
				Name:     "api-tenant",
				Domains:  []string{"api.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "api-backend", URL: apiBackend.URL, Health: "/health"},
				},
			},
			{
				Name:     "web-tenant",
				Domains:  []string{"web.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "web-backend", URL: webBackend.URL, Health: "/health"},
				},
			},
			{
				Name:     "mobile-tenant",
				Domains:  []string{"mobile.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "mobile-backend", URL: mobileBackend.URL, Health: "/health"},
				},
			},
			// Path-based tenants
			{
				Name:       "admin-tenant",
				PathPrefix: "/admin/",
				Interval:   30,
				Services: []config.Service{
					{Name: "admin-backend", URL: adminBackend.URL, Health: "/health"},
				},
			},
			{
				Name:       "api-path-tenant",
				PathPrefix: "/api/v1/",
				Interval:   30,
				Services: []config.Service{
					{Name: "api-path-backend", URL: apiPathBackend.URL, Health: "/health"},
				},
			},
			// Hybrid tenant (host + path)
			{
				Name:       "hybrid-tenant",
				Domains:    []string{"api.example.com"},
				PathPrefix: "/v2/",
				Interval:   30,
				Services: []config.Service{
					{Name: "hybrid-backend", URL: hybridBackend.URL, Health: "/health"},
				},
			},
		},
	}

	env := SetupGateway(t, cfg)
	
	// Store backend references for cleanup
	env.Backends = []*httptest.Server{
		apiBackend, webBackend, mobileBackend, 
		adminBackend, apiPathBackend, hybridBackend,
	}
	
	// Mark all backends as alive for testing
	tenantNames := []string{"api-tenant", "web-tenant", "mobile-tenant", "admin-tenant", "api-path-tenant", "hybrid-tenant"}
	for _, tenantName := range tenantNames {
		if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
			for _, backend := range tenantRouter.Backends {
				backend.Alive.Store(true)
			}
		}
	}
	
	return env
}

// SetupHealthAwareGateway creates a gateway with both API routes and health endpoint
func SetupHealthAwareGateway(t *testing.T, tenantName string) *GatewayTestEnv {
	// Create a health-aware mock backend
	backend := CreateHealthCheckBackend(t)
	
	cfg := CreateHealthAndAPIConfig(tenantName, backend.URL)
	env := SetupGateway(t, cfg)
	
	// Store backend reference for cleanup
	env.Backends = []*httptest.Server{backend}
	
	// Mark all backends as alive for testing
	tenantNames := []string{tenantName + "-health", tenantName}
	for _, tn := range tenantNames {
		if tenantRouter := env.Gateway.GetTenantRouter(tn); tenantRouter != nil {
			for _, gtwBackend := range tenantRouter.Backends {
				gtwBackend.Alive.Store(true)
			}
		}
	}
	
	return env
}

// SetupMethodAwareGateway creates a gateway with a method-aware backend for error testing
func SetupMethodAwareGateway(t *testing.T, tenantName, pathPrefix string) *GatewayTestEnv {
	// Create a method-aware mock backend
	backend := CreateMethodAwareBackend(t)
	
	cfg := CreateConfigWithBackend(tenantName, pathPrefix, backend.URL)
	env := SetupGateway(t, cfg)
	
	// Store backend reference for cleanup
	env.Backends = []*httptest.Server{backend}
	
	// Mark backend as alive for testing
	if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
		for _, gtwBackend := range tenantRouter.Backends {
			gtwBackend.Alive.Store(true)
		}
	}
	
	return env
}

// SetupRestrictiveGateway creates a gateway with a restrictive backend for path testing
func SetupRestrictiveGateway(t *testing.T, tenantName, pathPrefix string) *GatewayTestEnv {
	// Create a restrictive mock backend
	backend := CreateRestrictiveBackend(t)
	
	cfg := CreateConfigWithBackend(tenantName, pathPrefix, backend.URL)
	env := SetupGateway(t, cfg)
	
	// Store backend reference for cleanup
	env.Backends = []*httptest.Server{backend}
	
	// Mark backend as alive for testing
	if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
		for _, gtwBackend := range tenantRouter.Backends {
			gtwBackend.Alive.Store(true)
		}
	}
	
	return env
}

// Cleanup closes all backend servers to prevent resource leaks
func (env *GatewayTestEnv) Cleanup() {
	for _, backend := range env.Backends {
		if backend != nil {
			backend.Close()
		}
	}
}