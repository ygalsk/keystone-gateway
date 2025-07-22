package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/lua"
)

func TestLuaEngineFileReadErrors(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a directory that doesn't have read permissions
	restrictedDir := filepath.Join(tmpDir, "restricted")
	if err := os.Mkdir(restrictedDir, 0000); err != nil {
		t.Fatalf("failed to create restricted directory: %v", err)
	}
	defer os.Chmod(restrictedDir, 0755) // Restore permissions for cleanup

	// Try to create engine with restricted directory
	engine := lua.NewEngine(restrictedDir, router)

	// Test that script discovery handles permission errors gracefully
	scripts := engine.GetLoadedScripts()
	if len(scripts) != 0 {
		t.Errorf("expected no scripts from restricted directory, got %d", len(scripts))
	}
}

func TestLuaEngineNonExistentDirectory(t *testing.T) {
	router := chi.NewRouter()
	nonExistentDir := "/path/that/does/not/exist"

	// Engine should handle non-existent directories gracefully
	engine := lua.NewEngine(nonExistentDir, router)
	scripts := engine.GetLoadedScripts()

	if len(scripts) != 0 {
		t.Errorf("expected no scripts from non-existent directory, got %d", len(scripts))
	}

	// Test global script execution on non-existent directory
	err := engine.ExecuteGlobalScripts()
	if err != nil {
		t.Errorf("ExecuteGlobalScripts should handle non-existent directory gracefully: %v", err)
	}
}

func TestLuaEngineCorruptedScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a script file with binary content (corrupted)
	corruptedScript := filepath.Join(tmpDir, "corrupted.lua")
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	if err := os.WriteFile(corruptedScript, binaryContent, 0644); err != nil {
		t.Fatalf("failed to create corrupted script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that corrupted scripts are handled gracefully during discovery
	scripts := engine.GetLoadedScripts()

	// Should still discover the script file (name only, not content)
	found := false
	for _, script := range scripts {
		if script == "corrupted" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'corrupted' script in loaded scripts list")
	}
}

func TestLuaEngineScriptExecutionPanic(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a script that would cause a panic in Lua
	panicScript := filepath.Join(tmpDir, "panic-script.lua")
	panicContent := `
-- This script tries to access invalid memory or cause a panic
function invalid_function()
    -- Force a stack overflow
    return invalid_function()
end

invalid_function()
`
	if err := os.WriteFile(panicScript, []byte(panicContent), 0644); err != nil {
		t.Fatalf("failed to create panic script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that script with recursive calls is handled
	script, exists := engine.GetScript("panic-script")
	if !exists {
		t.Error("failed to get panic script")
	}
	if script == "" {
		t.Error("expected to get panic script content")
	}
}

func TestLuaEngineEmptyScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create an empty script file
	emptyScript := filepath.Join(tmpDir, "empty.lua")
	if err := os.WriteFile(emptyScript, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that empty scripts are handled gracefully
	script, exists := engine.GetScript("empty")
	if !exists {
		t.Error("failed to get empty script")
	}
	if len(script) != 0 {
		t.Errorf("expected empty script content, got %d bytes", len(script))
	}
}

func TestLuaEngineInvalidScriptSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create script with invalid Lua syntax
	invalidScript := filepath.Join(tmpDir, "invalid-syntax.lua")
	invalidContent := `
function test_handler(response, request)
    -- Missing end keyword and invalid syntax
    if true then
        response:write("Hello")
    -- Missing end for if statement
    -- Missing end for function
`
	if err := os.WriteFile(invalidScript, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to create invalid syntax script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Engine should still load the script content (validation happens at execution)
	script, exists := engine.GetScript("invalid-syntax")
	if !exists {
		t.Error("failed to get invalid syntax script")
	}
	if script == "" {
		t.Error("expected to get invalid syntax script content")
	}
}

func TestLuaEngineVeryLargeScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a very large script file (1MB+)
	largeScript := filepath.Join(tmpDir, "large-script.lua")
	var content strings.Builder
	content.WriteString("function test_handler(response, request)\n")

	// Add 100,000 lines of comments to make it large
	for i := 0; i < 100000; i++ {
		content.WriteString(fmt.Sprintf("    -- This is comment line %d\n", i))
	}
	content.WriteString("    response:write('Large script executed')\n")
	content.WriteString("end\n")

	if err := os.WriteFile(largeScript, []byte(content.String()), 0644); err != nil {
		t.Fatalf("failed to create large script: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that large scripts are handled (with appropriate timeout)
	start := time.Now()
	script, exists := engine.GetScript("large-script")
	duration := time.Since(start)

	if !exists {
		t.Error("failed to get large script")
	}
	if script == "" {
		t.Error("expected to get large script content")
	}

	// Should not take more than 5 seconds to read even a large file
	if duration > 5*time.Second {
		t.Errorf("reading large script took too long: %v", duration)
	}
}

func TestLuaEngineSymbolicLinkHandling(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Create a real script file
	realScript := filepath.Join(tmpDir, "real-script.lua")
	realContent := `
function test_handler(response, request)
    response:write("Real script")
end
`
	if err := os.WriteFile(realScript, []byte(realContent), 0644); err != nil {
		t.Fatalf("failed to create real script: %v", err)
	}

	// Create a symbolic link to the script
	symlinkScript := filepath.Join(tmpDir, "symlink-script.lua")
	if err := os.Symlink(realScript, symlinkScript); err != nil {
		// Skip this test if symbolic links are not supported (e.g., Windows without admin)
		t.Skipf("symbolic links not supported: %v", err)
	}

	engine := lua.NewEngine(tmpDir, router)

	// Test that symbolic links are handled properly
	scripts := engine.GetLoadedScripts()

	realFound := false
	symlinkFound := false
	for _, script := range scripts {
		if script == "real-script" {
			realFound = true
		}
		if script == "symlink-script" {
			symlinkFound = true
		}
	}

	if !realFound {
		t.Error("expected to find 'real-script' in loaded scripts")
	}
	if !symlinkFound {
		t.Error("expected to find 'symlink-script' in loaded scripts")
	}

	// Both should return the same content
	realScriptContent, exists1 := engine.GetScript("real-script")
	symlinkScriptContent, exists2 := engine.GetScript("symlink-script")

	if !exists1 {
		t.Error("failed to get real script")
	}
	if !exists2 {
		t.Error("failed to get symlink script")
	}

	if exists1 && exists2 {
		if realScriptContent != symlinkScriptContent {
			t.Error("real script and symlink script should have the same content")
		}
	}
}

func TestLuaEngineSpecialCharactersInFilename(t *testing.T) {
	tmpDir := t.TempDir()
	router := chi.NewRouter()

	// Test various special characters in filenames
	testCases := []struct {
		filename string
		expected string
	}{
		{"script-with-dashes.lua", "script-with-dashes"},
		{"script_with_underscores.lua", "script_with_underscores"},
		{"script.with.dots.lua", "script.with.dots"},
		{"script with spaces.lua", "script with spaces"},
	}

	for _, tc := range testCases {
		scriptPath := filepath.Join(tmpDir, tc.filename)
		content := fmt.Sprintf("-- Script: %s", tc.filename)

		if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create script %s: %v", tc.filename, err)
		}
	}

	engine := lua.NewEngine(tmpDir, router)
	scripts := engine.GetLoadedScripts()

	// Verify all scripts are discovered with correct names
	for _, tc := range testCases {
		found := false
		for _, script := range scripts {
			if script == tc.expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find script '%s' in loaded scripts", tc.expected)
		}
	}
}
