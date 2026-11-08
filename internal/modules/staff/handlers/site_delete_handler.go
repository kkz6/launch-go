package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/staff/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (h *AdminHandler) DeleteSite(c *fiber.Ctx) error {
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
	result, err := h.service.DeleteSite(c.UserContext(), c.Params("id"), actorID, request.ConfirmationDomain)
	if errors.Is(err, services.ErrSiteNotFound) {
		return fiberutil.RespondNotFound(c, "Site not found")
	}
	if err != nil {
		return fiberutil.HandleError(c, err)
	}
	return fiberutil.OK(c, "Site deletion requested", result)
}
