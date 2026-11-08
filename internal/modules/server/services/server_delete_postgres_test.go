package services

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

func TestDeleteCustomServerSerializesConcurrentWorkloadInsertion(t *testing.T) {
	dsn := os.Getenv("SERVER_DELETE_TEST_DSN")
	if dsn == "" {
		dsn = os.Getenv("MIGRATION_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("set SERVER_DELETE_TEST_DSN to a disposable PostgreSQL database")
	}
	for _, table := range []string{"sites", "docker_projects"} {
		t.Run(table, func(t *testing.T) {
			schema := "delete_service_" + strings.ToLower(util.NewULID())
			scopedDSN := dsn
			if strings.Contains(dsn, "://") {
				parsed, err := url.Parse(dsn)
				require.NoError(t, err)
				query := parsed.Query()
				query.Set("search_path", schema)
				parsed.RawQuery = query.Encode()
				scopedDSN = parsed.String()
			} else {
				scopedDSN += " search_path=" + schema
			}
			db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(8)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
			t.Cleanup(func() { require.NoError(t, db.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
			for _, statement := range []string{
				"CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, name TEXT, provider TEXT, status TEXT)",
				"CREATE TABLE sites (id TEXT PRIMARY KEY, server_id TEXT REFERENCES servers(id) ON DELETE CASCADE)",
				"CREATE TABLE docker_projects (id TEXT PRIMARY KEY, server_id TEXT REFERENCES servers(id) ON DELETE CASCADE, deleted_at TIMESTAMP)",
			} {
				require.NoError(t, db.Exec(statement).Error)
			}
			require.NoError(t, db.Exec("INSERT INTO servers VALUES (?, ?, ?, ?, ?)", "server-a", "team-a", "Custom server", "custom_server", "running").Error)
			log := zerolog.Nop()
			svc := NewService(ServiceDeps{Dependencies: service.Dependencies{DB: db, Logger: &log}, Repos: repositories.NewRegistry(db)})
			atDelete, releaseDelete := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(releaseDelete) }) }
			require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:block-server-delete", func(tx *gorm.DB) {
				if tx.Statement.Table == "servers" {
					close(atDelete)
					select {
					case <-releaseDelete:
					case <-tx.Statement.Context.Done():
					}
				}
			}))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			deleteError, deleteDone := make(chan error, 1), make(chan struct{})
			go func() {
				deleteError <- svc.DeleteServer(ctx, "server-a", "team-a", "staff-admin")
				close(deleteDone)
			}()
			defer func() { release(); <-deleteDone }()
			select {
			case <-atDelete:
			case <-ctx.Done():
				t.Fatal("service did not reach server deletion")
			}
			connection, err := sqlDB.Conn(ctx)
			require.NoError(t, err)
			defer func() { release(); _ = connection.Close() }()
			var pid int
			require.NoError(t, connection.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&pid))
			insertError := make(chan error, 1)
			go func() {
				_, err := connection.ExecContext(ctx, "INSERT INTO "+table+" (id, server_id) VALUES ($1, $2)", "new-workload", "server-a")
				insertError <- err
			}()
			require.Eventually(t, func() bool {
				var wait string
				err := db.Raw("SELECT COALESCE(wait_event_type, '') FROM pg_stat_activity WHERE pid = ?", pid).Scan(&wait).Error
				return err == nil && wait == "Lock"
			}, 2*time.Second, 10*time.Millisecond, "FK insertion must wait after resource checks until custom server deletion commits")
			release()
			require.NoError(t, <-deleteError)
			err = <-insertError
			require.Error(t, err)
			var state interface{ SQLState() string }
			require.True(t, errors.As(err, &state))
			require.Equal(t, "23503", state.SQLState())
			var count int64
			require.NoError(t, db.Table("servers").Count(&count).Error)
			require.Zero(t, count)
			require.NoError(t, db.Table(table).Count(&count).Error)
			require.Zero(t, count, "a newly created workload must not be silently cascaded away")
		})
	}
}
