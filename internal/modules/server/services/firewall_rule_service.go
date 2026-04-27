package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// ListFirewallRules returns all firewall rules for a server. Signature
// matches IndexNestedFunc.
func (s *Service) ListFirewallRules(ctx context.Context, serverID, teamID string) ([]dto.FirewallRuleResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	rules, err := s.repos.FirewallRule().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.FirewallRuleResponse, len(rules))
	for i := range rules {
		out[i] = dto.ToFirewallRuleResponse(&rules[i])
	}
	return out, nil
}

// CreateFirewallRule creates a new firewall rule. Signature matches CreateNestedFunc.
func (s *Service) CreateFirewallRule(ctx context.Context, serverID, teamID, userID string, req *dto.CreateFirewallRuleRequest) (dto.FirewallRuleResponse, error) {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return dto.FirewallRuleResponse{}, err
	}

	action, err := types.ParseRuleAction(req.Action)
	if err != nil {
		return dto.FirewallRuleResponse{}, err
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
		return dto.FirewallRuleResponse{}, err
	}

	s.LogSystemActivity(ctx, rule, "created", "Firewall rule was created")

	if err := s.dispatchFirewallRuleInstallJob(server, rule); err != nil {
		s.LogError(err, "Failed to dispatch firewall rule install job", "server_id", serverID, "rule_id", rule.ID)
	}
	return dto.ToFirewallRuleResponse(rule), nil
}

// UpdateFirewallRule updates a firewall rule. Signature matches UpdateNestedFunc.
func (s *Service) UpdateFirewallRule(ctx context.Context, ruleID, serverID, teamID, userID string, req *dto.UpdateFirewallRuleRequest) (dto.FirewallRuleResponse, error) {
	_ = userID
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.FirewallRuleResponse{}, err
	}

	rule, err := s.repos.FirewallRule().FindByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return dto.FirewallRuleResponse{}, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Note != nil {
		rule.Note = req.Note
	}

	if err := s.repos.FirewallRule().Update(ctx, rule); err != nil {
		return dto.FirewallRuleResponse{}, err
	}
	s.LogSystemActivity(ctx, rule, "updated", "Firewall rule was updated")
	return dto.ToFirewallRuleResponse(rule), nil
}

// DeleteFirewallRule deletes a firewall rule. Signature matches DeleteNestedFunc.
func (s *Service) DeleteFirewallRule(ctx context.Context, ruleID, serverID, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	rule, err := s.repos.FirewallRule().FindByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return err
	}

	s.LogSystemActivity(ctx, rule, "deleted", "Firewall rule deletion requested")

	return s.UninstallOrDelete(ctx, rule.IsInstalled(), ruleID,
		s.repos.FirewallRule().MarkAsUninstalling,
		s.repos.FirewallRule().Delete,
		func() error { return s.dispatchFirewallRuleUninstallJob(server, rule) },
	)
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

// CreateDefaultFirewallRules creates the default firewall rules for a
// new server (SSH, HTTP, HTTPS). These rules are created in the database
// but not installed until the server is provisioned.
func (s *Service) CreateDefaultFirewallRules(ctx context.Context, serverID string) error {
	defaultRules := []models.FirewallRule{
		{Name: "ssh", Port: "22", FromIPv4: strPtr("0.0.0.0"), Mask: strPtr("0"), Action: types.RuleActionAllow},
		{Name: "http", Port: "80", FromIPv4: strPtr("0.0.0.0"), Mask: strPtr("0"), Action: types.RuleActionAllow},
		{Name: "https", Port: "443", FromIPv4: strPtr("0.0.0.0"), Mask: strPtr("0"), Action: types.RuleActionAllow},
	}

	for i := range defaultRules {
		defaultRules[i].ServerID = serverID
		if err := s.repos.FirewallRule().Create(ctx, &defaultRules[i]); err != nil {
			return err
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }
