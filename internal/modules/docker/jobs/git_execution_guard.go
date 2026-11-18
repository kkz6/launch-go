package jobs

import (
	"context"
	"errors"
	"github.com/kkz6/launch-go/internal/modules/docker/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/registryaccess"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func (d *JobDeps) guardGitTask(task taskrunner.Task, teamID string, config map[string]any, snapshot *access.Snapshot, dependencies []access.Snapshot, deployment *models.Deployment, registryReference string) error {
	guarded, ok := task.(interface {
		SetBeforeExecution(func(context.Context) error)
	})
	if !ok {
		return errors.New("deployment task does not support repository authorization checks")
	}
	guarded.SetBeforeExecution(func(ctx context.Context) error {
		if _, err := gitaccess.Resolve(ctx, d.DB, d.GitProviders, teamID, config, access.Clone, snapshot, true); err != nil {
			return err
		}
		if _, err := gitaccess.ResolveDependencies(ctx, d.DB, d.GitProviders, teamID, config, dependencies, true); err != nil {
			return err
		}
		if registryReference != "" {
			if _, err := gitaccess.Resolve(ctx, d.DB, d.GitProviders, teamID, config, access.ActionsWrite, snapshot, true); err != nil {
				return err
			}
			packageName, _ := config["gha_image_repository"].(string)
			if _, err := registryaccess.Redeem(ctx, d.DB, registryReference, deployment, packageName, snapshot); err != nil {
				return err
			}
		}
		return nil
	})
	return nil
}
