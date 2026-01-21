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
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// SiteRepos combines site, server, and git repositories for the site module.
type SiteRepos struct {
	Site          *repositories.Registry
	Server        *serverrepos.Registry
	SourceControl *gitrepos.SourceControlRepository
}

// JobContext holds dependencies for site job execution.
// It embeds pkgjobs.ServerContext for common functionality, typed repository access,
// and server task execution capabilities.
//
// Access common dependencies via inherited methods:
//   - ctx.DB() - database connection
//   - ctx.Logger() - zerolog logger
//   - ctx.WS() - websocket broadcaster
//   - ctx.Queue() - queue client
//   - ctx.Repos() - repository registry (*SiteRepos)
//
// Module-specific fields provide direct access to repositories and services.
type JobContext struct {
	*pkgjobs.ServerContext[*SiteRepos]
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

// NewJobContext creates a new site job context.
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
	// Build a site registry from the individual repos for the new interface
	siteRegistry := &repositories.Registry{}

	deps := pkgjobs.ServerContextDeps{
		BaseDeps: pkgjobs.BaseDeps{
			DB:         db,
			Logger:     logger,
			WS:         ws,
			Dispatcher: dispatcher,
			Queue:      queueClient,
		},
	}

	serverCtx := pkgjobs.NewServerContext(deps, &SiteRepos{
		Site:          siteRegistry,
		Server:        serverRepos,
		SourceControl: sourceControlRepo,
	})

	return &JobContext{
		ServerContext:     serverCtx,
		SiteRepo:          siteRepo,
		CommandRepo:       commandRepo,
		DeploymentRepo:    deploymentRepo,
		CertificateRepo:   certificateRepo,
		QueueRepo:         queueRepo,
		RedirectRepo:      redirectRepo,
		ServerRepos:       serverRepos,
		SourceControlRepo: sourceControlRepo,
		ProviderFactory:   providerFactory,
		TaskRunnerDeps:    &servertasks.TaskRunnerDeps{ServerTaskDeps: serverCtx.TaskDeps()},
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
	if server != nil {
		c.ServerContext.BroadcastServerEvent(server.TeamID, event, data)
	}
}
