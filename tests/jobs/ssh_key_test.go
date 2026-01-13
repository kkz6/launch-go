package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)

func TestAddSshKeyPayload(t *testing.T) {
	payload := jobs.AddSshKeyPayload{
		ServerID: "server123",
		KeyID:    "key456",
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.KeyID != "key456" {
		t.Errorf("Expected KeyID to be 'key456', got '%s'", payload.KeyID)
	}
}

func TestNewAddSshKeyTask(t *testing.T) {
	task, err := jobs.NewAddSshKeyTask("server123", "key456")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeAddSshKey {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeAddSshKey, task.Type())
	}
}

func TestRemoveSshKeyPayload(t *testing.T) {
	payload := jobs.RemoveSshKeyPayload{
		ServerID: "server123",
		KeyID:    "key456",
		Force:    true,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if !payload.Force {
		t.Error("Expected Force to be true")
	}
}

func TestNewRemoveSshKeyTask(t *testing.T) {
	task, err := jobs.NewRemoveSshKeyTask("server123", "key456", true)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeRemoveSshKey {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeRemoveSshKey, task.Type())
	}
}
