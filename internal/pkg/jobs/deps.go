// Package jobs provides infrastructure for async job handling with asynq.
package jobs

import (
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Deps holds all dependencies a job might need.
// Modules create their own JobDeps struct that embeds this with extras.
//
// Example:
//
//	type JobDeps struct {
//	    *jobs.Deps
//	    Repos           contracts.RepositoryRegistry
//	    ProviderFactory *providers.Factory
//	}
type Deps struct {
	DB          *gorm.DB
	Logger      *zerolog.Logger
	Queue       *queue.Client
	Broadcaster broadcast.TeamBroadcaster
	Dispatcher  taskrunner.TaskDispatcher // nil if module doesn't need SSH
}

// Dispatch creates and enqueues a job with the given type and payload.
// Returns nil if the queue is nil (allows graceful degradation in tests).
func (d *Deps) Dispatch(jobType string, payload any, opts ...asynq.Option) error {
	if d.Queue == nil {
		return nil
	}

	task, err := Task(jobType, payload, opts...)
	if err != nil {
		d.Logger.Error().Err(err).Str("job_type", jobType).Msg("failed to create job task")
		return err
	}

	info, err := d.Queue.Enqueue(task)
	if err != nil {
		d.Logger.Error().Err(err).Str("job_type", jobType).Msg("failed to enqueue job")
		return err
	}

	d.Logger.Debug().
		Str("job_type", jobType).
		Str("task_id", info.ID).
		Str("queue", info.Queue).
		Msg("job dispatched")

	return nil
}

// DispatchWithID creates and enqueues a job with a specific task ID for deduplication.
func (d *Deps) DispatchWithID(jobType string, payload any, taskID string, opts ...asynq.Option) error {
	opts = append(opts, asynq.TaskID(taskID))
	return d.Dispatch(jobType, payload, opts...)
}

// DispatchIn creates and enqueues a job to be processed after a delay.
func (d *Deps) DispatchIn(jobType string, payload any, delay time.Duration, opts ...asynq.Option) error {
	opts = append(opts, asynq.ProcessIn(delay))
	return d.Dispatch(jobType, payload, opts...)
}

// DispatchToQueue creates and enqueues a job to a specific queue.
func (d *Deps) DispatchToQueue(jobType string, payload any, queueName string, opts ...asynq.Option) error {
	opts = append(opts, asynq.Queue(queueName))
	return d.Dispatch(jobType, payload, opts...)
}

// DispatchTask enqueues a pre-built asynq task.
// Returns nil if the queue is nil (allows graceful degradation in tests).
func (d *Deps) DispatchTask(task *asynq.Task, opts ...asynq.Option) error {
	if d.Queue == nil {
		return nil
	}

	info, err := d.Queue.Enqueue(task, opts...)
	if err != nil {
		d.Logger.Error().Err(err).Str("task_type", task.Type()).Msg("failed to enqueue task")
		return err
	}

	d.Logger.Debug().
		Str("task_type", task.Type()).
		Str("task_id", info.ID).
		Str("queue", info.Queue).
		Msg("task dispatched")

	return nil
}

// DispatchTaskIn enqueues a pre-built task to be processed after a delay.
func (d *Deps) DispatchTaskIn(task *asynq.Task, delay time.Duration, opts ...asynq.Option) error {
	opts = append(opts, asynq.ProcessIn(delay))
	return d.DispatchTask(task, opts...)
}

// BroadcastToTeam broadcasts an event to a team channel.
// Safe to call with nil broadcaster or empty teamID.
func (d *Deps) BroadcastToTeam(teamID, event string, data any) {
	if d.Broadcaster != nil && teamID != "" {
		d.Broadcaster.BroadcastToTeam(teamID, event, data)
	}
}
