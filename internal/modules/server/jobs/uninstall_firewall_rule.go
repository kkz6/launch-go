package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// UninstallFirewallRuleJob removes a firewall rule from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallFirewallRule
type UninstallFirewallRuleJob struct {
	ServerJobBase
	jobs.UninstallationTracker
	Payload UninstallFirewallRulePayload
}

// Type returns the job type identifier
func (j *UninstallFirewallRuleJob) Type() string {
	return TypeUninstallFirewall
}

// Handle processes the job
func (j *UninstallFirewallRuleJob) Handle(ctx context.Context) error {
	// Find the firewall rule with server preloaded
	rule, err := j.Repo().FindFirewallRuleByIDWithServer(ctx, j.Payload.RuleID)
	if err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	// Delete firewall rule
	fromIP := ""
	if rule.FromIPv4 != nil {
		fromIP = *rule.FromIPv4
	}
	task := tasks.DeleteFirewallRule(
		tasks.FirewallAction(rule.Action),
		rule.Port,
		"", // protocol is not stored in model
		fromIP,
	)

	result, err := j.RunTaskOnServer(rule.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to remove firewall rule: %s", result.GetOutput())
	}

	// Delete the rule record
	if err := j.Repo().DeleteFirewallRule(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	j.LogInfo("Firewall rule uninstalled successfully",
		"rule_id", rule.ID,
		"server_id", rule.ServerID,
	)

	// Broadcast event
	j.BroadcastServerEvent(rule.ServerID, "firewall_rule.uninstalled", map[string]interface{}{
		"rule_id":   rule.ID,
		"server_id": rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to uninstall firewall rule",
		"rule_id", j.Payload.RuleID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	if markErr := j.Repo().MarkFirewallRuleFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.LogError(markErr, "Failed to mark firewall rule failure")
	}
}

// NewUninstallFirewallRuleTask creates an asynq task for uninstalling a firewall rule
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallFirewall, UninstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
}
