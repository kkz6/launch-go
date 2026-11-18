package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Post-deployment actions retain the originating deployment's authorization.
// Explicit manual operations omit deploymentID and keep their existing policy.
type postDeploymentCheck func(context.Context) error

func (check postDeploymentCheck) validate(ctx context.Context) error {
	if check == nil {
		return nil
	}
	return check(ctx)
}
func (check postDeploymentCheck) attach(task taskrunner.Task) error {
	if check == nil {
		return nil
	}
	guarded, ok := task.(interface {
		SetBeforeExecution(func(context.Context) error)
	})
	if !ok {
		return fmt.Errorf("post-deployment task cannot verify repository authorization")
	}
	guarded.SetBeforeExecution(check)
	return nil
}

func newPostDeploymentCheck(ctx context.Context, deps *JobDeps, site *models.Site, deploymentID string) (postDeploymentCheck, error) {
	if deploymentID == "" {
		return nil, nil
	}
	if deps == nil || deps.DB == nil || site == nil || site.ID == "" || site.TeamID == "" {
		return nil, access.ErrStaleAccess
	}
	siteID, teamID, serverID := site.ID, site.TeamID, site.ServerID
	check := postDeploymentCheck(func(ctx context.Context) error {
		var current models.Site
		if err := deps.DB.WithContext(ctx).First(&current, "id = ? AND team_id = ? AND server_id = ?", siteID, teamID, serverID).Error; err != nil {
			return access.ErrStaleAccess
		}
		var deployment models.Deployment
		if err := deps.DB.WithContext(ctx).First(&deployment, "id = ? AND site_id = ? AND team_id = ?", deploymentID, siteID, teamID).Error; err != nil {
			return access.ErrStaleAccess
		}
		grant, err := siteaccess.ForDeployment(ctx, deps.DB, deps.ProviderFactory, &current, &deployment, access.Clone)
		if err != nil {
			return err
		}
		if grant == nil {
			return nil
		}
		if err := grant.Validate(ctx); err != nil {
			return err
		}
		raw, _ := deployment.GitAccess["dependencies"].(map[string]any)
		for id, value := range raw {
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			var expected access.Snapshot
			if err = json.Unmarshal(data, &expected); err != nil {
				return err
			}
			dependency, err := siteaccess.ResolveDependency(ctx, deps.DB, deps.ProviderFactory, teamID, id, &expected)
			if err != nil {
				return err
			}
			if err = dependency.Validate(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err := check(ctx); err != nil {
		return nil, err
	}
	return check, nil
}
