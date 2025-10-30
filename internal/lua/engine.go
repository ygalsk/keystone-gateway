package lua

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

const (
	MaxScriptExecutionTime = 10 * time.Second
	DefaultStatePoolSize   = 10
	LuaCallStackSize       = 120
	LuaRegistrySize        = 120 * 20
	DefaultDirMode         = 0755
)

// luaResponseWriter wraps http.ResponseWriter for Lua integration
type luaResponseWriter struct {
	w http.ResponseWriter
}

type Engine struct {
	scriptsDir    string
	scriptPaths   map[string]string
	globalPaths   map[string]string
	compiler      *ScriptCompiler
	router        *chi.Mux
	routeRegistry *routing.LuaRouteRegistry
	statePool     *LuaStatePool
	config        *config.Config
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

// ExecuteScriptHandler executes a Lua script handler function for HTTP requests
func (e *Engine) ExecuteScriptHandler(scriptKey, functionName string, w http.ResponseWriter, r *http.Request) error {
	// Get compiled script from unified compiler
	compiled, exists := e.compiler.GetScript(scriptKey)
	if !exists {
		return fmt.Errorf("compiled script not found: %s", scriptKey)
	}

	L := e.statePool.Get()
	defer e.statePool.Put(L)
	e.SetupChiBindings(L, e.router)

	// Use bytecode execution
	if err := ExecuteWithBytecode(L, compiled); err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	// Get the handler function and call it
	handlerFunc := L.GetGlobal(functionName)
	if handlerFunc.Type() != lua.LTFunction {
		return fmt.Errorf("handler function not found: %s", functionName)
	}

	// Create request/response tables and call the handler
	respWriter := &luaResponseWriter{w: w}
	respTable := createLuaResponse(L, respWriter)
	reqTable := createLuaRequest(L, r)

	return L.CallByParam(lua.P{
		Fn:      handlerFunc.(*lua.LFunction),
		NRet:    0,
		Protect: true,
	}, reqTable, respTable)
}

// GetScript returns script content (backward compatibility)
func (e *Engine) GetScript(scriptTag string) (string, bool) {
	if compiled, ok := e.loadScript(scriptTag, false); ok {
		return compiled.Content, true
	}
	return "", false
}

// CompileScript compiles a script to bytecode (public method for LuaHandler)
func (e *Engine) CompileScript(scriptKey, content string) error {
	_, err := e.compiler.CompileScript(scriptKey, content)
	return err
}

// createLuaRequest creates a Lua table representing an HTTP request
func createLuaRequest(L *lua.LState, r *http.Request) *lua.LTable {
	reqTable := L.NewTable()

	// Basic request info
	reqTable.RawSetString("method", lua.LString(r.Method))
	reqTable.RawSetString("url", lua.LString(r.URL.String()))
	reqTable.RawSetString("path", lua.LString(r.URL.Path))
	reqTable.RawSetString("host", lua.LString(r.Host))

	// Headers
	headersTable := L.NewTable()
	for key, values := range r.Header {
		if len(values) > 0 {
			headersTable.RawSetString(key, lua.LString(values[0]))
		}
	}
	reqTable.RawSetString("headers", headersTable)

	// URL parameters from Chi router
	paramsTable := L.NewTable()
	if r.Context() != nil {
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			for i, key := range rctx.URLParams.Keys {
				if i < len(rctx.URLParams.Values) {
					paramsTable.RawSetString(key, lua.LString(rctx.URLParams.Values[i]))
				}
			}
		}
	}
	reqTable.RawSetString("params", paramsTable)

	// Query parameters
	queryTable := L.NewTable()
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryTable.RawSetString(key, lua.LString(values[0]))
		}
	}
	reqTable.RawSetString("query", queryTable)

	// Body content
	var bodyContent string
	if r.Body != nil {
		if body, err := io.ReadAll(r.Body); err == nil {
			bodyContent = string(body)
		}
	}

	// Add helper methods
	reqTable.RawSetString("body", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(bodyContent))
		return 1
	}))

	reqTable.RawSetString("header", L.NewFunction(func(L *lua.LState) int {
		headerName := L.ToString(1)
		headerValue := r.Header.Get(headerName)
		L.Push(lua.LString(headerValue))
		return 1
	}))

	return reqTable
}

// createLuaResponse creates a Lua table representing an HTTP response
func createLuaResponse(L *lua.LState, w *luaResponseWriter) *lua.LTable {
	respTable := L.NewTable()

	writeFunc := L.NewFunction(func(L *lua.LState) int {
		content := L.ToString(1)
		if _, err := w.w.Write([]byte(content)); err != nil {
			slog.Error("lua_response_write_failed", "error", err)
		}
		return 0
	})

	headerFunc := L.NewFunction(func(L *lua.LState) int {
		key := L.ToString(1)
		value := L.ToString(2)
		w.w.Header().Set(key, value)
		return 0
	})

	statusFunc := L.NewFunction(func(L *lua.LState) int {
		statusCode := L.ToInt(1)
		w.w.WriteHeader(statusCode)
		return 0
	})

	jsonFunc := L.NewFunction(func(L *lua.LState) int {
		jsonContent := L.ToString(1)
		w.w.Header().Set("Content-Type", "application/json")
		if _, err := w.w.Write([]byte(jsonContent)); err != nil {
			slog.Error("lua_json_response_failed", "error", err)
		}
		return 0
	})

	respTable.RawSetString("write", writeFunc)
	respTable.RawSetString("header", headerFunc)
	respTable.RawSetString("status", statusFunc)
	respTable.RawSetString("json", jsonFunc)

	return respTable
}

