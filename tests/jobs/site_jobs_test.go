package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	sitejobs "github.com/kkz6/launch-go/internal/modules/site/jobs"
)

// Test CreateDeployment job

func TestCreateDeploymentPayload(t *testing.T) {
	userID := "user123"
	gitHash := "abc123def"
	branch := "main"
	payload := sitejobs.CreateDeploymentPayload{
		SiteID:  "site123",
		UserID:  &userID,
		GitHash: &gitHash,
		Branch:  &branch,
	}

	if payload.SiteID != "site123" {
		t.Errorf("Expected SiteID to be 'site123', got '%s'", payload.SiteID)
	}
	if payload.UserID == nil || *payload.UserID != "user123" {
		t.Error("Expected UserID to be 'user123'")
	}
	if payload.GitHash == nil || *payload.GitHash != "abc123def" {
		t.Error("Expected GitHash to be 'abc123def'")
	}
	if payload.Branch == nil || *payload.Branch != "main" {
		t.Error("Expected Branch to be 'main'")
	}
}

func TestNewCreateDeploymentTask(t *testing.T) {
	userID := "user123"
	gitHash := "abc123"
	branch := "develop"
	task, err := sitejobs.NewCreateDeploymentTask("site123", &userID, &gitHash, &branch)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != sitejobs.TypeCreateDeployment {
		t.Errorf("Expected task type to be '%s', got '%s'", sitejobs.TypeCreateDeployment, task.Type())
	}
}

func TestNewCreateDeploymentTask_WithoutOptionalFields(t *testing.T) {
	task, err := sitejobs.NewCreateDeploymentTask("site123", nil, nil, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
}

// Test UpdateSiteTLSSetting job

func TestUpdateSiteTLSSettingPayload(t *testing.T) {
	userID := "user123"
	payload := sitejobs.UpdateSiteTLSSettingPayload{
		SiteID:     "site123",
		TLSSetting: enums.TlsSettingAuto,
		UserID:     &userID,
	}

	if payload.SiteID != "site123" {
		t.Errorf("Expected SiteID to be 'site123', got '%s'", payload.SiteID)
	}
	if payload.TLSSetting != enums.TlsSettingAuto {
		t.Errorf("Expected TLSSetting to be 'auto', got '%s'", payload.TLSSetting)
	}
	if payload.UserID == nil || *payload.UserID != "user123" {
		t.Error("Expected UserID to be 'user123'")
	}
}

func TestNewUpdateSiteTLSSettingTask(t *testing.T) {
	task, err := sitejobs.NewUpdateSiteTLSSettingTask("site123", enums.TlsSettingAuto, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != sitejobs.TypeUpdateSiteTLSSetting {
		t.Errorf("Expected task type to be '%s', got '%s'", sitejobs.TypeUpdateSiteTLSSetting, task.Type())
	}
}

func TestNewUpdateSiteTLSSettingTask_AllTlsSettings(t *testing.T) {
	testCases := []enums.TlsSetting{
		enums.TlsSettingAuto,
		enums.TlsSettingCustom,
		enums.TlsSettingInternal,
		enums.TlsSettingOff,
	}

	for _, tlsSetting := range testCases {
		task, err := sitejobs.NewUpdateSiteTLSSettingTask("site123", tlsSetting, nil)
		if err != nil {
			t.Fatalf("Expected no error for TLS setting %s, got %v", tlsSetting, err)
		}
		if task == nil {
			t.Fatalf("Expected task to be created for TLS setting %s", tlsSetting)
		}
	}
}

// Test CleanupPendingSiteDeployment job

func TestCleanupPendingSiteDeploymentPayload(t *testing.T) {
	userID := "user123"
	payload := sitejobs.CleanupPendingSiteDeploymentPayload{
		SiteID:       "site123",
		DeploymentID: "deploy456",
		UserID:       &userID,
	}

	if payload.SiteID != "site123" {
		t.Errorf("Expected SiteID to be 'site123', got '%s'", payload.SiteID)
	}
	if payload.DeploymentID != "deploy456" {
		t.Errorf("Expected DeploymentID to be 'deploy456', got '%s'", payload.DeploymentID)
	}
	if payload.UserID == nil || *payload.UserID != "user123" {
		t.Error("Expected UserID to be 'user123'")
	}
}

func TestNewCleanupPendingSiteDeploymentTask(t *testing.T) {
	task, err := sitejobs.NewCleanupPendingSiteDeploymentTask("site123", "deploy456", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != sitejobs.TypeCleanupPendingSiteDeployment {
		t.Errorf("Expected task type to be '%s', got '%s'", sitejobs.TypeCleanupPendingSiteDeployment, task.Type())
	}
}

func TestNewCleanupPendingSiteDeploymentTask_WithUser(t *testing.T) {
	userID := "user123"
	task, err := sitejobs.NewCleanupPendingSiteDeploymentTask("site123", "deploy456", &userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
}

func TestNewCleanupPendingSiteDeploymentTaskDelayed(t *testing.T) {
	task, err := sitejobs.NewCleanupPendingSiteDeploymentTaskDelayed("site123", "deploy456", nil, 0)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != sitejobs.TypeCleanupPendingSiteDeployment {
		t.Errorf("Expected task type to be '%s', got '%s'", sitejobs.TypeCleanupPendingSiteDeployment, task.Type())
	}
}
