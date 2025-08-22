package router

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	lua_lib "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/metrics"
	"keystone-gateway/internal/middleware"
)

// RouterConfig contains all dependencies needed for router creation
type RouterConfig struct {
	Logger *slog.Logger
	Config *config.Config
}

// RouterResult contains the router and any optional Lua components
type RouterResult struct {
	Router       *chi.Mux
	LuaEngine    *lua.Engine
	LuaChiRouter *lua.ChiRouter
}

// NewRouter creates and configures a new Chi router with base middleware and optional Lua scripting
func NewRouter(cfg RouterConfig) (*RouterResult, error) {
	// Build a router with base middleware (excluding proxy middleware)
	r := chi.NewRouter()
	baseMiddleware := middleware.BuildBaseMiddleware(cfg.Logger, cfg.Config)
	for _, m := range baseMiddleware {
		r.Use(m)
	}

	result := &RouterResult{
		Router: r,
	}

	// Initialize Lua components based on configuration
	if cfg.Config.Lua != nil && cfg.Config.Lua.Enabled {
		luaEngine, luaChiRouter, err := initializeLuaComponents(r, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Lua components: %w", err)
		}

		result.LuaEngine = luaEngine
		result.LuaChiRouter = luaChiRouter

		cfg.Logger.Info("lua scripting enabled",
			"max_states", cfg.Config.Lua.MaxStates,
			"max_scripts", cfg.Config.Lua.MaxScripts,
			"scripts_dir", cfg.Config.Lua.ScriptsDir)
	} else {
		cfg.Logger.Info("lua scripting disabled")
	}

	return result, nil
}

// initializeLuaComponents handles Lua engine and ChiRouter initialization
func initializeLuaComponents(r *chi.Mux, cfg RouterConfig) (*lua.Engine, *lua.ChiRouter, error) {
	// Use configuration values or defaults
	maxStates := cfg.Config.Lua.MaxStates
	if maxStates <= 0 {
		maxStates = 10
	}
	maxScripts := cfg.Config.Lua.MaxScripts
	if maxScripts <= 0 {
		maxScripts = 100
	}

	luaEngine := lua.NewEngine(maxStates, maxScripts)
	luaMetrics := metrics.NewLuaMetrics()

	// Create state pool for the Chi router
	statePool := lua.NewLuaStatePool(maxStates, func() *lua_lib.LState {
		return lua.CreateSecureLuaState(lua.DefaultSecurityConfig())
	})

	// Initialize Lua Chi router with the main Chi router
	luaChiRouter := lua.NewChiRouter(r, statePool, luaMetrics, cfg.Logger)

	// Load tenant scripts - both middleware and routing
	for tenantID, tenantConfig := range cfg.Config.Lua.TenantScripts {
		if !tenantConfig.Enabled {
			continue
		}

		// Load middleware script first
		if tenantConfig.MiddlewareScript != "" {
			if err := loadTenantScript(luaChiRouter, statePool, cfg.Config.Lua.ScriptsDir, tenantConfig.MiddlewareScript, tenantID, cfg.Logger); err != nil {
				cfg.Logger.Error("failed to load tenant middleware script",
					"tenant", tenantID,
					"script", tenantConfig.MiddlewareScript,
					"error", err)
			} else {
				cfg.Logger.Info("loaded tenant middleware script",
					"tenant", tenantID,
					"script", tenantConfig.MiddlewareScript)
			}
		}

		// Load routing script second (so routes are registered after middleware)
		if tenantConfig.RoutingScript != "" {
			if err := loadTenantScript(luaChiRouter, statePool, cfg.Config.Lua.ScriptsDir, tenantConfig.RoutingScript, tenantID, cfg.Logger); err != nil {
				cfg.Logger.Error("failed to load tenant routing script",
					"tenant", tenantID,
					"script", tenantConfig.RoutingScript,
					"error", err)
			} else {
				cfg.Logger.Info("loaded tenant routing script",
					"tenant", tenantID,
					"script", tenantConfig.RoutingScript)
			}
		}
	}

	return luaEngine, luaChiRouter, nil
}

// loadTenantScript loads and executes a Lua script for a specific tenant
func loadTenantScript(luaChiRouter *lua.ChiRouter, statePool *lua.LuaStatePool, scriptsDir, scriptFile, tenantID string, logger *slog.Logger) error {
	// Read script file
	scriptPath := filepath.Join(scriptsDir, scriptFile)
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", scriptPath, err)
	}

	// Get Lua state from pool
	L := statePool.Get()
	defer statePool.Put(L)

	// Set up bindings so script can call chi_middleware()
	if err := luaChiRouter.SetupLuaBindings(L, scriptFile, tenantID); err != nil {
		return fmt.Errorf("failed to setup Lua bindings: %w", err)
	}

	// Execute script - this will register middleware
	if err := L.DoString(string(scriptContent)); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}

