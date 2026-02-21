package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RedirectHandler handles HTTP requests for redirects
type RedirectHandler struct {
	redirectService *services.RedirectService
}

// NewRedirectHandler creates a new redirect handler
func NewRedirectHandler(redirectService *services.RedirectService) *RedirectHandler {
	return &RedirectHandler{redirectService: redirectService}
}

// CreateRedirect creates a new redirect
func (h *RedirectHandler) CreateRedirect(c *fiber.Ctx) error {
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

	req, err := fiberctx.MustParseAndValidate[dto.CreateRedirectRequest](c)
	if err != nil {
		return err
	}

	redirect, err := h.redirectService.Create(c.Context(), siteID, serverID, userID, req)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.Created(c, "Redirect created", dto.ToRedirectResponse(redirect))
}

// ListRedirects returns all redirects for a site
func (h *RedirectHandler) ListRedirects(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	redirects, err := h.redirectService.List(c.Context(), siteID, serverID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	result := pkgdto.TransformSlice(redirects, dto.ToRedirectResponse)

	return fiberctx.OK(c, "Redirects retrieved", result)
}

// DeleteRedirect deletes a redirect
func (h *RedirectHandler) DeleteRedirect(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	redirectID, err := fiberctx.GetRedirectID(c)
	if err != nil {
		return err
	}

	if err := h.redirectService.Delete(c.Context(), redirectID, siteID, serverID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Redirect deleted", nil)
}
