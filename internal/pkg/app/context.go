package app

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Context holds all shared application dependencies.
// This is passed to module factories when creating modules.
// It embeds Deps so all dependency fields and methods are available.
type Context struct {
	Deps
}

// NewContext creates a new application context with all dependencies
func NewContext(
	cfg *config.Config,
	db *gorm.DB,
	logger *zerolog.Logger,
	queueClient *queue.Client,
	wsBroadcaster broadcast.ModelBroadcaster,
	dispatcher *taskrunner.Dispatcher,
	membershipCache *launchcache.TeamMembershipCache,
) *Context {
	return &Context{
		Deps: Deps{
			Config:          cfg,
			DB:              db,
			Logger:          logger,
			Queue:           queueClient,
			WebSocket:       wsBroadcaster,
			Dispatcher:      dispatcher,
			MembershipCache: membershipCache,
		},
	}
}
