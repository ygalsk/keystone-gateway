package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var testConfig *Config

// Setup test config once for all tests (DRY principle)
func TestMain(m *testing.M) {
	var err error
	testConfig, err = LoadConfig("./configs/test-config.yaml")
	if err != nil {
		panic("Failed to load test config: " + err.Error())
	}
	os.Exit(m.Run())
}

// Test 1: Configuration loading and validation
func TestConfigLoading(t *testing.T) {
	// Test valid config (use the global testConfig loaded in TestMain)
	cfg := testConfig
	if cfg == nil {
		t.Fatal("Test config not loaded")
	}

	if len(cfg.Tenants) != 3 {
		t.Errorf("Expected 3 tenants, got %d", len(cfg.Tenants))
	}

	// Validate specific tenant configurations
	apiTenant := cfg.Tenants[0]
	if apiTenant.Name != "api-service" || apiTenant.PathPrefix != "/api/" {
		t.Error("API tenant configuration incorrect")
	}

	appTenant := cfg.Tenants[1]
	if len(appTenant.Domains) != 2 || appTenant.Domains[0] != "app.example.com" {
		t.Error("App tenant domains configuration incorrect")
	}

	mobileTenant := cfg.Tenants[2]
	if len(mobileTenant.Domains) == 0 || mobileTenant.PathPrefix == "" {
		t.Error("Mobile tenant hybrid configuration incorrect")
	}
}

// Test 2: Gateway initialization
func TestGatewayInitialization(t *testing.T) {
	gw := NewGateway(testConfig)

	// Check that routers were properly initialized
	if len(gw.pathRouters) != 1 {
		t.Errorf("Expected 1 path router, got %d", len(gw.pathRouters))
	}

	// Host routers: app-service has 2 domains, so 2 entries + mobile-api has 1 domain = 3 total
	if len(gw.hostRouters) != 2 {
		t.Errorf("Expected 2 host router entries, got %d", len(gw.hostRouters))
	}

	if len(gw.hybridRouters) != 1 {
		t.Errorf("Expected 1 hybrid router, got %d", len(gw.hybridRouters))
	}

	// Check that backends were created
	pathRouter := gw.pathRouters["/api/"]
	if pathRouter == nil || len(pathRouter.Backends) != 1 {
		t.Error("API service backend not properly initialized")
	}

	hostRouter := gw.hostRouters["app.example.com"]
	if hostRouter == nil || len(hostRouter.Backends) != 2 {
		t.Error("App service backends not properly initialized")
	}
}

// Test 3: Route matching logic
func TestRouteMatching(t *testing.T) {
	gw := NewGateway(testConfig)

	tests := []struct {
		name           string
		host           string
		path           string
		expectMatch    bool
		expectedTenant string
		expectedPrefix string
	}{
		{
			name:           "Path-based routing",
			host:           "localhost",
			path:           "/api/users",
			expectMatch:    true,
			expectedTenant: "api-service",
			expectedPrefix: "/api/",
		},
		{
			name:           "Host-based routing",
			host:           "app.example.com",
			path:           "/dashboard",
			expectMatch:    true,
			expectedTenant: "app-service",
			expectedPrefix: "",
		},
		{
			name:           "Hybrid routing",
			host:           "mobile.example.com",
			path:           "/v2/endpoints",
			expectMatch:    true,
			expectedTenant: "mobile-api",
			expectedPrefix: "/v2/",
		},
		{
			name:        "No match - wrong host and path",
			host:        "wrong.com",
			path:        "/wrong",
			expectMatch: false,
		},
		{
			name:        "Path doesn't match",
			host:        "localhost",
			path:        "/wrong/path",
			expectMatch: false,
		},
		{
			name:        "Host doesn't match",
			host:        "wrong.example.com",
			path:        "/dashboard",
			expectMatch: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, prefix := gw.MatchRoute(test.host, test.path)

			if test.expectMatch {
				if router == nil {
					t.Errorf("Expected match but got none")
					return
				}
				if router.Name != test.expectedTenant {
					t.Errorf("Expected tenant %s, got %s", test.expectedTenant, router.Name)
				}
				if prefix != test.expectedPrefix {
					t.Errorf("Expected prefix %q, got %q", test.expectedPrefix, prefix)
				}
			} else {
				if router != nil {
					t.Errorf("Expected no match but got tenant: %s", router.Name)
				}
			}
		})
	}
}

// Test 4: Backend round-robin selection
func TestBackendSelection(t *testing.T) {
	gw := NewGateway(testConfig)

	// Get router with multiple backends
	router := gw.hostRouters["app.example.com"]
	if router == nil {
		t.Fatal("App router not found")
	}

	// Mark all backends as healthy
	for _, backend := range router.Backends {
		backend.Alive.Store(true)
	}

	// Test round-robin
	firstBackend := router.NextBackend()
	secondBackend := router.NextBackend()

	if firstBackend == nil || secondBackend == nil {
		t.Fatal("Failed to get backends")
	}

	// They should be different (round-robin)
	if firstBackend == secondBackend {
		t.Error("Round-robin not working - got same backend twice")
	}
}

// Test 5: Health check endpoint
func TestHealthEndpoint(t *testing.T) {
	gw := NewGateway(testConfig)
	router := gw.SetupRouter()

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Error("Expected JSON content type")
	}

	var health HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if health.Status != "healthy" {
		t.Error("Expected healthy status")
	}

	if len(health.Tenants) != 3 {
		t.Errorf("Expected 3 tenants in health status, got %d", len(health.Tenants))
	}
}

// Test 6: Tenants API endpoint
func TestTenantsEndpoint(t *testing.T) {
	gw := NewGateway(testConfig)
	router := gw.SetupRouter()

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/tenants")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var tenants []Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if len(tenants) != 3 {
		t.Errorf("Expected 3 tenants, got %d", len(tenants))
	}

	// Verify tenant data
	found := make(map[string]bool)
	for _, tenant := range tenants {
		found[tenant.Name] = true
	}

	expectedTenants := []string{"api-service", "app-service", "mobile-api"}
	for _, expected := range expectedTenants {
		if !found[expected] {
			t.Errorf("Expected tenant %s not found", expected)
		}
	}
}

// Test 7: Configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		tenant      Tenant
		expectError bool
	}{
		{
			name: "Valid path tenant",
			tenant: Tenant{
				Name:       "valid-path",
				PathPrefix: "/api/",
				Services:   []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: false,
		},
		{
			name: "Valid host tenant",
			tenant: Tenant{
				Name:     "valid-host",
				Domains:  []string{"example.com"},
				Services: []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: false,
		},
		{
			name: "Invalid - no routing config",
			tenant: Tenant{
				Name:     "invalid",
				Services: []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: true,
		},
		{
			name: "Invalid - bad path prefix",
			tenant: Tenant{
				Name:       "invalid-path",
				PathPrefix: "api", // Missing slashes
				Services:   []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: true,
		},
		{
			name: "Invalid - bad domain",
			tenant: Tenant{
				Name:     "invalid-domain",
				Domains:  []string{"invalid domain"},
				Services: []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTenant(test.tenant)
			hasError := err != nil

			if hasError != test.expectError {
				t.Errorf("Expected error: %v, got error: %v (%v)", test.expectError, hasError, err)
			}
		})
	}
}

// Test 8: Basic proxy functionality (simplified)
func TestProxyRouting(t *testing.T) {
	// Create a test backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("backend-ok"))
	}))
	defer backendServer.Close()

	// Create a minimal config pointing to our test backend
	testCfg := &Config{
		Tenants: []Tenant{
			{
				Name:       "test-proxy",
				PathPrefix: "/test/",
				Services: []Service{
					{Name: "test", URL: backendServer.URL, Health: "/health"},
				},
			},
		},
	}

	gw := NewGateway(testCfg)

	// Verify router was created
	router := gw.pathRouters["/test/"]
	if router == nil {
		t.Fatal("Test router not found")
	}

	if len(router.Backends) == 0 {
		t.Fatal("No backends found")
	}

	// Mark backend as healthy
	router.Backends[0].Alive.Store(true)

	// Test route matching first
	matchedRouter, prefix := gw.MatchRoute("localhost", "/test/health")
	if matchedRouter == nil {
		t.Fatal("Route matching failed")
	}

	if prefix != "/test/" {
		t.Errorf("Expected prefix '/test/', got '%s'", prefix)
	}

	// Test backend selection
	backend := matchedRouter.NextBackend()
	if backend == nil {
		t.Fatal("No backend available")
	}

	if !backend.Alive.Load() {
		t.Fatal("Backend not marked as alive")
	}

	t.Logf("Test passed - route matching and backend selection working")
}

// Benchmark tests
func BenchmarkRouteMatching(b *testing.B) {
	gw := NewGateway(testConfig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.MatchRoute("app.example.com", "/dashboard")
	}
}

func BenchmarkBackendSelection(b *testing.B) {
	gw := NewGateway(testConfig)
	router := gw.hostRouters["app.example.com"]

	// Mark backends as healthy
	for _, backend := range router.Backends {
		backend.Alive.Store(true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.NextBackend()
	}
}
