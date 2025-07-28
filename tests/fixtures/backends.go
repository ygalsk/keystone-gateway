package fixtures

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// BackendBehavior defines how a mock backend should behave
type BackendBehavior struct {
	ResponseMap map[string]BackendResponse // path -> response
	DefaultCode int                        // default status code for unmapped paths
}

// BackendResponse defines a backend response
type BackendResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Delay      time.Duration
}

// CreateSimpleBackend creates a backend that responds with OK to all requests
func CreateSimpleBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write OK response: %v", err)
		}
	}))
}

// CreateHealthCheckBackend creates a backend with a health endpoint
func CreateHealthCheckBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("service response"))
		}
	}))
}

// CreateCustomBackend creates a backend with custom behavior
func CreateCustomBackend(t *testing.T, behavior BackendBehavior) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if response, exists := behavior.ResponseMap[r.URL.Path]; exists {
			// Add delay if specified
			if response.Delay > 0 {
				time.Sleep(response.Delay)
			}

			// Set custom headers
			for key, value := range response.Headers {
				w.Header().Set(key, value)
			}

			w.WriteHeader(response.StatusCode)
			w.Write([]byte(response.Body))
		} else {
			code := behavior.DefaultCode
			if code == 0 {
				code = http.StatusNotFound
			}
			w.WriteHeader(code)
			w.Write([]byte("not found"))
		}
	}))
}

// CreateErrorBackend creates a backend that returns different error types
func CreateErrorBackend(t *testing.T) *httptest.Server {
	return CreateCustomBackend(t, BackendBehavior{
		ResponseMap: map[string]BackendResponse{
			"/500":     {StatusCode: http.StatusInternalServerError, Body: "Internal Server Error"},
			"/404":     {StatusCode: http.StatusNotFound, Body: "Not Found"},
			"/400":     {StatusCode: http.StatusBadRequest, Body: "Bad Request"},
			"/503":     {StatusCode: http.StatusServiceUnavailable, Body: "Service Unavailable"},
			"/timeout": {StatusCode: http.StatusRequestTimeout, Delay: 100 * time.Millisecond},
			"/empty":   {StatusCode: http.StatusOK, Body: ""},
			"/large":   {StatusCode: http.StatusOK, Body: strings.Repeat("x", 1024*1024)},
			"/invalid-json": {
				StatusCode: http.StatusOK,
				Body:       "invalid json {",
				Headers:    map[string]string{"Content-Type": "application/json"},
			},
		},
		DefaultCode: http.StatusOK,
	})
}

// CreateSlowBackend creates a backend that responds slowly
func CreateSlowBackend(t *testing.T, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}))
}

// CreateEchoBackend creates a backend that echoes request information
func CreateEchoBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		response := map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"query":   r.URL.RawQuery,
			"headers": r.Header,
			"body":    string(body),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
}

// CreateHeaderEchoBackend creates a backend that echoes headers back as JSON
func CreateHeaderEchoBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := make(map[string]string)
		for key, values := range r.Header {
			headers[key] = strings.Join(values, ",")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Response", "true")
		json.NewEncoder(w).Encode(headers)
	}))
}

// CreateBodySizeBackend creates a backend that reports request body size
func CreateBodySizeBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("received %d", len(body))))
	}))
}

// CreateDropConnectionBackend creates a backend that drops connections
func CreateDropConnectionBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't write any response, just close the connection immediately
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				if closeErr := conn.Close(); closeErr != nil {
					log.Printf("Failed to close hijacked connection: %v", closeErr)
				}
				return
			}
		}
		// If hijacking fails, return an error status
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("connection drop failed"))
	}))
}

// CreateMethodAwareBackend creates a backend that handles HTTP methods appropriately
func CreateMethodAwareBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow standard HTTP methods
		allowedMethods := map[string]bool{
			"GET":     true,
			"POST":    true,
			"PUT":     true,
			"DELETE":  true,
			"PATCH":   true,
			"HEAD":    true,
			"OPTIONS": true,
		}

		if !allowedMethods[r.Method] {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Method not allowed"))
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write OK response: %v", err)
		}
	}))
}

// CreateRestrictiveBackend creates a backend that only responds to specific paths
func CreateRestrictiveBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only respond to /test, not percent-encoded variations
		if r.URL.Path == "/test" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write OK response: %v", err)
		}
			return
		}

		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
}

// CreateHealthAwareBackend creates a backend that responds to both API routes and health endpoint
func CreateHealthAwareBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle /health at root level (before path stripping)
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"healthy"}`))
			return
		}

		// Handle API routes (these come with stripped paths)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write OK response: %v", err)
		}
	}))
}
