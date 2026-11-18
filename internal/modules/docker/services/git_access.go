package services

import (
	"context"
	"errors"
	"github.com/hibiken/asynq"
	"github.com/kkz6/launch-go/internal/modules/docker/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/git/access"
)

func newAuthorizedBootstrapTask(ctx context.Context, s *BaseService, kind, id, teamID string, config map[string]any, rotate bool, baseURL string) (*asynq.Task, error) {
	snapshot, err := gitaccess.Bind(ctx, s.DB(), s.GitProviders(), teamID, config, access.ActionsBootstrap)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("GitHub Actions requires a selected repository")
	}
	return jobs.NewGHABootstrapWorkflowTask(kind, id, rotate, baseURL, gitaccess.PinnedSnapshots(config, snapshot)...)
}
