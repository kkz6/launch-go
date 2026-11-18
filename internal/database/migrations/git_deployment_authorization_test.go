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

func TestGitDeploymentAndWebhookMigrationsPreserveHistory(t *testing.T) {
	dsn := os.Getenv("LAUNCH_GITHUB_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set LAUNCH_GITHUB_TEST_POSTGRES_DSN for PostgreSQL migration tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("git_deployment_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); pool, _ := admin.DB(); pool.Close() }()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	q := parsed.Query()
	q.Set("search_path", schema)
	q.Set("client_encoding", "UTF8")
	parsed.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	for _, sql := range []string{"CREATE TABLE deployments (id text primary key, site_id text)", "CREATE TABLE sites (id text primary key)", "INSERT INTO deployments VALUES ('existing','site')", "INSERT INTO sites VALUES ('site')"} {
		require.NoError(t, db.Exec(sql).Error)
	}
	for iteration := 0; iteration < 2; iteration++ {
		require.NoError(t, gitDeploymentAuthorizationUp(db))
		for _, migration := range registeredMigrations {
			if migration.ID == "0074_11_17_000000_github_webhook_intents" {
				require.NoError(t, migration.Up(db))
			}
		}
	}
	var old struct {
		ID         string
		GitAccess  *string
		WebhookKey *string
	}
	require.NoError(t, db.Table("deployments").Where("id = ?", "existing").Take(&old).Error)
	require.Nil(t, old.GitAccess)
	require.Nil(t, old.WebhookKey)
	require.NoError(t, db.Exec("INSERT INTO deployments(id,site_id,webhook_key) VALUES ('new','site','receipt:site')").Error)
	require.Error(t, db.Exec("INSERT INTO deployments(id,site_id,webhook_key) VALUES ('duplicate','site','receipt:site')").Error)
	require.True(t, db.Migrator().HasTable("git_hub_webhook_deliveries"))
	require.True(t, db.Migrator().HasTable("git_hub_webhook_intents"))
}
