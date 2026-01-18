package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for script job execution
type JobContext struct {
	DB             *gorm.DB
	Logger         *zerolog.Logger
	WS             broadcast.TeamBroadcaster
	Dispatcher     taskrunner.TaskDispatcher
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
		DB:          db,
		Logger:      logger,
		WS:          ws,
		Dispatcher:  dispatcher,
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

// BroadcastToTeam sends a websocket event to a team channel
func (c *JobContext) BroadcastToTeam(teamID, event string, data any) {
	if c.WS != nil {
		c.WS.BroadcastToTeam(teamID, event, data)
	}
}

// LogInfo logs an info message with optional fields
func (c *JobContext) LogInfo(msg string, fields ...any) {
	if c.Logger == nil {
		return
	}

	event := c.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error message with optional fields
func (c *JobContext) LogError(err error, msg string, fields ...any) {
	if c.Logger == nil {
		return
	}

	event := c.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
