package service

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewDependencies(t *testing.T) {
	logger := zerolog.Nop()

	deps := NewDependencies(nil, &logger, nil, nil, nil)

	if deps.Logger == nil {
		t.Error("Expected logger to be set")
	}
	if deps.DB != nil {
		t.Error("Expected DB to be nil")
	}
}

func TestNewBase(t *testing.T) {
	logger := zerolog.Nop()

	base := NewBase(nil, nil, &logger)

	if base.Logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestNewBaseFromDeps(t *testing.T) {
	logger := zerolog.Nop()
	deps := Dependencies{
		Logger: &logger,
	}

	base := NewBaseFromDeps(deps)

	if base.Logger == nil {
		t.Error("Expected logger to be set from deps")
	}
}

func TestBase_LogInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	base := NewBase(nil, nil, &logger)

	base.LogInfo("Test message", "key1", "value1")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Test message")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("key1")) {
		t.Errorf("Expected log to contain key1, got: %s", output)
	}
}

func TestBase_LogError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	base := NewBase(nil, nil, &logger)

	testErr := &testError{msg: "test error"}
	base.LogError(testErr, "Error occurred", "jobID", "12345")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Error occurred")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("test error")) {
		t.Errorf("Expected log to contain error, got: %s", output)
	}
}

func TestBase_LogWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	base := NewBase(nil, nil, &logger)

	base.LogWarn("Warning message", "count", 5)

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Warning message")) {
		t.Errorf("Expected log to contain message, got: %s", output)
	}
}

func TestBase_LogMethods_NilLogger(t *testing.T) {
	base := NewBase(nil, nil, nil)

	// These should not panic with nil logger
	base.LogInfo("Test")
	base.LogError(&testError{msg: "err"}, "Test")
	base.LogWarn("Test")
}

func TestBase_HasQueue(t *testing.T) {
	base := NewBase(nil, nil, nil)
	if base.HasQueue() {
		t.Error("Expected HasQueue to return false when queue is nil")
	}
}

func TestBase_HasWebsocket(t *testing.T) {
	base := NewBase(nil, nil, nil)
	if base.HasWebsocket() {
		t.Error("Expected HasWebsocket to return false when WS is nil")
	}
}

func TestBase_BroadcastMethods_NilWS(t *testing.T) {
	base := NewBase(nil, nil, nil)

	// These should not panic with nil WS
	base.BroadcastToTeam("team-123", "event", nil)
	base.BroadcastToServer("server-123", "event", nil)
	base.BroadcastToSite("site-123", "event", nil)
	base.BroadcastToDeployment("deployment-123", "event", nil)
	base.Broadcast("channel", "event", nil)
}

func TestBase_EnqueueTask_NilQueue(t *testing.T) {
	base := NewBase(nil, nil, nil)

	err := base.EnqueueTask(nil)
	if err != nil {
		t.Errorf("Expected nil error when queue is nil, got: %v", err)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
