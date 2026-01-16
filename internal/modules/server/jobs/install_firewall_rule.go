package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallFirewallRule = "server:install_firewall_rule"

type InstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallFirewallRuleJob installs a firewall rule on a server.
// Similar to Laravel's Modules\Server\Jobs\InstallFirewallRule
type InstallFirewallRuleJob struct {
	ctx     *JobContext
	Payload InstallFirewallRulePayload
}

// Handle processes the job
func (j *InstallFirewallRuleJob) Handle(ctx context.Context) error {
	// Find the firewall rule with server preloaded
	rule, err := j.ctx.Repo.FindFirewallRuleByIDWithServer(ctx, j.Payload.RuleID)
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

	result, err := j.ctx.ForServer(rule.Server).RunTask(task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to apply firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to apply firewall rule: %s", result.GetOutput())
	}

	// Mark as installed
	if err := j.ctx.Repo.MarkFirewallRuleInstalled(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to mark rule as installed: %w", err)
	}

	// Log activity
	logger := activity.New(j.ctx.DB).
		WithContext(ctx).
		UseLog("server").
		On(rule).
		WithEvent("installed")
	if j.Payload.UserID != nil {
		logger.CausedByUser(*j.Payload.UserID)
	}
	logger.Log("Firewall rule was installed")

	j.ctx.LogInfo("Firewall rule installed successfully",
		"rule_id", rule.ID,
		"server_id", rule.ServerID,
		"port", rule.Port,
	)

	// Broadcast event
	j.ctx.BroadcastToServer(rule.ServerID, "firewall_rule.installed", map[string]any{
		"rule_id":   rule.ID,
		"server_id": rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *InstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Failed to install firewall rule",
		"rule_id", j.Payload.RuleID,
		"server_id", j.Payload.ServerID,
	)

	// Mark installation as failed
	if markErr := j.ctx.Repo.MarkFirewallRuleFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.ctx.LogError(markErr, "Failed to mark firewall rule as failed")
	}
}

func NewInstallFirewallRuleJob(ctx *JobContext, payload InstallFirewallRulePayload) *InstallFirewallRuleJob {
	return &InstallFirewallRuleJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// NewInstallFirewallRuleTask creates an asynq task for installing a firewall rule
func NewInstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallFirewallRule, InstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
}
