package middleware

import (
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"keystone-gateway/internal/config"
	"keystone-gateway/internal/proxy"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// BuildBaseMiddleware creates the base middleware stack without proxy middleware
// This is used for the main router to handle admin endpoints directly
func BuildBaseMiddleware(logger *slog.Logger, cfg *config.Config) []func(http.Handler) http.Handler {
	var middlewareStack []func(http.Handler) http.Handler
	middlewareStack = append(middlewareStack, middleware.RequestID)
	//middlewareStack = append(middlewareStack, middleware.RealIP)
	middlewareStack = append(middlewareStack, middleware.Logger)
	middlewareStack = append(middlewareStack, middleware.Recoverer)
	middlewareStack = append(middlewareStack, middleware.Timeout(cfg.Server.ReadHeaderTimeout))
	middlewareStack = append(middlewareStack, middleware.CleanPath)
	middlewareStack = append(middlewareStack, cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	// TEMPORARILY DISABLED for memory leak testing
	middlewareStack = append(middlewareStack, middleware.Compress(5, "application/json", "text/html"))
	//middlewareStack = append(middlewareStack, httprate.LimitByIP(1000, time.Minute))
	return middlewareStack
}

// BuildMiddlewareStack creates the full middleware stack including proxy middleware
// This function is kept for backward compatibility if needed elsewhere
func BuildMiddlewareStack(logger *slog.Logger, cfg *config.Config, lb *proxy.LoadBalancer, hc *proxy.HealthChecker) []func(http.Handler) http.Handler {
	middlewareStack := BuildBaseMiddleware(logger, cfg)
	middlewareStack = append(middlewareStack, ProxyMiddleware(lb, hc, logger))
	return middlewareStack
}

func ProxyMiddleware(lb *proxy.LoadBalancer, hc *proxy.HealthChecker, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upstream := lb.SelectUpstream()
			if upstream == nil {
				logger.Warn("no healthy upstreams available")
				http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
				return
			}
			upstream.IncrementConnections()
			defer upstream.DecrementConnections() // Add this line
			upstream.Proxy.ServeHTTP(w, r)
		})
	}
}

// AdminSecurityMiddleware provides configurable security for admin endpoints
func AdminSecurityMiddleware(adminConfig *config.AdminConfig, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If admin endpoints are disabled, deny all access
			if adminConfig != nil && !adminConfig.Enabled {
				logger.Warn("admin endpoint access denied - admin endpoints disabled",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "Admin endpoints are disabled", http.StatusForbidden)
				return
			}

			// If admin config is nil, allow access (default behavior)
			if adminConfig == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get client IP address
			clientIP := getClientIP(r)
			if clientIP == "" {
				logger.Warn("admin endpoint access denied - unable to determine client IP",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "Unable to determine client IP", http.StatusForbidden)
				return
			}

			// Check localhost-only restriction
			if adminConfig.LocalhostOnly {
				if !isLocalhost(clientIP) {
					logger.Warn("admin endpoint access denied - not localhost",
						"path", r.URL.Path,
						"client_ip", clientIP)
					http.Error(w, "Admin endpoints restricted to localhost", http.StatusForbidden)
					return
				}
				// If localhost check passes, allow access
				next.ServeHTTP(w, r)
				return
			}

			// Check IP allowlist if configured
			if len(adminConfig.AllowedIPs) > 0 {
				allowed := false
				for _, allowedIP := range adminConfig.AllowedIPs {
					// Try CIDR match first
					if _, network, err := net.ParseCIDR(allowedIP); err == nil {
						if clientIPParsed := net.ParseIP(clientIP); clientIPParsed != nil {
							if network.Contains(clientIPParsed) {
								allowed = true
								break
							}
						}
					} else {
						// Try exact IP match
						if clientIP == allowedIP {
							allowed = true
							break
						}
					}
				}

				if !allowed {
					logger.Warn("admin endpoint access denied - not in allowed IPs list",
						"path", r.URL.Path,
						"client_ip", clientIP,
						"allowed_ips", adminConfig.AllowedIPs)
					http.Error(w, "Admin endpoints restricted to allowed IPs", http.StatusForbidden)
					return
				}
			}

			// If we reach here, allow access (no restrictions configured or all checks passed)
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the real client IP from request headers and connection info
func getClientIP(r *http.Request) string {
	// Check X-Real-IP header first (set by middleware.RealIP)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header (could contain multiple IPs)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP (original client)
		if ips := strings.Split(forwarded, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}

// isLocalhost checks if the given IP address is localhost
func isLocalhost(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check for IPv4 localhost (127.0.0.1) and IPv6 localhost (::1)
	return parsedIP.IsLoopback()
}
