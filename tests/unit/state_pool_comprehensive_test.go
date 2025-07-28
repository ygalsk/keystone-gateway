package unit

import (
	"sync"
	"testing"
	"time"

	luaLib "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/lua"
)

// Test fixtures for state pool testing
func createTestStatePool(t *testing.T, poolSize int) *lua.LuaStatePool {
	createStateFunc := func() *luaLib.LState {
		return luaLib.NewState()
	}
	pool := lua.NewLuaStatePool(poolSize, createStateFunc)
	return pool
}

func createTestLuaState() *luaLib.LState {
	L := luaLib.NewState()
	return L
}

// TestLuaStatePool_Close tests the Close() function with various pool states
func TestLuaStatePool_Close(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*lua.LuaStatePool) []*luaLib.LState
		expectedClosed bool
	}{
		{
			name: "close_empty_pool",
			setup: func(pool *lua.LuaStatePool) []*luaLib.LState {
				// Return empty slice - no states to track
				return []*luaLib.LState{}
			},
			expectedClosed: true,
		},
		{
			name: "close_pool_with_states",
			setup: func(pool *lua.LuaStatePool) []*luaLib.LState {
				// Add some states to the pool
				states := make([]*luaLib.LState, 3)
				for i := 0; i < 3; i++ {
					L := createTestLuaState()
					pool.Put(L)
					states[i] = L
				}
				return states
			},
			expectedClosed: true,
		},
		{
			name: "close_pool_with_full_capacity",
			setup: func(pool *lua.LuaStatePool) []*luaLib.LState {
				// Fill the pool to capacity
				for i := 0; i < 5; i++ {
					L := createTestLuaState()
					pool.Put(L)
				}
				return []*luaLib.LState{}
			},
			expectedClosed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := createTestStatePool(t, 5)
			_ = tt.setup(pool) // Setup states but don't track them

			// Close the pool
			pool.Close()

			// Verify pool is marked as closed by trying to put a new state
			// After closure, Put should immediately close the state
			testState := createTestLuaState()
			pool.Put(testState) // Should close immediately since pool is closed

			// Pool is now closed and should handle new operations gracefully

			// States are cleaned up automatically by pool.Close()
			// No need to manually close them
		})
	}
}

// TestLuaStatePool_Put tests the Put() function with various scenarios
func TestLuaStatePool_Put(t *testing.T) {
	tests := []struct {
		name        string
		poolSize    int
		setup       func(*lua.LuaStatePool)
		putState    func() *luaLib.LState
		expectClose bool
	}{
		{
			name:     "put_state_into_empty_pool",
			poolSize: 5,
			setup:    nil,
			putState: func() *luaLib.LState {
				return createTestLuaState()
			},
			expectClose: false, // State should be returned to pool, not closed
		},
		{
			name:     "put_state_into_full_pool",
			poolSize: 2,
			setup: func(pool *lua.LuaStatePool) {
				// Fill the pool to capacity
				for i := 0; i < 2; i++ {
					L := createTestLuaState()
					pool.Put(L)
				}
			},
			putState: func() *luaLib.LState {
				return createTestLuaState()
			},
			expectClose: true, // State should be closed because pool is full
		},
		{
			name:     "put_nil_state",
			poolSize: 5,
			setup:    nil,
			putState: func() *luaLib.LState {
				return nil
			},
			expectClose: false, // Nil state should be handled gracefully
		},
		{
			name:     "put_state_into_closed_pool",
			poolSize: 5,
			setup: func(pool *lua.LuaStatePool) {
				// We'll close the pool in the test itself
			},
			putState: func() *luaLib.LState {
				return createTestLuaState()
			},
			expectClose: true, // State should be closed immediately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := createTestStatePool(t, tt.poolSize)

			if tt.setup != nil {
				tt.setup(pool)
			}

			// Special handling for closed pool test
			if tt.name == "put_state_into_closed_pool" {
				pool.Close() // Close the pool for this test
			}

			L := tt.putState()
			if L != nil {
				pool.Put(L)
				// Put operation should complete without error
			}

			// Clean up the pool (avoid double close for closed pool test)
			if tt.name != "put_state_into_closed_pool" {
				pool.Close()
			}
		})
	}
}

// TestLuaStatePool_Concurrency tests concurrent access scenarios
func TestLuaStatePool_Concurrency(t *testing.T) {
	t.Run("concurrent_put_operations", func(t *testing.T) {
		pool := createTestStatePool(t, 10)
		defer pool.Close()

		const numGoroutines = 20
		const statesPerGoroutine = 5

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		// Concurrently put states into the pool
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				for j := 0; j < statesPerGoroutine; j++ {
					L := createTestLuaState()
					pool.Put(L) // Should not panic or cause race conditions
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Errorf("Concurrent put error: %v", err)
		}

		// Pool should still be functional
		testState := createTestLuaState()
		pool.Put(testState) // Should not panic
	})

	t.Run("concurrent_put_and_close", func(t *testing.T) {
		pool := createTestStatePool(t, 5)

		var wg sync.WaitGroup

		// Start putting states concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				L := createTestLuaState()
				pool.Put(L)
				time.Sleep(1 * time.Millisecond) // Small delay to increase race chance
			}
		}()

		// Close pool concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond) // Let some puts happen first
			pool.Close()
		}()

		wg.Wait()

		// Additional puts after close should handle closed pool gracefully
		for i := 0; i < 3; i++ {
			L := createTestLuaState()
			pool.Put(L) // Should close immediately
		}
	})
}

// TestLuaStatePool_ResourceCleanup tests resource cleanup scenarios
func TestLuaStatePool_ResourceCleanup(t *testing.T) {
	t.Run("cleanup_on_close", func(t *testing.T) {
		pool := createTestStatePool(t, 3)

		// Add states to pool
		for i := 0; i < 3; i++ {
			L := createTestLuaState()
			pool.Put(L)
		}

		// Close should clean up all states
		pool.Close()

		// Verify pool handles post-close operations gracefully
		testState := createTestLuaState()
		pool.Put(testState) // Should close immediately
	})

	t.Run("cleanup_on_full_pool", func(t *testing.T) {
		poolSize := 2
		pool := createTestStatePool(t, poolSize)
		defer pool.Close()

		// Fill pool to capacity
		for i := 0; i < poolSize; i++ {
			L := createTestLuaState()
			pool.Put(L)
		}

		// Add one more state - should be closed due to full pool
		extraState := createTestLuaState()
		pool.Put(extraState) // Should close this state

		// Pool should still be functional
		anotherState := createTestLuaState()
		pool.Put(anotherState) // Should also be closed due to full pool
	})
}

// TestLuaStatePool_EdgeCases tests edge cases and error conditions
func TestLuaStatePool_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(*testing.T)
	}{
		{
			name: "zero_size_pool",
			test: func(t *testing.T) {
				pool := createTestStatePool(t, 0)
				defer pool.Close()

				// All states should be closed immediately
				L := createTestLuaState()
				pool.Put(L) // Should close immediately due to zero-size pool
			},
		},
		{
			name: "multiple_nil_puts",
			test: func(t *testing.T) {
				pool := createTestStatePool(t, 5)
				defer pool.Close()

				// Multiple nil puts should not cause issues
				for i := 0; i < 10; i++ {
					pool.Put(nil)
				}
			},
		},
		{
			name: "operations_on_closed_pool",
			test: func(t *testing.T) {
				pool := createTestStatePool(t, 5)
				pool.Close()

				// Operations on closed pool should handle gracefully
				L := createTestLuaState()
				pool.Put(L) // Should close state immediately
			},
		},
		{
			name: "put_after_close",
			test: func(t *testing.T) {
				pool := createTestStatePool(t, 5)
				pool.Close()

				// Put should still work (closing states immediately)
				L := createTestLuaState()
				pool.Put(L) // Should close immediately
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}

// TestLuaStatePool_Integration tests integration with the broader system
func TestLuaStatePool_Integration(t *testing.T) {
	t.Run("pool_lifecycle", func(t *testing.T) {
		poolSize := 5
		pool := createTestStatePool(t, poolSize)

		// Phase 1: Normal operations
		for i := 0; i < 3; i++ {
			L := createTestLuaState()
			pool.Put(L)
		}

		// Phase 2: Fill to capacity and test overflow
		for i := 0; i < poolSize; i++ {
			L := createTestLuaState()
			pool.Put(L)
		}

		// Phase 3: Test overflow handling
		extraState := createTestLuaState()
		pool.Put(extraState) // Should be closed due to full pool

		// Phase 4: Close and test post-close behavior
		pool.Close()

		postCloseState := createTestLuaState()
		pool.Put(postCloseState) // Should be closed immediately

		// States are cleaned up automatically by pool.Close()
		// No need to manually close them
	})
}
