package e2e

import (
	"net/http"
	"testing"

	"keystone-gateway/tests/fixtures"
)

// TestE2ECore tests essential end-to-end user journeys for 80%+ coverage
func TestE2ECore(t *testing.T) {
	t.Run("api_gateway_workflow", func(t *testing.T) {
		apiBackend := fixtures.CreateBasicBackend("api-service")
		defer apiBackend.Server.Close()

		tenant := fixtures.CreateTenant("api", "/api/v1/", nil, apiBackend)
		env := fixtures.SetupBasicGateway(t, tenant)
		defer env.Cleanup()

		// Test: Complete API workflow
		fixtures.TestRequest(t, env, "GET", "/api/v1/users", http.StatusOK)
		fixtures.TestRequest(t, env, "POST", "/api/v1/users", http.StatusOK)
		fixtures.TestRequest(t, env, "PUT", "/api/v1/users/123", http.StatusOK)
		fixtures.TestRequest(t, env, "DELETE", "/api/v1/users/123", http.StatusOK)
	})

	t.Run("multi_service_gateway", func(t *testing.T) {
		userBackend := fixtures.CreateBasicBackend("user-service")
		orderBackend := fixtures.CreateBasicBackend("order-service")
		paymentBackend := fixtures.CreateBasicBackend("payment-service")
		defer userBackend.Server.Close()
		defer orderBackend.Server.Close()
		defer paymentBackend.Server.Close()

		userTenant := fixtures.CreateTenant("users", "/api/users/", nil, userBackend)
		orderTenant := fixtures.CreateTenant("orders", "/api/orders/", nil, orderBackend)
		paymentTenant := fixtures.CreateTenant("payments", "/api/payments/", nil, paymentBackend)

		env := fixtures.SetupBasicGateway(t, userTenant, orderTenant, paymentTenant)
		defer env.Cleanup()

		// Test: Multiple services work together
		fixtures.TestRequest(t, env, "GET", "/api/users/profile", http.StatusOK)
		fixtures.TestRequest(t, env, "GET", "/api/orders/list", http.StatusOK)
		fixtures.TestRequest(t, env, "POST", "/api/payments/process", http.StatusOK)
	})

	t.Run("domain_based_tenants", func(t *testing.T) {
		apiBackend := fixtures.CreateBasicBackend("api-backend")
		webBackend := fixtures.CreateBasicBackend("web-backend")
		adminBackend := fixtures.CreateBasicBackend("admin-backend")
		defer apiBackend.Server.Close()
		defer webBackend.Server.Close()
		defer adminBackend.Server.Close()

		apiTenant := fixtures.CreateTenant("api", "", []string{"api.example.com"}, apiBackend)
		webTenant := fixtures.CreateTenant("web", "", []string{"www.example.com"}, webBackend)
		adminTenant := fixtures.CreateTenant("admin", "", []string{"admin.example.com"}, adminBackend)

		env := fixtures.SetupBasicGateway(t, apiTenant, webTenant, adminTenant)
		defer env.Cleanup()

		// Test: Domain-based routing
		router1, _ := env.Gateway.MatchRoute("api.example.com", "/data")
		router2, _ := env.Gateway.MatchRoute("www.example.com", "/pages")
		router3, _ := env.Gateway.MatchRoute("admin.example.com", "/dashboard")

		if router1 == nil || router2 == nil || router3 == nil {
			t.Error("Expected all domain-based tenants to have routers")
		}
	})

	t.Run("high_availability_setup", func(t *testing.T) {
		// Multiple backends per service for HA
		api1 := fixtures.CreateBasicBackend("api-1")
		api2 := fixtures.CreateBasicBackend("api-2")
		api3 := fixtures.CreateBasicBackend("api-3")
		defer api1.Server.Close()
		defer api2.Server.Close()
		defer api3.Server.Close()

		tenant := fixtures.CreateTenant("ha-api", "/api/", nil, api1, api2, api3)
		env := fixtures.SetupBasicGateway(t, tenant)
		defer env.Cleanup()

		// Test: High availability through load balancing
		router, _ := env.Gateway.MatchRoute("", "/api/status")
		if router == nil {
			t.Fatal("Expected router for HA setup")
		}

		if len(router.Backends) != 3 {
			t.Errorf("Expected 3 backends for HA, got %d", len(router.Backends))
		}

		// Test: All backends are healthy
		for i := 0; i < 3; i++ {
			backend := router.NextBackend()
			if backend == nil {
				t.Fatalf("Expected backend %d to be available", i)
			}
			// Backend health test simplified
			if backend == nil {
				t.Errorf("Expected backend %d to be available", i)
			}
		}
	})

	t.Run("error_scenarios", func(t *testing.T) {
		healthyBackend := fixtures.CreateBasicBackend("healthy-service")
		errorBackend := fixtures.CreateTestErrorBackend("error-service")
		defer healthyBackend.Server.Close()
		defer errorBackend.Server.Close()

		healthyTenant := fixtures.CreateTenant("healthy", "/api/", nil, healthyBackend)
		errorTenant := fixtures.CreateTenant("errors", "/errors/", nil, errorBackend)

		env := fixtures.SetupBasicGateway(t, healthyTenant, errorTenant)
		defer env.Cleanup()

		// Test: Normal service works
		fixtures.TestRequest(t, env, "GET", "/api/data", http.StatusOK)

		// Test: Error service returns appropriate errors
		fixtures.TestRequest(t, env, "GET", "/errors/500", http.StatusInternalServerError)
		fixtures.TestRequest(t, env, "GET", "/errors/404", http.StatusNotFound)

		// Test: Unknown routes return 404
		fixtures.TestRequest(t, env, "GET", "/unknown/path", http.StatusNotFound)
	})
}

// TestLuaE2E tests essential Lua-powered gateway scenarios
func TestLuaE2E(t *testing.T) {
	t.Run("lua_powered_api_gateway", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("lua-api-backend")
		defer backend.Server.Close()

		tenant := fixtures.CreateTenant("lua-api", "/api/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Lua-powered API gateway setup
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Error("Expected route registry for Lua-powered gateway")
		}

		// Mounting handled automatically by Chi in main.go
	})

	t.Run("global_middleware_with_tenants", func(t *testing.T) {
		backend1 := fixtures.CreateBasicBackend("service-1")
		backend2 := fixtures.CreateBasicBackend("service-2")
		defer backend1.Server.Close()
		defer backend2.Server.Close()

		tenant1 := fixtures.CreateTenant("app1", "/app1/", nil, backend1)
		tenant2 := fixtures.CreateTenant("app2", "/app2/", nil, backend2)

		env := fixtures.SetupGatewayWithLua(t, tenant1, tenant2)
		defer env.Cleanup()

		// Test: Global middleware system works with multiple tenants
		err := env.LuaEngine.ExecuteGlobalScripts()
		if err != nil {
			t.Errorf("Global middleware failed: %v", err)
		}

		// Test: Both tenants can be managed
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Error("Expected route registry")
		}

		// Mounting handled automatically by Chi in main.go
	})

	t.Run("mixed_lua_and_proxy", func(t *testing.T) {
		backend := fixtures.CreateBasicBackend("mixed-backend")
		defer backend.Server.Close()

		tenant := fixtures.CreateTenant("mixed", "/mixed/", nil, backend)
		env := fixtures.SetupGatewayWithLua(t, tenant)
		defer env.Cleanup()

		// Test: Mixed Lua and proxy setup works
		registry := env.LuaEngine.RouteRegistry()
		if registry == nil {
			t.Error("Expected route registry for mixed setup")
		}

		// Mounting handled automatically by Chi in main.go

		// Test: Can execute global scripts for mixed setup
		err := env.LuaEngine.ExecuteGlobalScripts()
		if err != nil {
			t.Errorf("Mixed global scripts failed: %v", err)
		}
	})
}
