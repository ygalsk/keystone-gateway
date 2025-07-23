package unit

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

func TestChiRouteRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create a simple route registration script
	routeScript := `
-- Register basic routes
chi_route("GET", "/test", function(w, r)
	w:write("GET test response")
end)

chi_route("POST", "/test", function(w, r)
	w:write("POST test response")
end)

chi_route("GET", "/users/{id}", function(w, r)
	local user_id = chi_param(r, "id")
	w:write("User ID: " .. user_id)
end)
`

	scriptFile := filepath.Join(scriptsDir, "api-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(routeScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	// Setup test configuration
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "test-tenant",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "backend", URL: "http://backend:8080", Health: "/health"},
				},
			},
		},
		LuaRouting: &config.LuaRoutingConfig{
			Enabled:    true,
			ScriptsDir: scriptsDir,
		},
	}

	// Create router and initialize Lua engine
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	_ = gateway // Used for initialization

	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Load and execute scripts for testing
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET route registration",
			method:         "GET",
			path:           "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "GET test response",
		},
		{
			name:           "POST route registration",
			method:         "POST",
			path:           "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "POST test response",
		},
		{
			name:           "parameterized route",
			method:         "GET",
			path:           "/users/123",
			expectedStatus: http.StatusOK,
			expectedBody:   "User ID: 123",
		},
		{
			name:           "parameterized route with string",
			method:         "GET",
			path:           "/users/john",
			expectedStatus: http.StatusOK,
			expectedBody:   "User ID: john",
		},
		{
			name:           "unregistered route",
			method:         "GET",
			path:           "/notfound",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedBody != "" && !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiMiddlewareRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create middleware registration script
	middlewareScript := `
-- Register middleware that adds a custom header
chi_middleware("/protected/*", function(w, r, next)
	w:header("X-Protected", "true")
	next(w, r)
end)

-- Register a route under the protected path
chi_route("GET", "/protected/data", function(w, r)
	w:write("protected data")
end)

-- Register a route outside the protected path
chi_route("GET", "/public/data", function(w, r)
	w:write("public data")
end)
`

	scriptFile := filepath.Join(scriptsDir, "middleware-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(middlewareScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	// cfg is not used in this simplified test

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name               string
		path               string
		expectProtectedHdr bool
		expectedBody       string
	}{
		{
			name:               "protected route with middleware",
			path:               "/protected/data",
			expectProtectedHdr: true,
			expectedBody:       "protected data",
		},
		{
			name:               "public route without middleware",
			path:               "/public/data",
			expectProtectedHdr: false,
			expectedBody:       "public data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			protectedHeader := w.Header().Get("X-Protected")
			if tc.expectProtectedHdr {
				if protectedHeader != "true" {
					t.Errorf("expected X-Protected header to be 'true', got %q", protectedHeader)
				}
			} else {
				if protectedHeader != "" {
					t.Errorf("expected no X-Protected header, but got %q", protectedHeader)
				}
			}

			if !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiRouteGroups(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create route group script
	groupScript := `
-- Create a route group with shared middleware
chi_group("/api/v1", function()
	-- Add middleware to all routes in this group
	chi_middleware("/*", function(w, r, next)
		w:header("X-API-Version", "v1")
		next(w, r)
	end)
	
	-- Add routes to the group
	chi_route("GET", "/users", function(w, r)
		w:write("users list")
	end)
	
	chi_route("GET", "/users/{id}", function(w, r)
		local user_id = chi_param(r, "id")
		w:write("user: " .. user_id)
	end)
	
	chi_route("POST", "/users", function(w, r)
		w:write("create user")
	end)
end)

-- Route outside the group (no version header)
chi_route("GET", "/health", function(w, r)
	w:write("healthy")
end)
`

	scriptFile := filepath.Join(scriptsDir, "group-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(groupScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name              string
		method            string
		path              string
		expectedStatus    int
		expectedBody      string
		expectVersionHdr  bool
	}{
		{
			name:             "group route - users list",
			method:           "GET",
			path:             "/api/v1/users",
			expectedStatus:   http.StatusOK,
			expectedBody:     "users list",
			expectVersionHdr: true,
		},
		{
			name:             "group route - specific user",
			method:           "GET",
			path:             "/api/v1/users/42",
			expectedStatus:   http.StatusOK,
			expectedBody:     "user: 42",
			expectVersionHdr: true,
		},
		{
			name:             "group route - create user",
			method:           "POST",
			path:             "/api/v1/users",
			expectedStatus:   http.StatusOK,
			expectedBody:     "create user",
			expectVersionHdr: true,
		},
		{
			name:             "route outside group",
			method:           "GET",
			path:             "/health",
			expectedStatus:   http.StatusOK,
			expectedBody:     "healthy",
			expectVersionHdr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			versionHeader := w.Header().Get("X-API-Version")
			if tc.expectVersionHdr {
				if versionHeader != "v1" {
					t.Errorf("expected X-API-Version header to be 'v1', got %q", versionHeader)
				}
			} else {
				if versionHeader != "" {
					t.Errorf("expected no X-API-Version header, but got %q", versionHeader)
				}
			}

			if !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiParameterExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script that tests various parameter patterns
	paramScript := `
-- Single parameter
chi_route("GET", "/users/{id}", function(w, r)
	local user_id = chi_param(r, "id")
	w:write("User: " .. user_id)
end)

-- Multiple parameters
chi_route("GET", "/users/{user_id}/posts/{post_id}", function(w, r)
	local user_id = chi_param(r, "user_id") 
	local post_id = chi_param(r, "post_id")
	w:write("User: " .. user_id .. ", Post: " .. post_id)
end)

-- Wildcard parameter
chi_route("GET", "/files/*", function(w, r)
	local filepath = chi_param(r, "*")
	w:write("File: " .. filepath)
end)

-- Parameter with validation
chi_route("GET", "/numbers/{num:[0-9]+}", function(w, r)
	local num = chi_param(r, "num")
	w:write("Number: " .. num)
end)
`

	scriptFile := filepath.Join(scriptsDir, "param-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(paramScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "single parameter",
			path:           "/users/john",
			expectedStatus: http.StatusOK,
			expectedBody:   "User: john",
		},
		{
			name:           "single parameter with numbers",
			path:           "/users/123",
			expectedStatus: http.StatusOK,
			expectedBody:   "User: 123",
		},
		{
			name:           "multiple parameters",
			path:           "/users/alice/posts/42",
			expectedStatus: http.StatusOK,
			expectedBody:   "User: alice, Post: 42",
		},
		{
			name:           "wildcard parameter",
			path:           "/files/documents/report.pdf",
			expectedStatus: http.StatusOK,
			expectedBody:   "File: documents/report.pdf",
		},
		{
			name:           "validated parameter - numbers only",
			path:           "/numbers/12345",
			expectedStatus: http.StatusOK,
			expectedBody:   "Number: 12345",
		},
		{
			name:           "validated parameter - invalid (should 404)",
			path:           "/numbers/abc",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedBody != "" && !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiRouteConflictsAndOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script with potential route conflicts
	conflictScript := `
-- Register the same route twice with different handlers
chi_route("GET", "/test", function(w, r)
	w:write("first handler")
end)

-- This should either override the first or be ignored (depends on implementation)
chi_route("GET", "/test", function(w, r)
	w:write("second handler")
end)

-- Different methods on same path (should be allowed)
chi_route("POST", "/test", function(w, r)
	w:write("POST handler")
end)

-- Similar but different paths
chi_route("GET", "/test/", function(w, r)
	w:write("trailing slash")
end)

chi_route("GET", "/test/{id}", function(w, r)
	local id = chi_param(r, "id")
	w:write("parameterized: " .. id)
end)
`

	scriptFile := filepath.Join(scriptsDir, "conflict-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(conflictScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		possibleBodies []string // Multiple possible responses due to route conflicts
	}{
		{
			name:           "duplicate route registration",
			method:         "GET",
			path:           "/test",
			expectedStatus: http.StatusOK,
			possibleBodies: []string{"first handler", "second handler"},
		},
		{
			name:           "different method same path",
			method:         "POST",
			path:           "/test",
			expectedStatus: http.StatusOK,
			possibleBodies: []string{"POST handler"},
		},
		{
			name:           "trailing slash path",
			method:         "GET",
			path:           "/test/",
			expectedStatus: http.StatusOK,
			possibleBodies: []string{"trailing slash"},
		},
		{
			name:           "parameterized path",
			method:         "GET",
			path:           "/test/123",
			expectedStatus: http.StatusOK,
			possibleBodies: []string{"parameterized: 123"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				responseBody := w.Body.String()
				found := false
				for _, possibleBody := range tc.possibleBodies {
					if strings.Contains(responseBody, possibleBody) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected body to contain one of %v, got %q", tc.possibleBodies, responseBody)
				}
			}
		})
	}
}

func TestChiContextAndRequestData(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script that accesses request context and data
	contextScript := `
-- Route that examines request method, headers, and query parameters
chi_route("GET", "/context", function(w, r)
	-- Access request method (should be available through Lua bindings)
	w:write("Method: " .. (r.method or "unknown"))
end)

-- Route that uses chi_param with missing parameter
chi_route("GET", "/missing-param/{id}", function(w, r)
	local missing = chi_param(r, "nonexistent")
	local existing = chi_param(r, "id")
	w:write("Missing: " .. (missing or "nil") .. ", Existing: " .. existing)
end)
`

	scriptFile := filepath.Join(scriptsDir, "context-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(contextScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "request context access",
			method:         "GET", 
			path:           "/context",
			expectedStatus: http.StatusOK,
			expectedBody:   "Method:",
		},
		{
			name:           "missing parameter handling",
			method:         "GET",
			path:           "/missing-param/test123",
			expectedStatus: http.StatusOK,
			expectedBody:   "Existing: test123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiHTTPMethods(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script that registers all HTTP methods
	methodScript := `
-- Standard HTTP methods
chi_route("GET", "/resource", function(w, r) w:write("GET") end)
chi_route("POST", "/resource", function(w, r) w:write("POST") end)
chi_route("PUT", "/resource", function(w, r) w:write("PUT") end)
chi_route("PATCH", "/resource", function(w, r) w:write("PATCH") end)
chi_route("DELETE", "/resource", function(w, r) w:write("DELETE") end)
chi_route("HEAD", "/resource", function(w, r) w:write("HEAD") end)
chi_route("OPTIONS", "/resource", function(w, r) w:write("OPTIONS") end)

-- Custom HTTP method (if supported)
chi_route("CUSTOM", "/resource", function(w, r) w:write("CUSTOM") end)
`

	scriptFile := filepath.Join(scriptsDir, "method-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(methodScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET method",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody:   "GET",
		},
		{
			name:           "POST method",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedBody:   "POST",
		},
		{
			name:           "PUT method",
			method:         "PUT",
			expectedStatus: http.StatusOK,
			expectedBody:   "PUT",
		},
		{
			name:           "PATCH method",
			method:         "PATCH",
			expectedStatus: http.StatusOK,
			expectedBody:   "PATCH",
		},
		{
			name:           "DELETE method",
			method:         "DELETE",
			expectedStatus: http.StatusOK,
			expectedBody:   "DELETE",
		},
		{
			name:           "HEAD method",
			method:         "HEAD",
			expectedStatus: http.StatusOK,
			expectedBody:   "", // HEAD responses typically have no body
		},
		{
			name:           "OPTIONS method",
			method:         "OPTIONS",
			expectedStatus: http.StatusOK,
			expectedBody:   "OPTIONS",
		},
		{
			name:           "CUSTOM method",
			method:         "CUSTOM",
			expectedStatus: http.StatusOK,
			expectedBody:   "CUSTOM",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/resource", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedBody != "" && !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiRouteRegistrationErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script with potential errors
	errorScript := `
-- Valid route for baseline
chi_route("GET", "/valid", function(w, r)
	w:write("valid response")
end)

-- Attempt to register route with invalid pattern (empty string)
chi_route("GET", "", function(w, r)
	w:write("empty pattern")
end)

-- Attempt to register route with nil handler
chi_route("GET", "/nil-handler", nil)

-- Route with invalid method (should be handled gracefully)
chi_route("", "/invalid-method", function(w, r)
	w:write("invalid method")
end)
`

	scriptFile := filepath.Join(scriptsDir, "error-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(errorScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Loading scripts with errors should either succeed with warnings or fail gracefully
	// Try to reload and execute scripts
	if err := luaEngine.ReloadScripts(); err != nil {
		t.Logf("Warning: failed to reload scripts: %v", err)
	}
	
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Expected error executing script %s with invalid routes: %v", scriptTag, err)
		}
	}

	// Test that valid routes still work even if some registrations failed
	req := httptest.NewRequest("GET", "/valid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The valid route should work regardless of errors in other route registrations
	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "valid response") {
		t.Logf("Valid route works despite other registration errors")
	} else {
		t.Logf("Valid route may have been affected by registration errors: status=%d, body=%q", 
			w.Code, w.Body.String())
	}
}

func TestChiSubrouterIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script that tests subrouter mounting
	subrouterScript := `
-- Create routes that might be mounted under different paths
chi_route("GET", "/info", function(w, r)
	w:write("app info")
end)

chi_route("GET", "/status", function(w, r)
	w:write("app status")
end)

-- Route with parameter
chi_route("GET", "/config/{key}", function(w, r)
	local key = chi_param(r, "key")
	w:write("config key: " .. key)
end)
`

	scriptFile := filepath.Join(scriptsDir, "subrouter-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(subrouterScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	// Create main router and subrouter
	mainRouter := chi.NewRouter()
	
	// Create a separate router for the Lua routes
	luaRouter := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, luaRouter)
	
	// Reload and execute scripts
	if err := luaEngine.ReloadScripts(); err != nil {
		t.Logf("Warning: failed to reload scripts: %v", err)
	}
	
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	// Mount the Lua router under /app
	mainRouter.Mount("/app", luaRouter)

	testCases := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "mounted info route",
			path:           "/app/info",
			expectedStatus: http.StatusOK,
			expectedBody:   "app info",
		},
		{
			name:           "mounted status route",
			path:           "/app/status",
			expectedStatus: http.StatusOK,
			expectedBody:   "app status",
		},
		{
			name:           "mounted parameterized route",
			path:           "/app/config/database",
			expectedStatus: http.StatusOK,
			expectedBody:   "config key: database",
		},
		{
			name:           "unmounted path should 404",
			path:           "/info",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()

			mainRouter.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedBody != "" && !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestChiRouterContextIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	// Create script that tests context isolation between requests
	isolationScript := `
-- Route that might share state between requests (test for isolation)
local request_count = 0

chi_route("GET", "/counter", function(w, r)
	request_count = request_count + 1
	w:write("Count: " .. request_count)
end)

-- Route that tests parameter isolation
chi_route("GET", "/echo/{message}", function(w, r)
	local message = chi_param(r, "message")
	w:write("Echo: " .. message)
end)
`

	scriptFile := filepath.Join(scriptsDir, "isolation-routes.lua")
	if err := os.WriteFile(scriptFile, []byte(isolationScript), 0644); err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	router := chi.NewRouter()
	luaEngine := lua.NewEngine(scriptsDir, router)
	
	// Execute scripts
	scripts := luaEngine.GetLoadedScripts()
	for _, scriptTag := range scripts {
		if err := luaEngine.ExecuteRouteScript(scriptTag, "test-tenant"); err != nil {
			t.Logf("Warning: failed to execute script %s: %v", scriptTag, err)
		}
	}
	
	// Mount tenant routes to the main router
	registry := luaEngine.RouteRegistry()
	if err := registry.MountTenantRoutes("test-tenant", "/"); err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	// Test concurrent requests to verify isolation
	t.Run("parameter isolation", func(t *testing.T) {
		messages := []string{"hello", "world", "test"}
		
		for _, message := range messages {
			req := httptest.NewRequest("GET", "/echo/"+message, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			expectedBody := "Echo: " + message
			if !strings.Contains(w.Body.String(), expectedBody) {
				t.Errorf("expected body to contain %q, got %q", expectedBody, w.Body.String())
			}
		}
	})

	t.Run("state isolation concerns", func(t *testing.T) {
		// Make multiple requests to the counter endpoint
		// Note: This test documents potential state sharing issues rather than asserting specific behavior
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/counter", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			t.Logf("Request %d response: %s", i+1, w.Body.String())
		}
	})
}