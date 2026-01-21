package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ListFirewallRules returns all firewall rules for a server
func (h *Handler) ListFirewallRules(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	rules, err := h.service.ListFirewallRules(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch firewall rules")
	}

	result := make([]dto.FirewallRuleResponse, len(rules))
	for i, rule := range rules {
		result[i] = dto.ToFirewallRuleResponse(&rule)
	}

	return response.OK(c, "Firewall rules retrieved", result)
}

// CreateFirewallRule creates a new firewall rule
func (h *Handler) CreateFirewallRule(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.CreateFirewallRuleRequest](c)
	if err != nil {
		return err
	}

	rule, err := h.service.CreateFirewallRule(c.Context(), serverID, teamID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Firewall rule created", dto.ToFirewallRuleResponse(rule))
}

// UpdateFirewallRule updates a firewall rule
func (h *Handler) UpdateFirewallRule(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	req, err := fiberctx.MustParseAndValidate[dto.UpdateFirewallRuleRequest](c)
	if err != nil {
		return err
	}

	rule, err := h.service.UpdateFirewallRule(c.Context(), serverID, teamID, ruleID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Firewall rule updated", dto.ToFirewallRuleResponse(rule))
}

// DeleteFirewallRule deletes a firewall rule
func (h *Handler) DeleteFirewallRule(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	if err := h.service.DeleteFirewallRule(c.Context(), serverID, teamID, ruleID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}
