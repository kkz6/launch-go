package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGetFailedQueuesReturnsCheckedStoppedWorkersForTeam(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE servers (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			name TEXT NOT NULL,
			archived_at DATETIME NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE sites (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			server_id TEXT NOT NULL,
			address TEXT NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE queues (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			server_id TEXT NOT NULL,
			site_id TEXT NOT NULL,
			queue TEXT NOT NULL,
			queue_connection TEXT NOT NULL,
			installed_at DATETIME NULL,
			installation_failed_at DATETIME NULL,
			uninstallation_requested_at DATETIME NULL,
			last_status_check DATETIME NULL,
			running BOOLEAN NOT NULL DEFAULT FALSE
		)
	`).Error)

	checkedAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, db.Exec(`
		INSERT INTO servers (id, team_id, name, archived_at) VALUES
			('server-1', 'team-1', 'Production', NULL),
			('server-2', 'team-2', 'Other team', NULL),
			('server-archived', 'team-1', 'Archived', ?)
	`, checkedAt).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO sites (id, team_id, server_id, address) VALUES
			('site-1', 'team-1', 'server-1', 'example.test'),
			('site-2', 'team-2', 'server-2', 'other.test'),
			('site-archived', 'team-1', 'server-archived', 'archived.test')
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO queues
			(id, team_id, server_id, site_id, queue, queue_connection, installed_at, installation_failed_at, last_status_check, running)
		VALUES
			('stopped', 'team-1', 'server-1', 'site-1', 'default', 'redis', ?, NULL, ?, FALSE),
			('healthy', 'team-1', 'server-1', 'site-1', 'emails', 'redis', ?, NULL, ?, TRUE),
			('unchecked', 'team-1', 'server-1', 'site-1', 'reports', 'redis', ?, NULL, NULL, FALSE),
			('pending', 'team-1', 'server-1', 'site-1', 'pending', 'redis', NULL, NULL, ?, FALSE),
			('install-failed', 'team-1', 'server-1', 'site-1', 'failed', 'redis', ?, ?, ?, FALSE),
			('other-team', 'team-2', 'server-2', 'site-2', 'default', 'redis', ?, NULL, ?, FALSE),
			('archived', 'team-1', 'server-archived', 'site-archived', 'default', 'redis', ?, NULL, ?, FALSE)
	`,
		checkedAt, checkedAt,
		checkedAt, checkedAt,
		checkedAt,
		checkedAt,
		checkedAt, checkedAt, checkedAt,
		checkedAt, checkedAt,
		checkedAt, checkedAt,
	).Error)

	log := zerolog.Nop()
	service := NewDashboardService(db, &log, nil)

	queues, err := service.getFailedQueues(context.Background(), "team-1")

	require.NoError(t, err)
	require.Len(t, queues, 1)
	require.Equal(t, "stopped", queues[0].ID)
	require.Equal(t, "default", queues[0].Name)
	require.Equal(t, "redis", queues[0].Connection)
	require.Equal(t, "example.test", queues[0].SiteName)
	require.Equal(t, "Production", queues[0].ServerName)
	require.WithinDuration(t, checkedAt, *queues[0].LastStatusCheck, time.Second)
}

func TestGetFailedQueuesReturnsQueryError(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)

	log := zerolog.Nop()
	service := NewDashboardService(db, &log, nil)
	queues, err := service.getFailedQueues(context.Background(), "team-1")

	require.Error(t, err)
	require.Nil(t, queues)
}

func TestGetDashboardIncludesQueueHealthAndPropagatesQueueQueryErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		withQueues bool
		wantErr    bool
	}{
		{name: "includes queue health", withQueues: true},
		{name: "queue query error", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, err := gorm.Open(
				sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
			)
			require.NoError(t, err)
			for _, statement := range []string{
				`CREATE TABLE servers (id TEXT, team_id TEXT, name TEXT, connected BOOLEAN, provider TEXT, type TEXT, created_at DATETIME, archived_at DATETIME, deleted_at DATETIME)`,
				`CREATE TABLE sites (id TEXT, team_id TEXT, server_id TEXT, address TEXT, deleted_at DATETIME)`,
				`CREATE TABLE deployments (id TEXT, site_id TEXT, user_id TEXT, status TEXT, created_at DATETIME, commit_sha TEXT, commit_message TEXT)`,
				`CREATE TABLE users (id TEXT, name TEXT)`,
				`CREATE TABLE docker_applications (server_id TEXT, deleted_at DATETIME)`,
				`CREATE TABLE docker_composes (server_id TEXT, deleted_at DATETIME)`,
				`CREATE TABLE docker_databases (server_id TEXT, deleted_at DATETIME)`,
			} {
				require.NoError(t, db.Exec(statement).Error)
			}
			if tt.withQueues {
				require.NoError(t, db.Exec(`CREATE TABLE queues (
					id TEXT, team_id TEXT, server_id TEXT, site_id TEXT, queue TEXT,
					queue_connection TEXT, installed_at DATETIME, installation_failed_at DATETIME,
					uninstallation_requested_at DATETIME, last_status_check DATETIME, running BOOLEAN
				)`).Error)
			}

			log := zerolog.Nop()
			response, err := NewDashboardService(db, &log, nil).GetDashboard(context.Background(), "team-1")
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, response.FailedQueues)
			require.Empty(t, response.FailedQueues)
		})
	}
}
