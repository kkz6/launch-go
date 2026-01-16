package taskrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// CallbackType represents the type of callback
type CallbackType string

const (
	CallbackFinished CallbackType = "finished"
	CallbackFailed   CallbackType = "failed"
	CallbackTimeout  CallbackType = "timeout"
	CallbackCustom   CallbackType = "custom"
)

// QueueClient interface for enqueueing jobs
type QueueClient interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// CallbackContext provides dependencies to callback handlers.
// This is similar to Laravel's service container - it allows reconstructed
// handlers to access DB, queue, and other services.
type CallbackContext struct {
	DB     *gorm.DB
	Queue  QueueClient
	Logger *zerolog.Logger
}

// DispatchJob is a helper to dispatch an asynq job from a callback handler
func (c *CallbackContext) DispatchJob(jobType string, payload interface{}) error {
	if c.Queue == nil {
		if c.Logger != nil {
			c.Logger.Warn().Str("job_type", jobType).Msg("Queue not available, cannot dispatch job")
		}
		return fmt.Errorf("queue not available")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	task := asynq.NewTask(jobType, data)
	if _, err := c.Queue.Enqueue(task); err != nil {
		if c.Logger != nil {
			c.Logger.Error().Err(err).Str("job_type", jobType).Msg("Failed to dispatch job")
		}
		return err
	}

	if c.Logger != nil {
		c.Logger.Info().Str("job_type", jobType).Msg("Job dispatched from callback")
	}
	return nil
}

// CallbackHandler is implemented by tasks that need continuation logic after
// the task completes on the server. This is similar to Laravel's HasCallbacks interface.
type CallbackHandler interface {
	// OnSuccess is called when the task completes successfully
	OnSuccess(ctx context.Context, cbCtx *CallbackContext, taskID string) error

	// OnFailure is called when the task fails with an exit code
	OnFailure(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error

	// OnExpired is called when the task times out
	OnExpired(ctx context.Context, cbCtx *CallbackContext, taskID string) error
}

// CallbackPayload represents a task that can be serialized for storage
// and later reconstructed to handle callbacks
type CallbackPayload interface {
	CallbackHandler

	// TypeName returns the registered type name for reconstruction
	TypeName() string

	// MarshalPayload returns JSON representation of the task state
	// needed for callback handling
	MarshalPayload() ([]byte, error)
}

// TaskFactory creates a CallbackHandler from a JSON payload
type TaskFactory func(payload []byte) (CallbackHandler, error)

// TaskTypeRegistry manages task type registration and reconstruction
// This enables Go to recreate task instances from stored payloads,
// similar to PHP's unserialize()
type TaskTypeRegistry struct {
	mu        sync.RWMutex
	factories map[string]TaskFactory
}

// NewTaskTypeRegistry creates a new registry
func NewTaskTypeRegistry() *TaskTypeRegistry {
	return &TaskTypeRegistry{
		factories: make(map[string]TaskFactory),
	}
}

// Register adds a task type to the registry
func (r *TaskTypeRegistry) Register(typeName string, factory TaskFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[typeName] = factory
}

// Reconstruct creates a CallbackHandler from a stored type name and payload
func (r *TaskTypeRegistry) Reconstruct(typeName string, payload []byte) (CallbackHandler, error) {
	r.mu.RLock()
	factory, ok := r.factories[typeName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown task type: %s", typeName)
	}

	return factory(payload)
}

// HasType checks if a type is registered
func (r *TaskTypeRegistry) HasType(typeName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[typeName]
	return ok
}

// DefaultRegistry is the global task type registry
var DefaultRegistry = NewTaskTypeRegistry()

// Register registers a task type with the default registry
func Register(typeName string, factory TaskFactory) {
	DefaultRegistry.Register(typeName, factory)
}

// Reconstruct reconstructs a task from the default registry
func Reconstruct(typeName string, payload []byte) (CallbackHandler, error) {
	return DefaultRegistry.Reconstruct(typeName, payload)
}

// TaskInstance represents serialized task data stored in the database
type TaskInstance struct {
	TypeName string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
}

// MarshalInstance serializes a CallbackPayload for database storage
func MarshalInstance(task CallbackPayload) (string, error) {
	payload, err := task.MarshalPayload()
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	instance := TaskInstance{
		TypeName: task.TypeName(),
		Payload:  payload,
	}

	data, err := json.Marshal(instance)
	if err != nil {
		return "", fmt.Errorf("failed to marshal instance: %w", err)
	}

	return string(data), nil
}

// UnmarshalInstance deserializes a TaskInstance from database storage
func UnmarshalInstance(data string) (*TaskInstance, error) {
	if data == "" {
		return nil, nil
	}

	var instance TaskInstance
	if err := json.Unmarshal([]byte(data), &instance); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instance: %w", err)
	}

	return &instance, nil
}

// ReconstructFromInstance reconstructs a CallbackHandler from stored instance data
func ReconstructFromInstance(data string) (CallbackHandler, error) {
	instance, err := UnmarshalInstance(data)
	if err != nil {
		return nil, err
	}

	if instance == nil {
		return nil, nil
	}

	return Reconstruct(instance.TypeName, instance.Payload)
}

// RegisterCallback registers a task callback handler using generics.
// This provides a clean, one-line registration similar to job handlers.
//
// Type parameters:
//   - S: The state struct type that gets serialized (must contain only IDs/simple data)
//   - H: The callback handler type
//
// The newHandler function receives the unmarshaled state and returns the handler.
//
// Example usage in a module's register.go:
//
//	func RegisterTaskCallbacks() {
//	    taskrunner.RegisterCallback("site:deploy", NewDeploySiteCallback)
//	    taskrunner.RegisterCallback("site:rollback", NewRollbackCallback)
//	}
//
//	func NewDeploySiteCallback(state DeployTaskState) *deploySiteTask {
//	    return &deploySiteTask{state: state}
//	}
func RegisterCallback[S any, H CallbackHandler](typeName string, newHandler func(state S) H) {
	Register(typeName, func(payload []byte) (CallbackHandler, error) {
		var state S
		if err := json.Unmarshal(payload, &state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state for %s: %w", typeName, err)
		}
		return newHandler(state), nil
	})
}

// CallbackStateFactory is implemented by state structs that can create their own task.
// This eliminates the need for separate factory functions.
//
// Example:
//
//	type DeployState struct {
//	    SiteID string `json:"site_id"`
//	}
//
//	func (s DeployState) NewTask() taskrunner.CallbackHandler {
//	    return &deploySiteTask{callback: s}
//	}
type CallbackStateFactory interface {
	NewTask() CallbackHandler
}

// RegisterCallbackState registers a task using a state struct that implements CallbackStateFactory.
// This is the simplest registration - just pass the type name.
//
// Example usage:
//
//	func RegisterTaskCallbacks() {
//	    taskrunner.RegisterCallbackState[DeployState](DeploySiteTaskType)
//	}
func RegisterCallbackState[S CallbackStateFactory](typeName string) {
	Register(typeName, func(payload []byte) (CallbackHandler, error) {
		var state S
		if err := json.Unmarshal(payload, &state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state for %s: %w", typeName, err)
		}
		return state.NewTask(), nil
	})
}
