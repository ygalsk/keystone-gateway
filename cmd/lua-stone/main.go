package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// -------------------------
// 1. CONSTANTS & TYPES
// -------------------------

const (
	DefaultListenAddress   = ":8081"
	MaxScriptExecutionTime = 5 * time.Second
	MaxMemoryMB            = 10
)

// Request represents incoming routing request from Gateway
type RoutingRequest struct {
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Host     string            `json:"host"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body,omitempty"`
	Backends []Backend         `json:"backends"`
}

// Backend represents available backend services
type Backend struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Health bool   `json:"health"`
}

// Response represents the Lua script's routing decision
type RoutingResponse struct {
	SelectedBackend string            `json:"selected_backend"`
	ModifiedHeaders map[string]string `json:"modified_headers,omitempty"`
	ModifiedPath    string            `json:"modified_path,omitempty"`
	Reject          bool              `json:"reject,omitempty"`
	RejectReason    string            `json:"reject_reason,omitempty"`
}

// LuaEngine manages script execution in isolated environment
type LuaEngine struct {
	scriptsDir string
	scripts    map[string]string // tenant_name -> script_content
}

// -------------------------
// 2. LUA ENGINE CORE
// -------------------------

func NewLuaEngine(scriptsDir string) *LuaEngine {
	engine := &LuaEngine{
		scriptsDir: scriptsDir,
		scripts:    make(map[string]string),
	}
	engine.loadScripts()
	return engine
}

func (e *LuaEngine) loadScripts() {
	if _, err := os.Stat(e.scriptsDir); os.IsNotExist(err) {
		log.Printf("Scripts directory %s does not exist, creating...", e.scriptsDir)
		os.MkdirAll(e.scriptsDir, 0755)
		e.createExampleScript()
		return
	}

	err := filepath.Walk(e.scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".lua") {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Failed to read script %s: %v", path, err)
			return nil
		}

		tenantName := strings.TrimSuffix(filepath.Base(path), ".lua")
		e.scripts[tenantName] = string(content)
		log.Printf("Loaded script for tenant: %s", tenantName)
		return nil
	})

	if err != nil {
		log.Printf("Error walking scripts directory: %v", err)
	}
}

func (e *LuaEngine) createExampleScript() {
	exampleScript := `-- Example Canary Deployment Script
function on_route_request(request, backends)
    local canary_header = request.headers["X-Canary"]
    local canary_percent = tonumber(request.headers["X-Canary-Percent"]) or 10
    
    -- Simple canary routing based on header or percentage
    if canary_header == "true" then
        return select_backend_by_name(backends, "canary")
    end
    
    -- Random canary traffic (10% default)
    local random_val = math.random(100)
    if random_val <= canary_percent then
        return select_backend_by_name(backends, "canary")
    end
    
    -- Default to stable
    return select_backend_by_name(backends, "stable")
end

function select_backend_by_name(backends, name_pattern)
    for i, backend in ipairs(backends) do
        if string.find(backend.name, name_pattern) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-Backend-Type"] = name_pattern
                }
            }
        end
    end
    
    -- Fallback to first healthy backend
    for i, backend in ipairs(backends) do
        if backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine-fallback"
                }
            }
        end
    end
    
    return {
        reject = true,
        reject_reason = "No healthy backends available"
    }
end`

	examplePath := filepath.Join(e.scriptsDir, "example.lua")
	if err := os.WriteFile(examplePath, []byte(exampleScript), 0644); err != nil {
		log.Printf("Failed to create example script: %v", err)
	} else {
		log.Printf("Created example script at: %s", examplePath)
	}
}

// -------------------------
// 3. SCRIPT EXECUTION
// -------------------------

func (e *LuaEngine) ExecuteScript(tenantName string, req RoutingRequest) (*RoutingResponse, error) {
	script, exists := e.scripts[tenantName]
	if !exists {
		return &RoutingResponse{
			Reject:       true,
			RejectReason: fmt.Sprintf("No script found for tenant: %s", tenantName),
		}, nil
	}

	// Create isolated Lua state
	L := lua.NewState(lua.Options{
		CallStackSize: 120,
		RegistrySize:  120 * 20,
	})
	defer L.Close()

	// Set memory limit (basic protection) - disabled for performance  
	// L.SetMx(MaxMemoryMB)

	// Setup timeout
	ctx := make(chan bool, 1)
	go func() {
		time.Sleep(MaxScriptExecutionTime)
		ctx <- true
	}()

	// Setup Lua environment
	e.setupLuaEnvironment(L, req)

	// Execute script with timeout protection
	done := make(chan *RoutingResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := e.runScript(L, script)
		if err != nil {
			errChan <- err
			return
		}
		done <- result
	}()

	select {
	case result := <-done:
		return result, nil
	case err := <-errChan:
		return nil, fmt.Errorf("script execution failed: %w", err)
	case <-ctx:
		return nil, fmt.Errorf("script execution timeout after %v", MaxScriptExecutionTime)
	}
}

func (e *LuaEngine) setupLuaEnvironment(L *lua.LState, req RoutingRequest) {
	// Create request table
	requestTable := L.NewTable()
	requestTable.RawSetString("method", lua.LString(req.Method))
	requestTable.RawSetString("path", lua.LString(req.Path))
	requestTable.RawSetString("host", lua.LString(req.Host))
	requestTable.RawSetString("body", lua.LString(req.Body))

	// Headers table
	headersTable := L.NewTable()
	for k, v := range req.Headers {
		headersTable.RawSetString(k, lua.LString(v))
	}
	requestTable.RawSetString("headers", headersTable)

	// Backends table
	backendsTable := L.NewTable()
	for i, backend := range req.Backends {
		backendTable := L.NewTable()
		backendTable.RawSetString("name", lua.LString(backend.Name))
		backendTable.RawSetString("url", lua.LString(backend.URL))
		backendTable.RawSetString("health", lua.LBool(backend.Health))
		backendsTable.RawSetInt(i+1, backendTable)
	}

	// Set globals
	L.SetGlobal("request", requestTable)
	L.SetGlobal("backends", backendsTable)

	// Add utility functions
	L.SetGlobal("log", L.NewFunction(e.luaLogFunction))
	L.SetGlobal("math", L.NewTable()) // Empty math table for safety
	L.GetGlobal("math").(*lua.LTable).RawSetString("random", L.NewFunction(e.luaMathRandom))
}

func (e *LuaEngine) runScript(L *lua.LState, script string) (*RoutingResponse, error) {
	// Load and execute script
	if err := L.DoString(script); err != nil {
		return nil, fmt.Errorf("failed to load script: %w", err)
	}

	// Call main function
	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("on_route_request"),
		NRet:    1,
		Protect: true,
	}, L.GetGlobal("request"), L.GetGlobal("backends")); err != nil {
		return nil, fmt.Errorf("failed to call on_route_request: %w", err)
	}

	// Parse result
	result := L.Get(-1)
	if result == lua.LNil {
		return &RoutingResponse{
			Reject:       true,
			RejectReason: "Script returned nil",
		}, nil
	}

	return e.parseLuaResult(result)
}

func (e *LuaEngine) parseLuaResult(lv lua.LValue) (*RoutingResponse, error) {
	table, ok := lv.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("script must return a table")
	}

	response := &RoutingResponse{}

	// Extract fields
	if selectedBackend := table.RawGetString("selected_backend"); selectedBackend != lua.LNil {
		response.SelectedBackend = selectedBackend.String()
	}

	if modifiedPath := table.RawGetString("modified_path"); modifiedPath != lua.LNil {
		response.ModifiedPath = modifiedPath.String()
	}

	if reject := table.RawGetString("reject"); reject != lua.LNil {
		response.Reject = lua.LVAsBool(reject)
	}

	if rejectReason := table.RawGetString("reject_reason"); rejectReason != lua.LNil {
		response.RejectReason = rejectReason.String()
	}

	// Parse modified headers
	if headersLV := table.RawGetString("modified_headers"); headersLV != lua.LNil {
		if headersTable, ok := headersLV.(*lua.LTable); ok {
			response.ModifiedHeaders = make(map[string]string)
			headersTable.ForEach(func(k, v lua.LValue) {
				response.ModifiedHeaders[k.String()] = v.String()
			})
		}
	}

	return response, nil
}

// -------------------------
// 4. LUA UTILITY FUNCTIONS
// -------------------------

func (e *LuaEngine) luaLogFunction(L *lua.LState) int {
	message := L.ToString(1)
	log.Printf("[Lua]: %s", message)
	return 0
}

func (e *LuaEngine) luaMathRandom(L *lua.LState) int {
	max := L.ToInt(1)
	if max <= 0 {
		max = 100
	}
	// Simple random implementation
	result := time.Now().UnixNano()%int64(max) + 1
	L.Push(lua.LNumber(result))
	return 1
}

// -------------------------
// 5. HTTP HANDLERS
// -------------------------

func (e *LuaEngine) RouteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse tenant from path: /route/{tenant}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) != 2 || pathParts[0] != "route" {
		http.Error(w, "Invalid path. Use /route/{tenant}", http.StatusBadRequest)
		return
	}

	tenantName := pathParts[1]

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var routingReq RoutingRequest
	if err := json.Unmarshal(body, &routingReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Execute script
	response, err := e.ExecuteScript(tenantName, routingReq)
	if err != nil {
		log.Printf("Script execution error for tenant %s: %v", tenantName, err)
		http.Error(w, fmt.Sprintf("Script execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (e *LuaEngine) HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":         "healthy",
		"loaded_scripts": len(e.scripts),
		"scripts":        make([]string, 0, len(e.scripts)),
		"version":        "1.0.0",
	}

	for tenantName := range e.scripts {
		health["scripts"] = append(health["scripts"].([]string), tenantName)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (e *LuaEngine) ReloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	e.loadScripts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "scripts reloaded",
		"count":  fmt.Sprintf("%d", len(e.scripts)),
	})
}

// -------------------------
// 6. MAIN FUNCTION
// -------------------------

func main() {
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	scriptsDir := flag.String("scripts", "./scripts", "path to Lua scripts directory")
	flag.Parse()

	engine := NewLuaEngine(*scriptsDir)

	// Setup routes
	http.HandleFunc("/route/", engine.RouteHandler)
	http.HandleFunc("/health", engine.HealthHandler)
	http.HandleFunc("/reload", engine.ReloadHandler)

	log.Printf("Keystone Lua Engine v1.0.0 listening on %s", *addr)
	log.Printf("Scripts directory: %s", *scriptsDir)
	log.Printf("Loaded %d scripts", len(engine.scripts))

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
