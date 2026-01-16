package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	databasedto "github.com/kkz6/launch-go/internal/modules/database/dto"
	databaseservices "github.com/kkz6/launch-go/internal/modules/database/services"
	serverdto "github.com/kkz6/launch-go/internal/modules/server/dto"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	serverservices "github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SiteService handles business logic for sites
type SiteService struct {
	*BaseService
	serverRepos     *serverrepos.Registry
	serverService   *serverservices.Service
	databaseService *databaseservices.Service
}

// NewSiteService creates a new site service
func NewSiteService(deps *ServiceDeps) *SiteService {
	return &SiteService{
		BaseService: NewBaseService(deps),
	}
}

// SetServerRepos sets the server repository registry for cross-module queries
func (s *SiteService) SetServerRepos(repos *serverrepos.Registry) {
	s.serverRepos = repos
}

// SetServerService sets the server service for cross-module operations
func (s *SiteService) SetServerService(svc *serverservices.Service) {
	s.serverService = svc
}

// SetDatabaseService sets the database service for cross-module operations
func (s *SiteService) SetDatabaseService(svc *databaseservices.Service) {
	s.databaseService = svc
}

// List returns all sites for a server
func (s *SiteService) List(ctx context.Context, serverID string) ([]models.Site, error) {
	return s.Repos().Site().FindByServerWithLatestDeployment(ctx, serverID)
}

// Create creates a new site
func (s *SiteService) Create(ctx context.Context, serverID, userID, username string, req *dto.CreateSiteRequest) (*models.Site, error) {
	// Check if site with same address exists
	existing, _ := s.Repos().Site().FindByAddress(ctx, req.Address, serverID)
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

	// Convert string fields to pointers where needed
	var phpVersion *string
	if req.PhpVersion != "" {
		phpVersion = &req.PhpVersion
	}

	var repoBranch *string
	if req.RepositoryBranch != "" {
		repoBranch = &req.RepositoryBranch
	}

	// WordPress sites don't support zero-downtime deployment
	zeroDowntime := req.ZeroDowntimeDeployment
	if req.Type == enums.SiteTypeWordpress {
		zeroDowntime = false
	}

	site := &models.Site{
		ServerID:                    serverID,
		UserID:                      userID,
		Address:                     req.Address,
		Type:                        req.Type,
		TlsSetting:                  enums.TlsSettingAuto,
		ZeroDowntimeDeployment:      zeroDowntime,
		DeploymentReleasesRetention: 5,
		RepositoryBranch:            repoBranch,
		User:                        username,
		Path:                        path,
		WebFolder:                   webFolder,
		PhpVersion:                  phpVersion,
		SourceControlID:             req.SourceControlID,
		SourceControlRepositoriesID: req.SourceControlRepositoriesID,
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

	if err := s.Repos().Site().Create(ctx, site); err != nil {
		return nil, err
	}

	activity.New(s.Repos().Site().DB).
		WithContext(ctx).
		UseLog("site").
		CausedByUser(userID).
		On(site).
		WithEvent("created").
		Log("Site was created")

	// Handle database creation if requested
	var envVars map[string]string
	if req.CreateDatabase && s.databaseService != nil {
		envVars = s.handleDatabaseCreation(ctx, site, serverID, userID, req)
	}

	// Handle scheduler creation for Laravel and WordPress sites
	if req.CreateScheduler && s.serverService != nil {
		if req.Type == enums.SiteTypeLaravel || req.Type == enums.SiteTypeWordpress {
			s.handleSchedulerCreation(ctx, site, serverID, userID)
		}
	}

	// Handle queue worker creation for Laravel sites
	if req.CreateQueue && req.Type == enums.SiteTypeLaravel {
		s.handleQueueCreation(ctx, site, serverID, userID)
	}

	// Create initial deployment with environment variables
	if s.Services() != nil {
		commitData := make(map[string]interface{})
		if len(envVars) > 0 {
			commitData["env_variables"] = envVars
		}

		var deployCommitData map[string]interface{}
		if len(commitData) > 0 {
			deployCommitData = commitData
		}

		deployment, err := s.Services().Deployment().createDeployment(ctx, site, userID, deployCommitData)
		if err != nil {
			s.LogError(err, "Failed to create initial deployment", "site_id", site.ID)
		} else {
			site.LatestDeployment = deployment
		}
	}

	s.LogInfo("Site created", "site_id", site.ID, "address", site.Address)

	return site, nil
}

// handleDatabaseCreation creates a database during site creation
func (s *SiteService) handleDatabaseCreation(ctx context.Context, site *models.Site, serverID, userID string, req *dto.CreateSiteRequest) map[string]string {
	envVars := make(map[string]string)
	envVarNames := site.Type.GetDatabaseEnvVarNames()

	// Use existing database
	if req.DatabaseOption == "existing" && req.DatabaseID != nil {
		// Fetch existing database details and build env vars
		dbInfo := s.getExistingDatabaseInfo(ctx, *req.DatabaseID, serverID)
		if dbInfo != nil {
			if envVarNames["database"] != "" {
				envVars[envVarNames["database"]] = dbInfo.Name
			}
			if envVarNames["host"] != "" {
				envVars[envVarNames["host"]] = "127.0.0.1"
			}
			if envVarNames["connection"] != "" {
				envVars[envVarNames["connection"]] = "mysql"
			}
			if envVarNames["port"] != "" {
				envVars[envVarNames["port"]] = "3306"
			}
		}
		return envVars
	}

	// Create new database
	if req.DatabaseName == nil || *req.DatabaseName == "" {
		return envVars
	}

	dbReq := &databasedto.CreateDatabaseRequest{
		Name: *req.DatabaseName,
	}

	if req.DatabaseUserOption == "new" && req.DatabaseUserName != nil && req.DatabaseUserPassword != nil {
		dbReq.CreateUser = true
		dbReq.UserName = *req.DatabaseUserName
		dbReq.UserPassword = *req.DatabaseUserPassword
	} else if req.DatabaseUserOption == "existing" && req.DatabaseUserID != nil {
		dbReq.CreateUser = false
		dbReq.ExistingUserID = req.DatabaseUserID
	}

	database, err := s.databaseService.CreateDatabase(ctx, serverID, dbReq, &userID)
	if err != nil {
		s.LogError(err, "Failed to create database during site creation", "site_id", site.ID)
		return envVars
	}

	// Build environment variables
	if envVarNames["database"] != "" {
		envVars[envVarNames["database"]] = database.Name
	}
	if envVarNames["host"] != "" {
		envVars[envVarNames["host"]] = "127.0.0.1"
	}
	if envVarNames["connection"] != "" {
		envVars[envVarNames["connection"]] = "mysql"
	}
	if envVarNames["port"] != "" {
		envVars[envVarNames["port"]] = "3306"
	}

	// Add user credentials if new user was created
	if req.DatabaseUserOption == "new" && req.DatabaseUserName != nil && req.DatabaseUserPassword != nil {
		if envVarNames["username"] != "" {
			envVars[envVarNames["username"]] = *req.DatabaseUserName
		}
		if envVarNames["password"] != "" {
			envVars[envVarNames["password"]] = *req.DatabaseUserPassword
		}
	}

	s.LogInfo("Database created for site", "site_id", site.ID, "database_id", database.ID)

	return envVars
}

// getExistingDatabaseInfo fetches database info for existing database
func (s *SiteService) getExistingDatabaseInfo(ctx context.Context, databaseID, serverID string) *struct{ Name string } {
	if s.databaseService == nil {
		return nil
	}

	database, err := s.databaseService.GetDatabase(ctx, databaseID, serverID)
	if err != nil {
		return nil
	}

	return &struct{ Name string }{Name: database.Name}
}

// handleSchedulerCreation creates a cron job for Laravel/WordPress scheduler
func (s *SiteService) handleSchedulerCreation(ctx context.Context, site *models.Site, serverID, userID string) {
	// Get team ID from server
	server, err := s.serverRepos.Server().FindByID(ctx, serverID)
	if err != nil {
		s.LogError(err, "Failed to get server for scheduler creation", "site_id", site.ID)
		return
	}

	// Build the scheduler command based on site type
	phpBinary := "php"
	if site.PhpVersion != nil {
		phpBinary = fmt.Sprintf("php%s", *site.PhpVersion)
	}

	var command string
	switch site.Type {
	case enums.SiteTypeWordpress:
		// WordPress cron uses wp-cron.php
		command = fmt.Sprintf("cd %s && %s wp-cron.php >> /dev/null 2>&1", site.GetWebDirectory(), phpBinary)
	case enums.SiteTypeLaravel:
		// Laravel uses artisan schedule:run
		command = fmt.Sprintf("cd %s && %s artisan schedule:run >> /dev/null 2>&1", site.GetApplicationDirectory(), phpBinary)
	default:
		s.LogError(nil, "Scheduler not supported for site type", "site_id", site.ID, "type", site.Type)
		return
	}

	frequency := "every_minute"

	cronReq := &serverdto.CreateCronRequest{
		Expression: "* * * * *",
		Command:    command,
		User:       site.User,
		Frequency:  &frequency,
		SiteID:     &site.ID,
	}

	_, err = s.serverService.CreateCron(ctx, serverID, server.TeamID, cronReq)
	if err != nil {
		s.LogError(err, "Failed to create scheduler cron for site", "site_id", site.ID)
		return
	}

	s.LogInfo("Scheduler cron created for site", "site_id", site.ID)
}

// handleQueueCreation creates a queue worker for Laravel sites
func (s *SiteService) handleQueueCreation(ctx context.Context, site *models.Site, serverID, userID string) {
	queueReq := &dto.CreateQueueRequest{
		QueueConnection:       "database",
		Queue:                 "default",
		User:                  &site.User,
		RestSecondsOnEmpty:    3,
		MaxSecondsPerJob:      60,
		FailedJobDelaySeconds: 3,
		RunOnMaintenance:      false,
		RunWithListen:         false,
	}

	maxTries := 3
	queueReq.MaxTries = &maxTries

	maxMemory := 128
	queueReq.MaxMemory = &maxMemory

	numProcs := 1
	queueReq.NumProcs = &numProcs

	// Use service registry to access queue service
	_, err := s.Services().Queue().Create(ctx, site.ID, serverID, userID, queueReq)
	if err != nil {
		s.LogError(err, "Failed to create queue worker for site", "site_id", site.ID)
		return
	}

	s.LogInfo("Queue worker created for site", "site_id", site.ID)
}

// FindByID finds a site by ID
func (s *SiteService) FindByID(ctx context.Context, id, serverID string) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, _ := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	return site, nil
}

// Update updates a site
func (s *SiteService) Update(ctx context.Context, id, serverID, userID string, req *dto.UpdateSiteRequest) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
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
		if err := s.Repos().Site().UpdateFields(ctx, site.ID, updates); err != nil {
			return nil, err
		}
	}

	// Reload site to get updated values
	site, err = s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	activity.New(s.Repos().Site().DB).
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
		s.Repos().Site().Update(ctx, site)
		// TODO: Add UpdateCaddyfile job dispatch
	}

	// Load latest deployment
	deployment, _ := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	s.LogInfo("Site updated", "site_id", site.ID)

	return site, nil
}

// Delete deletes a site
func (s *SiteService) Delete(ctx context.Context, id, serverID string) error {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	activity.New(s.Repos().Site().DB).
		WithContext(ctx).
		UseLog("site").
		On(site).
		WithEvent("deleted").
		Log("Site deletion requested")

	now := time.Now()
	site.UninstallationRequestedAt = &now

	if err := s.Repos().Site().Update(ctx, site); err != nil {
		return err
	}

	// TODO: Dispatch site deletion job

	s.LogInfo("Site deletion requested", "site_id", site.ID, "address", site.Address)

	return nil
}

// GetDeletionSummary returns a summary of resources that will be deleted
func (s *SiteService) GetDeletionSummary(ctx context.Context, id, serverID string) (*dto.DeletionSummaryResponse, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	queueCount, _ := s.Repos().Queue().CountBySite(ctx, site.ID)
	// TODO: Add cron count

	return &dto.DeletionSummaryResponse{
		Queues: int(queueCount),
		Crons:  0,
	}, nil
}

// RegenerateDeployToken regenerates the deploy token for a site
func (s *SiteService) RegenerateDeployToken(ctx context.Context, id, serverID string) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	token := utils.GenerateBase64Token(32)
	site.DeployToken = &token

	if err := s.Repos().Site().Update(ctx, site); err != nil {
		return nil, err
	}

	return site, nil
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
	site, err := s.Repos().Site().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, _ := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	// Get active certificate
	activeCert, _ := s.Repos().Certificate().FindActiveBySite(ctx, site.ID)

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
	if s.serverRepos == nil {
		return nil
	}

	server, err := s.serverRepos.Server().FindByID(ctx, serverID)
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

	err := s.Repos().Site().DB.WithContext(ctx).
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

		err := s.Repos().Site().DB.WithContext(ctx).
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
