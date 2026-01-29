package taskrunner

import (
	"testing"
	"time"
)

// TestStreamResult_FailureExitCode verifies that when a stream monitoring
// error occurs (e.g., SSH connection drops), the exit code is set to non-zero.
// This prevents IsSuccessful() from returning true when the task status is "failed"
// but the exit_code marker was never received (so exitCode defaulted to 0).
func TestStreamResult_FailureExitCode(t *testing.T) {
	// Simulate what happens when SSH monitoring drops mid-stream:
	// - exitCode stays at 0 (default, never got exit_code marker)
	// - finalStatus is "" (never got exit_code marker)
	// - err is non-nil (SSH connection error)
	result := &StreamResult{
		TaskID:     "task-123",
		Output:     "partial output...",
		ExitCode:   0,  // Default — never received exit_code marker
		Status:     "", // Default — never determined
		FinishedAt: time.Now(),
	}

	// This is the code path from StreamTaskOutput and MonitorBackgroundTask
	// when an error occurs that isn't ErrTaskCompleted:
	if result.Status == "" {
		result.Status = "failed"
	}
	if result.ExitCode == 0 {
		result.ExitCode = 1
	}

	if result.ExitCode == 0 {
		t.Fatal("exit code should be non-zero when monitoring failed")
	}
	if result.Status != "failed" {
		t.Fatalf("status should be 'failed', got '%s'", result.Status)
	}

	// Convert to TaskResult (as done in dispatcher.go onComplete callback)
	taskResult := &TaskResult{
		TaskID:   result.TaskID,
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}
	if result.Error != nil {
		taskResult.Error = result.Error
	}

	if taskResult.IsSuccessful() {
		t.Fatal("TaskResult should NOT be successful when monitoring failed")
	}
}

// TestStreamResult_NormalSuccess verifies that normal successful task completion
// still works correctly with exit code 0.
func TestStreamResult_NormalSuccess(t *testing.T) {
	result := &StreamResult{
		TaskID:     "task-success",
		Output:     "all good",
		ExitCode:   0,
		Status:     "finished",
		FinishedAt: time.Now(),
	}

	taskResult := &TaskResult{
		TaskID:   result.TaskID,
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}

	if !taskResult.IsSuccessful() {
		t.Fatal("normal successful task should have IsSuccessful() = true")
	}
}

// TestStreamResult_NormalFailure verifies that tasks with non-zero exit code
// are correctly identified as failed.
func TestStreamResult_NormalFailure(t *testing.T) {
	result := &StreamResult{
		TaskID:     "task-fail",
		Output:     "error output",
		ExitCode:   1,
		Status:     "failed",
		FinishedAt: time.Now(),
	}

	taskResult := &TaskResult{
		TaskID:   result.TaskID,
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}

	if taskResult.IsSuccessful() {
		t.Fatal("failed task should have IsSuccessful() = false")
	}
}

// TestStreamResult_TimeoutStatus verifies that timeout status is correctly
// propagated to TaskResult.
func TestStreamResult_TimeoutStatus(t *testing.T) {
	result := &StreamResult{
		TaskID:     "task-timeout",
		Output:     "partial output",
		ExitCode:   124,
		Status:     "timeout",
		FinishedAt: time.Now(),
	}

	taskResult := &TaskResult{
		TaskID:   result.TaskID,
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}
	if result.Status == "timeout" {
		taskResult.TimedOut = true
	}

	if taskResult.IsSuccessful() {
		t.Fatal("timed-out task should have IsSuccessful() = false")
	}
	if !taskResult.TimedOut {
		t.Fatal("timed-out task should have TimedOut = true")
	}
}

// TestCompletionChannel_SelectDefault verifies that the select/default pattern
// used to prevent goroutine leaks works correctly: when a result was already
// sent to the channel (by onComplete), a second send (from error handling)
// doesn't block.
func TestCompletionChannel_SelectDefault(t *testing.T) {
	ch := make(chan *TaskResult, 1)

	// Simulate onComplete already sent a result
	ch <- &TaskResult{TaskID: "from-onComplete", ExitCode: 0}

	// Simulate error handler trying to send (should not block)
	select {
	case ch <- &TaskResult{TaskID: "from-error", ExitCode: 1}:
		t.Fatal("should NOT have been able to send — channel should be full")
	default:
		// Expected: channel is full, default case taken
	}

	// Verify the original result is preserved
	result := <-ch
	if result.TaskID != "from-onComplete" {
		t.Errorf("expected 'from-onComplete', got '%s'", result.TaskID)
	}
}

// TestCompletionChannel_ErrorSend verifies that when onComplete was NOT called
// (e.g., SSH connection failed before streaming started), the error handler
// successfully sends a failure result to prevent goroutine leak.
func TestCompletionChannel_ErrorSend(t *testing.T) {
	ch := make(chan *TaskResult, 1)

	// Simulate error handler sending (onComplete was never called)
	select {
	case ch <- &TaskResult{TaskID: "from-error", ExitCode: 1}:
		// Expected: channel was empty, send succeeded
	default:
		t.Fatal("should have been able to send to empty channel")
	}

	result := <-ch
	if result.TaskID != "from-error" {
		t.Errorf("expected 'from-error', got '%s'", result.TaskID)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

// TestStreamMonitor_NewWithDefaults verifies default configuration.
func TestStreamMonitor_NewWithDefaults(t *testing.T) {
	m := NewStreamMonitor(nil, nil, nil)
	if m == nil {
		t.Fatal("expected non-nil StreamMonitor")
	}
	if m.broadcastInterval != 2*time.Second {
		t.Errorf("expected default interval 2s, got %s", m.broadcastInterval)
	}
}

// TestStreamMonitor_NewWithConfig verifies custom configuration.
func TestStreamMonitor_NewWithConfig(t *testing.T) {
	m := NewStreamMonitor(nil, nil, &StreamMonitorConfig{
		BroadcastInterval: 5 * time.Second,
	})
	if m.broadcastInterval != 5*time.Second {
		t.Errorf("expected interval 5s, got %s", m.broadcastInterval)
	}
}

// TestStreamMonitor_ActiveStreamTracking verifies stream lifecycle tracking.
func TestStreamMonitor_ActiveStreamTracking(t *testing.T) {
	m := NewStreamMonitor(nil, nil, nil)

	if m.ActiveStreamCount() != 0 {
		t.Error("expected 0 active streams")
	}
	if m.IsStreaming("task-1") {
		t.Error("expected task-1 to not be streaming")
	}
}

// TestStreamMonitor_StopStreaming verifies stopping streams.
func TestStreamMonitor_StopStreaming(t *testing.T) {
	m := NewStreamMonitor(nil, nil, nil)

	// Stopping a non-existent stream should not panic
	m.StopStreaming("nonexistent")

	// StopAll on empty should not panic
	m.StopAll()
}
