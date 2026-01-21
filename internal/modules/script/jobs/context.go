package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for script job execution.
// It embeds pkgjobs.Base for common logging functionality.
type JobContext struct {
	pkgjobs.Base
	// Public fields for backward compatibility with existing jobs
	DB             *gorm.DB
	Logger         *zerolog.Logger
	WS             broadcast.TeamBroadcaster
	Queue          *queue.Client
	Repos          *repositories.Registry
	ServerRepos    *serverrepos.Registry
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new script job context
func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher taskrunner.TaskDispatcher,
	queueClient *queue.Client,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
) *JobContext {
	return &JobContext{
		Base: pkgjobs.NewBase(pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		}),
		// Public fields for backward compatibility
		DB:          db,
		Logger:      logger,
		WS:          ws,
		Queue:       queueClient,
		Repos:       repos,
		ServerRepos: serverRepos,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
		},
	}
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server
func (c *JobContext) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, task)
}
