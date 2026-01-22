package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// JobFactory creates a job from raw payload bytes.
type JobFactory func(payload []byte) (Handler, error)

// Register registers a job handler with the asynq mux.
// The factory function receives raw payload bytes and returns a Handler.
func Register(mux *asynq.ServeMux, jobType string, factory JobFactory) {
	mux.HandleFunc(jobType, func(ctx context.Context, t *asynq.Task) error {
		job, err := factory(t.Payload())
		if err != nil {
			return fmt.Errorf("create job: %w", err)
		}

		if err := job.Handle(ctx); err != nil {
			if failable, ok := job.(FailableHandler); ok {
				failable.Failed(ctx, err)
			}
			return err
		}

		return nil
	})
}

// RegisterTyped registers a job handler with automatic JSON unmarshaling.
// This is the preferred way to register jobs.
//
// Example:
//
//	jobs.RegisterTyped(mux, TypeRebootServer, NewRebootServerJob)
func RegisterTyped[P any](mux *asynq.ServeMux, jobType string, newJob func(P) Handler) {
	Register(mux, jobType, func(payload []byte) (Handler, error) {
		p, err := unmarshalJSON[P](payload)
		if err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
		return newJob(p), nil
	})
}

func unmarshalJSON[T any](data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, err
	}
	return v, nil
}
