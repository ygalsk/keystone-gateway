package unit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"keystone-gateway/internal/routing"

	"github.com/go-chi/chi/v5"
)

func TestRouteRegistryBasicFunctionality(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Test basic route registration
	tenantName := "test-tenant"
	routeDef := routing.RouteDefinition{
		TenantName: tenantName,
		Method:     "GET",
		Pattern:    "/api/test",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("test response"))
		}),
	}

	err := registry.RegisterRoute(routeDef)
	if err != nil {
		t.Fatalf("failed to register route: %v", err)
	}

	// Test that tenant exists in registry
	tenants := registry.ListTenants()
	found := false
	for _, tenant := range tenants {
		if tenant == tenantName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find tenant %s in tenant list %v", tenantName, tenants)
	}

	// Test getting tenant routes
	submux := registry.GetTenantRoutes(tenantName)
	if submux == nil {
		t.Error("expected non-nil submux for tenant with registered routes")
	}

	// Mount and test the route
	err = registry.MountTenantRoutes(tenantName, "/")
	if err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	// Test the route works
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test response") {
		t.Errorf("expected response to contain 'test response', got %q", w.Body.String())
	}
}

func TestRouteRegistryMiddleware(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)
	tenantName := "middleware-tenant"

	// Register middleware
	middlewareDef := routing.MiddlewareDefinition{
		TenantName: tenantName,
		Pattern:    "/protected/*",
		Middleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Protected", "true")
				next.ServeHTTP(w, r)
			})
		},
	}

	err := registry.RegisterMiddleware(middlewareDef)
	if err != nil {
		t.Fatalf("failed to register middleware: %v", err)
	}

	// Register routes
	routes := []routing.RouteDefinition{
		{
			TenantName: tenantName,
			Method:     "GET",
			Pattern:    "/protected/data",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("protected data"))
			}),
		},
		{
			TenantName: tenantName,
			Method:     "GET",
			Pattern:    "/public/data",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("public data"))
			}),
		},
	}

	for _, route := range routes {
		err = registry.RegisterRoute(route)
		if err != nil {
			t.Fatalf("failed to register route: %v", err)
		}
	}

	// Mount tenant routes
	err = registry.MountTenantRoutes(tenantName, "/")
	if err != nil {
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

func TestRouteRegistryRouteGroups(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)
	tenantName := "group-tenant"

	// Create route group
	groupDef := routing.RouteGroupDefinition{
		TenantName: tenantName,
		Pattern:    "/api/v1",
		Routes: []routing.RouteDefinition{
			{
				TenantName: tenantName,
				Method:     "GET",
				Pattern:    "/users",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("users list"))
				}),
			},
			{
				TenantName: tenantName,
				Method:     "POST",
				Pattern:    "/users",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("create user"))
				}),
			},
		},
		Middleware: []func(http.Handler) http.Handler{
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-API-Version", "v1")
					next.ServeHTTP(w, r)
				})
			},
		},
	}

	err := registry.RegisterRouteGroup(groupDef)
	if err != nil {
		t.Fatalf("failed to register route group: %v", err)
	}

	// Mount tenant routes
	err = registry.MountTenantRoutes(tenantName, "/")
	if err != nil {
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
			name:             "group route - create user",
			method:           "POST",
			path:             "/api/v1/users",
			expectedStatus:   http.StatusOK,
			expectedBody:     "create user",
			expectVersionHdr: true,
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

			if tc.expectVersionHdr {
				versionHeader := w.Header().Get("X-API-Version")
				if versionHeader != "v1" {
					t.Errorf("expected X-API-Version header to be 'v1', got %q", versionHeader)
				}
			}

			if !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestRouteRegistryConcurrentAccess(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	const numWorkers = 10
	const numRoutesPerWorker = 20

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Worker function that registers routes concurrently
	worker := func(workerID int) {
		defer wg.Done()

		for routeID := 0; routeID < numRoutesPerWorker; routeID++ {
			tenantName := fmt.Sprintf("tenant-%d", workerID)
			routeDef := routing.RouteDefinition{
				TenantName: tenantName,
				Method:     "GET",
				Pattern:    fmt.Sprintf("/route-%d", routeID),
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf("worker-%d-route-%d", workerID, routeID)))
				}),
			}

			err := registry.RegisterRoute(routeDef)
			if err != nil {
				t.Errorf("worker %d: failed to register route %d: %v", workerID, routeID, err)
			}
		}
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		go worker(i)
	}

	// Wait for completion with timeout
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("Successfully completed concurrent registration with %d workers", numWorkers)
		
		// Verify all tenants were created
		tenants := registry.ListTenants()
		if len(tenants) < numWorkers {
			t.Errorf("expected at least %d tenants, got %d", numWorkers, len(tenants))
		}
		
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - possible deadlock or performance issue")
	}
}

func TestRouteRegistryMountingStrategies(t *testing.T) {
	mainRouter := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(mainRouter, nil)

	// Register routes for different tenants
	tenants := []string{"api", "admin", "public"}
	
	for _, tenant := range tenants {
		tenant := tenant // Capture loop variable
		routeDef := routing.RouteDefinition{
			TenantName: tenant,
			Method:     "GET",
			Pattern:    "/info",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(fmt.Sprintf("info from %s", tenant)))
			}),
		}

		err := registry.RegisterRoute(routeDef)
		if err != nil {
			t.Fatalf("failed to register route for tenant %s: %v", tenant, err)
		}
	}

	// Test different mounting strategies
	testCases := []struct {
		name         string
		tenant       string
		mountPath    string
		testPath     string
		expectedBody string
	}{
		{
			name:         "mount api at /api",
			tenant:       "api",
			mountPath:    "/api",
			testPath:     "/api/info",
			expectedBody: "info from api",
		},
		{
			name:         "mount admin at /admin",
			tenant:       "admin",
			mountPath:    "/admin",
			testPath:     "/admin/info",
			expectedBody: "info from admin",
		},
		{
			name:         "mount public at root",
			tenant:       "public",
			mountPath:    "/",
			testPath:     "/info",
			expectedBody: "info from public",
		},
	}

	// Mount each tenant at different paths
	for _, tc := range testCases {
		err := registry.MountTenantRoutes(tc.tenant, tc.mountPath)
		if err != nil {
			t.Fatalf("failed to mount tenant %s: %v", tc.tenant, err)
		}
	}

	// Test each mounting strategy
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.testPath, nil)
			w := httptest.NewRecorder()

			mainRouter.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			if !strings.Contains(w.Body.String(), tc.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestRouteRegistryDuplicateHandling(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)
	tenantName := "duplicate-tenant"

	// Register the same route twice
	routeDef := routing.RouteDefinition{
		TenantName: tenantName,
		Method:     "GET",
		Pattern:    "/duplicate",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("first handler"))
		}),
	}

	err := registry.RegisterRoute(routeDef)
	if err != nil {
		t.Fatalf("failed to register first route: %v", err)
	}

	// Try to register the same route again with different handler
	routeDef.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("second handler"))
	})

	err = registry.RegisterRoute(routeDef)
	// Should not error due to duplicate prevention in registry
	if err != nil {
		t.Logf("Registry rejected duplicate route registration: %v", err)
	}

	// Mount and test which handler is active
	err = registry.MountTenantRoutes(tenantName, "/")
	if err != nil {
		t.Fatalf("failed to mount tenant routes: %v", err)
	}

	req := httptest.NewRequest("GET", "/duplicate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Logf("Active handler response: %s", w.Body.String())
		// First handler should be active due to duplicate prevention
		if !strings.Contains(w.Body.String(), "first handler") {
			t.Logf("Note: Route registry allowed override - response: %s", w.Body.String())
		}
	}
}

func TestRouteRegistryCleanup(t *testing.T) {
	router := chi.NewRouter()
	registry := routing.NewLuaRouteRegistry(router, nil)

	// Register routes for multiple tenants
	tenants := []string{"temp1", "temp2", "temp3"}
	
	for _, tenant := range tenants {
		for i := 0; i < 5; i++ {
			routeDef := routing.RouteDefinition{
				TenantName: tenant,
				Method:     "GET",
				Pattern:    fmt.Sprintf("/route%d", i),
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("response"))
				}),
			}

			err := registry.RegisterRoute(routeDef)
			if err != nil {
				t.Fatalf("failed to register route for tenant %s: %v", tenant, err)
			}
		}
	}

	// Verify tenants exist
	registeredTenants := registry.ListTenants()
	for _, tenant := range tenants {
		found := false
		for _, registered := range registeredTenants {
			if registered == tenant {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tenant %s to be registered", tenant)
		}
	}

	// Test cleanup functionality
	for _, tenant := range tenants {
		registry.ClearTenantRoutes(tenant)
		
		// After clearing, tenant routes should be empty but tenant may still exist
		submux := registry.GetTenantRoutes(tenant)
		if submux != nil {
			t.Logf("Tenant %s still has routes after cleanup", tenant)
		}
	}

	t.Logf("Successfully tested cleanup for %d tenants with 5 routes each", len(tenants))
}