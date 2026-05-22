package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ScheduleRepository handles persistence for application schedules.
type ScheduleRepository struct {
	repository.Base[models.ApplicationSchedule]
}

func NewScheduleRepository(db *gorm.DB) *ScheduleRepository {
	return &ScheduleRepository{Base: repository.NewBase[models.ApplicationSchedule](db)}
}

func (r *ScheduleRepository) FindByID(ctx context.Context, id string) (*models.ApplicationSchedule, error) {
	var s models.ApplicationSchedule
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &s, nil
}

func (r *ScheduleRepository) ListForApplication(
	ctx context.Context, applicationID string,
) ([]models.ApplicationSchedule, error) {
	var rows []models.ApplicationSchedule
	err := r.DB.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}
