package taskrunner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
	"github.com/rs/zerolog"
)

type monitorSSHClient struct {
	runResult       *SSHCommandResult
	runErr          error
	streamLines     []string
	streamErr       error
	ignoreCallback  bool
	closeErr        error
	runCommands     []string
	streamCommands  []string
	closeCalls      int
	streamCallCount int
}

func (c *monitorSSHClient) Run(_ context.Context, command string) (*SSHCommandResult, error) {
	c.runCommands = append(c.runCommands, command)
	return c.runResult, c.runErr
}

func (c *monitorSSHClient) StreamOutput(
	_ context.Context,
	command string,
	callback func(line string) error,
) error {
	c.streamCallCount++
	c.streamCommands = append(c.streamCommands, command)
	for _, line := range c.streamLines {
		if err := callback(line); err != nil && !c.ignoreCallback {
			return err
		}
	}
	return c.streamErr
}

func (c *monitorSSHClient) Close() error {
	c.closeCalls++
	return c.closeErr
}

type monitorBroadcast struct {
	channel string
	event   string
	data    any
}

type monitorBroadcaster struct {
	mu     sync.Mutex
	events []monitorBroadcast
}

func (b *monitorBroadcaster) Broadcast(channel, event string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, monitorBroadcast{channel: channel, event: event, data: data})
}

func (b *monitorBroadcaster) count(event string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, recorded := range b.events {
		if recorded.event == event {
			count++
		}
	}
	return count
}

func newCoverageMonitor(ws SimpleBroadcaster, taskLogger *zerolog.Logger) *StreamMonitor {
	logger := zerolog.New(io.Discard)
	return NewStreamMonitor(&logger, ws, &StreamMonitorConfig{
		BroadcastInterval: time.Nanosecond,
		TaskLogger:        taskLogger,
	})
}

func TestStreamMonitorDefaultDialer(t *testing.T) {
	monitor := newCoverageMonitor(nil, nil)
	_, err := monitor.dial(&Connection{PrivateKey: "not-a-private-key"})
	if err == nil {
		t.Fatal("expected the production dialer to reject an invalid private key")
	}
}

func coverageConnection() *Connection {
	return &Connection{
		Host:       "server.example",
		User:       "launcher",
		ScriptPath: "/srv/launch",
	}
}

func TestMarkerHandlerFuncAndStreamResultConversion(t *testing.T) {
	marker := &markers.Marker{Type: markers.Status, Value: "working"}
	called := false
	handlerErr := errors.New("marker rejected")
	handler := MarkerHandlerFunc(func(ctx context.Context, taskID string, got *markers.Marker) error {
		called = true
		if ctx != context.Background() || taskID != "task-1" || got != marker {
			t.Fatal("marker handler received unexpected arguments")
		}
		return handlerErr
	})
	if err := handler.OnMarker(context.Background(), "task-1", marker); !errors.Is(err, handlerErr) {
		t.Fatalf("expected marker error, got %v", err)
	}
	if !called {
		t.Fatal("marker handler was not called")
	}

	finishedAt := time.Now()
	streamErr := errors.New("stream failed")
	result := (&StreamResult{
		TaskID:     "task-2",
		Output:     "output",
		ExitCode:   124,
		Status:     "timeout",
		Error:      streamErr,
		FinishedAt: finishedAt,
	}).ToTaskResult()
	if result.TaskID != "task-2" || result.Output != "output" || result.ExitCode != 124 {
		t.Fatalf("unexpected task result: %#v", result)
	}
	if !result.TimedOut || !errors.Is(result.Error, streamErr) || !result.FinishedAt.Equal(finishedAt) {
		t.Fatalf("stream metadata was not preserved: %#v", result)
	}

	plain := (&StreamResult{TaskID: "plain", Status: "finished"}).ToTaskResult()
	if plain.TimedOut || plain.Error != nil {
		t.Fatalf("unexpected flags on successful result: %#v", plain)
	}
}

func TestStreamMonitorMarkerHandlingAndBroadcasts(t *testing.T) {
	broadcaster := &monitorBroadcaster{}
	monitor := newCoverageMonitor(broadcaster, nil)
	seen := 0
	monitor.SetMarkerHandler(MarkerHandlerFunc(func(_ context.Context, taskID string, marker *markers.Marker) error {
		seen++
		if taskID != "task-markers" || marker == nil {
			t.Fatal("unexpected marker callback")
		}
		return nil
	}))

	exitCode := 0
	status := ""
	tests := []struct {
		marker *markers.Marker
		event  string
	}{
		{marker: &markers.Marker{Type: markers.Progress, Value: "45"}, event: "task.progress"},
		{marker: &markers.Marker{Type: markers.StepCompleted, Value: "packages"}, event: "task.step_completed"},
		{marker: &markers.Marker{Type: markers.SoftwareInstalled, Value: "php83"}, event: "task.software_installed"},
		{marker: &markers.Marker{Type: markers.Status, Value: "configuring"}, event: "task.status"},
		{marker: &markers.Marker{Type: markers.Error, Value: "warning"}, event: "task.error"},
		{marker: &markers.Marker{Type: "future_marker", Value: "ignored"}},
	}
	for _, tt := range tests {
		if err := monitor.handleMarker(context.Background(), "task-markers", tt.marker, &exitCode, &status); err != nil {
			t.Fatalf("handle %s: %v", tt.marker.Type, err)
		}
		if tt.event != "" && broadcaster.count(tt.event) != 1 {
			t.Fatalf("expected one %s event", tt.event)
		}
	}

	err := monitor.handleMarker(
		context.Background(),
		"task-markers",
		&markers.Marker{Type: markers.ExitCode, Value: "0"},
		&exitCode,
		&status,
	)
	if !errors.Is(err, ErrTaskCompleted) || exitCode != 0 || status != "finished" {
		t.Fatalf("unexpected successful exit marker result: code=%d status=%q err=%v", exitCode, status, err)
	}

	err = monitor.handleMarker(
		context.Background(),
		"task-markers",
		&markers.Marker{Type: markers.ExitCode, Value: "7"},
		&exitCode,
		&status,
	)
	if !errors.Is(err, ErrTaskCompleted) || exitCode != 7 || status != "failed" {
		t.Fatalf("unexpected failed exit marker result: code=%d status=%q err=%v", exitCode, status, err)
	}
	if seen != len(tests)+2 {
		t.Fatalf("expected %d global marker callbacks, got %d", len(tests)+2, seen)
	}

	rejected := errors.New("stop")
	monitor.SetMarkerHandler(MarkerHandlerFunc(func(context.Context, string, *markers.Marker) error {
		return rejected
	}))
	before := len(broadcaster.events)
	err = monitor.handleMarker(
		context.Background(),
		"task-markers",
		&markers.Marker{Type: markers.Progress, Value: "99"},
		&exitCode,
		&status,
	)
	if !errors.Is(err, rejected) || len(broadcaster.events) != before {
		t.Fatal("custom marker errors should stop built-in marker handling")
	}

	monitor.broadcastOutput("task-markers", "hello", "running")
	monitor.broadcastProgress("task-markers", 80)
	data := map[string]interface{}{"message": "done"}
	monitor.broadcastMarkerEvent("task-markers", "task.custom", data)
	if data["task_id"] != "task-markers" {
		t.Fatalf("marker event did not include task id: %#v", data)
	}

	nilMonitor := newCoverageMonitor(nil, nil)
	nilMonitor.broadcastOutput("task", "output", "running")
	nilMonitor.broadcastProgress("task", 10)
	nilMonitor.broadcastMarkerEvent("task", "event", map[string]interface{}{})
}

func TestStreamMonitorStopsActiveStreams(t *testing.T) {
	monitor := newCoverageMonitor(nil, nil)
	ctxOne, cancelOne := context.WithCancel(context.Background())
	ctxTwo, cancelTwo := context.WithCancel(context.Background())
	monitor.activeStreams["one"] = cancelOne
	monitor.activeStreams["two"] = cancelTwo

	monitor.StopStreaming("one")
	select {
	case <-ctxOne.Done():
	default:
		t.Fatal("specific stream was not cancelled")
	}
	if monitor.IsStreaming("one") || monitor.ActiveStreamCount() != 1 {
		t.Fatal("specific stream was not removed")
	}

	monitor.StopAll()
	select {
	case <-ctxTwo.Done():
	default:
		t.Fatal("remaining stream was not cancelled")
	}
	if monitor.ActiveStreamCount() != 0 {
		t.Fatal("all streams were not removed")
	}
}

func TestStreamTaskOutputSuccess(t *testing.T) {
	var taskLogs bytes.Buffer
	taskLogger := zerolog.New(&taskLogs)
	broadcaster := &monitorBroadcaster{}
	monitor := newCoverageMonitor(broadcaster, &taskLogger)
	client := &monitorSSHClient{
		runErr: errors.New("log not created yet"),
		streamLines: []string{
			"starting",
			markers.FormatProgress(50),
			markers.FormatStatus("deploying"),
			markers.Format(markers.ExitCode, "0"),
		},
	}
	monitor.dial = func(*Connection) (streamClient, error) {
		return client, nil
	}

	var updates []string
	var completed *StreamResult
	conn := coverageConnection()
	err := monitor.StreamTaskOutput(
		context.Background(),
		conn,
		"task-success",
		func(output, status string) {
			updates = append(updates, status+":"+output)
		},
		func(result *StreamResult) { completed = result },
	)
	if err != nil {
		t.Fatalf("stream output: %v", err)
	}
	if completed == nil || completed.Status != "finished" || completed.ExitCode != 0 || completed.Error != nil {
		t.Fatalf("unexpected completion: %#v", completed)
	}
	if completed.Output != "starting\n" || len(updates) == 0 {
		t.Fatalf("unexpected streamed output: result=%q updates=%#v", completed.Output, updates)
	}
	if client.closeCalls != 1 || client.streamCallCount != 1 || len(client.runCommands) != 1 {
		t.Fatalf("unexpected client calls: %#v", client)
	}
	if !strings.Contains(client.runCommands[0], "task-success.log") || !strings.Contains(client.streamCommands[0], "tail -n +1 -f") {
		t.Fatalf("unexpected remote commands: run=%q stream=%q", client.runCommands[0], client.streamCommands[0])
	}
	if monitor.IsStreaming("task-success") || broadcaster.count("task.output") < 2 {
		t.Fatal("stream lifecycle or output broadcasts were not completed")
	}
	if !strings.Contains(taskLogs.String(), "starting") {
		t.Fatalf("task output was not logged: %s", taskLogs.String())
	}
}

func TestStreamTaskOutputFailurePaths(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		monitor.activeStreams["duplicate"] = func() {}
		err := monitor.StreamTaskOutput(context.Background(), coverageConnection(), "duplicate", nil, nil)
		if err == nil || !strings.Contains(err.Error(), "already streaming") {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	})

	t.Run("dial", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		dialErr := errors.New("dial failed")
		monitor.dial = func(*Connection) (streamClient, error) { return nil, dialErr }
		err := monitor.StreamTaskOutput(context.Background(), coverageConnection(), "dial", nil, nil)
		if !errors.Is(err, dialErr) || monitor.IsStreaming("dial") {
			t.Fatalf("unexpected dial result: %v", err)
		}
	})

	t.Run("cancelled while waiting", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		client := &monitorSSHClient{runErr: context.Canceled}
		monitor.dial = func(*Connection) (streamClient, error) { return client, nil }
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := monitor.StreamTaskOutput(ctx, coverageConnection(), "cancelled", nil, nil)
		if !errors.Is(err, context.Canceled) || client.streamCallCount != 0 || client.closeCalls != 1 {
			t.Fatalf("unexpected cancellation result: err=%v client=%#v", err, client)
		}
	})

	t.Run("stream failure", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		streamErr := errors.New("connection dropped")
		client := &monitorSSHClient{streamLines: []string{"partial"}, streamErr: streamErr}
		monitor.dial = func(*Connection) (streamClient, error) { return client, nil }
		err := monitor.StreamTaskOutput(context.Background(), coverageConnection(), "failed", nil, nil)
		if err != nil {
			t.Fatalf("monitor returns completion through callback, got %v", err)
		}

		var completed *StreamResult
		client = &monitorSSHClient{streamLines: []string{"partial"}, streamErr: streamErr}
		monitor.dial = func(*Connection) (streamClient, error) { return client, nil }
		err = monitor.StreamTaskOutput(
			context.Background(),
			coverageConnection(),
			"failed-result",
			nil,
			func(result *StreamResult) { completed = result },
		)
		if err != nil || completed == nil || completed.Status != "failed" || completed.ExitCode != 1 || !errors.Is(completed.Error, streamErr) {
			t.Fatalf("unexpected stream failure: err=%v result=%#v", err, completed)
		}
	})
}

func TestMonitorBackgroundTaskSuccess(t *testing.T) {
	var taskLogs bytes.Buffer
	taskLogger := zerolog.New(&taskLogs)
	broadcaster := &monitorBroadcaster{}
	monitor := newCoverageMonitor(broadcaster, &taskLogger)
	globalMarkers := 0
	monitor.SetMarkerHandler(MarkerHandlerFunc(func(context.Context, string, *markers.Marker) error {
		globalMarkers++
		return nil
	}))
	client := &monitorSSHClient{
		runErr: errors.New("wait timed out"),
		streamLines: []string{
			"booting",
			markers.FormatProgress(25),
			markers.FormatStepCompleted("packages"),
			markers.FormatSoftwareInstalled("php83"),
			markers.FormatStatus("deploying"),
			markers.Format(markers.Error, "recoverable warning"),
			markers.Format("future_marker", "value"),
			markers.Format(markers.ExitCode, "0"),
		},
	}
	monitor.dial = func(*Connection) (streamClient, error) { return client, nil }

	perTaskMarkers := 0
	perTaskErr := errors.New("callback failed")
	perTask := MarkerHandlerFunc(func(context.Context, string, *markers.Marker) error {
		perTaskMarkers++
		return perTaskErr
	})
	var updates []string
	var completed *StreamResult
	err := monitor.MonitorBackgroundTask(
		context.Background(),
		coverageConnection(),
		"task-background",
		"4321",
		perTask,
		func(output, status string) { updates = append(updates, status+":"+output) },
		func(result *StreamResult) { completed = result },
	)
	if err != nil {
		t.Fatalf("monitor background task: %v", err)
	}
	if completed == nil || completed.Status != "finished" || completed.ExitCode != 0 || completed.Error != nil {
		t.Fatalf("unexpected completion: %#v", completed)
	}
	if perTaskMarkers != 7 || globalMarkers != 7 {
		t.Fatalf("unexpected marker counts: per-task=%d global=%d", perTaskMarkers, globalMarkers)
	}
	if len(updates) < 7 || !strings.Contains(completed.Output, markers.FormatProgress(25)) {
		t.Fatalf("marker output was not streamed: updates=%d output=%q", len(updates), completed.Output)
	}
	if broadcaster.count("task.step_completed") != 1 || broadcaster.count("task.output") < 2 {
		t.Fatalf("expected marker and output broadcasts, got %#v", broadcaster.events)
	}
	if client.closeCalls != 1 || monitor.IsStreaming("task-background") {
		t.Fatal("background stream was not cleaned up")
	}
	if !strings.Contains(taskLogs.String(), "started") || !strings.Contains(taskLogs.String(), "completed status=finished") {
		t.Fatalf("task lifecycle was not logged: %s", taskLogs.String())
	}
}

func TestMonitorBackgroundTaskFailurePaths(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		monitor.activeStreams["duplicate"] = func() {}
		err := monitor.MonitorBackgroundTask(
			context.Background(), coverageConnection(), "duplicate", "1", nil, nil, nil,
		)
		if err == nil || !strings.Contains(err.Error(), "already monitoring") {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	})

	t.Run("dial", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		dialErr := errors.New("dial failed")
		monitor.dial = func(*Connection) (streamClient, error) { return nil, dialErr }
		err := monitor.MonitorBackgroundTask(
			context.Background(), coverageConnection(), "dial", "1", nil, nil, nil,
		)
		if !errors.Is(err, dialErr) || monitor.IsStreaming("dial") {
			t.Fatalf("unexpected dial result: %v", err)
		}
	})

	t.Run("cancelled while waiting", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		client := &monitorSSHClient{runErr: context.Canceled}
		monitor.dial = func(*Connection) (streamClient, error) { return client, nil }
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := monitor.MonitorBackgroundTask(ctx, coverageConnection(), "cancelled", "1", nil, nil, nil)
		if !errors.Is(err, context.Canceled) || client.streamCallCount != 0 || client.closeCalls != 1 {
			t.Fatalf("unexpected cancellation result: err=%v client=%#v", err, client)
		}
	})

	t.Run("stream failure", func(t *testing.T) {
		monitor := newCoverageMonitor(nil, nil)
		streamErr := errors.New("tail disconnected")
		client := &monitorSSHClient{streamLines: []string{"partial"}, streamErr: streamErr}
		monitor.dial = func(*Connection) (streamClient, error) { return client, nil }
		var completed *StreamResult
		err := monitor.MonitorBackgroundTask(
			context.Background(),
			coverageConnection(),
			"failed",
			"1",
			nil,
			nil,
			func(result *StreamResult) { completed = result },
		)
		if err != nil || completed == nil || completed.Status != "failed" || completed.ExitCode != 1 || !errors.Is(completed.Error, streamErr) {
			t.Fatalf("unexpected background failure: err=%v result=%#v", err, completed)
		}
	})
}

func TestStreamMonitorTaskLogging(t *testing.T) {
	monitor := newCoverageMonitor(nil, nil)
	monitor.logOutputLine("task", "host", "line")
	monitor.logTaskEvent("task", "host", "started")

	var output bytes.Buffer
	logger := zerolog.New(&output)
	monitor.taskLogger = &logger
	monitor.logOutputLine("task", "host", "line")
	monitor.logTaskEvent("task", "host", "started")
	logged := output.String()
	if !strings.Contains(logged, "line") || !strings.Contains(logged, "started") {
		t.Fatalf("expected task logs, got %s", logged)
	}
}
