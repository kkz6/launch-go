package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

// Kernel is the application kernel that manages all modules.
// Similar to NestJS's module system - you register modules and
// the kernel handles booting them all.
//
// Usage:
//
//	kernel := app.NewKernel(logger)
//	kernel.Register(authModule)
//	kernel.Register(serverModule)
//	kernel.Register(databaseModule)
//
//	// Boot HTTP routes
//	kernel.BootHTTP(api, authMiddleware)
//
//	// Boot background jobs
//	kernel.BootJobs(mux)
type Kernel struct {
	modules []Module
	logger  *zerolog.Logger
}

// NewKernel creates a new application kernel
func NewKernel(logger *zerolog.Logger) *Kernel {
	return &Kernel{
		modules: make([]Module, 0),
		logger:  logger,
	}
}

// Register adds a module to the kernel.
// Modules are booted in the order they are registered.
func (k *Kernel) Register(module Module) *Kernel {
	k.modules = append(k.modules, module)
	if k.logger != nil {
		k.logger.Debug().Str("module", module.Name()).Msg("Module registered")
	}
	return k
}

// BootHTTP registers all HTTP routes from all modules.
// Call this once after all modules are registered.
// The teamContextMiddleware is optional and can be used by modules that require team context.
func (k *Kernel) BootHTTP(router fiber.Router, authMiddleware fiber.Handler, teamContextMiddleware ...fiber.Handler) {
	// First, register public routes (no auth required)
	for _, module := range k.modules {
		if registrar, ok := module.(PublicRouteRegistrar); ok {
			registrar.RegisterPublicRoutes(router)
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("Public routes registered")
			}
		}
	}

	// Then, register authenticated routes
	for _, module := range k.modules {
		if registrar, ok := module.(RouteRegistrar); ok {
			registrar.RegisterRoutes(router, authMiddleware)
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("Routes registered")
			}
		}

		// Register team-scoped routes if the module supports it
		if len(teamContextMiddleware) > 0 {
			if registrar, ok := module.(TeamScopedRouteRegistrar); ok {
				registrar.RegisterTeamScopedRoutes(router, authMiddleware, teamContextMiddleware[0])
				if k.logger != nil {
					k.logger.Debug().Str("module", module.Name()).Msg("Team-scoped routes registered")
				}
			}
		}
	}
}

// BootWebhooks registers all webhook routes from all modules.
// Webhook routes are registered at the root level (not under /api).
func (k *Kernel) BootWebhooks(router fiber.Router) {
	for _, module := range k.modules {
		if registrar, ok := module.(WebhookRegistrar); ok {
			registrar.RegisterWebhookRoutes(router)
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("Webhook routes registered")
			}
		}
	}
}

// BootWebSocket registers all WebSocket routes from all modules.
// WebSocket routes are registered at the root level (not under /api).
func (k *Kernel) BootWebSocket(router fiber.Router) {
	for _, module := range k.modules {
		if registrar, ok := module.(WebSocketRegistrar); ok {
			registrar.RegisterWebSocketRoutes(router)
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("WebSocket routes registered")
			}
		}
	}
}

// BootJobs registers all background job handlers from all modules.
// Call this when setting up the worker.
func (k *Kernel) BootJobs(mux *asynq.ServeMux) {
	for _, module := range k.modules {
		if registrar, ok := module.(JobRegistrar); ok {
			registrar.RegisterJobs(mux)
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("Jobs registered")
			}
		}
	}
}

// BootTaskCallbacks registers all task callback handlers from all modules.
// Task callbacks handle completion events for SSH tasks.
// Call this when setting up the worker or API server.
func (k *Kernel) BootTaskCallbacks() {
	for _, module := range k.modules {
		if registrar, ok := module.(TaskCallbackRegistrar); ok {
			registrar.RegisterTaskCallbacks()
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("Task callbacks registered")
			}
		}
	}
}

// Boot calls Boot() on all bootable modules.
// Call this after all routes and jobs are registered.
func (k *Kernel) Boot() error {
	for _, module := range k.modules {
		if bootable, ok := module.(Bootable); ok {
			if err := bootable.Boot(); err != nil {
				return err
			}
			if k.logger != nil {
				k.logger.Debug().Str("module", module.Name()).Msg("Module booted")
			}
		}
	}
	return nil
}

// Shutdown calls Shutdown() on all shutdownable modules.
// Call this when the application is shutting down.
func (k *Kernel) Shutdown() error {
	// Shutdown in reverse order
	for i := len(k.modules) - 1; i >= 0; i-- {
		module := k.modules[i]
		if shutdownable, ok := module.(Shutdownable); ok {
			if err := shutdownable.Shutdown(); err != nil {
				if k.logger != nil {
					k.logger.Error().Err(err).Str("module", module.Name()).Msg("Module shutdown error")
				}
			}
		}
	}
	return nil
}

// GetModule returns a registered module by name.
// Returns nil if not found.
func (k *Kernel) GetModule(name string) Module {
	for _, module := range k.modules {
		if module.Name() == name {
			return module
		}
	}
	return nil
}

// Modules returns all registered modules
func (k *Kernel) Modules() []Module {
	return k.modules
}
