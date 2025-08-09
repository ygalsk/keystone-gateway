// Package fixtures provides KISS-principle test fixtures for Keystone Gateway
// Focus: Test what the gateway actually does, not edge cases or standard library behavior
package fixtures

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

// TestEnv provides everything needed to test Keystone Gateway functionality
type TestEnv struct {
	Gateway   *routing.Gateway
	Config    *config.Config
	Router    *chi.Mux
	LuaEngine *lua.Engine
	Backends  []*httptest.Server
	cleanup   []func()
}

// Cleanup cleans up all test resources
func (env *TestEnv) Cleanup() {
	if env.Gateway != nil {
		env.Gateway.StopHealthChecks()
	}
	for _, backend := range env.Backends {
		if backend != nil {
			backend.Close()
		}
	}
	for _, cleanup := range env.cleanup {
		cleanup()
	}
}

// Backend represents different backend behaviors for testing
type Backend struct {
	Name    string
	Handler http.HandlerFunc
	Server  *httptest.Server
}

// CreateBasicBackend creates a backend that echoes request info
func CreateBasicBackend(name string) *Backend {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%s: %s %s", name, r.Method, r.URL.Path)
	})

	return &Backend{
		Name:    name,
		Handler: handler,
		Server:  httptest.NewServer(handler),
	}
}

// CreateTestErrorBackend creates a backend that returns errors based on path
func CreateTestErrorBackend(name string) *Backend {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/500":
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		case "/404":
			http.Error(w, "Not Found", http.StatusNotFound)
		case "/503":
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, "Bad Request", http.StatusBadRequest)
		}
	})

	return &Backend{
		Name:    name,
		Handler: handler,
		Server:  httptest.NewServer(handler),
	}
}

// CreateHealthBackend creates a backend with health endpoint
func CreateHealthBackend(name string) *Backend {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		} else {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s: %s %s", name, r.Method, r.URL.Path)
		}
	})

	return &Backend{
		Name:    name,
		Handler: handler,
		Server:  httptest.NewServer(handler),
	}
}

// CreateConfig creates a basic config for testing
func CreateConfig(tenants ...config.Tenant) *config.Config {
	return &config.Config{
		Tenants:       tenants,
		AdminBasePath: "/admin",
		LuaRouting: &config.LuaRoutingConfig{
			Enabled:    true,
			ScriptsDir: "./scripts",
		},
	}
}

// CreateTenant creates a tenant configuration
func CreateTenant(name, pathPrefix string, domains []string, backends ...*Backend) config.Tenant {
	var services []config.Service
	for _, backend := range backends {
		services = append(services, config.Service{
			Name:   backend.Name,
			URL:    backend.Server.URL,
			Health: "/health",
		})
	}

	return config.Tenant{
		Name:       name,
		PathPrefix: pathPrefix,
		Domains:    domains,
		Services:   services,
	}
}

// SetupBasicGateway creates a basic gateway test environment
func SetupBasicGateway(t *testing.T, tenants ...config.Tenant) *TestEnv {
	cfg := CreateConfig(tenants...)
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	env := &TestEnv{
		Gateway: gateway,
		Config:  cfg,
		Router:  router,
	}

	return env
}

// SetupGatewayWithLua creates a gateway with Lua engine
func SetupGatewayWithLua(t *testing.T, tenants ...config.Tenant) *TestEnv {
	cfg := CreateConfig(tenants...)
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Create temporary scripts directory
	scriptsDir := t.TempDir()
	luaEngine := lua.NewEngine(scriptsDir, router)

	env := &TestEnv{
		Gateway:   gateway,
		Config:    cfg,
		Router:    router,
		LuaEngine: luaEngine,
		cleanup: []func(){
			func() { gateway.StopHealthChecks() },
		},
	}

	return env
}

// TestRequest tests a single HTTP request through the gateway
func TestRequest(t *testing.T, env *TestEnv, method, path string, expectedStatus int) *http.Response {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()

	// Use gateway routing to handle the request
	router, stripPrefix := env.Gateway.MatchRoute("", path)
	if router == nil {
		if expectedStatus == http.StatusNotFound {
			return &http.Response{StatusCode: http.StatusNotFound}
		}
		t.Fatalf("No router found for path: %s", path)
	}

	backend := router.NextBackend()
	if backend == nil {
		if expectedStatus == http.StatusBadGateway {
			return &http.Response{StatusCode: http.StatusBadGateway}
		}
		t.Fatalf("No backend available for path: %s", path)
	}

	proxy := env.Gateway.CreateProxy(backend, stripPrefix)
	proxy.ServeHTTP(w, req)

	if w.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d for %s %s", expectedStatus, w.Code, method, path)
	}

	return &http.Response{
		StatusCode: w.Code,
		Header:     w.Header(),
		Body:       nil, // Can be enhanced if needed
	}
}
