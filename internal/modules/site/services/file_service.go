package services

import (
	"context"
	"fmt"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/support"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// FileOnServer represents an editable file on the server
type FileOnServer struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Path        string                 `json:"path"`
	Context     string                 `json:"context,omitempty"`
	Type        string                 `json:"type"`                   // "default" or "environment"
	FileType    sitetypes.SiteFileType `json:"file_type"`              // enum value for identification
	ShowRoute   string                 `json:"show_route"`             // encrypted URL parameter for viewing
	UpdateRoute string                 `json:"update_route,omitempty"` // encrypted URL parameter for updating (not included for logs)
}

// FileService handles file operations on sites
type FileService struct {
	*BaseService
	serverReader contracts.ServerReader
}

// NewFileService creates a new FileService instance
func NewFileService(deps *ServiceDeps) *FileService {
	return &FileService{
		BaseService: NewBaseService(deps),
	}
}

// SetServerReader sets the server reader for cross-module queries
func (s *FileService) SetServerReader(reader contracts.ServerReader) {
	s.serverReader = reader
}

// ListFiles returns the list of editable files for a site
func (s *FileService) ListFiles(ctx context.Context, serverID, siteID string) ([]FileOnServer, error) {
	// Get site
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	return s.getEditableFiles(site), nil
}

// ListLogFiles returns the list of log files for a site
func (s *FileService) ListLogFiles(ctx context.Context, serverID, siteID string) ([]FileOnServer, error) {
	// Get site
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	return s.getLogFiles(site), nil
}

// getEditableFiles returns the list of editable files based on site type
func (s *FileService) getEditableFiles(site *models.Site) []FileOnServer {
	fileTypes := sitetypes.EditableFilesForSiteType(site.Type)
	return s.buildFileList(site, fileTypes, true)
}

// getLogFiles returns the list of log files based on site type
func (s *FileService) getLogFiles(site *models.Site) []FileOnServer {
	fileTypes := sitetypes.LogFilesForSiteType(site.Type)
	return s.buildFileList(site, fileTypes, false)
}

// buildFileList builds a list of FileOnServer from file types
// includeUpdateRoute determines if the update route should be included (false for log files)
func (s *FileService) buildFileList(site *models.Site, fileTypes []sitetypes.SiteFileType, includeUpdateRoute bool) []FileOnServer {
	files := make([]FileOnServer, 0, len(fileTypes))

	for _, ft := range fileTypes {
		path := ft.GetPath(site.Path, site.ZeroDowntimeDeployment)
		fileType := ft.FileType()

		// Generate encrypted route parameters
		showRoute, _ := support.EncodeFileRouteParam(path, fileType)

		file := FileOnServer{
			Name:        ft.Name(),
			Description: ft.Description(),
			Path:        path,
			Context:     site.Address,
			Type:        fileType,
			FileType:    ft,
			ShowRoute:   showRoute,
		}

		// Only include update route for editable files
		if includeUpdateRoute {
			file.UpdateRoute, _ = support.EncodeFileRouteParam(path, fileType)
		}

		files = append(files, file)
	}

	return files
}

// GetFileContent reads the content of a file from the server
func (s *FileService) GetFileContent(ctx context.Context, serverID, siteID, filePath string) (string, error) {
	// Get site to verify access
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return "", fmt.Errorf("site not found: %w", err)
	}

	// Verify the file path is in the allowed list
	if !s.isAllowedFilePath(site, filePath) {
		return "", fmt.Errorf("file path not allowed")
	}

	// Get the server for SSH connection
	if s.serverReader == nil {
		return "", fmt.Errorf("server reader not configured")
	}
	server, err := s.serverReader.FindServerByID(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Create task to read file content using the predefined GetFile task
	task := servertasks.GetFile(servertasks.GetFileConfig{
		Path: filePath,
	})

	// Run task using task runner as the server's configured user
	result, err := s.ServiceDeps().TaskRunnerDeps.NewRunner(server, task).
		AsRoot().
		Run(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return result.GetOutput(), nil
}

// UpdateFileOptions holds options for updating a file on the server
type UpdateFileOptions struct {
	FilePath       string
	FileType       string
	Content        string
	RunConfigCache bool
}

// UpdateFileContent updates the content of a file on the server
func (s *FileService) UpdateFileContent(ctx context.Context, serverID, siteID string, opts UpdateFileOptions) error {
	// Get site to verify access
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	// Verify the file path is in the allowed list (only editable files, not logs)
	if !s.isEditableFilePath(site, opts.FilePath) {
		return fmt.Errorf("file path not allowed for editing")
	}

	// Get the server for SSH connection
	if s.serverReader == nil {
		return fmt.Errorf("server reader not configured")
	}
	server, err := s.serverReader.FindServerByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Create task to write file content using the predefined UploadFile task
	task := servertasks.UploadFile(servertasks.UploadFileConfig{
		Path:     opts.FilePath,
		Contents: opts.Content,
	})

	// Run task using task runner as the server's configured user
	result, err := s.ServiceDeps().TaskRunnerDeps.NewRunner(server, task).
		AsRoot().
		Throw().
		Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to write file: exit code %d", result.GetExitCode())
	}

	// Run config:cache after environment file update for Laravel sites
	if opts.RunConfigCache && opts.FileType == "environment" && site.Type.IsLaravel() {
		s.runConfigCache(ctx, server, site)
	}

	return nil
}

// runConfigCache runs php artisan config:cache on the site
func (s *FileService) runConfigCache(ctx context.Context, server *servermodels.Server, site *models.Site) {
	appDir := site.GetApplicationDirectory()

	script := fmt.Sprintf("cd %s && php artisan config:cache", appDir)

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Config Cache"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)

	if result, err := s.ServiceDeps().TaskRunnerDeps.NewRunner(server, task).
		AsUser(site.User).
		Run(ctx); err != nil {
		s.LogError(err, "Failed to run config:cache", "site_id", site.ID)
	} else if !result.IsSuccessful() {
		s.LogError(fmt.Errorf("config:cache failed with exit code %d", result.GetExitCode()), "config:cache failed", "site_id", site.ID)
	}
}

// isAllowedFilePath checks if the file path is in the allowed list (editable or log files)
func (s *FileService) isAllowedFilePath(site *models.Site, filePath string) bool {
	// Check editable files
	editableFiles := s.getEditableFiles(site)
	for _, f := range editableFiles {
		if f.Path == filePath {
			return true
		}
	}

	// Check log files
	logFiles := s.getLogFiles(site)
	for _, f := range logFiles {
		if f.Path == filePath {
			return true
		}
	}

	return false
}

// isEditableFilePath checks if the file path is in the editable files list (not logs)
func (s *FileService) isEditableFilePath(site *models.Site, filePath string) bool {
	editableFiles := s.getEditableFiles(site)
	for _, f := range editableFiles {
		if f.Path == filePath {
			return true
		}
	}

	return false
}
