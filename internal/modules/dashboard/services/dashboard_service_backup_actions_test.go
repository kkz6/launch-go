package services

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dashboard/dto"
)

func TestActiveActionsIncludesAndMapsBackupRuns(t *testing.T) {
	t.Parallel()

	service, db := activeActionsBackupFixture(t)
	now := time.Now().UTC().Truncate(time.Second)

	activeActionsInsertBaseTargets(t, db)
	activeActionsExec(t, db, `
		INSERT INTO tasks
			(id, server_id, site_id, name, status, created_at, updated_at)
		VALUES
			('server-task-1', 'server-1', NULL, 'Server backup', 'running', ?, ?),
			('database-task-1', 'server-1', NULL, 'Database backup', 'running', ?, ?)
	`, now.Add(-3*time.Minute), now.Add(-3*time.Minute), now.Add(-time.Minute), now.Add(-time.Minute))
	activeActionsExec(t, db, `
		INSERT INTO backup_jobs
			(id, team_id, backup_id, status, task_id, error, created_at, updated_at)
		VALUES
			(?, 'team-1', 'server-backup-1', 'pending', NULL, NULL, ?, ?),
			(?, 'team-1', 'server-backup-1', 'running', 'server-task-1', NULL, ?, ?)
	`,
		"server-pending", now.Add(-4*time.Minute), now.Add(-4*time.Minute),
		"server-running", now.Add(-3*time.Minute), now.Add(-3*time.Minute),
	)
	activeActionsExec(t, db, `
		INSERT INTO docker_database_backup_runs
			(id, backup_id, status, task_id, error, started_at, finished_at, created_at, updated_at)
		VALUES
			(?, 'database-backup-1', 'triggered', NULL, NULL, ?, NULL, ?, ?),
			(?, 'database-backup-1', 'running', 'database-task-1', NULL, ?, NULL, ?, ?)
	`,
		"database-triggered", now.Add(-2*time.Minute), now.Add(-2*time.Minute), now.Add(-2*time.Minute),
		"database-running", now.Add(-time.Minute), now.Add(-time.Minute), now.Add(-time.Minute),
	)

	actions, err := service.ActiveActions(context.Background(), "team-1")
	require.NoError(t, err)
	require.Equal(t, []string{
		"database-running",
		"database-triggered",
		"server-running",
		"server-pending",
	}, activeActionIDs(actions))

	databaseRunning := activeActionByID(t, actions, "database-running")
	require.Equal(t, "database_backup", databaseRunning.Kind)
	require.Equal(t, "running", databaseRunning.Status)
	require.Equal(t, "Primary database", databaseRunning.Label)
	require.Empty(t, databaseRunning.Description)
	require.Equal(t, "server-1", databaseRunning.ServerID)
	require.Equal(t, "project-1", databaseRunning.ProjectID)
	require.Equal(t, "database", databaseRunning.TargetType)
	require.Equal(t, "database-1", databaseRunning.TargetID)
	require.NotNil(t, databaseRunning.TaskID)
	require.Equal(t, "database-task-1", *databaseRunning.TaskID)
	require.NotNil(t, databaseRunning.StartedAt)
	require.WithinDuration(t, now.Add(-time.Minute), *databaseRunning.StartedAt, time.Second)

	databaseTriggered := activeActionByID(t, actions, "database-triggered")
	require.Equal(t, "pending", databaseTriggered.Status, "triggered is normalized for the common action vocabulary")
	require.Nil(t, databaseTriggered.TaskID)

	serverRunning := activeActionByID(t, actions, "server-running")
	require.Equal(t, "server_backup", serverRunning.Kind)
	require.Equal(t, "running", serverRunning.Status)
	require.Equal(t, "Production", serverRunning.Label)
	require.Empty(t, serverRunning.Description)
	require.Equal(t, "server-1", serverRunning.ServerID)
	require.Empty(t, serverRunning.ProjectID)
	require.Equal(t, "server", serverRunning.TargetType)
	require.Equal(t, "server-1", serverRunning.TargetID)
	require.NotNil(t, serverRunning.TaskID)
	require.Equal(t, "server-task-1", *serverRunning.TaskID)
	require.NotNil(t, serverRunning.StartedAt)
	require.WithinDuration(t, now.Add(-3*time.Minute), *serverRunning.StartedAt, time.Second)

	serverPending := activeActionByID(t, actions, "server-pending")
	require.Equal(t, "pending", serverPending.Status)
	require.Nil(t, serverPending.TaskID)
	for _, action := range actions {
		require.NotEqual(t, "task", action.Kind, "backup tasks must use the backup-specific action")
	}
}

func TestActiveActionsRetainsRecentBackupFailuresWithinTeam(t *testing.T) {
	t.Parallel()

	service, db := activeActionsBackupFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	recentServerFailure := now.Add(-2 * time.Minute)
	recentDatabaseFailure := now.Add(-time.Minute)
	staleFailure := now.Add(-2 * failedBackupActionRetention)

	activeActionsInsertBaseTargets(t, db)
	activeActionsExec(t, db, `
		INSERT INTO tasks
			(id, server_id, site_id, name, status, created_at, updated_at)
		VALUES
			('server-failed-task', 'server-1', NULL, 'Server backup', 'failed', ?, ?),
			('database-failed-task', 'server-1', NULL, 'Database backup', 'failed', ?, ?)
	`, recentServerFailure, recentServerFailure, recentDatabaseFailure, recentDatabaseFailure)
	activeActionsExec(t, db, `
		INSERT INTO backup_jobs
			(id, team_id, backup_id, status, task_id, error, created_at, updated_at)
		VALUES
			('server-recent-failure', 'team-1', 'server-backup-1', 'failed', 'server-failed-task', 'server credentials are invalid', ?, ?),
			('server-stale-failure', 'team-1', 'server-backup-1', 'failed', NULL, 'old server failure', ?, ?),
			('server-finished', 'team-1', 'server-backup-1', 'finished', 'finished-task', NULL, ?, ?),
			('server-other-team', 'team-2', 'server-backup-2', 'running', NULL, NULL, ?, ?),
			('server-job-team-mismatch', 'team-1', 'server-backup-2', 'running', NULL, NULL, ?, ?),
			('server-target-team-mismatch', 'team-1', 'server-backup-cross-server', 'running', NULL, NULL, ?, ?)
	`,
		recentServerFailure, recentServerFailure,
		staleFailure, staleFailure,
		now, now,
		now, now,
		now, now,
		now, now,
	)
	activeActionsExec(t, db, `
		INSERT INTO docker_database_backup_runs
			(id, backup_id, status, task_id, error, started_at, finished_at, created_at, updated_at)
		VALUES
			('database-recent-failure', 'database-backup-1', 'failed', 'database-failed-task', 'database upload failed', ?, ?, ?, ?),
			('database-stale-failure', 'database-backup-1', 'failed', NULL, 'old database failure', ?, ?, ?, ?),
			('database-success', 'database-backup-1', 'success', 'success-task', NULL, ?, ?, ?, ?),
			('database-other-team', 'database-backup-2', 'running', NULL, NULL, ?, NULL, ?, ?),
			('database-config-team-mismatch', 'database-backup-cross-config', 'running', NULL, NULL, ?, NULL, ?, ?),
			('database-target-team-mismatch', 'database-backup-cross-database', 'running', NULL, NULL, ?, NULL, ?, ?),
			('database-server-team-mismatch', 'database-backup-cross-server', 'running', NULL, NULL, ?, NULL, ?, ?),
			('database-project-team-mismatch', 'database-backup-cross-project-team', 'running', NULL, NULL, ?, NULL, ?, ?),
			('database-project-server-mismatch', 'database-backup-cross-project-server', 'running', NULL, NULL, ?, NULL, ?, ?)
	`,
		recentDatabaseFailure, recentDatabaseFailure, recentDatabaseFailure, recentDatabaseFailure,
		staleFailure, staleFailure, staleFailure, staleFailure,
		now, now, now, now,
		now, now, now,
		now, now, now,
		now, now, now,
		now, now, now,
		now, now, now,
		now, now, now,
	)

	actions, err := service.ActiveActions(context.Background(), "team-1")
	require.NoError(t, err)
	require.Equal(t, []string{
		"database-recent-failure",
		"server-recent-failure",
	}, activeActionIDs(actions))

	databaseFailure := activeActionByID(t, actions, "database-recent-failure")
	require.Equal(t, "failed", databaseFailure.Status)
	require.Equal(t, "database upload failed", databaseFailure.Description)
	require.Equal(t, "database", databaseFailure.TargetType)
	require.Equal(t, "database-1", databaseFailure.TargetID)
	require.Equal(t, "database-failed-task", *databaseFailure.TaskID)

	serverFailure := activeActionByID(t, actions, "server-recent-failure")
	require.Equal(t, "failed", serverFailure.Status)
	require.Equal(t, "server credentials are invalid", serverFailure.Description)
	require.Equal(t, "server", serverFailure.TargetType)
	require.Equal(t, "server-1", serverFailure.TargetID)
	require.Equal(t, "server-failed-task", *serverFailure.TaskID)

	for _, excludedID := range []string{
		"server-stale-failure",
		"server-finished",
		"server-other-team",
		"server-job-team-mismatch",
		"server-target-team-mismatch",
		"database-stale-failure",
		"database-success",
		"database-other-team",
		"database-config-team-mismatch",
		"database-target-team-mismatch",
		"database-server-team-mismatch",
		"database-project-team-mismatch",
		"database-project-server-mismatch",
	} {
		require.NotContains(t, activeActionIDs(actions), excludedID)
	}
}

func TestActiveActionsReturnsBackupQueryErrors(t *testing.T) {
	t.Run("server backup query", func(t *testing.T) {
		service, db := activeActionsBackupFixture(t)
		require.NoError(t, db.Exec("DROP TABLE backup_jobs").Error)

		_, err := service.ActiveActions(context.Background(), "team-1")
		require.Error(t, err)
	})

	t.Run("database backup query", func(t *testing.T) {
		service, db := activeActionsBackupFixture(t)
		require.NoError(t, db.Exec("DROP TABLE docker_database_backup_runs").Error)

		_, err := service.ActiveActions(context.Background(), "team-1")
		require.Error(t, err)
	})
}

func activeActionsBackupFixture(t *testing.T) (*DashboardService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	for _, statement := range []string{
		`CREATE TABLE sites (id TEXT PRIMARY KEY, address TEXT, server_id TEXT)`,
		`CREATE TABLE deployments (
			id TEXT PRIMARY KEY, team_id TEXT, status TEXT, site_id TEXT,
			task_id TEXT, created_at DATETIME
		)`,
		`CREATE TABLE docker_deployments (
			id TEXT PRIMARY KEY, team_id TEXT, status TEXT, target_type TEXT,
			target_id TEXT, server_id TEXT, task_id TEXT, started_at DATETIME,
			created_at DATETIME
		)`,
		`CREATE TABLE docker_applications (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)`,
		`CREATE TABLE docker_composes (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)`,
		`CREATE TABLE commands (
			id TEXT PRIMARY KEY, team_id TEXT, site_id TEXT, command TEXT,
			status TEXT, created_at DATETIME
		)`,
		`CREATE TABLE tasks (
			id TEXT PRIMARY KEY, server_id TEXT, site_id TEXT, name TEXT,
			status TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, name TEXT)`,
		`CREATE TABLE backups (id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT)`,
		`CREATE TABLE backup_jobs (
			id TEXT PRIMARY KEY, team_id TEXT, backup_id TEXT, status TEXT,
			task_id TEXT, error TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE docker_databases (
			id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT, project_id TEXT,
			name TEXT
		)`,
		`CREATE TABLE docker_projects (
			id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT, name TEXT
		)`,
		`CREATE TABLE docker_database_backups (
			id TEXT PRIMARY KEY, team_id TEXT, database_id TEXT
		)`,
		`CREATE TABLE docker_database_backup_runs (
			id TEXT PRIMARY KEY, backup_id TEXT, status TEXT, task_id TEXT,
			error TEXT, started_at DATETIME, finished_at DATETIME,
			created_at DATETIME, updated_at DATETIME
		)`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}

	logger := zerolog.Nop()
	return NewDashboardService(db, &logger, nil), db
}

func activeActionsInsertBaseTargets(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, statement := range []string{
		`INSERT INTO servers (id, team_id, name) VALUES
			('server-1', 'team-1', 'Production'),
			('server-2', 'team-2', 'Other team')`,
		`INSERT INTO backups (id, team_id, server_id) VALUES
			('server-backup-1', 'team-1', 'server-1'),
			('server-backup-2', 'team-2', 'server-2'),
			('server-backup-cross-server', 'team-1', 'server-2')`,
		`INSERT INTO docker_databases (id, team_id, server_id, project_id, name) VALUES
			('database-1', 'team-1', 'server-1', 'project-1', 'Primary database'),
			('database-2', 'team-2', 'server-2', 'project-2', 'Other database'),
			('database-cross-server', 'team-1', 'server-2', 'project-1', 'Cross-server database'),
			('database-cross-project-team', 'team-1', 'server-1', 'project-cross-team', 'Cross-team project database'),
			('database-cross-project-server', 'team-1', 'server-1', 'project-cross-server', 'Cross-server project database')`,
		`INSERT INTO docker_projects (id, team_id, server_id, name) VALUES
			('project-1', 'team-1', 'server-1', 'Production project'),
			('project-2', 'team-2', 'server-2', 'Other project'),
			('project-cross-team', 'team-2', 'server-1', 'Cross-team project'),
			('project-cross-server', 'team-1', 'server-2', 'Cross-server project')`,
		`INSERT INTO docker_database_backups (id, team_id, database_id) VALUES
			('database-backup-1', 'team-1', 'database-1'),
			('database-backup-2', 'team-2', 'database-2'),
			('database-backup-cross-config', 'team-2', 'database-1'),
			('database-backup-cross-database', 'team-1', 'database-2'),
			('database-backup-cross-server', 'team-1', 'database-cross-server'),
			('database-backup-cross-project-team', 'team-1', 'database-cross-project-team'),
			('database-backup-cross-project-server', 'team-1', 'database-cross-project-server')`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
}

func activeActionsExec(t *testing.T, db *gorm.DB, statement string, args ...any) {
	t.Helper()
	require.NoError(t, db.Exec(statement, args...).Error)
}

func activeActionIDs(actions []dto.ActiveAction) []string {
	ids := make([]string, len(actions))
	for i := range actions {
		ids[i] = actions[i].ID
	}
	return ids
}

func activeActionByID(t *testing.T, actions []dto.ActiveAction, id string) dto.ActiveAction {
	t.Helper()
	for i := range actions {
		if actions[i].ID == id {
			return actions[i]
		}
	}
	t.Fatalf("active action %q was not returned", id)
	return dto.ActiveAction{}
}
