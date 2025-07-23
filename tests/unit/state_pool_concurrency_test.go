package unit

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"keystone-gateway/internal/lua"

	gopher "github.com/yuin/gopher-lua"
)

func TestLuaStatePoolRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	// Test with race detector enabled
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(3, createState)
	defer pool.Close()

	const numGoroutines = 50
	const numOperations = 200
	
	var wg sync.WaitGroup
	var getCount, putCount int64

	// Launch goroutines to stress test Get/Put operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Get state
				state := pool.Get()
				atomic.AddInt64(&getCount, 1)
				
				if state == nil {
					t.Errorf("Goroutine %d: received nil state from pool", id)
					return
				}

				// Very brief work to increase contention
				runtime.Gosched()

				// Return state
				pool.Put(state)
				atomic.AddInt64(&putCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// Verify get/put counts match
	finalGetCount := atomic.LoadInt64(&getCount)
	finalPutCount := atomic.LoadInt64(&putCount)
	
	expectedOps := int64(numGoroutines * numOperations)
	if finalGetCount != expectedOps {
		t.Errorf("Expected %d Get operations, got %d", expectedOps, finalGetCount)
	}
	if finalPutCount != expectedOps {
		t.Errorf("Expected %d Put operations, got %d", expectedOps, finalPutCount)
	}
}

func TestLuaStatePoolResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	// Very small pool to force exhaustion
	pool := lua.NewLuaStatePool(2, createState)
	defer pool.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan *gopher.LState, numGoroutines)

	// All goroutines request states simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := pool.Get()
			results <- state
			
			// Hold state for a moment to create contention
			time.Sleep(10 * time.Millisecond)
			pool.Put(state)
		}()
	}

	// Collect all results
	var states []*gopher.LState
	go func() {
		wg.Wait()
		close(results)
	}()

	for state := range results {
		if state == nil {
			t.Error("Received nil state during resource exhaustion")
		}
		states = append(states, state)
	}

	if len(states) != numGoroutines {
		t.Errorf("Expected %d states, got %d", numGoroutines, len(states))
	}
}

func TestLuaStatePoolConcurrentCreateAndClose(t *testing.T) {
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(5, createState)

	var wg sync.WaitGroup
	const numGoroutines = 20

	// Start goroutines using the pool
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Try to get/put states while pool might be closing
			for j := 0; j < 10; j++ {
				state := pool.Get()
				if state != nil {
					time.Sleep(1 * time.Millisecond)
					pool.Put(state)
				}
			}
		}()
	}

	// Close pool while operations are in progress
	time.Sleep(5 * time.Millisecond)
	pool.Close()

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Goroutines did not complete after pool close")
	}
}

func TestLuaStatePoolHighFrequencyOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high frequency test in short mode")
	}

	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(10, createState)
	defer pool.Close()

	const duration = 100 * time.Millisecond
	const numGoroutines = 100

	var wg sync.WaitGroup
	var operations int64
	
	start := time.Now()

	// Launch many goroutines doing rapid get/put operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for time.Since(start) < duration {
				state := pool.Get()
				if state != nil {
					atomic.AddInt64(&operations, 1)
					pool.Put(state)
				}
			}
		}()
	}

	wg.Wait()

	totalOps := atomic.LoadInt64(&operations)
	t.Logf("Completed %d operations in %v with %d goroutines", totalOps, duration, numGoroutines)
	
	// Verify we achieved reasonable throughput
	if totalOps < 1000 {
		t.Errorf("Expected at least 1000 operations, got %d", totalOps)
	}
}

func TestLuaStatePoolMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	createState := func() *gopher.LState {
		L := gopher.NewState()
		// Execute some Lua code to use memory
		L.DoString(`
			local t = {}
			for i=1,1000 do
				t[i] = "data_" .. i
			end
		`)
		return L
	}

	pool := lua.NewLuaStatePool(20, createState)
	defer pool.Close()

	var wg sync.WaitGroup
	const numGoroutines = 50
	const numOperations = 50

	// Force garbage collection before test
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				state := pool.Get()
				if state != nil {
					// Execute some Lua code that uses memory
					state.DoString(`
						local result = {}
						for i=1,100 do
							result[i] = math.sin(i) * math.cos(i)
						end
					`)
					pool.Put(state)
				}
			}
		}()
	}

	wg.Wait()

	// Check memory hasn't grown excessively
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	memGrowth := memAfter.HeapInuse - memBefore.HeapInuse
	t.Logf("Memory growth: %d bytes", memGrowth)
	
	// Memory growth should be reasonable (allow up to 50MB growth)
	if memGrowth > 50*1024*1024 {
		t.Errorf("Excessive memory growth: %d bytes", memGrowth)
	}
}

func TestLuaStatePoolConcurrentPoolCreation(t *testing.T) {
	// Test creating multiple pools concurrently
	const numPools = 10
	var wg sync.WaitGroup

	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pools := make([]*lua.LuaStatePool, numPools)

	for i := 0; i < numPools; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			// Create pool
			pool := lua.NewLuaStatePool(3, createState)
			pools[idx] = pool
			
			// Use pool briefly
			state := pool.Get()
			if state != nil {
				pool.Put(state)
			}
		}(i)
	}

	wg.Wait()

	// Clean up all pools
	for _, pool := range pools {
		if pool != nil {
			pool.Close()
		}
	}
}

func TestLuaStatePoolStateIsolation(t *testing.T) {
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(5, createState)
	defer pool.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)

	// Each goroutine sets a different global variable
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			state := pool.Get()
			if state == nil {
				results <- "ERROR: nil state"
				return
			}
			
			// Set a unique global variable
			err := state.DoString(fmt.Sprintf("test_var = %d", id))
			if err != nil {
				results <- fmt.Sprintf("ERROR: %v", err)
				pool.Put(state)
				return
			}
			
			// Read it back
			state.DoString("result = test_var")
			value := state.GetGlobal("result")
			
			pool.Put(state)
			
			if lv, ok := value.(gopher.LNumber); ok {
				results <- fmt.Sprintf("ID_%d_GOT_%d", id, int(lv))
			} else {
				results <- fmt.Sprintf("ERROR: unexpected type %T", value)
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Collect and verify results
	resultMap := make(map[int]int)
	for result := range results {
		if strings.HasPrefix(result, "ERROR") {
			t.Errorf("Lua execution error: %s", result)
			continue
		}
		
		var id, got int
		if n, err := fmt.Sscanf(result, "ID_%d_GOT_%d", &id, &got); n == 2 && err == nil {
			resultMap[id] = got
		}
	}

	// Verify each goroutine got its own value
	for id := 0; id < numGoroutines; id++ {
		if got, exists := resultMap[id]; !exists {
			t.Errorf("Missing result for goroutine %d", id)
		} else if got != id {
			t.Errorf("Goroutine %d: expected %d, got %d (state isolation failure)", id, id, got)
		}
	}
}