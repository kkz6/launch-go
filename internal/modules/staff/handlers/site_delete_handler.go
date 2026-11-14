package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/staff/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (h *AdminHandler) DeleteSite(c *fiber.Ctx) error {
	return h.deleteSite(c, false)
}

func (h *AdminHandler) RemoveSiteRecord(c *fiber.Ctx) error {
	return h.deleteSite(c, true)
}

func (h *AdminHandler) deleteSite(c *fiber.Ctx, removeRecord bool) error {
	var request struct {
		ConfirmationDomain string `json:"confirmation_domain"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}
	actorID, err := fiberutil.MustGetUserID(c)
	if err != nil {
		return err
	}
	result, err := h.service.DeleteSite(c.UserContext(), c.Params("id"), actorID, request.ConfirmationDomain, removeRecord)
	if errors.Is(err, services.ErrSiteNotFound) {
		return fiberutil.RespondNotFound(c, "Site not found")
	}
	if err != nil {
		return fiberutil.HandleError(c, err)
	}
	if removeRecord {
		return fiberutil.OK(c, "Site removed from launchctl", result)
	}
	return fiberutil.OK(c, "Site deletion requested", result)
}
