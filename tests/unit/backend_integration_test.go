package unit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keystone-gateway/tests/fixtures"
)

// TestErrorBackendIntegration tests integration with error-generating backends
func TestErrorBackendIntegration(t *testing.T) {
	env := fixtures.SetupErrorProxy(t, "error-tenant", "/error/", "/error/*")
	defer env.Cleanup()

	testCases := []fixtures.HTTPTestCase{
		{
			Name:           "backend 500 error propagation",
			Method:         "GET",
			Path:           "/error/500",
			ExpectedStatus: http.StatusInternalServerError,
		},
		{
			Name:           "backend 404 error propagation",
			Method:         "GET",
			Path:           "/error/404",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "backend 400 error propagation",
			Method:         "GET", 
			Path:           "/error/400",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "backend 503 service unavailable",
			Method:         "GET",
			Path:           "/error/503",
			ExpectedStatus: http.StatusServiceUnavailable,
		},
		{
			Name:           "POST request to error backend",
			Method:         "POST",
			Path:           "/error/500",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"test": "data"}`,
			ExpectedStatus: http.StatusInternalServerError,
		},
		{
			Name:           "PUT request to error backend",
			Method:         "PUT",
			Path:           "/error/404",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"update": "data"}`,
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "DELETE request to error backend",
			Method:         "DELETE",
			Path:           "/error/400",
			ExpectedStatus: http.StatusBadRequest,
		},
	}

	fixtures.RunHTTPTestCases(t, env.Router, testCases)
}

// TestSlowBackendIntegration tests integration with slow backends
func TestSlowBackendIntegration(t *testing.T) {
	// Create proxy with 100ms delay backend
	slowBackend := fixtures.CreateSlowBackend(t, 100*time.Millisecond)
	defer slowBackend.Close()

	env := fixtures.SetupProxy(t, "slow-tenant", "/slow/", slowBackend)
	defer env.Cleanup()

	testCases := []struct {
		name           string
		method         string
		path           string
		body           string
		headers        map[string]string
		expectedStatus int
		maxDuration    time.Duration
	}{
		{
			name:           "GET request to slow backend",
			method:         "GET",
			path:           "/slow/data",
			expectedStatus: http.StatusOK,
			maxDuration:    500 * time.Millisecond, // Should complete within 500ms
		},
		{
			name:           "POST request with data to slow backend",
			method:         "POST",
			path:           "/slow/upload",
			headers:        map[string]string{"Content-Type": "application/json"},
			body:           `{"large": "` + strings.Repeat("x", 1024) + `"}`,
			expectedStatus: http.StatusOK,
			maxDuration:    1 * time.Second,
		},
		{
			name:           "concurrent requests to slow backend",
			method:         "GET",
			path:           "/slow/concurrent",
			expectedStatus: http.StatusOK,
			maxDuration:    300 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			testCase := fixtures.HTTPTestCase{
				Name:           tc.name,
				Method:         tc.method,
				Path:           tc.path,
				Headers:        tc.headers,
				Body:           tc.body,
				ExpectedStatus: tc.expectedStatus,
			}

			fixtures.RunHTTPTestCases(t, env.Router, []fixtures.HTTPTestCase{testCase})

			duration := time.Since(start)
			if duration > tc.maxDuration {
				t.Errorf("Request took too long: %v > %v", duration, tc.maxDuration)
			}
		})
	}

	// Test concurrent requests
	t.Run("concurrent slow requests", func(t *testing.T) {
		concurrency := 5
		done := make(chan time.Duration, concurrency)

		start := time.Now()
		for i := 0; i < concurrency; i++ {
			go func() {
				requestStart := time.Now()
				testCase := fixtures.HTTPTestCase{
					Name:           "concurrent request",
					Method:         "GET",
					Path:           "/slow/concurrent",
					ExpectedStatus: http.StatusOK,
				}
				fixtures.RunHTTPTestCases(t, env.Router, []fixtures.HTTPTestCase{testCase})
				done <- time.Since(requestStart)
			}()
		}

		// Collect all results
		var maxDuration time.Duration
		for i := 0; i < concurrency; i++ {
			duration := <-done
			if duration > maxDuration {
				maxDuration = duration
			}
		}

		totalTime := time.Since(start)
		
		// All requests should complete relatively quickly due to concurrency
		if totalTime > 1*time.Second {
			t.Errorf("Concurrent requests took too long: %v", totalTime)
		}
	})
}

// TestEchoBackendIntegration tests integration with echo backends for request inspection
func TestEchoBackendIntegration(t *testing.T) {
	env := fixtures.SetupEchoProxy(t, "echo-tenant", "/echo/", "/echo/*")
	defer env.Cleanup()

	testCases := []struct {
		name            string
		method          string
		path            string
		headers         map[string]string
		body            string
		checkResponse   func(t *testing.T, body string)
	}{
		{
			name:   "echo GET request details",
			method: "GET",
			path:   "/echo/test",
			headers: map[string]string{
				"User-Agent":   "test-client/1.0",
				"X-Custom":     "custom-value",
			},
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "GET") {
					t.Error("Response should contain GET method")
				}
				if !strings.Contains(body, "/test") {
					t.Error("Response should contain request path")
				}
				if !strings.Contains(body, "test-client/1.0") {
					t.Error("Response should contain User-Agent header")
				}
				if !strings.Contains(body, "custom-value") {
					t.Error("Response should contain custom header")
				}
			},
		},
		{
			name:   "echo POST request with JSON body",
			method: "POST",
			path:   "/echo/data",
			headers: map[string]string{
				"Content-Type": "application/json",
				"Authorization": "Bearer token123",
			},
			body: `{"message": "test data", "id": 123}`,
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "POST") {
					t.Error("Response should contain POST method")
				}
				if !strings.Contains(body, "application/json") {
					t.Error("Response should contain Content-Type")
				}
				if !strings.Contains(body, "Bearer token123") {
					t.Error("Response should contain Authorization header")
				}
				if !strings.Contains(body, "test data") {
					t.Error("Response should contain request body")
				}
			},
		},
		{
			name:   "echo PUT request with form data",
			method: "PUT",
			path:   "/echo/form",
			headers: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			body: "name=test&value=123&description=form%20data",
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "PUT") {
					t.Error("Response should contain PUT method")
				}
				if !strings.Contains(body, "application/x-www-form-urlencoded") {
					t.Error("Response should contain form content type")
				}
				if !strings.Contains(body, "name=test") {
					t.Error("Response should contain form data")
				}
			},
		},
		{
			name:   "echo request with query parameters",
			method: "GET",
			path:   "/echo/search?q=test&page=1&limit=10",
			headers: map[string]string{
				"Accept": "application/json",
			},
			checkResponse: func(t *testing.T, body string) {
				if !strings.Contains(body, "q=test") {
					t.Error("Response should contain query parameters")
				}
				if !strings.Contains(body, "page=1") {
					t.Error("Response should contain page parameter")
				}
				if !strings.Contains(body, "limit=10") {
					t.Error("Response should contain limit parameter")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use custom test execution to check response body
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			resp := fixtures.ExecuteHTTPTestWithRequest(env.Router, req)
			
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			body := resp.Body
			tc.checkResponse(t, body)
		})
	}
}

// TestHeaderEchoBackendIntegration tests header echo backend functionality
func TestHeaderEchoBackendIntegration(t *testing.T) {
	headerEchoBackend := fixtures.CreateHeaderEchoBackend(t)
	env := fixtures.SetupProxy(t, "header-echo-tenant", "/headers/", headerEchoBackend)
	defer env.Cleanup()

	testCases := []struct {
		name     string
		headers  map[string]string
		checkHeaders func(t *testing.T, headers map[string]string)
	}{
		{
			name: "basic headers echo",
			headers: map[string]string{
				"X-Test-Header": "test-value",
				"User-Agent":    "test-client",
				"Accept":        "application/json",
			},
			checkHeaders: func(t *testing.T, headers map[string]string) {
				if headers["X-Test-Header"] != "test-value" {
					t.Error("Custom header not echoed correctly")
				}
				if headers["User-Agent"] != "test-client" {
					t.Error("User-Agent not echoed correctly")
				}
				if headers["Accept"] != "application/json" {
					t.Error("Accept header not echoed correctly")
				}
			},
		},
		{
			name: "authorization headers",
			headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				"X-API-Key":     "secret-api-key-123",
			},
			checkHeaders: func(t *testing.T, headers map[string]string) {
				if !strings.Contains(headers["Authorization"], "Bearer") {
					t.Error("Authorization header not echoed correctly")
				}
				if headers["X-API-Key"] != "secret-api-key-123" {
					t.Error("API key header not echoed correctly")
				}
			},
		},
		{
			name: "content headers",
			headers: map[string]string{
				"Content-Type":     "application/json; charset=utf-8",
				"Content-Encoding": "gzip",
				"Content-Language": "en-US",
			},
			checkHeaders: func(t *testing.T, headers map[string]string) {
				if !strings.Contains(headers["Content-Type"], "application/json") {
					t.Error("Content-Type not echoed correctly")
				}
				if headers["Content-Encoding"] != "gzip" {
					t.Error("Content-Encoding not echoed correctly")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/headers/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			resp := fixtures.ExecuteHTTPTestWithRequest(env.Router, req)
			
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Check that response headers contain echoed request headers
			responseHeaders := make(map[string]string)
			for key, values := range resp.Headers {
				if len(values) > 0 {
					responseHeaders[key] = values[0]
				}
			}

			tc.checkHeaders(t, responseHeaders)
		})
	}
}

// TestDropConnectionBackendIntegration tests connection dropping scenarios
func TestDropConnectionBackendIntegration(t *testing.T) {
	dropBackend := fixtures.CreateDropConnectionBackend(t)
	env := fixtures.SetupProxy(t, "drop-tenant", "/drop/", dropBackend)
	defer env.Cleanup()

	testCases := []struct {
		name           string
		method         string
		path           string
		expectError    bool
		timeoutLimit   time.Duration
	}{
		{
			name:         "GET request to dropping backend",
			method:       "GET",
			path:         "/drop/test",
			expectError:  true,
			timeoutLimit: 5 * time.Second,
		},
		{
			name:         "POST request to dropping backend",
			method:       "POST",
			path:         "/drop/data",
			expectError:  true,
			timeoutLimit: 5 * time.Second,
		},
		{
			name:         "PUT request to dropping backend",
			method:       "PUT",
			path:         "/drop/update",
			expectError:  true,
			timeoutLimit: 5 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp := fixtures.ExecuteHTTPTestWithRequest(env.Router, req)
			
			duration := time.Since(start)

			// Connection drop should result in 502 Bad Gateway or similar error
			if resp.StatusCode == http.StatusOK {
				t.Error("Expected error status due to dropped connection, got 200")
			}

			// Should not take longer than timeout limit
			if duration > tc.timeoutLimit {
				t.Errorf("Request took too long: %v > %v", duration, tc.timeoutLimit)
			}
		})
	}
}

// TestCustomBackendBehavior tests custom backend behavior configuration
func TestCustomBackendBehavior(t *testing.T) {
	// Create custom backend with specific behaviors
	behavior := fixtures.BackendBehavior{
		ResponseMap: map[string]fixtures.BackendResponse{
			"/api/users": {
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"users": ["alice", "bob"]}`,
			},
			"/api/auth": {
				StatusCode: http.StatusUnauthorized,
				Headers:    map[string]string{"WWW-Authenticate": "Bearer"},
				Body:       `{"error": "unauthorized"}`,
			},
			"/api/slow": {
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Content-Type": "text/plain"},
				Body:       "slow response",
				Delay:      200 * time.Millisecond,
			},
		},
	}

	customBackend := fixtures.CreateCustomBackend(t, behavior)
	defer customBackend.Close()

	env := fixtures.SetupProxy(t, "custom-tenant", "/api/", customBackend)
	defer env.Cleanup()

	testCases := []fixtures.HTTPTestCase{
		{
			Name:           "users endpoint with custom response",
			Method:         "GET",
			Path:           "/api/users",
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"users": ["alice", "bob"]}`,
			CheckHeaders:   map[string]string{"Content-Type": "application/json"},
		},
		{
			Name:           "auth endpoint with unauthorized response",
			Method:         "GET",
			Path:           "/api/auth",
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedBody:   `{"error": "unauthorized"}`,
			CheckHeaders:   map[string]string{"WWW-Authenticate": "Bearer"},
		},
		{
			Name:           "slow endpoint with delay",
			Method:         "GET",
			Path:           "/api/slow",
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   "slow response",
			CheckHeaders:   map[string]string{"Content-Type": "text/plain"},
		},
		{
			Name:           "undefined endpoint falls back to default",
			Method:         "GET",
			Path:           "/api/undefined",
			ExpectedStatus: http.StatusOK, // Default behavior
		},
	}

	// Test slow endpoint timing
	t.Run("slow endpoint timing", func(t *testing.T) {
		start := time.Now()
		
		testCase := fixtures.HTTPTestCase{
			Name:           "timing test",
			Method:         "GET",
			Path:           "/api/slow",
			ExpectedStatus: http.StatusOK,
		}
		
		fixtures.RunHTTPTestCases(t, env.Router, []fixtures.HTTPTestCase{testCase})
		
		duration := time.Since(start)
		
		// Should take at least the configured delay
		if duration < 200*time.Millisecond {
			t.Errorf("Request completed too quickly: %v < 200ms", duration)
		}
		
		// But not too much longer
		if duration > 500*time.Millisecond {
			t.Errorf("Request took too long: %v > 500ms", duration)
		}
	})

	fixtures.RunHTTPTestCases(t, env.Router, testCases)
}

// TestBackendIntegrationEdgeCases tests edge cases in backend integration
func TestBackendIntegrationEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		setupFunc func(t *testing.T) (*fixtures.ProxyTestEnv, func())
		testFunc  func(t *testing.T, env *fixtures.ProxyTestEnv)
	}{
		{
			name: "backend with very large response",
			setupFunc: func(t *testing.T) (*fixtures.ProxyTestEnv, func()) {
				largeBody := strings.Repeat("x", 1024*1024) // 1MB response
				behavior := fixtures.BackendBehavior{
					ResponseMap: map[string]fixtures.BackendResponse{
						"/large": {
							StatusCode: http.StatusOK,
							Headers:    map[string]string{"Content-Type": "text/plain"},
							Body:       largeBody,
						},
					},
				}
				backend := fixtures.CreateCustomBackend(t, behavior)
				env := fixtures.SetupProxy(t, "large-tenant", "/large/", backend)
				return env, func() { backend.Close(); env.Cleanup() }
			},
			testFunc: func(t *testing.T, env *fixtures.ProxyTestEnv) {
				req := httptest.NewRequest("GET", "/large/large", nil)
				resp := fixtures.ExecuteHTTPTestWithRequest(env.Router, req)
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}
				
				body := resp.Body
				if len(body) != 1024*1024 {
					t.Errorf("Expected body length 1MB, got %d bytes", len(body))
				}
			},
		},
		{
			name: "backend with special characters in response",
			setupFunc: func(t *testing.T) (*fixtures.ProxyTestEnv, func()) {
				specialBody := "Unicode: 测试 🚀 émojis and special chars: !@#$%^&*()"
				behavior := fixtures.BackendBehavior{
					ResponseMap: map[string]fixtures.BackendResponse{
						"/special": {
							StatusCode: http.StatusOK,
							Headers:    map[string]string{"Content-Type": "text/plain; charset=utf-8"},
							Body:       specialBody,
						},
					},
				}
				backend := fixtures.CreateCustomBackend(t, behavior)
				env := fixtures.SetupProxy(t, "special-tenant", "/special/", backend)
				return env, func() { backend.Close(); env.Cleanup() }
			},
			testFunc: func(t *testing.T, env *fixtures.ProxyTestEnv) {
				req := httptest.NewRequest("GET", "/special/special", nil)
				resp := fixtures.ExecuteHTTPTestWithRequest(env.Router, req)
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}
				
				if !strings.Contains(resp.Body, "测试") {
					t.Error("Response should contain Unicode characters")
				}
				if !strings.Contains(resp.Body, "🚀") {
					t.Error("Response should contain emoji")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env, cleanup := tc.setupFunc(t)
			defer cleanup()
			tc.testFunc(t, env)
		})
	}
}