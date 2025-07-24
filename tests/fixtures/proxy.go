package fixtures

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ProxyTestEnv represents a complete proxy testing environment
type ProxyTestEnv struct {
	*GatewayTestEnv
	Backend *httptest.Server
}

// SetupProxy creates a complete proxy test environment with backend
func SetupProxy(t *testing.T, tenantName, pathPrefix string, backend *httptest.Server) *ProxyTestEnv {
	cfg := CreateConfigWithBackend(tenantName, pathPrefix, backend.URL)
	gatewayEnv := SetupGateway(t, cfg)
	
	// Ensure backend is marked as alive
	if tenantRouter := gatewayEnv.Gateway.GetTenantRouter(tenantName); tenantRouter != nil {
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}
	}
	
	return &ProxyTestEnv{
		GatewayTestEnv: gatewayEnv,
		Backend:        backend,
	}
}

// SetupProxyWithHandler creates a proxy environment and registers a proxy handler
func SetupProxyWithHandler(t *testing.T, tenantName, pathPrefix, routePattern string, backend *httptest.Server) *ProxyTestEnv {
	env := SetupProxy(t, tenantName, pathPrefix, backend)
	
	// Register the standard proxy handler
	env.Router.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {
		router, stripPrefix := env.Gateway.MatchRoute(r.Host, r.URL.Path)
		if router == nil {
			http.NotFound(w, r)
			return
		}
		backend := router.NextBackend()
		if backend == nil {
			http.Error(w, "No backend available", http.StatusBadGateway)
			return
		}
		proxy := env.Gateway.CreateProxy(backend, stripPrefix)
		proxy.ServeHTTP(w, r)
	})
	
	return env
}

// SetupSimpleProxy creates a proxy with a simple backend for basic testing
func SetupSimpleProxy(t *testing.T, tenantName, pathPrefix, routePattern string) *ProxyTestEnv {
	backend := CreateSimpleBackend(t)
	return SetupProxyWithHandler(t, tenantName, pathPrefix, routePattern, backend)
}

// SetupErrorProxy creates a proxy with an error-producing backend
func SetupErrorProxy(t *testing.T, tenantName, pathPrefix, routePattern string) *ProxyTestEnv {
	backend := CreateErrorBackend(t)
	return SetupProxyWithHandler(t, tenantName, pathPrefix, routePattern, backend)
}

// SetupEchoProxy creates a proxy with an echo backend for request inspection
func SetupEchoProxy(t *testing.T, tenantName, pathPrefix, routePattern string) *ProxyTestEnv {
	backend := CreateEchoBackend(t)
	return SetupProxyWithHandler(t, tenantName, pathPrefix, routePattern, backend)
}

// Cleanup cleans up the proxy test environment
func (env *ProxyTestEnv) Cleanup() {
	if env.Backend != nil {
		env.Backend.Close()
	}
}