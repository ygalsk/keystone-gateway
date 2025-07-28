package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// E2EBackend represents a real backend server for E2E testing
type E2EBackend struct {
	Server   *http.Server
	URL      string
	Port     int
	Type     string
	listener net.Listener
}

// StartRealBackend starts a real backend server for E2E testing
func StartRealBackend(t *testing.T, backendType string) *E2EBackend {
	port := GetRandomPort()
	
	// Create handler based on backend type
	handler := createBackendHandler(backendType)
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
		// Set timeouts for backend server
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	
	// Start listening
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatalf("Failed to start E2E backend listener: %v", err)
	}
	
	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Errorf("E2E backend server error: %v", err)
		}
	}()
	
	url := fmt.Sprintf("http://localhost:%d", port)
	
	// Wait for backend to be ready
	if !waitForServer(url, 3*time.Second) {
		t.Fatalf("E2E backend server failed to start within timeout")
	}
	
	backend := &E2EBackend{
		Server:   server,
		URL:      url,
		Port:     port,
		Type:     backendType,
		listener: listener,
	}
	
	t.Logf("Started E2E %s backend at %s", backendType, url)
	return backend
}

// Stop gracefully stops the E2E backend server
func (b *E2EBackend) Stop() error {
	if b.Server == nil {
		return nil
	}
	
	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	err := b.Server.Shutdown(ctx)
	if b.listener != nil {
		if closeErr := b.listener.Close(); closeErr != nil {
			log.Printf("Failed to close listener: %v", closeErr)
		}
	}
	
	return err
}

// createBackendHandler creates different types of backend handlers for E2E testing
func createBackendHandler(backendType string) http.Handler {
	switch backendType {
	case "simple":
		return createSimpleBackendHandler()
	case "echo":
		return createEchoBackendHandler()
	case "health":
		return createHealthBackendHandler()
	case "api":
		return createAPIBackendHandler()
	case "slow":
		return createSlowBackendHandler()
	case "error":
		return createErrorBackendHandler()
	case "json":
		return createJSONBackendHandler()
	default:
		return createSimpleBackendHandler()
	}
}

// createSimpleBackendHandler creates a simple backend that responds with basic messages
func createSimpleBackendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Backend-Type", "simple")
		w.WriteHeader(http.StatusOK)
		
		response := fmt.Sprintf("Simple backend response for %s %s", r.Method, r.URL.Path)
		if _, err := w.Write([]byte(response)); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})
}

// createEchoBackendHandler creates an echo backend that returns request details
func createEchoBackendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Type", "echo")
		
		// Read request body
		body, _ := io.ReadAll(r.Body)
		
		// Create echo response
		response := map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"query":   r.URL.RawQuery,
			"headers": r.Header,
			"body":    string(body),
			"host":    r.Host,
			"remote":  r.RemoteAddr,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
	})
}

// createHealthBackendHandler creates a backend with health check endpoints
func createHealthBackendHandler() http.Handler {
	startTime := time.Now()
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Type", "health")
		
		switch r.URL.Path {
		case "/health":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			
			health := map[string]interface{}{
				"status":    "healthy",
				"uptime":    time.Since(startTime).String(),
				"timestamp": time.Now().Format(time.RFC3339),
				"version":   "1.0.0",
			}
			if err := json.NewEncoder(w).Encode(health); err != nil {
				log.Printf("Failed to encode health response: %v", err)
			}
			
		case "/ready":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			
			ready := map[string]interface{}{
				"ready":     true,
				"timestamp": time.Now().Format(time.RFC3339),
			}
			if err := json.NewEncoder(w).Encode(ready); err != nil {
				log.Printf("Failed to encode ready response: %v", err)
			}
			
		default:
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Health backend service response")); err != nil {
				log.Printf("Failed to write health response: %v", err)
			}
		}
	})
}

// createAPIBackendHandler creates a backend that simulates a REST API
func createAPIBackendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Type", "api")
		
		// Extract resource from path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		resource := "unknown"
		if len(pathParts) > 0 && pathParts[0] != "" {
			resource = pathParts[0]
		}
		
		switch r.Method {
		case "GET":
			if len(pathParts) > 1 && pathParts[1] != "" {
				// GET specific resource
				response := map[string]interface{}{
					"id":       pathParts[1],
					"resource": resource,
					"data":     fmt.Sprintf("Details for %s %s", resource, pathParts[1]),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
			} else {
				// GET collection
				response := map[string]interface{}{
					"resource": resource,
					"items": []map[string]interface{}{
						{"id": "1", "name": fmt.Sprintf("First %s", resource)},
						{"id": "2", "name": fmt.Sprintf("Second %s", resource)},
					},
					"count": 2,
					"timestamp": time.Now().Format(time.RFC3339),
				}
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
			}
			
		case "POST":
			// Read request body
			body, _ := io.ReadAll(r.Body)
			
			response := map[string]interface{}{
				"resource":  resource,
				"action":    "created",
				"id":        fmt.Sprintf("new-%d", time.Now().Unix()),
				"data":      string(body),
				"timestamp": time.Now().Format(time.RFC3339),
			}
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
			
		case "PUT", "PATCH":
			if len(pathParts) > 1 && pathParts[1] != "" {
				body, _ := io.ReadAll(r.Body)
				
				response := map[string]interface{}{
					"resource":  resource,
					"action":    "updated",
					"id":        pathParts[1],
					"data":      string(body),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				if err := json.NewEncoder(w).Encode(map[string]string{"error": "ID required for update"}); err != nil {
					log.Printf("Failed to encode error response: %v", err)
				}
			}
			
		case "DELETE":
			if len(pathParts) > 1 && pathParts[1] != "" {
				response := map[string]interface{}{
					"resource":  resource,
					"action":    "deleted",
					"id":        pathParts[1],
					"timestamp": time.Now().Format(time.RFC3339),
				}
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				if err := json.NewEncoder(w).Encode(map[string]string{"error": "ID required for delete"}); err != nil {
					log.Printf("Failed to encode error response: %v", err)
				}
			}
			
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"}); err != nil {
				log.Printf("Failed to encode error response: %v", err)
			}
		}
	})
}

// createSlowBackendHandler creates a backend that responds slowly
func createSlowBackendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow processing
		delay := 500 * time.Millisecond
		if delayParam := r.URL.Query().Get("delay"); delayParam != "" {
			if parsedDelay, err := time.ParseDuration(delayParam); err == nil {
				delay = parsedDelay
			}
		}
		
		time.Sleep(delay)
		
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Type", "slow")
		w.Header().Set("X-Processing-Time", delay.String())
		
		response := map[string]interface{}{
			"message":        "Slow backend response",
			"processing_time": delay.String(),
			"timestamp":      time.Now().Format(time.RFC3339),
			"path":          r.URL.Path,
		}
		
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
	})
}

// createErrorBackendHandler creates a backend that returns various errors
func createErrorBackendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Type", "error")
		
		// Determine error type from path
		path := strings.Trim(r.URL.Path, "/")
		
		switch path {
		case "400", "bad-request":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Bad Request",
				"code":  "400",
			})
		case "401", "unauthorized":
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized",
				"code":  "401",
			})
		case "403", "forbidden":
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Forbidden",
				"code":  "403",
			})
		case "404", "not-found":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not Found",
				"code":  "404",
			})
		case "500", "internal-error":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Internal Server Error",
				"code":  "500",
			})
		case "503", "unavailable":
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Service Unavailable",
				"code":  "503",
			})
		default:
			// Default error response
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unknown error type",
				"path":  path,
			})
		}
	})
}

// createJSONBackendHandler creates a backend that serves JSON data
func createJSONBackendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Type", "json")
		
		// Sample JSON data based on path
		path := strings.Trim(r.URL.Path, "/")
		
		var response interface{}
		
		switch path {
		case "users":
			response = []map[string]interface{}{
				{"id": 1, "name": "John Doe", "email": "john@example.com"},
				{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
			}
		case "products":
			response = []map[string]interface{}{
				{"id": 1, "name": "Product A", "price": 29.99},
				{"id": 2, "name": "Product B", "price": 49.99},
			}
		case "orders":
			response = []map[string]interface{}{
				{"id": 1, "user_id": 1, "total": 79.98, "status": "completed"},
				{"id": 2, "user_id": 2, "total": 29.99, "status": "pending"},
			}
		default:
			response = map[string]interface{}{
				"message":   "JSON Backend",
				"path":      path,
				"method":    r.Method,
				"timestamp": time.Now().Format(time.RFC3339),
			}
		}
		
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode JSON response: %v", err)
		}
	})
}

// E2EBackendCluster represents multiple backend servers for cluster testing
type E2EBackendCluster struct {
	Backends []E2EBackend
	Type     string
}

// StartBackendCluster starts multiple backend servers of the same type
func StartBackendCluster(t *testing.T, backendType string, count int) *E2EBackendCluster {
	if count <= 0 {
		t.Fatal("Backend cluster must have at least 1 backend")
	}
	
	var backends []E2EBackend
	
	for i := 0; i < count; i++ {
		backend := StartRealBackend(t, backendType)
		backends = append(backends, *backend)
		t.Logf("Started backend %d/%d of type %s at %s", i+1, count, backendType, backend.URL)
	}
	
	return &E2EBackendCluster{
		Backends: backends,
		Type:     backendType,
	}
}

// Stop stops all backends in the cluster
func (c *E2EBackendCluster) Stop() error {
	var lastErr error
	
	for i := range c.Backends {
		if err := c.Backends[i].Stop(); err != nil {
			lastErr = err
		}
	}
	
	return lastErr
}

// GetBackendURLs returns all backend URLs in the cluster
func (c *E2EBackendCluster) GetBackendURLs() []string {
	urls := make([]string, len(c.Backends))
	for i, backend := range c.Backends {
		urls[i] = backend.URL
	}
	return urls
}