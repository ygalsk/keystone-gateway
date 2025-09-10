package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/handlers"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

const DefaultRequestTimeout = 60 * time.Second

type Application struct {
	gateway   *routing.Gateway
	luaEngine *lua.Engine
	config    *config.Config
	handlers  *handlers.Handlers
	router    *chi.Mux
}

func New(cfg *config.Config, version string) (*Application, error) {
	router := chi.NewRouter()

	// Setup middleware FIRST, before any routes are defined
	setupMiddleware(router, cfg)

	// Now create the Gateway which will add routes
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Initialize Lua engine if enabled
	var luaEngine *lua.Engine
	if cfg.LuaRouting != nil && cfg.LuaRouting.Enabled {
		scriptsDir := cfg.LuaRouting.ScriptsDir
		if scriptsDir == "" {
			scriptsDir = "./scripts"
		}
		luaEngine = lua.NewEngine(scriptsDir, router)
	}

	// Create handlers
	appHandlers := handlers.New(gateway, luaEngine, version)

	app := &Application{
		gateway:   gateway,
		luaEngine: luaEngine,
		config:    cfg,
		handlers:  appHandlers,
		router:    router,
	}

	// Setup admin routes after Gateway routes are set up
	app.setupAdminRoutes()

	// Setup Lua routing if enabled
	if app.luaEngine != nil {
		app.setupLuaRouting()
	}

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

	compressionConfig := cfg.GetCompressionConfig()
	if compressionConfig.Enabled {
		r.Use(middleware.Compress(compressionConfig.Level, compressionConfig.ContentTypes...))
	}
	r.Use(middleware.CleanPath)
	r.Use(middleware.StripSlashes)
}

func (app *Application) Handler() http.Handler {
	// Use the Gateway's handler instead of our own router
	return app.gateway.Handler()
}

func (app *Application) Stop() {
	app.gateway.StopHealthChecks()
}

func (app *Application) setupAdminRoutes() {
	// Admin routes - add to the gateway's router
	basePath := app.config.AdminBasePath
	if basePath == "" {
		basePath = "/"
	}

	app.router.Route(basePath, func(r chi.Router) {
		r.Get("/health", app.handlers.Health)
		r.Get("/tenants", app.handlers.Tenants)
	})
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
		if tenant.LuaRoutes != "" {
			luaTenantsCount++
			slog.Info("lua_tenant_routes_starting", "tenant", tenant.Name, "script", tenant.LuaRoutes, "component", "lua")
			if err := app.luaEngine.ExecuteRouteScript(tenant.LuaRoutes); err != nil {
				slog.Error("lua_tenant_routes_failed", "tenant", tenant.Name, "script", tenant.LuaRoutes, "error", err, "component", "lua")
			} else {
				slog.Info("lua_tenant_routes_completed", "tenant", tenant.Name, "component", "lua")
			}
		}
	}

	if luaTenantsCount > 0 {
		slog.Info("lua_routing_initialized", "tenants_with_lua", luaTenantsCount, "component", "lua")
	}
}
