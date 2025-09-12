package http

import (
	"net/http"
	"time"
)

// CreateTransport creates a shared HTTP transport with connection pooling
func CreateTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
	}
}