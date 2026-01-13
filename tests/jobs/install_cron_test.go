package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)

func TestInstallCronPayload(t *testing.T) {
	userID := "user123"
	payload := jobs.InstallCronPayload{
		ServerID: "server123",
		CronID:   "cron456",
		UserID:   &userID,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.CronID != "cron456" {
		t.Errorf("Expected CronID to be 'cron456', got '%s'", payload.CronID)
	}

	if payload.UserID == nil || *payload.UserID != "user123" {
		t.Error("Expected UserID to be 'user123'")
	}
}

func TestNewInstallCronTask(t *testing.T) {
	userID := "user123"
	task, err := jobs.NewInstallCronTask("server123", "cron456", &userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeInstallCron {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeInstallCron, task.Type())
	}
}

func TestNewInstallCronTask_WithoutUser(t *testing.T) {
	task, err := jobs.NewInstallCronTask("server123", "cron456", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}
}
