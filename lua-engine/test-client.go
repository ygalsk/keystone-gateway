// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// LuaClient handles communication with Lua Engine
type LuaClient struct {
	baseURL string
	client  *http.Client
}

// RoutingRequest represents the request sent to Lua Engine
type RoutingRequest struct {
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Host     string            `json:"host"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body,omitempty"`
	Backends []Backend         `json:"backends"`
}

// Backend represents available backend services for Lua Engine
type Backend struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Health bool   `json:"health"`
}

// RoutingResponse represents the Lua script's routing decision
type RoutingResponse struct {
	SelectedBackend string            `json:"selected_backend"`
	ModifiedHeaders map[string]string `json:"modified_headers,omitempty"`
	ModifiedPath    string            `json:"modified_path,omitempty"`
	Reject          bool              `json:"reject,omitempty"`
	RejectReason    string            `json:"reject_reason,omitempty"`
}

func NewLuaClient(baseURL string) *LuaClient {
	return &LuaClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (lc *LuaClient) RouteRequest(tenantName string, req RoutingRequest) (*RoutingResponse, error) {
	url := fmt.Sprintf("%s/route/%s", lc.baseURL, tenantName)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := lc.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call lua engine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lua engine returned status: %d", resp.StatusCode)
	}

	var routingResp RoutingResponse
	if err := json.NewDecoder(resp.Body).Decode(&routingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &routingResp, nil
}

func (lc *LuaClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", lc.baseURL)

	resp, err := lc.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to call lua engine health endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lua engine health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test-client.go <tenant-name>")
		fmt.Println("Available tenants: canary, blue-green, ab-testing")
		os.Exit(1)
	}

	tenantName := os.Args[1]
	luaEngineURL := "http://localhost:8081"

	// Create Lua client
	client := NewLuaClient(luaEngineURL)

	// Test health check first
	if err := client.HealthCheck(); err != nil {
		log.Fatalf("Lua engine health check failed: %v", err)
	}
	fmt.Println("✅ Lua engine is healthy")

	// Create test request
	request := RoutingRequest{
		Method: "GET",
		Path:   "/api/users",
		Host:   "api.example.com",
		Headers: map[string]string{
			"User-Agent":       "test-client/1.0",
			"X-User-ID":        "user123",
			"X-Canary":         "false",
			"X-Canary-Percent": "20",
		},
		Backends: []Backend{
			{
				Name:   "api-stable",
				URL:    "http://backend1:8080",
				Health: true,
			},
			{
				Name:   "api-canary",
				URL:    "http://backend2:8080",
				Health: true,
			},
			{
				Name:   "service-version-a",
				URL:    "http://backend3:8080",
				Health: true,
			},
			{
				Name:   "service-version-b",
				URL:    "http://backend4:8080",
				Health: true,
			},
		},
	}

	// Test routing
	fmt.Printf("🧪 Testing routing for tenant: %s\n", tenantName)

	// Run multiple requests to see distribution
	for i := 0; i < 5; i++ {
		response, err := client.RouteRequest(tenantName, request)
		if err != nil {
			log.Printf("❌ Request %d failed: %v", i+1, err)
			continue
		}

		if response.Reject {
			fmt.Printf("❌ Request %d rejected: %s\n", i+1, response.RejectReason)
			continue
		}

		fmt.Printf("✅ Request %d: routed to %s\n", i+1, response.SelectedBackend)

		if len(response.ModifiedHeaders) > 0 {
			fmt.Printf("   📝 Modified headers: %v\n", response.ModifiedHeaders)
		}

		if response.ModifiedPath != "" {
			fmt.Printf("   🔄 Modified path: %s\n", response.ModifiedPath)
		}
	}

	// Test different headers for canary
	if tenantName == "canary" {
		fmt.Println("\n🧪 Testing canary deployment with forced routing...")
		request.Headers["X-Canary"] = "true"

		response, err := client.RouteRequest(tenantName, request)
		if err != nil {
			log.Printf("❌ Forced canary request failed: %v", err)
		} else {
			fmt.Printf("✅ Forced canary: routed to %s\n", response.SelectedBackend)
		}
	}

	// Test A/B with different user IDs
	if tenantName == "ab-testing" {
		fmt.Println("\n🧪 Testing A/B with different user IDs...")
		userIDs := []string{"user001", "user002", "user003", "user004", "user005"}

		for _, userID := range userIDs {
			request.Headers["X-User-ID"] = userID
			response, err := client.RouteRequest(tenantName, request)
			if err != nil {
				log.Printf("❌ A/B test for %s failed: %v", userID, err)
				continue
			}
			fmt.Printf("✅ User %s: routed to %s\n", userID, response.SelectedBackend)
		}
	}

	fmt.Println("\n✨ Test completed!")
}
