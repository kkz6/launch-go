package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// JobDeps holds all dependencies for git jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Service         *services.SourceControlService
	ProviderFactory *providers.ProviderFactory
	AppURL          string
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(
	appDeps app.Deps,
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
) *JobDeps {
	appURL := ""
	if appDeps.Config != nil {
		appURL = appDeps.Config.App.URL
	}
	return &JobDeps{
		AppURL: appURL,
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
		},
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
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
) *JobDeps {
	appURL := ""
	if service != nil {
		appURL = service.ServiceDeps().AppURL
	}
	return &JobDeps{
		AppURL: appURL,
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
		},
		Service:         service,
		ProviderFactory: providerFactory,
	}
}
