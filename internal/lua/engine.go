package lua

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/config"
)

const (
	MaxScriptExecutionTime = 10 * time.Second
	DefaultStatePoolSize   = 10
	LuaCallStackSize       = 120
	LuaRegistrySize        = 120 * 20
	DefaultDirMode         = 0755
)

type Engine struct {
	scriptsDir  string
	scriptPaths map[string]string
	globalPaths map[string]string
	compiler    *ScriptCompiler
	router      *chi.Mux
	statePool   *LuaStatePool
	config      *config.Config
}

func NewEngine(scriptsDir string, router *chi.Mux, cfg *config.Config) *Engine {
	engine := &Engine{
		scriptsDir:  scriptsDir,
		scriptPaths: make(map[string]string),
		globalPaths: make(map[string]string),
		compiler:    NewScriptCompiler(150), // Unified cache for all scripts
		router:      router,
		config:      cfg,
	}

	engine.statePool = NewLuaStatePool(DefaultStatePoolSize, func() *lua.LState {
		L := lua.NewState(lua.Options{
			CallStackSize: LuaCallStackSize,
			RegistrySize:  LuaRegistrySize,
		})
		// Bind Chi directly from your chi-bindings.go
		engine.SetupChiBindings(L, router)
		L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
			slog.Info("lua_log", "message", L.ToString(1))
			return 0
		}))
		return L
	})

	engine.loadScriptPaths()
	return engine
}

// loadScript is the unified script loader - no more duplication
func (e *Engine) loadScript(scriptTag string, isGlobal bool) (*CompiledScript, bool) {
	cacheKey := scriptTag
	if isGlobal {
		cacheKey = "global-" + scriptTag
	}

	// Check cache first
	if compiled, exists := e.compiler.GetScript(cacheKey); exists {
		return compiled, true
	}

	// Get file path
	var path string
	var exists bool
	if isGlobal {
		path, exists = e.globalPaths[scriptTag]
	} else {
		path, exists = e.scriptPaths[scriptTag]
	}
	if !exists {
		return nil, false
	}

	// Load and compile
	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("lua_script_load_failed", "script", cacheKey, "error", err)
		return nil, false
	}

	compiled, err := e.compiler.CompileScript(cacheKey, string(content))
	if err != nil {
		slog.Error("lua_script_compile_failed", "script", cacheKey, "error", err)
		return nil, false
	}

	return compiled, true
}

func (e *Engine) ExecuteRouteScript(scriptTag string) error {
	compiled, ok := e.loadScript(scriptTag, false)
	if !ok {
		return fmt.Errorf("route script not found: %s", scriptTag)
	}

	L := e.statePool.Get()
	defer e.statePool.Put(L)
	e.SetupChiBindings(L, e.router)

	if err := ExecuteWithBytecode(L, compiled); err != nil {
		return fmt.Errorf("lua script execution failed: %w", err)
	}
	return nil
}

func (e *Engine) ExecuteGlobalScripts() error {
	for name := range e.globalPaths {
		compiled, ok := e.loadScript(name, true)
		if !ok {
			slog.Warn("global_script_not_found", "script", name)
			continue
		}

		L := e.statePool.Get()
		defer e.statePool.Put(L)
		e.SetupChiBindings(L, e.router)

		ctx, cancel := context.WithTimeout(context.Background(), MaxScriptExecutionTime)
		defer cancel()

		done := make(chan error, 1)
		go func() { done <- ExecuteWithBytecode(L, compiled) }()

		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("global Lua script '%s' failed: %w", name, err)
			}
		case <-ctx.Done():
			return fmt.Errorf("global Lua script '%s' timed out", name)
		}
	}
	return nil
}

func (e *Engine) loadScriptPaths() {
	if _, err := os.Stat(e.scriptsDir); os.IsNotExist(err) {
		_ = os.MkdirAll(e.scriptsDir, DefaultDirMode)
		return
	}

	_ = filepath.Walk(e.scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".lua") {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".lua")
		if strings.HasPrefix(name, "global-") {
			e.globalPaths[strings.TrimPrefix(name, "global-")] = path
		} else {
			e.scriptPaths[name] = path
		}
		return nil
	})
}

func (e *Engine) ReloadScripts() {
	// Clear unified cache
	e.compiler.ClearCache()

	e.scriptPaths, e.globalPaths = make(map[string]string), make(map[string]string)
	e.loadScriptPaths()
}

func (e *Engine) GetLoadedScripts() []string {
	scripts := make([]string, 0, len(e.scriptPaths))
	for name := range e.scriptPaths {
		scripts = append(scripts, name)
	}
	return scripts
}

// GetScript returns script content (backward compatibility)
func (e *Engine) GetScript(scriptTag string) (string, bool) {
	if compiled, ok := e.loadScript(scriptTag, false); ok {
		return compiled.Content, true
	}
	return "", false
}
