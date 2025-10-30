package lua

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// LuaStatePool manages a pool of Lua states for thread-safe request handling
// This version fixes segfaults by using proper state isolation per goroutine
type LuaStatePool struct {
	pool        chan *lua.LState
	maxStates   int
	createState func() *lua.LState
	mu          sync.Mutex
	created     int64
	closed      bool
	// Removed scripts - now using Engine's ScriptCompiler
}

// CompiledScript type removed - now using ScriptCompiler.CompiledScript

// NewLuaStatePool creates a new pool of Lua states with improved thread safety
func NewLuaStatePool(maxStates int, createState func() *lua.LState) *LuaStatePool {
	return &LuaStatePool{
		pool:        make(chan *lua.LState, maxStates),
		maxStates:   maxStates,
		createState: createState,
		// scripts map removed - using Engine's ScriptCompiler
	}
}

// Get retrieves a Lua state from the pool or creates a new one
// This implementation prevents segfaults by ensuring proper state isolation
func (p *LuaStatePool) Get() *lua.LState {
	select {
	case L := <-p.pool:
		return L
	default:
		// Pool is empty, create new state if under limit
		if atomic.LoadInt64(&p.created) < int64(p.maxStates) {
			if atomic.AddInt64(&p.created, 1) <= int64(p.maxStates) {
				return p.createState()
			}
			// Rollback if we exceeded the limit
			atomic.AddInt64(&p.created, -1)
		}

		// Wait for a state to become available
		return <-p.pool
	}
}

// Put returns a Lua state to the pool
func (p *LuaStatePool) Put(L *lua.LState) {
	if L == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		// Pool is closed, just close the state
		L.Close()
		atomic.AddInt64(&p.created, -1)
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	select {
	case p.pool <- L:
		// Successfully returned to pool
	default:
		// Pool is full, close the state
		L.Close()
		atomic.AddInt64(&p.created, -1)
	}
}

// Close closes all states in the pool
func (p *LuaStatePool) Close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()

	close(p.pool)
	for L := range p.pool {
		L.Close()
	}
}

// RegisterScript and GetScript methods removed - using Engine's ScriptCompiler instead

// LuaHandler represents a thread-safe Lua function handler
// This version prevents segfaults through proper state isolation and pre-compilation
type LuaHandler struct {
	scriptKey    string
	functionName string
	tenantName   string
	scriptTag    string
	pool         *LuaStatePool
	engine       *Engine
}

// Constants to avoid magic numbers/strings
const (
	defaultHandlerTimeout = 10 * time.Second
)

// NewLuaHandler creates a new thread-safe Lua handler that delegates to Engine
func NewLuaHandler(scriptContent, functionName, tenantName, scriptTag string, pool *LuaStatePool, engine *Engine) *LuaHandler {
	scriptKey := fmt.Sprintf("%s_%s", tenantName, functionName)

	// Pre-compile script using Engine (single responsibility)
	if err := engine.CompileScript(scriptKey, scriptContent); err != nil {
		slog.Error("lua_handler_compile_failed", "script", scriptKey, "error", err)
	}

	return &LuaHandler{
		scriptKey:    scriptKey,
		functionName: functionName,
		tenantName:   tenantName,
		scriptTag:    scriptTag,
		pool:         pool,
		engine:       engine,
	}
}

// ServeHTTP implements http.Handler by delegating to Engine (single responsibility)
func (h *LuaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultHandlerTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Lua handler: %v", r)
			}
		}()
		// Delegate all script execution to Engine
		done <- h.engine.ExecuteScriptHandler(h.scriptKey, h.functionName, w, r)
	}()

	select {
	case err := <-done:
		if err != nil {
			http.Error(w, "Lua handler error: "+err.Error(), http.StatusInternalServerError)
		}
	case <-ctx.Done():
		http.Error(w, "Lua handler timeout", http.StatusRequestTimeout)
	}
}
