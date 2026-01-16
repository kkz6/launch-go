package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext provides shared dependencies for all backup jobs
type JobContext struct {
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
		DB:          db,
		Repos:       repos,
		ServerRepos: serverRepos,
		Logger:      logger,
		Queue:       queueClient,
	}
}

// LogInfo logs an info message
func (c *JobContext) LogInfo(msg string, fields ...interface{}) {
	event := c.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error message
func (c *JobContext) LogError(err error, msg string, fields ...interface{}) {
	event := c.Logger.Error()
	if err != nil {
		event = event.Err(err)
	}
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
