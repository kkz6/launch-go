package gitaccess

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	githubfixture "github.com/kkz6/launch-go/internal/modules/docker/testutil"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func isolationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:site-access-"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}
func selectedSite(t *testing.T, db *gorm.DB, team, name string) (*models.Site, *access.RepositoryAccess) {
	t.Helper()
	grant := githubfixture.SelectedRepository(t, db, team, name, map[string]any{})
	site := &models.Site{}
	site.ID = "site-" + team
	site.TeamID = team
	site.SourceControlID = &grant.SourceControl.ID
	site.SourceControlRepositoriesID = &grant.Repository.ID
	return site, grant
}
func capturedDeployment(t *testing.T, db *gorm.DB, site *models.Site) *models.Deployment {
	t.Helper()
	snapshot, err := Capture(context.Background(), db, nil, site)
	require.NoError(t, err)
	dep := &models.Deployment{GitAccess: snapshot}
	dep.SiteID = site.ID
	dep.TeamID = site.TeamID
	return dep
}

func TestQueuedSiteDeploymentCannotUseChangedDependencySelection(t *testing.T) {
	db := isolationDB(t)
	site, _ := selectedSite(t, db, "team", "org/main")
	dep := githubfixture.SelectedRepository(t, db, "team", "org/dependency", map[string]any{})
	site.RepositoryDependencyIDs = []string{strconv.FormatUint(dep.Repository.ID, 10)}
	deployment := capturedDeployment(t, db, site)
	_, err := ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
	require.NoError(t, err)
	site.RepositoryDependencyIDs = nil
	_, err = ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
	require.ErrorIs(t, err, access.ErrStaleAccess)
}

func TestQueuedGitHubDeploymentCannotBecomeAnUnconnectedDeployment(t *testing.T) {
	db := isolationDB(t)
	site, _ := selectedSite(t, db, "team", "org/main")
	deployment := capturedDeployment(t, db, site)
	site.SourceControlID = nil
	site.SourceControlRepositoriesID = nil
	_, err := ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
	require.ErrorIs(t, err, access.ErrStaleAccess)
}

func TestSiteAccessSeparatesConnectionsSharingOneGitHubInstallation(t *testing.T) {
	db := isolationDB(t)
	ctx := context.Background()
	a, grantA := selectedSite(t, db, "team-A", "org/frontend")
	b, grantB := selectedSite(t, db, "team-B", "org/backend")
	require.NoError(t, db.Model(grantB.SourceControl).Updates(map[string]any{"installation_id": grantA.SourceControl.InstallationID, "git_hub_installation_id": grantA.SourceControl.GitHubInstallationID}).Error)
	resolved, err := Resolve(ctx, db, nil, a, access.Clone, nil)
	require.NoError(t, err)
	require.Equal(t, "org/frontend", resolved.Repository.FullName)
	resolved, err = Resolve(ctx, db, nil, b, access.Clone, nil)
	require.NoError(t, err)
	require.Equal(t, "org/backend", resolved.Repository.FullName)
	foreign := *a
	foreign.SourceControlID = b.SourceControlID
	foreign.SourceControlRepositoriesID = b.SourceControlRepositoriesID
	_, err = Resolve(ctx, db, nil, &foreign, access.Clone, nil)
	require.Error(t, err)
	mismatched := *a
	mismatched.SourceControlRepositoriesID = b.SourceControlRepositoriesID
	_, err = Resolve(ctx, db, nil, &mismatched, access.Clone, nil)
	require.ErrorContains(t, err, "does not belong")
	_, err = ResolveDependency(ctx, db, nil, a.TeamID, strconv.FormatUint(grantB.Repository.ID, 10), nil)
	require.Error(t, err)
}

func TestSiteAccessRequiresExplicitSelectionAndCompletePair(t *testing.T) {
	db := isolationDB(t)
	site, grant := selectedSite(t, db, "team-A", "org/frontend")
	require.NoError(t, db.Model(grant.Repository).Update("selected", false).Error)
	_, err := Resolve(context.Background(), db, nil, site, access.Clone, nil)
	require.Error(t, err)
	site.SourceControlID = nil
	_, err = Resolve(context.Background(), db, nil, site, access.Clone, nil)
	require.ErrorContains(t, err, "together")
}

func TestSiteDeploymentRejectsRetiredGrantAndConnectionGenerations(t *testing.T) {
	for _, change := range []string{"reconnect", "deselect", "reselect"} {
		t.Run(change, func(t *testing.T) {
			db := isolationDB(t)
			site, grant := selectedSite(t, db, "team-A", "org/frontend")
			deployment := capturedDeployment(t, db, site)
			_, err := ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
			require.NoError(t, err)
			switch change {
			case "reconnect":
				require.NoError(t, db.Model(grant.SourceControl).Update("generation", 2).Error)
			case "deselect":
				require.NoError(t, db.Model(grant.Repository).Updates(map[string]any{"selected": false, "grant_version": 2}).Error)
			case "reselect":
				require.NoError(t, db.Model(grant.Repository).Update("grant_version", 3).Error)
			}
			_, err = ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
			require.Error(t, err)
		})
	}
}

func TestSiteDeploymentRejectsLegacyOrCrossTeamSnapshot(t *testing.T) {
	db := isolationDB(t)
	site, _ := selectedSite(t, db, "team-A", "org/frontend")
	deployment := capturedDeployment(t, db, site)
	deployment.TeamID = "team-B"
	_, err := ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
	require.ErrorContains(t, err, "does not belong")
	deployment.TeamID = site.TeamID
	deployment.GitAccess = nil
	_, err = ForDeployment(context.Background(), db, nil, site, deployment, access.Clone)
	require.ErrorContains(t, err, "legacy deployment")
}

func TestSiteCaptureStoresOnlyExplicitDependencyGrants(t *testing.T) {
	db := isolationDB(t)
	site, grant := selectedSite(t, db, "team-A", "org/frontend")
	dependency := gitmodels.SourceControlRepository{SourceControlID: grant.SourceControl.ID, Name: "dependency", FullName: "org/dependency", GitHubRepositoryID: "222", SSHURL: "git@github.com:org/dependency.git", DefaultBranch: "main", Selected: true, Available: true, GrantVersion: 1, VerifiedAt: grant.Repository.VerifiedAt}
	unused := dependency
	unused.Name = "unused"
	unused.FullName = "org/unused"
	unused.GitHubRepositoryID = "333"
	require.NoError(t, db.Create(&dependency).Error)
	require.NoError(t, db.Create(&unused).Error)
	site.RepositoryDependencyIDs = dbtype.JSONStringSlice{strconv.FormatUint(dependency.ID, 10)}
	snapshot, err := Capture(context.Background(), db, nil, site)
	require.NoError(t, err)
	encoded, err := json.Marshal(snapshot["dependencies"])
	require.NoError(t, err)
	var dependencies map[string]access.Snapshot
	require.NoError(t, json.Unmarshal(encoded, &dependencies))
	require.Len(t, dependencies, 1)
	require.Contains(t, dependencies, strconv.FormatUint(dependency.ID, 10))
	require.NotContains(t, dependencies, strconv.FormatUint(unused.ID, 10))
	require.Equal(t, grant.SourceControl.ID, dependencies[strconv.FormatUint(dependency.ID, 10)].ConnectionID)
}
