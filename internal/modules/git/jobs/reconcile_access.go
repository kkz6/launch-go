package jobs

import (
	"context"
	"errors"
	"github.com/hibiken/asynq"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"gorm.io/gorm"
	"strconv"
	"time"
)

const TypeReconcileGitHubAccess = "git:reconcile_access"

type ReconcileGitHubAccessPayload struct{}
type ReconcileGitHubAccessJob struct{ Deps *JobDeps }

func NewReconcileGitHubAccessJob(_ ReconcileGitHubAccessPayload) pkgjobs.Handler {
	return &ReconcileGitHubAccessJob{Deps: deps}
}
func NewReconcileGitHubAccessTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypeReconcileGitHubAccess, ReconcileGitHubAccessPayload{}, asynq.MaxRetry(3), asynq.Unique(time.Minute))
}
func (j *ReconcileGitHubAccessJob) Handle(ctx context.Context) error {
	resolver := access.NewResolver(j.Deps.DB, j.Deps.ProviderFactory)
	var connections []models.SourceControl
	if err := j.Deps.DB.WithContext(ctx).Where("provider = ? AND status = ?", "github", "active").Find(&connections).Error; err != nil {
		return err
	}
	var failures []error
	for _, sc := range connections {
		var authorization models.GitHubAuthorization
		if sc.GitHubAuthorizationID == nil || j.Deps.DB.WithContext(ctx).First(&authorization, "id = ?", *sc.GitHubAuthorizationID).Error != nil {
			continue
		}
		if access.RequireManager(ctx, j.Deps.DB, sc.TeamID, authorization.UserID) != nil {
			err := j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				update := tx.Model(&models.SourceControl{}).Where("id = ? AND generation = ? AND status = ?", sc.ID, sc.Generation, "active").Updates(map[string]any{"status": "reconnect_required", "generation": gorm.Expr("generation + 1")})
				if update.Error != nil {
					return update.Error
				}
				// Reconnecting may have replaced the sponsor while the manager
				// check was in flight. Old evidence cannot revoke the new grant.
				if update.RowsAffected == 0 {
					return nil
				}
				if err := access.ActivateCleanup(ctx, tx, sc.ID, ""); err != nil {
					return err
				}
				return tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ? AND status IN ?", sc.ID, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
			})
			if err != nil {
				failures = append(failures, err)
			}
			continue
		}
		var repos []models.SourceControlRepository
		if err := j.Deps.DB.WithContext(ctx).Where("source_control_id = ? AND selected = ? AND available = ?", sc.ID, true, true).Find(&repos).Error; err != nil {
			failures = append(failures, err)
			continue
		}
		for _, repo := range repos {
			if _, err := resolver.Resolve(ctx, access.Request{TeamID: sc.TeamID, SourceControlID: sc.ID, RepositoryID: strconv.FormatUint(repo.ID, 10), Operation: access.Read}); err != nil {
				j.Deps.Logger.Warn().Str("source_control_id", sc.ID).Uint64("repository_id", repo.ID).Msg("Repository access requires review")
			}
		}
	}
	retiredCleanup := j.Deps.DB.Model(&models.GitHubCleanupManifest{}).Select("id").Where("expires_at <= ? OR status IN ?", time.Now(), []string{"completed", "failed_manual"})
	if err := j.Deps.DB.WithContext(ctx).Model(&models.GitHubCredentialLease{}).Where("cleanup_manifest_id IN (?) AND status IN ?", retiredCleanup, []string{"issuing", "released"}).Update("status", "revoke_pending").Error; err != nil {
		failures = append(failures, err)
	}
	if err := resolver.RevokePending(ctx); err != nil {
		failures = append(failures, err)
	}
	// Expired bearer material has no remaining purpose and is removed even when
	// the original worker died before recording successful release/revocation.
	if err := j.Deps.DB.WithContext(ctx).Model(&models.GitHubCredentialLease{}).Where("expires_at < ? AND token IS NOT NULL", time.Now()).Updates(map[string]any{"token": nil, "status": "expired"}).Error; err != nil {
		failures = append(failures, err)
	}
	var cleanups []models.GitHubCleanupManifest
	if err := j.Deps.DB.WithContext(ctx).Where("status IN ? AND expires_at > ?", []string{"pending", "failed_cleanup"}, time.Now()).Find(&cleanups).Error; err != nil {
		failures = append(failures, err)
	}
	for _, cleanup := range cleanups {
		task, err := pkgjobs.Task("docker:gha_cleanup", map[string]string{"cleanup_id": cleanup.ID}, asynq.Unique(time.Minute), asynq.MaxRetry(3))
		if err == nil {
			if j.Deps.Queue == nil {
				err = errors.New("cleanup queue unavailable")
			} else {
				err = j.Deps.DispatchTask(task)
			}
		}
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			failures = append(failures, err)
		}
	}
	var receipts []string
	if err := j.Deps.DB.WithContext(ctx).Model(&models.GitHubWebhookIntent{}).Where("dispatched_at IS NULL").Distinct().Pluck("delivery_id", &receipts).Error; err != nil {
		failures = append(failures, err)
	}
	dispatcher := ProcessGitWebhookJob{Deps: j.Deps}
	for _, id := range receipts {
		if err := dispatcher.dispatchGitHubIntents(ctx, id); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
func (j *ReconcileGitHubAccessJob) Failed(_ context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("GitHub access reconciliation failed")
}
