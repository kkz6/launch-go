package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (h *AdminHandler) DeleteServer(c *fiber.Ctx) error {
	var request struct {
		ConfirmationName string `json:"confirmation_name"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}
	actorID, err := fiberutil.MustGetUserID(c)
	if err != nil {
		return err
	}
	result, err := h.service.DeleteServer(c.UserContext(), c.Params("id"), actorID, request.ConfirmationName)
	if errors.Is(err, services.ErrServerNotFound) {
		return fiberutil.RespondNotFound(c, "Server not found")
	}
	if err != nil {
		return fiberutil.HandleError(c, err)
	}
	return fiberutil.OK(c, "Server deletion requested", result)
}
