package services

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// ListFirewallRules returns all firewall rules for a server
func (s *Service) ListFirewallRules(ctx context.Context, serverID, teamID string) ([]models.FirewallRule, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.FirewallRule().FindByServer(ctx, serverID)
}

// CreateFirewallRule creates a new firewall rule
func (s *Service) CreateFirewallRule(ctx context.Context, serverID, teamID string, req *dto.CreateFirewallRuleRequest) (*models.FirewallRule, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	action, err := types.ParseRuleAction(req.Action)
	if err != nil {
		return nil, err
	}

	rule := &models.FirewallRule{
		Name:     req.Name,
		Action:   action,
		Port:     req.Port,
		FromIPv4: req.FromIPv4,
		Mask:     req.Mask,
		Note:     req.Note,
	}
	rule.ServerID = serverID

	if err := s.repos.FirewallRule().Create(ctx, rule); err != nil {
		return nil, err
	}

	// Using embedded ActivityMixin for consistent activity logging
	s.LogSystemActivity(ctx, rule, "created", "Firewall rule was created")

	if err := s.dispatchFirewallRuleInstallJob(server, rule); err != nil {
		s.LogError(err, "Failed to dispatch firewall rule install job", "server_id", serverID, "rule_id", rule.ID)
	}

	return rule, nil
}

// UpdateFirewallRule updates a firewall rule
func (s *Service) UpdateFirewallRule(ctx context.Context, serverID, teamID, ruleID string, req *dto.UpdateFirewallRuleRequest) (*models.FirewallRule, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	rule, err := s.repos.FirewallRule().FindByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}

	if req.Note != nil {
		rule.Note = req.Note
	}

	if err := s.repos.FirewallRule().Update(ctx, rule); err != nil {
		return nil, err
	}

	// Using embedded ActivityMixin for consistent activity logging
	s.LogSystemActivity(ctx, rule, "updated", "Firewall rule was updated")

	return rule, nil
}

// DeleteFirewallRule deletes a firewall rule
func (s *Service) DeleteFirewallRule(ctx context.Context, serverID, teamID, ruleID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	rule, err := s.repos.FirewallRule().FindByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return err
	}

	// Using embedded ActivityMixin for consistent activity logging
	s.LogSystemActivity(ctx, rule, "deleted", "Firewall rule deletion requested")

	if rule.IsInstalled() {
		return s.WithTransaction(ctx, func(tx *gorm.DB) error {
			now := time.Now()
			rule.UninstallationRequestedAt = &now
			if err := tx.Save(rule).Error; err != nil {
				return err
			}

			return s.dispatchFirewallRuleUninstallJob(server, rule)
		})
	}

	return s.repos.FirewallRule().Delete(ctx, ruleID)
}

func (s *Service) dispatchFirewallRuleInstallJob(server *models.Server, rule *models.FirewallRule) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewInstallFirewallRuleTask(server.ID, rule.ID, nil)
	})
}

func (s *Service) dispatchFirewallRuleUninstallJob(server *models.Server, rule *models.FirewallRule) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewUninstallFirewallRuleTask(server.ID, rule.ID, nil)
	})
}

// CreateDefaultFirewallRules creates the default firewall rules for a new server (SSH, HTTP, HTTPS)
// These rules are created in the database but not installed until the server is provisioned.
func (s *Service) CreateDefaultFirewallRules(ctx context.Context, serverID string) error {
	defaultRules := []models.FirewallRule{
		{
			Name:     "ssh",
			Port:     "22",
			FromIPv4: strPtr("0.0.0.0"),
			Mask:     strPtr("0"),
			Action:   types.RuleActionAllow,
		},
		{
			Name:     "http",
			Port:     "80",
			FromIPv4: strPtr("0.0.0.0"),
			Mask:     strPtr("0"),
			Action:   types.RuleActionAllow,
		},
		{
			Name:     "https",
			Port:     "443",
			FromIPv4: strPtr("0.0.0.0"),
			Mask:     strPtr("0"),
			Action:   types.RuleActionAllow,
		},
	}

	for i := range defaultRules {
		defaultRules[i].ServerID = serverID
		if err := s.repos.FirewallRule().Create(ctx, &defaultRules[i]); err != nil {
			return err
		}
	}

	return nil
}

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}
