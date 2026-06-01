package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// DeploymentRepository handles persistence for deployment history.
type DeploymentRepository struct {
	repository.Base[models.Deployment]
}

// NewDeploymentRepository wires the base repository for deployments.
func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{Base: repository.NewBase[models.Deployment](db)}
}

// FindByID looks up a deployment by ID; wraps NotFound for handler use.
func (r *DeploymentRepository) FindByID(ctx context.Context, id string) (*models.Deployment, error) {
	var d models.Deployment
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &d, nil
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
