package lua

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// StateConfig defines configuration for the memory-aware state pool
type StateConfig struct {
	MaxStates           int     // Maximum number of states in pool
	MaxTotalMemoryMB    int64   // Global memory limit across all states
	PerStateMemoryMB    int     // Per-state memory limit
	MemoryCheckEnabled  bool    // Enable memory tracking
	AggressiveCleanup   bool    // Enable enhanced cleanup procedures
	CleanupTimeoutMs    int     // Timeout for cleanup operations
	RecreationThreshold float64 // Memory threshold for state recreation (0.0-1.0)
	EmergencyThreshold  float64 // Memory threshold for emergency cleanup (0.0-1.0)
}

// DefaultStateConfig returns sensible defaults for state pool configuration
func DefaultStateConfig() StateConfig {
	return StateConfig{
		MaxStates:           10,
		MaxTotalMemoryMB:    500,  // Increased for higher state counts (was 100)
		PerStateMemoryMB:    10,
		MemoryCheckEnabled:  true,
		AggressiveCleanup:   true,
		CleanupTimeoutMs:    1000,
		RecreationThreshold: 0.70, // 70% of per-state limit
		EmergencyThreshold:  0.90, // 90% of global limit
	}
}

// LuaStatePool implements a memory-aware Lua state pool with comprehensive tracking
type LuaStatePool struct {
	// Pool management
	states   chan *lua.LState
	config   StateConfig
	factory  func() *lua.LState
	mu       sync.RWMutex
	shutdown atomic.Bool

	// Memory tracking (atomic for thread safety)
	activeStates       atomic.Int32
	currentMemoryBytes atomic.Int64
	memoryViolations   atomic.Int64
	statesRecreated    atomic.Int64
	emergencyCleanups  atomic.Int64

	// State lifecycle tracking
	stateCreationCount atomic.Int64
	stateDestroyCount  atomic.Int64

	// Performance metrics
	getOperations atomic.Int64
	putOperations atomic.Int64
	timeouts      atomic.Int64
}

// NewLuaStatePool creates a memory-aware Lua state pool with default configuration
// Supports both old signature NewLuaStatePool(maxStates int) and enhanced signature with factory
func NewLuaStatePool(maxStates int, factory ...func() *lua.LState) *LuaStatePool {
	config := DefaultStateConfig()
	config.MaxStates = maxStates

	var stateFactory func() *lua.LState
	if len(factory) > 0 && factory[0] != nil {
		stateFactory = factory[0]
	}

	return NewLuaStatePoolWithConfig(config, stateFactory)
}

// NewLuaStatePoolWithConfig creates a memory-aware pool with custom configuration
func NewLuaStatePoolWithConfig(config StateConfig, factory func() *lua.LState) *LuaStatePool {
	if factory == nil {
		factory = func() *lua.LState {
			opts := lua.Options{
				SkipOpenLibs: false,
			}

			L := lua.NewState(opts)

			// Note: gopher-lua doesn't have SetMx method for memory limits
			// Memory management is handled at the pool level

			return L
		}
	}

	p := &LuaStatePool{
		states:  make(chan *lua.LState, config.MaxStates),
		config:  config,
		factory: factory,
	}

	// Pre-populate the pool
	for i := 0; i < config.MaxStates; i++ {
		L := p.factory()
		p.stateCreationCount.Add(1)

		// Track initial memory usage
		if config.MemoryCheckEnabled {
			memUsage := p.estimateStateMemory(L)
			p.currentMemoryBytes.Add(memUsage)
		}

		p.states <- L
	}

	return p
}

// Get borrows a Lua state from the pool with context timeout and memory validation
func (p *LuaStatePool) Get(ctx context.Context) (*lua.LState, error) {
	if p.shutdown.Load() {
		return nil, fmt.Errorf("state pool is shutdown")
	}

	p.getOperations.Add(1)

	select {
	case L := <-p.states:
		// Check if emergency cleanup is needed
		if p.config.MemoryCheckEnabled {
			p.checkEmergencyMemoryPressure()
		}

		// Validate state before use
		if p.config.MemoryCheckEnabled {
			if !p.validateStateMemory(L) {
				// State exceeds memory threshold, recreate it
				p.recreateState(L)
				L = p.factory()
				p.stateCreationCount.Add(1)
				p.statesRecreated.Add(1)
				
				// Track memory for new state
				newMemUsage := p.estimateStateMemory(L)
				p.currentMemoryBytes.Add(newMemUsage)
			}
		}

		p.activeStates.Add(1)
		return L, nil

	case <-ctx.Done():
		p.timeouts.Add(1)
		return nil, fmt.Errorf("timeout waiting for Lua state: %v", ctx.Err())
	}
}

// Put returns a Lua state back to the pool with enhanced cleanup
func (p *LuaStatePool) Put(L *lua.LState) {
	if L == nil {
		return
	}

	p.putOperations.Add(1)

	// Perform cleanup based on configuration
	if p.config.AggressiveCleanup {
		p.enhancedCleanup(L)
	} else {
		p.basicCleanup(L)
	}

	p.activeStates.Add(-1)

	// Don't return state if pool is shutdown
	if p.shutdown.Load() {
		L.Close()
		p.stateDestroyCount.Add(1)
		return
	}

	// Non-blocking put to prevent deadlock during shutdown
	select {
	case p.states <- L:
		// Successfully returned to pool
	default:
		// Pool is full (shouldn't happen), close the state
		L.Close()
		p.stateDestroyCount.Add(1)
	}
}

// Shutdown gracefully closes all Lua states and stops the pool
func (p *LuaStatePool) Shutdown() {
	if p.shutdown.Swap(true) {
		return // Already shutdown
	}

	// Create timeout context for cleanup
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(p.config.CleanupTimeoutMs)*time.Millisecond)
	defer cancel()

	// Close the channel to prevent new states from being added
	close(p.states)

	// Close all states in the pool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for L := range p.states {
			if p.config.AggressiveCleanup {
				p.enhancedCleanup(L)
			}
			L.Close()
			p.stateDestroyCount.Add(1)
		}
	}()

	// Wait for cleanup to complete or timeout
	select {
	case <-done:
		// Cleanup completed successfully
	case <-ctx.Done():
		// Cleanup timed out, force close remaining states
		for L := range p.states {
			L.Close()
			p.stateDestroyCount.Add(1)
		}
	}

	// Reset memory counters
	p.currentMemoryBytes.Store(0)
	p.activeStates.Store(0)
}

// GetStats returns comprehensive pool statistics
func (p *LuaStatePool) GetStats() (active int, available int) {
	return int(p.activeStates.Load()), len(p.states)
}

// GetDetailedStats returns comprehensive pool statistics including memory usage
func (p *LuaStatePool) GetDetailedStats() map[string]interface{} {
	return map[string]interface{}{
		"active_states":      int(p.activeStates.Load()),
		"available_states":   len(p.states),
		"max_states":         p.config.MaxStates,
		"current_memory_mb":  float64(p.currentMemoryBytes.Load()) / (1024 * 1024),
		"max_memory_mb":      p.config.MaxTotalMemoryMB,
		"memory_violations":  p.memoryViolations.Load(),
		"states_recreated":   p.statesRecreated.Load(),
		"emergency_cleanups": p.emergencyCleanups.Load(),
		"states_created":     p.stateCreationCount.Load(),
		"states_destroyed":   p.stateDestroyCount.Load(),
		"get_operations":     p.getOperations.Load(),
		"put_operations":     p.putOperations.Load(),
		"timeouts":           p.timeouts.Load(),
		"shutdown":           p.shutdown.Load(),
	}
}

// estimateStateMemory estimates memory usage of a Lua state
func (p *LuaStatePool) estimateStateMemory(L *lua.LState) int64 {
	if L == nil {
		return 0
	}

	// gopher-lua doesn't expose direct memory counting APIs
	// Use a heuristic based on registry size and stack depth
	var memoryBase int64 = 8192 // Base memory per state (8KB)

	// Add memory for stack depth
	stackSize := int64(L.GetTop() * 64) // Estimate 64 bytes per stack item

	// Add memory for global variables (rough estimation)
	// This is a heuristic since gopher-lua doesn't expose direct memory stats
	globalsEstimate := int64(1024) // Base estimate for globals

	return memoryBase + stackSize + globalsEstimate
}

// validateStateMemory checks if a state's memory usage is within acceptable limits
func (p *LuaStatePool) validateStateMemory(L *lua.LState) bool {
	if !p.config.MemoryCheckEnabled || p.config.PerStateMemoryMB <= 0 {
		return true
	}

	memUsage := p.estimateStateMemory(L)
	threshold := int64(float64(p.config.PerStateMemoryMB*1024*1024) * p.config.RecreationThreshold)

	if memUsage > threshold {
		p.memoryViolations.Add(1)
		return false
	}

	return true
}

// recreateState safely closes and recreates a state
func (p *LuaStatePool) recreateState(L *lua.LState) {
	if L == nil {
		return
	}

	// Track old memory usage
	oldMem := p.estimateStateMemory(L)

	// Close the old state
	L.Close()
	p.stateDestroyCount.Add(1)

	// Update memory tracking
	if p.config.MemoryCheckEnabled {
		p.currentMemoryBytes.Add(-oldMem)
	}
}

// checkEmergencyMemoryPressure triggers emergency cleanup if memory usage is critical
func (p *LuaStatePool) checkEmergencyMemoryPressure() {
	if !p.config.MemoryCheckEnabled || p.config.MaxTotalMemoryMB <= 0 {
		return
	}

	currentMB := float64(p.currentMemoryBytes.Load()) / (1024 * 1024)
	thresholdMB := float64(p.config.MaxTotalMemoryMB) * p.config.EmergencyThreshold

	if currentMB > thresholdMB {
		p.performEmergencyCleanup()
	}
}

// performEmergencyCleanup closes half of the pooled states to free memory
func (p *LuaStatePool) performEmergencyCleanup() {
	p.emergencyCleanups.Add(1)

	// Calculate how many states to close (half of available states)
	available := len(p.states)
	toClose := available / 2

	if toClose <= 0 {
		return
	}

	// Close states from the pool
	for i := 0; i < toClose; i++ {
		select {
		case L := <-p.states:
			memUsage := p.estimateStateMemory(L)
			L.Close()
			p.stateDestroyCount.Add(1)

			// Update memory tracking
			if p.config.MemoryCheckEnabled {
				p.currentMemoryBytes.Add(-memUsage)
			}

			// Create a new clean state to replace it
			newL := p.factory()
			p.stateCreationCount.Add(1)

			if p.config.MemoryCheckEnabled {
				newMemUsage := p.estimateStateMemory(newL)
				p.currentMemoryBytes.Add(newMemUsage)
			}

			// Put the new state back
			select {
			case p.states <- newL:
				// Successfully added new state
			default:
				// Pool full, close the new state
				newL.Close()
				p.stateDestroyCount.Add(1)
			}

		default:
			// No states available to clean
			break
		}
	}

	// Force garbage collection
	runtime.GC()
}

// basicCleanup performs minimal cleanup of a Lua state
func (p *LuaStatePool) basicCleanup(L *lua.LState) {
	if L == nil {
		return
	}

	// Clear globals
	L.SetGlobal("_G", L.NewTable())

	// Trigger garbage collection using the collectgarbage function
	// This runs Go's garbage collector for the entire program
	runtime.GC()
}

// enhancedCleanup performs comprehensive cleanup of a Lua state
func (p *LuaStatePool) enhancedCleanup(L *lua.LState) {
	if L == nil {
		return
	}

	// Track memory before cleanup
	var memBefore int64
	if p.config.MemoryCheckEnabled {
		memBefore = p.estimateStateMemory(L)
	}

	// Clear the stack completely
	top := L.GetTop()
	if top > 0 {
		L.Pop(top)
		L.SetTop(0)
	}

	// Remove any context data - gopher-lua specific method
	// Note: RemoveContext may not exist, using alternative approach
	L.SetContext(context.Background())

	// Clear known global variables that might hold references
	globals := []string{
		"_G", "package", "require", "loadfile", "dofile",
		"pcall", "xpcall", "_ENV", "_VERSION",
	}

	newTable := L.NewTable()
	for _, global := range globals {
		L.SetGlobal(global, newTable)
	}

	// Multiple garbage collection cycles for thorough cleanup
	// Use Go's runtime GC since gopher-lua relies on it
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
	
	// Update memory tracking
	if p.config.MemoryCheckEnabled {
		memAfter := p.estimateStateMemory(L)
		memoryFreed := memBefore - memAfter
		if memoryFreed > 0 {
			p.currentMemoryBytes.Add(-memoryFreed)
		}
	}
}
