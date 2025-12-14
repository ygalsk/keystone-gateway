package modules

import (
	"net/http"
)

// Response is a wrapper around http.ResponseWriter to provide a simplified API for Lua.
// It is designed to be used with gopher-luar.
type Response struct {
	W http.ResponseWriter
}

// NewResponse creates a new Response wrapper.
func NewResponse(w http.ResponseWriter) *Response {
	return &Response{W: w}
}

// Status sets the HTTP response status code.
func (r *Response) Status(code int) {
	r.W.WriteHeader(code)
}

// Header sets a response header.
func (r *Response) Header(key, value string) {
	r.W.Header().Set(key, value)
}

// Write writes a string to the response body.
func (r *Response) Write(body string) (int, error) {
	return r.W.Write([]byte(body))
}
