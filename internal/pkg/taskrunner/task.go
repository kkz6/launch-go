package taskrunner

import (
	"context"
	"time"
)

// Task represents a runnable task with a pre-rendered script
type Task interface {
	Name() string
	Script() string
	Timeout() time.Duration
	OnOutput(output string)
	OnFinished(ctx context.Context, result *TaskResult)
	OnFailed(ctx context.Context, result *TaskResult)
	OnTimeout(ctx context.Context, result *TaskResult)
}

// TaskOption configures a BaseTask
type TaskOption func(*BaseTask)

// WithName sets the task name
func WithName(name string) TaskOption {
	return func(t *BaseTask) {
		t.name = name
	}
}

// WithScript sets the pre-rendered script content
func WithScript(script string) TaskOption {
	return func(t *BaseTask) {
		t.script = script
	}
}

// WithTimeout sets the task timeout using a time.Duration value.
func WithTimeout(timeout time.Duration) TaskOption {
	return func(t *BaseTask) {
		t.timeout = timeout
	}
}

// WithTimeoutSeconds sets the task timeout using seconds as an integer.
// This is a convenience wrapper around WithTimeout for cleaner task definitions.
func WithTimeoutSeconds(seconds int) TaskOption {
	return WithTimeout(time.Duration(seconds) * time.Second)
}

// NewBaseTask creates a new BaseTask with the given options
func NewBaseTask(opts ...TaskOption) *BaseTask {
	t := &BaseTask{
		timeout: 10 * time.Minute, // Default timeout
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// BaseTask provides the standard task implementation
type BaseTask struct {
	name    string
	script  string
	timeout time.Duration

	// Callbacks
	OutputCallback   func(output string)
	FinishedCallback func(ctx context.Context, result *TaskResult)
	FailedCallback   func(ctx context.Context, result *TaskResult)
	TimeoutCallback  func(ctx context.Context, result *TaskResult)
}

// Name returns the task name
func (t *BaseTask) Name() string {
	return t.name
}

// Script returns the pre-rendered script content
func (t *BaseTask) Script() string {
	return t.script
}

// Timeout returns the task timeout
func (t *BaseTask) Timeout() time.Duration {
	if t.timeout == 0 {
		return 10 * time.Minute
	}
	return t.timeout
}

// SetScript sets the script content
func (t *BaseTask) SetScript(script string) {
	t.script = script
}

// SetName sets the task name
func (t *BaseTask) SetName(name string) {
	t.name = name
}

// SetTimeout sets the task timeout
func (t *BaseTask) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// OnOutput handles output streaming
func (t *BaseTask) OnOutput(output string) {
	if t.OutputCallback != nil {
		t.OutputCallback(output)
	}
}

// OnFinished handles successful completion
func (t *BaseTask) OnFinished(ctx context.Context, result *TaskResult) {
	if t.FinishedCallback != nil {
		t.FinishedCallback(ctx, result)
	}
}

// OnFailed handles task failure
func (t *BaseTask) OnFailed(ctx context.Context, result *TaskResult) {
	if t.FailedCallback != nil {
		t.FailedCallback(ctx, result)
	}
}

// OnTimeout handles task timeout
func (t *BaseTask) OnTimeout(ctx context.Context, result *TaskResult) {
	if t.TimeoutCallback != nil {
		t.TimeoutCallback(ctx, result)
	}
}

// Pending creates a PendingTask from this task
func (t *BaseTask) Pending() *PendingTask {
	return NewPendingTask(t)
}

// ScriptTask is an alias for BaseTask for clarity
type ScriptTask = BaseTask

// NewScriptTask creates a new task with a pre-rendered script
func NewScriptTask(name string, script string, timeout time.Duration) *BaseTask {
	return NewBaseTask(
		WithName(name),
		WithScript(script),
		WithTimeout(timeout),
	)
}

// Script preamble helpers live in taskrunner/templates — ShellDefaults,
// CommonFunctions and AptFunctions there are the single source of truth.
