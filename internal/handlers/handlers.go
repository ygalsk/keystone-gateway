package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/routing"
)

type HealthStatus struct {
	Status  string            `json:"status"`
	Tenants map[string]string `json:"tenants"`
	Uptime  string            `json:"uptime"`
	Version string            `json:"version"`
}

type Handlers struct {
	gateway   *routing.Gateway
	luaEngine *lua.Engine
	version   string
	startTime time.Time
}

func New(gateway *routing.Gateway, luaEngine *lua.Engine, version string) *Handlers {
	return &Handlers{
		gateway:   gateway,
		luaEngine: luaEngine,
		version:   version,
		startTime: time.Now(),
	}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:  "healthy",
		Tenants: make(map[string]string),
		Version: h.version,
		Uptime:  time.Since(h.startTime).String(),
	}

	cfg := h.gateway.GetConfig()
	for _, tenant := range cfg.Tenants {
		// In the new simplified design, we don't track backend health the same way
		// For now, report all tenants as "healthy" - health checking is handled by Gateway
		status.Tenants[tenant.Name] = fmt.Sprintf("%d backends configured", len(tenant.Services))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *Handlers) Tenants(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := h.gateway.GetConfig()
	json.NewEncoder(w).Encode(cfg.Tenants)
}

func (h *Handlers) Proxy(w http.ResponseWriter, r *http.Request) {
	// In the new design, this handler should not be called directly
	// The Gateway handles all routing and proxying internally
	http.Error(w, "Direct proxy handler should not be called", http.StatusInternalServerError)
}

func (h *Handlers) LuaFallback(w http.ResponseWriter, r *http.Request) {
	// In the new design, this handler should not be called directly
	// The Gateway handles all routing and Lua fallback internally
	http.Error(w, "Lua fallback handler should not be called", http.StatusInternalServerError)
}
