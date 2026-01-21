package jobs

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewBase(t *testing.T) {
	logger := zerolog.Nop()
	deps := BaseDeps{
		Logger: &logger,
	}

	base := NewBase(deps)

	if base.Logger() == nil {
		t.Error("Expected logger to be set")
	}
}

func TestBase_LogInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	deps := BaseDeps{Logger: &logger}
	base := NewBase(deps)

	base.LogInfo("Test message", "key1", "value1", "key2", 123)

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Test message")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("key1")) {
		t.Errorf("Expected log to contain key1, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("value1")) {
		t.Errorf("Expected log to contain value1, got: %s", output)
	}
}

func TestBase_LogError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	deps := BaseDeps{Logger: &logger}
	base := NewBase(deps)

	testErr := &testError{msg: "test error"}
	base.LogError(testErr, "Error occurred", "jobID", "12345")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Error occurred")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("test error")) {
		t.Errorf("Expected log to contain error, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("jobID")) {
		t.Errorf("Expected log to contain jobID field, got: %s", output)
	}
}

func TestBase_LogError_NilError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	deps := BaseDeps{Logger: &logger}
	base := NewBase(deps)

	base.LogError(nil, "Warning message")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Warning message")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
}

func TestBase_LogWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	deps := BaseDeps{Logger: &logger}
	base := NewBase(deps)

	base.LogWarn("Warning message", "count", 5)

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Warning message")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("warn")) {
		t.Errorf("Expected log to contain warn level, got: %s", output)
	}
}

func TestBase_LogDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	deps := BaseDeps{Logger: &logger}
	base := NewBase(deps)

	base.LogDebug("Debug info", "data", "test")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Debug info")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
}

func TestBase_LogMethods_NilLogger(t *testing.T) {
	deps := BaseDeps{Logger: nil}
	base := NewBase(deps)

	// These should not panic with nil logger
	base.LogInfo("Test")
	base.LogError(&testError{msg: "err"}, "Test")
	base.LogWarn("Test")
	base.LogDebug("Test")
}

func TestBase_BroadcastToTeam(t *testing.T) {
	var called bool
	var capturedTeamID, capturedEvent string
	var capturedData any

	mock := &mockBroadcaster{
		broadcastFunc: func(teamID, event string, data any) {
			called = true
			capturedTeamID = teamID
			capturedEvent = event
			capturedData = data
		},
	}

	deps := BaseDeps{WS: mock}
	base := NewBase(deps)

	testData := map[string]string{"key": "value"}
	base.BroadcastToTeam("team-123", "test.event", testData)

	if !called {
		t.Error("Expected broadcaster to be called")
	}
	if capturedTeamID != "team-123" {
		t.Errorf("Expected teamID 'team-123', got '%s'", capturedTeamID)
	}
	if capturedEvent != "test.event" {
		t.Errorf("Expected event 'test.event', got '%s'", capturedEvent)
	}
	if capturedData == nil {
		t.Error("Expected data to be captured")
	}
}

func TestBase_BroadcastToTeam_NilBroadcaster(t *testing.T) {
	deps := BaseDeps{WS: nil}
	base := NewBase(deps)

	// Should not panic with nil broadcaster
	base.BroadcastToTeam("team-123", "test.event", nil)
}

func TestBase_Accessors(t *testing.T) {
	logger := zerolog.Nop()
	deps := BaseDeps{
		Logger: &logger,
	}
	base := NewBase(deps)

	if base.Logger() == nil {
		t.Error("Expected Logger() to return non-nil")
	}
	if base.DB() != nil {
		t.Error("Expected DB() to return nil when not set")
	}
	if base.WS() != nil {
		t.Error("Expected WS() to return nil when not set")
	}
	if base.Dispatcher() != nil {
		t.Error("Expected Dispatcher() to return nil when not set")
	}
	if base.Queue() != nil {
		t.Error("Expected Queue() to return nil when not set")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// mockBroadcaster implements broadcast.TeamBroadcaster for testing
type mockBroadcaster struct {
	broadcastFunc func(teamID, event string, data any)
}

func (m *mockBroadcaster) BroadcastToTeam(teamID, event string, data any) {
	if m.broadcastFunc != nil {
		m.broadcastFunc(teamID, event, data)
	}
}

func (m *mockBroadcaster) BroadcastToServer(serverID, event string, data any) {}
func (m *mockBroadcaster) BroadcastToSite(siteID, event string, data any)     {}
func (m *mockBroadcaster) BroadcastToDeployment(deploymentID, event string, data any) {
}
func (m *mockBroadcaster) BroadcastToUser(userID, event string, data any) {}
func (m *mockBroadcaster) Broadcast(channel, event string, data any)      {}
