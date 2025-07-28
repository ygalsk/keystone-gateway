package fixtures

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"
)

// E2EClient represents an HTTP client configured for E2E testing
type E2EClient struct {
	Client  *http.Client
	BaseURL string
	Headers map[string]string
}

// NewE2EClient creates a new E2E HTTP client with sensible defaults
func NewE2EClient() *E2EClient {
	return &E2EClient{
		Client: &http.Client{
			Timeout: 30 * time.Second,
			// Don't follow redirects automatically in E2E tests
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Headers: make(map[string]string),
	}
}

// NewE2EClientWithTimeout creates an E2E client with custom timeout
func NewE2EClientWithTimeout(timeout time.Duration) *E2EClient {
	client := NewE2EClient()
	client.Client.Timeout = timeout
	return client
}

// SetBaseURL sets the base URL for all requests
func (c *E2EClient) SetBaseURL(baseURL string) {
	c.BaseURL = strings.TrimRight(baseURL, "/")
}

// SetHeader sets a default header for all requests
func (c *E2EClient) SetHeader(key, value string) {
	c.Headers[key] = value
}

// SetHeaders sets multiple default headers
func (c *E2EClient) SetHeaders(headers map[string]string) {
	for key, value := range headers {
		c.Headers[key] = value
	}
}

// buildURL constructs the full URL for a request
func (c *E2EClient) buildURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	
	if c.BaseURL == "" {
		return path
	}
	
	return c.BaseURL + "/" + strings.TrimLeft(path, "/")
}

// addDefaultHeaders adds default headers to a request
func (c *E2EClient) addDefaultHeaders(req *http.Request) {
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}
}

// Get performs a GET request
func (c *E2EClient) Get(path string) (*http.Response, error) {
	return c.GetWithHeaders(path, nil)
}

// GetWithHeaders performs a GET request with additional headers
func (c *E2EClient) GetWithHeaders(path string, headers map[string]string) (*http.Response, error) {
	url := c.buildURL(path)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	
	c.addDefaultHeaders(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	return c.Client.Do(req)
}

// Post performs a POST request with JSON body
func (c *E2EClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.PostWithHeaders(path, body, nil)
}

// PostWithHeaders performs a POST request with JSON body and additional headers
func (c *E2EClient) PostWithHeaders(path string, body interface{}, headers map[string]string) (*http.Response, error) {
	url := c.buildURL(path)
	
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	
	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	
	c.addDefaultHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	return c.Client.Do(req)
}

// PostRaw performs a POST request with raw body
func (c *E2EClient) PostRaw(path string, body []byte, contentType string) (*http.Response, error) {
	url := c.buildURL(path)
	
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	
	c.addDefaultHeaders(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	
	return c.Client.Do(req)
}

// Put performs a PUT request with JSON body
func (c *E2EClient) Put(path string, body interface{}) (*http.Response, error) {
	return c.PutWithHeaders(path, body, nil)
}

// PutWithHeaders performs a PUT request with JSON body and additional headers
func (c *E2EClient) PutWithHeaders(path string, body interface{}, headers map[string]string) (*http.Response, error) {
	url := c.buildURL(path)
	
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	
	req, err := http.NewRequest("PUT", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	
	c.addDefaultHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	return c.Client.Do(req)
}

// Delete performs a DELETE request
func (c *E2EClient) Delete(path string) (*http.Response, error) {
	return c.DeleteWithHeaders(path, nil)
}

// DeleteWithHeaders performs a DELETE request with additional headers
func (c *E2EClient) DeleteWithHeaders(path string, headers map[string]string) (*http.Response, error) {
	url := c.buildURL(path)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}
	
	c.addDefaultHeaders(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	return c.Client.Do(req)
}

// DoRequest performs a custom HTTP request
func (c *E2EClient) DoRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.buildURL(path)
	
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", method, err)
	}
	
	c.addDefaultHeaders(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	return c.Client.Do(req)
}

// E2EResponse represents a response from an E2E request with helper methods
type E2EResponse struct {
	*http.Response
	BodyBytes []byte
	BodyString string
}

// GetResponse performs a request and returns an E2EResponse with body already read
func (c *E2EClient) GetResponse(path string) (*E2EResponse, error) {
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	
	return NewE2EResponse(resp)
}

// PostResponse performs a POST request and returns an E2EResponse
func (c *E2EClient) PostResponse(path string, body interface{}) (*E2EResponse, error) {
	resp, err := c.Post(path, body)
	if err != nil {
		return nil, err
	}
	
	return NewE2EResponse(resp)
}

// NewE2EResponse creates an E2EResponse from an http.Response
func NewE2EResponse(resp *http.Response) (*E2EResponse, error) {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()
	
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	return &E2EResponse{
		Response:   resp,
		BodyBytes:  bodyBytes,
		BodyString: string(bodyBytes),
	}, nil
}

// JSON unmarshals the response body as JSON
func (r *E2EResponse) JSON(v interface{}) error {
	return json.Unmarshal(r.BodyBytes, v)
}

// HasStatus checks if the response has the expected status code
func (r *E2EResponse) HasStatus(expected int) bool {
	return r.StatusCode == expected
}

// HasHeader checks if the response has a header with the expected value
func (r *E2EResponse) HasHeader(key, expected string) bool {
	actual := r.Header.Get(key)
	return actual == expected
}

// ContainsInBody checks if the response body contains the expected string
func (r *E2EResponse) ContainsInBody(expected string) bool {
	return strings.Contains(r.BodyString, expected)
}

// E2ETestSuite provides utilities for running E2E test suites
type E2ETestSuite struct {
	t          *testing.T
	client     *E2EClient
	gatewayURL string
}

// NewE2ETestSuite creates a new E2E test suite
func NewE2ETestSuite(t *testing.T, gatewayURL string) *E2ETestSuite {
	client := NewE2EClient()
	client.SetBaseURL(gatewayURL)
	
	return &E2ETestSuite{
		t:          t,
		client:     client,
		gatewayURL: gatewayURL,
	}
}

// Client returns the HTTP client for this test suite
func (s *E2ETestSuite) Client() *E2EClient {
	return s.client
}

// AssertResponse performs common response assertions
func (s *E2ETestSuite) AssertResponse(resp *E2EResponse, expectedStatus int, expectedBodyContains string) {
	if !resp.HasStatus(expectedStatus) {
		s.t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, resp.StatusCode, resp.BodyString)
	}
	
	if expectedBodyContains != "" && !resp.ContainsInBody(expectedBodyContains) {
		s.t.Errorf("Expected body to contain '%s', got: %s", expectedBodyContains, resp.BodyString)
	}
}

// AssertJSON performs JSON response assertions
func (s *E2ETestSuite) AssertJSON(resp *E2EResponse, expectedStatus int, jsonAssertions func(data map[string]interface{})) {
	if !resp.HasStatus(expectedStatus) {
		s.t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, resp.StatusCode, resp.BodyString)
		return
	}
	
	var data map[string]interface{}
	if err := resp.JSON(&data); err != nil {
		s.t.Errorf("Failed to parse JSON response: %v. Body: %s", err, resp.BodyString)
		return
	}
	
	if jsonAssertions != nil {
		jsonAssertions(data)
	}
}

// GetGatewayURL returns the gateway URL for this test suite
func (s *E2ETestSuite) GetGatewayURL() string {
	return s.gatewayURL
}

// RequestWithHost creates a request with a specific Host header for multi-tenant testing
func (c *E2EClient) RequestWithHost(method, path, host string, body io.Reader) (*http.Response, error) {
	url := c.buildURL(path)
	
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", method, err)
	}
	
	c.addDefaultHeaders(req)
	req.Host = host
	
	return c.Client.Do(req)
}

// GetWithHost performs a GET request with a specific Host header
func (c *E2EClient) GetWithHost(path, host string) (*http.Response, error) {
	return c.RequestWithHost("GET", path, host, nil)
}

// PostWithHost performs a POST request with a specific Host header
func (c *E2EClient) PostWithHost(path, host string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	
	url := c.buildURL(path)
	
	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	
	c.addDefaultHeaders(req)
	req.Host = host
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return c.Client.Do(req)
}

// ParallelRequests executes multiple requests in parallel and returns all responses
func (c *E2EClient) ParallelRequests(requests []func() (*http.Response, error)) ([]*http.Response, []error) {
	type result struct {
		index int
		resp  *http.Response
		err   error
	}
	
	results := make(chan result, len(requests))
	
	// Execute requests in parallel
	for i, request := range requests {
		go func(index int, req func() (*http.Response, error)) {
			resp, err := req()
			results <- result{index: index, resp: resp, err: err}
		}(i, request)
	}
	
	// Collect results
	responses := make([]*http.Response, len(requests))
	errors := make([]error, len(requests))
	
	for i := 0; i < len(requests); i++ {
		result := <-results
		responses[result.index] = result.resp
		errors[result.index] = result.err
	}
	
	return responses, errors
}

// LoadTest performs a simple load test by making concurrent requests
func (c *E2EClient) LoadTest(path string, concurrency int, duration time.Duration) *LoadTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	results := make(chan *http.Response, concurrency*10)
	errors := make(chan error, concurrency*10)
	done := make(chan struct{})
	
	// Start workers
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				done <- struct{}{}
			}()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					resp, err := c.Get(path)
					if err != nil {
						select {
						case errors <- err:
						case <-ctx.Done():
							return
						}
					} else {
						select {
						case results <- resp:
							if err := resp.Body.Close(); err != nil {
					log.Printf("Failed to close response body: %v", err)
				}
						case <-ctx.Done():
							if err := resp.Body.Close(); err != nil {
					log.Printf("Failed to close response body: %v", err)
				}
							return
						}
					}
				}
			}
		}()
	}
	
	// Wait for timeout
	<-ctx.Done()
	
	// Wait for all workers to finish
	for i := 0; i < concurrency; i++ {
		<-done
	}
	
	// Close channels after all workers are done
	close(results)
	close(errors)
	
	// Count results
	var statusCodes = make(map[int]int)
	var totalRequests int
	var totalErrors int
	
	for resp := range results {
		totalRequests++
		statusCodes[resp.StatusCode]++
	}
	
	for range errors {
		totalErrors++
	}
	
	return &LoadTestResult{
		TotalRequests: totalRequests,
		TotalErrors:   totalErrors,
		StatusCodes:   statusCodes,
		Duration:      duration,
		Concurrency:   concurrency,
	}
}

// LoadTestResult represents the results of a load test
type LoadTestResult struct {
	TotalRequests int
	TotalErrors   int
	StatusCodes   map[int]int
	Duration      time.Duration
	Concurrency   int
}

// RequestsPerSecond calculates the requests per second for the load test
func (r *LoadTestResult) RequestsPerSecond() float64 {
	if r.Duration.Seconds() == 0 {
		return 0
	}
	return float64(r.TotalRequests) / r.Duration.Seconds()
}

// SuccessRate calculates the success rate (2xx status codes) for the load test
func (r *LoadTestResult) SuccessRate() float64 {
	if r.TotalRequests == 0 {
		return 0
	}
	
	successCount := 0
	for status, count := range r.StatusCodes {
		if status >= 200 && status < 300 {
			successCount += count
		}
	}
	
	return float64(successCount) / float64(r.TotalRequests)
}