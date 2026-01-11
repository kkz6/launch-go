package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ListSshKeys returns all SSH keys for the team
func (h *Handler) ListSshKeys(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	keys, err := h.service.ListSshKeys(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch SSH keys")
	}

	result := make([]dto.SshKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSshKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

// ListServerSshKeys returns all SSH keys attached to a server
func (h *Handler) ListServerSshKeys(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	keys, err := h.service.ListServerSshKeys(c.Context(), serverID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		return response.InternalError(c, "Failed to fetch SSH keys")
	}

	result := make([]dto.SshKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSshKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

// CreateSshKey creates a new SSH key
func (h *Handler) CreateSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userID := c.Locals("userID").(string)

	var req dto.CreateSshKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	key, err := h.service.CreateSshKey(c.Context(), teamID, userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "SSH key created", dto.ToSshKeyResponse(key))
}

// AttachSshKey attaches an SSH key to a server
func (h *Handler) AttachSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	var req dto.AttachSshKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.AttachSshKey(c.Context(), serverID, teamID, req.SshKeyID); err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrSshKeyNotFound) {
			return response.NotFound(c, "SSH key not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "SSH key attached", nil)
}

// DetachSshKey detaches an SSH key from a server
func (h *Handler) DetachSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")
	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DetachSshKey(c.Context(), serverID, teamID, sshKeyID); err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			return response.NotFound(c, "Server not found")
		}

		if errors.Is(err, services.ErrSshKeyNotFound) {
			return response.NotFound(c, "SSH key not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// DeleteSshKey deletes an SSH key
func (h *Handler) DeleteSshKey(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DeleteSshKey(c.Context(), teamID, sshKeyID); err != nil {
		if errors.Is(err, services.ErrSshKeyNotFound) {
			return response.NotFound(c, "SSH key not found")
		}

		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}
