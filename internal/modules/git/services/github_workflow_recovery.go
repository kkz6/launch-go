package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const githubWorkflowRecoveryTask = "docker:gha_bootstrap_workflow"

// CaptureGitHubWorkflowRecovery records replacements for unusable workflow
// bindings inside the transaction that restores the explicit repository grants.
// It never updates workload snapshots: only a successful bootstrap can bind new
// callbacks, and previously queued deployments keep their original authority.
func CaptureGitHubWorkflowRecovery(ctx context.Context, tx *gorm.DB, factory *providers.ProviderFactory, receiptID string, sc *models.SourceControl, baseURL string) ([]string, error) {
	return captureGitHubWorkflowRecovery(ctx, tx, factory, receiptID, sc, baseURL, true)
}

// CaptureMissingGitHubWorkflowRecovery repairs connections restored before the
// recovery trigger existed. A terminal failure for the same authority waits for
// an explicit Save, so periodic reconciliation cannot repeatedly rotate secrets.
func CaptureMissingGitHubWorkflowRecovery(ctx context.Context, tx *gorm.DB, factory *providers.ProviderFactory, receiptID string, sc *models.SourceControl, baseURL string) ([]string, error) {
	return captureGitHubWorkflowRecovery(ctx, tx, factory, receiptID, sc, baseURL, false)
}

func captureGitHubWorkflowRecovery(ctx context.Context, tx *gorm.DB, factory *providers.ProviderFactory, receiptID string, sc *models.SourceControl, baseURL string, retryFailed bool) ([]string, error) {
	if sc == nil || sc.Status != "active" || string(sc.Provider) != "github" {
		return nil, nil
	}
	var intents []string
	for _, table := range []struct{ name, kind string }{{"docker_applications", "application"}, {"docker_composes", "compose"}} {
		if !tx.Migrator().HasTable(table.name) {
			continue
		}
		var workloads []struct {
			ID                 string
			TeamID             string
			SourceConfig       dbtype.JSONMap `gorm:"type:json"`
			GHADeployTokenHash string
		}
		if err := tx.WithContext(ctx).Table(table.name).Select("id,team_id,source_config,gha_deploy_token_hash").Where("team_id = ? AND build_location = ? AND deleted_at IS NULL", sc.TeamID, "github_actions").Find(&workloads).Error; err != nil {
			return nil, err
		}
		for _, workload := range workloads {
			source := workload.SourceConfig
			if source.GetString("source_control_id") != sc.ID {
				continue
			}
			// GitHub Actions does not support credentials for private dependencies;
			// matching the ordinary bootstrap rules avoids quietly broadening access.
			if hasGitHubWorkflowDependencies(source) {
				continue
			}
			repository := source.GetString("repo")
			if owner := source.GetString("owner"); owner != "" && !strings.Contains(repository, "/") {
				repository = owner + "/" + repository
			}
			request := access.Request{TeamID: sc.TeamID, SourceControlID: sc.ID, RepositoryID: source.GetString("repository_id"), Repository: repository, Operation: access.ActionsBootstrap}
			grant, err := access.NewResolver(tx, factory).Resolve(ctx, request)
			if errors.Is(err, access.ErrAccessDenied) || errors.Is(err, access.ErrStaleAccess) {
				continue
			}
			if err != nil {
				return nil, err
			}
			current := grant.Snapshot()
			if healthyGitHubWorkflow(source, workload.GHADeployTokenHash, current) {
				continue
			}
			if err := validateGitHubRecoveryURL(baseURL); err != nil {
				return nil, err
			}
			// The same pending recovery is reused across repeated Save clicks. A new
			// grant, prior binding, or terminal failure requires a new remote namespace.
			failedBinding := ""
			if source.GetString("gha_recovery_status") == "failed" {
				failedBinding = source.GetString("gha_recovery_binding_id")
			}
			if !retryFailed && failedBinding != "" {
				var failed models.GitHubWebhookIntent
				err := tx.WithContext(ctx).Where("task_type = ? AND payload ->> 'binding_id' = ?", githubWorkflowRecoveryTask, failedBinding).First(&failed).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				if err != nil {
					return nil, err
				}
				snapshot, err := access.SnapshotFromMap(failed.Payload)
				if err != nil || snapshot == nil || *snapshot == current {
					continue
				}
			}

			encoded, err := json.Marshal([]any{table.kind, workload.ID, current, source["gha_access"], source.GetString("gha_binding_id"), failedBinding})
			if err != nil {
				return nil, err
			}
			digest := sha256.Sum256(encoded)
			key := hex.EncodeToString(digest[:])
			intent := models.GitHubWebhookIntent{ID: key, DeliveryID: receiptID, TaskType: githubWorkflowRecoveryTask, Payload: dbtype.JSONMap{
				"workload_kind": table.kind, "workload_id": workload.ID, "git_access": current, "binding_id": util.NewULID(), "launch_base_url": strings.TrimRight(baseURL, "/"), "recovery": true, "rotate_token": true,
			}}
			if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&intent).Error; err != nil {
				return nil, err
			}
			intents = append(intents, key)
		}
	}
	return intents, nil
}

func hasGitHubWorkflowDependencies(source dbtype.JSONMap) bool {
	for _, key := range []string{"dependency_repository_ids", "git_dependencies"} {
		if raw, exists := source[key]; exists && raw != nil {
			data, err := json.Marshal(raw)
			if err != nil {
				return true
			}
			var dependencies []json.RawMessage
			if json.Unmarshal(data, &dependencies) != nil || len(dependencies) > 0 {
				return true
			}
		}
	}
	return false
}

func healthyGitHubWorkflow(source dbtype.JSONMap, tokenHash string, current access.Snapshot) bool {
	if source.GetString("repository_id") != current.RepositoryID || tokenHash == "" || source.GetString("gha_workflow_sha") == "" || source.GetString("gha_workflow_path") == "" || source.GetString("gha_deploy_secret_name") == "" {
		return false
	}
	if status := source.GetString("gha_install_status"); status != "" && status != "ok" {
		return false
	}
	selected, err := access.SnapshotFromMap(source)
	if err != nil || selected == nil || *selected != current {
		return false
	}
	workflow, err := access.SnapshotFromMap(map[string]any{"git_access": source["gha_access"]})
	return err == nil && workflow != nil && *workflow == current
}
func validateGitHubRecoveryURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("GitHub workflow recovery requires the configured application URL")
	}
	return nil
}

// DispatchGitHubIntents is shared by synchronous selection and durable replay.
// Queue failure leaves every undispatched intent available to reconciliation.
func DispatchGitHubIntents(ctx context.Context, db *gorm.DB, receiptID string, enqueue func(*asynq.Task) error) error {
	var ids []string
	if err := db.WithContext(ctx).Model(&models.GitHubWebhookIntent{}).Where("delivery_id = ? AND dispatched_at IS NULL", receiptID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	return DispatchGitHubIntentIDs(ctx, db, ids, enqueue)
}
func DispatchGitHubIntentIDs(ctx context.Context, db *gorm.DB, ids []string, enqueue func(*asynq.Task) error) error {
	if len(ids) == 0 {
		return nil
	}
	var intents []models.GitHubWebhookIntent
	if err := db.WithContext(ctx).Where("id IN ? AND dispatched_at IS NULL", ids).Find(&intents).Error; err != nil {
		return err
	}
	if len(intents) > 0 && enqueue == nil {
		return errors.New("GitHub recovery dispatch queue unavailable")
	}
	for _, intent := range intents {
		taskType := intent.TaskType
		if taskType == "" {
			taskType = "site:create_deployment"
		}
		if taskType != "site:create_deployment" && taskType != githubWorkflowRecoveryTask {
			return fmt.Errorf("unsupported GitHub intent type %q", taskType)
		}
		task, err := pkgjobs.TaskWithID(taskType, intent.Payload, "github-intent:"+intent.ID, asynq.MaxRetry(10))
		if err != nil {
			return err
		}
		if err = enqueue(task); err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		if err = db.WithContext(ctx).Model(&models.GitHubWebhookIntent{}).Where("id = ? AND dispatched_at IS NULL", intent.ID).Update("dispatched_at", time.Now()).Error; err != nil {
			return err
		}
	}
	return nil
}
