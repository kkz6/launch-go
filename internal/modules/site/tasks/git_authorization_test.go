package tasks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	githubfixture "github.com/kkz6/launch-go/internal/modules/docker/testutil"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSelectedSiteTaskKeepsCredentialsOutOfScriptsAndCallbackPayloads(t *testing.T) {
	registerReleaseTemplates(t)
	for _, zeroDowntime := range []bool{false, true} {
		t.Run(map[bool]string{false: "standard", true: "zero-downtime"}[zeroDowntime], func(t *testing.T) {
			_, site := releaseFixture(t)
			site.ZeroDowntimeDeployment = zeroDowntime
			token := "ghs_never_persist_this_test_token"
			credential := "https://x-access-token:" + token + "@github.com/org/selected.git\n"
			task := DeploySiteTask(DeployOptions{Site: site, Deployment: &models.Deployment{}, RepositoryURL: "https://github.com/org/selected.git", HasAppAuth: true, TempToken: token, AuthURL: strings.TrimSpace(credential), GitCredentials: credential, ReleaseTimestamp: candidateRelease}).(*deploySiteTask)
			require.NotContains(t, task.Script(), token)
			require.NotContains(t, task.Script(), "git config --global")
			require.Contains(t, task.Script(), "LAUNCH_SECRET_DIR")
			require.Contains(t, task.Script(), "credential.useHttpPath")
			payload, err := task.MarshalPayload()
			require.NoError(t, err)
			require.NotContains(t, string(payload), token)
			require.Equal(t, map[string]string{"git-credentials": credential}, task.SecretFiles())
			// Concurrent executions cannot mutate each other's credential maps.
			credentials := task.SecretFiles()
			credentials["git-credentials"] = "changed"
			require.Equal(t, credential, task.SecretFiles()["git-credentials"])
		})
	}
}

func TestSiteCredentialsIgnoreAnotherAccountsGlobalURLRewrite(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for credential isolation integration")
	}
	registerReleaseTemplates(t)
	_, site := releaseFixture(t)
	root := t.TempDir()
	site.Path = root
	ambient := filepath.Join(root, ".gitconfig")
	require.NoError(t, os.WriteFile(ambient, []byte("[url \"https://x-access-token:another-account-token@github.com/\"]\n\tinsteadOf = https://github.com/\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "git-credentials"), []byte("https://x-access-token:selected-token@github.com/org/selected.git\n"), 0600))
	script := renderUpdateRepository(DeployOptions{Site: site, Deployment: &models.Deployment{}, HasAppAuth: true, RepositoryURL: "https://github.com/org/selected.git"}, filepath.Join(root, "checkout"), "")
	// Exercise the generated credential setup without performing a clone.
	setup, _, ok := strings.Cut(script, "echo \"📦 Cloning/updating repository...\"")
	require.True(t, ok)
	cmd := exec.Command("bash", "-c", setup+"\ngit ls-remote --get-url https://github.com/org/unselected.git\n")
	cmd.Env = append(os.Environ(), "HOME="+root, "GIT_CONFIG_GLOBAL="+ambient, "LAUNCH_SECRET_DIR="+root)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, "https://github.com/org/unselected.git", strings.TrimSpace(string(output)))
}

func TestExistingSiteCheckoutCannotRetainAnotherRepositoryCredentials(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for checkout isolation integration")
	}
	registerReleaseTemplates(t)
	for _, bare := range []bool{false, true} {
		t.Run(map[bool]string{false: "checkout", true: "mirror"}[bare], func(t *testing.T) {
			_, site := releaseFixture(t)
			root := t.TempDir()
			site.Path, site.ZeroDowntimeDeployment = root, bare
			checkout := filepath.Join(root, "checkout")
			args := []string{"init", "--quiet"}
			if bare {
				args = append(args, "--bare")
			}
			require.NoError(t, exec.Command("git", append(args, checkout)...).Run())
			origin := "https://x-access-token:old-token@github.com/org/unselected.git"
			if bare {
				// Mirrors for another origin are discarded before cloning; retained
				// mirrors must also drop legacy authentication and URL rewrites.
				origin = "https://github.com/org/selected.git"
			}
			for key, value := range map[string]string{
				"remote.origin.url":                                          origin,
				"http.https://github.com/.extraheader":                       "AUTHORIZATION: old-token",
				"url.https://x-access-token:old-token@github.com/.insteadOf": "https://github.com/",
				"credential.helper":                                          "store --file=/tmp/old-credentials",
			} {
				require.NoError(t, exec.Command("git", "-C", checkout, "config", key, value).Run())
			}
			require.NoError(t, os.WriteFile(filepath.Join(root, "git-credentials"), []byte("https://x-access-token:selected-token@github.com/org/selected.git\n"), 0600))
			script := renderUpdateRepository(DeployOptions{Site: site, Deployment: &models.Deployment{}, HasAppAuth: true, RepositoryURL: "https://github.com/org/selected.git"}, checkout, "")
			setup, _, ok := strings.Cut(script, "echo \"📦 Cloning/updating repository...\"")
			require.True(t, ok)
			cmd := exec.Command("bash", "-c", setup+"\ncd \"$1\"\ngit remote get-url origin\ngit config --get-urlmatch http.extraheader https://github.com/org/selected.git || true\ngit ls-remote --get-url https://github.com/org/unselected.git\n", "bash", checkout)
			cmd.Env = append(os.Environ(), "LAUNCH_SECRET_DIR="+root)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, string(output))
			require.Equal(t, "https://github.com/org/selected.git\nhttps://github.com/org/unselected.git\n", string(output))
			require.NotContains(t, string(output), "old-token")
			cmd = exec.Command("bash", "-c", setup+"\ngit ls-remote ssh://github-alias/org/unselected.git\n")
			cmd.Env = append(os.Environ(), "LAUNCH_SECRET_DIR="+root)
			output, err = cmd.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(output), "transport 'ssh' not allowed")
		})
	}
}

func TestSelectedSiteTaskKeepsLegacyDeployKeyOutOfScript(t *testing.T) {
	registerReleaseTemplates(t)
	_, site := releaseFixture(t)
	site.DeployKeyPrivate = dbtype.EncryptedString("PRIVATE_DEPLOY_KEY_TEST_BYTES")
	task := DeploySiteTask(DeployOptions{Site: site, Deployment: &models.Deployment{}, RepositoryURL: "git@gitlab.com:org/repository.git", ReleaseTimestamp: candidateRelease}).(*deploySiteTask)
	require.NotContains(t, task.Script(), string(site.DeployKeyPrivate))
	require.Equal(t, string(site.DeployKeyPrivate), task.SecretFiles()["git-key"])
}

func TestRevokedSiteCompletionDoesNotDispatchWorkloadSideEffects(t *testing.T) {
	for _, change := range []string{"deselect", "reconnect", "dependency-revoked"} {
		t.Run(change, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:site-git-callback-"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			rawDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, rawDB.Close()) })
			require.NoError(t, db.AutoMigrate(&models.Site{}, &models.Deployment{}))
			grant := githubfixture.SelectedRepository(t, db, "team-1", "org/selected", map[string]any{})
			site := &models.Site{Address: "example.test", Path: "/home/launcher/example.test", SourceControlID: &grant.SourceControl.ID, SourceControlRepositoriesID: &grant.Repository.ID, Type: sitetypes.SiteTypeLaravel}
			site.ID = "site-1"
			site.TeamID = "team-1"
			site.ServerID = "server-1"
			var dependencyID uint64
			if change == "dependency-revoked" {
				dependency := githubfixture.SelectedRepository(t, db, "team-1", "org/dependency", map[string]any{})
				site.RepositoryDependencyIDs = dbtype.JSONStringSlice{dependency.Snapshot().RepositoryID}
				dependencyID = dependency.Repository.ID
			}
			require.NoError(t, db.Create(site).Error)
			snapshot, err := siteaccess.Capture(context.Background(), db, nil, site)
			require.NoError(t, err)
			deployment := &models.Deployment{GitAccess: snapshot, Status: sitetypes.DeploymentStatusInstalling}
			deployment.ID = "deployment-1"
			deployment.SiteID = site.ID
			deployment.TeamID = site.TeamID
			require.NoError(t, db.Create(deployment).Error)
			switch change {
			case "deselect":
				require.NoError(t, db.Model(grant.Repository).Updates(map[string]any{"selected": false, "grant_version": 2}).Error)
			case "reconnect":
				require.NoError(t, db.Model(grant.SourceControl).Update("generation", 2).Error)
			case "dependency-revoked":
				require.NoError(t, db.Table("source_control_repositories").Where("id = ?", dependencyID).Updates(map[string]any{"available": false, "grant_version": 2}).Error)
			}
			queue := &callbackQueueStub{}
			task := &deploySiteTask{callback: callbackData{SiteID: site.ID, ServerID: site.ServerID, TeamID: site.TeamID, DeploymentID: deployment.ID, SiteType: "laravel", IsFirstDeploy: true}}
			_ = task.OnSuccess(context.Background(), &taskrunner.CallbackContext{DB: db, Queue: queue}, "task-1")
			require.Empty(t, queue.tasks, "obsolete completion must not install Caddy, analyze source, restart queues, or publish deployment success")
			var saved models.Deployment
			require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
			require.Equal(t, sitetypes.DeploymentStatusFailed, saved.Status, "revoked completion must settle locally instead of remaining installing")
			var savedSite models.Site
			require.NoError(t, db.First(&savedSite, "id = ?", site.ID).Error)
			require.Nil(t, savedSite.InstalledAt)
			require.NotNil(t, saved.UpdatedAt)
			require.WithinDuration(t, time.Now(), *saved.UpdatedAt, time.Minute)
		})
	}
}
