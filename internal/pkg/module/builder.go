package module

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Deps holds common dependencies for all modules
type Deps struct {
	DB         *gorm.DB
	Queue      *queue.Client
	WebSocket  *websocket.Hub
	Dispatcher *taskrunner.Dispatcher
	Logger     *zerolog.Logger
	Config     *config.Config
}

// Builder helps construct modules with common dependencies
type Builder struct {
	deps Deps
}

// NewBuilder creates a new module builder with the given dependencies
func NewBuilder(deps Deps) *Builder {
	return &Builder{deps: deps}
}

// NewBuilderFromContext creates a builder from app context
func NewBuilderFromContext(ctx *app.Context) *Builder {
	return &Builder{
		deps: Deps{
			DB:         ctx.DB,
			Queue:      ctx.Queue,
			WebSocket:  ctx.WebSocket,
			Dispatcher: ctx.Dispatcher,
			Logger:     ctx.Logger,
			Config:     ctx.Config,
		},
	}
}

// Deps returns the common dependencies
func (b *Builder) Deps() Deps {
	return b.deps
}

// DB returns the database connection
func (b *Builder) DB() *gorm.DB {
	return b.deps.DB
}

// Queue returns the queue client
func (b *Builder) Queue() *queue.Client {
	return b.deps.Queue
}

// WebSocket returns the websocket hub
func (b *Builder) WebSocket() *websocket.Hub {
	return b.deps.WebSocket
}

// Dispatcher returns the task dispatcher
func (b *Builder) Dispatcher() *taskrunner.Dispatcher {
	return b.deps.Dispatcher
}

// Logger returns the logger
func (b *Builder) Logger() *zerolog.Logger {
	return b.deps.Logger
}

// Config returns the app config
func (b *Builder) Config() *config.Config {
	return b.deps.Config
}

// AppKey returns the app secret key (useful for webhooks)
func (b *Builder) AppKey() string {
	if b.deps.Config != nil {
		return b.deps.Config.App.Key
	}
	return ""
}

// Base provides common module functionality that can be embedded
type Base struct {
	name string
	deps Deps
}

// NewBase creates a new base module with name and dependencies
func NewBase(name string, b *Builder) Base {
	return Base{
		name: name,
		deps: b.Deps(),
	}
}

// Name returns the module name (implements app.Module)
func (b *Base) Name() string {
	return b.name
}

// Deps returns the module dependencies
func (b *Base) Deps() Deps {
	return b.deps
}
