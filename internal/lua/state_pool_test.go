package lua

import (
	lua "github.com/yuin/gopher-lua"
	"sync"
	"testing"
)

func TestStatePoolConcurrency(t *testing.T) {
	pool := NewLuaStatePool(5, func() *lua.LState {
		return lua.NewState()
	})
	defer pool.Shutdown()

	// Test concurrent access (50 goroutines like upstream tests)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			L := pool.Get()
			defer pool.Put(L)

			// Execute simple script
			if err := L.DoString(`return 1 + 1`); err != nil {
				t.Errorf("Script execution failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Verify pool statistics
	stats := pool.GetStats()
	if stats["total_executions"] != 50 {
		t.Errorf("Expected 50 executions, got %d", stats["total_executions"])
	}
}

func TestStateCleanup(t *testing.T) {
	pool := NewLuaStatePool(2, func() *lua.LState {
		return lua.NewState()
	})
	defer pool.Shutdown()

	// Get state and leave data on stack
	L := pool.Get()
	L.Push(lua.LString("test"))
	L.Push(lua.LNumber(42))

	// Return to pool (should clean stack)
	pool.Put(L)

	// Get state again - should be clean
	L2 := pool.Get()
	if L2.GetTop() != 0 {
		t.Errorf("Stack not cleaned: got %d items", L2.GetTop())
	}
	pool.Put(L2)
}
