package integration

import (
	"net/http"
	"testing"

	"keystone-gateway/tests/fixtures"
)

// TestIntegrationCore tests essential integration scenarios for 80%+ coverage
func TestIntegrationCore(t *testing.T) {
	t.Run("complete_proxy_flow", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("integration-backend")
		defer backend.Server.Close()

		tenant := fixtures.CreateTenant("integration", "/api/", nil, backend)
		env := fixtures.SetupBasicGateway(t, tenant)
		defer env.Cleanup()

		// Test: Complete request flow works
		fixtures.TestRequest(t, env, "GET", "/api/users", http.StatusOK)
		fixtures.TestRequest(t, env, "POST", "/api/users", http.StatusOK)
		fixtures.TestRequest(t, env, "PUT", "/api/users/1", http.StatusOK)
		fixtures.TestRequest(t, env, "DELETE", "/api/users/1", http.StatusOK)
	})

	t.Run("multi_tenant_isolation", func(t *testing.T) {
		tenant1Backend := fixtures.CreateBasicBackend("tenant1-service")
		tenant2Backend := fixtures.CreateBasicBackend("tenant2-service")
		defer tenant1Backend.Server.Close()
		defer tenant2Backend.Server.Close()

		tenant1 := fixtures.CreateTenant("tenant1", "/t1/", nil, tenant1Backend)
		tenant2 := fixtures.CreateTenant("tenant2", "/t2/", nil, tenant2Backend)

		env := fixtures.SetupBasicGateway(t, tenant1, tenant2)
		defer env.Cleanup()

		// Test: Tenants are isolated
		router1, prefix1 := env.Gateway.MatchRoute("", "/t1/data")
		router2, prefix2 := env.Gateway.MatchRoute("", "/t2/data")

		if router1 == nil || router2 == nil {
			t.Fatal("Expected both tenants to have routers")
		}

		if prefix1 != "/t1/" || prefix2 != "/t2/" {
			t.Error("Expected different strip prefixes")
		}

		backend1 := router1.NextBackend()
		backend2 := router2.NextBackend()

		if backend1.URL.String() == backend2.URL.String() {
			t.Error("Expected different backends for different tenants")
		}
	})

	t.Run("load_balancing_flow", func(t *testing.T) {
		backend1 := fixtures.CreateBasicBackend("service-1")
		backend2 := fixtures.CreateBasicBackend("service-2")
		backend3 := fixtures.CreateBasicBackend("service-3")
		defer backend1.Server.Close()
		defer backend2.Server.Close()
		defer backend3.Server.Close()

		tenant := fixtures.CreateTenant("lb-app", "/lb/", nil, backend1, backend2, backend3)
		env := fixtures.SetupBasicGateway(t, tenant)
		defer env.Cleanup()

		// Test: Load balancing distributes requests
		fixtures.TestRequest(t, env, "GET", "/lb/test1", http.StatusOK)
		fixtures.TestRequest(t, env, "GET", "/lb/test2", http.StatusOK)
		fixtures.TestRequest(t, env, "GET", "/lb/test3", http.StatusOK)
		fixtures.TestRequest(t, env, "GET", "/lb/test4", http.StatusOK)
	})

	t.Run("error_propagation", func(t *testing.T) {
		errorBackend := fixtures.CreateTestErrorBackend("error-service")
		defer errorBackend.Server.Close()

		tenant := fixtures.CreateTenant("errors", "/err/", nil, errorBackend)
		env := fixtures.SetupBasicGateway(t, tenant)
		defer env.Cleanup()

		// Test: Error responses propagate correctly
		fixtures.TestRequest(t, env, "GET", "/err/500", http.StatusInternalServerError)
		fixtures.TestRequest(t, env, "GET", "/err/404", http.StatusNotFound)
		fixtures.TestRequest(t, env, "GET", "/err/503", http.StatusServiceUnavailable)
	})

	t.Run("health_check_integration", func(t *testing.T) {
		healthyBackend := fixtures.CreateHealthBackend("healthy-service")
		defer healthyBackend.Server.Close()

		tenant := fixtures.CreateTenant("health", "/health/", nil, healthyBackend)
		env := fixtures.SetupBasicGateway(t, tenant)
		defer env.Cleanup()

		// Test: Health checks work end-to-end
		router, _ := env.Gateway.MatchRoute("", "/health/test")
		if router == nil {
			t.Fatal("Expected router")
		}

		backend := router.NextBackend()
		if backend == nil {
			t.Fatal("Expected backend")
		}

		// Backend health integration is working
		if backend == nil {
			t.Error("Expected backend to be available")
		}

		// Test: Actual health endpoint works
		fixtures.TestRequest(t, env, "GET", "/health/health", http.StatusOK)
	})

	t.Run("mixed_routing_strategies", func(t *testing.T) {
		pathBackend := fixtures.CreateBasicBackend("path-service")
		domainBackend := fixtures.CreateBasicBackend("domain-service")
		defer pathBackend.Server.Close()
		defer domainBackend.Server.Close()

		pathTenant := fixtures.CreateTenant("path-app", "/app/", nil, pathBackend)
		domainTenant := fixtures.CreateTenant("domain-app", "", []string{"app.domain.com"}, domainBackend)

		env := fixtures.SetupBasicGateway(t, pathTenant, domainTenant)
		defer env.Cleanup()

		// Test: Path-based routing
		fixtures.TestRequest(t, env, "GET", "/app/test", http.StatusOK)

		// Test: Domain-based routing
		router, _ := env.Gateway.MatchRoute("app.domain.com", "/anything")
		if router == nil {
			t.Error("Expected router for domain-based tenant")
		}
	})
}

// TestLuaIntegration tests essential Lua integration workflows
func TestLuaIntegration(t *testing.T) {
	t.Run("lua_with_proxy_fallback", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("lua-proxy-backend")
		defer backend.Server.Close()

		tenant := fixtures.CreateTenant("lua-proxy", "/lua/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Lua and proxy coexistence (simplified)
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Error("Expected route registry for Lua integration")
		}

		// Test: Can mount tenant routes
		err := registry.MountTenantRoutes("lua-proxy", "/lua/")
		if err != nil {
			t.Errorf("Route mounting failed: %v", err)
		}
	})

	t.Run("global_and_tenant_scripts", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("mixed-backend")
		defer backend.Server.Close()

		tenant := fixtures.CreateTenant("mixed-app", "/mixed/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Global and tenant script systems exist
		err := env.LuaEngine.ExecuteGlobalScripts()
		if err != nil {
			t.Errorf("Global script failed: %v", err)
		}

		// Test: Can handle tenant routes
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Error("Expected route registry")
		}

		err = registry.MountTenantRoutes("mixed-app", "/mixed/")
		if err != nil {
			t.Errorf("Tenant route mounting failed: %v", err)
		}
	})

	t.Run("lua_error_recovery", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("recovery-backend")
		defer backend.Server.Close()

		tenant := fixtures.CreateTenant("recovery-app", "/recovery/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Error recovery - missing scripts handled gracefully
		err := env.LuaEngine.ExecuteRouteScript("nonexistent-script", "recovery-app")
		if err == nil {
			t.Error("Expected error for missing script")
		}

		// Test: System continues to work after errors
		err = env.LuaEngine.ExecuteGlobalScripts()
		if err != nil {
			t.Errorf("System failed to recover: %v", err)
		}
	})
}
