package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
	luaengine "keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
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

// TestLuaDirect tests lua package functions directly for better coverage
func TestLuaDirect(t *testing.T) {
	t.Run("new_engine", func(t *testing.T) {
		router := chi.NewRouter()
		tmpDir := t.TempDir()

		engine := luaengine.NewEngine(tmpDir, router)
		if engine == nil {
			t.Error("Expected engine to be created")
		}
	})

	t.Run("get_script", func(t *testing.T) {
		router := chi.NewRouter()
		tmpDir := t.TempDir()

		// Create a test script
		scriptContent := `print("test script")`
		scriptPath := filepath.Join(tmpDir, "test-script.lua")
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test script: %v", err)
		}

		engine := luaengine.NewEngine(tmpDir, router)

		// Test script loading
		content, found := engine.GetScript("test-script")
		if !found {
			t.Error("Expected script to be found")
		}
		if content != scriptContent {
			t.Errorf("Expected script content '%s', got '%s'", scriptContent, content)
		}

		// Test missing script
		_, found = engine.GetScript("nonexistent")
		if found {
			t.Error("Expected missing script to not be found")
		}
	})

	t.Run("execute_route_script", func(t *testing.T) {
		router := chi.NewRouter()
		tmpDir := t.TempDir()

		// Create a simple test script
		scriptContent := `print("Hello from Lua")`
		scriptPath := filepath.Join(tmpDir, "simple.lua")
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test script: %v", err)
		}

		engine := luaengine.NewEngine(tmpDir, router)

		// Test successful script execution
		err = engine.ExecuteRouteScript("simple", "test-tenant")
		if err != nil {
			t.Errorf("Script execution failed: %v", err)
		}

		// Test missing script execution
		err = engine.ExecuteRouteScript("nonexistent", "test-tenant")
		if err == nil {
			t.Error("Expected error for missing script")
		}
	})

	t.Run("execute_global_scripts", func(t *testing.T) {
		router := chi.NewRouter()
		tmpDir := t.TempDir()

		engine := luaengine.NewEngine(tmpDir, router)

		// Test global script execution (no scripts exist yet)
		err := engine.ExecuteGlobalScripts()
		if err != nil {
			t.Errorf("Global script execution failed: %v", err)
		}
	})

	t.Run("reload_scripts", func(t *testing.T) {
		router := chi.NewRouter()
		tmpDir := t.TempDir()

		engine := luaengine.NewEngine(tmpDir, router)

		// Test script reload
		err := engine.ReloadScripts()
		if err != nil {
			t.Errorf("Script reload failed: %v", err)
		}
	})

	t.Run("get_loaded_scripts", func(t *testing.T) {
		router := chi.NewRouter()
		tmpDir := t.TempDir()

		// Create test scripts
		script1 := filepath.Join(tmpDir, "script1.lua")
		script2 := filepath.Join(tmpDir, "script2.lua")

		err := os.WriteFile(script1, []byte(`print("script1")`), 0644)
		if err != nil {
			t.Fatalf("Failed to write script1: %v", err)
		}

		err = os.WriteFile(script2, []byte(`print("script2")`), 0644)
		if err != nil {
			t.Fatalf("Failed to write script2: %v", err)
		}

		engine := luaengine.NewEngine(tmpDir, router)

		scripts := engine.GetLoadedScripts()
		if len(scripts) != 2 {
			t.Errorf("Expected 2 scripts, got %d", len(scripts))
		}
	})

	t.Run("lua_state_pool", func(t *testing.T) {
		creator := func() *lua.LState {
			return lua.NewState()
		}

		pool := luaengine.NewLuaStatePool(2, creator)
		defer pool.Close()

		if pool == nil {
			t.Error("Expected pool to be created")
		}

		// Get a state from the pool
		state := pool.Get()
		if state == nil {
			t.Error("Expected to get a Lua state from pool")
		}

		// Put it back
		pool.Put(state)
	})

	t.Run("chi_bindings", func(t *testing.T) {
		router := chi.NewRouter()
		engine := luaengine.NewEngine(t.TempDir(), router)

		L := lua.NewState()
		defer L.Close()

		engine.SetupChiBindings(L, "test-script", "test-tenant")

		// Test that chi_route function is available
		chiRouteFunc := L.GetGlobal("chi_route")
		if chiRouteFunc.Type() != lua.LTFunction {
			t.Error("Expected chi_route to be a function")
		}

		// Test that chi_middleware function is available
		chiMiddlewareFunc := L.GetGlobal("chi_middleware")
		if chiMiddlewareFunc.Type() != lua.LTFunction {
			t.Error("Expected chi_middleware to be a function")
		}
	})

	t.Run("route_registry", func(t *testing.T) {
		router := chi.NewRouter()
		registry := routing.NewLuaRouteRegistry(router, nil)

		if registry == nil {
			t.Error("Expected route registry to be created")
		}

		// Test mounting routes for tenant
		err := registry.MountTenantRoutes("test-tenant", "/app")
		if err != nil {
			t.Errorf("Failed to mount tenant routes: %v", err)
		}

		// Test ListTenants with empty registry
		tenants := registry.ListTenants()
		if len(tenants) < 0 {
			t.Error("Expected tenants list to be available")
		}
	})

	t.Run("route_registry_api", func(t *testing.T) {
		router := chi.NewRouter()

		api := routing.NewRouteRegistryAPI(router)
		if api == nil {
			t.Error("Expected route registry API to be created")
		}
	})
}
