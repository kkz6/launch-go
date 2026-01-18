package jobs

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context for script jobs
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// RegisterHandlers registers all script job handlers with the asynq mux
func RegisterHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterHandler(mux, TypeExecuteScript, jobContext, NewExecuteScriptJob)
}
