package lua

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	lua "github.com/yuin/gopher-lua"
	"keystone-gateway/internal/types"
)

// Engine represents the main Lua execution engine
type Engine struct {
	statePool *LuaStatePool
	compiler  *ScriptCompiler
	metrics   *LuaMetrics
	config    SecurityConfig

	// Atomic counters for engine state
	initialized atomic.Bool
	shutdown    atomic.Bool
}

// NewEngine creates a new Lua engine with proper initialization
func NewEngine(maxStates, maxScripts int) *Engine {
	config := DefaultSecurityConfig()

	engine := &Engine{
		compiler: NewScriptCompiler(maxScripts),
		metrics:  NewLuaMetrics(),
		config:   config,
	}

	// Initialize state pool with secure factory
	engine.statePool = NewLuaStatePool(maxStates, func() *lua.LState {
		return CreateSecureLuaState(config)
	})

	engine.initialized.Store(true)
	return engine
}

// ExecuteRouteScript executes a routing script for a request
func (e *Engine) ExecuteRouteScript(ctx context.Context, r *http.Request) (*types.RouteResult, error) {
	if !e.initialized.Load() || e.shutdown.Load() {
		return nil, fmt.Errorf("lua engine not initialized or shut down")
	}

	// Get tenant-specific script (implement based on your tenant resolution)
	scriptContent := e.getRoutingScript(r)
	if scriptContent == "" {
		return nil, nil // No Lua script for this tenant/route
	}

	// Track execution metrics
	done := e.metrics.TrackExecution()
	defer done(nil) // Will be updated with actual error

	// Get Lua state from pool
	L := e.statePool.Get()
	defer e.statePool.Put(L)

	// Set execution context with timeout
	L.SetContext(ctx)
	defer L.RemoveContext()

	// Compile script (with caching)
	script, err := e.compiler.CompileScript("routing", scriptContent)
	if err != nil {
		done(err)
		return nil, fmt.Errorf("script compilation failed: %w", err)
	}

	// Set up request context for Lua
	if err := e.setupRequestContext(L, r); err != nil {
		done(err)
		return nil, fmt.Errorf("context setup failed: %w", err)
	}

	// Execute bytecode
	if err := ExecuteWithBytecode(L, script); err != nil {
		done(err)
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	// Extract result
	result, err := e.extractRouteResult(L)
	done(err)
	return result, err
}

// setupRequestContext exposes request data to Lua
func (e *Engine) setupRequestContext(L *lua.LState, r *http.Request) error {
	// Create request table for Lua
	reqTable := L.NewTable()

	// Basic request info
	L.SetField(reqTable, "method", lua.LString(r.Method))
	L.SetField(reqTable, "path", lua.LString(r.URL.Path))
	L.SetField(reqTable, "host", lua.LString(r.Host))

	// Headers
	headersTable := L.NewTable()
	for key, values := range r.Header {
		if len(values) > 0 {
			L.SetField(headersTable, key, lua.LString(values[0]))
		}
	}
	L.SetField(reqTable, "headers", headersTable)

	// Query parameters
	queryTable := L.NewTable()
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			L.SetField(queryTable, key, lua.LString(values[0]))
		}
	}
	L.SetField(reqTable, "query", queryTable)

	L.SetGlobal("request", reqTable)
	return nil
}

// extractRouteResult extracts routing decision from Lua state
func (e *Engine) extractRouteResult(L *lua.LState) (*types.RouteResult, error) {
	// Get result table from global 'result'
	resultLV := L.GetGlobal("result")
	if resultLV == lua.LNil {
		return nil, nil // No routing decision
	}

	resultTable, ok := resultLV.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("result must be a table")
	}

	result := &types.RouteResult{
		Headers: make(map[string]string),
	}

	// Extract routing decision
	if shouldRoute := L.GetField(resultTable, "should_route"); shouldRoute != lua.LNil {
		result.ShouldRoute = lua.LVAsBool(shouldRoute)
	}

	// Extract target upstream
	if target := L.GetField(resultTable, "target_upstream"); target != lua.LNil {
		result.TargetUpstream = string(target.(lua.LString))
	}

	// Extract status code
	if status := L.GetField(resultTable, "status_code"); status != lua.LNil {
		result.StatusCode = int(status.(lua.LNumber))
	}

	// Extract headers
	if headers := L.GetField(resultTable, "headers"); headers != lua.LNil {
		if headersTable, ok := headers.(*lua.LTable); ok {
			headersTable.ForEach(func(key, value lua.LValue) {
				result.Headers[key.String()] = value.String()
			})
		}
	}

	return result, nil
}

// getRoutingScript returns the Lua script for the request (implement based on your needs)
func (e *Engine) getRoutingScript(r *http.Request) string {
	// TODO get from server.go!!!!!
	// This is where you'd implement tenant-specific script resolution
	// For now, return a simple example script
	return `
        -- Simple routing logic
        result = {
            should_route = true,
            target_upstream = "http://backend-1:8080",
            headers = {
                ["X-Routed-By"] = "lua"
            }
        }
    `
}

// GetStats returns engine statistics
func (e *Engine) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"initialized": e.initialized.Load(),
		"shutdown":    e.shutdown.Load(),
	}

	// Add pool stats
	for k, v := range e.statePool.GetStats() {
		stats["pool_"+k] = v
	}

	// Add execution metrics (Engine doesn't track router state, pass zeros)
	for k, v := range e.metrics.GetStats(0, 0, 0) {
		stats["exec_"+k] = v
	}

	// Add compiler stats
	compilerStats := e.compiler.GetCacheStats()
	for k, v := range compilerStats {
		stats["compiler_"+k] = v
	}

	return stats
}

// Shutdown gracefully shuts down the engine
func (e *Engine) Shutdown() error {
	if !e.shutdown.CompareAndSwap(false, true) {
		return fmt.Errorf("engine already shut down")
	}

	e.statePool.Shutdown()
	return nil
}
