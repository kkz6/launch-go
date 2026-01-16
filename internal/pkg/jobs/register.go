package jobs

import (
	"context"

	"github.com/hibiken/asynq"
)

// Handleable is the minimal interface for jobs.
// Jobs only need to implement Handle() to work with RegisterHandler.
type Handleable interface {
	Handle(ctx context.Context) error
}

// Failable is an optional interface for jobs that want failure notification.
// If a job implements this, Failed() will be called when Handle() returns an error.
type Failable interface {
	Failed(ctx context.Context, err error)
}

// RegisterHandler registers a job handler with the asynq mux using generics.
//
// Type parameters:
//   - C: The context type (module-specific JobContext)
//   - P: The payload type
//   - J: The job type (must implement Handleable, optionally Failable)
//
// The newJob function receives the context and parsed payload, and returns the job instance.
// If the job implements Failable, Failed() will be called when Handle() returns an error.
//
// Example usage:
//
//	func RegisterHandlers(mux *asynq.ServeMux) {
//	    pkgjobs.RegisterHandler(mux, TypeRebootServer, jobContext, NewRebootServerJob)
//	    pkgjobs.RegisterHandler(mux, TypeInstallCron, jobContext, NewInstallCronJob)
//	}
func RegisterHandler[C any, P any, J Handleable](mux *asynq.ServeMux, jobType string, ctx *C, newJob func(*C, P) J) {
	mux.HandleFunc(jobType, func(c context.Context, t *asynq.Task) error {
		payload, err := UnmarshalPayload[P](t)
		if err != nil {
			return err
		}

		job := newJob(ctx, payload)

		if err := job.Handle(c); err != nil {
			if failable, ok := any(job).(Failable); ok {
				failable.Failed(c, err)
			}
			return err
		}

		return nil
	})
}
