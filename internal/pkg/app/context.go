package app

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Context holds all shared application dependencies.
// This is passed to module factories when creating modules.
// Similar to NestJS's dependency injection container.
type Context struct {
	Config     *config.Config
	DB         *gorm.DB
	Logger     *zerolog.Logger
	Queue      *queue.Client
	WebSocket  *websocket.Hub
	Dispatcher *taskrunner.Dispatcher
}

// NewContext creates a new application context with all dependencies
func NewContext(
	cfg *config.Config,
	db *gorm.DB,
	logger *zerolog.Logger,
	queueClient *queue.Client,
	wsHub *websocket.Hub,
	dispatcher *taskrunner.Dispatcher,
) *Context {
	return &Context{
		Config:     cfg,
		DB:         db,
		Logger:     logger,
		Queue:      queueClient,
		WebSocket:  wsHub,
		Dispatcher: dispatcher,
	}
}
