package unit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"keystone-gateway/internal/config"
)

// TestCompressionConfiguration tests that compression can be configured via config
func TestCompressionConfiguration(t *testing.T) {
	tests := []struct {
		name               string
		compressionConfig  *config.CompressionConfig
		expectCompression  bool
		expectedLevel      int
		expectedTypes      []string
		description        string
	}{
		{
			name:              "Default compression when no config provided",
			compressionConfig: nil,
			expectCompression: true,
			expectedLevel:     5,
			expectedTypes:     []string{"text/html", "text/css", "text/javascript", "application/json", "application/xml", "text/plain"},
			description:       "Should use default settings when compression config is nil",
		},
		{
			name: "Custom compression level and types",
			compressionConfig: &config.CompressionConfig{
				Enabled: true,
				Level:   9,
				ContentTypes: []string{"application/json", "text/plain"},
			},
			expectCompression: true,
			expectedLevel:     9,
			expectedTypes:     []string{"application/json", "text/plain"},
			description:       "Should use custom compression settings",
		},
		{
			name: "Compression disabled",
			compressionConfig: &config.CompressionConfig{
				Enabled: false,
				Level:   5,
				ContentTypes: []string{"application/json"},
			},
			expectCompression: false,
			description:       "Should not compress when disabled",
		},
		{
			name: "Default level when not specified",
			compressionConfig: &config.CompressionConfig{
				Enabled: true,
				// Level not specified (0)
				ContentTypes: []string{"application/json"},
			},
			expectCompression: true,
			expectedLevel:     5,
			expectedTypes:     []string{"application/json"},
			description:       "Should use default level 5 when not specified",
		},
		{
			name: "Default content types when not specified",
			compressionConfig: &config.CompressionConfig{
				Enabled: true,
				Level:   3,
				// ContentTypes not specified (empty)
			},
			expectCompression: true,
			expectedLevel:     3,
			expectedTypes:     []string{"text/html", "text/css", "text/javascript", "application/json", "application/xml", "text/plain"},
			description:       "Should use default content types when not specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with compression settings
			cfg := &config.Config{
				Compression: tt.compressionConfig,
			}

			// Get compression config using the helper method
			compressionConfig := cfg.GetCompressionConfig()

			// Verify configuration values
			if compressionConfig.Enabled != tt.expectCompression {
				t.Errorf("%s: Expected compression enabled %v, got %v", tt.description, tt.expectCompression, compressionConfig.Enabled)
			}

			if tt.expectCompression {
				if compressionConfig.Level != tt.expectedLevel {
					t.Errorf("%s: Expected compression level %d, got %d", tt.description, tt.expectedLevel, compressionConfig.Level)
				}

				if len(compressionConfig.ContentTypes) != len(tt.expectedTypes) {
					t.Errorf("%s: Expected %d content types, got %d", tt.description, len(tt.expectedTypes), len(compressionConfig.ContentTypes))
				} else {
					for i, expectedType := range tt.expectedTypes {
						if compressionConfig.ContentTypes[i] != expectedType {
							t.Errorf("%s: Expected content type %q at index %d, got %q", tt.description, expectedType, i, compressionConfig.ContentTypes[i])
						}
					}
				}
			}

			t.Logf("%s: Configuration test passed - Enabled: %v, Level: %d, Types: %v", 
				tt.description, compressionConfig.Enabled, compressionConfig.Level, compressionConfig.ContentTypes)
		})
	}
}

// TestCompressionConfigurationIntegration tests compression configuration in a realistic scenario
func TestCompressionConfigurationIntegration(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.CompressionConfig
		contentType string
		acceptEnc   string
		expectComp  bool
		description string
	}{
		{
			name: "JSON compression with custom config",
			config: &config.CompressionConfig{
				Enabled: true,
				Level:   7,
				ContentTypes: []string{"application/json"},
			},
			contentType: "application/json",
			acceptEnc:   "gzip",
			expectComp:  true,
			description: "Should compress JSON with custom settings",
		},
		{
			name: "HTML not compressed with JSON-only config",
			config: &config.CompressionConfig{
				Enabled: true,
				Level:   5,
				ContentTypes: []string{"application/json"},
			},
			contentType: "text/html",
			acceptEnc:   "gzip",
			expectComp:  false,
			description: "Should not compress HTML when only JSON is configured",
		},
		{
			name: "Compression disabled globally",
			config: &config.CompressionConfig{
				Enabled: false,
			},
			contentType: "application/json",
			acceptEnc:   "gzip",
			expectComp:  false,
			description: "Should not compress when disabled globally",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create config and simulate application setup
			cfg := &config.Config{
				Compression: tc.config,
			}

			// Create router and apply compression middleware based on config
			r := chi.NewRouter()
			compressionConfig := cfg.GetCompressionConfig()
			if compressionConfig.Enabled {
				r.Use(middleware.Compress(compressionConfig.Level, compressionConfig.ContentTypes...))
			}

			// Add test route
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(http.StatusOK)
				// Large enough content to trigger compression
				content := strings.Repeat("This is test content for compression testing. ", 20)
				if _, err := w.Write([]byte(content)); err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			})

			// Make test request
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Accept-Encoding", tc.acceptEnc)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Check response
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			contentEncoding := w.Header().Get("Content-Encoding")
			
			if tc.expectComp {
				if contentEncoding == "" {
					t.Errorf("%s: Expected compression but got no Content-Encoding header", tc.description)
				} else if contentEncoding != "gzip" && contentEncoding != "deflate" {
					t.Errorf("%s: Expected gzip or deflate compression, got %q", tc.description, contentEncoding)
				}
				t.Logf("%s: Successfully compressed with %s (%d bytes)", tc.description, contentEncoding, w.Body.Len())
			} else {
				if contentEncoding != "" {
					t.Errorf("%s: Expected no compression but got Content-Encoding: %q", tc.description, contentEncoding)
				}
				t.Logf("%s: Correctly not compressed (%d bytes)", tc.description, w.Body.Len())
			}
		})
	}
}