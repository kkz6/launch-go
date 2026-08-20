package handlers

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/support"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// LogInfo represents available log information
type LogInfo struct {
	Name      string `json:"name"`
	Software  string `json:"software"`
	Path      string `json:"path"`
	ShowRoute string `json:"show_route"`
}

// localizedSoftwareLogName translates application-owned display metadata
// while keeping the stable software value and log path unchanged.
func localizedSoftwareLogName(ctx context.Context, software types.Software) string {
	name := fmt.Sprintf("%s Log", software.Label())
	if !software.IsValid() || !software.HasLogPath() {
		return name
	}
	return i18n.TContext(ctx, name)
}

// ListLogs returns available logs for a server based on installed services
func (h *Handler) ListLogs(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	// Get server with services
	server, err := h.service.GetServerWithRelations(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch server")
	}

	// Build list of available logs based on installed services
	var logs []LogInfo

	for _, service := range server.Services {
		software := types.Software(service.Software)
		if software.HasLogPath() {
			logPath := software.LogPath()
			// Generate encrypted route parameter
			showRoute, _ := support.EncodeFileRouteParam(logPath, "log")

			logs = append(logs, LogInfo{
				Name:      localizedSoftwareLogName(c.Context(), software),
				Software:  software.String(),
				Path:      logPath,
				ShowRoute: showRoute,
			})
		}
	}

	// If no logs found, return empty array (not null)
	if logs == nil {
		logs = []LogInfo{}
	}

	return fiberctx.OK(c, "Logs retrieved", logs)
}

// GetLogContent returns the content of a log file
// Route: GET /servers/:id/logs/:log
func (h *Handler) GetLogContent(c *fiber.Ctx) error {
	teamID, serverID, err := fiberctx.GetTeamAndServerID(c)
	if err != nil {
		return err
	}

	logParam := c.Params("log")

	if logParam == "" {
		return fiberctx.RespondBadRequest(c, "Log parameter is required")
	}

	// Decode the encrypted log parameter
	data, err := support.DecodeFileRouteParam(logParam)
	if err != nil {
		return fiberctx.RespondBadRequest(c, "Invalid log parameter")
	}

	// Verify the path is an allowed log path
	if !isAllowedLogPath(data.Path) {
		return fiberctx.RespondBadRequest(c, "Log path not allowed")
	}

	// Get server
	server, err := h.service.GetServerWithRelations(c.Context(), serverID, teamID)
	if err != nil {
		return fiberctx.HandleErrorOrInternal(c, err, "Failed to fetch server")
	}

	// Create task to read log content
	task := tasks.GetFile(tasks.GetFileConfig{
		Path: data.Path,
	})

	// Run task using task runner
	result, err := h.taskRunner.NewRunner(server, task).
		AsRoot().
		Run(c.Context())
	if err != nil {
		return fiberctx.RespondInternalError(c, i18n.T(c, "Failed to read log: %s", err.Error()))
	}

	return fiberctx.OK(c, "Log content retrieved", map[string]string{
		"content": result.GetOutput(),
		"path":    data.Path,
	})
}

// isAllowedLogPath checks if the path is in the allowed log paths
func isAllowedLogPath(path string) bool {
	for _, software := range types.AllSoftware() {
		if software.LogPath() == path {
			return true
		}
	}
	return false
}
