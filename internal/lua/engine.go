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
	MaxScriptExecutionTime = 5 * time.Second
	DefaultStatePoolSize   = 10
	LuaCallStackSize       = 120
	LuaRegistrySize        = 120 * 20
	DefaultDirMode         = 0755
)

type CompiledLuaScript struct {
	Script      *lua.LFunction
	Content     string
	CompileTime time.Time
}

type Engine struct {
	scriptsDir      string
	scriptPaths     map[string]string
	globalPaths     map[string]string
	scriptCache     map[string]string
	globalCache     map[string]string
	compiledScripts map[string]*CompiledLuaScript
	compiledGlobals map[string]*CompiledLuaScript
	cacheMutex      sync.RWMutex
	router          *chi.Mux
	routeRegistry   *routing.LuaRouteRegistry
	statePool       *LuaStatePool
}

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
	}
	engine.routeRegistry = routing.NewLuaRouteRegistry(router)

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
	go engine.startCacheCleanup()
	return engine
}

func (e *Engine) GetScript(scriptTag string) (string, bool) {
	e.cacheMutex.RLock()
	if s, ok := e.scriptCache[scriptTag]; ok {
		e.cacheMutex.RUnlock()
		return s, true
	}
	e.cacheMutex.RUnlock()

	path, exists := e.scriptPaths[scriptTag]
	if !exists {
		return "", false
	}

	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("lua_script_load_failed", "script", scriptTag, "error", err)
		return "", false
	}

	e.cacheMutex.Lock()
	e.scriptCache[scriptTag] = string(content)
	e.cacheMutex.Unlock()
	e.precompileScript(scriptTag, string(content))
	return string(content), true
}

func (e *Engine) precompileScript(scriptTag, content string) {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	if _, exists := e.compiledScripts[scriptTag]; !exists {
		e.compiledScripts[scriptTag] = &CompiledLuaScript{Content: content, CompileTime: time.Now()}
	}
}

func (e *Engine) precompileGlobalScript(scriptTag, content string) {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	if _, exists := e.compiledGlobals[scriptTag]; !exists {
		e.compiledGlobals[scriptTag] = &CompiledLuaScript{Content: content, CompileTime: time.Now()}
	}
}

func (e *Engine) ExecuteRouteScript(scriptTag string) error {
	_, ok := e.GetScript(scriptTag)
	if !ok {
		return fmt.Errorf("route script not found: %s", scriptTag)
	}

	e.cacheMutex.RLock()
	cached := e.compiledScripts[scriptTag]
	e.cacheMutex.RUnlock()

	L := e.statePool.Get()
	defer e.statePool.Put(L)
	e.SetupChiBindings(L, e.router)

	if err := L.DoString(cached.Content); err != nil {
		return fmt.Errorf("lua script execution failed: %w", err)
	}

	return nil
}

func (e *Engine) getGlobalScript(scriptTag string) (string, bool) {
	e.cacheMutex.RLock()
	if s, ok := e.globalCache[scriptTag]; ok {
		e.cacheMutex.RUnlock()
		return s, true
	}
	e.cacheMutex.RUnlock()

	path, exists := e.globalPaths[scriptTag]
	if !exists {
		return "", false
	}

	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("lua_global_script_load_failed", "script", scriptTag, "error", err)
		return "", false
	}

	e.cacheMutex.Lock()
	e.globalCache[scriptTag] = string(content)
	e.cacheMutex.Unlock()
	e.precompileGlobalScript(scriptTag, string(content))
	return string(content), true
}

func (e *Engine) ExecuteGlobalScripts() error {
	for name := range e.globalPaths {
		_, ok := e.getGlobalScript(name)
		if !ok {
			continue
		}

		e.cacheMutex.RLock()
		cached := e.compiledGlobals[name]
		e.cacheMutex.RUnlock()

		L := e.statePool.Get()
		defer e.statePool.Put(L)
		e.SetupChiBindings(L, e.router)

		ctx, cancel := context.WithTimeout(context.Background(), MaxScriptExecutionTime)
		defer cancel()

		done := make(chan error, 1)
		go func() { done <- L.DoString(cached.Content) }()

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
	e.cacheMutex.Lock()
	e.scriptCache, e.globalCache = make(map[string]string), make(map[string]string)
	e.compiledScripts, e.compiledGlobals = make(map[string]*CompiledLuaScript), make(map[string]*CompiledLuaScript)
	e.cacheMutex.Unlock()
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

func (e *Engine) startCacheCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		// placeholder
	}
}
