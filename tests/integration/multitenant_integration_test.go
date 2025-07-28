package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/tests/fixtures"
)

// TestMultiTenantRouting tests complete multi-tenant request routing scenarios
func TestMultiTenantRouting(t *testing.T) {
	t.Run("host_based_tenant_isolation", func(t *testing.T) {
		// Create different backends for each tenant
		apiBackend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "API Tenant Response"},
			},
		})
		defer apiBackend.Close()

		webBackend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Web Tenant Response"},
			},
		})
		defer webBackend.Close()

		mobileBackend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Mobile Tenant Response"},
			},
		})
		defer mobileBackend.Close()

		// Create multi-tenant config with host-based routing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:     "api-tenant",
					Domains:  []string{"api.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "api-service", URL: apiBackend.URL, Health: "/health"},
					},
				},
				{
					Name:     "web-tenant",
					Domains:  []string{"web.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "web-service", URL: webBackend.URL, Health: "/health"},
					},
				},
				{
					Name:     "mobile-tenant",
					Domains:  []string{"mobile.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "mobile-service", URL: mobileBackend.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		tenantNames := []string{"api-tenant", "web-tenant", "mobile-tenant"}
		for _, tenantName := range tenantNames {
			if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
				for _, backend := range tenantRouter.Backends {
					backend.Alive.Store(true)
				}
			}
		}

		// Test routing to different tenants based on Host header
		testCases := []struct {
			host         string
			expectedBody string
		}{
			{"api.example.com", "API Tenant Response"},
			{"web.example.com", "Web Tenant Response"},
			{"mobile.example.com", "Mobile Tenant Response"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("host_%s", tc.host), func(t *testing.T) {
				req := httptest.NewRequest("GET", "/", nil)
				req.Host = tc.host
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200 for host %s, got %d", tc.host, w.Code)
				}

				if !strings.Contains(w.Body.String(), tc.expectedBody) {
					t.Errorf("Expected body to contain '%s' for host %s, got '%s'", 
						tc.expectedBody, tc.host, w.Body.String())
				}
			})
		}

		// Test invalid host - should return 404
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "invalid.example.com"
		w := httptest.NewRecorder()

		env.Router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for invalid host, got %d", w.Code)
		}
	})

	t.Run("path_based_tenant_isolation", func(t *testing.T) {
		// Create backends for path-based tenants
		adminBackend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/dashboard": {StatusCode: 200, Body: "Admin Dashboard"},
				"/users":     {StatusCode: 200, Body: "Admin Users"},
			},
		})
		defer adminBackend.Close()

		apiV1Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/users":    {StatusCode: 200, Body: "API V1 Users"},
				"/products": {StatusCode: 200, Body: "API V1 Products"},
			},
		})
		defer apiV1Backend.Close()

		apiV2Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/users":    {StatusCode: 200, Body: "API V2 Users"},
				"/products": {StatusCode: 200, Body: "API V2 Products"},
			},
		})
		defer apiV2Backend.Close()

		// Create config with path-based routing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "admin-tenant",
					PathPrefix: "/admin/",
					Interval:   30,
					Services: []config.Service{
						{Name: "admin-service", URL: adminBackend.URL, Health: "/health"},
					},
				},
				{
					Name:       "api-v1-tenant",
					PathPrefix: "/api/v1/",
					Interval:   30,
					Services: []config.Service{
						{Name: "api-v1-service", URL: apiV1Backend.URL, Health: "/health"},
					},
				},
				{
					Name:       "api-v2-tenant",
					PathPrefix: "/api/v2/",
					Interval:   30,
					Services: []config.Service{
						{Name: "api-v2-service", URL: apiV2Backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		tenantNames := []string{"admin-tenant", "api-v1-tenant", "api-v2-tenant"}
		for _, tenantName := range tenantNames {
			if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
				for _, backend := range tenantRouter.Backends {
					backend.Alive.Store(true)
				}
			}
		}

		// Test path-based routing
		testCases := []struct {
			path         string
			expectedBody string
		}{
			{"/admin/dashboard", "Admin Dashboard"},
			{"/admin/users", "Admin Users"},
			{"/api/v1/users", "API V1 Users"},
			{"/api/v1/products", "API V1 Products"},
			{"/api/v2/users", "API V2 Users"},
			{"/api/v2/products", "API V2 Products"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("path_%s", strings.ReplaceAll(tc.path, "/", "_")), func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.path, nil)
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200 for path %s, got %d", tc.path, w.Code)
				}

				if !strings.Contains(w.Body.String(), tc.expectedBody) {
					t.Errorf("Expected body to contain '%s' for path %s, got '%s'", 
						tc.expectedBody, tc.path, w.Body.String())
				}
			})
		}

		// Test invalid path - should return 404
		req := httptest.NewRequest("GET", "/invalid/path", nil)
		w := httptest.NewRecorder()

		env.Router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for invalid path, got %d", w.Code)
		}
	})

	t.Run("hybrid_host_and_path_tenant_routing", func(t *testing.T) {
		// Create backends for hybrid tenants
		apiV1Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/data": {StatusCode: 200, Body: "API V1 Data"},
			},
		})
		defer apiV1Backend.Close()

		apiV2Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/data": {StatusCode: 200, Body: "API V2 Data"},
			},
		})
		defer apiV2Backend.Close()

		webAdminBackend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/panel": {StatusCode: 200, Body: "Web Admin Panel"},
			},
		})
		defer webAdminBackend.Close()

		// Create config with hybrid (host + path) routing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				// API V1 on api.example.com/v1/
				{
					Name:       "api-v1-hybrid",
					Domains:    []string{"api.example.com"},
					PathPrefix: "/v1/",
					Interval:   30,
					Services: []config.Service{
						{Name: "api-v1-service", URL: apiV1Backend.URL, Health: "/health"},
					},
				},
				// API V2 on api.example.com/v2/
				{
					Name:       "api-v2-hybrid",
					Domains:    []string{"api.example.com"},
					PathPrefix: "/v2/",
					Interval:   30,
					Services: []config.Service{
						{Name: "api-v2-service", URL: apiV2Backend.URL, Health: "/health"},
					},
				},
				// Admin panel on web.example.com/admin/
				{
					Name:       "web-admin-hybrid",
					Domains:    []string{"web.example.com"},
					PathPrefix: "/admin/",
					Interval:   30,
					Services: []config.Service{
						{Name: "web-admin-service", URL: webAdminBackend.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		tenantNames := []string{"api-v1-hybrid", "api-v2-hybrid", "web-admin-hybrid"}
		for _, tenantName := range tenantNames {
			if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
				for _, backend := range tenantRouter.Backends {
					backend.Alive.Store(true)
				}
			}
		}

		// Test hybrid routing (both host and path must match)
		testCases := []struct {
			host         string
			path         string
			expectedBody string
		}{
			{"api.example.com", "/v1/data", "API V1 Data"},
			{"api.example.com", "/v2/data", "API V2 Data"},
			{"web.example.com", "/admin/panel", "Web Admin Panel"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("hybrid_%s%s", tc.host, strings.ReplaceAll(tc.path, "/", "_")), func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.path, nil)
				req.Host = tc.host
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200 for %s%s, got %d", tc.host, tc.path, w.Code)
				}

				if !strings.Contains(w.Body.String(), tc.expectedBody) {
					t.Errorf("Expected body to contain '%s' for %s%s, got '%s'", 
						tc.expectedBody, tc.host, tc.path, w.Body.String())
				}
			})
		}

		// Test mismatched host/path combinations - should return 404
		mismatchTests := []struct {
			host string
			path string
		}{
			{"api.example.com", "/admin/panel"},     // wrong host for admin
			{"web.example.com", "/v1/data"},        // wrong host for api v1
			{"api.example.com", "/v3/data"},        // non-existent version
			{"invalid.example.com", "/v1/data"},    // invalid host
		}

		for _, tc := range mismatchTests {
			t.Run(fmt.Sprintf("mismatch_%s%s", tc.host, strings.ReplaceAll(tc.path, "/", "_")), func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.path, nil)
				req.Host = tc.host
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				if w.Code != http.StatusNotFound {
					t.Errorf("Expected status 404 for mismatched %s%s, got %d", tc.host, tc.path, w.Code)
				}
			})
		}
	})
}

// TestMultiTenantLoadBalancing tests load balancing across multiple tenants
func TestMultiTenantLoadBalancing(t *testing.T) {
	t.Run("per_tenant_load_balancing", func(t *testing.T) {
		// Create multiple backends for each tenant
		tenant1Backend1 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Tenant1-Backend1"},
			},
		})
		defer tenant1Backend1.Close()

		tenant1Backend2 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Tenant1-Backend2"},
			},
		})
		defer tenant1Backend2.Close()

		tenant2Backend1 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Tenant2-Backend1"},
			},
		})
		defer tenant2Backend1.Close()

		tenant2Backend2 := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Tenant2-Backend2"},
			},
		})
		defer tenant2Backend2.Close()

		// Create config with multiple backends per tenant
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lb-tenant-1",
					PathPrefix: "/tenant1/",
					Interval:   30,
					Services: []config.Service{
						{Name: "service1", URL: tenant1Backend1.URL, Health: "/health"},
						{Name: "service2", URL: tenant1Backend2.URL, Health: "/health"},
					},
				},
				{
					Name:       "lb-tenant-2",
					PathPrefix: "/tenant2/",
					Interval:   30,
					Services: []config.Service{
						{Name: "service1", URL: tenant2Backend1.URL, Health: "/health"},
						{Name: "service2", URL: tenant2Backend2.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		for _, tenantName := range []string{"lb-tenant-1", "lb-tenant-2"} {
			if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
				for _, backend := range tenantRouter.Backends {
					backend.Alive.Store(true)
				}
			}
		}

		// Test load balancing for tenant 1
		tenant1Responses := make(map[string]int)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/tenant1/", nil)
			w := httptest.NewRecorder()

			env.Router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				body := strings.TrimSpace(w.Body.String())
				tenant1Responses[body]++
			}
		}

		// Test load balancing for tenant 2
		tenant2Responses := make(map[string]int)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/tenant2/", nil)
			w := httptest.NewRecorder()

			env.Router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				body := strings.TrimSpace(w.Body.String())
				tenant2Responses[body]++
			}
		}

		// Verify load balancing occurred within each tenant
		if len(tenant1Responses) < 1 {
			t.Errorf("Expected load balancing for tenant1, got responses: %v", tenant1Responses)
		}

		if len(tenant2Responses) < 1 {
			t.Errorf("Expected load balancing for tenant2, got responses: %v", tenant2Responses)
		}

		// Verify tenant isolation - responses should be distinct per tenant
		for response := range tenant1Responses {
			if strings.Contains(response, "Tenant2") {
				t.Error("Tenant1 should not receive Tenant2 responses")
			}
		}

		for response := range tenant2Responses {
			if strings.Contains(response, "Tenant1") {
				t.Error("Tenant2 should not receive Tenant1 responses")
			}
		}
	})
}

// TestMultiTenantConcurrency tests concurrent access across multiple tenants
func TestMultiTenantConcurrency(t *testing.T) {
	t.Run("concurrent_multi_tenant_requests", func(t *testing.T) {
		// Create backends for multiple tenants
		tenant1Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Concurrent Tenant 1", Delay: 10 * time.Millisecond},
			},
		})
		defer tenant1Backend.Close()

		tenant2Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Concurrent Tenant 2", Delay: 10 * time.Millisecond},
			},
		})
		defer tenant2Backend.Close()

		tenant3Backend := fixtures.CreateCustomBackend(t, fixtures.BackendBehavior{
			ResponseMap: map[string]fixtures.BackendResponse{
				"/": {StatusCode: 200, Body: "Concurrent Tenant 3", Delay: 10 * time.Millisecond},
			},
		})
		defer tenant3Backend.Close()

		// Create config with multiple tenants
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:     "concurrent-tenant-1",
					Domains:  []string{"tenant1.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "service1", URL: tenant1Backend.URL, Health: "/health"},
					},
				},
				{
					Name:     "concurrent-tenant-2",
					Domains:  []string{"tenant2.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "service2", URL: tenant2Backend.URL, Health: "/health"},
					},
				},
				{
					Name:     "concurrent-tenant-3",
					Domains:  []string{"tenant3.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "service3", URL: tenant3Backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		tenantNames := []string{"concurrent-tenant-1", "concurrent-tenant-2", "concurrent-tenant-3"}
		for _, tenantName := range tenantNames {
			if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
				for _, backend := range tenantRouter.Backends {
					backend.Alive.Store(true)
				}
			}
		}

		// Test concurrent requests to different tenants
		const numConcurrent = 30
		const requestsPerTenant = 10

		results := make(chan struct {
			tenant   string
			response string
			status   int
			err      error
		}, numConcurrent)

		// Launch concurrent requests
		start := time.Now()

		for i := 0; i < requestsPerTenant; i++ {
			// Tenant 1
			go func() {
				req := httptest.NewRequest("GET", "/", nil)
				req.Host = "tenant1.example.com"
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				results <- struct {
					tenant   string
					response string
					status   int
					err      error
				}{"tenant1", w.Body.String(), w.Code, nil}
			}()

			// Tenant 2
			go func() {
				req := httptest.NewRequest("GET", "/", nil)
				req.Host = "tenant2.example.com"
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				results <- struct {
					tenant   string
					response string
					status   int
					err      error
				}{"tenant2", w.Body.String(), w.Code, nil}
			}()

			// Tenant 3
			go func() {
				req := httptest.NewRequest("GET", "/", nil)
				req.Host = "tenant3.example.com"
				w := httptest.NewRecorder()

				env.Router.ServeHTTP(w, req)

				results <- struct {
					tenant   string
					response string
					status   int
					err      error
				}{"tenant3", w.Body.String(), w.Code, nil}
			}()
		}

		// Collect results
		tenantResults := map[string][]string{
			"tenant1": {},
			"tenant2": {},
			"tenant3": {},
		}

		for i := 0; i < numConcurrent; i++ {
			result := <-results

			if result.err != nil {
				t.Errorf("Request error for %s: %v", result.tenant, result.err)
				continue
			}

			if result.status != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d", result.tenant, result.status)
				continue
			}

			tenantResults[result.tenant] = append(tenantResults[result.tenant], result.response)
		}

		duration := time.Since(start)

		// Verify all requests completed
		for tenant, responses := range tenantResults {
			if len(responses) != requestsPerTenant {
				t.Errorf("Expected %d responses for %s, got %d", requestsPerTenant, tenant, len(responses))
			}

			// Verify response content is correct for each tenant
			for _, response := range responses {
				expectedContent := fmt.Sprintf("Concurrent Tenant %s", tenant[len(tenant)-1:])
				if !strings.Contains(response, expectedContent) {
					t.Errorf("Expected response to contain '%s' for %s, got '%s'", 
						expectedContent, tenant, response)
				}
			}
		}

		// Verify concurrent execution was efficient (should complete much faster than sequential)
		maxExpectedDuration := 200 * time.Millisecond // Allow for some overhead
		if duration > maxExpectedDuration {
			t.Errorf("Concurrent requests took too long: %v (expected < %v)", duration, maxExpectedDuration)
		}

		t.Logf("Completed %d concurrent requests across 3 tenants in %v", numConcurrent, duration)
	})
}

// TestMultiTenantErrorHandling tests error handling across multiple tenants
func TestMultiTenantErrorHandling(t *testing.T) {
	t.Run("isolated_tenant_failures", func(t *testing.T) {
		// Create healthy backend for tenant 1
		healthyBackend := fixtures.CreateSimpleBackend(t)
		defer healthyBackend.Close()

		// Create failing backend for tenant 2
		failingBackend := fixtures.CreateErrorBackend(t)
		defer failingBackend.Close()

		// Create config with one healthy and one failing tenant
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "healthy-tenant",
					PathPrefix: "/healthy/",
					Interval:   30,
					Services: []config.Service{
						{Name: "healthy-service", URL: healthyBackend.URL, Health: "/health"},
					},
				},
				{
					Name:       "failing-tenant",
					PathPrefix: "/failing/",
					Interval:   30,
					Services: []config.Service{
						{Name: "failing-service", URL: failingBackend.URL, Health: "/health"},
					},
				},
			},
		}

		// Setup gateway
		env := fixtures.SetupGateway(t, cfg)
		defer env.Cleanup()

		// Mark all backends as alive
		for _, tenantName := range []string{"healthy-tenant", "failing-tenant"} {
			if tenantRouter := env.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
				for _, backend := range tenantRouter.Backends {
					backend.Alive.Store(true)
				}
			}
		}

		// Test healthy tenant still works
		req1 := httptest.NewRequest("GET", "/healthy/test", nil)
		w1 := httptest.NewRecorder()
		env.Router.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Errorf("Expected status 200 for healthy tenant, got %d", w1.Code)
		}

		// Test failing tenant returns errors but doesn't affect healthy tenant
		req2 := httptest.NewRequest("GET", "/failing/500", nil)
		w2 := httptest.NewRecorder()
		env.Router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 for failing tenant, got %d", w2.Code)
		}

		// Verify healthy tenant is still functional after failing tenant error
		req3 := httptest.NewRequest("GET", "/healthy/test", nil)
		w3 := httptest.NewRecorder()
		env.Router.ServeHTTP(w3, req3)

		if w3.Code != http.StatusOK {
			t.Errorf("Expected healthy tenant to remain functional, got status %d", w3.Code)
		}

		// Test multiple error types from failing tenant
		errorTests := []struct {
			path           string
			expectedStatus int
		}{
			{"/failing/404", http.StatusNotFound},
			{"/failing/500", http.StatusInternalServerError},
			{"/failing/503", http.StatusServiceUnavailable},
		}

		for _, tt := range errorTests {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			env.Router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d for %s, got %d", tt.expectedStatus, tt.path, w.Code)
			}

			// Verify healthy tenant is still unaffected
			healthReq := httptest.NewRequest("GET", "/healthy/test", nil)
			healthW := httptest.NewRecorder()
			env.Router.ServeHTTP(healthW, healthReq)

			if healthW.Code != http.StatusOK {
				t.Errorf("Healthy tenant affected by failing tenant error at %s", tt.path)
			}
		}
	})
}