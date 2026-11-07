package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

func TestDeviceAuthorizationMigrationIsIdempotentAndKeepsGrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, createDeviceAuthorizationsUp(db))
	grant := models.DeviceAuthorization{DeviceCodeHash: "secret-hash", UserCodeHash: "user-code-hash", Status: "pending", ExpiresAt: time.Now().Add(time.Minute)}
	require.NoError(t, db.Create(&grant).Error)
	require.NoError(t, createDeviceAuthorizationsUp(db))
	var saved models.DeviceAuthorization
	require.NoError(t, db.First(&saved, "id = ?", grant.ID).Error)
	require.Equal(t, grant.DeviceCodeHash, saved.DeviceCodeHash)
	require.Equal(t, "pending", saved.Status)
	require.True(t, db.Migrator().HasTable(&models.DeviceAuthorizationLimit{}))
}
