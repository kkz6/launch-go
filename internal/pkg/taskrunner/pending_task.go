package taskrunner

import "context"

// PendingTask wraps a task with execution options
type PendingTask struct {
	Task       Task
	Connection *Connection
	Background bool
	TaskID     string
	OutputPath string // Local path to write output

	// Output callback for streaming
	onOutput func(output string)
}

// NewPendingTask creates a new PendingTask wrapper
func NewPendingTask(task Task) *PendingTask {
	return &PendingTask{
		Task: task,
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

// Dispatch executes the task using the provided dispatcher
func (p *PendingTask) Dispatch(ctx context.Context, dispatcher *Dispatcher) (*TaskResult, error) {
	return dispatcher.Run(ctx, p)
}
