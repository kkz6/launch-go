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
