package services

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// AgentConfigService handles agent configuration logic
type AgentConfigService struct {
	*BaseService
}

// NewAgentConfigService creates a new agent config service
func NewAgentConfigService(deps *ServiceDeps) *AgentConfigService {
	return &AgentConfigService{
		BaseService: NewBaseService(deps),
	}
}

// GetAgentBackupConfig gets the complete backup configuration for the agent
func (s *AgentConfigService) GetAgentBackupConfig(ctx context.Context, backupID, webhookBaseURL string) (*dto.AgentBackupConfig, error) {
	backup, err := s.Repos().Backup().FindByID(ctx, backupID)
	if err != nil {
		return nil, err
	}

	// Get storage provider config
	storageConfig, err := s.Services().StorageProvider().GetStorageProviderConfig(ctx, backup.StorageProviderID, backup.TeamID)
	if err != nil {
		// Log error but continue with empty config
		s.Logger.Error().Err(err).Msg("Failed to get storage provider config")
		storageConfig = make(map[string]interface{})
	}

	// Get database IDs
	databaseIDs, err := s.Repos().Backup().GetBackupDatabaseIDs(ctx, backupID)
	if err != nil {
		return nil, err
	}

	// Parse include/exclude files from JSON strings
	var includeFiles []string
	if err := json.Unmarshal([]byte(backup.IncludeFiles), &includeFiles); err != nil {
		includeFiles = []string{}
	}

	var excludeFiles []string
	if err := json.Unmarshal([]byte(backup.ExcludeFiles), &excludeFiles); err != nil {
		excludeFiles = []string{}
	}

	return &dto.AgentBackupConfig{
		ID:             backup.ID,
		CronExpression: backup.CronExpression,
		Path:           backup.Path,
		Retention:      backup.Retention,
		WebhookURL:     util.New(webhookBaseURL).Path("backup", backup.ID, backup.DispatchToken).String(),
		IncludeFiles:   includeFiles,
		ExcludeFiles:   excludeFiles,
		Databases:      databaseIDs,
		Storage:        storageConfig,
		StorageDriver:  strconv.FormatUint(backup.StorageProviderID, 10),
	}, nil
}
