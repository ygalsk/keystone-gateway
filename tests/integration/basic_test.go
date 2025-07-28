package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/tests/fixtures"
)

// TestBasicIntegration tests basic gateway functionality with real components
func TestBasicIntegration(t *testing.T) {
	tests := []struct {
		name           string
		setupBackend   func() *httptest.Server
		setupConfig    func(backendURL string) 
		requestPath    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "basic_proxy_functionality",
			setupBackend: func() *httptest.Server {
				return fixtures.CreateSimpleBackend(t)
			},
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "health_check_integration", 
			setupBackend: func() *httptest.Server {
				return fixtures.CreateHealthCheckBackend(t)
			},
			requestPath:    "/health",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"healthy"}`,
		},
		{
			name: "error_handling_integration",
			setupBackend: func() *httptest.Server {
				return fixtures.CreateErrorBackend(t)
			},
			requestPath:    "/500",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup backend server
			backend := tt.setupBackend()
			defer backend.Close()
			
			// Create test configuration with backend URL
			cfg := fixtures.CreateTestConfig("test-tenant", "/")
			cfg.Tenants[0].Services[0].URL = backend.URL
			
			// Setup gateway environment
			env := fixtures.SetupGateway(t, cfg)
			defer env.Cleanup()
			
			// Make request through gateway
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			w := httptest.NewRecorder()
			
			env.Router.ServeHTTP(w, req)
			
			// Verify response
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			
			if body := strings.TrimSpace(w.Body.String()); body != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

// TestGatewayComponentIntegration tests integration between major gateway components
func TestGatewayComponentIntegration(t *testing.T) {
	t.Run("config_to_gateway_integration", func(t *testing.T) {
		// Setup backend
		backend := fixtures.CreateEchoBackend(t)
		defer backend.Close()
		
		// Create config
		cfg := fixtures.CreateTestConfig("echo-tenant", "/api/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		
		// Test config loading → gateway setup
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()
		
		// Verify gateway was configured correctly
		if env.Gateway == nil {
			t.Fatal("Gateway should be initialized")
		}
		
		if env.Config == nil {
			t.Fatal("Config should be set")
		}
		
		// Test request routing through configured gateway
		req := httptest.NewRequest("POST", "/api/test", strings.NewReader(`{"test": "data"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		env.Router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		// Verify request reached backend properly
		body, _ := io.ReadAll(w.Body)
		if !strings.Contains(string(body), "POST") {
			t.Error("Request method not properly forwarded to backend")
		}
	})
	
	t.Run("load_balancing_integration", func(t *testing.T) {
		// Setup multiple backends
		backend1 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/test": {StatusCode: 200, Body: "Backend 1 Response"},
			},
		})
		defer backend1.Close()
		
		backend2 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/test": {StatusCode: 200, Body: "Backend 2 Response"},
			},
		})
		defer backend2.Close()
		
		// Create config with multiple services
		cfg := fixtures.CreateTestConfig("lb-tenant", "/")
		cfg.Tenants[0].Services = []config.Service{
			{Name: "service1", URL: backend1.URL, Health: "/health"},
			{Name: "service2", URL: backend2.URL, Health: "/health"},
		}
		
		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()
		
		// Mark backends as alive for load balancing
		if tenantRouter := env.Gateway.GetTenantRouter("lb-tenant"); tenantRouter != nil {
			for _, backend := range tenantRouter.Backends {
				backend.Alive.Store(true)
			}
		}
		
		// Make multiple requests to test load balancing
		responses := make(map[string]int)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			
			env.Router.ServeHTTP(w, req)
			
			if w.Code == http.StatusOK {
				body := strings.TrimSpace(w.Body.String())
				responses[body]++
			}
		}
		
		// Verify load balancing occurred (both backends should receive requests)
		if len(responses) < 2 {
			t.Errorf("Expected load balancing between backends, got responses: %v", responses)
		}
	})
}

// TestErrorRecoveryIntegration tests error handling and recovery scenarios
func TestErrorRecoveryIntegration(t *testing.T) {
	t.Run("backend_failure_handling", func(t *testing.T) {
		// Setup backend that will be closed to simulate failure
		backend := fixtures.CreateSimpleBackend(t)
		backendURL := backend.URL
		backend.Close() // Close immediately to simulate failure
		
		// Create config with failed backend
		cfg := fixtures.CreateTestConfig("fail-tenant", "/")
		cfg.Tenants[0].Services[0].URL = backendURL
		
		// Setup gateway
		env := fixtures.SetupGateway(t, cfg) 
		defer env.Cleanup()
		
		// Make request to failed backend
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		env.Router.ServeHTTP(w, req)
		
		// Should handle failure gracefully (not panic)
		if w.Code != http.StatusBadGateway && w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected 502 or 503 for failed backend, got %d", w.Code)
		}
	})
	
	t.Run("timeout_handling", func(t *testing.T) {
		// Setup slow backend
		backend := fixtures.CreateSlowBackend(t, 2 * time.Second) // 2 second delay
		defer backend.Close()
		
		// Create config
		cfg := fixtures.CreateTestConfig("slow-tenant", "/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		
		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()
		
		// Make request with timeout expectations
		start := time.Now()
		req := httptest.NewRequest("GET", "/slow", nil)
		w := httptest.NewRecorder()
		
		env.Router.ServeHTTP(w, req)
		elapsed := time.Since(start)
		
		// Should either complete successfully or timeout appropriately
		if w.Code == http.StatusOK && elapsed < 1*time.Second {
			t.Error("Request completed too quickly for slow backend")
		}
		
		// Should not hang indefinitely
		if elapsed > 10*time.Second {
			t.Error("Request took too long - possible infinite hang")
		}
	})
}
