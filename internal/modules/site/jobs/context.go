package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for site job execution
type JobContext struct {
	DB              *gorm.DB
	Logger          *zerolog.Logger
	WS              jobs.Broadcaster
	Dispatcher      taskrunner.TaskDispatcher
	Queue           *queue.Client
	SiteRepo        *repositories.SiteRepository
	CommandRepo     *repositories.CommandRepository
	DeploymentRepo  *repositories.DeploymentRepository
	CertificateRepo *repositories.CertificateRepository
	QueueRepo       *repositories.QueueRepository
	RedirectRepo    *repositories.RedirectRepository
	ReleaseRepo     *repositories.ReleaseRepository
	ServerRepo      *serverrepos.Repository
	TaskRunnerDeps  *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new site job context
func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	ws jobs.Broadcaster,
	dispatcher taskrunner.TaskDispatcher,
	queueClient *queue.Client,
	siteRepo *repositories.SiteRepository,
	commandRepo *repositories.CommandRepository,
	deploymentRepo *repositories.DeploymentRepository,
	certificateRepo *repositories.CertificateRepository,
	queueRepo *repositories.QueueRepository,
	redirectRepo *repositories.RedirectRepository,
	releaseRepo *repositories.ReleaseRepository,
	serverRepo *serverrepos.Repository,
) *JobContext {
	return &JobContext{
		DB:              db,
		Logger:          logger,
		WS:              ws,
		Dispatcher:      dispatcher,
		Queue:           queueClient,
		SiteRepo:        siteRepo,
		CommandRepo:     commandRepo,
		DeploymentRepo:  deploymentRepo,
		CertificateRepo: certificateRepo,
		QueueRepo:       queueRepo,
		RedirectRepo:    redirectRepo,
		ReleaseRepo:     releaseRepo,
		ServerRepo:      serverRepo,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:         db,
			Queue:      queueClient,
			Dispatcher: dispatcher,
			Logger:     logger,
		},
	}
}

// SiteJobBase provides common functionality for site jobs
type SiteJobBase struct {
	jobs.BaseJob
	Ctx *JobContext
}

// SetContext implements jobs.ContextSettable for generic factory injection
func (j *SiteJobBase) SetContext(ctx any) {
	if c, ok := ctx.(*JobContext); ok {
		j.Ctx = c
		j.DB = c.DB
		j.Logger = c.Logger
		j.WS = c.WS
	}
}

// GetTaskRunnerDeps returns TaskRunnerDeps for running tasks
func (j *SiteJobBase) GetTaskRunnerDeps() *servertasks.TaskRunnerDeps {
	return j.Ctx.TaskRunnerDeps
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server
func (j *SiteJobBase) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return j.Ctx.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastSiteEvent broadcasts an event for a site
func (j *SiteJobBase) BroadcastSiteEvent(siteID, event string, data any) {
	if j.WS != nil {
		j.WS.BroadcastToSite(siteID, event, data)
	}
}

// BroadcastToServer broadcasts an event to a server channel
func (j *SiteJobBase) BroadcastToServer(serverID, event string, data any) {
	if j.WS != nil {
		j.WS.BroadcastToServer(serverID, event, data)
	}
}
