package modules

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	transport "keystone-gateway/internal/http"
)

// HTTPClient provides a simplified HTTP client for Lua scripts.
type HTTPClient struct {
	client *http.Client
}

// HTTPOptions defines options for an HTTP request made from Lua.
// gopher-luar automatically converts Lua tables to this struct.
type HTTPOptions struct {
	Headers         map[string]string
	FollowRedirects *bool // nil = default (true), false = explicit no-redirect, true = explicit redirect
	Timeout         time.Duration
}

// HTTPResponse is the simplified response returned to Lua.
type HTTPResponse struct {
	Body    string
	Status  int
	Headers map[string]string
}

// NewHTTPClient creates a new HTTPClient.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Transport: transport.CreateTransport(),
			Timeout:   10 * time.Second, // Default timeout
		},
	}
}

// Get performs an HTTP GET request.
// gopher-luar automatically converts Lua tables to HTTPOptions.
func (c *HTTPClient) Get(url string, options HTTPOptions) (*HTTPResponse, error) {
	return c.doRequest("GET", url, nil, options)
}

// Post performs an HTTP POST request.
// gopher-luar automatically converts Lua tables to HTTPOptions.
func (c *HTTPClient) Post(url string, body string, options HTTPOptions) (*HTTPResponse, error) {
	return c.doRequest("POST", url, []byte(body), options)
}

// Put performs an HTTP PUT request.
// gopher-luar automatically converts Lua tables to HTTPOptions.
func (c *HTTPClient) Put(url string, body string, options HTTPOptions) (*HTTPResponse, error) {
	return c.doRequest("PUT", url, []byte(body), options)
}

// Delete performs an HTTP DELETE request.
// gopher-luar automatically converts Lua tables to HTTPOptions.
func (c *HTTPClient) Delete(url string, options HTTPOptions) (*HTTPResponse, error) {
	return c.doRequest("DELETE", url, nil, options)
}

func (c *HTTPClient) doRequest(method, url string, body []byte, opts HTTPOptions) (*HTTPResponse, error) {
	// Apply defaults for zero values
	if opts.Timeout == 0 {
		opts.Timeout = c.client.Timeout
	}
	if opts.Headers == nil {
		opts.Headers = make(map[string]string)
	}
	// Default to following redirects if not explicitly set
	followRedirects := true
	if opts.FollowRedirects != nil {
		followRedirects = *opts.FollowRedirects
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	client := c.client
	if !followRedirects {
		client = &http.Client{
			Transport: c.client.Transport,
			Timeout:   opts.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	respHeaders := make(map[string]string)
	for name, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[name] = values[0]
		}
	}

	return &HTTPResponse{
		Body:    string(respBody),
		Status:  resp.StatusCode,
		Headers: respHeaders,
	}, nil
}
