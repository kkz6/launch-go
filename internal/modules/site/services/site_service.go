package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hibiken/asynq"

	databasedto "github.com/kkz6/launch-go/internal/modules/database/dto"
	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	gitcontracts "github.com/kkz6/launch-go/internal/modules/git/contracts"
	serverdto "github.com/kkz6/launch-go/internal/modules/server/dto"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/security"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

const (
	defaultDeploymentReleasesRetention = 5
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

// broadcastSiteEvent broadcasts a site event with standard fields
func (s *SiteService) broadcastSiteEvent(teamID, serverID string, site *models.Site, event string) {
	s.BroadcastToTeam(teamID, event, map[string]any{
		"team_id":   teamID,
		"server_id": serverID,
		"site_id":   site.ID,
		"site":      dto.ToSiteResponse(site),
	})
}

// broadcastSiteUpdate broadcasts a site.updated event, fetching the server via serverReader
func (s *SiteService) broadcastSiteUpdate(ctx context.Context, serverID string, site *models.Site) {
	if s.serverReader != nil {
		if server, err := s.serverReader.FindServerByID(ctx, serverID); err == nil {
			s.broadcastSiteEvent(server.TeamID, server.ID, site, "site.updated")
		}
	}
}

// List returns sites for a server as response DTOs. Signature matches
// IndexNestedFunc.
func (s *SiteService) List(ctx context.Context, serverID, teamID string) ([]dto.SiteResponse, error) {
	sites, err := s.listSites(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SiteResponse, len(sites))
	for i := range sites {
		out[i] = dto.ToSiteResponse(&sites[i])
	}
	return out, nil
}

func (s *SiteService) listSites(ctx context.Context, serverID, teamID string) ([]models.Site, error) {
	sites, err := s.Repos().Site().FindByServerAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	if len(sites) > 0 {
		siteIDs := make([]string, len(sites))
		for i := range sites {
			siteIDs[i] = sites[i].ID
		}
		deploymentMap, derr := s.Repos().Deployment().FindLatestBySiteIDs(ctx, siteIDs)
		if derr != nil {
			s.LogWarn("Failed to fetch latest deployments", "error", derr)
		} else {
			for i := range sites {
				sites[i].LatestDeployment = deploymentMap[sites[i].ID]
			}
		}
	}
	return sites, nil
}

// Create creates a new site under a server. Signature matches
// CreateNestedFunc: (ctx, parentID=serverID, teamID, userID, req).
func (s *SiteService) Create(ctx context.Context, serverID, teamID, userID string, req *dto.CreateSiteRequest) (dto.SiteResponse, error) {
	site, err := s.createSite(ctx, serverID, teamID, userID, req)
	if err != nil {
		return dto.SiteResponse{}, err
	}
	return dto.ToSiteResponse(site), nil
}

func (s *SiteService) createSite(ctx context.Context, serverID, teamID, userID string, req *dto.CreateSiteRequest) (*models.Site, error) {
	s.Logger.Debug().
		Str("server_id", serverID).
		Str("team_id", teamID).
		Str("address", req.Address).
		Str("type", string(req.Type)).
		Str("php_version", req.PhpVersion).
		Msg("creating site")

	// Get server to determine username and validate PHP version
	if s.serverReader == nil {
		s.Logger.Error().Msg("server reader not configured")
		return nil, errors.New("server reader not configured")
	}

	server, err := s.serverReader.FindServerByID(ctx, serverID)
	if err != nil {
		s.Logger.Error().Err(err).Str("server_id", serverID).Msg("failed to fetch server")
		return nil, fmt.Errorf("failed to fetch server: %w", err)
	}

	username := server.GetUsername()

	if err := validateActiveServerPHP(server, req.PhpVersion); err != nil {
		return nil, err
	}

	// Check if site with same address exists
	existing, err := s.Repos().Site().FindByAddress(ctx, req.Address, serverID)
	if err != nil && !fiberutil.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check for existing site: %w", err)
	}
	if existing != nil {
		return nil, fiberutil.NewValidationError(map[string][]string{
			"address": {"A site with this address already exists on this server"},
		})
	}

	// Validate site type
	if !req.Type.IsValid() {
		return nil, fiberutil.NewValidationError(map[string][]string{
			"type": {fmt.Sprintf("Invalid site type: %s", req.Type)},
		})
	}

	// Validate source control for non-WordPress/phpMyAdmin sites
	if req.Type != sitetypes.SiteTypeWordpress && req.Type != sitetypes.SiteTypePhpMyAdmin && req.SourceControlID != nil && *req.SourceControlID != "" {
		if err := s.validateSourceControl(ctx, teamID, req.SourceControlID, req.SourceControlRepositoriesID); err != nil {
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
	var phpVersion *sitetypes.PhpVersion
	if req.PhpVersion != "" {
		pv := sitetypes.PhpVersion(req.PhpVersion)
		phpVersion = &pv
	}

	var repoBranch *string
	if req.RepositoryBranch != "" {
		repoBranch = &req.RepositoryBranch
	}

	// WordPress and phpMyAdmin sites don't support zero-downtime deployment
	zeroDowntime := req.ZeroDowntimeDeployment
	if req.Type == sitetypes.SiteTypeWordpress || req.Type == sitetypes.SiteTypePhpMyAdmin {
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
		TLSSetting:                  sitetypes.TLSSettingAuto,
		ZeroDowntimeDeployment:      zeroDowntime,
		DeploymentReleasesRetention: defaultDeploymentReleasesRetention,
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

	// Create site with activity logging in a transaction
	var envVars map[string]string
	s.Logger.Debug().
		Str("site_address", site.Address).
		Str("site_server_id", site.ServerID).
		Str("site_team_id", site.TeamID).
		Str("site_user_id", site.UserID).
		Str("site_type", string(site.Type)).
		Str("site_path", site.Path).
		Str("site_user", site.User).
		Msg("about to create site record")

	err = s.Repos().Site().CreateWithActivity(ctx, site, userID)

	if err != nil {
		s.Logger.Error().Err(err).
			Str("address", req.Address).
			Str("server_id", serverID).
			Str("team_id", teamID).
			Str("type", string(req.Type)).
			Msg("failed to create site")
		return nil, err
	}

	// Save repository to source control provider (non-WordPress/phpMyAdmin sites only)
	if req.Type != sitetypes.SiteTypeWordpress && req.Type != sitetypes.SiteTypePhpMyAdmin && req.SourceControlID != nil && *req.SourceControlID != "" && sourceControlRepoID != nil {
		s.handleSourceControlRepository(ctx, teamID, *req.SourceControlID, *sourceControlRepoID)
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
		if req.Type == sitetypes.SiteTypeLaravel || req.Type == sitetypes.SiteTypeWordpress {
			s.handleSchedulerCreation(ctx, site, serverID, userID)
		}
	}

	// Handle queue worker creation for Laravel sites
	if req.CreateQueue && req.Type == sitetypes.SiteTypeLaravel {
		s.handleQueueCreation(ctx, site, serverID, userID)
	}

	// Create initial deployment with git commit data
	if s.Services() != nil {
		// Fetch git commit data if source control is configured
		commitData := s.Services().Deployment().FetchLatestCommitData(ctx, site)

		// Pass database/queue env vars through the job payload (not persisted in DB)
		deployment, err := s.Services().Deployment().createDeployment(ctx, site, userID, commitData, envVars)
		if err != nil {
			s.LogError(err, "Failed to create initial deployment", "site_id", site.ID)
		} else {
			site.LatestDeployment = deployment
		}
	}

	s.LogInfo("Site created", "site_id", site.ID, "address", site.Address)

	s.broadcastSiteEvent(server.TeamID, server.ID, site, "site.created")

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

		// Fetch database user credentials
		if req.DatabaseUserID != nil && s.databaseManager != nil {
			dbUser, err := s.databaseManager.GetDatabaseUser(ctx, *req.DatabaseUserID, serverID)
			if err != nil {
				s.LogError(err, "Failed to fetch database user for existing database", "user_id", *req.DatabaseUserID)
			} else {
				if envVarNames["username"] != "" {
					envVars[envVarNames["username"]] = dbUser.Name
				}
				if envVarNames["password"] != "" && dbUser.Password != nil && dbUser.Password.Valid {
					envVars[envVarNames["password"]] = dbUser.Password.String
				}
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

	// Add user credentials
	if req.DatabaseUserOption == "new" && req.DatabaseUserName != nil && req.DatabaseUserPassword != nil {
		if envVarNames["username"] != "" {
			envVars[envVarNames["username"]] = *req.DatabaseUserName
		}
		if envVarNames["password"] != "" {
			envVars[envVarNames["password"]] = *req.DatabaseUserPassword
		}
	} else if req.DatabaseUserOption == "existing" && req.DatabaseUserID != nil {
		dbUser, err := s.databaseManager.GetDatabaseUser(ctx, *req.DatabaseUserID, serverID)
		if err != nil {
			s.LogError(err, "Failed to fetch existing database user", "user_id", *req.DatabaseUserID)
		} else {
			if envVarNames["username"] != "" {
				envVars[envVarNames["username"]] = dbUser.Name
			}
			if envVarNames["password"] != "" && dbUser.Password != nil && dbUser.Password.Valid {
				envVars[envVarNames["password"]] = dbUser.Password.String
			}
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
func (s *SiteService) getDatabaseTypeForServer(ctx context.Context, serverID string) servertypes.Software {
	// Query installed services to find the database type
	if s.serverReader == nil {
		return servertypes.SoftwareMySQL80 // Default to MySQL if we can't determine
	}

	services, err := s.serverReader.FindServicesByServer(ctx, serverID)
	if err != nil {
		return servertypes.SoftwareMySQL80 // Default to MySQL if we can't determine
	}

	for _, service := range services {
		if service.Type == servertypes.ServiceTypeMySQL {
			return servertypes.SoftwareMySQL80
		}
		if service.Type == servertypes.ServiceTypePostgreSQL {
			return servertypes.SoftwarePostgreSQL16
		}
	}

	return servertypes.SoftwareMySQL80 // Default to MySQL
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
	case sitetypes.SiteTypeWordpress:
		// WordPress cron uses wp-cron.php
		command = fmt.Sprintf("cd %s && %s wp-cron.php >> /dev/null 2>&1", site.GetWebDirectory(), phpBinary)
	case sitetypes.SiteTypeLaravel:
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
	if err := s.Repos().Site().UpdateFields(ctx, site.ID, map[string]any{
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
	queue, err := s.Services().Queue().Create(ctx, site.ID, serverID, site.TeamID, userID, queueReq)
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
	if err := s.Repos().Site().UpdateFields(ctx, site.ID, map[string]any{
		"enabled_features": site.EnabledFeatures,
	}); err != nil {
		s.LogError(err, "Failed to update enabled_features for queue", "site_id", site.ID)
	}

	s.LogInfo("Queue worker created for site", "site_id", site.ID)
}

// validateSourceControl validates source control and repository exist
func (s *SiteService) validateSourceControl(ctx context.Context, teamID string, sourceControlID *string, repoID *string) error {
	if sourceControlID == nil || *sourceControlID == "" {
		return nil
	}

	// Check source control exists
	exists, err := s.Repos().Site().SourceControlExists(ctx, *sourceControlID, teamID)
	if err != nil {
		return fmt.Errorf("failed to validate source control: %w", err)
	}
	if !exists {
		return fiberutil.NewValidationError(map[string][]string{
			"source_control_id": {"The selected source control does not exist"},
		})
	}

	// Check repository exists if provided
	if repoID != nil && *repoID != "" {
		exists, err := s.Repos().Site().SourceControlRepositoryExists(ctx, *repoID, teamID)
		if err != nil {
			return fmt.Errorf("failed to validate repository: %w", err)
		}
		if !exists {
			return fiberutil.NewValidationError(map[string][]string{
				"source_control_repositories_id": {"The selected repository does not exist"},
			})
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
func (s *SiteService) handleSourceControlRepository(ctx context.Context, teamID, sourceControlID string, repoID uint64) {
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
	_, err = s.sourceControlService.SaveRepository(ctx, sourceControlID, teamID, repo.FullName)
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
	if err != nil && !fiberutil.IsNotFound(err) {
		s.LogWarn("Failed to fetch latest deployment", "siteID", site.ID, "error", err)
	}
	site.LatestDeployment = deployment

	return site, nil
}

// GetQueueCount returns the number of queue workers for a site
func (s *SiteService) GetQueueCount(ctx context.Context, siteID string) int {
	count, err := s.Repos().Queue().CountBySite(ctx, siteID)
	if err != nil {
		return 0
	}
	return int(count)
}

// Update updates a site
// Update updates a site. Signature matches UpdateNestedFunc:
// (ctx, id, parentID=serverID, teamID, userID, req).
func (s *SiteService) Update(ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateSiteRequest) (dto.SiteResponse, error) {
	site, err := s.updateSite(ctx, id, serverID, teamID, userID, req)
	if err != nil {
		return dto.SiteResponse{}, err
	}
	return dto.ToSiteResponse(site), nil
}

func (s *SiteService) updateSite(ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateSiteRequest) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Build updates map for changed fields
	updates := make(map[string]any)
	updateCaddyfile := false
	phpVersionChanged := req.PhpVersion != nil &&
		(site.PhpVersion == nil || *req.PhpVersion != site.PhpVersion.String())
	previousPHPVersionWasNull := site.PhpVersion == nil
	previousPHPVersion := ""
	if site.PhpVersion != nil {
		previousPHPVersion = site.PhpVersion.String()
	}

	if phpVersionChanged {
		if site.PendingCaddyfileUpdateSince != nil || site.PendingTLSUpdateSince != nil {
			return nil, fiberutil.Conflict("A site configuration update is already in progress")
		}
		if s.serverReader == nil {
			return nil, errors.New("server reader not configured")
		}
		server, findErr := s.serverReader.FindServerByID(ctx, serverID)
		if findErr != nil {
			return nil, fmt.Errorf("failed to fetch server: %w", findErr)
		}
		if err := validateActiveServerPHP(server, *req.PhpVersion); err != nil {
			return nil, err
		}
		if previousPHPVersionWasNull {
			previousPHPVersion, err = resolveServerPHPVersion(server)
			if err != nil {
				return nil, err
			}
		}
		if !s.HasQueue() {
			return nil, pkgservice.ErrQueueRequired
		}
	}

	// Simple pointer fields
	addIfSet(updates, "web_folder", req.WebFolder)
	addIfSet(updates, "deployment_releases_retention", req.DeploymentReleasesRetention)
	addIfSet(updates, "queue_deployments", req.QueueDeployments)

	// Conditional field (not for WordPress)
	if req.RepositoryBranch != nil && site.Type != sitetypes.SiteTypeWordpress && site.Type != sitetypes.SiteTypePhpMyAdmin {
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

	// PHP changes use a dedicated lifecycle job that applies Caddy, queue,
	// scheduler, and daemon configuration before committing the database
	// version. A web-folder-only change can keep using the lighter Caddy job.
	if req.WebFolder != nil && *req.WebFolder != site.WebFolder {
		updateCaddyfile = true
	}
	if phpVersionChanged {
		updateCaddyfile = false
	}

	siteFieldsUpdated := len(updates) > 0

	if phpVersionChanged {
		if s.ServiceDeps().DB == nil {
			return nil, errors.New("database not configured")
		}

		userIDPtr := userID
		task, taskErr := jobs.NewUpdateSitePHPVersionTask(
			site.ID,
			previousPHPVersion,
			*req.PhpVersion,
			previousPHPVersionWasNull,
			&userIDPtr,
		)
		if taskErr != nil {
			return nil, taskErr
		}

		now, reserveErr := s.reserveSiteConfigurationUpdate(
			ctx,
			site,
			updates,
			*req.PhpVersion,
			previousPHPVersionWasNull,
		)
		if reserveErr != nil {
			return nil, reserveErr
		}
		if enqueueErr := s.enqueueTaskStrict(task); enqueueErr != nil {
			s.rollbackSiteConfigurationUpdate(ctx, site, updates, *req.PhpVersion)
			return nil, enqueueErr
		}

		site.PendingCaddyfileUpdateSince = &now
		pendingPhpVersion := sitetypes.PhpVersion(*req.PhpVersion)
		site.PendingPhpVersion = &pendingPhpVersion
		activity.RecordEventWithProps(
			ctx,
			"php_version_update_requested",
			userID,
			site,
			fmt.Sprintf("PHP update to %s was requested for %s", *req.PhpVersion, site.Address),
			map[string]any{
				"previous_version": previousPHPVersion,
				"version":          *req.PhpVersion,
			},
		)
		s.BroadcastToTeam(teamID, "site.php_version_update_requested", map[string]any{
			"team_id":          teamID,
			"server_id":        serverID,
			"site_id":          site.ID,
			"address":          site.Address,
			"previous_version": previousPHPVersion,
			"php_version":      *req.PhpVersion,
		})
	} else if updateCaddyfile {
		if !s.HasQueue() {
			return nil, pkgservice.ErrQueueRequired
		}
		userIDPtr := userID
		task, taskErr := jobs.NewReservedUpdateCaddyfileTask(site.ID, &userIDPtr)
		if taskErr != nil {
			s.LogError(taskErr, "Failed to create Caddyfile update task", "site_id", site.ID)
			return nil, taskErr
		}

		now, reserveErr := s.reserveSiteConfigurationUpdate(ctx, site, updates, "", false)
		if reserveErr != nil {
			return nil, reserveErr
		}
		if enqueueErr := s.enqueueTaskStrict(task); enqueueErr != nil {
			s.rollbackSiteConfigurationUpdate(ctx, site, updates, "")
			s.LogError(enqueueErr, "Failed to enqueue Caddyfile update task", "site_id", site.ID)
			return nil, enqueueErr
		}
		site.PendingCaddyfileUpdateSince = &now
	} else if len(updates) > 0 {
		if err := s.Repos().Site().UpdateFields(ctx, site.ID, updates); err != nil {
			return nil, err
		}
	}

	// Reload the committed values, including pending operation state.
	site, err = s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	if siteFieldsUpdated {
		activity.RecordUpdated(ctx, userID, site, "Site was updated")
	}

	// Load latest deployment
	deployment, err := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	if err != nil && !fiberutil.IsNotFound(err) {
		s.LogWarn("Failed to fetch latest deployment", "siteID", site.ID, "error", err)
	}
	site.LatestDeployment = deployment

	s.LogInfo("Site updated", "site_id", site.ID)

	s.broadcastSiteUpdate(ctx, serverID, site)

	return site, nil
}

func validateActiveServerPHP(server *servermodels.Server, phpVersion string) error {
	software, err := servertypes.ParseSoftware(phpVersion)
	if err != nil || !software.IsPhp() {
		return fiberutil.NewValidationError(map[string][]string{
			"php_version": {fmt.Sprintf("Invalid PHP version: %s", phpVersion)},
		})
	}

	for i := range server.Services {
		service := &server.Services[i]
		if service.Type != servertypes.ServiceTypePhp ||
			service.Software != software.String() {
			continue
		}
		if !service.Status.IsActive() {
			return fiberutil.NewValidationError(map[string][]string{
				"php_version": {
					fmt.Sprintf(
						"PHP %s is installed but not active (status: %s)",
						software.GetVersion(),
						service.Status,
					),
				},
			})
		}
		return nil
	}

	return fiberutil.NewValidationError(map[string][]string{
		"php_version": {
			fmt.Sprintf("PHP %s is not installed on this server", software.GetVersion()),
		},
	})
}

func resolveServerPHPVersion(server *servermodels.Server) (string, error) {
	var fallback string
	for i := range server.Services {
		service := &server.Services[i]
		if service.Type != servertypes.ServiceTypePhp || !service.Status.IsActive() {
			continue
		}
		software := service.GetSoftware()
		if !software.IsPhp() {
			continue
		}
		if service.IsDefault {
			return software.String(), nil
		}
		if fallback == "" {
			fallback = software.String()
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fiberutil.BadRequest(
		"The site's current PHP version is unknown and the server has no active PHP default",
	)
}

func (s *SiteService) reserveSiteConfigurationUpdate(
	ctx context.Context,
	site *models.Site,
	updates map[string]any,
	pendingPHPVersion string,
	previousPHPVersionWasNull bool,
) (time.Time, error) {
	if s.ServiceDeps().DB == nil {
		return time.Time{}, errors.New("database not configured")
	}

	now := time.Now().UTC()
	claim := make(map[string]any, len(updates)+2)
	for field, value := range updates {
		claim[field] = value
	}
	claim["pending_caddyfile_update_since"] = now
	if pendingPHPVersion != "" {
		claim["pending_php_version"] = pendingPHPVersion
	}

	query := s.ServiceDeps().DB.WithContext(ctx).
		Model(&models.Site{}).
		Where(
			"id = ? AND pending_caddyfile_update_since IS NULL AND pending_php_version IS NULL AND pending_tls_update_since IS NULL",
			site.ID,
		)
	if pendingPHPVersion != "" {
		if previousPHPVersionWasNull {
			query = query.Where("php_version IS NULL")
		} else {
			query = query.Where("php_version = ?", site.PhpVersion.String())
		}
	}

	result := query.Updates(claim)
	if result.Error != nil {
		return time.Time{}, result.Error
	}
	if result.RowsAffected != 1 {
		return time.Time{}, fiberutil.Conflict("A site configuration update is already in progress")
	}
	return now, nil
}

func (s *SiteService) rollbackSiteConfigurationUpdate(
	ctx context.Context,
	site *models.Site,
	updates map[string]any,
	pendingPHPVersion string,
) {
	if s.ServiceDeps().DB == nil {
		return
	}
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()

	rollback := originalSiteUpdateFields(site, updates)
	rollback["pending_caddyfile_update_since"] = nil
	if pendingPHPVersion != "" {
		rollback["pending_php_version"] = nil
	}

	query := s.ServiceDeps().DB.WithContext(cleanupCtx).
		Model(&models.Site{}).
		Where("id = ?", site.ID)
	if pendingPHPVersion == "" {
		query = query.Where("pending_php_version IS NULL")
	} else {
		query = query.Where("pending_php_version = ?", pendingPHPVersion)
	}
	result := query.Updates(rollback)
	if result.Error != nil {
		s.LogError(
			result.Error,
			"Failed to roll back site configuration reservation",
			"site_id",
			site.ID,
		)
		return
	}
	if result.RowsAffected != 1 {
		s.LogError(
			errors.New("site configuration reservation could not be released"),
			"Failed to roll back site configuration reservation",
			"site_id",
			site.ID,
		)
	}
}

func (s *SiteService) enqueueTaskStrict(task *asynq.Task) error {
	if !s.HasQueue() {
		return pkgservice.ErrQueueRequired
	}
	_, err := s.Queue.EnqueueDefault(task)
	if errors.Is(err, asynq.ErrDuplicateTask) || errors.Is(err, asynq.ErrTaskIDConflict) {
		return fiberutil.Conflict("This site configuration update is already queued")
	}
	return err
}

func originalSiteUpdateFields(site *models.Site, updates map[string]any) map[string]any {
	original := make(map[string]any, len(updates))
	for field := range updates {
		switch field {
		case "web_folder":
			original[field] = site.WebFolder
		case "deployment_releases_retention":
			original[field] = site.DeploymentReleasesRetention
		case "queue_deployments":
			original[field] = site.QueueDeployments
		case "repository_branch":
			original[field] = site.RepositoryBranch
		case "hook_before_updating_repository":
			original[field] = site.HookBeforeUpdatingRepository
		case "hook_after_updating_repository":
			original[field] = site.HookAfterUpdatingRepository
		case "hook_before_making_current":
			original[field] = site.HookBeforeMakingCurrent
		case "hook_after_making_current":
			original[field] = site.HookAfterMakingCurrent
		case "shared_directories":
			original[field] = site.SharedDirectories
		case "shared_files":
			original[field] = site.SharedFiles
		case "writeable_directories":
			original[field] = site.WriteableDirectories
		}
	}
	return original
}

// Delete deletes a site
func (s *SiteService) Delete(ctx context.Context, id, serverID, teamID string) error {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "deleted", "", site, "Site deletion requested")

	now := time.Now()
	site.UninstallationRequestedAt = &now

	if err := s.Repos().Site().Update(ctx, site); err != nil {
		return err
	}

	s.broadcastSiteUpdate(ctx, serverID, site)

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
// GetDeletionSummary returns a summary of resources to be deleted.
// Signature matches ShowNestedFunc.
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

	var cronCount int64
	if s.cronCreator != nil {
		cronCount, err = s.cronCreator.CountCronsBySite(ctx, site.ID)
		if err != nil {
			s.LogWarn("Failed to count crons for deletion summary", "siteID", site.ID, "error", err)
			cronCount = 0
		}
	}

	return &dto.DeletionSummaryResponse{
		Queues: int(queueCount),
		Crons:  int(cronCount),
	}, nil
}

// RegenerateDeployToken regenerates the deploy token for a site
func (s *SiteService) RegenerateDeployToken(ctx context.Context, id, serverID, teamID string) (*models.Site, error) {
	site, err := s.Repos().Site().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return nil, err
	}

	deployToken := security.NewTokenGenerator(32).WithEncoding(security.TokenBase64URL).MustGenerate()
	site.DeployToken = &deployToken

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
	if err != nil && !fiberutil.IsNotFound(err) {
		s.LogWarn("Failed to fetch latest deployment for settings", "siteID", site.ID, "error", err)
	}
	site.LatestDeployment = deployment

	// Get active certificate
	activeCert, err := s.Repos().Certificate().FindActiveBySite(ctx, site.ID)
	if err != nil && !fiberutil.IsNotFound(err) {
		s.LogWarn("Failed to fetch active certificate", "siteID", site.ID, "error", err)
	}

	// Get PHP versions from server services
	phpVersions := s.getServerPhpVersions(ctx, serverID)

	// Get source control info if linked
	var sourceControl *dto.SourceControlResponse
	var repository *dto.SourceControlRepositoryResponse
	if site.SourceControlID != nil && *site.SourceControlID != "" {
		sourceControl, repository = s.GetSourceControlInfo(ctx, teamID, *site.SourceControlID, site.SourceControlRepositoriesID)
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
		if svc.Type == servertypes.ServiceTypePhp {
			result = append(result, dto.PhpVersionResponse{
				Version:   svc.Version,
				IsDefault: svc.IsDefault,
			})
		}
	}

	return result
}

// GetSourceControlInfo returns source control and repository info
func (s *SiteService) GetSourceControlInfo(ctx context.Context, teamID, sourceControlID string, repoID *uint64) (*dto.SourceControlResponse, *dto.SourceControlRepositoryResponse) {
	if s.gitReader == nil {
		return nil, nil
	}

	sc, err := s.gitReader.FindSourceControlByID(ctx, sourceControlID, teamID)
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
