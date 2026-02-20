package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
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
	Deps    *JobDeps
	Payload UninstallFirewallRulePayload

	rule *models.FirewallRule
}

func NewUninstallFirewallRuleJob(p UninstallFirewallRulePayload) pkgjobs.Handler {
	return &UninstallFirewallRuleJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *UninstallFirewallRuleJob) Handle(ctx context.Context) error {
	var err error
	j.rule, err = j.Deps.Repos.FirewallRule().FindByIDWithServer(ctx, j.Payload.RuleID)
	if err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	// Delete firewall rule
	fromIP := ""
	if j.rule.FromIPv4 != nil {
		fromIP = *j.rule.FromIPv4
	}
	task := tasks.DeleteFirewallRule(
		tasks.FirewallAction(j.rule.Action),
		j.rule.Port,
		"", // protocol is not stored in model
		fromIP,
	)

	result, err := j.Deps.RunTask(j.rule.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to remove firewall rule: %s", result.GetOutput())
	}

	// Log activity before deletion
	activity.RecordWithLogPtr(ctx, "server", "uninstalled", j.Payload.UserID, j.rule, "Firewall rule was uninstalled")

	// Delete the rule record
	if err := j.Deps.Repos.FirewallRule().Delete(ctx, j.rule.ID); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	j.Deps.Logger.Info().
		Str("rule_id", j.rule.ID).
		Str("server_id", j.rule.ServerID).
		Msg("Firewall rule uninstalled successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.rule.Server, "firewall_rule.uninstalled", map[string]any{
		"rule_id":   j.rule.ID,
		"server_id": j.rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *UninstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("rule_id", j.Payload.RuleID).
		Str("server_id", j.Payload.ServerID).
		Msg("Failed to uninstall firewall rule")

	// Mark uninstallation as failed (clears uninstallation_requested_at, sets uninstallation_failed_at)
	if markErr := j.Deps.Repos.FirewallRule().MarkUninstallationFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).Msg("Failed to mark firewall rule uninstallation failure")
	}
}

// NewUninstallFirewallRuleTask creates an asynq task for uninstalling a firewall rule
// Uses TaskID for deduplication to prevent duplicate firewall rule removals
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeUninstallFirewall,
		UninstallFirewallRulePayload{ServerID: serverID, RuleID: ruleID, UserID: userID},
		pkgjobs.Dedup("uninstall_firewall", serverID, ruleID),
	)
}
