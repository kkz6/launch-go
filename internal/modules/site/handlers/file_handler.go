package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/modules/site/support"
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
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	files, err := h.service.ListFiles(c.Context(), serverID, siteID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Files retrieved", files)
}

// GetFileContent gets the content of a file
// Accepts either encrypted 'file' param or plain 'path' query parameter
func (h *FileHandler) GetFileContent(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	// Try to get path from encrypted 'file' parameter first
	fileParam := c.Query("file", "")
	path := c.Query("path", "")

	if fileParam != "" {
		// Decode the encrypted file parameter
		data, err := support.DecodeFileRouteParam(fileParam)
		if err != nil {
			return response.Error(c, fiber.StatusBadRequest, "Invalid file parameter")
		}
		path = data.Path
	}

	if path == "" {
		return response.Error(c, fiber.StatusBadRequest, "Path is required (use 'file' or 'path' parameter)")
	}

	content, err := h.service.GetFileContent(c.Context(), serverID, siteID, path)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "File content retrieved", map[string]string{
		"content": content,
		"path":    path,
	})
}

// UpdateFileContent updates the content of a file
// Accepts either encrypted 'file' param or plain 'path' in body
func (h *FileHandler) UpdateFileContent(c *fiber.Ctx) error {
	serverID := c.Params("serverId")
	siteID := c.Params("id")

	var req struct {
		File    string `json:"file"`    // encrypted file parameter
		Path    string `json:"path"`    // plain path (fallback)
		Content string `json:"content"`
	}

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	path := req.Path

	// Try to decode encrypted file parameter first
	if req.File != "" {
		data, err := support.DecodeFileRouteParam(req.File)
		if err != nil {
			return response.Error(c, fiber.StatusBadRequest, "Invalid file parameter")
		}
		path = data.Path
	}

	if path == "" {
		return response.Error(c, fiber.StatusBadRequest, "Path is required (use 'file' or 'path' field)")
	}

	if err := h.service.UpdateFileContent(c.Context(), serverID, siteID, path, req.Content); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "File updated successfully", nil)
}
