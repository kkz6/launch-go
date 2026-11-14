package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/database/serializers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func inventoryFixture(t *testing.T) (*DeploymentService, *gorm.DB, *models.Site) {
	t.Helper()
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(t, db)
	site := createTestSite(t, db, nil, nil)
	configureInventoryProbe(t, svc, db, site)
	return svc, db, site
}

func configureInventoryProbe(t *testing.T, svc *DeploymentService, db *gorm.DB, site *models.Site) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &servermodels.Task{}))
	require.NoError(t, db.Model(site).Updates(map[string]any{"zero_downtime_deployment": true, "php_version": sitetypes.PhpVersion84}).Error)
	server := &servermodels.Server{BaseModel: basemodels.BaseModel{ID: site.ServerID}, Name: "Test server"}
	server.TeamID = site.TeamID
	require.NoError(t, db.Create(server).Error)
	svc.releaseProbe = func(ctx context.Context, gotServer *servermodels.Server, gotSite *models.Site) (*taskrunner.SSHCommandResult, error) {
		require.Equal(t, site.ServerID, gotServer.ID)
		require.Equal(t, site.User, gotSite.User)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 12*time.Second)
		return &taskrunner.SSHCommandResult{Stdout: inventoryTestOutput("live", "old", "live")}, nil
	}
}

func inventoryTestOutput(active string, releases ...string) string {
	output := "LAUNCH_RELEASES_V1\nactive\t" + active + "\n"
	for _, release := range releases {
		output += "release\t" + release + "\n"
	}
	return output + "END_LAUNCH_RELEASES_V1\n"
}

func inventoryDeployment(t *testing.T, db *gorm.DB, site *models.Site, id, release string, status sitetypes.DeploymentStatus) *models.Deployment {
	t.Helper()
	d := &models.Deployment{BaseModel: basemodels.BaseModel{ID: id}, Status: status}
	d.ReleaseName = stringToPtr(release)
	d.SiteID, d.TeamID = site.ID, site.TeamID
	require.NoError(t, db.Create(d).Error)
	return d
}

func TestReleaseInventoryUsesLiveLinkEvenWhenDeploymentFailed(t *testing.T) {
	svc, db, site := inventoryFixture(t)
	inventoryDeployment(t, db, site, "original", "old", sitetypes.DeploymentStatusFinished)
	inventoryDeployment(t, db, site, "rollback", "old", sitetypes.DeploymentStatusFinished)
	inventoryDeployment(t, db, site, "failed-but-live", "live", sitetypes.DeploymentStatusFailed)
	inventoryDeployment(t, db, site, "newer-pruned", "pruned", sitetypes.DeploymentStatusFinished)
	inventoryDeployment(t, db, site, "invalid", "../old", sitetypes.DeploymentStatusFinished)
	inventoryDeployment(t, db, site, "unknown", "", sitetypes.DeploymentStatusFinished)

	got, err := svc.ReleaseInventory(context.Background(), site.ID, site.ServerID, site.TeamID)
	require.NoError(t, err)
	require.Equal(t, "available", got.Status)
	require.Equal(t, "live", *got.ActiveRelease)
	require.Len(t, got.Releases, 2)
	require.Equal(t, "rollback", got.Releases[0].DeploymentID)
	require.Equal(t, []string{"rollback", "original"}, got.Releases[0].DeploymentIDs)
	require.True(t, got.Releases[0].RollbackAvailable)
	require.Equal(t, []string{"failed-but-live"}, got.Releases[1].DeploymentIDs)
	require.True(t, got.Releases[1].Active)
	require.Equal(t, "failed", got.Releases[1].Status)
	require.False(t, got.Releases[1].RollbackAvailable)
	require.Empty(t, got.Releases[1].DeploymentID)
	_, err = time.Parse(time.RFC3339Nano, got.CheckedAt)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&servermodels.Task{}).Count(&count).Error)
	require.Zero(t, count, "observing state must not create tracked actions")

	rollback, err := svc.Rollback(context.Background(), site.ID, site.ServerID, site.TeamID, "rollback", "user-1")
	require.NoError(t, err)
	require.Equal(t, "failed-but-live", rollback.CommitData["rollback_from"], "rollback metadata must describe the actual live release")
}

func TestReleaseInventoryScopesBeforeConnecting(t *testing.T) {
	for _, scenario := range []string{"team", "server", "server owner"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, site := inventoryFixture(t)
			teamID, serverID := site.TeamID, site.ServerID
			if scenario == "team" {
				teamID = "another-team"
			}
			if scenario == "server" {
				serverID = "another-server"
			}
			if scenario == "server owner" {
				require.NoError(t, db.Model(&servermodels.Server{}).Where("id = ?", serverID).Update("team_id", "another-team").Error)
			}
			svc.releaseProbe = func(context.Context, *servermodels.Server, *models.Site) (*taskrunner.SSHCommandResult, error) {
				t.Fatal("unauthorized request reached SSH")
				return nil, nil
			}
			_, err := svc.ReleaseInventory(context.Background(), site.ID, serverID, teamID)
			require.Error(t, err)
		})
	}
}

func TestReleaseInventoryDoesNotOfferUnknownOrUnfinishedDirectories(t *testing.T) {
	svc, db, site := inventoryFixture(t)
	inventoryDeployment(t, db, site, "failed", "failed", sitetypes.DeploymentStatusFailed)
	svc.releaseProbe = func(context.Context, *servermodels.Server, *models.Site) (*taskrunner.SSHCommandResult, error) {
		return &taskrunner.SSHCommandResult{Stdout: inventoryTestOutput("", "unknown", "failed")}, nil
	}
	got, err := svc.ReleaseInventory(context.Background(), site.ID, site.ServerID, site.TeamID)
	require.NoError(t, err)
	require.Equal(t, "available", got.Status)
	require.Nil(t, got.ActiveRelease)
	require.Len(t, got.Releases, 2)
	require.Equal(t, "unknown_deployment", got.Releases[0].UnavailableReason)
	require.Equal(t, "unfinished", got.Releases[1].UnavailableReason)
	for _, release := range got.Releases {
		require.False(t, release.RollbackAvailable)
		require.Empty(t, release.DeploymentID)
	}
}

func TestReleaseInventoryUnknownStateDoesNotGuessFromHistory(t *testing.T) {
	for _, scenario := range []string{"offline", "timeout", "busy", "malformed", "partial", "unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, site := inventoryFixture(t)
			inventoryDeployment(t, db, site, "old", "old", sitetypes.DeploymentStatusFinished)
			status := "unavailable"
			if scenario == "busy" || scenario == "unsupported" {
				status = scenario
			}
			if scenario == "unsupported" {
				require.NoError(t, db.Model(site).Update("zero_downtime_deployment", false).Error)
			}
			svc.releaseProbe = func(context.Context, *servermodels.Server, *models.Site) (*taskrunner.SSHCommandResult, error) {
				switch scenario {
				case "offline":
					return nil, errors.New("private key and server path must not be exposed")
				case "timeout":
					return &taskrunner.SSHCommandResult{ExitCode: 124, Stdout: inventoryTestOutput("old", "old")}, nil
				case "busy":
					return &taskrunner.SSHCommandResult{ExitCode: 75}, nil
				case "malformed":
					return &taskrunner.SSHCommandResult{Stdout: "untrusted output"}, nil
				case "partial":
					return &taskrunner.SSHCommandResult{Stdout: inventoryTestOutput("old", "old")}, errors.New("connection lost")
				default:
					t.Fatal("unsupported site reached SSH")
					return nil, nil
				}
			}
			got, err := svc.ReleaseInventory(context.Background(), site.ID, site.ServerID, site.TeamID)
			require.NoError(t, err)
			require.Equal(t, status, got.Status)
			require.Nil(t, got.ActiveRelease)
			require.Empty(t, got.Releases)
			data, err := json.Marshal(got)
			require.NoError(t, err)
			require.Contains(t, string(data), `"releases":[]`)
			require.NotContains(t, string(data), "private key")
		})
	}
}

func TestReleaseInventoryRecoversOnlyScopedExactLegacyIdentityWithoutWriting(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	svc, db, site := inventoryFixture(t)
	name := "20261109120000"
	svc.releaseProbe = func(context.Context, *servermodels.Server, *models.Site) (*taskrunner.SSHCommandResult, error) {
		return &taskrunner.SSHCommandResult{Stdout: inventoryTestOutput("", name)}, nil
	}
	for _, scenario := range []string{"valid", "other server", "other site", "ambiguous", "missing", "invalid recorded"} {
		d := inventoryDeployment(t, db, site, scenario, "", sitetypes.DeploymentStatusFinished)
		task := &servermodels.Task{BaseModel: basemodels.BaseModel{ID: scenario}, Script: dbtype.EncryptedString("DEPLOYMENT_KEEP=\"" + name + "\"\n")}
		task.ServerID, task.SiteID = site.ServerID, &site.ID
		if scenario == "other server" {
			task.ServerID = "another-server"
		}
		if scenario == "other site" {
			task.SiteID = stringToPtr("another-site")
		}
		if scenario == "ambiguous" {
			task.Script += task.Script
		}
		if scenario != "missing" {
			require.NoError(t, db.Create(task).Error)
		}
		require.NoError(t, db.Model(d).Update("task_id", task.ID).Error)
		if scenario == "invalid recorded" {
			require.NoError(t, db.Model(d).Update("release_name", "../bad").Error)
		}
	}
	got, err := svc.ReleaseInventory(context.Background(), site.ID, site.ServerID, site.TeamID)
	require.NoError(t, err)
	require.Len(t, got.Releases, 1)
	require.Equal(t, []string{"valid"}, got.Releases[0].DeploymentIDs)
	require.True(t, got.Releases[0].RollbackAvailable)
	var unchanged models.Deployment
	require.NoError(t, db.First(&unchanged, "id = ?", "valid").Error)
	require.Nil(t, unchanged.ReleaseName)
}

func TestRollbackPreflightRejectsUnavailableTargetsWithoutNewRows(t *testing.T) {
	for _, scenario := range []string{"active", "pruned", "missing identity", "invalid identity", "failed", "pending", "missing PHP", "invalid PHP"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, site := inventoryFixture(t)
			name := map[string]string{"active": "live", "pruned": "pruned", "missing identity": "", "invalid identity": "../old", "failed": "old", "pending": "old", "missing PHP": "old", "invalid PHP": "old"}[scenario]
			status := sitetypes.DeploymentStatusFinished
			if scenario == "failed" {
				status = sitetypes.DeploymentStatusFailed
			}
			target := inventoryDeployment(t, db, site, "target", name, status)
			wantCount := int64(1)
			if scenario == "missing PHP" {
				require.NoError(t, db.Model(site).Update("php_version", nil).Error)
			}
			if scenario == "invalid PHP" {
				require.NoError(t, db.Model(site).Update("php_version", "invalid").Error)
			}
			if scenario == "pending" {
				inventoryDeployment(t, db, site, "operation", "", sitetypes.DeploymentStatusPending)
				wantCount++
				svc.releaseProbe = func(context.Context, *servermodels.Server, *models.Site) (*taskrunner.SSHCommandResult, error) {
					t.Fatal("active deployment should be checked before SSH")
					return nil, nil
				}
			}
			got, err := svc.Rollback(context.Background(), site.ID, site.ServerID, site.TeamID, target.ID, "user-1")
			require.Error(t, err)
			require.Nil(t, got)
			var count int64
			require.NoError(t, db.Model(&models.Deployment{}).Count(&count).Error)
			require.Equal(t, wantCount, count)
		})
	}
}
