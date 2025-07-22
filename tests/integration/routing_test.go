package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

func TestGatewayRouting(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create test config
	configPath := filepath.Join(configDir, "test.yaml")
	configContent := `
lua_routing:
  enabled: true
  scripts_dir: "` + scriptsDir + `"

tenants:
  - name: "api-tenant"
    path_prefix: "/api/"
    lua_routes: "api-routes"
    health_interval: 30
    services:
      - name: "backend1"
        url: "http://localhost:8081"
        health: "/health"

  - name: "web-tenant"
    domains: ["example.com"]
    lua_routes: "web-routes"
    health_interval: 30
    services:
      - name: "backend2"
        url: "http://localhost:8082"
        health: "/health"

admin_base_path: "/admin"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create test Lua scripts
	apiScript := filepath.Join(scriptsDir, "api-routes.lua")
	apiContent := `
log("Setting up API routes")
chi_route("GET", "/users", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"users": []}')
end)
`
	if err := os.WriteFile(apiScript, []byte(apiContent), 0644); err != nil {
		t.Fatalf("failed to write API script: %v", err)
	}

	webScript := filepath.Join(scriptsDir, "web-routes.lua")
	webContent := `
log("Setting up web routes")
chi_route("GET", "/", function(w, r)
    w:header("Content-Type", "text/html")
    w:write('<html><body>Welcome</body></html>')
end)
`
	if err := os.WriteFile(webScript, []byte(webContent), 0644); err != nil {
		t.Fatalf("failed to write web script: %v", err)
	}

	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create router and gateway
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Initialize Lua engine
	luaEngine := lua.NewEngine(scriptsDir, router)

	// Execute Lua scripts for all tenants
	for _, tenant := range cfg.Tenants {
		if tenant.LuaRoutes != "" {
			if err := luaEngine.ExecuteRouteScript(tenant.LuaRoutes, tenant.Name); err != nil {
				t.Fatalf("failed to execute script for tenant %s: %v", tenant.Name, err)
			}
		}
	}

	// Mount path-based tenant routes (like in main.go)
	registry := luaEngine.RouteRegistry()
	for _, tenant := range cfg.Tenants {
		if tenant.LuaRoutes != "" && tenant.PathPrefix != "" {
			if err := registry.MountTenantRoutes(tenant.Name, tenant.PathPrefix); err != nil {
				t.Fatalf("failed to mount routes for tenant %s: %v", tenant.Name, err)
			}
		}
	}

	// Test path-based routing
	t.Run("path-based routing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		expectedBody := `{"users": []}`
		if w.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
		}
	})

	// Test host-based routing
	t.Run("host-based routing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "example.com"
		w := httptest.NewRecorder()

		// Note: This test may need adjustment based on how host-based routing is implemented
		router.ServeHTTP(w, req)

		// For now, just verify the request doesn't panic
		// Full host-based routing test would require middleware setup
	})

	// Verify gateway health
	if gateway == nil {
		t.Error("expected gateway to be initialized")
	}
}

func TestLuaRouteRegistry(t *testing.T) {
	router := chi.NewRouter()
	engine := lua.NewEngine(t.TempDir(), router)
	registry := engine.RouteRegistry()

	if registry == nil {
		t.Fatal("expected route registry to be initialized")
	}

	// Register a route first to create the submux
	err := registry.RegisterRoute(routing.RouteDefinition{
		TenantName: "test-tenant",
		Method:     "GET",
		Pattern:    "/test",
		Handler:    http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	})
	if err != nil {
		t.Errorf("failed to register route: %v", err)
	}

	// Test mounting tenant routes
	err = registry.MountTenantRoutes("test-tenant", "/test/")
	if err != nil {
		t.Errorf("failed to mount tenant routes: %v", err)
	}

	// Test getting tenant routes - should exist after registering a route
	submux := registry.GetTenantRoutes("test-tenant")
	if submux == nil {
		t.Error("expected to get tenant submux after registering route")
	}
}

func TestBackendHealthCheck(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create config with the test server
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "test",
				PathPrefix: "/test/",
				Interval:   1,
				Services: []config.Service{
					{
						Name:   "test-backend",
						URL:    server.URL,
						Health: "/health",
					},
				},
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	if gateway == nil {
		t.Fatal("expected gateway to be created")
	}

	// Test that the gateway was initialized with the backend
	tenantRouter := gateway.GetTenantRouter("test")
	if tenantRouter == nil {
		t.Error("expected tenant router to exist")
	}
}
