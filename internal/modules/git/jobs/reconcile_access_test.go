package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReconcileSponsorRemovalIsAccountScopedAndRejectsStaleEvidence(t *testing.T) {
	for _, reconnect := range []bool{false, true} {
		t.Run(fmt.Sprintf("reconnect=%v", reconnect), func(t *testing.T) {
			webhook, db := githubEventFixture(t)
			redis := miniredis.RunT(t)
			webhook.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
			t.Cleanup(func() { webhook.Deps.Queue.Close() })
			for _, team := range []string{"a", "b"} {
				var repo models.SourceControlRepository
				require.NoError(t, db.Where("source_control_id = ? AND selected = ?", "connection-"+team, true).First(&repo).Error)
				lease := models.GitHubCredentialLease{TeamID: team, SourceControlID: repo.SourceControlID, RepositoryID: repo.ID, ConnectionGeneration: 1, GrantVersion: 1, Status: "released", ExpiresAt: time.Now().Add(time.Hour)}
				lease.ID = "lease-" + team
				require.NoError(t, db.Create(&lease).Error)
				manifest := models.GitHubCleanupManifest{TeamID: team, SourceControlID: repo.SourceControlID, RepositoryID: fmt.Sprint(repo.ID), ConnectionGeneration: 1, GrantVersion: 1, Resources: "[]", Status: "registered"}
				manifest.ID = "cleanup-" + team
				require.NoError(t, db.Create(&manifest).Error)
			}
			require.NoError(t, db.Exec("UPDATE users SET status = 'suspended' WHERE id = 'alice'").Error)
			if reconnect {
				require.NoError(t, db.Exec("INSERT INTO team_user VALUES ('a','bob','admin')").Error)
				// Commit a reconnect after the old sponsor check but before its
				// conditional update, without relying on timing or sleeps.
				changed := false
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:reconnect", func(tx *gorm.DB) {
					if tx.Statement.Table != "source_controls" || changed {
						return
					}
					changed = true
					require.NoError(t, tx.Exec("UPDATE source_controls SET generation = 2, git_hub_authorization_id = 'auth-b' WHERE id = 'connection-a'").Error)
				}))
			}
			job := ReconcileGitHubAccessJob{Deps: webhook.Deps}
			require.NoError(t, job.Handle(context.Background()))
			for _, team := range []string{"a", "b"} {
				var sc models.SourceControl
				var lease models.GitHubCredentialLease
				var manifest models.GitHubCleanupManifest
				require.NoError(t, db.First(&sc, "id = ?", "connection-"+team).Error)
				require.NoError(t, db.First(&lease, "id = ?", "lease-"+team).Error)
				require.NoError(t, db.First(&manifest, "id = ?", "cleanup-"+team).Error)
				if team == "a" && !reconnect {
					require.Equal(t, "reconnect_required", sc.Status)
					require.Equal(t, "revoke_pending", lease.Status)
					require.Equal(t, "pending", manifest.Status)
				} else {
					require.Equal(t, "active", sc.Status)
					require.Equal(t, "released", lease.Status)
					require.Equal(t, "registered", manifest.Status)
				}
				if team == "a" {
					require.Equal(t, uint64(2), sc.Generation)
				} else {
					require.Equal(t, uint64(1), sc.Generation)
				}
			}
		})
	}
}
