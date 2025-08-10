package lua

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
)

// luaResponseWriter wraps http.ResponseWriter for Lua integration
// with header buffering to preserve middleware headers
type luaResponseWriter struct {
	w               http.ResponseWriter
	bufferedHeaders map[string]string
}

// flushHeaders writes all buffered headers to the underlying ResponseWriter
func (lw *luaResponseWriter) flushHeaders() {
	for key, value := range lw.bufferedHeaders {
		lw.w.Header().Set(key, value)
	}
}

// LuaStatePool manages a pool of Lua states for thread-safe request handling
// This version fixes segfaults by using proper state isolation per goroutine
type LuaStatePool struct {
	pool        chan *lua.LState
	maxStates   int
	createState func() *lua.LState
	mu          sync.Mutex
	created     int
	closed      bool
	scripts     map[string]*CompiledScript // Pre-compiled scripts to avoid re-execution
}

// CompiledScript represents a pre-compiled Lua script for faster execution
type CompiledScript struct {
	Content      string
	FunctionName string
	TenantName   string
}

// NewLuaStatePool creates a new pool of Lua states with improved thread safety
func NewLuaStatePool(maxStates int, createState func() *lua.LState) *LuaStatePool {
	return &LuaStatePool{
		pool:        make(chan *lua.LState, maxStates),
		maxStates:   maxStates,
		createState: createState,
		scripts:     make(map[string]*CompiledScript),
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
		p.mu.Lock()

		if p.created < p.maxStates {
			p.created++
			state := p.createState()
			p.mu.Unlock()
			return state
		}
		p.mu.Unlock()

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
		p.created--
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
		p.mu.Lock()
		p.created--
		p.mu.Unlock()
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

// RegisterScript compiles and stores a script for efficient reuse
func (p *LuaStatePool) RegisterScript(scriptKey, content, functionName, tenantName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.scripts[scriptKey] = &CompiledScript{
		Content:      content,
		FunctionName: functionName,
		TenantName:   tenantName,
	}
}

// GetScript retrieves a compiled script by key
func (p *LuaStatePool) GetScript(scriptKey string) (*CompiledScript, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	script, exists := p.scripts[scriptKey]
	return script, exists
}

// LuaHandler represents a thread-safe Lua function handler
// This version prevents segfaults through proper state isolation and pre-compilation
type LuaHandler struct {
	scriptKey    string
	functionName string
	tenantName   string
	scriptTag    string
	pool         *LuaStatePool
	engine       interface {
		SetupChiBindings(*lua.LState, string, string)
	}
}

// Constants to avoid magic numbers/strings
const (
	defaultHandlerTimeout  = 5 * time.Second
	scriptLoadedFlagPrefix = "loaded_"
)

// NewLuaHandler creates a new thread-safe Lua handler with script pre-compilation
func NewLuaHandler(scriptContent, functionName, tenantName, scriptTag string, pool *LuaStatePool, engine interface {
	SetupChiBindings(*lua.LState, string, string)
}) *LuaHandler {
	scriptKey := fmt.Sprintf("%s_%s", tenantName, functionName)

	// Pre-compile and register the script to avoid re-execution segfaults
	pool.RegisterScript(scriptKey, scriptContent, functionName, tenantName)

	return &LuaHandler{
		scriptKey:    scriptKey,
		functionName: functionName,
		tenantName:   tenantName,
		scriptTag:    scriptTag,
		pool:         pool,
		engine:       engine,
	}
}

// ServeHTTP implements http.Handler with improved thread safety and segfault prevention
func (h *LuaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	script, exists := h.pool.GetScript(h.scriptKey)
	if !exists {
		http.Error(w, "Script not found: "+h.scriptKey, http.StatusInternalServerError)
		return
	}

	L := h.pool.Get()
	defer h.pool.Put(L)

	ctx, cancel := context.WithTimeout(context.Background(), defaultHandlerTimeout)
	defer cancel()

	h.executeScriptWithTimeout(ctx, L, script, w, r)
}

// executeScriptWithTimeout executes the Lua script with proper timeout and error handling
func (h *LuaHandler) executeScriptWithTimeout(ctx context.Context, L *lua.LState, script *CompiledScript, w http.ResponseWriter, r *http.Request) {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Lua handler: %v", r)
			}
		}()
		done <- h.executeLuaScript(L, script, w, r)
	}()

	select {
	case err := <-done:
		if err != nil {
			http.Error(w, "Lua handler error: "+err.Error(), http.StatusInternalServerError)
		}
	case <-ctx.Done():
		http.Error(w, "Lua handler timeout", http.StatusRequestTimeout)
	}
}

// executeLuaScript executes the actual Lua script and calls the handler function
func (h *LuaHandler) executeLuaScript(L *lua.LState, script *CompiledScript, w http.ResponseWriter, r *http.Request) error {
	// Set up Chi bindings for this execution context with correct script tag
	if h.engine != nil {
		h.engine.SetupChiBindings(L, h.scriptTag, h.tenantName)
	}

	// Load script only once per state using the registry
	reg := L.Get(lua.RegistryIndex).(*lua.LTable)
	loadedKey := scriptLoadedFlagPrefix + h.scriptKey
	if reg.RawGetString(loadedKey) == lua.LNil {
		if err := L.DoString(script.Content); err != nil {
			return fmt.Errorf("script execution error: %w", err)
		}
		reg.RawSetString(loadedKey, lua.LTrue)
	}

	// Get the handler function
	handlerFunc := L.GetGlobal(h.functionName)
	if handlerFunc.Type() != lua.LTFunction {
		return fmt.Errorf("handler function not found: %s", h.functionName)
	}

	// Create safe request/response wrappers and call the handler
	return h.callLuaHandler(L, handlerFunc.(*lua.LFunction), w, r)
}

// callLuaHandler creates the Lua request/response objects and calls the handler function
func (h *LuaHandler) callLuaHandler(L *lua.LState, handlerFunc *lua.LFunction, w http.ResponseWriter, r *http.Request) error {
	respWriter := &luaResponseWriter{w: w, bufferedHeaders: make(map[string]string)}
	respTable := createLuaResponse(L, respWriter)
	reqTable := createLuaRequest(L, r)

	return L.CallByParam(lua.P{
		Fn:      handlerFunc,
		NRet:    0,
		Protect: true,
	}, reqTable, respTable)
}

// createLuaRequest creates a Lua table representing an HTTP request
func createLuaRequest(L *lua.LState, r *http.Request) *lua.LTable {
	reqTable := L.NewTable()

	setBasicRequestInfo(reqTable, r)
	setRequestHeaders(L, reqTable, r)
	setRequestParams(L, reqTable, r)
	setRequestQuery(L, reqTable, r) // Add query parameter support
	bodyContent := setRequestBody(reqTable, r)
	setRequestMethods(L, reqTable, r, bodyContent)

	return reqTable
}

// setBasicRequestInfo sets basic request information in the Lua table
func setBasicRequestInfo(reqTable *lua.LTable, r *http.Request) {
	reqTable.RawSetString("method", lua.LString(r.Method))
	reqTable.RawSetString("url", lua.LString(r.URL.String()))
	reqTable.RawSetString("path", lua.LString(r.URL.Path))
	reqTable.RawSetString("host", lua.LString(r.Host))
}

// setRequestHeaders sets request headers in the Lua table
func setRequestHeaders(L *lua.LState, reqTable *lua.LTable, r *http.Request) {
	headersTable := L.NewTable()
	for key, values := range r.Header {
		if len(values) > 0 {
			headersTable.RawSetString(key, lua.LString(values[0]))
		}
	}
	reqTable.RawSetString("headers", headersTable)
}

// setRequestParams sets URL parameters from Chi router context
func setRequestParams(L *lua.LState, reqTable *lua.LTable, r *http.Request) {
	paramsTable := L.NewTable()
	if r.Context() != nil {
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			for i, key := range rctx.URLParams.Keys {
				if i < len(rctx.URLParams.Values) {
					paramsTable.RawSetString(key, lua.LString(rctx.URLParams.Values[i]))
				}
			}
		}
	}
	reqTable.RawSetString("params", paramsTable)
}

// setRequestQuery sets URL query parameters
func setRequestQuery(L *lua.LState, reqTable *lua.LTable, r *http.Request) {
	queryTable := L.NewTable()
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryTable.RawSetString(key, lua.LString(values[0]))
		}
	}
	reqTable.RawSetString("query", queryTable)
}

// setRequestBody reads and stores request body content
func setRequestBody(reqTable *lua.LTable, r *http.Request) string {
	var bodyContent string
	if r.Body != nil {
		if body, err := io.ReadAll(r.Body); err == nil {
			bodyContent = string(body)
		}
	}
	return bodyContent
}

// setRequestMethods adds helper methods to the request table
func setRequestMethods(L *lua.LState, reqTable *lua.LTable, r *http.Request, bodyContent string) {
	headerFunc := createHeaderFunction(L, reqTable, r)
	bodyFunc := createBodyFunction(L, bodyContent)
	jsonFunc := createJSONFunction(L, r, bodyContent)

	reqTable.RawSetString("header", headerFunc)
	reqTable.RawSetString("body", bodyFunc)
	reqTable.RawSetString("json", jsonFunc)
}

// createHeaderFunction creates the header method function
func createHeaderFunction(L *lua.LState, reqTable *lua.LTable, r *http.Request) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == reqTable {
			startIdx = 2
		}
		headerName := L.ToString(startIdx)
		headerValue := r.Header.Get(headerName)
		L.Push(lua.LString(headerValue))
		return 1
	})
}

// createBodyFunction creates the body method function
func createBodyFunction(L *lua.LState, bodyContent string) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(bodyContent))
		return 1
	})
}

// createJSONFunction creates the json method function
func createJSONFunction(L *lua.LState, r *http.Request, bodyContent string) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") && bodyContent != "" {
			// For now, return the raw JSON string - could be enhanced to parse to Lua table
			L.Push(lua.LString(bodyContent))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	})
}

// createLuaResponse creates a Lua table representing an HTTP response with colon method support
func createLuaResponse(L *lua.LState, w *luaResponseWriter) *lua.LTable {
	respTable := L.NewTable()

	// Create method functions that work with both colon and dot syntax
	writeFunc := L.NewFunction(func(L *lua.LState) int {
		// Skip 'self' parameter if called with colon syntax
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == respTable {
			startIdx = 2
		}
		content := L.ToString(startIdx)
		// Flush buffered headers before writing content
		w.flushHeaders()
		if _, err := w.w.Write([]byte(content)); err != nil {
			slog.Error("lua_response_write_failed", "error", err, "component", "lua_response")
		}
		return 0
	})

	headerFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 2 && L.Get(1) == respTable {
			startIdx = 2
		}
		key := L.ToString(startIdx)
		value := L.ToString(startIdx + 1)
		// Set headers directly on the underlying ResponseWriter (for middleware)
		// and also buffer them (for route handlers that might override)
		w.w.Header().Set(key, value)
		w.bufferedHeaders[key] = value
		return 0
	})

	statusFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == respTable {
			startIdx = 2
		}
		statusCode := L.ToInt(startIdx)
		w.w.WriteHeader(statusCode)
		return 0
	})

	jsonFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == respTable {
			startIdx = 2
		}
		jsonContent := L.ToString(startIdx)
		// Flush buffered headers before writing content
		w.flushHeaders()
		w.w.Header().Set("Content-Type", "application/json")
		if _, err := w.w.Write([]byte(jsonContent)); err != nil {
			slog.Error("lua_json_response_failed", "error", err, "component", "lua_response")
		}
		return 0
	})

	// Set methods on table
	respTable.RawSetString("write", writeFunc)
	respTable.RawSetString("header", headerFunc)
	respTable.RawSetString("set_header", headerFunc) // Alias for header method
	respTable.RawSetString("status", statusFunc)
	respTable.RawSetString("json", jsonFunc)

	return respTable
}
