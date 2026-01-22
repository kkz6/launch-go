package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListSSHKeys returns all SSH keys for the team
func (h *Handler) ListSSHKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	keys, err := h.service.ListSSHKeys(c.Context(), teamID)
	if err != nil {
		return fiberctx.RespondInternalError(c, "Failed to fetch SSH keys")
	}

	result := make([]dto.SSHKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSSHKeyResponse(&key)
	}

	return fiberctx.OK(c, "SSH keys retrieved", result)
}

// ListServerSSHKeys returns all SSH keys attached to a server
func (h *Handler) ListServerSSHKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	keys, err := h.service.ListServerSSHKeys(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch SSH keys")
	}

	result := make([]dto.SSHKeyResponse, len(keys))
	for i, key := range keys {
		result[i] = dto.ToSSHKeyResponse(&key)
	}

	return fiberctx.OK(c, "SSH keys retrieved", result)
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
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "SSH key created", dto.ToSSHKeyResponse(key))
}

// AttachSSHKey attaches an SSH key to a server
func (h *Handler) AttachSSHKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.AttachSSHKeyRequest](c)
	if err != nil {
		return err
	}

	if err := h.service.AttachSSHKey(c.Context(), serverID, teamID, req.SSHKeyID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "SSH key attached", nil)
}

// DetachSSHKey detaches an SSH key from a server
func (h *Handler) DetachSSHKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverID, err := fiberctx.GetID(c)
	if err != nil {
		return err
	}

	sshKeyID, err := fiberctx.GetULIDParam(c, "sshKeyId")
	if err != nil {
		return err
	}

	if err := h.service.DetachSSHKey(c.Context(), serverID, teamID, sshKeyID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}

// DeleteSSHKey deletes an SSH key
func (h *Handler) DeleteSSHKey(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	sshKeyID, err := fiberctx.GetULIDParam(c, "sshKeyId")
	if err != nil {
		return err
	}

	if err := h.service.DeleteSSHKey(c.Context(), teamID, sshKeyID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.NoContent(c)
}
