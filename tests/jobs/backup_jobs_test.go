package jobs_test

import (
	"testing"

	backupjobs "github.com/kkz6/launch-go/internal/modules/backup/jobs"
)

// Test SyncServerLaunchConfig job

func TestSyncServerLaunchConfigPayload(t *testing.T) {
	userID := "user123"
	payload := backupjobs.SyncServerLaunchConfigPayload{
		ServerID: "server123",
		UserID:   &userID,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
	if payload.UserID == nil || *payload.UserID != "user123" {
		t.Error("Expected UserID to be 'user123'")
	}
}

func TestNewSyncServerLaunchConfigTask(t *testing.T) {
	task, err := backupjobs.NewSyncServerLaunchConfigTask("server123", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != backupjobs.TypeSyncServerLaunchConfig {
		t.Errorf("Expected task type to be '%s', got '%s'", backupjobs.TypeSyncServerLaunchConfig, task.Type())
	}
}

func TestNewSyncServerLaunchConfigTask_WithUser(t *testing.T) {
	userID := "user123"
	task, err := backupjobs.NewSyncServerLaunchConfigTask("server123", &userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
}
