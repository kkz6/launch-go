package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// JobDeps holds all dependencies for git jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos           *repositories.Registry
	Service         *services.SourceControlService
	ProviderFactory *providers.ProviderFactory
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(
	appDeps app.Deps,
	repos *repositories.Registry,
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
		},
		Repos:           repos,
		Service:         service,
		ProviderFactory: providerFactory,
	}
}

// NewJobDepsWithParams creates JobDeps with explicit parameters (for testing or custom setup).
func NewJobDepsWithParams(
	db *gorm.DB,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	queueClient *queue.Client,
	repos *repositories.Registry,
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
		},
		Repos:           repos,
		Service:         service,
		ProviderFactory: providerFactory,
	}
}
