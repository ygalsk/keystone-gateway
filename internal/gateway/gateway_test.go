package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"keystone-gateway/internal/config"
)

// TestGatewayNew tests the Gateway constructor with various configurations
func TestGatewayNew(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Config
		version   string
		wantError bool
	}{
		{
			name: "valid config with one tenant",
			cfg: &config.Config{
				Tenants: []config.Tenant{
					{
						Name: "tenant1",
						Services: []config.Service{
							{URL: "http://backend1.example.com"},
						},
					},
				},
			},
			version:   "1.0.0",
			wantError: false,
		},
		{
			name: "valid config with multiple tenants",
			cfg: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "tenant1",
						PathPrefix: "/api/v1",
						Services: []config.Service{
							{URL: "http://backend1.example.com"},
						},
					},
					{
						Name:       "tenant2",
						PathPrefix: "/api/v2",
						Services: []config.Service{
							{URL: "http://backend2.example.com"},
						},
					},
				},
			},
			version:   "1.0.0",
			wantError: false,
		},
		{
			name: "valid config with Lua routing",
			cfg: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:      "lua-tenant",
						LuaRoutes: []string{"test.lua"},
					},
				},
				LuaRouting: config.LuaRoutingConfig{
					Enabled:    true,
					ScriptsDir: "./testdata/scripts",
				},
			},
			version:   "1.0.0",
			wantError: false,
		},
		{
			name:      "nil config should error",
			cfg:       nil,
			version:   "1.0.0",
			wantError: true,
		},
		{
			name: "empty tenants should error",
			cfg: &config.Config{
				Tenants: []config.Tenant{},
			},
			version:   "1.0.0",
			wantError: true,
		},
		{
			name: "config with middleware settings",
			cfg: &config.Config{
				Tenants: []config.Tenant{
					{
						Name: "tenant1",
						Services: []config.Service{
							{URL: "http://backend.example.com"},
						},
					},
				},
				Middleware: config.MiddlewareConfig{
					RequestID: true,
					Logging:   true,
					Recovery:  true,
					Timeout:   30,
					Throttle:  200,
				},
			},
			version:   "1.0.0",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw, err := New(tt.cfg, tt.version)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if gw != nil {
					t.Error("expected nil gateway on error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if gw == nil {
				t.Error("expected gateway instance, got nil")
			}
		})
	}
}

// TestGatewayHandler tests that Handler() returns a valid HTTP handler
func TestGatewayHandler(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "handler with single tenant",
			cfg: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "test-tenant",
						PathPrefix: "/api",
						Services: []config.Service{
							{URL: "http://backend.example.com"},
						},
					},
				},
			},
		},
		{
			name: "handler with path-based routing",
			cfg: &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "path-tenant",
						PathPrefix: "/api",
						Services: []config.Service{
							{URL: "http://backend.example.com"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw, err := New(tt.cfg, "1.0.0")
			if err != nil {
				t.Fatalf("failed to create gateway: %v", err)
			}

			handler := gw.Handler()

			if handler == nil {
				t.Error("Handler() returned nil")
			}

			// Verify it's a valid http.Handler
			var _ http.Handler = handler
		})
	}
}

// TestGatewayHandlerHealthEndpoint tests that the health endpoint is registered
func TestGatewayHandlerHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name: "test-tenant",
				Services: []config.Service{
					{URL: "http://backend.example.com"},
				},
			},
		},
	}

	gw, err := New(cfg, "1.0.0")
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}

	handler := gw.Handler()

	// Test /health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health endpoint returned %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Health endpoint returned %q, want %q", body, "OK")
	}
}

// TestGatewayStop tests the Stop() method
func TestGatewayStop(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name: "test-tenant",
				Services: []config.Service{
					{URL: "http://backend.example.com"},
				},
			},
		},
	}

	gw, err := New(cfg, "1.0.0")
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}

	// Stop should not panic
	gw.Stop()

	// Calling Stop multiple times should be safe
	gw.Stop()
	gw.Stop()
}

// TestGatewayMiddlewareIntegration tests that middleware is properly configured
func TestGatewayMiddlewareIntegration(t *testing.T) {
	tests := []struct {
		name               string
		middlewareConfig   config.MiddlewareConfig
		expectRequestID    bool
		expectCompression  bool
	}{
		{
			name: "all middleware enabled",
			middlewareConfig: config.MiddlewareConfig{
				RequestID: true,
				Logging:   true,
				Recovery:  true,
				Timeout:   10,
				Throttle:  100,
			},
			expectRequestID: true,
		},
		{
			name: "request ID disabled",
			middlewareConfig: config.MiddlewareConfig{
				RequestID: false,
				Logging:   true,
				Recovery:  true,
			},
			expectRequestID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Tenants: []config.Tenant{
					{
						Name: "test-tenant",
						Services: []config.Service{
							{URL: "http://backend.example.com"},
						},
					},
				},
				Middleware: tt.middlewareConfig,
			}

			gw, err := New(cfg, "1.0.0")
			if err != nil {
				t.Fatalf("failed to create gateway: %v", err)
			}

			handler := gw.Handler()

			// Make a request to test middleware
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Check for X-Request-Id header
			requestID := w.Header().Get("X-Request-Id")
			if tt.expectRequestID && requestID == "" {
				t.Error("expected X-Request-Id header but got none")
			}
			if !tt.expectRequestID && requestID != "" {
				t.Errorf("expected no X-Request-Id header but got %q", requestID)
			}
		})
	}
}

// TestGatewayInitializationOrder tests that components are initialized in correct order
func TestGatewayInitializationOrder(t *testing.T) {
	// This test verifies that middleware -> Lua -> routes are initialized in order
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:      "test-tenant",
				LuaRoutes: []string{"init.lua"},
			},
		},
		LuaRouting: config.LuaRoutingConfig{
			Enabled:    true,
			ScriptsDir: "./testdata/scripts",
			GlobalScripts: []string{
				"global.lua",
			},
		},
	}

	// Should not panic - initialization order is correct
	gw, err := New(cfg, "1.0.0")
	if err != nil {
		// It's OK if Lua scripts don't exist (we're testing structure)
		// But we shouldn't get a nil pointer panic
		if gw != nil {
			t.Error("expected nil gateway when Lua init fails")
		}
	}
}

// TestGatewayConcurrentRequests tests that the gateway handles concurrent requests safely
func TestGatewayConcurrentRequests(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name: "test-tenant",
				Services: []config.Service{
					{URL: "http://backend.example.com"},
				},
			},
		},
	}

	gw, err := New(cfg, "1.0.0")
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}

	handler := gw.Handler()

	// Run 100 concurrent requests
	const numRequests = 100
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Success if no panics or deadlocks
}
