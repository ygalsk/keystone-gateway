// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Setup test scripts directory
	if err := setupTestScripts(); err != nil {
		fmt.Printf("Failed to setup test scripts: %v\n", err)
		os.Exit(1)
	}
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	os.RemoveAll("./test-integration-scripts")
	
	os.Exit(code)
}

func setupTestScripts() error {
	if err := os.MkdirAll("./test-integration-scripts", 0755); err != nil {
		return err
	}
	
	// Create test canary script
	canaryScript := `
function on_route_request(request, backends)
    local canary_header = request.headers["X-Canary"]
    if canary_header == "true" then
        for i, backend in ipairs(backends) do
            if string.find(backend.name, "canary") and backend.health then
                return {
                    selected_backend = backend.name,
                    modified_headers = {["X-Route-Type"] = "canary"}
                }
            end
        end
    end
    
    for i, backend in ipairs(backends) do
        if string.find(backend.name, "stable") and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {["X-Route-Type"] = "stable"}
            }
        end
    end
    
    return {reject = true, reject_reason = "No backends available"}
end`
	
	return os.WriteFile("./test-integration-scripts/canary.lua", []byte(canaryScript), 0644)
}

func TestLuaEngineHTTPHandlers(t *testing.T) {
	engine := NewLuaEngine("./test-integration-scripts")
	
	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/route/", engine.RouteHandler)
	mux.HandleFunc("/health", engine.HealthHandler)
	mux.HandleFunc("/reload", engine.ReloadHandler)
	
	server := httptest.NewServer(mux)
	defer server.Close()
	
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}
		
		if health["status"] != "healthy" {
			t.Errorf("Expected healthy status, got %v", health["status"])
		}
	})
	
	t.Run("RouteRequest_CanaryStable", func(t *testing.T) {
		request := RoutingRequest{
			Method: "GET",
			Path:   "/api/test",
			Host:   "example.com",
			Headers: map[string]string{
				"X-Canary": "false",
			},
			Backends: []Backend{
				{Name: "api-stable", URL: "http://stable:8080", Health: true},
				{Name: "api-canary", URL: "http://canary:8080", Health: true},
			},
		}
		
		jsonData, _ := json.Marshal(request)
		resp, err := http.Post(server.URL+"/route/canary", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			t.Fatalf("Route request failed: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		var response RoutingResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if response.SelectedBackend != "api-stable" {
			t.Errorf("Expected stable backend, got %s", response.SelectedBackend)
		}
	})
	
	t.Run("RouteRequest_CanaryForced", func(t *testing.T) {
		request := RoutingRequest{
			Method: "GET",
			Path:   "/api/test",
			Host:   "example.com",
			Headers: map[string]string{
				"X-Canary": "true",
			},
			Backends: []Backend{
				{Name: "api-stable", URL: "http://stable:8080", Health: true},
				{Name: "api-canary", URL: "http://canary:8080", Health: true},
			},
		}
		
		jsonData, _ := json.Marshal(request)
		resp, err := http.Post(server.URL+"/route/canary", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			t.Fatalf("Route request failed: %v", err)
		}
		defer resp.Body.Close()
		
		var response RoutingResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if response.SelectedBackend != "api-canary" {
			t.Errorf("Expected canary backend, got %s", response.SelectedBackend)
		}
		
		if response.ModifiedHeaders["X-Route-Type"] != "canary" {
			t.Errorf("Expected canary route type, got %s", response.ModifiedHeaders["X-Route-Type"])
		}
	})
	
	t.Run("RouteRequest_NonExistentTenant", func(t *testing.T) {
		request := RoutingRequest{
			Method:   "GET",
			Path:     "/api/test",
			Host:     "example.com",
			Headers:  map[string]string{},
			Backends: []Backend{},
		}
		
		jsonData, _ := json.Marshal(request)
		resp, err := http.Post(server.URL+"/route/nonexistent", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			t.Fatalf("Route request failed: %v", err)
		}
		defer resp.Body.Close()
		
		var response RoutingResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if !response.Reject {
			t.Error("Expected request to be rejected for nonexistent tenant")
		}
	})
	
	t.Run("ScriptReload", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/reload", "application/json", nil)
		if err != nil {
			t.Fatalf("Reload request failed: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode reload response: %v", err)
		}
		
		if result["status"] != "scripts reloaded" {
			t.Errorf("Expected scripts reloaded, got %s", result["status"])
		}
	})
}

func TestLuaEnginePerformance(t *testing.T) {
	engine := NewLuaEngine("./test-integration-scripts")
	
	request := RoutingRequest{
		Method: "GET",
		Path:   "/api/test",
		Host:   "example.com",
		Headers: map[string]string{
			"X-Canary": "false",
		},
		Backends: []Backend{
			{Name: "api-stable", URL: "http://stable:8080", Health: true},
			{Name: "api-canary", URL: "http://canary:8080", Health: true},
		},
	}
	
	// Warmup
	for i := 0; i < 10; i++ {
		_, err := engine.ExecuteScript("canary", request)
		if err != nil {
			t.Fatalf("Warmup execution failed: %v", err)
		}
	}
	
	// Performance test - reduced iterations to avoid memory issues
	iterations := 100
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		_, err := engine.ExecuteScript("canary", request)
		if err != nil {
			t.Fatalf("Performance test execution failed: %v", err)
		}
	}
	
	duration := time.Since(start)
	avgTime := duration / time.Duration(iterations)
	
	t.Logf("Executed %d requests in %v (avg: %v per request)", iterations, duration, avgTime)
	
	// Should process at least 100 requests per second
	if avgTime > 10*time.Millisecond {
		t.Errorf("Performance too slow: avg %v per request (expected < 10ms)", avgTime)
	}
}

func TestLuaEngineErrorHandling(t *testing.T) {
	engine := NewLuaEngine("./test-integration-scripts")
	
	// Add script with syntax error
	badScript := `
function on_route_request(request, backends)
    -- Syntax error: missing end
    if true then
        return {}
end`
	
	engine.scripts["bad"] = badScript
	
	request := RoutingRequest{
		Method:   "GET",
		Path:     "/api/test",
		Host:     "example.com",
		Headers:  map[string]string{},
		Backends: []Backend{},
	}
	
	_, err := engine.ExecuteScript("bad", request)
	if err == nil {
		t.Error("Expected error for bad script, got nil")
	}
	
	if err != nil {
		t.Logf("Expected error caught: %v", err)
	}
}