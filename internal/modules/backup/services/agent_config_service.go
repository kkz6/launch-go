package services

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
)

// AgentConfigService handles agent configuration logic
type AgentConfigService struct {
	backupRepo      *repositories.BackupRepository
	providerService *StorageProviderService
	logger          *zerolog.Logger
}

// NewAgentConfigService creates a new agent config service
func NewAgentConfigService(
	backupRepo *repositories.BackupRepository,
	providerService *StorageProviderService,
	logger *zerolog.Logger,
) *AgentConfigService {
	return &AgentConfigService{
		backupRepo:      backupRepo,
		providerService: providerService,
		logger:          logger,
	}
}

// GetAgentBackupConfig gets the complete backup configuration for the agent
func (s *AgentConfigService) GetAgentBackupConfig(ctx context.Context, backupID, webhookBaseURL string) (*dto.AgentBackupConfig, error) {
	backup, err := s.backupRepo.FindBackupByID(ctx, backupID)
	if err != nil {
		return nil, err
	}

	// Get storage provider config
	storageConfig, err := s.providerService.GetStorageProviderConfig(ctx, uint(0)) // Need to handle string to uint conversion
	if err != nil {
		// Log error but continue with empty config
		s.logger.Error().Err(err).Msg("Failed to get storage provider config")
		storageConfig = make(map[string]interface{})
	}

	// Get database IDs
	databaseIDs, err := s.backupRepo.GetBackupDatabaseIDs(ctx, backupID)
	if err != nil {
		return nil, err
	}

	includeFiles, _ := backup.IncludeFiles.ToStringSlice()
	excludeFiles, _ := backup.ExcludeFiles.ToStringSlice()

	return &dto.AgentBackupConfig{
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
