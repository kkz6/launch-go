package jobs

import (
	"context"
	"encoding/json"
	"github.com/kkz6/launch-go/internal/modules/docker/testutil"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
)

func TestDeploymentRevalidatesImmediatelyBeforeCredentialDelivery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	config := map[string]any{}
	resolved := testutil.SelectedRepository(t, db, "account-a", "acme/app", config)
	snapshot := resolved.Snapshot()
	task := taskrunner.NewBaseTask()
	deps := &JobDeps{Deps: &pkgjobs.Deps{DB: db}}
	require.NoError(t, deps.guardGitTask(task, "account-a", config, &snapshot, nil, nil, ""))
	require.NoError(t, task.BeforeExecution(context.Background()))
	require.NoError(t, db.Model(&gitmodels.SourceControlRepository{}).Where("id = ?", resolved.Repository.ID).Update("grant_version", 2).Error)
	require.Error(t, task.BeforeExecution(context.Background()))
}

func TestQueuePayloadPinsAccessAndRejectsPlainRegistrySecrets(t *testing.T) {
	snapshot := &access.Snapshot{ConnectionID: "connection", ConnectionGeneration: 3, RepositoryID: "1", GrantVersion: 2, AuthorizationID: "auth", AuthorizationGeneration: 2, InstallationGeneration: 1}
	task, err := NewDeployApplicationTask("app", "deployment", "server", "team", snapshot)
	require.NoError(t, err)
	var payload DeployApplicationPayload
	require.NoError(t, json.Unmarshal(task.Payload(), &payload))
	require.Equal(t, snapshot, payload.GitAccess)
	_, err = NewDeployApplicationTaskFromGHA("app", "deployment", "server", "team", "image", "ghcr.io", "oauth2", "secret", "0")
	require.Error(t, err)
	_, err = NewProtectedApplicationDeployTask(DeployApplicationPayload{RegistryCredentialReference: "reference", GitAccess: snapshot, OverrideRegistryPassword: "secret"})
	require.Error(t, err)
	protected, err := NewProtectedApplicationDeployTask(DeployApplicationPayload{DeploymentID: "deployment", RegistryCredentialReference: "opaque-reference", GitAccess: snapshot})
	require.NoError(t, err)
	require.NotContains(t, string(protected.Payload()), "override_registry_password")
	require.Contains(t, string(protected.Payload()), "opaque-reference")
}
