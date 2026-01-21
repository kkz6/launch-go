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
	serverID := c.Params("serverId")
	siteID := c.Params("id")
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
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.Created(c, "Redirect created", dto.ToRedirectResponse(redirect))
}

// ListRedirects returns all redirects for a site
func (h *RedirectHandler) ListRedirects(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	redirects, err := h.redirectService.List(c.Context(), siteID, serverID)
	if err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		return response.InternalError(c, response.MsgInternalError)
	}

	result := make([]dto.RedirectResponse, len(redirects))
	for i, redirect := range redirects {
		result[i] = dto.ToRedirectResponse(&redirect)
	}

	return response.OK(c, "Redirects retrieved", result)
}

// DeleteRedirect deletes a redirect
func (h *RedirectHandler) DeleteRedirect(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")
	redirectID := c.Params("redirectId")

	if err := h.redirectService.Delete(c.Context(), redirectID, siteID, serverID); err != nil {
		if errors.Is(err, repositories.ErrSiteNotFound) {
			return response.NotFound(c, response.MsgSiteNotFound)
		}

		if errors.Is(err, repositories.ErrRedirectNotFound) {
			return response.NotFound(c, response.MsgRedirectNotFound)
		}

		return response.HandleError(c, err)
	}

	return response.OK(c, "Redirect deleted", nil)
}
