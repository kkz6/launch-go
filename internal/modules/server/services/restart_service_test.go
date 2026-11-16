package services

import (
	"context"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

func TestRestartPersistsProgressRejectsDuplicatesAndScopesReads(t *testing.T) {
	svc, db, server, dispatcher, broadcaster := systemUpdatesServiceFixture(t)
	require.NoError(t, db.AutoMigrate(&models.ServerRestart{}))
	ctx := context.Background()
	state, err := svc.GetServerRestart(ctx, server.ID, server.TeamID)
	require.NoError(t, err)
	require.Equal(t, "idle", state.Status)
	require.NoError(t, svc.RebootServer(ctx, server.ID, server.TeamID, "user-a"))
	state, err = svc.GetServerRestart(ctx, server.ID, server.TeamID)
	require.NoError(t, err)
	require.Equal(t, "queued", state.Status)
	require.NotNil(t, state.RequestedAt)
	require.NotEmpty(t, state.OperationID)
	require.False(t, state.CanVerify)
	requireSystemUpdatesError(t, svc.RebootServer(ctx, server.ID, server.TeamID, "user-a"), fiber.StatusConflict)
	_, err = svc.GetServerRestart(ctx, server.ID, "another-team")
	requireSystemUpdatesError(t, err, fiber.StatusNotFound)
	requireSystemUpdatesError(t, svc.RebootServer(ctx, server.ID, "another-team", "user-a"), fiber.StatusNotFound)
	_, err = svc.VerifyServerRestart(ctx, server.ID, "another-team")
	requireSystemUpdatesError(t, err, fiber.StatusNotFound)
	require.Equal(t, []string{"server.reboot.updated"}, broadcaster.events)
	require.Empty(t, dispatcher.Executions, "progress reads must not require an online host")
}

func TestRestartGetExpiresStalledOperationAndCanSafelyVerifyOriginalBoot(t *testing.T) {
	svc, db, server, dispatcher, _ := systemUpdatesServiceFixture(t)
	require.NoError(t, db.AutoMigrate(&models.ServerRestart{}))
	ctx := context.Background()
	require.NoError(t, svc.RebootServer(ctx, server.ID, server.TeamID, "user-a"))
	require.NoError(t, db.Model(&models.ServerRestart{}).Where("server_id = ?", server.ID).Updates(map[string]any{
		"status": "verifying", "boot_id": "12345678-1234-1234-1234-123456789abc", "deadline_at": time.Now().Add(-time.Second),
	}).Error)
	state, err := svc.GetServerRestart(ctx, server.ID, server.TeamID)
	require.NoError(t, err)
	require.Equal(t, "failed", state.Status)
	require.True(t, state.CanVerify)
	operation := state.OperationID
	state, err = svc.VerifyServerRestart(ctx, server.ID, server.TeamID)
	require.NoError(t, err)
	require.Equal(t, "verifying", state.Status)
	require.NotEqual(t, operation, state.OperationID, "each recheck must fence old probes and timeout snapshots")
	require.Equal(t, "12345678-1234-1234-1234-123456789abc", state.BootID)
	require.True(t, state.DeadlineAt.After(time.Now()))
	require.Empty(t, dispatcher.Executions, "safe verification never issues a reboot command")
}

func TestRestartArchivedAndUnknownBaselineHaveActionableErrors(t *testing.T) {
	svc, db, server, _, _ := systemUpdatesServiceFixture(t)
	require.NoError(t, db.AutoMigrate(&models.ServerRestart{}))
	ctx := context.Background()
	_, err := svc.VerifyServerRestart(ctx, server.ID, server.TeamID)
	requireSystemUpdatesError(t, err, fiber.StatusConflict)
	require.NoError(t, db.Model(&server).Update("archived_at", time.Now()).Error)
	requireSystemUpdatesError(t, svc.RebootServer(ctx, server.ID, server.TeamID, "user-a"), fiber.StatusConflict)
	_, err = svc.VerifyServerRestart(ctx, server.ID, server.TeamID)
	requireSystemUpdatesError(t, err, fiber.StatusConflict)
}

func TestRestartGuardPreventsUpdateActionsDuringRestart(t *testing.T) {
	svc, db, server, _, _ := systemUpdatesServiceFixture(t)
	require.NoError(t, db.AutoMigrate(&models.ServerRestart{}))
	ctx := context.Background()
	require.NoError(t, svc.ensureNoServerRestart(ctx, server.ID))
	require.NoError(t, svc.RebootServer(ctx, server.ID, server.TeamID, "user-a"))
	requireSystemUpdatesError(t, svc.ensureNoServerRestart(ctx, server.ID), fiber.StatusConflict)
}
