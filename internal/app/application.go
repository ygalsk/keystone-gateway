package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

const DefaultRequestTimeout = 10 * time.Second

type Application struct {
	gateway   *routing.Gateway
	luaEngine *lua.Engine
	config    *config.Config
	router    *chi.Mux
}

func New(cfg *config.Config, version string) (*Application, error) {
	router := chi.NewRouter()

	// Setup middleware FIRST, before any routes are defined
	setupMiddleware(router, cfg)

	// Create Gateway WITHOUT setting up routes yet
	gateway := routing.NewGateway(cfg, router)

	// Initialize Lua engine if enabled
	var luaEngine *lua.Engine
	if cfg.LuaRouting.Enabled {
		scriptsDir := cfg.LuaRouting.ScriptsDir
		if scriptsDir == "" {
			scriptsDir = "./scripts"
		}
		luaEngine = lua.NewEngine(scriptsDir, router, cfg.RequestLimits.MaxBodySize)
	}

	app := &Application{
		gateway:   gateway,
		luaEngine: luaEngine,
		config:    cfg,
		router:    router,
	}

	// Setup Lua routing FIRST (registers middleware)
	if app.luaEngine != nil {
		app.setupLuaRouting()
	}

	// Simple health check endpoint (for load balancers to check if gateway is running)
	app.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Finally, setup Gateway routes (these are the tenant proxy routes)
	gateway.SetupRoutes()

	return app, nil
}

// setupMiddleware configures middleware for the router
func setupMiddleware(r *chi.Mux, cfg *config.Config) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(DefaultRequestTimeout))
	r.Use(middleware.Throttle(100))
	// Request size limits
	r.Use(middleware.RequestSize(cfg.RequestLimits.MaxBodySize))

	if cfg.Compression.Enabled {
		r.Use(middleware.Compress(cfg.Compression.Level, cfg.Compression.ContentTypes...))
	}
	r.Use(middleware.CleanPath)
	r.Use(middleware.StripSlashes)
}

func (app *Application) Handler() http.Handler {
	// Use the Gateway's handler instead of our own router
	return app.gateway.Handler()
}

func (app *Application) Stop() {
	app.gateway.Stop()
}

func (app *Application) setupLuaRouting() {
	// Execute global Lua scripts first
	if len(app.config.LuaRouting.GlobalScripts) > 0 {
		slog.Info("lua_global_scripts_starting", "count", len(app.config.LuaRouting.GlobalScripts), "component", "lua")
		if err := app.luaEngine.ExecuteGlobalScripts(); err != nil {
			slog.Error("lua_global_scripts_failed", "error", err, "component", "lua")
		} else {
			slog.Info("lua_global_scripts_completed", "component", "lua")
		}
	}

	// Execute tenant-specific Lua route scripts
	luaTenantsCount := 0
	for _, tenant := range app.config.Tenants {
		if len(tenant.LuaRoutes) > 0 {
			luaTenantsCount++
			slog.Info("lua_tenant_routes_starting", "tenant", tenant.Name, "scripts", tenant.LuaRoutes, "count", len(tenant.LuaRoutes), "component", "lua")
			for _, script := range tenant.LuaRoutes {
				slog.Info("lua_tenant_script_executing", "tenant", tenant.Name, "script", script, "component", "lua")
				if err := app.luaEngine.ExecuteRouteScript(script); err != nil {
					slog.Error("lua_tenant_script_failed", "tenant", tenant.Name, "script", script, "error", err, "component", "lua")
				} else {
					slog.Info("lua_tenant_script_completed", "tenant", tenant.Name, "script", script, "component", "lua")
				}
			}
			slog.Info("lua_tenant_routes_completed", "tenant", tenant.Name, "component", "lua")
		}
	}

	if luaTenantsCount > 0 {
		slog.Info("lua_routing_initialized", "tenants_with_lua", luaTenantsCount, "component", "lua")
	}
}
