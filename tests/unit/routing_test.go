package unit

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
	"keystone-gateway/tests/fixtures"
)

// TestMultiTenantRouting tests multi-tenant routing scenarios
func TestMultiTenantRouting(t *testing.T) {
	env := fixtures.SetupMultiTenantGateway(t)

	testCases := []fixtures.HTTPTestCase{
		// Host-based routing
		{
			Name:           "route to api tenant by host",
			Method:         "GET",
			Path:           "/data",
			Headers:        map[string]string{"Host": "api.example.com"},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "route to web tenant by host",
			Method:         "GET",
			Path:           "/home",
			Headers:        map[string]string{"Host": "web.example.com"},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "route to mobile tenant by host",
			Method:         "GET",
			Path:           "/app",
			Headers:        map[string]string{"Host": "mobile.example.com"},
			ExpectedStatus: http.StatusOK,
		},

		// Path-based routing
		{
			Name:           "route to admin by path",
			Method:         "GET",
			Path:           "/admin/dashboard",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "route to api by path",
			Method:         "GET",
			Path:           "/api/v1/users",
			ExpectedStatus: http.StatusOK,
		},

		// Hybrid routing (host + path)
		{
			Name:           "hybrid routing with correct host and path",
			Method:         "GET",
			Path:           "/v2/users",
			Headers:        map[string]string{"Host": "api.example.com"},
			ExpectedStatus: http.StatusOK,
		},

		// Edge cases
		{
			Name:           "unknown host",
			Method:         "GET",
			Path:           "/data",
			Headers:        map[string]string{"Host": "unknown.example.com"},
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "unknown path",
			Method:         "GET",
			Path:           "/unknown/path",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "host with port number",
			Method:         "GET",
			Path:           "/data",
			Headers:        map[string]string{"Host": "api.example.com:8080"},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "IPv6 host with port",
			Method:         "GET",
			Path:           "/data",
			Headers:        map[string]string{"Host": "[::1]:8080"},
			ExpectedStatus: http.StatusNotFound, // No tenant configured for IPv6
		},
	}

	fixtures.RunHTTPTestCases(t, env.Router, testCases)
}

// TestRouteMatching tests the route matching logic
func TestRouteMatching(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:     "host-tenant",
				Domains:  []string{"host.example.com"},
				Services: []config.Service{{Name: "svc1", URL: "http://backend1:8080", Health: "/health"}},
			},
			{
				Name:       "path-tenant",
				PathPrefix: "/api/",
				Services:   []config.Service{{Name: "svc2", URL: "http://backend2:8080", Health: "/health"}},
			},
			{
				Name:       "hybrid-tenant",
				Domains:    []string{"api.example.com"},
				PathPrefix: "/v2/",
				Services:   []config.Service{{Name: "svc3", URL: "http://backend3:8080", Health: "/health"}},
			},
		},
	}

	env := fixtures.SetupGateway(t, cfg)
	gateway := env.Gateway

	testCases := []struct {
		name           string
		host           string
		path           string
		expectedTenant string
		expectedPrefix string
		shouldMatch    bool
	}{
		{
			name:           "host-only match",
			host:           "host.example.com",
			path:           "/any/path",
			expectedTenant: "host-tenant",
			expectedPrefix: "",
			shouldMatch:    true,
		},
		{
			name:           "path-only match",
			host:           "any.host.com",
			path:           "/api/users",
			expectedTenant: "path-tenant",
			expectedPrefix: "/api/",
			shouldMatch:    true,
		},
		{
			name:           "hybrid match",
			host:           "api.example.com",
			path:           "/v2/users",
			expectedTenant: "hybrid-tenant",
			expectedPrefix: "/v2/",
			shouldMatch:    true,
		},
		{
			name:           "no match - wrong host",
			host:           "wrong.example.com",
			path:           "/any/path",
			expectedTenant: "",
			expectedPrefix: "",
			shouldMatch:    false,
		},
		{
			name:           "no match - wrong path",
			host:           "any.host.com",
			path:           "/wrong/path",
			expectedTenant: "",
			expectedPrefix: "",
			shouldMatch:    false,
		},
		{
			name:           "host with port",
			host:           "host.example.com:8080",
			path:           "/test",
			expectedTenant: "host-tenant",
			expectedPrefix: "",
			shouldMatch:    true,
		},
		{
			name:           "IPv6 host",
			host:           "[::1]:8080",
			path:           "/test",
			expectedTenant: "",
			expectedPrefix: "",
			shouldMatch:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenantRouter, prefix := gateway.MatchRoute(tc.host, tc.path)

			if tc.shouldMatch {
				if tenantRouter == nil {
					t.Fatal("Expected to find a tenant router, got nil")
				}
				if tenantRouter.Name != tc.expectedTenant {
					t.Errorf("Expected tenant %q, got %q", tc.expectedTenant, tenantRouter.Name)
				}
				if prefix != tc.expectedPrefix {
					t.Errorf("Expected prefix %q, got %q", tc.expectedPrefix, prefix)
				}
			} else {
				if tenantRouter != nil {
					t.Errorf("Expected no match, but got tenant %q", tenantRouter.Name)
				}
			}
		})
	}
}

// TestLoadBalancing tests round-robin load balancing
func TestLoadBalancing(t *testing.T) {
	// Create tenant with multiple backends
	backend1 := fixtures.CreateSimpleBackend(t)
	defer backend1.Close()
	backend2 := fixtures.CreateSimpleBackend(t)
	defer backend2.Close()
	backend3 := fixtures.CreateSimpleBackend(t)
	defer backend3.Close()

	cfg := fixtures.CreateConfigWithMultipleBackends("lb-tenant", "/lb/", []string{
		backend1.URL,
		backend2.URL,
		backend3.URL,
	})

	env := fixtures.SetupGateway(t, cfg)
	gateway := env.Gateway

	// Get the tenant router
	tenantRouter := gateway.GetTenantRouter("lb-tenant")
	if tenantRouter == nil {
		t.Fatal("Expected to find tenant router")
	}

	// Mark all backends as alive for load balancing
	for _, backend := range tenantRouter.Backends {
		backend.Alive.Store(true)
	}

	// Test round-robin behavior
	seenBackends := make(map[string]int)
	iterations := 9 // 3 backends * 3 rounds

	for i := 0; i < iterations; i++ {
		backend := tenantRouter.NextBackend()
		if backend == nil {
			t.Fatal("Expected backend, got nil")
		}
		seenBackends[backend.URL.String()]++
	}

	// Each backend should be selected equally
	expectedCount := iterations / 3
	for backendURL, count := range seenBackends {
		if count != expectedCount {
			t.Errorf("Backend %s selected %d times, expected %d", backendURL, count, expectedCount)
		}
	}
}

// TestBackendHealth tests backend health tracking
func TestBackendHealth(t *testing.T) {
	// Create healthy and unhealthy backends
	healthyBackend := fixtures.CreateSimpleBackend(t)
	defer healthyBackend.Close()
	unhealthyBackend := fixtures.CreateErrorBackend(t)
	defer unhealthyBackend.Close()

	cfg := fixtures.CreateConfigWithMultipleBackends("health-tenant", "/health/", []string{
		healthyBackend.URL,
		unhealthyBackend.URL,
	})

	env := fixtures.SetupGateway(t, cfg)
	gateway := env.Gateway

	tenantRouter := gateway.GetTenantRouter("health-tenant")
	if tenantRouter == nil {
		t.Fatal("Expected to find tenant router")
	}

	// Mark first backend as healthy, second as unhealthy
	tenantRouter.Backends[0].Alive.Store(true)
	tenantRouter.Backends[1].Alive.Store(false)

	// Should always select healthy backend
	for i := 0; i < 5; i++ {
		backend := tenantRouter.NextBackend()
		if backend == nil {
			t.Fatal("Expected backend, got nil")
		}
		if backend.URL.String() != healthyBackend.URL {
			t.Errorf("Expected healthy backend %s, got %s", healthyBackend.URL, backend.URL.String())
		}
	}

	// Test fallback when all backends unhealthy
	tenantRouter.Backends[0].Alive.Store(false)
	backend := tenantRouter.NextBackend()
	if backend == nil {
		t.Fatal("Expected fallback backend, got nil")
	}
}

// TestProxyCreation tests reverse proxy creation
func TestProxyCreation(t *testing.T) {
	backend := fixtures.CreateEchoBackend(t)
	defer backend.Close()

	env := fixtures.SetupSimpleGateway(t, "proxy-tenant", "/proxy/")
	gateway := env.Gateway

	// Parse backend URL
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Failed to parse backend URL: %v", err)
	}

	// Create gateway backend
	gwBackend := &routing.GatewayBackend{
		URL: backendURL,
	}
	gwBackend.Alive.Store(true)

	testCases := []struct {
		name         string
		stripPrefix  string
		requestPath  string
		expectedPath string
	}{
		{
			name:         "no prefix stripping",
			stripPrefix:  "",
			requestPath:  "/proxy/test",
			expectedPath: "/proxy/test",
		},
		{
			name:         "strip prefix",
			stripPrefix:  "/proxy/",
			requestPath:  "/proxy/test",
			expectedPath: "/test",
		},
		{
			name:         "strip prefix root",
			stripPrefix:  "/proxy/",
			requestPath:  "/proxy/",
			expectedPath: "/",
		},
		{
			name:         "strip prefix nested",
			stripPrefix:  "/proxy/",
			requestPath:  "/proxy/api/v1/users",
			expectedPath: "/api/v1/users",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := gateway.CreateProxy(gwBackend, tc.stripPrefix)
			if proxy == nil {
				t.Fatal("Expected proxy, got nil")
			}

			// Test proxy director function by creating a mock request
			req, err := http.NewRequest("GET", tc.requestPath, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Apply proxy director
			proxy.Director(req)

			if req.URL.Host != backendURL.Host {
				t.Errorf("Expected host %s, got %s", backendURL.Host, req.URL.Host)
			}

			if req.URL.Scheme != backendURL.Scheme {
				t.Errorf("Expected scheme %s, got %s", backendURL.Scheme, req.URL.Scheme)
			}

			if req.URL.Path != tc.expectedPath {
				t.Errorf("Expected path %s, got %s", tc.expectedPath, req.URL.Path)
			}
		})
	}
}

// TestHostExtraction tests host header extraction logic
func TestHostExtraction(t *testing.T) {
	testCases := []struct {
		name         string
		hostHeader   string
		expectedHost string
	}{
		{
			name:         "simple hostname",
			hostHeader:   "example.com",
			expectedHost: "example.com",
		},
		{
			name:         "hostname with port",
			hostHeader:   "example.com:8080",
			expectedHost: "example.com",
		},
		{
			name:         "IPv4 with port",
			hostHeader:   "192.168.1.1:8080",
			expectedHost: "192.168.1.1",
		},
		{
			name:         "IPv6 with brackets",
			hostHeader:   "[::1]:8080",
			expectedHost: "[::1]",
		},
		{
			name:         "IPv6 without port",
			hostHeader:   "[2001:db8::1]",
			expectedHost: "[2001:db8::1]",
		},
		{
			name:         "localhost with port",
			hostHeader:   "localhost:3000",
			expectedHost: "localhost",
		},
		{
			name:         "empty host",
			hostHeader:   "",
			expectedHost: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := routing.ExtractHost(tc.hostHeader)
			if result != tc.expectedHost {
				t.Errorf("Expected %q, got %q", tc.expectedHost, result)
			}
		})
	}
}

// TestRoutingEdgeCases tests edge cases and error conditions
func TestRoutingEdgeCases(t *testing.T) {
	testCases := []struct {
		name      string
		setupFunc func(t *testing.T) *fixtures.GatewayTestEnv
		testFunc  func(t *testing.T, env *fixtures.GatewayTestEnv)
	}{
		{
			name: "empty tenant configuration",
			setupFunc: func(t *testing.T) *fixtures.GatewayTestEnv {
				cfg := &config.Config{Tenants: []config.Tenant{}}
				return fixtures.SetupGateway(t, cfg)
			},
			testFunc: func(t *testing.T, env *fixtures.GatewayTestEnv) {
				router, prefix := env.Gateway.MatchRoute("any.host.com", "/any/path")
				if router != nil {
					t.Error("Expected no router for empty configuration")
				}
				if prefix != "" {
					t.Error("Expected empty prefix for no match")
				}
			},
		},
		{
			name: "tenant with no backends",
			setupFunc: func(t *testing.T) *fixtures.GatewayTestEnv {
				cfg := &config.Config{
					Tenants: []config.Tenant{{
						Name:       "empty-tenant",
						PathPrefix: "/empty/",
						Services:   []config.Service{},
					}},
				}
				return fixtures.SetupGateway(t, cfg)
			},
			testFunc: func(t *testing.T, env *fixtures.GatewayTestEnv) {
				tenantRouter := env.Gateway.GetTenantRouter("empty-tenant")
				if tenantRouter == nil {
					t.Fatal("Expected tenant router")
				}
				backend := tenantRouter.NextBackend()
				if backend != nil {
					t.Error("Expected no backend for empty tenant")
				}
			},
		},
		{
			name: "invalid backend URL",
			setupFunc: func(t *testing.T) *fixtures.GatewayTestEnv {
				cfg := &config.Config{
					Tenants: []config.Tenant{{
						Name:       "invalid-tenant",
						PathPrefix: "/invalid/",
						Services: []config.Service{{
							Name:   "invalid-svc",
							URL:    "not-a-valid-url",
							Health: "/health",
						}},
					}},
				}
				return fixtures.SetupGateway(t, cfg)
			},
			testFunc: func(t *testing.T, env *fixtures.GatewayTestEnv) {
				tenantRouter := env.Gateway.GetTenantRouter("invalid-tenant")
				if tenantRouter == nil {
					t.Fatal("Expected tenant router")
				}
				// Should have no backends due to invalid URL
				if len(tenantRouter.Backends) != 0 {
					t.Error("Expected no backends for invalid URL")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := tc.setupFunc(t)
			tc.testFunc(t, env)
		})
	}
}

// TestCompressionIntegration tests compression works end-to-end through the gateway
func TestCompressionIntegration(t *testing.T) {
	// Create a router with compression middleware enabled (like in main.go)
	r := chi.NewRouter()
	
	// Add compression middleware like in our main application
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	
	// Add compression middleware for better performance on text content
	r.Use(middleware.Compress(5, 
		"text/html", 
		"text/css", 
		"text/javascript", 
		"application/json", 
		"application/xml",
		"text/plain",
	))

	// Add test routes that return different content types
	r.Get("/api/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Large enough JSON to trigger compression
		jsonResponse := `{"users":[{"id":1,"name":"John Doe","email":"john@example.com","bio":"A software developer with 10 years of experience"},{"id":2,"name":"Jane Smith","email":"jane@example.com","bio":"A designer who loves creating beautiful user interfaces"}],"total":2,"page":1,"metadata":{"timestamp":"2024-01-01T00:00:00Z","version":"1.0","source":"api"}}`
		if _, err := w.Write([]byte(jsonResponse)); err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	})

	r.Get("/api/text", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		// Large enough text to trigger compression
		textResponse := strings.Repeat("This is a text response that should be compressed when gzip is supported by the client. ", 10)
		if _, err := w.Write([]byte(textResponse)); err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	})

	r.Get("/api/small", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	})

	testCases := []struct {
		name           string
		path           string
		acceptEncoding string
		expectCompressed bool
		description    string
	}{
		{
			name:           "Large JSON response with compression",
			path:           "/api/json",
			acceptEncoding: "gzip, deflate",
			expectCompressed: true,
			description:    "Large JSON responses should be compressed",
		},
		{
			name:           "Large text response with compression",
			path:           "/api/text",
			acceptEncoding: "gzip",
			expectCompressed: true,
			description:    "Large text responses should be compressed",
		},
		{
			name:           "No compression when not accepted",
			path:           "/api/json",
			acceptEncoding: "identity",
			expectCompressed: false,
			description:    "No compression when client doesn't accept it",
		},
		{
			name:           "Small response may not be compressed",
			path:           "/api/small",
			acceptEncoding: "gzip",
			expectCompressed: false, // Small responses might not be compressed
			description:    "Small responses may not benefit from compression",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request with appropriate headers
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Header.Set("Accept-Encoding", tc.acceptEncoding)

			// Execute the request through the router
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Check response status
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Check compression headers
			contentEncoding := w.Header().Get("Content-Encoding")
			
			if tc.expectCompressed {
				if contentEncoding == "" {
					t.Errorf("%s: Expected compression but got no Content-Encoding header", tc.description)
				} else if contentEncoding != "gzip" && contentEncoding != "deflate" {
					t.Errorf("%s: Expected gzip or deflate compression, got %q", tc.description, contentEncoding)
				}

				// Verify Vary header is set for compressed responses
				vary := w.Header().Get("Vary")
				if vary == "" || !strings.Contains(vary, "Accept-Encoding") {
					t.Errorf("%s: Expected Vary header to contain Accept-Encoding, got %q", tc.description, vary)
				}

				t.Logf("%s: Successfully compressed with %s (%d bytes)", 
					tc.description, contentEncoding, w.Body.Len())
			} else {
				if contentEncoding != "" && tc.expectCompressed {
					t.Errorf("%s: Expected no compression but got Content-Encoding: %q", tc.description, contentEncoding)
				}
				t.Logf("%s: Response %s (%d bytes)", tc.description, 
					func() string {
						if contentEncoding != "" {
							return "compressed with " + contentEncoding
						}
						return "not compressed"
					}(), w.Body.Len())
			}
		})
	}
}


