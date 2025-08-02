package unit

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

// Application represents the main application from cmd/main.go
type Application struct {
	gateway   *routing.Gateway
	luaEngine *lua.Engine
	config    *config.Config
}

// NewApplicationWithLuaRouting creates an application with embedded Lua routing
func NewApplicationWithLuaRouting(cfg *config.Config, router *chi.Mux) *Application {
	gateway := routing.NewGatewayWithRouter(cfg, router)

	var luaEngine *lua.Engine
	if cfg.LuaRouting != nil && cfg.LuaRouting.Enabled {
		scriptsDir := cfg.LuaRouting.ScriptsDir
		if scriptsDir == "" {
			scriptsDir = "./scripts"
		}
		luaEngine = lua.NewEngine(scriptsDir, router)
	}

	return &Application{
		gateway:   gateway,
		luaEngine: luaEngine,
		config:    cfg,
	}
}

func TestMainApplication(t *testing.T) {
	t.Run("new_application_with_lua_routing", func(t *testing.T) {
		cfg := &config.Config{
			LuaRouting: &config.LuaRoutingConfig{
				Enabled:    true,
				ScriptsDir: "./test-scripts",
			},
			Tenants: []config.Tenant{
				{
					Name:       "test",
					PathPrefix: "/test/",
					Services: []config.Service{
						{Name: "backend", URL: "http://localhost:8080", Health: "/health"},
					},
				},
			},
		}

		router := chi.NewRouter()
		app := NewApplicationWithLuaRouting(cfg, router)
		defer app.gateway.StopHealthChecks()

		if app == nil {
			t.Error("Expected application to be created")
		}

		if app.gateway == nil {
			t.Error("Expected gateway to be initialized")
		}

		if app.luaEngine == nil {
			t.Error("Expected Lua engine to be initialized when enabled")
		}

		if app.config != cfg {
			t.Error("Expected config to be set")
		}
	})

	t.Run("application_without_lua", func(t *testing.T) {
		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "test",
					PathPrefix: "/test/",
					Services: []config.Service{
						{Name: "backend", URL: "http://localhost:8080", Health: "/health"},
					},
				},
			},
		}

		router := chi.NewRouter()
		app := NewApplicationWithLuaRouting(cfg, router)
		defer app.gateway.StopHealthChecks()

		if app.luaEngine != nil {
			t.Error("Expected Lua engine to be nil when not enabled")
		}
	})
}
