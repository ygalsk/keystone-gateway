package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/lua"
)

func TestNewEngine(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	engine := lua.NewEngine(tmpDir, router)
	if engine == nil {
		t.Fatal("expected engine but got nil")
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("expected scripts directory to be created")
	}
}

func TestLoadScriptPaths(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create test script files
	routeScript := filepath.Join(tmpDir, "test-route.lua")
	globalScript := filepath.Join(tmpDir, "global-auth.lua")

	if err := os.WriteFile(routeScript, []byte("-- route script"), 0644); err != nil {
		t.Fatalf("failed to create route script: %v", err)
	}

	if err := os.WriteFile(globalScript, []byte("-- global script"), 0644); err != nil {
		t.Fatalf("failed to create global script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Check that scripts were discovered
	scripts := engine.GetLoadedScripts()
	if len(scripts) != 1 {
		t.Errorf("expected 1 route script, got %d", len(scripts))
	}

	found := false
	for _, script := range scripts {
		if script == "test-route" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'test-route' script")
	}
}

func TestGetScript(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create test script
	scriptPath := filepath.Join(tmpDir, "test.lua")
	scriptContent := "log('test script loaded')"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test getting existing script
	content, exists := engine.GetScript("test")
	if !exists {
		t.Error("expected script to exist")
	}
	if content != scriptContent {
		t.Errorf("expected content %q, got %q", scriptContent, content)
	}

	// Test getting non-existent script
	_, exists = engine.GetScript("nonexistent")
	if exists {
		t.Error("expected script to not exist")
	}
}

func TestGetScriptLazyLoading(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create script after engine initialization
	engine := lua.NewEngine(tmpDir, router)

	scriptPath := filepath.Join(tmpDir, "lazy.lua")
	scriptContent := "log('lazy loaded')"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	// Reload to discover new script
	if err := engine.ReloadScripts(); err != nil {
		t.Fatalf("failed to reload scripts: %v", err)
	}

	// Test lazy loading
	content, exists := engine.GetScript("lazy")
	if !exists {
		t.Error("expected script to exist after reload")
	}
	if content != scriptContent {
		t.Errorf("expected content %q, got %q", scriptContent, content)
	}

	// Test caching - second call should use cache
	content2, exists2 := engine.GetScript("lazy")
	if !exists2 {
		t.Error("expected cached script to exist")
	}
	if content2 != scriptContent {
		t.Errorf("expected cached content %q, got %q", scriptContent, content2)
	}
}

func TestReloadScripts(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create initial script
	scriptPath := filepath.Join(tmpDir, "reload-test.lua")
	initialContent := "log('initial')"
	if err := os.WriteFile(scriptPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Load script into cache
	content, exists := engine.GetScript("reload-test")
	if !exists || content != initialContent {
		t.Fatal("failed to load initial script")
	}

	// Modify script content
	newContent := "log('modified')"
	if err := os.WriteFile(scriptPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to modify script: %v", err)
	}

	// Should still return cached content
	content, _ = engine.GetScript("reload-test")
	if content != initialContent {
		t.Error("expected cached content before reload")
	}

	// Reload and verify new content
	if err := engine.ReloadScripts(); err != nil {
		t.Fatalf("failed to reload scripts: %v", err)
	}

	content, exists = engine.GetScript("reload-test")
	if !exists {
		t.Error("expected script to exist after reload")
	}
	if content != newContent {
		t.Errorf("expected new content %q, got %q", newContent, content)
	}
}

func TestExecuteRouteScript(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a simple script
	scriptPath := filepath.Join(tmpDir, "simple.lua")
	scriptContent := `log("executing simple script")`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test script execution
	err := engine.ExecuteRouteScript("simple", "test-tenant")
	if err != nil {
		t.Errorf("script execution failed: %v", err)
	}

	// Test non-existent script
	err = engine.ExecuteRouteScript("nonexistent", "test-tenant")
	if err == nil {
		t.Error("expected error for non-existent script")
	}
}
