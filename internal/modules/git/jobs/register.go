package jobs

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context.
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// GetJobContext returns the global job context.
func GetJobContext() *JobContext {
	return jobContext
}

// RegisterHandlers registers all job handlers with the asynq mux.
func RegisterHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterHandler(mux, TypeProcessGitWebhook, jobContext, NewProcessGitWebhookJob)
	pkgjobs.RegisterHandler(mux, TypeSyncInstallationRepos, jobContext, NewSyncInstallationReposJob)
}
