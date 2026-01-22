package app

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Deps holds common dependencies for all modules
type Deps struct {
	DB              *gorm.DB
	Queue           *queue.Client
	WebSocket       broadcast.ModelBroadcaster
	Dispatcher      *taskrunner.Dispatcher
	Logger          *zerolog.Logger
	Config          *config.Config
	MembershipCache *launchcache.TeamMembershipCache
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

// AppKey returns the app secret key (useful for webhooks)
func (d Deps) AppKey() string {
	if d.Config != nil {
		return d.Config.App.Key
	}
	return ""
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
func NewBuilderFromContext(ctx *Context) *Builder {
	return &Builder{deps: ctx.Deps}
}

// Deps returns the common dependencies
func (b *Builder) Deps() Deps {
	return b.deps
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
