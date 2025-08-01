// Package luaengine provides an embedded Lua scripting engine for dynamic route registration.
// This replaces the external lua-stone service with an embedded gopher-lua engine
// that can register routes directly with the Chi router.
package luaengine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	scriptsDir      string
	scriptPaths     map[string]string         // script_tag -> file_path
	globalPaths     map[string]string         // global_script_tag -> file_path
	scriptCache     map[string]string         // script_tag -> cached_content
	globalCache     map[string]string         // global_script_tag -> cached_content
	cacheMutex      sync.RWMutex              // Protects cache access
	router          *chi.Mux                  // Chi router for dynamic route registration
	routeRegistry   *routing.LuaRouteRegistry // Route registry for Lua integration
	statePool       *LuaStatePool             // Pool of Lua states for thread safety
	middlewareCache *MiddlewareCache          // Cache for middleware logic
}

// GetScript returns the script content for a given scriptTag, loading it if necessary
func (e *Engine) GetScript(scriptTag string) (string, bool) {
	e.cacheMutex.RLock()
	if script, cached := e.scriptCache[scriptTag]; cached {
		e.cacheMutex.RUnlock()
		return script, true
	}
	e.cacheMutex.RUnlock()

	// Check if we have the path for this script
	path, exists := e.scriptPaths[scriptTag]
	if !exists {
		return "", false
	}

	// Load the script content
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to load script %s: %v", scriptTag, err)
		return "", false
	}

	// Cache the loaded content
	e.cacheMutex.Lock()
	e.scriptCache[scriptTag] = string(content)
	e.cacheMutex.Unlock()

	return string(content), true
}

// RouteRegistry returns the route registry for mounting tenant routes
func (e *Engine) RouteRegistry() *routing.LuaRouteRegistry {
	return e.routeRegistry
}

// NewEngine creates a new embedded Lua engine
func NewEngine(scriptsDir string, router *chi.Mux) *Engine {
	engine := &Engine{
		scriptsDir:  scriptsDir,
		scriptPaths: make(map[string]string),
		globalPaths: make(map[string]string),
		scriptCache: make(map[string]string),
		globalCache: make(map[string]string),
		router:      router,
		middlewareCache: &MiddlewareCache{
			cache: make(map[string]*MiddlewareLogic),
		},
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

	engine.loadScriptPaths()
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

	// Register the chi module so scripts can use require('chi')
	e.registerChiModule(L)
}

// loadScriptPaths discovers and maps script files without loading content
func (e *Engine) loadScriptPaths() {
	if _, err := os.Stat(e.scriptsDir); os.IsNotExist(err) {
		log.Printf("Scripts directory %s does not exist, creating...", e.scriptsDir)
		if err := os.MkdirAll(e.scriptsDir, DefaultDirMode); err != nil {
			log.Printf("Failed to create scripts directory %s: %v", e.scriptsDir, err)
		}
		return
	}

	err := filepath.Walk(e.scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".lua") {
			return err
		}

		scriptName := strings.TrimSuffix(filepath.Base(path), ".lua")

		// Check if this is a global script (global-*.lua)
		if strings.HasPrefix(scriptName, "global-") {
			globalScriptName := strings.TrimPrefix(scriptName, "global-")
			e.globalPaths[globalScriptName] = path
			log.Printf("Discovered global script: %s at %s", globalScriptName, path)
		} else {
			e.scriptPaths[scriptName] = path
			log.Printf("Discovered route script: %s at %s", scriptName, path)
		}
		return nil
	})

	if err != nil {
		log.Printf("Error walking scripts directory: %v", err)
	}
}

// ExecuteRouteScript executes a Lua script that registers routes with Chi for a specific tenant
// This version prevents segfaults by using proper state management and isolation
func (e *Engine) ExecuteRouteScript(scriptTag, tenantName string) error {
	script, exists := e.GetScript(scriptTag)
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
			return fmt.Errorf("lua script execution failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("lua script execution timeout after %v", MaxScriptExecutionTime)
	}
}

// getGlobalScript loads a global script by name
func (e *Engine) getGlobalScript(scriptTag string) (string, bool) {
	e.cacheMutex.RLock()
	if script, cached := e.globalCache[scriptTag]; cached {
		e.cacheMutex.RUnlock()
		return script, true
	}
	e.cacheMutex.RUnlock()

	// Check if we have the path for this global script
	path, exists := e.globalPaths[scriptTag]
	if !exists {
		return "", false
	}

	// Load the script content
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to load global script %s: %v", scriptTag, err)
		return "", false
	}

	// Cache the loaded content
	e.cacheMutex.Lock()
	e.globalCache[scriptTag] = string(content)
	e.cacheMutex.Unlock()

	return string(content), true
}

// ExecuteGlobalScripts executes all global scripts that apply to all tenants
func (e *Engine) ExecuteGlobalScripts() error {
	for globalScriptName := range e.globalPaths {
		script, exists := e.getGlobalScript(globalScriptName)
		if !exists {
			log.Printf("Failed to load global script: %s", globalScriptName)
			continue
		}
		// Create isolated Lua state - this prevents shared state segfaults
		L := lua.NewState(lua.Options{
			CallStackSize: LuaCallStackSize,
			RegistrySize:  LuaRegistrySize,
		})
		defer L.Close()

		// Setup basic bindings first
		e.setupBasicBindings(L)

		// Setup Lua environment with Chi bindings for global scope
		e.SetupChiBindings(L, globalScriptName, "global")

		// Execute script with timeout protection using context
		ctx, cancel := context.WithTimeout(context.Background(), MaxScriptExecutionTime)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic during global script execution: %v", r)
				}
			}()
			err := L.DoString(script)
			done <- err
		}()

		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("global Lua script '%s' execution failed: %w", globalScriptName, err)
			}
		case <-ctx.Done():
			return fmt.Errorf("global Lua script '%s' execution timeout after %v", globalScriptName, MaxScriptExecutionTime)
		}
	}
	return nil
}

// ReloadScripts clears the cache and reloads script paths from disk
func (e *Engine) ReloadScripts() error {
	e.cacheMutex.Lock()
	e.scriptCache = make(map[string]string)
	e.globalCache = make(map[string]string)
	e.cacheMutex.Unlock()
	e.scriptPaths = make(map[string]string)
	e.globalPaths = make(map[string]string)
	e.loadScriptPaths()
	return nil
}

// GetLoadedScripts returns list of available script names
func (e *Engine) GetLoadedScripts() []string {
	scripts := make([]string, 0, len(e.scriptPaths))
	for name := range e.scriptPaths {
		scripts = append(scripts, name)
	}
	return scripts
}

// GetScriptMap returns the script paths map for testing purposes
func (e *Engine) GetScriptMap() map[string]string {
	return e.scriptPaths
}

// registerChiModule registers the chi module in the Lua state so scripts can use require('chi')
func (e *Engine) registerChiModule(L *lua.LState) {
	// Create a chi module table
	chiModule := L.NewTable()

	// Add NewRouter function that returns a router table with methods
	newRouterFunc := L.NewFunction(func(L *lua.LState) int {
		routerTable := L.NewTable()

		// Add router methods (Use, Get, Post, Put, Delete, Route, etc.)
		routerTable.RawSetString("Use", L.NewFunction(func(L *lua.LState) int {
			// For now, return 0 as these are placeholders for the test scripts
			return 0
		}))

		routerTable.RawSetString("Get", L.NewFunction(func(L *lua.LState) int {
			return 0
		}))

		routerTable.RawSetString("Post", L.NewFunction(func(L *lua.LState) int {
			return 0
		}))

		routerTable.RawSetString("Put", L.NewFunction(func(L *lua.LState) int {
			return 0
		}))

		routerTable.RawSetString("Delete", L.NewFunction(func(L *lua.LState) int {
			return 0
		}))

		routerTable.RawSetString("Route", L.NewFunction(func(L *lua.LState) int {
			return 0
		}))

		L.Push(routerTable)
		return 1
	})

	chiModule.RawSetString("NewRouter", newRouterFunc)

	// Register the chi module so it can be loaded with require('chi')
	L.PreloadModule("chi", func(L *lua.LState) int {
		L.Push(chiModule)
		return 1
	})
}
