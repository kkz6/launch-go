package backup

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/backup/storage"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

var (
	ErrStorageProviderHasBackups = errors.New("storage provider has associated backups and cannot be deleted")
	ErrInvalidStorageDriver      = errors.New("invalid storage driver")
	ErrConnectionFailed          = errors.New("failed to connect to storage provider")
)

// Service handles business logic for the backup module
type Service struct {
	repo           *Repository
	storageFactory *storage.Factory
	queue          *queue.Client
	ws             *websocket.Hub
	logger         *zerolog.Logger
}

// NewService creates a new backup service
func NewService(repo *Repository, queue *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Service {
	return &Service{
		repo:           repo,
		storageFactory: storage.NewFactory(),
		queue:          queue,
		ws:             ws,
		logger:         logger,
	}
}

// Backup operations

// CreateBackup creates a new backup configuration
func (s *Service) CreateBackup(ctx context.Context, serverID, userID string, req *CreateBackupRequest) (*Backup, error) {
	// Convert include/exclude files to JSON
	includeFiles, err := FromStringSlice(req.IncludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process include files: %w", err)
	}

	excludeFiles, err := FromStringSlice(req.ExcludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process exclude files: %w", err)
	}

	retention := req.Retention
	if retention == 0 {
		retention = 10
	}

	backup := &Backup{
		ServerID:              serverID,
		UserID:                userID,
		StorageProviderID:     req.StorageProviderID,
		CronExpression:        req.CronExpression,
		IncludeFiles:          includeFiles,
		ExcludeFiles:          excludeFiles,
		Retention:             retention,
		NotificationOnFailure: req.NotificationOnFailure,
		NotificationOnSuccess: req.NotificationOnSuccess,
		Enabled:               req.Enabled,
		Path:                  req.Path,
	}

	databaseIDs := []string{req.DatabaseID}

	if err := s.repo.CreateBackupWithDatabases(ctx, backup, databaseIDs); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Dispatch installation job
	s.dispatchInstallBackup(serverID, backup.ID)

	s.logger.Info().
		Str("backup_id", backup.ID).
		Str("server_id", serverID).
		Msg("Backup created successfully")

	return backup, nil
}

// UpdateBackup updates an existing backup configuration
func (s *Service) UpdateBackup(ctx context.Context, id string, req *UpdateBackupRequest) (*Backup, error) {
	backup, err := s.repo.FindBackupByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert include/exclude files to JSON
	includeFiles, err := FromStringSlice(req.IncludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process include files: %w", err)
	}

	excludeFiles, err := FromStringSlice(req.ExcludeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to process exclude files: %w", err)
	}

	backup.CronExpression = req.CronExpression
	backup.Path = req.Path
	backup.Enabled = req.Enabled
	backup.StorageProviderID = req.StorageProviderID
	backup.IncludeFiles = includeFiles
	backup.ExcludeFiles = excludeFiles
	if req.Retention > 0 {
		backup.Retention = req.Retention
	}
	backup.NotificationOnFailure = req.NotificationOnFailure
	backup.NotificationOnSuccess = req.NotificationOnSuccess

	databaseIDs := []string{req.DatabaseID}

	if err := s.repo.UpdateBackupWithDatabases(ctx, backup, databaseIDs); err != nil {
		return nil, fmt.Errorf("failed to update backup: %w", err)
	}

	s.logger.Info().
		Str("backup_id", backup.ID).
		Msg("Backup updated successfully")

	return backup, nil
}

// DeleteBackup deletes a backup configuration
func (s *Service) DeleteBackup(ctx context.Context, id, serverID string) error {
	backup, err := s.repo.FindBackupByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	// Dispatch deletion job
	s.dispatchDeleteBackup(serverID, backup.ID)

	if err := s.repo.DeleteBackup(ctx, id); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	s.logger.Info().
		Str("backup_id", id).
		Str("server_id", serverID).
		Msg("Backup deleted successfully")

	return nil
}

// GetBackup retrieves a backup by ID
func (s *Service) GetBackup(ctx context.Context, id string) (*Backup, error) {
	return s.repo.FindBackupByID(ctx, id)
}

// GetBackupByIDAndServer retrieves a backup by ID and server ID
func (s *Service) GetBackupByIDAndServer(ctx context.Context, id, serverID string) (*Backup, error) {
	return s.repo.FindBackupByIDAndServer(ctx, id, serverID)
}

// ListBackupsByServer lists all backups for a server
func (s *Service) ListBackupsByServer(ctx context.Context, serverID string) ([]Backup, error) {
	return s.repo.FindBackupsByServerID(ctx, serverID)
}

// RunBackup triggers a manual backup run
func (s *Service) RunBackup(ctx context.Context, id, serverID string) error {
	backup, err := s.repo.FindBackupByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	s.dispatchRunManualBackup(serverID, backup.ID)

	s.logger.Info().
		Str("backup_id", id).
		Str("server_id", serverID).
		Msg("Manual backup queued for execution")

	return nil
}

// BackupJob operations

// CreateBackupJob creates a new backup job (called by agent webhook)
func (s *Service) CreateBackupJob(ctx context.Context, backupID, token string, req *CreateBackupJobRequest) (*BackupJob, error) {
	backup, err := s.repo.FindBackupByID(ctx, backupID)
	if err != nil {
		return nil, err
	}

	// Verify dispatch token
	if backup.DispatchToken != token {
		return nil, errors.New("invalid dispatch token")
	}

	var errorMsg *string
	if req.Error != "" {
		errorMsg = &req.Error
	}

	job := &BackupJob{
		BackupID:          backupID,
		StorageProviderID: backup.StorageProviderID,
		Status:            req.Status,
		Size:              req.Size,
		Error:             errorMsg,
	}

	if err := s.repo.CreateBackupJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create backup job: %w", err)
	}

	// Broadcast job status
	s.broadcastBackupJobStatus(backup.ServerID, job)

	s.logger.Info().
		Str("job_id", job.ID).
		Str("backup_id", backupID).
		Str("status", string(job.Status)).
		Msg("Backup job created")

	return job, nil
}

// GetBackupJob retrieves a backup job by ID
func (s *Service) GetBackupJob(ctx context.Context, id string) (*BackupJob, error) {
	return s.repo.FindBackupJobByID(ctx, id)
}

// ListBackupJobs lists all jobs for a backup
func (s *Service) ListBackupJobs(ctx context.Context, backupID string) ([]BackupJob, error) {
	return s.repo.FindBackupJobsByBackupID(ctx, backupID)
}

// StorageProvider operations

// ConnectStorageProvider creates a new storage provider connection
func (s *Service) ConnectStorageProvider(ctx context.Context, userID, teamID string, req *CreateStorageProviderRequest) (*StorageProvider, error) {
	driver := StorageDriver(req.Provider)
	if !driver.IsValid() {
		return nil, ErrInvalidStorageDriver
	}

	// Build credentials based on provider type
	credentials := s.buildCredentials(req)

	// Create and test the storage provider connection
	storageProvider, err := s.storageFactory.Create(req.Provider, credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	if err := storageProvider.Connect(ctx); err != nil {
		s.logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to connect to storage provider")
		return nil, ErrConnectionFailed
	}

	// Create the storage provider record
	provider := &StorageProvider{
		UserID:    userID,
		TeamID:    teamID,
		Provider:  driver,
		Label:     req.Label,
		Connected: true,
	}

	if err := provider.SetCredentials(storageProvider.CredentialData(credentials)); err != nil {
		return nil, fmt.Errorf("failed to set credentials: %w", err)
	}

	if err := s.repo.CreateStorageProvider(ctx, provider); err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	s.logger.Info().
		Uint("provider_id", provider.ID).
		Str("provider_type", req.Provider).
		Msg("Storage provider connected successfully")

	return provider, nil
}

// UpdateStorageProvider updates an existing storage provider
func (s *Service) UpdateStorageProvider(ctx context.Context, id uint, req *UpdateStorageProviderRequest) (*StorageProvider, error) {
	provider, err := s.repo.FindStorageProviderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	driver := StorageDriver(req.Provider)
	if !driver.IsValid() {
		return nil, ErrInvalidStorageDriver
	}

	// Build credentials based on provider type
	credentials := s.buildCredentialsFromUpdate(req)

	// Create and test the storage provider connection
	storageProvider, err := s.storageFactory.Create(req.Provider, credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	if err := storageProvider.Connect(ctx); err != nil {
		s.logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to connect to storage provider")
		return nil, ErrConnectionFailed
	}

	provider.Label = req.Label
	provider.Provider = driver
	provider.Connected = true

	if err := provider.SetCredentials(storageProvider.CredentialData(credentials)); err != nil {
		return nil, fmt.Errorf("failed to set credentials: %w", err)
	}

	if err := s.repo.UpdateStorageProvider(ctx, provider); err != nil {
		return nil, fmt.Errorf("failed to update storage provider: %w", err)
	}

	// Dispatch config sync job for all servers using this provider
	s.dispatchSyncServerLaunchConfig(provider.ID)

	s.logger.Info().
		Uint("provider_id", provider.ID).
		Msg("Storage provider updated successfully")

	return provider, nil
}

// DeleteStorageProvider deletes a storage provider
func (s *Service) DeleteStorageProvider(ctx context.Context, id uint) error {
	// First verify the provider exists
	_, err := s.repo.FindStorageProviderByID(ctx, id)
	if err != nil {
		return err
	}

	hasBackups, err := s.repo.HasBackupsForStorageProvider(ctx, id)
	if err != nil {
		return err
	}

	if hasBackups {
		return ErrStorageProviderHasBackups
	}

	if err := s.repo.DeleteStorageProvider(ctx, id); err != nil {
		return fmt.Errorf("failed to delete storage provider: %w", err)
	}

	s.logger.Info().
		Uint("provider_id", id).
		Msg("Storage provider deleted successfully")

	return nil
}

// GetStorageProvider retrieves a storage provider by ID
func (s *Service) GetStorageProvider(ctx context.Context, id uint) (*StorageProvider, error) {
	return s.repo.FindStorageProviderByID(ctx, id)
}

// ListStorageProvidersByTeam lists all storage providers for a team
func (s *Service) ListStorageProvidersByTeam(ctx context.Context, teamID string) ([]StorageProvider, error) {
	return s.repo.FindStorageProvidersByTeamID(ctx, teamID)
}

// GetStorageProviderConfig gets the agent configuration for a storage provider
func (s *Service) GetStorageProviderConfig(ctx context.Context, id uint) (map[string]interface{}, error) {
	provider, err := s.repo.FindStorageProviderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	credentials, err := provider.GetCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	storageProvider, err := s.storageFactory.Create(string(provider.Provider), credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	return storageProvider.GetConfigForAgent(), nil
}

// GetAgentBackupConfig gets the complete backup configuration for the agent
func (s *Service) GetAgentBackupConfig(ctx context.Context, backupID, webhookBaseURL string) (*AgentBackupConfig, error) {
	backup, err := s.repo.FindBackupByID(ctx, backupID)
	if err != nil {
		return nil, err
	}

	// Get storage provider config
	storageConfig, err := s.GetStorageProviderConfig(ctx, uint(0)) // Need to handle string to uint conversion
	if err != nil {
		// Log error but continue with empty config
		s.logger.Error().Err(err).Msg("Failed to get storage provider config")
		storageConfig = make(map[string]interface{})
	}

	// Get database IDs
	databaseIDs, err := s.repo.GetBackupDatabaseIDs(ctx, backupID)
	if err != nil {
		return nil, err
	}

	includeFiles, _ := backup.IncludeFiles.ToStringSlice()
	excludeFiles, _ := backup.ExcludeFiles.ToStringSlice()

	return &AgentBackupConfig{
		ID:             backup.ID,
		CronExpression: backup.CronExpression,
		Path:           backup.Path,
		Retention:      backup.Retention,
		WebhookURL:     fmt.Sprintf("%s/backup/%s/%s", webhookBaseURL, backup.ID, backup.DispatchToken),
		IncludeFiles:   includeFiles,
		ExcludeFiles:   excludeFiles,
		Databases:      databaseIDs,
		Storage:        storageConfig,
		StorageDriver:  backup.StorageProviderID,
	}, nil
}

// Helper methods

func (s *Service) buildCredentials(req *CreateStorageProviderRequest) map[string]interface{} {
	credentials := make(map[string]interface{})

	switch req.Provider {
	case "s3":
		credentials["endpoint"] = req.Endpoint
		credentials["key"] = req.Key
		credentials["secret"] = req.Secret
		credentials["region"] = req.Region
		credentials["bucket"] = req.Bucket
		credentials["path"] = req.Path
		credentials["force_path_style"] = req.ForcePathStyle
	case "dropbox":
		credentials["token"] = req.Token
	}

	return credentials
}

func (s *Service) buildCredentialsFromUpdate(req *UpdateStorageProviderRequest) map[string]interface{} {
	credentials := make(map[string]interface{})

	switch req.Provider {
	case "s3":
		credentials["endpoint"] = req.Endpoint
		credentials["key"] = req.Key
		credentials["secret"] = req.Secret
		credentials["region"] = req.Region
		credentials["bucket"] = req.Bucket
		credentials["path"] = req.Path
		credentials["force_path_style"] = req.ForcePathStyle
	case "dropbox":
		credentials["token"] = req.Token
	}

	return credentials
}

// Job dispatch helpers

func (s *Service) dispatchInstallBackup(serverID, backupID string) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue an InstallBackup job
	s.logger.Debug().
		Str("server_id", serverID).
		Str("backup_id", backupID).
		Msg("Dispatching InstallBackup job")
}

func (s *Service) dispatchDeleteBackup(serverID, backupID string) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue a DeleteBackup job
	s.logger.Debug().
		Str("server_id", serverID).
		Str("backup_id", backupID).
		Msg("Dispatching DeleteBackup job")
}

func (s *Service) dispatchRunManualBackup(serverID, backupID string) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue a RunManualBackup job
	s.logger.Debug().
		Str("server_id", serverID).
		Str("backup_id", backupID).
		Msg("Dispatching RunManualBackup job")
}

func (s *Service) dispatchSyncServerLaunchConfig(providerID uint) {
	if s.queue == nil {
		return
	}
	// In production, this would enqueue a SyncServerLaunchConfig job
	s.logger.Debug().
		Uint("provider_id", providerID).
		Msg("Dispatching SyncServerLaunchConfig job")
}

// Broadcast helpers

func (s *Service) broadcastBackupJobStatus(serverID string, job *BackupJob) {
	if s.ws == nil {
		return
	}

	s.ws.BroadcastToServer(serverID, "backup.job.status", map[string]interface{}{
		"job_id":    job.ID,
		"backup_id": job.BackupID,
		"status":    string(job.Status),
		"size":      job.Size,
	})
}

// MarkBackupInstalled marks a backup as installed
func (s *Service) MarkBackupInstalled(ctx context.Context, id string) error {
	return s.repo.UpdateBackupFields(ctx, id, map[string]interface{}{
		"installed_at":           "NOW()",
		"installation_failed_at": nil,
	})
}

// MarkBackupInstallationFailed marks a backup installation as failed
func (s *Service) MarkBackupInstallationFailed(ctx context.Context, id string) error {
	return s.repo.UpdateBackupFields(ctx, id, map[string]interface{}{
		"installation_failed_at": "NOW()",
	})
}
