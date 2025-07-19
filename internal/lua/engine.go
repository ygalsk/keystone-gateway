// Package lua provides an embedded Lua scripting engine for dynamic route registration.
// This replaces the external lua-stone service with an embedded gopher-lua engine
// that can register routes directly with the Chi router.
package lua

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/routing"
)

const (
	// MaxScriptExecutionTime limits how long a lua script can run
	MaxScriptExecutionTime = 5 * time.Second
	// MaxMemoryMB limits lua script memory usage (disabled for performance)
	MaxMemoryMB = 10
)

// Engine manages embedded Lua script execution and route registration
type Engine struct {
	scriptsDir    string
	scripts       map[string]string         // tenant_name -> script_content
	router        *chi.Mux                  // Chi router for dynamic route registration
	routeRegistry *routing.LuaRouteRegistry // Route registry for Lua integration
}

// RouteRegistration represents a route registered by Lua
type RouteRegistration struct {
	Method      string
	Pattern     string
	HandlerFunc func(w http.ResponseWriter, r *http.Request)
	Middleware  []func(http.Handler) http.Handler
}

// NewEngine creates a new embedded Lua engine
func NewEngine(scriptsDir string, router *chi.Mux) *Engine {
	engine := &Engine{
		scriptsDir:    scriptsDir,
		scripts:       make(map[string]string),
		router:        router,
		routeRegistry: routing.NewLuaRouteRegistry(router),
	}
	engine.loadScripts()
	return engine
}

// loadScripts loads all .lua files from the scripts directory
func (e *Engine) loadScripts() {
	if _, err := os.Stat(e.scriptsDir); os.IsNotExist(err) {
		log.Printf("Scripts directory %s does not exist, creating...", e.scriptsDir)
		os.MkdirAll(e.scriptsDir, 0755)
		e.createExampleRouteScript()
		return
	}

	err := filepath.Walk(e.scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".lua") {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Failed to read script %s: %v", path, err)
			return nil
		}

		tenantName := strings.TrimSuffix(filepath.Base(path), ".lua")
		e.scripts[tenantName] = string(content)
		log.Printf("Loaded route script for tenant: %s", tenantName)
		return nil
	})

	if err != nil {
		log.Printf("Error walking scripts directory: %v", err)
	}
}

// ExecuteRouteScript executes a Lua script that registers routes with Chi
func (e *Engine) ExecuteRouteScript(tenantName string) error {
	script, exists := e.scripts[tenantName]
	if !exists {
		return fmt.Errorf("no route script found for tenant: %s", tenantName)
	}

	// Create isolated Lua state
	L := lua.NewState(lua.Options{
		CallStackSize: 120,
		RegistrySize:  120 * 20,
	})
	defer L.Close()

	// Setup timeout
	ctx := make(chan bool, 1)
	go func() {
		time.Sleep(MaxScriptExecutionTime)
		ctx <- true
	}()

	// Setup Lua environment with Chi bindings
	e.setupChiBindings(L, tenantName)

	// Execute script with timeout protection
	done := make(chan error, 1)
	go func() {
		err := L.DoString(script)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("script execution failed: %w", err)
		}
		return nil
	case <-ctx:
		return fmt.Errorf("script execution timeout after %v", MaxScriptExecutionTime)
	}
}

// ReloadScripts reloads all Lua scripts from disk
func (e *Engine) ReloadScripts() error {
	e.scripts = make(map[string]string)
	e.loadScripts()
	return nil
}

// GetLoadedScripts returns list of loaded script names
func (e *Engine) GetLoadedScripts() []string {
	scripts := make([]string, 0, len(e.scripts))
	for name := range e.scripts {
		scripts = append(scripts, name)
	}
	return scripts
}

// createExampleRouteScript creates an example Lua script for route registration
func (e *Engine) createExampleRouteScript() {
	exampleScript := `-- Example Dynamic Route Registration Script
-- This script demonstrates how to register custom routes with Chi router

-- Register a simple GET route
chi_route("GET", "/api/v1/health", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"status":"healthy","service":"example"}')
end)

-- Register a route with path parameters
chi_route("GET", "/api/v1/users/{id}", function(w, r)
    local user_id = chi_param(r, "id")
    w:header("Content-Type", "application/json")
    w:write('{"user_id":"' .. user_id .. '","name":"User ' .. user_id .. '"}')
end)

-- Register middleware for all routes under /api/v1/
chi_middleware("/api/v1/*", function(next)
    return function(w, r)
        -- Add custom headers
        w:header("X-API-Version", "v1")
        w:header("X-Powered-By", "Lua-Keystone")
        next(w, r)
    end
end)

-- Register a route group with shared middleware
chi_group("/api/v1/admin", function()
    -- Auth middleware for admin routes
    chi_middleware("/*", function(next)
        return function(w, r)
            local auth_header = r:header("Authorization")
            if not auth_header or auth_header ~= "Bearer admin-token" then
                w:status(401)
                w:write("Unauthorized")
                return
            end
            next(w, r)
        end
    end)
    
    -- Admin-only routes
    chi_route("GET", "/stats", function(w, r)
        w:header("Content-Type", "application/json") 
        w:write('{"requests":1234,"uptime":"24h"}')
    end)
    
    chi_route("POST", "/reload", function(w, r)
        -- Trigger script reload (this would be implemented)
        w:write("Scripts reloaded")
    end)
end)

log("Route registration complete for example tenant")`

	examplePath := filepath.Join(e.scriptsDir, "example.lua")
	if err := os.WriteFile(examplePath, []byte(exampleScript), 0644); err != nil {
		log.Printf("Failed to create example route script: %v", err)
	} else {
		log.Printf("Created example route script at: %s", examplePath)
	}
}
