package taskrunner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDispatcherRunRequiresTask(t *testing.T) {
	d := NewDispatcher(nil, nil)
	for name, pending := range map[string]*PendingTask{
		"nil pending task": nil,
		"nil task payload": NewPendingTask(nil),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := d.Run(context.Background(), pending)
			if err == nil || err.Error() != "task is required" {
				t.Fatalf("Run() error = %v, want task is required", err)
			}
		})
	}
}

func TestDispatcherRunLocalSuccess(t *testing.T) {
	var finished *TaskResult
	task := NewBaseTask(
		WithName("local success"),
		WithScript("printf 'hello\\nworld\\n'"),
		WithTimeoutSeconds(2),
	)
	task.FinishedCallback = func(_ context.Context, result *TaskResult) { finished = result }

	result, err := NewDispatcher(nil, nil).Run(context.Background(), NewPendingTask(task))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 || result.Output != "hello\nworld\n" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if finished != result {
		t.Fatal("expected finished callback to receive the result")
	}
}

func TestDispatcherRunLocalFailure(t *testing.T) {
	var failed *TaskResult
	task := NewBaseTask(WithScript("printf 'failure'; exit 7"), WithTimeoutSeconds(2))
	task.FailedCallback = func(_ context.Context, result *TaskResult) { failed = result }

	result, err := NewDispatcher(nil, nil).Run(context.Background(), NewPendingTask(task))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 7 || !strings.Contains(result.Output, "failure") {
		t.Fatalf("unexpected result: %+v", result)
	}
	if failed != result {
		t.Fatal("expected failed callback to receive the result")
	}
}

func TestDispatcherRunLocalTimeout(t *testing.T) {
	var timedOut *TaskResult
	task := NewBaseTask(WithScript("sleep 1"), WithTimeout(20*time.Millisecond))
	task.TimeoutCallback = func(_ context.Context, result *TaskResult) { timedOut = result }

	result, err := NewDispatcher(nil, nil).Run(context.Background(), NewPendingTask(task))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.TimedOut || result.ExitCode != 124 {
		t.Fatalf("unexpected timeout result: %+v", result)
	}
	if timedOut != result {
		t.Fatal("expected timeout callback to receive the result")
	}
}

func TestCompleteBackgroundTaskUsesTimeoutCallback(t *testing.T) {
	var timedOut bool
	task := NewBaseTask()
	task.TimeoutCallback = func(_ context.Context, result *TaskResult) {
		timedOut = result.TimedOut && errors.Is(result.Error, context.DeadlineExceeded)
	}
	pending := NewPendingTask(task).WithCompletionChannel(make(chan *TaskResult, 1))
	result := &TaskResult{TaskID: pending.TaskID, TimedOut: true, Error: context.DeadlineExceeded}

	NewDispatcher(nil, nil).completeBackgroundTask(context.Background(), pending, result)
	if !timedOut {
		t.Fatal("expected timeout callback to receive the timeout result")
	}
	if got := <-pending.GetCompletionChannel(); got != result {
		t.Fatal("expected completion channel to receive the result")
	}
}
