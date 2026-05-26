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

// ListEnabled returns every schedule row with enabled=true and a
// non-empty cron expression — the input set for the every-minute
// PollDueSchedulesJob. Soft-deleted rows are excluded by the default
// GORM scope, and we filter to rows whose parent application is also
// still live (joining via application_id) so deleting an application
// silently stops its schedules without needing an explicit cleanup.
func (r *ScheduleRepository) ListEnabled(ctx context.Context) ([]models.ApplicationSchedule, error) {
	var rows []models.ApplicationSchedule
	err := r.DB.WithContext(ctx).
		Joins("JOIN docker_applications ON docker_applications.id = docker_application_schedules.application_id AND docker_applications.deleted_at IS NULL").
		Where("docker_application_schedules.enabled = ?", true).
		Where("docker_application_schedules.cron <> ''").
		Find(&rows).Error
	return rows, err
}
