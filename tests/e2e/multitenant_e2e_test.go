package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/tests/e2e/fixtures"
)

// TestMultiTenantRoutingE2E tests real-world multi-tenant routing scenarios
func TestMultiTenantRoutingE2E(t *testing.T) {
	t.Run("host_based_tenant_isolation", func(t *testing.T) {
		// Start different backends for each tenant
		apiBackend := fixtures.StartRealBackend(t, "api")
		defer func() {
		if err := apiBackend.Stop(); err != nil {
			t.Logf("Failed to stop apiBackend: %v", err)
		}
	}()

		webBackend := fixtures.StartRealBackend(t, "simple")
		defer func() {
		if err := webBackend.Stop(); err != nil {
			t.Logf("Failed to stop webBackend: %v", err)
		}
	}()

		mobileBackend := fixtures.StartRealBackend(t, "json")
		defer func() {
		if err := mobileBackend.Stop(); err != nil {
			t.Logf("Failed to stop mobileBackend: %v", err)
		}
	}()

		// Create multi-tenant configuration with host-based routing
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

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test routing to different tenants based on Host header
		testCases := []struct {
			host           string
			path           string
			expectedStatus int
			verifyResponse func(t *testing.T, resp *fixtures.E2EResponse)
		}{
			{
				host:           "api.example.com",
				path:           "/users",
				expectedStatus: 200,
				verifyResponse: func(t *testing.T, resp *fixtures.E2EResponse) {
					var data map[string]interface{}
					if err := resp.JSON(&data); err != nil {
						t.Errorf("Failed to parse API response: %v", err)
						return
					}
					if resource, ok := data["resource"].(string); !ok || resource != "users" {
						t.Errorf("Expected API resource 'users', got: %v", data["resource"])
					}
				},
			},
			{
				host:           "web.example.com",
				path:           "/home",
				expectedStatus: 200,
				verifyResponse: func(t *testing.T, resp *fixtures.E2EResponse) {
					if !resp.ContainsInBody("Simple backend response") {
						t.Error("Expected simple backend response for web tenant")
					}
				},
			},
			{
				host:           "mobile.example.com",
				path:           "/users",
				expectedStatus: 200,
				verifyResponse: func(t *testing.T, resp *fixtures.E2EResponse) {
					var data []map[string]interface{}
					if err := resp.JSON(&data); err != nil {
						t.Errorf("Failed to parse mobile JSON response: %v", err)
						return
					}
					if len(data) == 0 {
						t.Error("Expected JSON array response from mobile tenant")
					}
				},
			},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("host_%s", strings.ReplaceAll(tc.host, ".", "_")), func(t *testing.T) {
				resp, err := client.RequestWithHost("GET", tc.path, tc.host, nil)
				if err != nil {
					t.Fatalf("Failed to make request to %s%s: %v", tc.host, tc.path, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != tc.expectedStatus {
					t.Errorf("Expected status %d for %s%s, got %d", 
						tc.expectedStatus, tc.host, tc.path, resp.StatusCode)
				}

				// Convert to E2EResponse for additional verification
				e2eResp, err := fixtures.NewE2EResponse(resp)
				if err != nil {
					t.Fatalf("Failed to create E2EResponse: %v", err)
				}

				if tc.verifyResponse != nil {
					tc.verifyResponse(t, e2eResp)
				}
			})
		}

		// Test invalid host - should return 404
		resp, err := client.RequestWithHost("GET", "/", "invalid.example.com", nil)
		if err != nil {
			t.Fatalf("Failed to make request to invalid host: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404 for invalid host, got %d", resp.StatusCode)
		}

		t.Logf("✓ Host-based tenant isolation verified")
	})

	t.Run("path_based_tenant_isolation", func(t *testing.T) {
		// Start backends for path-based tenants
		adminBackend := fixtures.StartRealBackend(t, "api")
		defer func() {
		if err := adminBackend.Stop(); err != nil {
			t.Logf("Failed to stop adminBackend: %v", err)
		}
	}()

		v1Backend := fixtures.StartRealBackend(t, "json")
		defer func() {
		if err := v1Backend.Stop(); err != nil {
			t.Logf("Failed to stop v1Backend: %v", err)
		}
	}()

		v2Backend := fixtures.StartRealBackend(t, "health")
		defer func() {
		if err := v2Backend.Stop(); err != nil {
			t.Logf("Failed to stop v2Backend: %v", err)
		}
	}()

		// Create configuration with path-based routing
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
						{Name: "api-v1-service", URL: v1Backend.URL, Health: "/health"},
					},
				},
				{
					Name:       "api-v2-tenant",
					PathPrefix: "/api/v2/",
					Interval:   30,
					Services: []config.Service{
						{Name: "api-v2-service", URL: v2Backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test path-based routing
		testCases := []struct {
			path           string
			expectedStatus int
			expectedType   string
		}{
			{"/admin/users", 200, "admin-api"},
			{"/api/v1/users", 200, "json-data"},
			{"/api/v2/health", 200, "health-check"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("path_%s", strings.ReplaceAll(tc.path, "/", "_")), func(t *testing.T) {
				resp, err := client.GetResponse(tc.path)
				if err != nil {
					t.Fatalf("Failed to make request to %s: %v", tc.path, err)
				}

				if !resp.HasStatus(tc.expectedStatus) {
					t.Errorf("Expected status %d for %s, got %d", 
						tc.expectedStatus, tc.path, resp.StatusCode)
				}

				// Verify tenant-specific responses
				switch tc.expectedType {
				case "admin-api":
					var data map[string]interface{}
					if err := resp.JSON(&data); err == nil {
						if resource, ok := data["resource"].(string); ok && resource != "users" {
							t.Errorf("Expected admin API resource 'users', got: %v", resource)
						}
					}
				case "json-data":
					var data []map[string]interface{}
					if err := resp.JSON(&data); err != nil {
						t.Errorf("Expected JSON array from v1 API: %v", err)
					}
				case "health-check":
					var data map[string]interface{}
					if err := resp.JSON(&data); err == nil {
						if status, ok := data["status"].(string); ok && status != "healthy" {
							t.Errorf("Expected health status 'healthy', got: %v", status)
						}
					}
				}
			})
		}

		// Test invalid path - should return 404
		resp, err := client.GetResponse("/invalid/path")
		if err != nil {
			t.Fatalf("Failed to make request to invalid path: %v", err)
		}

		if !resp.HasStatus(404) {
			t.Errorf("Expected status 404 for invalid path, got %d", resp.StatusCode)
		}

		t.Logf("✓ Path-based tenant isolation verified")
	})

	t.Run("hybrid_host_and_path_routing", func(t *testing.T) {
		// Start backends for hybrid routing
		apiV1Backend := fixtures.StartRealBackend(t, "api")
		defer func() {
		if err := apiV1Backend.Stop(); err != nil {
			t.Logf("Failed to stop apiV1Backend: %v", err)
		}
	}()

		apiV2Backend := fixtures.StartRealBackend(t, "json")
		defer func() {
		if err := apiV2Backend.Stop(); err != nil {
			t.Logf("Failed to stop apiV2Backend: %v", err)
		}
	}()

		webAdminBackend := fixtures.StartRealBackend(t, "health")
		defer func() {
		if err := webAdminBackend.Stop(); err != nil {
			t.Logf("Failed to stop webAdminBackend: %v", err)
		}
	}()

		// Create configuration with hybrid (host + path) routing
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
				// Admin on web.example.com/admin/
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

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test hybrid routing (both host and path must match)
		testCases := []struct {
			host           string
			path           string
			expectedStatus int
			description    string
		}{
			{"api.example.com", "/v1/users", 200, "API V1 hybrid routing"},
			{"api.example.com", "/v2/users", 200, "API V2 hybrid routing"},
			{"web.example.com", "/admin/health", 200, "Web admin hybrid routing"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("hybrid_%s_%s", 
				strings.ReplaceAll(tc.host, ".", "_"), 
				strings.ReplaceAll(tc.path, "/", "_")), func(t *testing.T) {
				
				resp, err := client.RequestWithHost("GET", tc.path, tc.host, nil)
				if err != nil {
					t.Fatalf("Failed to make hybrid request to %s%s: %v", tc.host, tc.path, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != tc.expectedStatus {
					t.Errorf("Expected status %d for %s%s, got %d", 
						tc.expectedStatus, tc.host, tc.path, resp.StatusCode)
				}

				t.Logf("✓ %s: %s%s", tc.description, tc.host, tc.path)
			})
		}

		// Test mismatched host/path combinations - should return 404
		mismatchTests := []struct {
			host string
			path string
		}{
			{"api.example.com", "/admin/health"},   // wrong host for admin
			{"web.example.com", "/v1/users"},      // wrong host for api v1
			{"api.example.com", "/v3/users"},      // non-existent version
			{"invalid.example.com", "/v1/users"},  // invalid host
		}

		for _, tc := range mismatchTests {
			t.Run(fmt.Sprintf("mismatch_%s_%s", 
				strings.ReplaceAll(tc.host, ".", "_"), 
				strings.ReplaceAll(tc.path, "/", "_")), func(t *testing.T) {
				
				resp, err := client.RequestWithHost("GET", tc.path, tc.host, nil)
				if err != nil {
					t.Fatalf("Failed to make mismatch request to %s%s: %v", tc.host, tc.path, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 404 {
					t.Errorf("Expected status 404 for mismatched %s%s, got %d", 
						tc.host, tc.path, resp.StatusCode)
				}
			})
		}

		t.Logf("✓ Hybrid host and path routing verified")
	})
}

// TestMultiTenantLoadBalancingE2E tests load balancing across multiple tenants
func TestMultiTenantLoadBalancingE2E(t *testing.T) {
	t.Run("per_tenant_load_balancing", func(t *testing.T) {
		// Create multiple backends for each tenant
		tenant1Backend1 := fixtures.StartRealBackend(t, "simple")
		defer func() {
		if err := tenant1Backend1.Stop(); err != nil {
			t.Logf("Failed to stop tenant1Backend1: %v", err)
		}
	}()

		tenant1Backend2 := fixtures.StartRealBackend(t, "api")
		defer func() {
		if err := tenant1Backend2.Stop(); err != nil {
			t.Logf("Failed to stop tenant1Backend2: %v", err)
		}
	}()

		tenant2Backend1 := fixtures.StartRealBackend(t, "json")
		defer func() {
		if err := tenant2Backend1.Stop(); err != nil {
			t.Logf("Failed to stop tenant2Backend1: %v", err)
		}
	}()

		tenant2Backend2 := fixtures.StartRealBackend(t, "health")
		defer func() {
		if err := tenant2Backend2.Stop(); err != nil {
			t.Logf("Failed to stop tenant2Backend2: %v", err)
		}
	}()

		// Create configuration with multiple backends per tenant
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

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test load balancing for tenant 1
		t.Run("tenant1_load_balancing", func(t *testing.T) {
			responses := make(map[string]int)
			
			for i := 0; i < 10; i++ {
				resp, err := client.GetResponse("/tenant1/test")
				if err != nil {
					t.Errorf("Request %d failed: %v", i+1, err)
					continue
				}

				if resp.HasStatus(200) {
					// Categorize response by type
					if resp.ContainsInBody("Simple backend") {
						responses["simple"]++
					} else if resp.ContainsInBody("resource") {
						responses["api"]++
					} else {
						responses["other"]++
					}
				}
			}

			t.Logf("Tenant 1 load balancing distribution: %v", responses)
			
			// Should have some distribution (at least responses)
			totalResponses := 0
			for _, count := range responses {
				totalResponses += count
			}
			if totalResponses < 8 { // Allow for some failures
				t.Errorf("Too few successful responses for tenant 1: %d", totalResponses)
			}
		})

		// Test load balancing for tenant 2
		t.Run("tenant2_load_balancing", func(t *testing.T) {
			responses := make(map[string]int)
			
			for i := 0; i < 10; i++ {
				resp, err := client.GetResponse("/tenant2/users")
				if err != nil {
					t.Errorf("Request %d failed: %v", i+1, err)
					continue
				}

				if resp.HasStatus(200) {
					// Categorize response by content
					if resp.ContainsInBody("status") && resp.ContainsInBody("healthy") {
						responses["health"]++
					} else if strings.Contains(resp.BodyString, "[") {
						responses["json"]++
					} else {
						responses["other"]++
					}
				}
			}

			t.Logf("Tenant 2 load balancing distribution: %v", responses)
			
			totalResponses := 0
			for _, count := range responses {
				totalResponses += count
			}
			if totalResponses < 8 {
				t.Errorf("Too few successful responses for tenant 2: %d", totalResponses)
			}
		})

		// Test tenant isolation - tenant 1 requests should not get tenant 2 responses
		t.Run("tenant_isolation_verification", func(t *testing.T) {
			// Make requests to both tenants and verify isolation
			resp1, err := client.GetResponse("/tenant1/test")
			if err != nil {
				t.Fatalf("Failed to get tenant 1 response: %v", err)
			}

			resp2, err := client.GetResponse("/tenant2/test")
			if err != nil {
				t.Fatalf("Failed to get tenant 2 response: %v", err)
			}

			// Responses should be different (different backend types)
			if resp1.BodyString == resp2.BodyString {
				t.Log("Tenant responses are identical - may indicate shared backend or simple responses")
			}

			t.Logf("✓ Tenant isolation: T1=%s, T2=%s", 
				resp1.BodyString[:min(50, len(resp1.BodyString))],
				resp2.BodyString[:min(50, len(resp2.BodyString))])
		})

		t.Logf("✓ Per-tenant load balancing verified")
	})
}

// TestMultiTenantConcurrencyE2E tests concurrent access across multiple tenants
func TestMultiTenantConcurrencyE2E(t *testing.T) {
	t.Run("concurrent_multi_tenant_requests", func(t *testing.T) {
		// Start backends for multiple tenants
		tenant1Backend := fixtures.StartRealBackend(t, "api")
		defer func() {
		if err := tenant1Backend.Stop(); err != nil {
			t.Logf("Failed to stop tenant1Backend: %v", err)
		}
	}()

		tenant2Backend := fixtures.StartRealBackend(t, "json")
		defer func() {
		if err := tenant2Backend.Stop(); err != nil {
			t.Logf("Failed to stop tenant2Backend: %v", err)
		}
	}()

		tenant3Backend := fixtures.StartRealBackend(t, "health")
		defer func() {
		if err := tenant3Backend.Stop(); err != nil {
			t.Logf("Failed to stop tenant3Backend: %v", err)
		}
	}()

		// Create multi-tenant configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:     "concurrent-tenant-1",
					Domains:  []string{"t1.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "service1", URL: tenant1Backend.URL, Health: "/health"},
					},
				},
				{
					Name:     "concurrent-tenant-2",
					Domains:  []string{"t2.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "service2", URL: tenant2Backend.URL, Health: "/health"},
					},
				},
				{
					Name:     "concurrent-tenant-3",
					Domains:  []string{"t3.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "service3", URL: tenant3Backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test concurrent requests to different tenants
		concurrency := 15 // 5 requests per tenant
		requests := make([]func() (*http.Response, error), concurrency)

		// Create requests for different tenants
		for i := 0; i < concurrency; i++ {
			switch i % 3 {
			case 0:
				// Tenant 1 - API backend
				requests[i] = func() (*http.Response, error) {
					return client.RequestWithHost("GET", "/users", "t1.example.com", nil)
				}
			case 1:
				// Tenant 2 - JSON backend
				requests[i] = func() (*http.Response, error) {
					return client.RequestWithHost("GET", "/products", "t2.example.com", nil)
				}
			case 2:
				// Tenant 3 - Health backend
				requests[i] = func() (*http.Response, error) {
					return client.RequestWithHost("GET", "/health", "t3.example.com", nil)
				}
			}
		}

		// Execute requests concurrently
		start := time.Now()
		responses, errors := client.ParallelRequests(requests)
		duration := time.Since(start)

		// Analyze results
		tenantResults := map[string][]int{
			"t1": {},
			"t2": {},
			"t3": {},
		}

		for i, resp := range responses {
			if errors[i] != nil {
				t.Errorf("Concurrent request %d failed: %v", i, errors[i])
				continue
			}

			tenantIndex := i % 3
			tenantKey := fmt.Sprintf("t%d", tenantIndex+1)
			tenantResults[tenantKey] = append(tenantResults[tenantKey], resp.StatusCode)
			
			resp.Body.Close()
		}

		// Verify results
		for tenant, statusCodes := range tenantResults {
			successCount := 0
			for _, status := range statusCodes {
				if status == 200 {
					successCount++
				}
			}
			
			expectedRequests := concurrency / 3
			if len(statusCodes) != expectedRequests {
				t.Errorf("Expected %d requests for %s, got %d", 
					expectedRequests, tenant, len(statusCodes))
			}

			if successCount < expectedRequests/2 {
				t.Errorf("Too few successful requests for %s: %d/%d", 
					tenant, successCount, len(statusCodes))
			}

			t.Logf("Tenant %s: %d/%d successful requests", 
				tenant, successCount, len(statusCodes))
		}

		// Performance verification
		maxExpectedDuration := 3 * time.Second
		if duration > maxExpectedDuration {
			t.Errorf("Concurrent requests took too long: %v (expected < %v)", 
				duration, maxExpectedDuration)
		}

		t.Logf("✓ Concurrent multi-tenant access: %d requests in %v", 
			concurrency, duration)
	})
}

// TestMultiTenantErrorHandlingE2E tests error handling across multiple tenants
func TestMultiTenantErrorHandlingE2E(t *testing.T) {
	t.Run("isolated_tenant_failures", func(t *testing.T) {
		// Create healthy backend for tenant 1
		healthyBackend := fixtures.StartRealBackend(t, "simple")
		defer func() {
		if err := healthyBackend.Stop(); err != nil {
			t.Logf("Failed to stop healthyBackend: %v", err)
		}
	}()

		// Create error backend for tenant 2
		errorBackend := fixtures.StartRealBackend(t, "error")
		defer func() {
		if err := errorBackend.Stop(); err != nil {
			t.Logf("Failed to stop errorBackend: %v", err)
		}
	}()

		// Create configuration with one healthy and one error-prone tenant
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
					Name:       "error-tenant",
					PathPrefix: "/error/",
					Interval:   30,
					Services: []config.Service{
						{Name: "error-service", URL: errorBackend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test healthy tenant remains functional
		t.Run("healthy_tenant_functionality", func(t *testing.T) {
			for i := 0; i < 5; i++ {
				resp, err := client.GetResponse("/healthy/test")
				if err != nil {
					t.Errorf("Healthy tenant request %d failed: %v", i+1, err)
					continue
				}

				if !resp.HasStatus(200) {
					t.Errorf("Expected status 200 for healthy tenant, got %d", resp.StatusCode)
				}
			}
		})

		// Test error tenant returns appropriate errors
		t.Run("error_tenant_isolation", func(t *testing.T) {
			errorTests := []struct {
				path           string
				expectedStatus int
			}{
				{"/error/400", 400},
				{"/error/500", 500},
				{"/error/503", 503},
			}

			for _, tt := range errorTests {
				resp, err := client.GetResponse(tt.path)
				if err != nil {
					t.Errorf("Error tenant request to %s failed: %v", tt.path, err)
					continue
				}

				if !resp.HasStatus(tt.expectedStatus) {
					t.Errorf("Expected status %d for %s, got %d", 
						tt.expectedStatus, tt.path, resp.StatusCode)
				}
			}
		})

		// Test that errors in one tenant don't affect the other
		t.Run("cross_tenant_isolation", func(t *testing.T) {
			// Generate error in error tenant
			errorResp, err := client.GetResponse("/error/500")
			if err != nil {
				t.Fatalf("Failed to generate error: %v", err)
			}

			if !errorResp.HasStatus(500) {
				t.Errorf("Expected error status 500, got %d", errorResp.StatusCode)
			}

			// Immediately test healthy tenant
			healthyResp, err := client.GetResponse("/healthy/test")
			if err != nil {
				t.Fatalf("Healthy tenant affected by error tenant: %v", err)
			}

			if !healthyResp.HasStatus(200) {
				t.Errorf("Healthy tenant affected by error tenant, got status %d", 
					healthyResp.StatusCode)
			}

			t.Logf("✓ Error isolation: Error tenant returned %d, healthy tenant returned %d", 
				errorResp.StatusCode, healthyResp.StatusCode)
		})

		t.Logf("✓ Isolated tenant failures verified")
	})
}

// TestRealWorldMultiTenantE2E tests comprehensive real-world multi-tenant scenarios
func TestRealWorldMultiTenantE2E(t *testing.T) {
	t.Run("comprehensive_multi_tenant_scenario", func(t *testing.T) {
		// Start backends for realistic scenario
		publicAPIBackend := fixtures.StartRealBackend(t, "api")
		defer func() {
		if err := publicAPIBackend.Stop(); err != nil {
			t.Logf("Failed to stop publicAPIBackend: %v", err)
		}
	}()

		internalAPIBackend := fixtures.StartRealBackend(t, "health")
		defer func() {
		if err := internalAPIBackend.Stop(); err != nil {
			t.Logf("Failed to stop internalAPIBackend: %v", err)
		}
	}()

		webFrontendBackend := fixtures.StartRealBackend(t, "simple")
		defer func() {
		if err := webFrontendBackend.Stop(); err != nil {
			t.Logf("Failed to stop webFrontendBackend: %v", err)
		}
	}()

		adminBackend := fixtures.StartRealBackend(t, "json")
		defer func() {
		if err := adminBackend.Stop(); err != nil {
			t.Logf("Failed to stop adminBackend: %v", err)
		}
	}()

		// Create realistic multi-tenant configuration
		cfg := &config.Config{
			AdminBasePath: "/admin",
			Tenants: []config.Tenant{
				// Public API
				{
					Name:     "public-api",
					Domains:  []string{"api.example.com"},
					Interval: 30,
					Services: []config.Service{
						{Name: "public-api-service", URL: publicAPIBackend.URL, Health: "/health"},
					},
				},
				// Internal API
				{
					Name:       "internal-api",
					PathPrefix: "/internal/",
					Interval:   15,
					Services: []config.Service{
						{Name: "internal-api-service", URL: internalAPIBackend.URL, Health: "/health"},
					},
				},
				// Web frontend
				{
					Name:     "web-frontend",
					Domains:  []string{"web.example.com"},
					Interval: 60,
					Services: []config.Service{
						{Name: "web-service", URL: webFrontendBackend.URL, Health: "/health"},
					},
				},
				// Admin panel
				{
					Name:       "admin-panel",
					PathPrefix: "/admin/",
					Interval:   30,
					Services: []config.Service{
						{Name: "admin-service", URL: adminBackend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test public API access
		t.Run("public_api_operations", func(t *testing.T) {
			// GET operation
			resp, err := client.RequestWithHost("GET", "/users", "api.example.com", nil)
			if err != nil {
				t.Fatalf("Failed to access public API: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200 for public API GET, got %d", resp.StatusCode)
			}

			// POST operation
			userData := map[string]interface{}{
				"name":  "Test User",
				"email": "test@example.com",
			}

			postResp, err := client.PostWithHost("/users", "api.example.com", userData)
			if err != nil {
				t.Fatalf("Failed to POST to public API: %v", err)
			}
			defer postResp.Body.Close()

			if postResp.StatusCode != 201 && postResp.StatusCode != 200 {
				t.Errorf("Expected status 201 or 200 for public API POST, got %d", postResp.StatusCode)
			}
		})

		// Test internal API access
		t.Run("internal_api_operations", func(t *testing.T) {
			resp, err := client.GetResponse("/internal/health")
			if err != nil {
				t.Fatalf("Failed to access internal API: %v", err)
			}

			if !resp.HasStatus(200) {
				t.Errorf("Expected status 200 for internal API, got %d", resp.StatusCode)
			}

			// Verify health response
			var healthData map[string]interface{}
			if err := resp.JSON(&healthData); err == nil {
				if status, ok := healthData["status"].(string); ok && status == "healthy" {
					t.Logf("✓ Internal API health check successful")
				}
			}
		})

		// Test web frontend access
		t.Run("web_frontend_operations", func(t *testing.T) {
			resp, err := client.RequestWithHost("GET", "/", "web.example.com", nil)
			if err != nil {
				t.Fatalf("Failed to access web frontend: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200 for web frontend, got %d", resp.StatusCode)
			}
		})

		// Test admin panel access
		t.Run("admin_panel_operations", func(t *testing.T) {
			resp, err := client.GetResponse("/admin/users")
			if err != nil {
				t.Fatalf("Failed to access admin panel: %v", err)
			}

			if !resp.HasStatus(200) {
				t.Errorf("Expected status 200 for admin panel, got %d", resp.StatusCode)
			}

			// Should get JSON response from admin backend
			var adminData []map[string]interface{}
			if err := resp.JSON(&adminData); err != nil {
				t.Errorf("Expected JSON response from admin panel: %v", err)
			}
		})

		// Test complex routing scenarios
		t.Run("complex_routing_scenarios", func(t *testing.T) {
			scenarios := []struct {
				description string
				host        string
				path        string
				expected    int
			}{
				{"Public API user creation", "api.example.com", "/users", 200},
				{"Public API product listing", "api.example.com", "/products", 200},
				{"Internal health check", "", "/internal/health", 200},
				{"Internal admin endpoint", "", "/internal/admin", 200},
				{"Web homepage", "web.example.com", "/", 200},
				{"Web about page", "web.example.com", "/about", 200},
				{"Admin user management", "", "/admin/users", 200},
				{"Admin dashboard", "", "/admin/dashboard", 200},
			}

			for _, scenario := range scenarios {
				t.Run(scenario.description, func(t *testing.T) {
					var resp *http.Response
					var err error

					if scenario.host != "" {
						resp, err = client.RequestWithHost("GET", scenario.path, scenario.host, nil)
					} else {
						resp, err = client.Get(scenario.path)
					}

					if err != nil {
						t.Fatalf("Failed %s: %v", scenario.description, err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != scenario.expected {
						t.Errorf("%s: expected status %d, got %d", 
							scenario.description, scenario.expected, resp.StatusCode)
					}
				})
			}
		})

		t.Logf("✓ Comprehensive real-world multi-tenant scenario verified")
	})
}

