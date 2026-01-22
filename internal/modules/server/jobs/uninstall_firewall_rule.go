package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUninstallFirewall = "server:uninstall_firewall_rule"

type UninstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallFirewallRuleJob removes a firewall rule from a server.
// Similar to Laravel's Modules\Server\Jobs\UninstallFirewallRule
type UninstallFirewallRuleJob struct {
	pkgjobs.BaseJob[*JobContext, UninstallFirewallRulePayload]
}

// Handle processes the job
func (j *UninstallFirewallRuleJob) Handle(ctx context.Context) error {
	// Find the firewall rule with server preloaded
	rule, err := j.Ctx.Repos().FirewallRule().FindByIDWithServer(ctx, j.Payload.RuleID)
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

	result, err := j.Ctx.ForServer(rule.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to remove firewall rule: %s", result.GetOutput())
	}

	// Log activity before deletion
	activity.RecordWithLogPtr(ctx, "server", "uninstalled", j.Payload.UserID, rule, "Firewall rule was uninstalled")

	// Delete the rule record
	if err := j.Ctx.Repos().FirewallRule().Delete(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	j.Ctx.LogInfo("Firewall rule uninstalled successfully",
		"rule_id", rule.ID,
		"server_id", rule.ServerID,
	)

	// Broadcast event
	j.Ctx.BroadcastServerEvent(rule.Server, "firewall_rule.uninstalled", map[string]any{
		"rule_id":   rule.ID,
		"server_id": rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to uninstall firewall rule",
		"rule_id", j.Payload.RuleID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	if markErr := j.Ctx.Repos().FirewallRule().MarkFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.Ctx.LogError(markErr, "Failed to mark firewall rule failure")
	}
}

func NewUninstallFirewallRuleJob(ctx *JobContext, payload UninstallFirewallRulePayload) *UninstallFirewallRuleJob {
	return &UninstallFirewallRuleJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// NewUninstallFirewallRuleTask creates an asynq task for uninstalling a firewall rule
// Uses TaskID for deduplication to prevent duplicate firewall rule removals
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallFirewall, UninstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	}, asynq.TaskID(fmt.Sprintf("uninstall_firewall:%s:%s", serverID, ruleID)))
}
