// Package lua provides an embedded Lua scripting engine for dynamic route registration.
// This replaces the external lua-stone service with an embedded gopher-lua engine
// that can register routes directly with the Chi router.
package lua

import (
	"context"
	"fmt"
	"log/slog"
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

// CompiledLuaScript represents a pre-compiled Lua script for faster execution
type CompiledLuaScript struct {
	Script      *lua.LFunction // Pre-compiled Lua function
	Content     string         // Original script content
	CompileTime time.Time      // When it was compiled
}

// Engine manages embedded Lua script execution and route registration
type Engine struct {
	scriptsDir      string
	scriptPaths     map[string]string             // script_tag -> file_path
	globalPaths     map[string]string             // global_script_tag -> file_path
	scriptCache     map[string]string             // script_tag -> cached_content
	globalCache     map[string]string             // global_script_tag -> cached_content
	compiledScripts map[string]*CompiledLuaScript // script_tag -> compiled_script
	compiledGlobals map[string]*CompiledLuaScript // global_script_tag -> compiled_script
	cacheMutex      sync.RWMutex                  // Protects cache access
	router          *chi.Mux                      // Chi router for dynamic route registration
	routeRegistry   *routing.LuaRouteRegistry     // Route registry for Lua integration
	statePool       *LuaStatePool                 // Pool of Lua states for thread safety
	middlewareCache *MiddlewareCache              // Cache for middleware logic
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
		slog.Error("lua_script_load_failed", "script", scriptTag, "error", err, "component", "lua_engine")
		return "", false
	}

	// Cache the loaded content
	e.cacheMutex.Lock()
	e.scriptCache[scriptTag] = string(content)
	e.cacheMutex.Unlock()

	// Pre-compile the script for performance
	if err := e.precompileScript(scriptTag, string(content)); err != nil {
		slog.Warn("script_precompilation_failed", "script", scriptTag, "error", err, "component", "lua_engine")
	}

	return string(content), true
}

// precompileScript stores content for faster execution (avoids file I/O per execution)
func (e *Engine) precompileScript(scriptTag, content string) error {
	// Check if already cached
	e.cacheMutex.RLock()
	if _, exists := e.compiledScripts[scriptTag]; exists {
		e.cacheMutex.RUnlock()
		return nil
	}
	e.cacheMutex.RUnlock()

	// Store script content and metadata (compilation happens per-state to avoid registry issues)
	e.cacheMutex.Lock()
	e.compiledScripts[scriptTag] = &CompiledLuaScript{
		Script:      nil, // Will compile per-state to avoid registry overflow
		Content:     content,
		CompileTime: time.Now(),
	}
	e.cacheMutex.Unlock()

	slog.Info("lua_script_cached", "script", scriptTag, "component", "lua_engine")
	return nil
}

// precompileGlobalScript stores content for faster execution (avoids file I/O per execution)
func (e *Engine) precompileGlobalScript(scriptTag, content string) error {
	// Check if already cached
	e.cacheMutex.RLock()
	if _, exists := e.compiledGlobals[scriptTag]; exists {
		e.cacheMutex.RUnlock()
		return nil
	}
	e.cacheMutex.RUnlock()

	// Store script content and metadata (compilation happens per-state to avoid registry issues)
	e.cacheMutex.Lock()
	e.compiledGlobals[scriptTag] = &CompiledLuaScript{
		Script:      nil, // Will compile per-state to avoid registry overflow
		Content:     content,
		CompileTime: time.Now(),
	}
	e.cacheMutex.Unlock()

	slog.Info("lua_global_script_cached", "script", scriptTag, "component", "lua_engine")
	return nil
}

// RouteRegistry returns the route registry for mounting tenant routes
func (e *Engine) RouteRegistry() *routing.LuaRouteRegistry {
	return e.routeRegistry
}

// NewEngine creates a new embedded Lua engine
func NewEngine(scriptsDir string, router *chi.Mux) *Engine {
	engine := &Engine{
		scriptsDir:      scriptsDir,
		scriptPaths:     make(map[string]string),
		globalPaths:     make(map[string]string),
		scriptCache:     make(map[string]string),
		globalCache:     make(map[string]string),
		compiledScripts: make(map[string]*CompiledLuaScript),
		compiledGlobals: make(map[string]*CompiledLuaScript),
		router:          router,
		middlewareCache: &MiddlewareCache{
			cache: make(map[string]*MiddlewareLogic),
		},
	}
	engine.routeRegistry = routing.NewLuaRouteRegistry(router, engine)

	// Keep state pool ONLY for runtime request handling (high-frequency operations)
	// Route registration now uses execute-and-discard pattern per lua_perf.md analysis
	engine.statePool = NewLuaStatePool(DefaultStatePoolSize, func() *lua.LState {
		L := lua.NewState(lua.Options{
			CallStackSize: LuaCallStackSize,
			RegistrySize:  LuaRegistrySize,
		})
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
		slog.Info("lua_log", "message", message, "component", "lua_script")
		return 0
	}))

	// Register the chi module so scripts can use require('chi')
	e.registerChiModule(L)
}

// loadScriptPaths discovers and maps script files without loading content
func (e *Engine) loadScriptPaths() {
	if _, err := os.Stat(e.scriptsDir); os.IsNotExist(err) {
		slog.Info("scripts_directory_creating",
			"directory", e.scriptsDir,
			"component", "lua_engine")
		if err := os.MkdirAll(e.scriptsDir, DefaultDirMode); err != nil {
			slog.Error("scripts_directory_create_failed",
				"directory", e.scriptsDir,
				"error", err,
				"component", "lua_engine")
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
			slog.Info("lua_global_script_discovered", "script", globalScriptName, "path", path, "component", "lua_engine")
		} else {
			e.scriptPaths[scriptName] = path
			slog.Info("lua_route_script_discovered", "script", scriptName, "path", path, "component", "lua_engine")
		}
		return nil
	})

	if err != nil {
		slog.Error("lua_scripts_walk_error", "error", err, "component", "lua_engine")
	}
}

// ExecuteRouteScript executes a cached Lua script that registers routes with Chi for a specific tenant
// This uses state pooling for optimal performance in both route registration and request handling
func (e *Engine) ExecuteRouteScript(scriptTag, tenantName string) error {
	// Ensure script is loaded and cached
	_, exists := e.GetScript(scriptTag)
	if !exists {
		return fmt.Errorf("no route script found for tag: %s", scriptTag)
	}

	// Get cached script content
	e.cacheMutex.RLock()
	cached, cachedExists := e.compiledScripts[scriptTag]
	e.cacheMutex.RUnlock()

	if !cachedExists {
		return fmt.Errorf("script not cached: %s", scriptTag)
	}

	// Use state pool for both route registration and request handling
	// Benchmarks show state pool is faster than execute-and-discard even for registration
	L := e.statePool.Get()
	defer e.statePool.Put(L)

	// Setup Lua environment with Chi bindings
	e.SetupChiBindings(L, scriptTag, tenantName)

	// Execute cached script content directly (no timeout for route registration)
	// Route registration should be fast and synchronous
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to error for graceful handling
			err := fmt.Errorf("panic during route registration: %v", r)
			slog.Error("lua_route_registration_panic", "script", scriptTag, "error", err, "component", "lua_engine")
		}
	}()

	// Use DoString with cached content (avoids file I/O but allows per-state compilation)
	err := L.DoString(cached.Content)
	if err != nil {
		return fmt.Errorf("lua script execution failed: %w", err)
	}
	return nil
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
		slog.Error("lua_global_script_load_failed", "script", scriptTag, "error", err, "component", "lua_engine")
		return "", false
	}

	// Cache the loaded content
	e.cacheMutex.Lock()
	e.globalCache[scriptTag] = string(content)
	e.cacheMutex.Unlock()

	// Pre-compile the global script for performance
	if err := e.precompileGlobalScript(scriptTag, string(content)); err != nil {
		slog.Warn("global_script_precompilation_failed", "script", scriptTag, "error", err, "component", "lua_engine")
	}

	return string(content), true
}

// ExecuteGlobalScripts executes all global scripts that apply to all tenants using cached content
func (e *Engine) ExecuteGlobalScripts() error {
	for globalScriptName := range e.globalPaths {
		// Ensure script is loaded and cached
		_, exists := e.getGlobalScript(globalScriptName)
		if !exists {
			slog.Error("lua_global_script_missing", "script", globalScriptName, "component", "lua_engine")
			continue
		}

		// Get cached global script content
		e.cacheMutex.RLock()
		cached, cachedExists := e.compiledGlobals[globalScriptName]
		e.cacheMutex.RUnlock()

		if !cachedExists {
			slog.Error("lua_global_script_not_cached", "script", globalScriptName, "component", "lua_engine")
			continue
		}

		// Use state pool for global script execution (better performance)
		L := e.statePool.Get()
		defer e.statePool.Put(L)

		// Setup Lua environment with Chi bindings for global scope
		e.SetupChiBindings(L, globalScriptName, "global")

		// Execute cached script content with timeout protection
		ctx, cancel := context.WithTimeout(context.Background(), MaxScriptExecutionTime)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic during global script execution: %v", r)
				}
			}()
			// Use DoString with cached content (avoids file I/O but allows per-state compilation)
			err := L.DoString(cached.Content)
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
	e.compiledScripts = make(map[string]*CompiledLuaScript)
	e.compiledGlobals = make(map[string]*CompiledLuaScript)
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

// EnableHotReload enables file watching for automatic script reloading
// Note: This is a placeholder for future fsnotify integration
func (e *Engine) EnableHotReload() error {
	slog.Info("hot_reload_placeholder",
		"message", "Hot reload support will be added with fsnotify integration",
		"component", "lua_engine")
	// TODO: Implement with fsnotify when dependency is added
	// This would watch e.scriptsDir and call e.ReloadScripts() on changes
	return nil
}
