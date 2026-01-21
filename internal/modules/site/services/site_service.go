package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	databasedto "github.com/kkz6/launch-go/internal/modules/database/dto"
	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	gitcontracts "github.com/kkz6/launch-go/internal/modules/git/contracts"
	serverdto "github.com/kkz6/launch-go/internal/modules/server/dto"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/activity"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SiteService handles business logic for sites
type SiteService struct {
	*BaseService
	serverReader         contracts.ServerReader
	gitReader            contracts.GitReader
	cronCreator          contracts.CronCreator
	databaseManager      contracts.DatabaseManager
	dnsRecordService     dnscontracts.DNSRecordService
	sourceControlService gitcontracts.SourceControlService
}

// NewSiteService creates a new site service
func NewSiteService(deps *ServiceDeps) *SiteService {
	return &SiteService{
		BaseService: NewBaseService(deps),
	}
}

// SetServerReader sets the server reader for cross-module queries
func (s *SiteService) SetServerReader(reader contracts.ServerReader) {
	s.serverReader = reader
}

// SetGitReader sets the git reader for cross-module queries
func (s *SiteService) SetGitReader(reader contracts.GitReader) {
	s.gitReader = reader
}

// SetCronCreator sets the cron creator for cross-module operations
func (s *SiteService) SetCronCreator(creator contracts.CronCreator) {
	s.cronCreator = creator
}

// SetDatabaseManager sets the database manager for cross-module operations
func (s *SiteService) SetDatabaseManager(manager contracts.DatabaseManager) {
	s.databaseManager = manager
}

// SetDNSRecordService sets the DNS record service for cross-module operations
func (s *SiteService) SetDNSRecordService(svc dnscontracts.DNSRecordService) {
	s.dnsRecordService = svc
}

// SetSourceControlService sets the source control service for cross-module operations
func (s *SiteService) SetSourceControlService(svc gitcontracts.SourceControlService) {
	s.sourceControlService = svc
}

// List returns all sites for a server filtered by team
func (s *SiteService) List(ctx context.Context, serverID, teamID string) ([]models.Site, error) {
	sites, err := s.Repos().Site().FindByServerAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment for each site
	for i := range sites {
		deployment, err := s.Repos().Deployment().FindLatestBySite(ctx, sites[i].ID)
		if err != nil && !errors.Is(err, repositories.ErrDeploymentNotFound) {
			s.LogWarn("Failed to fetch latest deployment", "siteID", sites[i].ID, "error", err)
		}
		sites[i].LatestDeployment = deployment
	}

	return sites, nil
}

// Create creates a new site
func (s *SiteService) Create(ctx context.Context, serverID, teamID, userID string, req *dto.CreateSiteRequest) (*models.Site, error) {
	// Get server to determine username and validate PHP version
	if s.serverReader == nil {
		return nil, errors.New("server reader not configured")
	}

	server, err := s.serverReader.FindServerByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch server: %w", err)
	}

	username := server.GetUsername()

	// Validate PHP version is a valid enum and installed on server
	phpSoftware, err := serverenums.ParseSoftware(req.PhpVersion)
	if err != nil || !phpSoftware.IsPhp() {
		return nil, fmt.Errorf("invalid PHP version: %s", req.PhpVersion)
	}

	// Check if PHP version is installed on the server
	phpInstalled := false
	expectedVersion := phpSoftware.GetVersion() // e.g., "8.3"
	for _, svc := range server.Services {
		if svc.Type == serverenums.ServiceTypePhp && svc.Version == expectedVersion {
			phpInstalled = true
			break
		}
	}
	if !phpInstalled {
		return nil, fmt.Errorf("PHP version %s is not installed on this server", req.PhpVersion)
	}

	// Check if site with same address exists
	existing, err := s.Repos().Site().FindByAddress(ctx, req.Address, serverID)
	if err != nil && !errors.Is(err, repositories.ErrSiteNotFound) {
		return nil, fmt.Errorf("failed to check for existing site: %w", err)
	}
	if existing != nil {
		return nil, errors.New("a site with this address already exists on this server")
	}

	// Validate site type
	if !req.Type.IsValid() {
		return nil, fmt.Errorf("invalid site type: %s", req.Type)
	}

	// Validate source control for non-WordPress sites
	if req.Type != enums.SiteTypeWordpress && req.SourceControlID != nil && *req.SourceControlID != "" {
		if err := s.validateSourceControl(ctx, req.SourceControlID, req.SourceControlRepositoriesID); err != nil {
			return nil, err
		}
	}

	// Build path
	path := fmt.Sprintf("/home/%s/%s", username, req.Address)

	// Set default web folder
	webFolder := req.WebFolder
	if webFolder == "" {
		webFolder = req.Type.GetDefaultWebFolder()
	}

	// Convert string fields to pointers where needed
	var phpVersion *enums.PhpVersion
	if req.PhpVersion != "" {
		pv := enums.PhpVersion(req.PhpVersion)
		phpVersion = &pv
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

	// Convert source control repositories ID from string to uint64
	var sourceControlRepoID *uint64
	if req.SourceControlRepositoriesID != nil && *req.SourceControlRepositoriesID != "" {
		parsed, err := strconv.ParseUint(*req.SourceControlRepositoriesID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid source_control_repositories_id: %w", err)
		}
		sourceControlRepoID = &parsed
	}

	site := &models.Site{
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
		SourceControlRepositoriesID: sourceControlRepoID,
		ConnectedDomainID:           req.ConnectedDomainID,
	}
	site.ServerID = serverID
	site.TeamID = teamID
	site.UserID = userID

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

	// Use transaction for site creation
	var envVars map[string]string
	err = s.Repos().Site().DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(site).Error; err != nil {
			return err
		}

		activity.LogCreated(ctx, tx, userID, site, "Site was created")

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Save repository to source control provider (non-WordPress sites only)
	if req.Type != enums.SiteTypeWordpress && req.SourceControlID != nil && *req.SourceControlID != "" && sourceControlRepoID != nil {
		s.handleSourceControlRepository(ctx, *req.SourceControlID, *sourceControlRepoID)
	}

	// Handle database creation if requested (outside main transaction)
	if req.CreateDatabase && s.databaseManager != nil {
		envVars = s.handleDatabaseCreation(ctx, site, serverID, teamID, userID, req)
	}

	// Handle DNS record creation (outside transaction - non-critical)
	if req.CreateDNSRecord && req.ConnectedDomainID != nil && *req.ConnectedDomainID != "" {
		serverIP := ""
		if server.PublicIPv4 != nil {
			serverIP = *server.PublicIPv4
		}
		s.handleDNSRecordCreation(ctx, site, *req.ConnectedDomainID, server.TeamID, serverIP)
	}

	// Handle scheduler creation for Laravel and WordPress sites
	if req.CreateScheduler && s.cronCreator != nil {
		if req.Type == enums.SiteTypeLaravel || req.Type == enums.SiteTypeWordpress {
			s.handleSchedulerCreation(ctx, site, serverID, userID)
		}
	}

	// Handle queue worker creation for Laravel sites
	if req.CreateQueue && req.Type == enums.SiteTypeLaravel {
		s.handleQueueCreation(ctx, site, serverID, userID)
	}

	// Create initial deployment with environment variables and git commit data
	if s.Services() != nil {
		// Fetch git commit data if source control is configured
		commitData := s.Services().Deployment().FetchLatestCommitData(ctx, site)
		if commitData == nil {
			commitData = make(map[string]interface{})
		}

		// Add environment variables to commit data
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

	// Broadcast site created event
	s.BroadcastToTeam(server.TeamID, "site.created", map[string]interface{}{
		"team_id": server.TeamID,
		"site":    dto.ToSiteResponse(site),
	})

	return site, nil
}

// handleDatabaseCreation creates a database during site creation
func (s *SiteService) handleDatabaseCreation(ctx context.Context, site *models.Site, serverID, teamID, userID string, req *dto.CreateSiteRequest) map[string]string {
	envVars := make(map[string]string)
	envVarNames := site.Type.GetDatabaseEnvVarNames()

	// Use existing database
	if req.DatabaseOption == "existing" && req.DatabaseID != nil {
		// Fetch existing database details and build env vars
		dbInfo := s.getExistingDatabaseInfo(ctx, *req.DatabaseID, serverID, teamID)
		if dbInfo != nil {
			if envVarNames["database"] != "" {
				envVars[envVarNames["database"]] = dbInfo.Name
			}
			if envVarNames["host"] != "" {
				envVars[envVarNames["host"]] = "127.0.0.1"
			}
			if envVarNames["connection"] != "" {
				envVars[envVarNames["connection"]] = dbInfo.ConnectionName
			}
			if envVarNames["port"] != "" {
				envVars[envVarNames["port"]] = dbInfo.Port
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

	database, err := s.databaseManager.CreateDatabase(ctx, serverID, teamID, dbReq, &userID)
	if err != nil {
		s.LogError(err, "Failed to create database during site creation", "site_id", site.ID)
		return envVars
	}

	// Get the database type from the server's installed services
	dbType := s.getDatabaseTypeForServer(ctx, serverID)

	// Build environment variables
	if envVarNames["database"] != "" {
		envVars[envVarNames["database"]] = database.Name
	}
	if envVarNames["host"] != "" {
		envVars[envVarNames["host"]] = "127.0.0.1"
	}
	if envVarNames["connection"] != "" {
		envVars[envVarNames["connection"]] = dbType.ConnectionName()
	}
	if envVarNames["port"] != "" {
		envVars[envVarNames["port"]] = dbType.Port()
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

// DatabaseInfo holds database connection details
type DatabaseInfo struct {
	Name           string
	ConnectionName string
	Port           string
}

// getExistingDatabaseInfo fetches database info for existing database
func (s *SiteService) getExistingDatabaseInfo(ctx context.Context, databaseID, serverID, teamID string) *DatabaseInfo {
	if s.databaseManager == nil {
		return nil
	}

	database, err := s.databaseManager.GetDatabase(ctx, databaseID, serverID, teamID)
	if err != nil {
		return nil
	}

	// Get the database type from the server's installed services
	dbType := s.getDatabaseTypeForServer(ctx, serverID)

	return &DatabaseInfo{
		Name:           database.Name,
		ConnectionName: dbType.ConnectionName(),
		Port:           dbType.Port(),
	}
}

// getDatabaseTypeForServer returns the database software type installed on the server
func (s *SiteService) getDatabaseTypeForServer(ctx context.Context, serverID string) serverenums.Software {
	// Query installed services to find the database type
	if s.serverReader == nil {
		return serverenums.SoftwareMySql80 // Default to MySQL if we can't determine
	}

	services, err := s.serverReader.FindServicesByServer(ctx, serverID)
	if err != nil {
		return serverenums.SoftwareMySql80 // Default to MySQL if we can't determine
	}

	for _, service := range services {
		if service.Type == serverenums.ServiceTypeMySql {
			return serverenums.SoftwareMySql80
		}
		if service.Type == serverenums.ServiceTypePostgreSql {
			return serverenums.SoftwarePostgreSql16
		}
	}

	return serverenums.SoftwareMySql80 // Default to MySQL
}

// handleSchedulerCreation creates a cron job for Laravel/WordPress scheduler
func (s *SiteService) handleSchedulerCreation(ctx context.Context, site *models.Site, serverID, userID string) {
	// Get team ID from server
	server, err := s.serverReader.FindServerByID(ctx, serverID)
	if err != nil {
		s.LogError(err, "Failed to get server for scheduler creation", "site_id", site.ID)
		return
	}

	// Build the scheduler command based on site type
	phpBinary := site.GetPhpBinary()

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

	cron, err := s.cronCreator.CreateCron(ctx, serverID, server.TeamID, cronReq)
	if err != nil {
		s.LogError(err, "Failed to create scheduler cron for site", "site_id", site.ID)
		return
	}

	// Update enabled_features with scheduler info
	now := time.Now()
	feature := models.EnabledFeature{
		Name:      "scheduler",
		CronID:    &cron.ID,
		EnabledAt: &now,
	}
	site.AddEnabledFeature(feature)
	if err := s.Repos().Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
	}); err != nil {
		s.LogError(err, "Failed to update enabled_features for scheduler", "site_id", site.ID)
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
	queue, err := s.Services().Queue().Create(ctx, site.ID, serverID, userID, queueReq)
	if err != nil {
		s.LogError(err, "Failed to create queue worker for site", "site_id", site.ID)
		return
	}

	// Update enabled_features with queue info
	now := time.Now()
	feature := models.EnabledFeature{
		Name:      "queue",
		QueueID:   &queue.ID,
		EnabledAt: &now,
	}
	site.AddEnabledFeature(feature)
	if err := s.Repos().Site().UpdateFields(ctx, site.ID, map[string]interface{}{
		"enabled_features": site.EnabledFeatures,
	}); err != nil {
		s.LogError(err, "Failed to update enabled_features for queue", "site_id", site.ID)
	}

	s.LogInfo("Queue worker created for site", "site_id", site.ID)
}

// validateSourceControl validates source control and repository exist
func (s *SiteService) validateSourceControl(ctx context.Context, sourceControlID *string, repoID *string) error {
	if sourceControlID == nil || *sourceControlID == "" {
		return nil
	}

	// Check source control exists
	var count int64
	err := s.Repos().Site().DB.WithContext(ctx).
		Table("source_controls").
		Where("id = ?", *sourceControlID).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to validate source control: %w", err)
	}
	if count == 0 {
		return errors.New("source control not found")
	}

	// Check repository exists if provided
	if repoID != nil && *repoID != "" {
		err := s.Repos().Site().DB.WithContext(ctx).
			Table("source_control_repositories").
			Where("id = ?", *repoID).
			Count(&count).Error
		if err != nil {
			return fmt.Errorf("failed to validate repository: %w", err)
		}
		if count == 0 {
			return errors.New("repository not found")
		}
	}

	return nil
}

// handleDNSRecordCreation creates a DNS A record for a site
func (s *SiteService) handleDNSRecordCreation(ctx context.Context, site *models.Site, domainID, teamID, serverIP string) {
	if s.dnsRecordService == nil {
		s.LogError(nil, "DNS service not configured, skipping DNS record creation", "site_id", site.ID)
		return
	}

	if serverIP == "" {
		s.LogError(nil, "Server has no public IP, skipping DNS record creation", "site_id", site.ID)
		return
	}

	// Create DNS record via service (handles subdomain calculation internally)
	err := s.dnsRecordService.CreateRecordForSite(ctx, domainID, teamID, site.Address, serverIP)
	if err != nil {
		s.LogError(err, "Failed to create DNS record for site", "site_id", site.ID, "domain_id", domainID)
		return
	}

	s.LogInfo("DNS record created for site", "site_id", site.ID)
}

// handleSourceControlRepository fetches and saves repository data from the git provider
func (s *SiteService) handleSourceControlRepository(ctx context.Context, sourceControlID string, repoID uint64) {
	if s.sourceControlService == nil {
		s.LogError(nil, "Source control service not configured, skipping repository save")
		return
	}

	if s.gitReader == nil {
		s.LogError(nil, "Git reader not configured, skipping repository save")
		return
	}

	// Get repository from database using the ID
	repo, err := s.gitReader.FindRepositoryByID(ctx, strconv.FormatUint(repoID, 10))
	if err != nil {
		s.LogError(err, "Failed to get repository", "repo_id", repoID)
		return
	}

	// Save repository to source control (fetches latest data from provider)
	_, err = s.sourceControlService.SaveRepository(ctx, sourceControlID, repo.FullName)
	if err != nil {
		s.LogError(err, "Failed to save repository to source control", "source_control_id", sourceControlID, "full_name", repo.FullName)
		return
	}

	s.LogInfo("Repository saved to source control", "source_control_id", sourceControlID, "full_name", repo.FullName)
}

// FindByID finds a site by ID with team validation
func (s *SiteService) FindByID(ctx context.Context, id, serverID, teamID string) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, err := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	if err != nil && !errors.Is(err, repositories.ErrDeploymentNotFound) {
		s.LogWarn("Failed to fetch latest deployment", "siteID", site.ID, "error", err)
	}
	site.LatestDeployment = deployment

	return site, nil
}

// Update updates a site
func (s *SiteService) Update(ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateSiteRequest) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
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
	if req.PhpVersion != nil && (site.PhpVersion == nil || *req.PhpVersion != site.PhpVersion.String()) {
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
	site, err = s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	activity.LogUpdated(ctx, s.Repos().Site().DB, userID, site, "Site was updated")

	// If PHP version or web folder changed, update Caddyfile and deploy
	if updateCaddyfile {
		now := time.Now()
		site.PendingCaddyfileUpdateSince = &now
		s.Repos().Site().Update(ctx, site)
		// TODO: Add UpdateCaddyfile job dispatch
	}

	// Load latest deployment
	deployment, err := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	if err != nil && !errors.Is(err, repositories.ErrDeploymentNotFound) {
		s.LogWarn("Failed to fetch latest deployment", "siteID", site.ID, "error", err)
	}
	site.LatestDeployment = deployment

	s.LogInfo("Site updated", "site_id", site.ID)

	// Broadcast site updated event
	if s.serverReader != nil {
		if server, err := s.serverReader.FindServerByID(ctx, serverID); err == nil {
			s.BroadcastToTeam(server.TeamID, "site.updated", map[string]interface{}{
				"team_id": server.TeamID,
				"site":    dto.ToSiteResponse(site),
			})
		}
	}

	return site, nil
}

// Delete deletes a site
func (s *SiteService) Delete(ctx context.Context, id, serverID, teamID string) error {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return err
	}

	activity.LogEvent(ctx, s.Repos().Site().DB, "deleted", "", site, "Site deletion requested")

	now := time.Now()
	site.UninstallationRequestedAt = &now

	if err := s.Repos().Site().Update(ctx, site); err != nil {
		return err
	}

	// Dispatch site uninstall job
	task, err := jobs.NewUninstallSiteTask(site.ID, serverID, nil)
	if err != nil {
		s.LogError(err, "Failed to create uninstall site task", "site_id", site.ID)
		return err
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue uninstall site task", "site_id", site.ID)
		return err
	}

	s.LogInfo("Site deletion requested", "site_id", site.ID, "address", site.Address)

	return nil
}

// GetDeletionSummary returns a summary of resources that will be deleted
func (s *SiteService) GetDeletionSummary(ctx context.Context, id, serverID, teamID string) (*dto.DeletionSummaryResponse, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	queueCount, err := s.Repos().Queue().CountBySite(ctx, site.ID)
	if err != nil {
		s.LogWarn("Failed to count queues for deletion summary", "siteID", site.ID, "error", err)
		queueCount = 0
	}
	// TODO: Add cron count

	return &dto.DeletionSummaryResponse{
		Queues: int(queueCount),
		Crons:  0,
	}, nil
}

// RegenerateDeployToken regenerates the deploy token for a site
func (s *SiteService) RegenerateDeployToken(ctx context.Context, id, serverID, teamID string) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
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
func (s *SiteService) GetSettings(ctx context.Context, id, serverID, teamID string) (*SiteSettingsData, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, err := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	if err != nil && !errors.Is(err, repositories.ErrDeploymentNotFound) {
		s.LogWarn("Failed to fetch latest deployment for settings", "siteID", site.ID, "error", err)
	}
	site.LatestDeployment = deployment

	// Get active certificate
	activeCert, err := s.Repos().Certificate().FindActiveBySite(ctx, site.ID)
	if err != nil && !errors.Is(err, repositories.ErrCertificateNotFound) {
		s.LogWarn("Failed to fetch active certificate", "siteID", site.ID, "error", err)
	}

	// Get PHP versions from server services
	phpVersions := s.getServerPhpVersions(ctx, serverID)

	// Get source control info if linked
	var sourceControl *dto.SourceControlResponse
	var repository *dto.SourceControlRepositoryResponse
	if site.SourceControlID != nil && *site.SourceControlID != "" {
		sourceControl, repository = s.GetSourceControlInfo(ctx, *site.SourceControlID, site.SourceControlRepositoriesID)
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
	if s.serverReader == nil {
		return nil
	}

	server, err := s.serverReader.FindServerByID(ctx, serverID)
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

// GetSourceControlInfo returns source control and repository info
func (s *SiteService) GetSourceControlInfo(ctx context.Context, sourceControlID string, repoID *uint64) (*dto.SourceControlResponse, *dto.SourceControlRepositoryResponse) {
	if s.gitReader == nil {
		return nil, nil
	}

	sc, err := s.gitReader.FindSourceControlByID(ctx, sourceControlID)
	if err != nil {
		return nil, nil
	}

	sourceControl := &dto.SourceControlResponse{
		ID:       sc.ID,
		Provider: string(sc.Provider),
		Login:    sc.Login,
		Name:     sc.Name,
		Type:     sc.Type,
	}

	var repository *dto.SourceControlRepositoryResponse
	if repoID != nil {
		repo, err := s.gitReader.FindRepositoryByID(ctx, strconv.FormatUint(*repoID, 10))
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
