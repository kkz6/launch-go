package jobs

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// jobContext is the shared job context for all git jobs
var jobContext *JobContext

// SetJobContext sets the shared job context for all git jobs
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// NewJobContext creates a new job context with all dependencies
func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
	scRepo *repositories.SourceControlRepository,
	queueClient QueueClient,
) *JobContext {
	return &JobContext{
		DB:              db,
		Logger:          logger,
		Service:         service,
		ProviderFactory: providerFactory,
		SCRepo:          scRepo,
		QueueClient:     queueClient,
	}
}

// Register registers all git jobs with the job registry
func Register(r *pkgjobs.Registry) {
	r.Register(TypeProcessGitWebhook, newProcessGitWebhookJob)
	r.Register(TypeSyncInstallationRepos, newSyncInstallationReposJob)
}

// RegisterHandlers registers all git job handlers with the asynq mux
func RegisterHandlers(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeProcessGitWebhook, handleProcessGitWebhook)
	mux.HandleFunc(TypeSyncInstallationRepos, handleSyncInstallationRepos)
}

// Factory functions

func newProcessGitWebhookJob(t *asynq.Task) (pkgjobs.Handler, error) {
	payload, err := pkgjobs.ParsePayload[ProcessGitWebhookPayload](t)
	if err != nil {
		return nil, err
	}
	return NewProcessGitWebhookJob(jobContext, payload), nil
}

func newSyncInstallationReposJob(t *asynq.Task) (pkgjobs.Handler, error) {
	payload, err := pkgjobs.ParsePayload[SyncInstallationReposPayload](t)
	if err != nil {
		return nil, err
	}
	return NewSyncInstallationReposJob(jobContext, payload), nil
}

// Handler functions for asynq mux

func handleProcessGitWebhook(ctx context.Context, t *asynq.Task) error {
	payload, err := pkgjobs.ParsePayload[ProcessGitWebhookPayload](t)
	if err != nil {
		return err
	}
	job := NewProcessGitWebhookJob(jobContext, payload)
	return job.Handle(ctx)
}

func handleSyncInstallationRepos(ctx context.Context, t *asynq.Task) error {
	payload, err := pkgjobs.ParsePayload[SyncInstallationReposPayload](t)
	if err != nil {
		return err
	}
	job := NewSyncInstallationReposJob(jobContext, payload)
	return job.Handle(ctx)
}
