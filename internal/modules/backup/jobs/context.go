package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext provides shared dependencies for all backup jobs.
// It embeds pkgjobs.Base for common logging functionality.
type JobContext struct {
	pkgjobs.Base
	// Public fields for backward compatibility with existing jobs
	DB          *gorm.DB
	Repos       *repositories.Registry
	ServerRepos servercontracts.RepositoryRegistry
	Logger      *zerolog.Logger
	Queue       *queue.Client
}

// NewJobContext creates a new job context
func NewJobContext(
	db *gorm.DB,
	repos *repositories.Registry,
	serverRepos servercontracts.RepositoryRegistry,
	logger *zerolog.Logger,
	queueClient *queue.Client,
) *JobContext {
	return &JobContext{
		Base: pkgjobs.NewBase(pkgjobs.BaseDeps{
			DB:     db,
			Logger: logger,
			Queue:  queueClient,
		}),
		// Public fields for backward compatibility
		DB:          db,
		Repos:       repos,
		ServerRepos: serverRepos,
		Logger:      logger,
		Queue:       queueClient,
	}
}
