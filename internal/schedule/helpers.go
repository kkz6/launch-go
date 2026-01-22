// Package schedule provides centralized scheduling for background tasks.
package schedule

import (
	"log"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// TaskFunc is a function that creates an asynq task.
type TaskFunc func() (*asynq.Task, error)

// TaskOptions configures a scheduled task.
type TaskOptions struct {
	Name      string         // Human-readable name for logging
	Queue     string         // Queue name (low, default, critical)
	AsynqOpts []asynq.Option // Additional asynq options
}

// TaskOption is a functional option for configuring scheduled tasks.
type TaskOption func(*TaskOptions)

// WithName sets a human-readable name for the task (used in logs).
func WithName(name string) TaskOption {
	return func(o *TaskOptions) {
		o.Name = name
	}
}

// LowPriority runs the task on the "low" queue.
func LowPriority() TaskOption {
	return func(o *TaskOptions) {
		o.Queue = "low"
		o.AsynqOpts = append(o.AsynqOpts, asynq.Queue("low"))
	}
}

// DefaultPriority runs the task on the "default" queue.
func DefaultPriority() TaskOption {
	return func(o *TaskOptions) {
		o.Queue = "default"
		o.AsynqOpts = append(o.AsynqOpts, asynq.Queue("default"))
	}
}

// CriticalPriority runs the task on the "critical" queue.
func CriticalPriority() TaskOption {
	return func(o *TaskOptions) {
		o.Queue = "critical"
		o.AsynqOpts = append(o.AsynqOpts, asynq.Queue("critical"))
	}
}

// WithMaxRetry sets the maximum number of retries for the task.
func WithMaxRetry(n int) TaskOption {
	return func(o *TaskOptions) {
		o.AsynqOpts = append(o.AsynqOpts, asynq.MaxRetry(n))
	}
}

// scheduled creates a ScheduledTask with the given cron spec and options.
func scheduled(cronSpec string, taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	options := &TaskOptions{
		Name:  "unnamed-task",
		Queue: "default",
	}
	for _, opt := range opts {
		opt(options)
	}

	task, err := taskFn()
	if err != nil {
		log.Printf("[schedule] failed to create task %q: %v", options.Name, err)
		return queue.ScheduledTask{
			Name:     options.Name,
			CronSpec: cronSpec,
			Task:     nil,
			Opts:     options.AsynqOpts,
		}
	}

	return queue.ScheduledTask{
		Name:     options.Name,
		CronSpec: cronSpec,
		Task:     task,
		Opts:     options.AsynqOpts,
	}
}

// EveryMinute schedules a task to run every minute.
//
// Cron: * * * * *
func EveryMinute(taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled("* * * * *", taskFn, opts...)
}

// Every5Minutes schedules a task to run every 5 minutes.
//
// Cron: */5 * * * *
func Every5Minutes(taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled("*/5 * * * *", taskFn, opts...)
}

// Hourly schedules a task to run every hour at minute 0.
//
// Cron: 0 * * * *
func Hourly(taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled("0 * * * *", taskFn, opts...)
}

// Daily schedules a task to run daily at midnight.
//
// Cron: 0 0 * * *
func Daily(taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled("0 0 * * *", taskFn, opts...)
}

// Weekly schedules a task to run weekly on Sunday at midnight.
//
// Cron: 0 0 * * 0
func Weekly(taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled("0 0 * * 0", taskFn, opts...)
}

// Monthly schedules a task to run monthly on the 1st at midnight.
//
// Cron: 0 0 1 * *
func Monthly(taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled("0 0 1 * *", taskFn, opts...)
}

// At schedules a task with a custom cron expression.
//
// Cron format: minute hour day-of-month month day-of-week
//
// Examples:
//   - "0 2 * * *"      - Daily at 2:00 AM
//   - "30 4 * * *"     - Daily at 4:30 AM
//   - "0 9 * * 1-5"    - Weekdays at 9:00 AM
//   - "0 0 15 * *"     - 15th of each month at midnight
//   - "*/10 * * * *"   - Every 10 minutes
//   - "0 */2 * * *"    - Every 2 hours
func At(cronSpec string, taskFn TaskFunc, opts ...TaskOption) queue.ScheduledTask {
	return scheduled(cronSpec, taskFn, opts...)
}
