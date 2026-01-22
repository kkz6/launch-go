package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all git job handlers.
func Register(
	mux *asynq.ServeMux,
	appDeps app.Deps,
	repos *repositories.Registry,
	service *services.SourceControlService,
	providerFactory *providers.ProviderFactory,
) {
	deps = NewJobDeps(appDeps, repos, service, providerFactory)
	registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterTyped(mux, TypeProcessGitWebhook, NewProcessGitWebhookJob)
	pkgjobs.RegisterTyped(mux, TypeSyncInstallationRepos, NewSyncInstallationReposJob)
}
