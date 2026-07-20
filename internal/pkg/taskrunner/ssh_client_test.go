package taskrunner

import (
	"context"
	"errors"
	"testing"
)

func TestShellQuote(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain path", input: "path with spaces", want: "'path with spaces'"},
		{name: "single quote", input: "it's safe", want: "'it'\"'\"'s safe'"},
		{name: "shell metacharacter", input: "$(not executed)", want: "'$(not executed)'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuote(tc.input); got != tc.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCompleteBackgroundTaskRoutesFailureToCallback(t *testing.T) {
	var failed bool
	task := NewBaseTask()
	task.FailedCallback = func(_ context.Context, result *TaskResult) {
		failed = errors.Is(result.Error, context.DeadlineExceeded)
	}

	pending := NewPendingTask(task).WithCompletionChannel(make(chan *TaskResult, 1))
	result := &TaskResult{TaskID: pending.TaskID, ExitCode: 1, Error: context.DeadlineExceeded}
	NewDispatcher(nil, nil).completeBackgroundTask(context.Background(), pending, result)

	if !failed {
		t.Fatal("expected the failed callback to receive the monitoring error")
	}
	if got := <-pending.GetCompletionChannel(); got != result {
		t.Fatal("expected the completion channel to receive the task result")
	}
}
