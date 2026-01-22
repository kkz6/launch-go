package jobs

import "context"

// Handler is the minimal interface for jobs.
// Jobs only need to implement Handle() to work with Register.
type Handler interface {
	Handle(ctx context.Context) error
}

// FailableHandler is for jobs that need cleanup on failure.
// If a job implements this, Failed() will be called when Handle() returns an error.
type FailableHandler interface {
	Handler
	Failed(ctx context.Context, err error)
}
