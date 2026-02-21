package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListFirewallRules returns all firewall rules for a server
func (h *Handler) ListFirewallRules(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	rules, err := h.service.ListFirewallRules(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch firewall rules")
	}

	return fiberctx.OK(c, "Firewall rules retrieved", pkgdto.TransformSlice(rules, dto.ToFirewallRuleResponse))
}

// CreateFirewallRule creates a new firewall rule
func (h *Handler) CreateFirewallRule(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateFirewallRuleRequest](c)
	if err != nil {
		return err
	}

	rule, err := h.service.CreateFirewallRule(c.Context(), serverID, teamID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Firewall rule created", dto.ToFirewallRuleResponse(rule))
}

// UpdateFirewallRule updates a firewall rule
func (h *Handler) UpdateFirewallRule(c *fiber.Ctx) error {
	teamID, serverID, ruleID, err := fiberctx.GetTeamServerAndEntityID(c, "ruleId")
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateFirewallRuleRequest](c)
	if err != nil {
		return err
	}

	rule, err := h.service.UpdateFirewallRule(c.Context(), serverID, teamID, ruleID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Firewall rule updated", dto.ToFirewallRuleResponse(rule))
}

// DeleteFirewallRule deletes a firewall rule
func (h *Handler) DeleteFirewallRule(c *fiber.Ctx) error {
	teamID, serverID, ruleID, err := fiberctx.GetTeamServerAndEntityID(c, "ruleId")
	if err != nil {
		return err
	}

	if err := h.service.DeleteFirewallRule(c.Context(), serverID, teamID, ruleID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}
