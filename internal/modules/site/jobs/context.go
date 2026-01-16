package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for site job execution
type JobContext struct {
	DB                *gorm.DB
	Logger            *zerolog.Logger
	WS                broadcast.TeamBroadcaster
	Dispatcher        taskrunner.TaskDispatcher
	Queue             *queue.Client
	SiteRepo          *repositories.SiteRepository
	CommandRepo       *repositories.CommandRepository
	DeploymentRepo    *repositories.DeploymentRepository
	CertificateRepo   *repositories.CertificateRepository
	QueueRepo         *repositories.QueueRepository
	RedirectRepo      *repositories.RedirectRepository
	ReleaseRepo       *repositories.ReleaseRepository
	ServerRepo        *serverrepos.Repository
	SourceControlRepo *gitrepos.SourceControlRepository
	ProviderFactory   *gitproviders.ProviderFactory
	TaskRunnerDeps    *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new site job context
func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
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
	sourceControlRepo *gitrepos.SourceControlRepository,
	providerFactory *gitproviders.ProviderFactory,
) *JobContext {
	return &JobContext{
		DB:                db,
		Logger:            logger,
		WS:                ws,
		Dispatcher:        dispatcher,
		Queue:             queueClient,
		SiteRepo:          siteRepo,
		CommandRepo:       commandRepo,
		DeploymentRepo:    deploymentRepo,
		CertificateRepo:   certificateRepo,
		QueueRepo:         queueRepo,
		RedirectRepo:      redirectRepo,
		ReleaseRepo:       releaseRepo,
		ServerRepo:        serverRepo,
		SourceControlRepo: sourceControlRepo,
		ProviderFactory:   providerFactory,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:         db,
			Queue:      queueClient,
			Dispatcher: dispatcher,
			Logger:     logger,
		},
	}
}

// ForServer creates a task runner bound to a specific server.
func (c *JobContext) ForServer(server *servermodels.Server) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, nil)
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server
func (c *JobContext) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastToTeam sends a websocket event to a team channel.
func (c *JobContext) BroadcastToTeam(teamID, event string, data any) {
	if c.WS != nil {
		c.WS.BroadcastToTeam(teamID, event, data)
	}
}

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (c *JobContext) BroadcastServerEvent(server *servermodels.Server, event string, data any) {
	if c.WS != nil && server != nil {
		c.WS.BroadcastToTeam(server.TeamID, event, data)
	}
}

// LogInfo logs an info message with optional fields.
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

// LogError logs an error message with optional fields.
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
