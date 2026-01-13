package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
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

	rule := &models.FirewallRule{
		ServerID: serverID,
		Name:     req.Name,
		Action:   action,
		Port:     req.Port,
		FromIPv4: req.FromIPv4,
		Mask:     req.Mask,
		Note:     req.Note,
	}

	if err := s.repo.CreateFirewallRule(ctx, rule); err != nil {
		return nil, err
	}

	activity.New(s.repo.DB()).
		WithContext(ctx).
		UseLog("server").
		On(rule).
		WithEvent("created").
		Log("Firewall rule was created")

	if server.IsProvisioned() {
		if err := s.dispatchFirewallRuleInstallJob(server, rule); err != nil {
			s.LogError(err, "Failed to dispatch firewall rule install job", "server_id", serverID, "rule_id", rule.ID)
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

	activity.New(s.repo.DB()).
		WithContext(ctx).
		UseLog("server").
		On(rule).
		WithEvent("updated").
		Log("Firewall rule was updated")

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

	activity.New(s.repo.DB()).
		WithContext(ctx).
		UseLog("server").
		On(rule).
		WithEvent("deleted").
		Log("Firewall rule deletion requested")

	if rule.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		rule.UninstallationRequestedAt = &now
		if err := s.repo.UpdateFirewallRule(ctx, rule); err != nil {
			return err
		}

		if err := s.dispatchFirewallRuleUninstallJob(server, rule); err != nil {
			s.LogError(err, "Failed to dispatch firewall rule uninstall job", "server_id", serverID, "rule_id", ruleID)
		}

		return nil
	}

	return s.repo.DeleteFirewallRule(ctx, ruleID)
}

func (s *Service) dispatchFirewallRuleInstallJob(server *models.Server, rule *models.FirewallRule) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewInstallFirewallRuleTask(server.ID, rule.ID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchFirewallRuleUninstallJob(server *models.Server, rule *models.FirewallRule) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewUninstallFirewallRuleTask(server.ID, rule.ID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}
