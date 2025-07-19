//go:build integration
// +build integration

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLuaIntegration(t *testing.T) {
	// Create test config with Lua engine
	testConfig := &Config{
		LuaEngine: &LuaEngineConfig{
			Enabled: true,
			URL:     "http://localhost:8081",
			Timeout: "5s",
		},
		Tenants: []Tenant{
			{
				Name:      "canary-test",
				Domains:   []string{"test.example.com"},
				LuaScript: "canary",
				Services: []Service{
					{
						Name:   "api-stable",
						URL:    "http://localhost:9001",
						Health: "/health",
						Labels: map[string]string{"version": "stable"},
					},
					{
						Name:   "api-canary",
						URL:    "http://localhost:9002",
						Health: "/health",
						Labels: map[string]string{"version": "canary"},
					},
				},
			},
		},
	}

	// Create test backends
	stableServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "stable")
		w.Write([]byte("stable response"))
	}))
	defer stableServer.Close()

	canaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "canary")
		w.Write([]byte("canary response"))
	}))
	defer canaryServer.Close()

	// Create mock Lua engine that returns canary backend when X-Canary header is true
	luaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
		case "/route/canary":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), `"X-Canary":"true"`) {
				// Force canary routing
				w.Write([]byte(`{
					"selected_backend": "api-canary",
					"modified_headers": {"X-Route-Type": "forced-canary"}
				}`))
			} else {
				// Default to stable
				w.Write([]byte(`{
					"selected_backend": "api-stable",
					"modified_headers": {"X-Route-Type": "stable"}
				}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer luaServer.Close()

	// Update config with actual server URLs
	testConfig.LuaEngine.URL = luaServer.URL
	testConfig.Tenants[0].Services[0].URL = stableServer.URL
	testConfig.Tenants[0].Services[1].URL = canaryServer.URL

	// Create gateway
	gateway := NewGateway(testConfig)
	router := gateway.SetupRouter()

	// Test server
	testServer := httptest.NewServer(router)
	defer testServer.Close()

	t.Run("LuaRouting_Stable", func(t *testing.T) {
		req, _ := http.NewRequest("GET", testServer.URL+"/test", nil)
		req.Host = "test.example.com"
		req.Header.Set("X-Canary", "false")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "stable") {
			t.Errorf("Expected stable response, got: %s", string(body))
		}
	})

	t.Run("LuaRouting_ForcedCanary", func(t *testing.T) {
		req, _ := http.NewRequest("GET", testServer.URL+"/test", nil)
		req.Host = "test.example.com"
		req.Header.Set("X-Canary", "true")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "canary") {
			t.Errorf("Expected canary response, got: %s", string(body))
		}
	})

	t.Run("LuaEngineDown_Fallback", func(t *testing.T) {
		// Stop mock Lua server to test fallback
		luaServer.Close()

		req, _ := http.NewRequest("GET", testServer.URL+"/test", nil)
		req.Host = "test.example.com"

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should fallback to round-robin and return one of the backends
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected fallback to work, got status: %d", resp.StatusCode)
		}
	})
}

func TestGatewayWithoutLua(t *testing.T) {
	// Test gateway works without Lua engine configured
	testConfig := &Config{
		Tenants: []Tenant{
			{
				Name:    "simple-test",
				Domains: []string{"simple.example.com"},
				Services: []Service{
					{
						Name:   "backend",
						URL:    "http://localhost:9000",
						Health: "/health",
					},
				},
			},
		},
	}

	// Create test backend
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("simple response"))
	}))
	defer backendServer.Close()

	testConfig.Tenants[0].Services[0].URL = backendServer.URL

	// Create gateway without Lua
	gateway := NewGateway(testConfig)
	router := gateway.SetupRouter()

	testServer := httptest.NewServer(router)
	defer testServer.Close()

	req, _ := http.NewRequest("GET", testServer.URL+"/test", nil)
	req.Host = "simple.example.com"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "simple response") {
		t.Errorf("Expected simple response, got: %s", string(body))
	}
}

func TestLuaConfigValidation(t *testing.T) {
	// Test configuration loading with Lua engine
	configContent := `
lua_engine:
  enabled: true
  url: "http://localhost:8081"
  timeout: "5s"

tenants:
  - name: "test-tenant"
    domains: ["test.com"]
    lua_script: "canary"
    services:
      - name: "service1"
        url: "http://localhost:3000"
        health: "/health"
        labels:
          version: "stable"
`

	// Write test config
	tempFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tempFile.Close()

	// Load config
	config, err := LoadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.LuaEngine == nil || !config.LuaEngine.Enabled {
		t.Error("Lua engine should be enabled")
	}

	if config.LuaEngine.URL != "http://localhost:8081" {
		t.Errorf("Expected Lua URL http://localhost:8081, got: %s", config.LuaEngine.URL)
	}

	if len(config.Tenants) != 1 {
		t.Errorf("Expected 1 tenant, got: %d", len(config.Tenants))
	}

	if config.Tenants[0].LuaScript != "canary" {
		t.Errorf("Expected lua_script 'canary', got: %s", config.Tenants[0].LuaScript)
	}

	if len(config.Tenants[0].Services[0].Labels) == 0 {
		t.Error("Expected service labels to be loaded")
	}
}
