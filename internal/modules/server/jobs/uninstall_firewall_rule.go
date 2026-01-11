package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeUninstallFirewallRule = "server:uninstall_firewall_rule"

// UninstallFirewallRulePayload contains data for uninstalling a firewall rule
type UninstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// UninstallFirewallRuleJob handles uninstalling a firewall rule from a server
type UninstallFirewallRuleJob struct {
	*JobContext
	jobs.UninstallationTracker
}

// NewUninstallFirewallRuleTask creates a new asynq task for uninstalling a firewall rule
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUninstallFirewallRule, UninstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
}

// Handle processes the uninstall firewall rule job
func (j *UninstallFirewallRuleJob) Handle(ctx context.Context, t *asynq.Task) error {
	payload, err := jobs.UnmarshalPayload[UninstallFirewallRulePayload](t)
	if err != nil {
		return err
	}

	j.Logger.Info().
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Uninstalling firewall rule")

	// Fetch the firewall rule with server
	rule, err := j.Repo.FindFirewallRuleByIDWithServer(ctx, payload.RuleID)
	if err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.RuleID, "uninstalling", fmt.Sprintf("Removing firewall rule: %s (port %s)", rule.Name, rule.Port))

	// TODO: Run the actual firewall rule uninstallation task
	// _, err = j.RunTask(rule.Server, tasks.NewRemoveFirewallRule(&rule)).
	//     AsRoot().
	//     Dispatch(ctx)
	// if err != nil {
	//     return err
	// }

	// Delete the firewall rule record
	if err := j.Repo.DeleteFirewallRule(ctx, rule.ID); err != nil {
		return fmt.Errorf("failed to delete firewall rule record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.RuleID, "uninstalled", "Firewall rule removed successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallFirewallRuleJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	payload, unmarshalErr := jobs.UnmarshalPayload[UninstallFirewallRulePayload](t)
	if unmarshalErr != nil {
		j.Logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.Logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Failed to uninstall firewall rule")

	// Fetch the rule for error message
	rule, findErr := j.Repo.FindFirewallRuleByID(ctx, payload.RuleID)
	if findErr != nil {
		j.broadcastProgress(payload.ServerID, payload.RuleID, "failed", "Failed to remove firewall rule")
		return
	}

	j.broadcastProgress(payload.ServerID, payload.RuleID, "failed", fmt.Sprintf("Failed to remove firewall rule: %s (port %s)", rule.Name, rule.Port))
}

func (j *UninstallFirewallRuleJob) broadcastProgress(serverID, ruleID, status, message string) {
	j.BroadcastToServer(serverID, "server.firewall.progress", map[string]interface{}{
		"server_id": serverID,
		"rule_id":   ruleID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
