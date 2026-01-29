package taskrunner

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// testTask is a minimal Task implementation for dispatcher tests
type testTask struct {
	*BaseTask
}

func newTestTask(name string) *testTask {
	return &testTask{
		BaseTask: NewBaseTask(
			WithName(name),
			WithScript("echo hello"),
			WithTimeoutSeconds(30),
		),
	}
}

func TestFakeDispatcher_Run(t *testing.T) {
	d := NewFakeDispatcher()
	task := newTestTask("Test Task")

	pt := NewPendingTask(task)
	pt.As("task-123")

	result, err := d.Run(context.Background(), pt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TaskID != "task-123" {
		t.Errorf("expected TaskID 'task-123', got '%s'", result.TaskID)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if d.ExecutionCount() != 1 {
		t.Errorf("expected 1 execution, got %d", d.ExecutionCount())
	}
}

func TestFakeDispatcher_SetFailure(t *testing.T) {
	d := NewFakeDispatcher()
	d.SetFailure("failing", 127, "command not found")

	task := newTestTask("failing")
	pt := NewPendingTask(task)
	pt.As("task-fail")

	result, err := d.Run(context.Background(), pt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 127 {
		t.Errorf("expected exit code 127, got %d", result.ExitCode)
	}
}

func TestFakeDispatcher_SetRunError(t *testing.T) {
	d := NewFakeDispatcher()
	d.SetRunError(fmt.Errorf("SSH connection failed"))

	task := newTestTask("Test Task")
	pt := NewPendingTask(task)

	_, err := d.Run(context.Background(), pt)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "SSH connection failed" {
		t.Errorf("expected 'SSH connection failed', got '%s'", err.Error())
	}
}

// TestFakeDispatcher_CompletionChannel verifies that FakeDispatcher sends results
// to the completion channel when set. This prevents goroutine leaks when
// RunInBackground blocks on <-completionChan.
func TestFakeDispatcher_CompletionChannel(t *testing.T) {
	d := NewFakeDispatcher()
	task := newTestTask("Background Task")

	ch := make(chan *TaskResult, 1)
	pt := NewPendingTask(task)
	pt.InBackground()
	pt.As("bg-task-1")
	pt.WithCompletionChannel(ch)

	result, err := d.Run(context.Background(), pt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from FakeDispatcher")
	}

	// Completion channel should have received the result
	select {
	case chResult := <-ch:
		if chResult == nil {
			t.Fatal("expected non-nil result from completion channel")
		}
		if chResult.TaskID != "bg-task-1" {
			t.Errorf("expected TaskID 'bg-task-1', got '%s'", chResult.TaskID)
		}
		if chResult.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", chResult.ExitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("completion channel timed out — would cause goroutine leak in RunInBackground")
	}
}

// TestFakeDispatcher_CompletionChannel_Failure verifies completion channel
// receives failure results so RunInBackground doesn't block forever.
func TestFakeDispatcher_CompletionChannel_Failure(t *testing.T) {
	d := NewFakeDispatcher()
	d.SetFailure("Failing Task", 1, "script failed")

	task := newTestTask("Failing Task")

	ch := make(chan *TaskResult, 1)
	pt := NewPendingTask(task)
	pt.InBackground()
	pt.As("bg-fail-1")
	pt.WithCompletionChannel(ch)

	_, err := d.Run(context.Background(), pt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case chResult := <-ch:
		if chResult.ExitCode != 1 {
			t.Errorf("expected exit code 1, got %d", chResult.ExitCode)
		}
		if chResult.IsSuccessful() {
			t.Error("expected failure result in completion channel")
		}
	case <-time.After(time.Second):
		t.Fatal("completion channel timed out — would cause goroutine leak")
	}
}

// TestFakeDispatcher_CompletionChannel_Timeout verifies completion channel
// receives timeout results.
func TestFakeDispatcher_CompletionChannel_Timeout(t *testing.T) {
	d := NewFakeDispatcher()
	d.SetTimeout("Timeout Task")

	task := newTestTask("Timeout Task")

	ch := make(chan *TaskResult, 1)
	pt := NewPendingTask(task)
	pt.InBackground()
	pt.As("bg-timeout-1")
	pt.WithCompletionChannel(ch)

	_, err := d.Run(context.Background(), pt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case chResult := <-ch:
		if !chResult.TimedOut {
			t.Error("expected TimedOut to be true")
		}
		if chResult.ExitCode != 124 {
			t.Errorf("expected exit code 124, got %d", chResult.ExitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("completion channel timed out — would cause goroutine leak")
	}
}

// TestFakeDispatcher_NoCompletionChannel verifies that FakeDispatcher works
// normally when no completion channel is set (foreground tasks).
func TestFakeDispatcher_NoCompletionChannel(t *testing.T) {
	d := NewFakeDispatcher()
	task := newTestTask("Foreground Task")

	pt := NewPendingTask(task)
	pt.As("fg-task-1")

	result, err := d.Run(context.Background(), pt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}
