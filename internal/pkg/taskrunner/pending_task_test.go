package taskrunner

import (
	"context"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

func TestNewPendingTask(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))
	pt := NewPendingTask(task)

	if pt.Task != task {
		t.Error("expected task to be set")
	}
	if pt.Background {
		t.Error("expected Background to default to false")
	}
	if pt.TaskID != "" {
		t.Error("expected TaskID to default to empty")
	}
}

func TestPendingTask_BuilderChaining(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))

	pt := NewPendingTask(task).
		WithID("id-1").
		InBackground().
		WriteOutputTo("/tmp/out.log")

	if pt.TaskID != "id-1" {
		t.Errorf("expected TaskID 'id-1', got '%s'", pt.TaskID)
	}
	if !pt.Background {
		t.Error("expected Background to be true")
	}
	if pt.OutputPath != "/tmp/out.log" {
		t.Errorf("expected OutputPath '/tmp/out.log', got '%s'", pt.OutputPath)
	}
}

func TestPendingTask_AsAlias(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))
	pt := NewPendingTask(task).As("my-id")

	if pt.TaskID != "my-id" {
		t.Errorf("expected TaskID 'my-id', got '%s'", pt.TaskID)
	}
}

func TestPendingTask_InForeground(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))
	pt := NewPendingTask(task).InBackground().InForeground()

	if pt.Background {
		t.Error("expected Background to be false after InForeground()")
	}
}

func TestPendingTask_ShouldRunHelpers(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))

	t.Run("foreground", func(t *testing.T) {
		pt := NewPendingTask(task)
		if !pt.ShouldRunInForeground() {
			t.Error("expected ShouldRunInForeground() to be true")
		}
		if pt.ShouldRunInBackground() {
			t.Error("expected ShouldRunInBackground() to be false")
		}
	})

	t.Run("background", func(t *testing.T) {
		pt := NewPendingTask(task).InBackground()
		if pt.ShouldRunInForeground() {
			t.Error("expected ShouldRunInForeground() to be false")
		}
		if !pt.ShouldRunInBackground() {
			t.Error("expected ShouldRunInBackground() to be true")
		}
	})
}

func TestPendingTask_OnOutput(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))

	t.Run("no callback set", func(t *testing.T) {
		pt := NewPendingTask(task)
		if pt.GetOnOutput() != nil {
			t.Error("expected nil callback when not set")
		}
	})

	t.Run("with callback", func(t *testing.T) {
		var received string
		pt := NewPendingTask(task).OnOutput(func(output string) {
			received = output
		})

		callback := pt.GetOnOutput()
		if callback == nil {
			t.Fatal("expected non-nil callback")
		}
		callback("test output")
		if received != "test output" {
			t.Errorf("expected 'test output', got '%s'", received)
		}
	})
}

// testMarkerHandler implements MarkerHandler for testing
type testMarkerHandler struct {
	markers []*markers.Marker
}

func (h *testMarkerHandler) OnMarker(_ context.Context, _ string, marker *markers.Marker) error {
	h.markers = append(h.markers, marker)
	return nil
}

func TestPendingTask_MarkerHandler(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))

	t.Run("no handler set", func(t *testing.T) {
		pt := NewPendingTask(task)
		if pt.GetMarkerHandler() != nil {
			t.Error("expected nil marker handler when not set")
		}
	})

	t.Run("with handler", func(t *testing.T) {
		handler := &testMarkerHandler{}
		pt := NewPendingTask(task).WithMarkerHandler(handler)

		if pt.GetMarkerHandler() == nil {
			t.Fatal("expected non-nil marker handler")
		}
		if pt.GetMarkerHandler() != handler {
			t.Error("expected same handler instance")
		}
	})
}

func TestPendingTask_CompletionChannel(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))

	t.Run("no channel set", func(t *testing.T) {
		pt := NewPendingTask(task)
		if pt.GetCompletionChannel() != nil {
			t.Error("expected nil completion channel when not set")
		}
	})

	t.Run("with channel", func(t *testing.T) {
		ch := make(chan *TaskResult, 1)
		pt := NewPendingTask(task).WithCompletionChannel(ch)

		gotCh := pt.GetCompletionChannel()
		if gotCh == nil {
			t.Fatal("expected non-nil completion channel")
		}

		// Verify it's the same channel by sending and receiving
		gotCh <- &TaskResult{TaskID: "test-123"}
		result := <-ch
		if result.TaskID != "test-123" {
			t.Errorf("expected TaskID 'test-123', got '%s'", result.TaskID)
		}
	})
}

// TestPendingTask_CompletionChannel_BackgroundFlow simulates the full
// background task flow: dispatcher sends result to completion channel,
// runner reads from it.
func TestPendingTask_CompletionChannel_BackgroundFlow(t *testing.T) {
	task := NewBaseTask(WithName("Background Deploy"), WithScript("echo deploying"))

	ch := make(chan *TaskResult, 1)
	pt := NewPendingTask(task)
	pt.InBackground()
	pt.As("deploy-001")
	pt.WithCompletionChannel(ch)

	// Simulate dispatcher sending result asynchronously (as MonitorBackgroundTask does)
	go func() {
		time.Sleep(10 * time.Millisecond)
		pt.GetCompletionChannel() <- &TaskResult{
			TaskID:   "deploy-001",
			ExitCode: 0,
			Output:   "deployment successful",
		}
	}()

	// Simulate runner waiting for result
	select {
	case result := <-ch:
		if result.TaskID != "deploy-001" {
			t.Errorf("expected TaskID 'deploy-001', got '%s'", result.TaskID)
		}
		if !result.IsSuccessful() {
			t.Error("expected successful result")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for completion — goroutine leak!")
	}
}

func TestPendingTask_Connection(t *testing.T) {
	task := NewBaseTask(WithName("test"), WithScript("echo hi"))
	conn := &Connection{Host: "192.168.1.1", User: "deploy"}

	pt := NewPendingTask(task).OnConnection(conn)

	if pt.GetConnection() == nil {
		t.Fatal("expected non-nil connection")
	}
	if pt.GetConnection().Host != "192.168.1.1" {
		t.Errorf("expected host '192.168.1.1', got '%s'", pt.GetConnection().Host)
	}
}
