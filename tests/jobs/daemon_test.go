package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)

func TestInstallDaemonPayload(t *testing.T) {
	userID := "user123"
	payload := jobs.InstallDaemonPayload{
		ServerID: "server123",
		DaemonID: "daemon456",
		UserID:   &userID,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.DaemonID != "daemon456" {
		t.Errorf("Expected DaemonID to be 'daemon456', got '%s'", payload.DaemonID)
	}
}

func TestNewInstallDaemonTask(t *testing.T) {
	userID := "user123"
	task, err := jobs.NewInstallDaemonTask("server123", "daemon456", &userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeInstallDaemon {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeInstallDaemon, task.Type())
	}
}

func TestUninstallDaemonPayload(t *testing.T) {
	payload := jobs.UninstallDaemonPayload{
		ServerID: "server123",
		DaemonID: "daemon456",
		UserID:   nil,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.DaemonID != "daemon456" {
		t.Errorf("Expected DaemonID to be 'daemon456', got '%s'", payload.DaemonID)
	}
}

func TestNewUninstallDaemonTask(t *testing.T) {
	task, err := jobs.NewUninstallDaemonTask("server123", "daemon456", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeUninstallDaemon {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeUninstallDaemon, task.Type())
	}
}

func TestRestartDaemonPayload(t *testing.T) {
	userID := "user123"
	payload := jobs.RestartDaemonPayload{
		ServerID: "server123",
		DaemonID: "daemon456",
		UserID:   &userID,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
}

func TestNewRestartDaemonTask(t *testing.T) {
	task, err := jobs.NewRestartDaemonTask("server123", "daemon456", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeRestartDaemon {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeRestartDaemon, task.Type())
	}
}
