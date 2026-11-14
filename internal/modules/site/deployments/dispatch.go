package deployments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
)

var ErrQueueUnavailable = errors.New("deployment queue not configured")

// Dispatcher keeps deployment records consistent when task construction or
// queueing fails. Enqueue must return queue errors without suppressing conflicts.
type Dispatcher struct {
	DB          *gorm.DB
	Enqueue     func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	Broadcaster broadcast.Broadcaster
}

func (d Dispatcher) Dispatch(ctx context.Context, deployment *models.Deployment, factory func() (*asynq.Task, error), opts ...asynq.Option) error {
	task, err := factory()
	if err == nil {
		if d.Enqueue == nil {
			err = ErrQueueUnavailable
		} else {
			_, err = d.Enqueue(task, opts...)
		}
	}
	if err == nil {
		return nil
	}

	// A cancelled request must still release its pending deployment. Only
	// repair pending rows: a worker may already have started after a lost reply.
	repairCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	result := d.DB.WithContext(repairCtx).Model(&models.Deployment{}).
		Where("id = ? AND site_id = ? AND team_id = ? AND status = ?",
			deployment.ID, deployment.SiteID, deployment.TeamID, sitetypes.DeploymentStatusPending).
		Update("status", sitetypes.DeploymentStatusFailed)
	if result.Error != nil {
		return errors.Join(err, fmt.Errorf("record deployment dispatch failure: %w", result.Error))
	}
	if result.RowsAffected > 0 {
		deployment.Status = sitetypes.DeploymentStatusFailed
		if d.Broadcaster != nil {
			for _, event := range []string{"deployment.failed", "deployment.progress"} {
				d.Broadcaster.BroadcastToTeam(deployment.TeamID, event, map[string]any{
					"team_id":       deployment.TeamID,
					"site_id":       deployment.SiteID,
					"deployment_id": deployment.ID,
					"status":        string(deployment.Status),
					"message":       "Failed to queue deployment",
				})
			}
		}
	}
	return err
}

// AdvanceNext claims one waiting deployment when the site has no active work.
// The queue call runs after the database transaction commits. A dispatch failure
// fails only this candidate, leaving the remaining queue and predecessor intact.
func (d Dispatcher) AdvanceNext(ctx context.Context, siteID string) (*models.Deployment, error) {
	var site models.Site
	var next models.Deployment
	claimed := false
	err := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&site, "id = ?", siteID).Error; err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&models.Deployment{}).
			Where("site_id = ? AND status IN ?", siteID, []sitetypes.DeploymentStatus{
				sitetypes.DeploymentStatusPending, sitetypes.DeploymentStatusInstalling,
			}).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return nil
		}
		err := tx.Where("site_id = ? AND status = ?", siteID, sitetypes.DeploymentStatusQueued).
			Order("created_at ASC, id ASC").First(&next).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		result := tx.Model(&next).Where("status = ?", sitetypes.DeploymentStatusQueued).
			Update("status", sitetypes.DeploymentStatusPending)
		claimed = result.Error == nil && result.RowsAffected == 1
		return result.Error
	})
	if err != nil || !claimed {
		return nil, err
	}
	if err := d.Dispatch(ctx, &next, func() (*asynq.Task, error) {
		return NewTask(site.ID, next.ID, "", nil, site.ZeroDowntimeDeployment)
	}); err != nil {
		return nil, err
	}
	return &next, nil
}
