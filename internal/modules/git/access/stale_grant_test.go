package access

import (
	"context"
	"strconv"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStaleSponsorCheckCannotRevokeReselectedRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { pool.Close() })
	require.NoError(t, db.AutoMigrate(&models.SourceControl{}, &models.SourceControlRepository{}, &models.GitHubCleanupManifest{}, &models.GitHubCredentialLease{}))
	connection := models.SourceControl{Generation: 1, Status: "active"}
	connection.TeamID = "team"
	connection.UserID = "user"
	require.NoError(t, db.Create(&connection).Error)
	repository := models.SourceControlRepository{SourceControlID: connection.ID, GitHubRepositoryID: "101", FullName: "org/repo", Selected: true, Available: true, GrantVersion: 3}
	require.NoError(t, db.Create(&repository).Error)
	repositoryID := strconv.FormatUint(repository.ID, 10)
	manifest := models.GitHubCleanupManifest{SourceControlID: connection.ID, TeamID: "team", RepositoryID: repositoryID, ConnectionGeneration: 1, GrantVersion: 3, Status: "registered", Resources: "[]"}
	require.NoError(t, db.Create(&manifest).Error)
	lease := models.GitHubCredentialLease{SourceControlID: connection.ID, TeamID: "team", RepositoryID: repository.ID, ConnectionGeneration: 1, GrantVersion: 3, Status: "released"}
	require.NoError(t, db.Create(&lease).Error)
	// The old request observed grant 1 before the user removed and reselected
	// this repository. Its late negative sponsor response cannot revoke grant 3.
	old := &RepositoryAccess{resolver: NewResolver(db, nil), repositoryID: repository.ID, snapshot: Snapshot{ConnectionID: connection.ID, ConnectionGeneration: 1, RepositoryID: repositoryID, GrantVersion: 1}}
	old.invalidateGrant(context.Background())
	require.NoError(t, db.First(&repository, "id = ?", repository.ID).Error)
	require.True(t, repository.Selected)
	require.True(t, repository.Available)
	require.Equal(t, uint64(3), repository.GrantVersion)
	require.NoError(t, db.First(&manifest, "id = ?", manifest.ID).Error)
	require.Equal(t, "registered", manifest.Status)
	require.NoError(t, db.First(&lease, "id = ?", lease.ID).Error)
	require.Equal(t, "released", lease.Status)
}
