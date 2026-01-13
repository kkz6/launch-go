package jobs_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)

func TestInstallFirewallRulePayload(t *testing.T) {
	userID := "user123"
	payload := jobs.InstallFirewallRulePayload{
		ServerID: "server123",
		RuleID:   "rule456",
		UserID:   &userID,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.RuleID != "rule456" {
		t.Errorf("Expected RuleID to be 'rule456', got '%s'", payload.RuleID)
	}
}

func TestNewInstallFirewallRuleTask(t *testing.T) {
	userID := "user123"
	task, err := jobs.NewInstallFirewallRuleTask("server123", "rule456", &userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeInstallFirewallRule {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeInstallFirewallRule, task.Type())
	}
}

func TestUninstallFirewallRulePayload(t *testing.T) {
	payload := jobs.UninstallFirewallRulePayload{
		ServerID: "server123",
		RuleID:   "rule456",
		UserID:   nil,
	}

	if payload.ServerID != "server123" {
		t.Errorf("Expected ServerID to be 'server123', got '%s'", payload.ServerID)
	}

	if payload.UserID != nil {
		t.Error("Expected UserID to be nil")
	}
}

func TestNewUninstallFirewallRuleTask(t *testing.T) {
	task, err := jobs.NewUninstallFirewallRuleTask("server123", "rule456", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if task.Type() != jobs.TypeUninstallFirewall {
		t.Errorf("Expected task type to be '%s', got '%s'", jobs.TypeUninstallFirewall, task.Type())
	}
}
