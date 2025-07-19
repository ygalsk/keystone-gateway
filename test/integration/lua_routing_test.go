package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"keystone-gateway/internal/lua"
)

type testRequestSpec struct {
	method               string
	path                 string
	body                 string
	expectedStatus       int
	expectedBody         string
	expectedBodyContains []string
	expectedHeaders      map[string]string
}

func TestLuaRoutingIntegration(t *testing.T) {
	// Create temporary directory for test scripts
	tmpDir, err := os.MkdirTemp("", "lua-routing-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		script   string
		requests []testRequestSpec
	}{
		{
			name: "basic route registration",
			script: `
-- Basic route registration test
chi_route("GET", "/api/test", function(w, r)
	w:write("Hello from Lua!")
end)

chi_route("POST", "/api/data", function(w, r)
	local body = r:body()
	w:header("Content-Type", "application/json")
	w:write('{"received":"' .. body .. '"}')
end)
`,
			requests: []testRequestSpec{
				{
					method:          "GET",
					path:            "/api/test",
					expectedStatus:  200,
					expectedBody:    "Hello from Lua!",
					expectedHeaders: map[string]string{},
				},
				{
					method:          "POST",
					path:            "/api/data",
					body:            "test data",
					expectedStatus:  200,
					expectedBody:    `{"received":"test data"}`,
					expectedHeaders: map[string]string{"Content-Type": "application/json"},
				},
			},
		},
		{
			name: "middleware registration",
			script: `
-- Middleware test
chi_middleware("/*", function(next)
	return function(w, r)
		w:header("X-Custom-Header", "lua-middleware")
		next(w, r)
	end
end)

chi_route("GET", "/api/middleware-test", function(w, r)
	w:write("middleware applied")
end)
`,
			requests: []testRequestSpec{
				{
					method:          "GET",
					path:            "/api/middleware-test",
					expectedStatus:  200,
					expectedBody:    "middleware applied",
					expectedHeaders: map[string]string{"X-Custom-Header": "lua-middleware"},
				},
			},
		},
		{
			name: "route groups",
			script: `
-- Route group test
chi_group("/api/v1", function()
	chi_middleware(function(next)
		return function(w, r)
			w:header("X-API-Version", "v1")
			next(w, r)
		end
	end)
	
	chi_route("GET", "/users", function(w, r)
		w:header("Content-Type", "application/json")
		w:write('{"users":["alice","bob"]}')
	end)
	
	chi_route("GET", "/users/{id}", function(w, r)
		local id = r:param("id")
		w:header("Content-Type", "application/json")
		w:write('{"user":"' .. id .. '"}')
	end)
end)
`,
			requests: []testRequestSpec{
				{
					method:          "GET",
					path:            "/api/v1/users",
					expectedStatus:  200,
					expectedBody:    `{"users":["alice","bob"]}`,
					expectedHeaders: map[string]string{"Content-Type": "application/json", "X-API-Version": "v1"},
				},
				{
					method:          "GET",
					path:            "/api/v1/users/123",
					expectedStatus:  200,
					expectedBody:    `{"user":"123"}`,
					expectedHeaders: map[string]string{"Content-Type": "application/json", "X-API-Version": "v1"},
				},
			},
		},
		{
			name: "complex routing patterns",
			script: `
-- Complex routing patterns test
chi_route("GET", "/api/{version}/users/{id}", function(w, r)
	local version = r:param("version")
	local id = r:param("id")
	
	-- Version-based backend selection logic
	local backend_url
	if version == "v1" then
		backend_url = "backend-v1:8080"
	elseif version == "v2" then
		backend_url = "backend-v2:8080"
	else
		w:status(400)
		w:write("Unsupported API version")
		return
	end
	
	w:header("Content-Type", "application/json")
	w:header("X-Backend", backend_url)
	w:write('{"version":"' .. version .. '","user_id":"' .. id .. '","backend":"' .. backend_url .. '"}')
end)

-- Health check endpoint
chi_route("GET", "/health", function(w, r)
	w:header("Content-Type", "application/json")
	w:write('{"status":"ok","timestamp":"' .. tostring(os.time()) .. '"}')
end)
`,
			requests: []testRequestSpec{
				{
					method:         "GET",
					path:           "/api/v1/users/alice",
					expectedStatus: 200,
					expectedHeaders: map[string]string{
						"Content-Type": "application/json",
						"X-Backend":    "backend-v1:8080",
					},
					expectedBodyContains: []string{`"version":"v1"`, `"user_id":"alice"`, `"backend":"backend-v1:8080"`},
				},
				{
					method:         "GET",
					path:           "/api/v2/users/bob",
					expectedStatus: 200,
					expectedHeaders: map[string]string{
						"Content-Type": "application/json",
						"X-Backend":    "backend-v2:8080",
					},
					expectedBodyContains: []string{`"version":"v2"`, `"user_id":"bob"`, `"backend":"backend-v2:8080"`},
				},
				{
					method:         "GET",
					path:           "/api/v99/users/test",
					expectedStatus: 400,
					expectedBody:   "Unsupported API version",
				},
				{
					method:         "GET",
					path:           "/health",
					expectedStatus: 200,
					expectedHeaders: map[string]string{
						"Content-Type": "application/json",
					},
					expectedBodyContains: []string{`"status":"ok"`, `"timestamp"`},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create specific test directory for this test
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			require.NoError(t, err)

			// Write test script to scripts directory with scriptTag
			tenantName := "test-tenant"
			scriptPath := filepath.Join(testDir, tenantName+".lua")
			err = os.WriteFile(scriptPath, []byte(tt.script), 0644)
			require.NoError(t, err)

			// Create Chi router and Lua engine
			router := chi.NewRouter()
			engine := lua.NewEngine(testDir, router)

			// Execute Lua script using scriptTag and tenantName
			scriptTag := strings.TrimSuffix(filepath.Base(scriptPath), ".lua")
			err = engine.ExecuteRouteScript(scriptTag, tenantName)
			require.NoError(t, err, "Failed to execute Lua script")

			// Mount tenant routes at root
			err = engine.RouteRegistry().MountTenantRoutes(tenantName, "/")
			require.NoError(t, err, "Failed to mount tenant routes")

			// Test each request
			for i, req := range tt.requests {
				t.Run(fmt.Sprintf("request_%d", i), func(t *testing.T) {
					executeTestRequest(t, router, req)
				})
			}
		})
	}
}

func TestLuaErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		script      string
		expectError bool
	}{
		{
			name: "syntax error",
			script: `
chi_route("GET", "/test", function(w, r)
	-- Missing end
`,
			expectError: true,
		},
		{
			name: "runtime error in route handler",
			script: `
chi_route("GET", "/error", function(w, r)
	error("Intentional error")
end)
`,
			expectError: false, // Script executes, but route handler will error
		},
		{
			name: "valid script",
			script: `
chi_route("GET", "/valid", function(w, r)
	w:write("OK")
end)
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir, err := os.MkdirTemp("", "lua-error-test-")
			require.NoError(t, err)
			defer os.RemoveAll(testDir)

			tenantName := "test-tenant"
			scriptPath := filepath.Join(testDir, tenantName+".lua")
			err = os.WriteFile(scriptPath, []byte(tt.script), 0644)
			require.NoError(t, err)

			router := chi.NewRouter()
			engine := lua.NewEngine(testDir, router)

			scriptTag := strings.TrimSuffix(filepath.Base(scriptPath), ".lua")
			err = engine.ExecuteRouteScript(scriptTag, tenantName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLuaScriptTimeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lua-timeout-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create script with infinite loop
	script := `
-- Infinite loop to test timeout
while true do
	-- This should timeout
end
`
	tenantName := "timeout-tenant"
	scriptPath := filepath.Join(tmpDir, tenantName+".lua")
	err = os.WriteFile(scriptPath, []byte(script), 0644)
	require.NoError(t, err)

	router := chi.NewRouter()
	engine := lua.NewEngine(tmpDir, router)

	scriptTag := strings.TrimSuffix(filepath.Base(scriptPath), ".lua")
	start := time.Now()
	err = engine.ExecuteRouteScript(scriptTag, tenantName)
	duration := time.Since(start)

	// Should error due to timeout
	assert.Error(t, err)
	// Should not take much longer than MaxScriptExecutionTime
	assert.Less(t, duration, 7*time.Second) // Allow some buffer
}

func TestConcurrentLuaExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lua-concurrent-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test script
	script := `
chi_route("GET", "/concurrent/{id}", function(w, r)
	local id = r:param("id")
	w:write("Response for " .. id)
end)
`
	scriptPath := filepath.Join(tmpDir, "concurrent.lua")
	err = os.WriteFile(scriptPath, []byte(script), 0644)
	require.NoError(t, err)

	router := chi.NewRouter()
	engine := lua.NewEngine(tmpDir, router)

	scriptTag := strings.TrimSuffix(filepath.Base(scriptPath), ".lua")
	err = engine.ExecuteRouteScript(scriptTag, scriptTag)
	require.NoError(t, err)

	// Test concurrent requests
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer func() { done <- true }()

			req := testRequestSpec{
				method:         "GET",
				path:           fmt.Sprintf("/concurrent/%d", id),
				expectedStatus: 200,
				expectedBody:   fmt.Sprintf("Response for %d", id),
			}

			executeTestRequest(t, router, req)
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		select {
		case <-done:
			// Request completed
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
}

// Helper function to execute test requests
func executeTestRequest(t *testing.T, router http.Handler, req testRequestSpec) {
	var body io.Reader
	if req.body != "" {
		body = strings.NewReader(req.body)
	}

	httpReq, err := http.NewRequest(req.method, req.path, body)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httpReq)

	// Check status code
	assert.Equal(t, req.expectedStatus, rr.Code)

	// Check exact body match
	if req.expectedBody != "" {
		assert.Equal(t, req.expectedBody, rr.Body.String())
	}

	// Check body contains
	for _, expected := range req.expectedBodyContains {
		assert.Contains(t, rr.Body.String(), expected)
	}

	// Check headers
	for key, expected := range req.expectedHeaders {
		assert.Equal(t, expected, rr.Header().Get(key))
	}
}
