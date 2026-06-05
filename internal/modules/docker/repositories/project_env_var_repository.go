package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ProjectEnvVarRepository handles persistence for project-scoped env
// vars. Same shape as EnvVarRepository (application-scoped); kept
// separate because the foreign-key column differs and a single generic
// repo would obscure that.
type ProjectEnvVarRepository struct {
	repository.Base[models.ProjectEnvVar]
}

func NewProjectEnvVarRepository(db *gorm.DB) *ProjectEnvVarRepository {
	return &ProjectEnvVarRepository{Base: repository.NewBase[models.ProjectEnvVar](db)}
}

func (r *ProjectEnvVarRepository) FindByID(ctx context.Context, id string) (*models.ProjectEnvVar, error) {
	return repository.FindOne[models.ProjectEnvVar](ctx, r.DB,
		repository.WithID(id),
	)
}

func (r *ProjectEnvVarRepository) ListForProject(
	ctx context.Context, projectID string,
) ([]models.ProjectEnvVar, error) {
	var rows []models.ProjectEnvVar
	err := r.DB.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order(`"key" ASC`).
		Find(&rows).Error
	return rows, err
}

// ListMapForProject returns a key→value map for the resolver hot
// path. Used by `services.env_interpolation.Resolve` when expanding
// `${{project.<KEY>}}` references at deploy/run time. Cheaper than
// re-iterating the slice on every substitution.
func (r *ProjectEnvVarRepository) ListMapForProject(
	ctx context.Context, projectID string,
) (map[string]string, error) {
	rows, err := r.ListForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, v := range rows {
		// EncryptedString → string for the resolver. The Scan side
		// already decrypted the column when GORM hydrated the row,
		// so `string(v.Value)` is the plaintext value.
		out[v.Key] = string(v.Value)
	}
	return out, nil
}

func (r *ProjectEnvVarRepository) ExistsByKey(
	ctx context.Context, projectID, key, excludeID string,
) (bool, error) {
	q := r.DB.WithContext(ctx).
		Model(&models.ProjectEnvVar{}).
		Where(`project_id = ? AND "key" = ?`, projectID, key)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
