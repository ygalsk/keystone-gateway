package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/lua"
)

func TestGlobalScriptExecution(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a global script
	globalScript := filepath.Join(tmpDir, "global-auth.lua")
	globalContent := `
log("Executing global auth script")
-- This would typically set up global middleware or authentication
`
	if err := os.WriteFile(globalScript, []byte(globalContent), 0644); err != nil {
		t.Fatalf("failed to create global script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test executing global scripts
	err := engine.ExecuteGlobalScripts()
	if err != nil {
		t.Errorf("failed to execute global scripts: %v", err)
	}
}

func TestGlobalScriptDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create multiple global scripts
	scripts := map[string]string{
		"global-security.lua": `log("Global security script")`,
		"global-logging.lua":  `log("Global logging script")`,
		"global-cors.lua":     `log("Global CORS script")`,
	}

	for filename, content := range scripts {
		scriptPath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create script %s: %v", filename, err)
		}
	}

	// Also create a regular route script to ensure it's not confused with global
	routeScript := filepath.Join(tmpDir, "api-routes.lua")
	if err := os.WriteFile(routeScript, []byte(`log("Route script")`), 0644); err != nil {
		t.Fatalf("failed to create route script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Execute global scripts
	err := engine.ExecuteGlobalScripts()
	if err != nil {
		t.Errorf("failed to execute global scripts: %v", err)
	}

	// Verify that only route scripts are in the loaded scripts list
	loadedScripts := engine.GetLoadedScripts()
	expectedRouteScripts := []string{"api-routes"}

	if len(loadedScripts) != len(expectedRouteScripts) {
		t.Errorf("expected %d route scripts, got %d", len(expectedRouteScripts), len(loadedScripts))
	}

	found := false
	for _, script := range loadedScripts {
		if script == "api-routes" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'api-routes' in loaded scripts")
	}
}

func TestGlobalScriptError(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a global script with syntax error
	globalScript := filepath.Join(tmpDir, "global-broken.lua")
	brokenContent := `
log("Starting global script")
invalid_lua_syntax_here!!!
log("This won't execute")
`
	if err := os.WriteFile(globalScript, []byte(brokenContent), 0644); err != nil {
		t.Fatalf("failed to create broken global script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that global script execution fails gracefully
	err := engine.ExecuteGlobalScripts()
	if err == nil {
		t.Error("expected error when executing broken global script")
	}
}

func TestGlobalScriptTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a global script that would run forever
	globalScript := filepath.Join(tmpDir, "global-infinite.lua")
	infiniteContent := `
log("Starting infinite script")
while true do
    -- This would run forever without timeout
end
`
	if err := os.WriteFile(globalScript, []byte(infiniteContent), 0644); err != nil {
		t.Fatalf("failed to create infinite global script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that global script execution times out
	err := engine.ExecuteGlobalScripts()
	if err == nil {
		t.Error("expected timeout error when executing infinite global script")
	}

	if err != nil && !containsString(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestGlobalScriptNaming(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Test various global script naming patterns
	testCases := []struct {
		filename       string
		shouldBeGlobal bool
	}{
		{"global-auth.lua", true},
		{"global-security.lua", true},
		{"global-.lua", true},         // Edge case: empty name after prefix
		{"auth-global.lua", false},    // Wrong position
		{"globalsecurity.lua", false}, // Missing dash
		{"routes.lua", false},         // Regular script
		{"api-routes.lua", false},     // Regular script with dash
	}

	for _, tc := range testCases {
		scriptPath := filepath.Join(tmpDir, tc.filename)
		content := `log("Script: ` + tc.filename + `")`
		if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create script %s: %v", tc.filename, err)
		}
	}

	engine := lua.NewEngine(tmpDir, router)

	// Execute global scripts - should only execute the global-* ones
	err := engine.ExecuteGlobalScripts()
	if err != nil {
		t.Errorf("failed to execute global scripts: %v", err)
	}

	// Verify route scripts are correctly identified
	loadedScripts := engine.GetLoadedScripts()
	expectedRouteScripts := []string{"auth-global", "globalsecurity", "routes", "api-routes"}

	if len(loadedScripts) != len(expectedRouteScripts) {
		t.Errorf("expected %d route scripts, got %d: %v", len(expectedRouteScripts), len(loadedScripts), loadedScripts)
	}
}

func TestEmptyGlobalScriptsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create engine with empty directory
	engine := lua.NewEngine(tmpDir, router)

	// Should not error when no global scripts exist
	err := engine.ExecuteGlobalScripts()
	if err != nil {
		t.Errorf("should not error with no global scripts: %v", err)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		(len(s) > len(substr) && containsString(s[1:], substr))
}
