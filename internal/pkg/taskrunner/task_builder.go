package taskrunner

import (
	"context"
	"encoding/json"
	"time"
)

// SuccessHandler is called when a task completes successfully
type SuccessHandler[C any] func(ctx context.Context, cbCtx *CallbackContext, callback C, taskID string) error

// FailureHandler is called when a task fails
type FailureHandler[C any] func(ctx context.Context, cbCtx *CallbackContext, callback C, taskID string, exitCode int) error

// ExpiredHandler is called when a task times out
type ExpiredHandler[C any] func(ctx context.Context, cbCtx *CallbackContext, callback C, taskID string) error

// TaskBuilder provides a fluent interface for creating tasks with callbacks.
// The type parameter C represents the callback data type that will be serialized
// and available in callback handlers.
//
// Example usage:
//
//	type DeployCallback struct {
//	    SiteID       string `json:"site_id"`
//	    DeploymentID string `json:"deployment_id"`
//	}
//
//	func DeploySiteTask(opts DeployOptions) *taskrunner.BuiltTask[DeployCallback] {
//	    return taskrunner.NewTaskBuilder[DeployCallback]("site:deploy").
//	        WithName("Deploy Site").
//	        WithScript(buildScript(opts)).
//	        WithTimeoutSeconds(600).
//	        WithCallback(DeployCallback{
//	            SiteID:       opts.Site.ID,
//	            DeploymentID: opts.Deployment.ID,
//	        }).
//	        OnSuccess(func(ctx context.Context, cbCtx *taskrunner.CallbackContext, cb DeployCallback, taskID string) error {
//	            // Update deployment status
//	            return cbCtx.DB.Model(&models.Deployment{}).
//	                Where("id = ?", cb.DeploymentID).
//	                Update("status", "finished").Error
//	        }).
//	        Build()
//	}
type TaskBuilder[C any] struct {
	typeName  string
	name      string
	script    string
	timeout   time.Duration
	callback  C
	onSuccess SuccessHandler[C]
	onFailure FailureHandler[C]
	onExpired ExpiredHandler[C]
}

// NewTaskBuilder creates a new task builder.
// The typeName is used for callback reconstruction (e.g., "site:deploy").
func NewTaskBuilder[C any](typeName string) *TaskBuilder[C] {
	return &TaskBuilder[C]{
		typeName: typeName,
		timeout:  10 * time.Minute, // Default timeout
	}
}

// WithName sets the task display name
func (b *TaskBuilder[C]) WithName(name string) *TaskBuilder[C] {
	b.name = name
	return b
}

// WithScript sets the bash script to execute
func (b *TaskBuilder[C]) WithScript(script string) *TaskBuilder[C] {
	b.script = script
	return b
}

// WithTimeout sets the task timeout
func (b *TaskBuilder[C]) WithTimeout(timeout time.Duration) *TaskBuilder[C] {
	b.timeout = timeout
	return b
}

// WithTimeoutSeconds sets the task timeout in seconds
func (b *TaskBuilder[C]) WithTimeoutSeconds(seconds int) *TaskBuilder[C] {
	b.timeout = time.Duration(seconds) * time.Second
	return b
}

// WithCallback sets the callback data that will be serialized and available in handlers
func (b *TaskBuilder[C]) WithCallback(callback C) *TaskBuilder[C] {
	b.callback = callback
	return b
}

// OnSuccess sets the handler called when the task completes successfully
func (b *TaskBuilder[C]) OnSuccess(handler SuccessHandler[C]) *TaskBuilder[C] {
	b.onSuccess = handler
	return b
}

// OnFailure sets the handler called when the task fails
func (b *TaskBuilder[C]) OnFailure(handler FailureHandler[C]) *TaskBuilder[C] {
	b.onFailure = handler
	return b
}

// OnExpired sets the handler called when the task times out
func (b *TaskBuilder[C]) OnExpired(handler ExpiredHandler[C]) *TaskBuilder[C] {
	b.onExpired = handler
	return b
}

// Build creates the final task with all configured options
func (b *TaskBuilder[C]) Build() *BuiltTask[C] {
	return &BuiltTask[C]{
		BaseTask: NewBaseTask(
			WithName(b.name),
			WithScript(b.script),
			WithTimeout(b.timeout),
		),
		typeName:  b.typeName,
		callback:  b.callback,
		onSuccess: b.onSuccess,
		onFailure: b.onFailure,
		onExpired: b.onExpired,
	}
}

// BuiltTask is the result of TaskBuilder.Build().
// It implements both Task and CallbackPayload interfaces.
type BuiltTask[C any] struct {
	*BaseTask
	typeName  string
	callback  C
	onSuccess SuccessHandler[C]
	onFailure FailureHandler[C]
	onExpired ExpiredHandler[C]
}

// TypeName returns the registered type name for callback reconstruction
func (t *BuiltTask[C]) TypeName() string {
	return t.typeName
}

// MarshalPayload returns JSON representation of the callback data
func (t *BuiltTask[C]) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

// GetCallback returns the callback data (useful for accessing in tests)
func (t *BuiltTask[C]) GetCallback() C {
	return t.callback
}

// OnSuccess implements CallbackHandler
func (t *BuiltTask[C]) OnSuccess(ctx context.Context, cbCtx *CallbackContext, taskID string) error {
	if t.onSuccess == nil {
		return nil
	}
	return t.onSuccess(ctx, cbCtx, t.callback, taskID)
}

// OnFailure implements CallbackHandler
func (t *BuiltTask[C]) OnFailure(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error {
	if t.onFailure == nil {
		return nil
	}
	return t.onFailure(ctx, cbCtx, t.callback, taskID, exitCode)
}

// OnExpired implements CallbackHandler
func (t *BuiltTask[C]) OnExpired(ctx context.Context, cbCtx *CallbackContext, taskID string) error {
	if t.onExpired == nil {
		return nil
	}
	return t.onExpired(ctx, cbCtx, t.callback, taskID)
}

// RegisterBuiltTask registers a BuiltTask type for callback reconstruction.
// The handlers must be provided at registration time since functions cannot be serialized.
//
// Example:
//
//	func RegisterTaskCallbacks() {
//	    taskrunner.RegisterBuiltTask[DeployCallback]("site:deploy",
//	        deployOnSuccess, deployOnFailure, deployOnExpired)
//	}
func RegisterBuiltTask[C any](
	typeName string,
	onSuccess SuccessHandler[C],
	onFailure FailureHandler[C],
	onExpired ExpiredHandler[C],
) {
	Register(typeName, func(payload []byte) (CallbackHandler, error) {
		var callback C
		if err := json.Unmarshal(payload, &callback); err != nil {
			return nil, err
		}

		return &BuiltTask[C]{
			BaseTask:  NewBaseTask(),
			typeName:  typeName,
			callback:  callback,
			onSuccess: onSuccess,
			onFailure: onFailure,
			onExpired: onExpired,
		}, nil
	})
}
