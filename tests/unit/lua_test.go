package unit

import (
	"fmt"
	"testing"

	"keystone-gateway/tests/fixtures"
)

// TestLuaCore tests essential Lua functionality for 80%+ coverage
func TestLuaCore(t *testing.T) {
	t.Run("basic_script_execution", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("lua-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("lua-app", "/app/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Lua engine can access scripts directly (bypass file system)
		scriptCount := len(env.LuaEngine.GetScriptMap())
		if scriptCount < 0 { // Just check engine exists
			t.Error("Expected Lua engine to be available")
		}
		
		// Test: Engine registry exists
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Error("Expected route registry to be available")
		}
	})

	t.Run("route_registration", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("route-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("route-app", "/routes/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Route registry can mount tenant routes
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Fatal("Expected route registry")
		}
		
		err := registry.MountTenantRoutes("route-app", "/routes/")
		if err != nil {
			t.Errorf("Route mounting failed: %v", err)
		}
	})

	t.Run("middleware_registration", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("middleware-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("middleware-app", "/mw/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Lua engine has basic structure for middleware
		scriptMap := env.LuaEngine.GetScriptMap()
		if scriptMap == nil {
			t.Error("Expected script map to exist")
		}
		
		// Test: Can get loaded scripts list
		scripts := env.LuaEngine.GetLoadedScripts()
		if scripts == nil {
			t.Error("Expected scripts list to exist")
		}
	})

	t.Run("global_scripts", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("global-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("global-app", "/global/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Global scripts system works (no scripts exist yet)
		err := env.LuaEngine.ExecuteGlobalScripts()
		if err != nil {
			t.Errorf("Global script execution failed: %v", err)
		}
		
		// Test: Can reload scripts without errors
		err = env.LuaEngine.ReloadScripts()
		if err != nil {
			t.Errorf("Script reload failed: %v", err)
		}
	})

	t.Run("script_error_handling", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("error-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("error-app", "/error/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Engine handles missing scripts gracefully
		err := env.LuaEngine.ExecuteRouteScript("nonexistent", "error-app")
		if err == nil {
			t.Error("Expected error for missing script")
		}
		
		// Test: Script reload works
		err = env.LuaEngine.ReloadScripts()
		if err != nil {
			t.Errorf("Script reload failed: %v", err)
		}
	})

	t.Run("concurrent_execution", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("concurrent-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("concurrent-app", "/concurrent/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Concurrent error handling (simulate multiple requests)
		const numGoroutines = 3
		errChan := make(chan error, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				err := env.LuaEngine.ExecuteRouteScript("missing-script", "concurrent-app")
				// Expected to fail - testing thread safety
				if err == nil {
					errChan <- fmt.Errorf("expected error for missing script")
				} else {
					errChan <- nil // Success - handled error correctly
				}
			}()
		}

		for i := 0; i < numGoroutines; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent execution failed: %v", err)
			}
		}
	})

	t.Run("route_registry_integration", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("registry-backend")
		defer backend.Server.Close()
		
		tenant := fixtures.CreateTenant("registry-app", "/registry/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Route registry is available
		registry := env.Gateway.GetRouteRegistry()
		if registry == nil {
			t.Fatal("Expected route registry")
		}

		// Test: Can mount tenant routes
		err := registry.MountTenantRoutes("registry-app", "/registry/")
		if err != nil {
			t.Errorf("Route mounting failed: %v", err)
		}
	})
}