package taskrunner

import (
	"context"
	"sync"
	"time"
)

// FakeExecution captures details of a task execution for testing.
type FakeExecution struct {
	Task       Task
	Connection *Connection
	Background bool
	TaskID     string
	Script     string
	AsUser     string
}

// FakeDispatcher is a mock dispatcher for testing that captures task executions
// without actually running SSH commands.
type FakeDispatcher struct {
	mu            sync.Mutex
	Executions    []*FakeExecution
	Results       map[string]*TaskResult
	DefaultResult *TaskResult
	FailOnRun     error
}

// NewFakeDispatcher creates a new FakeDispatcher with default successful result.
func NewFakeDispatcher() *FakeDispatcher {
	return &FakeDispatcher{
		Executions: make([]*FakeExecution, 0),
		Results:    make(map[string]*TaskResult),
		DefaultResult: &TaskResult{
			ExitCode:   0,
			Output:     "Fake execution successful",
			Duration:   100 * time.Millisecond,
			FinishedAt: time.Now(),
		},
	}
}

// Run captures the task execution and returns a fake result.
func (d *FakeDispatcher) Run(ctx context.Context, pt *PendingTask) (*TaskResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.FailOnRun != nil {
		return nil, d.FailOnRun
	}

	user := ""
	if pt.Connection != nil {
		user = pt.Connection.User
	}

	execution := &FakeExecution{
		Task:       pt.Task,
		Connection: pt.Connection,
		Background: pt.Background,
		TaskID:     pt.TaskID,
		Script:     pt.Task.Script(),
		AsUser:     user,
	}
	d.Executions = append(d.Executions, execution)

	// Return specific result if configured, otherwise default
	result := d.DefaultResult
	if pt.TaskID != "" {
		if r, ok := d.Results[pt.TaskID]; ok {
			result = r
		}
	}
	if r, ok := d.Results[pt.Task.Name()]; ok {
		result = r
	}

	// Copy result to avoid shared state
	taskResult := &TaskResult{
		TaskID:     pt.TaskID,
		Output:     result.Output,
		ExitCode:   result.ExitCode,
		Duration:   result.Duration,
		FinishedAt: time.Now(),
		TimedOut:   result.TimedOut,
		Error:      result.Error,
	}

	// Call appropriate callback
	if taskResult.TimedOut {
		pt.Task.OnTimeout(ctx, taskResult)
	} else if taskResult.ExitCode == 0 && taskResult.Error == nil {
		pt.Task.OnFinished(ctx, taskResult)
	} else {
		pt.Task.OnFailed(ctx, taskResult)
	}

	return taskResult, nil
}

// RunWithStreaming delegates to Run for testing purposes.
func (d *FakeDispatcher) RunWithStreaming(ctx context.Context, pt *PendingTask) (*TaskResult, error) {
	return d.Run(ctx, pt)
}

// SetResult configures a specific result for a task by name or ID.
func (d *FakeDispatcher) SetResult(nameOrID string, result *TaskResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Results[nameOrID] = result
}

// SetFailure configures a task to fail with a specific exit code and output.
func (d *FakeDispatcher) SetFailure(nameOrID string, exitCode int, output string) {
	d.SetResult(nameOrID, &TaskResult{
		ExitCode:   exitCode,
		Output:     output,
		Duration:   100 * time.Millisecond,
		FinishedAt: time.Now(),
	})
}

// SetTimeout configures a task to config.
func (d *FakeDispatcher) SetTimeout(nameOrID string) {
	d.SetResult(nameOrID, &TaskResult{
		ExitCode:   124,
		Output:     "Task timed out",
		Duration:   30 * time.Second,
		TimedOut:   true,
		FinishedAt: time.Now(),
	})
}

// SetDefaultFailure makes all tasks fail by default.
func (d *FakeDispatcher) SetDefaultFailure(exitCode int, output string) {
	d.DefaultResult = &TaskResult{
		ExitCode:   exitCode,
		Output:     output,
		Duration:   100 * time.Millisecond,
		FinishedAt: time.Now(),
	}
}

// SetRunError makes Run() return an error (connection failure, etc.)
func (d *FakeDispatcher) SetRunError(err error) {
	d.FailOnRun = err
}

// Reset clears all recorded executions and results.
func (d *FakeDispatcher) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Executions = make([]*FakeExecution, 0)
	d.Results = make(map[string]*TaskResult)
	d.FailOnRun = nil
	d.DefaultResult = &TaskResult{
		ExitCode:   0,
		Output:     "Fake execution successful",
		Duration:   100 * time.Millisecond,
		FinishedAt: time.Now(),
	}
}

// ExecutionCount returns the number of tasks that were executed.
func (d *FakeDispatcher) ExecutionCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.Executions)
}

// LastExecution returns the most recent execution, or nil if none.
func (d *FakeDispatcher) LastExecution() *FakeExecution {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.Executions) == 0 {
		return nil
	}
	return d.Executions[len(d.Executions)-1]
}

// GetExecution returns the execution at the given index.
func (d *FakeDispatcher) GetExecution(index int) *FakeExecution {
	d.mu.Lock()
	defer d.mu.Unlock()
	if index < 0 || index >= len(d.Executions) {
		return nil
	}
	return d.Executions[index]
}

// FindExecution finds an execution by task name.
func (d *FakeDispatcher) FindExecution(taskName string) *FakeExecution {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, e := range d.Executions {
		if e.Task.Name() == taskName {
			return e
		}
	}
	return nil
}

// AllScripts returns all executed scripts.
func (d *FakeDispatcher) AllScripts() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	scripts := make([]string, len(d.Executions))
	for i, e := range d.Executions {
		scripts[i] = e.Script
	}
	return scripts
}
