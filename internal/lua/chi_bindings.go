package lua

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
)

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

	// --- PARAMS ---
	L.SetGlobal("chi_param", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		key := L.CheckString(2)
		if req, ok := reqUD.Value.(*http.Request); ok {
			L.Push(lua.LString(chi.URLParam(req, key)))
			return 1
		}
		L.Push(lua.LString(""))
		return 1
	}))

	// --- REQUEST PROPERTIES ---
	L.SetGlobal("request_url", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		if req, ok := reqUD.Value.(*http.Request); ok {
			L.Push(lua.LString(req.URL.String()))
			return 1
		}
		L.Push(lua.LString(""))
		return 1
	}))

	L.SetGlobal("request_method", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		if req, ok := reqUD.Value.(*http.Request); ok {
			L.Push(lua.LString(req.Method))
			return 1
		}
		L.Push(lua.LString(""))
		return 1
	}))

	L.SetGlobal("request_header", L.NewFunction(func(L *lua.LState) int {
		reqUD := L.CheckUserData(1)
		headerName := L.CheckString(2)
		if req, ok := reqUD.Value.(*http.Request); ok {
			L.Push(lua.LString(req.Header.Get(headerName)))
			return 1
		}
		L.Push(lua.LString(""))
		return 1
	}))

	// --- RESPONSE FUNCTIONS ---
	L.SetGlobal("response_status", L.NewFunction(func(L *lua.LState) int {
		resUD := L.CheckUserData(1)
		statusCode := L.CheckInt(2)
		if w, ok := resUD.Value.(http.ResponseWriter); ok {
			w.WriteHeader(statusCode)
		}
		return 0
	}))

	L.SetGlobal("response_header", L.NewFunction(func(L *lua.LState) int {
		resUD := L.CheckUserData(1)
		headerName := L.CheckString(2)
		headerValue := L.CheckString(3)
		if w, ok := resUD.Value.(http.ResponseWriter); ok {
			w.Header().Set(headerName, headerValue)
		}
		return 0
	}))

	L.SetGlobal("response_write", L.NewFunction(func(L *lua.LState) int {
		resUD := L.CheckUserData(1)
		content := L.CheckString(2)
		if w, ok := resUD.Value.(http.ResponseWriter); ok {
			w.Write([]byte(content))
		}
		return 0
	}))

	// --- HTTP CLIENT FUNCTIONS ---
	L.SetGlobal("http_get", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)

		// Optional headers table
		var headers map[string]string
		if L.GetTop() >= 2 && L.Get(2).Type() == lua.LTTable {
			headers = make(map[string]string)
			L.Get(2).(*lua.LTable).ForEach(func(k, v lua.LValue) {
				if key, ok := k.(lua.LString); ok {
					if val, ok := v.(lua.LString); ok {
						headers[string(key)] = string(val)
					}
				}
			})
		}

		client := &http.Client{Timeout: 30 * time.Second}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			return 2
		}

		// Add headers if provided
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			return 2
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		L.Push(lua.LString(string(body)))
		L.Push(lua.LNumber(resp.StatusCode))
		return 2
	}))

	L.SetGlobal("http_post", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		postBody := L.CheckString(2)

		// Optional headers table
		var headers map[string]string
		if L.GetTop() >= 3 && L.Get(3).Type() == lua.LTTable {
			headers = make(map[string]string)
			L.Get(3).(*lua.LTable).ForEach(func(k, v lua.LValue) {
				if key, ok := k.(lua.LString); ok {
					if val, ok := v.(lua.LString); ok {
						headers[string(key)] = string(val)
					}
				}
			})
		}

		client := &http.Client{Timeout: 30 * time.Second}
		req, err := http.NewRequest("POST", url, strings.NewReader(postBody))
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			return 2
		}

		// Add headers if provided
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LString(""))
			L.Push(lua.LNumber(0))
			return 2
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		L.Push(lua.LString(string(body)))
		L.Push(lua.LNumber(resp.StatusCode))
		return 2
	}))

	// --- ENV ---
	L.SetGlobal("get_env", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(os.Getenv(L.CheckString(1))))
		return 1
	}))
}