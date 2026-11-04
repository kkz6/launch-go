package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeploymentReleaseNameMigrationPreservesLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("CREATE TABLE deployments (id TEXT PRIMARY KEY, status TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO deployments VALUES ('legacy', 'finished')").Error)

	require.NoError(t, addReleaseNameToDeploymentsUp(db))
	require.NoError(t, addReleaseNameToDeploymentsUp(db))
	var legacy struct {
		Status      string
		ReleaseName *string
	}
	require.NoError(t, db.Table("deployments").Where("id = ?", "legacy").Take(&legacy).Error)
	require.Equal(t, "finished", legacy.Status)
	require.Nil(t, legacy.ReleaseName)
	require.NoError(t, db.Table("deployments").Where("id = ?", "legacy").Update("release_name", "20261104050000").Error)
	require.NoError(t, db.Table("deployments").Where("id = ?", "legacy").Take(&legacy).Error)
	require.Equal(t, "20261104050000", *legacy.ReleaseName)
}
