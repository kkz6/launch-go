package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RollbackDeployment rolls back a site to a previous deployment release
type RollbackDeployment struct {
	BaseSiteTask
	currentDeployment *models.Deployment
	targetDeployment  *models.Deployment
}

// NewRollbackDeployment creates a new RollbackDeployment task
func NewRollbackDeployment(site *models.Site, currentDeployment, targetDeployment *models.Deployment) *RollbackDeployment {
	task := &RollbackDeployment{
		BaseSiteTask: BaseSiteTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "site/rollback-deployment",
				TaskTimeout:  120 * time.Second,
			},
			site: site,
		},
		currentDeployment: currentDeployment,
		targetDeployment:  targetDeployment,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// Data returns the template data
func (t *RollbackDeployment) Data() map[string]interface{} {
	return map[string]interface{}{
		"Site":             t.siteData(),
		"CurrentDirectory": t.currentDirectory(),
		"ReleaseDirectory": t.releaseDirectory(),
	}
}

func (t *RollbackDeployment) siteData() map[string]interface{} {
	return map[string]interface{}{
		"Path":    t.site.Path,
		"Address": t.site.Address,
	}
}

func (t *RollbackDeployment) currentDirectory() string {
	return fmt.Sprintf("%s/current", t.site.Path)
}

func (t *RollbackDeployment) releaseDirectory() string {
	if t.targetDeployment.CreatedAt != nil {
		return fmt.Sprintf("%s/releases/%d", t.site.Path, t.targetDeployment.CreatedAt.Unix())
	}

	return fmt.Sprintf("%s/releases/%s", t.site.Path, t.targetDeployment.ID)
}

func (t *RollbackDeployment) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update current deployment status to finished
	// Fire rollback completed event
}

func (t *RollbackDeployment) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update current deployment status to failed
	// Fire rollback failed event
}

func (t *RollbackDeployment) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update current deployment status to timeout
	// Fire rollback failed event
}
