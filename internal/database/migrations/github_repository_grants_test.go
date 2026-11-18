package migrations

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestGitHubRepositoryMigrationPreservesLegacyWithoutGranting(t *testing.T) {
	dsn := os.Getenv("LAUNCH_GITHUB_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set LAUNCH_GITHUB_TEST_POSTGRES_DSN for PostgreSQL migration test")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("git_migration_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); pool, _ := admin.DB(); pool.Close() }()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	query.Set("client_encoding", "UTF8")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, db.Exec("CREATE TABLE users (id varchar(26) PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE teams (id varchar(26) PRIMARY KEY, user_id varchar(26) REFERENCES users(id) ON DELETE CASCADE)").Error)
	require.NoError(t, db.Exec("INSERT INTO users VALUES ('former-creator'),('team-owner')").Error)
	require.NoError(t, db.Exec("INSERT INTO teams VALUES ('team-a','team-owner')").Error)
	require.NoError(t, db.Exec("CREATE TABLE source_controls (id varchar(26) PRIMARY KEY, provider varchar(255), provider_id varchar(255), team_id varchar(26) CONSTRAINT fk_source_controls_team REFERENCES teams(id) ON DELETE CASCADE,user_id varchar(26) CONSTRAINT fk_source_controls_user REFERENCES users(id) ON DELETE CASCADE)").Error)
	require.NoError(t, db.Exec("CREATE TABLE source_control_repositories (id bigint PRIMARY KEY, source_control_id varchar(26) REFERENCES source_controls(id), full_name varchar(255))").Error)
	require.NoError(t, db.Exec("CREATE TABLE legacy_workloads (repository_id bigint REFERENCES source_control_repositories(id))").Error)
	require.NoError(t, db.Exec("INSERT INTO source_controls VALUES ('existing', 'github', '42', 'team-a','former-creator')").Error)
	require.NoError(t, db.Exec("INSERT INTO source_control_repositories VALUES (91,'existing','org/private')").Error)
	require.NoError(t, db.Exec("INSERT INTO legacy_workloads VALUES (91)").Error)
	require.NoError(t, db.Exec("CREATE TABLE git_hub_installations (id char(26) PRIMARY KEY, host text NOT NULL, app_id text NOT NULL, installation_id text NOT NULL, status text NOT NULL, generation bigint NOT NULL DEFAULT 1)").Error)
	require.NoError(t, db.Exec("INSERT INTO git_hub_installations VALUES ('existing-installation','github.com','7','42','active',7)").Error)
	require.NoError(t, githubRepositoryGrantsUp(db))
	require.NoError(t, githubRepositoryGrantsUp(db))
	var installation struct {
		Generation         uint64
		RepositoryRevision uint64
	}
	require.NoError(t, db.Table("git_hub_installations").Where("id = ?", "existing-installation").First(&installation).Error)
	require.Equal(t, uint64(7), installation.Generation)
	require.Equal(t, uint64(1), installation.RepositoryRevision)
	require.NoError(t, db.Exec("DELETE FROM users WHERE id = ?", "former-creator").Error)
	require.True(t, db.Migrator().HasConstraint(&sourceControlWithFK{}, "Team"))
	require.False(t, db.Migrator().HasConstraint(&sourceControlWithFK{}, "User"))
	var status string
	require.NoError(t, db.Table("source_controls").Select("status").Where("id = ?", "existing").Scan(&status).Error)
	require.Equal(t, "reconnect_required", status)
	var row struct {
		Selected     bool
		Available    bool
		GrantVersion uint64
	}
	require.NoError(t, db.Table("source_control_repositories").Where("id = ?", 91).Scan(&row).Error)
	require.False(t, row.Selected)
	require.False(t, row.Available)
	require.Equal(t, uint64(1), row.GrantVersion)
	var count int64
	require.NoError(t, db.Table("legacy_workloads").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.True(t, db.Migrator().HasIndex("source_control_repositories", "github_connection_repository_id"))
}
