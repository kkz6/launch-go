package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
)

func TestDismissIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PlatformUpdateDismissal{}))
	repository := NewDismissalRepository(db)

	require.NoError(t, repository.Dismiss(context.Background(), "user-1", "update-1"))
	require.NoError(t, repository.Dismiss(context.Background(), "user-1", "update-1"))

	var count int64
	require.NoError(t, db.Model(&models.PlatformUpdateDismissal{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
