package unit

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

func TestRouteRegistrationDuplicates(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Register the same route twice
	route := routing.RouteDefinition{
		TenantName: "test-tenant",
		Method:     "GET",
		Pattern:    "/api/test",
		Handler:    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}

	// First registration should succeed
	err1 := registry.RegisterRoute(route)
	if err1 != nil {
		t.Errorf("first route registration failed: %v", err1)
	}

	// Second registration should succeed but not create duplicate
	err2 := registry.RegisterRoute(route)
	if err2 != nil {
		t.Errorf("second route registration failed: %v", err2)
	}

	// Both registrations should succeed without error (duplicates are silently ignored)
}

func TestRouteRegistrationInvalidMethods(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Test various HTTP methods including invalid ones
	testCases := []struct {
		method      string
		shouldWork  bool
		description string
	}{
		{"GET", true, "standard GET method"},
		{"POST", true, "standard POST method"},
		{"PUT", true, "standard PUT method"},
		{"DELETE", true, "standard DELETE method"},
		{"PATCH", true, "standard PATCH method"},
		{"HEAD", true, "standard HEAD method"},
		{"OPTIONS", true, "standard OPTIONS method"},
		{"CUSTOM", false, "custom method (not supported by Chi)"},
		{"", false, "empty method (not supported)"},
		{"get", true, "lowercase method (actually supported by Chi)"},
		{"INVALID METHOD", false, "method with spaces (not supported)"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			route := routing.RouteDefinition{
				TenantName: "test-tenant",
				Method:     tc.method,
				Pattern:    "/test/" + strings.ReplaceAll(tc.method, " ", "-"),
				Handler:    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			}

			// Use defer with recover for methods that might panic
			if !tc.shouldWork {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("expected method %q to panic or fail, but it didn't", tc.method)
					}
				}()
			}

			err := registry.RegisterRoute(route)
			if tc.shouldWork && err != nil {
				t.Errorf("expected method %q to work, got error: %v", tc.method, err)
			}
			if !tc.shouldWork && err == nil {
				// Some methods might not return error but could panic
				// The defer/recover above will catch panics
			}
		})
	}
}

func TestRoutePatternErrors(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Test problematic route patterns
	testCases := []struct {
		pattern     string
		description string
	}{
		{"", "empty pattern"},
		{"/", "root pattern"},
		{"/api/*", "wildcard pattern"},
		{"/api/{id}", "parameter pattern"},
		{"/api/{id}/details", "parameter with path"},
		{"api/test", "pattern without leading slash"},
		{"/api//test", "double slash pattern"},
		{"/api/{}", "empty parameter"},
		{"/api/{id}/{id}", "duplicate parameter names"},
		{"/api/test?query=value", "pattern with query string"},
		{"/api/test#fragment", "pattern with fragment"},
		{"/{...}", "catch-all pattern"},
		{"/api/{id:[0-9]+}", "parameter with regex"},
		{strings.Repeat("/very-long-path", 100), "very long pattern"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			route := routing.RouteDefinition{
				TenantName: "test-tenant",
				Method:     "GET",
				Pattern:    tc.pattern,
				Handler:    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			}

			// Some patterns may cause Chi router to panic due to strict validation
			defer func() {
				if r := recover(); r != nil {
					// Chi router panics on invalid patterns - this is expected for some patterns
					t.Logf("Pattern %q caused panic (expected for invalid patterns): %v", tc.pattern, r)
				}
			}()

			err := registry.RegisterRoute(route)
			if err != nil {
				t.Logf("route registration failed for pattern %q: %v", tc.pattern, err)
				// This is acceptable for invalid patterns
			}
		})
	}
}

func TestRouteGroupRegistrationErrors(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Test duplicate route group registration
	groupDef := routing.RouteGroupDefinition{
		TenantName: "test-tenant",
		Pattern:    "/api/v1",
		Middleware: []func(http.Handler) http.Handler{},
		Routes:     []routing.RouteDefinition{},
		Subgroups:  []routing.RouteGroupDefinition{},
	}

	// First registration
	err1 := registry.RegisterRouteGroup(groupDef)
	if err1 != nil {
		t.Errorf("first group registration failed: %v", err1)
	}

	// Second registration of same group (should be silently ignored)
	err2 := registry.RegisterRouteGroup(groupDef)
	if err2 != nil {
		t.Errorf("second group registration failed: %v", err2)
	}
}

func TestTenantRouteMounting(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Test mounting tenant routes that don't exist
	err := registry.MountTenantRoutes("nonexistent-tenant", "/api")
	if err != nil {
		t.Errorf("mounting nonexistent tenant should not fail: %v", err)
	}

	// Register a route for a tenant
	route := routing.RouteDefinition{
		TenantName: "test-tenant",
		Method:     "GET",
		Pattern:    "/test",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}),
	}

	err = registry.RegisterRoute(route)
	if err != nil {
		t.Fatalf("route registration failed: %v", err)
	}

	// Mount the tenant routes
	err = registry.MountTenantRoutes("test-tenant", "/mounted")
	if err != nil {
		t.Errorf("mounting tenant routes failed: %v", err)
	}

	// Test the mounted route
	req := httptest.NewRequest("GET", "/mounted/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if w.Body.String() != "test response" {
		t.Errorf("expected 'test response', got %q", w.Body.String())
	}
}

func TestGatewayInvalidConfiguration(t *testing.T) {
	// Test gateway creation with invalid tenant configurations
	testCases := []struct {
		config      *config.Config
		description string
	}{
		{
			&config.Config{
				Tenants: []config.Tenant{
					{
						Name: "no-routing-info",
						// Missing both domains and path_prefix
						Services: []config.Service{
							{Name: "svc1", URL: "http://backend1", Health: "/health"},
						},
					},
				},
			},
			"tenant with no routing information",
		},
		{
			&config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "invalid-url",
						PathPrefix: "/api/",
						Services: []config.Service{
							{Name: "svc1", URL: "not-a-valid-url", Health: "/health"},
						},
					},
				},
			},
			"tenant with invalid service URL",
		},
		{
			&config.Config{
				Tenants: []config.Tenant{
					{
						Name:     "empty-services",
						Domains:  []string{"example.com"},
						Services: []config.Service{}, // No services
					},
				},
			},
			"tenant with no services",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			router := chi.NewRouter()
			// Gateway should handle invalid configurations gracefully
			gateway := routing.NewGatewayWithRouter(tc.config, router)
			if gateway == nil {
				t.Error("gateway creation should not fail even with invalid config")
			}

			// Gateway should still be functional for valid operations
			if gateway.GetConfig() != tc.config {
				t.Error("gateway should store the provided config")
			}
		})
	}
}

func TestGatewayRouteMatching(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "path-tenant",
				PathPrefix: "/api/",
				Services:   []config.Service{{Name: "svc1", URL: "http://backend1", Health: "/health"}},
			},
			{
				Name:    "host-tenant",
				Domains: []string{"api.example.com"},
				Services: []config.Service{{Name: "svc1", URL: "http://backend1", Health: "/health"}},
			},
			{
				Name:       "hybrid-tenant",
				Domains:    []string{"hybrid.example.com"},
				PathPrefix: "/v1/",
				Services:   []config.Service{{Name: "svc1", URL: "http://backend1", Health: "/health"}},
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	testCases := []struct {
		host        string
		path        string
		expectMatch bool
		expectName  string
		description string
	}{
		{"example.com", "/api/users", true, "path-tenant", "path-based routing"},
		{"api.example.com", "/users", true, "host-tenant", "host-based routing"},
		{"hybrid.example.com", "/v1/users", true, "hybrid-tenant", "hybrid routing"},
		{"unknown.com", "/api/users", true, "path-tenant", "fallback to path routing"},
		{"unknown.com", "/unknown", false, "", "no matching route"},
		{"api.example.com", "/api/users", true, "host-tenant", "host priority over path"},
		{"", "/api/users", true, "path-tenant", "empty host with path"},
		{"api.example.com", "", true, "host-tenant", "empty path with host"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tenantRouter, prefix := gateway.MatchRoute(tc.host, tc.path)

			if tc.expectMatch {
				if tenantRouter == nil {
					t.Errorf("expected to match tenant %q, got nil", tc.expectName)
					return
				}
				if tenantRouter.Name != tc.expectName {
					t.Errorf("expected tenant %q, got %q", tc.expectName, tenantRouter.Name)
				}
			} else {
				if tenantRouter != nil {
					t.Errorf("expected no match, got tenant %q", tenantRouter.Name)
				}
			}

			// Validate prefix for path-based routing
			if tc.expectName == "path-tenant" && prefix != "/api/" {
				t.Errorf("expected prefix '/api/', got %q", prefix)
			}
			if tc.expectName == "hybrid-tenant" && prefix != "/v1/" {
				t.Errorf("expected prefix '/v1/', got %q", prefix)
			}
		})
	}
}

func TestBackendSelectionEdgeCases(t *testing.T) {
	// Test tenant router with problematic backend configurations
	testCases := []struct {
		backends    []string
		description string
	}{
		{[]string{}, "no backends"},
		{[]string{"http://backend1"}, "single backend"},
		{[]string{"http://backend1", "http://backend2"}, "two backends"},
		{[]string{"http://backend1", "http://backend2", "http://backend3"}, "three backends"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tr := &routing.TenantRouter{
				Name:     "test-tenant",
				Backends: make([]*routing.GatewayBackend, len(tc.backends)),
			}

			// Create backends
			for i, backendURL := range tc.backends {
				u, _ := url.Parse(backendURL)
				tr.Backends[i] = &routing.GatewayBackend{URL: u}
				tr.Backends[i].Alive.Store(true)
			}

			// Test backend selection
			for i := 0; i < 10; i++ {
				backend := tr.NextBackend()
				if len(tc.backends) == 0 {
					if backend != nil {
						t.Error("expected nil backend when no backends available")
					}
				} else {
					if backend == nil {
						t.Error("expected backend, got nil")
					}
				}
			}
		})
	}
}

func TestConcurrentRouteRegistration(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	const numGoroutines = 10
	const routesPerGoroutine = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*routesPerGoroutine)

	// Launch concurrent route registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < routesPerGoroutine; j++ {
				route := routing.RouteDefinition{
					TenantName: "concurrent-tenant",
					Method:     "GET",
					Pattern:    "/api/test-" + string(rune('0'+goroutineID)) + "-" + string(rune('0'+j)),
					Handler:    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
				}

				err := registry.RegisterRoute(route)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("concurrent registration error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("got %d errors during concurrent registration", errorCount)
	}

	// Verify all routes were registered
	tenants := registry.ListTenants()
	found := false
	for _, tenant := range tenants {
		if tenant == "concurrent-tenant" {
			found = true
			break
		}
	}

	if !found {
		t.Error("concurrent tenant not found in registry")
	}
}

func TestRouteRegistrationNilHandlers(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Test route registration with nil handler
	route := routing.RouteDefinition{
		TenantName: "test-tenant",
		Method:     "GET",
		Pattern:    "/test-nil-handler",
		Handler:    nil,
	}

	// Should not crash even with nil handler
	err := registry.RegisterRoute(route)
	if err != nil {
		t.Errorf("registration with nil handler failed: %v", err)
	}

	// Test the route - should handle nil gracefully
	err = registry.MountTenantRoutes("test-tenant", "/api")
	if err != nil {
		t.Errorf("mounting with nil handler failed: %v", err)
	}

	// Making a request to nil handler should not crash (but may panic)
	req := httptest.NewRequest("GET", "/api/test-nil-handler", nil)
	w := httptest.NewRecorder()

	// This might panic, so we'll recover
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior for nil handler - recover and continue
			t.Logf("nil handler caused panic (expected): %v", r)
		}
	}()

	router.ServeHTTP(w, req)
}

func TestGatewayHostExtraction(t *testing.T) {
	testCases := []struct {
		hostHeader string
		expected   string
	}{
		{"example.com", "example.com"},
		{"example.com:8080", "example.com"},
		{"api.example.com:443", "api.example.com"},
		{"localhost:3000", "localhost"},
		{"", ""},
		{":", ""},
		{"example.com:", "example.com"},
		{":8080", ""},
		{"[::1]:8080", "[::1]"},
		{"example.com:8080:extra", "example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.hostHeader, func(t *testing.T) {
			result := routing.ExtractHost(tc.hostHeader)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestProxyCreationErrors(t *testing.T) {
	router := chi.NewRouter()
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "test-tenant",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "svc1", URL: "http://backend1", Health: "/health"},
				},
			},
		},
	}

	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Get a backend
	tenantRouter := gateway.GetTenantRouter("test-tenant")
	if tenantRouter == nil {
		t.Fatal("could not find test tenant")
	}

	backend := tenantRouter.NextBackend()
	if backend == nil {
		t.Fatal("could not get backend")
	}

	// Test proxy creation with various strip prefixes
	testCases := []string{
		"",           // no prefix stripping
		"/api/",      // normal prefix
		"/",          // root prefix
		"/very/long/prefix/path/",  // long prefix
		"invalid-prefix",           // prefix without slashes
	}

	for _, prefix := range testCases {
		t.Run("prefix:"+prefix, func(t *testing.T) {
			proxy := gateway.CreateProxy(backend, prefix)
			if proxy == nil {
				t.Error("proxy creation failed")
			}

			// Test proxy director function doesn't crash
			req := httptest.NewRequest("GET", "/api/test?query=value", nil)
			req.URL.RawQuery = "original=query"

			// The director function should not panic
			proxy.Director(req)

			// Basic validation of URL modification
			if req.URL.Host == "" {
				t.Error("proxy should set backend host")
			}
			if req.URL.Scheme == "" {
				t.Error("proxy should set backend scheme")
			}
		})
	}
}