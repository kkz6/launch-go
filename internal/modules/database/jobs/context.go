package jobs

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

var jobContext *JobContext

type JobContext struct {
	DB             *gorm.DB
	Repo           *repositories.Repository
	Logger         *zerolog.Logger
	WS             jobs.Broadcaster
	Dispatcher     taskrunner.TaskDispatcher
	Queue          *queue.Client
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

func NewJobContext(
	db *gorm.DB,
	repo *repositories.Repository,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher taskrunner.TaskDispatcher,
	queueClient *queue.Client,
) *JobContext {
	return &JobContext{
		DB:         db,
		Repo:       repo,
		Logger:     logger,
		WS:         ws,
		Dispatcher: dispatcher,
		Queue:      queueClient,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:         db,
			Queue:      queueClient,
			Dispatcher: dispatcher,
			Logger:     logger,
		},
	}
}

func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

func GetJobContext() *JobContext {
	return jobContext
}

type DatabaseJobBase struct {
	jobs.BaseJob
	Ctx *JobContext
}

// SetContext implements jobs.ContextSettable for generic factory injection.
func (j *DatabaseJobBase) SetContext(ctx any) {
	if c, ok := ctx.(*JobContext); ok {
		j.Ctx = c
		j.DB = c.DB
		j.Logger = c.Logger
		j.WS = c.WS
		j.Dispatcher = c.Dispatcher
		j.Queue = c.Queue
	}
}

func (j *DatabaseJobBase) Repo() *repositories.Repository {
	return j.Ctx.Repo
}

func (j *DatabaseJobBase) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return j.Ctx.TaskRunnerDeps.NewRunner(server, task)
}

func (j *DatabaseJobBase) BroadcastDatabaseEvent(server *servermodels.Server, event string, data any) {
	if j.WS != nil && server != nil {
		j.WS.BroadcastToTeam(server.TeamID, event, data)
	}
}

func (j *DatabaseJobBase) GetDatabaseType(ctx context.Context, serverID string) string {
	service, err := repository.NewQuery[servermodels.InstalledService](j.DB, ctx).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(serverenums.ServiceTypeMySql),
			string(serverenums.ServiceTypePostgreSql),
		}).
		First()

	if err != nil {
		return "mysql"
	}

	if service.Type == serverenums.ServiceTypePostgreSql {
		return "postgresql"
	}
	return "mysql"
}

func (j *DatabaseJobBase) GetDatabaseServiceType(ctx context.Context, serverID string) serverenums.ServiceType {
	service, err := repository.NewQuery[servermodels.InstalledService](j.DB, ctx).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(serverenums.ServiceTypeMySql),
			string(serverenums.ServiceTypePostgreSql),
		}).
		First()

	if err != nil {
		return serverenums.ServiceTypeMySql
	}

	return service.Type
}

func (j *DatabaseJobBase) GetTaskFactory(ctx context.Context, serverID string) *tasks.Factory {
	dbType := j.GetDatabaseServiceType(ctx, serverID)
	return tasks.NewFactory(dbType)
}

// BroadcastDatabaseProgress broadcasts a standard progress event for database operations.
func (j *DatabaseJobBase) BroadcastDatabaseProgress(server *servermodels.Server, event, entityID, status, message string) {
	data := map[string]any{
		"server_id": server.ID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if entityID != "" {
		data["database_id"] = entityID
	}
	j.BroadcastDatabaseEvent(server, event, data)
}

// BroadcastUserProgress broadcasts a standard progress event for database user operations.
func (j *DatabaseJobBase) BroadcastUserProgress(server *servermodels.Server, event, userID, status, message string) {
	j.BroadcastDatabaseEvent(server, event, map[string]any{
		"server_id": server.ID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
