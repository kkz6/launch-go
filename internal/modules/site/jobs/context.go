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
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for site job execution.
// It embeds pkgjobs.Base for common logging functionality.
type JobContext struct {
	pkgjobs.Base
	// Public fields for backward compatibility with existing jobs
	DB                *gorm.DB
	Logger            *zerolog.Logger
	WS                broadcast.TeamBroadcaster
	Queue             *queue.Client
	SiteRepo          *repositories.SiteRepository
	CommandRepo       *repositories.CommandRepository
	DeploymentRepo    *repositories.DeploymentRepository
	CertificateRepo   *repositories.CertificateRepository
	QueueRepo         *repositories.QueueRepository
	RedirectRepo      *repositories.RedirectRepository
	ServerRepos       *serverrepos.Registry
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
	serverRepos *serverrepos.Registry,
	sourceControlRepo *gitrepos.SourceControlRepository,
	providerFactory *gitproviders.ProviderFactory,
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
		DB:                db,
		Logger:            logger,
		WS:                ws,
		Queue:             queueClient,
		SiteRepo:          siteRepo,
		CommandRepo:       commandRepo,
		DeploymentRepo:    deploymentRepo,
		CertificateRepo:   certificateRepo,
		QueueRepo:         queueRepo,
		RedirectRepo:      redirectRepo,
		ServerRepos:       serverRepos,
		SourceControlRepo: sourceControlRepo,
		ProviderFactory:   providerFactory,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
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

// BroadcastServerEvent broadcasts an event for a server to its team channel.
func (c *JobContext) BroadcastServerEvent(server *servermodels.Server, event string, data any) {
	if c.WS != nil && server != nil {
		c.WS.BroadcastToTeam(server.TeamID, event, data)
	}
}
