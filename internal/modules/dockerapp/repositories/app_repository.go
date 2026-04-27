package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// AppRepository handles docker-app database operations.
type AppRepository struct {
	repository.Base[models.App]
}

// NewAppRepository constructs a new repository.
func NewAppRepository(db *gorm.DB) *AppRepository {
	return &AppRepository{
		Base: repository.NewBase[models.App](db),
	}
}

// FindByIDWithRelations preloads env vars / ports / volumes / domains so
// the deploy job and detail page can render in one query.
func (r *AppRepository) FindByIDWithRelations(ctx context.Context, id string) (*models.App, error) {
	var app models.App
	err := r.DB.WithContext(ctx).
		Preload("EnvVars").
		Preload("Ports").
		Preload("Volumes").
		Preload("Domains").
		First(&app, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &app, nil
}

// FindByIDAndServer scopes a lookup to a server. Returns NotFound when
// the row does not exist or belongs to a different server.
func (r *AppRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.App, error) {
	var app models.App
	err := r.DB.WithContext(ctx).
		Where("id = ? AND server_id = ?", id, serverID).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &app, nil
}

// FindByServer returns every app for a server, ordered by creation time.
func (r *AppRepository) FindByServer(ctx context.Context, serverID string) ([]models.App, error) {
	var out []models.App
	err := r.DB.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at ASC").
		Find(&out).Error
	return out, err
}

// FindByNameAndServer returns the app for a (server, name) pair, or nil
// if none exists. Used to detect duplicates before insert.
func (r *AppRepository) FindByNameAndServer(ctx context.Context, name, serverID string) (*models.App, error) {
	var app models.App
	err := r.DB.WithContext(ctx).
		Where("server_id = ? AND name = ?", serverID, name).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &app, nil
}

// Update applies partial column updates to an app.
func (r *AppRepository) Update(ctx context.Context, id string, updates map[string]any) error {
	result := r.DB.WithContext(ctx).
		Model(&models.App{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fiberutil.NotFound()
	}
	return nil
}

// Delete removes an app row by ID. The FK CASCADE drops sub-resources.
func (r *AppRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.App{}, "id = ?", id).Error
}
