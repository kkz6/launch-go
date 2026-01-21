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
	base.BroadcastToUser("user-123", "event", nil)
	base.Broadcast("channel", "event", nil)
}

func TestBase_EnqueueTask_NilQueue(t *testing.T) {
	base := NewBase(nil, nil, nil)

	err := base.EnqueueTask(nil)
	if err != nil {
		t.Errorf("Expected nil error when queue is nil, got: %v", err)
	}
}

func TestBase_HasDB(t *testing.T) {
	base := NewBase(nil, nil, nil)
	if base.HasDB() {
		t.Error("Expected HasDB to return false when db is nil")
	}
}

func TestBase_DB(t *testing.T) {
	base := NewBase(nil, nil, nil)
	if base.DB() != nil {
		t.Error("Expected DB to return nil when db is not set")
	}
}

func TestNewBaseWithDB(t *testing.T) {
	logger := zerolog.Nop()

	base := NewBaseWithDB(nil, nil, nil, &logger)

	if base.Logger == nil {
		t.Error("Expected logger to be set")
	}
	if base.HasDB() {
		t.Error("Expected HasDB to return false when db is nil")
	}
}

func TestBase_ActivityLogger_NilDB(t *testing.T) {
	base := NewBase(nil, nil, nil)

	logger := base.ActivityLogger()
	if logger != nil {
		t.Error("Expected ActivityLogger to return nil when db is nil")
	}
}

func TestBase_LogActivity_NilDB(t *testing.T) {
	base := NewBase(nil, nil, nil)

	// This should not panic with nil db
	base.LogActivity(nil, "server", "created", "Test message", &testSubject{})
}

func TestBase_LogActivityByUser_NilDB(t *testing.T) {
	base := NewBase(nil, nil, nil)

	// This should not panic with nil db
	base.LogActivityByUser(nil, "user-123", "server", "created", "Test message", &testSubject{})
}

func TestBase_LogActivityWithProps_NilDB(t *testing.T) {
	base := NewBase(nil, nil, nil)

	// This should not panic with nil db
	base.LogActivityWithProps(nil, "server", "created", "Test message", &testSubject{}, map[string]any{
		"key": "value",
	})
}

func TestBase_LogActivityByUserWithProps_NilDB(t *testing.T) {
	base := NewBase(nil, nil, nil)

	// This should not panic with nil db
	base.LogActivityByUserWithProps(nil, "user-123", "server", "created", "Test message", &testSubject{}, map[string]any{
		"key": "value",
	})
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// testSubject implements activity.Subject for testing
type testSubject struct{}

func (s *testSubject) GetID() string {
	return "test-id"
}

func (s *testSubject) GetTeamID() string {
	return "test-team-id"
}
