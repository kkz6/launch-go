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

const TypeInstallFirewallRule = "server:install_firewall_rule"

// InstallFirewallRulePayload contains data for installing a firewall rule
type InstallFirewallRulePayload struct {
	ServerID string  `json:"server_id"`
	RuleID   string  `json:"rule_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// InstallFirewallRuleJob handles installing a firewall rule on a server
type InstallFirewallRuleJob struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewInstallFirewallRuleJob creates a new install firewall rule job handler
func NewInstallFirewallRuleJob(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *InstallFirewallRuleJob {
	return &InstallFirewallRuleJob{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// NewInstallFirewallRuleTask creates a new asynq task for installing a firewall rule
func NewInstallFirewallRuleTask(serverID, ruleID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallFirewallRulePayload{
		ServerID: serverID,
		RuleID:   ruleID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallFirewallRule, payload), nil
}

// Handle processes the install firewall rule job
func (j *InstallFirewallRuleJob) Handle(ctx context.Context, t *asynq.Task) error {
	var payload InstallFirewallRulePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	j.logger.Info().
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Installing firewall rule")

	// Fetch the firewall rule with server
	var rule models.FirewallRule
	if err := j.db.Preload("Server").First(&rule, "id = ?", payload.RuleID).Error; err != nil {
		return fmt.Errorf("failed to find firewall rule: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installing", fmt.Sprintf("Installing firewall rule: %s", rule.FormatAsUfwRule()))

	// TODO: Implement actual firewall rule installation:
	// 1. Connect to server via SSH
	// 2. Run AddFirewallRule task script
	// 3. Execute ufw command

	// Mark the rule as installed
	now := time.Now()
	if err := j.db.Model(&rule).Update("installed_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update rule status: %w", err)
	}

	j.broadcastProgress(payload.ServerID, "installed", "Firewall rule installed successfully")

	return nil
}

// Failed handles job failure
func (j *InstallFirewallRuleJob) Failed(ctx context.Context, t *asynq.Task, err error) {
	var payload InstallFirewallRulePayload
	if unmarshalErr := json.Unmarshal(t.Payload(), &payload); unmarshalErr != nil {
		j.logger.Error().Err(unmarshalErr).Msg("Failed to unmarshal payload in failure handler")
		return
	}

	j.logger.Error().
		Err(err).
		Str("server_id", payload.ServerID).
		Str("rule_id", payload.RuleID).
		Msg("Failed to install firewall rule")

	// Fetch the rule for updating status
	var rule models.FirewallRule
	if findErr := j.db.First(&rule, "id = ?", payload.RuleID).Error; findErr != nil {
		j.broadcastProgress(payload.ServerID, "failed", "Failed to install firewall rule")
		return
	}

	// Mark installation as failed
	now := time.Now()
	j.db.Model(&rule).Update("installation_failed_at", &now)

	j.broadcastProgress(payload.ServerID, "failed", "Failed to install firewall rule")
}

func (j *InstallFirewallRuleJob) broadcastProgress(serverID, status, message string) {
	j.ws.BroadcastToServer(serverID, "server.firewall.progress", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"message":   message,
	})
}
