package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// SiteService handles business logic for sites
type SiteService struct {
	*BaseService
	deploymentService *DeploymentService
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

	siteType, err := enums.ParseSiteType(req.Type)
	if err != nil {
		return nil, err
	}

	// Build path
	path := fmt.Sprintf("/home/%s/%s", username, req.Address)

	// Set default web folder
	webFolder := req.WebFolder
	if webFolder == "" {
		if siteType == enums.SiteTypeWordpress {
			webFolder = "/"
		} else {
			webFolder = "public"
		}
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
		Type:                        siteType,
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
		if err := site.SetAliases(req.Aliases); err != nil {
			return nil, err
		}
	}

	// Apply type-specific defaults
	defaults := siteType.GetDefaultAttributes(req.ZeroDowntimeDeployment)
	if dirs, ok := defaults["shared_directories"].([]string); ok {
		if err := site.SetSharedDirectories(dirs); err != nil {
			return nil, err
		}
	}

	if dirs, ok := defaults["writeable_directories"].([]string); ok {
		if err := site.SetWriteableDirectories(dirs); err != nil {
			return nil, err
		}
	}

	if files, ok := defaults["shared_files"].([]string); ok {
		if err := site.SetSharedFiles(files); err != nil {
			return nil, err
		}
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

	// Track if we need to update Caddyfile
	updateCaddyfile := false
	oldPhpVersion := site.PhpVersion
	oldWebFolder := site.WebFolder

	// Update fields
	if req.PhpVersion != nil {
		site.PhpVersion = req.PhpVersion
		if oldPhpVersion == nil || *req.PhpVersion != *oldPhpVersion {
			updateCaddyfile = true
		}
	}

	if req.WebFolder != nil {
		site.WebFolder = *req.WebFolder
		if *req.WebFolder != oldWebFolder {
			updateCaddyfile = true
		}
	}

	if req.RepositoryBranch != nil && site.Type != enums.SiteTypeWordpress {
		site.RepositoryBranch = req.RepositoryBranch
	}

	if req.DeployNotificationEmail != nil {
		site.DeployNotificationEmail = req.DeployNotificationEmail
	}

	if req.DeploymentReleasesRetention != nil {
		site.DeploymentReleasesRetention = *req.DeploymentReleasesRetention
	}

	if req.QueueDeployments != nil {
		site.QueueDeployments = *req.QueueDeployments
	}

	// Handle hooks
	if req.HookBeforeUpdatingRepository != nil {
		normalized := normalizeLineEndings(*req.HookBeforeUpdatingRepository)
		site.HookBeforeUpdatingRepository = &normalized
	}

	if req.HookAfterUpdatingRepository != nil {
		normalized := normalizeLineEndings(*req.HookAfterUpdatingRepository)
		site.HookAfterUpdatingRepository = &normalized
	}

	if req.HookBeforeMakingCurrent != nil {
		normalized := normalizeLineEndings(*req.HookBeforeMakingCurrent)
		site.HookBeforeMakingCurrent = &normalized
	}

	if req.HookAfterMakingCurrent != nil {
		normalized := normalizeLineEndings(*req.HookAfterMakingCurrent)
		site.HookAfterMakingCurrent = &normalized
	}

	// Handle directories and files
	if req.SharedDirectories != nil {
		dirs := parseMultilineToSlice(*req.SharedDirectories)
		if err := site.SetSharedDirectories(dirs); err != nil {
			return nil, err
		}
	}

	if req.SharedFiles != nil {
		files := parseMultilineToSlice(*req.SharedFiles)
		if err := site.SetSharedFiles(files); err != nil {
			return nil, err
		}
	}

	if req.WriteableDirectories != nil {
		dirs := parseMultilineToSlice(*req.WriteableDirectories)
		if err := site.SetWriteableDirectories(dirs); err != nil {
			return nil, err
		}
	}

	if err := s.siteRepo.Update(ctx, site); err != nil {
		return nil, err
	}

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

	token := models.GenerateRandomToken(32)
	site.DeployToken = &token

	return s.siteRepo.Update(ctx, site)
}
