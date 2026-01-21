package module

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// Deps holds common dependencies for all modules
type Deps struct {
	DB              *gorm.DB
	Queue           *queue.Client
	WebSocket       broadcast.ModelBroadcaster
	Dispatcher      *taskrunner.Dispatcher
	Logger          *zerolog.Logger
	Config          *config.Config
	MembershipCache *cache.TeamMembershipCache
}

// ServiceDeps returns the common dependencies as a service.Dependencies struct.
// This allows modules to easily create services using the standardized Dependencies type.
func (d Deps) ServiceDeps() service.Dependencies {
	return service.Dependencies{
		DB:          d.DB,
		Logger:      d.Logger,
		Queue:       d.Queue,
		Broadcaster: d.WebSocket,
		Dispatcher:  d.Dispatcher,
	}
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
			DB:              ctx.DB,
			Queue:           ctx.Queue,
			WebSocket:       ctx.WebSocket,
			Dispatcher:      ctx.Dispatcher,
			Logger:          ctx.Logger,
			Config:          ctx.Config,
			MembershipCache: ctx.MembershipCache,
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

// WebSocket returns the websocket broadcaster
func (b *Builder) WebSocket() broadcast.ModelBroadcaster {
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

// ServiceDeps returns the common dependencies as a service.Dependencies struct.
// This allows modules to easily create services using the standardized Dependencies type.
func (b *Builder) ServiceDeps() service.Dependencies {
	return service.Dependencies{
		DB:          b.deps.DB,
		Logger:      b.deps.Logger,
		Queue:       b.deps.Queue,
		Broadcaster: b.deps.WebSocket,
		Dispatcher:  b.deps.Dispatcher,
	}
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
