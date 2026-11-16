package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type systemUpdateNotifier struct {
	teamID string
	sent   []systemUpdateNotification
	err    error
}

func (n *systemUpdateNotifier) SendToTeam(_ context.Context, teamID string, msg taskrunner.Notification) error {
	if n.err != nil {
		return n.err
	}
	n.teamID = teamID
	n.sent = append(n.sent, msg.(systemUpdateNotification))
	return nil
}

func (*systemUpdateNotifier) SendToChannel(context.Context, string, taskrunner.Notification) error {
	panic("unexpected direct notification")
}

type systemUpdateBroadcaster struct {
	broadcast.NopBroadcaster
	events []map[string]any
}

func (b *systemUpdateBroadcaster) BroadcastToTeam(teamID, event string, data any) {
	b.events = append(b.events, map[string]any{"team": teamID, "event": event, "data": data})
}

func newSystemUpdateJob(t *testing.T) (*ReadSystemUpdatesJob, *taskrunner.FakeDispatcher, *systemUpdateNotifier, *systemUpdateBroadcaster) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "updates.sqlite")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}, &models.SystemUpdateMonitor{}))

	ip := "192.0.2.6"
	server := models.Server{
		BaseModel:  basemodels.BaseModel{ID: "server-a"},
		TeamScoped: basemodels.TeamScoped{TeamID: "team-a"},
		Name:       "web",
		Status:     types.ServerStatusRunning,
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString("fixture-key"),
	}
	require.NoError(t, db.Create(&server).Error)
	require.NoError(t, db.Create(&models.SystemUpdateMonitor{ServerID: server.ID}).Error)

	notifier := &systemUpdateNotifier{}
	broadcaster := &systemUpdateBroadcaster{}
	dispatcher := taskrunner.NewFakeDispatcher()
	log := zerolog.Nop()
	job := ReadSystemUpdatesJob{
		Deps: &JobDeps{
			Deps:           &pkgjobs.Deps{DB: db, Logger: &log, Dispatcher: dispatcher, Broadcaster: broadcaster},
			Repos:          repositories.NewRegistry(db),
			TaskRunnerDeps: &tasks.TaskRunnerDeps{Notifier: notifier},
		},
		Payload: ReadSystemUpdatesPayload{ServerID: server.ID},
	}
	return &job, dispatcher, notifier, broadcaster
}

func TestSystemUpdateNotificationsAreDeduplicatedAcrossPolls(t *testing.T) {
	job, dispatcher, notifier, broadcaster := newSystemUpdateJob(t)
	db := job.Deps.DB
	ctx := context.Background()
	dispatcher.DefaultResult.Output = `{"supported":true,"managed":true,"notifications":true,"reboot_required":true,"reboot_id":"boot-a","reports":[{"id":"run-a","finished_at":"2026-11-14T03:00:00Z","status":"succeeded","packages":[{"name":"openssl","version":"3.0.2"}]}]}`
	require.NoError(t, job.Handle(ctx))
	require.NoError(t, job.Handle(ctx))
	sent := notifier.sent
	require.Len(t, sent, 2)
	require.Len(t, broadcaster.events, 1)
	require.Equal(t, map[string]any{
		"team": "team-a", "event": "server.system_updates.updated",
		"data": map[string]any{"team_id": "team-a", "server_id": "server-a"},
	}, broadcaster.events[0])
	require.Equal(t, "team-a", notifier.teamID)
	require.Contains(t, sent[0].RawText(), "openssl 3.0.2")
	require.Equal(t, "Server reboot required", sent[1].ToEmail().Subject)

	dispatcher.DefaultResult.Output = `{"supported":true,"managed":true,"notifications":false,"reboot_required":true,"reboot_id":"boot-b","reports":[{"id":"run-b","finished_at":"2026-11-15T03:00:00Z"}]}`
	require.NoError(t, job.Handle(ctx))
	require.Len(t, notifier.sent, 2)
	require.Len(t, broadcaster.events, 2)
	var monitor models.SystemUpdateMonitor
	require.NoError(t, db.First(&monitor, "server_id = ?", job.Payload.ServerID).Error)
	require.Equal(t, "run-b", monitor.LastRunID)
	require.Equal(t, "boot-b", monitor.RebootID)

	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", job.Payload.ServerID).Update("archived_at", time.Now()).Error)
	require.NoError(t, job.Handle(ctx))
	require.Len(t, dispatcher.Executions, 3)
}

func TestSystemUpdateLiveStateTracksHostChangesWithoutNotifications(t *testing.T) {
	job, dispatcher, notifier, broadcaster := newSystemUpdateJob(t)
	state := tasks.SystemUpdateResult{SystemUpdateStatus: tasks.SystemUpdateStatus{
		SystemUpdateSettings: tasks.SystemUpdateSettings{Enabled: true, Schedule: "03:00"},
		Supported:            true, Managed: true, TimerActive: true,
	}}
	observe := func(expectedEvents int) {
		t.Helper()
		payload, err := json.Marshal(state)
		require.NoError(t, err)
		dispatcher.DefaultResult.Output = string(payload)
		require.NoError(t, job.Handle(context.Background()))
		require.Len(t, broadcaster.events, expectedEvents)
	}
	observe(1)
	observe(1)
	state.Running = true
	observe(2)
	state.Running = false
	state.MonthlyPatches = 2
	observe(3)
	state.RebootID = "boot-a"
	state.RebootRequired = true
	observe(4)
	state.RebootID = ""
	state.RebootRequired = false
	observe(5)
	state.TimerActive = false
	observe(6)
	state.Managed = false
	observe(7)
	require.Empty(t, notifier.sent)
}

func TestSystemUpdateNotificationFailureDoesNotBlockLiveStateAndRetriesReport(t *testing.T) {
	job, dispatcher, notifier, broadcaster := newSystemUpdateJob(t)
	dispatcher.DefaultResult.Output = `{"supported":true,"managed":true,"notifications":true,"reports":[{"id":"run-a","finished_at":"2026-11-14T03:00:00Z","status":"failed"}]}`
	notifier.err = errors.New("channel unavailable")
	require.Error(t, job.Handle(context.Background()))
	require.Len(t, broadcaster.events, 1)
	var monitor models.SystemUpdateMonitor
	require.NoError(t, job.Deps.DB.First(&monitor, "server_id = ?", job.Payload.ServerID).Error)
	require.Empty(t, monitor.LastRunID)
	notifier.err = nil
	require.NoError(t, job.Handle(context.Background()))
	require.Len(t, notifier.sent, 1)
	require.Contains(t, notifier.sent[0].RawText(), "Security updates failed")
	require.Len(t, broadcaster.events, 1)
}

func TestSystemUpdateEnablingNotificationsSkipsRunsCompletedWhileOptedOut(t *testing.T) {
	job, dispatcher, notifier, broadcaster := newSystemUpdateJob(t)
	dispatcher.DefaultResult.Output = `{"supported":true,"managed":true,"notifications":true,"reports":[{"id":"run-a","finished_at":"2026-11-14T03:00:00Z","status":"succeeded","notifications":false},{"id":"run-b","finished_at":"2026-11-15T03:00:00Z","status":"succeeded","notifications":true}]}`
	require.NoError(t, job.Handle(context.Background()))
	require.Len(t, notifier.sent, 1)
	require.Contains(t, notifier.sent[0].RawText(), "2026-11-15")
	require.Len(t, broadcaster.events, 1)
	var monitor models.SystemUpdateMonitor
	require.NoError(t, job.Deps.DB.First(&monitor, "server_id = ?", job.Payload.ServerID).Error)
	require.Equal(t, "run-b", monitor.LastRunID)
}
