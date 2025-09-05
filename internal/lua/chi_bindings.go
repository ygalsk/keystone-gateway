// Package lua provides Lua-to-Chi bridge functions that allow Lua scripts
// to register routes, middleware, and route groups directly with the Chi router.
package lua

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/routing"
)

// CacheEntry represents a cached value with expiration
type CacheEntry struct {
	Value     lua.LValue
	ExpiresAt int64
	CreatedAt int64
}

// LuaCache provides thread-safe caching with TTL support
type LuaCache struct {
	data    *sync.Map
	hits    int64
	misses  int64
	cleanMu sync.Mutex
}

// Global cache instance
var globalCache = &LuaCache{
	data: &sync.Map{},
}

// Helper functions for cache operations
func (c *LuaCache) isExpired(entry *CacheEntry) bool {
	return time.Now().Unix() >= entry.ExpiresAt
}

func (c *LuaCache) get(key string) (lua.LValue, bool) {
	if val, ok := c.data.Load(key); ok {
		if entry, ok := val.(*CacheEntry); ok {
			if !c.isExpired(entry) {
				atomic.AddInt64(&c.hits, 1)
				return entry.Value, true
			}
			c.data.Delete(key)
		}
	}
	atomic.AddInt64(&c.misses, 1)
	return lua.LNil, false
}

func (c *LuaCache) set(key string, value lua.LValue, ttlSeconds int) {
	if ttlSeconds <= 0 {
		ttlSeconds = 3600 // Default 1 hour
	}
	
	entry := &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Unix() + int64(ttlSeconds),
		CreatedAt: time.Now().Unix(),
	}
	
	c.data.Store(key, entry)
}

func (c *LuaCache) delete(key string) {
	c.data.Delete(key)
}

func (c *LuaCache) exists(key string) bool {
	if val, ok := c.data.Load(key); ok {
		if entry, ok := val.(*CacheEntry); ok {
			if !c.isExpired(entry) {
				return true
			}
			c.data.Delete(key)
		}
	}
	return false
}

func (c *LuaCache) ttl(key string) int64 {
	if val, ok := c.data.Load(key); ok {
		if entry, ok := val.(*CacheEntry); ok {
			remaining := entry.ExpiresAt - time.Now().Unix()
			if remaining > 0 {
				return remaining
			}
			c.data.Delete(key)
		}
	}
	return -1
}

func (c *LuaCache) addIfNotExists(key string, value lua.LValue, ttlSeconds int) bool {
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	
	entry := &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Unix() + int64(ttlSeconds),
		CreatedAt: time.Now().Unix(),
	}
	
	_, loaded := c.data.LoadOrStore(key, entry)
	return !loaded // true if successfully added, false if key already exists
}

func (c *LuaCache) clear() {
	c.data = &sync.Map{}
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

func (c *LuaCache) stats() (int64, int64) {
	return atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses)
}

func (c *LuaCache) cleanExpired() {
	c.cleanMu.Lock()
	defer c.cleanMu.Unlock()
	
	toDelete := make([]string, 0)
	c.data.Range(func(key, value interface{}) bool {
		if entry, ok := value.(*CacheEntry); ok {
			if c.isExpired(entry) {
				toDelete = append(toDelete, key.(string))
			}
		}
		return true
	})
	
	for _, key := range toDelete {
		c.data.Delete(key)
	}
}

// setupChiBindings sets up Lua bindings for Chi router functions
func (e *Engine) SetupChiBindings(L *lua.LState, scriptTag, tenantName string) {
	// Register Lua functions that can be called from scripts
	L.SetGlobal("chi_route", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiRoute(L, scriptTag, tenantName)
	}))
	L.SetGlobal("chi_middleware", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiMiddleware(L, scriptTag, tenantName)
	}))
	L.SetGlobal("chi_group", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiGroup(L, tenantName)
	}))
	L.SetGlobal("chi_param", L.NewFunction(func(L *lua.LState) int {
		// This will be overridden in the actual handler context with real parameter values
		requestTable := L.ToTable(1)
		paramName := L.ToString(2)

		if requestTable != nil {
			if paramsTable := requestTable.RawGetString("params"); paramsTable != lua.LNil {
				if paramTable, ok := paramsTable.(*lua.LTable); ok {
					if param := paramTable.RawGetString(paramName); param != lua.LNil {
						L.Push(param)
						return 1
					}
				}
			}
		}

		// Default fallback
		L.Push(lua.LString(""))
		return 1
	}))

	// Add HTTP POST function for OAuth and other HTTP requests
	L.SetGlobal("http_post", L.NewFunction(createHTTPPostFunction()))
	// Add HTTP GET function
	L.SetGlobal("http_get", L.NewFunction(createHTTPGetFunction()))
	// Add environment variable getter
	L.SetGlobal("get_env", L.NewFunction(createGetEnvFunction()))
	
	// Add cache functions
	L.SetGlobal("cache_get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		if value, found := globalCache.get(key); found {
			L.Push(value)
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	L.SetGlobal("cache_set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.Get(2)
		ttl := L.OptInt(3, 3600) // Default 1 hour if not specified
		globalCache.set(key, value, ttl)
		return 0
	}))

	L.SetGlobal("cache_delete", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		globalCache.delete(key)
		return 0
	}))

	L.SetGlobal("cache_exists", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		exists := globalCache.exists(key)
		L.Push(lua.LBool(exists))
		return 1
	}))

	L.SetGlobal("cache_ttl", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		ttl := globalCache.ttl(key)
		L.Push(lua.LNumber(ttl))
		return 1
	}))

	L.SetGlobal("cache_add", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.Get(2)
		ttl := L.OptInt(3, 3600)
		success := globalCache.addIfNotExists(key, value, ttl)
		L.Push(lua.LBool(success))
		return 1
	}))

	L.SetGlobal("cache_clear", L.NewFunction(func(L *lua.LState) int {
		globalCache.clear()
		return 0
	}))

	L.SetGlobal("cache_stats", L.NewFunction(func(L *lua.LState) int {
		hits, misses := globalCache.stats()
		statsTable := L.NewTable()
		statsTable.RawSetString("hits", lua.LNumber(hits))
		statsTable.RawSetString("misses", lua.LNumber(misses))
		L.Push(statsTable)
		return 1
	}))
}

// luaChiRoute handles route registration from Lua: chi_route(method, pattern, handler)
func (e *Engine) luaChiRoute(L *lua.LState, scriptTag, tenantName string) int {
	method, pattern, handlerFunc := e.extractRouteArgs(L)
	if method == "" || pattern == "" || handlerFunc == nil {
		L.ArgError(1, "chi_route requires method, pattern, and handler function")
		return 0
	}

	groupPattern := e.getGroupPattern(L)
	fullPattern := e.buildRoutePattern(pattern, groupPattern)
	functionName := e.generateHandlerName(method, fullPattern, L)

	L.SetGlobal(functionName, handlerFunc)

	scriptContent, exists := e.GetScript(scriptTag)
	if !exists {
		L.RaiseError("Script not found: %s", scriptTag)
		return 0
	}

	luaHandler := NewLuaHandler(scriptContent, functionName, tenantName, scriptTag, e.statePool, e)

	if err := e.registerRouteWithRegistry(tenantName, method, fullPattern, groupPattern, luaHandler); err != nil {
		L.RaiseError("Failed to register route: %v", err)
	}

	return 0
}

// extractRouteArgs extracts route arguments from Lua state
func (e *Engine) extractRouteArgs(L *lua.LState) (string, string, *lua.LFunction) {
	method := L.ToString(1)
	pattern := L.ToString(2)
	handlerFunc := L.ToFunction(3)
	return method, pattern, handlerFunc
}

// getGroupPattern retrieves current group pattern from Lua context
func (e *Engine) getGroupPattern(L *lua.LState) string {
	if groupCtx := L.GetGlobal("__current_group_pattern"); groupCtx != lua.LNil {
		return groupCtx.String()
	}
	return ""
}

// buildRoutePattern combines group pattern with route pattern
func (e *Engine) buildRoutePattern(pattern, groupPattern string) string {
	if groupPattern != "" {
		return groupPattern + pattern
	}
	return pattern
}

// generateHandlerName creates a unique handler function name
func (e *Engine) generateHandlerName(method, pattern string, L *lua.LState) string {
	return fmt.Sprintf("handler_%s_%s_%d", method, pattern, L.GetTop())
}

// generateMiddlewareName creates a unique middleware function name
func (e *Engine) generateMiddlewareName(pattern string, L *lua.LState) string {
	return fmt.Sprintf("middleware_%s_%d", pattern, L.GetTop())
}

// registerRouteWithRegistry registers a route with the route registry
func (e *Engine) registerRouteWithRegistry(tenantName, method, pattern, groupPattern string, luaHandler *LuaHandler) error {
	return e.routeRegistry.RegisterRoute(routing.RouteDefinition{
		TenantName:   tenantName,
		Method:       method,
		Pattern:      pattern,
		GroupPattern: groupPattern,
		Handler:      luaHandler.ServeHTTP,
	})
}

// luaChiMiddleware handles middleware registration: chi_middleware(pattern, middleware_func)
func (e *Engine) luaChiMiddleware(L *lua.LState, scriptTag, tenantName string) int {
	pattern, middlewareFunc := e.extractMiddlewareArgs(L)
	if pattern == "" || middlewareFunc == nil {
		L.ArgError(1, "chi_middleware requires pattern and middleware function")
		return 0
	}

	// Create direct Lua middleware handler
	functionName := e.generateMiddlewareName(pattern, L)
	L.SetGlobal(functionName, middlewareFunc)

	scriptContent, exists := e.GetScript(scriptTag)
	if !exists {
		L.RaiseError("Script not found: %s", scriptTag)
		return 0
	}

	// Create middleware that executes the cached Lua function directly
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get state from pool
			L := e.statePool.Get()
			defer e.statePool.Put(L)

			// Set up Chi bindings for this execution context
			e.SetupChiBindings(L, scriptTag, tenantName)

			// Load script only once per state using the registry (same pattern as route handlers)
			reg := L.Get(lua.RegistryIndex).(*lua.LTable)
			loadedKey := "script_loaded:" + scriptTag
			if reg.RawGetString(loadedKey) == lua.LNil {
				if err := L.DoString(scriptContent); err != nil {
					http.Error(w, fmt.Sprintf("Middleware script error: %v", err), http.StatusInternalServerError)
					return
				}
				reg.RawSetString(loadedKey, lua.LTrue)
			}

			// Get the middleware function from global scope
			middlewareFunc := L.GetGlobal(functionName)
			if middlewareFunc == lua.LNil {
				http.Error(w, "Middleware function not found", http.StatusInternalServerError)
				return
			}

			func_ptr, ok := middlewareFunc.(*lua.LFunction)
			if !ok {
				http.Error(w, "Invalid middleware function", http.StatusInternalServerError)
				return
			}

			// Create Lua request/response tables
			respWriter := &luaResponseWriter{w: w}
			respTable := createLuaResponse(L, respWriter)
			reqTable := createLuaRequest(L, r)

			// Create next function
			nextCalled := false
			nextFunc := L.NewFunction(func(L *lua.LState) int {
				nextCalled = true
				return 0
			})

			// Execute middleware function
			err := L.CallByParam(lua.P{
				Fn:      func_ptr,
				NRet:    0,
				Protect: true,
			}, reqTable, respTable, nextFunc)

			if err != nil {
				http.Error(w, fmt.Sprintf("Middleware execution error: %v", err), http.StatusInternalServerError)
				return
			}

			// Apply any changes from Lua request table back to Go request
			applyLuaRequestChanges(reqTable, r)

			// Call next handler if middleware called next()
			if nextCalled && next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}

	if err := e.registerMiddlewareWithRegistry(tenantName, pattern, "", middleware); err != nil {
		L.RaiseError("Failed to register middleware: %v", err)
	}
	return 0
}

// extractMiddlewareArgs extracts middleware arguments from Lua state
func (e *Engine) extractMiddlewareArgs(L *lua.LState) (string, *lua.LFunction) {
	pattern := L.ToString(1)
	middlewareFunc := L.ToFunction(2)
	return pattern, middlewareFunc
}

// registerMiddlewareWithRegistry registers middleware with the route registry
func (e *Engine) registerMiddlewareWithRegistry(tenantName, pattern, groupPattern string, middleware func(http.Handler) http.Handler) error {
	return e.routeRegistry.RegisterMiddleware(routing.MiddlewareDefinition{
		TenantName:   tenantName,
		Pattern:      pattern,
		GroupPattern: groupPattern,
		Middleware:   middleware,
	})
}

// luaChiGroup handles route group registration: chi_group(pattern, setup_func)
func (e *Engine) luaChiGroup(L *lua.LState, tenantName string) int {
	pattern, setupFunc := e.extractGroupArgs(L)
	if pattern == "" || setupFunc == nil {
		L.ArgError(1, "chi_group requires pattern and setup function")
		return 0
	}

	oldGroupContext := L.GetGlobal("__current_group_pattern")
	fullPattern := e.buildGroupPattern(pattern, oldGroupContext)

	e.executeGroupSetup(L, setupFunc, fullPattern, oldGroupContext)
	return 0
}

// extractGroupArgs extracts group arguments from Lua state
func (e *Engine) extractGroupArgs(L *lua.LState) (string, *lua.LFunction) {
	pattern := L.ToString(1)
	setupFunc := L.ToFunction(2)
	return pattern, setupFunc
}

// buildGroupPattern combines parent group pattern with current pattern
func (e *Engine) buildGroupPattern(pattern string, oldGroupContext lua.LValue) string {
	if oldGroupContext != lua.LNil {
		parentPattern := oldGroupContext.String()
		if parentPattern != "" {
			return parentPattern + pattern
		}
	}
	return pattern
}

// executeGroupSetup executes the group setup function with proper context management
func (e *Engine) executeGroupSetup(L *lua.LState, setupFunc *lua.LFunction, fullPattern string, oldGroupContext lua.LValue) {
	// Set new group context
	L.SetGlobal("__current_group_pattern", lua.LString(fullPattern))

	// Execute setup function
	err := L.CallByParam(lua.P{
		Fn:      setupFunc,
		NRet:    0,
		Protect: true,
	})

	// Restore previous group context
	L.SetGlobal("__current_group_pattern", oldGroupContext)

	if err != nil {
		L.RaiseError("Failed to execute group setup function: %v", err)
	}
}

// createHTTPPostFunction creates a simple HTTP POST helper for OAuth and other HTTP requests
func createHTTPPostFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		url := L.CheckString(1)
		body := L.CheckString(2)

		// Optional 3rd param: headers table
		var contentType string = "application/x-www-form-urlencoded"
		if L.GetTop() >= 3 {
			if tbl, ok := L.Get(3).(*lua.LTable); ok {
				// Look for "Content-Type" header
				tbl.ForEach(func(k, v lua.LValue) {
					if strings.ToLower(k.String()) == "content-type" {
						contentType = v.String()
					}
				})
			}
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(url, contentType, strings.NewReader(body))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer resp.Body.Close()

		responseBody, _ := io.ReadAll(resp.Body)
		L.Push(lua.LString(string(responseBody)))
		L.Push(lua.LNumber(resp.StatusCode))
		return 2
	}
}

// createGetEnvFunction creates a helper to read environment variables
func createGetEnvFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		value := os.Getenv(key)
		L.Push(lua.LString(value))
		return 1
	}
}

// createHTTPGetFunction creates a simple HTTP GET helper with optional headers table param
func createHTTPGetFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		url := L.CheckString(1)

		// Optional headers
		headers := make(http.Header)
		if L.GetTop() >= 2 {
			if tbl, ok := L.Get(2).(*lua.LTable); ok {
				tbl.ForEach(func(k, v lua.LValue) {
					headers.Set(k.String(), v.String())
				})
			}
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		for k, vals := range headers {
			for _, val := range vals {
				req.Header.Add(k, val)
			}
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)

		// Create Lua table for headers
		luaHeaders := L.NewTable()
		for k, vals := range resp.Header {
			if len(vals) == 1 {
				luaHeaders.RawSetString(k, lua.LString(vals[0]))
			} else {
				valTable := L.NewTable()
				for i, v := range vals {
					valTable.RawSetInt(i+1, lua.LString(v))
				}
				luaHeaders.RawSetString(k, valTable)
			}
		}

		L.Push(lua.LString(string(bodyBytes)))  // body
		L.Push(luaHeaders)                      // headers table
		L.Push(lua.LNumber(resp.StatusCode))    // status code
		return 3
	}
}
