package lua

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	httputil "keystone-gateway/internal/http"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
)

// httpClient is a shared HTTP client for all Lua bindings
var httpClient = &http.Client{
	Timeout:   5 * time.Second,
	Transport: httputil.CreateTransport(),
}

// luaCacheKey is the context key for Lua request caching
type luaCacheKey string

const luaCacheContext luaCacheKey = "lua_cache"

// getLuaCache retrieves the Lua cache from context, creating if necessary
func getLuaCache(ctx context.Context) map[string]string {
	if cache, ok := ctx.Value(luaCacheContext).(map[string]string); ok {
		return cache
	}
	return make(map[string]string)
}

// setLuaCache stores the Lua cache in context
func setLuaCache(ctx context.Context, cache map[string]string) context.Context {
	return context.WithValue(ctx, luaCacheContext, cache)
}

// SetupChiBindings exposes Lua functions for Chi routing and middleware
func (e *Engine) SetupChiBindings(L *lua.LState, r chi.Router) {
	// --- ROUTES ---
	L.SetGlobal("chi_route", L.NewFunction(func(L *lua.LState) int {
		method := L.CheckString(1)
		pattern := L.CheckString(2)
		handler := L.CheckFunction(3)

		r.Method(method, pattern, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			L := e.statePool.Get()
			defer e.statePool.Put(L)

			reqUD := L.NewUserData()
			reqUD.Value = req
			resUD := L.NewUserData()
			resUD.Value = w

			if err := L.CallByParam(lua.P{
				Fn:      handler,
				NRet:    0,
				Protect: true,
			}, reqUD, resUD); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))

		return 0
	}))

	// --- MIDDLEWARE ---
	L.SetGlobal("chi_middleware", L.NewFunction(func(L *lua.LState) int {
		handler := L.CheckFunction(1)

		// Safety check: catch panic if middleware is registered after routes
		defer func() {
			if r := recover(); r != nil {
				if panicStr := fmt.Sprint(r); strings.Contains(panicStr, "middlewares must be defined before routes") {
					L.RaiseError("middleware must be registered before routes - ensure Lua scripts execute before gateway routes are set up")
				} else {
					panic(r) // re-panic if it's a different error
				}
			}
		}()

		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				L := e.statePool.Get()
				defer e.statePool.Put(L)

				nextCalled := false
				nextFunc := L.NewFunction(func(L *lua.LState) int {
					nextCalled = true
					return 0
				})

				reqUD := L.NewUserData()
				reqUD.Value = req
				resUD := L.NewUserData()
				resUD.Value = w

				if err := L.CallByParam(lua.P{
					Fn:      handler,
					NRet:    0,
					Protect: true,
				}, reqUD, resUD, nextFunc); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if nextCalled {
					next.ServeHTTP(w, req)
				}
			})
		})
		return 0
	}))

	// --- ROUTE GROUPS ---
	L.SetGlobal("chi_group", L.NewFunction(func(L *lua.LState) int {
		setupFunc := L.CheckFunction(1)
		r.Group(func(gr chi.Router) {
			e.SetupChiBindings(L, gr)
			if err := L.CallByParam(lua.P{
				Fn:      setupFunc,
				NRet:    0,
				Protect: true,
			}); err != nil {
				L.RaiseError("Group setup error: %v", err)
			}
		})
		return 0
	}))

	L.SetGlobal("chi_route_group", L.NewFunction(func(L *lua.LState) int {
		pattern := L.CheckString(1)
		setupFunc := L.CheckFunction(2)
		r.Route(pattern, func(gr chi.Router) {
			e.SetupChiBindings(L, gr)
			if err := L.CallByParam(lua.P{
				Fn:      setupFunc,
				NRet:    0,
				Protect: true,
			}); err != nil {
				L.RaiseError("Route group setup error: %v", err)
			}
		})
		return 0
	}))

	L.SetGlobal("chi_mount", L.NewFunction(func(L *lua.LState) int {
		pattern := L.CheckString(1)
		handler := L.CheckFunction(2)

		// Create a simple handler for the mounted route
		mountHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			L := e.statePool.Get()
			defer e.statePool.Put(L)

			reqUD := L.NewUserData()
			reqUD.Value = req
			resUD := L.NewUserData()
			resUD.Value = w

			if err := L.CallByParam(lua.P{
				Fn:      handler,
				NRet:    0,
				Protect: true,
			}, reqUD, resUD); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})

		r.Mount(pattern, mountHandler)
		return 0
	}))

	// --- PARAMS ---
	L.SetGlobal("chi_param", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		key := L.CheckString(2)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("chi_param: first argument must be http.Request")
			return 0
		}
		L.Push(lua.LString(chi.URLParam(req, key)))
		return 1
	}))

	// --- CONTEXT CACHING ---
	L.SetGlobal("chi_context_set", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		key := L.CheckString(2)
		value := L.CheckString(3)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("chi_context_set: first argument must be http.Request")
			return 0
		}
		cache := getLuaCache(req.Context())
		newCache := make(map[string]string)
		for k, v := range cache {
			newCache[k] = v
		}
		newCache[key] = value

		// Create new request with updated context instead of mutating original
		newReq := req.WithContext(setLuaCache(req.Context(), newCache))
		reqUD.Value = newReq
		return 0
	}))

	L.SetGlobal("chi_context_get", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		key := L.CheckString(2)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("chi_context_get: first argument must be http.Request")
			return 0
		}
		cache := getLuaCache(req.Context())
		if value, exists := cache[key]; exists {
			L.Push(lua.LString(value))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	// --- ERROR HANDLERS ---
	L.SetGlobal("chi_not_found", L.NewFunction(func(L *lua.LState) int {
		handler := L.CheckFunction(1)
		r.NotFound(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			L := e.statePool.Get()
			defer e.statePool.Put(L)

			reqUD := L.NewUserData()
			reqUD.Value = req
			resUD := L.NewUserData()
			resUD.Value = w

			if err := L.CallByParam(lua.P{
				Fn:      handler,
				NRet:    0,
				Protect: true,
			}, reqUD, resUD); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		return 0
	}))

	L.SetGlobal("chi_method_not_allowed", L.NewFunction(func(L *lua.LState) int {
		handler := L.CheckFunction(1)
		r.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			L := e.statePool.Get()
			defer e.statePool.Put(L)

			reqUD := L.NewUserData()
			reqUD.Value = req
			resUD := L.NewUserData()
			resUD.Value = w

			if err := L.CallByParam(lua.P{
				Fn:      handler,
				NRet:    0,
				Protect: true,
			}, reqUD, resUD); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		return 0
	}))

	// --- REQUEST PROPERTIES ---
	L.SetGlobal("request_url", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("request_url: first argument must be http.Request")
			return 0
		}
		L.Push(lua.LString(req.URL.String()))
		return 1
	}))

	L.SetGlobal("request_body", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("request_body: first argument must be http.Request")
			return 0
		}
		// Check cache first to avoid multiple reads
		cache := getLuaCache(req.Context())
		if body, exists := cache["_request_body"]; exists {
			L.Push(lua.LString(body))
			return 1
		}

		// Read body once and cache it with size limit
		if req.Body != nil {
			// Use configured body size limit
			requestLimits := e.config.GetRequestLimits()
			limitedReader := &io.LimitedReader{
				R: req.Body,
				N: requestLimits.MaxBodySize + 1, // Read one extra byte to detect oversized requests
			}

			body, err := io.ReadAll(limitedReader)
			req.Body.Close()
			if err != nil {
				L.Push(lua.LString(""))
				return 1
			}

			// Check if body exceeds size limit
			if int64(len(body)) > requestLimits.MaxBodySize {
				L.RaiseError("request body exceeds maximum size limit of %d bytes", requestLimits.MaxBodySize)
				return 0
			}

			bodyStr := string(body)
			newCache := make(map[string]string)
			for k, v := range cache {
				newCache[k] = v
			}
			newCache["_request_body"] = bodyStr

			// Create new request with updated context instead of mutating original
			newReq := req.WithContext(setLuaCache(req.Context(), newCache))
			// Restore body for other consumers
			newReq.Body = io.NopCloser(strings.NewReader(bodyStr))
			reqUD.Value = newReq
			L.Push(lua.LString(bodyStr))
			return 1
		}
		L.Push(lua.LString(""))
		return 1
	}))

	L.SetGlobal("request_method", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("request_method: first argument must be http.Request")
			return 0
		}
		L.Push(lua.LString(req.Method))
		return 1
	}))

	L.SetGlobal("request_header", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		headerName := L.CheckString(2)
		req, ok := reqUD.Value.(*http.Request)
		if !ok {
			L.RaiseError("request_header: first argument must be http.Request")
			return 0
		}
		L.Push(lua.LString(req.Header.Get(headerName)))
		return 1
	}))

	// --- RESPONSE FUNCTIONS ---
	L.SetGlobal("response_status", L.NewFunction(func(L *lua.LState) int {
		resUD := L.CheckUserData(1)
		statusCode := L.CheckInt(2)
		w, ok := resUD.Value.(http.ResponseWriter)
		if !ok {
			L.RaiseError("response_status: first argument must be http.ResponseWriter")
			return 0
		}
		w.WriteHeader(statusCode)
		return 0
	}))

	L.SetGlobal("response_header", L.NewFunction(func(L *lua.LState) int {
		resUD := L.CheckUserData(1)
		headerName := L.CheckString(2)
		headerValue := L.CheckString(3)
		w, ok := resUD.Value.(http.ResponseWriter)
		if !ok {
			L.RaiseError("response_header: first argument must be http.ResponseWriter")
			return 0
		}
		w.Header().Set(headerName, headerValue)
		return 0
	}))

	L.SetGlobal("response_write", L.NewFunction(func(L *lua.LState) int {
		resUD := L.CheckUserData(1)
		content := L.CheckString(2)
		w, ok := resUD.Value.(http.ResponseWriter)
		if !ok {
			L.RaiseError("response_write: first argument must be http.ResponseWriter")
			return 0
		}
		w.Write([]byte(content))
		return 0
	}))

	// --- HTTP CLIENT FUNCTIONS ---
	L.SetGlobal("http_get", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)

		// Optional headers table (can be arg 2 or 3 depending on whether request context is provided)
		var headers map[string]string
		var baseCtx context.Context = context.Background()

		// Check if first optional arg is a request UserData (for context propagation)
		startArg := 2
		if L.GetTop() >= 2 {
			if reqUD := L.Get(2); reqUD.Type() == lua.LTUserData {
				if req, ok := reqUD.(*lua.LUserData).Value.(*http.Request); ok {
					baseCtx = req.Context()
					startArg = 3 // Headers would be in arg 3 if request context is provided
				}
			}
		}

		// Parse headers table if provided
		if L.GetTop() >= startArg && L.Get(startArg).Type() == lua.LTTable {
			headers = make(map[string]string)
			L.Get(startArg).(*lua.LTable).ForEach(func(k, v lua.LValue) {
				if key, ok := k.(lua.LString); ok {
					if val, ok := v.(lua.LString); ok {
						headers[string(key)] = string(val)
					}
				}
			})
		}

		// Create request with timeout context, propagating request context if available
		ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			L.Push(L.NewTable()) // empty headers table
			return 3
		}

		// Add headers if provided
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			L.Push(L.NewTable()) // empty headers table
			return 3
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			// Ensure response body is closed even on read error
			resp.Body.Close()
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			L.Push(L.NewTable()) // empty headers table
			return 3
		}

		// Create response headers table
		respHeaders := L.NewTable()
		for name, values := range resp.Header {
			if len(values) > 0 {
				respHeaders.RawSetString(name, lua.LString(values[0]))
			}
		}

		L.Push(lua.LString(string(body)))
		L.Push(lua.LNumber(resp.StatusCode))
		L.Push(respHeaders)
		return 3
	}))

	L.SetGlobal("http_post", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		postBody := L.CheckString(2)

		// Optional headers table (can be arg 3 or 4 depending on whether request context is provided)
		var headers map[string]string
		var baseCtx context.Context = context.Background()

		// Check if third arg is a request UserData (for context propagation)
		startArg := 3
		if L.GetTop() >= 3 {
			if reqUD := L.Get(3); reqUD.Type() == lua.LTUserData {
				if req, ok := reqUD.(*lua.LUserData).Value.(*http.Request); ok {
					baseCtx = req.Context()
					startArg = 4 // Headers would be in arg 4 if request context is provided
				}
			}
		}

		// Parse headers table if provided
		if L.GetTop() >= startArg && L.Get(startArg).Type() == lua.LTTable {
			headers = make(map[string]string)
			L.Get(startArg).(*lua.LTable).ForEach(func(k, v lua.LValue) {
				if key, ok := k.(lua.LString); ok {
					if val, ok := v.(lua.LString); ok {
						headers[string(key)] = string(val)
					}
				}
			})
		}

		// Create request with timeout context, propagating request context if available
		ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postBody))
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			L.Push(L.NewTable()) // empty headers table
			return 3
		}

		// Add headers if provided
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			L.Push(L.NewTable()) // empty headers table
			return 3
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			// Ensure response body is closed even on read error
			resp.Body.Close()
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			L.Push(L.NewTable()) // empty headers table
			return 3
		}

		// Create response headers table
		respHeaders := L.NewTable()
		for name, values := range resp.Header {
			if len(values) > 0 {
				respHeaders.RawSetString(name, lua.LString(values[0]))
			}
		}

		L.Push(lua.LString(string(body)))
		L.Push(lua.LNumber(resp.StatusCode))
		L.Push(respHeaders)
		return 3
	}))

	// --- ENV ---
	L.SetGlobal("get_env", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(os.Getenv(L.CheckString(1))))
		return 1
	}))
}
