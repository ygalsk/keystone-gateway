package lua

import (
	"sync"
	"sync/atomic"

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
