package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/pkg/utils"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// SiteService handles business logic for sites
type SiteService struct {
	*BaseService
	deploymentService *DeploymentService
	serverRepo        *serverrepos.Repository
}

// NewSiteService creates a new site service
func NewSiteService(
	siteRepo *repositories.SiteRepository,
	deploymentRepo *repositories.DeploymentRepository,
	certificateRepo *repositories.CertificateRepository,
	queueRepo *repositories.QueueRepository,
	commandRepo *repositories.CommandRepository,
	redirectRepo *repositories.RedirectRepository,
	releaseRepo *repositories.ReleaseRepository,
	queueClient *queue.Client,
	ws *websocket.Hub,
	logger *zerolog.Logger,
) *SiteService {
	baseService := NewBaseService(
		siteRepo,
		deploymentRepo,
		certificateRepo,
		queueRepo,
		commandRepo,
		redirectRepo,
		releaseRepo,
		queueClient,
		ws,
		logger,
	)

	return &SiteService{
		BaseService: baseService,
	}
}

// SetDeploymentService sets the deployment service (to avoid circular dependency)
func (s *SiteService) SetDeploymentService(ds *DeploymentService) {
	s.deploymentService = ds
}

// SetServerRepository sets the server repository for cross-module queries
func (s *SiteService) SetServerRepository(repo *serverrepos.Repository) {
	s.serverRepo = repo
}

// List returns all sites for a server
func (s *SiteService) List(ctx context.Context, serverID string) ([]models.Site, error) {
	return s.siteRepo.FindByServerWithLatestDeployment(ctx, serverID)
}

// Create creates a new site
func (s *SiteService) Create(ctx context.Context, serverID, userID, username string, req *dto.CreateSiteRequest) (*models.Site, error) {
	// Check if site with same address exists
	existing, _ := s.siteRepo.FindByAddress(ctx, req.Address, serverID)
	if existing != nil {
		return nil, errors.New("a site with this address already exists on this server")
	}

	// Validate site type
	if !req.Type.IsValid() {
		return nil, fmt.Errorf("invalid site type: %s", req.Type)
	}

	// Build path
	path := fmt.Sprintf("/home/%s/%s", username, req.Address)

	// Set default web folder
	webFolder := req.WebFolder
	if webFolder == "" {
		webFolder = req.Type.GetDefaultWebFolder()
	}

	// Convert SourceControlRepositoriesID from *string to *uint64
	var scRepoID *uint64
	if req.SourceControlRepositoriesID != nil && *req.SourceControlRepositoriesID != "" {
		id, err := strconv.ParseUint(*req.SourceControlRepositoriesID, 10, 64)
		if err == nil {
			scRepoID = &id
		}
	}

	// Convert string fields to pointers where needed
	var phpVersion *string
	if req.PhpVersion != "" {
		phpVersion = &req.PhpVersion
	}

	var repoBranch *string
	if req.RepositoryBranch != "" {
		repoBranch = &req.RepositoryBranch
	}

	site := &models.Site{
		ServerID:                    serverID,
		UserID:                      userID,
		Address:                     req.Address,
		Type:                        req.Type,
		TlsSetting:                  enums.TlsSettingAuto,
		ZeroDowntimeDeployment:      req.ZeroDowntimeDeployment,
		DeploymentReleasesRetention: 5,
		RepositoryBranch:            repoBranch,
		User:                        username,
		Path:                        path,
		WebFolder:                   webFolder,
		PhpVersion:                  phpVersion,
		SourceControlID:             req.SourceControlID,
		SourceControlRepositoriesID: scRepoID,
	}

	// Set aliases
	if len(req.Aliases) > 0 {
		site.Aliases = req.Aliases
	}

	// Apply type-specific defaults
	defaults := req.Type.GetDefaultAttributes(req.ZeroDowntimeDeployment)
	if dirs, ok := defaults["shared_directories"].([]string); ok {
		site.SharedDirectories = dirs
	}

	if dirs, ok := defaults["writeable_directories"].([]string); ok {
		site.WriteableDirectories = dirs
	}

	if files, ok := defaults["shared_files"].([]string); ok {
		site.SharedFiles = files
	}

	if hook, ok := defaults["hook_before_updating_repository"].(string); ok && hook != "" {
		site.HookBeforeUpdatingRepository = &hook
	}

	if hook, ok := defaults["hook_after_updating_repository"].(string); ok && hook != "" {
		site.HookAfterUpdatingRepository = &hook
	}

	if hook, ok := defaults["hook_before_making_current"].(string); ok && hook != "" {
		site.HookBeforeMakingCurrent = &hook
	}

	if hook, ok := defaults["hook_after_making_current"].(string); ok && hook != "" {
		site.HookAfterMakingCurrent = &hook
	}

	if err := s.siteRepo.Create(ctx, site); err != nil {
		return nil, err
	}

	activity.New(s.siteRepo.DB).
		WithContext(ctx).
		UseLog("site").
		CausedByUser(userID).
		On(site).
		WithEvent("created").
		Log("Site was created")

	// Create initial deployment
	if s.deploymentService != nil {
		deployment, err := s.deploymentService.createDeployment(ctx, site, userID, nil)
		if err != nil {
			s.LogError(err, "Failed to create initial deployment", "site_id", site.ID)
		} else {
			site.LatestDeployment = deployment
		}
	}

	s.LogInfo("Site created", "site_id", site.ID, "address", site.Address)

	return site, nil
}

// FindByID finds a site by ID
func (s *SiteService) FindByID(ctx context.Context, id, serverID string) (*models.Site, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, _ := s.deploymentRepo.FindLatestBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	return site, nil
}

// Update updates a site
func (s *SiteService) Update(ctx context.Context, id, serverID, userID string, req *dto.UpdateSiteRequest) (*models.Site, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Build updates map for changed fields
	updates := make(map[string]any)
	updateCaddyfile := false

	// Simple pointer fields
	addIfSet(updates, "php_version", req.PhpVersion)
	addIfSet(updates, "web_folder", req.WebFolder)
	addIfSet(updates, "deploy_notification_email", req.DeployNotificationEmail)
	addIfSet(updates, "deployment_releases_retention", req.DeploymentReleasesRetention)
	addIfSet(updates, "queue_deployments", req.QueueDeployments)

	// Conditional field (not for WordPress)
	if req.RepositoryBranch != nil && site.Type != enums.SiteTypeWordpress {
		updates["repository_branch"] = req.RepositoryBranch
	}

	// Hook fields with line ending normalization
	addHookIfSet(updates, "hook_before_updating_repository", req.HookBeforeUpdatingRepository)
	addHookIfSet(updates, "hook_after_updating_repository", req.HookAfterUpdatingRepository)
	addHookIfSet(updates, "hook_before_making_current", req.HookBeforeMakingCurrent)
	addHookIfSet(updates, "hook_after_making_current", req.HookAfterMakingCurrent)

	// Directory/file fields with multiline parsing
	addSliceIfSet(updates, "shared_directories", req.SharedDirectories)
	addSliceIfSet(updates, "shared_files", req.SharedFiles)
	addSliceIfSet(updates, "writeable_directories", req.WriteableDirectories)

	// Check if Caddyfile needs update (PHP version or web folder changed)
	if req.PhpVersion != nil && (site.PhpVersion == nil || *req.PhpVersion != *site.PhpVersion) {
		updateCaddyfile = true
	}
	if req.WebFolder != nil && *req.WebFolder != site.WebFolder {
		updateCaddyfile = true
	}

	// Apply updates if any
	if len(updates) > 0 {
		if err := s.siteRepo.UpdateFields(ctx, site.ID, updates); err != nil {
			return nil, err
		}
	}

	// Reload site to get updated values
	site, err = s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	activity.New(s.siteRepo.DB).
		WithContext(ctx).
		UseLog("site").
		CausedByUser(userID).
		On(site).
		WithEvent("updated").
		Log("Site was updated")

	// If PHP version or web folder changed, update Caddyfile and deploy
	if updateCaddyfile {
		now := time.Now()
		site.PendingCaddyfileUpdateSince = &now
		s.siteRepo.Update(ctx, site)
		// TODO: Add UpdateCaddyfile job dispatch
	}

	// Load latest deployment
	deployment, _ := s.deploymentRepo.FindLatestBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	s.LogInfo("Site updated", "site_id", site.ID)

	return site, nil
}

// Delete deletes a site
func (s *SiteService) Delete(ctx context.Context, id, serverID string) error {
	site, err := s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	activity.New(s.siteRepo.DB).
		WithContext(ctx).
		UseLog("site").
		On(site).
		WithEvent("deleted").
		Log("Site deletion requested")

	now := time.Now()
	site.UninstallationRequestedAt = &now

	if err := s.siteRepo.Update(ctx, site); err != nil {
		return err
	}

	// TODO: Dispatch site deletion job

	s.LogInfo("Site deletion requested", "site_id", site.ID, "address", site.Address)

	return nil
}

// GetDeletionSummary returns a summary of resources that will be deleted
func (s *SiteService) GetDeletionSummary(ctx context.Context, id, serverID string) (*dto.DeletionSummaryResponse, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	queueCount, _ := s.queueRepo.CountBySite(ctx, site.ID)
	// TODO: Add cron count

	return &dto.DeletionSummaryResponse{
		Queues: int(queueCount),
		Crons:  0,
	}, nil
}

// RegenerateDeployToken regenerates the deploy token for a site
func (s *SiteService) RegenerateDeployToken(ctx context.Context, id, serverID string) error {
	site, err := s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	token := utils.GenerateBase64Token(32)
	site.DeployToken = &token

	return s.siteRepo.Update(ctx, site)
}

// SiteSettingsData holds all data needed for the site settings page
type SiteSettingsData struct {
	Site              *models.Site
	ActiveCertificate *models.Certificate
	PhpVersions       []dto.PhpVersionResponse
	SourceControl     *dto.SourceControlResponse
	Repository        *dto.SourceControlRepositoryResponse
}

// GetSettings returns site settings data including the active certificate, PHP versions, and git info
func (s *SiteService) GetSettings(ctx context.Context, id, serverID string) (*SiteSettingsData, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, _ := s.deploymentRepo.FindLatestBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	// Get active certificate
	activeCert, _ := s.certificateRepo.FindActiveBySite(ctx, site.ID)

	// Get PHP versions from server services
	phpVersions := s.getServerPhpVersions(ctx, serverID)

	// Get source control info if linked
	var sourceControl *dto.SourceControlResponse
	var repository *dto.SourceControlRepositoryResponse
	if site.SourceControlID != nil && *site.SourceControlID != "" {
		sourceControl, repository = s.getSourceControlInfo(ctx, *site.SourceControlID, site.SourceControlRepositoriesID)
	}

	return &SiteSettingsData{
		Site:              site,
		ActiveCertificate: activeCert,
		PhpVersions:       phpVersions,
		SourceControl:     sourceControl,
		Repository:        repository,
	}, nil
}

// getServerPhpVersions returns installed PHP versions for a server using relationship
func (s *SiteService) getServerPhpVersions(ctx context.Context, serverID string) []dto.PhpVersionResponse {
	if s.serverRepo == nil {
		return nil
	}

	server, err := s.serverRepo.FindServerByID(ctx, serverID)
	if err != nil || server == nil {
		return nil
	}

	var result []dto.PhpVersionResponse
	for _, svc := range server.Services {
		if svc.Type == serverenums.ServiceTypePhp {
			result = append(result, dto.PhpVersionResponse{
				Version:   svc.Version,
				IsDefault: svc.IsDefault,
			})
		}
	}

	return result
}

// getSourceControlInfo returns source control and repository info
func (s *SiteService) getSourceControlInfo(ctx context.Context, sourceControlID string, repoID *uint64) (*dto.SourceControlResponse, *dto.SourceControlRepositoryResponse) {
	var sc struct {
		ID       string  `gorm:"column:id"`
		Provider string  `gorm:"column:provider"`
		Login    *string `gorm:"column:login"`
		Name     *string `gorm:"column:name"`
		Type     *string `gorm:"column:type"`
	}

	err := s.siteRepo.DB.WithContext(ctx).
		Table("source_controls").
		Select("id, provider, login, name, type").
		Where("id = ?", sourceControlID).
		First(&sc).Error

	if err != nil {
		return nil, nil
	}

	sourceControl := &dto.SourceControlResponse{
		ID:       sc.ID,
		Provider: sc.Provider,
		Login:    sc.Login,
		Name:     sc.Name,
		Type:     sc.Type,
	}

	var repository *dto.SourceControlRepositoryResponse
	if repoID != nil {
		var repo struct {
			ID            uint64  `gorm:"column:id"`
			Name          string  `gorm:"column:name"`
			FullName      string  `gorm:"column:full_name"`
			DefaultBranch string  `gorm:"column:default_branch"`
			HTMLURL       *string `gorm:"column:html_url"`
		}

		err := s.siteRepo.DB.WithContext(ctx).
			Table("source_control_repositories").
			Select("id, name, full_name, default_branch, html_url").
			Where("id = ?", *repoID).
			First(&repo).Error

		if err == nil {
			repository = &dto.SourceControlRepositoryResponse{
				ID:            repo.ID,
				Name:          repo.Name,
				FullName:      repo.FullName,
				DefaultBranch: repo.DefaultBranch,
				HTMLURL:       repo.HTMLURL,
			}
		}
	}

	return sourceControl, repository
}
