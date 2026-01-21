// Package jobs provides infrastructure for async job handling with asynq.
package jobs

import (
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/logger"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// Base provides common dependencies for all job contexts.
// Module-specific job contexts should embed this struct to get
// standard logging, broadcasting, and database access.
//
// Example usage:
//
//	type JobContext struct {
//	    jobs.Base
//	    Repos  *repositories.Registry
//	    // ... module-specific fields
//	}
//
//	func NewJobContext(deps jobs.BaseDeps, repos *repositories.Registry) *JobContext {
//	    return &JobContext{
//	        Base:  jobs.NewBase(deps),
//	        Repos: repos,
//	    }
//	}
type Base struct {
	db         *gorm.DB
	logger     *zerolog.Logger
	ws         broadcast.TeamBroadcaster
	dispatcher taskrunner.TaskDispatcher
	queue      *queue.Client
}

// BaseDeps holds the common dependencies for creating a Base context.
type BaseDeps struct {
	DB         *gorm.DB
	Logger     *zerolog.Logger
	WS         broadcast.TeamBroadcaster
	Dispatcher taskrunner.TaskDispatcher
	Queue      *queue.Client
}

// NewBase creates a new Base context with the provided dependencies.
func NewBase(deps BaseDeps) Base {
	return Base{
		db:         deps.DB,
		logger:     deps.Logger,
		ws:         deps.WS,
		dispatcher: deps.Dispatcher,
		queue:      deps.Queue,
	}
}

// DB returns the database connection.
func (b *Base) DB() *gorm.DB {
	return b.db
}

// Logger returns the logger.
func (b *Base) Logger() *zerolog.Logger {
	return b.logger
}

// WS returns the team broadcaster.
func (b *Base) WS() broadcast.TeamBroadcaster {
	return b.ws
}

// Dispatcher returns the task dispatcher.
func (b *Base) Dispatcher() taskrunner.TaskDispatcher {
	return b.dispatcher
}

// Queue returns the queue client.
func (b *Base) Queue() *queue.Client {
	return b.queue
}

// Dispatch creates and enqueues a job with the given type and payload.
// Returns nil if the queue is nil (allows graceful degradation in tests).
// Uses DefaultMaxRetry (0) by default - jobs run once without retry.
//
// Example:
//
//	err := ctx.Dispatch("site:install_caddyfile", CaddyfilePayload{SiteID: siteID})
func (b *Base) Dispatch(jobType string, payload any, opts ...asynq.Option) error {
	if b.queue == nil {
		return nil
	}

	task, err := NewTask(jobType, payload, opts...)
	if err != nil {
		b.LogError(err, "Failed to create job task", "job_type", jobType)
		return err
	}

	info, err := b.queue.Enqueue(task)
	if err != nil {
		b.LogError(err, "Failed to enqueue job", "job_type", jobType)
		return err
	}

	b.LogDebug("Job dispatched", "job_type", jobType, "task_id", info.ID, "queue", info.Queue)
	return nil
}

// DispatchWithID creates and enqueues a job with a specific task ID.
// Useful for idempotent jobs where you want to prevent duplicates.
//
// Example:
//
//	err := ctx.DispatchWithID("site:install_caddyfile", payload, "caddyfile:"+siteID)
func (b *Base) DispatchWithID(jobType string, payload any, taskID string, opts ...asynq.Option) error {
	opts = append(opts, asynq.TaskID(taskID))
	return b.Dispatch(jobType, payload, opts...)
}

// DispatchIn creates and enqueues a job to be processed after a delay.
//
// Example:
//
//	err := ctx.DispatchIn("site:deploy", payload, 5*time.Second)
func (b *Base) DispatchIn(jobType string, payload any, delay time.Duration, opts ...asynq.Option) error {
	opts = append(opts, asynq.ProcessIn(delay))
	return b.Dispatch(jobType, payload, opts...)
}

// DispatchToQueue creates and enqueues a job to a specific queue.
//
// Example:
//
//	err := ctx.DispatchToQueue("server:provision", payload, queue.QueueCritical)
func (b *Base) DispatchToQueue(jobType string, payload any, queueName string, opts ...asynq.Option) error {
	opts = append(opts, asynq.Queue(queueName))
	return b.Dispatch(jobType, payload, opts...)
}

// DispatchTask enqueues a pre-built asynq task.
// This is useful when using NewXXXTask helper functions.
// Returns nil if the queue is nil (allows graceful degradation in tests).
//
// Example:
//
//	task, err := NewInstallQueueTask(siteID, queueID, userID)
//	if err != nil {
//	    return err
//	}
//	return ctx.DispatchTask(task)
func (b *Base) DispatchTask(task *asynq.Task, opts ...asynq.Option) error {
	if b.queue == nil {
		return nil
	}

	info, err := b.queue.Enqueue(task, opts...)
	if err != nil {
		b.LogError(err, "Failed to enqueue task", "task_type", task.Type())
		return err
	}

	b.LogDebug("Task dispatched", "task_type", task.Type(), "task_id", info.ID, "queue", info.Queue)
	return nil
}

// DispatchTaskIn enqueues a pre-built task to be processed after a delay.
//
// Example:
//
//	task, _ := NewDeployTask(payload)
//	return ctx.DispatchTaskIn(task, 5*time.Second)
func (b *Base) DispatchTaskIn(task *asynq.Task, delay time.Duration, opts ...asynq.Option) error {
	opts = append(opts, asynq.ProcessIn(delay))
	return b.DispatchTask(task, opts...)
}

// LogInfo logs an info message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogInfo("Processing job", "jobID", job.ID, "userID", userID)
func (b *Base) LogInfo(msg string, fields ...any) {
	logger.Info(b.logger, msg, fields...)
}

// LogError logs an error message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogError(err, "Failed to process job", "jobID", job.ID)
func (b *Base) LogError(err error, msg string, fields ...any) {
	logger.Error(b.logger, err, msg, fields...)
}

// LogWarn logs a warning message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogWarn("Retry limit approaching", "attempts", attempts)
func (b *Base) LogWarn(msg string, fields ...any) {
	logger.Warn(b.logger, msg, fields...)
}

// LogDebug logs a debug message with optional key-value pairs.
// Fields should be provided as alternating key-value pairs.
//
// Example:
//
//	ctx.LogDebug("Job payload", "payload", payload)
func (b *Base) LogDebug(msg string, fields ...any) {
	logger.Debug(b.logger, msg, fields...)
}

// BroadcastToTeam sends a websocket event to a team channel.
// This is a no-op if the broadcaster is nil.
//
// Example:
//
//	ctx.BroadcastToTeam(teamID, "server.updated", serverData)
func (b *Base) BroadcastToTeam(teamID, event string, data any) {
	if b.ws != nil {
		b.ws.BroadcastToTeam(teamID, event, data)
	}
}

// BaseJob provides common job structure with typed context and payload.
// This eliminates the repeated ctx/payload field declarations across jobs.
//
// The context type parameter C allows each module to use its own JobContext type,
// which may have module-specific methods (e.g., GetTaskFactory, ForServer, etc.)
//
// Example usage:
//
//	type InstallDatabaseJob struct {
//	    jobs.BaseJob[*JobContext, InstallDatabasePayload]
//	    traits.InstallationTracker
//	}
//
//	func NewInstallDatabaseJob(ctx *JobContext, payload InstallDatabasePayload) *InstallDatabaseJob {
//	    return &InstallDatabaseJob{
//	        BaseJob: jobs.NewBaseJob(ctx, payload),
//	    }
//	}
//
//	func (j *InstallDatabaseJob) Handle(ctx context.Context) error {
//	    j.Ctx.LogInfo("Installing database", "database_id", j.Payload.DatabaseID)
//	    // ... use j.Ctx for context methods, j.Payload for payload fields
//	}
type BaseJob[C any, P any] struct {
	// Ctx is the job context providing access to DB, logging, broadcasting, etc.
	// Public field allows direct access: j.Ctx.LogInfo(...)
	Ctx C

	// Payload contains job-specific data passed when the job was enqueued.
	// Public field allows direct access: j.Payload.SomeField
	Payload P
}

// NewBaseJob creates a new base job with the given context and payload.
//
// Example:
//
//	func NewInstallDatabaseJob(ctx *JobContext, payload InstallDatabasePayload) *InstallDatabaseJob {
//	    return &InstallDatabaseJob{
//	        BaseJob: jobs.NewBaseJob(ctx, payload),
//	    }
//	}
func NewBaseJob[C any, P any](ctx C, payload P) BaseJob[C, P] {
	return BaseJob[C, P]{
		Ctx:     ctx,
		Payload: payload,
	}
}
