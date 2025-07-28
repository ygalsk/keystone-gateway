package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/lua"
	"keystone-gateway/tests/fixtures"
)

// TestLuaEngineCreation tests Lua engine initialization
func TestLuaEngineCreation(t *testing.T) {
	testCases := []struct {
		name          string
		setupFunc     func(t *testing.T) (*lua.Engine, string)
		expectError   bool
		checkScripts  bool
		expectedCount int
	}{
		{
			name: "basic engine creation",
			setupFunc: func(t *testing.T) (*lua.Engine, string) {
				env := fixtures.SetupLuaEngine(t)
				return env.Engine, env.ScriptsDir
			},
			expectError:   false,
			checkScripts:  true,
			expectedCount: 0, // No scripts in empty directory
		},
		{
			name: "engine with single script",
			setupFunc: func(t *testing.T) (*lua.Engine, string) {
				env := fixtures.SetupLuaEngineWithScript(t, fixtures.CreateChiBindingsScript())
				return env.Engine, env.ScriptsDir
			},
			expectError:   false,
			checkScripts:  true,
			expectedCount: 1,
		},
		{
			name: "engine with multiple scripts",
			setupFunc: func(t *testing.T) (*lua.Engine, string) {
				scripts := map[string]string{
					"routes1.lua":    fixtures.CreateChiBindingsScript(),
					"routes2.lua":    fixtures.CreateRouteGroupScript(),
					"middleware.lua": fixtures.CreateMiddlewareScript(),
				}
				env := fixtures.SetupLuaEngineWithScripts(t, scripts)
				return env.Engine, env.ScriptsDir
			},
			expectError:   false,
			checkScripts:  true,
			expectedCount: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			engine, scriptsDir := tc.setupFunc(t)

			if engine == nil && !tc.expectError {
				t.Fatal("Expected engine to be created, got nil")
			}

			if tc.checkScripts {
				scripts := engine.GetLoadedScripts()
				if len(scripts) != tc.expectedCount {
					t.Errorf("Expected %d scripts, got %d: %v", tc.expectedCount, len(scripts), scripts)
				}
			}

			// Verify scripts directory exists
			if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
				t.Errorf("Scripts directory %s should exist", scriptsDir)
			}
		})
	}
}

// TestLuaScriptLoading tests script loading functionality
func TestLuaScriptLoading(t *testing.T) {
	testCases := []struct {
		name          string
		scriptName    string
		scriptContent string
		expectFound   bool
	}{
		{
			name:          "load existing script",
			scriptName:    "test-script",
			scriptContent: fixtures.CreateChiBindingsScript(),
			expectFound:   true,
		},
		{
			name:          "load non-existent script",
			scriptName:    "missing-script",
			scriptContent: "",
			expectFound:   false,
		},
		{
			name:          "load empty script",
			scriptName:    "empty-script",
			scriptContent: "",
			expectFound:   true,
		},
		{
			name:          "load script with special characters",
			scriptName:    "special-script",
			scriptContent: "-- Script with special chars: éñ中文\nprint('Hello')",
			expectFound:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scripts := make(map[string]string)
			var env *fixtures.LuaTestEnv
			if tc.expectFound {
				// Create the script file (even if empty) when we expect it to be found
				scripts[tc.scriptName+".lua"] = tc.scriptContent
				env = fixtures.SetupLuaEngineWithScripts(t, scripts)
			} else {
				// Don't create any script file when we expect it not to be found
				env = fixtures.SetupLuaEngine(t)
			}
			engine := env.Engine

			content, found := engine.GetScript(tc.scriptName)

			if found != tc.expectFound {
				t.Errorf("Expected found=%v, got found=%v", tc.expectFound, found)
			}

			if tc.expectFound && tc.scriptContent != "" {
				if content != tc.scriptContent {
					t.Errorf("Expected content %q, got %q", tc.scriptContent, content)
				}
			}
		})
	}
}

// TestLuaScriptExecution tests script execution functionality
func TestLuaScriptExecution(t *testing.T) {
	testCases := []struct {
		name          string
		scriptContent string
		tenantName    string
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid script execution",
			scriptContent: fixtures.CreateChiBindingsScript(),
			tenantName:    "test-tenant",
			expectError:   false,
		},
		{
			name:          "script with syntax error",
			scriptContent: "invalid lua syntax {{{",
			tenantName:    "test-tenant",
			expectError:   true,
			errorContains: "Lua script execution failed",
		},
		{
			name:          "script with runtime error",
			scriptContent: "error('Runtime error test')",
			tenantName:    "test-tenant",
			expectError:   true,
			errorContains: "Runtime error test",
		},
		{
			name:          "empty script",
			scriptContent: "",
			tenantName:    "test-tenant",
			expectError:   false,
		},
		{
			name:          "script with infinite loop (timeout test)",
			scriptContent: "while true do end",
			tenantName:    "test-tenant",
			expectError:   true,
			errorContains: "timeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := fixtures.SetupLuaEngineWithScript(t, tc.scriptContent)
			engine := env.Engine

			err := engine.ExecuteRouteScript("test-script", tc.tenantName)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestLuaGlobalScripts tests global script functionality
func TestLuaGlobalScripts(t *testing.T) {
	env := fixtures.SetupLuaEngine(t)
	engine := env.Engine
	scriptsDir := env.ScriptsDir

	// Create global scripts manually
	globalScript1 := "print('Global script 1')"
	globalScript2 := "print('Global script 2')"

	// Write global scripts
	err := os.WriteFile(filepath.Join(scriptsDir, "global-script1.lua"), []byte(globalScript1), 0644)
	if err != nil {
		t.Fatalf("Failed to write global script: %v", err)
	}

	err = os.WriteFile(filepath.Join(scriptsDir, "global-script2.lua"), []byte(globalScript2), 0644)
	if err != nil {
		t.Fatalf("Failed to write global script: %v", err)
	}

	// Reload scripts to pick up global scripts
	err = engine.ReloadScripts()
	if err != nil {
		t.Fatalf("Failed to reload scripts: %v", err)
	}

	// Execute global scripts
	err = engine.ExecuteGlobalScripts()
	if err != nil {
		t.Errorf("Failed to execute global scripts: %v", err)
	}
}

// TestLuaScriptCaching tests script caching functionality
func TestLuaScriptCaching(t *testing.T) {
	env := fixtures.SetupLuaEngine(t)
	engine := env.Engine
	scriptsDir := env.ScriptsDir

	scriptContent := "print('Cached script')"
	scriptPath := filepath.Join(scriptsDir, "cached-script.lua")

	// Write script file
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Reload to discover the script
	err = engine.ReloadScripts()
	if err != nil {
		t.Fatalf("Failed to reload scripts: %v", err)
	}

	// First load (should read from file)
	content1, found1 := engine.GetScript("cached-script")
	if !found1 {
		t.Fatal("Script should be found")
	}

	// Second load (should read from cache)
	content2, found2 := engine.GetScript("cached-script")
	if !found2 {
		t.Fatal("Script should be found in cache")
	}

	if content1 != content2 {
		t.Error("Cached content should match original content")
	}

	if content1 != scriptContent {
		t.Errorf("Expected content %q, got %q", scriptContent, content1)
	}
}

// TestLuaScriptReloading tests script reloading functionality
func TestLuaScriptReloading(t *testing.T) {
	env := fixtures.SetupLuaEngine(t)
	engine := env.Engine
	scriptsDir := env.ScriptsDir

	scriptPath := filepath.Join(scriptsDir, "reload-script.lua")
	originalContent := "print('Original')"
	updatedContent := "print('Updated')"

	// Write original script
	err := os.WriteFile(scriptPath, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Load scripts
	err = engine.ReloadScripts()
	if err != nil {
		t.Fatalf("Failed to reload scripts: %v", err)
	}

	// Get original content
	content1, found1 := engine.GetScript("reload-script")
	if !found1 || content1 != originalContent {
		t.Fatalf("Expected original content, got %q", content1)
	}

	// Update script file
	err = os.WriteFile(scriptPath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to update script: %v", err)
	}

	// Reload scripts
	err = engine.ReloadScripts()
	if err != nil {
		t.Fatalf("Failed to reload scripts: %v", err)
	}

	// Get updated content
	content2, found2 := engine.GetScript("reload-script")
	if !found2 || content2 != updatedContent {
		t.Errorf("Expected updated content %q, got %q", updatedContent, content2)
	}
}

// TestLuaEngineEdgeCases tests edge cases and error conditions
func TestLuaEngineEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		setupFunc   func(t *testing.T) (*lua.Engine, string)
		testFunc    func(t *testing.T, engine *lua.Engine)
		expectPanic bool
	}{
		{
			name: "execute non-existent script",
			setupFunc: func(t *testing.T) (*lua.Engine, string) {
				env := fixtures.SetupLuaEngine(t)
				return env.Engine, env.ScriptsDir
			},
			testFunc: func(t *testing.T, engine *lua.Engine) {
				err := engine.ExecuteRouteScript("non-existent", "tenant")
				if err == nil {
					t.Error("Expected error for non-existent script")
				}
			},
		},
		{
			name: "script with memory allocation",
			setupFunc: func(t *testing.T) (*lua.Engine, string) {
				largeScript := "local t = {}\nfor i=1,1000 do t[i] = 'data' end"
				env := fixtures.SetupLuaEngineWithScript(t, largeScript)
				return env.Engine, env.ScriptsDir
			},
			testFunc: func(t *testing.T, engine *lua.Engine) {
				err := engine.ExecuteRouteScript("test-script", "tenant")
				if err != nil {
					t.Errorf("Memory allocation script failed: %v", err)
				}
			},
		},
		{
			name: "concurrent script execution",
			setupFunc: func(t *testing.T) (*lua.Engine, string) {
				script := "print('Concurrent execution')"
				env := fixtures.SetupLuaEngineWithScript(t, script)
				return env.Engine, env.ScriptsDir
			},
			testFunc: func(t *testing.T, engine *lua.Engine) {
				done := make(chan error, 3)

				// Execute same script concurrently
				for i := 0; i < 3; i++ {
					go func() {
						err := engine.ExecuteRouteScript("test-script", "tenant")
						done <- err
					}()
				}

				// Wait for all executions
				for i := 0; i < 3; i++ {
					select {
					case err := <-done:
						if err != nil {
							t.Errorf("Concurrent execution failed: %v", err)
						}
					case <-time.After(5 * time.Second):
						t.Error("Concurrent execution timed out")
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic, but none occurred")
					}
				}()
			}

			engine, _ := tc.setupFunc(t)
			tc.testFunc(t, engine)
		})
	}
}

// TestLuaEngineIntegration tests integration with routing system
func TestLuaEngineIntegration(t *testing.T) {
	env := fixtures.SetupSimpleGateway(t, "lua-tenant", "/lua/")

	// Test that engine integrates properly with gateway
	registry := env.Gateway.GetRouteRegistry()
	if registry == nil {
		t.Fatal("Gateway should have route registry")
	}

	// Test script execution through the integration
	script := fixtures.CreateChiBindingsScript()
	luaEnv := fixtures.SetupLuaEngineWithScript(t, script)

	err := luaEnv.Engine.ExecuteRouteScript("test-script", "lua-tenant")
	if err != nil {
		t.Errorf("Integration script execution failed: %v", err)
	}
}
