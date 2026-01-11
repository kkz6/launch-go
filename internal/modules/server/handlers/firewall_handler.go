package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListFirewallRules returns all firewall rules for a server
func (h *Handler) ListFirewallRules(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
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
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req dto.CreateFirewallRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	rule, err := h.service.CreateFirewallRule(c.Context(), serverID, teamID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "Firewall rule created", dto.ToFirewallRuleResponse(rule))
}

// UpdateFirewallRule updates a firewall rule
func (h *Handler) UpdateFirewallRule(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	var req dto.UpdateFirewallRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	rule, err := h.service.UpdateFirewallRule(c.Context(), serverID, teamID, ruleID, &req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Firewall rule updated", dto.ToFirewallRuleResponse(rule))
}

// DeleteFirewallRule deletes a firewall rule
func (h *Handler) DeleteFirewallRule(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	if err := h.service.DeleteFirewallRule(c.Context(), serverID, teamID, ruleID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}
