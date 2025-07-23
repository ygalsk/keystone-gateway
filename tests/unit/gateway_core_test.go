package unit

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

func TestExtractHost(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple hostname", "example.com", "example.com"},
		{"hostname with port", "example.com:8080", "example.com"},
		{"IPv4 with port", "192.168.1.1:3000", "192.168.1.1"},
		{"IPv6 with brackets and port", "[::1]:8080", "[::1]"},
		{"IPv6 with brackets only", "[2001:db8::1]", "[2001:db8::1]"},
		{"localhost with port", "localhost:3000", "localhost"},
		{"empty string", "", ""},
		{"colon only", ":", ""},
		{"hostname with colon at end", "example.com:", "example.com"},
		{"port only", ":8080", ""},
		{"malformed IPv6", "[::1:8080", "["}, // malformed, returns up to first colon
		{"multiple colons", "example.com:8080:extra", "example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := routing.ExtractHost(tc.input)
			if result != tc.expected {
				t.Errorf("ExtractHost(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestPathMatchingThroughMatchRoute(t *testing.T) {
	// Test path matching logic through the public MatchRoute method
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "api-tenant",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend1:8081"},
				},
			},
			{
				Name:       "long-path-tenant", 
				PathPrefix: "/api/v2/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend2:8082"},
				},
			},
			{
				Name:       "root-tenant",
				PathPrefix: "/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend3:8083"},
				},
			},
		},
	}

	router := chi.NewRouter()
	gw := routing.NewGatewayWithRouter(cfg, router)

	testCases := []struct {
		name           string
		path           string
		expectedTenant string
		expectedPrefix string
	}{
		{"exact api match", "/api/", "api-tenant", "/api/"},
		{"api subpath", "/api/users", "api-tenant", "/api/"},
		{"longer specific match", "/api/v2/users", "long-path-tenant", "/api/v2/"},
		{"exact longer match", "/api/v2/", "long-path-tenant", "/api/v2/"},
		{"root path", "/", "root-tenant", "/"},
		{"other path falls to root", "/unmatchable", "root-tenant", "/"},
		{"partial match should not work", "/ap", "root-tenant", "/"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use empty host to test pure path-based routing
			matched, prefix := gw.MatchRoute("", tc.path)
			
			if matched == nil {
				t.Errorf("Expected match for path %q, but got nil", tc.path)
				return
			}

			if matched.Name != tc.expectedTenant {
				t.Errorf("Expected tenant %q for path %q, got %q", tc.expectedTenant, tc.path, matched.Name)
			}

			if prefix != tc.expectedPrefix {
				t.Errorf("Expected prefix %q for path %q, got %q", tc.expectedPrefix, tc.path, prefix)
			}
		})
	}
}

func TestNextBackendRoundRobin(t *testing.T) {
	// Create test backends
	backend1, _ := url.Parse("http://backend1:8081")
	backend2, _ := url.Parse("http://backend2:8082") 
	backend3, _ := url.Parse("http://backend3:8083")

	testCases := []struct {
		name     string
		backends []*routing.GatewayBackend
		calls    int
		expected []string
	}{
		{
			name:     "no backends",
			backends: []*routing.GatewayBackend{},
			calls:    3,
			expected: []string{"", "", ""}, // nil backends should return empty
		},
		{
			name: "single backend",
			backends: []*routing.GatewayBackend{
				{URL: backend1},
			},
			calls:    3,
			expected: []string{"backend1:8081", "backend1:8081", "backend1:8081"},
		},
		{
			name: "two backends round robin",
			backends: []*routing.GatewayBackend{
				{URL: backend1},
				{URL: backend2},
			},
			calls:    4,
			expected: []string{"backend2:8082", "backend1:8081", "backend2:8082", "backend1:8081"}, // Starts at index 1
		},
		{
			name: "three backends round robin",
			backends: []*routing.GatewayBackend{
				{URL: backend1},
				{URL: backend2},
				{URL: backend3},
			},
			calls:    6,
			expected: []string{"backend2:8082", "backend3:8083", "backend1:8081", "backend2:8082", "backend3:8083", "backend1:8081"}, // Starts at index 1
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set all backends as alive
			for _, backend := range tc.backends {
				backend.Alive.Store(true)
			}

			tr := &routing.TenantRouter{
				Name:     "test-tenant",
				Backends: tc.backends,
				RRIndex:  0, // Start fresh
			}

			results := make([]string, tc.calls)
			for i := 0; i < tc.calls; i++ {
				backend := tr.NextBackend()
				if backend == nil {
					results[i] = ""
				} else {
					results[i] = backend.URL.Host
				}
			}

			for i, expected := range tc.expected {
				if results[i] != expected {
					t.Errorf("Call %d: expected %q, got %q", i+1, expected, results[i])
				}
			}
		})
	}
}

func TestNextBackendHealthChecking(t *testing.T) {
	// Create test backends
	backend1, _ := url.Parse("http://backend1:8081")
	backend2, _ := url.Parse("http://backend2:8082")
	backend3, _ := url.Parse("http://backend3:8083")

	backends := []*routing.GatewayBackend{
		{URL: backend1},
		{URL: backend2}, 
		{URL: backend3},
	}

	tr := &routing.TenantRouter{
		Name:     "test-tenant",
		Backends: backends,
		RRIndex:  0,
	}

	t.Run("all backends healthy", func(t *testing.T) {
		// Set all backends as alive
		for _, backend := range backends {
			backend.Alive.Store(true)
		}

		// Should rotate through all backends
		hosts := make(map[string]bool)
		for i := 0; i < 6; i++ { // 2 full rotations
			backend := tr.NextBackend()
			if backend != nil {
				hosts[backend.URL.Host] = true
			}
		}

		if len(hosts) != 3 {
			t.Errorf("Expected to see all 3 backends, got %d: %v", len(hosts), hosts)
		}
	})

	t.Run("some backends unhealthy", func(t *testing.T) {
		// Mark first and third backend as dead
		backends[0].Alive.Store(false)
		backends[1].Alive.Store(true)
		backends[2].Alive.Store(false)

		// Should only return the healthy backend
		for i := 0; i < 5; i++ {
			backend := tr.NextBackend()
			if backend == nil {
				t.Error("Expected healthy backend, got nil")
			} else if backend.URL.Host != "backend2:8082" {
				t.Errorf("Expected backend2:8082, got %s", backend.URL.Host)
			}
		}
	})

	t.Run("all backends unhealthy", func(t *testing.T) {
		// Mark all backends as dead
		for _, backend := range backends {
			backend.Alive.Store(false)
		}

		// Should return first backend as fallback (implementation behavior)
		backend := tr.NextBackend()
		if backend == nil {
			t.Error("Expected fallback to first backend when all unhealthy, got nil")
		} else if backend.URL.Host != "backend1:8081" {
			t.Errorf("Expected fallback to backend1:8081 when all unhealthy, got %s", backend.URL.Host)
		}
	})
}

func TestCreateProxy(t *testing.T) {
	cfg := &config.Config{}
	router := chi.NewRouter()
	gw := routing.NewGatewayWithRouter(cfg, router)

	// Test cases for backends without path
	t.Run("backend without path", func(t *testing.T) {
		backend, _ := url.Parse("http://backend.example.com:8080")
		gatewayBackend := &routing.GatewayBackend{URL: backend}

		testCases := []struct {
			name         string
			stripPrefix  string
			requestPath  string
			expectedPath string
		}{
			{"no prefix stripping", "", "/api/users", "/api/users"},
			{"strip api prefix", "/api/", "/api/users", "/users"},
			{"strip exact path", "/api/users", "/api/users", "/"},
			{"strip to root", "/api", "/api", "/"},
			{"strip longer prefix", "/api/v1/", "/api/v1/users/123", "/users/123"},
			{"no match for stripping", "/other/", "/api/users", "/api/users"},
			{"empty after strip", "/exact", "/exact", "/"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				proxy := gw.CreateProxy(gatewayBackend, tc.stripPrefix)
				
				// Create test request
				req := httptest.NewRequest("GET", "http://original.com"+tc.requestPath, nil)
				req.Header.Set("X-Test", "original")

				// Capture what the director function does
				proxy.Director(req)

				// Check scheme and host are updated
				if req.URL.Scheme != "http" {
					t.Errorf("Expected scheme 'http', got %q", req.URL.Scheme)
				}
				if req.URL.Host != "backend.example.com:8080" {
					t.Errorf("Expected host 'backend.example.com:8080', got %q", req.URL.Host)
				}

				// Check path stripping
				if req.URL.Path != tc.expectedPath {
					t.Errorf("Expected path %q, got %q", tc.expectedPath, req.URL.Path)
				}
			})
		}
	})

	// Test cases for backends with path
	t.Run("backend with path", func(t *testing.T) {
		backend, _ := url.Parse("http://backend.example.com:8080/service")
		gatewayBackend := &routing.GatewayBackend{URL: backend}

		testCases := []struct {
			name         string
			stripPrefix  string
			requestPath  string
			expectedPath string
		}{
			{"no prefix stripping", "", "/api/users", "/service/api/users"},
			{"strip api prefix", "/api/", "/api/users", "/service/users"},
			{"strip to root", "/api/users", "/api/users", "/service/"},
			{"strip longer prefix", "/api/v1/", "/api/v1/users/123", "/service/users/123"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				proxy := gw.CreateProxy(gatewayBackend, tc.stripPrefix)
				
				// Create test request
				req := httptest.NewRequest("GET", "http://original.com"+tc.requestPath, nil)

				// Capture what the director function does
				proxy.Director(req)

				// Check path with backend path prepended
				if req.URL.Path != tc.expectedPath {
					t.Errorf("Expected path %q, got %q", tc.expectedPath, req.URL.Path)
				}
			})
		}
	})
}

func TestMatchRouteComprehensive(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "host-tenant",
				Domains:    []string{"api.example.com"},
				Services: []config.Service{
					{Name: "backend", URL: "http://backend1:8081"},
				},
			},
			{
				Name:       "path-tenant", 
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend2:8082"},
				},
			},
			{
				Name:       "hybrid-tenant",
				Domains:    []string{"hybrid.example.com"},
				PathPrefix: "/v1/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend3:8083"},
				},
			},
		},
	}

	router := chi.NewRouter()
	gw := routing.NewGatewayWithRouter(cfg, router)

	testCases := []struct {
		name           string
		host           string
		path           string
		expectedTenant string
		expectedPrefix string
	}{
		// Host-based routing
		{"host match", "api.example.com", "/anything", "host-tenant", ""},
		{"host with port", "api.example.com:443", "/anything", "host-tenant", ""},
		
		// Path-based routing
		{"path match", "unknown.com", "/api/users", "path-tenant", "/api/"},
		{"path exact", "unknown.com", "/api/", "path-tenant", "/api/"},
		
		// Hybrid routing (should take priority)
		{"hybrid match", "hybrid.example.com", "/v1/users", "hybrid-tenant", "/v1/"},
		{"hybrid host no path match", "hybrid.example.com", "/other", "", ""}, // No fallback for hybrid tenants
		
		// Priority testing: hybrid > host > path
		{"priority test - hybrid wins", "hybrid.example.com", "/v1/test", "hybrid-tenant", "/v1/"},
		
		// No matches
		{"no match", "unknown.com", "/unknown", "", ""},
		{"empty host and path", "", "", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matched, prefix := gw.MatchRoute(tc.host, tc.path)

			if tc.expectedTenant == "" {
				if matched != nil {
					t.Errorf("Expected no match for host=%q path=%q, but got tenant %q", tc.host, tc.path, matched.Name)
				}
				return
			}

			if matched == nil {
				t.Errorf("Expected match for host=%q path=%q, but got nil", tc.host, tc.path)
				return
			}

			if matched.Name != tc.expectedTenant {
				t.Errorf("Expected tenant %q for host=%q path=%q, got %q", tc.expectedTenant, tc.host, tc.path, matched.Name)
			}

			if prefix != tc.expectedPrefix {
				t.Errorf("Expected prefix %q for host=%q path=%q, got %q", tc.expectedPrefix, tc.host, tc.path, prefix)
			}
		})
	}
}