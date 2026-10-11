package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/server/repositories"
)

func TestServerSSHKeysUniqueIndexRepairsLegacySchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	// The production legacy import has no composite primary key or unique index.
	require.NoError(t, db.Exec(`CREATE TABLE server_ssh_keys (
		server_id char(26) NOT NULL, ssh_key_id char(26) NOT NULL,
		created_at timestamp NULL, updated_at timestamp NULL
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO server_ssh_keys (server_id, ssh_key_id) VALUES ('existing', 'existing-key')`).Error)
	repo := repositories.NewSSHKeyRepository(db)
	ctx := context.Background()
	require.Error(t, repo.AttachToServer(ctx, "server", "key"))
	require.NoError(t, addServerSSHKeysUniqueIndexUp(db))
	require.NoError(t, addServerSSHKeysUniqueIndexUp(db))
	require.NoError(t, repo.AttachToServer(ctx, "existing", "existing-key"))
	require.NoError(t, repo.AttachToServer(ctx, "server", "key"))
	require.NoError(t, repo.AttachToServer(ctx, "server", "key"))
	var count int64
	require.NoError(t, db.Table("server_ssh_keys").Count(&count).Error)
	require.EqualValues(t, 2, count)
}
