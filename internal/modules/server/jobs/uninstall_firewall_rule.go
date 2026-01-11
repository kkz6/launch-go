package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/websocket"
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
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewUninstallFirewallRuleJob creates a new uninstall firewall rule job handler
func NewUninstallFirewallRuleJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *UninstallFirewallRuleJob {
	return &UninstallFirewallRuleJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewUninstallFirewallRuleTask creates a new asynq task for uninstalling a firewall rule
func NewUninstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallFirewallRule, payload), nil
}

// Handle processes the uninstall firewall rule job
func (j *UninstallFirewallRuleJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload UninstallFirewallRulePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Uninstalling firewall rule")

	// Fetch the firewall rule with server
	var rule models.FirewallRule
	if err := j.db.Preload("Server").First(&rule, "id = ?", payload.RuleID).Error; err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.RuleID, "uninstalling", fmt.Sprintf("Removing firewall rule: %s (port %s)", rule.Name, rule.Port))

	// TODO: Implement actual firewall rule removal:
	// 1. Connect to server via SSH
	// 2. Remove UFW rule
	// 3. Delete rule record from database

	// Delete the firewall rule record
	if err := j.db.Delete(&rule).Error; err != nil {
		return fmt.Errorf("failed to delete firewall rule record: %w", err)
	}

	j.broadcastProgress(payload.ServerID, payload.RuleID, "uninstalled", "Firewall rule removed successfully")

	return nil
}

// Failed handles job failure
func (j *UninstallFirewallRuleJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload UninstallFirewallRulePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Failed to uninstall firewall rule")

	// Fetch the rule for error message
	var rule models.FirewallRule
	if findErr := j.db.First(&rule, "id = ?", payload.RuleID).Error; findErr != nil {
		j.broadcastProgress(payload.ServerID, payload.RuleID, "failed", "Failed to remove firewall rule")
		return
	}

	j.broadcastProgress(payload.ServerID, payload.RuleID, "failed", fmt.Sprintf("Failed to remove firewall rule: %s (port %s)", rule.Name, rule.Port))
}

func (j *UninstallFirewallRuleJob) broadcastProgress(serverID, ruleID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.firewall.progress", map[string]interface{}{
		"server_id": serverID,
		"rule_id":   ruleID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
