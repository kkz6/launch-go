package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)

func TestAddSSHKeyPayload(t *testing.T) {
	payload := jobs.AddSSHKeyPayload{
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

func TestNewAddSSHKeyTask(t *testing.T) {
	task, err := jobs.NewAddSSHKeyTask("server123", "key456")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeAddSSHKey {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeAddSSHKey, task.Type())
	}
}

func TestRemoveSSHKeyPayload(t *testing.T) {
	payload := jobs.RemoveSSHKeyPayload{
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

func TestNewRemoveSSHKeyTask(t *testing.T) {
	task, err := jobs.NewRemoveSSHKeyTask("server123", "key456", true)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeRemoveSSHKey {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeRemoveSSHKey, task.Type())
	}
}
