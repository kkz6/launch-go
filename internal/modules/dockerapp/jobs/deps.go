package jobs

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/contracts"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	pkgtaskrunner "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// JobDeps holds dependencies for dockerapp jobs.
type JobDeps struct {
	*pkgjobs.Deps
	Repos          contracts.RepositoryRegistry
	Registry       contracts.RegistryCredentialReader
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobDeps wires JobDeps from app dependencies.
func NewJobDeps(appDeps app.Deps, repos contracts.RepositoryRegistry, registry contracts.RegistryCredentialReader) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos:    repos,
		Registry: registry,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          appDeps.DB,
			Queue:       appDeps.Queue,
			Dispatcher:  appDeps.Dispatcher,
			Logger:      appDeps.Logger,
			Broadcaster: appDeps.WebSocket,
			Notifier:    appDeps.Notifier,
		},
	}
}

// NewJobDepsWithParams creates JobDeps with explicit parameters (tests).
func NewJobDepsWithParams(
	db *gorm.DB,
	repos contracts.RepositoryRegistry,
	registry contracts.RegistryCredentialReader,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher pkgtaskrunner.TaskDispatcher,
	queueClient *queue.Client,
) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      logger,
			Queue:       queueClient,
			Broadcaster: ws,
			Dispatcher:  dispatcher,
		},
		Repos:    repos,
		Registry: registry,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
		},
	}
}

// RunTask creates a task runner for a server with a task.
func (d *JobDeps) RunTask(server *servermodels.Server, task pkgtaskrunner.Task) *servertasks.TaskRunner {
	return d.TaskRunnerDeps.NewRunner(server, task)
}

// GetServer returns a server by ID.
func (d *JobDeps) GetServer(ctx context.Context, serverID string) (*servermodels.Server, error) {
	return repository.Find[servermodels.Server](ctx, d.DB, serverID)
}

// BroadcastAppEvent broadcasts an app.progress event on the team channel.
func (d *JobDeps) BroadcastAppEvent(server *servermodels.Server, event, appID, status, message string) {
	if server == nil {
		return
	}
	data := map[string]any{
		"team_id":   server.TeamID,
		"server_id": server.ID,
		"app_id":    appID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	d.BroadcastToTeam(server.TeamID, event, data)
}
