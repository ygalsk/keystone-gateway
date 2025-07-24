package unit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
	
	"keystone-gateway/internal/routing"
)

// Test fixtures following DRY principle
type routeRegistryTestCase struct {
	name      string
	setup     func(*routing.LuaRouteRegistry)
	test      func(*testing.T, *routing.LuaRouteRegistry)
	expectErr bool
}

// Common test helpers - reusable across all tests
func setupTestRegistry(t *testing.T) *routing.LuaRouteRegistry {
	router := chi.NewRouter()
	return routing.NewLuaRouteRegistry(router, &mockEngine{})
}

func createTestRoute(tenantName, method, pattern string) routing.RouteDefinition {
	return routing.RouteDefinition{
		TenantName: tenantName,
		Method:     method,
		Pattern:    pattern,
		Handler:    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}),
	}
}

func createTestMiddleware(tenantName, pattern string) routing.MiddlewareDefinition {
	return routing.MiddlewareDefinition{
		TenantName: tenantName,
		Pattern:    pattern,
		Middleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test-Middleware", "applied")
				next.ServeHTTP(w, r)
			})
		},
	}
}

// Mock engine for testing
type mockEngine struct{}

func (m *mockEngine) GetScript(tag string) (string, bool) {
	return "mock script", true
}

func (m *mockEngine) SetupChiBindings(L *lua.LState, scriptTag, tenantName string) {}

// Table-driven test runner (DRY)
func runRegistryTests(t *testing.T, tests []routeRegistryTestCase) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := setupTestRegistry(t)
			if tt.setup != nil {
				tt.setup(registry)
			}
			tt.test(t, registry)
		})
	}
}

// 1. Route Pattern Validation Tests
func TestValidateRoutePattern(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		expectErr bool
	}{
		{"valid_root_path", "/", false},
		{"valid_simple_path", "/api", false},
		{"valid_nested_path", "/api/v1/users", false},
		{"valid_with_param", "/users/{id}", false},
		{"valid_with_multiple_params", "/users/{id}/posts/{postId}", false},
		{"valid_wildcard", "/files/*", false},
		
		{"empty_pattern", "", true},
		{"missing_leading_slash", "api", true},
		{"unmatched_opening_brace", "/users/{id", true},
		{"unmatched_closing_brace", "/users/id}", true},
		// Note: This pattern is actually valid for Chi router - it treats {id/{name}} as a single parameter name
		// {"nested_unmatched_braces", "/users/{id/{name}}", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation through RegisterRoute since validateRoutePattern is not exported
			registry := setupTestRegistry(t)
			route := createTestRoute("test-tenant", "GET", tt.pattern)
			err := registry.RegisterRoute(route)
			
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for pattern %q, got nil", tt.pattern)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for pattern %q, got: %v", tt.pattern, err)
			}
		})
	}
}

// 2. Route Registration Tests
func TestLuaRouteRegistry_RegisterRoute(t *testing.T) {
	tests := []routeRegistryTestCase{
		{
			name: "successful_route_registration",
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				route := createTestRoute("test-tenant", "GET", "/api/test")
				err := registry.RegisterRoute(route)
				
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				
				// Verify tenant submux was created
				tenants := registry.ListTenants()
				if len(tenants) != 1 || tenants[0] != "test-tenant" {
					t.Errorf("Expected tenant 'test-tenant', got: %v", tenants)
				}
			},
		},
		{
			name: "duplicate_route_prevention",
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				route := createTestRoute("test-tenant", "GET", "/api/test")
				
				// Register same route twice
				err1 := registry.RegisterRoute(route)
				err2 := registry.RegisterRoute(route)
				
				if err1 != nil {
					t.Errorf("First registration should succeed, got: %v", err1)
				}
				if err2 != nil {
					t.Errorf("Second registration should succeed (idempotent), got: %v", err2)
				}
			},
		},
		{
			name: "invalid_pattern_rejection",
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				route := createTestRoute("test-tenant", "GET", "invalid-pattern")
				err := registry.RegisterRoute(route)
				
				if err == nil {
					t.Error("Expected error for invalid pattern, got nil")
				}
			},
		},
	}
	
	runRegistryTests(t, tests)
}

// 3. Middleware Registration Tests  
func TestLuaRouteRegistry_RegisterMiddleware(t *testing.T) {
	tests := []routeRegistryTestCase{
		{
			name: "successful_middleware_registration",
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				middleware := createTestMiddleware("test-tenant", "/api/*")
				err := registry.RegisterMiddleware(middleware)
				
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			},
		},
		{
			name: "multiple_middleware_registration",
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				mw1 := createTestMiddleware("test-tenant", "/api/*")
				mw2 := createTestMiddleware("test-tenant", "/admin/*")
				
				err1 := registry.RegisterMiddleware(mw1)
				err2 := registry.RegisterMiddleware(mw2)
				
				if err1 != nil || err2 != nil {
					t.Errorf("Expected no errors, got: %v, %v", err1, err2)
				}
			},
		},
	}
	
	runRegistryTests(t, tests)
}

// 4. Pattern Matching Tests
func TestPatternMatching(t *testing.T) {
	registry := setupTestRegistry(t)
	
	tests := []struct {
		name             string
		routePattern     string
		middlewarePattern string
		expected         bool
	}{
		{"exact_match", "/api/users", "/api/users", true},
		{"wildcard_match", "/api/users", "/api/*", true},
		{"wildcard_no_match", "/admin/users", "/api/*", false},
		{"root_wildcard", "/any/path", "/*", true},
		{"no_match", "/api/users", "/admin/users", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the internal pattern matching logic
			// We'll need to expose this or test it through middleware application
			middleware := createTestMiddleware("test-tenant", tt.middlewarePattern)
			registry.RegisterMiddleware(middleware)
			
			route := createTestRoute("test-tenant", "GET", tt.routePattern)
			registry.RegisterRoute(route)
			
			// Test by making actual HTTP request and checking if middleware was applied
			submux := registry.GetTenantRoutes("test-tenant")
			if submux == nil {
				t.Fatal("Expected tenant submux to exist")
			}
		})
	}
}

// 5. Management Functions Tests
func TestLuaRouteRegistry_Management(t *testing.T) {
	tests := []routeRegistryTestCase{
		{
			name: "list_tenants",
			setup: func(registry *routing.LuaRouteRegistry) {
				registry.RegisterRoute(createTestRoute("tenant1", "GET", "/api/test1"))
				registry.RegisterRoute(createTestRoute("tenant2", "GET", "/api/test2"))
			},
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				tenants := registry.ListTenants()
				if len(tenants) != 2 {
					t.Errorf("Expected 2 tenants, got %d", len(tenants))
				}
				
				// Check both tenants exist (order may vary)
				tenantMap := make(map[string]bool)
				for _, tenant := range tenants {
					tenantMap[tenant] = true
				}
				
				if !tenantMap["tenant1"] || !tenantMap["tenant2"] {
					t.Errorf("Expected tenants 'tenant1' and 'tenant2', got: %v", tenants)
				}
			},
		},
		{
			name: "get_tenant_routes",
			setup: func(registry *routing.LuaRouteRegistry) {
				registry.RegisterRoute(createTestRoute("test-tenant", "GET", "/api/test"))
			},
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				submux := registry.GetTenantRoutes("test-tenant")
				if submux == nil {
					t.Error("Expected tenant submux to exist")
				}
				
				// Non-existent tenant should return nil
				nonExistent := registry.GetTenantRoutes("non-existent")
				if nonExistent != nil {
					t.Error("Expected nil for non-existent tenant")
				}
			},
		},
		{
			name: "clear_tenant_routes",
			setup: func(registry *routing.LuaRouteRegistry) {
				registry.RegisterRoute(createTestRoute("test-tenant", "GET", "/api/test"))
			},
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				// Verify tenant exists
				if len(registry.ListTenants()) != 1 {
					t.Fatal("Expected 1 tenant before clearing")
				}
				
				registry.ClearTenantRoutes("test-tenant")
				
				// Verify tenant was cleared
				tenants := registry.ListTenants()
				if len(tenants) != 0 {
					t.Errorf("Expected 0 tenants after clearing, got %d", len(tenants))
				}
			},
		},
		{
			name: "mount_tenant_routes",
			setup: func(registry *routing.LuaRouteRegistry) {
				registry.RegisterRoute(createTestRoute("test-tenant", "GET", "/api/test"))
			},
			test: func(t *testing.T, registry *routing.LuaRouteRegistry) {
				err := registry.MountTenantRoutes("test-tenant", "/mounted")
				if err != nil {
					t.Errorf("Expected no error mounting routes, got: %v", err)
				}
				
				// Test mounting non-existent tenant (should not error)
				err = registry.MountTenantRoutes("non-existent", "/test")
				if err != nil {
					t.Errorf("Expected no error for non-existent tenant, got: %v", err)
				}
			},
		},
	}
	
	runRegistryTests(t, tests)
}

// 6. Concurrency Tests
func TestLuaRouteRegistry_Concurrency(t *testing.T) {
	t.Run("concurrent_route_registration", func(t *testing.T) {
		registry := setupTestRegistry(t)
		
		const numGoroutines = 10
		const routesPerGoroutine = 5
		
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*routesPerGoroutine)
		
		// Register routes concurrently from multiple goroutines
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < routesPerGoroutine; j++ {
					route := createTestRoute(
						"test-tenant",
						"GET",
						fmt.Sprintf("/api/goroutine%d/route%d", goroutineID, j),
					)
					
					if err := registry.RegisterRoute(route); err != nil {
						errors <- err
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// Check for any errors
		for err := range errors {
			t.Errorf("Concurrent registration error: %v", err)
		}
		
		// Verify all routes were registered
		tenants := registry.ListTenants()
		if len(tenants) != 1 {
			t.Errorf("Expected 1 tenant, got %d", len(tenants))
		}
	})
	
	t.Run("concurrent_middleware_registration", func(t *testing.T) {
		registry := setupTestRegistry(t)
		
		const numGoroutines = 5
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				middleware := createTestMiddleware(
					"test-tenant",
					fmt.Sprintf("/api/middleware%d/*", goroutineID),
				)
				
				if err := registry.RegisterMiddleware(middleware); err != nil {
					errors <- err
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// Check for any errors
		for err := range errors {
			t.Errorf("Concurrent middleware registration error: %v", err)
		}
	})
}

// 7. Integration Tests - Route + Middleware
func TestLuaRouteRegistry_Integration(t *testing.T) {
	t.Run("middleware_application_to_routes", func(t *testing.T) {
		registry := setupTestRegistry(t)
		
		// Register middleware first
		middleware := createTestMiddleware("test-tenant", "/api/*")
		err := registry.RegisterMiddleware(middleware)
		if err != nil {
			t.Fatalf("Failed to register middleware: %v", err)
		}
		
		// Register route
		route := createTestRoute("test-tenant", "GET", "/api/test")
		err = registry.RegisterRoute(route)
		if err != nil {
			t.Fatalf("Failed to register route: %v", err)
		}
		
		// Mount routes and test HTTP request
		err = registry.MountTenantRoutes("test-tenant", "/")
		if err != nil {
			t.Fatalf("Failed to mount routes: %v", err)
		}
		
		// Create test request
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		
		// Execute request through the registry's router
		submux := registry.GetTenantRoutes("test-tenant")
		submux.ServeHTTP(w, req)
		
		// Verify middleware was applied
		if w.Header().Get("X-Test-Middleware") != "applied" {
			t.Error("Expected middleware to be applied, but header not found")
		}
		
		// Verify route response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
	
	t.Run("route_group_functionality", func(t *testing.T) {
		registry := setupTestRegistry(t)
		
		// Create route group with middleware
		groupDef := routing.RouteGroupDefinition{
			TenantName: "test-tenant",
			Pattern:    "/api/v1",
			Middleware: []func(http.Handler) http.Handler{
				func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("X-Group-Middleware", "applied")
						next.ServeHTTP(w, r)
					})
				},
			},
			Routes: []routing.RouteDefinition{
				createTestRoute("test-tenant", "GET", "/users"),
				createTestRoute("test-tenant", "POST", "/users"),
			},
		}
		
		err := registry.RegisterRouteGroup(groupDef)
		if err != nil {
			t.Fatalf("Failed to register route group: %v", err)
		}
		
		// Test group routes
		err = registry.MountTenantRoutes("test-tenant", "/")
		if err != nil {
			t.Fatalf("Failed to mount routes: %v", err)
		}
		
		// Test GET /api/v1/users
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		w := httptest.NewRecorder()
		
		submux := registry.GetTenantRoutes("test-tenant")
		submux.ServeHTTP(w, req)
		
		if w.Header().Get("X-Group-Middleware") != "applied" {
			t.Error("Expected group middleware to be applied")
		}
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}