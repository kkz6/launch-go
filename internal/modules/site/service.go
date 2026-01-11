package site

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

var (
	ErrPendingDeployment         = errors.New("a deployment is already in progress")
	ErrRollbackNotSupported      = errors.New("rollback is only available for sites with zero downtime deployment enabled")
	ErrInvalidRollbackTarget     = errors.New("can only rollback to a finished deployment")
	ErrDeploymentNotBelongToSite = errors.New("target deployment does not belong to this site")
	ErrSourceControlNotConnected = errors.New("source control is not connected")
	ErrSiteNotInstalled          = errors.New("site is not installed")
)

// Service handles business logic for sites
type Service struct {
	repo   *Repository
	queue  *queue.Client
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewService creates a new site service
func NewService(repo *Repository, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		queue:  queueClient,
		ws:     ws,
		logger: logger,
	}
}

// List returns all sites for a server
func (s *Service) List(ctx context.Context, serverID string) ([]Site, error) {
	return s.repo.FindByServerWithLatestDeployment(ctx, serverID)
}

// Create creates a new site
func (s *Service) Create(ctx context.Context, serverID, userID, username string, req *CreateSiteRequest) (*Site, error) {
	// Check if site with same address exists
	existing, _ := s.repo.FindByAddress(ctx, req.Address, serverID)
	if existing != nil {
		return nil, errors.New("a site with this address already exists on this server")
	}

	siteType, err := ParseSiteType(req.Type)
	if err != nil {
		return nil, err
	}

	// Build path
	path := fmt.Sprintf("/home/%s/%s", username, req.Address)

	// Set default web folder
	webFolder := req.WebFolder
	if webFolder == "" {
		if siteType == SiteTypeWordpress {
			webFolder = "/"
		} else {
			webFolder = "public"
		}
	}

	site := &Site{
		ServerID:                    serverID,
		UserID:                      userID,
		Address:                     req.Address,
		Type:                        siteType,
		TlsSetting:                  TlsSettingAuto,
		ZeroDowntimeDeployment:      req.ZeroDowntimeDeployment,
		DeploymentReleasesRetention: 5,
		RepositoryBranch:            req.RepositoryBranch,
		User:                        username,
		Path:                        path,
		WebFolder:                   webFolder,
		PhpVersion:                  req.PhpVersion,
		SourceControlID:             req.SourceControlID,
		SourceControlRepositoriesID: req.SourceControlRepositoriesID,
		ConnectedDomainID:           req.ConnectedDomainID,
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
	if hook, ok := defaults["hook_before_updating_repository"].(string); ok {
		site.HookBeforeUpdatingRepository = hook
	}
	if hook, ok := defaults["hook_after_updating_repository"].(string); ok {
		site.HookAfterUpdatingRepository = hook
	}
	if hook, ok := defaults["hook_before_making_current"].(string); ok {
		site.HookBeforeMakingCurrent = hook
	}
	if hook, ok := defaults["hook_after_making_current"].(string); ok {
		site.HookAfterMakingCurrent = hook
	}

	if err := s.repo.Create(ctx, site); err != nil {
		return nil, err
	}

	// Create initial deployment
	deployment, err := s.createDeployment(ctx, site, userID, nil)
	if err != nil {
		s.logger.Error().Err(err).Str("site_id", site.ID).Msg("Failed to create initial deployment")
	} else {
		site.LatestDeployment = deployment
	}

	s.logger.Info().
		Str("site_id", site.ID).
		Str("address", site.Address).
		Msg("Site created")

	return site, nil
}

// FindByID finds a site by ID
func (s *Service) FindByID(ctx context.Context, id, serverID string) (*Site, error) {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Load latest deployment
	deployment, _ := s.repo.FindLatestDeploymentBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	return site, nil
}

// Update updates a site
func (s *Service) Update(ctx context.Context, id, serverID, userID string, req *UpdateSiteRequest) (*Site, error) {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	// Track if we need to update Caddyfile
	updateCaddyfile := false
	oldPhpVersion := site.PhpVersion
	oldWebFolder := site.WebFolder

	// Update fields
	if req.PhpVersion != nil {
		site.PhpVersion = *req.PhpVersion
		if *req.PhpVersion != oldPhpVersion {
			updateCaddyfile = true
		}
	}
	if req.WebFolder != nil {
		site.WebFolder = *req.WebFolder
		if *req.WebFolder != oldWebFolder {
			updateCaddyfile = true
		}
	}
	if req.RepositoryBranch != nil && site.Type != SiteTypeWordpress {
		site.RepositoryBranch = *req.RepositoryBranch
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
		site.HookBeforeUpdatingRepository = normalizeLineEndings(*req.HookBeforeUpdatingRepository)
	}
	if req.HookAfterUpdatingRepository != nil {
		site.HookAfterUpdatingRepository = normalizeLineEndings(*req.HookAfterUpdatingRepository)
	}
	if req.HookBeforeMakingCurrent != nil {
		site.HookBeforeMakingCurrent = normalizeLineEndings(*req.HookBeforeMakingCurrent)
	}
	if req.HookAfterMakingCurrent != nil {
		site.HookAfterMakingCurrent = normalizeLineEndings(*req.HookAfterMakingCurrent)
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

	if err := s.repo.Update(ctx, site); err != nil {
		return nil, err
	}

	// If PHP version or web folder changed, update Caddyfile and deploy
	if updateCaddyfile {
		now := time.Now()
		site.PendingCaddyfileUpdateSince = &now
		s.repo.Update(ctx, site)
		// Dispatch Caddyfile update job
		// TODO: Add UpdateCaddyfile job dispatch
	}

	// Load latest deployment
	deployment, _ := s.repo.FindLatestDeploymentBySite(ctx, site.ID)
	site.LatestDeployment = deployment

	s.logger.Info().
		Str("site_id", site.ID).
		Msg("Site updated")

	return site, nil
}

// Delete deletes a site
func (s *Service) Delete(ctx context.Context, id, serverID string) error {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	now := time.Now()
	site.UninstallationRequestedAt = &now

	if err := s.repo.Update(ctx, site); err != nil {
		return err
	}

	// TODO: Dispatch site deletion job

	s.logger.Info().
		Str("site_id", site.ID).
		Str("address", site.Address).
		Msg("Site deletion requested")

	return nil
}

// Deploy triggers a new deployment for a site
func (s *Service) Deploy(ctx context.Context, id, serverID, userID string) (*Deployment, error) {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	return s.createDeployment(ctx, site, userID, nil)
}

// Rollback rolls back to a previous deployment
func (s *Service) Rollback(ctx context.Context, siteID, serverID, targetDeploymentID, userID string) (*Deployment, error) {
	site, err := s.repo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	if !site.ZeroDowntimeDeployment {
		return nil, ErrRollbackNotSupported
	}

	targetDeployment, err := s.repo.FindDeploymentByID(ctx, targetDeploymentID)
	if err != nil {
		return nil, err
	}

	if targetDeployment.SiteID != site.ID {
		return nil, ErrDeploymentNotBelongToSite
	}

	if targetDeployment.Status != DeploymentStatusFinished {
		return nil, ErrInvalidRollbackTarget
	}

	// Check for active deployment
	activeDeployment, _ := s.repo.FindActiveDeploymentBySite(ctx, site.ID)
	if activeDeployment != nil {
		return nil, ErrPendingDeployment
	}

	// Get latest deployment for rollback metadata
	latestDeployment, _ := s.repo.FindLatestDeploymentBySite(ctx, site.ID)

	// Create rollback deployment
	commitData := map[string]interface{}{
		"rollback_from": latestDeployment.ID,
		"rollback_to":   targetDeployment.ID,
	}
	if targetDeployment.CommitData != "" {
		existingData := targetDeployment.GetCommitData()
		for k, v := range existingData {
			if k != "rollback_from" && k != "rollback_to" {
				commitData[k] = v
			}
		}
	}

	deployment := &Deployment{
		SiteID:  site.ID,
		UserID:  &userID,
		Status:  DeploymentStatusPending,
		GitHash: targetDeployment.GitHash,
	}
	if err := deployment.SetCommitData(commitData); err != nil {
		return nil, err
	}

	if err := s.repo.CreateDeployment(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch rollback job
	task, err := jobs.NewRollbackTask(site.ID, deployment.ID, targetDeploymentID, userID)
	if err != nil {
		return nil, err
	}

	if s.queue != nil {
		if _, err := s.queue.EnqueueDefault(task); err != nil {
			s.logger.Error().Err(err).Str("deployment_id", deployment.ID).Msg("Failed to enqueue rollback job")
		}
	}

	s.logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Str("target_deployment_id", targetDeploymentID).
		Msg("Rollback initiated")

	return deployment, nil
}

// createDeployment creates a new deployment for a site
func (s *Service) createDeployment(ctx context.Context, site *Site, userID string, commitData map[string]interface{}) (*Deployment, error) {
	// Check for active deployment
	activeDeployment, _ := s.repo.FindActiveDeploymentBySite(ctx, site.ID)
	if activeDeployment != nil {
		if site.QueueDeployments {
			// Queue the deployment
			deployment := &Deployment{
				SiteID: site.ID,
				UserID: &userID,
				Status: DeploymentStatusQueued,
			}
			if commitData != nil {
				if err := deployment.SetCommitData(commitData); err != nil {
					return nil, err
				}
			}

			if err := s.repo.CreateDeployment(ctx, deployment); err != nil {
				return nil, err
			}

			s.logger.Info().
				Str("site_id", site.ID).
				Str("deployment_id", deployment.ID).
				Msg("Deployment queued")

			return deployment, nil
		}
		return nil, ErrPendingDeployment
	}

	deployment := &Deployment{
		SiteID: site.ID,
		UserID: &userID,
		Status: DeploymentStatusPending,
	}
	if commitData != nil {
		if err := deployment.SetCommitData(commitData); err != nil {
			return nil, err
		}
	}

	if err := s.repo.CreateDeployment(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch deployment job
	var task *asynq.Task
	var dispatchErr error
	if site.ZeroDowntimeDeployment {
		task, dispatchErr = jobs.NewDeployZeroDowntimeTask(site.ID, deployment.ID, userID)
	} else {
		task, dispatchErr = jobs.NewDeployTask(site.ID, deployment.ID, userID)
	}

	if dispatchErr != nil {
		return nil, dispatchErr
	}

	if s.queue != nil {
		if _, err := s.queue.EnqueueDefault(task); err != nil {
			s.logger.Error().Err(err).Str("deployment_id", deployment.ID).Msg("Failed to enqueue deployment job")
		}
	}

	s.logger.Info().
		Str("site_id", site.ID).
		Str("deployment_id", deployment.ID).
		Msg("Deployment started")

	return deployment, nil
}

// ListDeployments returns all deployments for a site
func (s *Service) ListDeployments(ctx context.Context, siteID, serverID string) ([]Deployment, error) {
	// Verify site exists and belongs to server
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.repo.FindDeploymentsBySite(ctx, siteID)
}

// FindDeployment finds a deployment by ID
func (s *Service) FindDeployment(ctx context.Context, id, siteID, serverID string) (*Deployment, error) {
	// Verify site exists and belongs to server
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.repo.FindDeploymentByIDAndSite(ctx, id, siteID)
}

// ProcessNextQueuedDeployment processes the next queued deployment
func (s *Service) ProcessNextQueuedDeployment(ctx context.Context, siteID string) (*Deployment, error) {
	site, err := s.repo.FindByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	// Check for active deployment
	activeDeployment, _ := s.repo.FindActiveDeploymentBySite(ctx, site.ID)
	if activeDeployment != nil {
		return nil, nil
	}

	// Get next queued deployment
	queuedDeployments, err := s.repo.FindQueuedDeploymentsBySite(ctx, site.ID)
	if err != nil {
		return nil, err
	}

	if len(queuedDeployments) == 0 {
		return nil, nil
	}

	deployment := &queuedDeployments[0]
	deployment.Status = DeploymentStatusPending

	if err := s.repo.UpdateDeployment(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch deployment job
	var task *asynq.Task
	var dispatchErr error
	if site.ZeroDowntimeDeployment {
		task, dispatchErr = jobs.NewDeployZeroDowntimeTask(site.ID, deployment.ID, "")
	} else {
		task, dispatchErr = jobs.NewDeployTask(site.ID, deployment.ID, "")
	}

	if dispatchErr != nil {
		return nil, dispatchErr
	}

	if s.queue != nil {
		if _, err := s.queue.EnqueueDefault(task); err != nil {
			s.logger.Error().Err(err).Str("deployment_id", deployment.ID).Msg("Failed to enqueue deployment job")
		}
	}

	return deployment, nil
}

// GetQueuedDeploymentsCount returns the count of queued deployments
func (s *Service) GetQueuedDeploymentsCount(ctx context.Context, siteID string) (int64, error) {
	return s.repo.CountQueuedDeploymentsBySite(ctx, siteID)
}

// CancelQueuedDeployments cancels all queued deployments
func (s *Service) CancelQueuedDeployments(ctx context.Context, siteID, serverID string) (int64, error) {
	// Verify site exists and belongs to server
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return 0, err
	}

	return s.repo.CancelQueuedDeployments(ctx, siteID)
}

// EnableAutoDeployment enables auto-deployment for a site
func (s *Service) EnableAutoDeployment(ctx context.Context, id, serverID string) error {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return ErrSourceControlNotConnected
	}

	site.AutoDeployment = true
	return s.repo.Update(ctx, site)
}

// DisableAutoDeployment disables auto-deployment for a site
func (s *Service) DisableAutoDeployment(ctx context.Context, id, serverID string) error {
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{
		"auto_deployment": false,
	})
}

// EnableAutoRestartQueue enables auto-restart for queue workers
func (s *Service) EnableAutoRestartQueue(ctx context.Context, id, serverID string) error {
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{
		"auto_restart_queue": true,
	})
}

// DisableAutoRestartQueue disables auto-restart for queue workers
func (s *Service) DisableAutoRestartQueue(ctx context.Context, id, serverID string) error {
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{
		"auto_restart_queue": false,
	})
}

// RegenerateDeployToken regenerates the deploy token for a site
func (s *Service) RegenerateDeployToken(ctx context.Context, id, serverID string) error {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	site.DeployToken = generateRandomToken(32)
	return s.repo.Update(ctx, site)
}

// GetDeletionSummary returns a summary of resources that will be deleted
func (s *Service) GetDeletionSummary(ctx context.Context, id, serverID string) (*DeletionSummaryResponse, error) {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	queueCount, _ := s.repo.CountQueuesBySite(ctx, site.ID)
	// TODO: Add cron count

	return &DeletionSummaryResponse{
		Queues: int(queueCount),
		Crons:  0,
	}, nil
}

// SSL operations

// UpdateSSL updates SSL settings for a site
func (s *Service) UpdateSSL(ctx context.Context, id, serverID, userID string, req *UpdateSSLRequest) error {
	site, err := s.repo.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	tlsSetting := TlsSetting(req.TlsSetting)
	if !tlsSetting.IsValid() {
		return errors.New("invalid TLS setting")
	}

	// Handle custom certificate
	if tlsSetting == TlsSettingCustom && req.PrivateKey != nil && req.Certificate != nil {
		// Deactivate existing certificates
		if err := s.repo.DeactivateAllCertificates(ctx, site.ID); err != nil {
			return err
		}

		// Create new certificate
		cert := &Certificate{
			SiteID:      site.ID,
			Type:        CertificateTypeCustom,
			PrivateKey:  req.PrivateKey,
			Certificate: req.Certificate,
			IsActive:    true,
		}

		domains := []string{site.Address}
		domains = append(domains, site.GetAliases()...)
		if err := cert.SetDomains(domains); err != nil {
			return err
		}

		now := time.Now()
		cert.UploadedAt = &now

		if err := s.repo.CreateCertificate(ctx, cert); err != nil {
			return err
		}

		// Dispatch certificate installation job
		task, err := jobs.NewInstallSSLTask(site.ID, site.Address)
		if err == nil && s.queue != nil {
			s.queue.EnqueueDefault(task)
		}
	}

	// Update TLS setting
	now := time.Now()
	site.TlsSetting = tlsSetting
	site.PendingTlsUpdateSince = &now

	return s.repo.Update(ctx, site)
}

// ListCertificates returns all certificates for a site
func (s *Service) ListCertificates(ctx context.Context, siteID, serverID string) ([]Certificate, error) {
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.repo.FindCertificatesBySite(ctx, siteID)
}

// Queue operations

// CreateQueue creates a new queue worker
func (s *Service) CreateQueue(ctx context.Context, siteID, serverID, userID string, req *CreateQueueRequest) (*Queue, error) {
	site, err := s.repo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	user := req.User
	if user == nil || *user == "" {
		user = &site.User
	}

	directory := req.Directory
	if directory == nil || *directory == "" {
		appDir := site.GetApplicationDirectory()
		directory = &appDir
	}

	queue := &Queue{
		SiteID:                site.ID,
		ServerID:              serverID,
		UserID:                userID,
		Name:                  fmt.Sprintf("%s-worker", site.Address),
		Directory:             *directory,
		User:                  *user,
		QueueConnection:       req.QueueConnection,
		QueueName:             req.Queue,
		RestSecondsOnEmpty:    req.RestSecondsOnEmpty,
		MaxSecondsPerJob:      req.MaxSecondsPerJob,
		FailedJobDelaySeconds: req.FailedJobDelaySeconds,
		RunOnMaintenance:      req.RunOnMaintenance,
		RunWithListen:         req.RunWithListen,
		AutoStart:             true,
		AutoRestart:           true,
		RedirectStderr:        true,
		StopWaitSeconds:       10,
		StopSignal:            "TERM",
	}

	if req.MaxTries != nil {
		queue.MaxTries = *req.MaxTries
	}
	if req.MaxMemory != nil {
		queue.MaxMemory = *req.MaxMemory
	} else {
		queue.MaxMemory = 128
	}
	if req.NumProcs != nil {
		queue.NumProcs = *req.NumProcs
	} else {
		queue.NumProcs = 1
	}
	if req.StopWaitSeconds != nil {
		queue.StopWaitSeconds = *req.StopWaitSeconds
	}
	if req.Environment != nil {
		queue.Environment = req.Environment
	}

	if err := s.repo.CreateQueue(ctx, queue); err != nil {
		return nil, err
	}

	// TODO: Dispatch queue installation job

	s.logger.Info().
		Str("site_id", site.ID).
		Str("queue_id", queue.ID).
		Msg("Queue created")

	return queue, nil
}

// ListQueues returns all queues for a site
func (s *Service) ListQueues(ctx context.Context, siteID, serverID string) ([]Queue, error) {
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.repo.FindQueuesBySite(ctx, siteID)
}

// DeleteQueue deletes a queue
func (s *Service) DeleteQueue(ctx context.Context, queueID, siteID, serverID string) error {
	queue, err := s.repo.FindQueueByIDAndSite(ctx, queueID, siteID)
	if err != nil {
		return err
	}

	now := time.Now()
	queue.UninstallationRequestedAt = &now

	if err := s.repo.UpdateQueue(ctx, queue); err != nil {
		return err
	}

	// TODO: Dispatch queue uninstallation job

	s.logger.Info().
		Str("queue_id", queueID).
		Msg("Queue deletion requested")

	return nil
}

// Command operations

// CreateCommand creates and executes a command
func (s *Service) CreateCommand(ctx context.Context, siteID, serverID, userID string, req *CreateCommandRequest) (*Command, error) {
	site, err := s.repo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	if !site.IsInstalled() {
		return nil, ErrSiteNotInstalled
	}

	cmd := &Command{
		SiteID:  site.ID,
		UserID:  userID,
		Command: req.Command,
		Status:  CommandStatusPending,
	}

	if err := s.repo.CreateCommand(ctx, cmd); err != nil {
		return nil, err
	}

	// TODO: Dispatch command execution job

	s.logger.Info().
		Str("site_id", site.ID).
		Str("command_id", cmd.ID).
		Msg("Command created")

	return cmd, nil
}

// ListCommands returns all commands for a site
func (s *Service) ListCommands(ctx context.Context, siteID, serverID string) ([]Command, error) {
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.repo.FindCommandsBySite(ctx, siteID)
}

// Redirect operations

// CreateRedirect creates a new redirect
func (s *Service) CreateRedirect(ctx context.Context, siteID, serverID, userID string, req *CreateRedirectRequest) (*Redirect, error) {
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	redirect := &Redirect{
		SiteID: siteID,
		UserID: userID,
		Mode:   RedirectMode(req.Mode),
		From:   req.From,
		To:     req.To,
		Status: "pending",
	}

	if err := s.repo.CreateRedirect(ctx, redirect); err != nil {
		return nil, err
	}

	// TODO: Dispatch Caddyfile update job

	return redirect, nil
}

// ListRedirects returns all redirects for a site
func (s *Service) ListRedirects(ctx context.Context, siteID, serverID string) ([]Redirect, error) {
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.repo.FindRedirectsBySite(ctx, siteID)
}

// DeleteRedirect deletes a redirect
func (s *Service) DeleteRedirect(ctx context.Context, redirectID, siteID, serverID string) error {
	if _, err := s.repo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	if _, err := s.repo.FindRedirectByID(ctx, redirectID); err != nil {
		return err
	}

	// TODO: Dispatch Caddyfile update job

	return s.repo.DeleteRedirect(ctx, redirectID)
}

// Broadcast helpers

// BroadcastDeploymentProgress broadcasts deployment progress
func (s *Service) BroadcastDeploymentProgress(siteID, deploymentID, status, message string) {
	s.ws.BroadcastToDeployment(deploymentID, "deployment.progress", map[string]interface{}{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

// Helper functions

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func parseMultilineToSlice(s string) []string {
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
