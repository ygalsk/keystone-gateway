package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LuaClient handles communication with lua-stone service
type LuaClient struct {
	baseURL string
	client  *http.Client
}

// RoutingRequest represents the request sent to lua-stone service
type RoutingRequest struct {
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Host     string            `json:"host"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body,omitempty"`
	Backends []Backend         `json:"backends"`
}

// Backend represents available backend services for lua-stone
type Backend struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Health bool   `json:"health"`
}

// RoutingResponse represents the lua script's routing decision
type RoutingResponse struct {
	SelectedBackend string            `json:"selected_backend"`
	ModifiedHeaders map[string]string `json:"modified_headers,omitempty"`
	ModifiedPath    string            `json:"modified_path,omitempty"`
	Reject          bool              `json:"reject,omitempty"`
	RejectReason    string            `json:"reject_reason,omitempty"`
}

// NewLuaClient creates a new lua-stone client
func NewLuaClient(baseURL string) *LuaClient {
	return &LuaClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// RouteRequest sends a routing request to lua-stone service
func (lc *LuaClient) RouteRequest(tenantName string, req RoutingRequest) (*RoutingResponse, error) {
	url := fmt.Sprintf("%s/route/%s", lc.baseURL, tenantName)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := lc.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call lua-stone: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lua-stone returned status: %d", resp.StatusCode)
	}

	var routingResp RoutingResponse
	if err := json.NewDecoder(resp.Body).Decode(&routingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &routingResp, nil
}

// HealthCheck checks if lua-stone service is healthy
func (lc *LuaClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", lc.baseURL)

	resp, err := lc.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to call lua-stone health endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lua-stone health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ReloadScripts reloads Lua scripts in lua-stone service
func (lc *LuaClient) ReloadScripts() error {
	url := fmt.Sprintf("%s/reload", lc.baseURL)

	resp, err := lc.client.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to reload lua scripts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("script reload failed with status: %d", resp.StatusCode)
	}

	return nil
}
