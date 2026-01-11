package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListFirewallRules returns all firewall rules for a server
func (s *Service) ListFirewallRules(ctx context.Context, serverID, teamID string) ([]models.FirewallRule, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindFirewallRulesByServer(ctx, serverID)
}

// CreateFirewallRule creates a new firewall rule
func (s *Service) CreateFirewallRule(ctx context.Context, serverID, teamID string, req *dto.CreateFirewallRuleRequest) (*models.FirewallRule, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	action, err := enums.ParseRuleAction(req.Action)
	if err != nil {
		return nil, err
	}

	port := ""
	if req.Port != nil {
		port = *req.Port
	}

	rule := &models.FirewallRule{
		ServerID: serverID,
		Name:     req.Name,
		Action:   action,
		Port:     port,
		FromIPv4: req.FromIPv4,
		Mask:     req.Mask,
		Note:     req.Note,
	}

	if err := s.repo.CreateFirewallRule(ctx, rule); err != nil {
		return nil, err
	}

	if server.IsProvisioned() {
		if err := s.dispatchFirewallRuleInstallJob(server, rule); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("rule_id", rule.ID).
				Msg("Failed to dispatch firewall rule install job")
		}
	}

	return rule, nil
}

// UpdateFirewallRule updates a firewall rule
func (s *Service) UpdateFirewallRule(ctx context.Context, serverID, teamID, ruleID string, req *dto.UpdateFirewallRuleRequest) (*models.FirewallRule, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	rule, err := s.repo.FindFirewallRuleByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}

	if req.Action != nil {
		action, err := enums.ParseRuleAction(*req.Action)
		if err != nil {
			return nil, err
		}

		rule.Action = action
	}

	if req.Port != nil {
		rule.Port = *req.Port
	}

	if req.FromIPv4 != nil {
		rule.FromIPv4 = req.FromIPv4
	}

	if req.Mask != nil {
		rule.Mask = req.Mask
	}

	if req.Note != nil {
		rule.Note = req.Note
	}

	if err := s.repo.UpdateFirewallRule(ctx, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

// DeleteFirewallRule deletes a firewall rule
func (s *Service) DeleteFirewallRule(ctx context.Context, serverID, teamID, ruleID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	rule, err := s.repo.FindFirewallRuleByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return err
	}

	if rule.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		rule.UninstallationRequestedAt = &now
		if err := s.repo.UpdateFirewallRule(ctx, rule); err != nil {
			return err
		}

		if err := s.dispatchFirewallRuleUninstallJob(server, rule); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("rule_id", ruleID).
				Msg("Failed to dispatch firewall rule uninstall job")
		}

		return nil
	}

	return s.repo.DeleteFirewallRule(ctx, ruleID)
}

func (s *Service) dispatchFirewallRuleInstallJob(server *models.Server, rule *models.FirewallRule) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewFirewallRuleTask(server.ID, rule.ID, "install")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchFirewallRuleUninstallJob(server *models.Server, rule *models.FirewallRule) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewFirewallRuleTask(server.ID, rule.ID, "uninstall")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}
