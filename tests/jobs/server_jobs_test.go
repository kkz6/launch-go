package jobs_test

import (
	"testing"

	serverjobs "github.com/kkz6/launch-go/internal/modules/server/jobs"
)

// Test CleanupFailedPhpInstallation job

func TestCleanupFailedPhpInstallationPayload(t *testing.T) {
	userID := "user123"
	payload := serverjobs.CleanupFailedPhpInstallationPayload{
		ServerID:  "server123",
		Version:   "8.2",
		ServiceID: "service456",
		UserID:    &userID,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
	if payload.Version != "8.2" {
		t.Errorf("Expected Version to be '8.2', got '%s'", payload.Version)
	}
	if payload.ServiceID != "service456" {
		t.Errorf("Expected ServiceID to be 'service456', got '%s'", payload.ServiceID)
	}
	if payload.UserID == nil || *payload.UserID != "user123" {
		t.Error("Expected UserID to be 'user123'")
	}
}

func TestNewCleanupFailedPhpInstallationTask(t *testing.T) {
	userID := "user123"
	task, err := serverjobs.NewCleanupFailedPhpInstallationTask("server123", "8.2", "service456", &userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeCleanupFailedPhpInstallation {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeCleanupFailedPhpInstallation, task.Type())
	}
}

// Test CleanupFailedPhpExtensionInstall job

func TestCleanupFailedPhpExtensionInstallPayload(t *testing.T) {
	payload := serverjobs.CleanupFailedPhpExtensionInstallPayload{
		ServerID:  "server123",
		Version:   "8.2",
		Extension: "redis",
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
	if payload.Version != "8.2" {
		t.Errorf("Expected Version to be '8.2', got '%s'", payload.Version)
	}
	if payload.Extension != "redis" {
		t.Errorf("Expected Extension to be 'redis', got '%s'", payload.Extension)
	}
}

func TestNewCleanupFailedPhpExtensionInstallTask(t *testing.T) {
	task, err := serverjobs.NewCleanupFailedPhpExtensionInstallTask("server123", "8.2", "redis", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeCleanupFailedPhpExtensionInstall {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeCleanupFailedPhpExtensionInstall, task.Type())
	}
}

// Test CleanupFailedPhpExtensionUninstall job

func TestCleanupFailedPhpExtensionUninstallPayload(t *testing.T) {
	payload := serverjobs.CleanupFailedPhpExtensionUninstallPayload{
		ServerID:  "server123",
		Version:   "8.1",
		Extension: "imagick",
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
	if payload.Version != "8.1" {
		t.Errorf("Expected Version to be '8.1', got '%s'", payload.Version)
	}
	if payload.Extension != "imagick" {
		t.Errorf("Expected Extension to be 'imagick', got '%s'", payload.Extension)
	}
}

func TestNewCleanupFailedPhpExtensionUninstallTask(t *testing.T) {
	task, err := serverjobs.NewCleanupFailedPhpExtensionUninstallTask("server123", "8.1", "imagick", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeCleanupFailedPhpExtensionUninstall {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeCleanupFailedPhpExtensionUninstall, task.Type())
	}
}

// Test CheckDaemonStatus job

func TestCheckDaemonStatusPayload(t *testing.T) {
	payload := serverjobs.CheckDaemonStatusPayload{
		ServerID: "server123",
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
}

func TestNewCheckDaemonStatusTask(t *testing.T) {
	task, err := serverjobs.NewCheckDaemonStatusTask("server123", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeCheckDaemonStatus {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeCheckDaemonStatus, task.Type())
	}
}

// Test UpdateTaskOutput job

func TestUpdateTaskOutputPayload(t *testing.T) {
	payload := serverjobs.UpdateTaskOutputPayload{
		TaskID:   "task123",
		ServerID: "server123",
		Output:   "Command completed successfully",
		Append:   true,
	}

	if payload.TaskID != "task123" {
		t.Errorf("Expected TaskID to be 'task123', got '%s'", payload.TaskID)
	}
	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
	if payload.Output != "Command completed successfully" {
		t.Errorf("Expected Output to be 'Command completed successfully', got '%s'", payload.Output)
	}
	if !payload.Append {
		t.Error("Expected Append to be true")
	}
}

func TestNewUpdateTaskOutputTask(t *testing.T) {
	task, err := serverjobs.NewUpdateTaskOutputTask("task123", "server123", "output text", true)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeUpdateTaskOutput {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeUpdateTaskOutput, task.Type())
	}
}

// Test UpdateUserPublicKey job

func TestUpdateUserPublicKeyPayload(t *testing.T) {
	payload := serverjobs.UpdateUserPublicKeyPayload{
		ServerID:  "server123",
		Username:  "launcher",
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAA...",
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}
	if payload.Username != "launcher" {
		t.Errorf("Expected Username to be 'launcher', got '%s'", payload.Username)
	}
	if payload.PublicKey != "ssh-rsa AAAAB3NzaC1yc2EAAA..." {
		t.Errorf("Expected PublicKey to be set")
	}
}

func TestNewUpdateUserPublicKeyTask(t *testing.T) {
	task, err := serverjobs.NewUpdateUserPublicKeyTask("server123", "launcher", "ssh-rsa key")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeUpdateUserPublicKey {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeUpdateUserPublicKey, task.Type())
	}
}

// Test InstallTaskCleanupCron job

func TestInstallTaskCleanupCronPayload(t *testing.T) {
	userID := "user123"
	payload := serverjobs.InstallTaskCleanupCronPayload{
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

func TestNewInstallTaskCleanupCronTask(t *testing.T) {
	task, err := serverjobs.NewInstallTaskCleanupCronTask("server123", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeInstallTaskCleanupCron {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeInstallTaskCleanupCron, task.Type())
	}
}

// Test RunAfterUpdate job

func TestRunAfterUpdatePayload(t *testing.T) {
	userID := "user123"
	payload := serverjobs.RunAfterUpdatePayload{
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

func TestNewRunAfterUpdateTask(t *testing.T) {
	task, err := serverjobs.NewRunAfterUpdateTask("server123", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	if task.Type() != serverjobs.TypeRunAfterUpdate {
		t.Errorf("Expected task type to be '%s', got '%s'", serverjobs.TypeRunAfterUpdate, task.Type())
	}
}
