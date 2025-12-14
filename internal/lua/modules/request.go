package modules

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

// Request is a wrapper around http.Request to provide a simplified API for Lua.
// It is designed to be used with gopher-luar to expose its fields and methods to Lua.
type Request struct {
	// Exported fields are accessible as Lua properties (e.g., req.Method)
	Method string
	URL    string
	Path   string
	Host   string

	// Internal fields (private, lowercase) - not accessible from Lua
	req         *http.Request
	body        []byte
	bodyErr     error
	bodyOnce    sync.Once
	maxBodySize int64
}

// NewRequest creates a new Request wrapper.
func NewRequest(r *http.Request, maxBodySize int64) *Request {
	return &Request{
		// Initialize exported fields for Lua property access
		Method: r.Method,
		URL:    r.URL.String(),
		Path:   r.URL.Path,
		Host:   r.Host,

		// Internal state
		req:         r,
		maxBodySize: maxBodySize,
	}
}

// --- Methods (operations exposed to Lua) ---

// Header returns the value of a request header.
func (r *Request) Header(key string) string {
	return r.req.Header.Get(key)
}

// Headers returns all request headers as a map.
func (r *Request) Headers() map[string][]string {
	return r.req.Header
}

// Query returns the value of a URL query parameter.
func (r *Request) Query(key string) string {
	return r.req.URL.Query().Get(key)
}

// Param returns the value of a URL path parameter from Chi's context.
func (r *Request) Param(key string) string {
	return chi.URLParam(r.req, key)
}

// Body reads and returns the request body as a string.
// It caches the body so subsequent calls are fast and don't re-read.
// It also enforces the maximum body size limit configured in the gateway.
func (r *Request) Body() (string, error) {
	r.bodyOnce.Do(func() {
		if r.req.Body == nil {
			r.body = []byte{}
			return
		}
		defer r.req.Body.Close()

		// Use a LimitedReader to enforce the max body size.
		lr := http.MaxBytesReader(nil, r.req.Body, r.maxBodySize)
		body, err := io.ReadAll(lr)
		if err != nil {
			r.bodyErr = err
			return
		}
		r.body = body

		// Restore the body so other handlers can read it if needed.
		r.req.Body = io.NopCloser(strings.NewReader(string(r.body)))
	})

	if r.bodyErr != nil {
		return "", r.bodyErr
	}
	return string(r.body), nil
}

// ContextSet sets a value in the request's context.
// This is useful for passing data between middleware and handlers.
func (r *Request) ContextSet(key string, value interface{}) {
	ctx := context.WithValue(r.req.Context(), key, value)
	r.req = r.req.WithContext(ctx)
}

// ContextGet retrieves a value from the request's context.
func (r *Request) ContextGet(key string) interface{} {
	return r.req.Context().Value(key)
}
