package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DeploymentRepository handles persistence for deployment history.
type DeploymentRepository struct {
	repository.Base[models.Deployment]
}

// DeploymentHistoryLimit is the maximum number of history rows retained for
// one workload. Keeping the rule here ensures every insertion path, including
// GitHub Actions webhooks, applies the same cap.
const DeploymentHistoryLimit = 10

// NewDeploymentRepository wires the base repository for deployments.
func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{Base: repository.NewBase[models.Deployment](db)}
}

// Create inserts a deployment and prunes older history for the same target in
// one transaction. This intentionally shadows Base.Create so callers cannot
// create an unbounded deployment history by bypassing a service helper.
func (r *DeploymentRepository) Create(ctx context.Context, deployment *models.Deployment) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(deployment).Error; err != nil {
			return err
		}

		return NewDeploymentRepository(tx).PruneForTarget(
			ctx,
			deployment.TargetType,
			deployment.TargetID,
			DeploymentHistoryLimit,
		)
	})
}

// FindByID looks up a deployment by ID; wraps NotFound for handler use.
func (r *DeploymentRepository) FindByID(ctx context.Context, id string) (*models.Deployment, error) {
	return repository.FindOne[models.Deployment](ctx, r.DB,
		repository.WithID(id),
	)
}

// Delete removes a deployment history row by id (hard delete — these are
// disposable log records).
func (r *DeploymentRepository) Delete(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.Deployment{}).Error
}

// ListForTarget returns deployment history for a given (type, id) pair,
// most recent first. Used by the Deployments subtab across all three
// workload kinds — applications, composes, and databases (target_type =
// "database", added in migration 0025).
func (r *DeploymentRepository) ListForTarget(
	ctx context.Context, targetType, targetID string,
) ([]models.Deployment, error) {
	var deployments []models.Deployment
	err := r.DB.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").
		Limit(50). // a single workload's history shouldn't dump unboundedly
		Find(&deployments).Error
	return deployments, err
}

// SupersedeInProgressForTarget marks every non-terminal deployment for a
// target (pending/building/deploying) as cancelled. Called when a newer
// deploy is triggered so stale rows — e.g. a placeholder stranded by a
// cancelled GitHub Actions run (#99) — don't linger as "Pending" forever.
// Returns the number of rows superseded.
func (r *DeploymentRepository) SupersedeInProgressForTarget(
	ctx context.Context, targetType, targetID string,
) (int64, error) {
	now := time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&models.Deployment{}).
		Where("target_type = ? AND target_id = ? AND status IN ?",
			targetType, targetID,
			[]string{
				string(dockertypes.DeploymentStatusPending),
				string(dockertypes.DeploymentStatusBuilding),
				string(dockertypes.DeploymentStatusDeploying),
			}).
		Updates(map[string]any{
			"status":      dockertypes.DeploymentStatusCancelled,
			"finished_at": now,
			"error":       "Superseded by a newer deployment",
		})
	return res.RowsAffected, res.Error
}

// PruneForTarget keeps only the newest `keep` deployment rows for a target,
// hard-deleting the rest. Deploy history is disposable, so we cap it (the
// UI only needs the recent few). No-op when keep <= 0.
func (r *DeploymentRepository) PruneForTarget(
	ctx context.Context, targetType, targetID string, keep int,
) error {
	if keep <= 0 {
		return nil
	}
	var keepIDs []string
	if err := r.DB.WithContext(ctx).Model(&models.Deployment{}).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").
		Order("id DESC").
		Limit(keep).
		Pluck("id", &keepIDs).Error; err != nil {
		return err
	}
	if len(keepIDs) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).
		Where("target_type = ? AND target_id = ? AND id NOT IN ?", targetType, targetID, keepIDs).
		Delete(&models.Deployment{}).Error
}

// LatestImageRefForTarget returns the image_ref of the most recent
// deployment for a target that recorded one. Used by the Reload/restart
// (recreate) path as a FALLBACK image source: the recreate script
// prefers the image the running container is actually using, and only
// falls back to this DB value when there's no container to inspect.
// Returns "" (no error) when no deployment has an image_ref yet.
func (r *DeploymentRepository) LatestImageRefForTarget(
	ctx context.Context, targetType, targetID string,
) (string, error) {
	var dep models.Deployment
	err := r.DB.WithContext(ctx).
		Where("target_type = ? AND target_id = ? AND image_ref IS NOT NULL AND image_ref <> ''", targetType, targetID).
		Order("created_at DESC").
		First(&dep).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if dep.ImageRef == nil {
		return "", nil
	}
	return *dep.ImageRef, nil
}
