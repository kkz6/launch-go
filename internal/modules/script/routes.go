package script

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/script/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers all script routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	service := m.createService()
	handler := handlers.NewScriptHandler(service)

	// Script routes (authenticated + team scoped + subscription required)
	scripts := router.Group("/scripts", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		scripts.Get("/", handler.List)
		scripts.Post("/", middleware.Can("script.create"), fiberutil.Validate(handler.Create))
		scripts.Get("/:id", handler.Show)
		scripts.Put("/:id", middleware.Can("script.update"), fiberutil.Validate(handler.Update))
		scripts.Delete("/:id", middleware.Can("script.delete"), handler.Delete)

		// Execution
		scripts.Post("/:id/execute", middleware.Can("script.execute"), fiberutil.Validate(handler.Execute))
		scripts.Get("/:id/executions", handler.ListExecutions)
	}

	// Execution routes (for fetching individual executions)
	executions := router.Group("/script-executions", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		executions.Get("/:id", handler.GetExecution)
	}
}
