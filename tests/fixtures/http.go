package fixtures

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// HTTPTestResult represents the result of an HTTP test
type HTTPTestResult struct {
	StatusCode int
	Body       string
	Headers    http.Header
}

// ExecuteHTTPTest executes a simple HTTP test
func ExecuteHTTPTest(router *chi.Mux, method, path string) *HTTPTestResult {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return &HTTPTestResult{
		StatusCode: w.Code,
		Body:       w.Body.String(),
		Headers:    w.Header(),
	}
}

// ExecuteHTTPTestWithRequest executes an HTTP test with a custom request
func ExecuteHTTPTestWithRequest(router *chi.Mux, req *http.Request) *HTTPTestResult {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return &HTTPTestResult{
		StatusCode: w.Code,
		Body:       w.Body.String(),
		Headers:    w.Header(),
	}
}

// ExecuteHTTPTestWithHeaders executes an HTTP test with custom headers
func ExecuteHTTPTestWithHeaders(router *chi.Mux, method, path string, headers map[string]string) *HTTPTestResult {
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return ExecuteHTTPTestWithRequest(router, req)
}

// HTTPTestCase represents a table-driven test case for HTTP testing
type HTTPTestCase struct {
	Name           string
	Method         string
	Path           string
	Headers        map[string]string
	Body           string
	ExpectedStatus int
	ExpectedBody   string
	CheckHeaders   map[string]string // headers that must be present
}

// RunHTTPTestCases runs a set of HTTP test cases
func RunHTTPTestCases(t *testing.T, router *chi.Mux, testCases []HTTPTestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(tc.Method, tc.Path, nil)
			
			// Set headers if provided
			for key, value := range tc.Headers {
				if key == "Host" {
					// Set the Host field directly for proper host-based routing
					req.Host = value
				} else {
					req.Header.Set(key, value)
				}
			}
			
			result := ExecuteHTTPTestWithRequest(router, req)
			
			// Assert status code
			if result.StatusCode != tc.ExpectedStatus {
				t.Errorf("expected status %d, got %d", tc.ExpectedStatus, result.StatusCode)
			}
			
			// Assert body if specified
			if tc.ExpectedBody != "" && result.Body != tc.ExpectedBody {
				t.Errorf("expected body %q, got %q", tc.ExpectedBody, result.Body)
			}
			
			// Assert required headers
			for key, expectedValue := range tc.CheckHeaders {
				if actualValue := result.Headers.Get(key); actualValue != expectedValue {
					t.Errorf("expected header %s=%q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// AssertHTTPResponse asserts HTTP response matches expectations
func AssertHTTPResponse(t *testing.T, result *HTTPTestResult, expectedStatus int, expectedBody string) {
	if result.StatusCode != expectedStatus {
		t.Errorf("expected status %d, got %d", expectedStatus, result.StatusCode)
	}
	if expectedBody != "" && result.Body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, result.Body)
	}
}

// AssertHTTPStatusCode asserts the HTTP status code matches expected
func AssertHTTPStatusCode(t *testing.T, result *HTTPTestResult, expectedCode int) {
	if result.StatusCode != expectedCode {
		t.Errorf("expected status %d, got %d", expectedCode, result.StatusCode)
	}
}

// AssertHTTPHeader asserts an HTTP header has the expected value
func AssertHTTPHeader(t *testing.T, result *HTTPTestResult, headerName, expectedValue string) {
	if actualValue := result.Headers.Get(headerName); actualValue != expectedValue {
		t.Errorf("expected header %s=%q, got %q", headerName, expectedValue, actualValue)
	}
}

// AssertNoHTTPHeader asserts an HTTP header is not present
func AssertNoHTTPHeader(t *testing.T, result *HTTPTestResult, headerName string) {
	if actualValue := result.Headers.Get(headerName); actualValue != "" {
		t.Errorf("expected header %s to be absent, got %q", headerName, actualValue)
	}
}