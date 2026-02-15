package repositories

import (
	"context"
	"errors"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
)

// PlatformUpdateRepository handles CRUD operations for platform updates
type PlatformUpdateRepository struct {
	db *gorm.DB
}

// NewPlatformUpdateRepository creates a new PlatformUpdateRepository
func NewPlatformUpdateRepository(db *gorm.DB) *PlatformUpdateRepository {
	return &PlatformUpdateRepository{db: db}
}

// FindByID finds a platform update by ID
func (r *PlatformUpdateRepository) FindByID(ctx context.Context, id string) (*models.PlatformUpdate, error) {
	var update models.PlatformUpdate
	err := r.db.WithContext(ctx).First(&update, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	return &update, nil
}

// FindByKey finds a platform update by its unique key
func (r *PlatformUpdateRepository) FindByKey(ctx context.Context, key string) (*models.PlatformUpdate, error) {
	var update models.PlatformUpdate
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&update).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}

	return &update, nil
}

// Create creates a new platform update
func (r *PlatformUpdateRepository) Create(ctx context.Context, update *models.PlatformUpdate) error {
	return r.db.WithContext(ctx).Create(update).Error
}

// FindAll returns all platform updates
func (r *PlatformUpdateRepository) FindAll(ctx context.Context) ([]models.PlatformUpdate, error) {
	var updates []models.PlatformUpdate
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&updates).Error

	return updates, err
}
