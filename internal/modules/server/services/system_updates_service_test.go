package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type systemUpdatesBroadcaster struct {
	broadcast.NopModelBroadcaster
	teams  []string
	events []string
	data   []any
}

func (b *systemUpdatesBroadcaster) BroadcastToTeam(teamID, event string, data any) {
	b.teams = append(b.teams, teamID)
	b.events = append(b.events, event)
	b.data = append(b.data, data)
}

func systemUpdatesServiceFixture(t *testing.T) (*Service, *gorm.DB, models.Server, *taskrunner.FakeDispatcher, *systemUpdatesBroadcaster) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}, &models.SystemUpdateMonitor{}, &models.ServerRestart{}))
	ip := "192.0.2.6"
	server := models.Server{
		BaseModel: basemodels.BaseModel{ID: "server-a"}, TeamScoped: basemodels.TeamScoped{TeamID: "team-a"},
		Status: types.ServerStatusRunning, PublicIPv4: &ip, PrivateKey: dbtype.EncryptedString("fixture-key"),
	}
	require.NoError(t, db.Create(&server).Error)
	log := zerolog.Nop()
	dispatcher := taskrunner.NewFakeDispatcher()
	dispatcher.DefaultResult.Output = `{"supported":true,"managed":true,"enabled":true,"schedule":"04:15","notifications":true,"timer_active":true}`
	ws := &systemUpdatesBroadcaster{}
	svc := NewService(ServiceDeps{
		Dependencies: pkgservice.Dependencies{DB: db, Logger: &log, Broadcaster: ws},
		Repos:        repositories.NewRegistry(db), TaskRunnerDeps: &tasks.TaskRunnerDeps{Dispatcher: dispatcher},
	})
	return svc, db, server, dispatcher, ws
}

func TestSystemUpdatesRejectsWrongTeamAndArchivedBeforeRemoteAccess(t *testing.T) {
	for _, tc := range []struct {
		name     string
		team     string
		archived bool
		code     int
	}{
		{"wrong team", "team-b", false, fiber.StatusNotFound},
		{"archived", "team-a", true, fiber.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
			if tc.archived {
				require.NoError(t, db.Model(&server).Update("archived_at", time.Now()).Error)
			}
			_, err := svc.GetSystemUpdates(context.Background(), server.ID, tc.team)
			requireSystemUpdatesError(t, err, tc.code)
			_, err = svc.UpdateSystemUpdates(context.Background(), server.ID, tc.team, "user-a", &tasks.SystemUpdateSettings{Schedule: "04:15"})
			requireSystemUpdatesError(t, err, tc.code)
			require.Empty(t, dispatcher.Executions)
			require.Empty(t, ws.events)
		})
	}
}

func TestSystemUpdatesRejectsInvalidTimesBeforeRemoteAccess(t *testing.T) {
	svc, _, server, dispatcher, _ := systemUpdatesServiceFixture(t)
	for _, schedule := range []string{"", "3:00", "24:00", "03:60", "03:00:00", "03:00 UTC", "03:00;reboot"} {
		t.Run(schedule, func(t *testing.T) {
			_, err := svc.UpdateSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a", &tasks.SystemUpdateSettings{Schedule: schedule})
			requireSystemUpdatesError(t, err, fiber.StatusBadRequest)
		})
	}
	require.Empty(t, dispatcher.Executions)
}

func TestSystemUpdatesReportsHostFailuresWithoutBroadcastingSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		code         int
		runError     bool
		exitCode     int
	}{
		{name: "unsupported", output: `{"supported":false}`, code: fiber.StatusBadRequest},
		{name: "busy", output: `{"error":"busy"}`, code: fiber.StatusConflict},
		{name: "missing package", output: `{"error":"unattended_upgrades_missing"}`, code: fiber.StatusBadRequest},
		{name: "invalid response", output: "not JSON", code: fiber.StatusBadRequest},
		{name: "command failed", output: "private diagnostic", code: fiber.StatusBadRequest, exitCode: 1},
		{name: "connection failed", code: fiber.StatusBadRequest, runError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
			dispatcher.DefaultResult.Output = tc.output
			dispatcher.DefaultResult.ExitCode = tc.exitCode
			if tc.runError {
				dispatcher.FailOnRun = errors.New("private diagnostic")
			}
			_, err := svc.UpdateSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a", &tasks.SystemUpdateSettings{Schedule: "04:15"})
			requireSystemUpdatesError(t, err, tc.code)
			require.NotContains(t, err.Error(), "private diagnostic")
			var count int64
			require.NoError(t, db.Model(&models.SystemUpdateMonitor{}).Count(&count).Error)
			require.EqualValues(t, 1, count)
			require.Empty(t, ws.events)
		})
	}
}

func TestSystemUpdatesCannotConfigureHostWithoutPersistingMonitor(t *testing.T) {
	svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
	require.NoError(t, db.Migrator().DropTable(&models.SystemUpdateMonitor{}))
	_, err := svc.UpdateSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a", &tasks.SystemUpdateSettings{Schedule: "04:15"})
	require.Error(t, err)
	require.Empty(t, dispatcher.Executions)
	require.Empty(t, ws.events)
}

func TestSystemUpdatesSaveRegistersMonitoringAndBroadcastsToOwningTeam(t *testing.T) {
	svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
	settings := &tasks.SystemUpdateSettings{Enabled: true, Schedule: "04:15", Notifications: true}
	status, err := svc.UpdateSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a", settings)
	require.NoError(t, err)
	require.Equal(t, *settings, status.SystemUpdateSettings)
	require.True(t, status.TimerActive)
	require.Equal(t, "root", dispatcher.LastExecution().AsUser)
	require.Contains(t, dispatcher.LastExecution().Script, "configure true '04:15' true")
	require.Equal(t, []string{"team-a"}, ws.teams)
	require.Equal(t, []string{"server.system_updates.updated"}, ws.events)
	require.Equal(t, "server-a", ws.data[0].(map[string]any)["server_id"])
	require.Equal(t, "team-a", ws.data[0].(map[string]any)["team_id"])

	monitor := models.SystemUpdateMonitor{ServerID: server.ID}
	require.NoError(t, db.Model(&monitor).Updates(map[string]any{"last_run_id": "run-1", "reboot_id": "boot-1"}).Error)
	_, err = svc.UpdateSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a", settings)
	require.NoError(t, err)
	require.NoError(t, db.First(&monitor, "server_id = ?", server.ID).Error)
	require.Equal(t, "run-1", monitor.LastRunID)
	require.Equal(t, "boot-1", monitor.RebootID)
	var count int64
	require.NoError(t, db.Model(&models.SystemUpdateMonitor{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestSystemUpdatesReadDoesNotMutateMonitorOrBroadcast(t *testing.T) {
	svc, db, server, _, ws := systemUpdatesServiceFixture(t)
	status, err := svc.GetSystemUpdates(context.Background(), server.ID, server.TeamID)
	require.NoError(t, err)
	require.True(t, status.Supported)
	var count int64
	require.NoError(t, db.Model(&models.SystemUpdateMonitor{}).Count(&count).Error)
	require.Zero(t, count)
	require.Empty(t, ws.events)
}

func requireSystemUpdatesError(t *testing.T, err error, code int) {
	t.Helper()
	var response *fiber.Error
	require.ErrorAs(t, err, &response)
	require.Equal(t, code, response.Code)
}

func TestSystemUpdateRemediesPreserveSavedSettingsAndBroadcast(t *testing.T) {
	for _, action := range []string{"retry", "repair"} {
		t.Run(action, func(t *testing.T) {
			svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
			var status tasks.SystemUpdateStatus
			var err error
			if action == "retry" {
				status, err = svc.RetrySystemUpdates(context.Background(), server.ID, server.TeamID, "user-a")
			} else {
				status, err = svc.RepairSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a")
			}
			require.NoError(t, err)
			require.Equal(t, "04:15", status.Schedule)
			require.Contains(t, dispatcher.LastExecution().Script, "python3 - "+action+" ")
			require.Equal(t, "root", dispatcher.LastExecution().AsUser)
			require.Equal(t, []string{"server.system_updates.updated"}, ws.events)
			require.Equal(t, []string{server.TeamID}, ws.teams)
			var count int64
			require.NoError(t, db.Model(&models.SystemUpdateMonitor{}).Where("server_id = ?", server.ID).Count(&count).Error)
			require.EqualValues(t, 1, count)
		})
	}
}

func TestSystemUpdateLogsAreReadOnlyAndScoped(t *testing.T) {
	svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
	dispatcher.DefaultResult.Output = `{"supported":true,"logs":"package download failed"}`
	logs, err := svc.GetSystemUpdateLogs(context.Background(), server.ID, server.TeamID)
	require.NoError(t, err)
	require.Equal(t, "package download failed", logs.Logs)
	require.Contains(t, dispatcher.LastExecution().Script, "python3 - logs ")
	require.Empty(t, ws.events)
	var count int64
	require.NoError(t, db.Model(&models.SystemUpdateMonitor{}).Count(&count).Error)
	require.Zero(t, count)
	_, err = svc.GetSystemUpdateLogs(context.Background(), server.ID, "team-b")
	requireSystemUpdatesError(t, err, fiber.StatusNotFound)
	require.Len(t, dispatcher.Executions, 1)
}

func TestSystemUpdateRemediesRejectHostErrorsAndWrongTeam(t *testing.T) {
	for _, tc := range []struct {
		hostError string
		code      int
	}{
		{"busy", fiber.StatusConflict},
		{"unattended_upgrades_missing", fiber.StatusBadRequest},
		{"schedule_disabled", fiber.StatusConflict},
	} {
		t.Run(tc.hostError, func(t *testing.T) {
			svc, _, server, dispatcher, ws := systemUpdatesServiceFixture(t)
			dispatcher.DefaultResult.Output = `{"error":"` + tc.hostError + `"}`
			_, err := svc.RetrySystemUpdates(context.Background(), server.ID, server.TeamID, "user-a")
			requireSystemUpdatesError(t, err, tc.code)
			_, err = svc.RepairSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a")
			requireSystemUpdatesError(t, err, tc.code)
			require.Empty(t, ws.events)
			_, err = svc.RetrySystemUpdates(context.Background(), server.ID, "team-b", "user-a")
			requireSystemUpdatesError(t, err, fiber.StatusNotFound)
			_, err = svc.RepairSystemUpdates(context.Background(), server.ID, "team-b", "user-a")
			requireSystemUpdatesError(t, err, fiber.StatusNotFound)
			require.Len(t, dispatcher.Executions, 2)
		})
	}
}

func TestSystemUpdateMutationsWaitForTrackedRestart(t *testing.T) {
	svc, db, server, dispatcher, ws := systemUpdatesServiceFixture(t)
	require.NoError(t, db.Create(&models.ServerRestart{ServerID: server.ID, OperationID: "operation-a", Status: "verifying"}).Error)
	_, err := svc.UpdateSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a", &tasks.SystemUpdateSettings{Schedule: "04:15"})
	requireSystemUpdatesError(t, err, fiber.StatusConflict)
	_, err = svc.RetrySystemUpdates(context.Background(), server.ID, server.TeamID, "user-a")
	requireSystemUpdatesError(t, err, fiber.StatusConflict)
	_, err = svc.RepairSystemUpdates(context.Background(), server.ID, server.TeamID, "user-a")
	requireSystemUpdatesError(t, err, fiber.StatusConflict)
	require.Empty(t, dispatcher.Executions)
	require.Empty(t, ws.events)
	_, err = svc.GetSystemUpdateLogs(context.Background(), server.ID, server.TeamID)
	require.NoError(t, err)
	require.Len(t, dispatcher.Executions, 1)
}
