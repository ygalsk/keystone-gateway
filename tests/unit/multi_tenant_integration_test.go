package unit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

// TestMultiTenantRoutingScenarios tests complex multi-tenant routing scenarios
func TestMultiTenantRoutingScenarios(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		requests []routingTestRequest
	}{
		{
			name: "hybrid routing with overlapping domains and paths",
			config: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "primary-api",
						Domains:    []string{"api.example.com"},
						PathPrefix: "/v1/",
						Services: []config.Service{
							{Name: "primary-service", URL: "http://localhost:8001", Health: "/health"},
						},
					},
					{
						Name:       "secondary-api",
						Domains:    []string{"api.example.com"},
						PathPrefix: "/v2/",
						Services: []config.Service{
							{Name: "secondary-service", URL: "http://localhost:8002", Health: "/health"},
						},
					},
					{
						Name:    "fallback-host",
						Domains: []string{"api.example.com"},
						Services: []config.Service{
							{Name: "fallback-service", URL: "http://localhost:8003", Health: "/health"},
						},
					},
					{
						Name:       "path-only",
						PathPrefix: "/legacy/",
						Services: []config.Service{
							{Name: "legacy-service", URL: "http://localhost:8004", Health: "/health"},
						},
					},
				},
			},
			requests: []routingTestRequest{
				{host: "api.example.com", path: "/v1/users", expectedTenant: "primary-api", expectedPrefix: "/v1/"},
				{host: "api.example.com", path: "/v2/orders", expectedTenant: "secondary-api", expectedPrefix: "/v2/"},
				{host: "api.example.com", path: "/other", expectedTenant: "fallback-host", expectedPrefix: ""},
				{host: "other.example.com", path: "/legacy/data", expectedTenant: "path-only", expectedPrefix: "/legacy/"},
				{host: "api.example.com", path: "/legacy/data", expectedTenant: "fallback-host", expectedPrefix: ""},
			},
		},
		{
			name: "path prefix priority ordering",
			config: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "short-prefix",
						PathPrefix: "/api/",
						Services: []config.Service{
							{Name: "short-service", URL: "http://localhost:8001", Health: "/health"},
						},
					},
					{
						Name:       "long-prefix",
						PathPrefix: "/api/v1/",
						Services: []config.Service{
							{Name: "long-service", URL: "http://localhost:8002", Health: "/health"},
						},
					},
					{
						Name:       "longest-prefix",
						PathPrefix: "/api/v1/admin/",
						Services: []config.Service{
							{Name: "admin-service", URL: "http://localhost:8003", Health: "/health"},
						},
					},
				},
			},
			requests: []routingTestRequest{
				{host: "example.com", path: "/api/v1/admin/users", expectedTenant: "longest-prefix", expectedPrefix: "/api/v1/admin/"},
				{host: "example.com", path: "/api/v1/public", expectedTenant: "long-prefix", expectedPrefix: "/api/v1/"},
				{host: "example.com", path: "/api/status", expectedTenant: "short-prefix", expectedPrefix: "/api/"},
				{host: "example.com", path: "/other", expectedTenant: "", expectedPrefix: ""},
			},
		},
		{
			name: "domain variations and wildcard scenarios",
			config: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:    "multi-domain",
						Domains: []string{"app.example.com", "service.example.com", "api.example.com"},
						Services: []config.Service{
							{Name: "multi-service", URL: "http://localhost:8001", Health: "/health"},
						},
					},
					{
						Name:    "specific-subdomain",
						Domains: []string{"admin.example.com"},
						Services: []config.Service{
							{Name: "admin-service", URL: "http://localhost:8002", Health: "/health"},
						},
					},
				},
			},
			requests: []routingTestRequest{
				{host: "app.example.com", path: "/dashboard", expectedTenant: "multi-domain", expectedPrefix: ""},
				{host: "service.example.com", path: "/metrics", expectedTenant: "multi-domain", expectedPrefix: ""},
				{host: "api.example.com", path: "/v1/data", expectedTenant: "multi-domain", expectedPrefix: ""},
				{host: "admin.example.com", path: "/users", expectedTenant: "specific-subdomain", expectedPrefix: ""},
				{host: "unknown.example.com", path: "/test", expectedTenant: "", expectedPrefix: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := routing.NewGatewayWithRouter(tt.config, chi.NewMux())

			for _, req := range tt.requests {
				t.Run(fmt.Sprintf("%s%s", req.host, req.path), func(t *testing.T) {
					tr, prefix := gw.MatchRoute(req.host, req.path)

					if req.expectedTenant == "" {
						if tr != nil {
							t.Errorf("Expected no tenant match, got %s", tr.Name)
						}
						return
					}

					if tr == nil {
						t.Fatalf("Expected tenant %s, got nil", req.expectedTenant)
					}

					if tr.Name != req.expectedTenant {
						t.Errorf("Expected tenant %s, got %s", req.expectedTenant, tr.Name)
					}

					if prefix != req.expectedPrefix {
						t.Errorf("Expected prefix %s, got %s", req.expectedPrefix, prefix)
					}
				})
			}
		})
	}
}

// TestConcurrentRouting tests thread safety under concurrent routing requests
func TestConcurrentRouting(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "concurrent-api",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "service1", URL: "http://localhost:8001", Health: "/health"},
					{Name: "service2", URL: "http://localhost:8002", Health: "/health"},
					{Name: "service3", URL: "http://localhost:8003", Health: "/health"},
				},
			},
			{
				Name:    "concurrent-host",
				Domains: []string{"concurrent.example.com"},
				Services: []config.Service{
					{Name: "host-service", URL: "http://localhost:8004", Health: "/health"},
				},
			},
		},
	}

	gw := routing.NewGatewayWithRouter(cfg, chi.NewMux())

	// Test concurrent route matching
	const numGoroutines = 50
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]int)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				// Alternate between different routing scenarios
				var tr *routing.TenantRouter
				var prefix string
				var key string

				if j%2 == 0 {
					tr, prefix = gw.MatchRoute("example.com", "/api/data")
					key = "path-routing"
				} else {
					tr, prefix = gw.MatchRoute("concurrent.example.com", "/users")
					key = "host-routing"
				}

				mu.Lock()
				if tr != nil {
					results[key+"-success"]++
					
					// Test round-robin load balancing under concurrency
					backend := tr.NextBackend()
					if backend != nil {
						results["backend-selection"]++
					}
				} else {
					results[key+"-failure"]++
				}
				
				// Verify prefix consistency
				if key == "path-routing" && prefix != "/api/" {
					results["prefix-inconsistency"]++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify results
	expectedSuccesses := numGoroutines * requestsPerGoroutine
	pathSuccesses := results["path-routing-success"]
	hostSuccesses := results["host-routing-success"]
	
	if pathSuccesses != expectedSuccesses/2 {
		t.Errorf("Expected %d path routing successes, got %d", expectedSuccesses/2, pathSuccesses)
	}
	
	if hostSuccesses != expectedSuccesses/2 {
		t.Errorf("Expected %d host routing successes, got %d", expectedSuccesses/2, hostSuccesses)
	}
	
	if results["prefix-inconsistency"] > 0 {
		t.Errorf("Found %d prefix inconsistencies under concurrent access", results["prefix-inconsistency"])
	}
	
	// Verify backends were selected
	if results["backend-selection"] != expectedSuccesses {
		t.Errorf("Expected %d backend selections, got %d", expectedSuccesses, results["backend-selection"])
	}
}

// TestLoadBalancingIntegration tests load balancing across multiple backends
func TestLoadBalancingIntegration(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "load-balanced",
				PathPrefix: "/lb/",
				Services: []config.Service{
					{Name: "backend1", URL: "http://localhost:8001", Health: "/health"},
					{Name: "backend2", URL: "http://localhost:8002", Health: "/health"},
					{Name: "backend3", URL: "http://localhost:8003", Health: "/health"},
				},
			},
		},
	}

	gw := routing.NewGatewayWithRouter(cfg, chi.NewMux())
	tr, _ := gw.MatchRoute("example.com", "/lb/test")
	
	if tr == nil {
		t.Fatal("Failed to match load-balanced tenant")
	}

	// Simulate all backends as healthy
	for _, backend := range tr.Backends {
		backend.Alive.Store(true)
	}

	// Test round-robin distribution
	backendCounts := make(map[string]int)
	const numRequests = 300

	for i := 0; i < numRequests; i++ {
		backend := tr.NextBackend()
		if backend != nil {
			backendCounts[backend.URL.Host]++
		}
	}

	// Verify relatively even distribution (within 10% of expected)
	expectedPerBackend := numRequests / len(tr.Backends)
	tolerance := expectedPerBackend / 10

	for host, count := range backendCounts {
		if abs(count-expectedPerBackend) > tolerance {
			t.Errorf("Backend %s received %d requests, expected ~%d (±%d)", 
				host, count, expectedPerBackend, tolerance)
		}
	}

	// Test fallback to unhealthy backend when all are down
	for _, backend := range tr.Backends {
		backend.Alive.Store(false)
	}

	backend := tr.NextBackend()
	if backend == nil {
		t.Error("Expected fallback to first backend when all are unhealthy, got nil")
	}
}

// TestProxyCreationAndConfiguration tests proxy setup for different routing scenarios
func TestProxyCreationAndConfiguration(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "proxy-test",
				PathPrefix: "/api/v1/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend.example.com:8080/service", Health: "/health"},
				},
			},
		},
	}

	gw := routing.NewGatewayWithRouter(cfg, chi.NewMux())
	tr, prefix := gw.MatchRoute("example.com", "/api/v1/users")
	
	if tr == nil {
		t.Fatal("Failed to match tenant for proxy test")
	}

	backend := &routing.GatewayBackend{
		URL: mustParseURL("http://backend.example.com:8080/service?default=param"),
	}
	backend.Alive.Store(true)

	proxy := gw.CreateProxy(backend, prefix)
	if proxy == nil {
		t.Fatal("Failed to create proxy")
	}

	// Test proxy director configuration
	testReq := httptest.NewRequest("GET", "/api/v1/users/123?filter=active", nil)
	testReq.Host = "example.com"

	// Create a recorder to capture the modified request
	original := testReq.Clone(testReq.Context())
	proxy.Director(testReq)

	// Verify URL transformation
	if testReq.URL.Scheme != "http" {
		t.Errorf("Expected scheme 'http', got '%s'", testReq.URL.Scheme)
	}
	
	if testReq.URL.Host != "backend.example.com:8080" {
		t.Errorf("Expected host 'backend.example.com:8080', got '%s'", testReq.URL.Host)
	}

	// Verify path prefix stripping
	expectedPath := "/service/users/123"
	if testReq.URL.Path != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, testReq.URL.Path)
	}

	// Verify query parameter merging
	expectedQuery := "default=param&filter=active"
	if testReq.URL.RawQuery != expectedQuery {
		t.Errorf("Expected query '%s', got '%s'", expectedQuery, testReq.URL.RawQuery)
	}

	// Ensure original request wasn't modified
	if original.URL.Path != "/api/v1/users/123" {
		t.Error("Original request was unexpectedly modified")
	}
}

// TestTenantIsolation tests that tenants are properly isolated from each other
func TestTenantIsolation(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:    "tenant-a",
				Domains: []string{"a.example.com"},
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
				PathPrefix: "/c/",
				Services: []config.Service{
					{Name: "service-c", URL: "http://localhost:8003", Health: "/health"},
				},
			},
		},
	}

	gw := routing.NewGatewayWithRouter(cfg, chi.NewMux())

	// Test cross-tenant isolation
	testCases := []struct {
		host         string
		path         string
		expectedName string
		shouldMatch  bool
	}{
		{"a.example.com", "/users", "tenant-a", true},
		{"b.example.com", "/users", "tenant-b", true},
		{"a.example.com", "/c/data", "tenant-a", true}, // Host routing takes precedence (Priority 2)
		{"b.example.com", "/c/data", "tenant-b", true}, // Host routing takes precedence (Priority 2)  
		{"c.example.com", "/users", "", false}, // No host match for c.example.com, no path match
		{"unknown.example.com", "/c/data", "tenant-c", true}, // Falls back to path routing
		{"a.example.com", "/wrong", "tenant-a", true}, // Host matches tenant-a (host-only routing ignores path)
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s%s", tc.host, tc.path), func(t *testing.T) {
			tr, _ := gw.MatchRoute(tc.host, tc.path)
			
			if !tc.shouldMatch {
				if tr != nil {
					t.Errorf("Expected no match, got tenant %s", tr.Name)
				}
				return
			}
			
			if tr == nil {
				t.Fatalf("Expected tenant %s, got nil", tc.expectedName)
			}
			
			if tr.Name != tc.expectedName {
				t.Errorf("Expected tenant %s, got %s", tc.expectedName, tr.Name)
			}
			
			// Verify tenant has correct backend isolation
			if len(tr.Backends) != 1 {
				t.Errorf("Expected 1 backend for %s, got %d", tc.expectedName, len(tr.Backends))
			}
		})
	}
}

// TestDynamicRouteRegistryIntegration tests integration with Lua route registry
func TestDynamicRouteRegistryIntegration(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "dynamic-tenant",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "main-service", URL: "http://localhost:8001", Health: "/health"},
				},
			},
		},
	}

	router := chi.NewMux()
	gw := routing.NewGatewayWithRouter(cfg, router)
	registry := gw.GetRouteRegistry()

	// Register some dynamic routes
	err := registry.RegisterRoute(routing.RouteDefinition{
		TenantName: "dynamic-tenant",
		Method:     "GET",
		Pattern:    "/dynamic/users",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("dynamic response"))
		},
	})
	if err != nil {
		t.Fatalf("Failed to register dynamic route: %v", err)
	}

	// Mount tenant routes
	err = registry.MountTenantRoutes("dynamic-tenant", "/api/")
	if err != nil {
		t.Fatalf("Failed to mount tenant routes: %v", err)
	}

	// Test that both static and dynamic routing work together
	tr, prefix := gw.MatchRoute("example.com", "/api/users")
	if tr == nil {
		t.Fatal("Static routing failed")
	}
	if tr.Name != "dynamic-tenant" {
		t.Errorf("Expected dynamic-tenant, got %s", tr.Name)
	}
	if prefix != "/api/" {
		t.Errorf("Expected prefix /api/, got %s", prefix)
	}

	// Test dynamic route through the router
	req := httptest.NewRequest("GET", "/api/dynamic/users", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "dynamic response" {
		t.Errorf("Expected 'dynamic response', got '%s'", rec.Body.String())
	}

	// Verify tenant isolation in registry
	tenants := registry.ListTenants()
	if len(tenants) != 1 || tenants[0] != "dynamic-tenant" {
		t.Errorf("Expected [dynamic-tenant], got %v", tenants)
	}
}

// Helper types and functions

type routingTestRequest struct {
	host           string
	path           string
	expectedTenant string
	expectedPrefix string
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL %s: %v", rawURL, err))
	}
	return u
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}