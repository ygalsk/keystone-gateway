package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	lua_lib "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/middleware"
)

// RouterConfig contains all dependencies needed for router creation
type RouterConfig struct {
	Logger *slog.Logger
	Config *config.Config
}

// RouterResult contains the router and any optional Lua components
type RouterResult struct {
	Router          *chi.Mux
	LuaEngine       *lua.Engine
	LuaRouteRegistry *LuaRouteRegistry
	Gateway         *Gateway
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

	// Create multi-tenant gateway with Lua route registry
	gateway, luaEngine, err := initializeGatewayWithLua(r, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gateway: %w", err)
	}

	result.Gateway = gateway
	result.LuaEngine = luaEngine
	result.LuaRouteRegistry = gateway.GetRouteRegistry()

	if cfg.Config.Lua != nil && cfg.Config.Lua.Enabled {
		cfg.Logger.Info("lua scripting enabled",
			"max_states", cfg.Config.Lua.MaxStates,
			"max_scripts", cfg.Config.Lua.MaxScripts,
			"scripts_dir", cfg.Config.Lua.ScriptsDir)
	} else {
		cfg.Logger.Info("lua scripting disabled")
	}

	return result, nil
}

// initializeGatewayWithLua creates the multi-tenant gateway with integrated Lua support
func initializeGatewayWithLua(r *chi.Mux, cfg RouterConfig) (*Gateway, *lua.Engine, error) {
	// Create the gateway with tenant routing + Lua registry
	gateway := NewGatewayWithRouter(cfg.Config, r)
	
	var luaEngine *lua.Engine
	
	// Initialize Lua engine if enabled
	if cfg.Config.Lua != nil && cfg.Config.Lua.Enabled {
		// Use configuration values or defaults
		maxStates := cfg.Config.Lua.MaxStates
		if maxStates <= 0 {
			maxStates = 10
		}
		maxScripts := cfg.Config.Lua.MaxScripts
		if maxScripts <= 0 {
			maxScripts = 100
		}

		luaEngine = lua.NewEngine(maxStates, maxScripts)
		
		// Load tenant Lua scripts through the route registry
		if err := loadTenantLuaScripts(gateway, luaEngine, cfg); err != nil {
			return nil, nil, fmt.Errorf("failed to load tenant Lua scripts: %w", err)
		}
	}
	
	return gateway, luaEngine, nil
}

// loadTenantLuaScripts loads Lua scripts for tenants using the new architecture
func loadTenantLuaScripts(gateway *Gateway, luaEngine *lua.Engine, cfg RouterConfig) error {
	if cfg.Config.Lua == nil || len(cfg.Config.Lua.TenantScripts) == 0 {
		return nil
	}
	
	registry := gateway.GetRouteRegistry()
	
	// Create state pool for Lua execution
	statePool := lua.NewLuaStatePool(cfg.Config.Lua.MaxStates, func() *lua_lib.LState {
		return lua.CreateSecureLuaState(lua.DefaultSecurityConfig())
	})
	
	// Load tenant scripts
	for tenantID, tenantConfig := range cfg.Config.Lua.TenantScripts {
		if !tenantConfig.Enabled {
			continue
		}

		// Load middleware script first
		if tenantConfig.MiddlewareScript != "" {
			if err := loadTenantScriptWithRegistry(registry, statePool, cfg.Config.Lua.ScriptsDir, tenantConfig.MiddlewareScript, tenantID, cfg.Logger); err != nil {
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
			if err := loadTenantScriptWithRegistry(registry, statePool, cfg.Config.Lua.ScriptsDir, tenantConfig.RoutingScript, tenantID, cfg.Logger); err != nil {
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
	
	return nil
}

// loadTenantScriptWithRegistry loads and executes a Lua script using the route registry
func loadTenantScriptWithRegistry(registry *LuaRouteRegistry, statePool *lua.LuaStatePool, scriptsDir, scriptFile, tenantID string, logger *slog.Logger) error {
	// Read script file
	scriptPath := filepath.Join(scriptsDir, scriptFile)
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", scriptPath, err)
	}

	// Get Lua state from pool with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	L, err := statePool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Lua state: %w", err)
	}
	defer statePool.Put(L)

	// Set up Lua bindings directly using the existing system
	// Create a simple router adapter that connects to the registry  
	routerAdapter := &simpleRegistryAdapter{registry: registry}
	bindings := lua.NewLuaBindings(routerAdapter, statePool, nil, logger)
	
	// Set up the bindings - this registers chi_route(), chi_middleware(), etc.
	if err := bindings.SetupLuaBindings(L, scriptFile, tenantID); err != nil {
		return fmt.Errorf("failed to setup Lua bindings: %w", err)
	}

	// Execute script - this will register routes/middleware through the registry
	if err := L.DoString(string(scriptContent)); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}

// simpleRegistryAdapter connects LuaBindings to the LuaRouteRegistry
type simpleRegistryAdapter struct {
	registry *LuaRouteRegistry
}

func (s *simpleRegistryAdapter) RegisterRoute(ctx context.Context, method, pattern string, handler http.HandlerFunc, tenantName, scriptTag string) error {
	return s.registry.RegisterRoute(RouteDefinition{
		TenantName: tenantName,
		Method:     method,
		Pattern:    pattern,
		Handler:    handler,
	})
}

func (s *simpleRegistryAdapter) RegisterMiddleware(ctx context.Context, pattern string, middleware func(http.Handler) http.Handler, tenantName, scriptTag string) error {
	return s.registry.RegisterMiddleware(MiddlewareDefinition{
		TenantName: tenantName,
		Pattern:    pattern,
		Middleware: middleware,
	})
}

func (s *simpleRegistryAdapter) CreateGroup(ctx context.Context, pattern string, setupFunc func(chi.Router), tenantName, scriptTag string) error {
	return nil // Groups can be implemented later if needed
}

func (s *simpleRegistryAdapter) GetRoutes() map[string]*lua.RouteInfo {
	return make(map[string]*lua.RouteInfo)
}

func (s *simpleRegistryAdapter) RemoveRoute(ctx context.Context, method, pattern string) error {
	return nil // Not needed for basic functionality
}
