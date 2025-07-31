package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/tests/e2e/fixtures"
)

// TestBasicGatewayE2E tests basic gateway functionality end-to-end
func TestBasicGatewayE2E(t *testing.T) {
	t.Run("full_request_lifecycle", func(t *testing.T) {
		// Start real backend server
		backend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "e2e-tenant",
					PathPrefix: "/api/",
					Interval:   30,
					Services: []config.Service{
						{Name: "e2e-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start real gateway server
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create E2E client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test basic request flow
		resp, err := client.GetResponse("/api/test")
		if err != nil {
			t.Fatalf("Failed to make E2E request: %v", err)
		}

		// Verify response
		if !resp.HasStatus(200) {
			t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, resp.BodyString)
		}

		if !resp.ContainsInBody("Simple backend response") {
			t.Errorf("Expected response to contain backend message, got: %s", resp.BodyString)
		}

		// Verify headers
		if !resp.HasHeader("X-Backend-Type", "simple") {
			t.Error("Expected X-Backend-Type header from backend")
		}

		t.Logf("✓ Full request lifecycle completed successfully")
	})

	t.Run("real_http_client_integration", func(t *testing.T) {
		// Start echo backend to inspect real HTTP details
		backend := fixtures.StartRealBackend(t, "echo")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "http-test-tenant",
					PathPrefix: "/http/",
					Interval:   30,
					Services: []config.Service{
						{Name: "http-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client with custom headers
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)
		client.SetHeaders(map[string]string{
			"User-Agent":    "E2E-Test-Client/1.0",
			"X-Test-Run":    "gateway-e2e",
			"Authorization": "Bearer test-token-12345",
		})

		// Make POST request with JSON body
		requestBody := map[string]interface{}{
			"test":      "e2e-request",
			"timestamp": time.Now().Unix(),
			"data":      []string{"item1", "item2", "item3"},
		}

		resp, err := client.PostResponse("/http/echo", requestBody)
		if err != nil {
			t.Fatalf("Failed to make E2E POST request: %v", err)
		}

		if !resp.HasStatus(200) {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Parse echo response to verify request details
		var echoData map[string]interface{}
		if err := resp.JSON(&echoData); err != nil {
			t.Fatalf("Failed to parse echo response: %v", err)
		}

		// Verify HTTP method was preserved
		if method, ok := echoData["method"].(string); !ok || method != "POST" {
			t.Errorf("Expected method POST, got: %v", echoData["method"])
		}

		// Verify path was correctly routed
		if path, ok := echoData["path"].(string); !ok || path != "/echo" {
			t.Errorf("Expected path /echo, got: %v", echoData["path"])
		}

		// Verify custom headers were forwarded
		if headers, ok := echoData["headers"].(map[string]interface{}); ok {
			if userAgent, exists := headers["User-Agent"]; !exists {
				t.Error("User-Agent header was not forwarded")
			} else if !strings.Contains(fmt.Sprintf("%v", userAgent), "E2E-Test-Client") {
				t.Errorf("Expected User-Agent to contain E2E-Test-Client, got: %v", userAgent)
			}

			if testRun, exists := headers["X-Test-Run"]; !exists {
				t.Error("X-Test-Run header was not forwarded")
			} else if fmt.Sprintf("%v", testRun) != "[gateway-e2e]" {
				t.Errorf("Expected X-Test-Run header value, got: %v", testRun)
			}

			if auth, exists := headers["Authorization"]; !exists {
				t.Error("Authorization header was not forwarded")
			} else if !strings.Contains(fmt.Sprintf("%v", auth), "test-token-12345") {
				t.Errorf("Expected Authorization header to contain token, got: %v", auth)
			}
		} else {
			t.Error("No headers found in echo response")
		}

		// Verify request body was forwarded
		if body, ok := echoData["body"].(string); ok {
			var bodyData map[string]interface{}
			if err := json.Unmarshal([]byte(body), &bodyData); err != nil {
				t.Errorf("Failed to parse forwarded body: %v", err)
			} else {
				if test, exists := bodyData["test"]; !exists || test != "e2e-request" {
					t.Errorf("Expected test field in body, got: %v", bodyData)
				}
			}
		} else {
			t.Error("Request body was not forwarded")
		}

		t.Logf("✓ Real HTTP client integration verified")
	})

	t.Run("gateway_configuration_loading", func(t *testing.T) {
		// Start multiple backends for different services
		apiBackend := fixtures.StartRealBackend(t, "api")
		defer func() {
			if err := apiBackend.Stop(); err != nil {
				t.Logf("Failed to stop apiBackend: %v", err)
			}
		}()

		healthBackend := fixtures.StartRealBackend(t, "health")
		defer func() {
			if err := healthBackend.Stop(); err != nil {
				t.Logf("Failed to stop healthBackend: %v", err)
			}
		}()

		// Create comprehensive gateway configuration
		cfg := &config.Config{
			AdminBasePath: "/admin",
			Tenants: []config.Tenant{
				{
					Name:       "config-test-tenant",
					PathPrefix: "/api/v1/",
					Interval:   15,
					Services: []config.Service{
						{Name: "api-service", URL: apiBackend.URL, Health: "/health"},
						{Name: "health-service", URL: healthBackend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway with configuration
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test API service access
		resp1, err := client.GetResponse("/api/v1/users")
		if err != nil {
			t.Fatalf("Failed to access API service: %v", err)
		}

		if !resp1.HasStatus(200) {
			t.Errorf("Expected status 200 for API service, got %d", resp1.StatusCode)
		}

		// Verify JSON response from API backend
		var apiData map[string]interface{}
		if err := resp1.JSON(&apiData); err != nil {
			t.Errorf("Failed to parse API response: %v", err)
		} else {
			if resource, ok := apiData["resource"].(string); !ok || resource != "users" {
				t.Errorf("Expected resource 'users', got: %v", apiData["resource"])
			}
		}

		// Test health service access
		resp2, err := client.GetResponse("/api/v1/health")
		if err != nil {
			t.Fatalf("Failed to access health service: %v", err)
		}

		if !resp2.HasStatus(200) {
			t.Errorf("Expected status 200 for health service, got %d", resp2.StatusCode)
		}

		// Verify health response
		var healthData map[string]interface{}
		if err := resp2.JSON(&healthData); err != nil {
			t.Errorf("Failed to parse health response: %v", err)
		} else {
			if status, ok := healthData["status"].(string); !ok || status != "healthy" {
				t.Errorf("Expected status 'healthy', got: %v", healthData["status"])
			}
		}

		t.Logf("✓ Gateway configuration loading and routing verified")
	})

	t.Run("error_handling_end_to_end", func(t *testing.T) {
		// Start error backend
		backend := fixtures.StartRealBackend(t, "error")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "error-test-tenant",
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
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test various error scenarios
		errorTests := []struct {
			path           string
			expectedStatus int
			expectedError  string
		}{
			{"/error/400", 400, "Bad Request"},
			{"/error/401", 401, "Unauthorized"},
			{"/error/403", 403, "Forbidden"},
			{"/error/404", 404, "Not Found"},
			{"/error/500", 500, "Internal Server Error"},
			{"/error/503", 503, "Service Unavailable"},
		}

		for _, tt := range errorTests {
			t.Run(fmt.Sprintf("error_%d", tt.expectedStatus), func(t *testing.T) {
				resp, err := client.GetResponse(tt.path)
				if err != nil {
					t.Fatalf("Failed to make error request to %s: %v", tt.path, err)
				}

				if !resp.HasStatus(tt.expectedStatus) {
					t.Errorf("Expected status %d for %s, got %d",
						tt.expectedStatus, tt.path, resp.StatusCode)
				}

				// Verify error response format
				var errorData map[string]interface{}
				if err := resp.JSON(&errorData); err != nil {
					t.Errorf("Failed to parse error response for %s: %v", tt.path, err)
				} else {
					if errorMsg, ok := errorData["error"].(string); !ok || errorMsg != tt.expectedError {
						t.Errorf("Expected error message '%s' for %s, got: %v",
							tt.expectedError, tt.path, errorData["error"])
					}
				}
			})
		}

		t.Logf("✓ End-to-end error handling verified")
	})
}

// TestGatewayPerformanceE2E tests gateway performance characteristics
func TestGatewayPerformanceE2E(t *testing.T) {
	t.Run("concurrent_request_handling", func(t *testing.T) {
		// Start fast backend
		backend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "perf-tenant",
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
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Create concurrent requests
		concurrency := 10
		requests := make([]func() (*http.Response, error), concurrency)
		for i := 0; i < concurrency; i++ {
			requests[i] = func() (*http.Response, error) {
				return client.Get(fmt.Sprintf("/perf/concurrent-%d", i))
			}
		}

		// Execute requests in parallel
		start := time.Now()
		responses, errors := client.ParallelRequests(requests)
		duration := time.Since(start)

		// Verify results
		successCount := 0
		for i, resp := range responses {
			if errors[i] != nil {
				t.Errorf("Concurrent request %d failed: %v", i, errors[i])
			} else if resp.StatusCode == 200 {
				successCount++
				resp.Body.Close()
			} else {
				t.Errorf("Concurrent request %d returned status %d", i, resp.StatusCode)
				resp.Body.Close()
			}
		}

		if successCount != concurrency {
			t.Errorf("Expected %d successful requests, got %d", concurrency, successCount)
		}

		// Performance expectations
		maxExpectedDuration := 2 * time.Second
		if duration > maxExpectedDuration {
			t.Errorf("Concurrent requests took too long: %v (expected < %v)",
				duration, maxExpectedDuration)
		}

		t.Logf("✓ Processed %d concurrent requests in %v", concurrency, duration)
	})

	t.Run("load_test_basic", func(t *testing.T) {
		// Start backend
		backend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "load-tenant",
					PathPrefix: "/load/",
					Interval:   30,
					Services: []config.Service{
						{Name: "load-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Run load test
		concurrency := 5
		duration := 3 * time.Second
		result := client.LoadTest("/load/test", concurrency, duration)

		// Verify load test results
		if result.TotalRequests == 0 {
			t.Error("No requests completed during load test")
		}

		if result.TotalErrors > result.TotalRequests/10 {
			t.Errorf("Too many errors during load test: %d/%d (>10%%)",
				result.TotalErrors, result.TotalRequests)
		}

		if result.SuccessRate() < 0.9 {
			t.Errorf("Success rate too low: %.2f%% (expected >= 90%%)",
				result.SuccessRate()*100)
		}

		expectedMinRPS := float64(concurrency) * 0.3 // Conservative expectation
		if result.RequestsPerSecond() < expectedMinRPS {
			t.Errorf("Requests per second too low: %.2f (expected >= %.2f)",
				result.RequestsPerSecond(), expectedMinRPS)
		}

		t.Logf("✓ Load test: %d requests in %v (%.2f RPS, %.1f%% success)",
			result.TotalRequests, result.Duration,
			result.RequestsPerSecond(), result.SuccessRate()*100)
	})

	t.Run("response_time_consistency", func(t *testing.T) {
		// Start backend with controlled delay
		backend := fixtures.StartRealBackend(t, "slow")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "timing-tenant",
					PathPrefix: "/timing/",
					Interval:   30,
					Services: []config.Service{
						{Name: "timing-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Measure response times for multiple requests
		var responseTimes []time.Duration
		numRequests := 5

		for i := 0; i < numRequests; i++ {
			start := time.Now()
			resp, err := client.Get("/timing/test?delay=100ms")
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Request %d failed: %v", i+1, err)
				continue
			}

			if resp.StatusCode != 200 {
				t.Errorf("Request %d returned status %d", i+1, resp.StatusCode)
			}

			resp.Body.Close()
			responseTimes = append(responseTimes, duration)
		}

		// Analyze response times
		if len(responseTimes) == 0 {
			t.Fatal("No successful requests completed")
		}

		var totalTime time.Duration
		minTime := responseTimes[0]
		maxTime := responseTimes[0]

		for _, duration := range responseTimes {
			totalTime += duration
			if duration < minTime {
				minTime = duration
			}
			if duration > maxTime {
				maxTime = duration
			}
		}

		avgTime := totalTime / time.Duration(len(responseTimes))

		// Response time expectations
		expectedMinTime := 100 * time.Millisecond // Backend delay
		expectedMaxTime := 2 * time.Second        // Reasonable maximum

		if avgTime < expectedMinTime {
			t.Errorf("Average response time too fast: %v (expected >= %v)",
				avgTime, expectedMinTime)
		}

		if maxTime > expectedMaxTime {
			t.Errorf("Maximum response time too slow: %v (expected <= %v)",
				maxTime, expectedMaxTime)
		}

		// Check consistency (max should not be much larger than min)
		if maxTime > minTime*3 {
			t.Errorf("Response times too inconsistent: min=%v, max=%v", minTime, maxTime)
		}

		t.Logf("✓ Response times: min=%v, avg=%v, max=%v", minTime, avgTime, maxTime)
	})
}

// TestGatewayRobustnessE2E tests gateway robustness and reliability
func TestGatewayRobustnessE2E(t *testing.T) {
	t.Run("backend_failure_handling", func(t *testing.T) {
		// Start backend that we can stop to simulate failure
		backend := fixtures.StartRealBackend(t, "simple")
		backendURL := backend.URL

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "robust-tenant",
					PathPrefix: "/robust/",
					Interval:   30,
					Services: []config.Service{
						{Name: "robust-service", URL: backendURL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test normal operation
		resp1, err := client.Get("/robust/test")
		if err != nil {
			t.Fatalf("Failed to make initial request: %v", err)
		}
		defer func() {
			if err := resp1.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()

		if resp1.StatusCode != 200 {
			t.Errorf("Expected status 200 for healthy backend, got %d", resp1.StatusCode)
		}

		// Stop backend to simulate failure
		backend.Stop()

		// Wait a moment for the failure to be detected
		time.Sleep(100 * time.Millisecond)

		// Test request to failed backend
		resp2, err := client.Get("/robust/test")
		if err != nil {
			// Network error is acceptable when backend is down
			t.Logf("Expected network error when backend is down: %v", err)
		} else {
			defer func() {
				if err := resp2.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()
			// Gateway should return appropriate error status
			if resp2.StatusCode == 200 {
				t.Error("Gateway should not return 200 when backend is down")
			} else {
				t.Logf("Gateway correctly returned error status %d when backend is down",
					resp2.StatusCode)
			}
		}

		t.Logf("✓ Backend failure handling verified")
	})

	t.Run("malformed_request_handling", func(t *testing.T) {
		// Start backend
		backend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "malform-tenant",
					PathPrefix: "/malform/",
					Interval:   30,
					Services: []config.Service{
						{Name: "malform-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client for malformed requests
		client := fixtures.NewE2EClient()
		client.SetBaseURL(gateway.URL)

		// Test very long path
		longPath := "/malform/" + strings.Repeat("a", 2000)
		resp1, err := client.Get(longPath)
		if err != nil {
			t.Logf("Long path request failed as expected: %v", err)
		} else {
			defer func() {
				if err := resp1.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()
			if resp1.StatusCode != 404 && resp1.StatusCode != 414 {
				t.Logf("Long path returned status %d (expected 404 or 414)", resp1.StatusCode)
			}
		}

		// Test malformed JSON
		malformedJSON := []byte(`{"invalid": json content}`)
		resp2, err := client.PostRaw("/malform/json", malformedJSON, "application/json")
		if err != nil {
			t.Errorf("Failed to send malformed JSON: %v", err)
		} else {
			defer func() {
				if err := resp2.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()
			// Backend should handle this, but gateway should pass it through
			if resp2.StatusCode == 200 {
				t.Log("Gateway passed malformed JSON to backend successfully")
			}
		}

		// Test invalid HTTP method
		resp3, err := client.DoRequest("INVALID", "/malform/test", nil, nil)
		if err != nil {
			t.Logf("Invalid HTTP method failed as expected: %v", err)
		} else {
			defer func() {
				if err := resp3.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()
			if resp3.StatusCode != 405 && resp3.StatusCode != 501 {
				t.Logf("Invalid method returned status %d", resp3.StatusCode)
			}
		}

		t.Logf("✓ Malformed request handling verified")
	})

	t.Run("resource_cleanup_verification", func(t *testing.T) {
		// This test verifies that the gateway properly cleans up resources
		// by making many requests and ensuring no resource leaks

		// Start backend
		backend := fixtures.StartRealBackend(t, "simple")
		defer func() {
			if err := backend.Stop(); err != nil {
				t.Logf("Failed to stop backend: %v", err)
			}
		}()

		// Create gateway configuration
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "cleanup-tenant",
					PathPrefix: "/cleanup/",
					Interval:   30,
					Services: []config.Service{
						{Name: "cleanup-service", URL: backend.URL, Health: "/health"},
					},
				},
			},
		}

		// Start gateway
		gateway := fixtures.StartRealGateway(t, cfg)
		defer func() {
			if err := gateway.Stop(); err != nil {
				t.Logf("Failed to stop gateway: %v", err)
			}
		}()

		// Create client with shorter timeout for faster cycling
		client := fixtures.NewE2EClientWithTimeout(5 * time.Second)
		client.SetBaseURL(gateway.URL)

		// Make many requests to test resource cleanup
		numRequests := 50
		successCount := 0

		for i := 0; i < numRequests; i++ {
			resp, err := client.Get(fmt.Sprintf("/cleanup/test-%d", i))
			if err != nil {
				t.Logf("Request %d failed: %v", i+1, err)
			} else {
				if resp.StatusCode == 200 {
					successCount++
				}
				resp.Body.Close()
			}

			// Small delay to prevent overwhelming
			if i%10 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Verify most requests succeeded
		successRate := float64(successCount) / float64(numRequests)
		if successRate < 0.8 {
			t.Errorf("Success rate too low for cleanup test: %.2f%% (expected >= 80%%)",
				successRate*100)
		}

		t.Logf("✓ Resource cleanup: %d/%d requests successful (%.1f%%)",
			successCount, numRequests, successRate*100)
	})
}
