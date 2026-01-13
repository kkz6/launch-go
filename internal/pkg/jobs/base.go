// Package jobs provides base infrastructure for async job handling
package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// Handler is the interface that all jobs must implement.
// Similar to Laravel's ShouldQueue with handle() and failed() methods.
type Handler interface {
	// Handle processes the job
	Handle(ctx context.Context) error

	// Failed is called when the job fails after all retries
	Failed(ctx context.Context, err error)

	// Type returns the job type identifier
	Type() string
}

// Payload is the interface for job payloads
type Payload interface {
	// Validate validates the payload
	Validate() error
}

// TaskExecutor interface for executing tasks on servers
type TaskExecutor interface {
	RunTask(ctx context.Context, serverID string, task interface{}, asRoot bool) error
}

// BaseJob provides common functionality for all jobs.
// Embed this in your job structs to get common features.
//
// Usage:
//
//	type InstallCronJob struct {
//	    jobs.BaseJob
//	    CronID string `json:"cron_id"`
//	}
type BaseJob struct {
	DB           *gorm.DB               `json:"-"`
	Logger       *zerolog.Logger        `json:"-"`
	WS           Broadcaster            `json:"-"`
	Dispatcher   *taskrunner.Dispatcher `json:"-"`
	Queue        *queue.Client          `json:"-"`
	TaskExecutor TaskExecutor           `json:"-"`
}

// SetDependencies sets the job dependencies
func (j *BaseJob) SetDependencies(db *gorm.DB, logger *zerolog.Logger, ws Broadcaster) {
	j.DB = db
	j.Logger = logger
	j.WS = ws
}

// SetDispatcher sets the task dispatcher
func (j *BaseJob) SetDispatcher(dispatcher *taskrunner.Dispatcher) {
	j.Dispatcher = dispatcher
}

// SetQueue sets the queue client
func (j *BaseJob) SetQueue(q *queue.Client) {
	j.Queue = q
}

// SetTaskExecutor sets the task executor for running tasks on servers
func (j *BaseJob) SetTaskExecutor(executor TaskExecutor) {
	j.TaskExecutor = executor
}

// ===============================
// Getter methods for DependencyAware interface
// ===============================

// GetDB returns the database connection
func (j *BaseJob) GetDB() *gorm.DB {
	return j.DB
}

// GetLogger returns the logger
func (j *BaseJob) GetLogger() *zerolog.Logger {
	return j.Logger
}

// GetWS returns the broadcaster
func (j *BaseJob) GetWS() Broadcaster {
	return j.WS
}

// GetDispatcher returns the task dispatcher
func (j *BaseJob) GetDispatcher() *taskrunner.Dispatcher {
	return j.Dispatcher
}

// GetQueue returns the queue client
func (j *BaseJob) GetQueue() *queue.Client {
	return j.Queue
}

// ===============================
// Individual setter methods
// ===============================

// SetDB sets the database connection
func (j *BaseJob) SetDB(db *gorm.DB) {
	j.DB = db
}

// SetLogger sets the logger
func (j *BaseJob) SetLogger(logger *zerolog.Logger) {
	j.Logger = logger
}

// SetWS sets the broadcaster
func (j *BaseJob) SetWS(ws Broadcaster) {
	j.WS = ws
}

// RunTask executes a task on a server
func (j *BaseJob) RunTask(ctx context.Context, serverID string, task interface{}, asRoot bool) error {
	if j.TaskExecutor == nil {
		return fmt.Errorf("task executor not configured")
	}
	return j.TaskExecutor.RunTask(ctx, serverID, task, asRoot)
}

// LogInfo logs an info message
func (j *BaseJob) LogInfo(msg string, fields ...interface{}) {
	if j.Logger == nil {
		return
	}
	event := j.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error message
func (j *BaseJob) LogError(err error, msg string, fields ...interface{}) {
	if j.Logger == nil {
		return
	}
	event := j.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// Broadcast sends a websocket broadcast
func (j *BaseJob) Broadcast(channel, event string, data interface{}) {
	if j.WS != nil {
		j.WS.Broadcast(channel, event, data)
	}
}

// BroadcastToServer sends a websocket broadcast to a server channel
func (j *BaseJob) BroadcastToServer(serverID, event string, data interface{}) {
	if j.WS != nil {
		j.WS.BroadcastToServer(serverID, event, data)
	}
}

// JobFactory creates job handlers from asynq tasks
type JobFactory interface {
	// Create creates a job handler from an asynq task
	Create(t *asynq.Task) (Handler, error)
}

// Registry manages job registration and creation
type Registry struct {
	factories  map[string]func(*asynq.Task) (Handler, error)
	db         *gorm.DB
	logger     *zerolog.Logger
	ws         Broadcaster
	dispatcher *taskrunner.Dispatcher
	queue      *queue.Client
}

// NewRegistry creates a new job registry
func NewRegistry(db *gorm.DB, logger *zerolog.Logger, ws Broadcaster) *Registry {
	return &Registry{
		factories: make(map[string]func(*asynq.Task) (Handler, error)),
		db:        db,
		logger:    logger,
		ws:        ws,
	}
}

// Register registers a job factory for a given type
func (r *Registry) Register(jobType string, factory func(*asynq.Task) (Handler, error)) {
	r.factories[jobType] = factory
}

// Create creates a job handler from an asynq task
func (r *Registry) Create(t *asynq.Task) (Handler, error) {
	factory, ok := r.factories[t.Type()]
	if !ok {
		return nil, fmt.Errorf("unknown job type: %s", t.Type())
	}
	return factory(t)
}

// HandlerFunc wraps the registry to be used as asynq handler
func (r *Registry) HandlerFunc(jobType string) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		handler, err := r.Create(t)
		if err != nil {
			return err
		}

		// Set dependencies if the job has BaseJob embedded
		if baseJob, ok := handler.(interface {
			SetDependencies(*gorm.DB, *zerolog.Logger, Broadcaster)
		}); ok {
			baseJob.SetDependencies(r.db, r.logger, r.ws)
		}

		// Set dispatcher if available and job supports it
		if r.dispatcher != nil {
			if dispatcherSetter, ok := handler.(interface {
				SetDispatcher(*taskrunner.Dispatcher)
			}); ok {
				dispatcherSetter.SetDispatcher(r.dispatcher)
			}
		}

		// Set queue if available and job supports it
		if r.queue != nil {
			if queueSetter, ok := handler.(interface {
				SetQueue(*queue.Client)
			}); ok {
				queueSetter.SetQueue(r.queue)
			}
		}

		return handler.Handle(ctx)
	}
}

// RegisterHandlers registers all job handlers with the asynq mux
func (r *Registry) RegisterHandlers(mux *asynq.ServeMux) {
	for jobType := range r.factories {
		mux.HandleFunc(jobType, r.HandlerFunc(jobType))
	}
}

// CreateTask creates an asynq task from a payload struct
func CreateTask[T any](jobType string, payload T) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(jobType, data), nil
}

// ParsePayload parses an asynq task payload into a struct
func ParsePayload[T any](t *asynq.Task) (T, error) {
	var payload T
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return payload, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return payload, nil
}
