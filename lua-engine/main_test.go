package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLuaEngine_ExecuteScript(t *testing.T) {
	engine := NewLuaEngine("./test-scripts")
	
	// Create test script content
	testScript := `
function on_route_request(request, backends)
    if request.method == "GET" then
        return {
            selected_backend = "test-backend",
            modified_headers = {
                ["X-Test"] = "true"
            }
        }
    end
    
    return {
        reject = true,
        reject_reason = "Only GET requests allowed"
    }
end`
	
	engine.scripts["test"] = testScript
	
	req := RoutingRequest{
		Method: "GET",
		Path:   "/test",
		Host:   "example.com",
		Headers: map[string]string{
			"User-Agent": "test",
		},
		Backends: []Backend{
			{Name: "test-backend", URL: "http://localhost:8080", Health: true},
		},
	}
	
	response, err := engine.ExecuteScript("test", req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if response.SelectedBackend != "test-backend" {
		t.Errorf("Expected selected_backend to be 'test-backend', got: %s", response.SelectedBackend)
	}
	
	if response.ModifiedHeaders["X-Test"] != "true" {
		t.Errorf("Expected X-Test header to be 'true', got: %s", response.ModifiedHeaders["X-Test"])
	}
}

func TestLuaEngine_ExecuteScript_Reject(t *testing.T) {
	engine := NewLuaEngine("./test-scripts")
	
	testScript := `
function on_route_request(request, backends)
    return {
        reject = true,
        reject_reason = "Test rejection"
    }
end`
	
	engine.scripts["test"] = testScript
	
	req := RoutingRequest{
		Method: "POST",
		Path:   "/test",
		Host:   "example.com",
		Headers: map[string]string{},
		Backends: []Backend{
			{Name: "test-backend", URL: "http://localhost:8080", Health: true},
		},
	}
	
	response, err := engine.ExecuteScript("test", req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if !response.Reject {
		t.Error("Expected request to be rejected")
	}
	
	if response.RejectReason != "Test rejection" {
		t.Errorf("Expected reject reason to be 'Test rejection', got: %s", response.RejectReason)
	}
}

func TestLuaEngine_ExecuteScript_NonExistentTenant(t *testing.T) {
	engine := NewLuaEngine("./test-scripts")
	
	req := RoutingRequest{
		Method:   "GET",
		Path:     "/test",
		Host:     "example.com",
		Headers:  map[string]string{},
		Backends: []Backend{},
	}
	
	response, err := engine.ExecuteScript("nonexistent", req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if !response.Reject {
		t.Error("Expected request to be rejected for nonexistent tenant")
	}
	
	expectedReason := "No script found for tenant: nonexistent"
	if response.RejectReason != expectedReason {
		t.Errorf("Expected reject reason to be '%s', got: %s", expectedReason, response.RejectReason)
	}
}

func TestLuaEngine_ExecuteScript_Timeout(t *testing.T) {
	engine := NewLuaEngine("./test-scripts")
	
	// Script that runs infinitely
	testScript := `
function on_route_request(request, backends)
    while true do
        -- infinite loop
    end
    return {}
end`
	
	engine.scripts["test"] = testScript
	
	req := RoutingRequest{
		Method:   "GET",
		Path:     "/test",
		Host:     "example.com",
		Headers:  map[string]string{},
		Backends: []Backend{},
	}
	
	start := time.Now()
	_, err := engine.ExecuteScript("test", req)
	duration := time.Since(start)
	
	if err == nil {
		t.Error("Expected timeout error")
	}
	
	if duration > MaxScriptExecutionTime+time.Second {
		t.Errorf("Script took too long to timeout: %v", duration)
	}
}

func TestRoutingRequest_JSON(t *testing.T) {
	req := RoutingRequest{
		Method: "GET",
		Path:   "/api/test",
		Host:   "example.com",
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
		Body: "test body",
		Backends: []Backend{
			{Name: "backend1", URL: "http://localhost:8080", Health: true},
			{Name: "backend2", URL: "http://localhost:8081", Health: false},
		},
	}
	
	// Test marshaling
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	
	// Test unmarshaling
	var unmarshaled RoutingRequest
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}
	
	if unmarshaled.Method != req.Method {
		t.Errorf("Method mismatch: expected %s, got %s", req.Method, unmarshaled.Method)
	}
	
	if len(unmarshaled.Backends) != len(req.Backends) {
		t.Errorf("Backends count mismatch: expected %d, got %d", len(req.Backends), len(unmarshaled.Backends))
	}
}

func TestRoutingResponse_JSON(t *testing.T) {
	resp := RoutingResponse{
		SelectedBackend: "backend1",
		ModifiedHeaders: map[string]string{
			"X-Custom": "value",
		},
		ModifiedPath: "/modified/path",
		Reject:       false,
	}
	
	// Test marshaling
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}
	
	// Test unmarshaling
	var unmarshaled RoutingResponse
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if unmarshaled.SelectedBackend != resp.SelectedBackend {
		t.Errorf("SelectedBackend mismatch: expected %s, got %s", resp.SelectedBackend, unmarshaled.SelectedBackend)
	}
	
	if len(unmarshaled.ModifiedHeaders) != len(resp.ModifiedHeaders) {
		t.Errorf("ModifiedHeaders count mismatch: expected %d, got %d", len(resp.ModifiedHeaders), len(unmarshaled.ModifiedHeaders))
	}
}