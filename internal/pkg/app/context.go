package app

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// Context holds all shared application dependencies.
// This is passed to module factories when creating modules.
// Similar to NestJS's dependency injection container.
type Context struct {
	Config          *config.Config
	DB              *gorm.DB
	Logger          *zerolog.Logger
	Queue           *queue.Client
	WebSocket       broadcast.ModelBroadcaster
	Dispatcher      *taskrunner.Dispatcher
	MembershipCache *cache.TeamMembershipCache
}

// NewContext creates a new application context with all dependencies
func NewContext(
	cfg *config.Config,
	db *gorm.DB,
	logger *zerolog.Logger,
	queueClient *queue.Client,
	wsBroadcaster broadcast.ModelBroadcaster,
	dispatcher *taskrunner.Dispatcher,
	membershipCache *cache.TeamMembershipCache,
) *Context {
	return &Context{
		Config:          cfg,
		DB:              db,
		Logger:          logger,
		Queue:           queueClient,
		WebSocket:       wsBroadcaster,
		Dispatcher:      dispatcher,
		MembershipCache: membershipCache,
	}
}
