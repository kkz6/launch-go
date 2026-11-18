package services

import (
	"context"
	"encoding/json"
	"github.com/kkz6/launch-go/internal/modules/docker/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"gorm.io/gorm"
)

func (s *BaseService) queueGHACleanup(ctx context.Context, config map[string]any) error {
	raw, err := json.Marshal(config["gha_cleanup_manifest_ids"])
	if err != nil {
		return err
	}
	var ids []string
	if err = json.Unmarshal(raw, &ids); err != nil {
		return err
	}
	for _, id := range ids {
		if err := access.ActivateCleanupManifest(ctx, s.DB(), id); err != nil {
			return err
		}
		task, err := jobs.NewGHACleanupTask(id)
		if err != nil {
			return err
		}
		if err = s.EnqueueTask(task); err != nil {
			return err
		}
	}
	return nil
}

type ghaWorkloadLockKey struct{}

func (s *BaseService) withWorkflowMutation(ctx context.Context, kind, id string, work func(context.Context) error) error {
	return gitaccess.WithWorkloadLock(ctx, s.DB(), kind, id, func(_ *gorm.DB) error { return work(context.WithValue(ctx, ghaWorkloadLockKey{}, true)) })
}
func workflowMutationLocked(ctx context.Context) bool {
	held, _ := ctx.Value(ghaWorkloadLockKey{}).(bool)
	return held
}
