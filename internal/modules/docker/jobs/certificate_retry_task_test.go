package jobs

import (
	"encoding/json"
	"testing"
)

func TestNewRetryTraefikConfigTaskForcesCertificateRetry(t *testing.T) {
	task, err := NewRetryTraefikConfigTask("app-1", "server-1", "team-1")
	if err != nil {
		t.Fatalf("create application retry task: %v", err)
	}

	var payload SyncTraefikConfigPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("decode application retry payload: %v", err)
	}
	if !payload.ForceCertificateRetry {
		t.Fatal("application retry task must force a certificate retry")
	}
}

func TestNewRetryComposeTraefikConfigTaskForcesCertificateRetry(t *testing.T) {
	task, err := NewRetryComposeTraefikConfigTask("compose-1", "server-1", "team-1")
	if err != nil {
		t.Fatalf("create compose retry task: %v", err)
	}

	var payload SyncComposeTraefikConfigPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatalf("decode compose retry payload: %v", err)
	}
	if !payload.ForceCertificateRetry {
		t.Fatal("compose retry task must force a certificate retry")
	}
}
