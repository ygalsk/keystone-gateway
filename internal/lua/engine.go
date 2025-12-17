// Package lua provides LuaJIT scripting engine with golua bindings.
// This is a deep module: simple interface, complex implementation (state pooling, bytecode compilation, etc.)
package lua

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	lua "github.com/aarzilli/golua/lua"
	"github.com/go-chi/chi/v5"
)

const (
	DefaultStatePoolSize = 10
	MaxBodySize          = 10 << 20 // 10MB
)

// Engine provides Lua scripting capabilities for the gateway.
// Deep module: Simple interface (3 methods), complex implementation hidden.
type Engine struct {
	scriptsDir    string
	statePool     *LuaStatePool
	poolSize      int
	modulePaths   []string
	moduleCPaths  []string
	globalScripts []string // Scripts to load into every state
}

// NewEngine creates a new Lua engine with LuaJIT support and LuaRocks compatibility.
// scriptsDir: directory containing .lua scripts
// poolSize: number of Lua states to pre-allocate (0 = default)
// modulePaths: Lua module search paths (package.path)
// moduleCPaths: C module search paths (package.cpath)
func NewEngine(scriptsDir string, poolSize int, modulePaths []string, moduleCPaths []string) *Engine {
	if poolSize == 0 {
		poolSize = DefaultStatePoolSize
	}

	engine := &Engine{
		scriptsDir:    scriptsDir,
		poolSize:      poolSize,
		modulePaths:   modulePaths,
		moduleCPaths:  moduleCPaths,
		globalScripts: []string{}, // Will be set by ExecuteGlobalScripts
	}

	// State pool will be created after global scripts are set
	engine.statePool = nil

	return engine
}

// Close shuts down the engine and cleans up all Lua states
func (e *Engine) Close() {
	if e.statePool != nil {
		e.statePool.Close()
	}
}

// Stats returns current pool statistics
func (e *Engine) Stats() PoolStats {
	if e.statePool == nil {
		return PoolStats{}
	}
	return e.statePool.Stats()
}

// initStatePool creates the state pool with global scripts loaded into each state
func (e *Engine) initStatePool() {
	if e.statePool != nil {
		e.statePool.Close() // Close old pool if exists
	}

	e.statePool = NewLuaStatePool(e.poolSize, func() *lua.State {
		L := lua.NewState()
		L.OpenLibs()

		// Restore pcall for LuaRocks compatibility
		RestorePCall(L)

		// Setup module paths for LuaRocks
		e.setupModulePaths(L)

		// Register Go primitives (log, http_get, http_post)
		e.registerPrimitives(L)

		// Load global scripts into this state
		for _, scriptName := range e.globalScripts {
			scriptPath := filepath.Join(e.scriptsDir, scriptName+".lua")
			if err := L.DoFile(scriptPath); err != nil {
				slog.Error("failed_to_load_global_script_in_state",
					"script", scriptName,
					"error", err,
					"component", "lua")
			}
		}

		return L
	})
}

// ExecuteGlobalScripts runs initialization scripts once at startup.
// These scripts define global functions, load libraries, set up globals.
func (e *Engine) ExecuteGlobalScripts(scriptNames []string) error {
	if len(scriptNames) == 0 {
		return nil
	}

	// Validate all scripts exist before initializing state pool
	for _, scriptName := range scriptNames {
		scriptPath := filepath.Join(e.scriptsDir, scriptName+".lua")
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			return fmt.Errorf("global script not found: %s", scriptPath)
		}
	}

	// Store script names for state pool factory
	e.globalScripts = scriptNames

	// Initialize state pool - each state will have global scripts loaded
	e.initStatePool()

	slog.Info("lua_state_pool_initialized", "scripts", scriptNames, "pool_size", e.poolSize, "component", "lua")
	return nil
}

// ExecuteHandler executes a Lua handler function for an HTTP request.
// Handler signature: function(req) return {status=200, body="...", headers={}} end
func (e *Engine) ExecuteHandler(handlerName string, w http.ResponseWriter, r *http.Request) error {
	L := e.statePool.Get()
	defer e.statePool.Put(L)

	// Get handler function from global scope
	L.GetGlobal(handlerName)
	if L.IsNil(-1) {
		L.Pop(1)
		return fmt.Errorf("handler not found: %s", handlerName)
	}

	if !L.IsFunction(-1) {
		L.Pop(1)
		return fmt.Errorf("handler is not a function: %s", handlerName)
	}

	// Push request table as argument
	if err := e.pushRequestTable(L, r); err != nil {
		L.Pop(1) // Pop handler function
		return fmt.Errorf("failed to create request table: %w", err)
	}

	// Call handler(req) -> returns response table
	if err := L.Call(1, 1); err != nil {
		return fmt.Errorf("handler execution failed: %w", err)
	}

	// Response table is now on top of stack
	if err := e.writeResponseFromTable(L, w); err != nil {
		L.Pop(1) // Clean up response table
		return fmt.Errorf("failed to write response: %w", err)
	}

	L.Pop(1) // Pop response table
	return nil
}

// ExecuteMiddleware executes a Lua middleware function.
// Middleware signature: function(req, next) return {status=...} or nil (if next called) end
func (e *Engine) ExecuteMiddleware(middlewareName string, w http.ResponseWriter, r *http.Request, next http.Handler) error {
	L := e.statePool.Get()
	defer e.statePool.Put(L)

	// Get middleware function
	L.GetGlobal(middlewareName)
	if L.IsNil(-1) {
		L.Pop(1)
		return fmt.Errorf("middleware not found: %s", middlewareName)
	}

	if !L.IsFunction(-1) {
		L.Pop(1)
		return fmt.Errorf("middleware is not a function: %s", middlewareName)
	}

	// Push request table
	if err := e.pushRequestTable(L, r); err != nil {
		L.Pop(1) // Pop middleware function
		return fmt.Errorf("failed to create request table: %w", err)
	}

	// Create and push 'next' function
	nextCalled := false
	L.PushGoFunction(func(L *lua.State) int {
		nextCalled = true
		return 0
	})

	// Call middleware(req, next) -> returns nil or response table
	if err := L.Call(2, 1); err != nil {
		return fmt.Errorf("middleware execution failed: %w", err)
	}

	// Check if middleware returned a response
	if !L.IsNil(-1) {
		// Middleware returned response, write it
		if err := e.writeResponseFromTable(L, w); err != nil {
			L.Pop(1)
			return fmt.Errorf("failed to write middleware response: %w", err)
		}
		L.Pop(1)
		return nil
	}

	L.Pop(1) // Pop nil return

	// If next() was called, continue the chain
	if nextCalled {
		next.ServeHTTP(w, r)
	}

	return nil
}

// pushRequestTable creates a Lua table with HTTP request data
// Optimized version: uses RawSet, pre-allocates table, reduces stack checks
func (e *Engine) pushRequestTable(L *lua.State, r *http.Request) error {
	// Single stack check for entire operation (reduced from multiple checks)
	if !L.CheckStack(10) {
		return fmt.Errorf("insufficient stack space")
	}

	// Pre-allocate table with known size (0 array, 10 hash slots)
	// This reduces rehashing during construction
	L.CreateTable(0, 10)

	// Batch set simple string fields (1 CGO call instead of 15)
	// Reduces: 5 fields × (PushString + PushString + RawSet) = 15 calls → 1 call
	BatchSetStringFields(L, -1, map[string]string{
		"method":      r.Method,
		"path":        r.URL.Path,
		"url":         r.URL.String(),
		"host":        r.Host,
		"remote_addr": r.RemoteAddr,
	})

	// req.headers = {["Content-Type"] = "application/json", ...}
	// Optimize by batching single-value headers, handle multi-value separately
	L.PushString("headers")
	L.CreateTable(0, len(r.Header)) // Pre-allocate
	headersTableIdx := L.GetTop()

	// Collect single-value headers for batching
	singleValueHeaders := make(map[string]string, len(r.Header))
	var multiValueHeaders []struct {
		key    string
		values []string
	}

	for key, values := range r.Header {
		n := len(values)
		if n == 0 {
			continue
		}

		if n == 1 {
			// Common case: single value (batch these)
			singleValueHeaders[key] = values[0]
		} else {
			// Rare case: multi-value header (handle separately)
			multiValueHeaders = append(multiValueHeaders, struct {
				key    string
				values []string
			}{key, values})
		}
	}

	// Batch set all single-value headers (1 CGO call for all headers)
	if len(singleValueHeaders) > 0 {
		BatchSetStringFields(L, headersTableIdx, singleValueHeaders)
	}

	// Handle multi-value headers individually (rare)
	for _, mvh := range multiValueHeaders {
		L.PushString(mvh.key)
		L.CreateTable(len(mvh.values), 0)
		for i, value := range mvh.values {
			L.PushInteger(int64(i + 1)) // Lua 1-indexed
			L.PushString(value)
			L.RawSet(-3)
		}
		L.RawSet(headersTableIdx)
	}

	L.RawSet(-3)

	// req.params = {id = "123", ...} (from Chi URLParam)
	// Batch set params using BatchSetTableField (reduces 3N+3 calls to 1 call)
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && len(rctx.URLParams.Keys) > 0 {
		params := make(map[string]string, len(rctx.URLParams.Keys))
		for i, key := range rctx.URLParams.Keys {
			if i < len(rctx.URLParams.Values) {
				params[key] = rctx.URLParams.Values[i]
			}
		}
		BatchSetTableField(L, -1, "params", params)
	} else {
		// Empty params table
		L.PushString("params")
		L.NewTable()
		L.RawSet(-3)
	}

	// req.query = {foo = "bar", ...}
	// Batch set query params (reduces 3N+3 calls to 1 call)
	query := r.URL.Query()
	if len(query) > 0 {
		queryParams := make(map[string]string, len(query))
		for key, values := range query {
			if len(values) > 0 {
				queryParams[key] = values[0] // Take first value only
			}
		}
		BatchSetTableField(L, -1, "query", queryParams)
	} else {
		// Empty query table
		L.PushString("query")
		L.NewTable()
		L.RawSet(-3)
	}

	// req.body = "..." (read body with size limit)
	// Only read body if Content-Length > 0 (optimization)
	if r.Body != nil && r.ContentLength > 0 {
		bodyBytes := make([]byte, MaxBodySize)
		n, err := io.ReadFull(r.Body, bodyBytes)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			// Only log error, don't fail the request
			slog.Warn("lua_request_body_read_error", "error", err, "component", "lua")
		}

		if n > 0 {
			L.PushString("body")
			L.PushString(string(bodyBytes[:n]))
			L.RawSet(-3) // Use RawSet for consistency
		}
	}

	return nil
}

// writeResponseFromTable writes HTTP response from Lua table
func (e *Engine) writeResponseFromTable(L *lua.State, w http.ResponseWriter) error {
	if !L.IsTable(-1) {
		return fmt.Errorf("response must be a table")
	}

	// Get status (default 200)
	status := 200
	L.GetField(-1, "status")
	if L.IsNumber(-1) {
		status = L.ToInteger(-1)
	}
	L.Pop(1)

	// Get and set headers
	L.GetField(-1, "headers")
	if L.IsTable(-1) {
		// Iterate over headers table
		L.PushNil() // First key
		for L.Next(-2) != 0 {
			// Key at -2, value at -1
			// ToString returns "" if not string (no need for IsString check)
			key := L.ToString(-2)
			value := L.ToString(-1)
			if key != "" && value != "" {
				w.Header().Set(key, value)
			}
			L.Pop(1) // Pop value, keep key for next iteration
		}
	}
	L.Pop(1) // Pop headers table/nil

	// Get body
	body := ""
	L.GetField(-1, "body")
	if L.IsString(-1) {
		body = L.ToString(-1)
	}
	L.Pop(1)

	// Write response
	w.WriteHeader(status)
	if body != "" {
		if _, err := w.Write([]byte(body)); err != nil {
			return fmt.Errorf("failed to write response body: %w", err)
		}
	}

	return nil
}

// registerPrimitives registers Go primitives as Lua global functions (currently just log)
func (e *Engine) registerPrimitives(L *lua.State) {
	// Register log() function
	L.Register("log", func(L *lua.State) int {
		if L.GetTop() >= 1 && L.IsString(1) {
			msg := L.ToString(1)
			slog.Info("lua_log", "message", msg, "component", "lua")
		}
		return 0
	})
}

// setupModulePaths configures Lua's package.path and package.cpath for LuaRocks
func (e *Engine) setupModulePaths(L *lua.State) {
	if len(e.modulePaths) == 0 && len(e.moduleCPaths) == 0 {
		return
	}

	L.GetGlobal("package")
	if L.IsNil(-1) {
		L.Pop(1)
		return
	}

	// Append custom Lua module paths (package.path)
	if len(e.modulePaths) > 0 {
		L.GetField(-1, "path")
		currentPath := L.ToString(-1)
		L.Pop(1)

		for _, customPath := range e.modulePaths {
			currentPath = currentPath + ";" + customPath
		}

		L.PushString(currentPath)
		L.SetField(-2, "path")
	}

	// Append custom C module paths (package.cpath)
	if len(e.moduleCPaths) > 0 {
		L.GetField(-1, "cpath")
		currentCPath := L.ToString(-1)
		L.Pop(1)

		for _, customCPath := range e.moduleCPaths {
			currentCPath = currentCPath + ";" + customCPath
		}

		L.PushString(currentCPath)
		L.SetField(-2, "cpath")
	}

	L.Pop(1) // Pop package table
}
