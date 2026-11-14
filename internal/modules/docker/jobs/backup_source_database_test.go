package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestBackupRunRecordsActualDumpNamespaceBeforeExecution(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(map[bool]string{false: "scheduled", true: "manual"}[manual], func(t *testing.T) {
			fixture := newBackupExecutionFixture(t, manual)
			require.NoError(t, fixture.db.Model(&models.Database{}).Where("id = ?", fixture.job.Payload.DatabaseID).Update("engine", dockertypes.DatabaseEngineMongo).Error)
			require.NoError(t, fixture.db.Model(&models.DatabaseBackup{}).Where("id = ?", fixture.backupID).Update("database_name", "analytics").Error)
			require.NoError(t, fixture.job.Handle(context.Background()))
			var run models.DatabaseBackupRun
			require.NoError(t, fixture.db.First(&run, "id = ?", fixture.job.runID).Error)
			require.NotNil(t, run.SourceDatabase)
			require.Equal(t, "analytics", *run.SourceDatabase)
			require.Contains(t, fixture.dispatcher.GetExecution(0).Script, "--db=analytics")
		})
	}
}

func TestBackupSourceRemainsImmutableAcrossConfigChanges(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Model(&models.DatabaseBackupRun{}).Where("id = ?", fixture.runID).Update("status", "running").Error)
	var run models.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&run, "id = ?", fixture.runID).Error)
	source, err := fixture.job.captureSourceDatabase(context.Background(), &run, "production")
	require.NoError(t, err)
	require.Equal(t, "production", source)
	// A retry reloads the run but observes a newer backup configuration.
	require.NoError(t, fixture.db.First(&run, "id = ?", fixture.runID).Error)
	source, err = fixture.job.captureSourceDatabase(context.Background(), &run, "changed-settings")
	require.NoError(t, err)
	require.Equal(t, "production", source)
}

func TestBackupDoesNotExecuteWhenSourceCannotBeRecorded(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Callback().Update().Before("gorm:update").Register("reject_source_metadata", func(tx *gorm.DB) {
		updates, _ := tx.Statement.Dest.(map[string]interface{})
		if _, exists := updates["source_database"]; exists {
			tx.AddError(errors.New("source metadata unavailable"))
		}
	}))
	err := fixture.job.Handle(context.Background())
	require.ErrorContains(t, err, "record backup source database")
	require.Zero(t, fixture.dispatcher.ExecutionCount())
}
