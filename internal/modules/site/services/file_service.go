package services

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/support"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// FileOnServer represents an editable file on the server
type FileOnServer struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Path        string             `json:"path"`
	Context     string             `json:"context,omitempty"`
	Type        string             `json:"type"`        // "default" or "environment"
	FileType    enums.SiteFileType `json:"file_type"`   // enum value for identification
	ShowRoute   string             `json:"show_route"`  // encrypted URL parameter for viewing
	UpdateRoute string             `json:"update_route"` // encrypted URL parameter for updating
}

// FileService handles file operations on sites
type FileService struct {
	db       *gorm.DB
	siteRepo *repositories.SiteRepository
	logger   *zerolog.Logger
}

// NewFileService creates a new FileService instance
func NewFileService(db *gorm.DB, siteRepo *repositories.SiteRepository, logger *zerolog.Logger) *FileService {
	return &FileService{
		db:       db,
		siteRepo: siteRepo,
		logger:   logger,
	}
}

// ListFiles returns the list of editable files for a site
func (s *FileService) ListFiles(ctx context.Context, serverID, siteID string) ([]FileOnServer, error) {
	// Get site
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	return s.getEditableFiles(site), nil
}

// ListLogFiles returns the list of log files for a site
func (s *FileService) ListLogFiles(ctx context.Context, serverID, siteID string) ([]FileOnServer, error) {
	// Get site
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	return s.getLogFiles(site), nil
}

// getEditableFiles returns the list of editable files based on site type
func (s *FileService) getEditableFiles(site *models.Site) []FileOnServer {
	fileTypes := enums.EditableFilesForSiteType(site.Type)
	return s.buildFileList(site, fileTypes)
}

// getLogFiles returns the list of log files based on site type
func (s *FileService) getLogFiles(site *models.Site) []FileOnServer {
	fileTypes := enums.LogFilesForSiteType(site.Type)
	return s.buildFileList(site, fileTypes)
}

// buildFileList builds a list of FileOnServer from file types
func (s *FileService) buildFileList(site *models.Site, fileTypes []enums.SiteFileType) []FileOnServer {
	files := make([]FileOnServer, 0, len(fileTypes))

	for _, ft := range fileTypes {
		path := ft.GetPath(site.Path, site.ZeroDowntimeDeployment)
		fileType := ft.FileType()

		// Generate encrypted route parameters
		showRoute, _ := support.EncodeFileRouteParam(path, fileType)
		updateRoute, _ := support.EncodeFileRouteParam(path, fileType)

		files = append(files, FileOnServer{
			Name:        ft.Name(),
			Description: ft.Description(),
			Path:        path,
			Context:     site.Address,
			Type:        fileType,
			FileType:    ft,
			ShowRoute:   showRoute,
			UpdateRoute: updateRoute,
		})
	}

	return files
}

// GetFileContent reads the content of a file from the server
func (s *FileService) GetFileContent(ctx context.Context, serverID, siteID, filePath string) (string, error) {
	// Get site to verify access
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return "", fmt.Errorf("site not found: %w", err)
	}

	// Verify the file path is in the allowed list
	if !s.isAllowedFilePath(site, filePath) {
		return "", fmt.Errorf("file path not allowed")
	}

	// Get the server for SSH connection
	var server servermodels.Server
	if err := s.db.First(&server, "id = ?", serverID).Error; err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Create SSH connection
	sshClient, err := s.createSSHClient(&server)
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer sshClient.Close()

	if err := sshClient.Connect(); err != nil {
		return "", fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	// Read file content
	cmd := fmt.Sprintf(`cat "%s" 2>/dev/null || echo ""`, filePath)
	result, err := sshClient.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return result.Stdout, nil
}

// UpdateFileContent updates the content of a file on the server
func (s *FileService) UpdateFileContent(ctx context.Context, serverID, siteID, filePath, content string) error {
	// Get site to verify access
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	// Verify the file path is in the allowed list (only editable files, not logs)
	if !s.isEditableFilePath(site, filePath) {
		return fmt.Errorf("file path not allowed for editing")
	}

	// Get the server for SSH connection
	var server servermodels.Server
	if err := s.db.First(&server, "id = ?", serverID).Error; err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Create SSH connection
	sshClient, err := s.createSSHClient(&server)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer sshClient.Close()

	if err := sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	// Write file content using heredoc
	cmd := fmt.Sprintf(`cat > "%s" << 'LAUNCH_EOF'
%s
LAUNCH_EOF`, filePath, content)

	result, err := sshClient.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write file: exit code %d", result.ExitCode)
	}

	return nil
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

// createSSHClient creates an SSH client for the server
func (s *FileService) createSSHClient(server *servermodels.Server) (*taskrunner.SSHClient, error) {
	host := ""
	if server.PublicIPv4 != nil {
		host = *server.PublicIPv4
	}

	port := 22
	if server.SSHPort != nil {
		port = *server.SSHPort
	}

	privateKey := ""
	if server.PrivateKey != nil {
		privateKey = *server.PrivateKey
	}

	return taskrunner.NewSSHClient(taskrunner.SSHConfig{
		Host:       host,
		Port:       port,
		User:       "launcher",
		PrivateKey: privateKey,
	})
}
