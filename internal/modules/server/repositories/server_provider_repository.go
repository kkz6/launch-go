package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// FindServerProvidersByTeam finds all server providers for a team
func (r *Repository) FindServerProvidersByTeam(ctx context.Context, teamID string) ([]models.ServerProvider, error) {
	var providers []models.ServerProvider
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&providers).Error

	return providers, err
}

// FindServerProviderByID finds a server provider by ID
func (r *Repository) FindServerProviderByID(ctx context.Context, id string) (*models.ServerProvider, error) {
	var provider models.ServerProvider
	err := r.db.WithContext(ctx).First(&provider, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

// CreateServerProvider creates a new server provider
func (r *Repository) CreateServerProvider(ctx context.Context, provider *models.ServerProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// DeleteServerProvider deletes a server provider
func (r *Repository) DeleteServerProvider(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.ServerProvider{}, "id = ?", id).Error
}
