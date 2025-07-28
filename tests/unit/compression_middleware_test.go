package unit

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// TestCompressionMiddleware tests the compression middleware functionality
func TestCompressionMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		acceptEncoding string
		body           string
		expectCompressed bool
		expectHeader   string
	}{
		{
			name:           "JSON content with gzip acceptance",
			contentType:    "application/json",
			acceptEncoding: "gzip, deflate",
			body:           `{"message": "Hello, World!", "data": [1, 2, 3, 4, 5]}`,
			expectCompressed: true,
			expectHeader:   "gzip",
		},
		{
			name:           "HTML content with gzip acceptance",
			contentType:    "text/html",
			acceptEncoding: "gzip",
			body:           "<html><body><h1>Test Page</h1><p>This is a test page with content.</p></body></html>",
			expectCompressed: true,
			expectHeader:   "gzip",
		},
		{
			name:           "CSS content with gzip acceptance",
			contentType:    "text/css",
			acceptEncoding: "gzip, deflate, br",
			body:           "body { font-family: Arial, sans-serif; background-color: #f0f0f0; }",
			expectCompressed: true,
			expectHeader:   "gzip",
		},
		{
			name:           "JavaScript content with gzip acceptance",
			contentType:    "text/javascript",
			acceptEncoding: "gzip",
			body:           "function test() { console.log('Hello, World!'); return true; }",
			expectCompressed: true,
			expectHeader:   "gzip",
		},
		{
			name:           "Plain text with gzip acceptance",
			contentType:    "text/plain",
			acceptEncoding: "gzip",
			body:           "This is a plain text response that should be compressed.",
			expectCompressed: true,
			expectHeader:   "gzip",
		},
		{
			name:           "XML content with gzip acceptance",
			contentType:    "application/xml",
			acceptEncoding: "gzip",
			body:           "<?xml version=\"1.0\"?><root><item>test</item></root>",
			expectCompressed: true,
			expectHeader:   "gzip",
		},
		{
			name:           "Image content should not be compressed",
			contentType:    "image/jpeg",
			acceptEncoding: "gzip",
			body:           "fake-image-binary-data",
			expectCompressed: false,
			expectHeader:   "",
		},
		{
			name:           "Binary content should not be compressed",
			contentType:    "application/octet-stream",
			acceptEncoding: "gzip",
			body:           "binary-data-content",
			expectCompressed: false,
			expectHeader:   "",
		},
		{
			name:           "No compression when client doesn't accept",
			contentType:    "application/json",
			acceptEncoding: "identity",
			body:           `{"message": "Hello, World!"}`,
			expectCompressed: false,
			expectHeader:   "",
		},
		{
			name:           "No Accept-Encoding header",
			contentType:    "application/json",
			acceptEncoding: "",
			body:           `{"message": "Hello, World!"}`,
			expectCompressed: false,
			expectHeader:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test router with compression middleware
			r := chi.NewRouter()
			r.Use(middleware.Compress(5, 
				"text/html", 
				"text/css", 
				"text/javascript", 
				"application/json", 
				"application/xml",
				"text/plain",
			))

			// Add a test handler that returns the test content
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			})

			// Create a test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			// Record the response
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Check response status
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Check Content-Encoding header
			contentEncoding := w.Header().Get("Content-Encoding")
			if tt.expectCompressed {
				if contentEncoding != tt.expectHeader {
					t.Errorf("Expected Content-Encoding %q, got %q", tt.expectHeader, contentEncoding)
				}

				// Verify Vary header is set when compression is applied
				vary := w.Header().Get("Vary")
				if !strings.Contains(vary, "Accept-Encoding") {
					t.Errorf("Expected Vary header to contain 'Accept-Encoding', got %q", vary)
				}

				// Verify the content is actually compressed
				if contentEncoding == "gzip" {
					body := w.Body.Bytes()
					if len(body) == 0 {
						t.Error("Expected compressed body, got empty response")
					}

					// Try to decompress and verify content matches
					reader, err := gzip.NewReader(bytes.NewReader(body))
					if err != nil {
						t.Fatalf("Failed to create gzip reader: %v", err)
					}
					defer func() {
						if err := reader.Close(); err != nil {
							t.Logf("Failed to close gzip reader: %v", err)
						}
					}()

					decompressed, err := io.ReadAll(reader)
					if err != nil {
						t.Fatalf("Failed to decompress response: %v", err)
					}

					if string(decompressed) != tt.body {
						t.Errorf("Decompressed content doesn't match original. Expected %q, got %q", tt.body, string(decompressed))
					}
				}
			} else {
				if contentEncoding != "" {
					t.Errorf("Expected no compression, but got Content-Encoding %q", contentEncoding)
				}

				// Verify content matches exactly when not compressed
				body := w.Body.String()
				if body != tt.body {
					t.Errorf("Uncompressed content doesn't match. Expected %q, got %q", tt.body, body)
				}
			}
		})
	}
}

// TestCompressionLevels tests different compression levels
func TestCompressionLevels(t *testing.T) {
	levels := []int{1, 5, 9} // fast, balanced, best compression
	testContent := strings.Repeat("This is test content that should compress well. ", 100)

	for _, level := range levels {
		t.Run(fmt.Sprintf("Level_%d", level), func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(middleware.Compress(level, "text/plain"))

			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(testContent)); err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			if w.Header().Get("Content-Encoding") != "gzip" {
				t.Error("Expected gzip compression")
			}

			// Verify the response is compressed (smaller than original)
			if w.Body.Len() >= len(testContent) {
				t.Errorf("Expected compressed size to be smaller than original. Original: %d, Compressed: %d", len(testContent), w.Body.Len())
			}

			// Verify we can decompress and get the original content
			reader, err := gzip.NewReader(w.Body)
			if err != nil {
				t.Fatalf("Failed to create gzip reader: %v", err)
			}
			defer func() {
				if err := reader.Close(); err != nil {
					t.Logf("Failed to close gzip reader: %v", err)
				}
			}()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to decompress: %v", err)
			}

			if string(decompressed) != testContent {
				t.Error("Decompressed content doesn't match original")
			}
		})
	}
}

// TestCompressionWithRealWorldScenarios tests compression in realistic scenarios
func TestCompressionWithRealWorldScenarios(t *testing.T) {
	scenarios := []struct {
		name        string
		handler     func(w http.ResponseWriter, r *http.Request)
		expectGzip  bool
		description string
	}{
		{
			name: "JSON API Response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]interface{}{
					"users": []map[string]string{
						{"id": "1", "name": "John Doe", "email": "john@example.com"},
						{"id": "2", "name": "Jane Smith", "email": "jane@example.com"},
					},
					"total": 2,
					"page":  1,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Fatalf("Failed to encode JSON: %v", err)
				}
			},
			expectGzip:  true,
			description: "API responses should be compressed",
		},
		{
			name: "Health Check Response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write([]byte(`{"status":"healthy","timestamp":"2024-01-01T00:00:00Z"}`)); err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			},
			expectGzip:  true,
			description: "Health check JSON should be compressed",
		},
		{
			name: "Error Response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				if _, err := w.Write([]byte(`{"error":"Invalid request","code":400}`)); err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			},
			expectGzip:  true,
			description: "Error responses should be compressed",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(middleware.Compress(5, 
				"text/html", 
				"text/css", 
				"text/javascript", 
				"application/json", 
				"application/xml",
				"text/plain",
			))

			r.Get("/test", scenario.handler)

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Accept-Encoding", "gzip, deflate")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if scenario.expectGzip {
				if w.Header().Get("Content-Encoding") != "gzip" {
					t.Errorf("%s: Expected gzip compression", scenario.description)
				}
			} else {
				if w.Header().Get("Content-Encoding") != "" {
					t.Errorf("%s: Expected no compression", scenario.description)
				}
			}
		})
	}
}