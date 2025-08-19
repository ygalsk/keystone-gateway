package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/proxy"
)

// BuildBaseMiddleware creates the base middleware stack without proxy middleware
// This is used for the main router to handle admin endpoints directly
func BuildBaseMiddleware(logger *slog.Logger, cfg *config.Config) []func(http.Handler) http.Handler {
	var middlewareStack []func(http.Handler) http.Handler
	middlewareStack = append(middlewareStack, middleware.RequestID)
	middlewareStack = append(middlewareStack, middleware.RealIP)
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
	middlewareStack = append(middlewareStack, middleware.Compress(5, "application/json", "text/html"))
	middlewareStack = append(middlewareStack, httprate.LimitByIP(1000, time.Minute))
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

// TODO Lua Middleware
