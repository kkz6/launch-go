package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestActiveActionsIncludesDeploymentsAndCommands(t *testing.T) {
	t.Parallel()

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
		`CREATE TABLE docker_databases (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)`,
		`CREATE TABLE commands (
			id TEXT PRIMARY KEY, team_id TEXT, site_id TEXT, command TEXT,
			status TEXT, created_at DATETIME
		)`,
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
	} {
		require.NoError(t, db.Exec(statement).Error)
	}

	logger := zerolog.Nop()
	service := NewDashboardService(db, &logger, nil)

	actions, err := service.ActiveActions(context.Background(), "team-1")
	require.NoError(t, err)
	require.Len(t, actions, 3)

	require.Equal(t, "command-running", actions[0].ID)
	require.Equal(t, "command", actions[0].Kind)
	require.Equal(t, "example.com", actions[0].Label)
	require.Equal(t, "php artisan migrate --force", actions[0].Description)
	require.Equal(t, "site", actions[0].TargetType)
	require.Equal(t, "site-1", actions[0].TargetID)
	require.Equal(t, "running", actions[0].Status)

	require.Equal(t, "database-deploying", actions[1].ID)
	require.Equal(t, "Primary database", actions[1].Label)
	require.Equal(t, "project-1", actions[1].ProjectID)
	require.Equal(t, "database", actions[1].TargetType)
	require.Equal(t, "deploying", actions[1].Status)

	require.Equal(t, "site-deploying", actions[2].ID)
	require.Equal(t, "example.com", actions[2].Label)
	require.Equal(t, "deploying", actions[2].Status)
}
