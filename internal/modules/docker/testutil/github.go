package testutil

import (
	"context"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"strconv"
	"testing"
	"time"
)

// SelectedRepository creates the complete trusted database state, rather than
// bypassing the resolver. Verification is fresh so tests need no GitHub network.
func SelectedRepository(t *testing.T, db *gorm.DB, teamID, fullName string, config map[string]any) *access.RepositoryAccess {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&gitmodels.SourceControl{}, &gitmodels.SourceControlRepository{}, &gitmodels.GitHubAuthorization{}, &gitmodels.GitHubInstallation{}, &gitmodels.GitHubCredentialLease{}, &gitmodels.GitHubCleanupManifest{}))
	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS teams (id TEXT PRIMARY KEY, user_id TEXT NOT NULL)").Error)
	var team struct{ UserID string }
	db.Table("teams").Where("id = ?", teamID).Take(&team)
	if team.UserID == "" {
		team.UserID = util.NewULID()
		require.NoError(t, db.Exec("INSERT INTO teams(id,user_id) VALUES(?,?)", teamID, team.UserID).Error)
	}
	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, status TEXT NOT NULL)").Error)
	var userCount int64
	require.NoError(t, db.Table("users").Where("id = ?", team.UserID).Count(&userCount).Error)
	if userCount == 0 {
		fields := map[string]any{"id": team.UserID, "status": "active"}
		for column, value := range map[string]any{"name": "Repository sponsor", "email": team.UserID + "@test.invalid", "password": "unused"} {
			if db.Migrator().HasColumn("users", column) {
				fields[column] = value
			}
		}
		require.NoError(t, db.Table("users").Create(fields).Error)
	}
	auth := gitmodels.GitHubAuthorization{UserID: team.UserID, Host: "github.com", AppID: "app", GitHubUserID: util.NewULID(), Generation: 1}
	require.NoError(t, db.Create(&auth).Error)
	inst := gitmodels.GitHubInstallation{Host: "github.com", AppID: "app", InstallationID: util.NewULID(), Status: "active", Generation: 1, Permissions: `{"contents":"write","actions":"write","workflows":"write","secrets":"write","variables":"write"}`}
	require.NoError(t, db.Create(&inst).Error)
	sc := gitmodels.SourceControl{Provider: "github", ProviderID: inst.InstallationID, InstallationID: &inst.InstallationID, GitHubAuthorizationID: &auth.ID, GitHubInstallationID: &inst.ID, Generation: 1, Status: "active"}
	sc.TeamID = teamID
	sc.UserID = team.UserID
	if id, _ := config["source_control_id"].(string); id != "" {
		sc.ID = id
	}
	require.NoError(t, db.Create(&sc).Error)
	now := time.Now()
	repo := gitmodels.SourceControlRepository{SourceControlID: sc.ID, Name: fullName, FullName: fullName, GitHubRepositoryID: strconv.FormatInt(time.Now().UnixNano(), 10), Selected: true, Available: true, GrantVersion: 1, VerifiedAt: &now, SSHURL: "git@github.com:" + fullName + ".git"}
	require.NoError(t, db.Create(&repo).Error)
	config["source_control_id"] = sc.ID
	config["repository_id"] = strconv.FormatUint(repo.ID, 10)
	config["repo"] = "https://github.com/" + fullName + ".git"
	resolved, err := access.NewResolver(db, nil).Resolve(context.Background(), access.Request{TeamID: teamID, SourceControlID: sc.ID, RepositoryID: strconv.FormatUint(repo.ID, 10), Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	access.StoreSnapshot(config, resolved.Snapshot())
	config["gha_access"] = resolved.Snapshot()
	return resolved
}
