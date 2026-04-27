package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SSLHandler holds the SSL endpoint that does not fit a generic route
// helper: UpdateSSL is a PUT to a site-singleton setting (no own id)
// with a body. ListCertificates uses IndexDoubleNested directly in
// routes.go.
type SSLHandler struct {
	sslService *services.SSLService
}

// NewSSLHandler creates a new SSL handler.
func NewSSLHandler(sslService *services.SSLService) *SSLHandler {
	return &SSLHandler{sslService: sslService}
}

// UpdateSSL updates SSL settings for a site.
func (h *SSLHandler) UpdateSSL(c *fiber.Ctx) error {
	serverID, siteID, err := fiberctx.GetServerAndSiteID(c)
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
		return err
	}
	return fiberctx.OK(c, "SSL settings updated", nil)
}
