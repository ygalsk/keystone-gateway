package fixtures

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/lua"
)

// LuaTestEnv represents a test environment for Lua engine and scripts
type LuaTestEnv struct {
	Engine     *lua.Engine
	Router     *chi.Mux
	ScriptsDir string
	TmpDir     string
}

// SetupLuaEngine creates a Lua engine with scripts directory
func SetupLuaEngine(t *testing.T) *LuaTestEnv {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("failed to create scripts directory: %v", err)
	}

	router := chi.NewRouter()
	engine := lua.NewEngine(scriptsDir, router)

	return &LuaTestEnv{
		Engine:     engine,
		Router:     router,
		ScriptsDir: scriptsDir,
		TmpDir:     tmpDir,
	}
}

// SetupLuaEngineWithScript creates a Lua engine and writes a script file
func SetupLuaEngineWithScript(t *testing.T, scriptContent string) *LuaTestEnv {
	env := SetupLuaEngine(t)
	
	if scriptContent != "" {
		scriptFile := filepath.Join(env.ScriptsDir, "test-script.lua")
		if err := os.WriteFile(scriptFile, []byte(scriptContent), 0644); err != nil {
			t.Fatalf("failed to create script file: %v", err)
		}
	}
	
	return env
}

// SetupLuaEngineWithScripts creates a Lua engine and writes multiple script files
func SetupLuaEngineWithScripts(t *testing.T, scripts map[string]string) *LuaTestEnv {
	env := SetupLuaEngine(t)

	for filename, content := range scripts {
		scriptPath := filepath.Join(env.ScriptsDir, filename)
		if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create script %s: %v", filename, err)
		}
	}

	return env
}

// CreateChiBindingsScript returns a sample Chi bindings Lua script
func CreateChiBindingsScript() string {
	return `
		local chi = require('chi')
		local router = chi.NewRouter()
		
		router:Use(function(w, r, next)
			w:Header():Set("X-Middleware", "applied")
			next()
		end)
		
		router:Get("/test", function(w, r)
			w:Write("test response")
		end)
		
		return router
	`
}

// CreateRouteGroupScript returns a sample route group Lua script
func CreateRouteGroupScript() string {
	return `
		local chi = require('chi')
		local router = chi.NewRouter()
		
		router:Use(function(w, r, next)
			w:Header():Set("X-API-Version", "v1")
			next()
		end)
		
		router:Route("/api/v1", function(r)
			r:Get("/users", function(w, r)
				w:Write("users list")
			end)
			r:Get("/users/{id}", function(w, r)
				w:Write("user " .. r:URLParam("id"))
			end)
			r:Post("/users", function(w, r)
				w:Write("user created")
			end)
		end)
		
		router:Get("/health", function(w, r)
			w:Write("healthy")
		end)
		
		return router
	`
}

// CreateMiddlewareScript returns a sample middleware Lua script
func CreateMiddlewareScript() string {
	return `
		local chi = require('chi')
		local router = chi.NewRouter()
		
		local function authMiddleware(w, r, next)
			local auth = r:Header():Get("Authorization")
			if auth == "" then
				w:WriteHeader(401)
				w:Write("Unauthorized")
				return
			end
			next()
		end
		
		local function loggingMiddleware(w, r, next)
			w:Header():Set("X-Request-ID", "test-123")
			next()
		end
		
		router:Use(loggingMiddleware)
		router:Use(authMiddleware)
		
		router:Get("/protected", function(w, r)
			w:Write("protected resource")
		end)
		
		return router
	`
}