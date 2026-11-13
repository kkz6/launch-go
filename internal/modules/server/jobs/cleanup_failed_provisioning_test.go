package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func newCleanupTestJob(t *testing.T, response int) (*CleanupFailedProvisioningJob, *gorm.DB, *deleteBroadcast, *int) {
	t.Helper()
	deletion, db, events, calls := newDeleteTestJob(t, response)
	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Updates(map[string]any{
		"status": types.ServerStatusFailed, "connected": true, "host_key": "pinned-host-key",
		"public_ipv4": "192.0.2.10", "private_ipv4": "10.0.0.10",
	}).Error)
	return &CleanupFailedProvisioningJob{Deps: deletion.Deps, Payload: CleanupFailedProvisioningPayload{
		ServerID: "server-a", TeamID: "team-a", ServerProviderID: "provider-a", Reason: "provisioning failed",
	}}, db, events, calls
}

func requireCleanupConnectionRetained(t *testing.T, db *gorm.DB, status types.ServerStatus) {
	t.Helper()
	var server models.Server
	require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
	require.Equal(t, status, server.Status)
	require.True(t, server.Connected)
	require.Equal(t, "pinned-host-key", server.HostKey)
	require.NotNil(t, server.PublicIPv4)
	require.NotNil(t, server.PrivateIPv4)
	require.Equal(t, "192.0.2.10", *server.PublicIPv4)
	require.Equal(t, "10.0.0.10", *server.PrivateIPv4)
	require.NoError(t, db.First(&models.SSHKey{}, "id = ?", "key-a").Error)
	require.NoError(t, db.First(&models.ServerSSHKey{}, "server_id = ? AND ssh_key_id = ?", "server-a", "key-a").Error)
}

func TestCleanupFailedProvisioningScopesOwnershipAndSkipsStaleJobs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status types.ServerStatus
	}{
		{"wrong team", types.ServerStatusFailed},
		{"missing server", types.ServerStatusFailed},
		{"recovered", types.ServerStatusRunning},
		{"retrying", types.ServerStatusProvisioning},
		{"archived", types.ServerStatusArchived},
		{"deleting", types.ServerStatusDeleting},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job, db, events, calls := newCleanupTestJob(t, http.StatusNoContent)
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("status", tc.status).Error)
			switch tc.name {
			case "wrong team":
				job.Payload.TeamID = "team-b"
			case "missing server":
				job.Payload.ServerID = "missing"
			}
			err := job.Handle(context.Background())
			if tc.status == types.ServerStatusFailed {
				require.Error(t, err)
				job.Failed(context.Background(), err)
			} else {
				require.NoError(t, err)
			}
			requireCleanupConnectionRetained(t, db, tc.status)
			require.Empty(t, events.events)
			require.Zero(t, *calls)
		})
	}
}

func TestCleanupFailedProvisioningFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setup    func(*testing.T, *CleanupFailedProvisioningJob, *gorm.DB)
		rejected bool
	}{
		{"provider failure", func(*testing.T, *CleanupFailedProvisioningJob, *gorm.DB) {}, false},
		{"wrong credential team", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.ServerProvider{}).Where("id = ?", "provider-a").Update("team_id", "team-b").Error)
		}, false},
		{"wrong credential provider", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.ServerProvider{}).Where("id = ?", "provider-a").Update("provider", types.ProviderHetzner).Error)
		}, false},
		{"missing current credential", func(t *testing.T, job *CleanupFailedProvisioningJob, db *gorm.DB) {
			job.Payload.ServerProviderID = ""
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("server_provider_id", nil).Error)
		}, false},
		{"payload account mismatch", func(_ *testing.T, job *CleanupFailedProvisioningJob, _ *gorm.DB) {
			job.Payload.ServerProviderID = "different-account"
		}, true},
		{"payload account with missing current reference", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("server_provider_id", nil).Error)
		}, true},
		{"missing instance ID", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider_data", dbtype.JSONMap{}).Error)
		}, false},
		{"site appeared", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Exec("INSERT INTO sites VALUES (?, ?)", "site-a", "server-a").Error)
		}, false},
		{"project appeared", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Exec("INSERT INTO docker_projects (id, server_id) VALUES (?, ?)", "project-a", "server-a").Error)
		}, false},
		{"workload query failure", func(t *testing.T, _ *CleanupFailedProvisioningJob, db *gorm.DB) {
			require.NoError(t, db.Exec("DROP TABLE sites").Error)
		}, false},
		{"custom deletion with a site", func(t *testing.T, job *CleanupFailedProvisioningJob, db *gorm.DB) {
			job.Payload.DeleteRecord = true
			require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider", types.ProviderCustom).Error)
			require.NoError(t, db.Exec("INSERT INTO sites VALUES (?, ?)", "site-a", "server-a").Error)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job, db, events, calls := newCleanupTestJob(t, http.StatusForbidden)
			tc.setup(t, job, db)
			err := job.Handle(context.Background())
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret-provider-response")
			require.Empty(t, events.events)
			if tc.name == "provider failure" {
				require.Equal(t, 1, *calls)
			} else {
				require.Zero(t, *calls)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			job.Failed(ctx, fmt.Errorf("secret-provider-response: %w", err))
			requireCleanupConnectionRetained(t, db, types.ServerStatusFailed)
			if tc.rejected {
				require.Empty(t, events.events)
			} else {
				require.Equal(t, []string{"server.cleanup_failed"}, events.events)
				payload, err := json.Marshal(events.data)
				require.NoError(t, err)
				require.NotContains(t, string(payload), "secret-provider-response")
				require.NotContains(t, string(payload), "test-token")
			}
		})
	}
}

func TestCleanupFailedProvisioningRetainsRecordAfterSuccess(t *testing.T) {
	for _, kind := range []string{"cloud", "already absent", "legacy payload and instance ID", "custom"} {
		t.Run(kind, func(t *testing.T) {
			response := http.StatusNoContent
			if kind == "already absent" {
				response = http.StatusNotFound
			}
			job, db, events, calls := newCleanupTestJob(t, response)
			providerData := dbtype.JSONMap{"provider_server_id": "instance-a"}
			if kind == "legacy payload and instance ID" {
				job.Payload.ServerProviderID = ""
				providerData = dbtype.JSONMap{"droplet_id": "instance-a"}
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider_data", providerData).Error)
			}
			if kind == "custom" {
				job.Deps.ProviderFactory = nil
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider", types.ProviderCustom).Error)
				// Retaining a custom host does not delete its workloads or connection.
				require.NoError(t, db.Exec("INSERT INTO sites VALUES (?, ?)", "site-a", "server-a").Error)
			} else {
				require.NoError(t, db.Exec("INSERT INTO docker_projects VALUES (?, ?, CURRENT_TIMESTAMP)", "removed-project", "server-a").Error)
			}
			require.NoError(t, job.Handle(context.Background()))
			var server models.Server
			require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
			require.Equal(t, types.ServerStatusFailed, server.Status)
			require.Equal(t, providerData, server.ProviderData)
			require.NoError(t, db.First(&models.SSHKey{}, "id = ?", "key-a").Error)
			if kind == "custom" {
				requireCleanupConnectionRetained(t, db, types.ServerStatusFailed)
				require.Zero(t, *calls)
			} else {
				require.False(t, server.Connected)
				require.Empty(t, server.HostKey)
				require.Nil(t, server.PublicIPv4)
				require.Nil(t, server.PrivateIPv4)
				require.Equal(t, 1, *calls)
			}
			require.Equal(t, []string{"server.provisioning_cleanup_complete"}, events.events)
			require.Equal(t, false, events.data[0].(map[string]any)["deleted"])
		})
	}
}

type cleanupCommitBroadcast struct {
	*deleteBroadcast
	check func()
}

func (b *cleanupCommitBroadcast) BroadcastToTeam(team, event string, data any) {
	b.check()
	b.deleteBroadcast.BroadcastToTeam(team, event, data)
}

func TestCleanupFailedProvisioningDeletesRecordBeforeAnnouncingSuccess(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", custom), func(t *testing.T) {
			job, db, events, calls := newCleanupTestJob(t, http.StatusNoContent)
			job.Payload.DeleteRecord = true
			if custom {
				job.Deps.ProviderFactory = nil
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("provider", types.ProviderCustom).Error)
			}
			global := models.SSHKey{BaseModel: basemodels.BaseModel{ID: "global-key"}, Name: "global", PublicKey: "ssh-ed25519 global", IsGlobal: true}
			require.NoError(t, db.Create(&global).Error)
			require.NoError(t, db.Create(&models.ServerSSHKey{ServerID: "server-a", SSHKeyID: global.ID}).Error)
			job.Deps.Broadcaster = &cleanupCommitBroadcast{deleteBroadcast: events, check: func() {
				var count int64
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Count(&count).Error)
				require.Zero(t, count, "success events must observe committed deletion")
				require.NoError(t, db.Model(&models.SSHKey{}).Where("id = ?", "key-a").Count(&count).Error)
				require.Zero(t, count)
			}}
			require.NoError(t, job.Handle(context.Background()))
			require.NoError(t, db.First(&models.SSHKey{}, "id = ?", global.ID).Error)
			require.ElementsMatch(t, []string{"server.provisioning_cleanup_complete", "server.deleted"}, events.events)
			if custom {
				require.Zero(t, *calls)
			} else {
				require.Equal(t, 1, *calls)
			}
		})
	}
}

func TestCleanupFailedProvisioningLocalDeleteFailureRollsBackKeysAndSuppressesSuccess(t *testing.T) {
	job, db, events, calls := newCleanupTestJob(t, http.StatusNoContent)
	job.Payload.DeleteRecord = true
	require.NoError(t, db.Exec("CREATE TRIGGER reject_cleanup_delete BEFORE DELETE ON servers BEGIN SELECT RAISE(ABORT, 'local deletion failed'); END").Error)
	err := job.Handle(context.Background())
	require.ErrorContains(t, err, "local deletion failed")
	require.Equal(t, 1, *calls)
	requireCleanupConnectionRetained(t, db, types.ServerStatusFailed)
	require.Empty(t, events.events)
}

func TestCleanupFailedProvisioningFailureDoesNotOverwriteRecoveredState(t *testing.T) {
	job, db, events, _ := newCleanupTestJob(t, http.StatusForbidden)
	err := job.Handle(context.Background())
	require.Error(t, err)
	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", "server-a").Update("status", types.ServerStatusRunning).Error)
	job.Failed(context.Background(), errors.New("late failure"))
	requireCleanupConnectionRetained(t, db, types.ServerStatusRunning)
	require.Empty(t, events.events)
}
