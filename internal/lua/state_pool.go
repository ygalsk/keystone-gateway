package lua

import (
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/aarzilli/golua/lua"
)

// LuaStatePool manages a pool of Lua states for thread-safe concurrent execution.
// States are reused to avoid the overhead of creating new states on each request.
type LuaStatePool struct {
	pool    chan *lua.State
	factory func() *lua.State
	mu      sync.Mutex
	closed  bool

	// Metrics
	poolHits     atomic.Int64 // States obtained from pool
	poolMisses   atomic.Int64 // States created dynamically
	totalWaitNs  atomic.Int64 // Total time waiting for states
	activeStates atomic.Int64 // Currently in-use states
}

// NewLuaStatePool creates a new state pool with the given size and factory function.
// The pool is pre-warmed with states created by the factory function.
func NewLuaStatePool(size int, factory func() *lua.State) *LuaStatePool {
	pool := &LuaStatePool{
		pool:    make(chan *lua.State, size),
		factory: factory,
	}

	// Pre-warm the pool
	for i := 0; i < size; i++ {
		pool.pool <- factory()
	}

	return pool
}

// Get retrieves a Lua state from the pool.
// Blocks until a state is available. Never creates new states dynamically.
func (p *LuaStatePool) Get() *lua.State {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		panic("state pool is closed")
	}
	p.mu.Unlock()

	start := time.Now()

	// Try to get from pool first (non-blocking check)
	select {
	case L := <-p.pool:
		p.poolHits.Add(1)
		p.activeStates.Add(1)
		return L
	default:
		// Pool exhausted - track miss
		p.poolMisses.Add(1)
	}

	// Block until state available
	L := <-p.pool
	waitDuration := time.Since(start)
	p.totalWaitNs.Add(int64(waitDuration))
	p.activeStates.Add(1)

	return L
}

// Put returns a Lua state to the pool.
// The state's stack is reset to clean state before returning to the pool.
func (p *LuaStatePool) Put(L *lua.State) {
	p.activeStates.Add(-1)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		L.Close()
		return
	}

	// Reset stack to clean state
	L.SetTop(0)

	select {
	case p.pool <- L:
		// Returned to pool
	default:
		// Pool full, close this state
		L.Close()
	}
}

// PoolStats returns current pool statistics
type PoolStats struct {
	PoolHits      int64   `json:"pool_hits"`
	PoolMisses    int64   `json:"pool_misses"`
	ActiveStates  int64   `json:"active_states"`
	AvgWaitTimeMs float64 `json:"avg_wait_time_ms"`
	HitRate       float64 `json:"hit_rate"`
}

// Stats returns current pool metrics
func (p *LuaStatePool) Stats() PoolStats {
	hits := p.poolHits.Load()
	misses := p.poolMisses.Load()
	totalReqs := hits + misses
	waitNs := p.totalWaitNs.Load()

	stats := PoolStats{
		PoolHits:     hits,
		PoolMisses:   misses,
		ActiveStates: p.activeStates.Load(),
	}

	if misses > 0 {
		stats.AvgWaitTimeMs = float64(waitNs) / float64(misses) / 1e6
	}

	if totalReqs > 0 {
		stats.HitRate = float64(hits) / float64(totalReqs) * 100
	}

	return stats
}

// Close shuts down the state pool and closes all pooled Lua states.
func (p *LuaStatePool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	close(p.pool)

	// Close all pooled states
	for L := range p.pool {
		L.Close()
	}
}
