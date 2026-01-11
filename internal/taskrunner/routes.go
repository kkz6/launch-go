package taskrunner

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/taskrunner/handlers"
	"github.com/kkz6/launch-go/internal/taskrunner/repositories"
	"github.com/kkz6/launch-go/internal/taskrunner/services"
)

// RegisterRoutes registers all taskrunner webhook routes
func RegisterRoutes(app fiber.Router, db *gorm.DB, secretKey string) *handlers.WebhookHandler {
	repo := repositories.NewTaskRepository(db)
	service := services.NewTaskService(repo)
	handler := handlers.NewWebhookHandler(service, secretKey)

	webhooks := app.Group("/webhooks/tasks")
	webhooks.Post("/:id/finished", handler.MarkAsFinished)
	webhooks.Post("/:id/failed", handler.MarkAsFailed)
	webhooks.Post("/:id/timeout", handler.MarkAsTimeout)

	return handler
}
