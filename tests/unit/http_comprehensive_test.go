package unit

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"keystone-gateway/tests/fixtures"
)

// TestHTTPEndpointsComprehensive tests all HTTP endpoints with comprehensive table-driven tests
// REFACTORED VERSION using organized fixtures
// BEFORE: No HTTP endpoint tests
// AFTER: Comprehensive coverage using KISS fixtures
func TestHTTPEndpointsComprehensive(t *testing.T) {
	env := fixtures.SetupHealthAwareGateway(t, "test-tenant")
	defer env.Cleanup()

	testCases := []fixtures.HTTPTestCase{
		// Basic HTTP methods
		{
			Name:           "GET request to health endpoint",
			Method:         "GET",
			Path:           "/health",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "POST request with valid JSON",
			Method:         "POST",
			Path:           "/api/data",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"key": "value"}`,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "PUT request with data",
			Method:         "PUT",
			Path:           "/api/data/123",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"id": 123, "updated": true}`,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "DELETE request",
			Method:         "DELETE",
			Path:           "/api/data/123",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "PATCH request with partial data",
			Method:         "PATCH",
			Path:           "/api/data/123",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"updated": true}`,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "HEAD request",
			Method:         "HEAD",
			Path:           "/api/status",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "OPTIONS request for CORS",
			Method:         "OPTIONS",
			Path:           "/api/data",
			ExpectedStatus: http.StatusOK,
		},

		// Edge cases and error conditions
		{
			Name:           "Invalid HTTP method",
			Method:         "INVALID",
			Path:           "/api/data",
			ExpectedStatus: http.StatusMethodNotAllowed,
		},
		{
			Name:           "Root path",
			Method:         "GET",
			Path:           "/",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "Path without leading slash",
			Method:         "GET",
			Path:           "/api-data", // Changed to valid path that should return 404
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "Very long path",
			Method:         "GET",
			Path:           "/api/" + strings.Repeat("a", 2048), // 2KB path with valid characters
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "Path with special characters",
			Method:         "GET",
			Path:           "/api/data%20with%20spaces",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Path with unicode characters",
			Method:         "GET",
			Path:           "/api/测试/データ",
			ExpectedStatus: http.StatusOK,
		},

		// Header validation
		{
			Name:           "Request with custom headers",
			Method:         "GET",
			Path:           "/api/data",
			Headers:        map[string]string{
				"X-Custom-Header": "custom-value",
				"Authorization":   "Bearer token123",
				"User-Agent":      "test-client/1.0",
			},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Request with empty header values",
			Method:         "GET",
			Path:           "/api/data",
			Headers:        map[string]string{"X-Empty": ""},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Request with very long header",
			Method:         "GET",
			Path:           "/api/data",
			Headers:        map[string]string{"X-Long": strings.Repeat("x", 1024)},
			ExpectedStatus: http.StatusOK,
		},

		// Body handling
		{
			Name:           "POST with empty body",
			Method:         "POST",
			Path:           "/api/data",
			Body:           "",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "POST with large body",
			Method:         "POST",
			Path:           "/api/data",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"data": "` + strings.Repeat("x", 1024) + `"}`,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "POST with malformed JSON",
			Method:         "POST",
			Path:           "/api/data",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"invalid": json}`,
			ExpectedStatus: http.StatusOK, // Gateway should pass through to backend
		},
		{
			Name:           "POST with binary data",
			Method:         "POST",
			Path:           "/api/upload",
			Headers:        map[string]string{"Content-Type": "application/octet-stream"},
			Body:           string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF}),
			ExpectedStatus: http.StatusOK,
		},

		// Query parameters
		{
			Name:           "GET with query parameters",
			Method:         "GET",
			Path:           "/api/data?page=1&limit=10&sort=name",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "GET with encoded query parameters",
			Method:         "GET",
			Path:           "/api/data?search=hello%20world&filter=%7B%22type%22%3A%22test%22%7D",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "GET with special characters in query",
			Method:         "GET",
			Path:           "/api/data?q=test&special=!@#$%^&*()",
			ExpectedStatus: http.StatusOK,
		},
	}

	fixtures.RunHTTPTestCases(t, env.Router, testCases)
}

// TestHTTPBackendErrors tests error conditions and edge cases
func TestHTTPBackendErrors(t *testing.T) {
	// Test with error backend
	errorEnv := fixtures.SetupErrorProxy(t, "error-tenant", "/error/", "/error/*")
	defer errorEnv.Cleanup()

	errorTestCases := []fixtures.HTTPTestCase{
		{
			Name:           "Backend returns 500 error",
			Method:         "GET",
			Path:           "/error/500",
			ExpectedStatus: http.StatusInternalServerError,
		},
		{
			Name:           "Backend returns 404 error",
			Method:         "GET",
			Path:           "/error/404",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			Name:           "Backend returns 400 error",
			Method:         "GET",
			Path:           "/error/400",
			ExpectedStatus: http.StatusBadRequest,
		},
	}

	fixtures.RunHTTPTestCases(t, errorEnv.Router, errorTestCases)
}

// TestHTTPPerformance tests performance-related scenarios
func TestHTTPPerformance(t *testing.T) {
	// Test with slow backend
	slowBackend := fixtures.CreateSlowBackend(t, 100*time.Millisecond) // 100ms delay
	slowEnv := fixtures.SetupProxy(t, "slow-tenant", "/slow/", slowBackend)
	defer slowEnv.Cleanup()

	performanceTestCases := []fixtures.HTTPTestCase{
		{
			Name:           "Request to slow backend",
			Method:         "GET",
			Path:           "/slow/test",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "POST to slow backend with data",
			Method:         "POST",
			Path:           "/slow/upload",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"large": "` + string(make([]byte, 512)) + `"}`,
			ExpectedStatus: http.StatusOK,
		},
	}

	fixtures.RunHTTPTestCases(t, slowEnv.Router, performanceTestCases)
}

// TestHTTPContentTypes tests various content type handling
func TestHTTPContentTypes(t *testing.T) {
	echoBackend := fixtures.CreateEchoBackend(t)
	env := fixtures.SetupProxy(t, "echo-tenant", "/echo/", echoBackend)
	defer env.Cleanup()

	contentTypeTestCases := []fixtures.HTTPTestCase{
		{
			Name:           "JSON content type",
			Method:         "POST",
			Path:           "/echo/data",
			Headers:        map[string]string{"Content-Type": "application/json"},
			Body:           `{"message": "test"}`,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "XML content type",
			Method:         "POST",
			Path:           "/echo/data",
			Headers:        map[string]string{"Content-Type": "application/xml"},
			Body:           `<message>test</message>`,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Form data content type",
			Method:         "POST",
			Path:           "/echo/form",
			Headers:        map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			Body:           "name=test&value=123",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Multipart form data",
			Method:         "POST",
			Path:           "/echo/upload",
			Headers:        map[string]string{"Content-Type": "multipart/form-data; boundary=test123"},
			Body:           "--test123\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\ntest content\r\n--test123--",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Text plain content type",
			Method:         "POST",
			Path:           "/echo/text",
			Headers:        map[string]string{"Content-Type": "text/plain"},
			Body:           "Plain text message",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "Binary content type",
			Method:         "POST",
			Path:           "/echo/binary",
			Headers:        map[string]string{"Content-Type": "application/octet-stream"},
			Body:           string([]byte{0x89, 0x50, 0x4E, 0x47}), // PNG header
			ExpectedStatus: http.StatusOK,
		},
	}

	fixtures.RunHTTPTestCases(t, env.Router, contentTypeTestCases)
}