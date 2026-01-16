package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServerProviderRepository handles server provider database operations
type ServerProviderRepository struct {
	BaseRepository
}

// NewServerProviderRepository creates a new ServerProviderRepository instance
func NewServerProviderRepository(db *gorm.DB) *ServerProviderRepository {
	return &ServerProviderRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new server provider
func (r *ServerProviderRepository) Create(ctx context.Context, provider *models.ServerProvider) error {
	return r.DB().WithContext(ctx).Create(provider).Error
}

// FindByID finds a server provider by ID
func (r *ServerProviderRepository) FindByID(ctx context.Context, id string) (*models.ServerProvider, error) {
	var provider models.ServerProvider
	err := r.DB().WithContext(ctx).First(&provider, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerProviderNotFound
		}
		return nil, err
	}
	return &provider, nil
}

// FindByTeam finds all server providers for a team
func (r *ServerProviderRepository) FindByTeam(ctx context.Context, teamID string) ([]models.ServerProvider, error) {
	var providers []models.ServerProvider
	err := r.DB().WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error
	return providers, err
}

// Delete deletes a server provider
func (r *ServerProviderRepository) Delete(ctx context.Context, id string) error {
	return r.DB().WithContext(ctx).Delete(&models.ServerProvider{}, "id = ?", id).Error
}
