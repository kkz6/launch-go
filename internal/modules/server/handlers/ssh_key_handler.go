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
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListSSHKeys returns all SSH keys for the team. Carries an optional
// `?global=true` query param so it does not fit the generic Index.
func (h *Handler) ListSSHKeys(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	keys, err := h.service.ListSSHKeys(c.Context(), teamID, c.QueryBool("global", false))
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "SSH keys retrieved", keys)
}

// GenerateSSHKey generates a new SSH key pair locally — no DB write or
// service call, just crypto. Stays bespoke.
func (h *Handler) GenerateSSHKey(c *fiber.Ctx, req *dto.GenerateSSHKeyRequest) error {
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
		privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privKeyBytes}))
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
		privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)}))
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

// AttachSSHKey attaches an SSH key (by id from the body) to a server.
// Action with body — does not fit a generic helper.
func (h *Handler) AttachSSHKey(c *fiber.Ctx, req *dto.AttachSSHKeyRequest) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	if err := h.service.AttachSSHKey(c.Context(), serverID, teamID, req.SSHKeyID); err != nil {
		return err
	}
	return fiberctx.OK(c, "SSH key attached", nil)
}
