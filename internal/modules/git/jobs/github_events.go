package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	sitejobs "github.com/kkz6/launch-go/internal/modules/site/jobs"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func eventHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// Recipient failures are retried without rolling back other accounts' intents.
// Database failures still abort the enclosing transaction normally.
type gitHubFanoutRetry struct{ err error }

func (e *gitHubFanoutRetry) Error() string { return e.err.Error() }
func (e *gitHubFanoutRetry) Unwrap() error { return e.err }

func (j *ProcessGitWebhookJob) processVerifiedGitHubEvent(ctx context.Context, data map[string]any) error {
	if j.Payload.Event == "" || j.Payload.DeliveryID == "" {
		return fmt.Errorf("missing GitHub event or delivery identity")
	}
	if j.Payload.Event == "ping" {
		return nil
	}
	switch j.Payload.Event {
	case "push", "installation", "installation_repositories", "github_app_authorization", "repository":
	default:
		return nil
	}
	provider, err := access.GitHubProvider(j.Deps.ProviderFactory)
	if err != nil {
		return err
	}
	receiptID := eventHash(provider.Config().AppID + ":" + j.Payload.DeliveryID)
	action, _ := data["action"].(string)
	installation, _ := data["installation"].(map[string]any)
	installationID := providers.ExtractFloatID(installation, "id")
	var refreshedPermissions string
	if j.Payload.Event == "installation" && action == "new_permissions_accepted" && installationID != "" {
		current, err := provider.GetInstallation(ctx, installationID)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(current.Permissions)
		if err != nil {
			return err
		}
		refreshedPermissions = string(encoded)
	}
	var fanoutErr error
	err = j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receipt := models.GitHubWebhookDelivery{ID: receiptID, Event: j.Payload.Event, PayloadHash: eventHash(j.Payload.Payload)}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&receipt, "id = ?", receiptID).Error; err != nil {
			return err
		}
		if receipt.Event != j.Payload.Event || receipt.PayloadHash != eventHash(j.Payload.Payload) {
			return fmt.Errorf("delivery identity reused with different event or payload")
		}
		if receipt.ProcessedAt != nil {
			return nil
		}
		switch j.Payload.Event {
		case "push":
			if err := j.captureGitHubPush(ctx, tx, receiptID, installationID, data); err != nil {
				var retry *gitHubFanoutRetry
				if !errors.As(err, &retry) {
					return err
				}
				fanoutErr = err
			}
		case "installation":
			// Installation events must not be inferred from another event's action.
			if data["repository"] != nil {
				return fmt.Errorf("unexpected repository in installation event")
			}
			if installationID == "" {
				return fmt.Errorf("missing installation identity")
			}
			switch action {
			case "deleted", "suspend", "unsuspend":
				if err := ensureWebhookInstallation(tx, provider.Config().AppID, installationID); err != nil {
					return err
				}
				status := "suspended"
				if action == "deleted" {
					status = "deleted"
				}
				if action == "unsuspend" {
					status = "active"
				}
				if err := tx.Model(&models.GitHubInstallation{}).Where("host = ? AND app_id = ? AND installation_id = ?", "github.com", provider.Config().AppID, installationID).Updates(map[string]any{"status": status, "generation": gorm.Expr("generation + 1")}).Error; err != nil {
					return err
				}
				if err := invalidateInstallation(tx, installationID); err != nil {
					return err
				}
			case "new_permissions_accepted":
				if err := tx.Model(&models.GitHubInstallation{}).Where("host = ? AND app_id = ? AND installation_id = ?", "github.com", provider.Config().AppID, installationID).Update("permissions", refreshedPermissions).Error; err != nil {
					return err
				}
				if err := j.capturePermissionRecovery(ctx, tx, receiptID, installationID); err != nil {
					var retry *gitHubFanoutRetry
					if !errors.As(err, &retry) {
						return err
					}
					fanoutErr = err
				}
			}
		case "installation_repositories":
			if action == "removed" {
				removed, _ := data["repositories_removed"].([]any)
				for _, item := range removed {
					repo, _ := item.(map[string]any)
					if id := providers.ExtractFloatID(repo, "id"); id != "" {
						if err := invalidateRepository(tx, provider.Config().AppID, installationID, id); err != nil {
							return err
						}
					}
				}
			}
			// Added repositories only become candidates on a later authenticated sync.
		case "repository":
			if action == "deleted" || action == "transferred" || action == "renamed" || action == "privatized" {
				repo, _ := data["repository"].(map[string]any)
				if id := providers.ExtractFloatID(repo, "id"); id != "" {
					if err := invalidateRepository(tx, provider.Config().AppID, installationID, id); err != nil {
						return err
					}
				}
			}
		case "github_app_authorization":
			if action == "revoked" {
				sender, _ := data["sender"].(map[string]any)
				id := providers.ExtractFloatID(sender, "id")
				if id == "" {
					return fmt.Errorf("missing revoked GitHub identity")
				}
				var ids []string
				if err := tx.Model(&models.GitHubAuthorization{}).Where("host = ? AND app_id = ? AND git_hub_user_id = ?", "github.com", provider.Config().AppID, id).Pluck("id", &ids).Error; err != nil {
					return err
				}
				now := time.Now()
				if err := tx.Model(&models.GitHubAuthorization{}).Where("id IN ?", ids).Updates(map[string]any{"revoked_at": now, "generation": gorm.Expr("generation + 1"), "access_token": nil, "refresh_token": nil}).Error; err != nil {
					return err
				}
				if err := tx.Model(&models.SourceControl{}).Where("git_hub_authorization_id IN ?", ids).Updates(map[string]any{"status": "reconnect_required", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
					return err
				}
				var scIDs []string
				if err := tx.Model(&models.SourceControl{}).Where("git_hub_authorization_id IN ?", ids).Pluck("id", &scIDs).Error; err != nil {
					return err
				}
				for _, id := range scIDs {
					if err := access.ActivateCleanup(ctx, tx, id, ""); err != nil {
						return err
					}
				}
				if err := tx.Model(&models.GitHubCredentialLease{}).Where("authorization_id IN ? AND status IN ?", ids, []string{"issuing", "released"}).Update("status", "revoke_pending").Error; err != nil {
					return err
				}
			}
		}
		if fanoutErr != nil {
			return nil // Commit successful recipients and retry only missing ones.
		}
		now := time.Now()
		return tx.Model(&receipt).Update("processed_at", now).Error
	})
	if err != nil {
		return err
	}
	// Publish authorized work independently of credential cleanup. A failed
	// revocation in another account must not block this delivery's deployments.
	dispatchErr := errors.Join(fanoutErr, j.dispatchGitHubIntents(ctx, receiptID))
	revokesAccess := false
	switch j.Payload.Event {
	case "installation":
		revokesAccess = action == "deleted" || action == "suspend" || action == "unsuspend"
	case "installation_repositories":
		revokesAccess = action == "removed"
	case "repository":
		revokesAccess = action == "deleted" || action == "transferred" || action == "renamed" || action == "privatized"
	case "github_app_authorization":
		revokesAccess = action == "revoked"
	}
	if !revokesAccess {
		return dispatchErr
	}
	// Revocation is durable and also retried by the recurring reconciler.
	return errors.Join(dispatchErr, access.NewResolver(j.Deps.DB, j.Deps.ProviderFactory).RevokePending(ctx))
}

func invalidateInstallation(tx *gorm.DB, id string) error {
	var ids []string
	if err := tx.Model(&models.SourceControl{}).Where("provider = ? AND installation_id = ?", "github", id).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.SourceControl{}).Where("id IN ?", ids).Updates(map[string]any{"status": "reconnect_required", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.SourceControlRepository{}).Where("source_control_id IN ?", ids).Updates(map[string]any{"available": false, "grant_version": gorm.Expr("grant_version + 1"), "verified_at": nil}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := access.ActivateCleanup(tx.Statement.Context, tx, id, ""); err != nil {
			return err
		}
	}
	return tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id IN ? AND status IN ?", ids, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
}

// Lifecycle evidence can arrive before the first local connection is saved.
// Keep it even when no source-control/grant rows exist yet.
func ensureWebhookInstallation(tx *gorm.DB, appID, installationID string) error {
	installation := models.GitHubInstallation{Host: "github.com", AppID: appID, InstallationID: installationID, Status: "active", Generation: 1, RepositoryRevision: 1}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "host"}, {Name: "app_id"}, {Name: "installation_id"}}, DoNothing: true}).Create(&installation).Error
}

func invalidateRepository(tx *gorm.DB, appID, installationID, repositoryID string) error {
	if installationID == "" {
		return fmt.Errorf("missing installation identity")
	}
	if err := ensureWebhookInstallation(tx, appID, installationID); err != nil {
		return err
	}
	// Fence selections of repositories with no local grant yet as well as
	// existing grants, without expiring unrelated workload snapshots.
	if err := tx.Model(&models.GitHubInstallation{}).Where("host = ? AND app_id = ? AND installation_id = ?", "github.com", appID, installationID).Update("repository_revision", gorm.Expr("repository_revision + 1")).Error; err != nil {
		return err
	}
	connections := tx.Model(&models.SourceControl{}).Select("id").Where("provider = ? AND installation_id = ?", "github", installationID)
	var ids []uint64
	if err := tx.Model(&models.SourceControlRepository{}).Where("source_control_id IN (?) AND git_hub_repository_id = ?", connections, repositoryID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.SourceControlRepository{}).Where("id IN ?", ids).Updates(map[string]any{"available": false, "grant_version": gorm.Expr("grant_version + 1"), "verified_at": nil}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		var repo models.SourceControlRepository
		if err := tx.First(&repo, "id = ?", id).Error; err != nil {
			return err
		}
		if err := access.ActivateCleanup(tx.Statement.Context, tx, repo.SourceControlID, fmt.Sprint(id)); err != nil {
			return err
		}
	}
	return tx.Model(&models.GitHubCredentialLease{}).Where("repository_id IN ? AND status IN ?", ids, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
}

func (j *ProcessGitWebhookJob) captureGitHubPush(ctx context.Context, tx *gorm.DB, receiptID, installationID string, data map[string]any) error {
	ref, _ := data["ref"].(string)
	deleted, _ := data["deleted"].(bool)
	if !strings.HasPrefix(ref, "refs/heads/") || deleted {
		return nil
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	repo, _ := data["repository"].(map[string]any)
	repoID := providers.ExtractFloatID(repo, "id")
	if repoID == "" || installationID == "" {
		return fmt.Errorf("missing repository or installation identity")
	}
	var sites []sitemodels.Site
	err := tx.Model(&sitemodels.Site{}).Select("sites.*").
		Joins("JOIN source_controls sc ON sc.id = sites.source_control_id AND sc.team_id = sites.team_id").
		Joins("JOIN source_control_repositories r ON r.id = sites.source_control_repositories_id AND r.source_control_id = sc.id").
		Where("sc.provider = ? AND sc.installation_id = ? AND sc.status = ? AND r.git_hub_repository_id = ? AND r.selected = ? AND r.available = ? AND COALESCE(NULLIF(sites.repository_branch, ''), 'main') = ? AND sites.auto_deployment = ?", "github", installationID, "active", repoID, true, true, branch, true).Find(&sites).Error
	if err != nil {
		return err
	}
	provider, err := j.Deps.ProviderFactory.GetProvider(providers.GitProviderGitHub)
	if err != nil {
		return err
	}
	commit := provider.GetCommitData(data)
	if commit == nil || commit.Branch != branch {
		return nil
	}
	var failures []error
	for _, site := range sites {
		key := eventHash(receiptID + ":" + site.ID)
		var existing int64
		if err := tx.Model(&models.GitHubWebhookIntent{}).Where("id = ?", key).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			continue
		}
		authorization, err := siteaccess.Capture(ctx, tx, j.Deps.ProviderFactory, &site)
		if err != nil {
			if errors.Is(err, access.ErrAccessDenied) || errors.Is(err, access.ErrStaleAccess) {
				continue
			}
			failures = append(failures, err)
			continue
		} // Denied recipients receive no deployment intent.
		payload := sitejobs.CreateDeploymentPayload{SiteID: site.ID, Branch: &branch, CommitData: commit.ToMap(), GitAccess: authorization, WebhookKey: key}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		var body dbtype.JSONMap
		if err := json.Unmarshal(encoded, &body); err != nil {
			return err
		}
		intent := models.GitHubWebhookIntent{ID: key, DeliveryID: receiptID, SiteID: site.ID, Payload: body}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&intent).Error; err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		return &gitHubFanoutRetry{err: errors.Join(failures...)}
	}
	return nil
}

func (j *ProcessGitWebhookJob) dispatchGitHubIntents(ctx context.Context, receiptID string) error {
	var enqueue func(*asynq.Task) error
	if j.Deps.Queue != nil {
		enqueue = func(task *asynq.Task) error { return j.Deps.DispatchTask(task) }
	}
	return services.DispatchGitHubIntents(ctx, j.Deps.DB, receiptID, enqueue)
}

func (j *ProcessGitWebhookJob) capturePermissionRecovery(ctx context.Context, tx *gorm.DB, receiptID, installationID string) error {
	var connections []models.SourceControl
	if err := tx.Where("provider = ? AND installation_id = ? AND status = ?", "github", installationID, "active").Find(&connections).Error; err != nil {
		return err
	}
	var failures []error
	for i := range connections {
		if _, err := services.CaptureGitHubWorkflowRecovery(ctx, tx, j.Deps.ProviderFactory, receiptID, &connections[i], j.Deps.AppURL); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return &gitHubFanoutRetry{err: errors.Join(failures...)}
	}
	return nil
}
