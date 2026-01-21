package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[dto.UpdateSSLRequest](c)
	if err != nil {
		return err
	}

	if err := h.sslService.UpdateSSL(c.Context(), siteID, serverID, userID, req); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "SSL settings updated", nil)
}

// ListCertificates returns all certificates for a site
func (h *SSLHandler) ListCertificates(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	certs, err := h.sslService.ListCertificates(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.CertificateResponse, len(certs))
	for i, cert := range certs {
		result[i] = dto.ToCertificateResponse(&cert)
	}

	return response.OK(c, "Certificates retrieved", result)
}
