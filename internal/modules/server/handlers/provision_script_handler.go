package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ProvisionScriptRepository interface for provision script handler
type ProvisionScriptRepository interface {
	FindServerByID(ctx context.Context, id string) (*models.Server, error)
}

// ProvisionScriptService interface for generating provision scripts
type ProvisionScriptService interface {
	GetProvisionScript(ctx context.Context, serverID, teamID string, autoStart bool) (string, error)
	StartCustomProvisioning(ctx context.Context, serverID string) error
}

// ProvisionScriptHandler handles provision script requests
type ProvisionScriptHandler struct {
	repo    ProvisionScriptRepository
	service ProvisionScriptService
}

// NewProvisionScriptHandler creates a new provision script handler
func NewProvisionScriptHandler(repo ProvisionScriptRepository, service ProvisionScriptService) *ProvisionScriptHandler {
	return &ProvisionScriptHandler{
		repo:    repo,
		service: service,
	}
}

// GetProvisionScript returns the provision script for a server
func (h *ProvisionScriptHandler) GetProvisionScript(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	ctx := c.Context()

	// Find the server (including archived)
	server, err := h.repo.FindServerByID(ctx, serverID)
	if err != nil {
		return fiberctx.RespondNotFound(c, "Server not found")
	}

	if server == nil {
		return fiberctx.RespondNotFound(c, "Server not found")
	}

	// Get the provision script
	script, err := h.service.GetProvisionScript(ctx, serverID, "", c.Query("start") == "1")
	if err != nil {
		return fiberctx.RespondInternalError(c, "Failed to generate provision script")
	}

	// Return as plain text (bash script)
	c.Set("Content-Type", "text/plain; charset=utf-8")
	c.Set("Cache-Control", "no-store")
	return c.SendString(script)
}

// StartProvisioning accepts the signed POST sent by the provisioning script.
func (h *ProvisionScriptHandler) StartProvisioning(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}
	if err := h.service.StartCustomProvisioning(c.Context(), serverID); err != nil {
		fiberctx.CaptureServerError(c, err)
		return fiberctx.Error(c, fiber.StatusUnprocessableEntity, "Could not start provisioning")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
