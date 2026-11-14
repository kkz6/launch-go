package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func mongoRestoreFixture(t *testing.T, source *string) restoreServiceFixture {
	t.Helper()
	f := newRestoreServiceFixture(t)
	require.NoError(t, f.db.Model(&models.Database{}).Where("id IN ?", []string{f.source.ID, f.target.ID}).Update("engine", dockertypes.DatabaseEngineMongo).Error)
	require.NoError(t, f.db.Model(f.run).Update("source_database", source).Error)
	return f
}

func TestMongoRestoreUsesSnapshotNamespaceAfterSettingsChange(t *testing.T) {
	for _, target := range []string{"source", "other database"} {
		t.Run(target, func(t *testing.T) {
			source := "archived_database"
			f := mongoRestoreFixture(t, &source)
			require.NoError(t, f.db.Model(f.backup).Update("database_name", "changed_setting").Error)
			var script string
			f.service.restoreTask = func(_ context.Context, _ *servermodels.Server, task taskrunner.Task) (*taskrunner.SSHCommandResult, error) {
				script = task.Script()
				return &taskrunner.SSHCommandResult{Stdout: "::LAUNCH::restore_step::done\n"}, nil
			}
			req := &dto.RestoreBackupRequest{RunID: f.run.ID}
			targetName := "production"
			if target == "other database" {
				req.TargetDatabaseID = &f.target.ID
				targetName = "staging"
			}
			require.NoError(t, f.service.Restore(context.Background(), f.source.ID, f.source.ProjectID, f.source.ServerID, f.source.TeamID, "user-1", req))
			require.Contains(t, script, "--nsInclude='archived_database.*'")
			require.Contains(t, script, "--nsFrom='archived_database.*'")
			require.Contains(t, script, "--nsTo='"+targetName+".*'")
			require.NotContains(t, script, "changed_setting")
		})
	}
}

func TestMongoRestoreRefusesUnknownOrUnsafeSourceBeforeDispatch(t *testing.T) {
	for _, source := range []string{"legacy", "", "admin", "production.*"} {
		t.Run(source, func(t *testing.T) {
			var recorded *string
			if source != "legacy" {
				recorded = &source
			}
			f := mongoRestoreFixture(t, recorded)
			called := false
			f.service.restoreTask = func(context.Context, *servermodels.Server, taskrunner.Task) (*taskrunner.SSHCommandResult, error) {
				called = true
				return nil, nil
			}
			err := f.service.Restore(context.Background(), f.source.ID, f.source.ProjectID, f.source.ServerID, f.source.TeamID, "user-1", &dto.RestoreBackupRequest{RunID: f.run.ID})
			require.Error(t, err)
			if source == "legacy" || source == "" {
				require.Contains(t, err.Error(), "Create a new backup with an explicit database name")
			}
			require.False(t, called)
			require.Zero(t, f.events.calls)
		})
	}
}

func TestMongoRestoreRejectsTargetOutsideScopeBeforeDispatch(t *testing.T) {
	for _, field := range []string{"team_id", "server_id", "engine"} {
		t.Run(field, func(t *testing.T) {
			source := "production"
			f := mongoRestoreFixture(t, &source)
			require.NoError(t, f.db.Model(f.target).Update(field, "other").Error)
			called := false
			f.service.restoreTask = func(context.Context, *servermodels.Server, taskrunner.Task) (*taskrunner.SSHCommandResult, error) {
				called = true
				return nil, nil
			}
			err := f.service.Restore(context.Background(), f.source.ID, f.source.ProjectID, f.source.ServerID, f.source.TeamID, "user-1", &dto.RestoreBackupRequest{RunID: f.run.ID, TargetDatabaseID: &f.target.ID})
			require.Error(t, err)
			require.False(t, called)
			require.Zero(t, f.events.calls)
		})
	}
}
