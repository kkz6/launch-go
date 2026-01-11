package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallFirewallRule = "server:install_firewall_rule"

// InstallFirewallRulePayload contains data for installing a firewall rule
type InstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallFirewallRuleJob handles installing a firewall rule on a server
type InstallFirewallRuleJob struct {
	*JobContext
	jobs.InstallationTracker
}

// NewInstallFirewallRuleTask creates a new asynq task for installing a firewall rule
func NewInstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallFirewallRule, InstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
}

// Handle processes the install firewall rule job
func (j *InstallFirewallRuleJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[InstallFirewallRulePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Installing firewall rule")

	// Fetch the firewall rule with server
	rule, err := j.Repo.FindFirewallRuleByIDWithServer(ctx, payload.RuleID)
	if err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing firewall rule: %s", rule.FormatAsUfwRule()))

	// Run the firewall rule installation task
	_, err = j.RunTask(rule.Server, tasks.NewAddFirewallRule(rule.Server, rule)).
		AsRoot().
		Throw().
		Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to add firewall rule: %w", err)
	}

	// Mark the rule as installed
	if err := j.Repo.MarkFirewallRuleInstalled(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to update rule status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installed", "Firewall rule installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallFirewallRuleJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[InstallFirewallRulePayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Failed to install firewall rule")

	// Fetch the rule for updating status
	rule, findErr := j.Repo.FindFirewallRuleByID(ctx, payload.RuleID)
	if findErr != nil {
		j.broadcastProgress(payload.ServerID, "failed", "Failed to install firewall rule")
		return
	}

	// Mark installation as failed
	j.Repo.MarkFirewallRuleFailed(ctx, rule.ID)

	j.broadcastProgress(payload.ServerID, "failed", "Failed to install firewall rule")
}

func (j *InstallFirewallRuleJob) broadcastProgress(serverID, status, message string) {
	j.BroadcastToServer(serverID, "server.firewall.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
