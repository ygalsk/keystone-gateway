package tests

import (
	"keystone-gateway/internal/routing"
	"net/http"
)

// responseRecorder is a simple implementation of http.ResponseWriter for testing
type responseRecorder struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (r *responseRecorder) Header() http.Header {
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	return r.Headers
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.Body = append(r.Body, data...)
	if r.StatusCode == 0 {
		r.StatusCode = 200
	}
	return len(data), nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
}

// createLoadTestHandler creates a handler similar to the main application
func createLoadTestHandler(gateway *routing.Gateway) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate headers for malformed content (same as main.go)
		for name := range r.Header {
			for _, char := range name {
				if char == 0 { // null byte
					http.Error(w, "Bad Request: Invalid header name", http.StatusBadRequest)
					return
				}
			}
		}

		// Validate path for null bytes and excessive length
		if len(r.URL.Path) > 1024 { // Reject paths longer than 1KB
			http.NotFound(w, r)
			return
		}
		for _, char := range r.URL.Path {
			if char == 0 { // null byte in path
				http.NotFound(w, r)
				return
			}
		}

		tenantRouter, stripPrefix := gateway.MatchRoute(r.Host, r.URL.Path)
		if tenantRouter == nil {
			http.NotFound(w, r)
			return
		}

		backend := tenantRouter.NextBackend()
		if backend == nil {
			http.Error(w, "No backend available", http.StatusBadGateway)
			return
		}

		proxy := gateway.CreateProxy(backend, stripPrefix)
		proxy.ServeHTTP(w, r)
	})
}
