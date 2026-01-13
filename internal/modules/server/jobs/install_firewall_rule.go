package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// InstallFirewallRuleJob installs a firewall rule on a server.
// Similar to Laravel's Modules\Server\Jobs\InstallFirewallRule
type InstallFirewallRuleJob struct {
	ServerJobBase
	jobs.InstallationTracker
	Payload InstallFirewallRulePayload
}

// Type returns the job type identifier
func (j *InstallFirewallRuleJob) Type() string {
	return TypeInstallFirewallRule
}

// Handle processes the job
func (j *InstallFirewallRuleJob) Handle(ctx context.Context) error {
	// Find the firewall rule with server preloaded
	rule, err := j.Repo().FindFirewallRuleByIDWithServer(ctx, j.Payload.RuleID)
	if err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	// Create and run the firewall task
	fromIP := ""
	if rule.FromIPv4 != nil {
		fromIP = *rule.FromIPv4
	}
	task := tasks.AddFirewallRule(
		tasks.FirewallAction(rule.Action),
		rule.Port,
		"", // protocol is not stored in model, defaults to both tcp/udp
		fromIP,
	)

	result, err := j.RunTaskOnServer(rule.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to apply firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to apply firewall rule: %s", result.GetOutput())
	}

	// Mark as installed
	if err := j.Repo().MarkFirewallRuleInstalled(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to mark rule as installed: %w", err)
	}

	j.LogInfo("Firewall rule installed successfully",
		"rule_id", rule.ID,
		"server_id", rule.ServerID,
		"port", rule.Port,
	)

	// Broadcast event
	j.BroadcastServerEvent(rule.ServerID, "firewall_rule.installed", map[string]interface{}{
		"rule_id":   rule.ID,
		"server_id": rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *InstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install firewall rule",
		"rule_id", j.Payload.RuleID,
		"server_id", j.Payload.ServerID,
	)

	// Mark installation as failed
	if markErr := j.Repo().MarkFirewallRuleFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.LogError(markErr, "Failed to mark firewall rule as failed")
	}
}

// NewInstallFirewallRuleTask creates an asynq task for installing a firewall rule
func NewInstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallFirewallRule, InstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
}
