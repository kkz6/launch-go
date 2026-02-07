package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

// Module is the interface that all application modules must implement.
// Similar to NestJS modules - each module is self-contained and registers
// its own routes, jobs, and other components.
//
// Usage:
//
//	type ServerModule struct { ... }
//
//	func (m *ServerModule) Name() string { return "server" }
//	func (m *ServerModule) RegisterRoutes(router fiber.Router, auth fiber.Handler) { ... }
//	func (m *ServerModule) RegisterJobs(mux *asynq.ServeMux) { ... }
type Module interface {
	// Name returns the module's unique identifier
	Name() string
}

// RouteRegistrar is implemented by modules that have HTTP routes
type RouteRegistrar interface {
	Module
	// RegisterRoutes registers the module's authenticated routes
	RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler)
}

// PublicRouteRegistrar is implemented by modules with public routes (no auth)
type PublicRouteRegistrar interface {
	Module
	// RegisterPublicRoutes registers routes that don't require authentication
	RegisterPublicRoutes(router fiber.Router)
}

// TeamScopedRouteRegistrar is implemented by modules with team-scoped routes
// These routes require both authentication AND a valid X-Team-ID header
type TeamScopedRouteRegistrar interface {
	Module
	// RegisterTeamScopedRoutes registers routes that require team context
	RegisterTeamScopedRoutes(router fiber.Router, authMiddleware, teamContextMiddleware fiber.Handler)
}

// WebhookRegistrar is implemented by modules that have webhook endpoints
type WebhookRegistrar interface {
	Module
	// RegisterWebhookRoutes registers webhook routes (no auth, external callbacks)
	RegisterWebhookRoutes(router fiber.Router)
}

// WebSocketRegistrar is implemented by modules that have WebSocket endpoints
type WebSocketRegistrar interface {
	Module
	// RegisterWebSocketRoutes registers WebSocket routes
	RegisterWebSocketRoutes(router fiber.Router)
}

// JobRegistrar is implemented by modules that have background jobs
type JobRegistrar interface {
	Module
	// RegisterJobs registers the module's job handlers with the queue
	RegisterJobs(mux *asynq.ServeMux)
}

// TaskCallbackRegistrar is implemented by modules that have task callbacks
// Task callbacks handle completion events (finished/failed/timeout) for
// long-running server tasks executed via SSH.
type TaskCallbackRegistrar interface {
	Module
	// RegisterTaskCallbacks registers the module's task callback handlers
	RegisterTaskCallbacks()
}

// Bootable is implemented by modules that need initialization after registration
type Bootable interface {
	Module
	// Boot is called after all modules are registered
	Boot() error
}

// Shutdownable is implemented by modules that need cleanup on shutdown
type Shutdownable interface {
	Module
	// Shutdown is called when the application is shutting down
	Shutdown() error
}
