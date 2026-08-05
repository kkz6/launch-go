package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type builderCallback struct {
	SiteID string `json:"site_id"`
}

type unmarshalableBuilderCallback struct {
	Channel chan struct{} `json:"channel"`
}

func TestBaseTaskCompleteBehavior(t *testing.T) {
	task := NewBaseTask()
	if task.Timeout() != 10*time.Minute {
		t.Fatalf("default Timeout() = %v", task.Timeout())
	}

	task.SetName("renamed")
	task.SetScript("echo changed")
	task.SetTimeout(45 * time.Second)
	if task.Name() != "renamed" || task.Script() != "echo changed" || task.Timeout() != 45*time.Second {
		t.Fatalf("task after setters = (%q, %q, %v)", task.Name(), task.Script(), task.Timeout())
	}

	var output string
	task.OutputCallback = func(value string) { output = value }
	task.OnOutput("line")
	if output != "line" {
		t.Fatalf("output callback received %q", output)
	}

	result := &TaskResult{TaskID: "task-1"}
	ctx := context.Background()
	var finished, failed, timedOut bool
	task.FinishedCallback = func(gotCtx context.Context, got *TaskResult) {
		finished = gotCtx == ctx && got == result
	}
	task.FailedCallback = func(gotCtx context.Context, got *TaskResult) {
		failed = gotCtx == ctx && got == result
	}
	task.TimeoutCallback = func(gotCtx context.Context, got *TaskResult) {
		timedOut = gotCtx == ctx && got == result
	}
	task.OnFinished(ctx, result)
	task.OnFailed(ctx, result)
	task.OnTimeout(ctx, result)
	if !finished || !failed || !timedOut {
		t.Fatalf("callbacks = finished:%v failed:%v timeout:%v", finished, failed, timedOut)
	}

	// Nil callbacks are deliberately supported and must remain no-ops.
	empty := &BaseTask{}
	empty.OnOutput("ignored")
	empty.OnFinished(ctx, result)
	empty.OnFailed(ctx, result)
	empty.OnTimeout(ctx, result)
	if empty.Timeout() != 10*time.Minute {
		t.Fatalf("zero BaseTask Timeout() = %v", empty.Timeout())
	}

	pending := task.Pending()
	if pending.Task != task || pending.GetID() == "" {
		t.Fatalf("Pending() = %#v", pending)
	}
}

// The script-preamble helpers this used to cover (WrapScript, ShellDefaults,
// CommonFunctions, AptFunctions) were unreferenced duplicates of the ones in
// taskrunner/templates and have been removed; templates owns them now.
func TestNewScriptTask(t *testing.T) {
	task := NewScriptTask("script task", "echo hello", 12*time.Second)
	if task.Name() != "script task" || task.Script() != "echo hello" || task.Timeout() != 12*time.Second {
		t.Fatalf("NewScriptTask() = (%q, %q, %v)", task.Name(), task.Script(), task.Timeout())
	}
}

func TestTaskBuilderCompleteBehavior(t *testing.T) {
	ctx := context.Background()
	cbCtx := &CallbackContext{}
	callback := builderCallback{SiteID: "site-1"}

	var successTaskID, failureTaskID, expiredTaskID string
	var failureExitCode int
	builder := NewTaskBuilder[builderCallback]("site:builder").
		WithName("Builder task").
		WithScript("echo builder").
		WithTimeout(30 * time.Second).
		WithTimeoutSeconds(45).
		WithCallback(callback).
		OnSuccess(func(_ context.Context, gotCtx *CallbackContext, got builderCallback, taskID string) error {
			if gotCtx != cbCtx || got != callback {
				return errors.New("unexpected success arguments")
			}
			successTaskID = taskID
			return nil
		}).
		OnFailure(func(_ context.Context, gotCtx *CallbackContext, got builderCallback, taskID string, exitCode int) error {
			if gotCtx != cbCtx || got != callback {
				return errors.New("unexpected failure arguments")
			}
			failureTaskID = taskID
			failureExitCode = exitCode
			return nil
		}).
		OnExpired(func(_ context.Context, gotCtx *CallbackContext, got builderCallback, taskID string) error {
			if gotCtx != cbCtx || got != callback {
				return errors.New("unexpected expiration arguments")
			}
			expiredTaskID = taskID
			return nil
		})

	task := builder.Build()
	if task.Name() != "Builder task" || task.Script() != "echo builder" || task.Timeout() != 45*time.Second {
		t.Fatalf("built task = (%q, %q, %v)", task.Name(), task.Script(), task.Timeout())
	}
	if task.TypeName() != "site:builder" || task.GetCallback() != callback {
		t.Fatalf("built callback identity = (%q, %#v)", task.TypeName(), task.GetCallback())
	}
	payload, err := task.MarshalPayload()
	if err != nil || string(payload) != `{"site_id":"site-1"}` {
		t.Fatalf("MarshalPayload() = (%s, %v)", payload, err)
	}
	if err := task.OnSuccess(ctx, cbCtx, "success-1"); err != nil {
		t.Fatalf("OnSuccess() error = %v", err)
	}
	if err := task.OnFailure(ctx, cbCtx, "failure-1", 17); err != nil {
		t.Fatalf("OnFailure() error = %v", err)
	}
	if err := task.OnExpired(ctx, cbCtx, "expired-1"); err != nil {
		t.Fatalf("OnExpired() error = %v", err)
	}
	if successTaskID != "success-1" || failureTaskID != "failure-1" || failureExitCode != 17 || expiredTaskID != "expired-1" {
		t.Fatalf("handler observations = (%q, %q, %d, %q)", successTaskID, failureTaskID, failureExitCode, expiredTaskID)
	}

	empty := NewTaskBuilder[builderCallback]("site:empty").Build()
	if err := empty.OnSuccess(ctx, cbCtx, "task"); err != nil {
		t.Fatalf("nil OnSuccess handler error = %v", err)
	}
	if err := empty.OnFailure(ctx, cbCtx, "task", 1); err != nil {
		t.Fatalf("nil OnFailure handler error = %v", err)
	}
	if err := empty.OnExpired(ctx, cbCtx, "task"); err != nil {
		t.Fatalf("nil OnExpired handler error = %v", err)
	}

	invalid := NewTaskBuilder[unmarshalableBuilderCallback]("site:invalid").
		WithCallback(unmarshalableBuilderCallback{Channel: make(chan struct{})}).
		Build()
	if _, err := invalid.MarshalPayload(); err == nil {
		t.Fatal("MarshalPayload() accepted an unsupported callback field")
	}
}

func TestRegisterBuiltTask(t *testing.T) {
	original := DefaultRegistry
	DefaultRegistry = NewTaskTypeRegistry()
	t.Cleanup(func() { DefaultRegistry = original })

	observed := make([]string, 0, 3)
	RegisterBuiltTask[builderCallback](
		"site:registered-builder",
		func(_ context.Context, _ *CallbackContext, callback builderCallback, taskID string) error {
			observed = append(observed, "success:"+callback.SiteID+":"+taskID)
			return nil
		},
		func(_ context.Context, _ *CallbackContext, callback builderCallback, taskID string, exitCode int) error {
			observed = append(observed, fmt.Sprintf("failure:%s:%s:%d", callback.SiteID, taskID, exitCode))
			return nil
		},
		func(_ context.Context, _ *CallbackContext, callback builderCallback, taskID string) error {
			observed = append(observed, "expired:"+callback.SiteID+":"+taskID)
			return nil
		},
	)

	if _, err := Reconstruct("site:registered-builder", []byte("{")); err == nil {
		t.Fatal("registered BuiltTask accepted invalid JSON")
	}
	handler, err := Reconstruct("site:registered-builder", []byte(`{"site_id":"site-2"}`))
	if err != nil {
		t.Fatalf("Reconstruct() error = %v", err)
	}
	task, ok := handler.(*BuiltTask[builderCallback])
	if !ok {
		t.Fatalf("handler type = %T", handler)
	}
	if task.TypeName() != "site:registered-builder" || task.GetCallback().SiteID != "site-2" {
		t.Fatalf("reconstructed task = %#v", task)
	}
	ctx := context.Background()
	cbCtx := &CallbackContext{}
	if err := task.OnSuccess(ctx, cbCtx, "task-1"); err != nil {
		t.Fatalf("OnSuccess() error = %v", err)
	}
	if err := task.OnFailure(ctx, cbCtx, "task-2", 9); err != nil {
		t.Fatalf("OnFailure() error = %v", err)
	}
	if err := task.OnExpired(ctx, cbCtx, "task-3"); err != nil {
		t.Fatalf("OnExpired() error = %v", err)
	}
	want := []string{"success:site-2:task-1", "failure:site-2:task-2:9", "expired:site-2:task-3"}
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("handler calls = %v, want %v", observed, want)
	}
}

func TestFakeDispatcherInspectionAndReset(t *testing.T) {
	dispatcher := NewFakeDispatcher()
	if dispatcher.LastExecution() != nil {
		t.Fatal("LastExecution() on a new dispatcher should be nil")
	}
	if dispatcher.GetExecution(-1) != nil || dispatcher.GetExecution(0) != nil {
		t.Fatal("GetExecution() accepted an out-of-range index")
	}
	if dispatcher.FindExecution("missing") != nil {
		t.Fatal("FindExecution() found a missing task")
	}
	if scripts := dispatcher.AllScripts(); len(scripts) != 0 {
		t.Fatalf("AllScripts() = %v, want empty", scripts)
	}

	defaultTask := newTestTask("default failure")
	dispatcher.SetDefaultFailure(23, "default failed")
	result, err := dispatcher.Run(context.Background(), NewPendingTask(defaultTask).WithID("default-id"))
	if err != nil || result.ExitCode != 23 || result.Output != "default failed" {
		t.Fatalf("default failure result = (%#v, %v)", result, err)
	}

	connection := &Connection{User: "launcher"}
	selectedTask := newTestTask("named task")
	dispatcher.SetResult("selected-id", &TaskResult{ExitCode: 11, Output: "id result"})
	dispatcher.SetResult("named task", &TaskResult{ExitCode: 0, Output: "name result"})
	result, err = dispatcher.Run(
		context.Background(),
		NewPendingTask(selectedTask).WithID("selected-id").OnConnection(connection).InBackground(),
	)
	if err != nil || result.ExitCode != 0 || result.Output != "name result" {
		t.Fatalf("named result override = (%#v, %v)", result, err)
	}

	last := dispatcher.LastExecution()
	if last == nil || last.TaskID != "selected-id" || last.AsUser != "launcher" || !last.Background {
		t.Fatalf("LastExecution() = %#v", last)
	}
	if first := dispatcher.GetExecution(0); first == nil || first.Task.Name() != "default failure" {
		t.Fatalf("GetExecution(0) = %#v", first)
	}
	if dispatcher.GetExecution(2) != nil {
		t.Fatal("GetExecution(len) should return nil")
	}
	if found := dispatcher.FindExecution("named task"); found != last {
		t.Fatalf("FindExecution() = %#v, want %#v", found, last)
	}
	if scripts := dispatcher.AllScripts(); !reflect.DeepEqual(scripts, []string{"echo hello", "echo hello"}) {
		t.Fatalf("AllScripts() = %v", scripts)
	}

	dispatcher.SetRunError(errors.New("temporary failure"))
	dispatcher.Reset()
	if dispatcher.ExecutionCount() != 0 || dispatcher.LastExecution() != nil || len(dispatcher.Results) != 0 || dispatcher.FailOnRun != nil {
		t.Fatalf("dispatcher after Reset() = %#v", dispatcher)
	}
	result, err = dispatcher.Run(context.Background(), NewPendingTask(newTestTask("after reset")))
	if err != nil || result.ExitCode != 0 || result.Output != "Fake execution successful" {
		t.Fatalf("reset default result = (%#v, %v)", result, err)
	}
}

func TestFakeDispatcherRoutesErrorResultToFailure(t *testing.T) {
	dispatcher := NewFakeDispatcher()
	task := newTestTask("error result")
	var failed bool
	task.FailedCallback = func(_ context.Context, result *TaskResult) {
		failed = result.Error != nil
	}
	dispatcher.SetResult("error result", &TaskResult{Error: errors.New("transport lost")})
	result, err := dispatcher.Run(context.Background(), NewPendingTask(task))
	if err != nil || result.Error == nil || !failed {
		t.Fatalf("error result = (%#v, %v), failed callback = %v", result, err, failed)
	}
}

func TestPendingTaskAccessorsAndDispatch(t *testing.T) {
	task := NewBaseTask(WithName("pending dispatch"), WithScript("printf pending"), WithTimeoutSeconds(5))
	pending := NewPendingTask(task).WithID("pending-id").WriteOutputTo("/tmp/pending.log")
	if pending.GetID() != "pending-id" || pending.GetOutputPath() != "/tmp/pending.log" {
		t.Fatalf("pending accessors = (%q, %q)", pending.GetID(), pending.GetOutputPath())
	}

	dispatcher := NewDispatcher(nil, nil)
	result, err := pending.Dispatch(context.Background(), dispatcher)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result == nil || !result.IsSuccessful() || !strings.Contains(result.Output, "pending") {
		t.Fatalf("Dispatch() result = %#v", result)
	}
}

func TestTaskResultTimedOut(t *testing.T) {
	if (&TaskResult{}).IsTimedOut() {
		t.Fatal("zero TaskResult should not be timed out")
	}
	if !(&TaskResult{TimedOut: true}).IsTimedOut() {
		t.Fatal("timed-out TaskResult was not recognized")
	}
}
