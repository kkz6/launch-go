package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/modules/site/support"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// FileHandler handles file management HTTP requests
type FileHandler struct {
	service *services.FileService
}

// NewFileHandler creates a new FileHandler instance
func NewFileHandler(service *services.FileService) *FileHandler {
	return &FileHandler{service: service}
}

// ListFiles returns the list of editable files for a site
func (h *FileHandler) ListFiles(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	files, err := h.service.ListFiles(c.Context(), serverID, siteID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Files retrieved", files)
}

// ListLogs returns the list of log files for a site
func (h *FileHandler) ListLogs(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	logs, err := h.service.ListLogFiles(c.Context(), serverID, siteID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Logs retrieved", logs)
}

// ShowFile gets the content of a file using the encoded file parameter in the URL path
// Route: GET /servers/:serverId/sites/:id/files/:file
func (h *FileHandler) ShowFile(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	fileParam := c.Params("file")
	if fileParam == "" {
		return response.BadRequest(c, response.MsgMissingRequiredParams)
	}

	// Decode the encrypted file parameter
	data, err := support.DecodeFileRouteParam(fileParam)
	if err != nil {
		return response.BadRequest(c, response.MsgInvalidRequestBody)
	}

	content, err := h.service.GetFileContent(c.Context(), serverID, siteID, data.Path)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "File content retrieved", map[string]string{
		"content": content,
		"path":    data.Path,
	})
}

// UpdateFile updates the content of a file using the encoded file parameter in the URL path
// Route: PUT /servers/:serverId/sites/:id/files/:file
func (h *FileHandler) UpdateFile(c *fiber.Ctx) error {
	serverID, err := fiberctx.GetServerID(c)
	if err != nil {
		return err
	}

	siteID, err := fiberctx.GetSiteID(c)
	if err != nil {
		return err
	}

	fileParam := c.Params("file")
	if fileParam == "" {
		return response.BadRequest(c, response.MsgMissingRequiredParams)
	}

	// Decode the encrypted file parameter
	data, err := support.DecodeFileRouteParam(fileParam)
	if err != nil {
		return response.BadRequest(c, response.MsgInvalidRequestBody)
	}

	req, err := fiberctx.MustParseAndValidate[struct {
		Content string `json:"content"`
	}](c)
	if err != nil {
		return err
	}

	if err := h.service.UpdateFileContent(c.Context(), serverID, siteID, data.Path, req.Content); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "File updated successfully", nil)
}
