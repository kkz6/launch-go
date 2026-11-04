package jobs

import (
	"context"
	"fmt"
	"regexp"

	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

var (
	deploymentReleaseNamePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	legacyDeploymentReleasePattern = regexp.MustCompile(`(?m)^DEPLOYMENT_KEEP="([0-9]{14})"$`)
)

// resolveDeploymentRelease uses the recorded release, or recovers the exact
// directory name from a legacy deployment's saved script. Creation time cannot
// identify a release because a deployment can wait in the queue before running.
func resolveDeploymentRelease(ctx context.Context, db *gorm.DB, deployment *models.Deployment, serverID string) (string, error) {
	if deployment.ReleaseName != nil {
		if !deploymentReleaseNamePattern.MatchString(*deployment.ReleaseName) {
			return "", fmt.Errorf("release information unavailable: invalid release name for deployment %s", deployment.ID)
		}
		return *deployment.ReleaseName, nil
	}
	if deployment.TaskID == nil || *deployment.TaskID == "" {
		return "", fmt.Errorf("release information unavailable: deployment %s has no saved task", deployment.ID)
	}

	var task servermodels.Task
	if err := db.WithContext(ctx).
		Select("id", "server_id", "site_id", "script").
		Where("id = ? AND server_id = ?", *deployment.TaskID, serverID).
		First(&task).Error; err != nil {
		return "", fmt.Errorf("release information unavailable: load deployment task: %w", err)
	}
	if task.SiteID != nil && *task.SiteID != deployment.SiteID {
		return "", fmt.Errorf("release information unavailable: saved task belongs to a different site")
	}
	matches := legacyDeploymentReleasePattern.FindAllStringSubmatch(string(task.Script), -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("release information unavailable: saved task has no unambiguous release name")
	}
	releaseName := matches[0][1]
	result := db.WithContext(ctx).Model(&models.Deployment{}).
		Where("id = ? AND site_id = ? AND team_id = ? AND release_name IS NULL", deployment.ID, deployment.SiteID, deployment.TeamID).
		Update("release_name", releaseName)
	if result.Error != nil {
		return "", fmt.Errorf("persist recovered deployment release: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return "", fmt.Errorf("release information unavailable: deployment changed while recovering its release")
	}
	deployment.ReleaseName = &releaseName
	return releaseName, nil
}
