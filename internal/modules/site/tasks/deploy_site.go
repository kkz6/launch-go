package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// DeploySite deploys a site with or without zero-downtime
type DeploySite struct {
	BaseSiteTask
	deployment *models.Deployment
}

// NewDeploySite creates a new DeploySite task
func NewDeploySite(site *models.Site, deployment *models.Deployment) *DeploySite {
	templateName := "site/deploy-site"
	if site.ZeroDowntimeDeployment {
		templateName = "site/deploy-site-without-downtime"
	}

	task := &DeploySite{
		BaseSiteTask: BaseSiteTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: templateName,
				TaskTimeout:  15 * time.Minute,
			},
			site: site,
		},
		deployment: deployment,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// Data returns the template data
func (t *DeploySite) Data() map[string]interface{} {
	return map[string]interface{}{
		"Site":                t.siteData(),
		"Deployment":          t.deploymentData(),
		"RepositoryDirectory": t.repositoryDirectory(),
		"ReleaseDirectory":    t.releaseDirectory(),
		"ReleasesDirectory":   t.releasesDirectory(),
		"SharedDirectory":     t.sharedDirectory(),
		"CurrentDirectory":    t.currentDirectory(),
		"SharedDirectories":   t.site.GetSharedDirectories(),
		"SharedFiles":         t.site.GetSharedFiles(),
		"WritableDirectories": t.site.GetWriteableDirectories(),
		"HasQueues":           t.hasQueues(),
		"AppName":             "", // TODO: Get from config
	}
}

func (t *DeploySite) siteData() map[string]interface{} {
	return map[string]interface{}{
		"Path":                         t.site.Path,
		"Type":                         t.site.Type,
		"User":                         t.site.User,
		"Address":                      t.site.Address,
		"PhpVersion":                   t.getPhpVersion(),
		"RepositoryBranch":             t.site.GetRepositoryBranch(),
		"ZeroDowntimeDeployment":       t.site.ZeroDowntimeDeployment,
		"DeployKeyPrivate":             t.getDeployKeyPrivate(),
		"DeploymentReleasesRetention":  t.site.DeploymentReleasesRetention,
		"ApplicationDirectory":         t.site.GetApplicationDirectory(),
		"HookBeforeUpdatingRepository": t.getHook(t.site.HookBeforeUpdatingRepository),
		"HookAfterUpdatingRepository":  t.getHook(t.site.HookAfterUpdatingRepository),
		"HookBeforeMakingCurrent":      t.getHook(t.site.HookBeforeMakingCurrent),
		"HookAfterMakingCurrent":       t.getHook(t.site.HookAfterMakingCurrent),
	}
}

func (t *DeploySite) getPhpVersion() string {
	if t.site.PhpVersion != nil {
		return *t.site.PhpVersion
	}
	return "8.3"
}

func (t *DeploySite) getDeployKeyPrivate() string {
	if t.site.DeployKeyPrivate != nil {
		return *t.site.DeployKeyPrivate
	}
	return ""
}

func (t *DeploySite) getHook(hook *string) string {
	if hook != nil {
		return *hook
	}
	return ""
}

func (t *DeploySite) deploymentData() map[string]interface{} {
	return map[string]interface{}{
		"ID": t.deployment.ID,
	}
}

func (t *DeploySite) repositoryDirectory() string {
	if t.site.ZeroDowntimeDeployment {
		return fmt.Sprintf("%s/releases/%s", t.site.Path, t.deployment.ID)
	}
	return fmt.Sprintf("%s/repository", t.site.Path)
}

func (t *DeploySite) releaseDirectory() string {
	return fmt.Sprintf("%s/releases/%s", t.site.Path, t.deployment.ID)
}

func (t *DeploySite) releasesDirectory() string {
	return fmt.Sprintf("%s/releases", t.site.Path)
}

func (t *DeploySite) sharedDirectory() string {
	return fmt.Sprintf("%s/shared", t.site.Path)
}

func (t *DeploySite) currentDirectory() string {
	return fmt.Sprintf("%s/current", t.site.Path)
}

func (t *DeploySite) hasQueues() bool {
	// TODO: Check if site has queue workers
	return false
}

func (t *DeploySite) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update deployment status to finished
	// Set finished_at timestamp
	// Fire DeploymentSuccessful event
}

func (t *DeploySite) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update deployment status to failed
	// Fire DeploymentFailed event
}
