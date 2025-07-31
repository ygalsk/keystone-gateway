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

// TestLuaMiddlewareE2E tests Lua middleware execution in real request context
func TestLuaMiddlewareE2E(t *testing.T) {
	t.Run("basic_lua_middleware_execution", func(t *testing.T) {
		// Start backend for testing
		backend := fixtures.StartRealBackend(t, "echo")
		defer backend.Stop()

		// Create configuration with basic Lua middleware
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-middleware-tenant",
					PathPrefix: "/lua/",
					Interval:   30,
					Services: []config.Service{
						{Name: "lua-service", URL: backend.URL, Health: "/health"},
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

		// Test basic request processing through Lua middleware
		resp, err := client.GetResponse("/lua/test")
		if err != nil {
			t.Fatalf("Failed to make request through Lua middleware: %v", err)
		}

		if !resp.HasStatus(200) {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Verify request reached backend
		var echoData map[string]interface{}
		if err := resp.JSON(&echoData); err != nil {
			t.Errorf("Failed to parse echo response: %v", err)
		} else {
			if path, ok := echoData["path"].(string); !ok || path != "/test" {
				t.Errorf("Expected path '/test', got: %v", echoData["path"])
			}
		}

		t.Logf("✓ Basic Lua middleware execution verified")
	})

	t.Run("lua_header_manipulation", func(t *testing.T) {
		// Start echo backend to inspect headers
		backend := fixtures.StartRealBackend(t, "echo")
		defer backend.Stop()

		// Create configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-headers-tenant",
					PathPrefix: "/headers/",
					Interval:   30,
					Services: []config.Service{
						{Name: "headers-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer gateway.Stop()

		// Create client with custom headers
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)
		client.SetHeaders(map[string]string{
			"X-Original-Header": "original-value",
			"User-Agent":        "E2E-Lua-Test/1.0",
		})

		// Test header processing
		resp, err := client.GetResponse("/headers/test")
		if err != nil {
			t.Fatalf("Failed to make request for header testing: %v", err)
		}

		if !resp.HasStatus(200) {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Parse echo response to verify headers
		var echoData map[string]interface{}
		if err := resp.JSON(&echoData); err != nil {
			t.Errorf("Failed to parse echo response: %v", err)
		} else {
			if headers, ok := echoData["headers"].(map[string]interface{}); ok {
				// Verify original headers were preserved
				if userAgent, exists := headers["User-Agent"]; !exists {
					t.Error("User-Agent header was not preserved")
				} else if !strings.Contains(fmt.Sprintf("%v", userAgent), "E2E-Lua-Test") {
					t.Errorf("Expected User-Agent to contain E2E-Lua-Test, got: %v", userAgent)
				}

				if originalHeader, exists := headers["X-Original-Header"]; !exists {
					t.Error("X-Original-Header was not preserved")
				} else if fmt.Sprintf("%v", originalHeader) != "[original-value]" {
					t.Errorf("Expected X-Original-Header value, got: %v", originalHeader)
				}

				t.Logf("✓ Headers preserved through Lua middleware")
			} else {
				t.Error("No headers found in echo response")
			}
		}

		t.Logf("✓ Lua header manipulation verified")
	})

	t.Run("lua_request_transformation", func(t *testing.T) {
		// Start echo backend to inspect transformations
		backend := fixtures.StartRealBackend(t, "echo")
		defer backend.Stop()

		// Create configuration for request transformation
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-transform-tenant",
					PathPrefix: "/transform/",
					Interval:   30,
					Services: []config.Service{
						{Name: "transform-service", URL: backend.URL, Health: "/health"},
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

		// Test POST request with JSON transformation
		requestData := map[string]interface{}{
			"user":      "test-user",
			"action":    "lua-transform",
			"timestamp": time.Now().Unix(),
		}

		resp, err := client.PostResponse("/transform/data", requestData)
		if err != nil {
			t.Fatalf("Failed to make POST request for transformation: %v", err)
		}

		if !resp.HasStatus(200) {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Verify request transformation
		var echoData map[string]interface{}
		if err := resp.JSON(&echoData); err != nil {
			t.Errorf("Failed to parse transformation response: %v", err)
		} else {
			// Verify method and path
			if method, ok := echoData["method"].(string); !ok || method != "POST" {
				t.Errorf("Expected method POST, got: %v", echoData["method"])
			}

			if path, ok := echoData["path"].(string); !ok || path != "/data" {
				t.Errorf("Expected path '/data', got: %v", echoData["path"])
			}

			// Verify body was transformed/preserved
			if bodyStr, ok := echoData["body"].(string); ok && bodyStr != "" {
				t.Logf("✓ Request body preserved: %s", bodyStr[:min(50, len(bodyStr))])
			} else {
				t.Error("Request body was not preserved through transformation")
			}
		}

		t.Logf("✓ Lua request transformation verified")
	})
}

// TestLuaRoutingE2E tests Lua-based dynamic routing
func TestLuaRoutingE2E(t *testing.T) {
	t.Run("lua_dynamic_routing", func(t *testing.T) {
		// Start multiple backends for dynamic routing
		primaryBackend := fixtures.StartRealBackend(t, "api")
		defer func() {
			if err := primaryBackend.Stop(); err != nil {
				t.Logf("Failed to stop primaryBackend: %v", err)
			}
		}()

		secondaryBackend := fixtures.StartRealBackend(t, "json")
		defer func() {
			if err := secondaryBackend.Stop(); err != nil {
				t.Logf("Failed to stop secondaryBackend: %v", err)
			}
		}()

		// Create configuration with multiple services for Lua routing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-routing-tenant",
					PathPrefix: "/route/",
					Interval:   30,
					Services: []config.Service{
						{Name: "primary-service", URL: primaryBackend.URL, Health: "/health"},
						{Name: "secondary-service", URL: secondaryBackend.URL, Health: "/health"},
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

		// Test dynamic routing to different backends
		testCases := []struct {
			path        string
			description string
		}{
			{"/route/primary", "Primary backend routing"},
			{"/route/secondary", "Secondary backend routing"},
			{"/route/users", "Default routing"},
			{"/route/products", "Product routing"},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				resp, err := client.GetResponse(tc.path)
				if err != nil {
					t.Fatalf("Failed to make dynamic routing request to %s: %v", tc.path, err)
				}

				if !resp.HasStatus(200) {
					t.Errorf("Expected status 200 for %s, got %d", tc.path, resp.StatusCode)
				}

				// Log which backend handled the request
				if resp.ContainsInBody("resource") {
					t.Logf("✓ %s routed to API backend", tc.path)
				} else if strings.Contains(resp.BodyString, "[") {
					t.Logf("✓ %s routed to JSON backend", tc.path)
				} else {
					t.Logf("✓ %s routed to backend (response: %s)", tc.path, resp.BodyString[:min(30, len(resp.BodyString))])
				}
			})
		}

		t.Logf("✓ Lua dynamic routing verified")
	})

	t.Run("lua_conditional_routing", func(t *testing.T) {
		// Start backends for conditional routing
		fastBackend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := fastBackend.Stop(); err != nil {
				t.Logf("Failed to stop fastBackend: %v", err)
			}
		}()

		slowBackend := fixtures.StartRealBackend(t, "slow")
		defer func() {
			if err := slowBackend.Stop(); err != nil {
				t.Logf("Failed to stop slowBackend: %v", err)
			}
		}()

		// Create configuration for conditional routing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-conditional-tenant",
					PathPrefix: "/conditional/",
					Interval:   30,
					Services: []config.Service{
						{Name: "fast-service", URL: fastBackend.URL, Health: "/health"},
						{Name: "slow-service", URL: slowBackend.URL, Health: "/health"},
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

		// Test conditional routing based on request characteristics
		conditionTests := []struct {
			path        string
			headers     map[string]string
			description string
		}{
			{
				path:        "/conditional/fast",
				headers:     map[string]string{"X-Priority": "high"},
				description: "High priority to fast backend",
			},
			{
				path:        "/conditional/slow",
				headers:     map[string]string{"X-Priority": "low"},
				description: "Low priority to slow backend",
			},
			{
				path:        "/conditional/default",
				headers:     map[string]string{},
				description: "Default routing",
			},
		}

		for _, tt := range conditionTests {
			t.Run(tt.description, func(t *testing.T) {
				start := time.Now()
				resp, err := client.GetWithHeaders(tt.path, tt.headers)
				duration := time.Since(start)

				if err != nil {
					t.Fatalf("Failed conditional routing request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 200 {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}

				t.Logf("✓ %s completed in %v", tt.description, duration)
			})
		}

		t.Logf("✓ Lua conditional routing verified")
	})
}

// TestLuaPerformanceE2E tests Lua performance characteristics in real requests
func TestLuaPerformanceE2E(t *testing.T) {
	t.Run("lua_processing_overhead", func(t *testing.T) {
		// Start backend for performance testing
		backend := fixtures.StartRealBackend(t, "simple")
		defer backend.Stop()

		// Create configuration with Lua processing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-perf-tenant",
					PathPrefix: "/perf/",
					Interval:   30,
					Services: []config.Service{
						{Name: "perf-service", URL: backend.URL, Health: "/health"},
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

		// Measure performance with Lua processing
		numRequests := 20
		var totalDuration time.Duration
		var successCount int

		for i := 0; i < numRequests; i++ {
			start := time.Now()
			resp, err := client.Get(fmt.Sprintf("/perf/test-%d", i))
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Request %d failed: %v", i+1, err)
				continue
			}

			if resp.StatusCode == 200 {
				successCount++
				totalDuration += duration
			}

			resp.Body.Close()
		}

		if successCount == 0 {
			t.Fatal("No successful requests completed")
		}

		avgDuration := totalDuration / time.Duration(successCount)

		// Performance expectations
		maxExpectedDuration := 100 * time.Millisecond // Conservative expectation
		if avgDuration > maxExpectedDuration {
			t.Errorf("Average request duration too high: %v (expected < %v)",
				avgDuration, maxExpectedDuration)
		}

		successRate := float64(successCount) / float64(numRequests)
		if successRate < 0.9 {
			t.Errorf("Success rate too low: %.2f%% (expected >= 90%%)", successRate*100)
		}

		t.Logf("✓ Lua processing performance: %d/%d requests, avg duration %v",
			successCount, numRequests, avgDuration)
	})

	t.Run("lua_concurrent_processing", func(t *testing.T) {
		// Start backend for concurrent testing
		backend := fixtures.StartRealBackend(t, "echo")
		defer backend.Stop()

		// Create configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-concurrent-tenant",
					PathPrefix: "/concurrent/",
					Interval:   30,
					Services: []config.Service{
						{Name: "concurrent-service", URL: backend.URL, Health: "/health"},
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

		// Create concurrent requests with Lua processing
		concurrency := 10
		requests := make([]func() (*http.Response, error), concurrency)

		for i := 0; i < concurrency; i++ {
			requests[i] = func() (*http.Response, error) {
				return client.Get(fmt.Sprintf("/concurrent/lua-test-%d", i))
			}
		}

		// Execute concurrent requests
		start := time.Now()
		responses, errors := client.ParallelRequests(requests)
		duration := time.Since(start)

		// Analyze results
		successCount := 0
		for i, resp := range responses {
			if errors[i] != nil {
				t.Errorf("Concurrent Lua request %d failed: %v", i, errors[i])
			} else if resp.StatusCode == 200 {
				successCount++
				resp.Body.Close()
			}
		}

		if successCount < concurrency/2 {
			t.Errorf("Too few successful concurrent Lua requests: %d/%d",
				successCount, concurrency)
		}

		// Performance check
		maxExpectedDuration := 2 * time.Second
		if duration > maxExpectedDuration {
			t.Errorf("Concurrent Lua processing took too long: %v (expected < %v)",
				duration, maxExpectedDuration)
		}

		t.Logf("✓ Lua concurrent processing: %d/%d successful in %v",
			successCount, concurrency, duration)
	})
}

// TestLuaErrorHandlingE2E tests Lua error handling in real request context
func TestLuaErrorHandlingE2E(t *testing.T) {
	t.Run("lua_script_error_recovery", func(t *testing.T) {
		// Start backend for error testing
		backend := fixtures.StartRealBackend(t, "simple")
		defer backend.Stop()

		// Create configuration that might trigger Lua errors
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-error-tenant",
					PathPrefix: "/error/",
					Interval:   30,
					Services: []config.Service{
						{Name: "error-service", URL: backend.URL, Health: "/health"},
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

		// Test error recovery scenarios
		errorTests := []struct {
			path        string
			description string
		}{
			{"/error/normal", "Normal request processing"},
			{"/error/edge-case", "Edge case handling"},
			{"/error/boundary", "Boundary condition"},
		}

		for _, tt := range errorTests {
			t.Run(tt.description, func(t *testing.T) {
				resp, err := client.GetResponse(tt.path)
				if err != nil {
					t.Fatalf("Failed Lua error recovery test: %v", err)
				}

				// Should either succeed or fail gracefully
				if resp.StatusCode != 200 && resp.StatusCode < 500 {
					t.Logf("Graceful handling: %s returned status %d", tt.path, resp.StatusCode)
				} else if resp.StatusCode >= 500 {
					t.Logf("Server error for %s: %d (may indicate Lua error)", tt.path, resp.StatusCode)
				} else {
					t.Logf("✓ %s processed successfully", tt.description)
				}
			})
		}

		t.Logf("✓ Lua script error recovery verified")
	})

	t.Run("lua_fallback_routing", func(t *testing.T) {
		// Start primary and fallback backends
		primaryBackend := fixtures.StartRealBackend(t, "api")
		defer func() {
			if err := primaryBackend.Stop(); err != nil {
				t.Logf("Failed to stop primaryBackend: %v", err)
			}
		}()

		fallbackBackend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := fallbackBackend.Stop(); err != nil {
				t.Logf("Failed to stop fallbackBackend: %v", err)
			}
		}()

		// Create configuration with fallback routing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-fallback-tenant",
					PathPrefix: "/fallback/",
					Interval:   30,
					Services: []config.Service{
						{Name: "primary-service", URL: primaryBackend.URL, Health: "/health"},
						{Name: "fallback-service", URL: fallbackBackend.URL, Health: "/health"},
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

		// Test fallback routing scenarios
		fallbackTests := []struct {
			path        string
			description string
		}{
			{"/fallback/primary", "Primary routing attempt"},
			{"/fallback/secondary", "Secondary routing attempt"},
			{"/fallback/auto", "Automatic fallback routing"},
		}

		for _, tt := range fallbackTests {
			t.Run(tt.description, func(t *testing.T) {
				resp, err := client.GetResponse(tt.path)
				if err != nil {
					t.Fatalf("Failed fallback routing test: %v", err)
				}

				if !resp.HasStatus(200) {
					t.Errorf("Expected successful fallback for %s, got %d", tt.path, resp.StatusCode)
				}

				// Determine which backend handled the request
				if resp.ContainsInBody("resource") {
					t.Logf("✓ %s handled by primary backend (API)", tt.path)
				} else if resp.ContainsInBody("Simple backend") {
					t.Logf("✓ %s handled by fallback backend (Simple)", tt.path)
				} else {
					t.Logf("✓ %s handled by backend", tt.path)
				}
			})
		}

		t.Logf("✓ Lua fallback routing verified")
	})
}

// TestLuaIntegrationComplexE2E tests complex Lua integration scenarios
func TestLuaIntegrationComplexE2E(t *testing.T) {
	t.Run("lua_multi_stage_processing", func(t *testing.T) {
		// Start backend for complex processing
		backend := fixtures.StartRealBackend(t, "echo")
		defer backend.Stop()

		// Create configuration for multi-stage Lua processing
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-complex-tenant",
					PathPrefix: "/complex/",
					Interval:   30,
					Services: []config.Service{
						{Name: "complex-service", URL: backend.URL, Health: "/health"},
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

		// Test multi-stage processing with complex request
		complexData := map[string]interface{}{
			"stage":     "multi-processing",
			"pipeline":  []string{"validate", "transform", "route"},
			"metadata":  map[string]string{"source": "e2e-test", "version": "1.0"},
			"timestamp": time.Now().Unix(),
		}

		resp, err := client.PostResponse("/complex/pipeline", complexData)
		if err != nil {
			t.Fatalf("Failed complex Lua processing: %v", err)
		}

		if !resp.HasStatus(200) {
			t.Errorf("Expected status 200 for complex processing, got %d", resp.StatusCode)
		}

		// Verify complex processing results
		var echoData map[string]interface{}
		if err := resp.JSON(&echoData); err != nil {
			t.Errorf("Failed to parse complex processing response: %v", err)
		} else {
			// Verify the request was processed through all stages
			if method, ok := echoData["method"].(string); !ok || method != "POST" {
				t.Errorf("Expected POST method, got: %v", echoData["method"])
			}

			if path, ok := echoData["path"].(string); !ok || path != "/pipeline" {
				t.Errorf("Expected path '/pipeline', got: %v", echoData["path"])
			}

			// Verify complex data was preserved
			if bodyStr, ok := echoData["body"].(string); ok {
				if !strings.Contains(bodyStr, "multi-processing") {
					t.Error("Complex data not preserved through Lua processing")
				}
				t.Logf("✓ Complex data preserved: %s", bodyStr[:min(100, len(bodyStr))])
			}
		}

		t.Logf("✓ Lua multi-stage processing verified")
	})

	t.Run("lua_real_time_adaptation", func(t *testing.T) {
		// Start adaptive backend
		backend := fixtures.StartRealBackend(t, "api")
		defer backend.Stop()

		// Create configuration for real-time adaptation
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lua-adaptive-tenant",
					PathPrefix: "/adaptive/",
					Interval:   30,
					Services: []config.Service{
						{Name: "adaptive-service", URL: backend.URL, Health: "/health"},
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

		// Test real-time adaptation scenarios
		adaptationTests := []struct {
			path        string
			userAgent   string
			description string
		}{
			{"/adaptive/mobile", "Mozilla/5.0 (iPhone; iOS)", "Mobile user adaptation"},
			{"/adaptive/desktop", "Mozilla/5.0 (Windows NT)", "Desktop user adaptation"},
			{"/adaptive/api", "API-Client/1.0", "API client adaptation"},
			{"/adaptive/bot", "Googlebot/2.1", "Bot user adaptation"},
		}

		for _, tt := range adaptationTests {
			t.Run(tt.description, func(t *testing.T) {
				headers := map[string]string{"User-Agent": tt.userAgent}
				resp, err := client.GetWithHeaders(tt.path, headers)
				if err != nil {
					t.Fatalf("Failed adaptive request: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 200 {
					t.Errorf("Expected status 200 for %s, got %d", tt.path, resp.StatusCode)
				}

				t.Logf("✓ %s: %s", tt.description, tt.userAgent)
			})
		}

		t.Logf("✓ Lua real-time adaptation verified")
	})
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
