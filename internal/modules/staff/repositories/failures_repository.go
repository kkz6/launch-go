package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// FailedDeploymentRow is a failed deployment joined to the error output of its
// backing task. TaskOutput/TaskExitCode are empty/nil when the deployment has
// no task_id or the joined task row is absent. TaskOutput maps onto the joined
// tasks.output column as an EncryptedString so its Scanner decrypts the
// ciphertext on read — the same way the Task model reads it.
type FailedDeploymentRow struct {
	sitemodels.Deployment
	TaskOutput   dbtype.EncryptedString `gorm:"column:task_output"`
	TaskExitCode *int                   `gorm:"column:task_exit_code"`
}

// FailedProvisions returns servers whose provisioning failed, newest first.
// A provision failure is status='failed' OR a non-empty provision_error.
func (r *Registry) FailedProvisions(ctx context.Context, limit int) ([]servermodels.Server, error) {
	var servers []servermodels.Server
	err := r.db.WithContext(ctx).
		Model(&servermodels.Server{}).
		Where("status = ? OR (provision_error IS NOT NULL AND provision_error <> '')",
			servertypes.ServerStatusFailed).
		Order("updated_at DESC").
		Limit(limit).
		Find(&servers).Error
	if err != nil {
		return nil, err
	}

	return servers, nil
}

// serviceOperationNamePatterns matches the software installation and update
// tasks surfaced by the admin failure monitor. Filtering on these names keeps
// unrelated task-runner activity (uploads, restarts and status checks) out of
// the operational failure feed while including Launch Agent upgrades. These
// tasks are built from the generic taskrunner.BaseTask, so their reflect `type`
// column is shared with unrelated tasks and can't distinguish them — only the
// human Name can. The trailing space avoids matching "Uninstall …".
var serviceOperationNamePatterns = []string{"Install %", "Update Launch Agent"}

// FailedServiceInstalls returns failed/timed-out software-installation tasks
// (PHP, MySQL, Caddy, Redis, …), newest first. The admin failures monitor only
// surfaces these — not the full task feed — so the query restricts by the
// "Install …" Name prefix. The Task's Output column is encrypted and decrypted
// on read.
func (r *Registry) FailedServiceInstalls(ctx context.Context, limit int) ([]servermodels.Task, error) {
	var tasks []servermodels.Task
	err := r.db.WithContext(ctx).
		Model(&servermodels.Task{}).
		Where("status IN ?", []string{
			string(servertypes.TaskStatusFailed),
			string(servertypes.TaskStatusTimeout),
		}).
		Where("name LIKE ? OR name = ?", serviceOperationNamePatterns[0], serviceOperationNamePatterns[1]).
		Order("updated_at DESC").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

// FailedTaskByID loads a single task by id (full, decrypted output). Returns
// nil when absent. Used to surface the complete log for one failure.
func (r *Registry) FailedTaskByID(ctx context.Context, id string) (*servermodels.Task, error) {
	if id == "" {
		return nil, nil
	}

	var task servermodels.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// FailedDeploymentByID loads a single failed deployment by id joined to its
// task's full output. Returns nil when absent.
func (r *Registry) FailedDeploymentByID(ctx context.Context, id string) (*FailedDeploymentRow, error) {
	if id == "" {
		return nil, nil
	}

	var row FailedDeploymentRow
	err := r.db.WithContext(ctx).
		Model(&sitemodels.Deployment{}).
		Select("deployments.*, tasks.output AS task_output, tasks.exit_code AS task_exit_code").
		Joins("LEFT JOIN tasks ON tasks.id = deployments.task_id").
		Where("deployments.id = ?", id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &row, nil
}

// FailedDeployments returns failed deployments joined to their task's error
// output, newest first. The LEFT JOIN to tasks keeps this a single query (no
// N+1) while still surfacing the underlying task exit code/output. The joined
// task.output column is mapped onto FailedDeploymentRow.TaskOutput, which is an
// EncryptedString, so GORM's Scanner decrypts it on read — the same path the
// Task model uses; no separate decrypt step is needed.
func (r *Registry) FailedDeployments(ctx context.Context, limit int) ([]FailedDeploymentRow, error) {
	var rows []FailedDeploymentRow
	err := r.db.WithContext(ctx).
		Model(&sitemodels.Deployment{}).
		Select("deployments.*, tasks.output AS task_output, tasks.exit_code AS task_exit_code").
		Joins("LEFT JOIN tasks ON tasks.id = deployments.task_id").
		Where("deployments.status = ?", string(sitetypes.DeploymentStatusFailed)).
		Order("deployments.updated_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}
