package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
	scRepo *repositories.SourceControlRepository,
) *JobContext {
	return &JobContext{
		DB:              db,
		Logger:          logger,
		Service:         service,
		ProviderFactory: providerFactory,
		SCRepo:          scRepo,
	}
}

// RegisterHandlers registers all job handlers with the asynq mux
func RegisterHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterHandler(mux, TypeProcessGitWebhook, jobContext, NewProcessGitWebhookJob)
	pkgjobs.RegisterHandler(mux, TypeSyncInstallationRepos, jobContext, NewSyncInstallationReposJob)
}
