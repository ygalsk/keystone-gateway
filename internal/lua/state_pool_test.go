package lua

import (
	"context"
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
			L, err := pool.Get(context.Background())
			if err != nil {
				t.Errorf("Failed to get state: %v", err)
				return
			}
			defer pool.Put(L)

			// Execute simple script
			if err := L.DoString(`return 1 + 1`); err != nil {
				t.Errorf("Script execution failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Verify pool statistics
	active, available := pool.GetStats()
	detailedStats := pool.GetDetailedStats()
	t.Logf("Pool stats: active=%d, available=%d", active, available)
	if detailedStats["get_operations"].(int64) != 50 {
		t.Errorf("Expected 50 get operations, got %d", detailedStats["get_operations"])
	}
}

func TestStateCleanup(t *testing.T) {
	pool := NewLuaStatePool(2, func() *lua.LState {
		return lua.NewState()
	})
	defer pool.Shutdown()

	// Get state and leave data on stack
	L, err := pool.Get(context.Background())
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	L.Push(lua.LString("test"))
	L.Push(lua.LNumber(42))

	// Return to pool (should clean stack)
	pool.Put(L)

	// Get state again - should be clean
	L2, err := pool.Get(context.Background())
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if L2.GetTop() != 0 {
		t.Errorf("Stack not cleaned: got %d items", L2.GetTop())
	}
	pool.Put(L2)
}
