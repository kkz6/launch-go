package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// JobDeps holds all dependencies for backup jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos       *repositories.Registry
	ServerRepos servercontracts.RepositoryRegistry
}

// NewJobDeps creates a new JobDeps from app dependencies.
func NewJobDeps(appDeps app.Deps, repos *repositories.Registry, serverRepos servercontracts.RepositoryRegistry) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos:       repos,
		ServerRepos: serverRepos,
	}
}

// NewJobDepsWithParams creates JobDeps with explicit parameters (for testing or custom setup).
func NewJobDepsWithParams(
	db *gorm.DB,
	repos *repositories.Registry,
	serverRepos servercontracts.RepositoryRegistry,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	queueClient *queue.Client,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
		},
		Repos:       repos,
		ServerRepos: serverRepos,
	}
}
