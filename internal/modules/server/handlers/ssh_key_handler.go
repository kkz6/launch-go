package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/ssh"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListSSHKeys returns all SSH keys for the team
func (h *Handler) ListSSHKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	globalOnly := c.QueryBool("global", false)

	keys, err := h.service.ListSSHKeys(c.Context(), teamID, globalOnly)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "SSH keys retrieved", pkgdto.TransformSlice(keys, dto.ToSSHKeyResponse))
}

// GenerateSSHKey generates a new SSH key pair
func (h *Handler) GenerateSSHKey(c *fiber.Ctx) error {
	req, err := fiberctx.MustParseAndValidate[dto.GenerateSSHKeyRequest](c)
	if err != nil {
		return err
	}

	var privateKeyPEM, publicKeyStr string

	switch req.Type {
	case "ed25519":
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return fiberctx.RespondInternalError(c, "Failed to generate ED25519 key")
		}

		privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
		if err != nil {
			return fiberctx.RespondInternalError(c, "Failed to marshal private key")
		}

		privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privKeyBytes,
		}))

		sshPubKey, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			return fiberctx.RespondInternalError(c, "Failed to create SSH public key")
		}
		publicKeyStr = string(ssh.MarshalAuthorizedKey(sshPubKey))

	default: // rsa
		privKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return fiberctx.RespondInternalError(c, "Failed to generate RSA key")
		}

		privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
		}))

		sshPubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
		if err != nil {
			return fiberctx.RespondInternalError(c, "Failed to create SSH public key")
		}
		publicKeyStr = string(ssh.MarshalAuthorizedKey(sshPubKey))
	}

	return fiberctx.OK(c, "SSH key generated", dto.GenerateSSHKeyResponse{
		PrivateKey: privateKeyPEM,
		PublicKey:  publicKeyStr,
	})
}

// ListServerSSHKeys returns all SSH keys attached to a server
func (h *Handler) ListServerSSHKeys(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	keys, err := h.service.ListServerSSHKeys(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch SSH keys")
	}

	return fiberctx.OK(c, "SSH keys retrieved", pkgdto.TransformSlice(keys, dto.ToSSHKeyResponse))
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
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
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
	teamID, serverID, sshKeyID, err := fiberctx.GetTeamServerAndEntityID(c, "sshKeyId")
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
