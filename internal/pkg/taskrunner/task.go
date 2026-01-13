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

// WithCallbackURL sets the callback URL for async task completion
func WithCallbackURL(url string) TaskOption {
	return func(t *BaseTask) {
		t.callbackURL = url
	}
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
	name        string
	script      string
	timeout     time.Duration
	callbackURL string

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

// GetCallbackURL returns the callback URL
func (t *BaseTask) GetCallbackURL() string {
	return t.callbackURL
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

// WrapScript wraps a script with shell defaults
func WrapScript(script string) string {
	return "#!/bin/bash\nset -euo pipefail\nexport DEBIAN_FRONTEND=noninteractive\n\n" + script
}

// ShellDefaults returns the standard shell script header
func ShellDefaults() string {
	return `set -euo pipefail
export DEBIAN_FRONTEND=noninteractive`
}

// CommonFunctions returns common bash helper functions
func CommonFunctions() string {
	return `# Send a POST request to the given URL, ignoring the response and errors
function httpPostSilently() {
    if [ -z "${2:-}" ]; then
        (curl -X POST --silent --max-time 15 --output /dev/null $1 || true)
    else
        (curl -X POST --silent --max-time 15 --output /dev/null $1 -H 'Content-Type: application/json' --data "$2" || true)
    fi
}

function httpPostRawSilently() {
    (curl -X POST --silent --max-time 15 --output /dev/null $1 --data "$2" || true)
}`
}

// AptFunctions returns apt-related bash functions
func AptFunctions() string {
	return `# Wait for apt to be unlocked
function waitForAptUnlock() {
    while ps -C apt,apt-get,dpkg >/dev/null 2>&1; do
        echo "apt, apt-get or dpkg is running..."
        sleep 5
    done

    while fuser /var/{lib/{dpkg,apt/lists},cache/apt/archives}/{lock,lock-frontend} >/dev/null 2>&1; do
        echo "Waiting: apt is locked..."
        sleep 5
    done

    if [ -f /var/log/unattended-upgrades/unattended-upgrades.log ]; then
        while fuser /var/log/unattended-upgrades/unattended-upgrades.log >/dev/null 2>&1; do
            echo "Waiting: unattended-upgrades is locked..."
            sleep 5
        done
    fi
}`
}
