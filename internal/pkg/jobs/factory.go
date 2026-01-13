package jobs

import (
	"github.com/hibiken/asynq"
)

// ContextSettable is an interface for jobs that need context injection.
type ContextSettable interface {
	SetContext(ctx any)
}

// JobConstructor is a function that creates a new job instance with a payload.
type JobConstructor[J any, P any] func(payload P) J

// MakeFactory creates a job factory function that:
// 1. Parses the payload from the asynq task
// 2. Creates a new job instance with the payload
// 3. Injects the module context if the job implements ContextSettable
//
// This eliminates the boilerplate factory functions like:
//
//	func newInstallCronJob(t *asynq.Task) (jobs.Handler, error) {
//	    payload, err := jobs.ParsePayload[InstallCronPayload](t)
//	    if err != nil { return nil, err }
//	    job := &InstallCronJob{Payload: payload}
//	    job.SetContext(jobContext)
//	    return job, nil
//	}
//
// Usage:
//
//	jobs.MakeFactory(func(p InstallCronPayload) *InstallCronJob {
//	    return &InstallCronJob{Payload: p}
//	}, getContext)
func MakeFactory[J Handler, P any](
	constructor JobConstructor[J, P],
	getContext func() any,
) func(*asynq.Task) (Handler, error) {
	return func(t *asynq.Task) (Handler, error) {
		payload, err := ParsePayload[P](t)
		if err != nil {
			return nil, err
		}

		job := constructor(payload)

		if setter, ok := any(job).(ContextSettable); ok && getContext != nil {
			setter.SetContext(getContext())
		}

		return job, nil
	}
}

// MakeSimpleFactory creates a factory for jobs without context injection.
func MakeSimpleFactory[J Handler, P any](
	constructor JobConstructor[J, P],
) func(*asynq.Task) (Handler, error) {
	return MakeFactory(constructor, nil)
}

// TypedJob is an interface for jobs that embed their type constant.
// This allows automatic Type() implementation.
type TypedJob interface {
	Handler
	JobType() string
}

// BaseTypedJob can be embedded to auto-implement Type() method.
// Usage:
//
//	type InstallCronJob struct {
//	    jobs.BaseTypedJob
//	    Payload InstallCronPayload
//	}
//
//	func (j *InstallCronJob) JobType() string { return TypeInstallCron }
type BaseTypedJob struct {
	BaseJob
}

// Type returns the job type by calling JobType() on the embedding struct.
// Note: This requires the embedding struct to implement JobType().
func (j *BaseTypedJob) Type() string {
	if typed, ok := any(j).(interface{ JobType() string }); ok {
		return typed.JobType()
	}
	return "unknown"
}
