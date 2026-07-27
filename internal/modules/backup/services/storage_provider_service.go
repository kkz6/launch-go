package services

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/storage"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// StorageProviderService handles business logic for storage providers.
type StorageProviderService struct {
	*BaseService
	storageFactory *storage.Factory
}

// NewStorageProviderService creates a new storage provider service.
func NewStorageProviderService(deps *ServiceDeps, storageFactory *storage.Factory) *StorageProviderService {
	return &StorageProviderService{
		BaseService:    NewBaseService(deps),
		storageFactory: storageFactory,
	}
}

// ConnectStorageProvider creates a new storage provider connection and
// returns the response DTO.
func (s *StorageProviderService) ConnectStorageProvider(ctx context.Context, teamID, userID string, req *dto.CreateStorageProviderRequest) (dto.StorageProviderResponse, error) {
	driver := backuptypes.StorageDriver(req.Provider)
	if !driver.IsValid() {
		return dto.StorageProviderResponse{}, ErrInvalidStorageDriver
	}

	credentials := s.buildCredentials(req)

	storageProvider, err := s.storageFactory.Create(req.Provider, credentials)
	if err != nil {
		return dto.StorageProviderResponse{}, fmt.Errorf("failed to create storage provider: %w", err)
	}

	if err := storageProvider.Connect(ctx); err != nil {
		s.Logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to connect to storage provider")
		return dto.StorageProviderResponse{}, ErrConnectionFailed
	}

	provider := &models.StorageProvider{
		UserID:    userID,
		TeamID:    teamID,
		Provider:  driver,
		Label:     &req.Label,
		Connected: true,
	}
	provider.SetCredentials(storageProvider.CredentialData(credentials))

	if err := s.Repos().StorageProvider().CreateStorageProvider(ctx, provider); err != nil {
		return dto.StorageProviderResponse{}, fmt.Errorf("failed to create storage provider: %w", err)
	}

	s.Logger.Info().Uint64("provider_id", provider.ID).Str("provider_type", req.Provider).Msg("Storage provider connected successfully")
	return dto.ToStorageProviderResponse(provider), nil
}

// UpdateStorageProvider updates an existing storage provider and returns
// the response DTO.
func (s *StorageProviderService) UpdateStorageProvider(ctx context.Context, id uint64, teamID string, req *dto.UpdateStorageProviderRequest) (dto.StorageProviderResponse, error) {
	provider, err := s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
	if err != nil {
		return dto.StorageProviderResponse{}, err
	}
	if provider.TeamID != teamID {
		return dto.StorageProviderResponse{}, fiberutil.NotFound()
	}

	driver := backuptypes.StorageDriver(req.Provider)
	if !driver.IsValid() {
		return dto.StorageProviderResponse{}, ErrInvalidStorageDriver
	}

	credentials := s.buildCredentialsFromUpdate(req)

	storageProvider, err := s.storageFactory.Create(req.Provider, credentials)
	if err != nil {
		return dto.StorageProviderResponse{}, fmt.Errorf("failed to create storage provider: %w", err)
	}

	if err := storageProvider.Connect(ctx); err != nil {
		s.Logger.Error().Err(err).Str("provider", req.Provider).Msg("Failed to connect to storage provider")
		return dto.StorageProviderResponse{}, ErrConnectionFailed
	}

	provider.Label = &req.Label
	provider.Provider = driver
	provider.Connected = true
	provider.SetCredentials(storageProvider.CredentialData(credentials))

	if err := s.Repos().StorageProvider().UpdateStorageProvider(ctx, provider); err != nil {
		return dto.StorageProviderResponse{}, fmt.Errorf("failed to update storage provider: %w", err)
	}

	s.dispatchSyncServerLaunchConfig(provider.ID)
	s.Logger.Info().Uint64("provider_id", provider.ID).Msg("Storage provider updated successfully")
	return dto.ToStorageProviderResponse(provider), nil
}

// DeleteStorageProvider deletes a storage provider after verifying it
// has no associated backups.
func (s *StorageProviderService) DeleteStorageProvider(ctx context.Context, id uint64, teamID string) error {
	provider, err := s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
	if err != nil {
		return err
	}
	if provider.TeamID != teamID {
		return fiberutil.NotFound()
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

	s.Logger.Info().Uint64("provider_id", id).Msg("Storage provider deleted successfully")
	return nil
}

// GetStorageProvider retrieves a storage provider by string id and
// returns the response DTO. The team is checked after lookup so a provider
// ID cannot be used to read another team's credentials.
func (s *StorageProviderService) GetStorageProvider(ctx context.Context, id, teamID string) (dto.StorageProviderResponse, error) {
	uid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return dto.StorageProviderResponse{}, fiberutil.BadRequest("Invalid provider ID")
	}
	provider, err := s.Repos().StorageProvider().FindStorageProviderByID(ctx, uid)
	if err != nil {
		return dto.StorageProviderResponse{}, err
	}
	if provider.TeamID != teamID {
		return dto.StorageProviderResponse{}, fiberutil.NotFound()
	}
	return dto.ToStorageProviderResponse(provider), nil
}

// ListStorageProviders lists all storage providers for a team. Signature
// matches IndexFunc.
func (s *StorageProviderService) ListStorageProviders(ctx context.Context, teamID string) ([]dto.StorageProviderResponse, error) {
	providers, err := s.Repos().StorageProvider().FindStorageProvidersByTeamID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	return pkgdto.TransformSlice(providers, dto.ToStorageProviderResponse), nil
}

// ListStorageProvidersDropdown returns a simplified id→label map suitable
// for dropdowns. Signature matches IndexFunc.
func (s *StorageProviderService) ListStorageProvidersDropdown(ctx context.Context, teamID string) (map[uint64]string, error) {
	providers, err := s.Repos().StorageProvider().FindStorageProvidersByTeamID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]string, len(providers))
	for _, p := range providers {
		label := ""
		if p.Label != nil {
			label = *p.Label
		}
		out[p.ID] = label
	}
	return out, nil
}

// GetStorageProviderRaw is the model-returning entrypoint preserved for
// cross-module callers. HTTP handlers should use GetStorageProvider.
func (s *StorageProviderService) GetStorageProviderRaw(ctx context.Context, id uint64) (*models.StorageProvider, error) {
	return s.Repos().StorageProvider().FindStorageProviderByID(ctx, id)
}

// GetStorageProviderConfig gets the agent configuration for a storage provider.
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
	s.Logger.Debug().Uint64("provider_id", providerID).Msg("Dispatching SyncServerLaunchConfig job")
}
