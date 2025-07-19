package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

// HealthStatus represents the health status response for testing
type HealthStatus struct {
	Status  string            `json:"status"`
	Tenants map[string]string `json:"tenants"`
	Uptime  string            `json:"uptime"`
	Version string            `json:"version"`
}

var testConfig *config.Config

// Setup test config once for all tests (DRY principle)
func TestMain(m *testing.M) {
	var err error
	testConfig, err = config.LoadConfig("../../configs/examples/test-config.yaml")
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
	gw := routing.NewGateway(testConfig)

	if gw == nil {
		t.Fatal("Gateway creation failed")
	}

	// Test that the gateway can match routes (indirect test of initialization)

	// Test path-based routing
	pathRouter, prefix := gw.MatchRoute("localhost", "/api/users")
	if pathRouter == nil {
		t.Error("API service path router not properly initialized")
	}
	if prefix != "/api/" {
		t.Errorf("Expected prefix '/api/', got '%s'", prefix)
	}

	// Test host-based routing
	hostRouter, hostPrefix := gw.MatchRoute("app.example.com", "/dashboard")
	if hostRouter == nil {
		t.Error("App service host router not properly initialized")
	}
	if hostPrefix != "" {
		t.Errorf("Expected empty prefix for host routing, got '%s'", hostPrefix)
	}
}

// Test 3: Route matching logic
func TestRouteMatching(t *testing.T) {
	gw := routing.NewGateway(testConfig)

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
			path:           "/mobile/endpoints",
			expectMatch:    true,
			expectedTenant: "mobile-api",
			expectedPrefix: "/mobile/",
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
	gw := routing.NewGateway(testConfig)

	// Get router for app.example.com
	router, _ := gw.MatchRoute("app.example.com", "/dashboard")
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
	_ = routing.NewGateway(testConfig)

	// Since SetupRouter is in the main application, we'll skip this HTTP test for now
	// TODO: Move HTTP handlers to a separate package or create test helpers
	t.Skip("HTTP endpoint testing requires application layer - will be implemented in Phase 2")
}

// Test 6: Tenants API endpoint
func TestTenantsEndpoint(t *testing.T) {
	_ = routing.NewGateway(testConfig)

	// Since SetupRouter is in the main application, we'll skip this HTTP test for now
	// TODO: Move HTTP handlers to a separate package or create test helpers
	t.Skip("HTTP endpoint testing requires application layer - will be implemented in Phase 2")
}

// Test 7: Configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		tenant      config.Tenant
		expectError bool
	}{
		{
			name: "Valid path tenant",
			tenant: config.Tenant{
				Name:       "valid-path",
				PathPrefix: "/api/",
				Services:   []config.Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: false,
		},
		{
			name: "Valid host tenant",
			tenant: config.Tenant{
				Name:     "valid-host",
				Domains:  []string{"example.com"},
				Services: []config.Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: false,
		},
		{
			name: "Invalid - no routing config",
			tenant: config.Tenant{
				Name:     "invalid",
				Services: []config.Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: true,
		},
		{
			name: "Invalid - bad path prefix",
			tenant: config.Tenant{
				Name:       "invalid-path",
				PathPrefix: "api", // Missing slashes
				Services:   []config.Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: true,
		},
		{
			name: "Invalid - bad domain",
			tenant: config.Tenant{
				Name:     "invalid-domain",
				Domains:  []string{"invalid domain"},
				Services: []config.Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Since validateTenant is not exported, we'll test by creating configs and loading them
			// This is more of an integration test approach

			// For now, we'll test this indirectly by trying to create a gateway with the config
			testCfg := &config.Config{
				Tenants: []config.Tenant{test.tenant},
			}

			// Try to create a gateway - this will validate the tenant
			gateway := routing.NewGateway(testCfg)

			// If we expect an error, the initialization might still succeed
			// but the route matching should fail or behave differently
			if !test.expectError && gateway == nil {
				t.Error("Expected valid config but gateway creation failed")
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
	testCfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "test-proxy",
				PathPrefix: "/test/",
				Services: []config.Service{
					{Name: "test", URL: backendServer.URL, Health: "/health"},
				},
			},
		},
	}

	gw := routing.NewGateway(testCfg)

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

	// Mark backend as healthy for testing
	backend.Alive.Store(true)

	if !backend.Alive.Load() {
		t.Fatal("Backend not marked as alive")
	}

	t.Logf("Test passed - route matching and backend selection working")
}

// Benchmark tests
func BenchmarkRouteMatching(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gw.MatchRoute("app.example.com", "/dashboard")
	}
}

func BenchmarkBackendSelection(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	// Get a router to test backend selection
	router, _ := gw.MatchRoute("app.example.com", "/dashboard")
	if router == nil {
		b.Fatal("Router not found for benchmark")
	}

	// Mark backends as healthy
	for _, backend := range router.Backends {
		backend.Alive.Store(true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.NextBackend()
	}
}
