package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Handler wraps the dockerapp service with HTTP plumbing for the
// endpoints that don't fit the framework's CRUD shape (uninstall with a
// body, logs with a query string).
type Handler struct {
	service *services.Service
}

// NewHandler constructs the handler.
func NewHandler(svc *services.Service) *Handler { return &Handler{service: svc} }

// extractTeamServerAppID extracts (teamID, serverID, appID). Different
// routes use "id" vs "appId" for the app, so the param name is passed.
func extractTeamServerAppID(c *fiber.Ctx, appParam string) (teamID, serverID, appID string, err error) {
	teamID, err = fiberctx.MustGetTeamID(c)
	if err != nil {
		return "", "", "", err
	}
	serverID, err = fiberctx.GetServerID(c)
	if err != nil {
		return "", "", "", err
	}
	appID, err = fiberctx.GetULIDParam(c, appParam)
	if err != nil {
		return "", "", "", err
	}
	return teamID, serverID, appID, nil
}

// Uninstall removes the app, optionally dropping its named volumes.
// Reads remove_data from JSON body OR ?remove_data=true query string.
func (h *Handler) Uninstall(c *fiber.Ctx) error {
	teamID, serverID, appID, err := extractTeamServerAppID(c, "id")
	if err != nil {
		return err
	}
	userID, _ := fiberctx.GetUserID(c)

	removeData := c.Query("remove_data") == "true"
	if !removeData && len(c.Body()) > 0 {
		var body struct {
			RemoveData bool `json:"remove_data"`
		}
		if err := c.BodyParser(&body); err == nil {
			removeData = body.RemoveData
		}
	}

	if err := h.service.UninstallWithOptions(c.Context(), appID, serverID, teamID, userID, removeData); err != nil {
		return err
	}
	return fiberctx.OK(c, "Application will be uninstalled shortly", nil)
}

// Logs returns the most recent log lines for an app.
func (h *Handler) Logs(c *fiber.Ctx) error {
	teamID, serverID, appID, err := extractTeamServerAppID(c, "appId")
	if err != nil {
		return err
	}
	tail := 200
	if v := c.Query("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}
	output, err := h.service.Logs(c.Context(), appID, serverID, teamID, tail)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Application logs retrieved", fiber.Map{
		"app_id": appID,
		"tail":   tail,
		"output": output,
	})
}
