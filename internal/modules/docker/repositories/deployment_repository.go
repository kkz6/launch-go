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
