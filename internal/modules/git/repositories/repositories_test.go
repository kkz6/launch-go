package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

func setupGitRepositoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SourceControl{}, &models.SourceControlRepository{}))

	return db
}

func createSourceControl(
	t *testing.T,
	db *gorm.DB,
	provider gittypes.GitProviderType,
	providerID string,
	teamID string,
) *models.SourceControl {
	t.Helper()

	installationID := providerID
	sourceControl := &models.SourceControl{
		Provider:       provider,
		ProviderID:     providerID,
		InstallationID: &installationID,
	}
	sourceControl.ID = util.NewULID()
	sourceControl.UserID = "user-1"
	sourceControl.TeamID = teamID
	require.NoError(t, db.Create(sourceControl).Error)

	return sourceControl
}

func TestUpsertRepositoryCreatesAndUpdatesAtomically(t *testing.T) {
	db := setupGitRepositoryDB(t)
	sourceControl := createSourceControl(t, db, gittypes.GitProviderGitHub, "installation-1", "team-1")
	repository := NewSourceControlRepoRepository(db)
	ctx := context.Background()

	created, err := repository.UpsertRepository(ctx, sourceControl.ID, &dto.RepositoryData{
		Name:          "app",
		FullName:      "owner/app",
		IsPublic:      true,
		DefaultBranch: "main",
		HTMLURL:       "https://github.com/owner/app",
		SSHURL:        "git@github.com:owner/app.git",
		AdditionalData: map[string]any{
			"id": float64(42),
		},
	})
	require.NoError(t, err)

	updated, err := repository.UpsertRepository(ctx, sourceControl.ID, &dto.RepositoryData{
		Name:           "renamed-app",
		FullName:       "owner/app",
		IsPublic:       false,
		DefaultBranch:  "trunk",
		HTMLURL:        "https://github.com/owner/app",
		SSHURL:         "git@github.com:owner/app.git",
		AdditionalData: map[string]any{"id": float64(43)},
	})
	require.NoError(t, err)

	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "renamed-app", updated.Name)
	assert.False(t, updated.Public)
	assert.Equal(t, "trunk", updated.DefaultBranch)
	require.NotNil(t, updated.AdditionalData)
	assert.JSONEq(t, `{"id":43}`, *updated.AdditionalData)

	var count int64
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestUpsertRepositoryReturnsMetadataSerializationError(t *testing.T) {
	db := setupGitRepositoryDB(t)
	sourceControl := createSourceControl(t, db, gittypes.GitProviderGitHub, "installation-1", "team-1")
	repository := NewSourceControlRepoRepository(db)

	_, err := repository.UpsertRepository(context.Background(), sourceControl.ID, &dto.RepositoryData{
		FullName:       "owner/app",
		AdditionalData: map[string]any{"invalid": func() {}},
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestDeleteByInstallationIDIsProviderScopedAndTransactional(t *testing.T) {
	db := setupGitRepositoryDB(t)
	github := createSourceControl(t, db, gittypes.GitProviderGitHub, "shared-id", "team-1")
	gitlab := createSourceControl(t, db, gittypes.GitProviderGitLab, "shared-id", "team-2")

	for _, sourceControl := range []*models.SourceControl{github, gitlab} {
		repo := &models.SourceControlRepository{
			SourceControlID: sourceControl.ID,
			Name:            "app",
			FullName:        sourceControl.Provider.String() + "/app",
			SSHURL:          "ssh://example/app",
			DefaultBranch:   "main",
		}
		require.NoError(t, db.Create(repo).Error)
	}

	repository := NewSourceControlRepository(db)
	deleted, err := repository.DeleteByInstallationID(context.Background(), gittypes.GitProviderGitHub, "shared-id")
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var remainingSourceControls []models.SourceControl
	require.NoError(t, db.Find(&remainingSourceControls).Error)
	require.Len(t, remainingSourceControls, 1)
	assert.Equal(t, gittypes.GitProviderGitLab, remainingSourceControls[0].Provider)

	var remainingRepositories []models.SourceControlRepository
	require.NoError(t, db.Find(&remainingRepositories).Error)
	require.Len(t, remainingRepositories, 1)
	assert.Equal(t, gitlab.ID, remainingRepositories[0].SourceControlID)
}

func TestDeleteByInstallationIDRollsBackChildDeletion(t *testing.T) {
	db := setupGitRepositoryDB(t)
	sourceControl := createSourceControl(t, db, gittypes.GitProviderGitHub, "installation-1", "team-1")
	require.NoError(t, db.Create(&models.SourceControlRepository{
		SourceControlID: sourceControl.ID,
		Name:            "app",
		FullName:        "owner/app",
		SSHURL:          "ssh://example/app",
		DefaultBranch:   "main",
	}).Error)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER reject_source_control_delete
		BEFORE DELETE ON source_controls
		BEGIN
			SELECT RAISE(ABORT, 'delete rejected');
		END
	`).Error)

	repository := NewSourceControlRepository(db)
	_, err := repository.DeleteByInstallationID(context.Background(), gittypes.GitProviderGitHub, "installation-1")
	require.Error(t, err)

	var repositoryCount int64
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Count(&repositoryCount).Error)
	assert.Equal(t, int64(1), repositoryCount)
}

func TestInstallationQueriesShareProviderAndTeamScoping(t *testing.T) {
	db := setupGitRepositoryDB(t)
	githubTeamOne := createSourceControl(t, db, gittypes.GitProviderGitHub, "github-1", "team-1")
	createSourceControl(t, db, gittypes.GitProviderGitHub, "github-2", "team-2")
	createSourceControl(t, db, gittypes.GitProviderGitLab, "gitlab-1", "team-1")
	repository := NewSourceControlRepository(db)
	ctx := context.Background()

	installations, err := repository.GetInstallations(
		ctx,
		gittypes.GitProviderGitHub,
		contracts.WithTeamID("team-1"),
		contracts.RequireInstallationID(),
	)
	require.NoError(t, err)
	require.Len(t, installations, 1)
	assert.Equal(t, githubTeamOne.ID, installations[0].ID)

	installation, err := repository.GetFirstInstallation(
		ctx,
		gittypes.GitProviderGitHub,
		contracts.WithTeamID("team-1"),
		contracts.WithProviderID("github-1"),
	)
	require.NoError(t, err)
	assert.Equal(t, githubTeamOne.ID, installation.ID)
}

func TestUpdateFieldsSupportsTypedPartialAndEmptyUpdates(t *testing.T) {
	db := setupGitRepositoryDB(t)
	sourceControl := createSourceControl(t, db, gittypes.GitProviderGitHub, "installation-1", "team-1")
	repository := NewSourceControlRepository(db)
	ctx := context.Background()

	require.NoError(t, repository.UpdateFields(ctx, sourceControl.ID, contracts.SourceControlUpdates{}))
	repositoryCount := 0
	providerData := `{"installer":"octocat"}`
	require.NoError(t, repository.UpdateFields(ctx, sourceControl.ID, contracts.SourceControlUpdates{
		RepositoryCount: &repositoryCount,
		ProviderData:    &providerData,
	}))

	var updated models.SourceControl
	require.NoError(t, db.First(&updated, "id = ?", sourceControl.ID).Error)
	require.NotNil(t, updated.RepositoryCount)
	assert.Zero(t, *updated.RepositoryCount)
	require.NotNil(t, updated.ProviderData)
	assert.JSONEq(t, providerData, *updated.ProviderData)
}
