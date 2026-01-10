package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

type Module struct {
	handler *Handler
}

func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, queueClient, ws, logger)
	handler := NewHandler(service)

	return &Module{
		handler: handler,
	}
}

func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	servers := router.Group("/servers", authMiddleware, middleware.TeamScope())

	// Server CRUD
	servers.Get("/", m.handler.List)
	servers.Post("/", m.handler.Create)
	servers.Get("/:id", m.handler.Show)
	servers.Put("/:id", m.handler.Update)
	servers.Delete("/:id", m.handler.Delete)

	// Server actions
	servers.Post("/:id/reboot", m.handler.Reboot)

	// Databases
	servers.Get("/:id/databases", m.handler.ListDatabases)
	servers.Post("/:id/databases", m.handler.CreateDatabase)
}
