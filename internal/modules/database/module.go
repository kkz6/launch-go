package database

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/handlers"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Module represents the database module
type Module struct {
	handler *handlers.Handler
}

// NewModule creates a new database module
func NewModule(db *gorm.DB, serverRepo services.ServerRepository, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, serverRepo, queueClient, ws, logger)
	handler := handlers.NewHandler(service)

	return &Module{
		handler: handler,
	}
}
