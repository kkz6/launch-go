package taskrunner

import (
	"context"

	"github.com/oklog/ulid/v2"
)

// PendingTask wraps a task with execution options
type PendingTask struct {
	Task       Task
	Connection *Connection
	Background bool
	TaskID     string
	OutputPath string // Local path to write output

	// Output callback for streaming (receives full accumulated output)
	onOutput func(output string)

	// Per-task marker handler for real-time marker processing during background monitoring.
	// This is called for each marker detected in the output stream, allowing tasks to
	// respond to progress updates, step completions, etc. as they happen.
	markerHandler MarkerHandler

	// Completion channel for background tasks
	// When set, the dispatcher will send the result here when the task completes
	completionChan chan *TaskResult
}

// NewPendingTask creates a new PendingTask wrapper
func NewPendingTask(task Task) *PendingTask {
	pending := &PendingTask{Task: task}
	pending.ensureTaskID()
	return pending
}

func (p *PendingTask) ensureTaskID() {
	if p.TaskID == "" {
		p.TaskID = ulid.Make().String()
	}
}

// OnConnection sets the SSH connection for remote execution
func (p *PendingTask) OnConnection(conn *Connection) *PendingTask {
	p.Connection = conn
	return p
}

// InBackground sets the task to run in background mode
func (p *PendingTask) InBackground() *PendingTask {
	p.Background = true
	return p
}

// InForeground sets the task to run in foreground mode (default)
func (p *PendingTask) InForeground() *PendingTask {
	p.Background = false
	return p
}

// WithID sets a custom task ID
func (p *PendingTask) WithID(id string) *PendingTask {
	p.TaskID = id
	return p
}

// As is an alias for WithID
func (p *PendingTask) As(id string) *PendingTask {
	return p.WithID(id)
}

// WriteOutputTo sets the local path to write output
func (p *PendingTask) WriteOutputTo(path string) *PendingTask {
	p.OutputPath = path
	return p
}

// OnOutput sets a callback for output streaming
func (p *PendingTask) OnOutput(callback func(output string)) *PendingTask {
	p.onOutput = callback
	return p
}

// GetOnOutput returns the output callback
func (p *PendingTask) GetOnOutput() func(output string) {
	return p.onOutput
}

// WithMarkerHandler sets a per-task marker handler for real-time processing
// during background monitoring. The handler is called for each marker detected
// in the output stream (e.g., progress, step_completed, software_installed).
func (p *PendingTask) WithMarkerHandler(handler MarkerHandler) *PendingTask {
	p.markerHandler = handler
	return p
}

// GetMarkerHandler returns the per-task marker handler
func (p *PendingTask) GetMarkerHandler() MarkerHandler {
	return p.markerHandler
}

// ShouldRunInBackground returns true if task should run in background
func (p *PendingTask) ShouldRunInBackground() bool {
	return p.Background
}

// ShouldRunInForeground returns true if task should run in foreground
func (p *PendingTask) ShouldRunInForeground() bool {
	return !p.Background
}

// GetConnection returns the SSH connection
func (p *PendingTask) GetConnection() *Connection {
	return p.Connection
}

// GetID returns the task ID
func (p *PendingTask) GetID() string {
	return p.TaskID
}

// GetOutputPath returns the output path
func (p *PendingTask) GetOutputPath() string {
	return p.OutputPath
}

// WithCompletionChannel sets a channel to receive the result when task completes
func (p *PendingTask) WithCompletionChannel(ch chan *TaskResult) *PendingTask {
	p.completionChan = ch
	return p
}

// GetCompletionChannel returns the completion channel if set
func (p *PendingTask) GetCompletionChannel() chan *TaskResult {
	return p.completionChan
}

// Dispatch executes the task using the provided dispatcher
func (p *PendingTask) Dispatch(ctx context.Context, dispatcher *Dispatcher) (*TaskResult, error) {
	return dispatcher.Run(ctx, p)
}
