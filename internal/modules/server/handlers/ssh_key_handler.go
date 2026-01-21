package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// ListSSHKeys returns all SSH keys for the team
func (h *Handler) ListSSHKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	keys, err := h.service.ListSSHKeys(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch SSH keys")
	}

	result := make([]dto.SSHKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSSHKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

// ListServerSSHKeys returns all SSH keys attached to a server
func (h *Handler) ListServerSSHKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	keys, err := h.service.ListServerSSHKeys(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch SSH keys")
	}

	result := make([]dto.SSHKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSSHKeyResponse(&key)
	}

	return response.OK(c, "SSH keys retrieved", result)
}

// CreateSSHKey creates a new SSH key
func (h *Handler) CreateSSHKey(c *fiber.Ctx) error {
	teamID, userID, err := fiberctx.MustGetTeamAndUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.CreateSSHKeyRequest](c)
	if err != nil {
		return err
	}

	key, err := h.service.CreateSSHKey(c.Context(), teamID, userID, req)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.Created(c, "SSH key created", dto.ToSSHKeyResponse(key))
}

// AttachSSHKey attaches an SSH key to a server
func (h *Handler) AttachSSHKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.AttachSSHKeyRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.AttachSSHKey(c.Context(), serverID, teamID, req.SSHKeyID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "SSH key attached", nil)
}

// DetachSSHKey detaches an SSH key from a server
func (h *Handler) DetachSSHKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID := c.Params("id")
	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DetachSSHKey(c.Context(), serverID, teamID, sshKeyID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// DeleteSSHKey deletes an SSH key
func (h *Handler) DeleteSSHKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	sshKeyID := c.Params("sshKeyId")

	if err := h.service.DeleteSSHKey(c.Context(), teamID, sshKeyID); err != nil {
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}
