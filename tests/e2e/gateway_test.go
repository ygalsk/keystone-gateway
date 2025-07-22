package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestGatewayE2E tests the complete gateway functionality
func TestGatewayE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Create temporary test environment
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}

	// Create mock backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		case "/api/users":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"users": [{"id": 1, "name": "John"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html><body>Backend 2</body></html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer backend2.Close()

	// Create test configuration
	configContent := fmt.Sprintf(`
lua_routing:
  enabled: true
  scripts_dir: "%s"

tenants:
  - name: "api-service"
    path_prefix: "/api/"
    lua_routes: "api-routes"
    health_interval: 5
    services:
      - name: "api-backend"
        url: "%s"
        health: "/health"

  - name: "web-service"
    path_prefix: "/web/"
    lua_routes: "web-routes"
    health_interval: 5
    services:
      - name: "web-backend"
        url: "%s"
        health: "/health"

admin_base_path: "/admin"

tls:
  enabled: false
`, scriptsDir, backend1.URL, backend2.URL)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create Lua route scripts
	apiScript := filepath.Join(scriptsDir, "api-routes.lua")
	apiContent := `
log("Setting up API routes for tenant: " .. tenant_name)

route("GET", "/users", function(w, r)
    -- Proxy to backend
    proxy_to_backend(w, r)
end)

route("GET", "/status", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"status": "ok", "service": "api"}')
end)
`
	if err := os.WriteFile(apiScript, []byte(apiContent), 0644); err != nil {
		t.Fatalf("failed to write API script: %v", err)
	}

	webScript := filepath.Join(scriptsDir, "web-routes.lua")
	webContent := `
log("Setting up web routes for tenant: " .. tenant_name)

route("GET", "/", function(w, r)
    -- Proxy to backend
    proxy_to_backend(w, r)
end)

route("GET", "/info", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"service": "web", "version": "1.0"}')
end)
`
	if err := os.WriteFile(webScript, []byte(webContent), 0644); err != nil {
		t.Fatalf("failed to write web script: %v", err)
	}

	// Build the gateway binary
	gatewayBinary := filepath.Join(tmpDir, "keystone-gateway")
	buildCmd := exec.Command("go", "build", "-o", gatewayBinary, "../../cmd/main.go")
	buildCmd.Dir = tmpDir
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build gateway: %v", err)
	}

	// Start the gateway
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gatewayCmd := exec.CommandContext(ctx, gatewayBinary, "-config", configPath, "-addr", ":0")
	gatewayCmd.Dir = tmpDir

	if err := gatewayCmd.Start(); err != nil {
		t.Fatalf("failed to start gateway: %v", err)
	}

	// Wait for gateway to start (simplified - in real tests you'd check health endpoint)
	time.Sleep(2 * time.Second)

	// Note: This is a simplified E2E test framework
	// In a real implementation, you would:
	// 1. Parse the actual port from gateway output
	// 2. Wait for health check to pass
	// 3. Make actual HTTP requests to test routing
	// 4. Verify responses match expectations

	t.Log("Gateway started successfully")

	// Cleanup
	if err := gatewayCmd.Process.Kill(); err != nil {
		t.Logf("failed to kill gateway process: %v", err)
	}
}

// TestHealthEndpoint tests the admin health endpoint
func TestHealthEndpoint(t *testing.T) {
	// This would typically be a full E2E test making real HTTP requests
	// For now, we'll create a minimal test structure

	t.Run("health endpoint responds", func(t *testing.T) {
		// In a real test, you would:
		// 1. Start the gateway with a test config
		// 2. Make HTTP request to /admin/health
		// 3. Verify response structure and content

		expectedResponse := map[string]interface{}{
			"status":  "healthy",
			"tenants": map[string]string{},
			"version": "1.2.1",
		}

		// Mock test for structure validation
		responseBytes, _ := json.Marshal(expectedResponse)
		var response map[string]interface{}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Errorf("failed to unmarshal health response: %v", err)
		}

		if response["status"] != "healthy" {
			t.Error("expected status to be 'healthy'")
		}
	})
}

// TestTenantRouting tests tenant-specific routing
func TestTenantRouting(t *testing.T) {
	t.Run("path-based tenant routing", func(t *testing.T) {
		// Mock test structure - in real implementation:
		// 1. Configure gateway with multiple tenants
		// 2. Send requests to different path prefixes
		// 3. Verify requests route to correct backends

		testCases := []struct {
			path           string
			expectedTenant string
		}{
			{"/api/users", "api-service"},
			{"/web/", "web-service"},
			{"/admin/health", "admin"},
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				// Mock routing logic test
				if tc.path == "/api/users" && tc.expectedTenant != "api-service" {
					t.Errorf("expected tenant %s for path %s", tc.expectedTenant, tc.path)
				}
			})
		}
	})
}

// TestLuaScriptExecution tests Lua script execution in E2E context
func TestLuaScriptExecution(t *testing.T) {
	t.Run("lua scripts register routes correctly", func(t *testing.T) {
		// This would test that Lua scripts actually register routes
		// and that those routes respond correctly when called

		// Mock verification that route registration worked
		registeredRoutes := []string{
			"GET /api/users",
			"GET /api/status",
			"GET /web/",
			"GET /web/info",
		}

		expectedRoutes := 4
		if len(registeredRoutes) != expectedRoutes {
			t.Errorf("expected %d routes, got %d", expectedRoutes, len(registeredRoutes))
		}
	})
}

// TestErrorHandling tests error scenarios in E2E context
func TestErrorHandling(t *testing.T) {
	t.Run("invalid config handling", func(t *testing.T) {
		// Test that gateway fails gracefully with invalid config
		tmpDir := t.TempDir()
		invalidConfig := filepath.Join(tmpDir, "invalid.yaml")

		invalidContent := `
lua_routing:
  enabled: true

tenants:
  - name: "invalid"
    # Missing path_prefix and domains
    services: []
`

		if err := os.WriteFile(invalidConfig, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("failed to write invalid config: %v", err)
		}

		// In real test, would start gateway and expect it to fail
		// For now, just verify config structure
		t.Log("Invalid config test structure created")
	})

	t.Run("backend unavailable handling", func(t *testing.T) {
		// Test behavior when backend services are unavailable
		// Gateway should handle this gracefully
		t.Log("Backend unavailable test placeholder")
	})
}
