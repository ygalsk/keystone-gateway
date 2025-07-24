// Package fixtures provides organized test fixtures following KISS and DRY principles
package fixtures

import (
	"testing"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

// GatewayTestEnv represents a complete gateway testing environment
type GatewayTestEnv struct {
	Gateway *routing.Gateway
	Router  *chi.Mux
	Config  *config.Config
}

// SetupGateway creates a basic gateway test environment
func SetupGateway(t *testing.T, cfg *config.Config) *GatewayTestEnv {
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	return &GatewayTestEnv{
		Gateway: gateway,
		Router:  router,
		Config:  cfg,
	}
}

// SetupSimpleGateway creates a gateway with a single tenant for simple tests
func SetupSimpleGateway(t *testing.T, tenantName, pathPrefix string) *GatewayTestEnv {
	cfg := CreateTestConfig(tenantName, pathPrefix)
	return SetupGateway(t, cfg)
}

// SetupMultiTenantGateway creates a gateway with multiple tenants for complex tests
func SetupMultiTenantGateway(t *testing.T) *GatewayTestEnv {
	cfg := CreateMultiTenantConfig()
	return SetupGateway(t, cfg)
}