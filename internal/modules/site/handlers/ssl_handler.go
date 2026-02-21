package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SSLHandler handles HTTP requests for SSL/TLS
type SSLHandler struct {
	sslService *services.SSLService
}

// NewSSLHandler creates a new SSL handler
func NewSSLHandler(sslService *services.SSLService) *SSLHandler {
	return &SSLHandler{sslService: sslService}
}

// UpdateSSL updates SSL settings
func (h *SSLHandler) UpdateSSL(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateSSLRequest](c)
	if err != nil {
		return err
	}

	if err := h.sslService.UpdateSSL(c.Context(), siteID, serverID, userID, req); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "SSL settings updated", nil)
}

// ListCertificates returns all certificates for a site
func (h *SSLHandler) ListCertificates(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	certs, err := h.sslService.ListCertificates(c.Context(), siteID, serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	result := pkgdto.TransformSlice(certs, dto.ToCertificateResponse)

	return fiberctx.OK(c, "Certificates retrieved", result)
}
