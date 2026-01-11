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
	Template() string
	Timeout() time.Duration
	Data() map[string]interface{}
	Script() (string, error)
	OnOutput(output string)
	OnFinished(ctx context.Context, result *TaskResult)
	OnFailed(ctx context.Context, result *TaskResult)
	OnTimeout(ctx context.Context, result *TaskResult)
}

// BaseTask provides common task functionality
type BaseTask struct {
	TaskName     string
	TaskTimeout  time.Duration
	TemplateName string
	TemplateData map[string]interface{}
	CallbackURL  string

	// Callbacks with context support
	OutputCallback   func(output string)
	FinishedCallback func(ctx context.Context, result *TaskResult)
	FailedCallback   func(ctx context.Context, result *TaskResult)
	TimeoutCallback  func(ctx context.Context, result *TaskResult)
}

func (t *BaseTask) Name() string {
	return t.TaskName
}

func (t *BaseTask) Template() string {
	return t.TemplateName
}

func (t *BaseTask) Timeout() time.Duration {
	if t.TaskTimeout == 0 {
		return 10 * time.Minute
	}
	return t.TaskTimeout
}

func (t *BaseTask) Script() (string, error) {
	tmpl, err := template.New("script").Parse(t.TemplateName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, t.TemplateData); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (t *BaseTask) Data() map[string]interface{} {
	return t.TemplateData
}

func (t *BaseTask) GetCallbackURL() string {
	return t.CallbackURL
}

func (t *BaseTask) OnOutput(output string) {
	if t.OutputCallback != nil {
		t.OutputCallback(output)
	}
}

func (t *BaseTask) OnFinished(ctx context.Context, result *TaskResult) {
	if t.FinishedCallback != nil {
		t.FinishedCallback(ctx, result)
	}
}

func (t *BaseTask) OnFailed(ctx context.Context, result *TaskResult) {
	if t.FailedCallback != nil {
		t.FailedCallback(ctx, result)
	}
}

func (t *BaseTask) OnTimeout(ctx context.Context, result *TaskResult) {
	if t.TimeoutCallback != nil {
		t.TimeoutCallback(ctx, result)
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

// Server resource calculation helpers

// SwapInMegabytes calculates swap size based on server memory
func SwapInMegabytes(memoryMB int) int {
	switch {
	case memoryMB <= 2048:
		return 1024
	case memoryMB <= 4096:
		return 2048
	case memoryMB <= 8192:
		return 3072
	default:
		return 4096
	}
}

// Swappiness calculates swappiness based on server memory
func Swappiness(memoryMB int) int {
	switch {
	case memoryMB <= 1024:
		return 20
	case memoryMB <= 2048:
		return 35
	case memoryMB <= 4096:
		return 50
	default:
		return 60
	}
}

// MySQLMaxConnections calculates max connections based on server memory
func MySQLMaxConnections(memoryMB int) int {
	switch {
	case memoryMB <= 1024:
		return 100
	case memoryMB <= 2048:
		return 200
	case memoryMB <= 4096:
		return 400
	default:
		return 500
	}
}

// MaxChildrenPhpPool calculates PHP-FPM max children based on server memory
func MaxChildrenPhpPool(memoryMB int) int {
	gigabytes := maxInt(1, memoryMB/1024-1)
	return int(float64(gigabytes) * 5 * 0.9)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
