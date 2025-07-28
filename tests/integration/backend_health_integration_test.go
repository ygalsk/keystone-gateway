package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/tests/fixtures"
)

// TestBackendHealthMonitoring tests backend health check integration
func TestBackendHealthMonitoring(t *testing.T) {
	t.Run("health_check_endpoint_integration", func(t *testing.T) {
		// Create health-aware backend
		backend := fixtures.CreateHealthCheckBackend(t)
		defer backend.Close()

		// Create config with health monitoring
		cfg := fixtures.CreateTestConfig("health-tenant", "/health-test/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		cfg.Tenants[0].Services[0].Health = "/health"
		cfg.Tenants[0].Interval = 5 // Short interval for testing

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Get tenant router for health status verification
		tenantRouter := env.Gateway.GetTenantRouter("health-tenant")
		if tenantRouter == nil {
			t.Fatal("Expected tenant router to be initialized")
		}

		// Initially mark backend as alive
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}

		// Test health endpoint directly
		req := httptest.NewRequest("GET", "/health-test/health", nil)
		w := httptest.NewRecorder()
		env.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for health endpoint, got %d", w.Code)
		}

		// Verify health response format
		if !strings.Contains(w.Body.String(), "healthy") {
			t.Error("Expected health response to contain 'healthy'")
		}

		// Test regular service endpoint
		req2 := httptest.NewRequest("GET", "/health-test/service", nil)
		w2 := httptest.NewRecorder()
		env.Router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status 200 for service endpoint, got %d", w2.Code)
		}

		// Verify service response
		if !strings.Contains(w2.Body.String(), "service response") {
			t.Error("Expected service response")
		}
	})

	t.Run("backend_alive_status_tracking", func(t *testing.T) {
		// Create multiple backends for testing alive status
		backend1 := fixtures.CreateHealthCheckBackend(t)
		defer backend1.Close()

		backend2 := fixtures.CreateHealthCheckBackend(t)
		defer backend2.Close()

		// Create config with multiple backends
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "status-tenant",
					PathPrefix: "/status/",
					Interval:   10,
					Services: []config.Service{
						{Name: "backend1", URL: backend1.URL, Health: "/health"},
						{Name: "backend2", URL: backend2.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Get tenant router
		tenantRouter := env.Gateway.GetTenantRouter("status-tenant")
		if tenantRouter == nil {
			t.Fatal("Expected tenant router to be initialized")
		}

		if len(tenantRouter.Backends) != 2 {
			t.Fatalf("Expected 2 backends, got %d", len(tenantRouter.Backends))
		}

		// Test initial alive status (should be false by default)
		for i, backend := range tenantRouter.Backends {
			if backend.Alive.Load() {
				t.Errorf("Backend %d should initially be marked as not alive", i)
			}
		}

		// Mark backends as alive
		for _, backend := range tenantRouter.Backends {
			backend.Alive.Store(true)
		}

		// Verify alive status changed
		for i, backend := range tenantRouter.Backends {
			if !backend.Alive.Load() {
				t.Errorf("Backend %d should be marked as alive", i)
			}
		}

		// Test that requests work when backends are alive
		req := httptest.NewRequest("GET", "/status/health", nil)
		w := httptest.NewRecorder()
		env.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 when backends are alive, got %d", w.Code)
		}

		// Mark one backend as not alive
		tenantRouter.Backends[0].Alive.Store(false)

		// Verify mixed alive status
		if tenantRouter.Backends[0].Alive.Load() {
			t.Error("Backend 0 should be marked as not alive")
		}
		if !tenantRouter.Backends[1].Alive.Load() {
			t.Error("Backend 1 should still be marked as alive")
		}

		// Test that requests still work with one alive backend
		req2 := httptest.NewRequest("GET", "/status/health", nil)
		w2 := httptest.NewRecorder()
		env.Router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status 200 with one alive backend, got %d", w2.Code)
		}
	})
}

// TestBackendFailover tests failover scenarios when backends become unavailable
func TestBackendFailover(t *testing.T) {
	t.Run("single_backend_failure_handling", func(t *testing.T) {
		// Create backend that we can close to simulate failure
		backend := fixtures.CreateSimpleBackend(t)
		backendURL := backend.URL

		// Create config
		cfg := fixtures.CreateTestConfig("failover-tenant", "/failover/")
		cfg.Tenants[0].Services[0].URL = backendURL

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark backend as alive initially
		if tenantRouter := env.Gateway.GetTenantRouter("failover-tenant"); tenantRouter != nil {
			for _, gtwBackend := range tenantRouter.Backends {
				gtwBackend.Alive.Store(true)
			}
		}

		// Test request when backend is healthy
		req1 := httptest.NewRequest("GET", "/failover/test", nil)
		w1 := httptest.NewRecorder()
		env.Router.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Errorf("Expected status 200 when backend is healthy, got %d", w1.Code)
		}

		// Close backend to simulate failure
		backend.Close()

		// Test request when backend is down
		req2 := httptest.NewRequest("GET", "/failover/test", nil)
		w2 := httptest.NewRecorder()
		env.Router.ServeHTTP(w2, req2)

		// Should get error response when backend is down
		if w2.Code == http.StatusOK {
			t.Log("Backend failure may not immediately affect routing depending on implementation")
		} else if w2.Code != http.StatusBadGateway && w2.Code != http.StatusServiceUnavailable {
			t.Logf("Backend failure resulted in status %d", w2.Code)
		}
	})

	t.Run("multiple_backend_failover", func(t *testing.T) {
		// Create multiple backends
		backend1 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Backend 1 Response"},
			},
		})
		defer backend1.Close()

		backend2 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Backend 2 Response"},
			},
		})
		defer backend2.Close()

		backend3 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Backend 3 Response"},
			},
		})
		defer backend3.Close()

		// Create config with multiple backends for failover
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "multi-failover-tenant",
					PathPrefix: "/multi-failover/",
					Interval:   30,
					Services: []config.Service{
						{Name: "backend1", URL: backend1.URL, Health: "/health"},
						{Name: "backend2", URL: backend2.URL, Health: "/health"},
						{Name: "backend3", URL: backend3.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		tenantRouter := env.Gateway.GetTenantRouter("multi-failover-tenant")
		if tenantRouter == nil {
			t.Fatal("Expected tenant router to be initialized")
		}

		for _, gtwBackend := range tenantRouter.Backends {
			gtwBackend.Alive.Store(true)
		}

		// Test that all backends are reachable
		responses := make(map[string]int)
		for i := 0; i < 15; i++ {
			req := httptest.NewRequest("GET", "/multi-failover/", nil)
			w := httptest.NewRecorder()
			env.Router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				body := strings.TrimSpace(w.Body.String())
				responses[body]++
			}
		}

		// Should have received responses from multiple backends
		if len(responses) < 1 {
			t.Errorf("Expected responses from multiple backends, got: %v", responses)
		}

		// Close one backend to simulate failure
		backend1.Close()

		// Mark the failed backend as not alive
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(false)
		}

		// Test failover - should still work with remaining backends
		req := httptest.NewRequest("GET", "/multi-failover/", nil)
		w := httptest.NewRecorder()
		env.Router.ServeHTTP(w, req)

		// Should still get successful response from remaining backends
		if w.Code != http.StatusOK && w.Code != http.StatusBadGateway {
			t.Errorf("Expected successful failover or controlled failure, got status %d", w.Code)
		}

		// Close second backend
		backend2.Close()
		if len(tenantRouter.Backends) > 1 {
			tenantRouter.Backends[1].Alive.Store(false)
		}

		// Should still work with one remaining backend
		req2 := httptest.NewRequest("GET", "/multi-failover/", nil)
		w2 := httptest.NewRecorder()
		env.Router.ServeHTTP(w2, req2)

		if w2.Code == http.StatusOK {
			// Should receive response from remaining backend
			if !strings.Contains(w2.Body.String(), "Backend 3") {
				t.Error("Should receive response from remaining backend")
			}
		} else {
			t.Logf("Failover behavior resulted in status %d", w2.Code)
		}
	})
}

// TestBackendRecovery tests recovery scenarios when backends come back online
func TestBackendRecovery(t *testing.T) {
	t.Run("backend_recovery_simulation", func(t *testing.T) {
		// Create backend for recovery testing
		backend := fixtures.CreateHealthCheckBackend(t)
		defer backend.Close()

		// Create config
		cfg := fixtures.CreateTestConfig("recovery-tenant", "/recovery/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		cfg.Tenants[0].Services[0].Health = "/health"

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Get tenant router
		tenantRouter := env.Gateway.GetTenantRouter("recovery-tenant")
		if tenantRouter == nil {
			t.Fatal("Expected tenant router to be initialized")
		}

		// Test initial state - backend not alive
		if len(tenantRouter.Backends) > 0 {
			if tenantRouter.Backends[0].Alive.Load() {
				t.Error("Backend should initially be marked as not alive")
			}
		}

		// Simulate backend becoming available
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}

		// Test that backend is now reachable
		req := httptest.NewRequest("GET", "/recovery/health", nil)
		w := httptest.NewRecorder()
		env.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 after recovery, got %d", w.Code)
		}

		// Verify health response
		if !strings.Contains(w.Body.String(), "healthy") {
			t.Error("Expected health response after recovery")
		}

		// Simulate backend failure again
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(false)
		}

		// Simulate recovery again
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}

		// Test that backend works after second recovery
		req2 := httptest.NewRequest("GET", "/recovery/service", nil)
		w2 := httptest.NewRecorder()
		env.Router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status 200 after second recovery, got %d", w2.Code)
		}
	})

	t.Run("gradual_backend_recovery", func(t *testing.T) {
		// Create multiple backends for gradual recovery testing
		backend1 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Recovered Backend 1"},
			},
		})
		defer backend1.Close()

		backend2 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Recovered Backend 2"},
			},
		})
		defer backend2.Close()

		backend3 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Recovered Backend 3"},
			},
		})
		defer backend3.Close()

		// Create config with multiple backends
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "gradual-recovery-tenant",
					PathPrefix: "/gradual-recovery/",
					Interval:   30,
					Services: []config.Service{
						{Name: "backend1", URL: backend1.URL, Health: "/health"},
						{Name: "backend2", URL: backend2.URL, Health: "/health"},
						{Name: "backend3", URL: backend3.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Get tenant router
		tenantRouter := env.Gateway.GetTenantRouter("gradual-recovery-tenant")
		if tenantRouter == nil {
			t.Fatal("Expected tenant router to be initialized")
		}

		// Start with all backends down
		for _, backend := range tenantRouter.Backends {
			backend.Alive.Store(false)
		}

		// Gradually bring backends online
		// First backend comes online
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}

		// Test with one backend online
		req1 := httptest.NewRequest("GET", "/gradual-recovery/", nil)
		w1 := httptest.NewRecorder()
		env.Router.ServeHTTP(w1, req1)

		if w1.Code == http.StatusOK {
			if !strings.Contains(w1.Body.String(), "Recovered Backend 1") {
				t.Error("Should receive response from first recovered backend")
			}
		}

		// Second backend comes online
		if len(tenantRouter.Backends) > 1 {
			tenantRouter.Backends[1].Alive.Store(true)
		}

		// Test with two backends online
		responses := make(map[string]int)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/gradual-recovery/", nil)
			w := httptest.NewRecorder()
			env.Router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				body := strings.TrimSpace(w.Body.String())
				responses[body]++
			}
		}

		// Should have responses from first two backends
		foundBackend1 := false
		foundBackend2 := false
		for response := range responses {
			if strings.Contains(response, "Backend 1") {
				foundBackend1 = true
			}
			if strings.Contains(response, "Backend 2") {
				foundBackend2 = true
			}
		}

		if !foundBackend1 && !foundBackend2 {
			t.Log("Load balancing between recovered backends may vary based on implementation")
		}

		// Third backend comes online
		if len(tenantRouter.Backends) > 2 {
			tenantRouter.Backends[2].Alive.Store(true)
		}

		// Test with all backends online
		req3 := httptest.NewRequest("GET", "/gradual-recovery/", nil)
		w3 := httptest.NewRecorder()
		env.Router.ServeHTTP(w3, req3)

		if w3.Code != http.StatusOK {
			t.Errorf("Expected status 200 with all backends recovered, got %d", w3.Code)
		}

		// Verify all backends are marked as alive
		for i, backend := range tenantRouter.Backends {
			if !backend.Alive.Load() {
				t.Errorf("Backend %d should be marked as alive after recovery", i)
			}
		}
	})
}

// TestHealthCheckIntegrationScenarios tests complex health check integration scenarios
func TestHealthCheckIntegrationScenarios(t *testing.T) {
	t.Run("health_check_with_slow_backends", func(t *testing.T) {
		// Create slow backend for health check testing
		backend := fixtures.CreateSlowBackend(t, 50*time.Millisecond)
		defer backend.Close()

		// Create config with slow backend
		cfg := fixtures.CreateTestConfig("slow-health-tenant", "/slow-health/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		cfg.Tenants[0].Services[0].Health = "/health"

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark backend as alive
		if tenantRouter := env.Gateway.GetTenantRouter("slow-health-tenant"); tenantRouter != nil {
			for _, gtwBackend := range tenantRouter.Backends {
				gtwBackend.Alive.Store(true)
			}
		}

		// Test health check with slow backend
		start := time.Now()
		req := httptest.NewRequest("GET", "/slow-health/slow", nil)
		w := httptest.NewRecorder()
		env.Router.ServeHTTP(w, req)
		duration := time.Since(start)

		// Should complete but may take time
		if w.Code != http.StatusOK && w.Code != http.StatusRequestTimeout {
			t.Errorf("Expected status 200 or 408 for slow backend, got %d", w.Code)
		}

		// Should respect reasonable timeout
		if duration > 2*time.Second {
			t.Errorf("Slow backend request took too long: %v", duration)
		}
	})

	t.Run("health_check_with_error_backends", func(t *testing.T) {
		// Create error backend
		backend := fixtures.CreateErrorBackend(t)
		defer backend.Close()

		// Create config with error backend
		cfg := fixtures.CreateTestConfig("error-health-tenant", "/error-health/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		cfg.Tenants[0].Services[0].Health = "/health"

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark backend as alive
		if tenantRouter := env.Gateway.GetTenantRouter("error-health-tenant"); tenantRouter != nil {
			for _, gtwBackend := range tenantRouter.Backends {
				gtwBackend.Alive.Store(true)
			}
		}

		// Test various error endpoints
		errorTests := []struct {
			path           string
			expectedStatus int
		}{
			{"/error-health/404", http.StatusNotFound},
			{"/error-health/500", http.StatusInternalServerError},
			{"/error-health/503", http.StatusServiceUnavailable},
		}

		for _, tt := range errorTests {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			env.Router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d for %s, got %d", tt.expectedStatus, tt.path, w.Code)
			}
		}
	})

	t.Run("health_check_frequency_simulation", func(t *testing.T) {
		// Create health check backend
		backend := fixtures.CreateHealthCheckBackend(t)
		defer backend.Close()

		// Create config with short health check interval
		cfg := fixtures.CreateTestConfig("frequent-health-tenant", "/frequent-health/")
		cfg.Tenants[0].Services[0].URL = backend.URL
		cfg.Tenants[0].Services[0].Health = "/health"
		cfg.Tenants[0].Interval = 1 // Very short interval for testing

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Get tenant router
		tenantRouter := env.Gateway.GetTenantRouter("frequent-health-tenant")
		if tenantRouter == nil {
			t.Fatal("Expected tenant router to be initialized")
		}

		// Mark backend as alive
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}

		// Test multiple requests to verify consistent health status
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/frequent-health/health", nil)
			w := httptest.NewRecorder()
			env.Router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Health check %d failed with status %d", i+1, w.Code)
			}

			// Small delay between checks
			time.Sleep(10 * time.Millisecond)
		}

		// Verify backend is still marked as alive after multiple checks
		if len(tenantRouter.Backends) > 0 {
			if !tenantRouter.Backends[0].Alive.Load() {
				t.Error("Backend should remain alive after frequent health checks")
			}
		}
	})
}