package handlers

import (
	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ListTasks returns tasks for a server. Carries an optional `limit`
// query param so it does not fit IndexNested.
func (h *Handler) ListTasks(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}
	limit := fiberctx.ParseLimit(c, 50, 100)
	tasks, err := h.service.ListTasks(c.Context(), serverID, teamID, limit)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Tasks retrieved", tasks)
}
