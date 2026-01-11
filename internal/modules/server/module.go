package server

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/handlers"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/taskrunner"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the server module
type Module struct {
	handler *handlers.Handler
}

// NewModule creates a new server module instance
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, queueClient, ws, dispatcher, logger)
	handler := handlers.NewHandler(service)

	return &Module{
		handler: handler,
	}
}
