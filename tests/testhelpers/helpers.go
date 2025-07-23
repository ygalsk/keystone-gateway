// Package testhelpers provides common utilities and fixtures for tests
package testhelpers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

// TestConfig provides common configuration fixtures for tests
var TestConfig = &config.Config{
	Tenants: []config.Tenant{
		{
			Name:       "api-v1",
			PathPrefix: "/api/v1/",
			Services: []config.Service{
				{Name: "api-service", URL: "http://localhost:8001", Health: "/health"},
			},
		},
		{
			Name:    "admin",
			Domains: []string{"admin.example.com"},
			Services: []config.Service{
				{Name: "admin-service", URL: "http://localhost:8002", Health: "/health"},
			},
		},
		{
			Name:       "hybrid",
			Domains:    []string{"api.example.com"},
			PathPrefix: "/v2/",
			Services: []config.Service{
				{Name: "hybrid-service", URL: "http://localhost:8003", Health: "/health"},
			},
		},
	},
}

// CreateTestGateway creates a gateway instance for testing
func CreateTestGateway(cfg *config.Config) *routing.Gateway {
	if cfg == nil {
		cfg = TestConfig
	}
	return routing.NewGatewayWithRouter(cfg, chi.NewMux())
}

// CreateMockBackend creates a mock HTTP backend for testing
func CreateMockBackend(t *testing.T, responseBody string, statusCode int) (*httptest.Server, *url.URL) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(responseBody))
	}))
	
	t.Cleanup(server.Close)
	
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse mock server URL: %v", err)
	}
	
	return server, u
}

// AssertRouteMatch verifies that a route matches expected tenant and prefix
func AssertRouteMatch(t *testing.T, gw *routing.Gateway, host, path, expectedTenant, expectedPrefix string) {
	t.Helper()
	
	tr, prefix := gw.MatchRoute(host, path)
	
	if expectedTenant == "" {
		if tr != nil {
			t.Errorf("Expected no tenant match for %s%s, got %s", host, path, tr.Name)
		}
		return
	}
	
	if tr == nil {
		t.Fatalf("Expected tenant %s for %s%s, got nil", expectedTenant, host, path)
	}
	
	if tr.Name != expectedTenant {
		t.Errorf("Expected tenant %s for %s%s, got %s", expectedTenant, host, path, tr.Name)
	}
	
	if prefix != expectedPrefix {
		t.Errorf("Expected prefix %s for %s%s, got %s", expectedPrefix, host, path, prefix)
	}
}

// AssertBackendHealthy sets a backend to healthy state for testing
func AssertBackendHealthy(backend *routing.GatewayBackend) {
	backend.Alive.Store(true)
}

// AssertBackendUnhealthy sets a backend to unhealthy state for testing
func AssertBackendUnhealthy(backend *routing.GatewayBackend) {
	backend.Alive.Store(false)
}

// MinimalConfig returns a minimal configuration for simple tests
func MinimalConfig(tenantName, pathPrefix string) *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       tenantName,
				PathPrefix: pathPrefix,
				Services: []config.Service{
					{Name: "test-service", URL: "http://localhost:8080", Health: "/health"},
				},
			},
		},
	}
}

// MultiTenantConfig returns a configuration with multiple tenants for complex tests
func MultiTenantConfig() *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "tenant-a",
				PathPrefix: "/a/",
				Services: []config.Service{
					{Name: "service-a", URL: "http://localhost:8001", Health: "/health"},
				},
			},
			{
				Name:    "tenant-b",
				Domains: []string{"b.example.com"},
				Services: []config.Service{
					{Name: "service-b", URL: "http://localhost:8002", Health: "/health"},
				},
			},
			{
				Name:       "tenant-c",
				Domains:    []string{"c.example.com"},
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "service-c1", URL: "http://localhost:8003", Health: "/health"},
					{Name: "service-c2", URL: "http://localhost:8004", Health: "/health"},
				},
			},
		},
	}
}

// TestRoutingScenario defines a test case for routing
type TestRoutingScenario struct {
	Name           string
	Host           string
	Path           string
	ExpectedTenant string
	ExpectedPrefix string
	ShouldMatch    bool
}

// RunRoutingScenarios runs a set of routing test scenarios
func RunRoutingScenarios(t *testing.T, gw *routing.Gateway, scenarios []TestRoutingScenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			if scenario.ShouldMatch {
				AssertRouteMatch(t, gw, scenario.Host, scenario.Path, 
					scenario.ExpectedTenant, scenario.ExpectedPrefix)
			} else {
				AssertRouteMatch(t, gw, scenario.Host, scenario.Path, "", "")
			}
		})
	}
}