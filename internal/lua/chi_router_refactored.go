package lua

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// ========================================
// Operation Infrastructure
// ========================================

// Operation represents a registrable operation
type Operation struct {
	Type     string      // "route_add", "middleware_add", "group_create"
	Key      string      // unique identifier
	Metadata interface{} // RouteInfo, MiddlewareInfo, or GroupInfo
	ChiFunc  func() error // actual Chi operation
}

// ========================================
// Validation Functions (Single Purpose Functions)
// ========================================

// validateRouteInput validates route registration input parameters
func validateRouteInput(method, pattern string) error {
	if method == "" {
		return errors.New("method cannot be empty")
	}
	if pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	return nil
}

// validateMiddlewareInput validates middleware registration input parameters
func validateMiddlewareInput(pattern string, middleware func(http.Handler) http.Handler) error {
	if pattern == "" {
		return errors.New("middleware pattern cannot be empty")
	}
	if middleware == nil {
		return errors.New("middleware function cannot be nil")
	}
	return nil
}

// validateGroupInput validates group creation input parameters
func validateGroupInput(pattern string, setupFunc func(chi.Router)) error {
	if pattern == "" {
		return errors.New("group pattern cannot be empty")
	}
	if setupFunc == nil {
		return errors.New("setup function cannot be nil")
	}
	return nil
}

// ========================================
// Context Management (Explicit Error Handling)
// ========================================

// checkContextDone validates context isn't cancelled
func checkContextDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ========================================
// Registration Engine (Core Logic Extraction)
// ========================================

// registerOperation handles the common registration pattern
func (cr *ChiRouter) registerOperation(ctx context.Context, op *Operation) error {
	// 1. Start timing
	start := time.Now()
	defer func() {
		cr.metrics.TrackOperation(op.Type, start, nil, cr.logger)
	}()

	// 2. Check context
	if err := checkContextDone(ctx); err != nil {
		cr.metrics.TrackOperation(op.Type, start, err, cr.logger)
		return err
	}

	// 3. Lock and check for duplicates
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.isDuplicate(op) {
		err := fmt.Errorf("%s already exists: %s", op.Type, op.Key)
		cr.metrics.TrackOperation(op.Type, start, err, cr.logger)
		return err
	}

	// 4. Execute operation-specific logic
	if err := op.ChiFunc(); err != nil {
		operationErr := fmt.Errorf("failed to execute %s operation: %w", op.Type, err)
		cr.metrics.TrackOperation(op.Type, start, operationErr, cr.logger)
		return operationErr
	}

	// 5. Store metadata
	cr.storeMetadata(op)

	// 6. Log success
	cr.logger.Info(fmt.Sprintf("%s registered successfully", op.Type),
		"key", op.Key)

	return nil
}

// ========================================
// Operation Builders (Factory Pattern)
// ========================================

// buildRouteOperation creates a route operation
func (cr *ChiRouter) buildRouteOperation(method, pattern string, handler http.HandlerFunc, tenantName, scriptTag string) (*Operation, error) {
	if err := validateRouteInput(method, pattern); err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, errors.New("handler cannot be nil")
	}

	routeKey := cr.generateRouteKey(method, pattern)

	return &Operation{
		Type: "route_add",
		Key:  routeKey,
		Metadata: &RouteInfo{
			Method:     method,
			Pattern:    pattern,
			Handler:    handler,
			TenantName: tenantName,
			ScriptTag:  scriptTag,
			Registered: time.Now(),
		},
		ChiFunc: func() error {
			return cr.executeChiRoute(method, pattern, handler)
		},
	}, nil
}

// buildMiddlewareOperation creates a middleware operation
func (cr *ChiRouter) buildMiddlewareOperation(pattern string, middleware func(http.Handler) http.Handler, tenantName, scriptTag string) (*Operation, error) {
	if err := validateMiddlewareInput(pattern, middleware); err != nil {
		return nil, err
	}

	return &Operation{
		Type: "middleware_add",
		Key:  pattern,
		Metadata: &MiddlewareInfo{
			Pattern:    pattern,
			Middleware: middleware,
			TenantName: tenantName,
			ScriptTag:  scriptTag,
			Registered: time.Now(),
		},
		ChiFunc: func() error {
			cr.router.Use(middleware)
			return nil
		},
	}, nil
}

// buildGroupOperation creates a group operation
func (cr *ChiRouter) buildGroupOperation(pattern string, setupFunc func(chi.Router), tenantName, scriptTag string) (*Operation, error) {
	if err := validateGroupInput(pattern, setupFunc); err != nil {
		return nil, err
	}

	return &Operation{
		Type: "group_create",
		Key:  pattern,
		Metadata: &GroupInfo{
			Pattern:    pattern,
			TenantName: tenantName,
			SetupFunc:  setupFunc,
			Registered: time.Now(),
		},
		ChiFunc: func() error {
			var groupRouter chi.Router
			cr.router.Route(pattern, func(r chi.Router) {
				groupRouter = r
				setupFunc(r)
			})
			// Store router reference in metadata after creation
			if groupInfo, ok := cr.groups[pattern]; ok {
				groupInfo.Router = groupRouter
			}
			return nil
		},
	}, nil
}

// ========================================
// Helper Functions (Single Responsibility)
// ========================================

// isDuplicate checks if operation already exists
func (cr *ChiRouter) isDuplicate(op *Operation) bool {
	switch op.Type {
	case "route_add":
		_, exists := cr.routes[op.Key]
		return exists
	case "middleware_add":
		_, exists := cr.middlewares[op.Key]
		return exists
	case "group_create":
		_, exists := cr.groups[op.Key]
		return exists
	}
	return false
}

// storeMetadata stores operation metadata in appropriate map
func (cr *ChiRouter) storeMetadata(op *Operation) {
	switch op.Type {
	case "route_add":
		cr.routes[op.Key] = op.Metadata.(*RouteInfo)
	case "middleware_add":
		cr.middlewares[op.Key] = op.Metadata.(*MiddlewareInfo)
	case "group_create":
		cr.groups[op.Key] = op.Metadata.(*GroupInfo)
	}
}

// executeChiRoute performs the method-specific route registration
func (cr *ChiRouter) executeChiRoute(method, pattern string, handler http.HandlerFunc) error {
	switch method {
	case http.MethodGet:
		cr.router.Get(pattern, handler)
	case http.MethodPost:
		cr.router.Post(pattern, handler)
	case http.MethodPut:
		cr.router.Put(pattern, handler)
	case http.MethodPatch:
		cr.router.Patch(pattern, handler)
	case http.MethodDelete:
		cr.router.Delete(pattern, handler)
	case http.MethodHead:
		cr.router.Head(pattern, handler)
	case http.MethodOptions:
		cr.router.Options(pattern, handler)
	default:
		cr.router.Method(method, pattern, handler)
	}
	return nil
}