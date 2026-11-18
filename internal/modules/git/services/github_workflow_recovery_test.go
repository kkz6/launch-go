package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createRecoveryTables(t *testing.T, f *gitFixture) {
	t.Helper()
	for _, table := range []string{"docker_applications", "docker_composes"} {
		require.NoError(t, f.db.Exec("CREATE TABLE "+table+" (id TEXT PRIMARY KEY, team_id TEXT, source_config TEXT, gha_deploy_token_hash TEXT, build_location TEXT, deleted_at timestamp)").Error)
	}
}
func recoverySource(t *testing.T, f *gitFixture, team, connection, repository string) dbtype.JSONMap {
	t.Helper()
	grant, err := access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(context.Background(), access.Request{TeamID: team, SourceControlID: connection, Repository: repository, Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	source := dbtype.JSONMap{"source_control_id": connection, "repository_id": fmt.Sprint(grant.Repository.ID), "repo": grant.CloneURL, "gha_access": grant.Snapshot(), "gha_workflow_sha": "old-sha", "gha_workflow_path": ".github/workflows/old.yml", "gha_deploy_secret_name": "OLD_SECRET", "gha_binding_id": "old-binding", "gha_install_status": "ok"}
	access.StoreSnapshot(source, grant.Snapshot())
	return source
}
func insertRecoveryWorkload(t *testing.T, f *gitFixture, table, id, team string, source dbtype.JSONMap) {
	t.Helper()
	require.NoError(t, f.db.Table(table).Create(map[string]any{"id": id, "team_id": team, "source_config": source, "gha_deploy_token_hash": "old-token-hash", "build_location": "github_actions"}).Error)
}
func recoveryIntents(t *testing.T, f *gitFixture) []models.GitHubWebhookIntent {
	t.Helper()
	var intents []models.GitHubWebhookIntent
	require.NoError(t, f.db.Order("id").Find(&intents).Error)
	return intents
}
func intentSnapshot(t *testing.T, intent models.GitHubWebhookIntent) access.Snapshot {
	t.Helper()
	snapshot, err := access.SnapshotFromMap(intent.Payload)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	return *snapshot
}

func TestGitHubReconnectCapturesDurableSelectedWorkflowRecovery(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	sc := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101", "202")
	source := recoverySource(t, f, team, sc.id, "org/repo101")
	removed := recoverySource(t, f, team, sc.id, "org/repo202")
	createRecoveryTables(t, f)
	insertRecoveryWorkload(t, f, "docker_applications", "application", team, source)
	legacy := dbtype.JSONMap{}
	for key, value := range source {
		legacy[key] = value
	}
	delete(legacy, "repository_id")
	legacy["owner"] = "org"
	legacy["repo"] = "repo101"
	insertRecoveryWorkload(t, f, "docker_composes", "compose", team, legacy)
	insertRecoveryWorkload(t, f, "docker_applications", "removed-repository", team, removed)
	insertRecoveryWorkload(t, f, "docker_applications", "other-team", "team-b00000000000000000000", source)
	for _, key := range []string{"dependency_repository_ids", "git_dependencies"} {
		withDependencies := dbtype.JSONMap{}
		for field, value := range source {
			withDependencies[field] = value
		}
		withDependencies[key] = []any{"private-dependency"}
		insertRecoveryWorkload(t, f, "docker_applications", key, team, withDependencies)
		withDependencies[key] = "malformed-dependency-list"
		insertRecoveryWorkload(t, f, "docker_applications", "malformed-"+key, team, withDependencies)
	}
	insertRecoveryWorkload(t, f, "docker_applications", "server-build", team, source)
	require.NoError(t, f.db.Table("docker_applications").Where("id = ?", "server-build").Update("build_location", "server").Error)
	insertRecoveryWorkload(t, f, "docker_applications", "deleted", team, source)
	require.NoError(t, f.db.Table("docker_applications").Where("id = ?", "deleted").Update("deleted_at", time.Now()).Error)
	require.NoError(t, f.service.Disconnect(ctx, sc.id, team, user))
	f.selectRepos(t, f.authorize(t, team, user, session, sc.id), user, session, "101")
	intents := recoveryIntents(t, f)
	require.Len(t, intents, 2)
	bindings := map[string]string{}
	for _, intent := range intents {
		kind, id := intent.Payload.GetString("workload_kind"), intent.Payload.GetString("workload_id")
		require.Contains(t, []string{"application", "compose"}, id)
		require.Equal(t, id, kind)
		require.Nil(t, intent.DispatchedAt, "queue absence retains a durable outbox entry")
		require.Equal(t, githubWorkflowRecoveryTask, intent.TaskType)
		require.Len(t, intent.Payload.GetString("binding_id"), 26)
		bindings[id] = intent.Payload.GetString("binding_id")
		require.Equal(t, "https://launch.test", intent.Payload.GetString("launch_base_url"))
		require.Equal(t, true, intent.Payload["recovery"])
		require.Equal(t, true, intent.Payload["rotate_token"])
		snapshot := intentSnapshot(t, intent)
		require.NoError(t, access.ValidateSnapshot(ctx, f.db, team, snapshot))
		require.Greater(t, snapshot.ConnectionGeneration, sc.generation)
	}
	// Selection creates fresh authority only in the new job payload. Old callbacks
	// and previously queued deployments retain their revoked snapshots.
	var row struct {
		SourceConfig       dbtype.JSONMap `gorm:"type:json"`
		GHADeployTokenHash string
	}
	require.NoError(t, f.db.Table("docker_applications").Where("id = ?", "application").Take(&row).Error)
	require.Equal(t, "old-token-hash", row.GHADeployTokenHash)
	old, err := access.SnapshotFromMap(row.SourceConfig)
	require.NoError(t, err)
	require.Error(t, access.ValidateSnapshot(ctx, f.db, team, *old))
	f.selectRepos(t, f.authorize(t, team, user, session, sc.id), user, session, "101")
	repeated := recoveryIntents(t, f)
	require.Len(t, repeated, 2)
	for _, intent := range repeated {
		require.Equal(t, bindings[intent.Payload.GetString("workload_id")], intent.Payload.GetString("binding_id"))
	}
	ids := []string{intents[0].ID, intents[1].ID}
	require.ErrorContains(t, DispatchGitHubIntentIDs(ctx, f.db, ids, func(*asynq.Task) error { return errors.New("queue offline") }), "offline")
	for _, intent := range recoveryIntents(t, f) {
		require.Nil(t, intent.DispatchedAt)
	}
	sent := 0
	require.NoError(t, DispatchGitHubIntentIDs(ctx, f.db, ids, func(task *asynq.Task) error {
		sent++
		require.Equal(t, githubWorkflowRecoveryTask, task.Type())
		return nil
	}))
	require.Equal(t, 2, sent)
	require.NoError(t, DispatchGitHubIntentIDs(ctx, f.db, ids, func(*asynq.Task) error { t.Fatal("already dispatched recovery repeated"); return nil }))
}

func TestGitHubRecoveryPreservesHealthyWorkflowAndSkipsEmptySelection(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	sc := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	healthy := recoverySource(t, f, team, sc.id, "org/repo101")
	delete(healthy, "gha_binding_id")
	createRecoveryTables(t, f)
	insertRecoveryWorkload(t, f, "docker_applications", "healthy", team, healthy)
	f.selectRepos(t, f.authorize(t, team, user, session, sc.id), user, session, "101", "202")
	require.Empty(t, recoveryIntents(t, f), "adding an unrelated repo must not recreate healthy workflows")
	f.selectRepos(t, f.authorize(t, team, user, session, sc.id), user, session)
	require.Empty(t, recoveryIntents(t, f), "empty selection cannot authorize recovery")
	var source struct {
		SourceConfig dbtype.JSONMap `gorm:"type:json"`
	}
	require.NoError(t, f.db.Table("docker_applications").Where("id = ?", "healthy").Take(&source).Error)
	require.Empty(t, source.SourceConfig.GetString("gha_binding_id"), "legacy healthy workflows do not need a new binding marker")
	_ = ctx
}

func TestGitHubRecoveryCommitsWithSelectionAndDispatchesImmediately(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	sc := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	source := recoverySource(t, f, team, sc.id, "org/repo101")
	createRecoveryTables(t, f)
	insertRecoveryWorkload(t, f, "docker_applications", "application", team, source)
	require.NoError(t, f.service.Disconnect(ctx, sc.id, team, user))
	redis := miniredis.RunT(t)
	client := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	f.service.Queue = client
	attempt := f.authorize(t, team, user, session, sc.id)
	// Failure after grants are written must roll back grants and the outbox.
	require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register("reject_recovery_completion", func(tx *gorm.DB) {
		if tx.Statement.Table == "git_hub_connection_attempts" {
			tx.AddError(errors.New("attempt persistence failed"))
		}
	}))
	_, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}, ExpectedGeneration: &attempt.ConnectionGeneration, ExpectedSelectionVersion: &attempt.SelectionVersion})
	require.ErrorContains(t, err, "attempt persistence failed")
	require.Empty(t, recoveryIntents(t, f))
	var connection models.SourceControl
	require.NoError(t, f.db.First(&connection, "id = ?", sc.id).Error)
	require.Equal(t, "disconnected", connection.Status)
	queued, err := client.Inspector().ListPendingTasks("default")
	require.ErrorIs(t, err, asynq.ErrQueueNotFound)
	require.Empty(t, queued)
	require.NoError(t, f.db.Callback().Update().Remove("reject_recovery_completion"))
	f.selectRepos(t, attempt, user, session, "101")
	intents := recoveryIntents(t, f)
	require.Len(t, intents, 1)
	require.NotNil(t, intents[0].DispatchedAt)
	queued, err = client.Inspector().ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, queued, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	require.Equal(t, intents[0].Payload.GetString("binding_id"), payload["binding_id"])
	// No retry or reconciler may substitute authority after a later disconnect.
	pinned := intentSnapshot(t, intents[0])
	require.NoError(t, f.service.Disconnect(ctx, sc.id, team, user))
	require.Error(t, access.ValidateSnapshot(ctx, f.db, team, pinned))
}

func TestGitHubTerminalRecoveryFailureCreatesNewBindingOnSave(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	sc := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	source := recoverySource(t, f, team, sc.id, "org/repo101")
	source["gha_install_status"] = "permissions_missing"
	createRecoveryTables(t, f)
	insertRecoveryWorkload(t, f, "docker_applications", "application", team, source)
	f.selectRepos(t, f.authorize(t, team, user, session, sc.id), user, session, "101")
	first := recoveryIntents(t, f)
	require.Len(t, first, 1)
	source["gha_recovery_status"] = "failed"
	source["gha_recovery_binding_id"] = first[0].Payload.GetString("binding_id")
	require.NoError(t, f.db.Table("docker_applications").Where("id = ?", "application").Update("source_config", source).Error)
	f.selectRepos(t, f.authorize(t, team, user, session, sc.id), user, session, "101")
	intents := recoveryIntents(t, f)
	require.Len(t, intents, 2)
	require.NotEqual(t, intents[0].Payload.GetString("binding_id"), intents[1].Payload.GetString("binding_id"))
	_ = ctx
}
