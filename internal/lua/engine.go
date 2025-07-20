// Package lua provides an embedded Lua scripting engine for dynamic route registration.
// This replaces the external lua-stone service with an embedded gopher-lua engine
// that can register routes directly with the Chi router.
package lua

import (
	"context"
	"fmt"
	"log"
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
	// DefaultStatePoolSize is the default number of Lua states in the pool
	DefaultStatePoolSize = 10
	// LuaCallStackSize sets the call stack size for Lua states
	LuaCallStackSize = 120
	// LuaRegistrySize sets the registry size for Lua states
	LuaRegistrySize = 120 * 20
	// DefaultDirMode is the default permission for created directories
	DefaultDirMode = 0755
	// DefaultFileMode is the default permission for created files
	DefaultFileMode = 0644
)

// Engine manages embedded Lua script execution and route registration
type Engine struct {
	scriptsDir    string
	scripts       map[string]string         // script_tag -> script_content
	router        *chi.Mux                  // Chi router for dynamic route registration
	routeRegistry *routing.LuaRouteRegistry // Route registry for Lua integration
	statePool     *LuaStatePool             // Pool of Lua states for thread safety
}

// GetScript returns the script content for a given scriptTag
func (e *Engine) GetScript(scriptTag string) (string, bool) {
	script, ok := e.scripts[scriptTag]
	return script, ok
}

// RouteRegistry returns the route registry for mounting tenant routes
func (e *Engine) RouteRegistry() *routing.LuaRouteRegistry {
	return e.routeRegistry
}

// NewEngine creates a new embedded Lua engine
func NewEngine(scriptsDir string, router *chi.Mux) *Engine {
	engine := &Engine{
		scriptsDir: scriptsDir,
		scripts:    make(map[string]string),
		router:     router,
	}
	engine.routeRegistry = routing.NewLuaRouteRegistry(router, engine)

	// Create Lua state pool for thread-safe request handling - prevents segfaults
	engine.statePool = NewLuaStatePool(DefaultStatePoolSize, func() *lua.LState {
		L := lua.NewState(lua.Options{
			CallStackSize: LuaCallStackSize,
			RegistrySize:  LuaRegistrySize,
		})
		// Setup basic Lua bindings for each state
		engine.setupBasicBindings(L)
		return L
	})

	engine.loadScripts()
	return engine
}

// setupBasicBindings sets up basic Lua functions that all states need
func (e *Engine) setupBasicBindings(L *lua.LState) {
	// Add basic logging function
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		message := L.ToString(1)
		log.Printf("[Lua] %s", message)
		return 0
	}))
}

// loadScripts loads all .lua files from the scripts directory
func (e *Engine) loadScripts() {
	if _, err := os.Stat(e.scriptsDir); os.IsNotExist(err) {
		log.Printf("Scripts directory %s does not exist, creating...", e.scriptsDir)
		os.MkdirAll(e.scriptsDir, DefaultDirMode)
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
		log.Printf("Loaded route script: %s", tenantName)
		return nil
	})

	if err != nil {
		log.Printf("Error walking scripts directory: %v", err)
	}
}

// ExecuteRouteScript executes a Lua script that registers routes with Chi for a specific tenant
// This version prevents segfaults by using proper state management and isolation
func (e *Engine) ExecuteRouteScript(scriptTag, tenantName string) error {
	script, exists := e.scripts[scriptTag]
	if !exists {
		return fmt.Errorf("no route script found for tag: %s", scriptTag)
	}

	// Create isolated Lua state - this prevents shared state segfaults
	L := lua.NewState(lua.Options{
		CallStackSize: LuaCallStackSize,
		RegistrySize:  LuaRegistrySize,
	})
	defer L.Close()

	// Setup basic bindings first
	e.setupBasicBindings(L)

	// Setup Lua environment with Chi bindings
	e.SetupChiBindings(L, scriptTag, tenantName)

	// Execute script with timeout protection using context
	ctx, cancel := context.WithTimeout(context.Background(), MaxScriptExecutionTime)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic during script execution: %v", r)
			}
		}()
		err := L.DoString(script)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("Lua script execution failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("Lua script execution timeout after %v", MaxScriptExecutionTime)
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

// GetScriptMap returns the scripts map for testing purposes
func (e *Engine) GetScriptMap() map[string]string {
	return e.scripts
}
