package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListFirewallRules returns all firewall rules for a server
func (h *Handler) ListFirewallRules(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	rules, err := h.service.ListFirewallRules(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.InternalError(c, "Failed to fetch firewall rules")
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

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	rule, err := h.service.CreateFirewallRule(c.Context(), serverID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
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

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	rule, err := h.service.UpdateFirewallRule(c.Context(), serverID, teamID, ruleID, &req)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrFirewallRuleNotFound) {
			return response.NotFound(c, "Firewall rule not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Firewall rule updated", dto.ToFirewallRuleResponse(rule))
}

// DeleteFirewallRule deletes a firewall rule
func (h *Handler) DeleteFirewallRule(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	ruleID := c.Params("ruleId")

	if err := h.service.DeleteFirewallRule(c.Context(), serverID, teamID, ruleID); err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrFirewallRuleNotFound) {
			return response.NotFound(c, "Firewall rule not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}
