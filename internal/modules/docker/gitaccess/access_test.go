package gitaccess

import (
	"context"
	"github.com/kkz6/launch-go/internal/modules/docker/testutil"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strconv"
	"testing"
)

func dbForTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return db
}

func TestPublicRepositoriesCannotUseAlternatePrivateCredentials(t *testing.T) {
	for _, repository := range []string{
		"ssh://github.com/org/private.git", "ssh://git@github.com/org/private.git",
		"git@github.com:org/private.git", "file:///var/lib/launch/private", "/var/lib/launch/private",
		"ext::sh -c true", "http://github.com/org/repo.git", "https://token@github.com/org/repo.git",
		"https://github.com/org/repo.git?token=private", "https://github.com/org/repo.git#private",
	} {
		t.Run(repository, func(t *testing.T) {
			_, err := Bind(context.Background(), nil, nil, "account-a", map[string]any{"repo": repository}, access.Clone)
			require.Error(t, err)
		})
	}
	snapshot, err := Bind(context.Background(), nil, nil, "account-a", map[string]any{"repo": "https://github.com/org/public.git"}, access.Clone)
	require.NoError(t, err)
	require.Nil(t, snapshot)
}

func TestRepositoryIsolationAndGenerationFencing(t *testing.T) {
	for _, scenario := range []string{"other account connection", "other account repository", "unselected", "foreign host", "removed after enqueue", "reselected after enqueue", "reconnected", "authorization replaced", "installation suspended", "legacy queue"} {
		t.Run(scenario, func(t *testing.T) {
			db := dbForTest(t)
			config := map[string]any{}
			a := testutil.SelectedRepository(t, db, "account-a", "acme/alpha", config)
			other := map[string]any{}
			b := testutil.SelectedRepository(t, db, "account-b", "acme/beta", other)
			snapshot := a.Snapshot()
			expected := &snapshot
			_, err := Resolve(context.Background(), db, nil, "account-a", config, access.Clone, expected, true)
			require.NoError(t, err)
			switch scenario {
			case "other account connection":
				config["source_control_id"] = b.SourceControl.ID
			case "other account repository":
				config["repository_id"] = other["repository_id"]
				config["repo"] = other["repo"]
			case "unselected", "removed after enqueue":
				require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("id = ?", a.Repository.ID).Update("selected", false).Error)
			case "foreign host":
				config["repo"] = "https://attacker.example/acme/alpha.git"
			case "reselected after enqueue":
				require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("id = ?", a.Repository.ID).Update("grant_version", 2).Error)
			case "reconnected":
				require.NoError(t, db.Model(&models.SourceControl{}).Where("id = ?", a.SourceControl.ID).Update("generation", 2).Error)
			case "authorization replaced":
				require.NoError(t, db.Model(&models.GitHubAuthorization{}).Where("id = ?", snapshot.AuthorizationID).Update("generation", 2).Error)
			case "installation suspended":
				require.NoError(t, db.Model(&models.GitHubInstallation{}).Where("id = ?", *a.SourceControl.GitHubInstallationID).Update("status", "suspended").Error)
			case "legacy queue":
				expected = nil
			}
			_, err = Resolve(context.Background(), db, nil, "account-a", config, access.Clone, expected, true)
			require.Error(t, err)
		})
	}
}

func TestDependenciesRequireOwnTeamSelectedGrantsAndPinnedGeneration(t *testing.T) {
	db := dbForTest(t)
	config := map[string]any{}
	testutil.SelectedRepository(t, db, "account-a", "acme/main", config)
	dependency := map[string]any{}
	dep := testutil.SelectedRepository(t, db, "account-a", "acme/library", dependency)
	other := map[string]any{}
	testutil.SelectedRepository(t, db, "account-b", "acme/other", other)
	config["dependency_repository_ids"] = []string{dependency["repository_id"].(string)}
	snapshot, err := Bind(context.Background(), db, nil, "account-a", config, access.Clone)
	require.NoError(t, err)
	pinned := PinnedSnapshots(config, snapshot)
	require.Len(t, pinned, 2)
	resolved, err := ResolveDependencies(context.Background(), db, nil, "account-a", config, []access.Snapshot{*pinned[1]}, true)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	config["dependency_repository_ids"] = []string{other["repository_id"].(string)}
	_, err = ResolveDependencies(context.Background(), db, nil, "account-a", config, nil, false)
	require.Error(t, err)
	config["dependency_repository_ids"] = []string{dependency["repository_id"].(string)}
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("id = ?", dep.Repository.ID).Update("grant_version", 2).Error)
	_, err = ResolveDependencies(context.Background(), db, nil, "account-a", config, []access.Snapshot{*pinned[1]}, true)
	require.Error(t, err)
}

func TestSharedSelectionDoesNotInterruptExistingActionsOwner(t *testing.T) {
	db := dbForTest(t)
	config := map[string]any{}
	a := testutil.SelectedRepository(t, db, "account-a", "acme/shared", config)
	_, err := Resolve(context.Background(), db, nil, "account-a", config, access.ActionsBootstrap, nil, false)
	require.NoError(t, err)
	other := map[string]any{}
	b := testutil.SelectedRepository(t, db, "account-b", "acme/shared", other)
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("id = ?", b.Repository.ID).Update("git_hub_repository_id", a.GitHubRepositoryID).Error)
	// Selecting the same repository is independent of who owns its CI resources.
	_, err = Resolve(context.Background(), db, nil, "account-a", config, access.ActionsBootstrap, nil, false)
	require.NoError(t, err)
	claim := models.GitHubCleanupManifest{SourceControlID: a.SourceControl.ID, TeamID: "account-a", RepositoryID: otherID(a.Repository.ID), GitHubRepositoryID: a.GitHubRepositoryID, Resources: "[]", Status: "registered"}
	require.NoError(t, db.Create(&claim).Error)
	_, err = Resolve(context.Background(), db, nil, "account-b", other, access.ActionsBootstrap, nil, false)
	require.Error(t, err)
	_, err = Resolve(context.Background(), db, nil, "account-b", other, access.Clone, nil, false)
	require.NoError(t, err)
	for _, change := range []map[string]any{{"selection_version": 2}, {"generation": 2}, {"status": "disconnected"}} {
		require.NoError(t, db.Model(&models.SourceControl{}).Where("id = ?", b.SourceControl.ID).Updates(change).Error)
		for _, operation := range []access.Operation{access.Clone, access.ActionsBootstrap, access.ActionsWrite} {
			_, err = Resolve(context.Background(), db, nil, "account-a", config, operation, nil, false)
			require.NoError(t, err)
		}
	}
	// A disconnect cannot hand CI secrets to B while remote cleanup is pending.
	require.NoError(t, db.Model(&models.SourceControl{}).Where("id = ?", a.SourceControl.ID).Update("status", "disconnected").Error)
	require.NoError(t, db.Model(&models.SourceControl{}).Where("id = ?", b.SourceControl.ID).Update("status", "active").Error)
	require.NoError(t, db.Model(&claim).Update("status", "failed_cleanup").Error)
	_, err = Resolve(context.Background(), db, nil, "account-b", other, access.ActionsBootstrap, nil, false)
	require.Error(t, err)
	require.NoError(t, db.Model(&claim).Update("status", "completed").Error)
	_, err = Resolve(context.Background(), db, nil, "account-b", other, access.ActionsBootstrap, nil, false)
	require.NoError(t, err)
}

func otherID(id uint64) string { return strconv.FormatUint(id, 10) }
