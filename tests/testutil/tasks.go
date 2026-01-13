package testutil

import (
	"strings"
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TaskAssertion provides fluent assertions for task testing.
type TaskAssertion struct {
	t      *testing.T
	task   taskrunner.Task
	script string
}

// AssertTask creates a new task assertion helper.
func AssertTask(t *testing.T, task taskrunner.Task) *TaskAssertion {
	t.Helper()
	return &TaskAssertion{
		t:      t,
		task:   task,
		script: task.Script(),
	}
}

// HasName asserts the task has the expected name.
func (a *TaskAssertion) HasName(expected string) *TaskAssertion {
	a.t.Helper()
	if a.task.Name() != expected {
		a.t.Errorf("Expected task name %q, got %q", expected, a.task.Name())
	}
	return a
}

// ScriptContains asserts the script contains the expected string.
func (a *TaskAssertion) ScriptContains(expected string) *TaskAssertion {
	a.t.Helper()
	if !strings.Contains(a.script, expected) {
		a.t.Errorf("Expected script to contain %q\n\nScript:\n%s", expected, a.script)
	}
	return a
}

// ScriptNotContains asserts the script does not contain the expected string.
func (a *TaskAssertion) ScriptNotContains(unexpected string) *TaskAssertion {
	a.t.Helper()
	if strings.Contains(a.script, unexpected) {
		a.t.Errorf("Expected script NOT to contain %q\n\nScript:\n%s", unexpected, a.script)
	}
	return a
}

// ScriptStartsWith asserts the script starts with the expected string.
func (a *TaskAssertion) ScriptStartsWith(expected string) *TaskAssertion {
	a.t.Helper()
	if !strings.HasPrefix(a.script, expected) {
		a.t.Errorf("Expected script to start with %q\n\nScript:\n%s", expected, a.script)
	}
	return a
}

// ScriptContainsAll asserts the script contains all expected strings.
func (a *TaskAssertion) ScriptContainsAll(expected ...string) *TaskAssertion {
	a.t.Helper()
	for _, exp := range expected {
		if !strings.Contains(a.script, exp) {
			a.t.Errorf("Expected script to contain %q\n\nScript:\n%s", exp, a.script)
		}
	}
	return a
}

// ScriptContainsAny asserts the script contains at least one of the expected strings.
func (a *TaskAssertion) ScriptContainsAny(expected ...string) *TaskAssertion {
	a.t.Helper()
	for _, exp := range expected {
		if strings.Contains(a.script, exp) {
			return a
		}
	}
	a.t.Errorf("Expected script to contain at least one of %v\n\nScript:\n%s", expected, a.script)
	return a
}

// ScriptMatches asserts the script matches the snapshot.
func (a *TaskAssertion) ScriptMatches(snapshotName string) *TaskAssertion {
	a.t.Helper()
	MatchSnapshot(a.t, snapshotName, a.script)
	return a
}

// GetScript returns the task script for custom assertions.
func (a *TaskAssertion) GetScript() string {
	return a.script
}

// ExecutionAssertion provides fluent assertions for fake executions.
type ExecutionAssertion struct {
	t   *testing.T
	exe *taskrunner.FakeExecution
}

// AssertExecution creates a new execution assertion helper.
func AssertExecution(t *testing.T, exe *taskrunner.FakeExecution) *ExecutionAssertion {
	t.Helper()
	if exe == nil {
		t.Fatal("Expected execution to exist, got nil")
	}
	return &ExecutionAssertion{t: t, exe: exe}
}

// HasTaskName asserts the execution has the expected task name.
func (a *ExecutionAssertion) HasTaskName(expected string) *ExecutionAssertion {
	a.t.Helper()
	if a.exe.Task.Name() != expected {
		a.t.Errorf("Expected task name %q, got %q", expected, a.exe.Task.Name())
	}
	return a
}

// WasBackground asserts the task was run in background mode.
func (a *ExecutionAssertion) WasBackground() *ExecutionAssertion {
	a.t.Helper()
	if !a.exe.Background {
		a.t.Error("Expected task to run in background mode")
	}
	return a
}

// WasForeground asserts the task was run in foreground mode.
func (a *ExecutionAssertion) WasForeground() *ExecutionAssertion {
	a.t.Helper()
	if a.exe.Background {
		a.t.Error("Expected task to run in foreground mode")
	}
	return a
}

// AsUser asserts the task was run as the expected user.
func (a *ExecutionAssertion) AsUser(expected string) *ExecutionAssertion {
	a.t.Helper()
	if a.exe.AsUser != expected {
		a.t.Errorf("Expected task to run as user %q, got %q", expected, a.exe.AsUser)
	}
	return a
}

// OnHost asserts the task was run on the expected host.
func (a *ExecutionAssertion) OnHost(expected string) *ExecutionAssertion {
	a.t.Helper()
	if a.exe.Connection == nil {
		a.t.Error("Expected task to have a connection")
		return a
	}
	if a.exe.Connection.Host != expected {
		a.t.Errorf("Expected task to run on host %q, got %q", expected, a.exe.Connection.Host)
	}
	return a
}

// ScriptContains asserts the execution script contains the expected string.
func (a *ExecutionAssertion) ScriptContains(expected string) *ExecutionAssertion {
	a.t.Helper()
	if !strings.Contains(a.exe.Script, expected) {
		a.t.Errorf("Expected script to contain %q\n\nScript:\n%s", expected, a.exe.Script)
	}
	return a
}

// DispatcherAssertion provides fluent assertions for the fake dispatcher.
type DispatcherAssertion struct {
	t *testing.T
	d *taskrunner.FakeDispatcher
}

// AssertDispatcher creates a new dispatcher assertion helper.
func AssertDispatcher(t *testing.T, d *taskrunner.FakeDispatcher) *DispatcherAssertion {
	t.Helper()
	return &DispatcherAssertion{t: t, d: d}
}

// ExecutedCount asserts the number of tasks executed.
func (a *DispatcherAssertion) ExecutedCount(expected int) *DispatcherAssertion {
	a.t.Helper()
	if a.d.ExecutionCount() != expected {
		a.t.Errorf("Expected %d executions, got %d", expected, a.d.ExecutionCount())
	}
	return a
}

// ExecutedTask asserts a task with the given name was executed.
func (a *DispatcherAssertion) ExecutedTask(taskName string) *ExecutionAssertion {
	a.t.Helper()
	exe := a.d.FindExecution(taskName)
	if exe == nil {
		a.t.Errorf("Expected task %q to be executed", taskName)
		return &ExecutionAssertion{t: a.t, exe: &taskrunner.FakeExecution{}}
	}
	return &ExecutionAssertion{t: a.t, exe: exe}
}

// NothingExecuted asserts no tasks were executed.
func (a *DispatcherAssertion) NothingExecuted() *DispatcherAssertion {
	a.t.Helper()
	if a.d.ExecutionCount() != 0 {
		a.t.Errorf("Expected no executions, got %d", a.d.ExecutionCount())
	}
	return a
}
