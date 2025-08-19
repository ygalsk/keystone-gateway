package lua

import (
	"sync"
	"sync/atomic"
	//"time"

	lua "github.com/yuin/gopher-lua"
)

// LuaStatePool - Official gopher-lua pattern enhanced with atomic operations
type LuaStatePool struct {
	mu      sync.Mutex         // Official pattern requires mutex for pool
	saved   []*lua.LState      // Pool of reusable states
	factory func() *lua.LState // State factory function
	maxSize int                // Maximum pool size

	// Atomic metrics (consistent with upstream architecture)
	activeStates    atomic.Int32 // Currently active states
	totalCreated    atomic.Int64 // Total states created
	totalExecutions atomic.Int64 // Total script executions
	poolHits        atomic.Int64 // Pool reuse statistics
	poolMisses      atomic.Int64 // Pool creation statistics
}

func NewLuaStatePool(maxSize int, factory func() *lua.LState) *LuaStatePool {
	return &LuaStatePool{
		saved:   make([]*lua.LState, 0, maxSize),
		factory: factory,
		maxSize: maxSize,
	}
}

// Get - Official gopher-lua pattern from documentation
func (p *LuaStatePool) Get() *lua.LState {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.saved)
	if n == 0 {
		// Pool empty - create new state
		L := p.factory()
		p.activeStates.Add(1)
		p.totalCreated.Add(1)
		p.poolMisses.Add(1)
		return L
	}

	// Reuse state from pool
	L := p.saved[n-1]
	p.saved = p.saved[0 : n-1]
	p.activeStates.Add(1)
	p.poolHits.Add(1)
	return L
}

// Put - Official gopher-lua pattern with mandatory cleanup
func (p *LuaStatePool) Put(L *lua.LState) {
	if L.IsClosed() {
		p.activeStates.Add(-1)
		return
	}

	// CRITICAL: Clean state before reuse (from official docs)
	L.Pop(L.GetTop()) // Clear entire stack
	L.SetTop(0)       // Reset stack pointer
	L.RemoveContext() // Clear execution context

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.saved) < p.maxSize {
		p.saved = append(p.saved, L)
	} else {
		L.Close() // Pool full, close excess state
	}
	p.activeStates.Add(-1)
}

// GetStats - Atomic statistics consistent with upstream patterns
func (p *LuaStatePool) GetStats() map[string]int64 {
	return map[string]int64{
		"active_states":    int64(p.activeStates.Load()),
		"total_created":    p.totalCreated.Load(),
		"total_executions": p.totalExecutions.Load(),
		"pool_hits":        p.poolHits.Load(),
		"pool_misses":      p.poolMisses.Load(),
		"pool_size":        int64(len(p.saved)),
		"max_pool_size":    int64(p.maxSize),
	}
}

// Shutdown - Clean shutdown with proper resource cleanup
func (p *LuaStatePool) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, L := range p.saved {
		L.Close()
	}
	p.saved = nil
}
