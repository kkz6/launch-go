package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ListSshKeys returns all SSH keys for the team
func (h *Handler) ListSshKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

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
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	keys, err := h.service.ListServerSshKeys(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch SSH keys")
	}

	result := make([]dto.SshKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSshKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

// CreateSshKey creates a new SSH key
func (h *Handler) CreateSshKey(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateSshKeyRequest](c)
	if err != nil {
		return err
	}

	key, err := h.service.CreateSshKey(c.Context(), teamID, userID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "SSH key created", dto.ToSshKeyResponse(key))
}

// AttachSshKey attaches an SSH key to a server
func (h *Handler) AttachSshKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.AttachSshKeyRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.AttachSshKey(c.Context(), serverID, teamID, req.SshKeyID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "SSH key attached", nil)
}

// DetachSshKey detaches an SSH key from a server
func (h *Handler) DetachSshKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DetachSshKey(c.Context(), serverID, teamID, sshKeyID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// DeleteSshKey deletes an SSH key
func (h *Handler) DeleteSshKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DeleteSshKey(c.Context(), teamID, sshKeyID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}
