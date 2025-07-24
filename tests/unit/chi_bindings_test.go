package unit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"keystone-gateway/tests/fixtures"
)

// TestChiRouteBinding tests chi_route function for route registration
func TestChiRouteBinding(t *testing.T) {
	testCases := []struct {
		name           string
		script         string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "basic GET route registration",
			script: `
chi_route("GET", "/test", function(response, request)
    response:set_header("Content-Type", "text/plain")
    response:write("Hello from Lua GET")
end)
`,
			method:         "GET",
			path:           "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello from Lua GET",
		},
		{
			name: "POST route with JSON response",
			script: `
chi_route("POST", "/api/data", function(response, request)
    response:set_header("Content-Type", "application/json")
    response:write('{"status": "created", "method": "POST"}')
end)
`,
			method:         "POST",
			path:           "/api/data",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status": "created", "method": "POST"}`,
		},
		{
			name: "PUT route with parameter handling",
			script: `
chi_route("PUT", "/users/{id}", function(response, request)
    local user_id = chi_param(request, "id")
    response:set_header("Content-Type", "application/json")
    response:write('{"user_id": "' .. user_id .. '", "action": "updated"}')
end)
`,
			method:         "PUT",
			path:           "/users/123",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"user_id": "123", "action": "updated"}`,
		},
		{
			name: "DELETE route",
			script: `
chi_route("DELETE", "/resources/{id}", function(response, request)
    response:set_header("Content-Type", "text/plain")
    response:write("Resource deleted")
end)
`,
			method:         "DELETE",
			path:           "/resources/456",
			expectedStatus: http.StatusOK,
			expectedBody:   "Resource deleted",
		},
		{
			name: "PATCH route with custom headers",
			script: `
chi_route("PATCH", "/patch-test", function(response, request)
    response:set_header("X-Custom-Header", "patch-value")
    response:set_header("Content-Type", "text/plain")
    response:write("PATCH successful")
end)
`,
			method:         "PATCH",
			path:           "/patch-test",
			expectedStatus: http.StatusOK,
			expectedBody:   "PATCH successful",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)
			engine := env.Engine

			// Execute script to register routes
			err := engine.ExecuteRouteScript("test-script", "test-tenant")
			if err != nil {
				t.Fatalf("Failed to execute route script: %v", err)
			}

			// Mount the tenant routes on the main router
			err = env.MountTenantRoutesAtRoot("test-tenant")
			if err != nil {
				t.Fatalf("Failed to mount tenant routes: %v", err)
			}

			// Test the registered route using the router from the environment
			router := env.Router

			testCase := fixtures.HTTPTestCase{
				Name:           tc.name,
				Method:         tc.method,
				Path:           tc.path,
				ExpectedStatus: tc.expectedStatus,
				ExpectedBody:   tc.expectedBody,
			}

			fixtures.RunHTTPTestCases(t, router, []fixtures.HTTPTestCase{testCase})
		})
	}
}

// TestChiParameterExtraction tests parameter extraction from routes
func TestChiParameterExtraction(t *testing.T) {
	testCases := []struct {
		name           string
		script         string
		requestPath    string
		expectedParams map[string]string
	}{
		{
			name: "single parameter extraction",
			script: `
chi_route("GET", "/users/{id}", function(response, request)
    local user_id = chi_param(request, "id")
    response:set_header("Content-Type", "text/plain")
    response:write("User ID: " .. user_id)
end)
`,
			requestPath:    "/users/12345",
			expectedParams: map[string]string{"id": "12345"},
		},
		{
			name: "multiple parameter extraction",
			script: `
chi_route("GET", "/users/{user_id}/posts/{post_id}", function(response, request)
    local user_id = chi_param(request, "user_id")
    local post_id = chi_param(request, "post_id")
    response:set_header("Content-Type", "application/json")
    response:write('{"user_id": "' .. user_id .. '", "post_id": "' .. post_id .. '"}')
end)
`,
			requestPath:    "/users/alice/posts/789",
			expectedParams: map[string]string{"user_id": "alice", "post_id": "789"},
		},
		{
			name: "parameter with special characters",
			script: `
chi_route("GET", "/search/{query}", function(response, request)
    local query = chi_param(request, "query")
    response:set_header("Content-Type", "text/plain")
    response:write("Search query: " .. query)
end)
`,
			requestPath:    "/search/test-query-123",
			expectedParams: map[string]string{"query": "test-query-123"},
		},
		{
			name: "numeric parameter",
			script: `
chi_route("GET", "/items/{item_id}", function(response, request)
    local item_id = chi_param(request, "item_id")
    response:set_header("Content-Type", "text/plain")
    response:write("Item: " .. item_id)
end)
`,
			requestPath:    "/items/999",
			expectedParams: map[string]string{"item_id": "999"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)
			engine := env.Engine

			err := engine.ExecuteRouteScript("test-script", "test-tenant")
			if err != nil {
				t.Fatalf("Failed to execute route script: %v", err)
			}

			// Mount the tenant routes on the main router
			err = env.MountTenantRoutesAtRoot("test-tenant")
			if err != nil {
				t.Fatalf("Failed to mount tenant routes: %v", err)
			}

			router := env.Router

			req := httptest.NewRequest("GET", tc.requestPath, nil)
			resp := fixtures.ExecuteHTTPTestWithRequest(router, req)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			body := resp.Body

			// Verify each expected parameter appears in the response
			for paramName, expectedValue := range tc.expectedParams {
				if !strings.Contains(body, expectedValue) {
					t.Errorf("Expected parameter %s=%s to appear in response, got: %s", 
						paramName, expectedValue, body)
				}
			}
		})
	}
}

// TestChiMiddlewareBinding tests chi_middleware function for middleware registration
func TestChiMiddlewareBinding(t *testing.T) {
	testCases := []struct {
		name             string
		script           string
		requestPath      string
		expectedHeaders  map[string]string
		expectedStatus   int
		checkHeadersOnly bool
	}{
		{
			name: "basic header middleware",
			script: `
chi_middleware("/api/*", function(response, request, next)
    response:set_header("X-API-Version", "v1.0")
    response:set_header("X-Middleware", "active")
    next()
end)

chi_route("GET", "/api/test", function(response, request)
    response:set_header("Content-Type", "text/plain")
    response:write("API response")
end)
`,
			requestPath: "/api/test",
			expectedHeaders: map[string]string{
				"X-API-Version": "v1.0",
				"X-Middleware":  "active",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "authentication middleware",
			script: `
chi_middleware("/secure/*", function(response, request, next)
    response:set_header("X-Auth-Check", "passed")
    response:set_header("X-Security", "enabled")
    next()
end)

chi_route("GET", "/secure/data", function(response, request)
    response:set_header("Content-Type", "application/json")
    response:write('{"secure": "data"}')
end)
`,
			requestPath: "/secure/data",
			expectedHeaders: map[string]string{
				"X-Auth-Check": "passed",
				"X-Security":   "enabled",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "logging middleware",
			script: `
chi_middleware("/logs/*", function(response, request, next)
    response:set_header("X-Request-Logged", "true")
    response:set_header("X-Log-Level", "info")
    next()
end)

chi_route("POST", "/logs/event", function(response, request)
    response:set_header("Content-Type", "text/plain")
    response:write("Event logged")
end)
`,
			requestPath: "/logs/event",
			expectedHeaders: map[string]string{
				"X-Request-Logged": "true",
				"X-Log-Level":      "info",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "middleware without next call (blocking)",
			script: `
chi_middleware("/blocked/*", function(response, request, next)
    response:set_header("X-Blocked", "true")
    -- Not calling next() - should block the request
end)

chi_route("GET", "/blocked/test", function(response, request)
    response:write("This should not be reached")
end)
`,
			requestPath: "/blocked/test",
			expectedHeaders: map[string]string{
				"X-Blocked": "true",
			},
			expectedStatus:   http.StatusOK,
			checkHeadersOnly: true, // Don't check body since middleware blocks
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)
			engine := env.Engine

			err := engine.ExecuteRouteScript("test-script", "test-tenant")
			if err != nil {
				t.Fatalf("Failed to execute middleware script: %v", err)
			}

			// Mount the tenant routes on the main router
			err = env.MountTenantRoutesAtRoot("test-tenant")
			if err != nil {
				t.Fatalf("Failed to mount tenant routes: %v", err)
			}

			router := env.Router

			method := "GET"
			if strings.Contains(tc.script, `"POST"`) {
				method = "POST"
			}

			req := httptest.NewRequest(method, tc.requestPath, nil)
			resp := fixtures.ExecuteHTTPTestWithRequest(router, req)
			
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			// Check that middleware headers are present
			for headerName, expectedValue := range tc.expectedHeaders {
				actualValue := resp.Headers.Get(headerName)
				if actualValue != expectedValue {
					t.Errorf("Expected header %s=%s, got %s", headerName, expectedValue, actualValue)
				}
			}

			if !tc.checkHeadersOnly {
				body := resp.Body
				if body == "" {
					t.Error("Expected response body, got empty")
				}
			}
		})
	}
}

// TestChiRouteGroups tests chi_group function for route group registration
func TestChiRouteGroups(t *testing.T) {
	testCases := []struct {
		name           string
		script         string
		requestPath    string
		expectedStatus int
		expectedBody   string
		expectedHeader string
	}{
		{
			name: "basic route group",
			script: `
chi_group("/v1", function()
    chi_route("GET", "/users", function(response, request)
        response:set_header("Content-Type", "application/json")
        response:write('{"version": "v1", "endpoint": "users"}')
    end)
    
    chi_route("GET", "/posts", function(response, request)
        response:set_header("Content-Type", "application/json")  
        response:write('{"version": "v1", "endpoint": "posts"}')
    end)
end)
`,
			requestPath:    "/v1/users",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"version": "v1", "endpoint": "users"}`,
		},
		{
			name: "nested route group with middleware",
			script: `
chi_group("/api", function()
    chi_group("/v2", function()
        chi_middleware("/*", function(response, request, next)
            response:set_header("X-API-Version", "v2")
            next()
        end)
        
        chi_route("GET", "/data", function(response, request)
            response:set_header("Content-Type", "text/plain")
            response:write("API v2 data")
        end)
    end)
end)
`,
			requestPath:    "/api/v2/data",
			expectedStatus: http.StatusOK,
			expectedBody:   "API v2 data",
			expectedHeader: "X-API-Version",
		},
		{
			name: "group with parameters",
			script: `
chi_group("/users", function()
    chi_route("GET", "/{id}", function(response, request)
        local user_id = chi_param(request, "id")
        response:set_header("Content-Type", "application/json")
        response:write('{"user_id": "' .. user_id .. '", "group": "users"}')
    end)
    
    chi_route("GET", "/{id}/profile", function(response, request)
        local user_id = chi_param(request, "id")
        response:set_header("Content-Type", "application/json")
        response:write('{"user_id": "' .. user_id .. '", "endpoint": "profile"}')
    end)
end)
`,
			requestPath:    "/users/alice/profile",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"user_id": "alice", "endpoint": "profile"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)
			engine := env.Engine

			err := engine.ExecuteRouteScript("test-script", "test-tenant")
			if err != nil {
				t.Fatalf("Failed to execute group script: %v", err)
			}

			// Mount the tenant routes on the main router
			err = env.MountTenantRoutesAtRoot("test-tenant")
			if err != nil {
				t.Fatalf("Failed to mount tenant routes: %v", err)
			}

			router := env.Router

			req := httptest.NewRequest("GET", tc.requestPath, nil)
			resp := fixtures.ExecuteHTTPTestWithRequest(router, req)
			
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if resp.Body != tc.expectedBody {
				t.Errorf("Expected body %q, got %q", tc.expectedBody, resp.Body)
			}

			if tc.expectedHeader != "" {
				if resp.Headers.Get(tc.expectedHeader) == "" {
					t.Errorf("Expected header %s to be present", tc.expectedHeader)
				}
			}
		})
	}
}

// TestChiBindingsErrorHandling tests error handling in Chi bindings
func TestChiBindingsErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		script         string
		expectError    bool
		errorContains  string
	}{
		{
			name: "invalid route method",
			script: `
chi_route("INVALID", "/test", function(response, request)
    response:write("test")
end)
`,
			expectError:   false, // Chi accepts any method
		},
		{
			name: "missing route handler",
			script: `
chi_route("GET", "/test")
`,
			expectError:   true,
			errorContains: "chi_route requires method, pattern, and handler function",
		},
		{
			name: "empty route pattern",
			script: `
chi_route("GET", "", function(response, request)
    response:write("test")
end)
`,
			expectError:   true,
			errorContains: "chi_route requires method, pattern, and handler function",
		},
		{
			name: "missing middleware handler",
			script: `
chi_middleware("/test/*")
`,
			expectError:   true,
			errorContains: "chi_middleware requires pattern and middleware function",
		},
		{
			name: "empty middleware pattern",
			script: `
chi_middleware("", function(response, request, next)
    next()
end)
`,
			expectError:   true,
			errorContains: "chi_middleware requires pattern and middleware function",
		},
		{
			name: "missing group setup function",
			script: `
chi_group("/api")
`,
			expectError:   true,
			errorContains: "chi_group requires pattern and setup function",
		},
		{
			name: "runtime error in route handler",
			script: `
chi_route("GET", "/error", function(response, request)
    error("Intentional runtime error")
end)
`,
			expectError: false, // Route registration should succeed, error happens at runtime
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.script)
			engine := env.Engine

			err := engine.ExecuteRouteScript("test-script", "test-tenant")

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				} else {
					// Mount the tenant routes on the main router if script execution succeeded
					err = env.MountTenantRoutesAtRoot("test-tenant")
					if err != nil {
						t.Fatalf("Failed to mount tenant routes: %v", err)
					}
				}
			}
		})
	}
}

// TestChiBindingsIntegration tests integration between different Chi binding functions
func TestChiBindingsIntegration(t *testing.T) {
	script := `
-- Global middleware for all API routes
chi_middleware("/api/*", function(response, request, next)
    response:set_header("X-Global-Middleware", "active")
    next()
end)

-- API v1 group with its own middleware
chi_group("/api/v1", function()
    chi_middleware("/*", function(response, request, next)
        response:set_header("X-V1-Middleware", "active")
        next()
    end)
    
    chi_route("GET", "/users", function(response, request)
        response:set_header("Content-Type", "application/json")
        response:write('{"endpoint": "users", "version": "v1"}')
    end)
    
    chi_route("GET", "/users/{id}", function(response, request)
        local user_id = chi_param(request, "id")
        response:set_header("Content-Type", "application/json")
        response:write('{"user_id": "' .. user_id .. '", "version": "v1"}')
    end)
end)

-- API v2 group with different middleware
chi_group("/api/v2", function()
    chi_middleware("/*", function(response, request, next)
        response:set_header("X-V2-Middleware", "active")
        response:set_header("X-Enhanced", "true")
        next()
    end)
    
    chi_route("GET", "/users", function(response, request)
        response:set_header("Content-Type", "application/json")
        response:write('{"endpoint": "users", "version": "v2", "enhanced": true}')
    end)
end)

-- Standalone route outside groups
chi_route("GET", "/health", function(response, request)
    response:set_header("Content-Type", "text/plain")
    response:write("OK")
end)
`

	env := fixtures.SetupLuaEngineWithScript(t, script)
	engine := env.Engine

	err := engine.ExecuteRouteScript("test-script", "test-tenant")
	if err != nil {
		t.Fatalf("Failed to execute integration script: %v", err)
	}

	// Mount the tenant routes on the main router
	err = env.MountTenantRoutesAtRoot("test-tenant")
	if err != nil {
		t.Fatalf("Failed to mount tenant routes: %v", err)
	}

	router := env.Router

	testCases := []fixtures.HTTPTestCase{
		{
			Name:           "v1 users endpoint with middleware chain",
			Method:         "GET",
			Path:           "/api/v1/users",
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"endpoint": "users", "version": "v1"}`,
			CheckHeaders: map[string]string{
				"X-Global-Middleware": "active",
				"X-V1-Middleware":     "active",
			},
		},
		{
			Name:           "v1 user by ID with parameters",
			Method:         "GET",
			Path:           "/api/v1/users/12345",
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"user_id": "12345", "version": "v1"}`,
			CheckHeaders: map[string]string{
				"X-Global-Middleware": "active",
				"X-V1-Middleware":     "active",
			},
		},
		{
			Name:           "v2 users endpoint with different middleware",
			Method:         "GET",
			Path:           "/api/v2/users",
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"endpoint": "users", "version": "v2", "enhanced": true}`,
			CheckHeaders: map[string]string{
				"X-Global-Middleware": "active",
				"X-V2-Middleware":     "active",
				"X-Enhanced":          "true",
			},
		},
		{
			Name:           "standalone health endpoint",
			Method:         "GET",
			Path:           "/health",
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   "OK",
		},
	}

	fixtures.RunHTTPTestCases(t, router, testCases)
}