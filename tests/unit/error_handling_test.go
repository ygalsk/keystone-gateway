package unit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/tests/fixtures"
)

// TestConfigurationErrorHandling tests error handling in configuration loading
func TestConfigurationErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		configContent  string
		expectError    bool
		errorSubstring string
	}{
		{
			name: "malformed YAML syntax",
			configContent: `
tenants:
  - name: test
    invalid_yaml: [
    missing_closing_bracket
`,
			expectError:    true,
			errorSubstring: "failed to parse config",
		},
		{
			name: "tenant with invalid domain format",
			configContent: `
tenants:
  - name: invalid-domain-tenant
    domains: ["invalid domain with spaces"]
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
`,
			expectError:    true,
			errorSubstring: "invalid domain",
		},
		{
			name: "tenant missing both domain and path",
			configContent: `
tenants:
  - name: incomplete-tenant
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
`,
			expectError:    true,
			errorSubstring: "must specify either domains or path_prefix",
		},
		{
			name: "tenant with malformed path prefix",
			configContent: `
tenants:
  - name: bad-path-tenant
    path_prefix: "missing-slashes"
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
`,
			expectError:    true,
			errorSubstring: "path_prefix must start and end with '/'",
		},
		{
			name: "empty tenant name",
			configContent: `
tenants:
  - name: ""
    path_prefix: /empty/
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
`,
			expectError: false, // Empty name is technically valid, just not recommended
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := tempDir + "/config.yaml"
			
			err := os.WriteFile(configPath, []byte(tc.configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err = config.LoadConfig(configPath)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errorSubstring) {
					t.Errorf("Expected error containing %q, got %q", tc.errorSubstring, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestLuaScriptErrorHandling tests error handling in Lua script execution
func TestLuaScriptErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		script         string
		expectError    bool
		errorSubstring string
	}{
		{
			name: "Lua syntax error",
			script: `
-- Invalid Lua syntax
chi_route("GET", "/test", function(response, request
    -- Missing closing parenthesis and 'end'
    response:write("test")
`,
			expectError:    true,
			errorSubstring: "Lua script execution failed",
		},
		{
			name: "runtime error in script",
			script: `
chi_route("GET", "/error", function(response, request)
    error("Intentional runtime error for testing")
end)
`,
			expectError: false, // Route registration should succeed
		},
		{
			name: "calling undefined function",
			script: `
chi_route("GET", "/undefined", function(response, request)
    undefined_function()
end)
`,
			expectError: false, // Route registration should succeed, error at runtime
		},
		{
			name: "infinite loop causing timeout",
			script: `
while true do
    -- Infinite loop to test timeout handling
end
`,
			expectError:    true,
			errorSubstring: "timeout",
		},
		{
			name: "nil access error",
			script: `
local nil_table = nil
local value = nil_table.some_field
`,
			expectError:    true,
			errorSubstring: "Lua script execution failed",
		},
		{
			name: "invalid chi_route arguments",
			script: `
-- Missing required arguments
chi_route("GET")
`,
			expectError:    true,
			errorSubstring: "chi_route requires method, pattern, and handler function",
		},
		{
			name: "invalid chi_middleware arguments",
			script: `
-- Missing middleware function
chi_middleware("/test/*")
`,
			expectError:    true,
			errorSubstring: "chi_middleware requires pattern and middleware function",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)

			err := env.Engine.ExecuteRouteScript("test-script", "test-tenant")

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tc.errorSubstring != "" && !strings.Contains(err.Error(), tc.errorSubstring) {
					t.Errorf("Expected error containing %q, got %q", tc.errorSubstring, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestHTTPErrorHandling tests HTTP error handling at the gateway level
func TestHTTPErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		setupFunc      func(t *testing.T) (*fixtures.GatewayTestEnv, func())
		requestMethod  string
		requestPath    string
		requestHeaders map[string]string
		requestBody    string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *http.Response)
	}{
		{
			name: "request to non-existent route",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				env := fixtures.SetupSimpleGateway(t, "test-tenant", "/api/")
				return env, func() {}
			},
			requestMethod:  "GET",
			requestPath:    "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "unsupported HTTP method",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				env := fixtures.SetupMethodAwareGateway(t, "test-tenant", "/api/")
				return env, func() { env.Cleanup() }
			},
			requestMethod:  "TRACE",
			requestPath:    "/api/test",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name: "request with malformed headers",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				env := fixtures.SetupSimpleGateway(t, "test-tenant", "/api/")
				return env, func() { env.Cleanup() }
			},
			requestMethod: "GET",
			requestPath:   "/api/test",
			requestHeaders: map[string]string{
				"Invalid\x00Header": "value", // Null byte in header name
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "request to backend that drops connections",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				backend := fixtures.CreateDropConnectionBackend(t)
				proxyEnv := fixtures.SetupProxy(t, "drop-tenant", "/drop/", backend)
				env := &fixtures.GatewayTestEnv{
					Router:  proxyEnv.Router,
					Gateway: proxyEnv.Gateway,
				}
				return env, func() { backend.Close(); proxyEnv.Cleanup() }
			},
			requestMethod:  "GET",
			requestPath:    "/drop/test",
			expectedStatus: http.StatusBadGateway, // or similar connection error
		},
		{
			name: "request to slow backend with timeout",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				// Create very slow backend (5 seconds)
				backend := fixtures.CreateSlowBackend(t, 5*time.Second)
				proxyEnv := fixtures.SetupProxy(t, "slow-tenant", "/slow/", backend)
				env := &fixtures.GatewayTestEnv{
					Router:  proxyEnv.Router,
					Gateway: proxyEnv.Gateway,
				}
				return env, func() { backend.Close(); proxyEnv.Cleanup() }
			},
			requestMethod:  "GET",
			requestPath:    "/slow/test",
			expectedStatus: http.StatusOK, // Should eventually succeed
			checkResponse: func(t *testing.T, resp *http.Response) {
				// Could add timeout checks here
			},
		},
		{
			name: "request with very large body",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				backend := fixtures.CreateEchoBackend(t)
				proxyEnv := fixtures.SetupProxy(t, "echo-tenant", "/echo/", backend)
				env := &fixtures.GatewayTestEnv{
					Router:  proxyEnv.Router,
					Gateway: proxyEnv.Gateway,
				}
				return env, func() { backend.Close(); proxyEnv.Cleanup() }
			},
			requestMethod:  "POST",
			requestPath:    "/echo/large",
			requestBody:    strings.Repeat("x", 1024*1024), // 1MB body
			expectedStatus: http.StatusOK,
		},
		{
			name: "request with percent encoded path",
			setupFunc: func(t *testing.T) (*fixtures.GatewayTestEnv, func()) {
				env := fixtures.SetupRestrictiveGateway(t, "test-tenant", "/api/")
				return env, func() { env.Cleanup() }
			},
			requestMethod:  "GET",
			requestPath:    "/api/test%20encoded", // Valid percent encoding for space  
			expectedStatus: http.StatusNotFound, // Backend only responds to /test, not encoded paths
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env, cleanup := tc.setupFunc(t)
			defer cleanup()

			req := httptest.NewRequest(tc.requestMethod, tc.requestPath, strings.NewReader(tc.requestBody))
			for k, v := range tc.requestHeaders {
				req.Header.Set(k, v)
			}
			resp := fixtures.ExecuteHTTPTestWithRequest(env.Router, req)

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.checkResponse != nil {
				// Convert HTTPTestResult to http.Response for compatibility
				httpResp := &http.Response{
					StatusCode: resp.StatusCode,
					Header:     resp.Headers,
				}
				tc.checkResponse(t, httpResp)
			}
		})
	}
}

// TestRoutingErrorHandling tests error handling in routing logic
func TestRoutingErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		setupConfig    *config.Config
		requestHost    string
		requestPath    string
		expectedResult bool // true if route should be found
	}{
		{
			name: "host with invalid characters",
			setupConfig: &config.Config{
				Tenants: []config.Tenant{{
					Name:     "host-tenant",
					Domains:  []string{"valid.example.com"},
					Services: []config.Service{{Name: "svc", URL: "http://backend:8080", Health: "/health"}},
				}},
			},
			requestHost:    "invalid\x00host.com",
			requestPath:    "/test",
			expectedResult: false,
		},
		{
			name: "extremely long hostname",
			setupConfig: &config.Config{
				Tenants: []config.Tenant{{
					Name:     "host-tenant",
					Domains:  []string{"example.com"},
					Services: []config.Service{{Name: "svc", URL: "http://backend:8080", Health: "/health"}},
				}},
			},
			requestHost:    strings.Repeat("a", 1000) + ".com",
			requestPath:    "/test",
			expectedResult: false,
		},
		{
			name: "path with null bytes",
			setupConfig: &config.Config{
				Tenants: []config.Tenant{{
					Name:       "path-tenant",
					PathPrefix: "/api/",
					Services:   []config.Service{{Name: "svc", URL: "http://backend:8080", Health: "/health"}},
				}},
			},
			requestHost:    "example.com",
			requestPath:    "/api/test\x00path",
			expectedResult: false, // Should not match due to null byte
		},
		{
			name: "IPv6 host handling",
			setupConfig: &config.Config{
				Tenants: []config.Tenant{{
					Name:     "ipv6-tenant",
					Domains:  []string{"[::1]"},
					Services: []config.Service{{Name: "svc", URL: "http://backend:8080", Health: "/health"}},
				}},
			},
			requestHost:    "[::1]:8080",
			requestPath:    "/test",
			expectedResult: true,
		},
		{
			name: "malformed IPv6 host",
			setupConfig: &config.Config{
				Tenants: []config.Tenant{{
					Name:     "ipv6-tenant",
					Domains:  []string{"[::1]"},
					Services: []config.Service{{Name: "svc", URL: "http://backend:8080", Health: "/health"}},
				}},
			},
			requestHost:    "[::1", // Missing closing bracket
			requestPath:    "/test",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupGateway(t, tc.setupConfig)
			
			tenantRouter, _ := env.Gateway.MatchRoute(tc.requestHost, tc.requestPath)
			found := tenantRouter != nil

			if found != tc.expectedResult {
				t.Errorf("Expected route found=%v, got found=%v", tc.expectedResult, found)
			}
		})
	}
}

// TestConcurrentErrorHandling tests error handling under concurrent load
func TestConcurrentErrorHandling(t *testing.T) {
	// Create a setup that might have race conditions
	env := fixtures.SetupMultiTenantGateway(t)
	_ = env

	// Add a Lua script that might have concurrency issues
	script := `
chi_route("GET", "/concurrent", function(response, request)
    -- Simulate some work that might cause issues under load
    for i = 1, 100 do
        local temp = "processing_" .. i
    end
    response:set_header("Content-Type", "text/plain")
    response:write("Concurrent request processed")
end)
`

	luaEnv := fixtures.SetupLuaEngineWithScript(t, script)
	err := luaEnv.Engine.ExecuteRouteScript("test-script", "test-tenant")
	if err != nil {
		t.Fatalf("Failed to setup concurrent test script: %v", err)
	}

	// Mount the Lua routes into the gateway
	err = luaEnv.MountTenantRoutesAtRoot("test-tenant")
	if err != nil {
		t.Fatalf("Failed to mount tenant routes: %v", err)
	}
	
	router := luaEnv.Router

	// Run concurrent requests
	concurrency := 50
	done := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(requestID int) {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic in request %d: %v", requestID, r)
					return
				}
			}()

			req := httptest.NewRequest("GET", "/concurrent", nil)
			resp := fixtures.ExecuteHTTPTestWithRequest(router, req)
			
			if resp.StatusCode != http.StatusOK {
				done <- fmt.Errorf("request %d failed with status %d", requestID, resp.StatusCode)
				return
			}

			if !strings.Contains(resp.Body, "Concurrent request processed") {
				done <- fmt.Errorf("request %d got unexpected body: %s", requestID, resp.Body)
				return
			}

			done <- nil
		}(i)
	}

	// Collect results
	var errors []error
	for i := 0; i < concurrency; i++ {
		if err := <-done; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent requests had %d errors:", len(errors))
		for _, err := range errors[:min(5, len(errors))] { // Show first 5 errors
			t.Logf("  %v", err)
		}
	}
}

// TestMemoryErrorHandling tests handling of memory-related errors
func TestMemoryErrorHandling(t *testing.T) {
	testCases := []struct {
		name          string
		script        string
		expectSuccess bool
	}{
		{
			name: "large table allocation in Lua",
			script: `
chi_route("GET", "/memory", function(response, request)
    local large_table = {}
    for i = 1, 10000 do
        large_table[i] = "data_" .. i
    end
    response:set_header("Content-Type", "text/plain")
    response:write("Memory test completed")
end)
`,
			expectSuccess: true, // Should handle reasonable memory usage
		},
		{
			name: "nested function calls",
			script: `
local function recursive_function(n)
    if n > 0 then
        return recursive_function(n - 1)
    end
    return "done"
end

chi_route("GET", "/recursive", function(response, request)
    local result = recursive_function(100) -- Reasonable recursion depth
    response:set_header("Content-Type", "text/plain") 
    response:write("Recursion test: " .. result)
end)
`,
			expectSuccess: true,
		},
		{
			name: "string concatenation stress",
			script: `
chi_route("GET", "/strings", function(response, request)
    local big_string = ""
    for i = 1, 1000 do
        big_string = big_string .. "chunk_" .. i .. "_"
    end
    response:set_header("Content-Type", "text/plain")
    response:write("String test completed: " .. string.len(big_string) .. " chars")
end)
`,
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)

			err := env.Engine.ExecuteRouteScript("test-script", "test-tenant")
			
			if tc.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected error due to memory pressure, got success")
				}
			}
		})
	}
}

// Helper function for min operation
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}