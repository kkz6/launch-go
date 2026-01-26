package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/storage"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
)

// StorageProviderService handles business logic for storage providers
type StorageProviderService struct {
	*BaseService
	storageFactory *storage.Factory
}

// NewStorageProviderService creates a new storage provider service
func NewStorageProviderService(deps *ServiceDeps, storageFactory *storage.Factory) *StorageProviderService {
	return &StorageProviderService{
		BaseService:    NewBaseService(deps),
		storageFactory: storageFactory,
	}
}

// ConnectStorageProvider creates a new storage provider connection
func (s *StorageProviderService) ConnectStorageProvider(ctx context.Context, userID, teamID string, req *dto.CreateStorageProviderRequest) (*models.StorageProvider, error) {
	driver := backuptypes.StorageDriver(req.Provider)
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
		s.Logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to connect to storage provider")
		return nil, ErrConnectionFailed
	}

	// Create the storage provider record
	provider := &models.StorageProvider{
		UserID:    userID,
		TeamID:    teamID,
		Provider:  driver,
		Label:     &req.Label,
		Connected: true,
	}

	provider.SetCredentials(storageProvider.CredentialData(credentials))

	if err := s.Repos().StorageProvider().CreateStorageProvider(ctx, provider); err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	s.Logger.Info().
		Uint64("provider_id", provider.ID).
		Str("provider_type", req.Provider).
		Msg("Storage provider connected successfully")

	return provider, nil
}

// UpdateStorageProvider updates an existing storage provider
func (s *StorageProviderService) UpdateStorageProvider(ctx context.Context, id uint64, req *dto.UpdateStorageProviderRequest) (*models.StorageProvider, error) {
	provider, err := s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	driver := backuptypes.StorageDriver(req.Provider)
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
		s.Logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to connect to storage provider")
		return nil, ErrConnectionFailed
	}

	provider.Label = &req.Label
	provider.Provider = driver
	provider.Connected = true

	provider.SetCredentials(storageProvider.CredentialData(credentials))

	if err := s.Repos().StorageProvider().UpdateStorageProvider(ctx, provider); err != nil {
		return nil, fmt.Errorf("failed to update storage provider: %w", err)
	}

	// Dispatch config sync job for all servers using this provider
	s.dispatchSyncServerLaunchConfig(provider.ID)

	s.Logger.Info().
		Uint64("provider_id", provider.ID).
		Msg("Storage provider updated successfully")

	return provider, nil
}

// DeleteStorageProvider deletes a storage provider
func (s *StorageProviderService) DeleteStorageProvider(ctx context.Context, id uint64) error {
	// First verify the provider exists
	_, err := s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
	if err != nil {
		return err
	}

	hasBackups, err := s.Repos().StorageProvider().HasBackupsForStorageProvider(ctx, id)
	if err != nil {
		return err
	}

	if hasBackups {
		return ErrStorageProviderHasBackups
	}

	if err := s.Repos().StorageProvider().DeleteStorageProvider(ctx, id); err != nil {
		return fmt.Errorf("failed to delete storage provider: %w", err)
	}

	s.Logger.Info().
		Uint64("provider_id", id).
		Msg("Storage provider deleted successfully")

	return nil
}

// GetStorageProvider retrieves a storage provider by ID
func (s *StorageProviderService) GetStorageProvider(ctx context.Context, id uint64) (*models.StorageProvider, error) {
	return s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
}

// ListStorageProvidersByTeam lists all storage providers for a team
func (s *StorageProviderService) ListStorageProvidersByTeam(ctx context.Context, teamID string) ([]models.StorageProvider, error) {
	return s.Repos().StorageProvider().FindStorageProvidersByTeamID(ctx, teamID)
}

// GetStorageProviderConfig gets the agent configuration for a storage provider
func (s *StorageProviderService) GetStorageProviderConfig(ctx context.Context, id uint64) (map[string]any, error) {
	provider, err := s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	credentials := provider.GetCredentials()

	storageProvider, err := s.storageFactory.Create(string(provider.Provider), credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	return storageProvider.GetConfigForAgent(), nil
}

// Helper methods

func (s *StorageProviderService) buildCredentials(req *dto.CreateStorageProviderRequest) map[string]any {
	credentials := make(map[string]any)

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

func (s *StorageProviderService) buildCredentialsFromUpdate(req *dto.UpdateStorageProviderRequest) map[string]any {
	credentials := make(map[string]any)

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

func (s *StorageProviderService) dispatchSyncServerLaunchConfig(providerID uint64) {
	if s.Queue == nil {
		return
	}
	// In production, this would enqueue a SyncServerLaunchConfig job
	s.Logger.Debug().
		Uint64("provider_id", providerID).
		Msg("Dispatching SyncServerLaunchConfig job")
}
