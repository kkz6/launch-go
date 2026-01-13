package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

type JobContext struct {
	DB              *gorm.DB
	Repo            contracts.Repository
	Logger          *zerolog.Logger
	WS              jobs.Broadcaster
	Dispatcher      taskrunner.TaskDispatcher
	ProviderFactory *providers.Factory
	TaskRunnerDeps  *tasks.TaskRunnerDeps
}

func NewJobContext(
	db *gorm.DB,
	repo contracts.Repository,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher *taskrunner.Dispatcher,
	providerFactory *providers.Factory,
	queueClient *queue.Client,
) *JobContext {
	return &JobContext{
		DB:              db,
		Repo:            repo,
		Logger:          logger,
		WS:              ws,
		Dispatcher:      dispatcher,
		ProviderFactory: providerFactory,
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
			DB:         db,
			Queue:      queueClient,
			Dispatcher: dispatcher,
			Logger:     logger,
		},
	}
}

type ServerTaskRunner struct {
	ctx    *JobContext
	server *models.Server
}

func (c *JobContext) ForServer(server *models.Server) *ServerTaskRunner {
	return &ServerTaskRunner{
		ctx:    c,
		server: server,
	}
}

func (s *ServerTaskRunner) RunTask(task taskrunner.Task) *tasks.TaskRunner {
	return s.ctx.TaskRunnerDeps.NewRunner(s.server, task)
}

type ServerJobBase struct {
	jobs.BaseJob
	Ctx *JobContext
}

// SetContext implements jobs.ContextSettable for generic factory injection.
func (j *ServerJobBase) SetContext(ctx any) {
	if c, ok := ctx.(*JobContext); ok {
		j.Ctx = c
		j.DB = c.DB
		j.Logger = c.Logger
		j.WS = c.WS
	}
}

func (j *ServerJobBase) RunTaskOnServer(server *models.Server, task taskrunner.Task) *tasks.TaskRunner {
	return j.Ctx.TaskRunnerDeps.NewRunner(server, task)
}

func (j *ServerJobBase) Repo() contracts.Repository {
	return j.Ctx.Repo
}

func (j *ServerJobBase) ProviderFactory() *providers.Factory {
	return j.Ctx.ProviderFactory
}

func (j *ServerJobBase) BroadcastServerEvent(serverID, event string, data any) {
	if j.WS != nil {
		j.WS.BroadcastToServer(serverID, event, data)
	}
}
