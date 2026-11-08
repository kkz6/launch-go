package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (h *AdminHandler) DeleteServer(c *fiber.Ctx) error {
	return h.deleteServer(c, false)
}

func (h *AdminHandler) RemoveServerRecord(c *fiber.Ctx) error {
	return h.deleteServer(c, true)
}

func (h *AdminHandler) deleteServer(c *fiber.Ctx, removeRecord bool) error {
	var request struct {
		ConfirmationName string `json:"confirmation_name"`
		Mode             string `json:"mode"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}
	// A dedicated route keeps local removal safe when clients encounter an
	// older API: an unknown route fails instead of ignoring a new body field.
	if request.Mode != "" {
		return fiberutil.HandleError(c, fiberutil.Validation("Server deletion modes are not supported"))
	}
	actorID, err := fiberutil.MustGetUserID(c)
	if err != nil {
		return err
	}
	mode := ""
	if removeRecord {
		mode = "remove_record"
	}
	result, err := h.service.DeleteServer(c.UserContext(), c.Params("id"), actorID, request.ConfirmationName, mode)
	if errors.Is(err, services.ErrServerNotFound) {
		return fiberutil.RespondNotFound(c, "Server not found")
	}
	if err != nil {
		return fiberutil.HandleError(c, err)
	}
	if result.Status == "removed" {
		return fiberutil.OK(c, "Server removed from launchctl", result)
	}
	return fiberutil.OK(c, "Server deletion requested", result)
}
