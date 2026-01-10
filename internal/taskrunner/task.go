package taskrunner

import (
	"bytes"
	"context"
	"text/template"
	"time"
)

// Task represents a runnable task with a script template
type Task interface {
	Name() string
	Timeout() time.Duration
	Script() (string, error)
	OnOutput(output string)
	OnFinished(result *TaskResult)
	OnFailed(result *TaskResult)
	OnTimeout(result *TaskResult)
}

// BaseTask provides common task functionality
type BaseTask struct {
	TaskName    string
	TaskTimeout time.Duration
	Template    string
	Data        map[string]interface{}

	// Callbacks
	OutputCallback   func(output string)
	FinishedCallback func(result *TaskResult)
	FailedCallback   func(result *TaskResult)
	TimeoutCallback  func(result *TaskResult)
}

func (t *BaseTask) Name() string {
	return t.TaskName
}

func (t *BaseTask) Timeout() time.Duration {
	if t.TaskTimeout == 0 {
		return 10 * time.Minute
	}
	return t.TaskTimeout
}

func (t *BaseTask) Script() (string, error) {
	tmpl, err := template.New("script").Parse(t.Template)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, t.Data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (t *BaseTask) OnOutput(output string) {
	if t.OutputCallback != nil {
		t.OutputCallback(output)
	}
}

func (t *BaseTask) OnFinished(result *TaskResult) {
	if t.FinishedCallback != nil {
		t.FinishedCallback(result)
	}
}

func (t *BaseTask) OnFailed(result *TaskResult) {
	if t.FailedCallback != nil {
		t.FailedCallback(result)
	}
}

func (t *BaseTask) OnTimeout(result *TaskResult) {
	if t.TimeoutCallback != nil {
		t.TimeoutCallback(result)
	}
}

// TaskResult contains the execution result
type TaskResult struct {
	TaskID     string
	Output     string
	ExitCode   int
	Duration   time.Duration
	TimedOut   bool
	Error      error
	FinishedAt time.Time
}

func (r *TaskResult) IsSuccessful() bool {
	return r.ExitCode == 0 && !r.TimedOut && r.Error == nil
}

// Connection represents SSH connection details
type Connection struct {
	Host       string
	Port       int
	User       string
	PrivateKey string
	ScriptPath string // Remote path for scripts (default: ~/.launch-tasks)
}

func (c *Connection) GetScriptPath() string {
	if c.ScriptPath == "" {
		if c.User == "root" {
			return "/root/.launch-tasks"
		}
		return "/home/" + c.User + "/.launch-tasks"
	}
	return c.ScriptPath
}

// PendingTask wraps a task with execution options
type PendingTask struct {
	Task       Task
	Connection *Connection
	Background bool
	TaskID     string
	OutputPath string // Local path to write output
}

func NewPendingTask(task Task) *PendingTask {
	return &PendingTask{
		Task: task,
	}
}

func (p *PendingTask) OnConnection(conn *Connection) *PendingTask {
	p.Connection = conn
	return p
}

func (p *PendingTask) InBackground() *PendingTask {
	p.Background = true
	return p
}

func (p *PendingTask) WithID(id string) *PendingTask {
	p.TaskID = id
	return p
}

func (p *PendingTask) WriteOutputTo(path string) *PendingTask {
	p.OutputPath = path
	return p
}

func (p *PendingTask) Dispatch(ctx context.Context, dispatcher *Dispatcher) (*TaskResult, error) {
	return dispatcher.Run(ctx, p)
}
