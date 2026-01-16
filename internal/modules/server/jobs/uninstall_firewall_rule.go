package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	ctx     *JobContext
	Payload UninstallFirewallRulePayload
}

// Handle processes the job
func (j *UninstallFirewallRuleJob) Handle(ctx context.Context) error {
	// Find the firewall rule with server preloaded
	rule, err := j.ctx.Repo.FindFirewallRuleByIDWithServer(ctx, j.Payload.RuleID)
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

	result, err := j.ctx.ForServer(rule.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to remove firewall rule: %s", result.GetOutput())
	}

	// Log activity before deletion
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(rule).
		WithEvent("uninstalled")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Firewall rule was uninstalled")

	// Delete the rule record
	if err := j.ctx.Repo.DeleteFirewallRule(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	j.ctx.LogInfo("Firewall rule uninstalled successfully",
		"rule_id", rule.ID,
		"server_id", rule.ServerID,
	)

	// Broadcast event
	j.ctx.BroadcastServerEvent(rule.Server, "firewall_rule.uninstalled", map[string]any{
		"rule_id":   rule.ID,
		"server_id": rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to uninstall firewall rule",
		"rule_id", j.Payload.RuleID,
		"server_id", j.Payload.ServerID,
	)

	// Mark uninstallation as failed
	if markErr := j.ctx.Repo.MarkFirewallRuleFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.ctx.LogError(markErr, "Failed to mark firewall rule failure")
	}
}

func NewUninstallFirewallRuleJob(ctx *JobContext, payload UninstallFirewallRulePayload) *UninstallFirewallRuleJob {
	return &UninstallFirewallRuleJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewUninstallFirewallRuleTask creates an asynq task for uninstalling a firewall rule
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeUninstallFirewall, UninstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
}
