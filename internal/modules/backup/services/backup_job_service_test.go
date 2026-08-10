package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

type backupJobBroadcastRecorder struct {
	broadcast.NopModelBroadcaster
	serverID    string
	serverEvent string
	teamID      string
	teamEvent   string
	teamData    map[string]any
}

func (r *backupJobBroadcastRecorder) BroadcastToServer(serverID, event string, _ any) {
	r.serverID = serverID
	r.serverEvent = event
}

func (r *backupJobBroadcastRecorder) BroadcastToTeam(teamID, event string, data any) {
	r.teamID = teamID
	r.teamEvent = event
	r.teamData, _ = data.(map[string]any)
}

func TestCreateBackupJobBroadcastsStatusToTeamActiveActions(t *testing.T) {
	_, db := newBackupServiceForTest(t)

	backup := models.Backup{
		StorageProviderID: 1,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "scheduled-backup"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)

	recorder := &backupJobBroadcastRecorder{}
	logger := zerolog.Nop()
	service := NewBackupJobService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{
				DB:          db,
				Logger:      &logger,
				Broadcaster: recorder,
			},
			Repos: repositories.NewRegistry(db),
		},
	})

	job, err := service.CreateBackupJob(
		context.Background(),
		backup.ID,
		backup.DispatchToken,
		&dto.CreateBackupJobRequest{Status: backuptypes.BackupJobStatusRunning},
	)
	require.NoError(t, err)

	require.Equal(t, backup.ServerID, recorder.serverID)
	require.Equal(t, "backup.job.status", recorder.serverEvent)
	require.Equal(t, backup.TeamID, recorder.teamID)
	require.Equal(t, "backup.job.status", recorder.teamEvent)
	require.Equal(t, job.ID, recorder.teamData["job_id"])
	require.Equal(t, backup.ID, recorder.teamData["backup_id"])
	require.Equal(t, backup.ServerID, recorder.teamData["server_id"])
	require.Equal(t, backup.TeamID, recorder.teamData["team_id"])
	require.Equal(t, "running", recorder.teamData["status"])
}
