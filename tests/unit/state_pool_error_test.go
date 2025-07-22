package unit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gopher "github.com/yuin/gopher-lua"
	"keystone-gateway/internal/lua"
)

func TestLuaStatePoolExhaustion(t *testing.T) {
	// Create a state pool with limited capacity
	maxStates := 2
	createState := func() *gopher.LState {
		L := gopher.NewState()
		return L
	}

	pool := lua.NewLuaStatePool(maxStates, createState)
	defer pool.Close()

	// Get all available states
	state1 := pool.Get()
	state2 := pool.Get()

	// Getting a third state should block (we'll test with timeout)
	done := make(chan *gopher.LState, 1)
	go func() {
		state3 := pool.Get()
		done <- state3
	}()

	// Should not receive third state immediately
	select {
	case <-done:
		t.Error("expected pool to block when exhausted")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior
	}

	// Return one state to unblock the waiting goroutine
	pool.Put(state1)

	// Now should receive the third state
	select {
	case state3 := <-done:
		if state3 == nil {
			t.Error("expected to receive valid state after returning one to pool")
		}
		pool.Put(state3)
	case <-time.After(1 * time.Second):
		t.Error("expected to receive state after returning one to pool")
	}

	pool.Put(state2)
}

func TestLuaStatePoolNilStateHandling(t *testing.T) {
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(2, createState)
	defer pool.Close()

	// Test putting nil state (should not crash)
	pool.Put(nil)

	// Pool should still work normally
	state := pool.Get()
	if state == nil {
		t.Error("expected to get valid state after putting nil")
	}
	pool.Put(state)
}

func TestLuaHandlerScriptNotFound(t *testing.T) {
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(1, createState)
	defer pool.Close()

	// Note: LuaHandler testing would require more complex setup
	// For now, we focus on testing the state pool's script management

	// For now, test script registration and retrieval
	scriptKey := "nonexistent_script"
	script, exists := pool.GetScript(scriptKey)
	if exists {
		t.Error("expected script to not exist")
	}
	if script != nil {
		t.Error("expected nil script for nonexistent key")
	}
}

func TestLuaHandlerPanicRecovery(t *testing.T) {
	createState := func() *gopher.LState {
		L := gopher.NewState()
		return L
	}

	pool := lua.NewLuaStatePool(2, createState)
	defer pool.Close()

	// Register a script that will cause panic
	panicScript := `
function test_handler(response, request)
    -- This will cause a Lua error
    error("Intentional error for testing")
end
`

	pool.RegisterScript("panic_test", panicScript, "test_handler", "test_tenant")

	// Create a simple engine mock for testing
	engine := &mockEngine{}

	handler := lua.NewLuaHandler(panicScript, "test_handler", "test_tenant", "panic_test", pool, engine)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// This should not crash the program
	handler.ServeHTTP(w, req)

	// Should receive an error response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 status code, got %d", w.Code)
	}

	respBody := w.Body.String()
	if !strings.Contains(respBody, "Lua handler error") {
		t.Errorf("expected error message in response, got: %s", respBody)
	}
}

func TestLuaHandlerTimeout(t *testing.T) {
	createState := func() *gopher.LState {
		L := gopher.NewState()
		return L
	}

	pool := lua.NewLuaStatePool(2, createState)
	defer pool.Close()

	// Register a script that will run indefinitely
	infiniteScript := `
function test_handler(response, request)
    -- Infinite loop to trigger timeout
    while true do
        -- This will run forever
    end
end
`

	pool.RegisterScript("infinite_test", infiniteScript, "test_handler", "test_tenant")

	engine := &mockEngine{}
	handler := lua.NewLuaHandler(infiniteScript, "test_handler", "test_tenant", "infinite_test", pool, engine)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(w, req)
	duration := time.Since(start)

	// Should timeout within reasonable time (5 seconds + some buffer)
	if duration > 10*time.Second {
		t.Errorf("handler took too long to timeout: %v", duration)
	}

	// Should receive timeout error
	if w.Code != http.StatusRequestTimeout {
		t.Errorf("expected 408 status code, got %d", w.Code)
	}

	respBody := w.Body.String()
	if !strings.Contains(respBody, "timeout") {
		t.Errorf("expected timeout message in response, got: %s", respBody)
	}
}

func TestLuaHandlerInvalidFunction(t *testing.T) {
	createState := func() *gopher.LState {
		L := gopher.NewState()
		return L
	}

	pool := lua.NewLuaStatePool(2, createState)
	defer pool.Close()

	// Register a script without the expected function
	scriptWithoutHandler := `
-- This script doesn't have the expected handler function
function wrong_function_name(response, request)
    response:write("This won't be called")
end
`

	pool.RegisterScript("no_handler_test", scriptWithoutHandler, "test_handler", "test_tenant")

	engine := &mockEngine{}
	handler := lua.NewLuaHandler(scriptWithoutHandler, "test_handler", "test_tenant", "no_handler_test", pool, engine)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should receive an error about missing function
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 status code, got %d", w.Code)
	}

	respBody := w.Body.String()
	if !strings.Contains(respBody, "handler function not found") {
		t.Errorf("expected function not found error, got: %s", respBody)
	}
}

func TestLuaStatePoolConcurrentAccess(t *testing.T) {
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(5, createState)
	defer pool.Close()

	const numGoroutines = 20
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines to stress test the pool
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Get state
				state := pool.Get()
				if state == nil {
					t.Errorf("received nil state from pool")
					return
				}

				// Simulate some work
				time.Sleep(1 * time.Millisecond)

				// Return state
				pool.Put(state)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(30 * time.Second):
			t.Fatal("concurrent access test timed out")
		}
	}
}

func TestLuaStatePoolClosedPool(t *testing.T) {
	createState := func() *gopher.LState {
		return gopher.NewState()
	}

	pool := lua.NewLuaStatePool(2, createState)

	// Get a state first
	state := pool.Get()
	if state == nil {
		t.Fatal("expected to get valid state")
	}

	// Close the pool
	pool.Close()

	// Trying to put state back to closed pool should not panic
	pool.Put(state)

	// Note: Getting from closed pool might block indefinitely,
	// so we don't test that scenario to avoid hanging tests
}

// mockEngine implements the engine interface for testing
type mockEngine struct{}

func (e *mockEngine) SetupChiBindings(L *gopher.LState, scriptTag, tenantName string) {
	// Mock implementation - do nothing for these tests
}
