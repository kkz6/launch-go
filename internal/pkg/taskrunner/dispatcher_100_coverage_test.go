package taskrunner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
	"github.com/rs/zerolog"
)

type dispatcherRunResponse struct {
	result *SSHCommandResult
	err    error
}

type dispatcherFakeSSHClient struct {
	runResponses []dispatcherRunResponse
	runCommands  []string
	uploadErr    error
	uploaded     []byte
	uploadPath   string
	uploadMode   os.FileMode
	downloaded   []byte
	downloadErr  error
	closeCount   int
}

func (c *dispatcherFakeSSHClient) Run(_ context.Context, command string) (*SSHCommandResult, error) {
	c.runCommands = append(c.runCommands, command)
	if len(c.runResponses) == 0 {
		return &SSHCommandResult{}, nil
	}
	response := c.runResponses[0]
	c.runResponses = c.runResponses[1:]
	return response.result, response.err
}

func (c *dispatcherFakeSSHClient) Upload(
	_ context.Context,
	content []byte,
	remotePath string,
	mode os.FileMode,
) error {
	c.uploaded = append([]byte(nil), content...)
	c.uploadPath = remotePath
	c.uploadMode = mode
	return c.uploadErr
}

func (c *dispatcherFakeSSHClient) Download(_ context.Context, _ string) ([]byte, error) {
	return append([]byte(nil), c.downloaded...), c.downloadErr
}

func (c *dispatcherFakeSSHClient) Close() error {
	c.closeCount++
	return nil
}

func newDispatcherCoverageHarness(client dispatcherSSHClient) *Dispatcher {
	logger := zerolog.Nop()
	dispatcher := NewDispatcher(&logger, nil)
	dispatcher.dial = func(*Connection) (dispatcherSSHClient, error) {
		return client, nil
	}
	return dispatcher
}

func dispatcherCoverageTask() *BaseTask {
	return NewBaseTask(
		WithName("coverage task"),
		WithScript("printf coverage"),
		WithTimeoutSeconds(2),
	)
}

func TestDispatcher100ConstructorsAndDefaultSeams(t *testing.T) {
	logger := zerolog.Nop()
	taskLogger := zerolog.Nop()

	plain := NewDispatcher(&logger, nil)
	if plain.GetStreamMonitor() == nil || plain.monitorBackgroundTask == nil {
		t.Fatal("plain dispatcher did not initialize its execution dependencies")
	}

	withLogger := NewDispatcherWithTaskLogger(&logger, nil, &taskLogger)
	if withLogger.taskLogger != &taskLogger || withLogger.GetStreamMonitor().taskLogger != &taskLogger {
		t.Fatal("task logger was not propagated to the dispatcher and monitor")
	}

	defaultConfig := NewDispatcherWithConfig(&logger, nil, nil)
	if defaultConfig.GetStreamMonitor().broadcastInterval != 2*time.Second {
		t.Fatalf("default interval = %s, want 2s", defaultConfig.GetStreamMonitor().broadcastInterval)
	}

	configured := NewDispatcherWithConfig(&logger, nil, &DispatcherConfig{
		BroadcastInterval: time.Millisecond,
		TaskLogger:        &taskLogger,
	})
	if configured.GetStreamMonitor().broadcastInterval != time.Millisecond || configured.taskLogger != &taskLogger {
		t.Fatal("explicit dispatcher configuration was not applied")
	}

	withoutMonitor := newConfiguredDispatcher(&logger, nil, nil, nil)
	if withoutMonitor.streamMonitor != nil || withoutMonitor.monitorBackgroundTask != nil {
		t.Fatal("nil stream monitor should remain disabled")
	}

	if _, err := plain.dial(&Connection{}); err == nil || !strings.Contains(err.Error(), "no authentication method") {
		t.Fatalf("default dial seam error = %v, want authentication error", err)
	}
}

func TestDispatcher100LocalFailureSeamsAndCleanup(t *testing.T) {
	t.Run("create temp file", func(t *testing.T) {
		dispatcher := NewDispatcher(nil, nil)
		dispatcher.createTemp = func(string, string) (*os.File, error) {
			return nil, errors.New("temp unavailable")
		}

		result, err := dispatcher.Run(context.Background(), NewPendingTask(dispatcherCoverageTask()))
		if result != nil || err == nil || !strings.Contains(err.Error(), "failed to create temp file") {
			t.Fatalf("Run() = (%+v, %v), want create failure", result, err)
		}
	})

	t.Run("write and cleanup temp file", func(t *testing.T) {
		dispatcher := NewDispatcher(nil, nil)
		var created *os.File
		dispatcher.createTemp = func(dir, pattern string) (*os.File, error) {
			file, err := os.CreateTemp(dir, pattern)
			created = file
			return file, err
		}
		dispatcher.writeTempScript = func(*os.File, string) (int, error) {
			return 0, errors.New("disk full")
		}

		result, err := dispatcher.Run(context.Background(), NewPendingTask(dispatcherCoverageTask()))
		if result != nil || err == nil || !strings.Contains(err.Error(), "failed to write script") {
			t.Fatalf("Run() = (%+v, %v), want write failure", result, err)
		}
		if created == nil {
			t.Fatal("expected a temporary file")
		}
		if _, statErr := os.Stat(created.Name()); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("temporary script still exists: %v", statErr)
		}
		if _, writeErr := created.WriteString("closed"); writeErr == nil {
			t.Fatal("temporary script was not closed on write failure")
		}
	})

	t.Run("command start", func(t *testing.T) {
		dispatcher := NewDispatcher(nil, nil)
		dispatcher.commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "launch-command-that-does-not-exist")
		}

		var failed *TaskResult
		task := dispatcherCoverageTask()
		task.FailedCallback = func(_ context.Context, result *TaskResult) { failed = result }

		result, err := dispatcher.Run(context.Background(), NewPendingTask(task))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if result.ExitCode != 1 || result.Error == nil || failed != result {
			t.Fatalf("unexpected command-start result: %+v", result)
		}
	})
}

func TestDispatcher100RemoteSetupAndDispatchModes(t *testing.T) {
	connection := &Connection{Host: "example.test", User: "launcher", ScriptPath: "/tmp/launch tasks"}

	t.Run("dial failure", func(t *testing.T) {
		logger := zerolog.Nop()
		dispatcher := NewDispatcher(&logger, nil)
		dispatcher.dial = func(*Connection) (dispatcherSSHClient, error) {
			return nil, errors.New("dial failed")
		}

		result, err := dispatcher.Run(
			context.Background(),
			NewPendingTask(dispatcherCoverageTask()).OnConnection(connection),
		)
		if result != nil || err == nil || !strings.Contains(err.Error(), "failed to connect") {
			t.Fatalf("Run() = (%+v, %v), want dial failure", result, err)
		}
	})

	t.Run("create script directory failure", func(t *testing.T) {
		client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{{err: errors.New("mkdir failed")}}}
		dispatcher := newDispatcherCoverageHarness(client)

		result, err := dispatcher.Run(
			context.Background(),
			NewPendingTask(dispatcherCoverageTask()).OnConnection(connection),
		)
		if result != nil || err == nil || !strings.Contains(err.Error(), "failed to create script directory") {
			t.Fatalf("Run() = (%+v, %v), want mkdir failure", result, err)
		}
		if client.closeCount != 1 {
			t.Fatalf("close count = %d, want 1", client.closeCount)
		}
	})

	t.Run("upload failure", func(t *testing.T) {
		client := &dispatcherFakeSSHClient{
			runResponses: []dispatcherRunResponse{{result: &SSHCommandResult{}}},
			uploadErr:    errors.New("upload failed"),
		}
		dispatcher := newDispatcherCoverageHarness(client)

		result, err := dispatcher.Run(
			context.Background(),
			NewPendingTask(dispatcherCoverageTask()).OnConnection(connection),
		)
		if result != nil || err == nil || !strings.Contains(err.Error(), "failed to upload script") {
			t.Fatalf("Run() = (%+v, %v), want upload failure", result, err)
		}
		if client.closeCount != 1 {
			t.Fatalf("close count = %d, want 1", client.closeCount)
		}
	})

	t.Run("foreground", func(t *testing.T) {
		client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{
			{result: &SSHCommandResult{}},
			{result: &SSHCommandResult{Stdout: "foreground output"}},
		}}
		dispatcher := newDispatcherCoverageHarness(client)
		task := dispatcherCoverageTask()
		var finished *TaskResult
		task.FinishedCallback = func(_ context.Context, result *TaskResult) { finished = result }

		result, err := dispatcher.Run(
			context.Background(),
			NewPendingTask(task).WithID("foreground-id").OnConnection(connection).InForeground(),
		)
		if err != nil || result == nil || result.Output != "foreground output" || finished != result {
			t.Fatalf("unexpected foreground result: (%+v, %v)", result, err)
		}
		if client.closeCount != 1 || string(client.uploaded) != task.Script() || client.uploadMode != 0o755 {
			t.Fatalf("foreground cleanup/upload mismatch: %+v", client)
		}
		if !strings.Contains(client.uploadPath, "foreground-id") || len(client.runCommands) != 2 {
			t.Fatalf("unexpected remote commands/upload path: %+v", client)
		}
	})

	t.Run("background", func(t *testing.T) {
		client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{
			{result: &SSHCommandResult{}},
			{result: &SSHCommandResult{Stdout: " 4242\n"}},
		}}
		dispatcher := newDispatcherCoverageHarness(client)
		dispatcher.streamMonitor = nil
		completion := make(chan *TaskResult, 1)

		result, err := dispatcher.Run(
			context.Background(),
			NewPendingTask(dispatcherCoverageTask()).WithID("background-id").OnConnection(connection).
				InBackground().WithCompletionChannel(completion),
		)
		if err != nil || result != nil {
			t.Fatalf("Run() = (%+v, %v), want asynchronous result", result, err)
		}
		select {
		case completed := <-completion:
			if !strings.Contains(completed.Output, "PID: 4242") {
				t.Fatalf("unexpected completion output: %q", completed.Output)
			}
		default:
			t.Fatal("background fallback did not complete")
		}
	})
}

func TestDispatcher100RemoteForegroundOutcomes(t *testing.T) {
	connection := &Connection{Host: "example.test"}
	taskPaths := paths.GetTaskPaths("/tmp/tasks", "foreground-outcome")

	tests := []struct {
		name       string
		response   dispatcherRunResponse
		wantExit   int
		wantOutput string
		wantEvent  string
	}{
		{
			name:       "success combines streams",
			response:   dispatcherRunResponse{result: &SSHCommandResult{Stdout: "stdout", Stderr: "stderr"}},
			wantOutput: "stdoutstderr",
			wantEvent:  "finished",
		},
		{
			name:      "timeout",
			response:  dispatcherRunResponse{result: &SSHCommandResult{ExitCode: 124}},
			wantExit:  124,
			wantEvent: "timeout",
		},
		{
			name:      "nonzero exit",
			response:  dispatcherRunResponse{result: &SSHCommandResult{ExitCode: 9}},
			wantExit:  9,
			wantEvent: "failed",
		},
		{
			name:      "session error without result",
			response:  dispatcherRunResponse{err: errors.New("session failed")},
			wantEvent: "failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{test.response}}
			dispatcher := newDispatcherCoverageHarness(client)
			task := dispatcherCoverageTask()
			var event string
			task.FinishedCallback = func(context.Context, *TaskResult) { event = "finished" }
			task.FailedCallback = func(context.Context, *TaskResult) { event = "failed" }
			task.TimeoutCallback = func(context.Context, *TaskResult) { event = "timeout" }
			pending := NewPendingTask(task).WithID("foreground-outcome").OnConnection(connection)

			result, err := dispatcher.runRemoteForeground(
				context.Background(), client, pending, taskPaths, time.Now(),
			)
			if err != nil {
				t.Fatalf("runRemoteForeground() error = %v", err)
			}
			if result.ExitCode != test.wantExit || result.Output != test.wantOutput || event != test.wantEvent {
				t.Fatalf("result/event = (%+v, %q), want exit=%d output=%q event=%q",
					result, event, test.wantExit, test.wantOutput, test.wantEvent)
			}
			if test.response.err != nil && !errors.Is(result.Error, test.response.err) {
				t.Fatalf("result error = %v, want %v", result.Error, test.response.err)
			}
		})
	}
}

func TestDispatcher100RemoteBackgroundStartFailures(t *testing.T) {
	connection := &Connection{Host: "example.test"}
	taskPaths := paths.GetTaskPaths("/tmp/tasks", "background-start")
	pending := NewPendingTask(dispatcherCoverageTask()).WithID("background-start").OnConnection(connection).InBackground()

	for _, test := range []struct {
		name     string
		response dispatcherRunResponse
		message  string
	}{
		{
			name:     "command failure",
			response: dispatcherRunResponse{err: errors.New("nohup failed")},
			message:  "nohup failed",
		},
		{
			name:     "nil SSH response",
			response: dispatcherRunResponse{},
			message:  "empty SSH response",
		},
		{
			name:     "blank process ID",
			response: dispatcherRunResponse{result: &SSHCommandResult{Stdout: " \n"}},
			message:  "invalid process ID",
		},
		{
			name:     "non-numeric process ID",
			response: dispatcherRunResponse{result: &SSHCommandResult{Stdout: "pid-123"}},
			message:  "invalid process ID",
		},
		{
			name:     "zero process ID",
			response: dispatcherRunResponse{result: &SSHCommandResult{Stdout: "0"}},
			message:  "invalid process ID",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{test.response}}
			dispatcher := newDispatcherCoverageHarness(client)

			result, err := dispatcher.runRemoteBackground(
				context.Background(), client, pending, taskPaths, time.Now(),
			)
			if result != nil || err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("runRemoteBackground() = (%+v, %v), want %q", result, err, test.message)
			}
			if client.closeCount != 1 {
				t.Fatalf("close count = %d, want 1", client.closeCount)
			}
		})
	}
}

func TestDispatcher100RemoteBackgroundMonitoring(t *testing.T) {
	connection := &Connection{Host: "example.test"}
	taskPaths := paths.GetTaskPaths("/tmp/tasks", "background-monitor")

	t.Run("successful stream", func(t *testing.T) {
		client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{{
			result: &SSHCommandResult{Stdout: " 7070\n"},
		}}}
		dispatcher := newDispatcherCoverageHarness(client)
		updates := make(chan string, 2)
		finished := make(chan *TaskResult, 1)
		task := dispatcherCoverageTask()
		task.OutputCallback = func(output string) { updates <- "task:" + output }
		task.FinishedCallback = func(_ context.Context, result *TaskResult) { finished <- result }
		completion := make(chan *TaskResult, 1)
		pending := NewPendingTask(task).WithID("background-monitor").OnConnection(connection).InBackground().
			OnOutput(func(output string) { updates <- "pending:" + output }).
			WithCompletionChannel(completion)

		dispatcher.monitorBackgroundTask = func(
			_ context.Context,
			_ *Connection,
			_ string,
			pid string,
			_ MarkerHandler,
			onUpdate func(string, string),
			onComplete func(*StreamResult),
		) error {
			if pid != "7070" {
				return errors.New("unexpected pid")
			}
			onUpdate("streamed output", "running")
			onComplete(&StreamResult{
				TaskID:     "background-monitor",
				Output:     "streamed output",
				Status:     "finished",
				FinishedAt: time.Now(),
			})
			return nil
		}

		result, err := dispatcher.runRemoteBackground(
			context.Background(), client, pending, taskPaths, time.Now(),
		)
		if result != nil || err != nil {
			t.Fatalf("runRemoteBackground() = (%+v, %v)", result, err)
		}

		select {
		case completed := <-completion:
			if completed.Output != "streamed output" || completed.ExitCode != 0 {
				t.Fatalf("unexpected completion: %+v", completed)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for stream completion")
		}
		select {
		case callbackResult := <-finished:
			if callbackResult.Output != "streamed output" {
				t.Fatalf("unexpected callback result: %+v", callbackResult)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for finished callback")
		}

		gotUpdates := map[string]bool{}
		for range 2 {
			select {
			case update := <-updates:
				gotUpdates[update] = true
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for output callback")
			}
		}
		if !gotUpdates["pending:streamed output"] || !gotUpdates["task:streamed output"] {
			t.Fatalf("missing output callbacks: %#v", gotUpdates)
		}
	})

	t.Run("stream failure", func(t *testing.T) {
		streamErr := errors.New("monitor disconnected")
		client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{{
			result: &SSHCommandResult{Stdout: "8080"},
		}}}
		dispatcher := newDispatcherCoverageHarness(client)
		dispatcher.monitorBackgroundTask = func(
			context.Context,
			*Connection,
			string,
			string,
			MarkerHandler,
			func(string, string),
			func(*StreamResult),
		) error {
			return streamErr
		}
		failed := make(chan *TaskResult, 1)
		task := dispatcherCoverageTask()
		task.FailedCallback = func(_ context.Context, result *TaskResult) { failed <- result }
		completion := make(chan *TaskResult, 1)
		pending := NewPendingTask(task).WithID("background-monitor-error").OnConnection(connection).
			InBackground().WithCompletionChannel(completion)

		result, err := dispatcher.runRemoteBackground(
			context.Background(), client, pending, taskPaths, time.Now(),
		)
		if result != nil || err != nil {
			t.Fatalf("runRemoteBackground() = (%+v, %v)", result, err)
		}

		select {
		case completed := <-completion:
			if completed.ExitCode != 1 || !errors.Is(completed.Error, streamErr) ||
				!strings.Contains(completed.Output, streamErr.Error()) {
				t.Fatalf("unexpected failed completion: %+v", completed)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for failed completion")
		}
		select {
		case callbackResult := <-failed:
			if !errors.Is(callbackResult.Error, streamErr) {
				t.Fatalf("unexpected failed callback: %+v", callbackResult)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for failed callback")
		}
	})
}

func TestDispatcher100CompleteBackgroundTaskOutcomes(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)

	t.Run("successful result with full completion channel", func(t *testing.T) {
		finished := false
		task := dispatcherCoverageTask()
		task.FinishedCallback = func(context.Context, *TaskResult) { finished = true }
		completion := make(chan *TaskResult, 1)
		original := &TaskResult{TaskID: "original"}
		completion <- original
		pending := NewPendingTask(task).WithCompletionChannel(completion)

		dispatcher.completeBackgroundTask(context.Background(), pending, &TaskResult{})
		if !finished {
			t.Fatal("successful result did not invoke finished callback")
		}
		if got := <-completion; got != original {
			t.Fatal("a full completion channel should retain its first result")
		}
	})

	t.Run("failed result without completion channel", func(t *testing.T) {
		failed := false
		task := dispatcherCoverageTask()
		task.FailedCallback = func(context.Context, *TaskResult) { failed = true }
		pending := NewPendingTask(task)

		dispatcher.completeBackgroundTask(context.Background(), pending, &TaskResult{ExitCode: 2})
		if !failed {
			t.Fatal("failed result did not invoke failed callback")
		}
	})
}

func TestDispatcher100TaskOutputAndStatusHelpers(t *testing.T) {
	connection := &Connection{Host: "example.test", ScriptPath: "/tmp/tasks"}
	dialErr := errors.New("dial failed")

	t.Run("output dial failure", func(t *testing.T) {
		dispatcher := NewDispatcher(nil, nil)
		dispatcher.dial = func(*Connection) (dispatcherSSHClient, error) { return nil, dialErr }
		output, err := dispatcher.GetTaskOutput(context.Background(), connection, "task")
		if output != "" || !errors.Is(err, dialErr) {
			t.Fatalf("GetTaskOutput() = (%q, %v)", output, err)
		}
	})

	t.Run("output download failure", func(t *testing.T) {
		downloadErr := errors.New("download failed")
		client := &dispatcherFakeSSHClient{downloadErr: downloadErr}
		dispatcher := newDispatcherCoverageHarness(client)
		output, err := dispatcher.GetTaskOutput(context.Background(), connection, "task")
		if output != "" || !errors.Is(err, downloadErr) || client.closeCount != 1 {
			t.Fatalf("GetTaskOutput() = (%q, %v), close=%d", output, err, client.closeCount)
		}
	})

	t.Run("output success", func(t *testing.T) {
		client := &dispatcherFakeSSHClient{downloaded: []byte("task output")}
		dispatcher := newDispatcherCoverageHarness(client)
		output, err := dispatcher.GetTaskOutput(context.Background(), connection, "task")
		if err != nil || output != "task output" || client.closeCount != 1 {
			t.Fatalf("GetTaskOutput() = (%q, %v), close=%d", output, err, client.closeCount)
		}
	})

	t.Run("status dial failure", func(t *testing.T) {
		dispatcher := NewDispatcher(nil, nil)
		dispatcher.dial = func(*Connection) (dispatcherSSHClient, error) { return nil, dialErr }
		running, code, err := dispatcher.CheckTaskStatus(context.Background(), connection, "1")
		if running || code != 0 || !errors.Is(err, dialErr) {
			t.Fatalf("CheckTaskStatus() = (%v, %d, %v)", running, code, err)
		}
	})

	t.Run("status rejects unsafe process ID", func(t *testing.T) {
		dispatcher := NewDispatcher(nil, nil)
		dispatcher.dial = func(*Connection) (dispatcherSSHClient, error) {
			t.Fatal("invalid process ID should be rejected before dialing")
			return nil, nil
		}
		running, code, err := dispatcher.CheckTaskStatus(context.Background(), connection, "42; touch /tmp/injected")
		if running || code != 0 || err == nil || !strings.Contains(err.Error(), "invalid process ID") {
			t.Fatalf("CheckTaskStatus() = (%v, %d, %v)", running, code, err)
		}
	})

	for _, test := range []struct {
		name        string
		response    dispatcherRunResponse
		wantRunning bool
		wantErr     error
	}{
		{
			name:     "status command failure",
			response: dispatcherRunResponse{err: errors.New("ps failed")},
			wantErr:  errors.New("ps failed"),
		},
		{
			name:     "status empty response",
			response: dispatcherRunResponse{},
		},
		{
			name:        "status running",
			response:    dispatcherRunResponse{result: &SSHCommandResult{Stdout: " 42\n"}},
			wantRunning: true,
		},
		{
			name:     "status stopped",
			response: dispatcherRunResponse{result: &SSHCommandResult{}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &dispatcherFakeSSHClient{runResponses: []dispatcherRunResponse{test.response}}
			dispatcher := newDispatcherCoverageHarness(client)
			running, code, err := dispatcher.CheckTaskStatus(context.Background(), connection, "42")
			if running != test.wantRunning || code != 0 || (test.wantErr != nil) != (err != nil) {
				t.Fatalf("CheckTaskStatus() = (%v, %d, %v)", running, code, err)
			}
			if test.wantErr != nil && err.Error() != test.wantErr.Error() {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if client.closeCount != 1 {
				t.Fatalf("close count = %d, want 1", client.closeCount)
			}
		})
	}
}

func TestDispatcher100TaskLogging(t *testing.T) {
	dispatcher := NewDispatcher(nil, nil)
	dispatcher.logTaskOutput("task", "host", "ignored")
	dispatcher.logTaskEvent("task", "host", "ignored")

	var output bytes.Buffer
	logger := zerolog.New(&output)
	dispatcher.taskLogger = &logger
	dispatcher.logTaskOutput("task-1", "server-1", "\nfirst line\n\nsecond line\n")
	dispatcher.logTaskEvent("task-1", "server-1", "finished")

	logged := output.String()
	for _, want := range []string{"task-1", "server-1", "first line", "second line", "finished"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("task log missing %q: %s", want, logged)
		}
	}
}
