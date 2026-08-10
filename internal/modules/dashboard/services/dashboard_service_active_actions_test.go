package services

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestActiveActionsIncludesDeploymentsCommandsAndServerTasks(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	for _, statement := range []string{
		`CREATE TABLE servers (
			id TEXT PRIMARY KEY, team_id TEXT, name TEXT
		)`,
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
		`CREATE TABLE docker_databases (
			id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT, name TEXT, project_id TEXT
		)`,
		`CREATE TABLE commands (
			id TEXT PRIMARY KEY, team_id TEXT, site_id TEXT, command TEXT,
			status TEXT, created_at DATETIME
		)`,
		`CREATE TABLE tasks (
			id TEXT PRIMARY KEY, server_id TEXT, site_id TEXT, name TEXT,
			status TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE backups (id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT)`,
		`CREATE TABLE backup_jobs (
			id TEXT PRIMARY KEY, team_id TEXT, backup_id TEXT, status TEXT,
			task_id TEXT, error TEXT, created_at DATETIME, updated_at DATETIME
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
		`INSERT INTO servers (id, team_id, name)
			VALUES
			('server-1', 'team-1', 'Production'),
			('server-2', 'team-2', 'Other team')`,
		`INSERT INTO sites (id, address, server_id)
			VALUES ('site-1', 'example.com', 'server-1')`,
		`INSERT INTO deployments (id, team_id, status, site_id, task_id, created_at)
			VALUES
			('site-deploying', 'team-1', 'installing', 'site-1', 'task-1', '2026-07-27 10:00:00'),
			('site-finished', 'team-1', 'finished', 'site-1', 'task-2', '2026-07-27 09:00:00')`,
		`INSERT INTO docker_databases (id, name, project_id)
			VALUES ('database-1', 'Primary database', 'project-1')`,
		`INSERT INTO docker_deployments (
			id, team_id, status, target_type, target_id, server_id,
			task_id, started_at, created_at
		) VALUES (
			'database-deploying', 'team-1', 'deploying', 'database',
			'database-1', 'server-1', 'task-3',
			'2026-07-27 10:01:00', '2026-07-27 10:01:00'
		)`,
		`INSERT INTO commands (id, team_id, site_id, command, status, created_at)
			VALUES
			('command-running', 'team-1', 'site-1', 'php artisan migrate --force', 'running', '2026-07-27 10:02:00'),
			('command-finished', 'team-1', 'site-1', 'php artisan about', 'finished', '2026-07-27 10:03:00')`,
		`INSERT INTO tasks (id, server_id, site_id, name, status, created_at, updated_at)
			VALUES
			('agent-update', 'server-1', NULL, 'Update Launch Agent', 'running', '2026-07-27 10:04:00', '2026-07-27 10:04:00'),
			('task-3', 'server-1', NULL, 'Deploy database', 'running', '2026-07-27 10:01:00', '2026-07-27 10:01:00'),
			('finished-task', 'server-1', NULL, 'Install Redis', 'finished', '2026-07-27 10:05:00', '2026-07-27 10:05:00'),
			('other-team-task', 'server-2', NULL, 'Update Launch Agent', 'running', '2026-07-27 10:06:00', '2026-07-27 10:06:00')`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}

	logger := zerolog.Nop()
	service := NewDashboardService(db, &logger, nil)

	actions, err := service.ActiveActions(context.Background(), "team-1")
	require.NoError(t, err)
	require.Len(t, actions, 4)

	require.Equal(t, "agent-update", actions[0].ID)
	require.Equal(t, "task", actions[0].Kind)
	require.Equal(t, "Update Launch Agent", actions[0].Label)
	require.Equal(t, "Production", actions[0].Description)
	require.Equal(t, "server", actions[0].TargetType)
	require.Equal(t, "server-1", actions[0].TargetID)
	require.Equal(t, "agent-update", *actions[0].TaskID)
	require.Equal(t, "running", actions[0].Status)

	require.Equal(t, "command-running", actions[1].ID)
	require.Equal(t, "command", actions[1].Kind)
	require.Equal(t, "example.com", actions[1].Label)
	require.Equal(t, "php artisan migrate --force", actions[1].Description)
	require.Equal(t, "site", actions[1].TargetType)
	require.Equal(t, "site-1", actions[1].TargetID)
	require.Equal(t, "running", actions[1].Status)

	require.Equal(t, "database-deploying", actions[2].ID)
	require.Equal(t, "Primary database", actions[2].Label)
	require.Equal(t, "project-1", actions[2].ProjectID)
	require.Equal(t, "database", actions[2].TargetType)
	require.Equal(t, "deploying", actions[2].Status)

	require.Equal(t, "site-deploying", actions[3].ID)
	require.Equal(t, "example.com", actions[3].Label)
	require.Equal(t, "deploying", actions[3].Status)
}

// activeActionsTaskFixture builds the minimum schema the tasks branch of
// ActiveActions touches, so these cases stay readable next to the broader
// fixture above.
func activeActionsTaskFixture(t *testing.T, taskRows string) *DashboardService {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	for _, statement := range []string{
		`CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, name TEXT)`,
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
		`CREATE TABLE docker_databases (
			id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT, name TEXT, project_id TEXT
		)`,
		`CREATE TABLE commands (
			id TEXT PRIMARY KEY, team_id TEXT, site_id TEXT, command TEXT,
			status TEXT, created_at DATETIME
		)`,
		`CREATE TABLE tasks (
			id TEXT PRIMARY KEY, server_id TEXT, site_id TEXT, name TEXT,
			status TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE backups (id TEXT PRIMARY KEY, team_id TEXT, server_id TEXT)`,
		`CREATE TABLE backup_jobs (
			id TEXT PRIMARY KEY, team_id TEXT, backup_id TEXT, status TEXT,
			task_id TEXT, error TEXT, created_at DATETIME, updated_at DATETIME
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
		`INSERT INTO servers (id, team_id, name) VALUES ('server-1', 'team-1', 'Production')`,
		`INSERT INTO sites (id, address, server_id) VALUES ('site-1', 'example.com', 'server-1')`,
		taskRows,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}

	logger := zerolog.Nop()
	return NewDashboardService(db, &logger, nil)
}

// A site PHP switch is server work that belongs to a site. Reporting it as a
// bare server action left the UI unable to link to the site — the domain was
// only readable inside the task name.
func TestActiveActionsTargetsTheSiteForSiteScopedTasks(t *testing.T) {
	t.Parallel()

	service := activeActionsTaskFixture(t, `INSERT INTO tasks
		(id, server_id, site_id, name, status, created_at, updated_at) VALUES
		('php-switch', 'server-1', 'site-1', 'Switch example.com to PHP 8.3', 'running',
		 '2026-07-27 10:00:00', '2026-07-27 10:00:00')`)

	actions, err := service.ActiveActions(context.Background(), "team-1")
	require.NoError(t, err)
	require.Len(t, actions, 1)

	require.Equal(t, "task", actions[0].Kind)
	require.Equal(t, "Switch example.com to PHP 8.3", actions[0].Label)
	require.Equal(t, "site", actions[0].TargetType)
	require.Equal(t, "site-1", actions[0].TargetID)
	require.Equal(t, "example.com", actions[0].Description)
	require.Equal(t, "server-1", actions[0].ServerID, "the server is still reported for routing")
}

// A failed patch or switch used to drop out of the list the moment it
// finished — exactly when the user wants to open its log.
func TestActiveActionsKeepsRecentlyFailedTasks(t *testing.T) {
	t.Parallel()

	recent := time.Now().Add(-time.Minute).Format("2006-01-02 15:04:05")
	stale := time.Now().Add(-2 * failedActionRetention).Format("2006-01-02 15:04:05")

	service := activeActionsTaskFixture(t, `INSERT INTO tasks
		(id, server_id, site_id, name, status, created_at, updated_at) VALUES
		('patch-failed', 'server-1', NULL, 'Patch PHP 8.3', 'failed', '`+recent+`', '`+recent+`'),
		('patch-timeout', 'server-1', NULL, 'Patch PHP 8.2', 'timeout', '`+recent+`', '`+recent+`'),
		('patch-old-failure', 'server-1', NULL, 'Patch PHP 8.1', 'failed', '`+stale+`', '`+stale+`'),
		('install-finished', 'server-1', NULL, 'Install Redis', 'finished', '`+recent+`', '`+recent+`')`)

	actions, err := service.ActiveActions(context.Background(), "team-1")
	require.NoError(t, err)

	ids := make(map[string]string, len(actions))
	for _, action := range actions {
		ids[action.ID] = action.Status
	}

	require.Contains(t, ids, "patch-failed")
	require.Equal(t, "failed", ids["patch-failed"])
	require.Contains(t, ids, "patch-timeout")

	require.NotContains(t, ids, "patch-old-failure",
		"a failure past the retention window is history, not current work")
	require.NotContains(t, ids, "install-finished",
		"successes still drop out immediately; only failures are retained")
}

// ActiveActions runs four independent queries and returns on the first
// failure. Dropping one table at a time reaches each error path in turn,
// because a query only runs once the ones before it have succeeded.
func TestActiveActionsSurfacesQueryFailures(t *testing.T) {
	t.Parallel()

	schema := map[string]string{
		"servers":             `CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, name TEXT)`,
		"sites":               `CREATE TABLE sites (id TEXT PRIMARY KEY, address TEXT, server_id TEXT)`,
		"deployments":         `CREATE TABLE deployments (id TEXT PRIMARY KEY, team_id TEXT, status TEXT, site_id TEXT, task_id TEXT, created_at DATETIME)`,
		"docker_deployments":  `CREATE TABLE docker_deployments (id TEXT PRIMARY KEY, team_id TEXT, status TEXT, target_type TEXT, target_id TEXT, server_id TEXT, task_id TEXT, started_at DATETIME, created_at DATETIME)`,
		"docker_applications": `CREATE TABLE docker_applications (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)`,
		"docker_composes":     `CREATE TABLE docker_composes (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)`,
		"docker_databases":    `CREATE TABLE docker_databases (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)`,
		"commands":            `CREATE TABLE commands (id TEXT PRIMARY KEY, team_id TEXT, site_id TEXT, command TEXT, status TEXT, created_at DATETIME)`,
		"tasks":               `CREATE TABLE tasks (id TEXT PRIMARY KEY, server_id TEXT, site_id TEXT, name TEXT, status TEXT, created_at DATETIME, updated_at DATETIME)`,
	}

	// Each entry omits the one table whose absence fails that query first.
	for _, omit := range []string{"deployments", "docker_deployments", "commands", "tasks"} {
		t.Run("missing "+omit, func(t *testing.T) {
			db, err := gorm.Open(
				sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
				&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
			)
			require.NoError(t, err)

			for name, statement := range schema {
				if name == omit {
					continue
				}
				require.NoError(t, db.Exec(statement).Error)
			}

			logger := zerolog.Nop()
			service := NewDashboardService(db, &logger, nil)

			actions, err := service.ActiveActions(context.Background(), "team-1")
			require.Error(t, err, "a broken %s query must surface, not return a partial list", omit)
			require.Nil(t, actions)
		})
	}
}
