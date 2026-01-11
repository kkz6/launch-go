package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/handlers"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the server module
type Module struct {
	handler        *handlers.Handler
	webhookHandler *handlers.TaskWebhookHandler
	repo           *repositories.Repository
}

// NewModule creates a new server module instance
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger, webhookSecretKey string) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, queueClient, ws, dispatcher, logger)
	handler := handlers.NewHandler(service)
	webhookHandler := handlers.NewTaskWebhookHandler(repo, webhookSecretKey)

	return &Module{
		handler:        handler,
		webhookHandler: webhookHandler,
		repo:           repo,
	}
}

// RegisterWebhookRoutes registers webhook routes (no auth required)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	m.webhookHandler.RegisterRoutes(router)
}

// GetWebhookHandler returns the webhook handler for generating callback URLs
func (m *Module) GetWebhookHandler() *handlers.TaskWebhookHandler {
	return m.webhookHandler
}
