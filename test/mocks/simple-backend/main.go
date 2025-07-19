// Package main provides a simple Go-based mock backend for testing.
// This replaces the Node.js-based mock backends for lighter testing.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// Response represents a simple API response
type Response struct {
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers,omitempty"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status  string    `json:"status"`
	Uptime  string    `json:"uptime"`
	Version string    `json:"version"`
	Time    time.Time `json:"time"`
}

var startTime = time.Now()

func main() {
	port := "8081"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := HealthResponse{
			Status:  "healthy",
			Uptime:  time.Since(startTime).String(),
			Version: "mock-1.0.0",
			Time:    time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Generic API endpoint
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}

		response := Response{
			Message:   "Mock API Response",
			Timestamp: time.Now(),
			Headers:   headers,
			Method:    r.Method,
			Path:      r.URL.Path,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			response := Response{
				Message:   "Simple Go Mock Backend",
				Timestamp: time.Now(),
				Method:    r.Method,
				Path:      r.URL.Path,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			http.NotFound(w, r)
		}
	})

	log.Printf("Simple Go mock backend starting on port %s", port)
	log.Printf("Endpoints: /health, /api/*, /")

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
