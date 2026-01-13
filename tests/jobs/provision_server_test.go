package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)

func TestProvisionServerPayload(t *testing.T) {
	userID := "user123"
	payload := jobs.ProvisionServerPayload{
		ServerID:  "server123",
		TeamID:    "team456",
		UserID:    &userID,
		SSHKeyIDs: []string{"key1", "key2"},
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.TeamID != "team456" {
		t.Errorf("Expected TeamID to be 'team456', got '%s'", payload.TeamID)
	}

	if len(payload.SSHKeyIDs) != 2 {
		t.Errorf("Expected 2 SSH key IDs, got %d", len(payload.SSHKeyIDs))
	}
}

func TestNewProvisionServerTask(t *testing.T) {
	userID := "user123"
	sshKeyIDs := []string{"key1", "key2"}

	task, err := jobs.NewProvisionServerTask("server123", "team456", &userID, sshKeyIDs)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeProvisionServer {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeProvisionServer, task.Type())
	}
}

func TestNewProvisionServerTask_MinimalConfig(t *testing.T) {
	task, err := jobs.NewProvisionServerTask("server123", "team456", nil, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}
}
