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

const TypeInstallFirewallRule = "server:install_firewall_rule"

type InstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallFirewallRuleJob installs a firewall rule on a server.
// Similar to Laravel's Modules\Server\Jobs\InstallFirewallRule
type InstallFirewallRuleJob struct {
	Deps    *JobDeps
	Payload InstallFirewallRulePayload

	rule *models.FirewallRule
}

func NewInstallFirewallRuleJob(p InstallFirewallRulePayload) pkgjobs.Handler {
	return &InstallFirewallRuleJob{Deps: deps, Payload: p}
}

// Handle processes the job
func (j *InstallFirewallRuleJob) Handle(ctx context.Context) error {
	var err error

	// Find the firewall rule with server preloaded
	j.rule, err = j.Deps.Repos.FirewallRule().FindByIDWithServer(ctx, j.Payload.RuleID)
	if err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	// Create and run the firewall task
	fromIP := ""
	if j.rule.FromIPv4 != nil {
		fromIP = *j.rule.FromIPv4
	}
	task := tasks.AddFirewallRule(
		tasks.FirewallAction(j.rule.Action),
		j.rule.Port,
		"", // protocol is not stored in model, defaults to both tcp/udp
		fromIP,
	)

	result, err := j.Deps.RunTask(j.rule.Server, task).
		AsRoot().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to apply firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to apply firewall rule: %s", result.GetOutput())
	}

	// Mark as installed
	if err := j.Deps.Repos.FirewallRule().MarkAsInstalled(ctx, j.rule.ID); err != nil {
		return fmt.Errorf("failed to mark rule as installed: %w", err)
	}

	// Log activity
	activity.RecordWithLogPtr(ctx, "server", "installed", j.Payload.UserID, j.rule, "Firewall rule was installed")

	j.Deps.Logger.Info().
		Str("rule_id", j.rule.ID).
		Str("server_id", j.rule.ServerID).
		Str("port", j.rule.Port).
		Msg("firewall rule installed successfully")

	// Broadcast event
	j.Deps.BroadcastServerEvent(j.rule.Server, "firewall_rule.installed", map[string]any{
		"rule_id":   j.rule.ID,
		"server_id": j.rule.ServerID,
	})

	return nil
}

// Failed is called when the job fails after all retries
func (j *InstallFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("rule_id", j.Payload.RuleID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to install firewall rule")

	// Mark installation as failed
	if markErr := j.Deps.Repos.FirewallRule().MarkAsFailed(ctx, j.Payload.RuleID); markErr != nil {
		j.Deps.Logger.Error().Err(markErr).Msg("failed to mark firewall rule as failed")
	}
}

// NewInstallFirewallRuleTask creates an asynq task for installing a firewall rule
// Uses TaskID for deduplication to prevent duplicate firewall rule installations
func NewInstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallFirewallRule, InstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	}, asynq.TaskID(pkgjobs.Dedup("install_firewall", serverID, ruleID)))
}
